package service

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openCraftUsageServiceDB applies the real SQLite migrations (000043_craft_usage
// included) into an isolated temp database, following openDurableRunTestDB.
func openCraftUsageServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dbPath := filepath.Join(t.TempDir(), "craft-usage.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"

	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file:"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	return db
}

// newCraftUsageFixture wires the real durable store behind the service.
func newCraftUsageFixture(t *testing.T) (*CraftUsageService, *repository.CraftUsageStore) {
	t.Helper()
	db := openCraftUsageServiceDB(t)
	store := repository.NewCraftUsageStore(db)
	return NewCraftUsageService(store), store
}

// mainCall builds a main-runtime physical call with the gateway-issued
// identity; usage nil means the stream broke before usage was observed.
func mainCall(tenant uint64, runID, callID, attemptID string, usage *craft.UsageTotals) PhysicalCall {
	return PhysicalCall{
		TenantID: tenant, RunID: runID, CallID: callID, AttemptID: attemptID,
		Runtime: craft.RuntimeMain, ModelID: "gpt-main",
		Funding: commercial.FundingPlatform, Usage: usage,
	}
}

// childCall builds an OC-runtime physical call of one delegation.
func childCall(tenant uint64, runID, delegationID, callID, attemptID string, usage *craft.UsageTotals) PhysicalCall {
	return PhysicalCall{
		TenantID: tenant, RunID: runID, DelegationID: delegationID,
		CallID: callID, AttemptID: attemptID,
		Runtime: craft.RuntimeOC, ModelID: "gpt-child",
		Funding: commercial.FundingPlatform, Usage: usage,
	}
}

func totals(in, out, cached int64) *craft.UsageTotals {
	return &craft.UsageTotals{Input: in, Output: out, Cached: cached}
}

// TestCraftUsageServiceCountsPhysicalAttemptsAcrossRuntimes is the Step 7
// acceptance: 2 main calls + 3 OC child calls produce exactly 5 physical
// facts; a duplicated OC aggregate event still leaves 5 (aggregates verify,
// they never bill); one real model retry becomes a 6th fact; the same call
// identity under another tenant never mixes into this tenant's ledger.
func TestCraftUsageServiceCountsPhysicalAttemptsAcrossRuntimes(t *testing.T) {
	svc, _ := newCraftUsageFixture(t)
	ctx := context.Background()

	// 2 main runtime calls.
	_, err := svc.RecordPhysicalCall(ctx, mainCall(1, "run-1", "call-m1", "att-1", totals(100, 40, 10)))
	require.NoError(t, err)
	_, err = svc.RecordPhysicalCall(ctx, mainCall(1, "run-1", "call-m2", "att-1", totals(80, 20, 0)))
	require.NoError(t, err)
	// 3 OC child runtime calls under one delegation.
	for i, u := range []*craft.UsageTotals{totals(50, 10, 0), totals(60, 15, 5), totals(70, 25, 10)} {
		callID := svc.IssueCallID(1, "run-1", "dlg-1", "gpt-child", commercial.FundingPlatform, int64(i+1))
		attID := svc.IssueAttemptID(callID, 1)
		_, err = svc.RecordPhysicalCall(ctx, childCall(1, "run-1", "dlg-1", callID, attID, u))
		require.NoError(t, err)
	}

	facts, err := svc.Facts(ctx, 1, "run-1")
	require.NoError(t, err)
	require.Len(t, facts, 5, "2 main + 3 child = 5 physical facts")
	got, err := svc.Totals(ctx, 1, "run-1")
	require.NoError(t, err)
	require.EqualValues(t, 360, got.Input)
	require.EqualValues(t, 110, got.Output)
	require.EqualValues(t, 25, got.Cached)
	require.Equal(t, 5, got.Facts)

	// The delegation's aggregate event verifies once...
	agg := OCDelegateUsage{TenantID: 1, DelegationID: "dlg-1", Totals: craft.UsageTotals{
		Input: 180, Output: 50, Cached: 15, Facts: 3,
	}}
	verdict, err := svc.ObserveOCAggregate(ctx, agg)
	require.NoError(t, err)
	require.True(t, verdict.Matched, "aggregate matches recorded child facts")
	// ...and duplicates are free: still 5 facts.
	verdict, err = svc.ObserveOCAggregate(ctx, agg)
	require.NoError(t, err)
	require.True(t, verdict.Matched)
	facts, err = svc.Facts(ctx, 1, "run-1")
	require.NoError(t, err)
	require.Len(t, facts, 5, "duplicate OC aggregate event must not add facts")

	// One real model retry of a main call is a NEW physical attempt.
	_, err = svc.RecordPhysicalCall(ctx, mainCall(1, "run-1", "call-m2", "att-2", totals(90, 30, 0)))
	require.NoError(t, err)
	facts, err = svc.Facts(ctx, 1, "run-1")
	require.NoError(t, err)
	require.Len(t, facts, 6, "a real retry is a new physical fact")
	got, err = svc.Totals(ctx, 1, "run-1")
	require.NoError(t, err)
	require.EqualValues(t, 450, got.Input, "the retry's real tokens are counted")

	// The same call identity under another tenant is a different fact.
	_, err = svc.RecordPhysicalCall(ctx, mainCall(2, "run-2", "call-m1", "att-1", totals(10, 5, 0)))
	require.NoError(t, err)
	facts, err = svc.Facts(ctx, 1, "run-1")
	require.NoError(t, err)
	require.Len(t, facts, 6, "cross-tenant call ids never mix")
	factsOther, err := svc.Facts(ctx, 2, "run-2")
	require.NoError(t, err)
	require.Len(t, factsOther, 1)
}

// TestCraftUsageServiceUnknownStaysSeparateAndLateUsageCorrects: a stream
// break records an unknown fact with NO numbers that is listed separately
// and never summed; the late usage arrives as an explicit correction — the
// same physical attempt, a new revision, no second charge; the revision
// trail is the reconciliation record. A post-cancellation late fact walks
// the same path.
func TestCraftUsageServiceUnknownStaysSeparateAndLateUsageCorrects(t *testing.T) {
	svc, store := newCraftUsageFixture(t)
	ctx := context.Background()

	_, err := svc.RecordPhysicalCall(ctx, mainCall(1, "run-1", "call-m1", "att-1", totals(100, 40, 10)))
	require.NoError(t, err)
	// Stream breaks mid-flight: usage never observed.
	_, err = svc.RecordPhysicalCall(ctx, mainCall(1, "run-1", "call-m2", "att-1", nil))
	require.NoError(t, err)

	facts, err := svc.Facts(ctx, 1, "run-1")
	require.NoError(t, err)
	require.Len(t, facts, 2)
	got, err := svc.Totals(ctx, 1, "run-1")
	require.NoError(t, err)
	require.EqualValues(t, 100, got.Input, "unknown fact contributes no tokens")
	require.EqualValues(t, 40, got.Output)
	require.Equal(t, 1, got.Facts, "only the observed attempt is counted")
	require.Equal(t, 1, got.Unknown, "unknown is its own line item")
	unknown, err := svc.UnknownConsumption(ctx, 1, "run-1")
	require.NoError(t, err)
	require.Len(t, unknown, 1)
	require.Equal(t, craft.UsageStatusUnknown, unknown[0].Status)
	require.Zero(t, unknown[0].Input)
	require.Zero(t, unknown[0].Output)

	// Late usage for the SAME attempt: a correction, not a new charge.
	err = svc.CorrectLateUsage(ctx, mainCall(1, "run-1", "call-m2", "att-1", totals(200, 80, 20)))
	require.NoError(t, err)
	facts, err = svc.Facts(ctx, 1, "run-1")
	require.NoError(t, err)
	require.Len(t, facts, 2, "late usage must not add a physical fact")
	got, err = svc.Totals(ctx, 1, "run-1")
	require.NoError(t, err)
	require.EqualValues(t, 300, got.Input, "late tokens counted exactly once")
	require.EqualValues(t, 120, got.Output)
	require.Equal(t, 2, got.Facts)
	require.Equal(t, 0, got.Unknown, "corrected attempt is no longer unknown")
	unknown, err = svc.UnknownConsumption(ctx, 1, "run-1")
	require.NoError(t, err)
	require.Empty(t, unknown)

	// Reconciliation record: revision 1 unknown, revision 2 corrected.
	trail, err := svc.Reconciliation(ctx, 1, "call-m2", "att-1")
	require.NoError(t, err)
	require.Len(t, trail, 2)
	require.Equal(t, craft.UsageStatusUnknown, trail[0].Status)
	require.Equal(t, craft.UsageStatusCorrected, trail[1].Status)

	// Both revisions carry a delivery event: downstream sees the trail.
	events, err := store.ListUsageEvents(ctx, 1, "call-m2", "att-1")
	require.NoError(t, err)
	require.Len(t, events, 2)

	// Cancellation then late arrival: the same reconciliation shape.
	_, err = svc.RecordPhysicalCall(ctx, childCall(1, "run-1", "dlg-1", "call-c1", "att-1", nil))
	require.NoError(t, err)
	err = svc.CorrectLateUsage(ctx, childCall(1, "run-1", "dlg-1", "call-c1", "att-1", totals(30, 12, 0)))
	require.NoError(t, err)
	trail, err = svc.Reconciliation(ctx, 1, "call-c1", "att-1")
	require.NoError(t, err)
	require.Len(t, trail, 2)
	require.Equal(t, craft.UsageStatusUnknown, trail[0].Status)
	require.Equal(t, craft.UsageStatusCorrected, trail[1].Status)
}

// TestCraftUsageServiceHandoffNeverRecountsChildTokens: handing the OC
// child's tool result to the main agent adds no child tokens to the main
// runtime; the main agent's NEXT real model call is its own new fact. An OC
// aggregate that folds parent tokens into its rollup honestly mismatches.
func TestCraftUsageServiceHandoffNeverRecountsChildTokens(t *testing.T) {
	svc, _ := newCraftUsageFixture(t)
	ctx := context.Background()

	// 2 main calls, one delegation with 1 child call.
	_, err := svc.RecordPhysicalCall(ctx, mainCall(1, "run-1", "call-m1", "att-1", totals(100, 40, 10)))
	require.NoError(t, err)
	_, err = svc.RecordPhysicalCall(ctx, mainCall(1, "run-1", "call-m2", "att-1", totals(80, 20, 0)))
	require.NoError(t, err)
	_, err = svc.RecordPhysicalCall(ctx, childCall(1, "run-1", "dlg-1", "call-c1", "att-1", totals(50, 10, 0)))
	require.NoError(t, err)

	// The child finishes; its tool result is handed to the main agent,
	// which then makes its next REAL model call — one new main fact.
	_, err = svc.RecordPhysicalCall(ctx, mainCall(1, "run-1", "call-m3", "att-1", totals(120, 60, 20)))
	require.NoError(t, err)

	facts, err := svc.Facts(ctx, 1, "run-1")
	require.NoError(t, err)
	require.Len(t, facts, 4)

	var main, child craft.UsageTotals
	for _, f := range facts {
		switch f.Runtime {
		case craft.RuntimeMain:
			main.Add(f)
		case craft.RuntimeOC:
			child.Add(f)
		}
	}
	require.EqualValues(t, 300, main.Input, "main totals cover exactly the 3 main calls")
	require.EqualValues(t, 120, main.Output)
	require.EqualValues(t, 50, child.Input, "handoff did not re-count child tokens")
	require.EqualValues(t, 10, child.Output)

	// The child aggregate verifies against child facts only...
	verdict, err := svc.ObserveOCAggregate(ctx, OCDelegateUsage{
		TenantID: 1, DelegationID: "dlg-1",
		Totals: craft.UsageTotals{Input: 50, Output: 10, Facts: 1},
	})
	require.NoError(t, err)
	require.True(t, verdict.Matched)

	// ...while a parent-style rollup folding the main agent's tokens is a
	// reported mismatch, never a second billing source.
	rollup, err := svc.ObserveOCAggregate(ctx, OCDelegateUsage{
		TenantID: 1, DelegationID: "dlg-1",
		Totals: craft.UsageTotals{Input: 350, Output: 130, Facts: 4},
	})
	require.NoError(t, err)
	require.False(t, rollup.Matched, "a parent rollup can never verify as child usage")

	facts, err = svc.Facts(ctx, 1, "run-1")
	require.NoError(t, err)
	require.Len(t, facts, 4, "verification writes nothing")
}

// TestCraftUsageServiceFundingAndUnknownChildVerification: funding the
// server never produced is rejected before the ledger; BYOK facts are
// recorded as observations (billability is the commercial settlement's
// filter, not this ledger's); an aggregate over an unobserved child cannot
// verify exactly.
func TestCraftUsageServiceFundingAndUnknownChildVerification(t *testing.T) {
	svc, _ := newCraftUsageFixture(t)
	ctx := context.Background()

	_, err := svc.RecordPhysicalCall(ctx, PhysicalCall{
		TenantID: 1, RunID: "run-1", CallID: "call-x", AttemptID: "att-1",
		Runtime: craft.RuntimeMain, ModelID: "gpt-main", Funding: "from_model_argument",
		Usage: totals(10, 5, 0),
	})
	require.ErrorIs(t, err, craft.ErrInvalidInput, "untrusted funding never enters the ledger")

	// BYOK records an observation only; no platform model Credits arise
	// from this ledger (commercial.BillableModel decides downstream).
	byok := mainCall(1, "run-1", "call-byok", "att-1", totals(64, 32, 8))
	byok.Funding = commercial.FundingBYOK
	fact, err := svc.RecordPhysicalCall(ctx, byok)
	require.NoError(t, err)
	require.Equal(t, commercial.FundingBYOK, fact.Funding)
	require.False(t, commercial.BillableModel(fact.Funding), "BYOK model dimension is not billable")

	// An unobserved child blocks exact aggregate verification.
	_, err = svc.RecordPhysicalCall(ctx, childCall(1, "run-1", "dlg-1", "call-c1", "att-1", nil))
	require.NoError(t, err)
	verdict, err := svc.ObserveOCAggregate(ctx, OCDelegateUsage{
		TenantID: 1, DelegationID: "dlg-1",
		Totals: craft.UsageTotals{Input: 0, Output: 0, Facts: 1},
	})
	require.NoError(t, err)
	require.False(t, verdict.Matched, "unknown children are unverifiable, not zero-verified")
	require.Equal(t, 1, verdict.Recorded.Unknown)
}
