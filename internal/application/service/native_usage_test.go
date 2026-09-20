package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
)

type nativeUsageStoreFake struct {
	mu        sync.Mutex
	seen      map[string]nativecontract.UsageObservation
	confirmed map[string]bool
	claimed   map[string]bool
}

func (s *nativeUsageStoreFake) ObserveDelta(_ context.Context, _ nativecontract.Fence, o nativecontract.UsageObservation) (NativeUsageDelta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := o.AttemptID + ":" + o.ObservationID
	old, ok := s.seen[k]
	intent := "usage-settlement:" + k + ":" + fmt.Sprint(o.Revision)
	if ok && old.Revision == o.Revision {
		if !reflect.DeepEqual(old, o) {
			return NativeUsageDelta{}, &nativecontract.Failure{Code: nativecontract.ErrConflict}
		}
		if s.confirmed[intent] {
			return NativeUsageDelta{}, nil
		}
		return NativeUsageDelta{TotalTokens: o.TotalTokens, PromptTokens: o.PromptTokens, CompletionTokens: o.CompletionTokens, IntentID: intent, Pending: true}, nil
	}
	if ok && old.Revision > o.Revision {
		return NativeUsageDelta{}, &nativecontract.Failure{Code: nativecontract.ErrConflict}
	}
	s.seen[k] = o
	return NativeUsageDelta{TotalTokens: o.TotalTokens - old.TotalTokens, PromptTokens: o.PromptTokens - old.PromptTokens, CompletionTokens: o.CompletionTokens - old.CompletionTokens, IntentID: intent, Pending: true}, nil
}
func (s *nativeUsageStoreFake) ConfirmSettlement(_ context.Context, _ nativecontract.Fence, intent string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.confirmed[intent] = true
	delete(s.claimed, intent)
	return nil
}
func (s *nativeUsageStoreFake) ClaimSettlement(_ context.Context, _ nativecontract.Fence, intent string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimed[intent] || s.confirmed[intent] {
		return false, nil
	}
	s.claimed[intent] = true
	return true, nil
}
func (s *nativeUsageStoreFake) ReleaseSettlement(_ context.Context, _ nativecontract.Fence, intent string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.claimed, intent)
	return nil
}

type nativeUsageFundingFake struct{ funding nativecontract.FundingBinding }

func (f nativeUsageFundingFake) Funding(context.Context, nativecontract.RunIdentity) (nativecontract.FundingBinding, error) {
	return f.funding, nil
}

type nativeUsageBudgetFake struct {
	mu                          sync.Mutex
	reserves, settles, unknowns int
	roots                       []string
	settleErr                   error
	unknownErr                  error
	remaining                   int64
}

func (b *nativeUsageBudgetFake) Reserve(_ context.Context, root nativecontract.RunIdentity, _ string, units int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.roots = append(b.roots, root.RunID)
	if units > b.remaining {
		return &nativecontract.Failure{Code: nativecontract.ErrBudget}
	}
	b.remaining -= units
	b.reserves++
	return nil
}
func (b *nativeUsageBudgetFake) Settle(_ context.Context, root nativecontract.RunIdentity, _ string, delta NativeUsageDelta) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.roots = append(b.roots, root.RunID)
	if b.settleErr != nil {
		return b.settleErr
	}
	b.settles++
	return nil
}
func (b *nativeUsageBudgetFake) MarkUnknown(_ context.Context, root nativecontract.RunIdentity, _ string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.roots = append(b.roots, root.RunID)
	if b.unknownErr != nil {
		return b.unknownErr
	}
	b.unknowns++
	return nil
}

func nativeUsageServiceObservation() nativecontract.UsageObservation {
	return nativecontract.UsageObservation{Version: 1, Run: nativecontract.RunIdentity{TenantID: 1, RunID: "child", BudgetRootRunID: "root"}, AttemptID: "a", ObservationID: "o", ProviderRequestID: "p", Revision: 1, PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10, AccountingStatus: "known", Funding: nativecontract.FundingBinding{BudgetRef: "budget", BudgetRootRunID: "root", Funding: "platform", Service: "model", PriceVersion: "v1"}, OccurredAt: time.Now().UTC()}
}
func nativeUsageServiceFence() nativecontract.Fence {
	return nativecontract.Fence{Run: nativeUsageServiceObservation().Run, Owner: "worker", Epoch: 1}
}

func TestNativeUsageServiceSharesParentBudgetAndSettlesFailedUsageOnce(t *testing.T) {
	o := nativeUsageServiceObservation()
	store := &nativeUsageStoreFake{seen: map[string]nativecontract.UsageObservation{}, confirmed: map[string]bool{}, claimed: map[string]bool{}}
	budget := &nativeUsageBudgetFake{remaining: 10}
	svc := NewNativeUsageService(store, nativeUsageFundingFake{funding: o.Funding}, budget)
	require.NoError(t, svc.Reserve(context.Background(), nativeUsageServiceFence(), "call", 10))
	require.NoError(t, svc.Observe(context.Background(), nativeUsageServiceFence(), o))
	require.NoError(t, svc.Observe(context.Background(), nativeUsageServiceFence(), o))
	require.Equal(t, 1, budget.settles)
	require.Equal(t, []string{"root", "root"}, budget.roots)
	require.Equal(t, "root", svc.BudgetRoot(o))
}

func TestNativeUsageServiceRacingChildrenCannotOverspendRoot(t *testing.T) {
	o := nativeUsageServiceObservation()
	store := &nativeUsageStoreFake{seen: map[string]nativecontract.UsageObservation{}, confirmed: map[string]bool{}, claimed: map[string]bool{}}
	budget := &nativeUsageBudgetFake{remaining: 10}
	svc := NewNativeUsageService(store, nativeUsageFundingFake{funding: o.Funding}, budget)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, call := range []string{"child-a", "child-b"} {
		wg.Add(1)
		go func(call string) {
			defer wg.Done()
			results <- svc.Reserve(context.Background(), nativeUsageServiceFence(), call, 7)
		}(call)
	}
	wg.Wait()
	close(results)
	var ok, exhausted int
	for err := range results {
		if err == nil {
			ok++
		} else {
			var f *nativecontract.Failure
			if errors.As(err, &f) && f.Code == nativecontract.ErrBudget {
				exhausted++
			}
		}
	}
	require.Equal(t, 1, ok)
	require.Equal(t, 1, exhausted)
}

func TestNativeUsageServiceUnknownIsReconciledNotFreedAndFundingIsServerOwned(t *testing.T) {
	o := nativeUsageServiceObservation()
	store := &nativeUsageStoreFake{seen: map[string]nativecontract.UsageObservation{}, confirmed: map[string]bool{}, claimed: map[string]bool{}}
	budget := &nativeUsageBudgetFake{remaining: 10}
	svc := NewNativeUsageService(store, nativeUsageFundingFake{funding: o.Funding}, budget)
	o.AccountingStatus = "unknown"
	require.NoError(t, svc.Observe(context.Background(), nativeUsageServiceFence(), o))
	require.Equal(t, 1, budget.unknowns)
	require.Equal(t, []string{"root"}, budget.roots)
	require.Zero(t, budget.settles)
	forged := o
	forged.Funding.BudgetRootRunID = "other"
	require.Equal(t, nativecontract.ErrForbidden, nativeUsageFailureCode(t, svc.Observe(context.Background(), nativeUsageServiceFence(), forged)))
}

func TestNativeUsageServiceBudgetFailureReplaysPendingSettlement(t *testing.T) {
	o := nativeUsageServiceObservation()
	store := &nativeUsageStoreFake{seen: map[string]nativecontract.UsageObservation{}, confirmed: map[string]bool{}, claimed: map[string]bool{}}
	budget := &nativeUsageBudgetFake{remaining: 10, settleErr: errors.New("budget unavailable")}
	svc := NewNativeUsageService(store, nativeUsageFundingFake{funding: o.Funding}, budget)
	require.Error(t, svc.Observe(context.Background(), nativeUsageServiceFence(), o))
	budget.settleErr = nil
	require.NoError(t, svc.Observe(context.Background(), nativeUsageServiceFence(), o))
	require.Equal(t, 1, budget.settles, "pending durable intent must replay after a failed settlement")
}

func TestNativeUsageServiceZeroUnknownCreatesReconciliationIntent(t *testing.T) {
	o := nativeUsageServiceObservation()
	o.PromptTokens, o.CompletionTokens, o.TotalTokens, o.AccountingStatus = 0, 0, 0, "unknown"
	store := &nativeUsageStoreFake{seen: map[string]nativecontract.UsageObservation{}, confirmed: map[string]bool{}, claimed: map[string]bool{}}
	budget := &nativeUsageBudgetFake{remaining: 10}
	svc := NewNativeUsageService(store, nativeUsageFundingFake{funding: o.Funding}, budget)
	require.NoError(t, svc.Observe(context.Background(), nativeUsageServiceFence(), o))
	require.Equal(t, 1, budget.unknowns)
}

func TestNativeUsageServiceFailedUnknownReplaysPendingIntent(t *testing.T) {
	o := nativeUsageServiceObservation()
	o.AccountingStatus = "unknown"
	store := &nativeUsageStoreFake{seen: map[string]nativecontract.UsageObservation{}, confirmed: map[string]bool{}, claimed: map[string]bool{}}
	budget := &nativeUsageBudgetFake{remaining: 10, unknownErr: errors.New("reconciliation unavailable")}
	svc := NewNativeUsageService(store, nativeUsageFundingFake{funding: o.Funding}, budget)
	require.Error(t, svc.Observe(context.Background(), nativeUsageServiceFence(), o))
	budget.unknownErr = nil
	require.NoError(t, svc.Observe(context.Background(), nativeUsageServiceFence(), o))
	require.Equal(t, 1, budget.unknowns)
}

func TestNativeUsageServiceConcurrentDuplicateHasOneBudgetEffect(t *testing.T) {
	o := nativeUsageServiceObservation()
	store := &nativeUsageStoreFake{seen: map[string]nativecontract.UsageObservation{}, confirmed: map[string]bool{}, claimed: map[string]bool{}}
	budget := &nativeUsageBudgetFake{remaining: 10}
	svc := NewNativeUsageService(store, nativeUsageFundingFake{funding: o.Funding}, budget)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- svc.Observe(context.Background(), nativeUsageServiceFence(), o) }()
	}
	wg.Wait()
	close(errs)
	var success, conflict int
	for err := range errs {
		if err == nil {
			success++
			continue
		}
		if nativeUsageFailureCode(t, err) == nativecontract.ErrConflict {
			conflict++
		}
	}
	require.Equal(t, 2, success+conflict)
	require.Equal(t, 1, budget.settles)
}

func TestNativeUsageServicePartialUsageIsReconciled(t *testing.T) {
	o := nativeUsageServiceObservation()
	o.AccountingStatus = "partial"
	store := &nativeUsageStoreFake{seen: map[string]nativecontract.UsageObservation{}, confirmed: map[string]bool{}, claimed: map[string]bool{}}
	budget := &nativeUsageBudgetFake{remaining: 10}
	svc := NewNativeUsageService(store, nativeUsageFundingFake{funding: o.Funding}, budget)
	require.NoError(t, svc.Observe(context.Background(), nativeUsageServiceFence(), o))
	require.Equal(t, 1, budget.unknowns)
	require.Zero(t, budget.settles)
}

func nativeUsageFailureCode(t *testing.T, err error) nativecontract.ErrorCode {
	t.Helper()
	var f *nativecontract.Failure
	require.True(t, errors.As(err, &f))
	return f.Code
}
