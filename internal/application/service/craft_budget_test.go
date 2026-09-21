package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// craftBudgetPolicy is the explicit deployment-owned admission policy used by
// the contract tests: every money-bearing number (per-call hold, task limit)
// arrives from configuration — the craft adapter never invents rates.
func craftBudgetPolicy() CraftBudgetPolicy {
	return CraftBudgetPolicy{
		GrantWindow: time.Hour,
		MaxCalls:    5,
		CallUpper:   commercial.Credits(500),
		TaskLimit:   commercial.Credits(5000),
	}
}

// openCraftBudgetTestDB applies the full migration chain (craft tables
// included) and then aligns the G4 commercial rows with their gorm models the
// same way G4's own service fixtures do (budgetTestEnv AutoMigrates them):
// the shipped SQL migrations for commercial_reservations predate the U04
// lease-takeover `owner` column, so without this alignment every
// Reserve insert would fail and surface as budget_contention. Recorded as a
// G4 gap in the O02 report: the commercial migration chain must catch up
// before charging deployments run on it.
func openCraftBudgetTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openCraftUsageServiceDB(t)
	if pool, err := db.DB(); err == nil {
		pool.SetMaxOpenConns(1) // serialize SQLite writers; races stay logical
	}
	require.NoError(t, db.AutoMigrate(
		&repocommercial.BudgetAccountRow{}, &repocommercial.TaskBudgetRow{},
		&repocommercial.ReservationRow{}, &repocommercial.BudgetLotRow{},
		&repocommercial.BudgetLotAllocationRow{}, &repocommercial.TaskBudgetExtensionRow{},
		&commercialsvc.SettlementRecord{},
	))
	return db
}

// craftBudgetEnv wires the real G4 surfaces (budget store + U04 budget
// service) behind the craft budget service over the real migration chain
// (000044_craft_budget included). The clock is injectable for expiry cases.
func craftBudgetEnv(t *testing.T) (*CraftBudgetService, *gorm.DB, *atomic.Value) {
	t.Helper()
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	now := &atomic.Value{}
	now.Store(time.Now().UTC())
	svc.now = func() time.Time { return now.Load().(time.Time) }
	return svc, db, now
}

// seedCraftFundedTenant funds a tenant exactly like the commercial budget
// tests do: a verified balance projection plus one credits lot. Task budgets
// are NOT seeded here — Admit registers them through G4's EnsureTaskBudget.
func seedCraftFundedTenant(t *testing.T, db *gorm.DB, tenant uint64, verified int64) {
	t.Helper()
	end := time.Now().UTC().Add(2 * time.Hour)
	require.NoError(t, db.Create(&repocommercial.BudgetAccountRow{
		TenantID: tenant, VerifiedMicro: verified, Watermark: "w0", Version: 1, VerifiedUntil: end,
	}).Error)
	require.NoError(t, db.Create(&repocommercial.BudgetLotRow{
		TenantID: tenant, LotID: fmt.Sprintf("lot-%d", tenant), RemainingMicro: verified, IssuedAt: time.Now().UTC(),
	}).Error)
}

func craftGrantCalls(t *testing.T, db *gorm.DB, tenant uint64, grantID string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&CraftBudgetCallRow{}).
		Where("tenant_id = ? AND grant_id = ?", tenant, grantID).Count(&n).Error)
	return n
}

func craftReservations(t *testing.T, db *gorm.DB, tenant uint64) []repocommercial.ReservationRow {
	t.Helper()
	var rows []repocommercial.ReservationRow
	require.NoError(t, db.Where("tenant_id = ?", tenant).Find(&rows).Error)
	return rows
}

func craftAccountRow(t *testing.T, db *gorm.DB, tenant uint64) repocommercial.BudgetAccountRow {
	t.Helper()
	var acct repocommercial.BudgetAccountRow
	require.NoError(t, db.Where("tenant_id = ?", tenant).First(&acct).Error)
	return acct
}

func domainReservationStateDispatched() string { return commercial.ReservationStateDispatched }
func domainReservationStateReleased() string   { return commercial.ReservationStateReleased }

// TestCraftBudgetAdmitRegistersGrantAndCommercialTaskBudget is the admission
// contract: Admit issues a durable grant AND registers the G4 task budget
// with the configured limit/deadline; re-admission is idempotent; a revoked
// grant never resurrects.
func TestCraftBudgetAdmitRegistersGrantAndCommercialTaskBudget(t *testing.T) {
	svc, db, _ := craftBudgetEnv(t)
	ctx := context.Background()
	seedCraftFundedTenant(t, db, 7, 5000)

	scope := craft.Scope{TenantID: 7, UserID: "u1", SessionID: "s1"}
	g, err := svc.Admit(ctx, scope, "run-1")
	require.NoError(t, err)
	require.True(t, g.Allowed)
	require.NotEmpty(t, g.ID)
	require.Equal(t, 5, g.MaxCalls)
	require.Equal(t, 0, g.UsedCalls)
	require.True(t, g.Deadline.After(time.Now().UTC()))

	var task repocommercial.TaskBudgetRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", uint64(7), "run-1").First(&task).Error)
	require.Equal(t, int64(5000), task.LimitMicro)
	require.Equal(t, g.Deadline.Unix(), task.Deadline.Unix())

	again, err := svc.Admit(ctx, scope, "run-1")
	require.NoError(t, err)
	require.Equal(t, g.ID, again.ID)

	// Revoke is durable and idempotent; re-admission never resurrects.
	require.NoError(t, svc.RevokeGrant(ctx, g.ID))
	require.NoError(t, svc.RevokeGrant(ctx, g.ID))
	after, err := svc.Admit(ctx, scope, "run-1")
	require.NoError(t, err)
	require.Equal(t, g.ID, after.ID)
	require.False(t, after.Allowed)
}

// TestCraftBudgetAuthorizeCallReservesBudgetAndCountsCalls is the Step 6
// core: every authorized logical call holds real commercial budget in the
// dispatched state under the craft call key; the same callID retried never
// reserves twice; the grant's UsedCalls counts authorized calls.
func TestCraftBudgetAuthorizeCallReservesBudgetAndCountsCalls(t *testing.T) {
	svc, db, _ := craftBudgetEnv(t)
	ctx := context.Background()
	seedCraftFundedTenant(t, db, 7, 5000)

	g, err := svc.Admit(ctx, craft.Scope{TenantID: 7, UserID: "u", SessionID: "s"}, "run-1")
	require.NoError(t, err)

	first, err := svc.AuthorizeBinding(ctx, g.ID, CraftCallBinding{ModelID: "m1", Funding: commercial.FundingPlatform})
	require.NoError(t, err)
	second, err := svc.AuthorizeBinding(ctx, g.ID, CraftCallBinding{ModelID: "m1", Funding: commercial.FundingPlatform})
	require.NoError(t, err)
	require.NotEqual(t, first, second)

	reservations := craftReservations(t, db, 7)
	require.Len(t, reservations, 2)
	for _, r := range reservations {
		require.Equal(t, domainReservationStateDispatched(), r.State)
	}
	require.Equal(t, CraftCallKey(first), reservations[1].Key)
	require.Equal(t, int64(500), reservations[0].UpperMicro)
	require.Equal(t, int64(1000), craftAccountRow(t, db, 7).HeldMicro)

	// The stored identities are exactly the O01 derivation from the durable
	// per-binding sequence: seq 1 and 2 of binding (run-1, "", m1, platform).
	var calls []CraftBudgetCallRow
	require.NoError(t, db.Where("tenant_id = ?", uint64(7)).Order("call_seq").Find(&calls).Error)
	require.Len(t, calls, 2)
	require.Equal(t, int64(1), calls[0].CallSeq)
	require.Equal(t, int64(2), calls[1].CallSeq)
	require.Equal(t, craft.DeriveCallID(7, "run-1", "", "m1", commercial.FundingPlatform, 1), calls[0].CallID)

	// Port-level retry of the SAME callID is idempotent: no third hold.
	require.NoError(t, svc.AuthorizeCall(ctx, g.ID, first))
	require.Len(t, craftReservations(t, db, 7), 2)
	require.Equal(t, int64(1000), craftAccountRow(t, db, 7).HeldMicro)

	snapshot, err := svc.Admit(ctx, craft.Scope{TenantID: 7, UserID: "u", SessionID: "s"}, "run-1")
	require.NoError(t, err)
	require.Equal(t, 2, snapshot.UsedCalls)
}

// TestCraftBudgetLastQuotaConcurrentAuthorizationAdmitsExactlyOne is the Step 7
// concurrency acceptance: two calls race for the LAST quota (account, task
// headroom and lot each fit exactly one per-call hold) — exactly one admits.
func TestCraftBudgetLastQuotaConcurrentAuthorizationAdmitsExactlyOne(t *testing.T) {
	policy := craftBudgetPolicy()
	policy.TaskLimit = commercial.Credits(500) // room for exactly one hold
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, policy)
	require.NoError(t, err)
	ctx := context.Background()
	seedCraftFundedTenant(t, db, 7, 500) // exactly one CallUpper of funds

	g, err := svc.Admit(ctx, craft.Scope{TenantID: 7, UserID: "u", SessionID: "s"}, "run-1")
	require.NoError(t, err)

	var ok, denied atomic.Int64
	var wg sync.WaitGroup
	for _, model := range []string{"m-a", "m-b"} {
		wg.Add(1)
		go func(m string) {
			defer wg.Done()
			_, err := svc.AuthorizeBinding(ctx, g.ID, CraftCallBinding{ModelID: m, Funding: commercial.FundingPlatform})
			if err == nil {
				ok.Add(1)
				return
			}
			if errors.Is(err, craft.ErrBudgetDenied) {
				denied.Add(1)
			}
		}(model)
	}
	wg.Wait()
	require.Equal(t, int64(1), ok.Load(), "exactly one last-quota request may admit")
	require.Equal(t, int64(1), denied.Load())
	require.Equal(t, int64(500), craftAccountRow(t, db, 7).HeldMicro)
	require.Len(t, craftReservations(t, db, 7), 1)
	require.Equal(t, int64(1), craftGrantCalls(t, db, 7, g.ID))
}

// TestCraftBudgetRevokedGrantBlocksNewCallsAndKeepsAdmittedHolds: cancel
// forbids NEW calls but already-admitted work keeps its protection — a
// dispatched reservation is never zeroed by reconciliation without
// confirmation, while a provably unstarted hold releases cleanly.
func TestCraftBudgetRevokedGrantBlocksNewCallsAndKeepsAdmittedHolds(t *testing.T) {
	svc, db, _ := craftBudgetEnv(t)
	ctx := context.Background()
	seedCraftFundedTenant(t, db, 7, 5000)

	g, err := svc.Admit(ctx, craft.Scope{TenantID: 7, UserID: "u", SessionID: "s"}, "run-1")
	require.NoError(t, err)
	admitted, err := svc.AuthorizeBinding(ctx, g.ID, CraftCallBinding{ModelID: "m1", Funding: commercial.FundingPlatform})
	require.NoError(t, err)

	// A second authorized-but-unstarted call: reserved, not yet dispatched.
	unstarted := craft.DeriveCallID(7, "run-1", "", "m1", commercial.FundingPlatform, 99)
	require.NoError(t, db.Create(&CraftBudgetCallRow{
		TenantID: 7, CallKey: CraftCallKey(unstarted), GrantID: g.ID, RunID: "run-1",
		ModelID: "m1", Funding: commercial.FundingPlatform, CallSeq: 99, CallID: unstarted,
	}).Error)
	require.NoError(t, svc.reserveCall(ctx, g.ID, unstarted))
	// The reserved-but-undispatched call holds budget without being dispatched.
	require.Equal(t, int64(1000), craftAccountRow(t, db, 7).HeldMicro)

	require.NoError(t, svc.RevokeGrant(ctx, g.ID))
	_, err = svc.AuthorizeBinding(ctx, g.ID, CraftCallBinding{ModelID: "m2", Funding: commercial.FundingPlatform})
	require.ErrorIs(t, err, craft.ErrGrantRevoked)
	require.Len(t, craftReservations(t, db, 7), 2)

	// Reconciliation keeps the admitted (dispatched) hold — outcome unproven —
	// and releases the provably unstarted one.
	err = svc.Reconcile(ctx, g.ID)
	require.ErrorIs(t, err, craft.ErrReconcilePending)
	require.Equal(t, int64(500), craftAccountRow(t, db, 7).HeldMicro)
	for _, r := range craftReservations(t, db, 7) {
		if r.Key == CraftCallKey(admitted) {
			require.Equal(t, domainReservationStateDispatched(), r.State)
		} else {
			require.Equal(t, domainReservationStateReleased(), r.State)
		}
	}
}

// TestCraftBudgetExpiredGrantRefusesCalls: past the grant deadline (and the
// task budget deadline it registered) no new call admits; G4's expired-quota
// refusal surfaces as the craft ErrGrantExpired.
func TestCraftBudgetExpiredGrantRefusesCalls(t *testing.T) {
	svc, db, now := craftBudgetEnv(t)
	ctx := context.Background()
	seedCraftFundedTenant(t, db, 7, 5000)

	g, err := svc.Admit(ctx, craft.Scope{TenantID: 7, UserID: "u", SessionID: "s"}, "run-1")
	require.NoError(t, err)

	now.Store(time.Now().UTC().Add(2 * time.Hour))
	_, err = svc.AuthorizeBinding(ctx, g.ID, CraftCallBinding{ModelID: "m1", Funding: commercial.FundingPlatform})
	require.ErrorIs(t, err, craft.ErrGrantExpired)
	require.Empty(t, craftReservations(t, db, 7))

	snapshot, err := svc.Admit(ctx, craft.Scope{TenantID: 7, UserID: "u", SessionID: "s"}, "run-1")
	require.NoError(t, err)
	require.False(t, craft.BudgetAllows(snapshot, time.Now().UTC().Add(2*time.Hour)))
}

// TestCraftBudgetUnfundedTenantAdmitsButBlocksCalls: admission registers the
// run, but without a funded account every call is denied BEFORE any forward —
// and the denial compensates the call ledger so a later top-up can retry.
func TestCraftBudgetUnfundedTenantAdmitsButBlocksCalls(t *testing.T) {
	svc, db, _ := craftBudgetEnv(t)
	ctx := context.Background()

	g, err := svc.Admit(ctx, craft.Scope{TenantID: 8, UserID: "u", SessionID: "s"}, "run-8")
	require.NoError(t, err)
	require.True(t, g.Allowed)

	_, err = svc.AuthorizeBinding(ctx, g.ID, CraftCallBinding{ModelID: "m1", Funding: commercial.FundingPlatform})
	require.ErrorIs(t, err, craft.ErrBudgetDenied)
	require.Empty(t, craftReservations(t, db, 8))
	require.Equal(t, int64(0), craftGrantCalls(t, db, 8, g.ID))
}

// TestCraftBudgetExtendRaisesCallCapAndCommercialLimit is the augmentation
// mapping: Extend raises the G4 task limit exactly once per idempotency key
// and the craft call cap by the configured calls.
func TestCraftBudgetExtendRaisesCallCapAndCommercialLimit(t *testing.T) {
	policy := craftBudgetPolicy()
	policy.MaxCalls = 3
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, policy)
	require.NoError(t, err)
	ctx := context.Background()
	seedCraftFundedTenant(t, db, 7, 10000)

	g, err := svc.Admit(ctx, craft.Scope{TenantID: 7, UserID: "u", SessionID: "s"}, "run-1")
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err := svc.AuthorizeBinding(ctx, g.ID, CraftCallBinding{ModelID: "m1", Funding: commercial.FundingPlatform})
		require.NoError(t, err)
	}
	_, err = svc.AuthorizeBinding(ctx, g.ID, CraftCallBinding{ModelID: "m1", Funding: commercial.FundingPlatform})
	require.ErrorIs(t, err, craft.ErrGrantExhausted)

	require.NoError(t, svc.Extend(ctx, g.ID, "ext-1", 2, commercial.Credits(1000)))
	require.NoError(t, svc.Extend(ctx, g.ID, "ext-1", 2, commercial.Credits(1000))) // idempotent key

	var task repocommercial.TaskBudgetRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", uint64(7), "run-1").First(&task).Error)
	require.Equal(t, int64(6000), task.LimitMicro)

	for i := 0; i < 2; i++ {
		_, err := svc.AuthorizeBinding(ctx, g.ID, CraftCallBinding{ModelID: "m1", Funding: commercial.FundingPlatform})
		require.NoError(t, err)
	}
	_, err = svc.AuthorizeBinding(ctx, g.ID, CraftCallBinding{ModelID: "m1", Funding: commercial.FundingPlatform})
	require.ErrorIs(t, err, craft.ErrGrantExhausted)
}

// TestCraftBudgetGrantsOfDifferentTenantsNeverMix: the same runID under two
// tenants resolves to two isolated grants — counts, holds and identities
// never cross the tenant boundary.
func TestCraftBudgetGrantsOfDifferentTenantsNeverMix(t *testing.T) {
	svc, db, _ := craftBudgetEnv(t)
	ctx := context.Background()
	seedCraftFundedTenant(t, db, 7, 5000)
	seedCraftFundedTenant(t, db, 8, 5000)

	g7, err := svc.Admit(ctx, craft.Scope{TenantID: 7, UserID: "u", SessionID: "s"}, "shared-run")
	require.NoError(t, err)
	g8, err := svc.Admit(ctx, craft.Scope{TenantID: 8, UserID: "u", SessionID: "s"}, "shared-run")
	require.NoError(t, err)
	require.NotEqual(t, g7.ID, g8.ID)

	for i := 0; i < 2; i++ {
		_, err := svc.AuthorizeBinding(ctx, g7.ID, CraftCallBinding{ModelID: "m1", Funding: commercial.FundingPlatform})
		require.NoError(t, err)
	}
	snapshot7, err := svc.Admit(ctx, craft.Scope{TenantID: 7, UserID: "u", SessionID: "s"}, "shared-run")
	require.NoError(t, err)
	snapshot8, err := svc.Admit(ctx, craft.Scope{TenantID: 8, UserID: "u", SessionID: "s"}, "shared-run")
	require.NoError(t, err)
	require.Equal(t, 2, snapshot7.UsedCalls)
	require.Equal(t, 0, snapshot8.UsedCalls)
	require.Equal(t, int64(1000), craftAccountRow(t, db, 7).HeldMicro)
	require.Equal(t, int64(0), craftAccountRow(t, db, 8).HeldMicro)
}
