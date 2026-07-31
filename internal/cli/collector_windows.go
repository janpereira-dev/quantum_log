//go:build windows

package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const windowsCollectorTaskName = `QUANTUM_LOG Collector`

type windowsCollectorManager struct{}

func newCollectorManager() collectorManager { return windowsCollectorManager{} }

func collectorStateDir() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "QUANTUM_LOG", "collector")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Local", "QUANTUM_LOG", "collector")
}

func collectorLogPath() string { return filepath.Join(collectorStateDir(), "collector.log") }

func collectorTaskDefinitionPath() string {
	return filepath.Join(collectorStateDir(), "collector-task.xml")
}

func windowsCollectorStatus(ctx context.Context, listen string) (CollectorStatus, error) {
	status := CollectorStatus{Listen: listen, ServiceID: windowsCollectorTaskName, StatePath: collectorStateDir(), LogPath: collectorLogPath()}
	output, err := exec.CommandContext(ctx, "schtasks.exe", "/Query", "/TN", windowsCollectorTaskName, "/FO", "LIST", "/V").CombinedOutput()
	if err == nil {
		status.Installed = true
		status.Running = strings.Contains(string(output), "Running")
	}
	health := probeCollectorHealth(ctx, listen)
	status.Reachable = health.Reachable
	if health.Reachable {
		status.Running = true
	}
	status.Message = health.Health
	return status, nil
}

func (windowsCollectorManager) Install(home, listen string) (CollectorStatus, error) {
	if err := validateCollectorListen(listen); err != nil {
		return CollectorStatus{}, err
	}
	if err := os.MkdirAll(collectorStateDir(), 0o700); err != nil {
		return CollectorStatus{}, err
	}
	executable, err := os.Executable()
	if err != nil {
		return CollectorStatus{}, err
	}
	if err := os.WriteFile(collectorTaskDefinitionPath(), []byte(windowsCollectorTaskDefinition(executable, home, listen)), 0o600); err != nil {
		return CollectorStatus{}, err
	}
	if err := exec.Command("schtasks.exe", "/Create", "/TN", windowsCollectorTaskName, "/XML", collectorTaskDefinitionPath(), "/F").Run(); err != nil {
		return CollectorStatus{}, err
	}
	status, err := windowsCollectorStatus(context.Background(), listen)
	if err != nil {
		return CollectorStatus{}, err
	}
	status.Message = "collector task installed"
	return status, nil
}

func (windowsCollectorManager) Start(home, listen string) (CollectorStatus, error) {
	if err := validateCollectorListen(listen); err != nil {
		return CollectorStatus{}, err
	}
	if _, err := (windowsCollectorManager{}).Install(home, listen); err != nil {
		return CollectorStatus{}, err
	}
	if err := exec.Command("schtasks.exe", "/Run", "/TN", windowsCollectorTaskName).Run(); err != nil {
		return CollectorStatus{}, err
	}
	status, err := windowsCollectorStatus(context.Background(), listen)
	if err != nil {
		return CollectorStatus{}, err
	}
	status.Message = "collector task start requested; health=" + status.Message
	return status, nil
}

func (windowsCollectorManager) Stop() (CollectorStatus, error) {
	status, err := windowsCollectorStatus(context.Background(), defaultCollectorListen)
	if err != nil {
		return CollectorStatus{}, err
	}
	if !status.Installed {
		status.Message = "collector task is not installed"
		return status, nil
	}
	if err := exec.Command("schtasks.exe", "/End", "/TN", windowsCollectorTaskName).Run(); err != nil {
		return CollectorStatus{}, err
	}
	status, err = windowsCollectorStatus(context.Background(), defaultCollectorListen)
	if err != nil {
		return CollectorStatus{}, err
	}
	status.Message = "collector task stopped"
	return status, nil
}

func (manager windowsCollectorManager) Restart(home, listen string) (CollectorStatus, error) {
	if _, err := manager.Stop(); err != nil {
		return CollectorStatus{}, err
	}
	return manager.Start(home, listen)
}

func (windowsCollectorManager) Status(ctx context.Context, listen string) (CollectorStatus, error) {
	if err := validateCollectorListen(listen); err != nil {
		return CollectorStatus{}, err
	}
	return windowsCollectorStatus(ctx, listen)
}

func (windowsCollectorManager) Logs() (string, error) {
	contents, err := os.ReadFile(collectorLogPath())
	if os.IsNotExist(err) {
		return "collector log is empty\n", nil
	}
	return string(contents), err
}

func (manager windowsCollectorManager) Uninstall() (CollectorStatus, error) {
	status, err := manager.Stop()
	if err != nil {
		return CollectorStatus{}, err
	}
	if status.Installed {
		if err := exec.Command("schtasks.exe", "/Delete", "/TN", windowsCollectorTaskName, "/F").Run(); err != nil {
			return CollectorStatus{}, err
		}
	}
	if err := os.RemoveAll(collectorStateDir()); err != nil {
		return CollectorStatus{}, err
	}
	status.Installed = false
	status.Running = false
	status.Message = "collector task uninstalled"
	return status, nil
}
