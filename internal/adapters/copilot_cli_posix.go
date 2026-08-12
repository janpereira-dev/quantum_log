package adapters

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	copilotCLIPosixBlockStart = "# >>> qlog Copilot CLI OTel >>>"
	copilotCLIPosixBlockEnd   = "# <<< qlog Copilot CLI OTel <<<"
)

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

func copilotCLIPosixBlock() string {
	return copilotCLIPosixBlockStart + "\n" +
		"copilot() {\n" +
		"  COPILOT_OTEL_ENABLED=true COPILOT_OTEL_EXPORTER_TYPE=otlp-http OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 OTEL_EXPORTER_OTLP_PROTOCOL=http/json OTEL_METRICS_EXPORTER=none OTEL_LOGS_EXPORTER=none OTEL_SERVICE_NAME=github-copilot OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=false command copilot \"$@\"\n" +
		"}\n" +
		copilotCLIPosixBlockEnd + "\n"
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
	path := a.posixProfilePath()
	contents, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return SetupChange{}, fmt.Errorf("read shell profile: %w", err)
	}
	next, err := withCopilotCLIPosixBlock(string(contents))
	if err != nil {
		return SetupChange{}, err
	}
	if !dryRun && !os.IsNotExist(err) && string(contents) != next {
		if err := os.WriteFile(path+".qlog-backup", contents, 0o600); err != nil {
			return SetupChange{}, fmt.Errorf("backup shell profile: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return SetupChange{}, err
	}
	change, err := applyManagedFile(path, next, dryRun)
	if err == nil {
		change.Description = "configured qlog-owned Copilot-only OTel shell function"
	}
	return change, err
}

func (a copilotCLIAdapter) uninstallPosixProfile(dryRun bool) (SetupChange, error) {
	path := a.posixProfilePath()
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return SetupChange{Path: path, Action: "unchanged", Description: "qlog Copilot OTel shell function already absent"}, nil
	}
	if err != nil {
		return SetupChange{}, err
	}
	next, removed, err := withoutCopilotCLIPosixBlock(string(contents))
	if err != nil {
		return SetupChange{}, err
	}
	if !removed {
		return SetupChange{Path: path, Action: "unchanged", Description: "qlog Copilot OTel shell function already absent"}, nil
	}
	change, err := applyManagedFile(path, next, dryRun)
	if err == nil {
		change.Action = "removed"
		change.Description = "removed qlog-owned Copilot OTel shell function"
	}
	return change, err
}

func (a copilotCLIAdapter) posixProfileInstalled() bool {
	contents, err := os.ReadFile(a.posixProfilePath())
	return err == nil && strings.Contains(string(contents), copilotCLIPosixBlockStart) && strings.Contains(string(contents), copilotCLIPosixBlockEnd)
}
