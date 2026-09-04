package acceptance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const RealAgentBoundarySchemaVersion = "qlog.acceptance.real-agent-boundary/v1"

// RealAgentBoundary is a qlog-created, persisted pre-action checkpoint.
type RealAgentBoundary struct {
	SchemaVersion         string    `json:"schema_version"`
	ID                    string    `json:"id"`
	Challenge             string    `json:"challenge"`
	CandidateTag          string    `json:"candidate_tag"`
	CandidateCommit       string    `json:"candidate_commit"`
	CandidateBinarySHA256 string    `json:"candidate_binary_sha256"`
	Platform              string    `json:"platform"`
	AgentID               string    `json:"agent_id"`
	AgentVersion          string    `json:"agent_version"`
	StartedAt             time.Time `json:"started_at"`
	LedgerPositionSHA256  string    `json:"ledger_position_sha256"`
	LedgerEventCount      int64     `json:"ledger_event_count"`
	LedgerEventSequence   int64     `json:"ledger_event_sequence"`
	AgentSourceModelCalls int64     `json:"agent_source_model_calls"`
}

// BoundaryID binds every boundary field, including the random challenge.
func BoundaryID(boundary RealAgentBoundary) string {
	boundary.ID = ""
	encoded, _ := json.Marshal(boundary)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// ValidateRealAgentBoundary verifies integrity, age, and exact runtime identity.
func ValidateRealAgentBoundary(boundary RealAgentBoundary, now time.Time, candidateTag, candidateCommit, binarySHA256, platform string) error {
	if boundary.SchemaVersion != RealAgentBoundarySchemaVersion {
		return fmt.Errorf("unsupported real-agent boundary schema %q", boundary.SchemaVersion)
	}
	if !fullSHA256(boundary.ID) || !fullSHA256(boundary.Challenge) || boundary.ID != BoundaryID(boundary) {
		return errors.New("real-agent boundary integrity check failed")
	}
	if boundary.CandidateTag != candidateTag || !strings.EqualFold(boundary.CandidateCommit, candidateCommit) || !strings.EqualFold(boundary.CandidateBinarySHA256, binarySHA256) || boundary.Platform != platform {
		return errors.New("real-agent boundary does not match the exact qlog runtime")
	}
	if !fullSHA256(boundary.LedgerPositionSHA256) || boundary.LedgerEventCount < 0 || boundary.LedgerEventSequence < 0 || boundary.AgentSourceModelCalls < 0 {
		return errors.New("real-agent boundary has an invalid ledger position")
	}
	if boundary.StartedAt.IsZero() || boundary.StartedAt.After(now) || now.Sub(boundary.StartedAt) > MaxRealAgentEvidenceWindow {
		return errors.New("real-agent boundary is future-dated or outside the evidence window")
	}
	_, offset := boundary.StartedAt.Zone()
	if offset != 0 {
		return errors.New("real-agent boundary timestamp must be UTC")
	}
	probe := RealAgentEvidence{SchemaVersion: RealAgentSchemaVersion, CandidateTag: boundary.CandidateTag, CandidateCommit: boundary.CandidateCommit, CandidateBinarySHA256: boundary.CandidateBinarySHA256, Platform: boundary.Platform, AgentID: boundary.AgentID, AgentVersion: boundary.AgentVersion, BoundaryID: boundary.ID, StartedAt: boundary.StartedAt, EndedAt: boundary.StartedAt.Add(time.Nanosecond), LedgerStatus: StatusPendingExternalE2E, PrivacyStatus: StatusPendingExternalE2E, ReplayStatus: StatusPendingExternalE2E}
	if err := validateRealAgentEvidence(probe); err != nil {
		return fmt.Errorf("invalid real-agent boundary metadata: %w", err)
	}
	return nil
}

func fullSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
