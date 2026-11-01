package codexsession

import (
	"context"
	"strings"
	"testing"
)

func TestSubsecondSuffixCheckpointOrdering(t *testing.T) {
	service, _ := fixture(t)
	half := strings.Replace(sample(1, `{"total_tokens":150}`), "09:00:01Z", "09:00:01.500000001Z", 1)
	importLog(t, service, header+sample(1, `{"total_tokens":100}`)+half)
	later := importLog(t, service, header+sample(2, `{"total_tokens":200}`))
	if later.Samples != 1 || usage(t, service).TotalTokens != 200 {
		t.Fatalf("subsecond suffix order: %+v total=%d want200", later, usage(t, service).TotalTokens)
	}
}

func TestKnownChildConflictingBootstrapCannotSeedLaterDelta(t *testing.T) {
	service, _ := fixture(t)
	child := childHeader(`"parent_thread_id":"session-1"`, "2026-10-13T09:00:05Z")
	importLog(t, service, child+sample(4, `{"total_tokens":100}`)+childSample(6, 130, 30))
	result, err := Import(context.Background(), service, strings.NewReader(child+childSample(6, 999, 30)+childSample(7, 1029, 30)))
	if err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("conflicting bootstrap accepted: result=%+v err=%v total=%d", result, err, usage(t, service).TotalTokens)
	}
	if usage(t, service).TotalTokens != 30 {
		t.Fatalf("conflict changed prior usage: %d", usage(t, service).TotalTokens)
	}
}
