// #102 / Lago 30 — the closure tombstone and its guards: the tombstone is
// one row per workspace, first caller wins, and from the instant it exists
// (either state) new charge actions are refused while other tenants keep
// reserving; the local subscription disposal caps the paid term at the
// closure instant so no later month is ever due.
package commercial

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
)

// TestClosureTombstoneIsSingleRowAndIdempotent pins the tombstone shape:
// RecordClosing lands exactly one row per tenant (replays read it back,
// never a second row), MarkClosed keeps the FIRST closed_at across
// replays, and the external identity is the deterministic derivation.
func TestClosureTombstoneIsSingleRowAndIdempotent(t *testing.T) {
	_, db := testBudgetStore(t)
	store := NewWorkspaceClosureStore(db)
	ctx := context.Background()

	row, err := store.RecordClosing(ctx, 7, domain.ExternalCustomerID(7))
	if err != nil {
		t.Fatal(err)
	}
	if row.State != WorkspaceClosureStateClosing || row.ExternalCustomerID != "weknora-tenant-7" {
		t.Fatalf("first record: %+v", row)
	}
	if err := store.MarkClosed(ctx, 7, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	first, err := store.Get(ctx, 7)
	if err != nil || first.State != WorkspaceClosureStateClosed || first.ClosedAt == nil {
		t.Fatalf("closed row: %v %+v", err, first)
	}

	later := time.Now().UTC().Add(time.Hour)
	if err := store.MarkClosed(ctx, 7, later); err != nil {
		t.Fatal(err)
	}
	again, err := store.Get(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !again.ClosedAt.Equal(*first.ClosedAt) {
		t.Fatalf("closed_at audit fact must survive replays: %v then %v", first.ClosedAt, again.ClosedAt)
	}

	replay, err := store.RecordClosing(ctx, 7, domain.ExternalCustomerID(7))
	if err != nil || replay.State != WorkspaceClosureStateClosed {
		t.Fatalf("record-after-close must read the closed row back: %v %+v", err, replay)
	}
	var count int64
	db.Table("commercial_workspace_closures").Where("tenant_id = ?", 7).Count(&count)
	if count != 1 {
		t.Fatalf("exactly one tombstone row per workspace, got %d", count)
	}
	if _, err := store.Get(ctx, 8); !errors.Is(err, ErrWorkspaceClosureNotFound) {
		t.Fatalf("unclosed tenant: want ErrWorkspaceClosureNotFound, got %v", err)
	}
}

// TestReserveDeniesClosedWorkspace pins AC1's execution half: the same
// workspace reserves freely, then the tombstone lands and every NEW
// reserve is refused with the closed sentinel — in state closing already,
// closed likewise — while the pre-closure hold stays untouched.
func TestReserveDeniesClosedWorkspace(t *testing.T) {
	s, db := testBudgetStore(t)
	seedLagBudget(t, db)
	// Production wiring order: the closure table exists from boot (the
	// closure store's constructor), BEFORE any reserve probes for it.
	closures := NewWorkspaceClosureStore(db)
	ctx := context.Background()

	if err := lagReserve(ctx, s, "r1", "res_open_before"); err != nil {
		t.Fatalf("open workspace reserve must flow: %v", err)
	}

	if _, err := closures.RecordClosing(ctx, 7, domain.ExternalCustomerID(7)); err != nil {
		t.Fatal(err)
	}
	if err := lagReserve(ctx, s, "r1", "res_closed_1"); !errors.Is(err, domain.ErrWorkspaceClosed) {
		t.Fatalf("closing workspace reserve: want ErrWorkspaceClosed, got %v", err)
	}
	if err := lagReserve(ctx, s, "r2", "res_closed_2"); !errors.Is(err, domain.ErrWorkspaceClosed) {
		t.Fatalf("closed workspace refuses every run, got %v", err)
	}

	// The pre-closure hold is untouched and replays identically.
	res, err := s.Reserve(ctx, domain.BudgetRequest{
		TenantID: 7, RunID: "r1", Key: "res_open_before", Upper: domain.Credits(100_000),
		Deadline: time.Now().UTC().Add(time.Hour),
	})
	if err != nil || res.ID != "res_open_before" {
		t.Fatalf("pre-closure hold must replay identically, got %v %+v", err, res)
	}
}

// TestRecordWorkspaceClosureCapsPaidTerm pins AC2's local disposal: the
// paid term is capped at the closure instant (DueMonths grants nothing
// past it), the closure reason is recorded exactly once, and the row is
// never deleted (financial history).
func TestRecordWorkspaceClosureCapsPaidTerm(t *testing.T) {
	_, db := testBudgetStore(t)
	subs := NewSubscriptionStore(db)
	if err := db.AutoMigrate(&Subscription{}); err != nil {
		t.Fatal(err)
	}
	anchor := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	paidUntil := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	if err := subs.SaveSubscription(context.Background(), &Subscription{
		ID: "sub-7", TenantID: 7, PlanKey: "pro", PlanVersion: 1,
		PlanSnapshotJSON: "{}", Anchor: anchor, PaidUntil: paidUntil,
	}); err != nil {
		t.Fatal(err)
	}

	closedAt := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	if err := subs.RecordWorkspaceClosure(context.Background(), 7, closedAt); err != nil {
		t.Fatal(err)
	}
	sub, err := subs.Current(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if !sub.PaidUntil.Equal(closedAt) {
		t.Fatalf("paid term must cap at closure, got %v", sub.PaidUntil)
	}
	if sub.DowngradeReason != domain.DowngradeReasonWorkspaceClosed {
		t.Fatalf("closure reason must be recorded, got %q", sub.DowngradeReason)
	}
	domSub := domain.Subscription{Anchor: sub.Anchor, PaidUntil: sub.PaidUntil}
	if got := domSub.DueMonths(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)); len(got) != 0 {
		t.Fatalf("no month past closure may be due, got %v", got)
	}

	// Replay keeps the audit fact (reason written once) and stays idempotent.
	if err := subs.RecordWorkspaceClosure(context.Background(), 7, closedAt.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	again, _ := subs.Current(context.Background(), 7)
	if !again.PaidUntil.Equal(closedAt) {
		t.Fatalf("earlier cap must win across replays, got %v", again.PaidUntil)
	}
	var rows int64
	db.Model(&Subscription{}).Where("tenant_id = ?", 7).Count(&rows)
	if rows != 1 {
		t.Fatalf("closure never deletes the subscription row, got %d rows", rows)
	}
}
