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

// OrderService implements the order pipeline over the existing stores:
// quote (price an exact published plan for one space), order (consume the
// quote, register the attempt, open the channel checkout), and payment
// status recovery (re-query the channel by the ORIGINAL merchant order id
// and confirm through the same ConfirmPayment transaction the callback
// path uses).
type OrderService struct {
	db        *gorm.DB
	quotes    *repocommercial.CatalogStore
	orders    *repocommercial.OrderStore
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
		db:        db,
		quotes:    repocommercial.NewCatalogStore(db),
		orders:    repocommercial.NewOrderStore(db),
		providers: providers,
	}, nil
}

// subscriptionVersion returns the latest subscription version of the space,
// or 0 for a space on the base tier (no subscription row yet). Quotes are
// cut against this version so a concurrent plan change invalidates them.
func (s *OrderService) subscriptionVersion(ctx context.Context, tenantID uint64) (int64, error) {
	var version int64
	err := s.db.WithContext(ctx).Raw(
		`SELECT version FROM commercial_subscriptions
		WHERE tenant_id = ? ORDER BY version DESC LIMIT 1`, tenantID).Scan(&version).Error
	return version, err
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
// subscription change refuses the quote at consumption time.
func (s *OrderService) CreateQuote(ctx context.Context, tenantID uint64, planKey string) (QuoteView, error) {
	if tenantID == 0 || planKey == "" {
		return QuoteView{}, repocommercial.ErrInvalidQuoteRow
	}
	// Latest published definition of the plan.
	var row repocommercial.PlanRow
	err := s.db.WithContext(ctx).
		Where("plan_key = ? AND state = ?", planKey, domain.PlanStatePublished).
		Order("version DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return QuoteView{}, repocommercial.ErrPlanNotFound
	}
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
	subVersion, err := s.subscriptionVersion(ctx, tenantID)
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
type OrderView struct {
	ID          string `json:"id"`
	QuoteID     string `json:"quote_id"`
	State       string `json:"state"`
	AmountFen   int64  `json:"amount_fen"`
	Currency    string `json:"currency"`
	Provider    string `json:"provider,omitempty"`
	CheckoutURL string `json:"checkout_url,omitempty"`
	Version     int64  `json:"version"`
}

// CreateOrder consumes the quote and opens one pending order with one
// registered pending attempt. Order of operations: the order row is written
// FIRST — its quote_id UNIQUE index makes a quote consumable by exactly one
// order even under a race — and the quote is consumed immediately after; a
// rejected consumption (expired, superseded) deletes the just-written row so
// no orphan pending order survives a failed checkout.
func (s *OrderService) CreateOrder(ctx context.Context, tenantID uint64, quoteID, providerName string) (OrderView, error) {
	if tenantID == 0 || quoteID == "" {
		return OrderView{}, repocommercial.ErrInvalidOrderRow
	}
	provider, ok := s.providers[providerName]
	if !ok || provider == nil {
		return OrderView{}, fmt.Errorf("%w: %q", ErrPaymentProviderUnconfigured, providerName)
	}
	q, err := s.quotes.GetQuote(ctx, quoteID)
	if err != nil {
		return OrderView{}, err
	}
	if q.TenantID != tenantID {
		return OrderView{}, ErrQuoteTenantMismatch
	}
	var snap quoteSnapshot
	if err := json.Unmarshal([]byte(q.SnapshotJSON), &snap); err != nil {
		return OrderView{}, fmt.Errorf("%w: %v", repocommercial.ErrInvalidQuoteRow, err)
	}

	id := "ord_" + newLeaseToken()
	merchantOrderID := "mo_" + newLeaseToken()
	if err := s.orders.CreateOrder(ctx, repocommercial.OrderRow{
		ID: id, TenantID: tenantID, QuoteID: quoteID,
		AmountFen: snap.PriceFen, Currency: "CNY",
	}); err != nil {
		return OrderView{}, err
	}
	subVersion, err := s.subscriptionVersion(ctx, tenantID)
	if err != nil {
		return OrderView{}, err
	}
	if _, err := s.quotes.ConsumeQuote(ctx, quoteID, subVersion, id, time.Now()); err != nil {
		// The quote cannot settle: remove the orphan order row.
		_ = s.db.WithContext(ctx).Where("id = ?", id).Delete(&repocommercial.OrderRow{}).Error
		return OrderView{}, err
	}
	if err := s.orders.RegisterAttempt(ctx, repocommercial.PaymentAttemptRow{
		ID: "att_" + newLeaseToken(), TenantID: tenantID, OrderID: id,
		Provider: providerName, Merchant: providerName, MerchantOrderID: merchantOrderID,
		AmountFen: snap.PriceFen, Currency: "CNY",
	}); err != nil {
		return OrderView{}, err
	}
	res, err := provider.Create(ctx, payment.OrderRequest{
		OrderID: id, MerchantOrderID: merchantOrderID,
		AmountFen: snap.PriceFen, Currency: "CNY",
	})
	if err != nil {
		// The channel call failed (or timed out into StateUnknown): the
		// pending order and attempt REMAIN so recovery re-queries the
		// original identifier instead of re-keying a second checkout.
		return OrderView{ID: id, QuoteID: quoteID, State: domain.OrderStatePending,
			AmountFen: snap.PriceFen, Currency: "CNY", Provider: providerName, Version: 1},
			fmt.Errorf("channel checkout failed for %s: %w", id, err)
	}
	return OrderView{ID: id, QuoteID: quoteID, State: domain.OrderStatePending,
		AmountFen: snap.PriceFen, Currency: "CNY", Provider: providerName,
		CheckoutURL: res.CheckoutURL, Version: 1}, nil
}

// ListOrders returns the caller space's orders, newest first. Never crosses
// spaces: the tenant comes exclusively from the authenticated context.
func (s *OrderService) ListOrders(ctx context.Context, tenantID uint64) ([]OrderView, error) {
	if tenantID == 0 {
		return nil, repocommercial.ErrOrderNotFound
	}
	var rows []repocommercial.OrderRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).Order("id DESC").Find(&rows).Error; err != nil {
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
	var att repocommercial.PaymentAttemptRow
	err = s.db.WithContext(ctx).
		Where("order_id = ? AND state = ?", orderID, repocommercial.PaymentAttemptStatePending).
		Order("id").First(&att).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
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
