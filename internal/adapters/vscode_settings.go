package adapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

const qlogVSCodeManagedKey = "qlog.managed.github.copilot.chat.otel"

func vscodeSettingsPath(variant string) string {
	if variant == "" {
		variant = "Code"
	}
	if root := os.Getenv("QLOG_ADAPTER_CONFIG_HOME"); root != "" {
		return filepath.Join(root, variant, "User", "settings.json")
	}
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, variant, "User", "settings.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(variant, "User", "settings.json")
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", variant, "User", "settings.json")
	default:
		return filepath.Join(home, ".config", variant, "User", "settings.json")
	}
}

func applyVSCodeSettings(path string, desired map[string]any, owner string, dryRun bool) (SetupChange, error) {
	settings, original, existed, err := readVSCodeSettings(path)
	if err != nil {
		return SetupChange{}, err
	}

	changed := false
	managed := make(map[string]any, len(desired))
	for key, value := range desired {
		if !reflect.DeepEqual(settings[key], value) {
			settings[key] = value
			changed = true
		}
		managed[key] = value
	}
	marker := map[string]any{owner: managed}
	if !reflect.DeepEqual(settings[qlogVSCodeManagedKey], marker) {
		settings[qlogVSCodeManagedKey] = marker
		changed = true
	}
	if !changed {
		return SetupChange{Path: path, Action: "unchanged", Description: "qlog settings already up to date"}, nil
	}

	action := "created"
	if existed {
		action = "updated"
	}
	if dryRun {
		change := SetupChange{Path: path, Action: action, Description: "dry run: qlog settings would be written"}
		if existed {
			change.BackupPath = plannedBackupPath(path)
		}
		return change, nil
	}
	return writeVSCodeSettings(path, settings, original, existed, action)
}

func removeVSCodeSettings(path string, desired map[string]any, owner string, dryRun bool) (SetupChange, error) {
	settings, original, existed, err := readVSCodeSettings(path)
	if err != nil {
		return SetupChange{}, err
	}
	if !existed {
		return SetupChange{Path: path, Action: "unchanged", Description: "settings file does not exist"}, nil
	}

	managed := managedVSCodeSettings(settings, owner)
	changed := false
	for key := range managed {
		if _, found := settings[key]; found {
			delete(settings, key)
			changed = true
		}
	}
	if _, found := settings[qlogVSCodeManagedKey]; found {
		delete(settings, qlogVSCodeManagedKey)
		changed = true
	}
	if !changed {
		return SetupChange{Path: path, Action: "unchanged", Description: "no qlog-managed settings found"}, nil
	}
	if dryRun {
		return SetupChange{Path: path, Action: "removed", BackupPath: plannedBackupPath(path), Description: "dry run: qlog settings would be removed"}, nil
	}
	return writeVSCodeSettings(path, settings, original, true, "removed")
}

func readVSCodeSettings(path string) (map[string]any, []byte, bool, error) {
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	settings := map[string]any{}
	cleaned := stripJSONC(contents)
	if len(bytes.TrimSpace(cleaned)) > 0 {
		if err := json.Unmarshal(cleaned, &settings); err != nil {
			return nil, nil, false, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	return settings, contents, true, nil
}

func writeVSCodeSettings(path string, settings map[string]any, original []byte, existed bool, action string) (SetupChange, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return SetupChange{}, fmt.Errorf("create parent directory: %w", err)
	}
	change := SetupChange{Path: path, Action: action, Description: "qlog settings written"}
	if existed {
		backupPath := plannedBackupPath(path)
		if err := os.WriteFile(backupPath, original, 0o600); err != nil {
			return SetupChange{}, fmt.Errorf("write backup: %w", err)
		}
		change.BackupPath = backupPath
	}
	next, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return SetupChange{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".qlog-tmp-")
	if err != nil {
		return SetupChange{}, fmt.Errorf("create temp settings: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(append(next, '\n')); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return SetupChange{}, fmt.Errorf("write temp settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return SetupChange{}, fmt.Errorf("close temp settings: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return SetupChange{}, fmt.Errorf("replace %s: %w", path, err)
	}
	return change, nil
}

func managedVSCodeSettings(settings map[string]any, owner string) map[string]any {
	marker, ok := settings[qlogVSCodeManagedKey].(map[string]any)
	if !ok {
		return nil
	}
	managed, ok := marker[owner].(map[string]any)
	if !ok {
		return nil
	}
	return managed
}

func stripJSONC(input []byte) []byte {
	var output strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if inString {
			output.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			output.WriteByte(ch)
			continue
		}
		if ch == '/' && i+1 < len(input) && input[i+1] == '/' {
			for i < len(input) && input[i] != '\n' {
				i++
			}
			output.WriteByte('\n')
			continue
		}
		if ch == '/' && i+1 < len(input) && input[i+1] == '*' {
			i += 2
			for i+1 < len(input) && !(input[i] == '*' && input[i+1] == '/') {
				i++
			}
			i++
			continue
		}
		output.WriteByte(ch)
	}
	return removeTrailingCommas([]byte(output.String()))
}

func removeTrailingCommas(input []byte) []byte {
	output := make([]byte, 0, len(input))
	for i := 0; i < len(input); i++ {
		if input[i] == ',' {
			j := i + 1
			for j < len(input) && (input[j] == ' ' || input[j] == '\n' || input[j] == '\r' || input[j] == '\t') {
				j++
			}
			if j < len(input) && (input[j] == '}' || input[j] == ']') {
				continue
			}
		}
		output = append(output, input[i])
	}
	return output
}
