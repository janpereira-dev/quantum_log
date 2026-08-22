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
)

type windowsCollectorManager struct{}

var runWindowsSchedulerCommand = func(args ...string) ([]byte, error) {
	return exec.Command("schtasks.exe", args...).CombinedOutput()
}

var queryWindowsCollectorTask = func(ctx context.Context) ([]byte, error) {
	return exec.CommandContext(ctx, "schtasks.exe", "/Query", "/TN", windowsCollectorTaskName, "/FO", "LIST", "/V").CombinedOutput()
}

var windowsCollectorTaskExists = func() (bool, error) {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = os.Getenv("WINDIR")
	}
	if root == "" {
		return false, fmt.Errorf("windows system root is unavailable")
	}
	_, err := os.Stat(filepath.Join(root, "System32", "Tasks", windowsCollectorTaskName))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

var windowsCollectorStatusFn = windowsCollectorStatus
var stopWindowsCollectorFallbackFn = stopWindowsCollectorFallback
var unregisterWindowsCollectorFallbackFn = unregisterWindowsCollectorFallback

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
	output, err := queryWindowsCollectorTask(ctx)
	if err == nil {
		status.Installed = true
		status.Running = strings.Contains(string(output), "Running")
		status.Mode = windowsCollectorSchedulerMode
	} else if exists, existsErr := windowsCollectorTaskExists(); exists || existsErr != nil {
		settings, settingsErr := readWindowsCollectorTaskDefinitionSettings()
		if settingsErr == nil {
			// A local qlog-owned definition remains the only durable description of a
			// task whose Scheduler ACL or policy no longer permits /Query. Treat it as
			// installed conservatively so setup cannot redirect a surviving task.
			status.Installed = true
			status.Mode = windowsCollectorSchedulerMode
			status.Listen = settings.Listen
			status.Message = "collector task status unavailable; using persisted task settings"
		}
	}
	if status.Mode == windowsCollectorNoMode {
		if fallback, fallbackErr := readWindowsCollectorFallbackState(); fallbackErr == nil && windowsCollectorFallbackRegistered(fallback) {
			status.Installed = true
			status.Mode = windowsCollectorFallbackMode
			status.ServiceID = windowsCollectorRunValue
			status.Listen = fallback.Listen
			status.LogPath = fallback.LogPath
			status.Running = windowsCollectorFallbackRunning(fallback)
		}
	}
	health := probeCollectorHealth(ctx, status.Listen)
	return collectorStatusWithHealth(status, health), nil
}

func windowsCollectorRunCommand(state windowsCollectorFallbackState) string {
	return windowsCommandLineQuote(state.Executable) + " --home " + windowsCommandLineQuote(state.Home) + " collector serve --listen " + state.Listen + " --log-file " + windowsCommandLineQuote(state.LogPath) + " --fallback-state " + windowsCommandLineQuote(collectorFallbackStatePath())
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
	// Keep the currently registered task's settings readable until /Create
	// succeeds. A failed replacement must not overwrite the recovery source.
	stagedDefinition := collectorTaskDefinitionPath() + ".next"
	if err := writeWindowsCollectorTaskDefinition(stagedDefinition, executable, home, listen, userID, collectorLogPath()); err != nil {
		return CollectorStatus{}, err
	}
	defer func() { _ = os.Remove(stagedDefinition) }()
	if err := createWindowsCollectorTask(stagedDefinition); err != nil {
		return CollectorStatus{}, err
	}
	if err := os.Rename(stagedDefinition, collectorTaskDefinitionPath()); err != nil {
		return CollectorStatus{}, fmt.Errorf("persist installed collector task definition: %w", err)
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
		return CollectorStatus{}, legacyWindowsFallbackError()
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
	current, err := windowsCollectorStatusFn(context.Background(), listen)
	if err != nil {
		return CollectorStatus{}, err
	}
	if current.Mode == windowsCollectorFallbackMode {
		return CollectorStatus{}, legacyWindowsFallbackError()
	}
	if _, err := manager.Stop(); err != nil {
		return CollectorStatus{}, err
	}
	status, err := manager.Start(home, listen)
	if err == nil || !isWindowsSchedulerPolicyDenial(err) {
		return status, err
	}
	restored, restoreErr := manager.RestartExisting(listen)
	if restoreErr != nil {
		return CollectorStatus{}, fmt.Errorf("%w; resume existing collector: %v", err, restoreErr)
	}
	if windowsCollectorSettingsMatch(home, listen) {
		return restored, nil
	}
	return restored, err
}

// RestartExisting starts a collector already owned by qlog without creating a
// new Task Scheduler definition or a legacy Run-key fallback.
func (manager windowsCollectorManager) RestartExisting(listen string) (CollectorStatus, error) {
	if err := validateCollectorListen(listen); err != nil {
		return CollectorStatus{}, err
	}
	status, err := windowsCollectorStatusFn(context.Background(), listen)
	if err != nil {
		return CollectorStatus{}, err
	}
	if !status.Installed {
		return CollectorStatus{}, fmt.Errorf("existing collector is not installed")
	}
	if status.Mode == windowsCollectorFallbackMode {
		return CollectorStatus{}, legacyWindowsFallbackError()
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

func (windowsCollectorManager) Status(ctx context.Context, listen string) (CollectorStatus, error) {
	if err := validateCollectorListen(listen); err != nil {
		return CollectorStatus{}, err
	}
	return windowsCollectorStatus(ctx, listen)
}

func legacyWindowsFallbackError() error {
	return fmt.Errorf("legacy Windows Run-key fallback is installed; run qlog collector uninstall before installing or starting a managed collector")
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
	status, err := windowsCollectorStatusFn(context.Background(), listen)
	if err != nil {
		return home, listen
	}
	switch status.Mode {
	case windowsCollectorSchedulerMode:
		// The scheduled task is the active collector whenever it exists. A stale
		// legacy fallback file must not redirect lifecycle commands away from it.
		if taskHome, taskListen, err := readWindowsCollectorTaskSettings(); err == nil {
			if !homeExplicit {
				home = taskHome
			}
			if !listenExplicit {
				listen = taskListen
			}
		}
	case windowsCollectorFallbackMode:
		state, err := readWindowsCollectorFallbackState()
		if err != nil {
			break
		}
		if !homeExplicit {
			home = state.Home
		}
		if !listenExplicit {
			listen = state.Listen
		}
	}
	return home, listen
}

type windowsCollectorTaskSettings struct {
	Executable string
	Home       string
	Listen     string
}

func windowsCollectorSettingsMatch(home, listen string) bool {
	executable, err := durableExecutablePath("")
	if err != nil {
		return false
	}
	return windowsCollectorTaskTargetMatches(home, listen, executable)
}

func (windowsCollectorManager) MatchesInstalledTarget(home, listen string) bool {
	return windowsCollectorSettingsMatch(home, listen)
}

func windowsCollectorTaskTargetMatches(home, listen, executable string) bool {
	status, err := windowsCollectorStatusFn(context.Background(), listen)
	if err != nil {
		return false
	}
	switch status.Mode {
	case windowsCollectorSchedulerMode:
		settings, err := readWindowsCollectorTaskDefinitionSettings()
		return err == nil &&
			strings.EqualFold(filepath.Clean(settings.Executable), filepath.Clean(executable)) &&
			strings.EqualFold(filepath.Clean(settings.Home), filepath.Clean(home)) &&
			settings.Listen == listen
	case windowsCollectorFallbackMode:
		state, err := readWindowsCollectorFallbackState()
		return err == nil &&
			strings.EqualFold(filepath.Clean(state.Executable), filepath.Clean(executable)) &&
			strings.EqualFold(filepath.Clean(state.Home), filepath.Clean(home)) &&
			state.Listen == listen
	default:
		return false
	}
}

func readWindowsCollectorTaskSettings() (string, string, error) {
	settings, err := readWindowsCollectorTaskDefinitionSettings()
	if err != nil {
		return "", "", err
	}
	return settings.Home, settings.Listen, nil
}

func readWindowsCollectorTaskDefinitionSettings() (windowsCollectorTaskSettings, error) {
	contents, err := os.ReadFile(collectorTaskDefinitionPath())
	if err != nil {
		return windowsCollectorTaskSettings{}, err
	}
	if len(contents) < 2 || contents[0] != 0xFF || contents[1] != 0xFE || len(contents)%2 != 0 {
		return windowsCollectorTaskSettings{}, fmt.Errorf("invalid collector task encoding")
	}
	codeUnits := make([]uint16, 0, (len(contents)-2)/2)
	for offset := 2; offset < len(contents); offset += 2 {
		codeUnits = append(codeUnits, uint16(contents[offset])|uint16(contents[offset+1])<<8)
	}
	definition := strings.Replace(string(utf16.Decode(codeUnits)), `encoding="UTF-16"`, `encoding="UTF-8"`, 1)
	var task struct {
		Actions struct {
			Exec struct {
				Command   string `xml:"Command"`
				Arguments string `xml:"Arguments"`
			} `xml:"Exec"`
		} `xml:"Actions"`
	}
	if err := xml.Unmarshal([]byte(definition), &task); err != nil {
		return windowsCollectorTaskSettings{}, fmt.Errorf("parse collector task definition: %w", err)
	}
	executable := task.Actions.Exec.Command
	arguments := task.Actions.Exec.Arguments
	const homePrefix = `--home "`
	if !strings.HasPrefix(arguments, homePrefix) {
		return windowsCollectorTaskSettings{}, fmt.Errorf("collector task has no quoted home")
	}
	remainder := strings.TrimPrefix(arguments, homePrefix)
	endHome := strings.Index(remainder, `" collector serve --listen `)
	if endHome < 0 {
		return windowsCollectorTaskSettings{}, fmt.Errorf("collector task has invalid home arguments")
	}
	home := remainder[:endHome]
	remainder = remainder[endHome+len(`" collector serve --listen `):]
	endListen := strings.IndexByte(remainder, ' ')
	if endListen < 0 {
		return windowsCollectorTaskSettings{}, fmt.Errorf("collector task has no log-file argument")
	}
	listen := remainder[:endListen]
	if !filepath.IsAbs(executable) || !filepath.IsAbs(home) || validateCollectorListen(listen) != nil {
		return windowsCollectorTaskSettings{}, fmt.Errorf("collector task has invalid managed settings")
	}
	return windowsCollectorTaskSettings{Executable: executable, Home: home, Listen: listen}, nil
}
