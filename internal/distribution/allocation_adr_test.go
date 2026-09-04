package distribution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllocationRecoveryADRContract(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "architecture", "ADR-007-allocation-corrections-and-recovery.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(data))
	for _, required := range []string{
		"status: accepted", "append-only", "revision_id", "parent_revision_id", "idempotency",
		"basis points", "revert", "projection", "crash recovery", "anchor", "migration 014",
		"design a", "design b", "transactionality", "auditability", "recovery determinism",
		"allocation_history_test.go", "concurrent writers", "forward migrations",
	} {
		if !strings.Contains(text, strings.ToLower(required)) {
			t.Errorf("ADR-007 missing %q", required)
		}
	}
}
