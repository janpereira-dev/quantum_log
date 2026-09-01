package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	storelock "github.com/janpereira-dev/quantum_log/internal/storage/lock"
)

// PurgeGuard owns a destructive ledger removal. Its marker and detached
// directory live beside (not inside) the qlog home: RemoveAll can therefore
// never unlink the signal that blocks cooperative openers. The ledger home is
// atomically renamed before removal, which also makes a partially removed
// ledger unreachable when a process is interrupted.
type PurgeGuard struct {
	quiescence   *storelock.Handle
	homePath     string
	detachedPath string
	markerPath   string
	detached     bool
	closeOnce    sync.Once
	closeErr     error
}

// PreparePurge validates an idle qlog ledger before destructive removal. An
// interrupted purge is resumable: if its detached directory remains, callers
// receive a guard that can remove it without recreating or reopening a ledger.
func PreparePurge(ctx context.Context, path string) (_ *PurgeGuard, result error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	homePath := filepath.Dir(absolutePath)
	markerPath := purgeMarkerPath(absolutePath)
	detachedPath := purgeDetachedPath(absolutePath)

	if marker, err := os.Lstat(markerPath); err == nil {
		if err := validatePurgeMarker(markerPath, marker); err != nil {
			return nil, err
		}
		if info, statErr := os.Lstat(detachedPath); statErr == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("refuse unsafe detached qlog ledger %q", detachedPath)
			}
			return &PurgeGuard{homePath: homePath, detachedPath: detachedPath, markerPath: markerPath, detached: true}, nil
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect detached qlog ledger: %w", statErr)
		}
		// A completed deletion can be interrupted after RemoveAll but before the
		// marker cleanup. It is safe to finish that cleanup only when the original
		// home no longer exists; callers can then treat purge as idempotent.
		if _, homeErr := os.Lstat(homePath); errors.Is(homeErr, os.ErrNotExist) {
			return &PurgeGuard{homePath: homePath, detachedPath: detachedPath, markerPath: markerPath, detached: true}, nil
		} else if homeErr != nil {
			return nil, fmt.Errorf("inspect qlog home: %w", homeErr)
		}
	}
	if _, err := os.Stat(absolutePath); err != nil {
		return nil, fmt.Errorf("open local database: %w; run qlog init first", err)
	}
	quiescence, err := storelock.AcquireExclusiveExisting(quiescenceLockPath(absolutePath))
	if err != nil {
		return nil, readerQuiescenceError(err)
	}
	defer func() {
		if result != nil {
			result = fmt.Errorf("prepare qlog ledger purge: %w", result)
			_ = quiescence.Close()
		}
	}()
	if _, err := os.Stat(writerLockPath(absolutePath)); err != nil {
		return nil, readerWriterLockError(err)
	}
	if err := rejectActiveWAL(absolutePath); err != nil {
		return nil, err
	}
	dsn := "file:" + filepath.ToSlash(absolutePath) + "?mode=ro&immutable=1&_pragma=query_only(1)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open read-only sqlite: %w", err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("open read-only sqlite: %w", err)
	}
	if err := validatePurgeOwnership(ctx, store); err != nil {
		return nil, err
	}
	if err := acquirePurgeMarker(markerPath); err != nil {
		return nil, err
	}
	return &PurgeGuard{quiescence: quiescence, homePath: homePath, detachedPath: detachedPath, markerPath: markerPath}, nil
}

// validatePurgeOwnership accepts any recognisable historical qlog migration
// history. A purge must not depend on the current binary being able to query a
// newer schema, but it must still refuse foreign or corrupt SQLite files.
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
	applied := make(map[string]struct{})
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("read qlog migration history: %w", err)
		}
		if _, found := known[version]; !found {
			return fmt.Errorf("database schema has unrecognised migration %q", version)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read qlog migration history: %w", err)
	}
	if _, found := applied["001_initial.sql"]; !found {
		return errors.New("database schema has no qlog migration history")
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

// knownQlogMigrationIDs makes the ownership boundary follow the embedded,
// authoritative migration source rather than a broad filename convention.
func knownQlogMigrationIDs() (map[string]struct{}, error) {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded qlog migrations: %w", err)
	}
	known := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			known[entry.Name()] = struct{}{}
		}
	}
	return known, nil
}

func acquirePurgeMarker(markerPath string) error {
	info, err := os.Lstat(markerPath)
	if err == nil {
		return validatePurgeMarker(markerPath, info)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect ledger purge marker: %w", err)
	}
	marker, err := os.OpenFile(markerPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			info, statErr := os.Lstat(markerPath)
			if statErr != nil {
				return fmt.Errorf("inspect ledger purge marker after concurrent creation: %w", statErr)
			}
			return validatePurgeMarker(markerPath, info)
		}
		return fmt.Errorf("mark ledger purge in progress: %w", err)
	}
	if err := marker.Close(); err != nil {
		_ = os.Remove(markerPath)
		return fmt.Errorf("close ledger purge marker: %w", err)
	}
	return nil
}

func validatePurgeMarker(markerPath string, info os.FileInfo) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refuse unsafe non-regular ledger purge marker %q", markerPath)
	}
	return nil
}

func rejectPurgeInProgress(databasePath string) error {
	info, err := os.Lstat(purgeMarkerPath(databasePath))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect ledger purge marker: %w", err)
	}
	if err := validatePurgeMarker(purgeMarkerPath(databasePath), info); err != nil {
		return err
	}
	return errors.New("ledger purge is in progress; retry after it completes")
}

// ReleaseForPurge closes Windows-incompatible lock handles only after the
// external marker is durable. Openers remain blocked while the locks are gone.
func (g *PurgeGuard) ReleaseForPurge() error {
	if g == nil || g.quiescence == nil {
		return nil
	}
	g.closeOnce.Do(func() { g.closeErr = g.quiescence.Close() })
	return g.closeErr
}

// DetachForPurge atomically moves the whole qlog home out of its public path.
// The returned path is the only directory a caller may recursively remove.
func (g *PurgeGuard) DetachForPurge() (string, error) {
	if g == nil {
		return "", errors.New("nil qlog ledger purge guard")
	}
	if err := g.ReleaseForPurge(); err != nil {
		return "", err
	}
	if g.detached {
		return g.detachedPath, nil
	}
	if _, err := os.Lstat(g.detachedPath); err == nil {
		return "", fmt.Errorf("refuse to replace existing detached qlog ledger %q", g.detachedPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect detached qlog ledger: %w", err)
	}
	if err := os.Rename(g.homePath, g.detachedPath); err != nil {
		return "", fmt.Errorf("atomically detach qlog ledger: %w", err)
	}
	g.detached = true
	return g.detachedPath, nil
}

// Complete removes durable purge state after the detached tree is gone.
func (g *PurgeGuard) Complete() error {
	if g == nil {
		return nil
	}
	if err := g.ReleaseForPurge(); err != nil {
		return err
	}
	if _, err := os.Lstat(g.detachedPath); err == nil {
		return fmt.Errorf("cannot complete qlog purge while detached ledger remains %q", g.detachedPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect detached qlog ledger: %w", err)
	}
	if err := os.Remove(g.markerPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove ledger purge marker: %w", err)
	}
	return nil
}

// Abort restores a successfully detached directory if no destructive removal
// was completed. If restoration fails, the external marker remains to fail
// closed rather than allowing a writer into partially deleted data.
func (g *PurgeGuard) Abort() error {
	if g == nil {
		return nil
	}
	result := g.ReleaseForPurge()
	if g.detached {
		if _, err := os.Lstat(g.detachedPath); err == nil {
			if err := os.Rename(g.detachedPath, g.homePath); err != nil {
				return errors.Join(result, fmt.Errorf("restore detached qlog ledger: %w", err))
			}
			g.detached = false
		} else if !errors.Is(err, os.ErrNotExist) {
			return errors.Join(result, fmt.Errorf("inspect detached qlog ledger: %w", err))
		}
	}
	if err := os.Remove(g.markerPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		result = errors.Join(result, fmt.Errorf("remove ledger purge marker: %w", err))
	}
	return result
}

func (g *PurgeGuard) Close() error { return g.Abort() }

func purgeMarkerPath(databasePath string) string {
	home := filepath.Dir(databasePath)
	digest := sha256.Sum256([]byte(filepath.Clean(home)))
	return filepath.Join(filepath.Dir(home), ".qlog-purge-"+fmt.Sprintf("%x", digest[:8])+".pending")
}

func purgeDetachedPath(databasePath string) string {
	home := filepath.Dir(databasePath)
	digest := sha256.Sum256([]byte(filepath.Clean(home)))
	return filepath.Join(filepath.Dir(home), ".qlog-purge-"+fmt.Sprintf("%x", digest[:8])+".deleting")
}
