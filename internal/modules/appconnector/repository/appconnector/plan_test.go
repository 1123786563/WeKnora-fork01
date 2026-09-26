package appconnector

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openPlanStoreDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&ActionPlanRow{}, &ActionPlanItemRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func planRow(id string) ActionPlanRow {
	return ActionPlanRow{
		ID: id, TenantID: 7, ActorID: "user-a",
		Digest: "digest-" + id, State: PlanStateAwaitingApproval,
	}
}

func planItems(tenant uint64, planID string, actionIDs ...string) []ActionPlanItemRow {
	items := make([]ActionPlanItemRow, 0, len(actionIDs))
	for i, a := range actionIDs {
		items = append(items, ActionPlanItemRow{TenantID: tenant, PlanID: planID, Seq: i + 1, ActionID: a})
	}
	return items
}

// TestPlanCreateFindRoundTrip: the plan + ordered items round-trip; a
// zero-item plan is structurally impossible; a cross-tenant lookup is
// indistinguishable from a missing one (existence never leaks).
func TestPlanCreateFindRoundTrip(t *testing.T) {
	db := openPlanStoreDB(t)
	store := NewPlanStore(db)
	ctx := context.Background()
	if err := store.CreatePlan(ctx, planRow("plan-1"), planItems(7, "plan-1", "act-1", "act-2")); err != nil {
		t.Fatal(err)
	}
	got, err := store.FindPlan(ctx, 7, "plan-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Digest != "digest-plan-1" || got.State != PlanStateAwaitingApproval || got.ActorID != "user-a" {
		t.Fatalf("round trip drift: %+v", got)
	}
	items, err := store.ListPlanItems(ctx, 7, "plan-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Seq != 1 || items[0].ActionID != "act-1" || items[1].Seq != 2 || items[1].ActionID != "act-2" {
		t.Fatalf("items must come back in seq order: %+v", items)
	}
	if err := store.CreatePlan(ctx, planRow("plan-2"), nil); !errors.Is(err, ErrPlanState) {
		t.Fatalf("zero-item plan must be refused, got %v", err)
	}
	if _, err := store.FindPlan(ctx, 8, "plan-1"); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("cross-tenant lookup must be not-found, got %v", err)
	}
	if _, err := store.FindPlan(ctx, 7, "plan-x"); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("missing plan must be not-found, got %v", err)
	}
}

// TestPlanApproveBindsDigestAndExclusions: the approval CAS binds the
// plan digest AND the state — a foreign digest moves nothing; the
// matching CAS records state/exclusions/approver/time and stays
// idempotent for the recovery re-approval path.
func TestPlanApproveBindsDigestAndExclusions(t *testing.T) {
	db := openPlanStoreDB(t)
	store := NewPlanStore(db)
	ctx := context.Background()
	if err := store.CreatePlan(ctx, planRow("plan-1"), planItems(7, "plan-1", "act-1", "act-2", "act-3")); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := store.ApprovePlan(ctx, 7, "plan-1", "digest-other", "boss", `[]`, now); !errors.Is(err, ErrPlanState) {
		t.Fatalf("approve with foreign digest must be refused, got %v", err)
	}
	got, _ := store.FindPlan(ctx, 7, "plan-1")
	if got.State != PlanStateAwaitingApproval {
		t.Fatalf("refused approve must not move state: %s", got.State)
	}
	if err := store.ApprovePlan(ctx, 7, "plan-1", "digest-plan-1", "boss", `[2]`, now); err != nil {
		t.Fatal(err)
	}
	got, _ = store.FindPlan(ctx, 7, "plan-1")
	if got.State != PlanStateAuthorized || got.ExcludedJSON != `[2]` || got.ApprovedBy != "boss" || got.ApprovedAt == nil {
		t.Fatalf("approval must record state/exclusions/approver/time: %+v", got)
	}
	// Re-approval (recovery path) stays legal from authorized.
	if err := store.ApprovePlan(ctx, 7, "plan-1", "digest-plan-1", "boss", `[2]`, now); err != nil {
		t.Fatalf("idempotent re-approve must stay legal, got %v", err)
	}
}
