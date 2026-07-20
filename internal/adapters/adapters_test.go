package adapters

import (
	"context"
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
