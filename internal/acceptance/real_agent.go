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

var supportedAgentIDs = map[string]bool{
	"claude-code": true,
	"codex":       true,
	"opencode":    true,
}

// RealAgentEvidence is the sanitized result of one bounded, operator-observed
// real-agent exercise. It intentionally has no prompt, response, path, command,
// environment, or raw-log fields.
type RealAgentEvidence struct {
	SchemaVersion   string    `json:"schema_version"`
	CandidateTag    string    `json:"candidate_tag"`
	CandidateCommit string    `json:"candidate_commit"`
	Platform        string    `json:"platform"`
	AgentID         string    `json:"agent_id"`
	AgentVersion    string    `json:"agent_version"`
	StartedAt       time.Time `json:"started_at"`
	EndedAt         time.Time `json:"ended_at"`
	SourceEvidence  bool      `json:"source_evidence"`
	LedgerStatus    string    `json:"ledger_status"`
	PrivacyStatus   string    `json:"privacy_status"`
	ReplayStatus    string    `json:"replay_status"`
	Status          string    `json:"status"`
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
