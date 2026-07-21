package adapters

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CaptureQuality string

const (
	CaptureProviderReported CaptureQuality = "provider_reported"
	CaptureAgentReported    CaptureQuality = "agent_reported"
	CaptureOTELReported     CaptureQuality = "otel_reported"
	CaptureLifecycleOnly    CaptureQuality = "lifecycle_only"
	CaptureExperimental     CaptureQuality = "experimental"
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

func ApplyMarkerBlock(path, marker, content string, dryRun bool) (SetupChange, error) {
	begin, end := markerBounds(marker)
	replacement := begin + "\n" + content + "\n" + end + "\n"
	currentBytes, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return SetupChange{}, fmt.Errorf("read %s: %w", path, err)
	}
	current := string(currentBytes)
	action := "created"
	next := replacement
	if err == nil {
		action = "updated"
		next = replaceOrAppendMarker(current, begin, end, replacement)
	}
	if current == next {
		return SetupChange{Path: path, Action: "unchanged", Description: "qlog marker block already up to date"}, nil
	}
	if dryRun {
		if err == nil {
			action = "update"
		} else {
			action = "create"
		}
		change := SetupChange{Path: path, Action: action, Description: "dry run: qlog marker block would be written"}
		if err == nil {
			change.BackupPath = plannedBackupPath(path)
		}
		return change, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return SetupChange{}, fmt.Errorf("create parent directory: %w", err)
	}
	change := SetupChange{Path: path, Action: action, Description: "qlog marker block written"}
	if err == nil {
		backupPath := plannedBackupPath(path)
		if err := os.WriteFile(backupPath, currentBytes, 0o600); err != nil {
			return SetupChange{}, fmt.Errorf("write backup: %w", err)
		}
		change.BackupPath = backupPath
	}
	if err := os.WriteFile(path, []byte(next), 0o600); err != nil {
		return SetupChange{}, fmt.Errorf("write %s: %w", path, err)
	}
	return change, nil
}

func RemoveMarkerBlock(path, marker string, dryRun bool) (SetupChange, error) {
	begin, end := markerBounds(marker)
	currentBytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SetupChange{Path: path, Action: "unchanged", Description: "qlog marker block not installed"}, nil
		}
		return SetupChange{}, fmt.Errorf("read %s: %w", path, err)
	}
	current := string(currentBytes)
	start := strings.Index(current, begin)
	finish := strings.Index(current, end)
	if start < 0 || finish < start {
		return SetupChange{Path: path, Action: "unchanged", Description: "qlog marker block not installed"}, nil
	}
	finish += len(end)
	for finish < len(current) && (current[finish] == '\r' || current[finish] == '\n') {
		finish++
	}
	next := current[:start] + current[finish:]
	change := SetupChange{Path: path, Action: "removed", BackupPath: plannedBackupPath(path), Description: "qlog marker block removed"}
	if dryRun {
		change.Description = "dry run: " + change.Description
		return change, nil
	}
	if err := os.WriteFile(change.BackupPath, currentBytes, 0o600); err != nil {
		return SetupChange{}, fmt.Errorf("write backup: %w", err)
	}
	if err := os.WriteFile(path, []byte(next), 0o600); err != nil {
		return SetupChange{}, fmt.Errorf("write %s: %w", path, err)
	}
	return change, nil
}

func plannedBackupPath(path string) string {
	return fmt.Sprintf("%s.qlog-backup-%s", path, time.Now().UTC().Format("20060102150405"))
}

func HasMarkerBlock(path, marker string) bool {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	begin, end := markerBounds(marker)
	text := string(contents)
	return strings.Contains(text, begin) && strings.Contains(text, end)
}

func markerBounds(marker string) (string, string) {
	return "<!-- qlog:begin " + marker + " -->", "<!-- qlog:end " + marker + " -->"
}

func replaceOrAppendMarker(current, begin, end, replacement string) string {
	start := strings.Index(current, begin)
	finish := strings.Index(current, end)
	if start >= 0 && finish >= start {
		finish += len(end)
		for finish < len(current) && (current[finish] == '\r' || current[finish] == '\n') {
			finish++
		}
		return current[:start] + replacement + current[finish:]
	}
	separator := ""
	if current != "" && !strings.HasSuffix(current, "\n") {
		separator = "\n"
	}
	return current + separator + replacement
}
