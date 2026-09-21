package service

import (
	"context"
	"reflect"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
)

type NativeUsageDelta = repository.NativeUsageDelta
type NativeUsageStore interface {
	ObserveDelta(context.Context, nativecontract.Fence, nativecontract.UsageObservation) (NativeUsageDelta, error)
	ConfirmSettlement(context.Context, nativecontract.Fence, string) error
	ClaimSettlement(context.Context, nativecontract.Fence, string) (bool, error)
	ReleaseSettlement(context.Context, nativecontract.Fence, string) error
}
type NativeUsageFunding interface {
	Funding(context.Context, nativecontract.RunIdentity) (nativecontract.FundingBinding, error)
}

// NativeUsageBudget owns the commercial reservation lifecycle. It receives the
// root identity for every child, so implementation may use one atomic root CAS.
type NativeUsageBudget interface {
	Reserve(context.Context, nativecontract.RunIdentity, string, int64) error
	Settle(context.Context, nativecontract.RunIdentity, string, NativeUsageDelta) error
	MarkUnknown(context.Context, nativecontract.RunIdentity, string) error
}
type NativeUsageService struct {
	store   NativeUsageStore
	funding NativeUsageFunding
	budget  NativeUsageBudget
}

func NewNativeUsageService(store NativeUsageStore, funding NativeUsageFunding, budget NativeUsageBudget) *NativeUsageService {
	return &NativeUsageService{store: store, funding: funding, budget: budget}
}

var _ nativecontract.UsageLedger = (*NativeUsageService)(nil)

func (s *NativeUsageService) BudgetRoot(o nativecontract.UsageObservation) string {
	if o.Funding.BudgetRootRunID != "" {
		return o.Funding.BudgetRootRunID
	}
	return o.Run.BudgetRootRunID
}
func (s *NativeUsageService) Reserve(ctx context.Context, fence nativecontract.Fence, key string, units int64) error {
	if s == nil || s.budget == nil || units <= 0 || key == "" {
		return nativeUsageServiceFailure(nativecontract.ErrInvalid, "usage reservation is incomplete")
	}
	funding, err := s.authoritativeFunding(ctx, fence.Run)
	if err != nil {
		return err
	}
	root := fence.Run
	root.RunID = funding.BudgetRootRunID
	if root.RunID == "" {
		root.RunID = fence.Run.BudgetRootRunID
	}
	if root.RunID == "" {
		root.RunID = fence.Run.RunID
	}
	return s.budget.Reserve(ctx, root, key, units)
}
func (s *NativeUsageService) Observe(ctx context.Context, fence nativecontract.Fence, o nativecontract.UsageObservation) error {
	if s == nil || s.store == nil || s.budget == nil {
		return nativeUsageServiceFailure(nativecontract.ErrStore, "usage service is unavailable")
	}
	funding, err := s.authoritativeFunding(ctx, fence.Run)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(o.Funding, funding) {
		return nativeUsageServiceFailure(nativecontract.ErrForbidden, "usage funding is not server owned")
	}
	delta, err := s.store.ObserveDelta(ctx, fence, o)
	if err != nil {
		return err
	}
	root := fence.Run
	root.RunID = funding.BudgetRootRunID
	if root.RunID == "" {
		root.RunID = fence.Run.BudgetRootRunID
	}
	if root.RunID == "" {
		root.RunID = fence.Run.RunID
	}
	key := repository.NativeUsageRevisionIdentity(o)
	if !delta.Pending {
		return nil
	}
	claimed, err := s.store.ClaimSettlement(ctx, fence, delta.IntentID)
	if err != nil {
		return err
	}
	if !claimed {
		return nativeUsageServiceFailure(nativecontract.ErrConflict, "usage settlement is being recovered by another worker")
	}
	if o.AccountingStatus == "partial" {
		// A partial receipt is not a zero-cost receipt: settle the provider's
		// reported cumulative delta, then retain the unresolved portion for
		// commercial reconciliation. Distinct idempotency keys let a replay
		// complete either side after a failure without conflating them.
		if delta.TotalTokens != 0 {
			if err := s.budget.Settle(ctx, root, key+":known", delta); err != nil {
				if releaseErr := s.store.ReleaseSettlement(ctx, fence, delta.IntentID); releaseErr != nil {
					return nativeUsageServiceFailure(nativecontract.ErrStore, "partial usage settlement and claim release failed")
				}
				return err
			}
		}
		if err := s.budget.MarkUnknown(ctx, root, key+":unknown"); err != nil {
			if releaseErr := s.store.ReleaseSettlement(ctx, fence, delta.IntentID); releaseErr != nil {
				return nativeUsageServiceFailure(nativecontract.ErrStore, "partial usage reconciliation and claim release failed")
			}
			return err
		}
		return s.store.ConfirmSettlement(ctx, fence, delta.IntentID)
	}
	if o.AccountingStatus == "unknown" {
		if err := s.budget.MarkUnknown(ctx, root, key+":unknown"); err != nil {
			if releaseErr := s.store.ReleaseSettlement(ctx, fence, delta.IntentID); releaseErr != nil {
				return nativeUsageServiceFailure(nativecontract.ErrStore, "usage reconciliation and claim release failed")
			}
			return err
		}
		return s.store.ConfirmSettlement(ctx, fence, delta.IntentID)
	}
	if o.AccountingStatus != "known" || delta.TotalTokens == 0 {
		return s.store.ConfirmSettlement(ctx, fence, delta.IntentID)
	}
	if err := s.budget.Settle(ctx, root, key+":known", delta); err != nil {
		if releaseErr := s.store.ReleaseSettlement(ctx, fence, delta.IntentID); releaseErr != nil {
			return nativeUsageServiceFailure(nativecontract.ErrStore, "usage settlement and claim release failed")
		}
		return err
	}
	return s.store.ConfirmSettlement(ctx, fence, delta.IntentID)
}
func (s *NativeUsageService) authoritativeFunding(ctx context.Context, run nativecontract.RunIdentity) (nativecontract.FundingBinding, error) {
	if s.funding == nil {
		return nativecontract.FundingBinding{}, nativeUsageServiceFailure(nativecontract.ErrStore, "usage funding authority is unavailable")
	}
	f, err := s.funding.Funding(ctx, run)
	if err != nil {
		return nativecontract.FundingBinding{}, nativeUsageServiceFailure(nativecontract.ErrStore, "usage funding authority is unavailable")
	}
	if f.BudgetRootRunID == "" {
		f.BudgetRootRunID = run.BudgetRootRunID
	}
	return f, nil
}
func nativeUsageServiceFailure(code nativecontract.ErrorCode, message string) error {
	return &nativecontract.Failure{Code: code, Message: message, Effect: nativecontract.EffectNotDispatched}
}
