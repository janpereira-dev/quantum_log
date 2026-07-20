package adapters

import "time"

type CaptureQuality string

const (
	CaptureProviderReported CaptureQuality = "provider_reported"
	CaptureAgentReported    CaptureQuality = "agent_reported"
	CaptureOTELReported     CaptureQuality = "otel_reported"
	CaptureLifecycleOnly    CaptureQuality = "lifecycle_only"
	CaptureEstimated        CaptureQuality = "estimated"
	CaptureManualImport     CaptureQuality = "manual_import"
	CaptureUnavailable      CaptureQuality = "unavailable"
)

type SetupState string

const (
	SetupAvailable   SetupState = "available"
	SetupInstalled   SetupState = "installed"
	SetupPartial     SetupState = "partial"
	SetupDrifted     SetupState = "drifted"
	SetupUnavailable SetupState = "unavailable"
)

type SetupOptions struct {
	DryRun bool
	Yes    bool
}

type SetupChange struct {
	Path        string `json:"path"`
	Action      string `json:"action"`
	BackupPath  string `json:"backup_path,omitempty"`
	Description string `json:"description"`
}

type SetupPlan struct {
	AdapterID      string         `json:"adapter_id"`
	State          SetupState     `json:"state"`
	CaptureQuality CaptureQuality `json:"capture_quality"`
	Changes        []SetupChange  `json:"changes"`
	Notes          []string       `json:"notes"`
}

type SetupStatus struct {
	AdapterID      string         `json:"adapter_id"`
	Available      bool           `json:"available"`
	Installed      bool           `json:"installed"`
	State          SetupState     `json:"state"`
	CaptureQuality CaptureQuality `json:"capture_quality"`
	Evidence       string         `json:"evidence"`
	Notes          []string       `json:"notes,omitempty"`
}

type TestResult struct {
	AdapterID      string         `json:"adapter_id"`
	Passed         bool           `json:"passed"`
	CaptureQuality CaptureQuality `json:"capture_quality"`
	Message        string         `json:"message"`
	TestedAt       time.Time      `json:"tested_at"`
}
