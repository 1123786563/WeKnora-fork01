package service

// CFT-S02-T018: the budget×usage wiring matrix. The four acceptance
// assertions in one place over the REAL budget and usage services (the
// point suites live in craft_budget_test.go and craft_usage_test.go):
//  1. a denied budget issues ZERO model-call records
//  2. replaying the same physical attempt's events records ONCE
//  3. a genuine retry (a NEW physical attempt) meters as its own fact
//  4. unknown usage carries NO numbers — it never renders as zero
import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

func TestCraftExecutionBudgetDeniedIssuesNoModelCalls(t *testing.T) {
	svc, db, _ := craftBudgetEnv(t)
	ctx := context.Background()

	// An unfunded tenant admits the run but denies every call authorization.
	g, err := svc.Admit(ctx, craft.Scope{TenantID: 9, UserID: "u", SessionID: "s"}, "run-budget-1")
	require.NoError(t, err)
	require.True(t, g.Allowed)
	_, err = svc.AuthorizeBinding(ctx, g.ID, CraftCallBinding{ModelID: "m1", Funding: "platform"})
	require.ErrorIs(t, err, craft.ErrBudgetDenied)

	// The denial leaves no reservation and no call row — nothing metered.
	require.Empty(t, craftReservations(t, db, 9))
	require.Equal(t, int64(0), craftGrantCalls(t, db, 9, g.ID))
}

func TestCraftExecutionBudgetReplayAndRetryMetering(t *testing.T) {
	usage, _ := newCraftUsageFixture(t)
	ctx := context.Background()

	// The SAME physical attempt observed twice (worker replay) records once:
	// the store's UsageKey dedup makes the second record a no-op, proven by
	// the aggregate staying unchanged.
	_, err := usage.RecordPhysicalCall(ctx, mainCall(1, "run-b", "call-1", "att-1", totals(100, 40, 0)))
	require.NoError(t, err)
	_, err = usage.RecordPhysicalCall(ctx, mainCall(1, "run-b", "call-1", "att-1", totals(100, 40, 0)))
	require.NoError(t, err)

	got, err := usage.Totals(ctx, 1, "run-b")
	require.NoError(t, err)
	require.EqualValues(t, 100, got.Input, "replay added nothing")
	require.Equal(t, 1, got.Facts)

	// A genuine RETRY is a new physical attempt and meters honestly as its own fact.
	_, err = usage.RecordPhysicalCall(ctx, mainCall(1, "run-b", "call-1", "att-2", totals(60, 20, 0)))
	require.NoError(t, err)
	got, err = usage.Totals(ctx, 1, "run-b")
	require.NoError(t, err)
	require.EqualValues(t, 160, got.Input, "the retry's tokens count — different physical request")
	require.Equal(t, 2, got.Facts)
}

func TestCraftExecutionBudgetUnknownNeverZero(t *testing.T) {
	usage, _ := newCraftUsageFixture(t)
	ctx := context.Background()

	// A stream that broke before usage observation: the fact is unknown and
	// carries NO numbers (never fabricated as zero).
	_, err := usage.RecordPhysicalCall(ctx, mainCall(1, "run-u", "call-u", "att-u", nil))
	require.NoError(t, err)
	totals, err := usage.Totals(ctx, 1, "run-u")
	require.NoError(t, err)
	require.Equal(t, 0, totals.Facts, "no KNOWN fact exists")
	require.Equal(t, 1, totals.Unknown, "exactly one unknown — counted as unknown, never as zero tokens")
	require.EqualValues(t, 0, totals.Input)
	require.EqualValues(t, 0, totals.Output)

	facts, err := usage.Facts(ctx, 1, "run-u")
	require.NoError(t, err)
	require.Len(t, facts, 1)
	require.Equal(t, craft.UsageStatusUnknown, facts[0].Status, "the fact stays unknown — the UI shows 待核对, not 0")
}
