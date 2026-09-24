package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"

	"gorm.io/gorm"
)

// PurchaseService coordinates the payment-gated purchase (#81): a valid
// frozen Quote → the lazy billing-account ensure (#78) → the authority's
// payment-gated purchase subscription (identity-idempotent, #80 algorithm
// with activation gating) → the quote-vs-authority match gate (AC2: ANY
// mismatch aborts BEFORE a channel payment request exists) → the existing
// order replay or one atomic CreateOrder (AC4 retry/concurrency). The
// service holds no direct authority transport: every platform step goes
// through the frozen seam.
var (
	// ErrInvoiceQuoteMismatch (AC2): the authoritative purchase face does
	// not match the Quote (plan code, currency or amount) — the purchase
	// aborts and NO channel payment request is created.
	ErrInvoiceQuoteMismatch = errors.New("invoice_quote_mismatch")
	// ErrQuoteLegacySnapshot: the quote was cut before the #81 freeze
	// fields existed — the caller must re-quote, never guess.
	ErrQuoteLegacySnapshot = errors.New("quote_legacy_snapshot")
	// ErrPurchasePlanConflict (AC4): a purchase subscription already held on
	// a DIFFERENT plan — the first purchase wins, the late caller re-quotes.
	ErrPurchasePlanConflict = errors.New("purchase_plan_conflict")
	// ErrPurchasePlanCharges: the plan version carries usage charges, which
	// the first-period line-item equivalence slice does not cover yet (D2).
	ErrPurchasePlanCharges = errors.New("purchase_plan_charges")
)

// PurchaseView is the Billing API projection of one purchase. State is the
// CLOSED product token set (awaiting_payment|active|absent|canceled — D4);
// Reason is the closed platform-failure token, empty on a healthy answer.
type PurchaseView struct {
	State       string     `json:"state"` // 闭合 awaiting_payment|active|absent|canceled
	Order       *OrderView `json:"order,omitempty"`
	PlanKey     string     `json:"plan_key,omitempty"`
	PlanVersion int64      `json:"plan_version,omitempty"`
	AmountFen   int64      `json:"amount_fen,omitempty"`
	Currency    string     `json:"currency,omitempty"`
	Reason      string     `json:"reason,omitempty"` // 闭合 unconfigured|unreachable|invalid_response|unsupported
}

// PurchaseService wires the collaborators; db is kept for future
// transactional units (the service holds no queries of its own yet — every
// read goes through the repository stores inside the collaborators).
type PurchaseService struct {
	db       *gorm.DB
	accounts *BillingAccountService
	plans    *PlanVersionService
	orders   *OrderService
	platform domain.CommercialPlatform
}

// NewPurchaseService wires the purchase coordination over the shared
// collaborators. platform may be nil (blocked-env): the purchase then
// surfaces the closed unconfigured posture instead of fabricating states.
func NewPurchaseService(db *gorm.DB, accounts *BillingAccountService, plans *PlanVersionService,
	orders *OrderService, platform domain.CommercialPlatform) (*PurchaseService, error) {
	if db == nil {
		return nil, errors.New("purchase_database_missing")
	}
	if accounts == nil || plans == nil || orders == nil {
		return nil, errors.New("purchase_collaborators_missing")
	}
	return &PurchaseService{db: db, accounts: accounts, plans: plans, orders: orders, platform: platform}, nil
}

// Purchase runs the gated purchase algorithm. Caller-visible errors are the
// closed purchase sentinels (and the repository quote guards); platform
// failures are STATES (pending view + closed Reason, the #78 posture).
func (s *PurchaseService) Purchase(ctx context.Context, tenantID uint64, quoteID, providerName, actor string) (PurchaseView, error) {
	if tenantID == 0 || quoteID == "" {
		return PurchaseView{}, repocommercial.ErrInvalidQuoteRow
	}
	if s.platform == nil {
		return PurchaseView{State: domain.PurchaseStateAbsent, Reason: "unconfigured"}, nil
	}

	// 1. The frozen quote (tenant-guarded read).
	q, snap, err := s.orders.QuoteSnapshotForTenant(ctx, tenantID, quoteID)
	if err != nil {
		return PurchaseView{}, err
	}
	if snap.Currency == "" {
		// A pre-#81 snapshot cannot prove its currency/line items — re-quote.
		return PurchaseView{}, ErrQuoteLegacySnapshot
	}
	// 2. Expiry pre-check (same bound the order consumption enforces).
	if !q.ExpiresAt.After(time.Now()) {
		return PurchaseView{}, repocommercial.ErrQuoteExpired
	}
	// 2b. Channel availability PRE-check (review F1): a pure map lookup —
	// the checkout provider must be wired BEFORE any seam-side effect, or a
	// blocked-env submit would leave a payment-gated subscription (and its
	// gating invoice) on the authority with no way to pay it.
	if !s.orders.ProviderConfigured(providerName) {
		return PurchaseView{}, fmt.Errorf("%w: %q", ErrPaymentProviderUnconfigured, providerName)
	}
	// 3. The publication pins the deterministic plan code (the version must
	// be published first — #79 deliverable).
	pub, err := s.plans.GetPublication(ctx, snap.PlanKey, snap.PlanVersion)
	if err != nil {
		if errors.Is(err, repocommercial.ErrPublicationNotFound) || errors.Is(err, repocommercial.ErrPlanNotFound) {
			return PurchaseView{}, repocommercial.ErrPlanNotFound
		}
		return PurchaseView{}, err
	}
	// 4. The no-charges slice (D2): publishable usage charges would enter
	// pay-in-arrears invoices the first-period line equivalence cannot see.
	if err := s.ensureNoCharges(ctx, snap.PlanKey, snap.PlanVersion); err != nil {
		return PurchaseView{}, err
	}
	// 5. The lazy billing-account ensure (#78); only a linked account may
	// purchase.
	acct, err := s.accounts.EnsureBillingAccount(ctx, tenantID, "space", actor)
	if err != nil {
		return PurchaseView{}, err
	}
	if acct.State != BillingAccountLinked {
		return PurchaseView{State: domain.PurchaseStateAbsent, Reason: platformReason(errors.New(acct.Reason))}, nil
	}

	// 6. The payment-gated subscription create (identity-idempotent at the
	// seam: a replay NEVER re-POSTs, F7).
	cmd := domain.Command{
		Kind:  domain.CommandKindCreatePurchaseSubscription,
		Key:   domain.CreatePurchaseSubscriptionCommandKey(domain.ExternalPurchaseSubscriptionID(tenantID), pub.PlanCode),
		Actor: actor, Reason: "purchase",
		Payload: domain.CreatePurchaseSubscriptionPayload{
			TenantID:                       tenantID,
			ExternalCustomerID:             domain.ExternalCustomerID(tenantID),
			ExternalPurchaseSubscriptionID: domain.ExternalPurchaseSubscriptionID(tenantID),
			PlanCode:                       pub.PlanCode,
			AmountFen:                      snap.PriceFen,
			Currency:                       domain.CurrencyCNY,
		},
	}
	if _, err := s.platform.SubmitCommand(ctx, cmd); err != nil {
		if errors.Is(err, domain.ErrPlatformInvalidResponse) {
			// Distinguish the definitive concurrent-plan-change conflict
			// from other invalid answers by the authority's own truth.
			if held, serr := s.platform.ReadSnapshot(ctx, domain.SnapshotQuery{
				Kind: domain.SnapshotKindPurchase, TenantID: tenantID,
			}); serr == nil && held.Purchase != nil &&
				held.Purchase.State == domain.PurchaseStateAwaitingPayment &&
				held.Purchase.PlanCode != pub.PlanCode {
				return PurchaseView{}, ErrPurchasePlanConflict
			}
		}
		return s.pendingView(err), nil
	}

	// 7. The match gate (AC2): the authoritative purchase face must equal
	// the Quote in plan code, currency and integer amount — ANY mismatch
	// aborts BEFORE any channel payment request exists.
	psnap, err := s.platform.ReadSnapshot(ctx, domain.SnapshotQuery{
		Kind: domain.SnapshotKindPurchase, TenantID: tenantID,
	})
	if err != nil {
		return s.pendingView(err), nil
	}
	p := psnap.Purchase
	if p == nil || p.State == domain.PurchaseStateAbsent ||
		p.PlanCode != pub.PlanCode ||
		p.Currency != domain.CurrencyCNY ||
		p.Currency != snap.Currency ||
		p.AmountFen != snap.PriceFen {
		return PurchaseView{}, ErrInvoiceQuoteMismatch
	}

	// 8. The order replay-or-create (AC4): an existing order for this quote
	// returns verbatim; otherwise ONE atomic CreateOrder; a concurrent
	// consumption race re-reads the winner.
	if existing, err := s.orders.orders.GetOrderByQuote(ctx, tenantID, quoteID); err == nil {
		ov := orderViewFromRow(existing)
		return s.purchaseView(p, snap, pub, &ov), nil
	} else if !errors.Is(err, repocommercial.ErrOrderNotFound) {
		return PurchaseView{}, err
	}
	ov, err := s.orders.CreateOrder(ctx, tenantID, quoteID, providerName)
	if errors.Is(err, repocommercial.ErrQuoteAlreadyUsed) {
		// The concurrent race lost the quote consumption: answer the
		// winner's order, never a second channel request.
		existing, gerr := s.orders.orders.GetOrderByQuote(ctx, tenantID, quoteID)
		if gerr != nil {
			return PurchaseView{}, repocommercial.ErrQuoteAlreadyUsed
		}
		ovExisting := orderViewFromRow(existing)
		return s.purchaseView(p, snap, pub, &ovExisting), nil
	}
	if err != nil {
		return PurchaseView{}, err
	}
	return s.purchaseView(p, snap, pub, &ov), nil
}

// PurchaseStatus answers the purchase projection for one tenant: the
// authority truth (closed state + frozen price face) plus the local order
// when one exists.
func (s *PurchaseService) PurchaseStatus(ctx context.Context, tenantID uint64) (PurchaseView, error) {
	if tenantID == 0 {
		return PurchaseView{}, repocommercial.ErrInvalidQuoteRow
	}
	if s.platform == nil {
		return PurchaseView{State: domain.PurchaseStateAbsent, Reason: "unconfigured"}, nil
	}
	psnap, err := s.platform.ReadSnapshot(ctx, domain.SnapshotQuery{
		Kind: domain.SnapshotKindPurchase, TenantID: tenantID,
	})
	if err != nil {
		return s.pendingView(err), nil
	}
	p := psnap.Purchase
	if p == nil {
		return PurchaseView{State: domain.PurchaseStateAbsent}, nil
	}
	out := PurchaseView{State: p.State, AmountFen: p.AmountFen, Currency: p.Currency}
	if pub, err := s.plans.FindPublicationByCode(ctx, p.PlanCode); err == nil {
		out.PlanKey = pub.PlanKey
		out.PlanVersion = pub.Version
	}
	if rows, err := s.orders.orders.ListOrdersByTenant(ctx, tenantID); err == nil && len(rows) > 0 {
		ov := orderViewFromRow(rows[0])
		out.Order = &ov
	}
	return out, nil
}

// ensureNoCharges reads the frozen definition and refuses versions with
// usage charges (the D2 slice boundary — they are future top-slice work).
func (s *PurchaseService) ensureNoCharges(ctx context.Context, planKey string, version int64) error {
	view, err := s.plans.GetVersion(ctx, planKey, version)
	if err != nil {
		return err
	}
	var def domain.PlanVersion
	if err := json.Unmarshal([]byte(view.DefinitionJSON), &def); err != nil {
		return ErrQuoteLegacySnapshot
	}
	if len(def.Charges) > 0 {
		return ErrPurchasePlanCharges
	}
	return nil
}

// purchaseView assembles the healthy answer.
func (s *PurchaseService) purchaseView(p *domain.PurchaseSnapshot, snap quoteSnapshot, pub repocommercial.PublicationRow, ov *OrderView) PurchaseView {
	return PurchaseView{
		State:       p.State,
		Order:       ov,
		PlanKey:     snap.PlanKey,
		PlanVersion: pub.Version,
		AmountFen:   snap.PriceFen,
		Currency:    domain.CurrencyCNY,
	}
}

// pendingView maps a platform failure onto the closed posture: the
// purchase is NOT confirmable, the reason is the closed token — never a
// fabricated state (the #78 outage posture).
func (s *PurchaseService) pendingView(err error) PurchaseView {
	return PurchaseView{State: domain.PurchaseStateAbsent, Reason: platformReason(err)}
}

// orderViewFromRow projects a stored order row (the OrderService keeps this
// mapping private to its list paths; the purchase replay needs it too).
func orderViewFromRow(r repocommercial.OrderRow) OrderView {
	return OrderView{ID: r.ID, QuoteID: r.QuoteID, State: r.State,
		AmountFen: r.AmountFen, Currency: r.Currency, Version: r.Version}
}
