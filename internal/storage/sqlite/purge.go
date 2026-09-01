package sqlite

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	storelock "github.com/janpereira-dev/quantum_log/internal/storage/lock"
)

// InitializationGuard retains the shared legacy quiescence lock from the
// pre-configuration sentinel check until a newly opened Store owns its own
// shared lock. It prevents an RC.9 purge from beginning between those steps.
type InitializationGuard struct{ quiescence *storelock.Handle }

// AcquireInitializationGuard rejects a ledger left protected by the RC.9
// in-home purge sentinel. The sentinel is compatibility protection only:
// RC.10 deliberately performs no automatic destructive purge.
//
// When the home already exists, the check is made while holding the existing
// cooperative quiescence lock. A nonexistent home cannot contain a sentinel:
// RC.9 requires an existing database before it can acquire the exclusive
// legacy quiescence lock, config.Ensure does not create that database, and
// Open acquires the shared legacy lock before creating it. Therefore no
// RC.9-compatible purge can begin during absent-home initialization.
func AcquireInitializationGuard(path string) (*InitializationGuard, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	if _, err := os.Stat(filepath.Dir(absolutePath)); errors.Is(err, os.ErrNotExist) {
		return &InitializationGuard{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect local data directory: %w", err)
	}
	quiescence, err := storelock.AcquireSharedCreate(quiescenceLockPath(absolutePath))
	if err != nil {
		return nil, writerQuiescenceError(err)
	}
	if err := rejectPurgeMarker(absolutePath); err != nil {
		_ = quiescence.Close()
		return nil, err
	}
	return &InitializationGuard{quiescence: quiescence}, nil
}

func (g *InitializationGuard) Close() error {
	if g == nil || g.quiescence == nil {
		return nil
	}
	return g.quiescence.Close()
}

// CheckPurgePending is retained for callers that only need the compatibility
// preflight. Initialization must use AcquireInitializationGuard instead.
func CheckPurgePending(path string) error {
	guard, err := AcquireInitializationGuard(path)
	if err != nil {
		return err
	}
	return guard.Close()
}

func rejectPurgeMarker(databasePath string) error {
	markerPath := purgeMarkerPath(databasePath)
	info, err := os.Lstat(markerPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect ledger purge marker: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refuse unsafe non-regular ledger purge marker %q", markerPath)
	}
	return errors.New("ledger purge is pending; local data is protected and automatic purge is unavailable in this release")
}
