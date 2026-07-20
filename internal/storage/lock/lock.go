// Package lock coordinates cooperative access to local storage files.
package lock

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

var (
	ErrContended = errors.New("database lock is held")
	ErrMissing   = errors.New("database lock is missing")
)

type Handle struct {
	file   *os.File
	unlock func() error
	once   sync.Once
	err    error
}

func AcquireExclusive(path string) (*Handle, error) {
	return acquire(path, true, true)
}

func AcquireExclusiveExisting(path string) (*Handle, error) {
	return acquire(path, true, false)
}

func AcquireShared(path string) (*Handle, error) {
	return acquire(path, false, false)
}

func AcquireSharedCreate(path string) (*Handle, error) {
	return acquire(path, false, true)
}

func (h *Handle) Close() error {
	h.once.Do(func() {
		h.err = errors.Join(h.unlock(), h.file.Close())
	})
	return h.err
}

func acquire(path string, exclusive, create bool) (*Handle, error) {
	var (
		file *os.File
		err  error
	)
	if create {
		file, err = os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	} else {
		file, err = os.Open(path)
	}
	if errors.Is(err, os.ErrNotExist) && !create {
		return nil, fmt.Errorf("%w: %s", ErrMissing, path)
	}
	if err != nil {
		return nil, fmt.Errorf("open database lock: %w", err)
	}
	unlock, err := lockFile(file, exclusive)
	if err != nil {
		_ = file.Close()
		if errors.Is(err, ErrContended) {
			return nil, fmt.Errorf("%w: %s", ErrContended, path)
		}
		return nil, fmt.Errorf("acquire database lock: %w", err)
	}
	return &Handle{file: file, unlock: unlock}, nil
}
