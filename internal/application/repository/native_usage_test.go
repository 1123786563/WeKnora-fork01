package repository

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
)

func nativeUsageFixture(t *testing.T) (*NativeUsageLedger, nativecontract.Fence, nativecontract.UsageObservation) {
	t.Helper()
	coordinator, fence := nativeCommitFixture(t)
	require.NoError(t, coordinator.db.Exec(`INSERT INTO native_agent_attempts (tenant_id, run_id, attempt_id, lease_epoch) VALUES (?, ?, ?, ?)`, 1, "run-1", "attempt-1", fence.Epoch).Error)
	observation := nativecontract.UsageObservation{Version: 1, Run: fence.Run, AttemptID: "attempt-1", ObservationID: "provider-request-1", ProviderRequestID: "provider-request-1", Revision: 1, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15, AccountingStatus: "known", Funding: nativecontract.FundingBinding{BudgetRef: "budget-1", BudgetRootRunID: "run-1", Funding: "platform", Service: "model", PriceVersion: "v1"}, OccurredAt: time.Now().UTC()}
	return NewNativeUsageLedger(coordinator.db), fence, observation
}

func nativeUsageCode(t *testing.T, err error) nativecontract.ErrorCode {
	t.Helper()
	var failure *nativecontract.Failure
	require.True(t, errors.As(err, &failure), "expected typed failure, got %v", err)
	return failure.Code
}

func TestNativeUsageDuplicateCallbacksRecordOneDelta(t *testing.T) {
	ledger, fence, observation := nativeUsageFixture(t)
	first, err := ledger.ObserveDelta(context.Background(), fence, observation)
	require.NoError(t, err)
	require.EqualValues(t, 15, first.TotalTokens)
	claimed, err := ledger.ClaimSettlement(context.Background(), fence, first.IntentID)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, ledger.ConfirmSettlement(context.Background(), fence, first.IntentID))
	replay, err := ledger.ObserveDelta(context.Background(), fence, observation)
	require.NoError(t, err)
	require.Zero(t, replay.TotalTokens)
	var count int64
	require.NoError(t, ledger.db.Table("native_agent_usage_observations").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestNativeToolDispatchReservationConsumesOneDecisionAndPreservesLargeIntegers(t *testing.T) {
	fence := nativeUsageReservationFence(1)
	root := fence.Run
	root.RunID = "root-1"
	const unsafeJavaScriptInteger = int64(9_007_199_254_740_993)
	coordinator := NewInMemoryNativeToolDispatchReservation(NativeToolDispatchBudget{Root: root, Available: unsafeJavaScriptInteger + 11})
	coordinator.SetLiveFence(fence)

	req := nativeUsageReservationRequest(fence, root, unsafeJavaScriptInteger)
	require.NoError(t, coordinator.ReserveAndConsume(context.Background(), req))
	require.NoError(t, coordinator.ReserveAndConsume(context.Background(), req), "an exact replay must not reserve again")

	remaining, ok := coordinator.Remaining(root)
	require.True(t, ok)
	require.Equal(t, int64(11), remaining)
}

func TestNativeToolDispatchReservationRejectsStaleFenceWithoutConsumingDecision(t *testing.T) {
	fence := nativeUsageReservationFence(1)
	root := fence.Run
	root.RunID = "root-1"
	coordinator := NewInMemoryNativeToolDispatchReservation(NativeToolDispatchBudget{Root: root, Available: 10})
	coordinator.SetLiveFence(nativeUsageReservationFence(2))

	req := nativeUsageReservationRequest(fence, root, 7)
	require.Equal(t, nativecontract.ErrLeaseLost, nativeUsageCode(t, coordinator.ReserveAndConsume(context.Background(), req)))
	remaining, ok := coordinator.Remaining(root)
	require.True(t, ok)
	require.Equal(t, int64(10), remaining)

	fresh := nativeUsageReservationFence(2)
	coordinator.SetLiveFence(fresh)
	req.Fence, req.Plan.Run, req.Attempt.Run = fresh, fresh.Run, fresh.Run
	req.Attempt.Epoch = fresh.Epoch
	require.NoError(t, coordinator.ReserveAndConsume(context.Background(), req), "a stale fence must leave the decision reusable")
}

func TestNativeToolDispatchReservationRejectsMismatchedAttemptEpoch(t *testing.T) {
	for _, epoch := range []int64{0, 1, 3} {
		t.Run(strconv.FormatInt(epoch, 10), func(t *testing.T) {
			fence := nativeUsageReservationFence(2)
			root := fence.Run
			root.RunID = "root-1"
			coordinator := NewInMemoryNativeToolDispatchReservation(NativeToolDispatchBudget{Root: root, Available: 10})
			coordinator.SetLiveFence(fence)
			req := nativeUsageReservationRequest(fence, root, 7)
			req.Attempt.Epoch = epoch
			require.Equal(t, nativecontract.ErrLeaseLost, nativeUsageCode(t, coordinator.ReserveAndConsume(context.Background(), req)))
			remaining, ok := coordinator.Remaining(root)
			require.True(t, ok)
			require.Equal(t, int64(10), remaining)
			req.Attempt.Epoch = fence.Epoch
			require.NoError(t, coordinator.ReserveAndConsume(context.Background(), req), "rejected attempt must not consume the decision")
		})
	}
}

func TestNativeToolDispatchReservationRejectsChangedAuthorizationReplay(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*nativecontract.ToolDispatchRequest)
	}{
		{"schema", func(r *nativecontract.ToolDispatchRequest) { r.Plan.Tool.SchemaHash = "schema-2" }},
		{"config", func(r *nativecontract.ToolDispatchRequest) { r.Plan.Tool.ConfigVersion = "config-2" }},
		{"tool_name", func(r *nativecontract.ToolDispatchRequest) { r.Plan.Tool.Name = "delete" }},
		{"tool_service", func(r *nativecontract.ToolDispatchRequest) { r.Plan.Tool.ServiceID = "service-2" }},
		{"installation", func(r *nativecontract.ToolDispatchRequest) { r.Plan.Tool.InstallationID = "installation-2" }},
		{"model_attempt", func(r *nativecontract.ToolDispatchRequest) { r.Plan.ModelAttemptID = "model-2" }},
		{"required_grants", func(r *nativecontract.ToolDispatchRequest) {
			r.Plan.RequiredGrants = []nativecontract.ResourceGrant{{ResourceType: "file", ResourceID: "secret", Action: "write"}}
		}},
		{"scope_grants", func(r *nativecontract.ToolDispatchRequest) {
			r.Scope.Grants = []nativecontract.ResourceGrant{{ResourceType: "file", ResourceID: "secret", Action: "write"}}
		}},
		{"policy_revision", func(r *nativecontract.ToolDispatchRequest) { r.Scope.PolicyRevision++ }},
		{"recovery_policy", func(r *nativecontract.ToolDispatchRequest) { r.Plan.Policy = nativecontract.RecoveryHold }},
		{"funding", func(r *nativecontract.ToolDispatchRequest) { r.Funding.Funding = "byok" }},
		{"budget_ref", func(r *nativecontract.ToolDispatchRequest) { r.Funding.BudgetRef = "budget-2" }},
		{"price_version", func(r *nativecontract.ToolDispatchRequest) { r.Funding.PriceVersion = "price-2" }},
		{"credential_version", func(r *nativecontract.ToolDispatchRequest) { r.Funding.CredentialVersion++ }},
		{"funding_root", func(r *nativecontract.ToolDispatchRequest) { r.Funding.BudgetRootRunID = "root-2" }},
		{"run_root", func(r *nativecontract.ToolDispatchRequest) {
			r.Fence.Run.BudgetRootRunID = "root-2"
			r.Plan.Run = r.Fence.Run
			r.Attempt.Run = r.Fence.Run
		}},
		{"session_owner", func(r *nativecontract.ToolDispatchRequest) { r.Scope.SessionOwnerID = "owner-2" }},
		{"actor", func(r *nativecontract.ToolDispatchRequest) { r.Scope.ActorUserID = "actor-2" }},
		{"principal", func(r *nativecontract.ToolDispatchRequest) { r.Scope.Principal.ID = "principal-2" }},
		{"args", func(r *nativecontract.ToolDispatchRequest) { r.Plan.Args = []byte(`{"target":"other"}`) }},
		{"expiry", func(r *nativecontract.ToolDispatchRequest) {
			r.Plan.IdempotencyExpiresAt = r.Plan.IdempotencyExpiresAt.Add(time.Hour)
		}},
		{"attempt_invocation", func(r *nativecontract.ToolDispatchRequest) { r.Attempt.InvocationID = "invocation-2" }},
		{"units", func(r *nativecontract.ToolDispatchRequest) { r.ReservationUnits++ }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fence := nativeUsageReservationFence(1)
			root := fence.Run
			root.RunID = "root-1"
			coordinator := NewInMemoryNativeToolDispatchReservation(NativeToolDispatchBudget{Root: root, Available: 10})
			coordinator.SetLiveFence(fence)
			req := nativeUsageReservationRequest(fence, root, 7)
			require.NoError(t, coordinator.ReserveAndConsume(context.Background(), req))
			changed := req
			tc.mutate(&changed)
			require.Equal(t, nativecontract.ErrConflict, nativeUsageCode(t, coordinator.ReserveAndConsume(context.Background(), changed)))
			require.NoError(t, coordinator.ReserveAndConsume(context.Background(), req), "original authorization must remain replayable")
			remaining, ok := coordinator.Remaining(root)
			require.True(t, ok)
			require.Equal(t, int64(3), remaining)
		})
	}
}

func TestNativeToolDispatchReservationRejectsInsufficientBudgetWithoutConsumingDecision(t *testing.T) {
	fence := nativeUsageReservationFence(1)
	root := fence.Run
	root.RunID = "root-1"
	coordinator := NewInMemoryNativeToolDispatchReservation(NativeToolDispatchBudget{Root: root, Available: 6})
	coordinator.SetLiveFence(fence)
	req := nativeUsageReservationRequest(fence, root, 7)

	require.Equal(t, nativecontract.ErrBudget, nativeUsageCode(t, coordinator.ReserveAndConsume(context.Background(), req)))
	remaining, ok := coordinator.Remaining(root)
	require.True(t, ok)
	require.Equal(t, int64(6), remaining)

	coordinator.SetBudget(root, 7)
	require.NoError(t, coordinator.ReserveAndConsume(context.Background(), req), "failed reservation must leave the decision unconsumed")
}

func nativeUsageReservationFence(epoch int64) nativecontract.Fence {
	return nativecontract.Fence{Run: nativecontract.RunIdentity{TenantID: 1, SessionID: "session-1", RunID: "child-1", BudgetRootRunID: "root-1"}, Owner: "worker", Epoch: epoch}
}

func nativeUsageReservationRequest(fence nativecontract.Fence, root nativecontract.RunIdentity, units int64) nativecontract.ToolDispatchRequest {
	return nativecontract.ToolDispatchRequest{
		Scope:             nativecontract.Scope{TenantID: fence.Run.TenantID, SessionOwnerID: "owner-1"},
		Fence:             fence,
		Plan:              nativecontract.ToolPlan{Version: 1, Run: fence.Run, CallID: "call-1", ModelAttemptID: "model-1", Tool: nativecontract.ToolIdentity{Name: "write", SchemaHash: "schema-1", ConfigVersion: "config-1"}, Args: []byte(`{}`), ArgsHash: "args-1", Policy: nativecontract.RecoveryIdempotent, IdempotencyKey: "provider-key-1", IdempotencyExpiresAt: time.Now().UTC().Add(time.Hour)},
		Attempt:           nativecontract.Attempt{ID: "tool-attempt-1", Run: fence.Run, Kind: nativecontract.ToolAttempt, LogicalCallID: "call-1", Number: 1, Epoch: fence.Epoch, StartedAt: time.Now().UTC()},
		DecisionReference: "pending-1",
		Funding:           nativecontract.FundingBinding{BudgetRootRunID: root.RunID},
		ReservationUnits:  units,
	}
}

func TestNativeUsageHigherRevisionSettlesOnlyCumulativeDelta(t *testing.T) {
	ledger, fence, observation := nativeUsageFixture(t)
	first, err := ledger.ObserveDelta(context.Background(), fence, observation)
	require.NoError(t, err)
	claimed, err := ledger.ClaimSettlement(context.Background(), fence, first.IntentID)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, ledger.ConfirmSettlement(context.Background(), fence, first.IntentID))
	correction := observation
	correction.Revision, correction.PromptTokens, correction.CompletionTokens, correction.TotalTokens = 2, 17, 9, 26
	delta, err := ledger.ObserveDelta(context.Background(), fence, correction)
	require.NoError(t, err)
	require.EqualValues(t, 7, delta.PromptTokens)
	require.EqualValues(t, 4, delta.CompletionTokens)
	require.EqualValues(t, 11, delta.TotalTokens)
}

func TestNativeUsageLargeIntegersRemainExactAcrossRevisionAndReplay(t *testing.T) {
	ledger, fence, observation := nativeUsageFixture(t)
	const unsafeJavaScriptInteger = int64(9_007_199_254_740_993)
	observation.PromptTokens = unsafeJavaScriptInteger
	observation.CompletionTokens = 11
	observation.TotalTokens = unsafeJavaScriptInteger + 11

	first, err := ledger.ObserveDelta(context.Background(), fence, observation)
	require.NoError(t, err)
	require.Equal(t, observation.TotalTokens, first.TotalTokens)
	claimed, err := ledger.ClaimSettlement(context.Background(), fence, first.IntentID)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, ledger.ConfirmSettlement(context.Background(), fence, first.IntentID))

	correction := observation
	correction.Revision++
	correction.PromptTokens += 7
	correction.CompletionTokens += 3
	correction.TotalTokens += 10
	delta, err := ledger.ObserveDelta(context.Background(), fence, correction)
	require.NoError(t, err)
	require.EqualValues(t, 7, delta.PromptTokens)
	require.EqualValues(t, 3, delta.CompletionTokens)
	require.EqualValues(t, 10, delta.TotalTokens)
	claimed, err = ledger.ClaimSettlement(context.Background(), fence, delta.IntentID)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, ledger.ConfirmSettlement(context.Background(), fence, delta.IntentID))

	replay, err := ledger.ObserveDelta(context.Background(), fence, correction)
	require.NoError(t, err)
	require.Zero(t, replay.TotalTokens)
	var stored struct {
		Revision, PromptTokens, CompletionTokens int64
	}
	require.NoError(t, ledger.db.Table("native_agent_usage_observations").
		Select("revision, input_tokens AS prompt_tokens, output_tokens AS completion_tokens").
		Where("tenant_id=? AND run_id=? AND attempt_id=? AND observation_id=?", 1, "run-1", observation.AttemptID, observation.ObservationID).
		Take(&stored).Error)
	require.Equal(t, correction.Revision, stored.Revision)
	require.Equal(t, correction.PromptTokens, stored.PromptTokens)
	require.Equal(t, correction.CompletionTokens, stored.CompletionTokens)
	var payload string
	require.NoError(t, ledger.db.Table("native_agent_commit_intents").
		Select("payload").
		Where("tenant_id=? AND run_id=? AND intent_id=?", 1, "run-1", delta.IntentID).
		Scan(&payload).Error)
	require.Contains(t, payload, strconv.FormatInt(correction.PromptTokens, 10))
}

func TestNativeUsageCollidingLegacyKeysPersistSeparateSettlementIntents(t *testing.T) {
	ledger, fence, observation := nativeUsageFixture(t)
	first := observation
	first.AttemptID, first.ObservationID = "a:b", "c"
	second := observation
	second.AttemptID, second.ObservationID = "a", "b:c"
	require.Equal(t, first.AttemptID+":"+first.ObservationID+":1", second.AttemptID+":"+second.ObservationID+":1")
	require.NoError(t, ledger.db.Exec(`INSERT INTO native_agent_attempts (tenant_id, run_id, attempt_id, attempt_number, lease_epoch) VALUES (?, ?, ?, ?, ?)`, 1, "run-1", first.AttemptID, 1, fence.Epoch).Error)
	require.NoError(t, ledger.db.Exec(`INSERT INTO native_agent_attempts (tenant_id, run_id, attempt_id, attempt_number, lease_epoch) VALUES (?, ?, ?, ?, ?)`, 1, "run-1", second.AttemptID, 2, fence.Epoch).Error)

	firstDelta, err := ledger.ObserveDelta(context.Background(), fence, first)
	require.NoError(t, err)
	secondDelta, err := ledger.ObserveDelta(context.Background(), fence, second)
	require.NoError(t, err)
	require.NotEqual(t, firstDelta.IntentID, secondDelta.IntentID)
	for _, delta := range []NativeUsageDelta{firstDelta, secondDelta} {
		claimed, err := ledger.ClaimSettlement(context.Background(), fence, delta.IntentID)
		require.NoError(t, err)
		require.True(t, claimed)
		require.NoError(t, ledger.ConfirmSettlement(context.Background(), fence, delta.IntentID))
	}
	var receipts int64
	require.NoError(t, ledger.db.Table("native_agent_usage_observations").Count(&receipts).Error)
	require.EqualValues(t, 2, receipts)
}

func TestNativeUsageRejectsChangedReplayStaleRevisionAndStaleFence(t *testing.T) {
	ledger, fence, observation := nativeUsageFixture(t)
	require.NoError(t, ledger.Observe(context.Background(), fence, observation))
	changed := observation
	changed.ProviderRequestID = "changed-provider-request"
	require.Equal(t, nativecontract.ErrConflict, nativeUsageCode(t, ledger.Observe(context.Background(), fence, changed)))
	stale := observation
	stale.Revision = 0
	require.Equal(t, nativecontract.ErrConflict, nativeUsageCode(t, ledger.Observe(context.Background(), fence, stale)))
	fence.Epoch--
	require.Equal(t, nativecontract.ErrLeaseLost, nativeUsageCode(t, ledger.Observe(context.Background(), fence, observation)))
}

func TestNativeUsageRejectsCumulativeRegression(t *testing.T) {
	ledger, fence, observation := nativeUsageFixture(t)
	require.NoError(t, ledger.Observe(context.Background(), fence, observation))
	regression := observation
	regression.Revision, regression.PromptTokens, regression.CompletionTokens, regression.TotalTokens = 2, 9, 5, 14
	require.Equal(t, nativecontract.ErrConflict, nativeUsageCode(t, ledger.Observe(context.Background(), fence, regression)))
}

func TestNativeUsageConcurrentIdenticalCallbackConverges(t *testing.T) {
	ledger, fence, observation := nativeUsageFixture(t)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- ledger.Observe(context.Background(), fence, observation) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var count int64
	require.NoError(t, ledger.db.Table("native_agent_usage_observations").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestNativeUsagePendingRevisionBlocksLaterRevisionUntilConfirmed(t *testing.T) {
	ledger, fence, first := nativeUsageFixture(t)
	delta, err := ledger.ObserveDelta(context.Background(), fence, first)
	require.NoError(t, err)
	later := first
	later.Revision, later.PromptTokens, later.CompletionTokens, later.TotalTokens = 2, 12, 6, 18
	require.Equal(t, nativecontract.ErrConflict, nativeUsageCode(t, ledger.Observe(context.Background(), fence, later)))
	claimed, err := ledger.ClaimSettlement(context.Background(), fence, delta.IntentID)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, ledger.ConfirmSettlement(context.Background(), fence, delta.IntentID))
	require.NoError(t, ledger.Observe(context.Background(), fence, later))
}

func TestNativeUsageCorruptPendingIntentReturnsStoreFailure(t *testing.T) {
	ledger, fence, observation := nativeUsageFixture(t)
	delta, err := ledger.ObserveDelta(context.Background(), fence, observation)
	require.NoError(t, err)
	require.NoError(t, ledger.db.Exec(`UPDATE native_agent_commit_intents SET payload = ? WHERE tenant_id=? AND run_id=? AND intent_id=?`, "{", 1, "run-1", delta.IntentID).Error)
	_, err = ledger.ObserveDelta(context.Background(), fence, observation)
	require.Equal(t, nativecontract.ErrStore, nativeUsageCode(t, err))
}

func TestNativeUsageApplyingIntentIsReclaimedByNewLeaseEpoch(t *testing.T) {
	ledger, fence, observation := nativeUsageFixture(t)
	delta, err := ledger.ObserveDelta(context.Background(), fence, observation)
	require.NoError(t, err)
	claimed, err := ledger.ClaimSettlement(context.Background(), fence, delta.IntentID)
	require.NoError(t, err)
	require.True(t, claimed)
	// Simulate a crashed worker: the run lease is recovered by a new owner and
	// epoch, which is the durable drain authority for an applying intent.
	require.NoError(t, ledger.db.Exec(`UPDATE native_agent_runs SET lease_owner=?, lease_epoch=?, lease_expires_at=? WHERE tenant_id=? AND run_id=?`, "worker-2", fence.Epoch+1, time.Now().Add(time.Hour), 1, "run-1").Error)
	recovered := fence
	recovered.Owner = "worker-2"
	recovered.Epoch++
	claimed, err = ledger.ClaimSettlement(context.Background(), recovered, delta.IntentID)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, ledger.ConfirmSettlement(context.Background(), recovered, delta.IntentID))
}
