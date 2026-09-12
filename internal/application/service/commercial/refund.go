package commercial

import (
	"context"
	"errors"
	"fmt"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/payment"

	"gorm.io/gorm"
)

var (
	// ErrRefundOrderMismatch rejects a refund request whose order belongs
	// to a different tenant than the authenticated caller.
	ErrRefundOrderMismatch = errors.New("refund_order_mismatch")
	// ErrInvalidRefundOrderState rejects refunds against orders that were
	// never paid (nothing to refund).
	ErrInvalidRefundOrderState = errors.New("invalid_refund_order_state")
	// ErrRefundChannelUnconfigured marks real-channel refunds blocked-env:
	// the provider is not wired yet.
	ErrRefundChannelUnconfigured = errors.New("refund_channel_unconfigured")
	// ErrRefundGatewayUnconfigured marks revocation blocked-env: the
	// commercial gateway is not wired yet.
	ErrRefundGatewayUnconfigured = errors.New("refund_gateway_unconfigured")
)

// RefundService coordinates locked commercial benefits with channel
// payouts and precise-credits revocation (C05). DEPENDENCY HONESTY: the
// P03 budget coordinator's occupancy-refundable check does not exist yet,
// so the default eligibility seam (domain.DefaultRefundEligibility)
// refuses and Approve keeps refunds in requested/reviewing with the
// reason durably recorded — it never approves into a paid-out state. When
// P03 lands it implements domain.RefundEligibilityChecker and is injected
// here; the concurrency tests inject a ready stub to exercise the
// admission transaction that P03 will share.
type RefundService struct {
	db          *gorm.DB
	refunds     *repocommercial.RefundStore
	orders      *repocommercial.OrderStore
	gateway     domain.CommercialGateway
	provider    payment.Provider
	eligibility domain.RefundEligibilityChecker
}

// NewRefundService validates its wiring. A nil gateway/provider/eligibility
// is legal (blocked-env): request creation and review recording work, real
// channel payouts and revocations surface explicit unconfigured errors
// instead of fabricating outcomes.
func NewRefundService(db *gorm.DB, gateway domain.CommercialGateway, provider payment.Provider, eligibility domain.RefundEligibilityChecker) (*RefundService, error) {
	if db == nil {
		return nil, errors.New("refund_database_missing")
	}
	return &RefundService{
		db:          db,
		refunds:     repocommercial.NewRefundStore(db),
		orders:      repocommercial.NewOrderStore(db),
		gateway:     gateway,
		provider:    provider,
		eligibility: eligibility,
	}, nil
}

// CreateRequest registers a space-scoped refund request. The tenant is the
// authenticated caller's tenant and must own the order; the order must be
// paid or fulfilled (an unpaid order has nothing to refund). credits
// defaults to the fixed book rate when omitted.
func (s *RefundService) CreateRequest(ctx context.Context, tenantID uint64, orderID, orderLineID string, amount domain.CNYFen, credits domain.Credits) (domain.RefundRequestState, error) {
	row, err := s.orders.GetOrder(ctx, orderID)
	if err != nil {
		return domain.RefundRequestState{}, err
	}
	if row.TenantID != tenantID {
		return domain.RefundRequestState{}, ErrRefundOrderMismatch
	}
	if row.State != domain.OrderStatePaid && row.State != domain.OrderStateFulfilled {
		return domain.RefundRequestState{}, ErrInvalidRefundOrderState
	}
	if amount <= 0 {
		return domain.RefundRequestState{}, repocommercial.ErrInvalidRefundRow
	}
	if credits <= 0 {
		credits = TopUpCredits(amount)
	}
	id := "rfd_" + newLeaseToken()
	if err := s.refunds.CreateRefundRequest(ctx, repocommercial.RefundRow{
		ID: id, TenantID: tenantID, OrderID: orderID, OrderLineID: orderLineID,
		AmountFen: int64(amount), CreditsMicro: int64(credits),
	}); err != nil {
		return domain.RefundRequestState{}, err
	}
	stored, err := s.refunds.GetRefund(ctx, id)
	if err != nil {
		return domain.RefundRequestState{}, err
	}
	return stored.Domain(), nil
}

// reviewBasis derives the manual approval basis the review must record:
// whether the period covered by the refunded line had not started yet or
// was already effective when the review ran.
func (s *RefundService) reviewBasis(ctx context.Context, rf repocommercial.RefundRow) string {
	var rec FulfillmentRecord
	err := s.db.WithContext(ctx).
		Where("order_id = ? AND line_id = ?", rf.OrderID, rf.OrderLineID).First(&rec).Error
	if err == nil && rec.EffectiveAt.After(time.Now()) {
		return domain.RefundBasisPeriodNotStarted
	}
	return domain.RefundBasisPeriodEffective
}

// Approve records the review decision and runs the admission transaction.
// Without a configured eligibility policy (P03 not implemented yet) it
// NEVER auto-approves: the refund stays requested/reviewing with the
// reason recorded in the ledger row, and domain.ErrRefundNotReady is
// returned so the handler can surface the honest state.
func (s *RefundService) Approve(ctx context.Context, id, reviewer string) error {
	rf, err := s.refunds.GetRefund(ctx, id)
	if err != nil {
		return err
	}
	basis := s.reviewBasis(ctx, rf)
	return s.refunds.ApproveRefund(ctx, id, reviewer, s.eligibility, basis)
}

// mapChannelState projects the C02 provider refund states onto the refund
// channel vocabulary. Anything unproven (unknown, abnormal, empty) maps to
// unknown: the caller stops at reviewing and re-queries the ORIGINAL
// refund key — an unconfirmed outcome never unlocks and never re-keys.
func mapChannelState(state payment.AttemptState) domain.RefundChannelState {
	switch state {
	case payment.StateSucceeded:
		return domain.RefundChannelSucceeded
	case payment.StatePending:
		return domain.RefundChannelPending
	case payment.StateClosed:
		return domain.RefundChannelFailed
	default:
		return domain.RefundChannelUnknown
	}
}

func (s *RefundService) succeededAttempt(ctx context.Context, orderID string) (repocommercial.PaymentAttemptRow, error) {
	var att repocommercial.PaymentAttemptRow
	err := s.db.WithContext(ctx).
		Where("order_id = ? AND state = ?", orderID, repocommercial.PaymentAttemptStateSucceeded).
		Order("id").First(&att).Error
	if err != nil {
		return repocommercial.PaymentAttemptRow{}, fmt.Errorf("%w: %v", ErrRefundChannelUnconfigured, err)
	}
	return att, nil
}

// ProcessPayouts drives every refund in a payout-eligible state one step:
// pending issues the channel refund under the refund's OWN key (retries
// reuse that key — a manual retry NEVER re-keys or duplicates the payout),
// reviewing-with-a-prior-attempt re-queries by the original refund key,
// and revocation_pending retries the precise-credits revocation only.
func (s *RefundService) ProcessPayouts(ctx context.Context) error {
	var rows []repocommercial.RefundRow
	if err := s.db.WithContext(ctx).
		Where("state IN ?", []domain.RefundState{domain.RefundStatePending, domain.RefundStateReviewing, domain.RefundStateRevocationPending}).
		Find(&rows).Error; err != nil {
		return err
	}
	for _, rf := range rows {
		if err := s.driveRefund(ctx, rf); err != nil {
			return fmt.Errorf("refund %s: %w", rf.ID, err)
		}
	}
	return nil
}

func (s *RefundService) driveRefund(ctx context.Context, rf repocommercial.RefundRow) error {
	switch rf.State {
	case domain.RefundStateRevocationPending:
		// Channel money is out; only the revocation may be retried.
		return s.revokeAllocations(ctx, rf)
	case domain.RefundStateReviewing:
		if rf.ChannelAttempts == 0 {
			// Pre-approval review (e.g. P03 not ready): no channel op.
			return nil
		}
		if s.provider == nil {
			return fmt.Errorf("%w: re-query of %s impossible", ErrRefundChannelUnconfigured, rf.ID)
		}
		// Unknown channel outcome: re-query by the ORIGINAL refund key.
		qr, err := s.provider.QueryRefund(ctx, rf.ID)
		if err != nil {
			return err
		}
		return s.applyChannelResult(ctx, rf.ID, qr.ProviderID, mapChannelState(qr.State))
	case domain.RefundStatePending:
		if s.provider == nil {
			return fmt.Errorf("%w: real-channel refunds blocked-env", ErrRefundChannelUnconfigured)
		}
		att, err := s.succeededAttempt(ctx, rf.OrderID)
		if err != nil {
			return err
		}
		res, err := s.provider.Refund(ctx, payment.RefundRequest{
			RefundID: rf.ID, ProviderID: att.MerchantOrderID, AmountFen: rf.AmountFen,
		})
		if err != nil {
			// The channel call itself was indeterminate: park the refund in
			// reviewing (attempt counted) and reconcile by querying the
			// original refund key — never a second Refund call under a new
			// key.
			_ = s.refunds.MarkRefundChannelResult(ctx, rf.ID, nil, domain.RefundChannelUnknown)
			return fmt.Errorf("channel refund indeterminate for %s: %w", rf.ID, err)
		}
		return s.applyChannelResult(ctx, rf.ID, res.ProviderID, mapChannelState(res.State))
	}
	return nil
}

func (s *RefundService) applyChannelResult(ctx context.Context, refundID, providerRefundID string, channelState domain.RefundChannelState) error {
	var pid *string
	if providerRefundID != "" {
		pid = &providerRefundID
	}
	if err := s.refunds.MarkRefundChannelResult(ctx, refundID, pid, channelState); err != nil {
		return err
	}
	rf, err := s.refunds.GetRefund(ctx, refundID)
	if err != nil {
		return err
	}
	if rf.State == domain.RefundStateRevocationPending {
		return s.revokeAllocations(ctx, rf)
	}
	return nil
}

// revokeAllocations revokes EXACTLY the locked micro amount of every lot
// this refund holds. Any failure keeps the refund in revocation_pending
// with its locks: recovery retries the revocation only — the payout is
// never repeated.
func (s *RefundService) revokeAllocations(ctx context.Context, rf repocommercial.RefundRow) error {
	if s.gateway == nil {
		return fmt.Errorf("%w: revocation for %s blocked-env", ErrRefundGatewayUnconfigured, rf.ID)
	}
	allocs, err := s.refunds.ListRefundAllocations(ctx, rf.ID)
	if err != nil {
		return err
	}
	if len(allocs) == 0 {
		// Nothing locked: nothing to revoke precisely.
		return s.refunds.CompleteRevocation(ctx, rf.ID, true)
	}
	gwCtx := domain.WithBenefitCustomer(ctx, OrderCustomerID(rf.TenantID))
	for _, a := range allocs {
		if err := s.gateway.RevokeBenefit(gwCtx, a.LotID, domain.Credits(a.LockedMicro)); err != nil {
			// Refund succeeded, revocation failed: keep revocation_pending
			// and the locks; the retry path revokes again and never pays
			// out a second time.
			return nil
		}
	}
	return s.refunds.CompleteRevocation(ctx, rf.ID, true)
}

// RetryRevocation is the manual retry entry: it retries ONLY the
// precise-credits revocation of a revocation_pending refund. It never
// re-keys and never issues another channel payout.
func (s *RefundService) RetryRevocation(ctx context.Context, id string) error {
	rf, err := s.refunds.GetRefund(ctx, id)
	if err != nil {
		return err
	}
	if rf.State != domain.RefundStateRevocationPending {
		return repocommercial.ErrInvalidRefundState
	}
	return s.revokeAllocations(ctx, rf)
}

// NOTE (batch-void fallback): voiding a whole order batch can never
// satisfy a PARTIAL refund — when a full-batch void is the only channel
// remedy, the fallback is the V03-verified compensation path (see the V03
// selected-model record), not a re-keyed refund. This service therefore
// never voids an order to serve a partial refund.
