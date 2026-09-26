package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// benefitKindPurchaseActivation marks the purchase activation record: the
// local exactly-once claim whose ExternalID carries the grant receipt. Like
// the plan_switch marker it is settled inside the database, never sent to
// the legacy gateway.
const benefitKindPurchaseActivation = "purchase_activation"

// Observation knobs (D7): the first pass watches the authority for a SHORT
// window, then returns the event to pending so the shared drain keeps
// moving; the total budget bounds how long the composition may stay
// activating before surfacing attention.
const (
	settleObserveTick        = 3 * time.Second
	settleObserveFirstPass   = 15 * time.Second
	settleObserveTotalBudget = 10 * time.Minute
)

// PurchaseFulfiller orchestrates the α dual-track fulfillment of one PAID
// purchase order (#82, R-4): drive the settle rail (the authority's own
// provider rails), observe the authority turn active (the built-in webhook
// finalize owns the transition — never this code), re-check the finalized
// invoice lines (D6'), and grant the first-period credits into the purchase
// wallet (D4). Exactly-once layers: the channel ConfirmPayment guard (one
// succeeded transaction per order), the outbox fulfill:<order> unique key,
// the adapter's already-active settle short-circuit, and THIS record's
// unique purchase_activation key.
type PurchaseFulfiller struct {
	db        *gorm.DB
	orders    *repocommercial.OrderStore
	platform  domain.CommercialPlatform
	now       func() time.Time
	tick      time.Duration
	firstPass time.Duration
	budget    time.Duration
}

// NewPurchaseFulfiller validates the wiring: a nil platform is a
// construction error (the container's blocked-env passes nil into the
// FulfillmentService instead, which routes purchase events to attention).
func NewPurchaseFulfiller(db *gorm.DB, platform domain.CommercialPlatform, _ *BillingAccountService) (*PurchaseFulfiller, error) {
	if db == nil {
		return nil, errors.New("purchase fulfiller requires a database")
	}
	if platform == nil {
		return nil, errors.New("purchase fulfiller requires a commercial platform")
	}
	return &PurchaseFulfiller{
		db:        db,
		orders:    repocommercial.NewOrderStore(db),
		platform:  platform,
		now:       time.Now,
		tick:      settleObserveTick,
		firstPass: settleObserveFirstPass,
		budget:    settleObserveTotalBudget,
	}, nil
}

// Fulfill drives one purchase fulfill event toward fulfillment. A nil
// return WITHOUT the order reaching fulfilled means "still activating" —
// the caller keeps the outbox event pending and the next drain pass
// replays (every step here is idempotent).
func (p *PurchaseFulfiller) Fulfill(ctx context.Context, ev repocommercial.OutboxEvent) error {
	var payload fulfillEventPayload
	if err := json.Unmarshal([]byte(ev.PayloadJSON), &payload); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	row, err := p.orders.GetOrder(ctx, payload.OrderID)
	if err != nil {
		return err
	}
	order := row.Domain()
	switch order.State {
	case domain.OrderStateFulfilled:
		return nil // replay after completion: nothing to do
	case domain.OrderStatePaid:
	default:
		return nil // not payable (still pending): leave for a later pass
	}

	// The frozen quote pins the plan identity, price and credit face.
	var q repocommercial.QuoteRow
	if err := p.db.WithContext(ctx).Where("id = ?", row.QuoteID).First(&q).Error; err != nil {
		return fmt.Errorf("purchase quote %s: %w", row.QuoteID, err)
	}
	var snap struct {
		PlanKey      string `json:"plan_key"`
		PlanVersion  int64  `json:"plan_version"`
		PriceFen     int64  `json:"price_fen"`
		CreditsMicro int64  `json:"credits_micro"`
		Currency     string `json:"currency"`
	}
	if err := json.Unmarshal([]byte(q.SnapshotJSON), &snap); err != nil {
		return fmt.Errorf("purchase quote snapshot %s: %w", row.QuoteID, err)
	}
	var pub repocommercial.PublicationRow
	if err := p.db.WithContext(ctx).Where("plan_key = ? AND version = ?", snap.PlanKey, snap.PlanVersion).
		First(&pub).Error; err != nil {
		return fmt.Errorf("purchase publication %s v%d: %w", snap.PlanKey, snap.PlanVersion, err)
	}

	// The verified channel transaction is the settle idempotency anchor.
	attempt, err := p.succeededAttempt(ctx, order.ID)
	if err != nil {
		return err
	}

	// ② The settle rail (provider rails only; activation is the built-in
	// webhook chain's, D2').
	if _, err := p.platform.SubmitCommand(ctx, domain.Command{
		Kind: domain.CommandKindSettlePurchasePayment,
		Key:  domain.SettlePurchasePaymentCommandKey(domain.ExternalPurchaseSubscriptionID(order.TenantID), attempt),
		Actor: "fulfiller", Reason: "purchase settle",
		Payload: domain.SettlePurchasePaymentPayload{
			TenantID:                       order.TenantID,
			ExternalCustomerID:             domain.ExternalCustomerID(order.TenantID),
			ExternalPurchaseSubscriptionID: domain.ExternalPurchaseSubscriptionID(order.TenantID),
			PlanCode:                       pub.PlanCode,
			ChannelTransaction:             attempt,
			AmountFen:                      int64(order.Amount),
			Currency:                       domain.CurrencyCNY,
		},
	}); err != nil {
		// Retryable platform failures keep the event pending (the next pass
		// re-drives; the settle command is idempotent). Definitive
		// invalid-response failures surface attention — never a grant.
		if errors.Is(err, domain.ErrPlatformUnreachable) || errors.Is(err, domain.ErrPlatformUnconfigured) {
			return nil
		}
		return p.markActivationState(ctx, order.ID, order.TenantID, domain.FulfillmentStateAttention)
	}

	// ③ The short-window cross-pass observation (D7): never block the shared
	// drain with a long poll — the first pass watches briefly, then hands
	// the event back to pending; replays converge because settle is
	// idempotent. The total budget bounds the composition before attention.
	active, state, err := p.observeActivation(ctx, order.TenantID)
	if err != nil {
		return err
	}
	if state == domain.PurchaseStateCanceled {
		// Authority canceled + local paid: benefits never open (honest
		// attention; the anomaly face belongs to #84).
		return p.markActivationState(ctx, order.ID, order.TenantID, domain.FulfillmentStateAttention)
	}
	if !active {
		return nil // still activating: the event stays pending for the next pass
	}

	// ④ The D6' line-item re-check on the finalized invoice (proration
	// aware: the gating invoice may legitimately be SMALLER than the order
	// face, never larger, never zero).
	psnap, err := p.platform.ReadSnapshot(ctx, domain.SnapshotQuery{
		Kind: domain.SnapshotKindPurchase, TenantID: order.TenantID,
	})
	if err != nil {
		return err
	}
	if psnap.Purchase == nil ||
		len(psnap.Purchase.InvoiceFees) != 1 ||
		psnap.Purchase.InvoiceFees[0].Kind != "subscription_fee" ||
		psnap.Purchase.InvoiceFees[0].AmountFen <= 0 ||
		psnap.Purchase.InvoiceFees[0].AmountFen > int64(order.Amount) ||
		psnap.Purchase.InvoicePaymentStatus != "succeeded" {
		return p.markActivationState(ctx, order.ID, order.TenantID, domain.FulfillmentStateRefused)
	}

	// ⑤ The first-period grant rides the ACTIVATION month (D4): a purchase
	// placed at month-end and activated next month belongs to the activation
	// period — the grant validation (exclusive end in the future) can only
	// pass on that identity.
	period := domain.MonthlyPeriod(p.now())
	end, err := domain.PeriodEnd(period)
	if err != nil {
		return err
	}
	grant, err := p.platform.SubmitCommand(ctx, domain.Command{
		Kind: domain.CommandKindGrantIncludedCredits,
		Key: domain.GrantCreditsCommandKey(
			domain.ExternalPurchaseSubscriptionID(order.TenantID), period),
		Actor: "fulfiller", Reason: "purchase first period",
		Payload: domain.GrantIncludedCreditsPayload{
			TenantID:           order.TenantID,
			ExternalCustomerID: domain.ExternalCustomerID(order.TenantID),
			Period:             period,
			CreditsMicro:       snap.CreditsMicro,
			ExpiresAt:          end,
			WalletName:         domain.PurchaseWalletName(order.TenantID, period),
		},
	})
	if err != nil {
		if errors.Is(err, domain.ErrPlatformUnreachable) || errors.Is(err, domain.ErrPlatformUnconfigured) {
			return nil // transient: the next pass re-drives (grant is identity-idempotent)
		}
		return p.markActivationState(ctx, order.ID, order.TenantID, domain.FulfillmentStateAttention)
	}

	// ⑥ The durable activation claim: exactly one purchase_activation
	// record carrying the grant receipt, then the fulfilled flip.
	rec := FulfillmentRecord{
		Key:         domain.FulfillmentKey(order.ID, benefitKindPurchaseActivation),
		TenantID:    order.TenantID,
		OrderID:     order.ID,
		LineID:      benefitKindPurchaseActivation,
		Kind:        benefitKindPurchaseActivation,
		PlanRef:     pub.PlanCode,
		CustomerID:  OrderCustomerID(order.TenantID),
		Credits:     snap.CreditsMicro,
		EffectiveAt: domain.FirstEffectiveAt(time.Time{}, p.now()),
		ExpiresAt:   end,
		ExternalID:  grant.ExternalID,
		State:       domain.FulfillmentStateApplied,
		LeaseUntil:  p.now(),
	}
	if err := p.db.WithContext(ctx).Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(&rec).Error; err != nil {
		return err
	}
	return p.orders.MarkFulfilled(ctx, order.ID)
}

// succeededAttempt reads the order's one succeeded channel transaction (the
// settle idempotency anchor; ConfirmPayment recorded it).
func (p *PurchaseFulfiller) succeededAttempt(ctx context.Context, orderID string) (string, error) {
	var att repocommercial.PaymentAttemptRow
	err := p.db.WithContext(ctx).
		Where("order_id = ? AND state = ?", orderID, repocommercial.PaymentAttemptStateSucceeded).
		Order("id").First(&att).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("purchase order %s has no succeeded attempt", orderID)
	}
	if err != nil {
		return "", err
	}
	if att.ProviderTransactionID == nil || *att.ProviderTransactionID == "" {
		return "", fmt.Errorf("purchase order %s succeeded attempt carries no channel transaction", orderID)
	}
	return *att.ProviderTransactionID, nil
}

// observeActivation watches the authority snapshot for the webhook-driven
// activation within the FIRST-PASS window only (D7): done or not, this
// returns — the event goes back to pending and the next drain pass replays.
// The cumulative budget is tracked by attempt_count on the outbox event
// (each drain pass increments it); when it exceeds the pass budget the
// composition surfaces attention instead of spinning forever.
func (p *PurchaseFulfiller) observeActivation(ctx context.Context, tenantID uint64) (bool, string, error) {
	deadline := time.Now().Add(p.firstPass)
	for {
		snap, err := p.platform.ReadSnapshot(ctx, domain.SnapshotQuery{
			Kind: domain.SnapshotKindPurchase, TenantID: tenantID,
		})
		if err != nil {
			return false, "", err
		}
		if snap.Purchase == nil {
			return false, "", fmt.Errorf("purchase snapshot missing for tenant %d", tenantID)
		}
		switch snap.Purchase.State {
		case domain.PurchaseStateActive:
			return true, domain.PurchaseStateActive, nil
		case domain.PurchaseStateCanceled:
			return false, domain.PurchaseStateCanceled, nil
		}
		if !time.Now().Before(deadline) {
			return false, snap.Purchase.State, nil
		}
		select {
		case <-ctx.Done():
			return false, snap.Purchase.State, ctx.Err()
		case <-time.After(p.tick):
		}
	}
}

// markActivationState lands the purchase_activation record in a terminal
// non-grant state (attention/refused) — benefits never open, facts stay.
func (p *PurchaseFulfiller) markActivationState(ctx context.Context, orderID string, tenantID uint64, state string) error {
	rec := FulfillmentRecord{
		Key:         domain.FulfillmentKey(orderID, benefitKindPurchaseActivation),
		TenantID:    tenantID,
		OrderID:     orderID,
		LineID:      benefitKindPurchaseActivation,
		Kind:        benefitKindPurchaseActivation,
		CustomerID:  OrderCustomerID(tenantID),
		EffectiveAt: domain.FirstEffectiveAt(time.Time{}, p.now()),
		ExpiresAt:   p.now(),
		State:       state,
		LeaseUntil:  p.now(),
	}
	if err := p.db.WithContext(ctx).Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(&rec).Error; err != nil {
		return err
	}
	logger.Warnf(ctx, "[CommercialFulfillment] purchase %s activation landed %s", orderID, state)
	return nil
}
