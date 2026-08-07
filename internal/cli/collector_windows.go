//go:build windows

package cli

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const windowsCollectorTaskName = `QUANTUM_LOG Collector`

const (
	windowsCollectorSchedulerMode = "scheduler"
	windowsCollectorFallbackMode  = "user_fallback"
	windowsCollectorNoMode        = "none"
	windowsCollectorRunKey        = `Software\Microsoft\Windows\CurrentVersion\Run`
	windowsCollectorRunValue      = "QUANTUM_LOG Collector"
	windowsDetachedProcess        = 0x00000008
)

type windowsCollectorManager struct{}

var runWindowsSchedulerCommand = func(args ...string) ([]byte, error) {
	return exec.Command("schtasks.exe", args...).CombinedOutput()
}

var windowsCollectorStatusFn = windowsCollectorStatus
var stopWindowsCollectorFallbackFn = stopWindowsCollectorFallback
var unregisterWindowsCollectorFallbackFn = unregisterWindowsCollectorFallback

var startWindowsFallbackCollector = func(executable, home, listen, logPath string) (int, int64, error) {
	command := exec.Command(executable, "--home", home, "collector", "serve", "--listen", listen, "--log-file", logPath, "--fallback-state", collectorFallbackStatePath())
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | windowsDetachedProcess}
	if err := command.Start(); err != nil {
		return 0, 0, err
	}
	pid := command.Process.Pid
	startedAt, err := windowsProcessStartedAt(pid)
	_ = command.Process.Release()
	if err != nil {
		return 0, 0, err
	}
	return pid, startedAt, nil
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

func collectorFallbackStatePath() string {
	return filepath.Join(collectorStateDir(), "user-fallback.json")
}

type windowsCollectorFallbackState struct {
	Mode       string `json:"mode"`
	Executable string `json:"executable"`
	Home       string `json:"home"`
	Listen     string `json:"listen"`
	LogPath    string `json:"log_path"`
	Command    string `json:"command"`
	PID        int    `json:"pid"`
	StartedAt  int64  `json:"started_at"`
}

func windowsCollectorStatus(ctx context.Context, listen string) (CollectorStatus, error) {
	status := CollectorStatus{Listen: listen, Mode: windowsCollectorNoMode, ServiceID: windowsCollectorTaskName, StatePath: collectorStateDir(), LogPath: collectorLogPath()}
	output, err := exec.CommandContext(ctx, "schtasks.exe", "/Query", "/TN", windowsCollectorTaskName, "/FO", "LIST", "/V").CombinedOutput()
	if err == nil {
		status.Installed = true
		status.Running = strings.Contains(string(output), "Running")
		status.Mode = windowsCollectorSchedulerMode
	} else if fallback, fallbackErr := readWindowsCollectorFallbackState(); fallbackErr == nil && windowsCollectorFallbackRegistered(fallback) {
		status.Installed = true
		status.Mode = windowsCollectorFallbackMode
		status.ServiceID = windowsCollectorRunValue
		status.Listen = fallback.Listen
		status.LogPath = fallback.LogPath
		status.Running = windowsCollectorFallbackRunning(fallback)
	}
	health := probeCollectorHealth(ctx, status.Listen)
	return collectorStatusWithHealth(status, health), nil
}

func (windowsCollectorManager) InstallFallback(home, listen string) (CollectorStatus, error) {
	if err := validateCollectorListen(listen); err != nil {
		return CollectorStatus{}, err
	}
	if err := os.MkdirAll(collectorStateDir(), 0o700); err != nil {
		return CollectorStatus{}, err
	}
	executable, err := durableExecutablePath("")
	if err != nil {
		return CollectorStatus{}, err
	}
	state := windowsCollectorFallbackState{Mode: windowsCollectorFallbackMode, Executable: executable, Home: home, Listen: listen, LogPath: collectorLogPath()}
	state.Command = windowsCollectorRunCommand(state)
	if err := registerWindowsCollectorFallback(state.Command); err != nil {
		return CollectorStatus{}, err
	}
	if err := writeWindowsCollectorFallbackState(state); err != nil {
		_ = unregisterWindowsCollectorFallback()
		return CollectorStatus{}, err
	}
	pid, startedAt, err := startWindowsFallbackCollector(state.Executable, state.Home, state.Listen, state.LogPath)
	if err != nil {
		_ = os.Remove(collectorFallbackStatePath())
		_ = unregisterWindowsCollectorFallback()
		return CollectorStatus{}, fmt.Errorf("start Windows user fallback collector: %w", err)
	}
	state.PID, state.StartedAt = pid, startedAt
	if err := writeWindowsCollectorFallbackState(state); err != nil {
		_ = stopWindowsCollectorFallback(state)
		_ = unregisterWindowsCollectorFallback()
		return CollectorStatus{}, err
	}
	return CollectorStatus{Installed: true, Running: true, Listen: listen, Mode: windowsCollectorFallbackMode, ServiceID: windowsCollectorRunValue, StatePath: collectorStateDir(), LogPath: state.LogPath, Message: "user fallback installed and started"}, nil
}

func windowsCollectorRunCommand(state windowsCollectorFallbackState) string {
	return windowsCommandLineQuote(state.Executable) + " --home " + windowsCommandLineQuote(state.Home) + " collector serve --listen " + state.Listen + " --log-file " + windowsCommandLineQuote(state.LogPath) + " --fallback-state " + windowsCommandLineQuote(collectorFallbackStatePath())
}

func registerWindowsCollectorFallback(command string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, windowsCollectorRunKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open user Run key: %w", err)
	}
	defer func() { _ = key.Close() }()
	if err := key.SetStringValue(windowsCollectorRunValue, command); err != nil {
		return fmt.Errorf("register user collector fallback: %w", err)
	}
	return nil
}

func unregisterWindowsCollectorFallback() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, windowsCollectorRunKey, registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return fmt.Errorf("open user Run key: %w", err)
	}
	defer func() { _ = key.Close() }()
	if err := key.DeleteValue(windowsCollectorRunValue); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("remove user collector fallback: %w", err)
	}
	return nil
}

func windowsCollectorFallbackRegistered(state windowsCollectorFallbackState) bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, windowsCollectorRunKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer func() { _ = key.Close() }()
	command, _, err := key.GetStringValue(windowsCollectorRunValue)
	return err == nil && command == state.Command
}

func writeWindowsCollectorFallbackState(state windowsCollectorFallbackState) error {
	contents, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode user collector fallback state: %w", err)
	}
	if err := os.WriteFile(collectorFallbackStatePath(), contents, 0o600); err != nil {
		return fmt.Errorf("write user collector fallback state: %w", err)
	}
	return nil
}

func readWindowsCollectorFallbackState() (windowsCollectorFallbackState, error) {
	contents, err := os.ReadFile(collectorFallbackStatePath())
	if err != nil {
		return windowsCollectorFallbackState{}, err
	}
	var state windowsCollectorFallbackState
	if err := json.Unmarshal(contents, &state); err != nil {
		return windowsCollectorFallbackState{}, fmt.Errorf("parse user collector fallback state: %w", err)
	}
	if state.Mode != windowsCollectorFallbackMode || state.Command != windowsCollectorRunCommand(state) {
		return windowsCollectorFallbackState{}, fmt.Errorf("invalid user collector fallback state")
	}
	return state, nil
}

func windowsProcessStartedAt(pid int) (int64, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0, err
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return 0, err
	}
	return created.Nanoseconds(), nil
}

func windowsCollectorFallbackRunning(state windowsCollectorFallbackState) bool {
	if state.PID <= 0 || state.StartedAt <= 0 {
		return false
	}
	startedAt, err := windowsProcessStartedAt(state.PID)
	return err == nil && startedAt == state.StartedAt
}

func stopWindowsCollectorFallback(state windowsCollectorFallbackState) error {
	if !windowsCollectorFallbackRunning(state) {
		return nil
	}
	if err := exec.Command("taskkill.exe", "/PID", strconv.Itoa(state.PID), "/T", "/F").Run(); err != nil {
		return fmt.Errorf("stop user collector fallback: %w", err)
	}
	return nil
}

func (windowsCollectorManager) Install(home, listen string) (CollectorStatus, error) {
	if err := validateCollectorListen(listen); err != nil {
		return CollectorStatus{}, err
	}
	if err := os.MkdirAll(collectorStateDir(), 0o700); err != nil {
		return CollectorStatus{}, err
	}
	executable, err := durableExecutablePath("")
	if err != nil {
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
	status, err := windowsCollectorStatusFn(context.Background(), listen)
	if err != nil {
		return CollectorStatus{}, err
	}
	if status.Mode == windowsCollectorFallbackMode {
		if status.Running {
			return windowsCollectorStartupStatus(status, status.Listen)
		}
		state, err := readWindowsCollectorFallbackState()
		if err != nil {
			return CollectorStatus{}, err
		}
		pid, startedAt, err := startWindowsFallbackCollector(state.Executable, state.Home, state.Listen, state.LogPath)
		if err != nil {
			return CollectorStatus{}, fmt.Errorf("start Windows user fallback collector: %w", err)
		}
		state.PID, state.StartedAt = pid, startedAt
		if err := writeWindowsCollectorFallbackState(state); err != nil {
			_ = stopWindowsCollectorFallbackFn(state)
			return CollectorStatus{}, err
		}
		status.Running = true
		status.Listen = state.Listen
		status.LogPath = state.LogPath
		status.ServiceID = windowsCollectorRunValue
		return windowsCollectorStartupStatus(status, state.Listen)
	}
	if _, err := (windowsCollectorManager{}).Install(home, listen); err != nil {
		return CollectorStatus{}, err
	}
	if _, err := runWindowsSchedulerCommand("/Run", "/TN", windowsCollectorTaskName); err != nil {
		return CollectorStatus{}, err
	}
	status, err = windowsCollectorStatusFn(context.Background(), listen)
	if err != nil {
		return CollectorStatus{}, err
	}
	return windowsCollectorStartupStatus(status, listen)
}

func windowsCollectorStartupStatus(status CollectorStatus, listen string) (CollectorStatus, error) {
	return collectorStartupStatus(context.Background(), status, func(ctx context.Context) (CollectorStatus, error) {
		return windowsCollectorStatusFn(ctx, listen)
	})
}

func (windowsCollectorManager) Stop() (CollectorStatus, error) {
	status, err := windowsCollectorStatusFn(context.Background(), defaultCollectorListen)
	if err != nil {
		return CollectorStatus{}, err
	}
	if !status.Installed {
		status.Message = "collector is not installed"
		return status, nil
	}
	if status.Mode == windowsCollectorFallbackMode {
		state, err := readWindowsCollectorFallbackState()
		if err != nil {
			return CollectorStatus{}, err
		}
		if err := stopWindowsCollectorFallbackFn(state); err != nil {
			return CollectorStatus{}, err
		}
		status.Running = false
		status.Message = "user fallback collector stopped"
		return status, nil
	}
	if _, err := runWindowsSchedulerCommand("/End", "/TN", windowsCollectorTaskName); err != nil {
		return CollectorStatus{}, err
	}
	status, err = windowsCollectorStatusFn(context.Background(), defaultCollectorListen)
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
	status, err := windowsCollectorStatusFn(context.Background(), defaultCollectorListen)
	if err != nil {
		return CollectorStatus{}, err
	}
	if state, stateErr := readWindowsCollectorFallbackState(); stateErr == nil {
		if err := stopWindowsCollectorFallbackFn(state); err != nil {
			return CollectorStatus{}, err
		}
	} else if !os.IsNotExist(stateErr) {
		return CollectorStatus{}, stateErr
	}
	if err := unregisterWindowsCollectorFallbackFn(); err != nil {
		return CollectorStatus{}, err
	}
	if status.Mode == windowsCollectorSchedulerMode && status.Installed {
		if _, err := manager.Stop(); err != nil {
			return CollectorStatus{}, err
		}
		if _, err := runWindowsSchedulerCommand("/Delete", "/TN", windowsCollectorTaskName, "/F"); err != nil {
			return CollectorStatus{}, err
		}
	}
	if err := os.RemoveAll(collectorStateDir()); err != nil {
		return CollectorStatus{}, err
	}
	status.Installed = false
	status.Running = false
	status.Mode = windowsCollectorNoMode
	status.Message = "collector uninstalled"
	return status, nil
}

func (windowsCollectorManager) ResolveManagedCollectorSettings(home, listen string, homeExplicit, listenExplicit bool) (string, string) {
	state, err := readWindowsCollectorFallbackState()
	if err != nil {
		return home, listen
	}
	if !homeExplicit {
		home = state.Home
	}
	if !listenExplicit {
		listen = state.Listen
	}
	return home, listen
}
