package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/payment"

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

// quoteSnapshot is the exact offer frozen at quote time: the published plan
// version it buys, its exact price in fen and the monthly credits it grants.
// The order is priced from THIS snapshot, never re-read from the (possibly
// republished) catalog at order time.
type quoteSnapshot struct {
	PlanKey      string `json:"plan_key"`
	PlanVersion  int64  `json:"plan_version"`
	PriceFen     int64  `json:"price_fen"`
	CreditsMicro int64  `json:"credits_micro"`
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
	return &OrderService{
		quotes:    repocommercial.NewCatalogStore(db),
		orders:    repocommercial.NewOrderStore(db),
		subs:      repocommercial.NewSubscriptionStore(db),
		providers: providers,
	}, nil
}

// QuoteView is the produced quote projection.
type QuoteView struct {
	ID           string `json:"id"`
	PlanKey      string `json:"plan_key"`
	PlanVersion  int64  `json:"plan_version"`
	AmountFen    int64  `json:"amount_fen"`
	CreditsMicro int64  `json:"credits_micro"`
	ExpiresAt    string `json:"expires_at"`
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
	}, nil
}

// OrderView is the produced order projection. CheckoutURL carries the
// customer-facing payment link when a channel adapter produced one.
// CheckoutError is non-empty when the channel call failed AFTER the order
// was durably opened: the pending order stays recoverable through
// GetOrder/RecoverOrderStatus, and the write answer still carries the
// operation ID and state as the product contract requires.
type OrderView struct {
	ID            string `json:"id"`
	QuoteID       string `json:"quote_id"`
	State         string `json:"state"`
	AmountFen     int64  `json:"amount_fen"`
	Currency      string `json:"currency"`
	Provider      string `json:"provider,omitempty"`
	CheckoutURL   string `json:"checkout_url,omitempty"`
	CheckoutError string `json:"checkout_error,omitempty"`
	Version       int64  `json:"version"`
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
	return s.openOrder(ctx, tenantID, q, providerName, snap.PriceFen)
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

// openOrder runs the shared checkout path: atomic quote+order+attempt unit,
// then the channel call. A channel failure returns the PENDING order view
// with CheckoutError set and a nil error — the write already succeeded.
func (s *OrderService) openOrder(ctx context.Context, tenantID uint64, q repocommercial.QuoteRow, providerName string, amountFen int64) (OrderView, error) {
	provider, ok := s.providers[providerName]
	if !ok || provider == nil {
		return OrderView{}, fmt.Errorf("%w: %q", ErrPaymentProviderUnconfigured, providerName)
	}
	subVersion, err := s.subs.LatestVersion(ctx, tenantID)
	if err != nil {
		return OrderView{}, err
	}
	id := "ord_" + newLeaseToken()
	merchantOrderID := "mo_" + newLeaseToken()
	if err := s.orders.OpenOrder(ctx, repocommercial.OrderRow{
		ID: id, TenantID: tenantID, QuoteID: q.ID,
		AmountFen: amountFen, Currency: "CNY",
	}, repocommercial.PaymentAttemptRow{
		ID: "att_" + newLeaseToken(), TenantID: tenantID, OrderID: id,
		Provider: providerName, Merchant: providerName, MerchantOrderID: merchantOrderID,
		AmountFen: amountFen, Currency: "CNY",
	}, q.ID, subVersion, time.Now()); err != nil {
		return OrderView{}, err
	}
	res, err := provider.Create(ctx, payment.OrderRequest{
		OrderID: id, MerchantOrderID: merchantOrderID,
		AmountFen: amountFen, Currency: "CNY",
	})
	if err != nil {
		// The channel call failed (or timed out into StateUnknown): the
		// pending order and attempt REMAIN, the quote is consumed, and the
		// response still carries the operation ID + state so recovery goes
		// through GetOrder — never through a second checkout of the same
		// quote.
		return OrderView{ID: id, QuoteID: q.ID, State: domain.OrderStatePending,
			AmountFen: amountFen, Currency: "CNY", Provider: providerName, Version: 1,
			CheckoutError: fmt.Sprintf("channel checkout failed for %s: %v", id, err)}, nil
	}
	return OrderView{ID: id, QuoteID: q.ID, State: domain.OrderStatePending,
		AmountFen: amountFen, Currency: "CNY", Provider: providerName,
		CheckoutURL: res.CheckoutURL, Version: 1}, nil
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
		// honestly pending, nothing to recover.
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
	return OrderView{ID: row.ID, QuoteID: row.QuoteID, State: row.State,
		AmountFen: row.AmountFen, Currency: row.Currency, Provider: att.Provider,
		Version: row.Version}, nil
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
		order, err := s.openOrder(ctx, tenantID, q, providerName, amount)
		if err != nil {
			return ChangePlanView{}, err
		}
		return ChangePlanView{Change: "upgrade", Order: &order}, nil
	}

	// SCHEDULED switch (downgrade or lateral): takes effect at paid_until.
	// The quote is consumed with a marker (no order row: nothing is charged)
	// and the future interval is recorded under the version guard, so the
	// arrangement is durable, visible in the subscription row, and a
	// concurrent change loses cleanly to a re-quote answer.
	future := struct {
		PlanKey     string `json:"plan_key"`
		PlanVersion int64  `json:"plan_version"`
		EffectiveAt string `json:"effective_at"`
		QuoteID     string `json:"quote_id"`
	}{PlanKey: snap.PlanKey, PlanVersion: snap.PlanVersion,
		EffectiveAt: sub.PaidUntil.UTC().Format(time.RFC3339), QuoteID: q.ID}
	futureJSON, err := json.Marshal(future)
	if err != nil {
		return ChangePlanView{}, err
	}
	marker := "chg_" + newLeaseToken()
	if err := s.subs.SchedulePlanChange(ctx, sub, q.ID, q.SubscriptionVersion, marker, string(futureJSON), now); err != nil {
		return ChangePlanView{}, err
	}
	return ChangePlanView{
		Change:               "scheduled_switch",
		ScheduledPlanKey:     snap.PlanKey,
		ScheduledPlanVersion: snap.PlanVersion,
		EffectiveAt:          future.EffectiveAt,
		SubscriptionVersion:  sub.Version + 1,
	}, nil
}
