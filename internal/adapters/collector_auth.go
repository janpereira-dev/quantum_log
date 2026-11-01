package adapters

import (
	"os"
	"path/filepath"

	"github.com/janpereira-dev/quantum_log/internal/config"
)

func collectorCredentials(home string, dryRun bool) (config.Paths, string, error) {
	paths, err := config.Resolve(home)
	if err != nil {
		return config.Paths{}, "", err
	}
	if dryRun {
		return paths, "<qlog-collector-token>", nil
	}
	if err := config.Ensure(paths); err != nil {
		return paths, "", err
	}
	if err := config.EnsureCollectorTLS(paths, "127.0.0.1:4318"); err != nil {
		return paths, "", err
	}
	token, err := config.CollectorToken(paths)
	return paths, token, err
}

func writeCollectorTokenFile(home, token string, dryRun bool) (string, error) {
	path := filepath.Join(home, "collector.token")
	if dryRun {
		return path, nil
	}
	if err := writePrivateAdapterFile(path, []byte(token)); err != nil {
		return "", err
	}
	return path, nil
}

func ensurePrivateAdapterPaths(paths ...string) error {
	for _, path := range paths {
		if err := config.EnsurePrivateDirectory(filepath.Dir(path)); err != nil {
			return err
		}
		if err := config.CheckPrivateFile(path); err != nil {
			return err
		}
	}
	return nil
}

func writePrivateAdapterFile(path string, data []byte) error {
	if err := ensurePrivateAdapterPaths(path); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
