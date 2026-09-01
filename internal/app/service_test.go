package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/config"
)

func TestInitializeRejectsPurgeSentinelBeforeConfigurationCreation(t *testing.T) {
	home := filepath.Join(t.TempDir(), "ledger")
	paths, err := config.Resolve(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Database+".purge.pending", nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Initialize(context.Background(), home); err == nil || !strings.Contains(err.Error(), "purge is pending") {
		t.Fatalf("Initialize() error = %v", err)
	}
	if _, err := os.Stat(paths.ConfigFile); !os.IsNotExist(err) {
		t.Fatalf("Initialize created configuration despite the purge sentinel: %v", err)
	}
}
