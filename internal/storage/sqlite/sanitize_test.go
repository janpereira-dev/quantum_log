package sqlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestSanitizeEvidenceStripsSensitiveKeys(t *testing.T) {
	t.Parallel()
	raw := `{"prompt":"hi","api_key":"secret","nested":{"token":"x","keep":"ok"},"cookie":"c"}`
	out, err := sanitizeEvidence(raw)
	if err != nil {
		t.Fatalf("sanitizeEvidence: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if _, exists := got["prompt"]; exists {
		t.Errorf("prompt not stripped")
	}
	if _, exists := got["api_key"]; exists {
		t.Errorf("api_key not stripped")
	}
	if _, exists := got["cookie"]; exists {
		t.Errorf("cookie not stripped")
	}
	nested, ok := got["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested missing: %v", got)
	}
	if _, exists := nested["token"]; exists {
		t.Errorf("nested token not stripped")
	}
	if nested["keep"] != "ok" {
		t.Errorf("nested keep altered: %v", nested["keep"])
	}
}

func TestSanitizeEvidenceMalformedFallsBack(t *testing.T) {
	t.Parallel()
	out, err := sanitizeEvidence("{not json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" || out == "{not json" {
		t.Errorf("expected fallback sanitized output, got %q", out)
	}
}

func TestAppendRawEventStripsSensitiveKeysFromEvidence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	store, err := Open(context.Background(), filepath.Join(dir, "qlog.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	raw := `{"prompt":"hi","keep":"ok","authorization":"bearer xyz"}`
	id, err := store.AppendRawEvent(context.Background(), RawEventInput{
		Source:       "test",
		SessionID:    "s1",
		EventType:    "tool",
		Payload:      []byte("{}"),
		OccurredAt:   time.Now().UTC(),
		EvidenceJSON: raw,
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if id == "" {
		t.Fatal("empty id")
	}
	var stored string
	if err := store.db.QueryRow(`SELECT project_resolution_evidence_json FROM raw_events WHERE id = ?`, id).Scan(&stored); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if contains(stored, "hi") || contains(stored, "bearer") {
		t.Errorf("sensitive data persisted in evidence: %s", stored)
	}
	if !contains(stored, "ok") {
		t.Errorf("non-sensitive data lost: %s", stored)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
