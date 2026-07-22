package adapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
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
	state, hasState := vscodeManagedState(settings, owner)
	if !hasState {
		state = vscodeSettingsMarker{Managed: make(map[string]any, len(desired)), Previous: make(map[string]any, len(desired)), PreviousPresent: make(map[string]bool, len(desired))}
		for key := range desired {
			previous, found := settings[key]
			state.PreviousPresent[key] = found
			if found {
				state.Previous[key] = previous
			}
		}
	}
	state.Managed = make(map[string]any, len(desired))
	for key, value := range desired {
		if !reflect.DeepEqual(settings[key], value) {
			settings[key] = value
			changed = true
		}
		state.Managed[key] = value
	}
	marker := state.toMap(owner)
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
	return writeVSCodeSettings(path, settings, original, existed, action, append(vscodeSettingKeys(desired), qlogVSCodeManagedKey))
}

func removeVSCodeSettings(path string, desired map[string]any, owner string, dryRun bool) (SetupChange, error) {
	settings, original, existed, err := readVSCodeSettings(path)
	if err != nil {
		return SetupChange{}, err
	}
	if !existed {
		return SetupChange{Path: path, Action: "unchanged", Description: "settings file does not exist"}, nil
	}

	state, hasState := vscodeManagedState(settings, owner)
	managed := state.Managed
	if !hasState {
		managed = managedVSCodeSettings(settings, owner)
	}
	changed := false
	for key := range managed {
		if state.PreviousPresent[key] {
			if !reflect.DeepEqual(settings[key], state.Previous[key]) {
				settings[key] = state.Previous[key]
				changed = true
			}
		} else if _, found := settings[key]; found {
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
	return writeVSCodeSettings(path, settings, original, true, "removed", append(vscodeSettingKeys(managed), qlogVSCodeManagedKey))
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

func writeVSCodeSettings(path string, settings map[string]any, original []byte, existed bool, action string, replaceKeys []string) (SetupChange, error) {
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
	next, err := marshalVSCodeSettings(settings, original, existed, replaceKeys)
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

type vscodeSettingsMarker struct {
	Managed         map[string]any
	Previous        map[string]any
	PreviousPresent map[string]bool
}

func (m vscodeSettingsMarker) toMap(owner string) map[string]any {
	return map[string]any{owner: map[string]any{"managed": m.Managed, "previous": m.Previous, "previous_present": m.PreviousPresent}}
}

func vscodeManagedState(settings map[string]any, owner string) (vscodeSettingsMarker, bool) {
	marker, ok := settings[qlogVSCodeManagedKey].(map[string]any)
	if !ok {
		return vscodeSettingsMarker{}, false
	}
	ownerValue, ok := marker[owner].(map[string]any)
	if !ok {
		return vscodeSettingsMarker{}, false
	}
	managed, ok := ownerValue["managed"].(map[string]any)
	if !ok {
		return vscodeSettingsMarker{}, false
	}
	previous, _ := ownerValue["previous"].(map[string]any)
	previousPresent := map[string]bool{}
	if raw, ok := ownerValue["previous_present"].(map[string]any); ok {
		for key, value := range raw {
			if present, ok := value.(bool); ok {
				previousPresent[key] = present
			}
		}
	}
	return vscodeSettingsMarker{Managed: managed, Previous: previous, PreviousPresent: previousPresent}, true
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

func vscodeSettingKeys(settings map[string]any) []string {
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func marshalVSCodeSettings(settings map[string]any, original []byte, existed bool, replaceKeys []string) ([]byte, error) {
	if !existed || len(bytes.TrimSpace(original)) == 0 {
		return json.MarshalIndent(settings, "", "  ")
	}
	next, ok, err := patchJSONCObject(original, settings, replaceKeys)
	if err != nil || !ok {
		return json.MarshalIndent(settings, "", "  ")
	}
	return next, nil
}

func patchJSONCObject(original []byte, settings map[string]any, replaceKeys []string) ([]byte, bool, error) {
	text := string(original)
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return nil, false, nil
	}
	replace := make(map[string]bool, len(replaceKeys))
	for _, key := range replaceKeys {
		replace[key] = true
	}
	entries := splitJSONCEntries(text[start+1 : end])
	kept := make([]string, 0, len(entries)+len(replaceKeys))
	for _, entry := range entries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		key, found := jsonCEntryKey(trimmed)
		if found && replace[key] {
			continue
		}
		kept = append(kept, trimmed)
	}
	for _, key := range replaceKeys {
		value, found := settings[key]
		if !found {
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, false, err
		}
		kept = append(kept, fmt.Sprintf("%q: %s", key, encoded))
	}
	var output strings.Builder
	output.WriteString(text[:start+1])
	if len(kept) > 0 {
		output.WriteByte('\n')
		for index, entry := range kept {
			output.WriteString(indentJSONCEntry(entry))
			if index < len(kept)-1 {
				output.WriteByte(',')
			}
			output.WriteByte('\n')
		}
	}
	output.WriteString(text[end:])
	return []byte(output.String()), true, nil
}

func splitJSONCEntries(input string) []string {
	entries := []string{}
	start := 0
	depth := 0
	inString := false
	escaped := false
	inLineComment := false
	inBlockComment := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && i+1 < len(input) && input[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inString {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '/':
			if i+1 < len(input) && input[i+1] == '/' {
				inLineComment = true
				i++
			} else if i+1 < len(input) && input[i+1] == '*' {
				inBlockComment = true
				i++
			}
		case '{', '[':
			depth++
		case '}', ']':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				entries = append(entries, input[start:i])
				start = i + 1
			}
		}
	}
	entries = append(entries, input[start:])
	return entries
}

func jsonCEntryKey(entry string) (string, bool) {
	for {
		entry = strings.TrimSpace(entry)
		if strings.HasPrefix(entry, "//") {
			if index := strings.IndexByte(entry, '\n'); index >= 0 {
				entry = entry[index+1:]
				continue
			}
			return "", false
		}
		if strings.HasPrefix(entry, "/*") {
			if index := strings.Index(entry, "*/"); index >= 0 {
				entry = entry[index+2:]
				continue
			}
			return "", false
		}
		break
	}
	if !strings.HasPrefix(entry, "\"") {
		return "", false
	}
	decoder := json.NewDecoder(strings.NewReader(entry))
	var key string
	if err := decoder.Decode(&key); err != nil {
		return "", false
	}
	return key, true
}

func indentJSONCEntry(entry string) string {
	lines := strings.Split(entry, "\n")
	for index, line := range lines {
		lines[index] = "  " + strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
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
			for i+1 < len(input) && (input[i] != '*' || input[i+1] != '/') {
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
	inString := false
	escaped := false
	for i := 0; i < len(input); i++ {
		if inString {
			output = append(output, input[i])
			if escaped {
				escaped = false
			} else if input[i] == '\\' {
				escaped = true
			} else if input[i] == '"' {
				inString = false
			}
			continue
		}
		if input[i] == '"' {
			inString = true
			output = append(output, input[i])
			continue
		}
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
