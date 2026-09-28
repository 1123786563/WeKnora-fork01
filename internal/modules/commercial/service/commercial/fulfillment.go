package commercial

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrFulfillmentDatabaseMissing rejects wiring without durable storage:
	// fulfillment state must survive process restarts by construction.
	ErrFulfillmentDatabaseMissing = errors.New("fulfillment_database_missing")
	// ErrFulfillmentGatewayMissing rejects wiring without a benefit gateway.
	ErrFulfillmentGatewayMissing = errors.New("fulfillment_gateway_missing")
)

// fulfillmentRecordPending marks a claimed line whose outcome is not yet
// decided; the terminal states reuse the domain vocabulary (applied /
// attention / refused).
const fulfillmentRecordPending = "pending"

// FulfillmentRecord is the durable claim on one order line's benefit. The
// primary key IS the domain FulfillmentKey, so process restarts, worker
// crashes, and concurrent workers can all converge on exactly one external
// benefit per line. EffectiveAt is pinned when the record is created (the
// first attempt) and never shifted afterwards, so a top-up recovered hours
// later still covers the interval it was purchased in.
type FulfillmentRecord struct {
	Key         string    `gorm:"primaryKey;column:key"`
	TenantID    uint64    `gorm:"column:tenant_id;not null"`
	OrderID     string    `gorm:"column:order_id;not null;index"`
	LineID      string    `gorm:"column:line_id;not null"`
	Kind        string    `gorm:"column:kind;not null"`
	PlanRef     string    `gorm:"column:plan_ref;not null;default:''"`
	CustomerID  string    `gorm:"column:customer_id;not null"`
	Credits     int64     `gorm:"column:credits;not null"`
	EffectiveAt time.Time `gorm:"column:effective_at;not null"`
	ExpiresAt   time.Time `gorm:"column:expires_at;not null;default:CURRENT_TIMESTAMP"`
	ExternalID  string    `gorm:"column:external_id;not null;default:''"`
	State       string    `gorm:"column:state;not null;default:pending"`
	LeaseToken  string    `gorm:"column:lease_token;not null;default:''"`
	LeaseUntil  time.Time `gorm:"column:lease_until;not null;default:CURRENT_TIMESTAMP"`
}

func (FulfillmentRecord) TableName() string { return "commercial_fulfillment_records" }

// FulfillmentLine is one benefit line of an order as derived by the pricing
// policy. ID feeds FulfillmentKey; Kind selects the verified settlement
// path (subscription grants and top-ups never share a settlement identity).
type FulfillmentLine struct {
	ID        string
	Kind      string
	PlanRef   string
	Credits   domain.Credits
	ExpiresAt time.Time
}

// TopUpOrderLines is the default pricing policy: a paid credit order
// carries exactly one top-up line, priced at the fixed book rate of one
// Credit per CNY. Keeping the derivation here (rather than inside the
// worker loop) lets the container swap in a richer price book without
// touching recovery logic.
func TopUpOrderLines(order domain.Order) []FulfillmentLine {
	return []FulfillmentLine{{
		ID:      "credits",
		Kind:    domain.BenefitKindTopUp,
		PlanRef: "credits",
		Credits: TopUpCredits(order.Amount),
	}}
}

// TopUpCredits converts a paid CNY amount to credits at the fixed book
// rate of one Credit per CNY (credits are millionths; amounts are fen).
func TopUpCredits(amount domain.CNYFen) domain.Credits {
	return domain.Credits(amount) * 10_000
}

// OrderCustomerID derives the provider customer from the ORDER's tenant —
// never from the calling user — so the orderer leaving the space after
// payment cannot block fulfillment of what was already paid.
func OrderCustomerID(tenantID uint64) string {
	return "tenant_" + strconv.FormatUint(tenantID, 10)
}

// fulfillEventPayload mirrors the C01 outbox payload written by
// ConfirmPayment. The order row remains the source of truth: the worker
// re-reads tenant and state from storage instead of trusting the payload.
type fulfillEventPayload struct {
	OrderID     string `json:"order_id"`
	QuoteID     string `json:"quote_id,omitempty"`
	TenantID    uint64 `json:"tenant_id"`
	AttemptID   string `json:"attempt_id,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Merchant    string `json:"merchant,omitempty"`
	Transaction string `json:"transaction,omitempty"`
	AmountFen   int64  `json:"amount_fen"`
	Currency    string `json:"currency"`
}

// FulfillmentService drains the C01 fulfillment outbox into external
// benefits and flips paid orders to fulfilled only once every line carries
// an explicit external receipt. One pass:
//
//   - Leases each pending fulfill event (durable claim token), so
//     concurrent workers never process the same event at the same time.
//   - Ensures one FulfillmentRecord per order line, keyed by the unique
//     FulfillmentKey — the storage-level exactly-once identity.
//   - Calls FindBenefit FIRST: a benefit the remote already saved (e.g. an
//     apply whose response was lost) is discovered instead of re-granted.
//     Only a provable miss (ErrBenefitNotFound) leads to ApplyBenefit.
//   - Timeouts and dropped responses stay attention/unknown; business
//     refusals are failures. Neither is ever recorded as success.
//   - Marks the outbox event sent and the order fulfilled only when every
//     line has an explicit ExternalID. Paid and fulfilled stay distinct,
//     so a paid-but-not-yet-fulfilled order is visible and recoverable.
//
// Payment callbacks are never blocked by this service: they only write
// durable outbox events, and recovery runs as a background registration.
type FulfillmentService struct {
	db        *gorm.DB
	orders    *repocommercial.OrderStore
	gateway   domain.CommercialGateway
	purchaser *PurchaseFulfiller
	lines     func(domain.Order) []FulfillmentLine
	leaseTTL  time.Duration
	interval  time.Duration
	now       func() time.Time

	stopMu sync.Mutex
	stop   chan struct{}
}

// NewFulfillmentService validates its wiring and migrates the fulfillment
// record table. The gateway arriving unconfigured (blocked-env) is legal:
// recovery passes will classify its calls as unknown and leave orders paid
// until the connector is configured.
func NewFulfillmentService(db *gorm.DB, gateway domain.CommercialGateway, purchaser *PurchaseFulfiller) (*FulfillmentService, error) {
	if db == nil {
		return nil, ErrFulfillmentDatabaseMissing
	}
	if gateway == nil {
		return nil, ErrFulfillmentGatewayMissing
	}
	if err := db.AutoMigrate(&FulfillmentRecord{}); err != nil {
		return nil, err
	}
	return &FulfillmentService{
		db:        db,
		orders:    repocommercial.NewOrderStore(db),
		gateway:   gateway,
		purchaser: purchaser,
		lines:     TopUpOrderLines,
		leaseTTL:  time.Minute,
		interval:  30 * time.Second,
		now:       time.Now,
	}, nil
}

// Recover runs one full recovery pass and returns when every leased event
// has been driven to a stable state for this pass (applied+sent, or left
// pending-with-released-lease for the next pass).
func (s *FulfillmentService) Recover(ctx context.Context) error {
	now := s.now()
	events, err := s.leasePendingEvents(ctx, now)
	if err != nil {
		return err
	}
	for _, ev := range events {
		// (#84 / G3) over_payment events are DISPOSAL work, not fulfillment:
		// a dispose failure keeps its own event pending for the next pass
		// and never starves the fulfill events of the same batch (A-32
		// per-event isolation discipline) — the failure stays a Warn.
		if ev.Kind == repocommercial.OutboxKindOverPaid {
			if err := s.disposeOverPayment(ctx, ev, now); err != nil {
				logger.Warnf(ctx, "[CommercialFulfillment] over_payment dispose %s failed (stays pending): %v", ev.EventKey, err)
			}
			continue
		}
		if err := s.fulfillEvent(ctx, ev, now); err != nil {
			return fmt.Errorf("fulfill event %s: %w", ev.EventKey, err)
		}
	}
	return nil
}

// StartBackground registers the background recovery loop: one immediate
// pass, then one per interval. It returns without blocking — payment
// callbacks never wait on the gateway. Safe to call more than once.
func (s *FulfillmentService) StartBackground(ctx context.Context) {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()
	if s.stop != nil {
		return
	}
	stop := make(chan struct{})
	s.stop = stop
	go func() {
		run := func() {
			if err := s.Recover(ctx); err != nil {
				logger.Warnf(ctx, "[CommercialFulfillment] recovery pass failed: %v", err)
			}
		}
		run()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				run()
			case <-stop:
				return
			}
		}
	}()
}

// Stop terminates the background loop (registered with the container's
// resource cleaner so shutdown does not orphan the goroutine).
func (s *FulfillmentService) Stop() {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()
	if s.stop != nil {
		close(s.stop)
		s.stop = nil
	}
}

// leasePendingEvents selects eligible events and claims each with a durable
// claim token via a guarded update: only the worker whose update moves
// lease_until forward wins, so concurrent workers lease disjoint events and
// a crashed worker's lease simply expires. (#84 / G3) The lease covers BOTH
// drainable kinds — fulfill and over_payment — the latter consumed by
// disposeOverPayment instead of the fulfillment leg.
func (s *FulfillmentService) leasePendingEvents(ctx context.Context, now time.Time) ([]repocommercial.OutboxEvent, error) {
	var pending []repocommercial.OutboxEvent
	err := s.db.WithContext(ctx).
		Where("kind IN ? AND state = ? AND lease_until <= ?",
			[]string{repocommercial.OutboxKindFulfill, repocommercial.OutboxKindOverPaid},
			repocommercial.OutboxStatePending, now).
		Find(&pending).Error
	if err != nil {
		return nil, err
	}
	leased := make([]repocommercial.OutboxEvent, 0, len(pending))
	for _, ev := range pending {
		token := newLeaseToken()
		res := s.db.WithContext(ctx).
			Model(&repocommercial.OutboxEvent{}).
			Where("event_key = ? AND state = ? AND lease_until <= ?",
				ev.EventKey, repocommercial.OutboxStatePending, now).
			Updates(map[string]interface{}{
				"lease_token":   token,
				"lease_until":   now.Add(s.leaseTTL),
				"attempt_count": gorm.Expr("attempt_count + 1"),
			})
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected == 1 {
			ev.LeaseToken = token
			leased = append(leased, ev)
		}
	}
	return leased, nil
}

func (s *FulfillmentService) fulfillEvent(ctx context.Context, ev repocommercial.OutboxEvent, now time.Time) error {
	var payload fulfillEventPayload
	orderID, ok := strings.CutPrefix(ev.EventKey, repocommercial.OutboxKindFulfill+":")
	if ev.Kind != repocommercial.OutboxKindFulfill || !ok || orderID == "" || strings.Contains(orderID, ":") ||
		ev.EventKey != repocommercial.OutboxKindFulfill+":"+orderID {
		return s.quarantineFulfillEvent(ctx, ev, "invalid_event_key")
	}
	if err := json.Unmarshal([]byte(ev.PayloadJSON), &payload); err != nil {
		return s.quarantineFulfillEvent(ctx, ev, "malformed_payload")
	}
	row, err := s.orders.GetOrder(ctx, orderID)
	if err != nil {
		if errors.Is(err, repocommercial.ErrOrderNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			return s.quarantineFulfillEvent(ctx, ev, "order_missing")
		}
		return err
	}
	if payload.OrderID != orderID || payload.TenantID != row.TenantID || payload.AmountFen != row.AmountFen ||
		payload.Currency != row.Currency || (payload.QuoteID != "" && payload.QuoteID != row.QuoteID) || ev.TenantID != row.TenantID {
		return s.quarantineFulfillEvent(ctx, ev, "order_envelope_mismatch")
	}
	payload.OrderID = orderID
	payload.QuoteID = row.QuoteID
	order := row.Domain()
	// The order row — not the calling user — carries the tenant; an orderer
	// who left the space after paying cannot block fulfillment.
	if order.State == domain.OrderStateFulfilled {
		return s.completeEvent(ctx, ev, repocommercial.OutboxStateSent)
	}
	if order.State != domain.OrderStatePaid {
		// Not payable (still pending): leave the event for a later pass.
		return s.completeEvent(ctx, ev, repocommercial.OutboxStatePending)
	}
	// The outbox transaction identity is authoritative for every paid
	// purchase route. Validate it before quote reads or purchaser wiring can
	// defer deterministic disposition.
	if row.Kind == domain.OrderKindPurchase {
		_, winnerErr := winningPaymentTransaction(ctx, s.db, order, payload)
		if errors.Is(winnerErr, errInvalidWinningAttempt) || errors.Is(winnerErr, gorm.ErrRecordNotFound) {
			if row.QuoteID == "" {
				return s.quarantineFulfillEvent(ctx, ev, "invalid_winning_payment")
			}
			if err := persistPurchaseActivationState(ctx, s.db, s.now(), order.ID, order.TenantID, domain.FulfillmentStateAttention); err != nil {
				return err
			}
			return s.completeEvent(ctx, ev, repocommercial.OutboxStatePending)
		}
		if winnerErr != nil {
			return s.completeEvent(ctx, ev, repocommercial.OutboxStatePending)
		}
	}
	// (#82 D5) SUBSCRIPTION purchase orders route to the PurchaseFulfiller
	// — the dual-track settle/observe/grant orchestration. The order kind
	// alone cannot tell them apart (every wallet top-up is also a purchase
	// order — domain: "a purchase order settles as credit top-up"); the
	// discriminator is the #81 frozen quote face: a subscription purchase
	// carries exactly one subscription_fee line item. A nil purchaser
	// (blocked-env wiring) never blocks the shared drain: the event stays
	// pending for a later pass with a configured fulfiller.
	if row.Kind == domain.OrderKindPurchase {
		subscription, invalid, qerr := s.subscriptionPurchase(ctx, row)
		if qerr != nil {
			return s.completeEvent(ctx, ev, repocommercial.OutboxStatePending)
		}
		if invalid {
			return s.quarantineFulfillEvent(ctx, ev, "invalid_purchase_quote")
		}
		if subscription {
			if s.purchaser == nil {
				return s.completeEvent(ctx, ev, repocommercial.OutboxStatePending)
			}
			if err := s.purchaser.Fulfill(ctx, ev); err != nil {
				return err
			}
			// Completion is observed on the order row (Fulfill is idempotent and
			// returns nil both for "still activating" and for completion).
			if row2, rerr := s.orders.GetOrder(ctx, order.ID); rerr == nil &&
				row2.Domain().State == domain.OrderStateFulfilled {
				return s.completeEvent(ctx, ev, repocommercial.OutboxStateSent)
			}
			return s.completeEvent(ctx, ev, repocommercial.OutboxStatePending)
		}
	}
	var lines []FulfillmentLine
	if row.Kind == domain.OrderKindUpgrade {
		lines, err = s.prepareUpgrade(ctx, row, now)
	} else {
		lines = s.lines(order)
	}
	if err != nil {
		return err
	}
	for _, line := range lines {
		rec, err := s.ensureRecord(ctx, order, line, now)
		if err != nil {
			return err
		}
		if err := s.processRecord(ctx, rec, now); err != nil {
			return err
		}
	}
	complete, err := s.orderComplete(ctx, order.ID)
	if err != nil {
		return err
	}
	if !complete {
		// Some line is still attention/unknown: release the lease so the
		// next pass reconciles it via FindBenefit.
		return s.completeEvent(ctx, ev, repocommercial.OutboxStatePending)
	}
	if err := s.orders.MarkFulfilled(ctx, order.ID); err != nil {
		return err
	}
	return s.completeEvent(ctx, ev, repocommercial.OutboxStateSent)
}

// overPaymentPayload mirrors the over_payment outbox payload written by
// ConfirmPayment (#84): the multiple-success SECOND payment's fund fact.
type overPaymentPayload struct {
	OrderID     string `json:"order_id"`
	TenantID    uint64 `json:"tenant_id"`
	AttemptID   string `json:"attempt_id"`
	Provider    string `json:"provider"`
	Merchant    string `json:"merchant"`
	Transaction string `json:"transaction"`
	AmountFen   int64  `json:"amount_fen"`
	Currency    string `json:"currency"`
}

// disposeOverPayment consumes one over_payment event into the operator
// disposition surface (#84 / G3 / AC2): the multiple-success second payment
// is retained as an awaiting-disposition over_payment anomaly — Expected
// AmountFen deliberately 0 (this is not a single-payment wrong amount; the
// expected face lives on the order row) — and the event completes sent.
// RecordPaymentAnomaly is idempotent on (provider, merchant, transaction),
// so a redelivered/replayed event never mints a second row. A malformed
// payload is a deterministic dead end: the event is completed sent and the
// malformation logged — no retry can fix unparseable bytes. A retention
// error keeps the event pending for the next pass (the drain loop isolates
// it from the fulfill events of the same batch).
func (s *FulfillmentService) disposeOverPayment(ctx context.Context, ev repocommercial.OutboxEvent, now time.Time) error {
	var payload overPaymentPayload
	if err := json.Unmarshal([]byte(ev.PayloadJSON), &payload); err != nil {
		logger.Warnf(ctx, "[CommercialFulfillment] dropping malformed over_payment event %s: %v", ev.EventKey, err)
		return s.completeEvent(ctx, ev, repocommercial.OutboxStateSent)
	}
	if err := s.orders.RecordPaymentAnomaly(ctx, repocommercial.PaymentAnomalyRow{
		TenantID: payload.TenantID, OrderID: payload.OrderID, AttemptID: payload.AttemptID,
		Provider: payload.Provider, Merchant: payload.Merchant, Transaction: payload.Transaction,
		Kind:              repocommercial.PaymentAnomalyKindOverPaid,
		ExpectedAmountFen: 0, ActualAmountFen: payload.AmountFen,
		ExpectedCurrency: payload.Currency, ActualCurrency: payload.Currency,
	}); err != nil {
		return err
	}
	return s.completeEvent(ctx, ev, repocommercial.OutboxStateSent)
}

// isSubscriptionPurchase reports whether a purchase order is a SUBSCRIPTION
// purchase (#81/#82): its frozen quote carries the subscription_fee line
// item. Orders without a readable quote (the legacy top-up seeds and the
// pre-#81 pipeline) are not subscription purchases.
func (s *FulfillmentService) isSubscriptionPurchase(ctx context.Context, row repocommercial.OrderRow) bool {
	b, _, _ := s.subscriptionPurchase(ctx, row)
	return b
}

func (s *FulfillmentService) subscriptionPurchase(ctx context.Context, row repocommercial.OrderRow) (bool, bool, error) {
	if row.QuoteID == "" {
		return false, false, nil
	}
	var q repocommercial.QuoteRow
	if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", row.QuoteID, row.TenantID).First(&q).Error; err != nil {
		// (r2:341) A paid order misrouting into the top-up settlement (book
		// rate credits instead of the settle/activation chain) is the most
		// expensive silent failure this dispatcher has: a NotFound is the
		// frozen legacy-seed semantics (silently false), but any OTHER read
		// failure leaves one Warn so the misroute stays diagnosable.
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warnf(ctx, "[CommercialFulfillment] subscription-purchase quote read failed for order %s: %v", row.ID, err)
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// A non-empty QuoteID is a durable purchase reference. Missing or
			// wrong-tenant rows cannot authorize a top-up fallback.
			return false, true, nil
		}
		return false, false, err
	}
	var snap struct {
		LineItems []struct {
			Kind string `json:"kind"`
		} `json:"line_items"`
	}
	if err := json.Unmarshal([]byte(q.SnapshotJSON), &snap); err != nil {
		logger.Warnf(ctx, "[CommercialFulfillment] subscription-purchase quote snapshot unparsable for order %s: %v", row.ID, err)
		return false, true, nil
	}
	for _, line := range snap.LineItems {
		if line.Kind == "subscription_fee" {
			return true, false, nil
		}
	}
	if len(snap.LineItems) == 0 {
		return false, true, nil
	}
	return false, false, nil
}

func (s *FulfillmentService) quarantineFulfillEvent(ctx context.Context, ev repocommercial.OutboxEvent, reason string) error {
	logger.Warnf(ctx, "[CommercialFulfillment] quarantining event %s: %s", ev.EventKey, reason)
	return s.completeEvent(ctx, ev, repocommercial.OutboxStateDead)
}

// ensureRecord claims the unique FulfillmentRecord for one order line,
// creating it with the pinned effective time on first sight. Concurrent or
// restarted workers converge on the same stored row.
// benefitKindPlanSwitch marks the LOCAL plan-switch marker record of an
// upgrade order. It is settled inside the database transaction, never sent
// to the external gateway, and its applied state plus local receipt are what
// make the switch exactly-once across passes and crashes.
const benefitKindPlanSwitch = "plan_switch"

// prepareUpgrade drives the UPGRADE settlement of a paid upgrade order
// (design 6.2): in ONE transaction it claims the unique plan-switch marker,
// pins the prorated monthly-credit delta line and switches the subscription
// to the target plan — keeping anchor and paid_until, the paid interval is
// never reset and no already-used monthly grant is re-issued. A crash at
// any point either lands the whole switch or leaves it for the next pass to
// discover; the pinned delta line is then granted through the SAME gateway
// path as every other benefit line. The returned lines mirror the order's
// still-pending benefit records, so replays re-drive exactly what storage
// says is outstanding — never a recomputation from the already-switched
// subscription.
func (s *FulfillmentService) prepareUpgrade(ctx context.Context, row repocommercial.OrderRow, now time.Time) ([]FulfillmentLine, error) {
	var q repocommercial.QuoteRow
	if err := s.db.WithContext(ctx).Where("id = ?", row.QuoteID).First(&q).Error; err != nil {
		return nil, fmt.Errorf("upgrade quote %s: %w", row.QuoteID, err)
	}
	var snap struct {
		PlanKey      string `json:"plan_key"`
		PlanVersion  int64  `json:"plan_version"`
		PriceFen     int64  `json:"price_fen"`
		CreditsMicro int64  `json:"credits_micro"`
	}
	if err := json.Unmarshal([]byte(q.SnapshotJSON), &snap); err != nil {
		return nil, fmt.Errorf("upgrade quote snapshot %s: %w", row.QuoteID, err)
	}
	var plan repocommercial.PlanRow
	if err := s.db.WithContext(ctx).Where("plan_key = ? AND version = ?", snap.PlanKey, snap.PlanVersion).First(&plan).Error; err != nil {
		return nil, fmt.Errorf("upgrade plan %s v%d: %w", snap.PlanKey, snap.PlanVersion, err)
	}
	var sub repocommercial.Subscription
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", row.TenantID).First(&sub).Error; err != nil {
		return nil, fmt.Errorf("upgrade subscription tenant=%d: %w", row.TenantID, err)
	}
	var current domain.PlanVersion
	if err := json.Unmarshal([]byte(sub.PlanSnapshotJSON), &current); err != nil {
		return nil, fmt.Errorf("subscription snapshot tenant=%d: %w", row.TenantID, err)
	}

	// Prorated monthly-credit delta for the REMAINING part of the current
	// benefit month (design 6.2: 只补额度差额, floor to credit precision; the
	// delta batch expires with the current month).
	monthStart, monthEnd := domain.MonthWindowAt(sub.Anchor, now)
	monthSpan := monthEnd.Sub(monthStart)
	remaining := monthEnd.Sub(now)
	if remaining < 0 {
		remaining = 0
	}
	if monthSpan > 0 && remaining > monthSpan {
		remaining = monthSpan
	}
	var delta int64
	if snap.CreditsMicro > int64(current.Monthly) && monthSpan > 0 && remaining > 0 {
		// big.Rat proration (floor to credit precision, design 6.2): a
		// naive credits×nanoseconds product overflows int64.
		prorated, err := domain.Prorate(snap.CreditsMicro-int64(current.Monthly),
			int64(remaining), int64(monthSpan), false)
		if err != nil {
			return nil, err
		}
		delta = prorated
	}

	switchKey := domain.FulfillmentKey(row.ID, "plan_switch")
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		marker := FulfillmentRecord{
			Key: switchKey, TenantID: row.TenantID, OrderID: row.ID, LineID: "plan_switch",
			Kind: benefitKindPlanSwitch, PlanRef: snap.PlanKey,
			CustomerID: OrderCustomerID(row.TenantID), Credits: 0,
			EffectiveAt: domain.FirstEffectiveAt(time.Time{}, now), ExpiresAt: monthEnd,
			ExternalID: "local:" + row.ID, State: domain.FulfillmentStateApplied,
			LeaseUntil: now,
		}
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&marker)
		if res.Error != nil {
			return res.Error
		}
		if delta > 0 {
			deltaRec := FulfillmentRecord{
				Key: domain.FulfillmentKey(row.ID, "upgrade_delta"), TenantID: row.TenantID,
				OrderID: row.ID, LineID: "upgrade_delta",
				Kind: domain.BenefitKindSubscription, PlanRef: snap.PlanKey,
				CustomerID: OrderCustomerID(row.TenantID), Credits: delta,
				EffectiveAt: domain.FirstEffectiveAt(time.Time{}, now), ExpiresAt: monthEnd,
				State: fulfillmentRecordPending, LeaseUntil: now,
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&deltaRec).Error; err != nil {
				return err
			}
		}
		if res.RowsAffected == 1 {
			// First pass owns the switch: apply it under the version guard so a
			// concurrent writer loses cleanly instead of half-applying.
			upd := tx.Model(&repocommercial.Subscription{}).
				Where("id = ? AND version = ?", sub.ID, sub.Version).
				Updates(map[string]interface{}{
					"plan_key":           snap.PlanKey,
					"plan_version":       snap.PlanVersion,
					"plan_snapshot_json": plan.DefinitionJSON,
					"version":            sub.Version + 1,
				})
			if upd.Error != nil {
				return upd.Error
			}
			if upd.RowsAffected == 0 {
				return fmt.Errorf("upgrade switch lost the subscription version race for %s", sub.ID)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Replay-safe line set: exactly the order's still-pending benefit
	// records (a pinned delta survives crashes; a completed one is never
	// re-driven).
	var pending []FulfillmentRecord
	if err := s.db.WithContext(ctx).
		Where("order_id = ? AND state = ?", row.ID, fulfillmentRecordPending).
		Find(&pending).Error; err != nil {
		return nil, err
	}
	lines := make([]FulfillmentLine, 0, len(pending))
	for _, rec := range pending {
		lines = append(lines, FulfillmentLine{
			ID: rec.LineID, Kind: rec.Kind, PlanRef: rec.PlanRef,
			Credits: domain.Credits(rec.Credits), ExpiresAt: rec.ExpiresAt,
		})
	}
	return lines, nil
}
func (s *FulfillmentService) ensureRecord(ctx context.Context, order domain.Order, line FulfillmentLine, now time.Time) (FulfillmentRecord, error) {
	candidate := FulfillmentRecord{
		Key:         domain.FulfillmentKey(order.ID, line.ID),
		TenantID:    order.TenantID,
		OrderID:     order.ID,
		LineID:      line.ID,
		Kind:        line.Kind,
		PlanRef:     line.PlanRef,
		CustomerID:  OrderCustomerID(order.TenantID),
		Credits:     int64(line.Credits),
		ExpiresAt:   line.ExpiresAt,
		EffectiveAt: domain.FirstEffectiveAt(time.Time{}, now),
		State:       fulfillmentRecordPending,
		LeaseUntil:  now,
	}
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate).Error; err != nil {
		return FulfillmentRecord{}, err
	}
	var stored FulfillmentRecord
	if err := s.db.WithContext(ctx).Where("key = ?", candidate.Key).First(&stored).Error; err != nil {
		return FulfillmentRecord{}, err
	}
	return stored, nil
}

// processRecord drives one line to a stable state for this pass.
// FindBenefit runs FIRST on every attempt, so a benefit the remote already
// persisted is discovered rather than re-granted; ApplyBenefit runs only
// on a provable miss, keyed by the unique FulfillmentKey.
func (s *FulfillmentService) processRecord(ctx context.Context, rec FulfillmentRecord, now time.Time) error {
	if rec.State == domain.FulfillmentStateApplied && rec.ExternalID != "" {
		return nil // already fulfilled with an explicit external receipt
	}
	if !s.leaseRecord(ctx, &rec, now) {
		return nil // another worker owns this record right now
	}
	gwCtx := domain.WithBenefitCustomer(ctx, rec.CustomerID)
	receipt, err := s.gateway.FindBenefit(gwCtx, rec.Key)
	switch domain.ClassifyFulfillment(err) {
	case domain.FulfillmentApplied:
		if receipt.ExternalID == "" {
			// An empty receipt proves nothing; it is never a success.
			return s.recordAttention(ctx, rec)
		}
		return s.recordApplied(ctx, rec, receipt)
	case domain.FulfillmentMissing:
		req := domain.BenefitRequest{
			Key:         rec.Key,
			TenantID:    rec.TenantID,
			CustomerID:  rec.CustomerID,
			Kind:        rec.Kind,
			PlanRef:     rec.PlanRef,
			Credits:     domain.Credits(rec.Credits),
			ExpiresAt:   rec.ExpiresAt,
			EffectiveAt: domain.FirstEffectiveAt(rec.EffectiveAt, now),
		}
		applied, applyErr := s.gateway.ApplyBenefit(gwCtx, req)
		switch domain.ClassifyFulfillment(applyErr) {
		case domain.FulfillmentApplied:
			if applied.ExternalID == "" {
				return s.recordAttention(ctx, rec)
			}
			return s.recordApplied(ctx, rec, applied)
		case domain.FulfillmentRefused:
			// A definitive business rejection is a failure — never a success.
			return s.recordState(ctx, rec, domain.FulfillmentStateRefused)
		default:
			// Timeout / dropped response: the remote may have persisted.
			return s.recordAttention(ctx, rec)
		}
	case domain.FulfillmentRefused:
		return s.recordState(ctx, rec, domain.FulfillmentStateRefused)
	default:
		// The lookup itself was indeterminate: no license to re-apply.
		return s.recordAttention(ctx, rec)
	}
}

func (s *FulfillmentService) leaseRecord(ctx context.Context, rec *FulfillmentRecord, now time.Time) bool {
	token := newLeaseToken()
	res := s.db.WithContext(ctx).
		Model(&FulfillmentRecord{}).
		Where("key = ? AND state <> ? AND lease_until <= ?",
			rec.Key, domain.FulfillmentStateApplied, now).
		Updates(map[string]interface{}{
			"lease_token": token,
			"lease_until": now.Add(s.leaseTTL),
		})
	if res.Error != nil {
		return false
	}
	if res.RowsAffected != 1 {
		return false
	}
	rec.LeaseToken = token
	return true
}

// recordApplied stores the explicit external receipt. EffectiveAt stays
// pinned to the first-attempt value even when the receipt was discovered
// later, so a recovered top-up never shifts the interval it covers.
func (s *FulfillmentService) recordApplied(ctx context.Context, rec FulfillmentRecord, receipt domain.BenefitReceipt) error {
	return s.db.WithContext(ctx).
		Model(&FulfillmentRecord{}).
		Where("key = ? AND lease_token = ?", rec.Key, rec.LeaseToken).
		Updates(map[string]interface{}{
			"state":       domain.FulfillmentStateApplied,
			"external_id": receipt.ExternalID,
			"lease_until": s.now(),
		}).Error
}

func (s *FulfillmentService) recordAttention(ctx context.Context, rec FulfillmentRecord) error {
	return s.recordState(ctx, rec, domain.FulfillmentStateAttention)
}

func (s *FulfillmentService) recordState(ctx context.Context, rec FulfillmentRecord, state string) error {
	return s.db.WithContext(ctx).
		Model(&FulfillmentRecord{}).
		Where("key = ? AND lease_token = ?", rec.Key, rec.LeaseToken).
		Updates(map[string]interface{}{
			"state":       state,
			"lease_until": s.now(), // release: the next pass may reconcile
		}).Error
}

// orderComplete reports whether every line of the order carries an
// explicit external_id; only then does paid become fulfilled.
func (s *FulfillmentService) orderComplete(ctx context.Context, orderID string) (bool, error) {
	var recs []FulfillmentRecord
	if err := s.db.WithContext(ctx).Where("order_id = ?", orderID).Find(&recs).Error; err != nil {
		return false, err
	}
	if len(recs) == 0 {
		return false, nil
	}
	for _, r := range recs {
		if r.State != domain.FulfillmentStateApplied || r.ExternalID == "" {
			return false, nil
		}
	}
	return true, nil
}

// completeEvent finishes the event for this pass. Only the lease owner
// completes; "pending" releases the lease so the next pass retries
// without waiting out the full TTL.
func (s *FulfillmentService) completeEvent(ctx context.Context, ev repocommercial.OutboxEvent, state string) error {
	return s.db.WithContext(ctx).
		Model(&repocommercial.OutboxEvent{}).
		Where("event_key = ? AND lease_token = ?", ev.EventKey, ev.LeaseToken).
		Updates(map[string]interface{}{
			"state":       state,
			"lease_until": s.now(),
		}).Error
}

func newLeaseToken() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(b[:])
}
