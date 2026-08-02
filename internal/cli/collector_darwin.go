//go:build darwin

package cli

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const darwinCollectorLabel = "dev.quantum-log.collector"

type darwinCollectorManager struct{}

func newCollectorManager() collectorManager { return darwinCollectorManager{} }

func darwinCollectorPlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", darwinCollectorLabel+".plist")
}

func darwinCollectorStatePath() string { return darwinCollectorPlistPath() + ".state" }

func darwinCollectorDomain() string { return "gui/" + fmt.Sprint(os.Getuid()) }

func (darwinCollectorManager) Install(home, listen string) (CollectorStatus, error) {
	if err := validateCollectorListen(listen); err != nil {
		return CollectorStatus{}, err
	}
	executable, err := os.Executable()
	if err != nil {
		return CollectorStatus{}, err
	}
	if err := validateCollectorExecutable(executable); err != nil {
		return CollectorStatus{}, err
	}
	if err := os.MkdirAll(filepath.Join(home, "collector"), 0o700); err != nil {
		return CollectorStatus{}, err
	}
	if err := os.MkdirAll(filepath.Dir(darwinCollectorPlistPath()), 0o700); err != nil {
		return CollectorStatus{}, err
	}
	if err := os.WriteFile(darwinCollectorPlistPath(), []byte(darwinCollectorLaunchAgentDefinition(executable, home, listen)), 0o600); err != nil {
		return CollectorStatus{}, err
	}
	if err := os.WriteFile(darwinCollectorStatePath(), []byte(home), 0o600); err != nil {
		return CollectorStatus{}, err
	}
	return CollectorStatus{Installed: true, Listen: listen, ServiceID: darwinCollectorLabel, StatePath: filepath.Join(home, "collector"), LogPath: filepath.Join(home, "collector", "collector.log"), Message: "collector LaunchAgent installed"}, nil
}

func (manager darwinCollectorManager) Start(home, listen string) (CollectorStatus, error) {
	if _, err := manager.Install(home, listen); err != nil {
		return CollectorStatus{}, err
	}
	service := darwinCollectorDomain() + "/" + darwinCollectorLabel
	if err := exec.Command("launchctl", "print", service).Run(); err != nil {
		if err := exec.Command("launchctl", "bootstrap", darwinCollectorDomain(), darwinCollectorPlistPath()).Run(); err != nil {
			return CollectorStatus{}, err
		}
	}
	if err := exec.Command("launchctl", "kickstart", "-k", service).Run(); err != nil {
		return CollectorStatus{}, err
	}
	status, err := manager.Status(context.Background(), listen)
	if err != nil {
		return CollectorStatus{}, err
	}
	status.Message = "collector LaunchAgent start requested; health=" + status.Message
	return status, nil
}

func (darwinCollectorManager) Stop() (CollectorStatus, error) {
	if err := exec.Command("launchctl", "bootout", darwinCollectorDomain()+"/"+darwinCollectorLabel).Run(); err != nil && fileExists(darwinCollectorPlistPath()) {
		return CollectorStatus{}, err
	}
	return CollectorStatus{Installed: fileExists(darwinCollectorPlistPath()), ServiceID: darwinCollectorLabel, StatePath: filepath.Dir(darwinCollectorPlistPath()), Message: "collector LaunchAgent stopped"}, nil
}

func (manager darwinCollectorManager) Restart(home, listen string) (CollectorStatus, error) {
	if _, err := manager.Stop(); err != nil {
		return CollectorStatus{}, err
	}
	return manager.Start(home, listen)
}

func (darwinCollectorManager) Status(ctx context.Context, listen string) (CollectorStatus, error) {
	if err := validateCollectorListen(listen); err != nil {
		return CollectorStatus{}, err
	}
	home := readCollectorHome(darwinCollectorStatePath())
	status := CollectorStatus{Installed: fileExists(darwinCollectorPlistPath()), Listen: listen, ServiceID: darwinCollectorLabel, StatePath: filepath.Join(home, "collector"), LogPath: filepath.Join(home, "collector", "collector.log")}
	if status.Installed && exec.CommandContext(ctx, "launchctl", "print", darwinCollectorDomain()+"/"+darwinCollectorLabel).Run() == nil {
		status.Running = true
	}
	health := probeCollectorHealth(ctx, listen)
	status.Reachable = health.Reachable
	if health.Reachable {
		status.Running = true
	}
	status.Message = health.Health
	return status, nil
}

func (darwinCollectorManager) Logs() (string, error) {
	home := readCollectorHome(darwinCollectorStatePath())
	contents, err := os.ReadFile(filepath.Join(home, "collector", "collector.log"))
	if os.IsNotExist(err) {
		return "collector log is empty\n", nil
	}
	return string(contents), err
}

func (manager darwinCollectorManager) Uninstall() (CollectorStatus, error) {
	if _, err := manager.Stop(); err != nil {
		return CollectorStatus{}, err
	}
	if err := os.Remove(darwinCollectorPlistPath()); err != nil && !os.IsNotExist(err) {
		return CollectorStatus{}, err
	}
	home := readCollectorHome(darwinCollectorStatePath())
	if home != "" {
		if err := os.RemoveAll(filepath.Join(home, "collector")); err != nil {
			return CollectorStatus{}, err
		}
	}
	if err := os.Remove(darwinCollectorStatePath()); err != nil && !os.IsNotExist(err) {
		return CollectorStatus{}, err
	}
	return CollectorStatus{ServiceID: darwinCollectorLabel, StatePath: filepath.Join(home, "collector"), LogPath: filepath.Join(home, "collector", "collector.log"), Message: "collector LaunchAgent uninstalled"}, nil
}

func darwinCollectorLaunchAgentDefinition(executable, home, listen string) string {
	logPath := collectorLogPathForHome(home)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>Label</key><string>dev.quantum-log.collector</string><key>ProgramArguments</key><array><string>%s</string><string>--home</string><string>%s</string><string>collector</string><string>serve</string><string>--listen</string><string>%s</string></array><key>RunAtLoad</key><true/><key>KeepAlive</key><true/><key>StandardOutPath</key><string>%s</string><key>StandardErrorPath</key><string>%s</string></dict></plist>`, xmlEscape(executable), xmlEscape(home), xmlEscape(listen), xmlEscape(logPath), xmlEscape(logPath))
}

func collectorLogPathForHome(home string) string {
	return strings.TrimRight(home, "/\\") + "/collector/collector.log"
}

func xmlEscape(value string) string {
	var escaped strings.Builder
	_ = xml.EscapeText(&escaped, []byte(value))
	return escaped.String()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readCollectorHome(path string) string {
	contents, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(contents))
}
