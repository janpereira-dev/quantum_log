package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/adapters"
	"github.com/janpereira-dev/quantum_log/internal/config"
)

func TestAdapterInstallCredentialsUseExplicitHomeInsteadOfEnvironment(t *testing.T) {
	selected, ambient, settings := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("QLOG_HOME", ambient)
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", settings)
	configPath := filepath.Join(settings, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatal(err)
	}
	original := "model = \"user-model\"\n[otel]\nexporter = \"none\"\nlog_user_prompt = true\n"
	if err := os.WriteFile(configPath, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := runQLog(t, selected, "adapter", "install", "codex"); err != nil {
		t.Fatal(err)
	}
	paths, _ := config.Resolve(selected)
	if _, err := os.Stat(config.CollectorCertificatePath(paths)); err != nil {
		t.Fatalf("explicit-home identity was not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ambient, "prompt-hash.key")); !os.IsNotExist(err) {
		t.Fatal("credentials written to ambient home")
	}
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), strconv.Quote(config.CollectorCertificatePath(paths))) {
		t.Fatalf("native exporter trusts wrong home: %s", contents)
	}
	if _, err := runQLog(t, selected, "adapter", "uninstall", "codex"); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(configPath)
	if err != nil || string(restored) != original {
		t.Fatalf("explicit-home uninstall did not restore user config: %s %v", restored, err)
	}
}

func TestAdapterInstallDryRunCarriesExplicitHomeWithoutCreatingCredentials(t *testing.T) {
	selected, ambient := t.TempDir(), t.TempDir()
	t.Setenv("QLOG_HOME", ambient)
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	output, err := runQLog(t, selected, "adapter", "install", "claude-code", "--dry-run", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var result adapters.InstallResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Changes) != 1 || !strings.Contains(result.Changes[0].Description, strconv.Quote(selected)) {
		t.Fatalf("dry run lost explicit home: %+v", result.Changes)
	}
	for _, home := range []string{selected, ambient} {
		entries, err := os.ReadDir(home)
		if err != nil || len(entries) != 0 {
			t.Fatalf("dry run created credentials in %s: %v %v", home, entries, err)
		}
	}
}
