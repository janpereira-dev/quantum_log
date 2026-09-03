// Package acceptance defines portable, privacy-safe acceptance evidence.
package acceptance

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	RealAgentSchemaVersion     = "qlog.acceptance.real-agent/v1"
	StatusPass                 = "PASS"
	StatusFail                 = "FAIL"
	StatusPendingExternalE2E   = "PENDING_EXTERNAL_E2E"
	MaxRealAgentEvidenceWindow = 30 * time.Minute
)

var fullCommitPattern = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
var candidateTagPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)
var platformPattern = regexp.MustCompile(`^[a-z0-9]+/[a-z0-9]+$`)
var agentVersionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._+-]{0,63}$`)

var supportedAgentIDs = map[string]bool{
	"claude-code": true,
	"codex":       true,
	"opencode":    true,
}

// RealAgentEvidence is the sanitized result of one bounded, operator-observed
// real-agent exercise. It intentionally has no prompt, response, path, command,
// environment, or raw-log fields.
type RealAgentEvidence struct {
	SchemaVersion         string    `json:"schema_version"`
	CandidateTag          string    `json:"candidate_tag"`
	CandidateCommit       string    `json:"candidate_commit"`
	Platform              string    `json:"platform"`
	AgentID               string    `json:"agent_id"`
	AgentVersion          string    `json:"agent_version"`
	BoundaryID            string    `json:"boundary_id"`
	CandidateBinarySHA256 string    `json:"candidate_binary_sha256"`
	StartedAt             time.Time `json:"started_at"`
	EndedAt               time.Time `json:"ended_at"`
	SourceEvidence        bool      `json:"source_evidence"`
	LedgerStatus          string    `json:"ledger_status"`
	PrivacyStatus         string    `json:"privacy_status"`
	ReplayStatus          string    `json:"replay_status"`
	CaptureQuality        string    `json:"capture_quality"`
	ObservedMetrics       []string  `json:"observed_metrics"`
	Status                string    `json:"status"`
}

// EvaluateRealAgentEvidence validates evidence and derives Status. The input
// Status is never trusted. Unsupported adapters remain pending even when every
// supplied check says PASS.
func EvaluateRealAgentEvidence(e RealAgentEvidence) (RealAgentEvidence, error) {
	e.Status = StatusPendingExternalE2E
	if err := validateRealAgentEvidence(e); err != nil {
		return e, err
	}
	unsupportedCopilot := e.AgentID == "copilot" || e.AgentID == "copilot-vscode"
	if !supportedAgentIDs[e.AgentID] && !unsupportedCopilot {
		return e, fmt.Errorf("agent_id %q is outside the real-agent acceptance contract", e.AgentID)
	}
	expectedQuality := map[string]string{"codex": "otel_reported", "claude-code": "otel_reported", "opencode": "agent_reported"}[e.AgentID]
	if e.SourceEvidence && !unsupportedCopilot && (e.CaptureQuality != expectedQuality || !canonicalObservedMetrics(e.ObservedMetrics)) {
		return e, errors.New("source evidence requires observed capture quality and metrics")
	}
	if e.LedgerStatus == StatusFail || e.PrivacyStatus == StatusFail || e.ReplayStatus == StatusFail {
		e.Status = StatusFail
		return e, nil
	}
	if unsupportedCopilot {
		return e, nil
	}
	if e.SourceEvidence && e.LedgerStatus == StatusPass && e.PrivacyStatus == StatusPass && e.ReplayStatus == StatusPass {
		e.Status = StatusPass
	}
	return e, nil
}

func canonicalObservedMetrics(metrics []string) bool {
	allowed := map[string]bool{"input_tokens": true, "output_tokens": true, "reasoning_tokens": true, "cached_input_tokens": true, "cache_write_tokens": true, "total_tokens": true, "estimated_cost_usd_micros": true, "duration_ms": true}
	previous := ""
	for _, metric := range metrics {
		if !allowed[metric] || metric <= previous {
			return false
		}
		previous = metric
	}
	return len(metrics) > 0
}

// EvaluateRealAgentEvidenceForCandidate additionally binds the evidence to the
// exact tag and full commit embedded in the qlog binary packaging it.
func EvaluateRealAgentEvidenceForCandidate(e RealAgentEvidence, candidateTag, candidateCommit string) (RealAgentEvidence, error) {
	evaluated, err := EvaluateRealAgentEvidence(e)
	if err != nil {
		return evaluated, err
	}
	if e.CandidateTag != candidateTag || !strings.EqualFold(e.CandidateCommit, candidateCommit) {
		evaluated.Status = StatusPendingExternalE2E
		return evaluated, errors.New("real-agent evidence does not match the exact qlog candidate")
	}
	return evaluated, nil
}

func validateRealAgentEvidence(e RealAgentEvidence) error {
	if e.SchemaVersion != RealAgentSchemaVersion {
		return fmt.Errorf("unsupported real-agent evidence schema %q", e.SchemaVersion)
	}
	for name, value := range map[string]string{
		"candidate_tag": e.CandidateTag,
		"platform":      e.Platform,
		"agent_id":      e.AgentID,
		"agent_version": e.AgentVersion,
	} {
		if !safeMetadata(value) {
			return fmt.Errorf("%s must be non-empty sanitized metadata", name)
		}
	}
	if !fullCommitPattern.MatchString(e.CandidateCommit) {
		return errors.New("candidate_commit must be a full 40-character Git commit")
	}
	if !candidateTagPattern.MatchString(e.CandidateTag) || !platformPattern.MatchString(e.Platform) || !agentVersionPattern.MatchString(e.AgentVersion) || !fullSHA256(e.BoundaryID) || !fullSHA256(e.CandidateBinarySHA256) {
		return errors.New("real-agent identity metadata is not canonical or privacy-safe")
	}
	if secretLikeMetadata(e.AgentVersion) {
		return errors.New("agent_version resembles secret material")
	}
	if e.StartedAt.IsZero() || e.EndedAt.IsZero() || !e.EndedAt.After(e.StartedAt) {
		return errors.New("real-agent evidence window must have ordered UTC timestamps")
	}
	_, startOffset := e.StartedAt.Zone()
	_, endOffset := e.EndedAt.Zone()
	if startOffset != 0 || endOffset != 0 {
		return errors.New("real-agent evidence timestamps must be UTC")
	}
	if e.EndedAt.Sub(e.StartedAt) > MaxRealAgentEvidenceWindow {
		return fmt.Errorf("real-agent evidence window exceeds %s", MaxRealAgentEvidenceWindow)
	}
	for name, value := range map[string]string{
		"ledger_status":  e.LedgerStatus,
		"privacy_status": e.PrivacyStatus,
		"replay_status":  e.ReplayStatus,
	} {
		if value != StatusPass && value != StatusFail && value != StatusPendingExternalE2E {
			return fmt.Errorf("%s has unsupported status %q", name, value)
		}
	}
	return nil
}

func secretLikeMetadata(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"sk-", "ghp_", "github_pat_", "akia", "secret", "token", "bearer", "password", "private"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func safeMetadata(value string) bool {
	trimmed := strings.TrimSpace(value)
	if value != trimmed || trimmed == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}
