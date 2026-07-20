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
	"time"
)

func TestOpenCreatesExclusiveLockFile(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	store, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	for _, lockPath := range []string{databasePath + ".quiescence.lock", databasePath + ".writer.lock"} {
		info, err := os.Stat(lockPath)
		if err != nil {
			t.Fatalf("stat lock file %s: %v", lockPath, err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Fatalf("lock file mode = %o, want 600", info.Mode().Perm())
		}
	}
}

func TestOpenReadOnlyDoesNotCreateMissingLock(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	writer, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	for _, lockPath := range []string{databasePath + ".quiescence.lock", databasePath + ".writer.lock"} {
		if err := os.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("remove lock: %v", err)
		}
	}

	reader, err := OpenReadOnly(context.Background(), databasePath)
	if err == nil {
		_ = reader.Close()
		t.Fatal("OpenReadOnly accepted a missing lock")
	}
	if !strings.Contains(err.Error(), "quiescence lock is missing") {
		t.Fatalf("OpenReadOnly missing lock error = %v", err)
	}
	for _, lockPath := range []string{databasePath + ".quiescence.lock", databasePath + ".writer.lock"} {
		if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("reader created lock or stat failed: %v", err)
		}
	}
}

func TestOpenReadOnlyBlockedWhileWriterHoldsLock(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	writer, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	if _, err := writer.AppendRawEvent(context.Background(), RawEventInput{Source: "writer", EventType: "model.call", Payload: []byte(`{}`), OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatalf("append event: %v", err)
	}

	reader, err := OpenReadOnly(context.Background(), databasePath)
	if err == nil {
		_ = reader.Close()
		t.Fatal("OpenReadOnly acquired a lock held by writer")
	}
	if !strings.Contains(err.Error(), "quiescence lock is held") {
		t.Fatalf("OpenReadOnly writer lock error = %v", err)
	}
}

func TestOpenReadOnlyBlocksWhileReaderHoldsQuiescence(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	writer, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close setup writer: %v", err)
	}
	firstReader, err := OpenReadOnly(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("open first reader: %v", err)
	}
	if secondReader, err := OpenReadOnly(context.Background(), databasePath); err == nil {
		_ = secondReader.Close()
		t.Fatal("OpenReadOnly acquired exclusive quiescence while a reader held it")
	} else if !strings.Contains(err.Error(), "quiescence lock is held") {
		t.Fatalf("OpenReadOnly reader lock error = %v", err)
	}
	if err := firstReader.Close(); err != nil {
		t.Fatalf("close first reader: %v", err)
	}
	secondReader, err := OpenReadOnly(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("OpenReadOnly after reader close: %v", err)
	}
	if err := secondReader.Close(); err != nil {
		t.Fatalf("close second reader: %v", err)
	}
}

func TestCloseReturnsBusyCheckpointErrorAndReleasesLock(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	writer, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := writer.AppendRawEvent(context.Background(), RawEventInput{Source: "writer", EventType: "model.call", Payload: []byte(`{}`), OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatalf("append first event: %v", err)
	}
	reader, err := sql.Open("sqlite", "file:"+filepath.ToSlash(databasePath)+"?mode=ro")
	if err != nil {
		t.Fatalf("open raw reader: %v", err)
	}
	defer func() { _ = reader.Close() }()
	rows, err := reader.Query(`SELECT event_hash FROM raw_events`)
	if err != nil {
		t.Fatalf("query raw reader: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Fatal("raw reader did not receive first event")
	}
	if _, err := writer.AppendRawEvent(context.Background(), RawEventInput{Source: "writer", EventType: "model.call", Payload: []byte(`{}`), OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatalf("append second event: %v", err)
	}

	if err := writer.Close(); err == nil || !strings.Contains(err.Error(), "WAL checkpoint busy") {
		t.Fatalf("Close() busy checkpoint error = %v", err)
	}
	if nextWriter, err := Open(context.Background(), databasePath); err != nil {
		t.Fatalf("Open after busy close: %v", err)
	} else {
		if err := rows.Close(); err != nil {
			t.Fatalf("close raw rows: %v", err)
		}
		if err := reader.Close(); err != nil {
			t.Fatalf("close raw reader: %v", err)
		}
		if err := nextWriter.Close(); err != nil {
			t.Fatalf("close writer after busy close: %v", err)
		}
	}
}

func TestCloseCheckpointsWALBeforeReadOnlyVerification(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	writer, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := writer.AppendRawEvent(context.Background(), RawEventInput{Source: "writer", EventType: "model.call", Payload: []byte(`{}`), OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatalf("append event: %v", err)
	}
	if info, err := os.Stat(databasePath + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("active WAL missing or empty: info=%#v err=%v", info, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if info, err := os.Stat(databasePath + "-wal"); err == nil && info.Size() != 0 {
		t.Fatalf("WAL remains non-empty after close: %d bytes", info.Size())
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat checkpointed WAL: %v", err)
	}

	reader, err := OpenReadOnly(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("OpenReadOnly() error = %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	if err := reader.VerifyLedger(context.Background(), ""); err != nil {
		t.Fatalf("VerifyLedger() error = %v", err)
	}
}

func TestOpenReadOnlyRejectsStaleWAL(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	writer, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := os.WriteFile(databasePath+"-wal", []byte("stale WAL"), 0o600); err != nil {
		t.Fatalf("write stale WAL: %v", err)
	}

	reader, err := OpenReadOnly(context.Background(), databasePath)
	if err == nil {
		_ = reader.Close()
		t.Fatal("OpenReadOnly accepted a stale WAL")
	}
	if !strings.Contains(err.Error(), "active WAL") {
		t.Fatalf("OpenReadOnly stale WAL error = %v", err)
	}
}

func TestCheckpointClearsWAL(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "qlog.db")
	writer, err := Open(context.Background(), databasePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close setup writer: %v", err)
	}
	rawWriter, err := sql.Open("sqlite", "file:"+filepath.ToSlash(databasePath))
	if err != nil {
		t.Fatalf("open raw writer: %v", err)
	}
	defer func() { _ = rawWriter.Close() }()
	if _, err := rawWriter.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatalf("write WAL frame: %v", err)
	}
	if info, err := os.Stat(databasePath + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("WAL missing or empty before checkpoint: info=%#v err=%v", info, err)
	}

	if err := Checkpoint(context.Background(), databasePath); err != nil {
		t.Fatalf("Checkpoint() error = %v", err)
	}
	if info, err := os.Stat(databasePath + "-wal"); err == nil && info.Size() != 0 {
		t.Fatalf("WAL remains non-empty after checkpoint: %d bytes", info.Size())
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat checkpointed WAL: %v", err)
	}
}
