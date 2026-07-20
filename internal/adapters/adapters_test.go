package adapters

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultRegistryDeclaresOnlyVerifiedCapabilities(t *testing.T) {
	registry := Default()
	items := registry.List()
	if len(items) != 3 {
		t.Fatalf("List() returned %d adapters, want 3", len(items))
	}
	generic, found := registry.Get("generic-jsonl")
	if !found || !generic.Descriptor().Capabilities.StructuredEvents {
		t.Fatal("generic JSONL adapter must declare structured event support")
	}
	if generic.Descriptor().Capabilities.Costs || generic.Descriptor().Capabilities.InputTokens {
		t.Fatal("generic JSONL must not claim metrics supplied only by callers")
	}
	for _, id := range []string{"opencode", "claude-code"} {
		adapter, found := registry.Get(id)
		if !found {
			t.Fatalf("missing %s adapter", id)
		}
		if adapter.Descriptor().Capabilities != (Capabilities{}) {
			t.Fatalf("%s minimal adapter claimed unsupported capture capability", id)
		}
	}
}

func TestDefaultRegistryAdaptersExposeSetupLifecycle(t *testing.T) {
	t.Setenv("QLOG_ADAPTER_CONFIG_HOME", t.TempDir())
	for _, adapter := range Default().List() {
		status, err := adapter.Status(context.Background())
		if err != nil {
			t.Fatalf("%s status: %v", adapter.Descriptor().ID, err)
		}
		if status.AdapterID != adapter.Descriptor().ID || status.State == "" || status.CaptureQuality == "" {
			t.Fatalf("%s status = %#v", adapter.Descriptor().ID, status)
		}
		result, err := adapter.Test(context.Background())
		if err != nil {
			t.Fatalf("%s test: %v", adapter.Descriptor().ID, err)
		}
		if result.AdapterID != adapter.Descriptor().ID || result.CaptureQuality == "" {
			t.Fatalf("%s test = %#v", adapter.Descriptor().ID, result)
		}
	}
}

func TestMinimalAdapterDryRunIsIdempotentAndDoesNotWrite(t *testing.T) {
	adapter, _ := Default().Get("opencode")
	first, err := adapter.Install(context.Background(), InstallOptions{DryRun: true})
	if err != nil {
		t.Fatalf("first dry run: %v", err)
	}
	second, err := adapter.Install(context.Background(), InstallOptions{DryRun: true})
	if err != nil {
		t.Fatalf("second dry run: %v", err)
	}
	if first.Changed || second.Changed || len(first.Actions) != 1 || first.Actions[0] != second.Actions[0] || !strings.Contains(first.Actions[0], "dry run") {
		t.Fatalf("dry-run install = %#v then %#v", first, second)
	}
}

func TestApplyMarkerBlockCreatesUpdatesBacksUpAndStaysIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent", "config.md")
	change, err := ApplyMarkerBlock(path, "agent-auto-capture", "first", false)
	if err != nil {
		t.Fatalf("create marker block: %v", err)
	}
	if change.Action != "created" || change.BackupPath != "" {
		t.Fatalf("create change = %#v", change)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if !strings.Contains(string(contents), "<!-- qlog:begin agent-auto-capture -->") || !strings.Contains(string(contents), "first") {
		t.Fatalf("created marker content = %q", contents)
	}
	if !HasMarkerBlock(path, "agent-auto-capture") {
		t.Fatal("marker block not detected")
	}

	change, err = ApplyMarkerBlock(path, "agent-auto-capture", "second", false)
	if err != nil {
		t.Fatalf("update marker block: %v", err)
	}
	if change.Action != "updated" || change.BackupPath == "" {
		t.Fatalf("update change = %#v", change)
	}
	backup, err := os.ReadFile(change.BackupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !strings.Contains(string(backup), "first") {
		t.Fatalf("backup = %q", backup)
	}

	change, err = ApplyMarkerBlock(path, "agent-auto-capture", "second", false)
	if err != nil {
		t.Fatalf("idempotent marker block: %v", err)
	}
	if change.Action != "unchanged" || change.BackupPath != "" {
		t.Fatalf("idempotent change = %#v", change)
	}
}

func TestApplyMarkerBlockDryRunDoesNotWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "config.md")
	change, err := ApplyMarkerBlock(path, "agent-auto-capture", "content", true)
	if err != nil {
		t.Fatalf("dry-run marker block: %v", err)
	}
	if change.Action != "create" {
		t.Fatalf("dry-run change = %#v", change)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote file: %v", err)
	}
}
