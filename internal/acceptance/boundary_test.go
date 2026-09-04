package acceptance

import (
	"strings"
	"testing"
	"time"
)

func validBoundary(t *testing.T) RealAgentBoundary {
	t.Helper()
	boundary := RealAgentBoundary{
		SchemaVersion:         RealAgentBoundarySchemaVersion,
		Challenge:             strings.Repeat("1", 64),
		CandidateTag:          "v0.4.0-rc11",
		CandidateCommit:       strings.Repeat("a", 40),
		CandidateBinarySHA256: strings.Repeat("b", 64),
		Platform:              "windows/amd64",
		AgentID:               "codex",
		AgentVersion:          "0.151.0",
		StartedAt:             time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC),
		LedgerPositionSHA256:  strings.Repeat("c", 64),
		LedgerEventCount:      10,
	}
	boundary.ID = BoundaryID(boundary)
	return boundary
}

func TestRealAgentBoundaryRejectsFutureStaleAndBackdatedWindows(t *testing.T) {
	now := time.Date(2026, 9, 3, 10, 10, 0, 0, time.UTC)
	for name, startedAt := range map[string]time.Time{
		"future": now.Add(time.Second),
		"stale":  now.Add(-MaxRealAgentEvidenceWindow - time.Second),
	} {
		t.Run(name, func(t *testing.T) {
			boundary := validBoundary(t)
			boundary.StartedAt = startedAt
			boundary.ID = BoundaryID(boundary)
			if err := ValidateRealAgentBoundary(boundary, now, boundary.CandidateTag, boundary.CandidateCommit, boundary.CandidateBinarySHA256, boundary.Platform); err == nil {
				t.Fatalf("accepted %s boundary", name)
			}
		})
	}
}

func TestRealAgentBoundaryBindsCandidatePlatformAndLedgerPosition(t *testing.T) {
	boundary := validBoundary(t)
	now := boundary.StartedAt.Add(time.Minute)
	tests := map[string]func(*RealAgentBoundary){
		"candidate": func(b *RealAgentBoundary) { b.CandidateCommit = strings.Repeat("d", 40) },
		"binary":    func(b *RealAgentBoundary) { b.CandidateBinarySHA256 = strings.Repeat("d", 64) },
		"platform":  func(b *RealAgentBoundary) { b.Platform = "linux/amd64" },
		"ledger":    func(b *RealAgentBoundary) { b.LedgerPositionSHA256 = "invalid" },
		"integrity": func(b *RealAgentBoundary) { b.AgentVersion = "0.152.0" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := boundary
			mutate(&candidate)
			if err := ValidateRealAgentBoundary(candidate, now, boundary.CandidateTag, boundary.CandidateCommit, boundary.CandidateBinarySHA256, boundary.Platform); err == nil {
				t.Fatalf("accepted boundary with invalid %s binding", name)
			}
		})
	}
}
