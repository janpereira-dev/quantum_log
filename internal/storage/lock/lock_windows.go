//go:build windows

package lock

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func lockFile(file *os.File, exclusive bool) (func() error, error) {
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY)
	if exclusive {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	overlapped := &windows.Overlapped{}
	handle := windows.Handle(file.Fd())
	if err := windows.LockFileEx(handle, flags, 0, 1, 0, overlapped); err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, ErrContended
		}
		return nil, err
	}
	return func() error { return windows.UnlockFileEx(handle, 0, 1, 0, overlapped) }, nil
}
