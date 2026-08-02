//go:build linux

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const linuxCollectorUnitName = "quantum-log-collector.service"

type linuxCollectorManager struct{}

type linuxCollectorState struct {
	Home   string `json:"home"`
	Listen string `json:"listen,omitempty"`
}

var runLinuxSystemctl = func(args ...string) error {
	return exec.Command("systemctl", args...).Run()
}

var removeLinuxCollectorUnit = os.Remove
var removeLinuxCollectorTree = os.RemoveAll
var removeLinuxCollectorState = os.Remove
var stopLinuxCollector = func() (CollectorStatus, error) { return linuxCollectorManager{}.Stop() }
var readManagedLinuxCollectorState = readLinuxCollectorState

func newCollectorManager() collectorManager { return linuxCollectorManager{} }

func linuxCollectorUnitPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", linuxCollectorUnitName)
}

func linuxCollectorStatePath() string { return linuxCollectorUnitPath() + ".state" }

func (linuxCollectorManager) ResolveManagedCollectorSettings(home, listen string, homeExplicit, listenExplicit bool) (string, string) {
	state := readLinuxCollectorState(linuxCollectorStatePath())
	if !homeExplicit && state.Home != "" {
		home = state.Home
	}
	return home, linuxCollectorListen(state, listen, listenExplicit)
}

func (linuxCollectorManager) Install(home, listen string) (CollectorStatus, error) {
	if err := validateCollectorListen(listen); err != nil {
		return CollectorStatus{}, err
	}
	if !filepath.IsAbs(home) {
		return CollectorStatus{}, fmt.Errorf("collector home must be an absolute path")
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
	if err := os.MkdirAll(filepath.Dir(linuxCollectorUnitPath()), 0o700); err != nil {
		return CollectorStatus{}, err
	}
	if err := os.WriteFile(linuxCollectorUnitPath(), []byte(linuxCollectorUnitDefinition(executable, home, listen)), 0o600); err != nil {
		return CollectorStatus{}, err
	}
	if err := writeLinuxCollectorState(linuxCollectorStatePath(), linuxCollectorState{Home: home, Listen: listen}); err != nil {
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
	state := readLinuxCollectorState(linuxCollectorStatePath())
	return CollectorStatus{Installed: fileExists(linuxCollectorUnitPath()), ServiceID: linuxCollectorUnitName, StatePath: filepath.Join(state.Home, "collector"), LogPath: filepath.Join(state.Home, "collector", "collector.log"), Message: "collector user service stopped"}, nil
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
	home := readLinuxCollectorState(linuxCollectorStatePath()).Home
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
	home := readLinuxCollectorState(linuxCollectorStatePath()).Home
	contents, err := os.ReadFile(filepath.Join(home, "collector", "collector.log"))
	if os.IsNotExist(err) {
		return "collector log is empty\n", nil
	}
	return string(contents), err
}

func (manager linuxCollectorManager) Uninstall() (CollectorStatus, error) {
	if _, err := stopLinuxCollector(); err != nil {
		return CollectorStatus{}, err
	}
	if err := runLinuxSystemctl("--user", "disable", linuxCollectorUnitName); err != nil {
		return CollectorStatus{}, err
	}
	if err := removeLinuxCollectorUnit(linuxCollectorUnitPath()); err != nil && !os.IsNotExist(err) {
		return CollectorStatus{}, err
	}
	if err := runLinuxSystemctl("--user", "daemon-reload"); err != nil {
		return CollectorStatus{}, err
	}
	home := readManagedLinuxCollectorState(linuxCollectorStatePath()).Home
	if home != "" {
		if err := removeLinuxCollectorTree(filepath.Join(home, "collector")); err != nil {
			return CollectorStatus{}, err
		}
	}
	if err := removeLinuxCollectorState(linuxCollectorStatePath()); err != nil && !os.IsNotExist(err) {
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

func linuxCollectorListen(state linuxCollectorState, listen string, explicit bool) string {
	if !explicit && state.Listen != "" {
		return state.Listen
	}
	return listen
}

func writeLinuxCollectorState(path string, state linuxCollectorState) error {
	if !filepath.IsAbs(state.Home) {
		return fmt.Errorf("collector home must be an absolute path")
	}
	if state.Listen != "" {
		if err := validateCollectorListen(state.Listen); err != nil {
			return err
		}
	}
	contents, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode collector state: %w", err)
	}
	return os.WriteFile(path, contents, 0o600)
}

func readLinuxCollectorState(path string) linuxCollectorState {
	contents, err := os.ReadFile(path)
	if err != nil {
		return linuxCollectorState{}
	}
	value := strings.TrimSpace(string(contents))
	state := linuxCollectorState{}
	if err := json.Unmarshal([]byte(value), &state); err != nil {
		state.Home = value
	}
	if !filepath.IsAbs(state.Home) {
		return linuxCollectorState{}
	}
	if state.Listen != "" && validateCollectorListen(state.Listen) != nil {
		return linuxCollectorState{}
	}
	return state
}
