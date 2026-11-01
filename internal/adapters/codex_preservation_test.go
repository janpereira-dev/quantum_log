package adapters

import (
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestCodexUninstallKeepsUserOwnedSettingsInsideOTelTable(t *testing.T) {
	for _, original := range []string{
		"model = \"user-model\"\n[unrelated]\nkeep = 7\n",
		"model = \"user-model\"\n[otel.exporter.otlp-http]\nendpoint = \"https://user.invalid/logs\"\nprotocol = \"json\"\n",
	} {
		t.Run(original, func(t *testing.T) {
			desired := codexOTelSettings("synthetic-token", "C:/synthetic-ca.pem")
			installed, state, _, err := updateCodexOTel(original, desired)
			if err != nil {
				t.Fatal(err)
			}
			if !state.CreatedOTel {
				t.Fatal("fixture must have a qlog-created root header")
			}
			edited := strings.Replace(installed, "[otel]\n", "[otel]\nenvironment = \"user-environment\"\n", 1)
			if !strings.Contains(original, "[otel.exporter") {
				edited = strings.Replace(edited, "exporter = "+desired["exporter"], "exporter = \"user-owned-exporter\"", 1)
			}
			var before map[string]any
			if err := toml.Unmarshal([]byte(edited), &before); err != nil {
				t.Fatal(err)
			}
			restored, _, err := restoreCodexOTel(edited, state, desired)
			if err != nil {
				t.Fatal(err)
			}
			var after map[string]any
			if err := toml.Unmarshal([]byte(restored), &after); err != nil {
				t.Fatalf("uninstall produced invalid TOML: %v %s", err, restored)
			}
			otel, ok := after["otel"].(map[string]any)
			if !ok || otel["environment"] != "user-environment" {
				t.Fatalf("user OTel setting moved out of its table: %s", restored)
			}
			if !reflect.DeepEqual(before["unrelated"], after["unrelated"]) || before["model"] != after["model"] {
				t.Fatalf("unrelated table/root values changed: %s", restored)
			}
			if strings.Contains(original, "[otel.exporter") {
				var originalDoc map[string]any
				if err := toml.Unmarshal([]byte(original), &originalDoc); err != nil {
					t.Fatal(err)
				}
				originalExporter := originalDoc["otel"].(map[string]any)["exporter"]
				if !reflect.DeepEqual(originalExporter, otel["exporter"]) {
					t.Fatalf("nested user exporter not restored: %s", restored)
				}
			} else if otel["exporter"] != "user-owned-exporter" {
				t.Fatalf("edited user exporter changed: %s", restored)
			}
		})
	}
}

func TestCodexUninstallRemovesOnlyEmptyInjectedOTelRootHeader(t *testing.T) {
	for _, original := range []string{"model = \"user-model\"\n", "[otel.exporter.otlp-http]\nendpoint = \"https://user.invalid/logs\"\n"} {
		desired := codexOTelSettings("synthetic-token", "C:/synthetic-ca.pem")
		installed, state, _, err := updateCodexOTel(original, desired)
		if err != nil {
			t.Fatal(err)
		}
		restored, _, err := restoreCodexOTel(installed, state, desired)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(restored, "[otel]\n") {
			t.Fatalf("empty injected root header retained: %s", restored)
		}
		var before, after map[string]any
		if err := toml.Unmarshal([]byte(original), &before); err != nil {
			t.Fatal(err)
		}
		if err := toml.Unmarshal([]byte(restored), &after); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("uninstall changed original config semantics: %s", restored)
		}
	}
}
