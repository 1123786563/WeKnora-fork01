package repository

import (
	"context"
	"errors"
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
	replay, err := ledger.ObserveDelta(context.Background(), fence, observation)
	require.NoError(t, err)
	require.Zero(t, replay.TotalTokens)
	var count int64
	require.NoError(t, ledger.db.Table("native_agent_usage_observations").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestNativeUsageHigherRevisionSettlesOnlyCumulativeDelta(t *testing.T) {
	ledger, fence, observation := nativeUsageFixture(t)
	_, err := ledger.ObserveDelta(context.Background(), fence, observation)
	require.NoError(t, err)
	correction := observation
	correction.Revision, correction.PromptTokens, correction.CompletionTokens, correction.TotalTokens = 2, 17, 9, 26
	delta, err := ledger.ObserveDelta(context.Background(), fence, correction)
	require.NoError(t, err)
	require.EqualValues(t, 7, delta.PromptTokens)
	require.EqualValues(t, 4, delta.CompletionTokens)
	require.EqualValues(t, 11, delta.TotalTokens)
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
