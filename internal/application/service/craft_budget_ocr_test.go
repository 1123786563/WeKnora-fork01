package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

// TestCraftBudgetSandboxSecondDistinctActivitySucceeds is the OCR
// high-finding regression: the sandbox facet namespace must allow the
// SECOND distinct activity of one run. The pre-fix fresh branch wrote every
// sandbox activity into the empty-facet tuple (run,”,”,0) and the unique
// index rejected the second distinct sandbox/<activityID> with a raw
// constraint error.
func TestCraftBudgetSandboxSecondDistinctActivitySucceeds(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	svc, err := NewCraftBudgetService(db, nil, craftBudgetPolicy())
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 193, 10000)
	ctx := context.Background()
	seedCraftBudgetRun(t, db, 193, "run-sandbox-ocr", "craft107-t19")
	grant, err := svc.Admit(ctx, craft.Scope{TenantID: 193, UserID: "owner", SessionID: "craft107-t19"}, "run-sandbox-ocr")
	require.NoError(t, err)

	require.NoError(t, svc.AuthorizeSandbox(ctx, grant.ID, "start/sandbox-a"))
	require.NoError(t, svc.AuthorizeSandbox(ctx, grant.ID, "start/sandbox-a"),
		"the same activity must stay idempotent")
	require.NoError(t, svc.AuthorizeSandbox(ctx, grant.ID, "start/sandbox-b"),
		"a second distinct sandbox activity of the same run must be authorizable")
	require.NoError(t, svc.AuthorizeSandbox(ctx, grant.ID, "start/sandbox-c"))
	require.Equal(t, int64(3), craftGrantCalls(t, db, 193, grant.ID),
		"three distinct activities hold three ledger rows")
}

// TestCraftBudgetExtensionSameSizeDifferentKeySucceeds is the OCR
// high-finding regression: two same-sized extensions of one run under
// DIFFERENT keys must both commit. The pre-fix marker tuple carried the
// constant model id, so the unique index rejected the second one forever
// (while the G4 limit had already been raised, desynchronizing the caps).
func TestCraftBudgetExtensionSameSizeDifferentKeySucceeds(t *testing.T) {
	db := openCraftBudgetTestDB(t)
	policy := craftBudgetPolicy()
	policy.TaskLimit = commercial.Credits(5000)
	svc, err := NewCraftBudgetService(db, nil, policy)
	require.NoError(t, err)
	seedCraftFundedTenant(t, db, 194, 100000)
	ctx := context.Background()
	scope := craft.Scope{TenantID: 194, UserID: "owner", SessionID: "craft107-t19"}
	seedCraftBudgetRun(t, db, 194, "run-ext-ocr", scope.SessionID)
	grant, err := svc.Admit(ctx, scope, "run-ext-ocr")
	require.NoError(t, err)

	require.NoError(t, svc.Extend(ctx, grant.ID, "ext-key-1", 3, commercial.Credits(1000)))
	require.NoError(t, svc.Extend(ctx, grant.ID, "ext-key-2", 3, commercial.Credits(1000)),
		"a second same-sized extension under a different key must commit")
	require.NoError(t, svc.Extend(ctx, grant.ID, "ext-key-2", 3, commercial.Credits(1000)),
		"the same key replays idempotently")

	require.ErrorIs(t, svc.Extend(ctx, grant.ID, "ext-key-1", 4, commercial.Credits(1000)), craft.ErrConflict,
		"the same key with a different size is refused")
	var markers int64
	require.NoError(t, db.Table("craft_budget_calls").
		Where("tenant_id = ? AND run_id = ? AND model_id LIKE '__craft_budget_extension__/%'", 194, "run-ext-ocr").
		Count(&markers).Error)
	require.EqualValues(t, 2, markers, "two distinct extension markers coexist")
}
