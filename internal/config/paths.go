// Package config resolves local-first QUANTUM_LOG paths.
package config

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Paths struct {
	Home          string
	ConfigFile    string
	Database      string
	PromptHashKey string
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
	return Paths{Home: abs, ConfigFile: filepath.Join(abs, "config.yaml"), Database: filepath.Join(abs, "qlog.db"), PromptHashKey: filepath.Join(abs, "prompt-hash.key")}, nil
}

func Ensure(paths Paths) error {
	if err := os.MkdirAll(paths.Home, 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(paths.ConfigFile); errors.Is(err, os.ErrNotExist) {
		const defaultConfig = "schemaVersion: 1\nprivacy:\n  promptCapture: hash\n  capturePromptContent: false\n  captureResponseContent: false\n  captureToolArguments: false\n  captureToolResults: false\n  captureAbsolutePathLocally: true\n  hashPathsOnExport: true\n  redactSecrets: true\n  redactPII: true\n"
		if err := os.WriteFile(paths.ConfigFile, []byte(defaultConfig), 0o600); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	_, err := PromptHashKey(paths)
	return err
}

// PromptHashKey returns the installation-local secret used to prevent prompt
// fingerprints from being comparable or reversible across ledgers.
func PromptHashKey(paths Paths) ([]byte, error) {
	keyPath := paths.PromptHashKey
	if keyPath == "" {
		keyPath = filepath.Join(paths.Home, "prompt-hash.key")
	}
	key, err := os.ReadFile(keyPath)
	if err == nil && len(key) >= 32 {
		return key, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func SetPromptCaptureMode(paths Paths, mode string) error {
	if mode != "off" && mode != "hash" && mode != "full" {
		return errors.New("prompt capture mode must be off, hash, or full")
	}
	if err := Ensure(paths); err != nil {
		return err
	}
	contents, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		return err
	}
	lines := strings.Split(string(contents), "\n")
	found := false
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "promptCapture:") {
			lines[index] = "  promptCapture: " + mode
			found = true
		}
	}
	if !found {
		for index, line := range lines {
			if line == "privacy:" {
				lines = append(lines[:index+1], append([]string{"  promptCapture: " + mode}, lines[index+1:]...)...)
				found = true
				break
			}
		}
	}
	if !found {
		return errors.New("privacy configuration is missing")
	}
	return os.WriteFile(paths.ConfigFile, []byte(strings.Join(lines, "\n")), 0o600)
}

func PromptCaptureMode(paths Paths) string {
	contents, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		return "hash"
	}
	for _, line := range strings.Split(string(contents), "\n") {
		key, _, found := strings.Cut(strings.TrimSpace(line), ":")
		if found && key == "promptCapture" {
			mode := strings.TrimSpace(valueAfterColon(line))
			if mode == "off" || mode == "hash" || mode == "full" {
				return mode
			}
		}
	}
	return "hash"
}

func valueAfterColon(line string) string { _, value, _ := strings.Cut(line, ":"); return value }
