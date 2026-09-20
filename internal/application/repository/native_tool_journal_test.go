package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
)

func nativeToolJournalFixture(t *testing.T) (*NativeToolJournal, nativecontract.Fence) {
	t.Helper()
	db := openRunTestDB(t)
	require.NoError(t, db.Exec("INSERT INTO native_agent_tenants (tenant_id) VALUES (?)", 1).Error)
	require.NoError(t, db.Exec("INSERT INTO native_agent_sessions (tenant_id, owner_id, session_id) VALUES (?, ?, ?)", 1, "owner", "session").Error)
	require.NoError(t, db.Exec(`INSERT INTO native_agent_runs
		(tenant_id, run_id, owner_id, session_id, status, lease_owner, lease_epoch, lease_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, 1, "run-1", "owner", "session", "running", "worker-1", 7, time.Now().Add(time.Hour)).Error)
	require.NoError(t, db.Exec(`INSERT INTO native_agent_attempts
		(tenant_id, run_id, attempt_id, lease_epoch, kind, attempt_number)
		VALUES (?, ?, ?, ?, ?, ?)`, 1, "run-1", "model-1", 7, "model", 1).Error)
	return NewNativeToolJournal(db), nativecontract.Fence{
		Run: nativecontract.RunIdentity{TenantID: 1, SessionID: "session", RunID: "run-1"}, Owner: "worker-1", Epoch: 7,
	}
}

func nativeToolPlan() nativecontract.ToolPlan {
	return nativecontract.ToolPlan{
		Version: 1, Run: nativecontract.RunIdentity{TenantID: 1, SessionID: "session", RunID: "run-1"},
		CallID: "call-1", ProviderToolCallID: "provider-call-1", ModelAttemptID: "model-1",
		Tool: nativecontract.ToolIdentity{Kind: "connector", ServiceID: "svc", InstallationID: "install", Name: "write", SchemaHash: "schema-v1", ConfigVersion: "config-v1"},
		Args: json.RawMessage(`{"record":"one"}`), ArgsHash: "args-v1", Policy: nativecontract.RecoveryHold,
	}
}

func TestNativeToolPlanIsFencedAndChangedArgumentsConflict(t *testing.T) {
	journal, fence := nativeToolJournalFixture(t)
	ctx := context.Background()
	plan := nativeToolPlan()

	stored, err := journal.Plan(ctx, fence, plan)
	require.NoError(t, err)
	require.Equal(t, plan, stored)
	_, err = journal.Plan(ctx, fence, plan)
	require.NoError(t, err, "the same call plan is idempotent")
	changed := plan
	changed.Args = json.RawMessage(`{"record":"two"}`)
	changed.ArgsHash = "args-v2"
	_, err = journal.Plan(ctx, fence, changed)
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, err))

	stale := fence
	stale.Epoch--
	_, err = journal.Plan(ctx, stale, nativeToolPlan())
	require.Equal(t, nativecontract.ErrLeaseLost, failureCode(t, err))
}

func TestNativeToolAttemptPersistsUnknownEffectWithoutForgingResult(t *testing.T) {
	journal, fence := nativeToolJournalFixture(t)
	ctx := context.Background()
	require.NoError(t, func() error { _, err := journal.Plan(ctx, fence, nativeToolPlan()); return err }())

	attempt, err := journal.Begin(ctx, fence, nativecontract.Attempt{ID: "tool-1", Run: fence.Run, Kind: nativecontract.ToolAttempt, LogicalCallID: "call-1", Number: 2, Epoch: fence.Epoch, StartedAt: time.Now()})
	require.NoError(t, err)
	require.Equal(t, nativecontract.ToolAttempt, attempt.Kind)
	require.NoError(t, journal.Finish(ctx, fence, attempt.ID, nativecontract.EffectUnknown, &nativecontract.Failure{Code: nativecontract.ErrUnknownEffect, Effect: nativecontract.EffectUnknown}))

	got, err := journal.Get(ctx, nativecontract.Scope{TenantID: 1}, fence.Run, attempt.ID)
	require.NoError(t, err)
	require.Equal(t, attempt.ID, got.ID)
	var effect string
	require.NoError(t, journal.db.Table("native_agent_attempts").Select("effect_state").Where("tenant_id=? AND run_id=? AND attempt_id=?", 1, "run-1", attempt.ID).Scan(&effect).Error)
	require.Equal(t, string(nativecontract.EffectUnknown), effect)
	_, err = journal.LookupResult(ctx, nativecontract.Scope{TenantID: 1}, fence.Run, "call-1")
	require.Equal(t, nativecontract.ErrNotFound, failureCode(t, err), "unknown effect is not a confirmed result")
}

func TestNativeToolConfirmedOutcomeIsImmutableAndVisibleOnlyInScope(t *testing.T) {
	journal, fence := nativeToolJournalFixture(t)
	ctx := context.Background()
	require.NoError(t, func() error { _, err := journal.Plan(ctx, fence, nativeToolPlan()); return err }())
	attempt, err := journal.Begin(ctx, fence, nativecontract.Attempt{ID: "tool-1", Run: fence.Run, Kind: nativecontract.ToolAttempt, LogicalCallID: "call-1", Number: 2, Epoch: fence.Epoch, StartedAt: time.Now()})
	require.NoError(t, err)
	outcome := nativecontract.ToolOutcome{AttemptID: attempt.ID, CallID: "call-1", ProviderReceipt: "receipt-1", ResultHash: "result-1", Effect: nativecontract.EffectConfirmed, Content: json.RawMessage(`{"ok":true}`)}
	require.NoError(t, journal.RecordOutcome(ctx, fence, outcome))
	require.NoError(t, journal.RecordOutcome(ctx, fence, outcome))
	changed := outcome
	changed.ResultHash = "result-2"
	require.Equal(t, nativecontract.ErrConflict, failureCode(t, journal.RecordOutcome(ctx, fence, changed)))

	got, err := journal.LookupResult(ctx, nativecontract.Scope{TenantID: 1}, fence.Run, "call-1")
	require.NoError(t, err)
	require.Equal(t, outcome, got)
	_, err = journal.LookupResult(ctx, nativecontract.Scope{TenantID: 2}, fence.Run, "call-1")
	require.Equal(t, nativecontract.ErrForbidden, failureCode(t, err))
}
