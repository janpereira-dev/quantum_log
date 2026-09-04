package sqlite

import (
	"context"
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
