// Package config resolves local-first QUANTUM_LOG paths.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

type Paths struct {
	Home       string
	ConfigFile string
	Database   string
}

func Resolve(homeOverride string) (Paths, error) {
	home := homeOverride
	if home == "" {
		home = os.Getenv("QLOG_HOME")
	}
	if home == "" {
		var err error
		switch runtime.GOOS {
		case "windows":
			home = os.Getenv("LOCALAPPDATA")
			if home == "" {
				home, err = os.UserConfigDir()
			}
			if err == nil {
				home = filepath.Join(home, "QUANTUM_LOG")
			}
		case "darwin":
			base, baseErr := os.UserConfigDir()
			err = baseErr
			if err == nil {
				home = filepath.Join(base, "QUANTUM_LOG")
			}
		default:
			base := os.Getenv("XDG_DATA_HOME")
			if base == "" {
				userHome, homeErr := os.UserHomeDir()
				err = homeErr
				base = filepath.Join(userHome, ".local", "share")
			}
			if err == nil {
				home = filepath.Join(base, "quantum-log")
			}
		}
		if err != nil {
			return Paths{}, err
		}
	}
	if home == "" {
		return Paths{}, errors.New("could not resolve QLOG_HOME")
	}
	abs, err := filepath.Abs(home)
	if err != nil {
		return Paths{}, err
	}
	return Paths{Home: abs, ConfigFile: filepath.Join(abs, "config.yaml"), Database: filepath.Join(abs, "qlog.db")}, nil
}

func Ensure(paths Paths) error {
	if err := os.MkdirAll(paths.Home, 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(paths.ConfigFile); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	const defaultConfig = "schemaVersion: 1\nprivacy:\n  capturePromptContent: false\n  captureResponseContent: false\n  captureToolArguments: false\n  captureToolResults: false\n  captureAbsolutePathLocally: true\n  hashPathsOnExport: true\n  redactSecrets: true\n  redactPII: true\n"
	return os.WriteFile(paths.ConfigFile, []byte(defaultConfig), 0o600)
}
