//go:build linux

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const linuxCollectorUnitName = "quantum-log-collector.service"

type linuxCollectorManager struct{}

func newCollectorManager() collectorManager { return linuxCollectorManager{} }

func linuxCollectorUnitPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", linuxCollectorUnitName)
}

func linuxCollectorStatePath() string { return linuxCollectorUnitPath() + ".state" }

func (linuxCollectorManager) Install(home, listen string) (CollectorStatus, error) {
	if err := validateCollectorListen(listen); err != nil {
		return CollectorStatus{}, err
	}
	if err := os.MkdirAll(filepath.Join(home, "collector"), 0o700); err != nil {
		return CollectorStatus{}, err
	}
	if err := os.MkdirAll(filepath.Dir(linuxCollectorUnitPath()), 0o700); err != nil {
		return CollectorStatus{}, err
	}
	executable, err := os.Executable()
	if err != nil {
		return CollectorStatus{}, err
	}
	if err := os.WriteFile(linuxCollectorUnitPath(), []byte(linuxCollectorUnitDefinition(executable, home, listen)), 0o600); err != nil {
		return CollectorStatus{}, err
	}
	if err := os.WriteFile(linuxCollectorStatePath(), []byte(home), 0o600); err != nil {
		return CollectorStatus{}, err
	}
	return CollectorStatus{Installed: true, Listen: listen, ServiceID: linuxCollectorUnitName, StatePath: filepath.Join(home, "collector"), LogPath: filepath.Join(home, "collector", "collector.log"), Message: "collector user service installed"}, nil
}

func (manager linuxCollectorManager) Start(home, listen string) (CollectorStatus, error) {
	if _, err := manager.Install(home, listen); err != nil {
		return CollectorStatus{}, err
	}
	for _, args := range [][]string{{"--user", "daemon-reload"}, {"--user", "enable", linuxCollectorUnitName}, {"--user", "start", linuxCollectorUnitName}} {
		if err := exec.Command("systemctl", args...).Run(); err != nil {
			return CollectorStatus{}, err
		}
	}
	status, err := manager.Status(context.Background(), listen)
	if err != nil {
		return CollectorStatus{}, err
	}
	status.Message = "collector user service start requested; health=" + status.Message
	return status, nil
}

func (linuxCollectorManager) Stop() (CollectorStatus, error) {
	if err := exec.Command("systemctl", "--user", "stop", linuxCollectorUnitName).Run(); err != nil && fileExists(linuxCollectorUnitPath()) {
		return CollectorStatus{}, err
	}
	return CollectorStatus{Installed: fileExists(linuxCollectorUnitPath()), ServiceID: linuxCollectorUnitName, StatePath: filepath.Dir(linuxCollectorUnitPath()), Message: "collector user service stopped"}, nil
}

func (manager linuxCollectorManager) Restart(home, listen string) (CollectorStatus, error) {
	if _, err := manager.Stop(); err != nil {
		return CollectorStatus{}, err
	}
	return manager.Start(home, listen)
}

func (linuxCollectorManager) Status(ctx context.Context, listen string) (CollectorStatus, error) {
	if err := validateCollectorListen(listen); err != nil {
		return CollectorStatus{}, err
	}
	home := readCollectorHome(linuxCollectorStatePath())
	status := CollectorStatus{Installed: fileExists(linuxCollectorUnitPath()), Listen: listen, ServiceID: linuxCollectorUnitName, StatePath: filepath.Join(home, "collector"), LogPath: filepath.Join(home, "collector", "collector.log")}
	if status.Installed && exec.CommandContext(ctx, "systemctl", "--user", "is-active", "--quiet", linuxCollectorUnitName).Run() == nil {
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

func (linuxCollectorManager) Logs() (string, error) {
	home := readCollectorHome(linuxCollectorStatePath())
	contents, err := os.ReadFile(filepath.Join(home, "collector", "collector.log"))
	if os.IsNotExist(err) {
		return "collector log is empty\n", nil
	}
	return string(contents), err
}

func (manager linuxCollectorManager) Uninstall() (CollectorStatus, error) {
	if _, err := manager.Stop(); err != nil {
		return CollectorStatus{}, err
	}
	for _, args := range [][]string{{"--user", "disable", linuxCollectorUnitName}, {"--user", "daemon-reload"}} {
		if err := exec.Command("systemctl", args...).Run(); err != nil {
			return CollectorStatus{}, err
		}
	}
	if err := os.Remove(linuxCollectorUnitPath()); err != nil && !os.IsNotExist(err) {
		return CollectorStatus{}, err
	}
	home := readCollectorHome(linuxCollectorStatePath())
	if home != "" {
		if err := os.RemoveAll(filepath.Join(home, "collector")); err != nil {
			return CollectorStatus{}, err
		}
	}
	if err := os.Remove(linuxCollectorStatePath()); err != nil && !os.IsNotExist(err) {
		return CollectorStatus{}, err
	}
	return CollectorStatus{ServiceID: linuxCollectorUnitName, StatePath: filepath.Join(home, "collector"), LogPath: filepath.Join(home, "collector", "collector.log"), Message: "collector user service uninstalled"}, nil
}

func linuxCollectorUnitDefinition(executable, home, listen string) string {
	return fmt.Sprintf("[Unit]\nDescription=QUANTUM_LOG Collector\n\n[Service]\nExecStart=%s --home %s collector serve --listen %s\nRestart=on-failure\nStandardOutput=append:%s\nStandardError=append:%s\n\n[Install]\nWantedBy=default.target\n", systemdQuote(executable), systemdQuote(home), systemdQuote(listen), systemdQuote(collectorLogPathForHome(home)), systemdQuote(collectorLogPathForHome(home)))
}

func collectorLogPathForHome(home string) string {
	return strings.TrimRight(home, "/\\") + "/collector/collector.log"
}

func systemdQuote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
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
