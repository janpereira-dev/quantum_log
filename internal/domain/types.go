// Package domain holds provider-independent business concepts.
package domain

import "time"

type ResolutionMethod string

const (
	ResolutionExplicit    ResolutionMethod = "explicit"
	ResolutionEnvironment ResolutionMethod = "environment"
	ResolutionCWD         ResolutionMethod = "cwd"
	ResolutionGitRoot     ResolutionMethod = "git_root"
	ResolutionPath        ResolutionMethod = "registered_path"
	ResolutionUnresolved  ResolutionMethod = "unresolved"
)

type Confidence string

const (
	ConfidenceExact   Confidence = "exact"
	ConfidenceHigh    Confidence = "high"
	ConfidenceUnknown Confidence = "unknown"
)

type Host struct {
	ID                     string
	Name                   string
	OS                     string
	Arch                   string
	MachineFingerprintHash string
	FirstSeenAt            time.Time
	LastSeenAt             time.Time
}

type Project struct {
	ID           string
	Slug         string
	Name         string
	CanonicalKey string
	CreatedAt    time.Time
}

type ProjectLocation struct {
	ID           string
	ProjectID    string
	AbsolutePath string
	PathHash     string
	CreatedAt    time.Time
}

type WorkContext struct {
	ID                string
	PrimaryProjectID  string
	ProjectLocationID string
	SessionID         string
	CWD               string
	GitRoot           string
	GitBranch         string
	GitCommit         string
	StartedAt         time.Time
	FinishedAt        *time.Time
	ResolutionMethod  ResolutionMethod
	Confidence        Confidence
	EvidenceJSON      string
}

type UsageAllocation struct {
	SubjectType           string
	SubjectID             string
	ProjectID             string
	AllocationBasisPoints int64
	Method                string
	Confidence            Confidence
}

type RawEvent struct {
	ID                   string
	Source               string
	SessionID            string
	EventType            string
	OccurredAt           time.Time
	ProjectID            string
	ProjectLocationID    string
	WorkContextID        string
	ResolutionMethod     ResolutionMethod
	ResolutionConfidence Confidence
	EvidenceJSON         string
	PayloadJSONSanitized string
	PreviousEventHash    string
	EventHash            string
}

type Task struct {
	ID               string
	PrimaryProjectID string
	Title            string
	TaskType         string
	Status           string
	StartedAt        time.Time
	FinishedAt       *time.Time
}

type ModelCall struct {
	ID                     string
	PrimaryProjectID       string
	TaskID                 string
	SessionID              string
	Provider               string
	ModelID                string
	InputTokens            int64
	OutputTokens           int64
	TotalTokens            int64
	EstimatedCostUSDMicros int64
	ActualCostUSDMicros    *int64
	OccurredAt             time.Time
}

type ToolCall struct {
	ID               string
	PrimaryProjectID string
	ModelCallID      string
	TaskID           string
	SessionID        string
	ToolName         string
	StartedAt        time.Time
	FinishedAt       *time.Time
	Success          bool
}
