package sqlite

// Purge state is kept in a private per-user control directory, never beside a
// user-supplied ledger. The held lifecycle lock protects all cooperative
// openers until deletion and journal cleanup have completed.

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	storelock "github.com/janpereira-dev/quantum_log/internal/storage/lock"
)

type lifecycleControl struct{ root, identity, lockPath, journalPath string }
type purgeJournal struct {
	Home string `json:"home"`
}

type PurgeGuard struct {
	lifecycle, legacyQuiescence, legacyWriter *storelock.Handle
	homePath, journalPath, controlRoot        string
	createdJournal, absent                    bool
	legacyOnce                                sync.Once
	legacyErr                                 error
}

// Kept injectable so power-loss durability is tested rather than assumed.
// A platform that cannot sync its directory returns an error here and purge
// fails closed before destructive work proceeds.
var syncLifecycleControlDirectory = syncLifecycleDirectory

// SetLifecycleControlSyncForTesting lets cross-package tests model a failed
// directory sync without weakening the production fail-closed path.
func SetLifecycleControlSyncForTesting(syncDirectory func(string) error) func() {
	original := syncLifecycleControlDirectory
	syncLifecycleControlDirectory = syncDirectory
	return func() { syncLifecycleControlDirectory = original }
}

func canonicalDatabasePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve database path: %w", err)
	}
	parent := filepath.Dir(abs)
	resolved, err := filepath.EvalSymlinks(parent)
	if err == nil {
		return filepath.Join(resolved, filepath.Base(abs)), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("resolve database parent: %w", err)
	}
	return filepath.Clean(abs), nil
}

func lifecycleControlFor(databasePath string) (lifecycleControl, error) {
	identity, err := canonicalDatabasePath(databasePath)
	if err != nil {
		return lifecycleControl{}, err
	}
	root := os.Getenv("QLOG_LIFECYCLE_DIR")
	if root == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return lifecycleControl{}, fmt.Errorf("resolve qlog lifecycle control directory: %w", err)
		}
		root = filepath.Join(base, "quantum-log", "lifecycle")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return lifecycleControl{}, fmt.Errorf("resolve qlog lifecycle control directory: %w", err)
	}
	if err := ensurePrivateControlRoot(root); err != nil {
		return lifecycleControl{}, err
	}
	digest := sha256.Sum256([]byte(identity))
	id := fmt.Sprintf("%x", digest[:])
	return lifecycleControl{root, identity, filepath.Join(root, id+".lock"), filepath.Join(root, id+".purge.json")}, nil
}

func ensurePrivateControlRoot(root string) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("create qlog lifecycle control directory: %w", err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("inspect qlog lifecycle control directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refuse unsafe qlog lifecycle control directory %q", root)
	}
	return nil
}

func acquireLifecycleShared(databasePath string) (*storelock.Handle, lifecycleControl, error) {
	control, err := lifecycleControlFor(databasePath)
	if err != nil {
		return nil, lifecycleControl{}, err
	}
	handle, err := storelock.AcquireSharedCreate(control.lockPath)
	if err != nil {
		if errors.Is(err, storelock.ErrContended) {
			return nil, lifecycleControl{}, errors.New("ledger purge is in progress; retry after it completes")
		}
		return nil, lifecycleControl{}, fmt.Errorf("acquire ledger lifecycle lock: %w", err)
	}
	if err := rejectPurgeInProgressControl(control); err != nil {
		_ = handle.Close()
		return nil, lifecycleControl{}, err
	}
	return handle, control, nil
}

func acquireLifecycleExclusive(databasePath string) (*storelock.Handle, lifecycleControl, error) {
	control, err := lifecycleControlFor(databasePath)
	if err != nil {
		return nil, lifecycleControl{}, err
	}
	handle, err := storelock.AcquireExclusive(control.lockPath)
	if err != nil {
		if errors.Is(err, storelock.ErrContended) {
			return nil, lifecycleControl{}, errors.New("ledger purge is in progress; retry after it completes")
		}
		return nil, lifecycleControl{}, fmt.Errorf("acquire ledger lifecycle lock: %w", err)
	}
	return handle, control, nil
}

func rejectPurgeInProgressControl(control lifecycleControl) error {
	info, err := os.Lstat(control.journalPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect ledger purge journal: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refuse unsafe non-regular ledger purge journal %q", control.journalPath)
	}
	journal, err := readPurgeJournal(control)
	if err != nil {
		return err
	}
	if journal.Home != filepath.Dir(control.identity) {
		return fmt.Errorf("refuse invalid ledger purge journal %q", control.journalPath)
	}
	return errors.New("ledger purge is in progress; retry after it completes")
}
func readPurgeJournal(control lifecycleControl) (purgeJournal, error) {
	contents, err := os.ReadFile(control.journalPath)
	if err != nil {
		return purgeJournal{}, fmt.Errorf("read ledger purge journal: %w", err)
	}
	var journal purgeJournal
	if err := json.Unmarshal(contents, &journal); err != nil || journal.Home == "" {
		return purgeJournal{}, fmt.Errorf("refuse invalid ledger purge journal %q", control.journalPath)
	}
	return journal, nil
}
func writePurgeJournal(control lifecycleControl) error {
	contents, err := json.Marshal(purgeJournal{Home: filepath.Dir(control.identity)})
	if err != nil {
		return fmt.Errorf("encode ledger purge journal: %w", err)
	}
	temp, err := os.CreateTemp(control.root, ".qlog-purge-*")
	if err != nil {
		return fmt.Errorf("create ledger purge journal: %w", err)
	}
	name := temp.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := temp.Write(contents); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write ledger purge journal: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync ledger purge journal: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close ledger purge journal: %w", err)
	}
	if err := os.Rename(name, control.journalPath); err != nil {
		return fmt.Errorf("persist ledger purge journal: %w", err)
	}
	if err := syncLifecycleControlDirectory(control.root); err != nil {
		return fmt.Errorf("durably persist ledger purge journal: %w", err)
	}
	return nil
}

func removePurgeJournal(root, path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove ledger purge journal: %w", err)
	}
	if err := syncLifecycleControlDirectory(root); err != nil {
		return fmt.Errorf("durably remove ledger purge journal: %w", err)
	}
	return nil
}

func PreparePurge(ctx context.Context, path string) (_ *PurgeGuard, result error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	lifecycle, control, err := acquireLifecycleExclusive(abs)
	if err != nil {
		return nil, err
	}
	guard := &PurgeGuard{lifecycle: lifecycle, homePath: filepath.Dir(abs), journalPath: control.journalPath, controlRoot: control.root}
	fail := func(e error) (*PurgeGuard, error) { _ = guard.releaseAll(); return nil, e }
	if _, err := os.Lstat(control.journalPath); err == nil {
		journal, jerr := readPurgeJournal(control)
		if jerr != nil || journal.Home != filepath.Dir(control.identity) {
			return fail(fmt.Errorf("refuse invalid ledger purge journal %q", control.journalPath))
		}
		if err := safePurgeHome(guard.homePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fail(err)
		}
		if _, err := os.Lstat(guard.homePath); errors.Is(err, os.ErrNotExist) {
			guard.absent = true
			// No-home recovery only completes a prior intent; there is no tree to
			// validate or delete, but exclusive lifecycle ownership is still held.
			return guard, nil
		}
		if err := validatePresentPurgeLedger(ctx, abs, guard); err != nil {
			return fail(err)
		}
		return guard, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fail(fmt.Errorf("inspect ledger purge journal: %w", err))
	}
	if err := safePurgeHome(guard.homePath); errors.Is(err, os.ErrNotExist) {
		guard.absent = true
		return guard, nil
	} else if err != nil {
		return fail(err)
	}
	if err := validatePresentPurgeLedger(ctx, abs, guard); err != nil {
		return fail(err)
	}
	if err := writePurgeJournal(control); err != nil {
		return fail(err)
	}
	guard.createdJournal = true
	if err := guard.ReleaseForPurge(); err != nil {
		return fail(err)
	}
	return guard, nil
}

// validatePresentPurgeLedger rechecks any existing recovery target while the
// exclusive lifecycle lock is held. A stale journal is never proof of
// ownership: a foreign directory recreated at the old path must be retained.
func validatePresentPurgeLedger(ctx context.Context, abs string, guard *PurgeGuard) error {
	if _, err := os.Lstat(abs); err != nil {
		return fmt.Errorf("open local database: %w; run qlog init first", err)
	}
	q, err := storelock.AcquireExclusiveExisting(quiescenceLockPath(abs))
	if err != nil {
		return readerQuiescenceError(err)
	}
	guard.legacyQuiescence = q
	w, err := storelock.AcquireExclusiveExisting(writerLockPath(abs))
	if err != nil {
		return readerWriterLockError(err)
	}
	guard.legacyWriter = w
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(abs)+"?mode=ro&immutable=1&_pragma=query_only(1)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return fmt.Errorf("open read-only sqlite: %w", err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("open read-only sqlite: %w", err)
	}
	if err := validatePurgeOwnership(ctx, &Store{db: db}); err != nil {
		return err
	}
	return nil
}

func validatePurgeOwnership(ctx context.Context, store *Store) error {
	known, err := knownQlogMigrationIDs()
	if err != nil {
		return err
	}
	rows, err := store.db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("database schema is not a recognised qlog ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()
	applied := map[string]struct{}{}
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("read qlog migration history: %w", err)
		}
		if _, ok := known[version]; !ok {
			return fmt.Errorf("database schema has unrecognised migration %q", version)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read qlog migration history: %w", err)
	}
	if _, ok := applied["001_initial.sql"]; !ok {
		return errors.New("database schema has no qlog migration history")
	}
	for _, table := range []string{"raw_events", "projects", "schema_migrations"} {
		var name string
		if err := store.db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			return fmt.Errorf("database lacks stable qlog schema object %q", table)
		}
	}
	var integrity string
	if err := store.db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("verify qlog database integrity: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("database integrity check failed: %s", integrity)
	}
	return nil
}
func knownQlogMigrationIDs() (map[string]struct{}, error) {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded qlog migrations: %w", err)
	}
	known := map[string]struct{}{}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			known[e.Name()] = struct{}{}
		}
	}
	return known, nil
}
func safePurgeHome(home string) error {
	info, err := os.Lstat(home)
	if errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err != nil {
		return fmt.Errorf("inspect qlog home before purge: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refuse unsafe qlog home %q", home)
	}
	return filepath.WalkDir(home, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0 {
			return fmt.Errorf("refuse unsafe qlog purge topology at %q", path)
		}
		return nil
	})
}

func (g *PurgeGuard) DetachForPurge() (string, error) {
	if g == nil {
		return "", errors.New("nil qlog ledger purge guard")
	}
	if err := g.ReleaseForPurge(); err != nil {
		return "", err
	}
	if g.absent {
		return "", nil
	}
	if err := safePurgeHome(g.homePath); err != nil {
		return "", err
	}
	return g.homePath, nil
}
func (g *PurgeGuard) ReleaseForPurge() error {
	if g == nil {
		return nil
	}
	g.legacyOnce.Do(func() {
		if g.legacyWriter != nil {
			g.legacyErr = errors.Join(g.legacyErr, g.legacyWriter.Close())
		}
		if g.legacyQuiescence != nil {
			g.legacyErr = errors.Join(g.legacyErr, g.legacyQuiescence.Close())
		}
	})
	return g.legacyErr
}
func (g *PurgeGuard) Complete() error {
	if g == nil {
		return nil
	}
	if err := g.ReleaseForPurge(); err != nil {
		return err
	}
	if _, err := os.Lstat(g.homePath); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return fmt.Errorf("cannot complete qlog purge while ledger remains %q", g.homePath)
		}
		return fmt.Errorf("inspect qlog home after purge: %w", err)
	}
	if err := removePurgeJournal(g.controlRoot, g.journalPath); err != nil {
		return err
	}
	return g.lifecycle.Close()
}
func (g *PurgeGuard) Abort() error {
	if g == nil {
		return nil
	}
	result := g.ReleaseForPurge()
	if g.createdJournal {
		result = errors.Join(result, removePurgeJournal(g.controlRoot, g.journalPath))
	}
	return errors.Join(result, g.lifecycle.Close())
}
func (g *PurgeGuard) Close() error      { return g.Abort() }
func (g *PurgeGuard) releaseAll() error { return errors.Join(g.ReleaseForPurge(), g.lifecycle.Close()) }
func purgeMarkerPath(databasePath string) string {
	c, err := lifecycleControlFor(databasePath)
	if err != nil {
		return ""
	}
	return c.journalPath
}
