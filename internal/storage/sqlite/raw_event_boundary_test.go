package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRawEventIDsAfterAcceptanceBoundaryUsesSequenceNotOccurredAt(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	}()
	if _, err := s.AppendRawEvent(ctx, RawEventInput{Source: "agent", EventType: "before", Payload: []byte(`{"ok":true}`), OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	marker := AcceptanceBoundaryMarker{BoundaryID: "boundary-1", Challenge: strings.Repeat("a", 64), LedgerPositionSHA256: strings.Repeat("b", 64), LedgerEventCount: 1}
	if _, err := s.AppendAcceptanceBoundaryMarker(ctx, marker, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	future, err := s.AppendRawEvent(ctx, RawEventInput{Source: "agent", EventType: "after", Payload: []byte(`{"ok":true}`), OccurredAt: time.Now().UTC().Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	ids, err := s.RawEventIDsAfterAcceptanceBoundary(ctx, marker)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != future.ID {
		t.Fatalf("post-boundary IDs = %#v, want %s", ids, future.ID)
	}
	marker.Challenge = strings.Repeat("c", 64)
	if _, err := s.RawEventIDsAfterAcceptanceBoundary(ctx, marker); err == nil {
		t.Fatal("accepted tampered boundary marker")
	}
}

func TestRawEventSequenceMigrationUsesWindowBackfill(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("migrations", "018_raw_event_sequence.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sql := string(data)
	if !strings.Contains(sql, "ROW_NUMBER() OVER") || strings.Contains(sql, "COUNT(*)") {
		t.Fatalf("migration does not use the bounded window backfill: %s", sql)
	}
}

func TestVerifyLedgerDetectsEventSequenceTampering(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, eventType := range []string{"one", "two"} {
		if _, err := s.AppendRawEvent(ctx, RawEventInput{Source: "agent", EventType: eventType, Payload: []byte(`{"ok":true}`), OccurredAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.VerifyLedger(ctx, ""); err != nil {
		t.Fatalf("baseline verification: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE raw_events SET event_sequence = 99 WHERE event_type = 'one'`); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifyLedger(ctx, ""); err == nil {
		t.Fatal("VerifyLedger accepted tampered event sequence")
	}
}

func TestVerifyLedgerDetectsEventSequenceSwapAcrossChains(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	first, err := s.AppendRawEvent(ctx, RawEventInput{Source: "agent-a", SessionID: "session-a", EventType: "one", Payload: []byte(`{"n":1}`), OccurredAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.AppendRawEvent(ctx, RawEventInput{Source: "agent-b", SessionID: "session-b", EventType: "one", Payload: []byte(`{"n":1}`), OccurredAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendRawEvent(ctx, RawEventInput{Source: "agent-a", SessionID: "session-a", EventType: "two", Payload: []byte(`{"n":2}`), OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendRawEvent(ctx, RawEventInput{Source: "agent-b", SessionID: "session-b", EventType: "two", Payload: []byte(`{"n":2}`), OccurredAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifyLedger(ctx, ""); err != nil {
		t.Fatalf("baseline verification: %v", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE raw_events SET event_sequence = CASE id WHEN ? THEN -1 WHEN ? THEN -2 END WHERE id IN (?, ?)`, first.ID, second.ID, first.ID, second.ID); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE raw_events SET event_sequence = CASE id WHEN ? THEN ? WHEN ? THEN ? END WHERE id IN (?, ?)`, first.ID, second.Sequence, second.ID, first.Sequence, first.ID, second.ID); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifyLedger(ctx, ""); err == nil {
		t.Fatal("VerifyLedger accepted an event sequence swap across independent chains")
	}
}
