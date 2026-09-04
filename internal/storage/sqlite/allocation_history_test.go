package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func allocationFixture(t *testing.T) (*Store, string, string, string) {
	t.Helper()
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "qlog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	a, _, err := s.RegisterProject(ctx, "A", "a", filepath.Join(t.TempDir(), "a"))
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := s.RegisterProject(ctx, "B", "b", filepath.Join(t.TempDir(), "b"))
	if err != nil {
		t.Fatal(err)
	}
	call, err := s.RecordModelCall(ctx, ModelCallInput{ProjectID: a.ID, Provider: "test", ModelID: "test"})
	if err != nil {
		t.Fatal(err)
	}
	return s, call, a.ID, b.ID
}

func TestAllocationRevisionHistoryIsAppendOnly(t *testing.T) {
	ctx := context.Background()
	s, call, a, b := allocationFixture(t)
	first, err := s.AppendAllocationRevision(ctx, AllocationRevisionInput{SubjectType: "model_call", SubjectID: call, Allocations: []AllocationInput{{ProjectID: a, BasisPoints: 10000}}, IdempotencyKey: "first", Source: "test", Reason: "assign"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.AppendAllocationRevision(ctx, AllocationRevisionInput{SubjectType: "model_call", SubjectID: call, Allocations: []AllocationInput{{ProjectID: a, BasisPoints: 6000}, {ProjectID: b, BasisPoints: 4000}}, IdempotencyKey: "second", Source: "test", Reason: "correct"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || second.ParentRevisionID != first.ID {
		t.Fatalf("revision chain = %#v -> %#v", first, second)
	}
	history, err := s.AllocationHistory(ctx, "model_call", call)
	if err != nil || len(history) != 3 {
		t.Fatalf("history = %#v, %v", history, err)
	}
	if history[0].IdempotencyKey != "direct:"+call || len(history[0].Allocations) != 1 || history[0].Allocations[0].BasisPoints != 10000 {
		t.Fatalf("first revision mutated: %#v", history[0])
	}
}

func TestAllocationRevertAppendsRevision(t *testing.T) {
	ctx := context.Background()
	s, call, a, b := allocationFixture(t)
	first, _ := s.AppendAllocationRevision(ctx, AllocationRevisionInput{SubjectType: "model_call", SubjectID: call, Allocations: []AllocationInput{{ProjectID: a, BasisPoints: 10000}}, IdempotencyKey: "first"})
	second, err := s.AppendAllocationRevision(ctx, AllocationRevisionInput{SubjectType: "model_call", SubjectID: call, Allocations: []AllocationInput{{ProjectID: b, BasisPoints: 10000}}, IdempotencyKey: "second"})
	if err != nil {
		t.Fatal(err)
	}
	reverted, err := s.RevertAllocationRevision(ctx, second.ID, "revert", "restore prior assignment")
	if err != nil {
		t.Fatal(err)
	}
	if reverted.ParentRevisionID != second.ID || reverted.Allocations[0].ProjectID != a {
		t.Fatalf("revert = %#v, want parent %s and project %s", reverted, second.ID, a)
	}
	got, err := s.ModelCallAllocations(ctx, call)
	if err != nil || len(got) != 1 || got[0].ProjectID != a {
		t.Fatalf("projection after revert = %#v, %v", got, err)
	}
	_ = first
}

func TestAllocationRevisionIdempotency(t *testing.T) {
	ctx := context.Background()
	s, call, a, _ := allocationFixture(t)
	one, err := s.AppendAllocationRevision(ctx, AllocationRevisionInput{SubjectType: "model_call", SubjectID: call, Allocations: []AllocationInput{{ProjectID: a, BasisPoints: 10000}}, IdempotencyKey: "same"})
	if err != nil {
		t.Fatal(err)
	}
	two, err := s.AppendAllocationRevision(ctx, AllocationRevisionInput{SubjectType: "model_call", SubjectID: call, Allocations: []AllocationInput{{ProjectID: a, BasisPoints: 10000}}, IdempotencyKey: "same"})
	if err != nil {
		t.Fatal(err)
	}
	if one.ID != two.ID {
		t.Fatalf("replay IDs = %s and %s", one.ID, two.ID)
	}
	h, err := s.AllocationHistory(ctx, "model_call", call)
	if err != nil || len(h) != 2 {
		t.Fatalf("history after replay = %#v, %v", h, err)
	}
}

func TestAllocationProjectionRebuild(t *testing.T) {
	ctx := context.Background()
	s, call, a, b := allocationFixture(t)
	_, err := s.AppendAllocationRevision(ctx, AllocationRevisionInput{SubjectType: "model_call", SubjectID: call, Allocations: []AllocationInput{{ProjectID: a, BasisPoints: 5000}, {ProjectID: b, BasisPoints: 5000}}, IdempotencyKey: "split"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM usage_allocations`); err != nil {
		t.Fatal(err)
	}
	if err := s.RebuildAllocationProjection(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := s.ModelCallAllocations(ctx, call)
	if err != nil || len(got) != 2 {
		t.Fatalf("rebuilt projection = %#v, %v", got, err)
	}
}

func TestVerifyLedgerDetectsTamperedAllocationRevision(t *testing.T) {
	ctx := context.Background()
	s, call, a, _ := allocationFixture(t)
	r, err := s.AppendAllocationRevision(ctx, AllocationRevisionInput{SubjectType: "model_call", SubjectID: call, Allocations: []AllocationInput{{ProjectID: a, BasisPoints: 10000}}, IdempotencyKey: "integrity"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.VerifyLedger(ctx, ""); err != nil {
		t.Fatalf("baseline verification: %v", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE allocation_revisions SET reason = 'tampered' WHERE revision_id = ?`, r.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifyLedger(ctx, ""); err == nil {
		t.Fatal("VerifyLedger accepted tampered allocation revision")
	}
}

func TestVerifyLedgerDetectsDeletedAllocationRevision(t *testing.T) {
	ctx := context.Background()
	s, call, a, b := allocationFixture(t)
	first, err := s.AppendAllocationRevision(ctx, AllocationRevisionInput{SubjectType: "model_call", SubjectID: call, Allocations: []AllocationInput{{ProjectID: a, BasisPoints: 10000}}, IdempotencyKey: "first"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendAllocationRevision(ctx, AllocationRevisionInput{SubjectType: "model_call", SubjectID: call, Allocations: []AllocationInput{{ProjectID: b, BasisPoints: 10000}}, IdempotencyKey: "second"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM allocation_revisions WHERE revision_id = ?`, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifyLedger(ctx, ""); err == nil {
		t.Fatal("VerifyLedger accepted deleted allocation revision")
	}
}
