//go:build windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type windowsCollectorManager struct{}

func newCollectorManager() collectorManager { return windowsCollectorManager{} }

func collectorStateDir() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "QUANTUM_LOG", "collector")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Local", "QUANTUM_LOG", "collector")
}

func collectorPIDPath() string { return filepath.Join(collectorStateDir(), "collector.pid") }

func collectorLogPath() string { return filepath.Join(collectorStateDir(), "collector.log") }

func (windowsCollectorManager) Install(_, _ string) (string, error) {
	if err := os.MkdirAll(collectorStateDir(), 0o700); err != nil {
		return "", err
	}
	return fmt.Sprintf("collector installed for user session at %s", collectorStateDir()), nil
}

func (windowsCollectorManager) Start(home, listen string) (string, error) {
	if err := os.MkdirAll(collectorStateDir(), 0o700); err != nil {
		return "", err
	}
	if pidBytes, err := os.ReadFile(collectorPIDPath()); err == nil && strings.TrimSpace(string(pidBytes)) != "" {
		return "", fmt.Errorf("collector pid file already exists at %s; run qlog collector stop before starting another collector", collectorPIDPath())
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	logFile, err := os.OpenFile(collectorLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	defer func() { _ = logFile.Close() }()
	args := []string{"collector", "serve", "--listen", listen}
	if home != "" {
		args = append([]string{"--home", home}, args...)
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return "", err
	}
	if err := os.WriteFile(collectorPIDPath(), []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o600); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Process.Release()
		return "", err
	}
	if err := cmd.Process.Release(); err != nil {
		_ = cmd.Process.Kill()
		_ = os.Remove(collectorPIDPath())
		return "", err
	}
	return fmt.Sprintf("collector started with pid %d", cmd.Process.Pid), nil
}

func (windowsCollectorManager) Stop() (string, error) {
	pidBytes, err := os.ReadFile(collectorPIDPath())
	if err != nil {
		return "collector is not running", nil
	}
	pid := strings.TrimSpace(string(pidBytes))
	if pid == "" {
		_ = os.Remove(collectorPIDPath())
		return "collector is not running", nil
	}
	_ = exec.Command("taskkill", "/PID", pid, "/T", "/F").Run()
	_ = os.Remove(collectorPIDPath())
	return "collector stopped", nil
}

func (manager windowsCollectorManager) Restart(home, listen string) (string, error) {
	_, _ = manager.Stop()
	return manager.Start(home, listen)
}

func (windowsCollectorManager) Logs() (string, error) {
	contents, err := os.ReadFile(collectorLogPath())
	if err != nil {
		return "collector log is empty", nil
	}
	return string(contents), nil
}

func (manager windowsCollectorManager) Uninstall() (string, error) {
	_, _ = manager.Stop()
	if err := os.RemoveAll(collectorStateDir()); err != nil {
		return "", err
	}
	return "collector uninstalled", nil
}
