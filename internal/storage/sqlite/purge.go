package sqlite

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	storelock "github.com/janpereira-dev/quantum_log/internal/storage/lock"
)

// CheckPurgePending rejects access to a ledger left protected by the RC.9
// in-home purge sentinel. The sentinel is compatibility protection only:
// RC.10 deliberately performs no automatic destructive purge.
//
// When the home already exists, the check is made while holding the existing
// cooperative quiescence lock. A nonexistent home cannot contain a sentinel
// and is left for normal initialization to create.
func CheckPurgePending(path string) error {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	if _, err := os.Stat(filepath.Dir(absolutePath)); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect local data directory: %w", err)
	}
	quiescence, err := storelock.AcquireSharedCreate(quiescenceLockPath(absolutePath))
	if err != nil {
		return writerQuiescenceError(err)
	}
	defer func() { _ = quiescence.Close() }()
	return rejectPurgeMarker(absolutePath)
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
