//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package lock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func lockFile(file *os.File, exclusive bool) (func() error, error) {
	how := unix.LOCK_SH
	if exclusive {
		how = unix.LOCK_EX
	}
	if err := unix.Flock(int(file.Fd()), how|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrContended
		}
		return nil, err
	}
	return func() error { return unix.Flock(int(file.Fd()), unix.LOCK_UN) }, nil
}
