//go:build !windows

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

func ensurePrivateDirectory(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refuse non-directory or linked path %q", path)
	}
	return checkPrivatePermissions(path, info)
}

func checkPrivatePermissions(path string, info os.FileInfo) error {
	if info.Mode().Perm()&0077 != 0 {
		// Older SQLite files may be 0644 inside an owner-only home.
		if info.Mode().IsRegular() {
			parent, err := os.Stat(filepath.Dir(path))
			if err == nil && parent.Mode().Perm()&0077 == 0 {
				return nil
			}
		}
		return fmt.Errorf("group or other access on %q; restrict permissions explicitly before continuing", path)
	}
	return nil
}
