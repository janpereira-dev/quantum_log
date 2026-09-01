package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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
	} else if !strings.Contains(err.Error(), "purge is in progress") {
		t.Fatalf("active ledger error = %v", err)
	}
}

func TestPreparePurgeAcceptsRecognisedHistoricalMigrationHistory(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	store, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(context.Background(), `DELETE FROM schema_migrations WHERE version = '013_reconciled_model_usage.sql'`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	guard, err := PreparePurge(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("PreparePurge() rejected recognised historical ledger: %v", err)
	}
	if err := guard.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPreparePurgeRejectsForeignMigrationTableLookalike(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(databasePath))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY); INSERT INTO schema_migrations(version) VALUES ('foreign_ledger.sql')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, lockPath := range []string{quiescenceLockPath(databasePath), writerLockPath(databasePath)} {
		if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := PreparePurge(context.Background(), databasePath); err == nil {
		t.Fatal("PreparePurge accepted a foreign schema_migrations lookalike")
	} else if !strings.Contains(err.Error(), "unrecognised migration") {
		t.Fatalf("foreign lookalike error = %v", err)
	}
}

func TestPurgeGuardKeepsLedgerUnreachableUntilAbort(t *testing.T) {
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
	if err := guard.ReleaseForPurge(); err != nil {
		t.Fatalf("release purge guard: %v", err)
	}
	if nextWriter, err := Open(context.Background(), databasePath); err == nil {
		_ = nextWriter.Close()
		t.Fatal("Open accepted a ledger marked for purge")
	} else if !strings.Contains(err.Error(), "purge is in progress") {
		t.Fatalf("Open while purging error = %v", err)
	}
	if err := guard.Abort(); err != nil {
		t.Fatalf("abort purge: %v", err)
	}
	nextWriter, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("Open after abort = %v", err)
	}
	if err := nextWriter.Close(); err != nil {
		t.Fatalf("close restored ledger: %v", err)
	}
}

func TestPurgeMarkerBlocksEveryCooperativeDatabaseEntryPoint(t *testing.T) {
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
		t.Fatal(err)
	}
	if err := guard.ReleaseForPurge(); err != nil {
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
		if err := open(); err == nil || !strings.Contains(err.Error(), "purge is in progress") {
			t.Errorf("%s during purge error = %v", name, err)
		}
	}
	if err := guard.Abort(); err != nil {
		t.Fatal(err)
	}
}

func TestPreparePurgeResumesAnInterruptedPurge(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	writer, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	abandoned, err := PreparePurge(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("prepare initial purge: %v", err)
	}
	if err := abandoned.ReleaseForPurge(); err != nil {
		t.Fatalf("release initial purge: %v", err)
	}
	// Simulate process death: intent remains durable, but the held OS lock is
	// released only when its owner exits.
	if err := abandoned.lifecycle.Close(); err != nil {
		t.Fatalf("release abandoned lifecycle lock: %v", err)
	}

	resumed, err := PreparePurge(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("resume interrupted purge: %v", err)
	}
	if err := resumed.ReleaseForPurge(); err != nil {
		t.Fatalf("release resumed purge: %v", err)
	}
	if nextWriter, err := Open(context.Background(), databasePath); err == nil {
		_ = nextWriter.Close()
		t.Fatal("Open accepted a ledger while its resumed purge was pending")
	}
	if err := resumed.Abort(); err != nil {
		t.Fatalf("abort resumed purge: %v", err)
	}
}

func TestPreparePurgeRefusesResumeWithoutExclusiveOwnership(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	writer, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	if err := os.WriteFile(purgeMarkerPath(databasePath), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(purgeMarkerPath(databasePath)) })
	if _, err := PreparePurge(context.Background(), databasePath); err == nil {
		t.Fatal("PreparePurge resumed without exclusive quiescence ownership")
	} else if !strings.Contains(err.Error(), "purge is in progress") {
		t.Fatalf("PreparePurge resume error = %v", err)
	}
}

func TestPreparePurgeRejectsForeignHomeRecreatedAfterCrash(t *testing.T) {
	home := filepath.Join(t.TempDir(), "ledger")
	databasePath := filepath.Join(home, "qlog.db")
	store, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	abandoned, err := PreparePurge(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := abandoned.ReleaseForPurge(); err != nil {
		t.Fatal(err)
	}
	if err := abandoned.lifecycle.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(home); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	foreign, err := sql.Open("sqlite", "file:"+filepath.ToSlash(databasePath))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := foreign.Exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY); INSERT INTO schema_migrations(version) VALUES ('001_initial.sql')`); err != nil {
		t.Fatal(err)
	}
	if err := foreign.Close(); err != nil {
		t.Fatal(err)
	}
	for _, lockPath := range []string{quiescenceLockPath(databasePath), writerLockPath(databasePath)} {
		if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := PreparePurge(context.Background(), databasePath); err == nil || !strings.Contains(err.Error(), "stable qlog schema") {
		t.Fatalf("recovery trusted foreign recreated home: %v", err)
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatalf("foreign recreated home was removed: %v", err)
	}
}

func TestLifecycleJournalSyncFailureFailsClosed(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	store, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	restore := SetLifecycleControlSyncForTesting(func(string) error { return errors.New("simulated directory sync failure") })
	_, err = PreparePurge(context.Background(), databasePath)
	restore()
	if err == nil || !strings.Contains(err.Error(), "durably persist") {
		t.Fatalf("journal sync failure did not fail closed: %v", err)
	}
	if _, err := os.Lstat(purgeMarkerPath(databasePath)); err != nil {
		t.Fatalf("durability failure removed pending journal: %v", err)
	}
}

func TestLifecycleLockUsesCanonicalSymlinkIdentity(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "ledger")
	databasePath := filepath.Join(home, "qlog.db")
	store, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "ledger-alias")
	if err := os.Symlink(home, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	guard, err := PreparePurge(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), filepath.Join(alias, "qlog.db")); err == nil || !strings.Contains(err.Error(), "purge is in progress") {
		t.Fatalf("symlink alias bypassed lifecycle lock: %v", err)
	}
	if err := guard.Abort(); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsLifecycleJournalDurabilityUsesProductionSync(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("exercises the Windows CreateFile/FlushFileBuffers implementation")
	}
	home := filepath.Join(t.TempDir(), "ledger")
	databasePath := filepath.Join(home, "qlog.db")
	store, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	guard, err := PreparePurge(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("production directory sync did not permit journal creation: %v", err)
	}
	if _, err := os.Lstat(purgeMarkerPath(databasePath)); err != nil {
		t.Fatalf("journal missing after durable creation: %v", err)
	}
	deletionPath, err := guard.DetachForPurge()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(deletionPath); err != nil {
		t.Fatal(err)
	}
	if err := guard.Complete(); err != nil {
		t.Fatalf("production directory sync did not permit journal removal: %v", err)
	}
	if _, err := os.Lstat(purgeMarkerPath(databasePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal remained after durable removal: %v", err)
	}
}
