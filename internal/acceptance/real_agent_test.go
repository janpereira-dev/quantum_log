package acceptance

import (
	"strings"
	"testing"
	"time"
)

func validRealAgentEvidence() RealAgentEvidence {
	start := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	return RealAgentEvidence{
		SchemaVersion:         RealAgentSchemaVersion,
		CandidateTag:          "v0.4.0-rc11",
		CandidateCommit:       strings.Repeat("a", 40),
		CandidateBinarySHA256: strings.Repeat("b", 64),
		Platform:              "windows/amd64",
		AgentID:               "codex",
		AgentVersion:          "0.151.0",
		BoundaryID:            strings.Repeat("d", 64),
		StartedAt:             start,
		EndedAt:               start.Add(5 * time.Minute),
		SourceEvidence:        true,
		LedgerStatus:          StatusPass,
		PrivacyStatus:         StatusPass,
		ReplayStatus:          StatusPass,
		CaptureQuality:        "otel_reported",
		ObservedMetrics:       []string{"input_tokens", "output_tokens"},
		Status:                StatusFail,
	}
}

func TestRealAgentEvidenceRejectsPathAndSecretLikeMetadata(t *testing.T) {
	for name, mutate := range map[string]func(*RealAgentEvidence){
		"path":   func(e *RealAgentEvidence) { e.AgentVersion = `C:\\Users\\name\\agent.exe` },
		"secret": func(e *RealAgentEvidence) { e.AgentVersion = "sk-secretvalue" },
	} {
		t.Run(name, func(t *testing.T) {
			evidence := validRealAgentEvidence()
			mutate(&evidence)
			if got, err := EvaluateRealAgentEvidence(evidence); err == nil || got.Status == StatusPass {
				t.Fatalf("unsafe metadata = %#v, %v", got, err)
			}
		})
	}
}

func TestRealAgentEvidenceDerivesPassOnlyFromCompleteEvidence(t *testing.T) {
	got, err := EvaluateRealAgentEvidence(validRealAgentEvidence())
	if err != nil {
		t.Fatalf("evaluate evidence: %v", err)
	}
	if got.Status != StatusPass {
		t.Fatalf("status = %q, want %q", got.Status, StatusPass)
	}
}

func TestRealAgentEvidenceCannotPromoteSyntheticOrSetupOnlyEvidence(t *testing.T) {
	for _, name := range []string{"synthetic", "setup-only"} {
		t.Run(name, func(t *testing.T) {
			evidence := validRealAgentEvidence()
			evidence.SourceEvidence = false
			evidence.Status = StatusPass
			got, err := EvaluateRealAgentEvidence(evidence)
			if err != nil {
				t.Fatalf("evaluate evidence: %v", err)
			}
			if got.Status != StatusPendingExternalE2E {
				t.Fatalf("status = %q, want %q", got.Status, StatusPendingExternalE2E)
			}
		})
	}
}

func TestRealAgentEvidenceRequiresIdentityWindowAndVerification(t *testing.T) {
	tests := map[string]func(*RealAgentEvidence){
		"candidate tag":    func(e *RealAgentEvidence) { e.CandidateTag = "" },
		"candidate commit": func(e *RealAgentEvidence) { e.CandidateCommit = "short" },
		"platform":         func(e *RealAgentEvidence) { e.Platform = "" },
		"agent id":         func(e *RealAgentEvidence) { e.AgentID = "" },
		"agent version":    func(e *RealAgentEvidence) { e.AgentVersion = "" },
		"window order":     func(e *RealAgentEvidence) { e.EndedAt = e.StartedAt },
		"bounded window":   func(e *RealAgentEvidence) { e.EndedAt = e.StartedAt.Add(MaxRealAgentEvidenceWindow + time.Second) },
		"ledger":           func(e *RealAgentEvidence) { e.LedgerStatus = StatusPendingExternalE2E },
		"privacy":          func(e *RealAgentEvidence) { e.PrivacyStatus = StatusPendingExternalE2E },
		"replay":           func(e *RealAgentEvidence) { e.ReplayStatus = StatusPendingExternalE2E },
		"capture quality":  func(e *RealAgentEvidence) { e.CaptureQuality = "lifecycle_only" },
		"observed metrics": func(e *RealAgentEvidence) { e.ObservedMetrics = []string{"prompt_body"} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			evidence := validRealAgentEvidence()
			mutate(&evidence)
			got, _ := EvaluateRealAgentEvidence(evidence)
			if got.Status == StatusPass {
				t.Fatal("incomplete evidence was promoted to PASS")
			}
		})
	}
}

func TestRealAgentEvidenceRejectsUnsupportedCopilotAdapters(t *testing.T) {
	for _, agentID := range []string{"copilot", "copilot-vscode"} {
		t.Run(agentID, func(t *testing.T) {
			evidence := validRealAgentEvidence()
			evidence.AgentID = agentID
			got, err := EvaluateRealAgentEvidence(evidence)
			if err != nil {
				t.Fatalf("evaluate evidence: %v", err)
			}
			if got.Status != StatusPendingExternalE2E {
				t.Fatalf("status = %q, want pending", got.Status)
			}
		})
	}
}

func TestRealAgentEvidenceRejectsUnknownAgentIdentity(t *testing.T) {
	evidence := validRealAgentEvidence()
	evidence.AgentID = "made-up-agent"
	got, err := EvaluateRealAgentEvidence(evidence)
	if err == nil {
		t.Fatal("expected unknown agent identity error")
	}
	if got.Status == StatusPass {
		t.Fatal("unknown agent identity was promoted to PASS")
	}
}

func TestRealAgentEvidenceMustMatchExactCandidate(t *testing.T) {
	evidence := validRealAgentEvidence()
	got, err := EvaluateRealAgentEvidenceForCandidate(evidence, evidence.CandidateTag, strings.Repeat("b", 40))
	if err == nil {
		t.Fatal("expected candidate mismatch")
	}
	if got.Status == StatusPass {
		t.Fatal("candidate mismatch was promoted to PASS")
	}
}

func TestRealAgentEvidenceAcceptsNumericUTCOffset(t *testing.T) {
	evidence := validRealAgentEvidence()
	zeroOffset := time.FixedZone("+00:00", 0)
	evidence.StartedAt = evidence.StartedAt.In(zeroOffset)
	evidence.EndedAt = evidence.EndedAt.In(zeroOffset)
	got, err := EvaluateRealAgentEvidence(evidence)
	if err != nil || got.Status != StatusPass {
		t.Fatalf("numeric UTC evidence = %#v, %v", got, err)
	}
}
