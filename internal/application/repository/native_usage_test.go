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
