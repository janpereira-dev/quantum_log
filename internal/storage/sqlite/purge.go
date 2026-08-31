package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	storelock "github.com/janpereira-dev/quantum_log/internal/storage/lock"
)

// PurgeGuard proves that a ledger is initialized and keeps its cooperative
// quiescence lock while the caller completes destructive preflight.
// It never creates or modifies database files.
type PurgeGuard struct {
	quiescence *storelock.Handle
}

// PreparePurge validates a qlog ledger before destructive removal. The caller
// must Close the returned guard before deleting files; Windows cannot remove a
// directory while its lock handles are open.
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
	return &PurgeGuard{quiescence: quiescence}, nil
}

// Close releases the guard acquired by PreparePurge.
func (g *PurgeGuard) Close() error {
	if g == nil || g.quiescence == nil {
		return nil
	}
	return g.quiescence.Close()
}
