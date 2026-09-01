package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPurgeSentinelBlocksEveryCooperativeDatabaseEntryPoint(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	writer, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(purgeMarkerPath(databasePath), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	for name, open := range map[string]func() error{
		"open": func() error {
			store, err := Open(context.Background(), databasePath)
			if store != nil {
				_ = store.Close()
			}
			return err
		},
		"read-only": func() error {
			store, err := OpenReadOnly(context.Background(), databasePath)
			if store != nil {
				_ = store.Close()
			}
			return err
		},
		"snapshot": func() error {
			store, err := OpenSnapshotReadOnly(context.Background(), databasePath)
			if store != nil {
				_ = store.Close()
			}
			return err
		},
		"checkpoint": func() error { return Checkpoint(context.Background(), databasePath) },
	} {
		if err := open(); err == nil || !strings.Contains(err.Error(), "purge is pending") {
			t.Errorf("%s while sentinel is present error = %v", name, err)
		}
	}
}

func TestCheckPurgePendingAcceptsAbsentHome(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "absent", "qlog.db")
	if err := CheckPurgePending(databasePath); err != nil {
		t.Fatalf("CheckPurgePending(absent home) = %v", err)
	}
}

func TestPurgeSentinelRejectsUnsafeTopology(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	writer, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(purgeMarkerPath(databasePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := CheckPurgePending(databasePath); err == nil || !strings.Contains(err.Error(), "non-regular") {
		t.Fatalf("unsafe purge sentinel error = %v", err)
	}
}
