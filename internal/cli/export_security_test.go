package cli

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

func TestLegacyCSVExportNeutralizesFormulaMetadata(t *testing.T) {
	var output bytes.Buffer
	if err := writeCSV(&output, []sqlite.ExportRecord{{ProjectSlug: "=1+1", Provider: "+provider", TotalTokens: 7}}); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(output.String())).ReadAll()
	if err != nil || len(rows) != 2 || rows[1][2] != "'=1+1" || rows[1][4] != "'+provider" || rows[1][12] != "7" {
		t.Fatalf("unsafe metadata or changed numeric token field: %v err=%v", rows, err)
	}
}
