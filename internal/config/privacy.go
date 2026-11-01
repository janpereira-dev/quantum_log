package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsurePrivateDirectory protects new directories and validates existing ones.
// Existing permissions are never changed implicitly.
func EnsurePrivateDirectory(path string) error {
	if err := ensurePrivateDirectory(path); err != nil {
		return fmt.Errorf("private directory permissions: %w", err)
	}
	return nil
}

// CheckPrivateFile refuses unsafe existing sensitive files without changing them.
func CheckPrivateFile(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect private file permissions: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("private file permissions: refuse non-regular file %q", path)
	}
	if err := checkPrivatePermissions(path, info); err != nil {
		return fmt.Errorf("private file permissions: %w", err)
	}
	return nil
}

// EnsurePrivateHome applies the ledger's privacy boundary before sensitive writes.
func EnsurePrivateHome(home string) error {
	if err := EnsurePrivateDirectory(home); err != nil {
		return err
	}
	for _, name := range []string{"config.yaml", "prompt-hash.key", "collector.token", "qlog.db", "qlog.db-wal", "qlog.db-shm", "qlog.db.quiescence.lock", "qlog.db.writer.lock"} {
		if err := CheckPrivateFile(filepath.Join(home, name)); err != nil {
			return err
		}
	}
	identityDirectory := filepath.Join(home, "collector-tls")
	if info, err := os.Lstat(identityDirectory); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("private collector TLS identity must be an unlinked directory")
		}
		if err := checkPrivatePermissions(identityDirectory, info); err != nil {
			return fmt.Errorf("private collector TLS directory permissions: %w", err)
		}
		for _, name := range []string{"cert.pem", "key.pem"} {
			if err := CheckPrivateFile(filepath.Join(identityDirectory, name)); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}
