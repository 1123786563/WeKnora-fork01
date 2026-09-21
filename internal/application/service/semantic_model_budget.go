package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"
)

var ErrSemanticModelRatesUnavailable = errors.New("semantic_model_rates_unavailable")

// SemanticModelBudgetAdapter keeps the semantic pre-send boundary separate
// from ExecutionGate.Begin, whose historical contract marks dispatch itself.
// It is deliberately consumer-owned: other ExecutionGate users retain their
// existing irreversible Begin behaviour.
type SemanticModelBudgetAdapter struct {
	store *repocommercial.BudgetStore
	gate  domain.ExecutionGate
	rates repocommercial.RateResolver
	usage *repocommercial.UsageStore
	// Test seams keep gateway ordering observable without changing the
	// production commercial interfaces. They are intentionally package-local.
	reserveFunc               func(context.Context, semanticCapability) (domain.Reservation, error)
	releaseFunc               func(context.Context, semanticCapability, domain.Reservation) error
	markDispatchedFunc        func(context.Context, semanticCapability, domain.Reservation) error
	finishFunc                func(context.Context, semanticCapability, domain.Reservation, int64, int64) error
	releaseCalls, finishCalls int
}

// WithUsageStore installs the durable observation ledger used for BYOK calls.
// Platform calls are recorded by ExecutionGate.Finish in its settlement
// transaction; recording them again here would double-count a physical call.
func (b *SemanticModelBudgetAdapter) WithUsageStore(usage *repocommercial.UsageStore) *SemanticModelBudgetAdapter {
	b.usage = usage
	return b
}

func NewSemanticModelBudgetAdapter(store *repocommercial.BudgetStore, gate domain.ExecutionGate, rates repocommercial.RateResolver) *SemanticModelBudgetAdapter {
	return &SemanticModelBudgetAdapter{store: store, gate: gate, rates: rates}
}

func (b *SemanticModelBudgetAdapter) reserve(ctx context.Context, c semanticCapability) (domain.Reservation, error) {
	if b != nil && b.reserveFunc != nil {
		return b.reserveFunc(ctx, c)
	}
	if b == nil || b.rates == nil || c.priceVersion == "" {
		return domain.Reservation{}, ErrSemanticModelRatesUnavailable
	}
	if rates, err := b.rates(c.priceVersion); err != nil || rates.Version != c.priceVersion {
		return domain.Reservation{}, ErrSemanticModelRatesUnavailable
	}
	if c.funding == domain.FundingBYOK {
		// BYOK waives platform credits, not immutable price/rate binding or
		// durable raw-usage accounting. Admit it only after that binding.
		if b.usage == nil {
			return domain.Reservation{}, ErrSemanticModelRatesUnavailable
		}
		return domain.Reservation{}, nil
	}
	if c.funding != domain.FundingPlatform || b.store == nil {
		return domain.Reservation{}, ErrSemanticModelRatesUnavailable
	}
	return b.store.Reserve(ctx, domain.BudgetRequest{TenantID: c.owner, RunID: c.runID, Key: c.callID, Upper: domain.Credits(c.upper), Deadline: c.deadline})
}
func (b *SemanticModelBudgetAdapter) release(ctx context.Context, c semanticCapability, r domain.Reservation) error {
	if b != nil && b.releaseFunc != nil {
		return b.releaseFunc(ctx, c, r)
	}
	if c.funding != domain.FundingPlatform || r.ID == "" {
		return nil
	}
	return b.store.ReleaseReservation(ctx, c.owner, r.ID)
}
func (b *SemanticModelBudgetAdapter) markDispatched(ctx context.Context, c semanticCapability, r domain.Reservation) error {
	if b != nil && b.markDispatchedFunc != nil {
		return b.markDispatchedFunc(ctx, c, r)
	}
	if c.funding != domain.FundingPlatform {
		return nil
	}
	if r.ID == "" {
		return ErrSemanticModelRatesUnavailable
	}
	return b.store.MarkReservationDispatched(ctx, c.owner, r.ID)
}
func (b *SemanticModelBudgetAdapter) finish(ctx context.Context, c semanticCapability, r domain.Reservation, input, output int64) error {
	if b != nil && b.finishFunc != nil {
		return b.finishFunc(ctx, c, r, input, output)
	}
	fact := domain.UsageFact{TenantID: c.owner, RunID: c.runID, CallID: c.callID, AttemptID: c.callID, Funding: c.funding, Service: domain.ServiceModel, PriceVersion: c.priceVersion, Revision: 1, OccurredAt: time.Now().UTC(), Dimensions: map[string]int64{domain.DimensionModel: input + output}, Status: domain.UsageStatusFinal}
	if c.funding == domain.FundingBYOK {
		if b == nil || b.usage == nil || b.rates == nil {
			return ErrSemanticModelRatesUnavailable
		}
		if rates, err := b.rates(c.priceVersion); err != nil || rates.Version != c.priceVersion {
			return ErrSemanticModelRatesUnavailable
		}
		return b.usage.RecordRawModelUsage(ctx, fact)
	}
	if b == nil || b.gate == nil || b.rates == nil || r.ID == "" {
		return ErrSemanticModelRatesUnavailable
	}
	// Resolve before Finish as well as reserve. This refuses a missing or
	// changed immutable version instead of accepting a fabricated zero charge.
	if rates, err := b.rates(c.priceVersion); err != nil || rates.Version != c.priceVersion {
		return ErrSemanticModelRatesUnavailable
	}
	return b.gate.Finish(ctx, r.ID, fact)
}

func semanticBudgetFailure(err error) error { return fmt.Errorf("semantic model budget: %w", err) }
