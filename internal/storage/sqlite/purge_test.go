package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreparePurgeValidatesInitializedIdleLedger(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	writer, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	guard, err := PreparePurge(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("PreparePurge() error = %v", err)
	}
	if err := guard.Close(); err != nil {
		t.Fatalf("close purge guard: %v", err)
	}
}

func TestPreparePurgeRejectsCorruptOrActiveLedger(t *testing.T) {
	corrupt := filepath.Join(t.TempDir(), "qlog.db")
	initialized, err := Open(context.Background(), corrupt)
	if err != nil {
		t.Fatal(err)
	}
	if err := initialized.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corrupt, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PreparePurge(context.Background(), corrupt); err == nil {
		t.Fatal("PreparePurge accepted a corrupt database")
	} else if strings.Contains(err.Error(), "quiescence lock is missing") {
		t.Fatalf("corrupt ledger error = %v", err)
	}

	active := filepath.Join(t.TempDir(), "qlog.db")
	writer, err := Open(context.Background(), active)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	if _, err := PreparePurge(context.Background(), active); err == nil {
		t.Fatal("PreparePurge accepted an active writer")
	} else if !strings.Contains(err.Error(), "quiescence lock is held") {
		t.Fatalf("active ledger error = %v", err)
	}
}
