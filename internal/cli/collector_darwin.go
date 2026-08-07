//go:build darwin

package cli

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const darwinCollectorLabel = "dev.quantum-log.collector"

type darwinCollectorManager struct{}

type darwinCollectorState struct {
	Home   string `json:"home"`
	Listen string `json:"listen,omitempty"`
}

var runDarwinLaunchctl = func(args ...string) error {
	return exec.Command("launchctl", args...).Run()
}

var installDarwinCollector = func(home, listen string) (CollectorStatus, error) {
	return darwinCollectorManager{}.Install(home, listen)
}

var statusDarwinCollector = func(ctx context.Context, listen string) (CollectorStatus, error) {
	return darwinCollectorManager{}.Status(ctx, listen)
}

func newCollectorManager() collectorManager { return darwinCollectorManager{} }

func darwinCollectorPlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", darwinCollectorLabel+".plist")
}

func darwinCollectorStatePath() string { return darwinCollectorPlistPath() + ".state" }

func darwinCollectorDomain() string { return "gui/" + fmt.Sprint(os.Getuid()) }

func (darwinCollectorManager) ResolveManagedCollectorSettings(home, listen string, homeExplicit, listenExplicit bool) (string, string) {
	state := readDarwinCollectorState(darwinCollectorStatePath())
	if !homeExplicit && state.Home != "" {
		home = state.Home
	}
	if !listenExplicit && state.Listen != "" {
		listen = state.Listen
	}
	return home, listen
}

func (darwinCollectorManager) Install(home, listen string) (CollectorStatus, error) {
	if err := validateCollectorListen(listen); err != nil {
		return CollectorStatus{}, err
	}
	executable, err := durableExecutablePath("")
	if err != nil {
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
	if err := writeDarwinCollectorState(darwinCollectorStatePath(), darwinCollectorState{Home: home, Listen: listen}); err != nil {
		return CollectorStatus{}, err
	}
	return CollectorStatus{Installed: true, Listen: listen, ServiceID: darwinCollectorLabel, StatePath: filepath.Join(home, "collector"), LogPath: filepath.Join(home, "collector", "collector.log"), Message: "collector LaunchAgent installed"}, nil
}

func (manager darwinCollectorManager) Start(home, listen string) (CollectorStatus, error) {
	if _, err := installDarwinCollector(home, listen); err != nil {
		return CollectorStatus{}, err
	}
	service := darwinCollectorDomain() + "/" + darwinCollectorLabel
	if err := runDarwinLaunchctl("print", service); err == nil {
		if err := runDarwinLaunchctl("bootout", service); err != nil {
			return CollectorStatus{}, err
		}
	}
	if err := runDarwinLaunchctl("bootstrap", darwinCollectorDomain(), darwinCollectorPlistPath()); err != nil {
		return CollectorStatus{}, err
	}
	if err := runDarwinLaunchctl("kickstart", "-k", service); err != nil {
		return CollectorStatus{}, err
	}
	status, err := statusDarwinCollector(context.Background(), listen)
	if err != nil {
		return CollectorStatus{}, err
	}
	return collectorStartupStatus(context.Background(), status, func(ctx context.Context) (CollectorStatus, error) {
		return statusDarwinCollector(ctx, listen)
	})
}

func (darwinCollectorManager) Stop() (CollectorStatus, error) {
	service := darwinCollectorDomain() + "/" + darwinCollectorLabel
	if runDarwinLaunchctl("print", service) == nil {
		if err := runDarwinLaunchctl("bootout", service); err != nil {
			return CollectorStatus{}, err
		}
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
	home := readDarwinCollectorState(darwinCollectorStatePath()).Home
	status := CollectorStatus{Installed: fileExists(darwinCollectorPlistPath()), Listen: listen, ServiceID: darwinCollectorLabel, StatePath: filepath.Join(home, "collector"), LogPath: filepath.Join(home, "collector", "collector.log")}
	if status.Installed && exec.CommandContext(ctx, "launchctl", "print", darwinCollectorDomain()+"/"+darwinCollectorLabel).Run() == nil {
		status.Running = true
	}
	health := probeCollectorHealth(ctx, listen)
	return collectorStatusWithHealth(status, health), nil
}

func (darwinCollectorManager) Logs() (string, error) {
	home := readDarwinCollectorState(darwinCollectorStatePath()).Home
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
	home := readDarwinCollectorState(darwinCollectorStatePath()).Home
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

func writeDarwinCollectorState(path string, state darwinCollectorState) error {
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

func readDarwinCollectorState(path string) darwinCollectorState {
	contents, err := os.ReadFile(path)
	if err != nil {
		return darwinCollectorState{}
	}
	value := strings.TrimSpace(string(contents))
	state := darwinCollectorState{}
	if err := json.Unmarshal([]byte(value), &state); err != nil {
		state.Home = value
	}
	if !filepath.IsAbs(state.Home) || (state.Listen != "" && validateCollectorListen(state.Listen) != nil) {
		return darwinCollectorState{}
	}
	return state
}
