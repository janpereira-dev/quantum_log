package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestLedgerAnchorsAndTruncationDetection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store, err := Open(context.Background(), filepath.Join(dir, "qlog.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		if _, err := store.AppendRawEvent(context.Background(), RawEventInput{
			Source:     "s1",
			SessionID:  "sess",
			EventType:  "tool",
			Payload:    []byte("{}"),
			OccurredAt: now,
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	anchors, err := store.LedgerAnchors(context.Background())
	if err != nil {
		t.Fatalf("anchors: %v", err)
	}
	if len(anchors) != 1 || anchors[0].Source != "s1" || anchors[0].HeadHash == "" {
		t.Fatalf("unexpected anchors: %+v", anchors)
	}
	if anchors[0].Events != 3 {
		t.Errorf("event count = %d, want 3", anchors[0].Events)
	}
	expected := anchors
	mismatches, err := store.VerifyAnchors(context.Background(), expected)
	if err != nil {
		t.Fatalf("verify ok: %v", err)
	}
	if len(mismatches) != 0 {
		t.Errorf("expected no mismatches, got %+v", mismatches)
	}
	tampered := []LedgerAnchor{{Source: "s1", SessionID: "sess", HeadHash: "deadbeef", Events: 3}}
	mismatches, err = store.VerifyAnchors(context.Background(), tampered)
	if err != nil {
		t.Fatalf("verify tampered: %v", err)
	}
	if len(mismatches) != 1 || mismatches[0].Actual == "deadbeef" {
		t.Fatalf("expected one mismatch, got %+v", mismatches)
	}
	truncated := []LedgerAnchor{{Source: "s1", SessionID: "missing", HeadHash: "x", Events: 1}}
	mismatches, err = store.VerifyAnchors(context.Background(), truncated)
	if err != nil {
		t.Fatalf("verify missing: %v", err)
	}
	if len(mismatches) != 1 || !mismatches[0].Truncated {
		t.Fatalf("expected truncated, got %+v", mismatches)
	}
}
