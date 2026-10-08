package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
)

var (
	ErrInvalidOrderRow        = errors.New("invalid_order_row")
	ErrOrderNotFound          = errors.New("order_not_found")
	ErrInvalidOrderState      = errors.New("invalid_order_state")
	ErrInvalidPaymentAttempt  = errors.New("invalid_payment_attempt")
	ErrPaymentAttemptNotFound = errors.New("payment_attempt_not_found")
	// ErrPurchasePendingExists (R2-26): the partial unique index rejected a
	// second concurrent payable purchase order for the same tenant — the
	// caller replays the existing pending order (the ErrQuoteAlreadyUsed
	// shape, one purchase identity ahead).
	ErrPurchasePendingExists = errors.New("purchase_pending_exists")
)

// Payment attempt lifecycle states: an attempt is registered pending and
// becomes succeeded once its provider transaction is confirmed. A pending
// attempt whose channel entry was deliberately retired (CloseChannelOrder,
// #83) lands closed — the row keeps the channel identity for a late
// confirmation (ConfirmPayment still resolves it) but the attempt is no
// longer the recovery path's payable entry.
const (
	PaymentAttemptStatePending   = "pending"
	PaymentAttemptStateSucceeded = "succeeded"
	PaymentAttemptStateClosed    = "closed"
)

// Outbox event kinds emitted by payment confirmation.
const (
	OutboxKindFulfill   = "fulfill"
	OutboxKindOverPaid  = "over_payment"
	OutboxOverPaidState = "over_paid"
)

// OrderRow stores a commercial order. quote_id UNIQUE makes a quote
// consumable by exactly one order; version guards the pending→paid
// transition so exactly one confirmation wins.
type OrderRow struct {
	ID        string `gorm:"primaryKey;column:id"`
	TenantID  uint64 `gorm:"column:tenant_id;not null"`
	QuoteID   string `gorm:"column:quote_id;uniqueIndex;not null"`
	Kind      string `gorm:"column:kind;not null;default:'purchase'"`
	AmountFen int64  `gorm:"column:amount_fen;not null"`
	Currency  string `gorm:"column:currency;not null"`
	State     string `gorm:"column:state;not null"`
	Version   int64  `gorm:"column:version;not null;default:1"`
	// CreatedAt (R1-20): order ids are "ord_"+random hex, so id order is NOT
	// a recency order — the timestamp is the only deterministic "newest"
	// anchor for the current-purchase resolution (same-price historical
	// purchases, repurchases after cancellation).
	CreatedAt time.Time `gorm:"column:created_at"`
	// CheckoutURL persists the channel's customer-facing payment link
	// (R1-35): it arrives from the provider AFTER the order unit commits, so
	// it is written in its own step and the replay/recovery projections
	// re-serve it verbatim — a client that lost the first answer (timeout,
	// refresh) must not lose the only payment entry to an already-consumed
	// quote. Empty when the channel call failed (the CheckoutError posture)
	// or never made; never fabricated.
	CheckoutURL string `gorm:"column:checkout_url"`
	// ChannelFailed (R2-28): the channel Create call FAILED for this order
	// (gateway 5xx / transport timeout) — the order stays pending for the
	// recovery paths but is NOT a payable entry (no checkout link exists,
	// the channel most likely never saw the order), so it must neither be
	// replayed as a payment entry nor block a fresh quote's checkout (the
	// partial pending-uniqueness index excludes it). A late channel
	// confirmation still pays it through the standard ConfirmPayment path.
	ChannelFailed bool `gorm:"column:channel_failed;not null;default:false"`
}

func (OrderRow) TableName() string { return "commercial_orders" }

// Domain projects the persisted row onto the commercial domain type.
func (r OrderRow) Domain() domain.Order {
	return domain.Order{
		ID:       r.ID,
		TenantID: r.TenantID,
		Amount:   domain.CNYFen(r.AmountFen),
		Currency: r.Currency,
		State:    r.State,
		Version:  r.Version,
	}
}

// PaymentAttemptRow is the registered merchant-side attempt for one order.
// (provider, merchant, merchant_order_id) is unique, and once a provider
// transaction lands, (provider, merchant, provider_transaction_id) is unique
// too, so the same channel transaction can never confirm twice as different
// attempts. provider_transaction_id stays NULL until confirmation, letting
// several pending attempts coexist.
type PaymentAttemptRow struct {
	ID                    string  `gorm:"primaryKey;column:id"`
	TenantID              uint64  `gorm:"column:tenant_id;not null"`
	OrderID               string  `gorm:"column:order_id;not null"`
	Provider              string  `gorm:"column:provider;not null"`
	Merchant              string  `gorm:"column:merchant;not null"`
	MerchantOrderID       string  `gorm:"column:merchant_order_id;not null"`
	ProviderTransactionID *string `gorm:"column:provider_transaction_id"`
	AmountFen             int64   `gorm:"column:amount_fen;not null"`
	Currency              string  `gorm:"column:currency;not null"`
	State                 string  `gorm:"column:state;not null"`
}

func (PaymentAttemptRow) TableName() string { return "commercial_payment_attempts" }

type OrderStore struct{ db *gorm.DB }

func NewOrderStore(db *gorm.DB) *OrderStore { return &OrderStore{db: db} }

// CreateOrder registers a pending order. The quote_id unique index guarantees
// a quote is consumable by exactly one order, concurrent creators included.
func (s *OrderStore) CreateOrder(ctx context.Context, row OrderRow) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return createOrderTx(tx, row)
	})
}

func createOrderTx(tx *gorm.DB, row OrderRow) error {
	if row.ID == "" || row.TenantID == 0 || row.QuoteID == "" || row.AmountFen <= 0 || row.Currency == "" {
		return ErrInvalidOrderRow
	}
	if row.State != "" && row.State != domain.OrderStatePending {
		return ErrInvalidOrderRow
	}
	row.State = domain.OrderStatePending
	if row.Version == 0 {
		row.Version = 1
	}
	// (R1-20) creation timestamp: the deterministic recency anchor. Tests
	// may pre-set it; the store never overwrites a caller-provided value.
	// (A-30 / F110) Whatever the source, the instant is NORMALIZED to UTC
	// before it lands: the SQLite driver stores time.Time as text and the
	// recency ORDER BY / sweep age gate compare that text LEXICALLY, so a
	// local-offset rendering (+08:00) would sort behind UTC rows of the
	// same instant and push the sweep gate across a day boundary (or, on a
	// negative offset, sweep a just-created order immediately). Normalizing
	// changes the RENDERING only — never the instant itself. The zero value
	// is filled explicitly (a zero CreatedAt would otherwise be auto-filled
	// by gorm in the SERVER's local zone, re-introducing the skew).
	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now()
	}
	row.CreatedAt = row.CreatedAt.UTC()
	var existing OrderRow
	err := tx.Where("quote_id = ?", row.QuoteID).First(&existing).Error
	if err == nil {
		return ErrQuoteAlreadyUsed
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err := tx.Create(&row).Error; err != nil {
		// (R2-26) The partial unique index uq_purchase_pending_per_tenant
		// makes "one payable pending purchase order per tenant" a DATABASE
		// invariant: a concurrent checkout's insert loses here (both
		// pre-checks passed before the winner committed). SQLite reports
		// "UNIQUE constraint failed: commercial_orders.tenant_id" (column
		// face — the quote_id index reports quote_id instead), PostgreSQL
		// names the index itself.
		if row.Kind == domain.OrderKindPurchase && isPendingPurchaseConflict(err) {
			return ErrPurchasePendingExists
		}
		// (A-20 / F89) The quote face of the same race: two concurrent
		// checkouts on the SAME quote both pass the First pre-check above
		// and the loser's insert hits the quote_id unique INDEX here — the
		// sentinel translation the service's replay branch
		// (errors.Is(err, ErrQuoteAlreadyUsed)) exists for. Without it the
		// loser surfaces a raw driver error (an unmapped 500) instead of
		// the idempotent replay of the winner's order.
		if isQuoteUniqueConflict(err) {
			return ErrQuoteAlreadyUsed
		}
		return err
	}
	return nil
}

// isPendingPurchaseConflict matches the partial pending-uniqueness index
// violation (R2-26) across the two supported drivers. The quote_id unique
// index is the OTHER conflict shape and deliberately does not match.
func isPendingPurchaseConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "uq_purchase_pending_per_tenant") {
		return true
	}
	return strings.Contains(msg, "UNIQUE constraint failed") && strings.Contains(msg, "tenant_id")
}

// isQuoteUniqueConflict matches the quote_id unique index violation (A-20)
// across the two supported drivers: SQLite reports the column face
// ("UNIQUE constraint failed: commercial_orders.quote_id"), PostgreSQL
// names the index — in the UNDERSCORE form gorm's default NamingStrategy
// actually generates (idx_commercial_orders_quote_id, r2:212 — the dotted
// literal the previous matcher relied on never appears on a live PG; it
// stays matched as a defensive shape). The pending-uniqueness shape
// deliberately does not match (it reports tenant_id).
func isQuoteUniqueConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "UNIQUE constraint failed") && strings.Contains(msg, "commercial_orders.quote_id") {
		return true
	}
	return strings.Contains(msg, "idx_commercial_orders_quote_id") ||
		strings.Contains(msg, "idx_commercial_orders.quote_id")
}

// OpenOrderCommand is the SINGLE input from which the order row and its
// payment-attempt row are derived: identity, tenant, amount and currency
// exist exactly once, so the two persisted rows cannot drift apart and no
// caller can atomically commit inconsistent data (the repository validates
// the command once, then derives both rows).
type OpenOrderCommand struct {
	OrderID   string
	AttemptID string
	TenantID  uint64
	QuoteID   string
	// Kind selects the fulfillment policy (domain.OrderKindPurchase /
	// OrderKindUpgrade); empty defaults to purchase.
	Kind            string
	AmountFen       int64
	Currency        string
	Provider        string
	Merchant        string
	MerchantOrderID string
	// SubscriptionVersion guards quote consumption (the quote must have
	// been cut against this version of the subscription).
	SubscriptionVersion int64
	// ClaimSubscriptionID / ClaimVersion: when set (upgrade orders), the
	// SAME transaction atomically advances the subscription version — a
	// concurrent upgrade based on a stale version loses the WHOLE unit
	// (order + quote + attempt), never just an after-the-fact check.
	ClaimSubscriptionID string
	ClaimVersion        int64
	Now                 time.Time
}

func (c *OpenOrderCommand) validate() error {
	if c.OrderID == "" || c.AttemptID == "" || c.TenantID == 0 || c.QuoteID == "" ||
		c.AmountFen <= 0 || c.Currency == "" || c.Provider == "" || c.Merchant == "" ||
		c.MerchantOrderID == "" || c.Now.IsZero() {
		return ErrInvalidOrderRow
	}
	if c.Kind == "" {
		c.Kind = domain.OrderKindPurchase
	}
	if c.Kind != domain.OrderKindPurchase && c.Kind != domain.OrderKindUpgrade {
		return ErrInvalidOrderRow
	}
	if c.ClaimSubscriptionID != "" && c.ClaimVersion <= 0 {
		return ErrInvalidOrderRow
	}
	return nil
}

// OpenOrder is the atomic unit of work behind a checkout (and an upgrade):
// optionally claiming the subscription version, creating the pending order,
// consuming the quote and registering the pending attempt in ONE
// transaction. Any failure — stale claim, stale quote, expiry, version
// conflict, unique violation — rolls the whole unit back: no orphan order
// row survives a refused checkout and no compensation delete is needed.
func (s *OrderStore) OpenOrder(ctx context.Context, cmd OpenOrderCommand) error {
	if err := cmd.validate(); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if cmd.ClaimSubscriptionID != "" {
			res := tx.Model(&Subscription{}).
				Where("id = ? AND version = ?", cmd.ClaimSubscriptionID, cmd.ClaimVersion).
				Update("version", gorm.Expr("version + 1"))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return ErrSubscriptionVersionConflict
			}
		}
		if err := createOrderTx(tx, OrderRow{
			ID: cmd.OrderID, TenantID: cmd.TenantID, QuoteID: cmd.QuoteID,
			Kind: cmd.Kind, AmountFen: cmd.AmountFen, Currency: cmd.Currency,
			// (R1-20) the command's clock: the order's creation timestamp
			// shares the unit's time base (quote consumption included).
			CreatedAt: cmd.Now,
		}); err != nil {
			return err
		}
		if _, err := consumeQuoteTx(tx, cmd.QuoteID, cmd.SubscriptionVersion, cmd.OrderID, cmd.Now); err != nil {
			return err
		}
		return registerAttemptTx(tx, PaymentAttemptRow{
			ID: cmd.AttemptID, TenantID: cmd.TenantID, OrderID: cmd.OrderID,
			Provider: cmd.Provider, Merchant: cmd.Merchant, MerchantOrderID: cmd.MerchantOrderID,
			AmountFen: cmd.AmountFen, Currency: cmd.Currency,
		})
	})
}

// ListOrdersByTenant returns a space's orders, newest first. The tenant
// comes exclusively from the authenticated context; the query never
// accepts a tenant from the caller's payload.
func (s *OrderStore) ListOrdersByTenant(ctx context.Context, tenantID uint64) ([]OrderRow, error) {
	if tenantID == 0 {
		return nil, ErrOrderNotFound
	}
	var rows []OrderRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// CurrentPurchaseOrder returns the order row that belongs to the CURRENT
// purchase (#81): purchase-kind orders at the held purchase's frozen price
// face (amount+currency), the NEWEST unfinished (pending) checkout
// preferred (R1-20: order ids are "ord_"+random hex, so id order is not a
// recency order — created_at DESC is the recency anchor, id the deterministic
// tie-break; same-price historical purchases and repurchases after
// cancellation must not shadow the current order, and a stale pending
// leftover must not outrank the current one). All bounds are
// parameter-bound; no matching order reports ErrOrderNotFound.
func (s *OrderStore) CurrentPurchaseOrder(ctx context.Context, tenantID uint64, amountFen int64, currency string) (OrderRow, error) {
	if tenantID == 0 {
		return OrderRow{}, ErrOrderNotFound
	}
	var rows []OrderRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND kind = ? AND amount_fen = ? AND currency = ?",
			tenantID, domain.OrderKindPurchase, amountFen, currency).
		Order("created_at DESC, id ASC").Find(&rows).Error; err != nil {
		return OrderRow{}, err
	}
	for _, row := range rows {
		// (A-29 / F109) A channel-failed pending row is a DEAD payment
		// entry (R2-28 — no checkout link exists, it is neither payable nor
		// replayable): it must never win the pending preference and shadow a
		// real paid order of the same price face (the caller would project a
		// permanently-pending dead order and MISS the paid_awaiting_activation
		// synthesis window). The pending preference carries the SAME payable
		// predicate as CurrentPendingPurchaseOrder — including the persisted
		// checkout_url (OCR84-R1-03: a persistence-degraded row whose channel
		// Create succeeded but whose SetCheckoutURL failed carries
		// channel_failed=false with an EMPTY link and is just as unpayable;
		// letting it win projected a link-less permanent pending order that
		// shadowed a same-price paid order until the stale-linkless sweep).
		if row.State == domain.OrderStatePending && !row.ChannelFailed && row.CheckoutURL != "" {
			return row, nil
		}
	}
	// (r2:348) The fallback carries the SAME dead-row discipline: any
	// pending row still reaching the fallback is a DEAD payment entry
	// (channel-failed, or link-less — a payable one would have won the
	// preference loop above), and the newest row being one (a paid order
	// followed by a failed channel Create within the awaiting window) must
	// not shadow the newest LIVE row. The first non-pending row (the newest
	// paid/fulfilled one) answers; an all-pending price face reports
	// not-found rather than projecting a permanently-dead entry.
	for _, row := range rows {
		if row.State == domain.OrderStatePending {
			continue // dead payment entry — never the current purchase face
		}
		return row, nil
	}
	return OrderRow{}, ErrOrderNotFound
}

// CurrentPaidAwaitingActivationPurchaseOrder answers the NEWEST purchase
// order sitting in the paid-awaiting-activation window (D11 / r2:253):
// purchase-kind at the held purchase's frozen price face, state=paid —
// "paid but not yet fulfilled" is exactly the OrderRow.State=paid shape
// (MarkFulfilled moves it to fulfilled, which leaves the window and this
// read). The pre-create probe uses it to REPLAY the paid order instead of
// opening a second channel order for the same gating subscription (the
// pending uniqueness index only guards state='pending' — a paid order has
// left its range, so the database no longer rejects the double open).
// No matching order reports ErrOrderNotFound; all bounds are
// parameter-bound.
func (s *OrderStore) CurrentPaidAwaitingActivationPurchaseOrder(ctx context.Context, tenantID uint64, amountFen int64, currency string) (OrderRow, error) {
	if tenantID == 0 {
		return OrderRow{}, ErrOrderNotFound
	}
	var row OrderRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND kind = ? AND amount_fen = ? AND currency = ? AND state = ?",
			tenantID, domain.OrderKindPurchase, amountFen, currency, domain.OrderStatePaid).
		Order("created_at DESC, id ASC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OrderRow{}, ErrOrderNotFound
	}
	if err != nil {
		return OrderRow{}, err
	}
	return row, nil
}

// CurrentPendingPurchaseOrder returns the NEWEST still-PENDING, PAYABLE
// purchase order at the held purchase's frozen price face (R1-22, R2-28):
// at most ONE payable channel order may exist per purchase, so a checkout
// under a fresh quote must replay the existing pending order instead of
// opening a second concurrent channel order for the same gating invoice
// (two payable orders would double-charge the same subscription — each
// callback confirms independently). Payable (R2-28) means the channel
// Create SUCCEEDED: channel-failed orders carry no checkout link and are
// not replayed as payment entries, and an empty checkout_url (persistence
// degraded, R2-27) is not a payment entry either. No payable pending order
// reports ErrOrderNotFound.
func (s *OrderStore) CurrentPendingPurchaseOrder(ctx context.Context, tenantID uint64, amountFen int64, currency string) (OrderRow, error) {
	if tenantID == 0 {
		return OrderRow{}, ErrOrderNotFound
	}
	var row OrderRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND kind = ? AND amount_fen = ? AND currency = ? AND state = ? AND channel_failed = ? AND checkout_url <> ''",
			tenantID, domain.OrderKindPurchase, amountFen, currency, domain.OrderStatePending, false).
		Order("created_at DESC, id ASC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OrderRow{}, ErrOrderNotFound
	}
	if err != nil {
		return OrderRow{}, err
	}
	return row, nil
}

// CurrentPayablePendingOrder returns the tenant's NEWEST payable pending
// purchase order WITHOUT the price-face filter (R2-26): it is the conflict
// replay read — the partial unique index just proved such an order exists,
// so the price face would only risk a read mismatch (the concurrent winner
// consumed a quote of the SAME purchase, hence the same frozen face, but
// the looser read keeps the replay unconditional on matching keys).
// (R3-26) PAYABLE is the SAME predicate as the index: channel_failed =
// false AND a persisted checkout_url — a link-less row (persist-degraded,
// ctx-cancelled channel failure, pre-column legacy) is never replayed as a
// clean winner; the caller that cannot find a replayable row surfaces the
// conflict error instead of answering an unpayable 201.
func (s *OrderStore) CurrentPayablePendingOrder(ctx context.Context, tenantID uint64) (OrderRow, error) {
	if tenantID == 0 {
		return OrderRow{}, ErrOrderNotFound
	}
	var row OrderRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND kind = ? AND state = ? AND channel_failed = ? AND checkout_url <> ''",
			tenantID, domain.OrderKindPurchase, domain.OrderStatePending, false).
		Order("created_at DESC, id ASC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OrderRow{}, ErrOrderNotFound
	}
	if err != nil {
		return OrderRow{}, err
	}
	return row, nil
}

// MarkChannelFailed persists the channel entry's retirement on the order row
// (R2-28, #83): the order stays pending (the recovery paths and a late
// channel confirmation still work) but is marked NOT payable — the channel
// entry is no longer open (a failed channel Create left no checkout link, or
// CloseChannelOrder deliberately closed a pending order). The partial
// pending-uniqueness index excludes channel-failed rows, so a fresh quote's
// checkout is never blocked by a dead order.
func (s *OrderStore) MarkChannelFailed(ctx context.Context, orderID string) error {
	if orderID == "" {
		return ErrInvalidOrderRow
	}
	res := s.db.WithContext(ctx).Model(&OrderRow{}).
		Where("id = ? AND state = ?", orderID, domain.OrderStatePending).
		Update("channel_failed", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrOrderNotFound
	}
	return nil
}

// SweepStaleLinklessPending marks the tenant's pending purchase rows that
// carry channel_failed=false but NO persisted checkout link (R3-26): the
// persist-degraded residue (the channel call succeeded but the link never
// landed — including pre-column legacy rows before the migration backfill).
// Such a row is not a replayable payment entry, yet it occupies the
// pending-uniqueness slot; sweeping it to channel-failed releases the slot
// so a fresh quote's checkout can proceed. Parameter-bound; returns the
// number of rows swept.
func (s *OrderStore) SweepStaleLinklessPending(ctx context.Context, tenantID uint64) (int64, error) {
	// (OCR r4) Time-scoped: only rows OLDER than the sweep age are stale —
	// a just-created order whose checkout link is still mid-landing must
	// never be swept out from under its own checkout.
	staleBefore := time.Now().UTC().Add(-SweepStaleAge)
	res := s.db.WithContext(ctx).Model(&OrderRow{}).
		Where("tenant_id = ? AND kind = ? AND state = ? AND channel_failed = ? AND (checkout_url IS NULL OR checkout_url = '') AND created_at < ?",
			tenantID, domain.OrderKindPurchase, domain.OrderStatePending, false, staleBefore).
		Update("channel_failed", true)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// SweepStaleAge is the age at which a link-less pending purchase row counts
// as sweepable residue (OCR r4: a fresh checkout's persist window is
// seconds; anything still link-less after this is degraded residue).
const SweepStaleAge = 15 * time.Minute

// FirstPendingAttempt returns the oldest still-pending attempt of an order —
// the identifier payment recovery re-queries the channel with. An order with
// no pending attempt reports ErrPaymentAttemptNotFound.
func (s *OrderStore) FirstPendingAttempt(ctx context.Context, orderID string) (PaymentAttemptRow, error) {
	var att PaymentAttemptRow
	err := s.db.WithContext(ctx).
		Where("order_id = ? AND state = ?", orderID, PaymentAttemptStatePending).
		Order("id").First(&att).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PaymentAttemptRow{}, ErrPaymentAttemptNotFound
	}
	return att, err
}

// MarkAttemptClosed retires every still-pending attempt of an order to the
// closed state (#83 CloseChannelOrder landing): the channel entry is gone,
// but the row — and its (provider, merchant, merchant_order_id) identity —
// survives so a LATE channel success still confirms through ConfirmPayment
// (the fund fact is never destroyed by a close). Idempotent by construction:
// only pending rows move, so a replay (or a race with a confirmation that
// already succeeded the attempt) is a no-op. Parameter-bound.
func (s *OrderStore) MarkAttemptClosed(ctx context.Context, orderID string) error {
	if orderID == "" {
		return ErrInvalidOrderRow
	}
	res := s.db.WithContext(ctx).Model(&PaymentAttemptRow{}).
		Where("order_id = ? AND state = ?", orderID, PaymentAttemptStatePending).
		Update("state", PaymentAttemptStateClosed)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

// CloseAttemptAndRetireChannel lands the close-decision PAIR — every
// still-pending attempt to closed AND the order row to channel_failed — in
// ONE transaction (OCR C-07): the pair is the single close landing, and
// landing it as two independent UPDATEs left a recoverable-only-by-hand
// half state on a crash between them (attempt closed but the order still
// payable: channel_failed=false with a persisted checkout_url has NO API
// recovery path — the switch branch needs a pending attempt, the sweep only
// touches link-less rows). Both writes carry the same predicates as the
// standalone methods (MarkAttemptClosed / MarkChannelFailed); a concurrent
// payment that already advanced the order past pending refuses the half
// landing (ErrOrderNotFound — the whole transaction rolls back).
// Parameter-bound throughout.
func (s *OrderStore) CloseAttemptAndRetireChannel(ctx context.Context, orderID string) error {
	if orderID == "" {
		return ErrInvalidOrderRow
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&PaymentAttemptRow{}).
			Where("order_id = ? AND state = ?", orderID, PaymentAttemptStatePending).
			Update("state", PaymentAttemptStateClosed).Error; err != nil {
			return err
		}
		res := tx.Model(&OrderRow{}).
			Where("id = ? AND state = ?", orderID, domain.OrderStatePending).
			Update("channel_failed", true)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrOrderNotFound
		}
		return nil
	})
}

// GetOrder returns an order by ID; readers distinguish paid from fulfilled.
func (s *OrderStore) GetOrder(ctx context.Context, id string) (OrderRow, error) {
	var row OrderRow
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OrderRow{}, ErrOrderNotFound
	}
	return row, err
}

// SetCheckoutURL persists the channel checkout link after the order unit
// committed (R1-35): the link is produced by the provider call that follows
// OpenOrder, so it lands in its own bounded update. An empty url is refused
// (nothing to persist — the CheckoutError posture stays the single marker of
// a failed channel call); the update is parameter-bound and reports
// ErrOrderNotFound when the order row is gone.
func (s *OrderStore) SetCheckoutURL(ctx context.Context, orderID, checkoutURL string) error {
	if orderID == "" || checkoutURL == "" {
		return ErrInvalidOrderRow
	}
	res := s.db.WithContext(ctx).Model(&OrderRow{}).
		Where("id = ?", orderID).
		Update("checkout_url", checkoutURL)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrOrderNotFound
	}
	return nil
}

// GetOrderByQuote returns the order opened against one quote (#81): the
// retry idempotency surface — a replayed purchase resolves the EXISTING
// order instead of opening a second channel request. Both bounds are
// parameter-bound; an order of another tenant is simply not found.
func (s *OrderStore) GetOrderByQuote(ctx context.Context, tenantID uint64, quoteID string) (OrderRow, error) {
	var row OrderRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND quote_id = ?", tenantID, quoteID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OrderRow{}, ErrOrderNotFound
	}
	if err != nil {
		return OrderRow{}, err
	}
	return row, nil
}

// RegisterAttempt records the merchant-side attempt before any payment
// result arrives; ConfirmPayment only accepts facts whose merchant and
// attempt were registered this way.
func (s *OrderStore) RegisterAttempt(ctx context.Context, row PaymentAttemptRow) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return registerAttemptTx(tx, row)
	})
}

func registerAttemptTx(tx *gorm.DB, row PaymentAttemptRow) error {
	if row.ID == "" || row.TenantID == 0 || row.OrderID == "" || row.Provider == "" ||
		row.Merchant == "" || row.MerchantOrderID == "" || row.AmountFen <= 0 || row.Currency == "" {
		return ErrInvalidPaymentAttempt
	}
	if row.State != "" && row.State != PaymentAttemptStatePending {
		return ErrInvalidPaymentAttempt
	}
	if row.ProviderTransactionID != nil {
		return ErrInvalidPaymentAttempt
	}
	row.State = PaymentAttemptStatePending
	return tx.Create(&row).Error
}

type paymentEventPayload struct {
	OrderID   string `json:"order_id"`
	QuoteID   string `json:"quote_id"`
	TenantID  uint64 `json:"tenant_id"`
	AttemptID string `json:"attempt_id"`
	// MerchantOrderID (OCR84-R1-13) is the attempt's CHANNEL order identity
	// (mo_…). ConfirmPayment stores it in PaymentAnomalyRow.AttemptID and
	// the consumer validates it against the registered attempt. Legacy
	// payloads without the field recover it from that validated attempt.
	MerchantOrderID string `json:"merchant_order_id,omitempty"`
	Provider        string `json:"provider"`
	Merchant        string `json:"merchant"`
	Transaction     string `json:"transaction"`
	AmountFen       int64  `json:"amount_fen"`
	Currency        string `json:"currency"`
	Reason          string `json:"reason,omitempty"`
}

func (p paymentEventPayload) toJSON() (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidOutboxEvent, err)
	}
	return string(b), nil
}

// ConfirmPayment records a provider payment result in ONE transaction: it
// validates the registered merchant and attempt, checks the order version,
// saves the payment fact, marks the order paid, and inserts the unique
// fulfillment outbox event. A replayed same-channel transaction returns the
// original result with no second event. A later distinct successful
// collection atomically records an over-payment anomaly and its audit event;
// the fulfillment event is never duplicated. A failure at any point rolls
// back the whole transaction, including both audit records.
//
// (#84, spec L127) Two refused classes split: a NON-SUCCEEDED verified fact
// answers ErrPaymentNotSucceeded with zero persistence (the channel's retry
// is the correct posture for a not-yet/never collection), while an
// AMOUNT/CURRENCY mismatch still rolls the confirmation back but its
// external fund fact is retained — AFTER the transaction, in its own
// RecordPaymentAnomaly transaction — as an awaiting-disposition anomaly.
// A retention failure propagates over ErrPaymentMismatch so the callback
// face answers non-2xx (never "acknowledged" without the fact).
func (s *OrderStore) ConfirmPayment(ctx context.Context, fact domain.PaymentFact) error {
	var anomaly *PaymentAnomalyRow
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var attempt PaymentAttemptRow
		err := tx.Where("provider = ? AND merchant = ? AND merchant_order_id = ?",
			fact.Provider, fact.Merchant, fact.AttemptID).First(&attempt).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPaymentAttemptNotFound
		}
		if err != nil {
			return err
		}
		// (#84 / Review Focus 1) A verified fact whose channel state is not
		// succeeded is its own class: no anomaly (it is not a collection),
		// no fulfillment, whole rollback. This fires BEFORE the identity /
		// amount comparison so a "right amount, closed state" fact is never
		// misfiled as an amount mismatch.
		if fact.State != "succeeded" {
			return domain.ErrPaymentNotSucceeded
		}
		if attempt.OrderID != fact.OrderID || attempt.TenantID != fact.TenantID ||
			attempt.AmountFen != int64(fact.Amount) || attempt.Currency != fact.Currency {
			snap := buildMismatchAnomaly(attempt, fact)
			anomaly = &snap
			return domain.ErrPaymentMismatch
		}

		var row OrderRow
		if err := tx.Where("id = ?", fact.OrderID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrOrderNotFound
			}
			return err
		}
		if err := domain.ValidatePayment(row.Domain(), fact); err != nil {
			snap := buildMismatchAnomaly(attempt, fact)
			anomaly = &snap
			return err
		}

		// The first successful collection claims the attempt's transaction
		// identity. Later distinct collections are retained by the over-payment
		// path below without replacing that identity.
		sameTxn := attempt.State == PaymentAttemptStateSucceeded &&
			attempt.ProviderTransactionID != nil && *attempt.ProviderTransactionID == fact.Transaction
		if !sameTxn {
			txn := fact.Transaction
			claim := tx.Model(&PaymentAttemptRow{}).
				Where("id = ? AND state IN ? AND provider_transaction_id IS NULL", attempt.ID,
					[]string{PaymentAttemptStatePending, PaymentAttemptStateClosed}).
				Updates(map[string]interface{}{
					"state":                   PaymentAttemptStateSucceeded,
					"provider_transaction_id": txn,
				})
			if claim.Error != nil {
				return claim.Error
			}
			if claim.RowsAffected == 0 {
				// A concurrent confirmation may have won since the initial read.
				// Classify against the durable winner, never the stale snapshot.
				if err := tx.Where("id = ?", attempt.ID).First(&attempt).Error; err != nil {
					return err
				}
				sameTxn = attempt.State == PaymentAttemptStateSucceeded &&
					attempt.ProviderTransactionID != nil && *attempt.ProviderTransactionID == fact.Transaction
			}
		}

		// Guarded pending→paid transition: exactly one confirmation wins the
		// order version and therefore the single fulfillment right.
		res := tx.Model(&OrderRow{}).
			Where("id = ? AND state = ? AND version = ?", row.ID, domain.OrderStatePending, row.Version).
			Updates(map[string]interface{}{
				"state":   domain.OrderStatePaid,
				"version": row.Version + 1,
			})
		if res.Error != nil {
			return res.Error
		}
		payload := paymentEventPayload{
			OrderID:     row.ID,
			QuoteID:     row.QuoteID,
			TenantID:    row.TenantID,
			AttemptID:   attempt.ID,
			Provider:    fact.Provider,
			Merchant:    fact.Merchant,
			Transaction: fact.Transaction,
			AmountFen:   int64(fact.Amount),
			Currency:    fact.Currency,
			// MerchantOrderID carries the registered channel identity. On a
			// later successful collection, ConfirmPayment stores it in the
			// anomaly fact in this transaction; the consumer validates it.
			MerchantOrderID: attempt.MerchantOrderID,
		}
		if res.RowsAffected == 1 {
			payloadJSON, err := payload.toJSON()
			if err != nil {
				return err
			}
			return insertOutboxEvent(tx, OutboxEvent{
				EventKey:    OutboxKindFulfill + ":" + row.ID,
				TenantID:    row.TenantID,
				Kind:        OutboxKindFulfill,
				PayloadJSON: payloadJSON,
			})
		}

		// The order is already paid or fulfilled. A replay of the winning
		// transaction returns the original success silently. A later distinct
		// successful collection gets an immutable anomaly fact plus its outbox
		// event in this same transaction.
		if sameTxn {
			return nil
		}
		if err := recordPaymentAnomalyTx(tx, PaymentAnomalyRow{
			TenantID:          attempt.TenantID,
			OrderID:           row.ID,
			AttemptID:         attempt.MerchantOrderID,
			Provider:          fact.Provider,
			Merchant:          fact.Merchant,
			Transaction:       fact.Transaction,
			Kind:              PaymentAnomalyKindOverPaid,
			ExpectedAmountFen: 0,
			ActualAmountFen:   int64(fact.Amount),
			ExpectedCurrency:  fact.Currency,
			ActualCurrency:    fact.Currency,
		}); err != nil {
			return err
		}
		payload.Reason = OutboxOverPaidState
		payloadJSON, err := payload.toJSON()
		if err != nil {
			return err
		}
		err = insertOutboxEvent(tx, OutboxEvent{
			EventKey:    fmt.Sprintf("%s:%s:%s:%s:%s", OutboxKindOverPaid, row.ID, fact.Provider, fact.Merchant, fact.Transaction),
			TenantID:    row.TenantID,
			Kind:        OutboxKindOverPaid,
			PayloadJSON: payloadJSON,
		})
		if errors.Is(err, ErrOutboxEventDuplicate) {
			return nil
		}
		return err
	})
	// (#84) The mismatch fund fact is retained OUTSIDE the rolled-back
	// confirmation transaction, in its own transaction: the anomaly survives
	// the rollback, and a retention failure overrides the mismatch sentinel
	// so the callback face answers non-2xx (the channel retries — Review
	// Focus 2).
	if err != nil && anomaly != nil && errors.Is(err, domain.ErrPaymentMismatch) {
		if rerr := s.RecordPaymentAnomaly(ctx, *anomaly); rerr != nil {
			return rerr
		}
	}
	return err
}

// buildMismatchAnomaly snapshots a refused amount/currency/identity mismatch
// as an awaiting-disposition anomaly row (#84, spec L127): expected values
// come from the REGISTERED attempt (the order's frozen face), actual values
// from the verified fact. The row's ID is minted at insert time inside
// RecordPaymentAnomaly — never here — so a rollback-and-retry keeps one
// identity per (provider, merchant, transaction).
func buildMismatchAnomaly(attempt PaymentAttemptRow, fact domain.PaymentFact) PaymentAnomalyRow {
	return PaymentAnomalyRow{
		TenantID:          attempt.TenantID,
		OrderID:           attempt.OrderID,
		AttemptID:         attempt.MerchantOrderID,
		Provider:          fact.Provider,
		Merchant:          fact.Merchant,
		Transaction:       fact.Transaction,
		Kind:              ClassifyPaymentAnomaly(attempt.AmountFen, int64(fact.Amount), attempt.Currency, fact.Currency),
		ExpectedAmountFen: attempt.AmountFen,
		ActualAmountFen:   int64(fact.Amount),
		ExpectedCurrency:  attempt.Currency,
		ActualCurrency:    fact.Currency,
	}
}

// MarkFulfilled transitions a paid order to fulfilled once its benefit event
// has been delivered; it is idempotent for an already-fulfilled order.
func (s *OrderStore) MarkFulfilled(ctx context.Context, id string) error {
	res := s.db.WithContext(ctx).Model(&OrderRow{}).
		Where("id = ? AND state = ?", id, domain.OrderStatePaid).
		Updates(map[string]interface{}{
			"state":   domain.OrderStateFulfilled,
			"version": gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 1 {
		return nil
	}
	var row OrderRow
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrOrderNotFound
	}
	if err != nil {
		return err
	}
	if row.State == domain.OrderStateFulfilled {
		return nil
	}
	return ErrInvalidOrderState
}
