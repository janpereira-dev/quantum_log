package codexsession

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janpereira-dev/quantum_log/internal/app"
	storepkg "github.com/janpereira-dev/quantum_log/internal/storage/sqlite"
)

const header = `{"type":"session_meta","payload":{"id":"session-1","cli_version":"0.1.0","model_provider":"openai"}}` + "\n" + `{"type":"turn_context","payload":{"model":"example-model"}}` + "\n"

func sample(second int, counters string) string {
	return fmt.Sprintf(`{"type":"event_msg","timestamp":"2026-10-13T09:00:%02dZ","payload":{"type":"token_count","info":{"total_token_usage":%s}}}`+"\n", second, counters)
}

func fixture(t *testing.T) (*app.Service, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "qlog.db")
	store, err := storepkg.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return &app.Service{Store: store}, path
}

func importLog(t *testing.T, service *app.Service, input string) Result {
	t.Helper()
	result, err := Import(context.Background(), service, strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func usage(t *testing.T, service *app.Service) storepkg.UsageReport {
	t.Helper()
	report, err := service.Store.Usage(context.Background(), storepkg.UsageQuery{GroupBy: []string{"project", "agent", "model"}})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func childHeader(ancestry, created string) string {
	return fmt.Sprintf(`{"type":"session_meta","payload":{"id":"child-session","model_provider":"openai","timestamp":%q,%s}}`+"\n"+`{"type":"turn_context","payload":{"model":"example-model"}}`+"\n", created, ancestry)
}

func childSample(second int, total, last int64) string {
	return fmt.Sprintf(`{"type":"event_msg","timestamp":"2026-10-13T09:00:%02dZ","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":%d},"last_token_usage":{"total_tokens":%d}}}}`+"\n", second, total, last)
}

func TestSpawnedAndForkedSessionsExcludeInheritedUsage(t *testing.T) {
	for _, ancestry := range []string{
		`"parent_thread_id":"session-1"`,
		`"source":{"subagent":{"thread_spawn":{"parent_thread_id":"session-1"}}}`,
		`"forked_from_id":"session-1"`,
	} {
		t.Run(ancestry, func(t *testing.T) {
			service, _ := fixture(t)
			prefix := sample(1, `{"total_tokens":100}`) + sample(2, `{"total_tokens":150}`)
			parent := header + prefix
			child := childHeader(ancestry, "2026-10-13T09:00:05Z") + prefix + childSample(6, 170, 20) + childSample(7, 185, 15)
			importLog(t, service, parent)
			result := importLog(t, service, child)
			if result.Samples != 2 || result.Skipped != 2 || usage(t, service).TotalTokens != 185 {
				t.Fatalf("inherited prefix counted: %+v %d", result, usage(t, service).TotalTokens)
			}
			if replay := importLog(t, service, child); replay.Samples != 0 || replay.Duplicates != 2 || usage(t, service).TotalTokens != 185 {
				t.Fatalf("child replay counted: %+v", replay)
			}
		})
	}
}

func TestChildCounterResetAndRepeatedParentSnapshot(t *testing.T) {
	service, _ := fixture(t)
	prefix := sample(1, `{"total_tokens":100}`)
	importLog(t, service, header+prefix)
	child := childHeader(`"parent_thread_id":"session-1"`, "2026-10-13T09:00:05Z") + prefix + childSample(6, 100, 100) + childSample(7, 20, 20) + childSample(8, 40, 20)
	result := importLog(t, service, child)
	if result.Samples != 2 || result.Skipped != 2 || usage(t, service).TotalTokens != 140 {
		t.Fatalf("child reset/prefix: %+v %d", result, usage(t, service).TotalTokens)
	}
}

func TestReplayedParentMetadataCannotReplaceChildIdentity(t *testing.T) {
	service, path := fixture(t)
	prefix := sample(1, `{"total_tokens":100}`)
	parentContext := `{"type":"turn_context","timestamp":"2026-10-13T09:00:02Z","payload":{"model":"parent-model"}}` + "\n"
	child := childHeader(`"parent_thread_id":"session-1"`, "2026-10-13T09:00:05Z") + header + parentContext + prefix + childSample(6, 20, 20) + childSample(7, 40, 20)
	result := importLog(t, service, child)
	if result.Samples != 2 || result.Skipped != 1 || usage(t, service).TotalTokens != 40 {
		t.Fatalf("replayed parent became owner: %+v", result)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	}()
	var childRows, parentRows int
	if err := db.QueryRow("SELECT COUNT(*) FROM model_calls WHERE session_id='child-session'").Scan(&childRows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM model_calls WHERE session_id='session-1'").Scan(&parentRows); err != nil {
		t.Fatal(err)
	}
	if childRows != 2 || parentRows != 0 {
		t.Fatalf("wrong owners: child=%d parent=%d", childRows, parentRows)
	}
	if report := usage(t, service); len(report.Rows) != 1 || report.Rows[0].Model != "example-model" {
		t.Fatalf("parent context replaced child model: %+v", report.Rows)
	}
	if replay := importLog(t, service, child); replay.Duplicates != 2 || replay.Samples != 0 {
		t.Fatalf("replay: %+v", replay)
	}
}

func TestAmbiguousChildHistoryIsNotInvented(t *testing.T) {
	for _, created := range []string{"", "invalid"} {
		service, _ := fixture(t)
		child := childHeader(`"parent_thread_id":"session-1"`, created) + childSample(6, 20, 20)
		result := importLog(t, service, child)
		if result.Samples != 0 || result.Skipped != 1 || usage(t, service).TotalTokens != 0 {
			t.Fatalf("unknown boundary counted: %+v", result)
		}
	}
	service, _ := fixture(t)
	child := childHeader(`"parent_thread_id":"session-1"`, "2026-10-13T09:00:05Z") + sample(6, `{"total_tokens":170}`) + childSample(7, 200, 30)
	result := importLog(t, service, child)
	if result.Samples != 1 || result.Skipped != 1 || usage(t, service).TotalTokens != 30 {
		t.Fatalf("unknown starting total invented: %+v", result)
	}
}

func TestReplayCopyAndGrowingLog(t *testing.T) {
	service, _ := fixture(t)
	first := header + sample(1, `{"input_tokens":100,"cached_input_tokens":40,"output_tokens":20,"reasoning_output_tokens":5,"total_tokens":120}`)
	if result := importLog(t, service, first); result.Samples != 1 {
		t.Fatalf("first import = %#v", result)
	}
	// Identity does not include source path; a copied log has the same result as replay.
	if result := importLog(t, service, first); result.Duplicates != 1 || result.Samples != 0 {
		t.Fatalf("replayed copy = %#v", result)
	}
	growing := first + sample(2, `{"input_tokens":100,"cached_input_tokens":40,"output_tokens":20,"reasoning_output_tokens":5,"total_tokens":120}`) + sample(3, `{"input_tokens":140,"cached_input_tokens":50,"output_tokens":30,"reasoning_output_tokens":8,"total_tokens":170}`)
	if result := importLog(t, service, growing); result.Samples != 1 || result.Duplicates != 1 || result.Skipped != 1 {
		t.Fatalf("growing import = %#v", result)
	}
	report := usage(t, service)
	if report.TotalTokens != 170 {
		t.Fatalf("subset counts inflated cumulative total: %#v", report)
	}
	if err := service.Store.VerifyLedger(context.Background(), "session-1"); err != nil {
		t.Fatal(err)
	}
}

func TestMissingCountersZeroAndReset(t *testing.T) {
	service, _ := fixture(t)
	input := header + sample(1, `{"input_tokens":12}`) + sample(2, `{"total_tokens":0}`) + sample(3, `{"total_tokens":100}`) + sample(4, `{"total_tokens":20}`) + sample(5, `{"total_tokens":30}`)
	result := importLog(t, service, input)
	if result.Samples != 3 || result.Skipped != 2 {
		t.Fatalf("missing / reset result = %#v", result)
	}
	if report := usage(t, service); report.TotalTokens != 110 {
		t.Fatalf("reset must skip snapshot and rebase next delta: %#v", report)
	}
	capability, err := service.Store.CapabilityReport(context.Background(), storepkg.CapabilityQuery{})
	if err != nil {
		t.Fatal(err)
	}
	for _, metric := range capability.MetricCoverage {
		if metric.Name == "input_tokens" && metric.ReportedCount != 0 {
			t.Fatalf("missing input manufactured: %#v", metric)
		}
		if metric.Name == "total_tokens" && (metric.ReportedCount != 3 || metric.ReportedZeroCount != 1) {
			t.Fatalf("source zero not preserved: %#v", metric)
		}
	}
}

func TestPrivateContentNeverPersists(t *testing.T) {
	service, path := fixture(t)
	secret := "PRIVATE-PROMPT-AND-API-SECRET"
	input := strings.Replace(header, `"cli_version":"0.1.0"`, `"cli_version":"0.1.0","instructions":"`+secret+`","environment":{"API_KEY":"`+secret+`"}`, 1)
	input += `{"type":"response_item","payload":{"content":"` + secret + `","arguments":"` + secret + `"}}` + "\n"
	input += strings.Replace(sample(1, `{"total_tokens":12}`), `"info":`, `"prompt":"`+secret+`","info":`, 1)
	importLog(t, service, input)
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	}()
	var payload, evidence string
	if err := database.QueryRow(`SELECT payload_json_sanitized, project_resolution_evidence_json FROM raw_events LIMIT 1`).Scan(&payload, &evidence); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload+evidence, secret) || strings.Contains(payload, "prompt") || strings.Contains(payload, "arguments") {
		t.Fatal("private content persisted")
	}
	var persisted map[string]any
	if err := json.Unmarshal([]byte(payload), &persisted); err != nil || len(persisted) != 4 {
		t.Fatalf("raw evidence whitelist = %s, %v", payload, err)
	}
}

func TestMissingMetadataAndActiveTrailingLine(t *testing.T) {
	service, _ := fixture(t)
	if result := importLog(t, service, sample(1, `{"total_tokens":12}`)); result.Samples != 0 || result.Skipped != 1 {
		t.Fatalf("missing session identity = %#v", result)
	}
	input := `{"type":"session_meta","payload":{"id":"session-2"}}` + "\n" + sample(1, `{"total_tokens":12}`) + `{"type":"event_msg","payload":`
	if result := importLog(t, service, input); result.Samples != 1 || result.Skipped != 1 {
		t.Fatalf("active final line = %#v", result)
	}
	report := usage(t, service)
	if len(report.Rows) != 1 || report.Rows[0].Model != "unavailable" {
		t.Fatalf("missing model must remain unavailable: %#v", report)
	}
	if _, err := Import(context.Background(), service, strings.NewReader(header+"{bad}\n")); err == nil {
		t.Fatal("malformed complete line accepted")
	}
	if _, err := Import(context.Background(), service, strings.NewReader(header+sample(1, `{"total_tokens":"secret"}`))); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("malformed usage must fail without leaking contents: %v", err)
	}
}

func TestFinishedTrailingLineAddsOnlyNewUsage(t *testing.T) {
	service, _ := fixture(t)
	first := header + sample(1, `{"total_tokens":10}`)
	next := sample(2, `{"total_tokens":25}`)
	partial := first + next[:len(next)/2]
	if result := importLog(t, service, partial); result.Samples != 1 || result.Skipped != 1 {
		t.Fatalf("partial import = %#v", result)
	}
	if result := importLog(t, service, first+next); result.Samples != 1 || result.Duplicates != 1 {
		t.Fatalf("finished import = %#v", result)
	}
	if report := usage(t, service); report.TotalTokens != 25 {
		t.Fatalf("finished tail duplicated old usage: %#v", report)
	}
}

func TestNewlyAvailableCounterIsNotManufactured(t *testing.T) {
	service, _ := fixture(t)
	importLog(t, service, header+sample(1, `{"total_tokens":10}`)+sample(2, `{"total_tokens":20,"input_tokens":15}`)+sample(3, `{"total_tokens":30,"input_tokens":20}`))
	capability, err := service.Store.CapabilityReport(context.Background(), storepkg.CapabilityQuery{})
	if err != nil {
		t.Fatal(err)
	}
	for _, metric := range capability.MetricCoverage {
		if metric.Name == "input_tokens" && metric.ReportedCount != 1 {
			t.Fatalf("counter without baseline was fabricated: %#v", metric)
		}
	}
}

func TestMissingCWDDoesNotUseImporterProject(t *testing.T) {
	t.Setenv("QLOG_PROJECT", "ambient-must-not-apply")
	service, _ := fixture(t)
	importLog(t, service, header+sample(1, `{"total_tokens":15}`))
	capability, err := service.Store.CapabilityReport(context.Background(), storepkg.CapabilityQuery{})
	if err != nil || capability.UnattributedTokens != 15 || capability.UnattributedModelCalls != 1 {
		t.Fatalf("missing source cwd must remain unattributed: %#v, %v", capability, err)
	}
}

func TestHistoricalGitAttributionIgnoresAmbientProject(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	t.Setenv("QLOG_PROJECT", "ambient-must-not-apply")
	directory := t.TempDir()
	if output, err := exec.Command("git", "init", directory).CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", output, err)
	}
	service, _ := fixture(t)
	cwd, _ := json.Marshal(directory)
	input := strings.Replace(header, `"id":"session-1"`, `"id":"session-1","cwd":`+string(cwd), 1) + sample(1, `{"total_tokens":10}`) + sample(2, `{"total_tokens":15}`)
	if result := importLog(t, service, input); result.Samples != 2 {
		t.Fatalf("git attributed import = %#v", result)
	}
	capability, err := service.Store.CapabilityReport(context.Background(), storepkg.CapabilityQuery{})
	if err != nil || capability.UnattributedTokens != 0 {
		t.Fatalf("source Git project unresolved: %#v, %v", capability, err)
	}
}

func TestBatchReusesSourceProjectAndResetsFileBaseline(t *testing.T) {
	service, _ := fixture(t)
	directory := t.TempDir()
	if _, _, err := service.Store.RegisterProject(context.Background(), "Source", "source", directory); err != nil {
		t.Fatal(err)
	}
	cwd, _ := json.Marshal(directory)
	log := strings.Replace(header, `"id":"session-1"`, `"id":"session-1","cwd":`+string(cwd), 1) + sample(1, `{"total_tokens":10}`)
	batch := New(service)
	for _, input := range []string{log, strings.Replace(log, "session-1", "session-2", 1)} {
		if result, err := batch.Import(context.Background(), strings.NewReader(input)); err != nil || result.Samples != 1 {
			t.Fatalf("batch import = %#v, %v", result, err)
		}
	}
	if len(batch.projects) != 1 {
		t.Fatalf("same source cwd must share one batch attribution, got %d", len(batch.projects))
	}
	if report := usage(t, service); report.TotalTokens != 20 || len(report.Rows) != 1 || report.Rows[0].ProjectSlug != "source" {
		t.Fatalf("batch must retain attribution and independent file baselines: %#v", report)
	}
}

func TestReplayRecoversInterruptedModelWrite(t *testing.T) {
	service, path := fixture(t)
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	}()
	if _, err := database.Exec(`CREATE TRIGGER fail_model_write BEFORE INSERT ON model_calls BEGIN SELECT RAISE(ABORT, 'simulated interruption'); END`); err != nil {
		t.Fatal(err)
	}
	log := header + sample(1, `{"total_tokens":12}`)
	if _, err := Import(context.Background(), service, strings.NewReader(log)); err == nil {
		t.Fatal("simulated normalized write failure was ignored")
	}
	if _, err := database.Exec(`DROP TRIGGER fail_model_write`); err != nil {
		t.Fatal(err)
	}
	if result := importLog(t, service, log); result.Duplicates != 1 || result.Samples != 0 {
		t.Fatalf("interrupted raw event replay = %#v", result)
	}
	if report := usage(t, service); report.TotalTokens != 12 {
		t.Fatalf("missing normalized row not recovered: %#v", report)
	}
	if result := importLog(t, service, log); result.Duplicates != 1 || usage(t, service).TotalTokens != 12 {
		t.Fatalf("linked duplicate counted twice: %#v", result)
	}
}

func TestBoundedLinesAndCancellation(t *testing.T) {
	service, _ := fixture(t)
	if _, err := Import(context.Background(), service, strings.NewReader(strings.Repeat("x", maxLineBytes+1))); err == nil {
		t.Fatal("unbounded source line accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Import(ctx, service, strings.NewReader(header)); err != context.Canceled {
		t.Fatalf("cancellation = %v", err)
	}
}

func BenchmarkImport100Samples(b *testing.B) {
	var input strings.Builder
	input.WriteString(header)
	for index := 1; index <= 100; index++ {
		input.WriteString(sample(index/2, fmt.Sprintf(`{"input_tokens":%d,"cached_input_tokens":%d,"output_tokens":%d,"reasoning_output_tokens":%d,"total_tokens":%d}`, index*100, index*20, index*50, index*10, index*150)))
	}
	log := input.String()
	for _, mode := range []string{"Append", "Replay"} {
		b.Run(mode, func(b *testing.B) {
			ctx := context.Background()
			store, err := storepkg.Open(ctx, filepath.Join(b.TempDir(), "qlog.db"))
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { _ = store.Close() })
			service := &app.Service{Store: store}
			batch := New(service)
			if mode == "Replay" {
				if _, err := batch.Import(ctx, strings.NewReader(log)); err != nil {
					b.Fatal(err)
				}
			}
			b.SetBytes(int64(len(log)))
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				current := log
				if mode == "Append" {
					b.StopTimer()
					current = strings.Replace(log, `"id":"session-1"`, fmt.Sprintf(`"id":"benchmark-%d"`, index), 1)
					b.StartTimer()
				}
				if _, err := batch.Import(ctx, strings.NewReader(current)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
