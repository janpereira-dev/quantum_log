package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenReadOnlyDoesNotCreateMissingDatabase(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "missing", "qlog.db")
	store, err := OpenReadOnly(context.Background(), databasePath)
	if err == nil {
		_ = store.Close()
		t.Fatal("OpenReadOnly accepted a missing database")
	}
	if _, err := os.Stat(filepath.Dir(databasePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("database parent exists or stat failed: %v", err)
	}
}
