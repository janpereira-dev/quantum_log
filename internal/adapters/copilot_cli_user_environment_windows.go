//go:build windows

package adapters

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const copilotCLIUserEnvironmentKey = `Environment`

type windowsUserEnvironmentStore struct{}

var sendMessageTimeout = windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")

func newCopilotCLIUserEnvironmentStore() copilotCLIUserEnvironmentStore {
	return windowsUserEnvironmentStore{}
}

func (windowsUserEnvironmentStore) Get(name string) (string, bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, copilotCLIUserEnvironmentKey, registry.QUERY_VALUE)
	if err == registry.ErrNotExist {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer func() { _ = key.Close() }()
	value, _, err := key.GetStringValue(name)
	if err == registry.ErrNotExist {
		return "", false, nil
	}
	return value, err == nil, err
}

func (windowsUserEnvironmentStore) Set(name, value string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, copilotCLIUserEnvironmentKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() { _ = key.Close() }()
	return key.SetStringValue(name, value)
}

func (windowsUserEnvironmentStore) Delete(name string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, copilotCLIUserEnvironmentKey, registry.SET_VALUE)
	if err == registry.ErrNotExist {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = key.Close() }()
	err = key.DeleteValue(name)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}

func notifyCopilotCLIUserEnvironment() error {
	environment, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return err
	}
	var result uintptr
	const (
		hwndBroadcast    = 0xffff
		wmSettingChange  = 0x001a
		smtoAbortIfHung  = 0x0002
		timeoutMillisecs = 5000
	)
	returned, _, callErr := sendMessageTimeout.Call(
		hwndBroadcast,
		wmSettingChange,
		0,
		uintptr(unsafe.Pointer(environment)),
		smtoAbortIfHung,
		timeoutMillisecs,
		uintptr(unsafe.Pointer(&result)),
	)
	if returned == 0 {
		return fmt.Errorf("notify Windows environment change: %w", callErr)
	}
	return nil
}
