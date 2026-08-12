package adapters

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	copilotCLIProfileBlockStart = "# >>> qlog Copilot CLI OTel >>>"
	copilotCLIProfileBlockEnd   = "# <<< qlog Copilot CLI OTel <<<"
)

var copilotCLIPowerShellProfileCommand = func(name string, args ...string) (string, error) {
	output, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run %s: %w", name, err)
	}
	return string(output), nil
}

var copilotCLIPowerShellProfileDiscovery = discoverCopilotCLIPowerShellProfile

var copilotCLIPowerShellProfileWriteFile = os.WriteFile

var copilotCLIPowerShellProfileWriteCommand = func(name string, args ...string) (string, error) {
	output, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run %s: %w", name, err)
	}
	return string(output), nil
}

const copilotCLIPowerShellProfileWriteScript = `& {
param([string]$target, [string]$payload, [string]$expectedHash)
$ErrorActionPreference = 'Stop'
$contents = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($payload))
if (Test-Path -LiteralPath $target -PathType Leaf) {
    if ($expectedHash -eq '') { throw 'PowerShell profile appeared during qlog setup' }
    $currentHash = ([BitConverter]::ToString([Security.Cryptography.SHA256]::Create().ComputeHash([IO.File]::ReadAllBytes($target)))).Replace('-', '').ToLowerInvariant()
    if ($currentHash -ne $expectedHash) { throw 'PowerShell profile changed during qlog setup' }
} else {
    if ($expectedHash -ne '') { throw 'PowerShell profile disappeared during qlog setup' }
    [void](New-Item -ItemType File -Path $target -ErrorAction Stop)
}
Set-Content -LiteralPath $target -Value $contents -NoNewline -Encoding utf8 -ErrorAction Stop
}`

// SetCopilotCLIPowerShellProfileDiscoveryForTesting isolates callers in other
// internal test packages without changing production command behavior.
func SetCopilotCLIPowerShellProfileDiscoveryForTesting(discover func() (string, error)) func() {
	original := copilotCLIPowerShellProfileDiscovery
	copilotCLIPowerShellProfileDiscovery = discover
	return func() { copilotCLIPowerShellProfileDiscovery = original }
}

func discoverCopilotCLIPowerShellProfile() (string, error) {
	output, err := copilotCLIPowerShellProfileCommand("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "$PROFILE.CurrentUserCurrentHost")
	if err != nil {
		return "", err
	}
	profile := strings.TrimSpace(output)
	if profile == "" || !isAbsoluteWindowsPath(profile) {
		return "", fmt.Errorf("PowerShell returned invalid current-user profile path %q", profile)
	}
	return profile, nil
}

func isAbsoluteWindowsPath(path string) bool {
	if filepath.IsAbs(path) {
		return true
	}
	return len(path) >= 3 && ((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z')) && path[1] == ':' && (path[2] == '\\' || path[2] == '/')
}

func copilotCLIPowerShellProfilePath() string {
	profile, err := copilotCLIPowerShellProfileDiscovery()
	if err == nil {
		return profile
	}
	// Discovery can be unavailable before Windows PowerShell is installed. This is
	// only a fallback; a discovered profile always wins, including OneDrive paths.
	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return filepath.Join("Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")
	}
	return filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")
}

func copilotCLIProfileBlock() string {
	return copilotCLIProfileBlockStart + "\n" +
		"function global:copilot {\n" +
		"  $qlogCopilot = (Get-Command copilot -CommandType Application -ErrorAction Stop).Source\n" +
		"  $qlogPrevious = @{}\n" +
		"  foreach ($qlogPair in @(@('COPILOT_OTEL_ENABLED','true'), @('COPILOT_OTEL_EXPORTER_TYPE','otlp-http'), @('OTEL_EXPORTER_OTLP_ENDPOINT','http://127.0.0.1:4318'), @('OTEL_EXPORTER_OTLP_PROTOCOL','http/json'), @('OTEL_METRICS_EXPORTER','none'), @('OTEL_LOGS_EXPORTER','none'), @('OTEL_SERVICE_NAME','github-copilot'), @('OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT','false'))) { $qlogPrevious[$qlogPair[0]] = [Environment]::GetEnvironmentVariable($qlogPair[0], 'Process'); Set-Item -Path ('Env:' + $qlogPair[0]) -Value $qlogPair[1] }\n" +
		"  try { & $qlogCopilot @args } finally { foreach ($qlogKey in $qlogPrevious.Keys) { if ($null -eq $qlogPrevious[$qlogKey]) { Remove-Item -Path ('Env:' + $qlogKey) -ErrorAction SilentlyContinue } else { Set-Item -Path ('Env:' + $qlogKey) -Value $qlogPrevious[$qlogKey] } } }\n" +
		"}\n" +
		copilotCLIProfileBlockEnd + "\n"
}

func writeCopilotCLIPowerShellProfile(path string, contents []byte, perm os.FileMode, previous []byte, exists bool) error {
	err := copilotCLIPowerShellProfileWriteFile(path, contents, perm)
	if err == nil || !os.IsNotExist(err) {
		return err
	}
	expectedHash := ""
	if exists {
		hash := sha256.Sum256(previous)
		expectedHash = hex.EncodeToString(hash[:])
	}
	_, err = copilotCLIPowerShellProfileWriteCommand(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		copilotCLIPowerShellProfileWriteScript,
		path,
		base64.StdEncoding.EncodeToString(contents),
		expectedHash,
	)
	if err != nil {
		return fmt.Errorf("write PowerShell profile through PowerShell: %w", err)
	}
	return nil
}

func withCopilotCLIProfileBlock(contents string) (string, error) {
	start := strings.Index(contents, copilotCLIProfileBlockStart)
	end := strings.Index(contents, copilotCLIProfileBlockEnd)
	if start == -1 && end != -1 || start != -1 && end == -1 || end < start {
		return "", fmt.Errorf("PowerShell profile contains incomplete qlog Copilot OTel block")
	}
	if start != -1 {
		end += len(copilotCLIProfileBlockEnd)
		if end < len(contents) && contents[end] == '\r' {
			end++
		}
		if end < len(contents) && contents[end] == '\n' {
			end++
		}
		return contents[:start] + copilotCLIProfileBlock() + contents[end:], nil
	}
	if contents != "" && !strings.HasSuffix(contents, "\n") {
		contents += "\n"
	}
	return contents + copilotCLIProfileBlock(), nil
}

func withoutCopilotCLIProfileBlock(contents string) (string, bool, error) {
	start := strings.Index(contents, copilotCLIProfileBlockStart)
	end := strings.Index(contents, copilotCLIProfileBlockEnd)
	if start == -1 && end == -1 {
		return contents, false, nil
	}
	if start == -1 || end == -1 || end < start {
		return "", false, fmt.Errorf("PowerShell profile contains incomplete qlog Copilot OTel block")
	}
	end += len(copilotCLIProfileBlockEnd)
	if end < len(contents) && contents[end] == '\r' {
		end++
	}
	if end < len(contents) && contents[end] == '\n' {
		end++
	}
	return contents[:start] + contents[end:], true, nil
}

func (a copilotCLIAdapter) installWindowsPowerShellProfile(dryRun bool) (SetupChange, error) {
	profile := copilotCLIPowerShellProfilePath()
	contents, err := os.ReadFile(profile)
	if err != nil && !os.IsNotExist(err) {
		return SetupChange{}, fmt.Errorf("read PowerShell profile: %w", err)
	}
	updated, err := withCopilotCLIProfileBlock(string(contents))
	if err != nil {
		return SetupChange{}, err
	}
	change, err := applyManagedFileWithWrite(profile, updated, dryRun, writeCopilotCLIPowerShellProfile)
	if err != nil {
		return SetupChange{}, fmt.Errorf("update PowerShell profile: %w", err)
	}
	if !dryRun {
		if _, err := applyManagedFile(a.windowsPowerShellProfileStatePath(), profile+"\n", false); err != nil {
			return SetupChange{}, err
		}
	}
	change.Description = "configured qlog-owned Copilot OTel block in discovered PowerShell profile"
	return change, nil
}

func (a copilotCLIAdapter) uninstallWindowsPowerShellProfile(dryRun bool) (SetupChange, error) {
	state, err := os.ReadFile(a.windowsPowerShellProfileStatePath())
	if os.IsNotExist(err) {
		return SetupChange{Path: "PowerShell CurrentUserCurrentHost profile", Action: "unchanged", Description: "qlog-owned Copilot OTel profile block already absent"}, nil
	}
	if err != nil {
		return SetupChange{}, fmt.Errorf("read qlog PowerShell profile state: %w", err)
	}
	profile := strings.TrimSpace(string(state))
	if profile == "" || !filepath.IsAbs(profile) {
		return SetupChange{}, fmt.Errorf("invalid qlog PowerShell profile state")
	}
	contents, err := os.ReadFile(profile)
	if os.IsNotExist(err) {
		if !dryRun {
			_, err = removeManagedFile(a.windowsPowerShellProfileStatePath(), "Copilot CLI qlog PowerShell profile state", false)
		}
		return SetupChange{Path: profile, Action: "unchanged", Description: "qlog-owned Copilot OTel profile block already absent"}, err
	}
	if err != nil {
		return SetupChange{}, fmt.Errorf("read PowerShell profile: %w", err)
	}
	updated, removed, err := withoutCopilotCLIProfileBlock(string(contents))
	if err != nil {
		return SetupChange{}, err
	}
	change := SetupChange{Path: profile, Action: "unchanged", Description: "qlog-owned Copilot OTel profile block already absent"}
	if removed {
		if updated == "" {
			change.Action = "removed"
			change.Description = "removed qlog-owned Copilot OTel PowerShell profile"
			if !dryRun {
				if err := os.Remove(profile); err != nil {
					return SetupChange{}, fmt.Errorf("remove PowerShell profile: %w", err)
				}
			}
		} else {
			change = SetupChange{Path: profile, Action: "updated", Description: "removed qlog-owned Copilot OTel block from PowerShell profile"}
			if !dryRun {
				if err := writeCopilotCLIPowerShellProfile(profile, []byte(updated), 0o600, contents, true); err != nil {
					return SetupChange{}, fmt.Errorf("update PowerShell profile: %w", err)
				}
			}
		}
	}
	if !dryRun {
		if _, err := removeManagedFile(a.windowsPowerShellProfileStatePath(), "Copilot CLI qlog PowerShell profile state", false); err != nil {
			return SetupChange{}, err
		}
	}
	return change, nil
}

func (a copilotCLIAdapter) windowsPowerShellProfileInstalled() bool {
	contents, err := os.ReadFile(copilotCLIPowerShellProfilePath())
	if err != nil {
		return false
	}
	return strings.Contains(string(contents), copilotCLIProfileBlockStart) && strings.Contains(string(contents), copilotCLIProfileBlockEnd)
}
