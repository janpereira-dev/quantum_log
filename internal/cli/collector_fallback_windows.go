//go:build windows

package cli

import (
	"fmt"
	"os"
)

func recordCollectorFallbackProcess(statePath, home, listen, logPath string) error {
	if statePath == "" {
		return nil
	}
	state, err := readWindowsCollectorFallbackState()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve user collector fallback executable: %w", err)
	}
	if state.Executable != executable || state.Home != home || state.Listen != listen || state.LogPath != logPath {
		return fmt.Errorf("user collector fallback identity does not match managed state")
	}
	startedAt, err := windowsProcessStartedAt(os.Getpid())
	if err != nil {
		return fmt.Errorf("inspect user collector fallback process: %w", err)
	}
	state.PID = os.Getpid()
	state.StartedAt = startedAt
	return writeWindowsCollectorFallbackState(state)
}
