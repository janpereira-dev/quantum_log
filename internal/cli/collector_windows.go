//go:build windows

package cli

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

const windowsCollectorTaskName = `QUANTUM_LOG Collector`

type windowsCollectorManager struct{}

var runWindowsSchedulerCommand = func(args ...string) ([]byte, error) {
	return exec.Command("schtasks.exe", args...).CombinedOutput()
}

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
	if err := validateCollectorExecutable(executable); err != nil {
		return CollectorStatus{}, err
	}
	userID, err := currentWindowsTokenIdentity()
	if err != nil {
		return CollectorStatus{}, err
	}
	if err := writeWindowsCollectorTaskDefinition(collectorTaskDefinitionPath(), executable, home, listen, userID, collectorLogPath()); err != nil {
		return CollectorStatus{}, err
	}
	if err := createWindowsCollectorTask(collectorTaskDefinitionPath()); err != nil {
		return CollectorStatus{}, err
	}
	status, err := windowsCollectorStatus(context.Background(), listen)
	if err != nil {
		return CollectorStatus{}, err
	}
	status.Message = "collector task installed"
	return status, nil
}

func currentWindowsTokenIdentity() (string, error) {
	token := windows.GetCurrentProcessToken()
	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("get current process token user: %w", err)
	}
	account, domain, _, err := tokenUser.User.Sid.LookupAccount("")
	if err != nil {
		return "", fmt.Errorf("resolve current process token user: %w", err)
	}
	if account == "" || domain == "" {
		return "", fmt.Errorf("resolve current process token user: empty account or domain")
	}
	return domain + `\` + account, nil
}

func writeWindowsCollectorTaskDefinition(path, executable, home, listen, userID, logPath string) error {
	definition := strings.Replace(windowsCollectorTaskDefinition(executable, home, listen, userID, logPath), `encoding="UTF-8"`, `encoding="UTF-16"`, 1)
	encoded := utf16.Encode([]rune(definition))
	contents := make([]byte, 2, 2+len(encoded)*2)
	contents[0], contents[1] = 0xFF, 0xFE
	for _, codeUnit := range encoded {
		contents = append(contents, byte(codeUnit), byte(codeUnit>>8))
	}
	return os.WriteFile(path, contents, 0o600)
}

func createWindowsCollectorTask(definitionPath string) error {
	output, err := runWindowsSchedulerCommand("/Create", "/TN", windowsCollectorTaskName, "/XML", definitionPath, "/F")
	if err == nil {
		return nil
	}
	diagnostic := strings.TrimSpace(string(output))
	if diagnostic == "" {
		return fmt.Errorf("task scheduler operation /Create for task %q failed: %w", windowsCollectorTaskName, err)
	}
	return fmt.Errorf("task scheduler operation /Create for task %q failed: %w: %s", windowsCollectorTaskName, err, diagnostic)
}

func windowsCollectorTaskDefinition(executable, home, listen, userID, logPath string) string {
	arguments := "--home " + windowsCommandLineQuote(home) + " collector serve --listen " + listen + " --log-file " + windowsCommandLineQuote(logPath)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task"><Triggers><LogonTrigger><Enabled>true</Enabled></LogonTrigger></Triggers><Principals><Principal id="Author"><UserId>%s</UserId><LogonType>InteractiveToken</LogonType><RunLevel>LeastPrivilege</RunLevel></Principal></Principals><Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><StartWhenAvailable>true</StartWhenAvailable><RestartOnFailure><Interval>PT1M</Interval><Count>3</Count></RestartOnFailure></Settings><Actions Context="Author"><Exec><Command>%s</Command><Arguments>%s</Arguments></Exec></Actions></Task>`, xmlEscape(userID), xmlEscape(executable), xmlEscape(arguments))
}

func xmlEscape(value string) string {
	var escaped strings.Builder
	_ = xml.EscapeText(&escaped, []byte(value))
	return escaped.String()
}

func windowsCommandLineQuote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
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
