package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type codexAdapter struct{ commandAdapter }

type codexManagedValue struct {
	Exists bool   `json:"exists"`
	Value  string `json:"value,omitempty"`
}

type codexManagedState struct {
	OriginalExporter  codexManagedValue `json:"original_exporter"`
	OriginalLogPrompt codexManagedValue `json:"original_log_user_prompt"`
	ManagedExporter   bool              `json:"managed_exporter"`
	ManagedLogPrompt  bool              `json:"managed_log_user_prompt"`
	CreatedOTel       bool              `json:"created_otel"`
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
	updated, state, changed, err := updateCodexOTel(string(contents), codexOTelSettings())
	if err != nil {
		return SetupChange{}, err
	}
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
	_, stateErr := readCodexManagedState(statePath)
	if stateErr != nil && !os.IsNotExist(stateErr) {
		return SetupChange{}, stateErr
	}
	if os.IsNotExist(stateErr) {
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
	updated, changed, err := restoreCodexOTel(string(contents), state, codexOTelSettings())
	if err != nil {
		return SetupChange{}, err
	}
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

type codexOTelRegion struct {
	start int
	end   int
	text  string
}

type codexTextRange struct {
	start int
	end   int
}

type parsedCodexOTel struct {
	hasOTel        bool
	exporter       codexManagedValue
	logUserPrompt  codexManagedValue
	exporterRanges []codexTextRange
	logRange       *codexTextRange
	otelHeaderEnd  int
}

func updateCodexOTel(contents string, desired map[string]string) (string, codexManagedState, bool, error) {
	var document map[string]any
	if err := toml.Unmarshal([]byte(contents), &document); err != nil {
		return "", codexManagedState{}, false, fmt.Errorf("parse Codex TOML: %w", err)
	}
	region := locateOTelRegion(contents)
	current, err := parseOTelRegion(region.text)
	if err != nil {
		return "", codexManagedState{}, false, err
	}
	exporterChanged := !exporterMatches(current.exporter, desired["exporter"])
	logChanged := !exporterMatches(current.logUserPrompt, desired["log_user_prompt"])
	if !exporterChanged && !logChanged {
		return contents, codexManagedState{}, false, nil
	}
	state := codexManagedState{CreatedOTel: !current.hasOTel}
	if exporterChanged {
		state.OriginalExporter = current.exporter
		state.ManagedExporter = true
	}
	if logChanged {
		state.OriginalLogPrompt = current.logUserPrompt
		state.ManagedLogPrompt = true
	}
	return replaceOTelRegion(contents, region, renderOTelRegion(region.text, current, desired, exporterChanged, logChanged)), state, true, nil
}

func restoreCodexOTel(contents string, state codexManagedState, desired map[string]string) (string, bool, error) {
	var document map[string]any
	if err := toml.Unmarshal([]byte(contents), &document); err != nil {
		return "", false, fmt.Errorf("parse Codex TOML: %w", err)
	}
	region := locateOTelRegion(contents)
	if region.start == -1 {
		return contents, false, nil
	}
	current, err := parseOTelRegion(region.text)
	if err != nil {
		return "", false, err
	}
	updated := region.text
	changed := false
	if state.ManagedLogPrompt {
		if exporterMatches(current.logUserPrompt, desired["log_user_prompt"]) && current.logRange != nil {
			replacement := ""
			if state.OriginalLogPrompt.Exists {
				replacement = state.OriginalLogPrompt.Value
			}
			updated = replaceTextRange(updated, *current.logRange, replacement)
			changed = true
		}
	}
	if state.ManagedExporter {
		current, err = parseOTelRegion(updated)
		if err != nil {
			return "", false, err
		}
		if exporterMatches(current.exporter, desired["exporter"]) && len(current.exporterRanges) == 1 {
			replacement := ""
			if state.OriginalExporter.Exists {
				replacement = state.OriginalExporter.Value
			}
			updated = replaceTextRange(updated, current.exporterRanges[0], replacement)
			changed = true
		}
	}
	if state.CreatedOTel && strings.HasPrefix(updated, "[otel]\n") {
		updated = strings.TrimPrefix(updated, "[otel]\n")
		changed = true
	}
	if !changed {
		return contents, false, nil
	}
	return replaceOTelRegion(contents, region, updated), true, nil
}

func locateOTelRegion(contents string) codexOTelRegion {
	headers := tomlTableHeaders(contents)
	for index, header := range headers {
		if header.name != "otel" && !strings.HasPrefix(header.name, "otel.") {
			continue
		}
		end := len(contents)
		for _, next := range headers[index+1:] {
			if next.name != "otel" && !strings.HasPrefix(next.name, "otel.") {
				end = next.start
				break
			}
		}
		return codexOTelRegion{start: header.start, end: end, text: contents[header.start:end]}
	}
	return codexOTelRegion{start: -1, end: -1}
}

func parseOTelRegion(contents string) (parsedCodexOTel, error) {
	if contents == "" {
		return parsedCodexOTel{}, nil
	}
	headers := tomlTableHeaders(contents)
	parsed := parsedCodexOTel{}
	for index, header := range headers {
		if header.name == "otel" {
			parsed.hasOTel = true
			parsed.otelHeaderEnd = header.end
			sectionEnd := len(contents)
			if index+1 < len(headers) {
				sectionEnd = headers[index+1].start
			}
			for _, line := range textLines(contents[header.end:sectionEnd], header.end) {
				key, _, found := strings.Cut(strings.TrimSpace(line.text), "=")
				if !found {
					continue
				}
				key = strings.TrimSpace(key)
				switch {
				case key == "exporter" || strings.HasPrefix(key, "exporter."):
					parsed.exporter.Exists = true
					parsed.exporter.Value += line.text
					parsed.exporterRanges = append(parsed.exporterRanges, codexTextRange{start: line.start, end: line.end})
				case key == "log_user_prompt":
					value := codexTextRange{start: line.start, end: line.end}
					parsed.logUserPrompt = codexManagedValue{Exists: true, Value: line.text}
					parsed.logRange = &value
				}
			}
		}
		if strings.HasPrefix(header.name, "otel.exporter") {
			sectionEnd := len(contents)
			if index+1 < len(headers) {
				sectionEnd = headers[index+1].start
			}
			parsed.exporter.Exists = true
			parsed.exporter.Value += contents[header.start:sectionEnd]
			parsed.exporterRanges = append(parsed.exporterRanges, codexTextRange{start: header.start, end: sectionEnd})
		}
	}
	sort.Slice(parsed.exporterRanges, func(left, right int) bool {
		return parsed.exporterRanges[left].start < parsed.exporterRanges[right].start
	})
	return parsed, nil
}

func exporterMatches(value codexManagedValue, want string) bool {
	if !value.Exists {
		return false
	}
	line := strings.TrimSpace(value.Value)
	if strings.Contains(line, "\n") {
		return false
	}
	_, actual, found := strings.Cut(line, "=")
	return found && strings.TrimSpace(actual) == want
}

func renderOTelRegion(contents string, current parsedCodexOTel, desired map[string]string, exporterChanged, logChanged bool) string {
	if !current.hasOTel {
		contents = "[otel]\n" + contents
		offset := len("[otel]\n")
		for index := range current.exporterRanges {
			current.exporterRanges[index].start += offset
			current.exporterRanges[index].end += offset
		}
		if current.logRange != nil {
			current.logRange.start += offset
			current.logRange.end += offset
		}
		current.otelHeaderEnd = offset
	}
	type replacement struct {
		codexTextRange
		text string
	}
	replacements := []replacement{}
	if exporterChanged {
		if len(current.exporterRanges) == 0 {
			current.exporterRanges = []codexTextRange{{start: current.otelHeaderEnd, end: current.otelHeaderEnd}}
		}
		for index, value := range current.exporterRanges {
			text := ""
			if index == 0 {
				text = "exporter = " + desired["exporter"] + "\n"
				if !current.logUserPrompt.Exists && logChanged {
					text += "log_user_prompt = " + desired["log_user_prompt"] + "\n"
				}
			}
			replacements = append(replacements, replacement{codexTextRange: value, text: text})
		}
	}
	if logChanged && current.logRange != nil {
		replacements = append(replacements, replacement{codexTextRange: *current.logRange, text: "log_user_prompt = " + desired["log_user_prompt"] + "\n"})
	}
	if logChanged && current.logRange == nil && !exporterChanged && len(current.exporterRanges) == 1 {
		exporter := current.exporterRanges[0]
		replacements = append(replacements, replacement{codexTextRange: codexTextRange{start: exporter.end, end: exporter.end}, text: "log_user_prompt = " + desired["log_user_prompt"] + "\n"})
	}
	sort.Slice(replacements, func(left, right int) bool { return replacements[left].start < replacements[right].start })
	var rendered strings.Builder
	position := 0
	for _, replacement := range replacements {
		rendered.WriteString(contents[position:replacement.start])
		rendered.WriteString(replacement.text)
		position = replacement.end
	}
	rendered.WriteString(contents[position:])
	return rendered.String()
}

func replaceOTelRegion(contents string, region codexOTelRegion, replacement string) string {
	if region.start == -1 {
		if contents != "" && !strings.HasSuffix(contents, "\n") {
			contents += "\n"
		}
		return contents + "\n" + replacement
	}
	return contents[:region.start] + replacement + contents[region.end:]
}

func replaceTextRange(contents string, value codexTextRange, replacement string) string {
	return contents[:value.start] + replacement + contents[value.end:]
}

type tomlHeader struct {
	name       string
	start, end int
}

func tomlTableHeaders(contents string) []tomlHeader {
	headers := []tomlHeader{}
	for _, line := range textLines(contents, 0) {
		trimmed := strings.TrimSpace(line.text)
		if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") || strings.HasPrefix(trimmed, "[[") {
			continue
		}
		name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
		headers = append(headers, tomlHeader{name: name, start: line.start, end: line.end})
	}
	return headers
}

type textLine struct {
	text       string
	start, end int
}

func textLines(contents string, offset int) []textLine {
	lines := []textLine{}
	for start := 0; start < len(contents); {
		end := strings.IndexByte(contents[start:], '\n')
		if end < 0 {
			end = len(contents)
		} else {
			end += start + 1
		}
		lines = append(lines, textLine{text: contents[start:end], start: offset + start, end: offset + end})
		start = end
	}
	return lines
}

func codexOTelConfigured(path string) bool {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	updated, _, changed, err := updateCodexOTel(string(contents), codexOTelSettings())
	return err == nil && !changed && updated != ""
}
