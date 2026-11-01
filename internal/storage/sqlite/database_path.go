package sqlite

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func databaseURI(path, query string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path, RawQuery: query}).String()
}

// CheckDatabaseLocation preserves files misplaced by the former unescaped URI.
func CheckDatabaseLocation(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	if _, err := os.Lstat(absolute); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	legacy := filepath.ToSlash(absolute)
	if index := strings.IndexAny(legacy, "?#"); index >= 0 {
		legacy = legacy[:index]
	}
	if decoded, err := url.PathUnescape(legacy); err == nil {
		legacy = decoded
	}
	legacy = filepath.FromSlash(legacy)
	if legacy == absolute {
		return nil
	}
	info, err := os.Lstat(legacy)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect legacy database location: %w", err)
	}
	if info.IsDir() {
		return nil
	}
	return fmt.Errorf("legacy SQLite URI may have stored this ledger at %q; all files were retained; explicitly recover that ledger to %q before continuing", legacy, absolute)
}
