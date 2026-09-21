package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// A regression here catches a store that allocates a second quota slot when
// two workers receive the same authenticated invocation concurrently.
func TestSemanticModelInvocationClaimConvergesAndProtectsQuota(t *testing.T) {
	store := NewSemanticModelInvocationStore(newSemanticSQLiteTestDB(t))
	capability := types.SemanticModelCapability{OwnerTenantID: 7, KBID: "kb-1", ScopeRef: "scope", ScopeHash: "hash", PolicyVersion: 3, RunID: "semantic-run:test", CallID: "semantic-call:test", Funding: "byok", MaxCallsPerTask: 1, MaxInputTokensPerCall: 10, MaxOutputTokensPerCall: 20, MaxInputTokensPerTask: 10, MaxOutputTokensPerTask: 20, ExpiresAt: time.Now().Add(time.Hour).UTC()}
	require.NoError(t, store.EnsureRun(context.Background(), capability))

	var wg sync.WaitGroup
	claims := make(chan types.SemanticModelInvocationClaim, 2)
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claim, err := store.Claim(context.Background(), capability, "request-hash")
			claims <- claim
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	close(claims)
	newClaims := 0
	for claim := range claims {
		if claim.Disposition == types.SemanticModelInvocationClaimedNew {
			newClaims++
		}
	}
	require.Equal(t, 1, newClaims)

	conflicting := capability
	conflicting.CallID = "semantic-call:other"
	_, err := store.Claim(context.Background(), conflicting, "other-request")
	require.ErrorIs(t, err, ErrSemanticModelInvocationQuotaExceeded)
	_, err = store.Claim(context.Background(), capability, "changed-request")
	require.ErrorIs(t, err, ErrSemanticModelInvocationConflict)
}

// A failure known to precede provider I/O must give the quota back; unknown
// outcomes must retain it so a second provider dispatch cannot be authorized.
func TestSemanticModelInvocationTerminalTransitions(t *testing.T) {
	store := NewSemanticModelInvocationStore(newSemanticSQLiteTestDB(t))
	capability := types.SemanticModelCapability{OwnerTenantID: 7, KBID: "kb-1", ScopeRef: "scope", ScopeHash: "hash", PolicyVersion: 3, RunID: "semantic-run:terminal", CallID: "semantic-call:one", Funding: "byok", MaxCallsPerTask: 1, MaxInputTokensPerCall: 10, MaxOutputTokensPerCall: 20, MaxInputTokensPerTask: 10, MaxOutputTokensPerTask: 20, ExpiresAt: time.Now().Add(time.Hour).UTC()}
	require.NoError(t, store.EnsureRun(context.Background(), capability))
	claim, err := store.Claim(context.Background(), capability, "request-one")
	require.NoError(t, err)
	require.Equal(t, types.SemanticModelInvocationClaimedNew, claim.Disposition)
	require.NoError(t, store.FailBeforeDispatch(context.Background(), capability))

	capability.CallID = "semantic-call:two"
	claim, err = store.Claim(context.Background(), capability, "request-two")
	require.NoError(t, err)
	require.Equal(t, types.SemanticModelInvocationClaimedNew, claim.Disposition)
	require.NoError(t, store.MarkUnknown(context.Background(), capability))
	claim, err = store.Claim(context.Background(), capability, "request-two")
	require.NoError(t, err)
	require.Equal(t, types.SemanticModelInvocationUnknown, claim.Disposition)
	capability.CallID = "semantic-call:three"
	_, err = store.Claim(context.Background(), capability, "request-three")
	require.ErrorIs(t, err, ErrSemanticModelInvocationQuotaExceeded)
}

// Completion keeps a task's consumed quota, swaps the pessimistic envelope
// for observed usage, and returns the saved result on the exact replay.
func TestSemanticModelInvocationCompleteReplaysAndCountsObservedUsage(t *testing.T) {
	store := NewSemanticModelInvocationStore(newSemanticSQLiteTestDB(t))
	capability := types.SemanticModelCapability{OwnerTenantID: 7, KBID: "kb-1", ScopeRef: "scope", ScopeHash: "hash", PolicyVersion: 3, RunID: "semantic-run:complete", CallID: "semantic-call:one", Funding: "byok", MaxCallsPerTask: 2, MaxInputTokensPerCall: 10, MaxOutputTokensPerCall: 20, MaxInputTokensPerTask: 11, MaxOutputTokensPerTask: 21, ExpiresAt: time.Now().Add(time.Hour).UTC()}
	require.NoError(t, store.EnsureRun(context.Background(), capability))
	claim, err := store.Claim(context.Background(), capability, "request-one")
	require.NoError(t, err)
	require.Equal(t, types.SemanticModelInvocationClaimedNew, claim.Disposition)
	require.NoError(t, store.Complete(context.Background(), capability, types.SemanticModelInvocationResult{Result: []byte("saved"), InputTokens: 1, OutputTokens: 1}))
	replayed, err := store.Claim(context.Background(), capability, "request-one")
	require.NoError(t, err)
	require.Equal(t, types.SemanticModelInvocationCompletedReplay, replayed.Disposition)
	require.Equal(t, []byte("saved"), replayed.Result.Result)
	capability.CallID = "semantic-call:two"
	_, err = store.Claim(context.Background(), capability, "request-two")
	require.NoError(t, err)
}

func TestSemanticModelInvocationEnsureRunConvergesConcurrently(t *testing.T) {
	store := NewSemanticModelInvocationStore(newSemanticSQLiteTestDB(t))
	capability := types.SemanticModelCapability{OwnerTenantID: 7, KBID: "kb-1", ScopeHash: "hash", PolicyVersion: 3, RunID: "semantic-run:concurrent", CallID: "semantic-call:concurrent", Funding: "byok", MaxCallsPerTask: 1, MaxInputTokensPerCall: 10, MaxOutputTokensPerCall: 20, MaxInputTokensPerTask: 10, MaxOutputTokensPerTask: 20, ExpiresAt: time.Now().Add(time.Hour).UTC()}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- store.EnsureRun(context.Background(), capability) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var runs int
	require.NoError(t, store.db.Raw("SELECT COUNT(*) FROM semantic_model_invocation_runs WHERE tenant_id=? AND run_id=?", "7", capability.RunID).Scan(&runs).Error)
	require.Equal(t, 1, runs)
}

func TestSemanticModelInvocationEnsureRunRejectsExpiryBindingDrift(t *testing.T) {
	store := NewSemanticModelInvocationStore(newSemanticSQLiteTestDB(t))
	capability := types.SemanticModelCapability{OwnerTenantID: 7, KBID: "kb-1", ScopeHash: "hash", PolicyVersion: 3, RunID: "semantic-run:expiry", CallID: "semantic-call:expiry", Funding: "byok", MaxCallsPerTask: 1, MaxInputTokensPerCall: 10, MaxOutputTokensPerCall: 20, MaxInputTokensPerTask: 10, MaxOutputTokensPerTask: 20, ExpiresAt: time.Now().UTC().Truncate(time.Second).Add(time.Hour)}
	require.NoError(t, store.EnsureRun(context.Background(), capability))
	capability.ExpiresAt = capability.ExpiresAt.Add(time.Minute)
	require.ErrorIs(t, store.EnsureRun(context.Background(), capability), ErrSemanticModelInvocationConflict)
}
