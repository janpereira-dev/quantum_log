package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type codexAdapter struct{ commandAdapter }

type codexManagedValue struct {
	Exists bool   `json:"exists"`
	Value  string `json:"value,omitempty"`
}

type codexManagedState struct {
	Original map[string]codexManagedValue `json:"original"`
}

func newCodexAdapter() codexAdapter {
	return codexAdapter{commandAdapter: newCommandAdapter("codex", "Codex", "codex", ".codex/config.toml")}
}

func (a codexAdapter) Descriptor() Descriptor {
	return Descriptor{
		ID:      "codex",
		Name:    "Codex",
		Version: "otel-logs",
		Stable:  true,
		Capabilities: Capabilities{
			ModelIdentity:    true,
			InputTokens:      true,
			OutputTokens:     true,
			CacheTokens:      true,
			ReasoningTokens:  true,
			StructuredEvents: true,
		},
	}
}

func (a codexAdapter) Install(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := applyCodexOTelConfig(a.configPath(), a.statePath(), options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Changed: !options.DryRun && (change.Action == "created" || change.Action == "updated"), Actions: []string{formatChange(change)}, Changes: []SetupChange{change}}, nil
}

func (a codexAdapter) Uninstall(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := removeCodexOTelConfig(a.configPath(), a.statePath(), options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Changed: !options.DryRun && change.Action == "removed", Actions: []string{formatChange(change)}, Changes: []SetupChange{change}}, nil
}

func (a codexAdapter) PlanInstall(_ context.Context, options SetupOptions) (SetupPlan, error) {
	change, err := applyCodexOTelConfig(a.configPath(), a.statePath(), true)
	if err != nil {
		return SetupPlan{}, err
	}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
	}
	return SetupPlan{AdapterID: a.id, State: SetupAvailable, CaptureQuality: CaptureOTELReported, Changes: []SetupChange{change}, Notes: []string{"qlog manages user-level Codex OTLP logs with user prompt logging disabled"}}, nil
}

func (a codexAdapter) Status(ctx context.Context) (SetupStatus, error) {
	detection, err := a.Detect(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	_, stateErr := os.Stat(a.statePath())
	installed := codexOTelConfigured(a.configPath()) && stateErr == nil
	state := SetupUnavailable
	if detection.Available {
		state = SetupAvailable
	}
	if installed {
		state = SetupInstalled
	}
	return SetupStatus{AdapterID: a.id, Available: detection.Available, Installed: installed, State: state, InstallationState: state, CaptureQuality: CaptureOTELReported, Evidence: detection.Evidence, Notes: []string{"Codex OTLP logs become verifiable only after clean-device response.completed evidence"}}, nil
}

func (a codexAdapter) Test(ctx context.Context) (TestResult, error) {
	status, err := a.Status(ctx)
	if err != nil {
		return TestResult{}, err
	}
	return TestResult{AdapterID: a.id, Passed: status.Available && status.Installed, CaptureQuality: CaptureOTELReported, Message: status.Evidence, TestedAt: time.Now().UTC()}, nil
}

func (a codexAdapter) statePath() string {
	return filepath.Join(filepath.Dir(a.configPath()), "qlog-otel-state.json")
}

func codexOTelSettings() map[string]string {
	return map[string]string{
		"exporter":        `{ otlp-http = { endpoint = "http://127.0.0.1:4318/v1/logs", protocol = "binary" } }`,
		"log_user_prompt": "false",
	}
}

func applyCodexOTelConfig(configPath, statePath string, dryRun bool) (SetupChange, error) {
	contents, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return SetupChange{}, fmt.Errorf("read Codex config: %w", err)
	}
	updated, original, changed := updateCodexOTel(string(contents), codexOTelSettings())
	action := "unchanged"
	if changed {
		action = "updated"
		if os.IsNotExist(err) {
			action = "created"
		}
	}
	change := SetupChange{Path: configPath, Action: action, Description: "configure qlog-managed Codex OTLP logs"}
	if !changed || dryRun {
		return change, nil
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return SetupChange{}, fmt.Errorf("create Codex config directory: %w", err)
	}
	state, stateErr := readCodexManagedState(statePath)
	if stateErr != nil && !os.IsNotExist(stateErr) {
		return SetupChange{}, stateErr
	}
	if os.IsNotExist(stateErr) {
		state = codexManagedState{}
	}
	if state.Original == nil {
		state.Original = original
		encoded, err := json.Marshal(state)
		if err != nil {
			return SetupChange{}, fmt.Errorf("encode Codex OTel state: %w", err)
		}
		if err := os.WriteFile(statePath, append(encoded, '\n'), 0o600); err != nil {
			return SetupChange{}, fmt.Errorf("write Codex OTel state: %w", err)
		}
	}
	if err := os.WriteFile(configPath, []byte(updated), 0o600); err != nil {
		return SetupChange{}, fmt.Errorf("write Codex config: %w", err)
	}
	return change, nil
}

func removeCodexOTelConfig(configPath, statePath string, dryRun bool) (SetupChange, error) {
	state, err := readCodexManagedState(statePath)
	if os.IsNotExist(err) {
		return SetupChange{Path: configPath, Action: "unchanged", Description: "remove qlog-managed Codex OTLP logs"}, nil
	}
	if err != nil {
		return SetupChange{}, err
	}
	contents, err := os.ReadFile(configPath)
	if err != nil {
		return SetupChange{}, fmt.Errorf("read Codex config: %w", err)
	}
	updated, changed := restoreCodexOTel(string(contents), state.Original, codexOTelSettings())
	change := SetupChange{Path: configPath, Action: "unchanged", Description: "remove qlog-managed Codex OTLP logs"}
	if changed {
		change.Action = "removed"
	}
	if dryRun {
		return change, nil
	}
	if changed {
		if err := os.WriteFile(configPath, []byte(updated), 0o600); err != nil {
			return SetupChange{}, fmt.Errorf("restore Codex config: %w", err)
		}
	}
	if err := os.Remove(statePath); err != nil {
		return SetupChange{}, fmt.Errorf("remove Codex OTel state: %w", err)
	}
	return change, nil
}

func readCodexManagedState(path string) (codexManagedState, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return codexManagedState{}, err
	}
	var state codexManagedState
	if err := json.Unmarshal(contents, &state); err != nil {
		return codexManagedState{}, fmt.Errorf("decode Codex OTel state: %w", err)
	}
	return state, nil
}

func updateCodexOTel(contents string, desired map[string]string) (string, map[string]codexManagedValue, bool) {
	lines := strings.Split(contents, "\n")
	sectionStart, sectionEnd := tomlSectionRange(lines, "otel")
	if sectionStart == -1 {
		if contents != "" && !strings.HasSuffix(contents, "\n") {
			contents += "\n"
		}
		contents += "\n[otel]\n"
		for _, key := range []string{"exporter", "log_user_prompt"} {
			contents += key + " = " + desired[key] + "\n"
		}
		return contents, map[string]codexManagedValue{"exporter": {}, "log_user_prompt": {}}, true
	}
	original := map[string]codexManagedValue{}
	changed := false
	for _, key := range []string{"exporter", "log_user_prompt"} {
		index, value := tomlKey(lines, sectionStart+1, sectionEnd, key)
		original[key] = codexManagedValue{Exists: index >= 0, Value: value}
		if value == desired[key] {
			continue
		}
		changed = true
		if index >= 0 {
			lines[index] = key + " = " + desired[key]
		} else {
			lines = append(lines[:sectionEnd], append([]string{key + " = " + desired[key]}, lines[sectionEnd:]...)...)
			sectionEnd++
		}
	}
	return strings.Join(lines, "\n"), original, changed
}

func restoreCodexOTel(contents string, original map[string]codexManagedValue, desired map[string]string) (string, bool) {
	lines := strings.Split(contents, "\n")
	sectionStart, sectionEnd := tomlSectionRange(lines, "otel")
	if sectionStart == -1 {
		return contents, false
	}
	changed := false
	for _, key := range []string{"exporter", "log_user_prompt"} {
		index, value := tomlKey(lines, sectionStart+1, sectionEnd, key)
		if index < 0 || value != desired[key] {
			continue
		}
		changed = true
		if original[key].Exists {
			lines[index] = key + " = " + original[key].Value
		} else {
			lines = append(lines[:index], lines[index+1:]...)
			sectionEnd--
		}
	}
	return strings.Join(lines, "\n"), changed
}

func codexOTelConfigured(path string) bool {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	updated, _, changed := updateCodexOTel(string(contents), codexOTelSettings())
	return !changed && updated != ""
}

func tomlSectionRange(lines []string, name string) (int, int) {
	start := -1
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "["+name+"]" {
			start = index
			continue
		}
		if start >= 0 && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			return start, index
		}
	}
	return start, len(lines)
}

func tomlKey(lines []string, start, end int, want string) (int, string) {
	for index := start; index < end; index++ {
		line := strings.TrimSpace(lines[index])
		key, value, found := strings.Cut(line, "=")
		if found && strings.TrimSpace(key) == want {
			return index, strings.TrimSpace(value)
		}
	}
	return -1, ""
}
