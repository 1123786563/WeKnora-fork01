package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	ErrPaymentProviderUnconfigured = errors.New("payment_provider_unconfigured")
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
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.Subscription{}); err != nil {
		return nil, err
	}
	// (R2-26) Database-level invariant: at most ONE payable (not
	// channel-failed) pending purchase order per tenant. The purchase
	// path's read-decide-write only narrowed the race window — two
	// concurrent POSTs with two fresh quotes could both pass the pre-checks
	// and commit; the partial unique index closes the gap at insert time
	// (loser answers ErrPurchasePendingExists and replays the winner, the
	// ErrQuoteAlreadyUsed shape). A deployment holding pre-invariant
	// duplicates fails HERE loudly (the index cannot be created) instead of
	// silently continuing without the invariant. SQLite and PostgreSQL
	// share this partial-index syntax.
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_pending_per_tenant
		ON commercial_orders (tenant_id)
		WHERE kind = 'purchase' AND state = 'pending' AND channel_failed = 0`).Error; err != nil {
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
	ID                   string `json:"id"`
	QuoteID              string `json:"quote_id"`
	State                string `json:"state"`
	AmountFen            int64  `json:"amount_fen"`
	Currency             string `json:"currency"`
	Provider             string `json:"provider,omitempty"`
	CheckoutURL          string `json:"checkout_url,omitempty"`
	CheckoutError        string `json:"checkout_error,omitempty"`
	CheckoutLinkDegraded bool   `json:"checkout_link_degraded,omitempty"`
	Version              int64  `json:"version"`
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
	if err != nil {
		// The channel call failed (or timed out into StateUnknown): the
		// pending order and attempt REMAIN (channel-failed — see
		// MarkChannelFailed below), the quote is consumed, and the response
		// still carries the operation ID + state so recovery goes through
		// GetOrder — never through a second checkout of the same quote.
		// (R2-28) The channel failure is ALSO persisted on the row: the
		// channel-failed pending order is not a payable entry and must not
		// block a fresh quote's checkout.
		if ferr := s.orders.MarkChannelFailed(ctx, id); ferr != nil {
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
	if err := s.orders.SetCheckoutURL(ctx, id, res.CheckoutURL); err != nil {
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
	out := make([]OrderView, 0, len(rows))
	for _, r := range rows {
		out = append(out, OrderView{ID: r.ID, QuoteID: r.QuoteID, State: r.State,
			AmountFen: r.AmountFen, Currency: r.Currency, Version: r.Version})
	}
	return out, nil
}

// RecoverOrderStatus is the payment state-recovery path for a PENDING order
// whose channel result may have been missed (closed tab, lost callback): it
// re-queries the provider by the ORIGINAL merchant order id and, on a
// succeeded fact, confirms the payment through the SAME ConfirmPayment
// transaction the callback path uses. Paid/fulfilled orders are returned
// untouched; a pending order whose channel is still pending stays pending.
func (s *OrderService) RecoverOrderStatus(ctx context.Context, tenantID uint64, orderID string) (OrderView, error) {
	row, err := s.orders.GetOrder(ctx, orderID)
	if err != nil {
		return OrderView{}, err
	}
	if row.TenantID != tenantID {
		return OrderView{}, ErrOrderTenantMismatch
	}
	if row.State != domain.OrderStatePending {
		return OrderView{ID: row.ID, QuoteID: row.QuoteID, State: row.State,
			AmountFen: row.AmountFen, Currency: row.Currency, Version: row.Version}, nil
	}
	att, err := s.orders.FirstPendingAttempt(ctx, orderID)
	if errors.Is(err, repocommercial.ErrPaymentAttemptNotFound) {
		// No attempt ever registered (e.g. channel unconfigured at creation):
		// honestly pending, nothing to recover. The persisted checkout link
		// (R1-35) still rides along so the customer keeps a payment entry.
		return OrderView{ID: row.ID, QuoteID: row.QuoteID, State: row.State,
			AmountFen: row.AmountFen, Currency: row.Currency, CheckoutURL: row.CheckoutURL,
			Version: row.Version}, nil
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
		return OrderView{}, fmt.Errorf("channel query failed for %s: %w", orderID, err)
	}
	if res.State == payment.StateSucceeded {
		txn := res.ProviderID
		if txn == "" {
			txn = att.MerchantOrderID
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
	return OrderView{ID: row.ID, QuoteID: row.QuoteID, State: row.State,
		AmountFen: row.AmountFen, Currency: row.Currency, Provider: att.Provider,
		CheckoutURL: row.CheckoutURL, Version: row.Version}, nil
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
