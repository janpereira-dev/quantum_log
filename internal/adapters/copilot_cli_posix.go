package adapters

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	copilotCLIPosixBlockStart = "# >>> qlog Copilot CLI OTel >>>"
	copilotCLIPosixBlockEnd   = "# <<< qlog Copilot CLI OTel <<<"
)

var copilotCLIPosixWrapper = regexp.MustCompile(`(?m)^\s*(?:function\s+)?copilot\s*(?:\(\s*\)|\{)|^\s*alias\s+copilot\s*=`)

func (a copilotCLIAdapter) posixProfilePath() string {
	if root := os.Getenv("QLOG_ADAPTER_CONFIG_HOME"); root != "" {
		return filepath.Join(root, ".profile")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".profile"
	}
	switch filepath.Base(os.Getenv("SHELL")) {
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "bash":
		return filepath.Join(home, ".bashrc")
	default:
		return filepath.Join(home, ".profile")
	}
}

func unsupportedPosixShell() bool { return filepath.Base(os.Getenv("SHELL")) == "fish" }

func (a copilotCLIAdapter) posixProfileStatePath() string {
	return filepath.Join(filepath.Dir(a.hooksPath()), "qlog-copilot-otel-posix-profile")
}

func (a copilotCLIAdapter) posixManagedProfiles() []string {
	if state, err := os.ReadFile(a.posixProfileStatePath()); err == nil {
		paths := splitManagedPaths(string(state))
		if len(paths) > 0 {
			return paths
		}
	}
	if root := os.Getenv("QLOG_ADAPTER_CONFIG_HOME"); root != "" {
		return []string{filepath.Join(root, ".profile")}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return []string{a.posixProfilePath()}
	}
	return []string{filepath.Join(home, ".zshrc"), filepath.Join(home, ".bashrc"), filepath.Join(home, ".profile")}
}

func splitManagedPaths(state string) []string {
	seen := map[string]bool{}
	paths := []string{}
	for _, value := range strings.Split(state, "\n") {
		if path := strings.TrimSpace(value); path != "" && filepath.IsAbs(path) && !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	return paths
}

func copilotCLIPosixBlock() string {
	return copilotCLIPosixBlockStart + "\n" +
		"copilot() {\n" +
		"  COPILOT_OTEL_ENABLED=true COPILOT_OTEL_EXPORTER_TYPE=otlp-http OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 OTEL_EXPORTER_OTLP_PROTOCOL=http/json OTEL_METRICS_EXPORTER=none OTEL_LOGS_EXPORTER=none OTEL_SERVICE_NAME=github-copilot OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=false command copilot \"$@\"\n" +
		"}\n" +
		copilotCLIPosixBlockEnd + "\n"
}

func validCopilotCLIPosixBlock(contents string) bool {
	return strings.Contains(contents, copilotCLIPosixBlockStart) &&
		strings.Contains(contents, copilotCLIPosixBlockEnd) &&
		strings.Contains(contents, "copilot() {") &&
		strings.Contains(contents, "COPILOT_OTEL_ENABLED=true") &&
		strings.Contains(contents, "OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318") &&
		strings.Contains(contents, "command copilot \"$@\"")
}

func hasUnmanagedCopilotWrapper(contents string) (bool, error) {
	without, _, err := withoutCopilotCLIPosixBlock(contents)
	if err != nil {
		return false, err
	}
	return copilotCLIPosixWrapper.MatchString(without), nil
}

func withCopilotCLIPosixBlock(contents string) (string, error) {
	start, end := strings.Index(contents, copilotCLIPosixBlockStart), strings.Index(contents, copilotCLIPosixBlockEnd)
	if (start == -1) != (end == -1) || end < start {
		return "", fmt.Errorf("shell profile contains incomplete qlog Copilot OTel block")
	}
	if start != -1 {
		end += len(copilotCLIPosixBlockEnd)
		if end < len(contents) && contents[end] == '\n' {
			end++
		}
		return contents[:start] + copilotCLIPosixBlock() + contents[end:], nil
	}
	if contents != "" && !strings.HasSuffix(contents, "\n") {
		contents += "\n"
	}
	return contents + copilotCLIPosixBlock(), nil
}

func withoutCopilotCLIPosixBlock(contents string) (string, bool, error) {
	start, end := strings.Index(contents, copilotCLIPosixBlockStart), strings.Index(contents, copilotCLIPosixBlockEnd)
	if start == -1 && end == -1 {
		return contents, false, nil
	}
	if start == -1 || end == -1 || end < start {
		return "", false, fmt.Errorf("shell profile contains incomplete qlog Copilot OTel block")
	}
	end += len(copilotCLIPosixBlockEnd)
	if end < len(contents) && contents[end] == '\n' {
		end++
	}
	return contents[:start] + contents[end:], true, nil
}

func (a copilotCLIAdapter) installPosixProfile(dryRun bool) (SetupChange, error) {
	if unsupportedPosixShell() {
		return SetupChange{}, fmt.Errorf("unsupported shell %q: Copilot OTel launcher requires bash, zsh, or a POSIX shell", os.Getenv("SHELL"))
	}
	path := a.posixProfilePath()
	contents, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return SetupChange{}, fmt.Errorf("read shell profile: %w", readErr)
	}
	missing := os.IsNotExist(readErr)
	if conflict, err := hasUnmanagedCopilotWrapper(string(contents)); err != nil {
		return SetupChange{}, err
	} else if conflict {
		return SetupChange{}, fmt.Errorf("existing copilot shell wrapper in %s: qlog will not replace it", path)
	}
	next, err := withCopilotCLIPosixBlock(string(contents))
	if err != nil {
		return SetupChange{}, err
	}
	if !dryRun && !missing && string(contents) != next {
		if err := os.WriteFile(path+".qlog-backup", contents, 0o600); err != nil {
			return SetupChange{}, fmt.Errorf("backup shell profile: %w", err)
		}
	}
	if !dryRun {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return SetupChange{}, err
		}
	}
	change, err := applyManagedFile(path, next, dryRun)
	if err != nil {
		return change, err
	}
	if !dryRun {
		if _, err := applyManagedFile(a.posixProfileStatePath(), path+"\n", false); err != nil {
			return SetupChange{}, err
		}
	}
	change.Description = "configured qlog-owned Copilot-only OTel shell function"
	return change, nil
}

func (a copilotCLIAdapter) uninstallPosixProfile(dryRun bool) (SetupChange, error) {
	profiles := a.posixManagedProfiles()
	change := SetupChange{Path: strings.Join(profiles, ", "), Action: "unchanged", Description: "qlog Copilot OTel shell function already absent"}
	for _, path := range profiles {
		contents, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return SetupChange{}, fmt.Errorf("read shell profile: %w", err)
		}
		next, removed, err := withoutCopilotCLIPosixBlock(string(contents))
		if err != nil {
			return SetupChange{}, err
		}
		if !removed {
			continue
		}
		if _, err := applyManagedFile(path, next, dryRun); err != nil {
			return SetupChange{}, err
		}
		change.Action, change.Description = "removed", "removed qlog-owned Copilot OTel shell functions"
	}
	if !dryRun {
		if _, err := removeManagedFile(a.posixProfileStatePath(), "Copilot CLI qlog POSIX profile state", false); err != nil {
			return SetupChange{}, err
		}
	}
	return change, nil
}

func (a copilotCLIAdapter) posixProfileInstalled() bool {
	if unsupportedPosixShell() {
		return false
	}
	for _, path := range a.posixManagedProfiles() {
		contents, err := os.ReadFile(path)
		if err == nil && validCopilotCLIPosixBlock(string(contents)) {
			return true
		}
	}
	return false
}
