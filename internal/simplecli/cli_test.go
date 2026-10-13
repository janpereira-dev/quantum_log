package simplecli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func execute(t *testing.T, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	if err := Run(context.Background(), args, &out, &errOut, Version{Version: "test"}); err != nil {
		t.Fatalf("Run: %v; stderr: %s", err, errOut.String())
	}
	return out.String()
}

func nativeSession(at time.Time) string {
	return fmt.Sprintf("{\"type\":\"session_meta\",\"payload\":{\"id\":\"native-session\",\"model_provider\":\"openai\"}}\n{\"type\":\"turn_context\",\"payload\":{\"model\":\"test-model\"}}\n{\"timestamp\":%q,\"type\":\"event_msg\",\"payload\":{\"type\":\"token_count\",\"info\":{\"total_token_usage\":{\"input_tokens\":100,\"cached_input_tokens\":40,\"output_tokens\":30,\"reasoning_output_tokens\":10,\"total_tokens\":130}}}}\n", at.Format(time.RFC3339Nano))
}

func TestImportReplayReportAndExportPreserveSourceAndLedger(t *testing.T) {
	home := filepath.Join(t.TempDir(), "data")
	file := filepath.Join(t.TempDir(), "session.jsonl")
	source := nativeSession(time.Now())
	if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		var imported ImportResult
		if err := json.Unmarshal([]byte(execute(t, "--home", home, "import", "--json", file)), &imported); err != nil {
			t.Fatal(err)
		}
		if imported.Files != 1 || i == 0 && imported.Samples != 1 || i == 1 && imported.Duplicates != 1 {
			t.Fatalf("import %d: %+v", i, imported)
		}
	}
	before, err := os.ReadFile(filepath.Join(home, "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	var summary Summary
	if err := json.Unmarshal([]byte(execute(t, "report", "--home", home, "--json")), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.UsageSamples != 1 || summary.TotalTokens != 130 || summary.CostQuality != "unavailable" || len(summary.Rows) != 1 || summary.Rows[0].Project != "unattributed" {
		t.Fatalf("summary: %+v", summary)
	}
	rows, err := csv.NewReader(strings.NewReader(execute(t, "--home", home, "export"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1][2] != "openai" || rows[1][4] != "130" || rows[1][6] != "unavailable" {
		t.Fatalf("CSV: %v", rows)
	}
	after, err := os.ReadFile(filepath.Join(home, "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("report/export changed the ledger")
	}
	unchanged, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != source {
		t.Fatal("source changed")
	}
}

func TestProvidersRemainDistinctAcrossReportFormats(t *testing.T) {
	home := t.TempDir()
	file := filepath.Join(t.TempDir(), "providers.jsonl")
	first := nativeSession(time.Now())
	second := strings.ReplaceAll(strings.ReplaceAll(first, "native-session", "custom-session"), "openai", "custom-provider")
	if err := os.WriteFile(file, []byte(first), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(file), "custom.jsonl"), []byte(second), 0o600); err != nil {
		t.Fatal(err)
	}
	execute(t, "--home", home, "import", filepath.Dir(file))
	var summary Summary
	if err := json.Unmarshal([]byte(execute(t, "--home", home, "report", "--json")), &summary); err != nil {
		t.Fatal(err)
	}
	if len(summary.Rows) != 2 || summary.TotalTokens != 260 || summary.Rows[0].Provider == summary.Rows[1].Provider {
		t.Fatalf("provider distinction lost: %+v", summary)
	}
	text := execute(t, "--home", home, "report")
	if !strings.Contains(text, "openai") || !strings.Contains(text, "custom-provider") {
		t.Fatal("text lost provider")
	}
	rows, err := csv.NewReader(strings.NewReader(execute(t, "--home", home, "export"))).ReadAll()
	if err != nil || len(rows) != 3 || rows[1][2] == rows[2][2] {
		t.Fatalf("CSV lost provider: %v %v", rows, err)
	}
}

func TestEmptyExplicitHomeFailsWithoutUsingDefaultLedger(t *testing.T) {
	root := t.TempDir()
	t.Setenv("QLOG_HOME", root)
	for _, args := range [][]string{{"--home=", "import"}, {"--home", "", "import"}, {"--home", "   ", "report"}} {
		var out, errOut bytes.Buffer
		if err := Run(context.Background(), args, &out, &errOut, Version{}); err == nil {
			t.Fatalf("accepted empty override: %v", args)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "simple")); !os.IsNotExist(err) {
		t.Fatal("empty override used default ledger")
	}
}

func TestSelectedSymlinkRootIsFollowedButNestedLinksAreNot(t *testing.T) {
	root, target := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "own.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := t.TempDir()
	if err := os.WriteFile(filepath.Join(nested, "excluded.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "sessions")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(nested, filepath.Join(target, "nested")); err != nil {
		t.Fatal(err)
	}
	files, count, err := discover(context.Background(), link, time.Time{}, 100)
	if err != nil || count != 1 || len(files) != 1 || filepath.Base(files[0]) != "own.jsonl" {
		t.Fatalf("root/nested link contract: %v %d %v", files, count, err)
	}
}

func TestExplicitMigrationPreservesOlderLedgerAndEnablesReadonlyReports(t *testing.T) {
	home := t.TempDir()
	database := filepath.Join(home, "qlog.db")
	db, err := sql.Open("sqlite", database)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE schema_migrations(version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join("..", "storage", "sqlite", "migrations", "*.sql"))
	if err != nil || len(files) < 13 {
		t.Fatal("missing historical schema fixture")
	}
	for _, file := range files[:13] {
		contents, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(contents)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO schema_migrations VALUES(?,?)", filepath.Base(file), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO model_calls(id,started_at,agent_name,provider,model_id,input_tokens,total_tokens,capture_quality,created_at) VALUES('legacy',?,'codex','openai','legacy-model',20,20,'agent_reported',?)`, time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{".quiescence.lock", ".writer.lock"} {
		if err := os.WriteFile(database+suffix, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.ReadFile(database)
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	err = Run(context.Background(), []string{"--home", home, "report"}, &out, &errOut, Version{})
	if err == nil || !strings.Contains(err.Error(), "qlog migrate") || strings.Contains(err.Error(), "qlog init") {
		t.Fatalf("missing usable migration advice: %v", err)
	}
	after, err := os.ReadFile(database)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("readonly failure mutated old database")
	}
	execute(t, "--home", home, "migrate")
	var summary Summary
	if err := json.Unmarshal([]byte(execute(t, "--home", home, "report", "--json")), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.TotalTokens != 20 || summary.UsageSamples != 1 {
		t.Fatalf("migration lost evidence: %+v", summary)
	}
	rows, err := csv.NewReader(strings.NewReader(execute(t, "--home", home, "export"))).ReadAll()
	if err != nil || len(rows) != 2 || rows[1][4] != "20" {
		t.Fatalf("export after migration: %v %v", rows, err)
	}
}

func TestDefaultReportDoesNotInitializeOrTouchLegacyLedger(t *testing.T) {
	legacy := t.TempDir()
	t.Setenv("QLOG_HOME", legacy)
	if err := os.WriteFile(filepath.Join(legacy, "qlog.db"), []byte("legacy evidence"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out := execute(t); !strings.Contains(out, "No imported usage") {
		t.Fatal(out)
	}
	if _, err := os.Stat(filepath.Join(legacy, "simple")); !os.IsNotExist(err) {
		t.Fatalf("empty report created data directory: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(legacy, "qlog.db"))
	if err != nil || string(contents) != "legacy evidence" {
		t.Fatal("legacy ledger changed")
	}
	execute(t, "--help")
	execute(t, "--version")
}

func TestDefaultImportReadsCodexHomeIntoSeparateLedger(t *testing.T) {
	legacy := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("QLOG_HOME", legacy)
	t.Setenv("CODEX_HOME", codexHome)
	if err := os.Mkdir(filepath.Join(codexHome, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "sessions", "session.jsonl"), []byte(nativeSession(time.Now())), 0o600); err != nil {
		t.Fatal(err)
	}
	execute(t, "import")
	if _, err := os.Stat(filepath.Join(legacy, "simple", "qlog.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(legacy, "qlog.db")); !os.IsNotExist(err) {
		t.Fatal("created legacy database")
	}
}

func TestDiscoverySelectsRecentNewestFilesAndBoundsImport(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	for i := 0; i < 5; i++ {
		file := filepath.Join(root, fmt.Sprintf("%d.jsonl", i))
		if err := os.WriteFile(file, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		modified := now.Add(time.Duration(-i) * 24 * time.Hour)
		if err := os.Chtimes(file, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
	files, candidates, err := discover(context.Background(), root, now.Add(-72*time.Hour), 2)
	if err != nil || candidates != 4 || len(files) != 2 || filepath.Base(files[0]) != "0.jsonl" || filepath.Base(files[1]) != "1.jsonl" {
		t.Fatalf("discovery: %v %d %v", files, candidates, err)
	}
	files, candidates, err = discover(context.Background(), filepath.Join(root, "4.jsonl"), now, 2)
	if err != nil || len(files) != 1 || candidates != 1 {
		t.Fatal("explicit older file was excluded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := discover(ctx, root, now, 2); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestReportTodayAndProjectFilters(t *testing.T) {
	home := t.TempDir()
	file := filepath.Join(t.TempDir(), "old.jsonl")
	if err := os.WriteFile(file, []byte(nativeSession(time.Now().AddDate(0, 0, -2))), 0o600); err != nil {
		t.Fatal(err)
	}
	execute(t, "--home", home, "import", file)
	for _, filter := range [][]string{{"--today"}, {"--project", "missing"}} {
		var summary Summary
		args := append([]string{"--home", home, "report", "--json"}, filter...)
		if err := json.Unmarshal([]byte(execute(t, args...)), &summary); err != nil {
			t.Fatal(err)
		}
		if summary.UsageSamples != 0 || summary.TotalTokens != 0 || len(summary.Rows) != 0 {
			t.Fatalf("filter ignored: %+v", summary)
		}
	}
}

func TestInvalidInputDoesNotInitialize(t *testing.T) {
	home := filepath.Join(t.TempDir(), "data")
	for _, args := range [][]string{{"setup"}, {"collector", "serve"}, {"mcp", "serve"}, {"import", "--limit", "0"}, {"import", "--days", "0"}, {"import", filepath.Join(t.TempDir(), "missing.jsonl")}, {"report", "unexpected"}, {"export", "--json"}} {
		var out, errOut bytes.Buffer
		if err := Run(context.Background(), append([]string{"--home", home}, args...), &out, &errOut, Version{}); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatal("invalid input initialized ledger")
	}
}
