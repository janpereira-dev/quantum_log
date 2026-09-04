package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"
)

func TestVerifyRawEventSentinelMatchesExactRecord(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	payload := []byte(`{"sentinel":"ok"}`)
	if _, err := s.AppendRawEvent(ctx, RawEventInput{Source: "acceptance", SourceVersion: "v1", EventType: "probe", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	fingerprint := hex.EncodeToString(sum[:])
	verified, err := s.VerifyRawEventSentinel(ctx, "acceptance", "v1", "probe", fingerprint)
	if err != nil || !verified {
		t.Fatalf("sentinel = %v, %v", verified, err)
	}
	verified, err = s.VerifyRawEventSentinel(ctx, "acceptance", "v2", "probe", fingerprint)
	if err != nil || verified {
		t.Fatalf("mismatched sentinel = %v, %v", verified, err)
	}
}
