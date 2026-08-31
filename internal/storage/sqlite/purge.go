package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	storelock "github.com/janpereira-dev/quantum_log/internal/storage/lock"
)

// PurgeGuard proves that a ledger is initialized and makes it unreachable to
// cooperative writers until the destructive operation either succeeds or is
// aborted. The marker remains after the lock handles are closed so Windows can
// remove the directory without opening a writer race.
type PurgeGuard struct {
	quiescence *storelock.Handle
	markerPath string
	closeOnce  sync.Once
	closeErr   error
}

// PreparePurge validates a qlog ledger before destructive removal. The caller
// must call ReleaseForPurge before deleting files, then Abort if deletion
// fails. Windows cannot remove a directory while its lock handles are open.
func PreparePurge(ctx context.Context, path string) (_ *PurgeGuard, result error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
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
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("open read-only sqlite: %w", err)
	}
	if err := store.validateSchema(ctx); err != nil {
		return nil, err
	}
	if err := store.VerifyLedger(ctx, ""); err != nil {
		return nil, err
	}
	markerPath := purgeMarkerPath(absolutePath)
	if err := acquirePurgeMarker(markerPath); err != nil {
		return nil, err
	}
	return &PurgeGuard{quiescence: quiescence, markerPath: markerPath}, nil
}

// acquirePurgeMarker either records a new purge or resumes an interrupted one.
// PreparePurge holds exclusive quiescence before calling this helper, so an
// existing marker cannot be resumed while a cooperative writer is active.
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

// ReleaseForPurge releases lock handles while retaining the purge marker. This
// makes the old ledger unreachable before a caller removes it, including on
// Windows where open lock handles prevent directory removal.
func (g *PurgeGuard) ReleaseForPurge() error {
	if g == nil || g.quiescence == nil {
		return nil
	}
	g.closeOnce.Do(func() { g.closeErr = g.quiescence.Close() })
	return g.closeErr
}

// Abort restores access to a ledger when deletion did not complete.
func (g *PurgeGuard) Abort() error {
	if g == nil {
		return nil
	}
	result := g.ReleaseForPurge()
	if g.markerPath != "" {
		if err := os.Remove(g.markerPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			result = errors.Join(result, fmt.Errorf("remove ledger purge marker: %w", err))
		}
	}
	return result
}

// Close releases the guard and removes its marker without deleting data.
func (g *PurgeGuard) Close() error {
	return g.Abort()
}
