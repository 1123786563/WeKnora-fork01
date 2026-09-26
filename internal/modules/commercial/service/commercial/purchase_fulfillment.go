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
	// fulfillmentDrainInterval mirrors FulfillmentService's background pass
	// cadence (the unit of the D7 total-budget pass math).
	fulfillmentDrainInterval = 30 * time.Second
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
	// drainInterval is the FulfillmentService pass cadence (30s): one
	// drain pass costs roughly interval+firstPass of wall clock, the unit
	// the total-budget pass math uses.
	drainInterval time.Duration
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
		db:            db,
		orders:        repocommercial.NewOrderStore(db),
		platform:      platform,
		now:           time.Now,
		tick:          settleObserveTick,
		firstPass:     settleObserveFirstPass,
		budget:        settleObserveTotalBudget,
		drainInterval: fulfillmentDrainInterval,
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

	// (D7 total budget) A paid order whose activation never lands (dead
	// webhook leg, permanently unconfigured settle instrument, authority
	// outage...) must NOT spin forever: once the event's cumulative drain
	// passes exhaust the budget, surface attention (the operations face)
	// and stop driving. ONE cheap snapshot read keeps the recovery path
	// open — if the authority HAS activated, fulfillment proceeds.
	// (A-31 / F114) The budget-exhausted probe's OWN failure is classified,
	// never blanket-attention: a transient unreachable/unconfigured read
	// (429/5xx/network blip — exactly the errors steps ②/⑤ treat as
	// return-nil-pending) keeps the event pending, so a blip landing on the
	// first over-budget pass cannot mint a terminal attention record (an
	// operator refunding on it while the authority later activates would
	// open the refund+grant double-win window).
	if p.overBudget(&ev) {
		snap, serr := p.platform.ReadSnapshot(ctx, domain.SnapshotQuery{
			Kind: domain.SnapshotKindPurchase, TenantID: order.TenantID,
		})
		if serr != nil {
			return p.settleSnapshotFailure(ctx, order, serr)
		}
		if snap.Purchase == nil ||
			snap.Purchase.State != domain.PurchaseStateActive {
			return p.markActivationState(ctx, order.ID, order.TenantID, domain.FulfillmentStateAttention)
		}
		// Active: fall through — the observation loop below returns
		// immediately and the grant path proceeds.
	}

	// The verified channel transaction is the settle idempotency anchor.
	attempt, err := p.succeededAttempt(ctx, order.ID)
	if err != nil {
		return err
	}

	// ② The settle rail (provider rails only; activation is the built-in
	// webhook chain's, D2').
	if _, err := p.platform.SubmitCommand(ctx, domain.Command{
		Kind:  domain.CommandKindSettlePurchasePayment,
		Key:   domain.SettlePurchasePaymentCommandKey(domain.ExternalPurchaseSubscriptionID(order.TenantID), attempt),
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
	// (A-32 / F115) A snapshot failure here is CLASSIFIED, never re-raised
	// raw: an error return from this point used to abort the whole shared
	// drain pass (fulfillment.go Recover stops at the batch's FIRST error),
	// starving every later event — including ordinary top-up orders — and a
	// deterministic invalid-response would re-fail every pass forever.
	active, state, err := p.observeActivation(ctx, order.TenantID)
	if err != nil {
		return p.settleSnapshotFailure(ctx, order, err)
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
		// (A-32) Same classification as ③: the shared drain keeps moving.
		return p.settleSnapshotFailure(ctx, order, err)
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
	// The terminal APPLIED record overwrites an earlier transient attention
	// marker (the settle-vs-finalize window may have parked attention on the
	// same unique key before the webhook landed — the final fact wins).
	if err := p.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"state", "external_id", "expires_at", "credits", "plan_ref"}),
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

// overBudget reports whether the event's cumulative drain passes have
// exhausted the D7 total budget (each pass costs roughly one drain interval
// plus one first-pass observation window). attempt_count is the durable,
// crash-surviving pass counter the drain loop already increments.
func (p *PurchaseFulfiller) overBudget(ev *repocommercial.OutboxEvent) bool {
	if p.budget <= 0 || ev == nil {
		return false
	}
	perPass := p.drainInterval + p.firstPass
	if perPass <= 0 {
		return false
	}
	return time.Duration(ev.AttemptCount)*perPass >= p.budget
}

// observeActivation watches the authority snapshot for the webhook-driven
// activation within the FIRST-PASS window only: done or not, this returns —
// the event goes back to pending and the next drain pass replays. The
// cumulative TOTAL budget (attention past the cap) is enforced at Fulfill's
// entry via the outbox event's attempt_count — NOT here.
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

// settleSnapshotFailure maps ONE authority snapshot failure onto the
// drain-safe posture (A-32 / F115): the shared drain pass must never be
// aborted by a purchase event's authority read. context cancellation is
// the pass's own end (shutdown) and still propagates; transient platform
// failures (unreachable/unconfigured — the same sentinel split as steps
// ②/⑤) keep the event pending for the next pass; every DETERMINISTIC
// failure (invalid response — e.g. a malformed authority answer) lands the
// terminal attention record and returns nil, the file's established
// contract for a definitive purchase failure (never an error, never a
// grant).
func (p *PurchaseFulfiller) settleSnapshotFailure(ctx context.Context, order domain.Order, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err // the drain pass itself is ending: propagate
	}
	if errors.Is(err, domain.ErrPlatformUnreachable) || errors.Is(err, domain.ErrPlatformUnconfigured) {
		return nil // transient: the event stays pending, the next pass re-drives
	}
	return p.markActivationState(ctx, order.ID, order.TenantID, domain.FulfillmentStateAttention)
}

// markActivationState lands the purchase_activation record in a terminal
// non-grant state (attention/refused) — benefits never open, facts stay.
// (A-33 / F116) The Warn fires only on the FIRST landing of the record
// (RowsAffected == 1 under the OnConflict DoNothing): an event parked in
// attention replays on every 30s drain pass, and an unconditional log
// would emit the same line ~2880×/day per stuck order. The record itself
// still stays exactly-once — only the log is gated.
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
	res := p.db.WithContext(ctx).Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(&rec)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 1 {
		logger.Warnf(ctx, "[CommercialFulfillment] purchase %s activation landed %s", orderID, state)
	}
	return nil
}
