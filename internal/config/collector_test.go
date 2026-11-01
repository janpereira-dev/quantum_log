package config

import (
	"encoding/hex"
	"os"
	"testing"
)

func TestCollectorTokenIsInstallationScopedAndDomainSeparated(t *testing.T) {
	paths, _ := Resolve(t.TempDir())
	first, err := CollectorToken(paths)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CollectorToken(paths)
	if err != nil || first != second || len(first) != 64 {
		t.Fatalf("unstable or malformed token: %v", err)
	}
	key, err := os.ReadFile(paths.PromptHashKey)
	if err != nil {
		t.Fatal(err)
	}
	if first == hex.EncodeToString(key) {
		t.Fatal("collector exposes the prompt hashing key")
	}
	otherPaths, _ := Resolve(t.TempDir())
	other, err := CollectorToken(otherPaths)
	if err != nil || first == other {
		t.Fatalf("token not installation scoped: %v", err)
	}
}
