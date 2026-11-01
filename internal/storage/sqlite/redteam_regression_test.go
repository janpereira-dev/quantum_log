package sqlite

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReservedURICharactersKeepDistinctDatabasesInSelectedHomes(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	for _, name := range []string{"hash#first", "hash#second", "percent%25home"} {
		database := filepath.Join(root, name, "qlog.db")
		store, err := Open(ctx, database)
		if err != nil {
			t.Fatalf("Open(%q): %v", name, err)
		}
		if _, err := store.RecordModelCall(ctx, ModelCallInput{Provider: "synthetic", ModelID: name, InputTokens: 15}); err != nil {
			_ = store.Close()
			t.Fatal(err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(database); err != nil {
			t.Fatalf("selected database %q missing: %v", name, err)
		}
		for _, open := range []struct {
			name string
			fn   func(context.Context, string) (*Store, error)
		}{{"immutable", OpenReadOnly}, {"snapshot", OpenSnapshotReadOnly}} {
			reader, err := open.fn(ctx, database)
			if err != nil {
				t.Fatalf("%s(%q): %v", open.name, name, err)
			}
			usage, err := reader.Usage(ctx, UsageQuery{GroupBy: []string{"project"}})
			_ = reader.Close()
			if err != nil || usage.TotalTokens != 15 {
				t.Fatalf("%s(%q) usage=%#v err=%v", open.name, name, usage, err)
			}
		}
		if err := Checkpoint(ctx, database); err != nil {
			t.Fatalf("Checkpoint(%q): %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "hash")); !os.IsNotExist(err) {
		t.Fatalf("unexpected redirected sibling: %v", err)
	}
}

func TestMisroutedExistingDatabaseIsRetainedAndRequiresExplicitRecovery(t *testing.T) {
	root := t.TempDir()
	redirected := filepath.Join(root, "hash")
	original := []byte("synthetic original data that must not be replaced")
	if err := os.WriteFile(redirected, original, 0600); err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(root, "hash#home", "qlog.db")
	if store, err := Open(context.Background(), database); err == nil {
		_ = store.Close()
		t.Fatal("expected explicit legacy-location recovery error")
	} else if !strings.Contains(strings.ToLower(err.Error()), "legacy") {
		t.Fatalf("missing legacy recovery guidance: %v", err)
	}
	retained, err := os.ReadFile(redirected)
	if err != nil || string(retained) != string(original) {
		t.Fatalf("existing misplaced file changed: %q %v", retained, err)
	}
	if _, err := os.Stat(database); !os.IsNotExist(err) {
		t.Fatalf("unexpected replacement ledger: %v", err)
	}
}

func TestApportionShareDoesNotOverflowForRepresentableUsage(t *testing.T) {
	for _, value := range []int64{1000000000000000, math.MaxInt64} {
		if got := apportionShare(value, 0, 10000); got != value {
			t.Errorf("full allocation(%d)=%d", value, got)
		}
		first, last := apportionShare(value, 0, 3333), apportionShare(value, 3333, 6667)
		if first < 0 || last < 0 || first+last != value {
			t.Errorf("split allocation(%d)=%d+%d", value, first, last)
		}
	}
}

func TestRecordModelCallRejectsUnrepresentableAggregateWithoutPoisoningLedger(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.RecordModelCall(ctx, ModelCallInput{Provider: "synthetic", ModelID: "model", InputTokens: math.MaxInt64}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordModelCall(ctx, ModelCallInput{Provider: "synthetic", ModelID: "model", InputTokens: 1}); err == nil {
		t.Fatal("expected aggregate overflow rejection")
	}
	var count int
	if err := store.db.QueryRow("SELECT COUNT(*) FROM model_calls").Scan(&count); err != nil || count != 1 {
		t.Fatalf("accepted rows=%d err=%v", count, err)
	}
	usage, err := store.Usage(ctx, UsageQuery{GroupBy: []string{"project"}})
	if err != nil || usage.TotalTokens != math.MaxInt64 || len(usage.Rows) != 1 || usage.Rows[0].TotalTokens != math.MaxInt64 {
		t.Fatalf("ledger poisoned: %#v err=%v", usage, err)
	}
}

func TestRecordModelCallRejectsNegativeAndOverflowingComponents(t *testing.T) {
	for name, input := range map[string]ModelCallInput{
		"negative":     {Provider: "synthetic", ModelID: "model", InputTokens: -1},
		"sum_overflow": {Provider: "synthetic", ModelID: "model", InputTokens: math.MaxInt64, OutputTokens: 1},
	} {
		t.Run(name, func(t *testing.T) {
			store, err := Open(context.Background(), filepath.Join(t.TempDir(), "qlog.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			if _, err := store.RecordModelCall(context.Background(), input); err == nil {
				t.Fatal("invalid components accepted")
			}
		})
	}
}

func TestUsageCapacitySurvivesReopenAndDuplicateAtCapacity(t *testing.T) {
	ctx := context.Background()
	database := filepath.Join(t.TempDir(), "qlog.db")
	store, err := Open(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := store.AppendRawEvent(ctx, RawEventInput{Source: "synthetic", EventType: "model.call", Payload: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	input := ModelCallInput{RawEventID: raw.ID, Provider: "synthetic", ModelID: "model", InputTokens: math.MaxInt64}
	original, err := store.RecordModelCall(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if replay, err := store.RecordModelCall(ctx, input); err != nil || replay != original {
		t.Fatalf("duplicate at capacity=%q %v", replay, err)
	}
	if _, err := store.RecordModelCall(ctx, ModelCallInput{Provider: "synthetic", ModelID: "model", InputTokens: 1}); err == nil {
		t.Fatal("reopened writer lost aggregate capacity")
	}
}

func TestReportRejectsExistingInvalidUsageWithoutRewritingEvidence(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	id, err := store.RecordModelCall(ctx, ModelCallInput{Provider: "synthetic", ModelID: "model", InputTokens: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE model_calls SET input_tokens=-1,total_tokens=-1 WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Usage(ctx, UsageQuery{GroupBy: []string{"project"}}); err == nil {
		t.Fatal("invalid legacy usage was reported as normal")
	}
	var preserved int64
	if err := store.db.QueryRow(`SELECT total_tokens FROM model_calls WHERE id=?`, id).Scan(&preserved); err != nil || preserved != -1 {
		t.Fatalf("legacy evidence rewritten: %d %v", preserved, err)
	}
}
