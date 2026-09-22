package service

// O04 usage view tests: the composition service over the real migrations —
// aggregation fidelity (unknown stays visible), the access matrix (owner /
// viewer / admin / cross-tenant), the failure surface, sandbox residency and
// the as_of refresh stamp.

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// usageViewSessions mirrors the production session read ACL the same way the
// W03 handler harness does: the session must live in the caller's tenant and
// the caller must be its owner or a tenant admin.
type usageViewSessions struct {
	interfaces.SessionService
	rows map[string]types.Session
}

func (f *usageViewSessions) GetSession(ctx context.Context, id string) (*types.Session, error) {
	tenant, _ := types.TenantIDFromContext(ctx)
	user := types.SessionOwnerIDFromContext(ctx)
	if user == "" {
		user, _ = types.UserIDFromContext(ctx)
	}
	session, ok := f.rows[id]
	if !ok || session.TenantID != tenant {
		return nil, craft.ErrNotFound
	}
	if session.UserID != user && !types.TenantRoleFromContext(ctx).HasPermission(types.TenantRoleAdmin) {
		return nil, craft.ErrNotFound
	}
	return &session, nil
}

func newUsageViewEnv(t *testing.T) (env *lifecycleEnv, svc *CraftUsageViewService, sessions *usageViewSessions, clock int64) {
	t.Helper()
	env = newLifecycleEnv(t)
	require.NoError(t, env.db.Exec(
		"INSERT OR IGNORE INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 'usage', 'u1', 'trpc')").Error)
	sessions = &usageViewSessions{rows: map[string]types.Session{
		"s1": {ID: "s1", TenantID: 1, UserID: "u1", EngineType: "trpc"},
	}}
	usage := NewCraftUsageService(repository.NewCraftUsageStore(env.db))
	view, err := NewCraftUsageViewService(CraftUsageViewConfig{
		DB: env.db, Sessions: sessions, Usage: usage,
		Residency: env.svc, Versions: env.versions,
		Now: func() time.Time { return time.Unix(20_000, 0).UTC() },
	})
	require.NoError(t, err)
	return env, view, sessions, 20_000
}

func usageScopeCtx(tenant uint64, user string, admin bool) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenant)
	ctx = context.WithValue(ctx, types.UserIDContextKey, user)
	if admin {
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleAdmin)
	}
	return ctx
}

func recordUsageFact(t *testing.T, usage *CraftUsageService, in PhysicalCall) {
	t.Helper()
	_, err := usage.RecordPhysicalCall(context.Background(), in)
	require.NoError(t, err)
}

// The aggregation never folds unknown consumption into zeros: known calls
// carry tokens, unknown calls stay their own line, the funding mix and the
// BYOK marker are derived from the recorded facts only.
func TestCraftUsageViewAggregatesKnownAndUnknown(t *testing.T) {
	env, view, _, _ := newUsageViewEnv(t)
	env.insertActiveRun(t, "s1")
	usage := NewCraftUsageService(repository.NewCraftUsageStore(env.db))

	mainCall := usage.IssueCallID(1, "run-s1", "", "m-main", "platform", 1)
	recordUsageFact(t, usage, PhysicalCall{
		TenantID: 1, RunID: "run-s1", CallID: mainCall,
		AttemptID: usage.IssueAttemptID(mainCall, 1),
		Runtime:   craft.RuntimeMain, ModelID: "m-main", Funding: "platform",
		Usage: &craft.UsageTotals{Input: 10, Output: 20, Cached: 5, Facts: 1},
	})
	ocCall := usage.IssueCallID(1, "run-s1", "dlg-1", "m-child", "byok", 1)
	recordUsageFact(t, usage, PhysicalCall{
		TenantID: 1, RunID: "run-s1", DelegationID: "dlg-1", CallID: ocCall,
		AttemptID: usage.IssueAttemptID(ocCall, 1),
		Runtime:   craft.RuntimeOC, ModelID: "m-child", Funding: "byok",
		Usage: &craft.UsageTotals{Input: 1, Output: 2, Facts: 1},
	})
	ocUnknown := usage.IssueCallID(1, "run-s1", "dlg-1", "m-child", "byok", 2)
	recordUsageFact(t, usage, PhysicalCall{
		TenantID: 1, RunID: "run-s1", DelegationID: "dlg-1", CallID: ocUnknown,
		AttemptID: usage.IssueAttemptID(ocUnknown, 1),
		Runtime:   craft.RuntimeOC, ModelID: "m-child", Funding: "byok",
		Usage: nil, // stream broke: an unknown observation, never zeros
	})
	// A late correction of the unknown attempt replaces it in the CURRENT
	// view but keeps the revision trail (reconciled, not fabricated).
	err := usage.CorrectLateUsage(context.Background(), PhysicalCall{
		TenantID: 1, RunID: "run-s1", DelegationID: "dlg-1", CallID: ocUnknown,
		AttemptID: usage.IssueAttemptID(ocUnknown, 1),
		Runtime:   craft.RuntimeOC, ModelID: "m-child", Funding: "byok",
		Usage: &craft.UsageTotals{Input: 3, Output: 4, Facts: 1},
	})
	require.NoError(t, err)

	got, err := view.SessionUsage(usageScopeCtx(1, "u1", false), craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"})
	require.NoError(t, err)
	require.Equal(t, 3, got.KnownCalls)
	require.Equal(t, 0, got.UnknownCalls)
	require.Equal(t, int64(14), got.InputTokens)
	require.Equal(t, int64(26), got.OutputTokens)
	require.Equal(t, int64(5), got.CachedTokens)
	require.Equal(t, CraftFundingMixed, got.Funding)
	require.True(t, got.ByokModelBorneBySpace)
	require.Len(t, got.Calls, 3)
	for _, call := range got.Calls {
		require.NotEqual(t, craft.UsageStatusUnknown, call.Status, "corrected attempt is no longer unknown")
	}
	require.Len(t, got.Runs, 1)
	require.Equal(t, "run-s1", got.Runs[0].RunID)
}

// Unknown-first shape: without the correction the unknown call stays on its
// own line and contributes no tokens.
func TestCraftUsageViewUnknownStaysVisible(t *testing.T) {
	env, view, _, _ := newUsageViewEnv(t)
	env.insertActiveRun(t, "s1")
	usage := NewCraftUsageService(repository.NewCraftUsageStore(env.db))
	call := usage.IssueCallID(1, "run-s1", "dlg-2", "m-child", "byok", 1)
	recordUsageFact(t, usage, PhysicalCall{
		TenantID: 1, RunID: "run-s1", DelegationID: "dlg-2", CallID: call,
		AttemptID: usage.IssueAttemptID(call, 1),
		Runtime:   craft.RuntimeOC, ModelID: "m-child", Funding: "byok", Usage: nil,
	})

	got, err := view.SessionUsage(usageScopeCtx(1, "u1", false), craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"})
	require.NoError(t, err)
	require.Equal(t, 0, got.KnownCalls)
	require.Equal(t, 1, got.UnknownCalls)
	require.Equal(t, int64(0), got.InputTokens)
	require.Equal(t, int64(0), got.OutputTokens)
	require.Equal(t, "byok", got.Funding)
	require.True(t, got.ByokModelBorneBySpace)
}

// The access matrix: the owner reads, a tenant admin reads, a plain member
// of another owner's session cannot even see it, and another tenant's
// session does not exist here.
func TestCraftUsageViewAccessMatrix(t *testing.T) {
	_, view, _, _ := newUsageViewEnv(t)
	owner := craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"}

	if _, err := view.SessionUsage(usageScopeCtx(1, "u1", false), owner); err != nil {
		t.Fatalf("owner read failed: %v", err)
	}
	if _, err := view.SessionUsage(usageScopeCtx(1, "u2", true), owner); err != nil {
		t.Fatalf("admin read failed: %v", err)
	}
	require.ErrorIs(t, func() error {
		_, err := view.SessionUsage(usageScopeCtx(1, "u2", false), owner)
		return err
	}(), craft.ErrNotFound)
	require.ErrorIs(t, func() error {
		_, err := view.SessionUsage(usageScopeCtx(2, "u1", false), owner)
		return err
	}(), craft.ErrNotFound)
}

// The failure surface is the run table's recorded reason — a failed run
// carries its wait reason, other statuses carry none. Residency comes from
// the lifecycle events; as_of follows the injected clock across refreshes.
func TestCraftUsageViewFailureResidencyAndAsOf(t *testing.T) {
	env, _, _, _ := newUsageViewEnv(t)
	clock := int64(20_000)
	view, err := NewCraftUsageViewService(CraftUsageViewConfig{
		DB: env.db,
		Sessions: &usageViewSessions{rows: map[string]types.Session{
			"s1": {ID: "s1", TenantID: 1, UserID: "u1", EngineType: "trpc"},
		}},
		Usage:     NewCraftUsageService(repository.NewCraftUsageStore(env.db)),
		Residency: env.svc,
		Now:       func() time.Time { return time.Unix(clock, 0).UTC() },
	})
	require.NoError(t, err)

	require.NoError(t, env.db.Exec("INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at) VALUES (1, 'run-f', 's1', 'u1', 'req-f', 'am-f', 'rh', '{}', 'failed', 'budget exceeded', ?, ?, ?)",
		time.Unix(19_000, 0).UTC(), time.Unix(19_100, 0).UTC(), time.Unix(19_200, 0).UTC()).Error)

	start := time.Unix(19_500, 0).UTC()
	_, err = env.svc.RecordSandboxEvent(context.Background(), 1, "s1", "sbx-1", craft.LifecycleEventSandboxStart, start)
	require.NoError(t, err)
	_, err = env.svc.RecordSandboxEvent(context.Background(), 1, "s1", "sbx-1", craft.LifecycleEventSandboxStop, start.Add(2*time.Minute))
	require.NoError(t, err)

	first, err := view.SessionUsage(usageScopeCtx(1, "u1", false), craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"})
	require.NoError(t, err)
	require.Len(t, first.Runs, 1)
	require.Equal(t, "failed", first.Runs[0].Status)
	require.Equal(t, "budget exceeded", first.Runs[0].FailureReason)
	require.NotNil(t, first.Residency)
	require.Equal(t, 1, first.Residency.Starts)
	require.InDelta(t, 120.0, first.Residency.DwellSeconds, 1e-9)
	require.Equal(t, time.Unix(20_000, 0).UTC(), first.AsOf)

	// A refreshed read after a late correction moves as_of forward.
	clock = 20_900
	second, err := view.SessionUsage(usageScopeCtx(1, "u1", false), craft.Scope{TenantID: 1, UserID: "u1", SessionID: "s1"})
	require.NoError(t, err)
	require.True(t, second.AsOf.After(first.AsOf), "refresh must advance the as_of stamp")
}
