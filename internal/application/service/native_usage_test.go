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
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type nativeUsageStoreFake struct {
	mu        sync.Mutex
	seen      map[string]nativecontract.UsageObservation
	confirmed map[string]bool
	claimed   map[string]bool
	deltas    map[string]NativeUsageDelta
}

func (s *nativeUsageStoreFake) ObserveDelta(_ context.Context, _ nativecontract.Fence, o nativecontract.UsageObservation) (NativeUsageDelta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deltas == nil {
		s.deltas = map[string]NativeUsageDelta{}
	}
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
		return s.deltas[intent], nil
	}
	if ok && old.Revision > o.Revision {
		return NativeUsageDelta{}, &nativecontract.Failure{Code: nativecontract.ErrConflict}
	}
	s.seen[k] = o
	delta := NativeUsageDelta{TotalTokens: o.TotalTokens - old.TotalTokens, PromptTokens: o.PromptTokens - old.PromptTokens, CompletionTokens: o.CompletionTokens - old.CompletionTokens, IntentID: intent, Pending: true}
	s.deltas[intent] = delta
	return delta, nil
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
	settledDeltas               []NativeUsageDelta
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
	b.settledDeltas = append(b.settledDeltas, delta)
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

func nativeUsageRealLedgerService(t *testing.T, budget *nativeUsageBudgetFake, funding nativecontract.FundingBinding) (*NativeUsageService, nativecontract.Fence) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:native-usage-service?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	for _, statement := range []string{
		`CREATE TABLE native_agent_runs (tenant_id INTEGER, run_id TEXT, lease_owner TEXT, lease_epoch INTEGER, lease_expires_at DATETIME, updated_at DATETIME, PRIMARY KEY (tenant_id,run_id))`,
		`CREATE TABLE native_agent_attempts (tenant_id INTEGER, run_id TEXT, attempt_id TEXT, lease_epoch INTEGER, PRIMARY KEY (tenant_id,run_id,attempt_id))`,
		`CREATE TABLE native_agent_usage_observations (tenant_id INTEGER, run_id TEXT, attempt_id TEXT, observation_id TEXT, revision INTEGER, provider TEXT, model TEXT, provider_request_id TEXT, funding_ref TEXT, budget_root_run_id TEXT, input_tokens INTEGER, output_tokens INTEGER, cached_tokens INTEGER, cache_read_tokens INTEGER, cache_create_tokens INTEGER, accounting_status TEXT, dimensions TEXT, occurred_at DATETIME, payload_hash TEXT, PRIMARY KEY (tenant_id,run_id,attempt_id,observation_id))`,
		`CREATE TABLE native_agent_commit_intents (tenant_id INTEGER, run_id TEXT, intent_id TEXT, payload_hash TEXT, lease_epoch INTEGER, state TEXT, version INTEGER, payload TEXT, terminal_status TEXT, applied_at DATETIME, PRIMARY KEY (tenant_id,run_id,intent_id))`,
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
	now := time.Now().UTC()
	require.NoError(t, db.Exec(`INSERT INTO native_agent_runs VALUES (?,?,?,?,?,?)`, 1, "child", "worker", 1, now.Add(time.Hour), now).Error)
	require.NoError(t, db.Exec(`INSERT INTO native_agent_attempts VALUES (?,?,?,?)`, 1, "child", "a", 1).Error)
	fence := nativeUsageServiceFence()
	return NewNativeUsageService(repository.NewNativeUsageLedger(db), nativeUsageFundingFake{funding: funding}, budget), fence
}

func TestNativeUsageServiceRealLedgerFailedRevisionBlocksThenSettlesDelta(t *testing.T) {
	first := nativeUsageServiceObservation()
	budget := &nativeUsageBudgetFake{remaining: 100, settleErr: errors.New("settlement unavailable")}
	svc, fence := nativeUsageRealLedgerService(t, budget, first.Funding)
	require.Error(t, svc.Observe(context.Background(), fence, first))
	later := first
	later.Revision, later.PromptTokens, later.CompletionTokens, later.TotalTokens = 2, 17, 9, 26
	require.Equal(t, nativecontract.ErrConflict, nativeUsageFailureCode(t, svc.Observe(context.Background(), fence, later)))
	budget.settleErr = nil
	require.NoError(t, svc.Observe(context.Background(), fence, first))
	require.NoError(t, svc.Observe(context.Background(), fence, later))
	require.Len(t, budget.settledDeltas, 2)
	require.EqualValues(t, 15, budget.settledDeltas[0].TotalTokens)
	require.EqualValues(t, 11, budget.settledDeltas[1].TotalTokens)
}

func nativeUsageFailureCode(t *testing.T, err error) nativecontract.ErrorCode {
	t.Helper()
	var f *nativecontract.Failure
	require.True(t, errors.As(err, &f))
	return f.Code
}
