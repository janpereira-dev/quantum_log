package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type claudeCodeAdapter struct{}

func newClaudeCodeAdapter() claudeCodeAdapter { return claudeCodeAdapter{} }

func (claudeCodeAdapter) Descriptor() Descriptor {
	return Descriptor{ID: "claude-code", Name: "Claude Code", Version: "hooks-otel", Stable: true, Capabilities: Capabilities{ModelIdentity: true, InputTokens: true, OutputTokens: true, CacheTokens: true, SessionLifecycle: true, ProjectIdentity: true, WorkingDirectory: true, StructuredEvents: true}}
}

func (claudeCodeAdapter) Detect(context.Context) (Detection, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return Detection{Evidence: "claude not found on PATH"}, nil
	}
	return Detection{Available: true, Evidence: path}, nil
}

func (a claudeCodeAdapter) Install(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := a.applySettings(options.DryRun, options.Home, options.ExecutablePath)
	if err != nil {
		return InstallResult{}, err
	}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
	}
	return InstallResult{Changed: !options.DryRun && (change.Action == "created" || change.Action == "updated"), Actions: []string{formatChange(change)}, Changes: []SetupChange{change}}, nil
}

func (a claudeCodeAdapter) PlanInstall(_ context.Context, options SetupOptions) (SetupPlan, error) {
	change, err := a.applySettings(true, options.Home, "")
	if err != nil {
		return SetupPlan{}, err
	}
	if options.DryRun {
		change.Description = "dry run: " + change.Description
	}
	return SetupPlan{AdapterID: "claude-code", State: SetupPartial, CaptureQuality: CaptureOTELReported, Changes: []SetupChange{change}, Notes: []string{"installs Claude Code lifecycle hooks and trace-only OTel configuration", "OTel message content capture is disabled; source E2E evidence remains required"}}, nil
}

func (a claudeCodeAdapter) Status(ctx context.Context) (SetupStatus, error) {
	detection, err := a.Detect(ctx)
	if err != nil {
		return SetupStatus{}, err
	}
	installed := claudeSettingsHasQlog(a.settingsPath())
	state := SetupUnavailable
	if detection.Available || installed {
		state = SetupPartial
	}
	quality := CaptureLifecycleOnly
	if claudeSettingsHasOTEL(a.settingsPath()) {
		quality = CaptureOTELReported
	}
	return SetupStatus{AdapterID: "claude-code", Available: detection.Available, Installed: installed, State: state, InstallationState: state, CaptureQuality: quality, Evidence: detection.Evidence, Notes: []string{"Claude Code hooks retain lifecycle and CWD evidence; trace-only OTel disables message content capture", "No source E2E evidence is claimed by setup"}}, nil
}

func (a claudeCodeAdapter) Test(ctx context.Context) (TestResult, error) {
	detection, err := a.Detect(ctx)
	if err != nil {
		return TestResult{}, err
	}
	message := detection.Evidence
	if !detection.Available {
		message = "adapter unavailable: " + detection.Evidence
	}
	return TestResult{AdapterID: "claude-code", Passed: detection.Available, CaptureQuality: CaptureLifecycleOnly, Message: message, TestedAt: time.Now().UTC()}, nil
}

func (a claudeCodeAdapter) Uninstall(_ context.Context, options InstallOptions) (InstallResult, error) {
	change, err := a.removeSettings(options.DryRun)
	if err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Changed: !options.DryRun && change.Action == "removed", Actions: []string{formatChange(change)}, Changes: []SetupChange{change}}, nil
}

func (a claudeCodeAdapter) HealthCheck(ctx context.Context) error {
	detection, err := a.Detect(ctx)
	if err != nil {
		return err
	}
	if !detection.Available {
		return errors.New(detection.Evidence)
	}
	return nil
}

func (claudeCodeAdapter) Ingest(context.Context, io.Reader) ([]RawRecord, error) {
	return nil, errors.New("claude code hooks post directly to qlog /v1/events")
}

func (claudeCodeAdapter) Normalize(record RawRecord) (RawRecord, error) { return record, nil }

func (claudeCodeAdapter) ExtractProjectSignals(RawRecord) ProjectSignals { return ProjectSignals{} }

func (a claudeCodeAdapter) applySettings(dryRun bool, home, executablePath string) (SetupChange, error) {
	path := a.settingsPath()
	current, _ := os.ReadFile(path)
	next, err := claudeSettingsWithQlogHooks(current, claudeCodeHookCommand(home, executablePath))
	if err != nil {
		return SetupChange{}, err
	}
	action := "created"
	if len(current) > 0 {
		action = "updated"
	}
	if string(current) == string(next) {
		action = "unchanged"
	}
	change := SetupChange{Path: path, Action: action, Description: "Claude Code lifecycle hooks call " + claudeCodeHookCommand(home, executablePath)}
	if dryRun || action == "unchanged" {
		return change, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return SetupChange{}, err
	}
	if len(current) > 0 {
		backup := path + ".qlog-backup-" + time.Now().UTC().Format("20060102150405")
		if err := os.WriteFile(backup, current, 0o600); err != nil {
			return SetupChange{}, err
		}
		change.BackupPath = backup
	}
	return change, os.WriteFile(path, next, 0o600)
}

func (a claudeCodeAdapter) removeSettings(dryRun bool) (SetupChange, error) {
	path := a.settingsPath()
	current, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return SetupChange{Path: path, Action: "unchanged", Description: "Claude Code qlog lifecycle hooks already absent"}, nil
	}
	if err != nil {
		return SetupChange{}, err
	}
	next, err := claudeSettingsWithoutQlogHooks(current)
	if err != nil {
		return SetupChange{}, err
	}
	change := SetupChange{Path: path, Action: "unchanged", Description: "Claude Code qlog lifecycle hooks already absent"}
	if string(current) == string(next) {
		return change, nil
	}
	change.Action = "removed"
	change.Description = "removed qlog-owned Claude Code lifecycle hooks and OTel environment"
	if dryRun {
		return change, nil
	}
	backup := path + ".qlog-backup-" + time.Now().UTC().Format("20060102150405")
	if err := os.WriteFile(backup, current, 0o600); err != nil {
		return SetupChange{}, err
	}
	change.BackupPath = backup
	return change, os.WriteFile(path, next, 0o600)
}

func (claudeCodeAdapter) settingsPath() string {
	if root := os.Getenv("QLOG_ADAPTER_CONFIG_HOME"); root != "" {
		return filepath.Join(root, ".claude", "settings.json")
	}
	if dir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(dir, ".claude", "settings.json")
	}
	return filepath.Join(".claude", "settings.json")
}

func claudeSettingsHasQlog(path string) bool {
	contents, err := os.ReadFile(path)
	return err == nil && bytesContains(contents, []byte("qlog")) && bytesContains(contents, []byte("hook claude-code"))
}

func claudeSettingsHasOTEL(path string) bool {
	contents, err := os.ReadFile(path)
	return err == nil && bytesContains(contents, []byte("CLAUDE_CODE_ENABLE_TELEMETRY")) && bytesContains(contents, []byte("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT"))
}

func claudeCodeHookCommand(home, executablePath string) string {
	if strings.TrimSpace(executablePath) != "" {
		command := shellQuote(executablePath)
		if strings.TrimSpace(home) != "" {
			command += " --home " + shellQuote(home)
		}
		return command + " hook claude-code"
	}
	if strings.TrimSpace(home) == "" {
		return "qlog hook claude-code"
	}
	return "qlog --home " + strconv.Quote(home) + " hook claude-code"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func claudeSettingsWithQlogHooks(current []byte, command string) ([]byte, error) {
	settings := map[string]any{}
	if len(current) > 0 {
		if err := json.Unmarshal(current, &settings); err != nil {
			return nil, err
		}
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	for _, event := range []string{"SessionStart", "UserPromptSubmit", "Stop", "SubagentStop"} {
		hooks[event] = claudeHookEntriesWithQlog(hooks[event], command)
	}
	settings["hooks"] = hooks
	env, _ := settings["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
	}
	for key, value := range claudeCodeOTELEnvironment() {
		env[key] = value
	}
	settings["env"] = env
	return json.MarshalIndent(settings, "", "  ")
}

func claudeCodeOTELEnvironment() map[string]string {
	return map[string]string{
		"CLAUDE_CODE_ENABLE_TELEMETRY":                       "1",
		"CLAUDE_CODE_ENHANCED_TELEMETRY_BETA":                "1",
		"OTEL_TRACES_EXPORTER":                               "otlp",
		"OTEL_METRICS_EXPORTER":                              "none",
		"OTEL_LOGS_EXPORTER":                                 "none",
		"OTEL_EXPORTER_OTLP_PROTOCOL":                        "http/json",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT":                 "http://127.0.0.1:4318/v1/traces",
		"OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT": "false",
	}
}

func claudeSettingsWithoutQlogHooks(current []byte) ([]byte, error) {
	settings := map[string]any{}
	if err := json.Unmarshal(current, &settings); err != nil {
		return nil, err
	}
	changed := false
	if hooks, ok := settings["hooks"].(map[string]any); ok {
		for event, entries := range hooks {
			currentEntries, ok := entries.([]any)
			if !ok {
				continue
			}
			nextEntries := make([]any, 0, len(currentEntries))
			for _, entry := range currentEntries {
				cleaned, keep := claudeHookEntryWithoutQlog(entry)
				if !keep {
					changed = true
					continue
				}
				if !reflect.DeepEqual(cleaned, entry) {
					changed = true
				}
				nextEntries = append(nextEntries, cleaned)
			}
			if len(nextEntries) == 0 {
				delete(hooks, event)
				changed = true
				continue
			}
			hooks[event] = nextEntries
		}
		if len(hooks) == 0 {
			delete(settings, "hooks")
		}
	}
	if env, ok := settings["env"].(map[string]any); ok {
		for key, value := range claudeCodeOTELEnvironment() {
			if env[key] == value {
				delete(env, key)
				changed = true
			}
		}
		if len(env) == 0 {
			delete(settings, "env")
		}
	}
	if !changed {
		return current, nil
	}
	return json.MarshalIndent(settings, "", "  ")
}

func claudeHookEntriesWithQlog(current any, command string) []any {
	entries, _ := current.([]any)
	next := make([]any, 0, len(entries)+1)
	for _, entry := range entries {
		cleaned, keep := claudeHookEntryWithoutQlog(entry)
		if keep {
			next = append(next, cleaned)
		}
	}
	return append(next, map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command}}})
}

func claudeHookEntryWithoutQlog(entry any) (any, bool) {
	object, ok := entry.(map[string]any)
	if !ok {
		return entry, true
	}
	rawHooks, ok := object["hooks"].([]any)
	if !ok {
		return entry, true
	}
	cleanHooks := make([]any, 0, len(rawHooks))
	for _, hook := range rawHooks {
		if !isQlogClaudeCommandHook(hook) {
			cleanHooks = append(cleanHooks, hook)
		}
	}
	if len(cleanHooks) == 0 {
		return nil, false
	}
	clone := make(map[string]any, len(object))
	for key, value := range object {
		clone[key] = value
	}
	clone["hooks"] = cleanHooks
	return clone, true
}

func isQlogClaudeCommandHook(hook any) bool {
	object, ok := hook.(map[string]any)
	if !ok {
		return false
	}
	command, _ := object["command"].(string)
	typeName, _ := object["type"].(string)
	if typeName != "command" {
		return false
	}
	if command == claudeCodeHookCommand("", "") {
		return true
	}
	if isQlogExecutableHookCommand(command) {
		return true
	}
	const prefix = "qlog --home "
	const suffix = " hook claude-code"
	encodedHome, ok := strings.CutPrefix(command, prefix)
	if !ok {
		return false
	}
	encodedHome, ok = strings.CutSuffix(encodedHome, suffix)
	if !ok {
		return false
	}
	home, err := strconv.Unquote(encodedHome)
	return err == nil && home != ""
}

func isQlogExecutableHookCommand(command string) bool {
	const suffix = " hook claude-code"
	if !strings.HasSuffix(command, suffix) || !strings.HasPrefix(command, "'") {
		return false
	}
	withoutSuffix := strings.TrimSuffix(command, suffix)
	separator := "' --home "
	index := strings.Index(withoutSuffix, separator)
	if index == -1 {
		return false
	}
	executable := withoutSuffix[1:index]
	return strings.HasSuffix(executable, "/qlog") || strings.HasSuffix(strings.ToLower(executable), `\qlog.exe`)
}

func bytesContains(haystack, needle []byte) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if string(haystack[i:i+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}
