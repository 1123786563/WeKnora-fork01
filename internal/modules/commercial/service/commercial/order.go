package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/modules/commercial/payment"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"

	"gorm.io/gorm"
)

var (
	// ErrPaymentProviderUnconfigured marks order checkout blocked-env: no
	// payment channel adapter is wired in this process. The quote and order
	// pipeline still works; only the channel call is refused, and it is
	// refused EXPLICITLY instead of fabricating a checkout URL.
	ErrPaymentProviderUnconfigured   = errors.New("payment_provider_unconfigured")
	ErrPaymentObservationUnavailable = errors.New("payment_status_unavailable")
	// ErrQuoteTenantMismatch rejects use of a quote that was cut for a
	// different space than the authenticated caller.
	ErrQuoteTenantMismatch = errors.New("quote_tenant_mismatch")
	// ErrOrderTenantMismatch rejects access to an order of another space.
	ErrOrderTenantMismatch = errors.New("order_tenant_mismatch")
	// ErrUnknownPaymentProvider rejects a checkout request naming a provider
	// this process does not have configured.
	ErrUnknownPaymentProvider = errors.New("unknown_payment_provider")
	// ErrNoSubscriptionToChange rejects a plan change for a space on the
	// base tier: the first purchase is a plain order, not a change.
	ErrNoSubscriptionToChange = errors.New("no_subscription_to_change")
)

// quoteValidity bounds how long a cut quote may settle an order. Long
// enough for a human to pay; short enough that a price change lands soon.
const quoteValidity = 30 * time.Minute

// checkoutPersistTimeout (R3-28) bounds the order-row persisting writes
// (channel-failed mark, checkout_url) that ride a context detached from the
// caller's cancellation: long enough for a single bounded UPDATE, short
// enough to never leak a request's cleanup.
const checkoutPersistTimeout = 5 * time.Second

// QuoteLineItem is one frozen invoice line of the offer (#81 AC1): the
// first slice prices exactly one subscription fee; integer fen only.
type QuoteLineItem struct {
	Kind      string `json:"kind"`       // 闭合 "subscription_fee"
	Name      string `json:"name"`       // plan 显示名
	AmountFen int64  `json:"amount_fen"` // 整数分
}

// quoteSnapshot is the exact offer frozen at quote time: the published plan
// version it buys, its exact price in fen, the monthly credits it grants
// and — additive #81 fields — the closed currency, the frozen entitlement
// features and the line items. The order is priced from THIS snapshot,
// never re-read from the (possibly republished) catalog at order time.
// New fields are JSON-backward-compatible: a legacy snapshot unmarshals
// them as zero values, and the purchase path refuses legacy snapshots with
// ErrQuoteLegacySnapshot (re-quote) instead of guessing.
type quoteSnapshot struct {
	PlanKey      string          `json:"plan_key"`
	PlanVersion  int64           `json:"plan_version"`
	PriceFen     int64           `json:"price_fen"`
	CreditsMicro int64           `json:"credits_micro"`
	Currency     string          `json:"currency"` // "CNY"
	Features     map[string]bool `json:"features,omitempty"`
	LineItems    []QuoteLineItem `json:"line_items,omitempty"` // 首期恰一行 subscription_fee
}

// OrderService implements the order pipeline over the repository stores:
// quote (price an exact published plan for one space), order (consume the
// quote, register the attempt, open the channel checkout), payment status
// recovery (re-query the channel by the ORIGINAL merchant order id and
// confirm through the same ConfirmPayment transaction the callback path
// uses) and Commerce.ChangePlan (prorated upgrade order or a scheduled
// future downgrade). The service holds NO direct database handle: every
// persistence goes through the repository's transactional units of work,
// so quote consumption, order creation and attempt registration commit or
// roll back as ONE atomic operation.
type OrderService struct {
	quotes    *repocommercial.CatalogStore
	orders    *repocommercial.OrderStore
	subs      *repocommercial.SubscriptionStore
	providers map[string]payment.Provider
}

// NewOrderService wires the pipeline. providers may be empty (blocked-env):
// quoting and ordering still work against the database, and channel calls
// surface ErrPaymentProviderUnconfigured.
func NewOrderService(db *gorm.DB, providers map[string]payment.Provider) (*OrderService, error) {
	if db == nil {
		return nil, errors.New("order_database_missing")
	}
	if providers == nil {
		providers = map[string]payment.Provider{}
	}
	// Additive column upgrades (order kind, scheduled plan change) so a
	// deployment created before them migrates in place.
	if err := normalizeMigratedCommercialUniques(db); err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.Subscription{}, &repocommercial.PaymentAnomalyRow{}); err != nil {
		return nil, err
	}
	// (R3-26) Migration of PRE-INVARIANT rows, BEFORE the index is created:
	// a pending purchase order whose checkout link never landed (the old
	// POST /orders pipeline predates the checkout_url column — every legacy
	// pending row is link-less; SetCheckoutURL degradations and
	// ctx-cancelled channel failures leave the same shape) is NOT a payable
	// entry and must not enter the invariant's scope on upgrade day. Mark
	// those rows channel-failed (the recovery paths and a late channel
	// confirmation still work; the row simply stops blocking fresh
	// checkouts). created_at is backfilled for rows created before the
	// column existed (NULL/zero on PostgreSQL sorts NULLS FIRST and would
	// shadow every newer row). Parameter-bound; no external input.
	// (OCR r4) The backfill is scoped to PRE-INVARIANT rows ONLY: a
	// created_at IS NULL/zero row predates the column (the legacy
	// pipeline); every row the new pipeline writes carries created_at, so
	// a restart while a fresh checkout is mid-landing (order created,
	// checkout_url not yet written) never gets swept here.
	if err := db.Exec(`UPDATE commercial_orders SET channel_failed = true
		WHERE kind = 'purchase' AND state = 'pending' AND channel_failed = false
		AND (checkout_url IS NULL OR checkout_url = '')
		AND (created_at IS NULL OR created_at < '1970-01-02 00:00:00')`).Error; err != nil {
		return nil, fmt.Errorf("commercial pending-purchase backfill: %w", err)
	}
	if err := db.Exec(`UPDATE commercial_orders SET created_at = ?
		WHERE created_at IS NULL OR created_at < '1970-01-02 00:00:00'`,
		time.Now().UTC()).Error; err != nil {
		return nil, fmt.Errorf("commercial created_at backfill: %w", err)
	}
	// (R2-26/R3-27) Database-level invariant: at most ONE payable pending
	// purchase order per tenant. The purchase path's read-decide-write only
	// narrowed the race window — two concurrent POSTs with two fresh quotes
	// could both pass the pre-checks and commit; the partial unique index
	// closes the gap at INSERT time (loser answers ErrPurchasePendingExists
	// and replays the winner, the ErrQuoteAlreadyUsed shape). The predicate
	// is deliberately channel_failed = false ONLY (NOT "and a checkout_url"):
	// including the link would move the conflict from the atomic INSERT to
	// the later SetCheckoutURL UPDATE (two concurrent checkouts both insert
	// link-less, the second link persistence then hits the index) — the
	// link-less rows that DO occupy the slot are only the persist-degraded
	// residue, and the service's conflict path unlocks those by sweeping
	// them to channel-failed before retrying once. A deployment holding
	// pre-invariant duplicates fails HERE loudly (the index cannot be
	// created) instead of silently continuing without the invariant. The
	// boolean is spelled `false` / `true` (NOT 0/1): GORM migrates the Go
	// bool to a PostgreSQL boolean column, and `boolean = integer` has no
	// implicit cast there — the 0/1 spelling failed at PARSE time on every
	// PostgreSQL deployment (R3-27); SQLite 3.23+ and PostgreSQL both
	// accept the boolean literals.
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_pending_per_tenant
		ON commercial_orders (tenant_id)
		WHERE kind = 'purchase' AND state = 'pending' AND channel_failed = false`).Error; err != nil {
		return nil, fmt.Errorf("commercial pending-purchase invariant: %w", err)
	}
	return &OrderService{
		quotes:    repocommercial.NewCatalogStore(db),
		orders:    repocommercial.NewOrderStore(db),
		subs:      repocommercial.NewSubscriptionStore(db),
		providers: providers,
	}, nil
}

// normalizeMigratedCommercialUniques reconciles the two commercial tables
// that exist in both the versioned migrations and the AutoMigrate models.
// The migrations declared quote_id / tenant_id as inline column UNIQUE
// constraints (PostgreSQL *_key default names); the models declare
// uniqueIndex (gorm uni_* index names). AutoMigrate resolves that
// difference by dropping a constraint under its own name, which fails on
// migration-provisioned databases where only the *_key constraint exists.
// Create the model-expected unique index first, then drop the legacy
// constraint by its real name, so quote/tenant uniqueness never has a gap.
func normalizeMigratedCommercialUniques(db *gorm.DB) error {
	type legacyUnique struct {
		table      interface{}
		field      string
		tableName  string
		constraint string
	}
	for _, legacy := range []legacyUnique{
		{table: &repocommercial.OrderRow{}, field: "QuoteID", tableName: "commercial_orders", constraint: "commercial_orders_quote_id_key"},
		{table: &repocommercial.Subscription{}, field: "TenantID", tableName: "commercial_subscriptions", constraint: "commercial_subscriptions_tenant_id_key"},
	} {
		if !db.Migrator().HasTable(legacy.tableName) {
			continue
		}
		if db.Migrator().HasConstraint(legacy.tableName, legacy.constraint) {
			if err := db.Migrator().CreateIndex(legacy.table, legacy.field); err != nil {
				return err
			}
			if err := db.Migrator().DropConstraint(legacy.tableName, legacy.constraint); err != nil {
				return err
			}
		}
	}
	return nil
}

// QuoteView is the produced quote projection. Currency, Features and
// LineItems are the additive #81 AC1 freeze: the customer sees exactly the
// currency, entitlements and invoice lines the quote commits to.
type QuoteView struct {
	ID           string          `json:"id"`
	PlanKey      string          `json:"plan_key"`
	PlanVersion  int64           `json:"plan_version"`
	AmountFen    int64           `json:"amount_fen"`
	CreditsMicro int64           `json:"credits_micro"`
	ExpiresAt    string          `json:"expires_at"`
	Currency     string          `json:"currency,omitempty"`
	Features     map[string]bool `json:"features,omitempty"`
	LineItems    []QuoteLineItem `json:"line_items,omitempty"`
}

// CreateQuote cuts an offer for the LATEST PUBLISHED version of planKey in
// the caller's space: exact price from the immutable published definition,
// expiry bounded, and the subscription version recorded so a concurrent
// subscription change invalidates the quote at consumption time.
func (s *OrderService) CreateQuote(ctx context.Context, tenantID uint64, planKey string) (QuoteView, error) {
	if tenantID == 0 || planKey == "" {
		return QuoteView{}, repocommercial.ErrInvalidQuoteRow
	}
	row, err := s.quotes.LatestPublishedPlan(ctx, planKey)
	if err != nil {
		return QuoteView{}, err
	}
	var plan domain.PlanVersion
	if err := json.Unmarshal([]byte(row.DefinitionJSON), &plan); err != nil {
		return QuoteView{}, fmt.Errorf("%w: %v", repocommercial.ErrInvalidPlanRow, err)
	}
	if err := plan.ValidateForPublish(); err != nil {
		return QuoteView{}, err
	}
	subVersion, err := s.subs.LatestVersion(ctx, tenantID)
	if err != nil {
		return QuoteView{}, err
	}
	snap := quoteSnapshot{
		PlanKey:      plan.Key,
		PlanVersion:  plan.Version,
		PriceFen:     int64(plan.Price),
		CreditsMicro: int64(plan.Monthly),
		// #81 AC1 freeze: closed currency, the version's entitlements and the
		// single first-period subscription-fee line (no pay-in-advance
		// charges exist on publishable plans — usage charges are
		// pay-in-arrears and never enter the first invoice).
		Currency:  domain.CurrencyCNY,
		Features:  plan.Features,
		LineItems: []QuoteLineItem{{Kind: "subscription_fee", Name: plan.Name, AmountFen: int64(plan.Price)}},
	}
	snapJSON, err := json.Marshal(snap)
	if err != nil {
		return QuoteView{}, err
	}
	id := "qt_" + newLeaseToken()
	expires := time.Now().Add(quoteValidity)
	if err := s.quotes.CreateQuote(ctx, repocommercial.QuoteRow{
		ID:                  id,
		TenantID:            tenantID,
		SubscriptionVersion: subVersion,
		SnapshotJSON:        string(snapJSON),
		ExpiresAt:           expires,
	}); err != nil {
		return QuoteView{}, err
	}
	return QuoteView{
		ID: id, PlanKey: snap.PlanKey, PlanVersion: snap.PlanVersion,
		AmountFen: snap.PriceFen, CreditsMicro: snap.CreditsMicro,
		ExpiresAt: expires.UTC().Format(time.RFC3339),
		Currency:  snap.Currency, Features: snap.Features, LineItems: snap.LineItems,
	}, nil
}

// CreateTopUpQuote cuts a one-shot credit top-up offer (#85 G-A / R-GA):
// CreditsMicro = AmountFen × 100 — one CNY buys 10,000 micro-credits, the
// BasePlanSeedIncludedCreditsMicro convention. No plan version, no
// subscription, no recurrence: the frozen quote's single top_up line is
// the invoice face the payment channel collects, and the same
// Quote→Invoice→Payment chain as a subscription purchase settles it.
// Amounts are whole-CNY only — the product keeps the granted credits
// cent-aligned (the platform grant contract).
func (s *OrderService) CreateTopUpQuote(ctx context.Context, tenantID uint64, amountFen int64) (QuoteView, error) {
	if tenantID == 0 || amountFen <= 0 || amountFen%100 != 0 {
		return QuoteView{}, fmt.Errorf("%w: top-up amount must be a positive whole-CNY fen amount", repocommercial.ErrInvalidQuoteRow)
	}
	subVersion, err := s.subs.LatestVersion(ctx, tenantID)
	if err != nil {
		return QuoteView{}, err
	}
	snap := quoteSnapshot{
		PlanKey:      "credits",
		PriceFen:     amountFen,
		CreditsMicro: amountFen * 100,
		Currency:     domain.CurrencyCNY,
		LineItems:    []QuoteLineItem{{Kind: "top_up", Name: "充值 Credits", AmountFen: amountFen}},
	}
	snapJSON, err := json.Marshal(snap)
	if err != nil {
		return QuoteView{}, err
	}
	id := "qt_" + newLeaseToken()
	expires := time.Now().Add(quoteValidity)
	if err := s.quotes.CreateQuote(ctx, repocommercial.QuoteRow{
		ID:                  id,
		TenantID:            tenantID,
		SubscriptionVersion: subVersion,
		SnapshotJSON:        string(snapJSON),
		ExpiresAt:           expires,
	}); err != nil {
		return QuoteView{}, err
	}
	return QuoteView{
		ID: id, PlanKey: snap.PlanKey,
		AmountFen: snap.PriceFen, CreditsMicro: snap.CreditsMicro,
		ExpiresAt: expires.UTC().Format(time.RFC3339),
		Currency:  snap.Currency, LineItems: snap.LineItems,
	}, nil
}

// OrderView is the produced order projection. CheckoutURL carries the
// customer-facing payment link when a channel adapter produced one.
// CheckoutError is non-empty when the channel call failed AFTER the order
// was durably opened: the pending order stays recoverable through
// GetOrder/RecoverOrderStatus, and the write answer still carries the
// operation ID and state as the product contract requires.
// CheckoutLinkDegraded (R2-27) marks a DIFFERENT posture: the channel call
// succeeded and the answer carries a working CheckoutURL, but persisting
// the link for later replays failed (closed marker, raw error in the server
// log only) — the answer itself stays a clean success.
type OrderView struct {
	ID        string `json:"id"`
	QuoteID   string `json:"quote_id"`
	State     string `json:"state"`
	AmountFen int64  `json:"amount_fen"`
	Currency  string `json:"currency"`
	Provider  string `json:"provider,omitempty"`
	// CheckoutURL carries the customer-facing payment link when a channel
	// adapter produced one.
	CheckoutURL string `json:"checkout_url,omitempty"`
	// CheckoutError is non-empty when the channel call failed AFTER the order
	// was durably opened: the pending order stays recoverable through
	// GetOrder/RecoverOrderStatus, and the write answer still carries the
	// operation ID and state as the product contract requires.
	// CheckoutLinkDegraded (R2-27) marks a DIFFERENT posture: the channel call
	// succeeded and the answer carries a working CheckoutURL, but persisting
	// the link for later replays failed (closed marker, raw error in the
	// server log only) — the answer itself stays a clean success.
	CheckoutError        string `json:"checkout_error,omitempty"`
	CheckoutLinkDegraded bool   `json:"checkout_link_degraded,omitempty"`
	// PaymentAttention (#84, spec L169 operator attention) marks an
	// unresolved retained payment anomaly on this order: an abnormal
	// collection (mismatched / partial / wrong currency / multiple success)
	// whose fund fact is recorded but which must never expand benefits. It
	// rides along on every state (a fulfilled order with a pending
	// over-payment disposition still carries it); the wire projection maps
	// it onto fulfillment=attention ONLY for pending reads.
	PaymentAttention bool  `json:"payment_attention,omitempty"`
	Version          int64 `json:"version"`
}

// CreateOrder consumes the quote and opens one pending order with one
// registered pending attempt. Persistence is ONE atomic unit of work
// (OrderStore.OpenOrder): the order row, the guarded quote consumption and
// the attempt row commit together or not at all, so a refused consumption
// leaves no orphan order behind. Only AFTER that unit commits is the
// channel called; a channel failure or timeout KEEPS the pending order and
// is reported through OrderView.CheckoutError with a nil error — the
// client always receives the operation ID and current state and recovers
// through GetOrder instead of retrying a consumed quote into a conflict.
func (s *OrderService) CreateOrder(ctx context.Context, tenantID uint64, quoteID, providerName string) (OrderView, error) {
	q, snap, err := s.quoteForTenant(ctx, tenantID, quoteID)
	if err != nil {
		return OrderView{}, err
	}
	return s.openOrder(ctx, tenantID, q, providerName, snap.PriceFen, domain.OrderKindPurchase, nil)
}

// QuoteSnapshotForTenant is the public quote read (#81): the purchase path
// loads the same guarded snapshot the order path consumes. quoteForTenant
// keeps the historical private name so existing call sites stay untouched.
func (s *OrderService) QuoteSnapshotForTenant(ctx context.Context, tenantID uint64, quoteID string) (repocommercial.QuoteRow, quoteSnapshot, error) {
	return s.quoteForTenant(ctx, tenantID, quoteID)
}

// ProviderConfigured reports whether the named channel provider is wired
// (a pure in-memory map lookup — no I/O). The purchase path checks it
// BEFORE any seam-side effect so a blocked-env checkout can never leave a
// payment-gated subscription behind (review F1).
func (s *OrderService) ProviderConfigured(name string) bool {
	provider, ok := s.providers[name]
	return ok && provider != nil
}

// CurrentPayablePendingOrderView projects the tenant's newest payable
// pending purchase order (R3-25): the handler's conflict answer for a
// rejected duplicate checkout attaches it so the client keeps a payment
// entry instead of a bare error token.
func (s *OrderService) CurrentPayablePendingOrderView(ctx context.Context, tenantID uint64) (OrderView, error) {
	row, err := s.orders.CurrentPayablePendingOrder(ctx, tenantID)
	if err != nil {
		return OrderView{}, err
	}
	return s.withAttention(ctx, OrderView{ID: row.ID, QuoteID: row.QuoteID, State: row.State,
		AmountFen: row.AmountFen, Currency: row.Currency, CheckoutURL: row.CheckoutURL,
		Version: row.Version}), nil
}

// quoteForTenant loads and validates the quote snapshot for a tenant.
func (s *OrderService) quoteForTenant(ctx context.Context, tenantID uint64, quoteID string) (repocommercial.QuoteRow, quoteSnapshot, error) {
	if tenantID == 0 || quoteID == "" {
		return repocommercial.QuoteRow{}, quoteSnapshot{}, repocommercial.ErrInvalidOrderRow
	}
	q, err := s.quotes.GetQuote(ctx, quoteID)
	if err != nil {
		return repocommercial.QuoteRow{}, quoteSnapshot{}, err
	}
	if q.TenantID != tenantID {
		return repocommercial.QuoteRow{}, quoteSnapshot{}, ErrQuoteTenantMismatch
	}
	var snap quoteSnapshot
	if err := json.Unmarshal([]byte(q.SnapshotJSON), &snap); err != nil {
		return repocommercial.QuoteRow{}, quoteSnapshot{}, fmt.Errorf("%w: %v", repocommercial.ErrInvalidQuoteRow, err)
	}
	return q, snap, nil
}

// subscriptionClaim identifies the subscription version an upgrade
// atomically claims inside OpenOrder's transaction, so two upgrades cut
// against the same version can never both open (and later pay) — the
// second loses the whole unit and must re-quote.
type subscriptionClaim struct {
	id      string
	version int64
}

// openOrder runs the shared checkout path: atomic quote+order+attempt unit,
// then the channel call. A channel failure returns the PENDING order view
// with CheckoutError set and a nil error — the write already succeeded.
func (s *OrderService) openOrder(ctx context.Context, tenantID uint64, q repocommercial.QuoteRow, providerName string, amountFen int64, kind string, claim *subscriptionClaim) (OrderView, error) {
	provider, ok := s.providers[providerName]
	if !ok || provider == nil {
		return OrderView{}, fmt.Errorf("%w: %q", ErrPaymentProviderUnconfigured, providerName)
	}
	subVersion, err := s.subs.LatestVersion(ctx, tenantID)
	if err != nil {
		return OrderView{}, err
	}
	if claim != nil && claim.version != subVersion {
		// The subscription moved between the read and the open: refuse now
		// instead of letting the atomic claim inside the transaction be the
		// only loser (same error either way, cheaper to detect early).
		return OrderView{}, repocommercial.ErrSubscriptionVersionConflict
	}
	id := "ord_" + newLeaseToken()
	merchantOrderID := "mo_" + newLeaseToken()
	cmd := repocommercial.OpenOrderCommand{
		OrderID: id, AttemptID: "att_" + newLeaseToken(), TenantID: tenantID,
		QuoteID: q.ID, Kind: kind, AmountFen: amountFen, Currency: "CNY",
		// The attempt's merchant MUST be the channel merchant identity the
		// provider's verified callbacks carry (SellerID/MchID) — NOT the
		// provider name: resolveByMerchantOrderID and ConfirmPayment key on
		// (provider, merchant, merchant_order_id), so any other value makes
		// every genuine channel callback unresolvable (issue #82 flow
		// defect 2: registered 'alipay' vs notified SellerID → 404).
		Provider: providerName, Merchant: provider.MerchantID(), MerchantOrderID: merchantOrderID,
		SubscriptionVersion: subVersion, Now: time.Now(),
	}
	if claim != nil {
		cmd.ClaimSubscriptionID = claim.id
		cmd.ClaimVersion = claim.version
	}
	if err := s.orders.OpenOrder(ctx, cmd); err != nil {
		return OrderView{}, err
	}
	res, err := provider.Create(ctx, payment.OrderRequest{
		OrderID: id, MerchantOrderID: merchantOrderID,
		AmountFen: amountFen, Currency: "CNY",
	})
	// (R3-26 trigger b) A channel Create that "succeeded" without producing
	// a checkout link is NOT a payable outcome: SetCheckoutURL refuses empty
	// strings, so persisting the link is impossible and the row would sit
	// pending+link-less forever. Treat the empty link exactly like a channel
	// failure (same posture below) — the row is marked channel-failed, the
	// answer carries CheckoutError, and the tenant's next checkout is not
	// blocked.
	if err == nil && res.CheckoutURL == "" {
		err = errors.New("channel answered without a checkout link")
	}
	// (R3-28) The two row-persisting writes below ride a context DETACHED
	// from the caller's cancellation: a channel Create that failed BECAUSE
	// the client disconnected (ctx cancelled) used to take the
	// channel-failed mark down with it (same cancelled ctx) — leaving a
	// zombie pending row with channel_failed=false and no link that blocked
	// the tenant's every future checkout. The detached context keeps the
	// short UPDATE alive past the request's end; only its own timeout
	// bounds it.
	persistCtx, persistCancel := context.WithTimeout(context.WithoutCancel(ctx), checkoutPersistTimeout)
	defer persistCancel()
	if err != nil {
		// The channel call failed (or timed out into StateUnknown, or
		// answered without a link): the pending order and attempt REMAIN
		// (channel-failed — see MarkChannelFailed below), the quote is
		// consumed, and the response still carries the operation ID + state
		// so recovery goes through GetOrder — never through a second
		// checkout of the same quote.
		// (R2-28) The channel failure is ALSO persisted on the row: the
		// channel-failed pending order is not a payable entry and must not
		// block a fresh quote's checkout.
		if ferr := s.orders.MarkChannelFailed(persistCtx, id); ferr != nil {
			log.Printf("commercial: channel-failed mark lost for order %s: %v", id, ferr)
		}
		return OrderView{ID: id, QuoteID: q.ID, State: domain.OrderStatePending,
			AmountFen: amountFen, Currency: "CNY", Provider: providerName, Version: 1,
			CheckoutError: fmt.Sprintf("channel checkout failed for %s: %v", id, err)}, nil
	}
	view := OrderView{ID: id, QuoteID: q.ID, State: domain.OrderStatePending,
		AmountFen: amountFen, Currency: "CNY", Provider: providerName,
		CheckoutURL: res.CheckoutURL, Version: 1}
	// (R1-35) Persist the channel link so the POST replay (the existing-order
	// branch) and the GetOrder recovery path re-serve it verbatim — without
	// the persistence the first answer carried the ONLY copy of the payment
	// entry to an already-consumed quote, and a client that lost it (timeout,
	// refresh) had no way back to the checkout. A persistence failure does
	// NOT undo the answer: the write-answer contract still holds (the client
	// gets the link this once).
	// (R2-27) The degradation is its OWN closed marker — NOT the channel
	// failure's CheckoutError: the channel call SUCCEEDED here (the client
	// holds a working link), so the handler must answer a clean 201, never
	// the channel-failure 202; and the raw error (SQL/driver detail) stays
	// in the server log, never on the wire.
	if err := s.orders.SetCheckoutURL(persistCtx, id, res.CheckoutURL); err != nil {
		log.Printf("commercial: checkout_url persistence failed for order %s: %v", id, err)
		view.CheckoutLinkDegraded = true
	}
	return view, nil
}

// ListOrders returns the caller space's orders, newest first. Never crosses
// spaces: the tenant comes exclusively from the authenticated context.
func (s *OrderService) ListOrders(ctx context.Context, tenantID uint64) ([]OrderView, error) {
	rows, err := s.orders.ListOrdersByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	attention, attentionErr := s.orders.ListAwaitingPaymentAnomalyOrderIDs(ctx, tenantID)
	if attentionErr != nil {
		attention = nil
	} // fail open: preserve the primary list
	out := make([]OrderView, 0, len(rows))
	for _, r := range rows {
		out = append(out, OrderView{ID: r.ID, QuoteID: r.QuoteID, State: r.State,
			AmountFen: r.AmountFen, Currency: r.Currency, Version: r.Version,
			PaymentAttention: hasOrderAttention(attention, r.ID)})
	}
	return out, nil
}

// hasOrderAttention checks the batched tenant anomaly set.
func hasOrderAttention(ids map[string]struct{}, orderID string) bool {
	_, ok := ids[orderID]
	return ok
}

func (s *OrderService) withAttention(ctx context.Context, view OrderView) OrderView {
	if ok, err := s.orders.HasUnresolvedPaymentAnomaly(ctx, view.ID); err == nil && ok {
		view.PaymentAttention = true
	}
	return view
}

// collectedAmountMismatch（#84 / G2）reports whether the channel Query's
// COLLECTED face contradicts the registered attempt face — amount OR
// currency: the recovery paths refuse to confirm such a payment (confirming
// at the attempt's face would silently absorb a wrong amount; building the
// fact from the attempt's own currency would LAUNDER a wrong-currency
// collection — the real-stack defect the #84 leg-1 currency variant caught)
// and retain it as an anomaly instead. 0 / "" (= channel did not report)
// never mismatches.
func collectedAmountMismatch(reportedFen, attemptFen int64, reportedCurrency, attemptCurrency string) bool {
	if reportedFen > 0 && reportedFen != attemptFen {
		return true
	}
	return reportedCurrency != "" && reportedCurrency != attemptCurrency
}

func validateCollectionObservation(fen int64, currency string) error {
	if fen <= 0 || strings.TrimSpace(currency) == "" {
		return ErrPaymentObservationUnavailable
	}
	return nil
}

// recoverMismatchedCollection retains a query-path collected-face mismatch
// as an awaiting anomaly (#84, spec L127) and returns the pending order view
// flagged for operator attention. The confirmation is skipped entirely: the
// order keeps its payable entry (a correct later callback or retry still
// confirms through the normal leg).
func (s *OrderService) recoverMismatchedCollection(ctx context.Context, row repocommercial.OrderRow, att repocommercial.PaymentAttemptRow, txn string, collectedFen int64, collectedCurrency string) (OrderView, error) {
	actualFen := collectedFen
	actualCurrency := collectedCurrency
	if err := s.orders.RecordPaymentAnomaly(ctx, repocommercial.PaymentAnomalyRow{
		TenantID: row.TenantID, OrderID: row.ID, AttemptID: att.MerchantOrderID,
		Provider: att.Provider, Merchant: att.Merchant, Transaction: txn,
		Kind:              repocommercial.ClassifyPaymentAnomaly(att.AmountFen, actualFen, att.Currency, actualCurrency),
		ExpectedAmountFen: att.AmountFen, ActualAmountFen: actualFen,
		ExpectedCurrency: att.Currency, ActualCurrency: actualCurrency,
	}); err != nil {
		return OrderView{}, err
	}
	return OrderView{ID: row.ID, QuoteID: row.QuoteID, State: domain.OrderStatePending,
		AmountFen: row.AmountFen, Currency: row.Currency, Provider: att.Provider,
		CheckoutURL: row.CheckoutURL, PaymentAttention: true, Version: row.Version}, nil
}

// RecoverOrderStatus is the payment state-recovery path for a PENDING order
// whose channel result may have been missed (closed tab, lost callback): it
// re-queries the provider by the ORIGINAL merchant order id and, on a
// succeeded fact, confirms the payment through the SAME ConfirmPayment
// transaction the callback path uses. (#84/G2) A succeeded fact whose
// COLLECTED amount contradicts the attempt face is never confirmed — the
// mismatch is retained as an anomaly and the order surfaces operator
// attention. Paid/fulfilled orders are returned untouched; a pending order
// whose channel is still pending stays pending.
func (s *OrderService) RecoverOrderStatus(ctx context.Context, tenantID uint64, orderID string) (OrderView, error) {
	row, err := s.orders.GetOrder(ctx, orderID)
	if err != nil {
		return OrderView{}, err
	}
	if row.TenantID != tenantID {
		return OrderView{}, ErrOrderTenantMismatch
	}
	if row.State != domain.OrderStatePending {
		return s.withAttention(ctx, OrderView{ID: row.ID, QuoteID: row.QuoteID, State: row.State,
			AmountFen: row.AmountFen, Currency: row.Currency, Version: row.Version}), nil
	}
	att, err := s.orders.FirstPendingAttempt(ctx, orderID)
	if errors.Is(err, repocommercial.ErrPaymentAttemptNotFound) {
		// No attempt ever registered (e.g. channel unconfigured at creation):
		// honestly pending, nothing to recover. The persisted checkout link
		// (R1-35) still rides along so the customer keeps a payment entry.
		return s.withAttention(ctx, OrderView{ID: row.ID, QuoteID: row.QuoteID, State: row.State,
			AmountFen: row.AmountFen, Currency: row.Currency, CheckoutURL: row.CheckoutURL,
			Version: row.Version}), nil
	}
	if err != nil {
		return OrderView{}, err
	}
	provider, ok := s.providers[att.Provider]
	if !ok || provider == nil {
		return OrderView{}, fmt.Errorf("%w: %q", ErrPaymentProviderUnconfigured, att.Provider)
	}
	res, err := provider.Query(ctx, att.MerchantOrderID)
	if err != nil {
		return OrderView{}, fmt.Errorf("%w: channel query failed for %s: %v", ErrPaymentObservationUnavailable, orderID, err)
	}
	if res.State == payment.StateSucceeded {
		txn := res.ProviderID
		if txn == "" {
			txn = att.MerchantOrderID
		}
		// (#84/G2) Compare what the channel ACTUALLY collected against the
		// attempt face BEFORE confirming: a mismatched collection is an
		// abnormal fact, never a fulfillment.
		if strings.TrimSpace(res.AmountCurrency) != "" && res.AmountCurrency != att.Currency {
			return s.recoverMismatchedCollection(ctx, row, att, txn, res.AmountFen, res.AmountCurrency)
		}
		if err := validateCollectionObservation(res.AmountFen, res.AmountCurrency); err != nil {
			return OrderView{}, err
		}
		if collectedAmountMismatch(res.AmountFen, att.AmountFen, res.AmountCurrency, att.Currency) {
			return s.recoverMismatchedCollection(ctx, row, att, txn, res.AmountFen, res.AmountCurrency)
		}
		if err := s.orders.ConfirmPayment(ctx, domain.PaymentFact{
			Provider: att.Provider, Merchant: att.Merchant,
			AttemptID: att.MerchantOrderID, OrderID: orderID, TenantID: tenantID,
			Amount: domain.CNYFen(att.AmountFen), Currency: att.Currency,
			Transaction: txn, State: payment.StateSucceeded.String(),
		}); err != nil {
			return OrderView{}, err
		}
		row, err = s.orders.GetOrder(ctx, orderID)
		if err != nil {
			return OrderView{}, err
		}
	}
	// (R1-35) A still-pending order re-serves the persisted checkout link
	// verbatim: the channel said pending (or the confirm re-read the row),
	// and the customer must be able to reach the payment page again without
	// a second checkout of the same consumed quote.
	return s.withAttention(ctx, OrderView{ID: row.ID, QuoteID: row.QuoteID, State: row.State,
		AmountFen: row.AmountFen, Currency: row.Currency, Provider: att.Provider,
		CheckoutURL: row.CheckoutURL, Version: row.Version}), nil
}

// CloseChannelOrder retires one pending order's channel entry in a
// race-safe way (#83, spec L123 close + L165 indeterminate outcomes): the
// channel Close is driven against the ORIGINAL merchant order id; ANY close
// failure (ORDER_PAID race, transport timeout, already-closed) is decided by
// the channel Query — a succeeded query confirms the payment through the
// SAME ConfirmPayment transaction the callback path uses (the fund fact
// survives the close, the fulfillment right is minted exactly once), a
// closed query lands like a clean close, and anything else (NOTPAY /
// unknown / query failure) stays UNRESOLVED: the error surfaces, nothing is
// marked, and the order keeps its payable entry so the caller may retry.
func (s *OrderService) CloseChannelOrder(ctx context.Context, tenantID uint64, orderID string) (OrderView, error) {
	row, err := s.orders.GetOrder(ctx, orderID)
	if err != nil {
		return OrderView{}, err
	}
	if row.TenantID != tenantID {
		return OrderView{}, ErrOrderTenantMismatch
	}
	if row.State != domain.OrderStatePending {
		// No payable channel entry exists for a paid/fulfilled order; the
		// caller answers the current state (zero channel calls).
		return OrderView{ID: row.ID, QuoteID: row.QuoteID, State: row.State,
			AmountFen: row.AmountFen, Currency: row.Currency, Version: row.Version}, nil
	}
	att, err := s.orders.FirstPendingAttempt(ctx, orderID)
	if errors.Is(err, repocommercial.ErrPaymentAttemptNotFound) {
		// A pending order with no attempt is not payable at all (e.g. a
		// channel-unconfigured leftover): retire the slot and answer pending.
		if err := s.orders.MarkChannelFailed(ctx, orderID); err != nil {
			return OrderView{}, err
		}
		return OrderView{ID: row.ID, QuoteID: row.QuoteID, State: row.State,
			AmountFen: row.AmountFen, Currency: row.Currency, Version: row.Version}, nil
	}
	if err != nil {
		return OrderView{}, err
	}
	provider, ok := s.providers[att.Provider]
	if !ok || provider == nil {
		return OrderView{}, fmt.Errorf("%w: %q", ErrPaymentProviderUnconfigured, att.Provider)
	}
	landClosed := func() (OrderView, error) {
		// (OCR C-07) The attempt-close and the channel_failed marking land
		// as ONE transactional pair: two independent UPDATEs left a
		// hand-recovery-only half state (attempt closed, order still
		// payable) when the process died between them.
		if err := s.orders.CloseAttemptAndRetireChannel(ctx, orderID); err != nil {
			return OrderView{}, err
		}
		return OrderView{ID: row.ID, QuoteID: row.QuoteID, State: domain.OrderStatePending,
			AmountFen: row.AmountFen, Currency: row.Currency, Provider: att.Provider,
			Version: row.Version}, nil
	}
	cerr := provider.Close(ctx, att.MerchantOrderID)
	if cerr == nil {
		return landClosed()
	}
	// Indeterminate on the close side (ORDER_PAID race, timeout, already
	// closed, transport error): the channel alone knows the truth — decide
	// by querying the ORIGINAL identifier, never by parsing the close error.
	res, qerr := provider.Query(ctx, att.MerchantOrderID)
	if qerr != nil {
		// Both legs failed: the outcome stays unknown — surface it, mark
		// nothing, keep the payable entry for a retry.
		return OrderView{}, fmt.Errorf("%w: channel query failed during close recovery: %v", ErrPaymentObservationUnavailable, qerr)
	}
	switch res.State {
	case payment.StateSucceeded:
		txn := res.ProviderID
		if txn == "" {
			txn = att.MerchantOrderID
		}
		// (#84/G2) The decisive query carries the COLLECTED amount: a
		// mismatched collection is retained as an anomaly instead of being
		// confirmed at the attempt's face (the fund fact survives the close,
		// the fulfillment right is never minted from a wrong collection).
		if strings.TrimSpace(res.AmountCurrency) != "" && res.AmountCurrency != att.Currency {
			return s.recoverMismatchedCollection(ctx, row, att, txn, res.AmountFen, res.AmountCurrency)
		}
		if err := validateCollectionObservation(res.AmountFen, res.AmountCurrency); err != nil {
			return OrderView{}, err
		}
		if collectedAmountMismatch(res.AmountFen, att.AmountFen, res.AmountCurrency, att.Currency) {
			return s.recoverMismatchedCollection(ctx, row, att, txn, res.AmountFen, res.AmountCurrency)
		}
		if err := s.orders.ConfirmPayment(ctx, domain.PaymentFact{
			Provider: att.Provider, Merchant: att.Merchant,
			AttemptID: att.MerchantOrderID, OrderID: orderID, TenantID: tenantID,
			Amount: domain.CNYFen(att.AmountFen), Currency: att.Currency,
			Transaction: txn, State: payment.StateSucceeded.String(),
		}); err != nil {
			return OrderView{}, err
		}
		after, err := s.orders.GetOrder(ctx, orderID)
		if err != nil {
			return OrderView{}, err
		}
		return OrderView{ID: after.ID, QuoteID: after.QuoteID, State: after.State,
			AmountFen: after.AmountFen, Currency: after.Currency, Provider: att.Provider,
			Version: after.Version}, nil
	case payment.StateClosed:
		// The channel already closed it (or the close landed first): same
		// landing as a clean close.
		return landClosed()
	default:
		// NOTPAY / unknown: the close did not resolve and the channel has
		// no terminal answer — return the original close error, mark
		// nothing, keep the payable entry.
		return OrderView{}, cerr
	}
}

// ChangePlanView is the Commerce.ChangePlan answer: either an UPGRADE order
// (prorated, immediately effective once paid, same recoverable checkout
// contract as CreateOrder) or a SCHEDULED switch that takes effect at the
// end of the currently paid period (B17: a downgrade never ends a paid
// period early).
type ChangePlanView struct {
	Change               string     `json:"change"`
	Order                *OrderView `json:"order,omitempty"`
	ScheduledPlanKey     string     `json:"scheduled_plan_key,omitempty"`
	ScheduledPlanVersion int64      `json:"scheduled_plan_version,omitempty"`
	EffectiveAt          string     `json:"effective_at,omitempty"`
	SubscriptionVersion  int64      `json:"subscription_version"`
}

// ChangePlan implements Commerce.ChangePlan: quote_id names the TARGET plan
// (a quote cut through CreateQuote), expected_subscription_version guards
// against concurrent changes (a conflict answers with a re-quote signal,
// never a silent overwrite). A price INCREASE settles as a prorated upgrade
// order for the REMAINING time of the current paid period (B17: 补差价后
// 立即生效); anything else is scheduled to take effect at paid_until — the
// paid period is never cut short.
func (s *OrderService) ChangePlan(ctx context.Context, tenantID uint64, quoteID string, expectedSubscriptionVersion int64, providerName string) (ChangePlanView, error) {
	q, snap, err := s.quoteForTenant(ctx, tenantID, quoteID)
	if err != nil {
		return ChangePlanView{}, err
	}
	sub, err := s.subs.Current(ctx, tenantID)
	if errors.Is(err, repocommercial.ErrSubscriptionNotFound) {
		return ChangePlanView{}, ErrNoSubscriptionToChange
	}
	if err != nil {
		return ChangePlanView{}, err
	}
	if expectedSubscriptionVersion != sub.Version {
		return ChangePlanView{}, repocommercial.ErrSubscriptionVersionConflict
	}
	var current domain.PlanVersion
	if err := json.Unmarshal([]byte(sub.PlanSnapshotJSON), &current); err != nil {
		return ChangePlanView{}, fmt.Errorf("%w: %v", repocommercial.ErrInvalidPlanRow, err)
	}

	now := time.Now()
	if snap.PriceFen > int64(current.Price) {
		// UPGRADE: charge the price difference prorated over the REMAINING
		// span of the paid period (anchor..paid_until), rounded up once so
		// the platform never undercharges a partial period.
		duration := sub.PaidUntil.Sub(sub.Anchor)
		remaining := sub.PaidUntil.Sub(now)
		if remaining < 0 {
			remaining = 0
		}
		if duration <= 0 {
			duration = remaining
		}
		amount, err := domain.Prorate(snap.PriceFen-int64(current.Price),
			int64(remaining), int64(duration), true)
		if err != nil {
			return ChangePlanView{}, err
		}
		if amount < 1 {
			// A proration that rounds below one fen still settles as a
			// one-fen order: the pipeline requires a positive amount and a
			// free switch would bypass the paid-upgrade path entirely.
			amount = 1
		}
		order, err := s.openOrder(ctx, tenantID, q, providerName, amount, domain.OrderKindUpgrade,
			&subscriptionClaim{id: sub.ID, version: sub.Version})
		if err != nil {
			return ChangePlanView{}, err
		}
		return ChangePlanView{Change: domain.ChangeKindUpgrade.String(), Order: &order}, nil
	}

	// SCHEDULED switch (downgrade or lateral): takes effect at paid_until —
	// the paid interval is never cut short and a purchased future interval
	// (early renewal) is never overwritten: the arrangement lives in its OWN
	// column, one pending switch at a time. The quote is consumed with a
	// marker (no order row: nothing is charged) and the version guard makes
	// a concurrent change lose cleanly to a re-quote answer.
	targetPlan, err := s.quotes.GetPlan(ctx, snap.PlanKey, snap.PlanVersion)
	if err != nil {
		return ChangePlanView{}, err
	}
	change := domain.ScheduledPlanChange{
		PlanKey: snap.PlanKey, PlanVersion: snap.PlanVersion,
		PlanSnapshotJSON: targetPlan.DefinitionJSON,
		EffectiveAt:      sub.PaidUntil, QuoteID: q.ID, CreatedAt: now,
	}
	marker := "chg_" + newLeaseToken()
	if err := s.subs.SchedulePlanChange(ctx, sub, change, q.ID, q.SubscriptionVersion, marker, now); err != nil {
		return ChangePlanView{}, err
	}
	return ChangePlanView{
		Change:               domain.ChangeKindScheduledSwitch.String(),
		ScheduledPlanKey:     snap.PlanKey,
		ScheduledPlanVersion: snap.PlanVersion,
		EffectiveAt:          change.EffectiveAt.UTC().Format(time.RFC3339),
		SubscriptionVersion:  sub.Version + 1,
	}, nil
}
