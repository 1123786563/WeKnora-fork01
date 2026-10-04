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
	// ErrPurchaseNotAwaiting (R1-V03): the authority already holds the
	// purchase in a NON-awaiting state (active/canceled) while the caller
	// submitted a fresh quote — a new channel order must NOT be opened for it
	// (an active plan would be charged a second time with no gating invoice;
	// a canceled subscription's gating invoice can never be settled). The
	// existing-order replay path stays open: a POST replay after payment
	// still returns the paid order verbatim.
	ErrPurchaseNotAwaiting = errors.New("purchase_not_awaiting_payment")
	// ErrPurchaseUnavailable (R1-V04): the platform seam could not answer
	// (unconfigured seam, failed ensure, failed submit or failed snapshot
	// read) — the POST creates NOTHING and answers 503 with the closed
	// reason token, never a 201 with a fabricated-absent view. GET
	// PurchaseStatus keeps the state-view posture.
	ErrPurchaseUnavailable = errors.New("purchase_unavailable")
	// ErrPurchasePlanInvalid (R1-V18): the frozen plan-version definition is
	// corrupt and does not deserialize — a data problem, never a
	// "quote predates the freeze" re-quote signal.
	ErrPurchasePlanInvalid = errors.New("purchase_plan_invalid")
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
	closures *repocommercial.WorkspaceClosureStore
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
	return &PurchaseService{db: db, accounts: accounts, plans: plans, orders: orders,
		closures: repocommercial.NewWorkspaceClosureStore(db), platform: platform}, nil
}

// Purchase runs the gated purchase algorithm. Caller-visible errors are the
// closed purchase sentinels (and the repository quote guards); a platform
// failure during the POST surfaces ErrPurchaseUnavailable (503 + closed
// reason on the wire) — the POST never answers success for a purchase it
// did not create. Only the GET status endpoint keeps the state-view
// posture (pendingView).
func (s *PurchaseService) Purchase(ctx context.Context, tenantID uint64, quoteID, providerName, actor, displayName string) (PurchaseView, error) {
	if tenantID == 0 || quoteID == "" {
		return PurchaseView{}, repocommercial.ErrInvalidQuoteRow
	}
	// #102 / Lago 30, AC1: a closed workspace accepts no new purchase.
	if s.closures != nil {
		if _, err := s.closures.Get(ctx, tenantID); err == nil {
			return PurchaseView{}, domain.ErrWorkspaceClosed
		}
	}
	if s.platform == nil {
		// R1-V04: a blocked seam is a failure for the POST surface (503 +
		// closed reason), not a fabricated-absent success.
		return PurchaseView{}, fmt.Errorf("%w: %w", ErrPurchaseUnavailable, domain.ErrPlatformUnconfigured)
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
	acct, err := s.accounts.EnsureBillingAccount(ctx, tenantID, displayName, actor)
	if err != nil {
		return PurchaseView{}, err
	}
	if acct.State != BillingAccountLinked {
		// R1-V04/R1-V16: an unlinked account is a failed ensure — the POST
		// answers the unavailable sentinel carrying the account's own closed
		// token (never re-matched as a new error), never a 201 absent view.
		return PurchaseView{}, fmt.Errorf("%w: %w", ErrPurchaseUnavailable, platformSentinel(acct.Reason))
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
			// Distinguish the definitive plan-change conflict from other
			// invalid answers by the authority's own truth. (OCR final
			// audit 11) ANY held state counts — an ACTIVE held purchase on
			// a different plan is just as deterministic a conflict as an
			// awaiting one (409, never an unavailable 503).
			if held, serr := s.platform.ReadSnapshot(ctx, domain.SnapshotQuery{
				Kind: domain.SnapshotKindPurchase, TenantID: tenantID,
			}); serr == nil && held.Purchase != nil &&
				held.Purchase.State != domain.PurchaseStateAbsent &&
				held.Purchase.PlanCode != pub.PlanCode {
				return PurchaseView{}, ErrPurchasePlanConflict
			}
		}
		// R1-V04: a submit failure leaves nothing created — the POST answers
		// the unavailable sentinel (503 + closed reason), never a 201 view.
		return PurchaseView{}, fmt.Errorf("%w: %w", ErrPurchaseUnavailable, err)
	}

	// 7. The match gate (AC2): the authoritative purchase face must equal
	// the Quote in plan code, currency and integer amount — ANY mismatch
	// aborts BEFORE any channel payment request exists.
	psnap, err := s.platform.ReadSnapshot(ctx, domain.SnapshotQuery{
		Kind: domain.SnapshotKindPurchase, TenantID: tenantID,
	})
	if err != nil {
		// R1-V04: same posture as the submit failure — nothing was created.
		return PurchaseView{}, fmt.Errorf("%w: %w", ErrPurchaseUnavailable, err)
	}
	p := psnap.Purchase
	if p == nil || p.State == domain.PurchaseStateAbsent ||
		p.PlanCode != pub.PlanCode ||
		p.Currency != domain.CurrencyCNY ||
		p.Currency != snap.Currency ||
		p.AmountFen != snap.PriceFen {
		purchaseAudit(tenantID, quoteID, "invoice_quote_mismatch")
		return PurchaseView{}, ErrInvoiceQuoteMismatch
	}

	// 8. The order replay-or-create (AC4): an existing order for this quote
	// returns verbatim; otherwise ONE atomic CreateOrder; a concurrent
	// consumption race re-reads the winner. (R1-V03) A NEW channel order is
	// opened only while the held purchase is still awaiting payment — an
	// active plan must not be charged twice and a canceled subscription's
	// gating invoice can never be settled; the replay paths stay open so a
	// POST replay after payment still answers the paid order verbatim.
	if existing, err := s.orders.orders.GetOrderByQuote(ctx, tenantID, quoteID); err == nil {
		ov := orderViewFromRow(existing)
		return s.purchaseView(p, snap, pub, &ov), nil
	} else if !errors.Is(err, repocommercial.ErrOrderNotFound) {
		return PurchaseView{}, err
	}
	if p.State != domain.PurchaseStateAwaitingPayment {
		purchaseAudit(tenantID, quoteID, "purchase_not_awaiting")
		return PurchaseView{}, fmt.Errorf("%w: %s", ErrPurchaseNotAwaiting, p.State)
	}
	// (R1-22) ONE payable channel order per purchase: the idempotency key so
	// far was only the quote, so a FRESH quote (re-quote after expiry, a
	// second tab) reached CreateOrder while the purchase's previous pending
	// order was still payable — two concurrent channel orders on the same
	// gating invoice, each callback confirming independently, is the
	// awaiting-period shape of the "must not be charged twice" contract
	// below. Resolve the purchase's existing PENDING order first and answer
	// it verbatim (the match gate above already proved the quote buys the
	// same plan at the same frozen face, so the replayed order IS this
	// purchase's order). (R2-28) Only a PAYABLE pending order is replayed:
	// a channel-failed or link-less pending order is not a payment entry
	// and does not block this checkout.
	// (A-21 / F90) The price face alone does NOT prove ownership: the match
	// gate proved THIS quote buys plan P — not that the pending order's OWN
	// quote bought P. A legacy same-price order on a different plan (the
	// old POST /commercial/orders face during a republish window) must not
	// be replayed as this purchase's payment entry — its CheckoutURL would
	// settle the OLD plan's quote. The replay requires the existing
	// order's frozen quote to have bought the SAME plan as this purchase
	// (quoteBoughtPlan failing closed to "" never matches a non-empty
	// snap.PlanKey).
	if existing, perr := s.orders.orders.CurrentPendingPurchaseOrder(ctx, tenantID, p.AmountFen, p.Currency); perr == nil {
		if quoteBoughtPlan(ctx, s.orders, tenantID, existing.QuoteID) != snap.PlanKey {
			return PurchaseView{}, ErrPurchasePlanConflict
		}
		// (#83 channel switch) A pending order on a DIFFERENT provider than
		// the requested one is retired first — race-safely: the old channel
		// entry is closed (an in-flight payment decides through the query
		// and lands as the PAID old order, never a second channel request),
		// and only a cleanly released slot falls through to open the new
		// channel order below. An unresolved close outcome surfaces its
		// error: the caller retries — a second channel order must NEVER be
		// opened while the old one's outcome is unknown.
		//
		// (OCR C-10) The attempt read is CLASSIFIED, never blanket-else'd:
		// no attempt row is the frozen #82 replay shape (the atomic open
		// writes both rows, so a pending order with no attempt cannot prove
		// any channel) — but any OTHER read error (connection fault,
		// transient DB failure) propagates: converting a failed query into
		// a success answer would both mask an infrastructure failure and
		// leave an explicit channel-switch request answered with the OLD
		// channel's entry.
		att, aerr := s.orders.orders.FirstPendingAttempt(ctx, existing.ID)
		switch {
		case errors.Is(aerr, repocommercial.ErrPaymentAttemptNotFound):
			// No attempt row: keep the frozen #82 replay semantics — answer
			// the existing entry, never a second channel request.
			ov := orderViewFromRow(existing)
			return s.purchaseView(p, snap, pub, &ov), nil
		case aerr != nil:
			return PurchaseView{}, aerr
		case att.Provider != providerName:
			closeView, cerr := s.orders.CloseChannelOrder(ctx, tenantID, existing.ID)
			if cerr != nil {
				return PurchaseView{}, cerr
			}
			// (OCR C-09) EXCLUSIVE-DECISION check: anything that already
			// left pending is decided for this order (paid — or even
			// fulfilled, which the fulfill worker can reach between the
			// close's ConfirmPayment and this read). Answering the
			// closeView for every decided outcome means a fulfilled order
			// can never fall through to CreateOrder and mint a SECOND
			// channel order for an already-effective purchase.
			if closeView.State != domain.OrderStatePending {
				return s.purchaseView(p, snap, pub, &closeView), nil
			}
			// Closed cleanly (still pending, slot retired): fall through to
			// CreateOrder(quoteID, providerName) on the new channel.
		default:
			// Same-channel replay: #82 frozen semantics.
			ov := orderViewFromRow(existing)
			return s.purchaseView(p, snap, pub, &ov), nil
		}
	} else if !errors.Is(perr, repocommercial.ErrOrderNotFound) {
		return PurchaseView{}, perr
	}
	// (D11 / r2:253 high) The paid-awaiting window: the payment SUCCEEDED
	// (the order is locally paid) while the authority has not been observed
	// active yet — normally the drain+settle+webhook chain's tens of
	// seconds, at most the 10-minute budget. The pending probe above just
	// missed (the order left state=pending) AND the pending uniqueness
	// index no longer guards this tenant (its predicate is state='pending'
	// — a paid order is out of range), so WITHOUT this probe a fresh quote
	// would open a SECOND channel order for the SAME gating subscription:
	// its callback confirms independently and its settle idempotency key
	// binds to its OWN channel transaction — a real double charge. Replay
	// the paid order instead, synthesized to paid_awaiting_activation
	// exactly like PurchaseStatus projects it. The SAME plan-ownership
	// proof as the pending replay (A-21): a same-face paid order whose
	// frozen quote bought a DIFFERENT plan is a conflict, never a replayed
	// payment entry.
	if paidRow, paidErr := s.orders.orders.CurrentPaidAwaitingActivationPurchaseOrder(ctx, tenantID, p.AmountFen, p.Currency); paidErr == nil {
		if quoteBoughtPlan(ctx, s.orders, tenantID, paidRow.QuoteID) != snap.PlanKey {
			return PurchaseView{}, ErrPurchasePlanConflict
		}
		paidView := orderViewFromRow(paidRow)
		out := s.purchaseView(p, snap, pub, &paidView)
		if p.State == domain.PurchaseStateAwaitingPayment {
			out.State = domain.PurchaseStatePaidAwaitingActivation
		}
		return out, nil
	} else if !errors.Is(paidErr, repocommercial.ErrOrderNotFound) {
		return PurchaseView{}, paidErr
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
	if errors.Is(err, repocommercial.ErrPurchasePendingExists) {
		// (R2-26) The database-level pending invariant rejected this insert:
		// a concurrent checkout of the SAME purchase (fresh-quote race,
		// re-quote speed-run) committed its order first. Answer the winner's
		// order verbatim — never a second channel request. The loosened
		// conflict read needs no price-face match: the index already proved
		// a payable pending order for this tenant exists.
		// (A-21 / F105) No price-face match, but the SAME plan-ownership
		// proof as the replay branch above: the winner's frozen quote must
		// have bought THIS purchase's plan, else the conflict is a legacy
		// same-price foreign-plan order holding the slot — surfacing the
		// plan conflict (the caller re-quotes), never replaying a payment
		// entry that settles another plan's quote.
		existing, gerr := s.orders.orders.CurrentPayablePendingOrder(ctx, tenantID)
		if gerr == nil {
			if quoteBoughtPlan(ctx, s.orders, tenantID, existing.QuoteID) != snap.PlanKey {
				return PurchaseView{}, ErrPurchasePlanConflict
			}
			ovExisting := orderViewFromRow(existing)
			return s.purchaseView(p, snap, pub, &ovExisting), nil
		}
		if !errors.Is(gerr, repocommercial.ErrOrderNotFound) {
			return PurchaseView{}, err
		}
		// (R3-26) The slot is held by a link-less residue (channel call
		// succeeded but the link never persisted — the only shape the
		// migration backfill does not already cover): it is not a replayable
		// payment entry, so sweep it to channel-failed and retry the create
		// ONCE. A second conflict is a genuine race beyond the sweep —
		// surface the conflict error, never loop. (r2:306) The sweep's own
		// failure is LOGGED, not silently discarded — the caller still sees
		// the original conflict sentinel either way, but the sweep failure
		// stays diagnosable.
		if _, serr := s.orders.orders.SweepStaleLinklessPending(ctx, tenantID); serr != nil {
			logger.Warnf(ctx, "[CommercialPurchase] link-less sweep failed for tenant %d: %v", tenantID, serr)
			return PurchaseView{}, err
		}
		ov2, rerr := s.orders.CreateOrder(ctx, tenantID, quoteID, providerName)
		if rerr == nil {
			return s.purchaseView(p, snap, pub, &ov2), nil
		}
		if errors.Is(rerr, repocommercial.ErrQuoteAlreadyUsed) {
			// The retry's quote consumption lost its own race: answer the
			// winner's order, never a second channel request.
			if existing, gerr := s.orders.orders.GetOrderByQuote(ctx, tenantID, quoteID); gerr == nil {
				ovExisting := orderViewFromRow(existing)
				return s.purchaseView(p, snap, pub, &ovExisting), nil
			}
			return PurchaseView{}, repocommercial.ErrQuoteAlreadyUsed
		}
		if errors.Is(rerr, repocommercial.ErrPurchasePendingExists) {
			// (D12 / r2:306) The retry hit the pending invariant again — the
			// classic shape is the winner still inside its channel-Create
			// window (link-less, too fresh for the sweep). Re-read the
			// conflict entry once more: the winner has usually persisted its
			// checkout link by now and replays cleanly. Still unreadable →
			// the conflict SENTINEL itself (the handler maps it to a
			// retryable 409) — never a bare escape into an unmapped 500,
			// never a second channel order.
			if existing, gerr := s.orders.orders.CurrentPayablePendingOrder(ctx, tenantID); gerr == nil {
				if quoteBoughtPlan(ctx, s.orders, tenantID, existing.QuoteID) != snap.PlanKey {
					return PurchaseView{}, ErrPurchasePlanConflict
				}
				ovExisting := orderViewFromRow(existing)
				return s.purchaseView(p, snap, pub, &ovExisting), nil
			}
			return PurchaseView{}, rerr
		}
		return PurchaseView{}, rerr
	}
	if err != nil {
		return PurchaseView{}, err
	}
	return s.purchaseView(p, snap, pub, &ov), nil
}

// purchaseAudit is the attention-audit sink (OCR final audit 10, minimal
// face): a purchase attempt that leaves the authority holding an
// awaiting/terminal subscription it can no longer settle writes ONE
// closed-token attention line (log by default; tests capture). The
// lifecycle face (auto-cancel/terminate) belongs to #84/#92 and is
// explicitly NOT taken here.
var purchaseAudit = func(tenantID uint64, quoteID, token string) {
	logger.Warnf(context.Background(),
		"[CommercialPurchase] attention: tenant=%d quote=%s token=%s", tenantID, quoteID, token)
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
	// (A-28 / F104) ONE publication read serves both the plan projection and
	// the order-ownership proof below — the second FindPublicationByCode this
	// path used to make (per poll!) is gone, and pubErr is SHARED: a failed
	// read means the plan face cannot be PROVEN, in which case the order
	// projection is skipped entirely (both sides unprovable ⇒ no projection
	// — the "" == "" vacuous match that would otherwise project a foreign
	// historical order, even synthesizing paid_awaiting_activation for it,
	// is structurally impossible now).
	pub, pubErr := s.plans.FindPublicationByCode(ctx, p.PlanCode)
	if pubErr == nil {
		out.PlanKey = pub.PlanKey
		out.PlanVersion = pub.Version
	} else {
		// (A-28) A failed publication read degrades this projection SILENTLY
		// no longer: the plan face and the order ownership are both
		// unprovable, the order projection is skipped — leave one Warn so a
		// permanently missing publication row (migration gap, DB outage) is
		// diagnosable from the log instead of surfacing as "state without
		// plan_key".
		logger.Warnf(ctx, "[CommercialPurchase] publication read failed for plan code %q: %v", p.PlanCode, pubErr)
	}
	// (R1-V02) Only an order that BELONGS to the current purchase is
	// projected: purchase-kind at the held purchase's frozen price face.
	// ListOrdersByTenant's `id DESC` is a random lexicographic order (order
	// ids are "ord_"+random hex) and a tenant may hold upgrade orders and
	// historical purchases — taking its rows[0] projected an arbitrary old
	// order as the current purchase state. An absent purchase attaches no
	// order at all.
	if p.State != domain.PurchaseStateAbsent && pubErr == nil {
		// (OCR final audit 9) Only an order whose frozen quote bought THIS
		// purchase's plan is projectable: a historical same-price order on
		// a different plan (pro-max at the pro price face during a
		// republish window) must never shadow the current purchase.
		if row, err := s.orders.orders.CurrentPurchaseOrder(ctx, tenantID, p.AmountFen, p.Currency); err == nil &&
			quoteBoughtPlan(ctx, s.orders, tenantID, row.QuoteID) == pub.PlanKey {
			ov := orderViewFromRow(row)
			// (#84 Task 5 / G4) the purchase's order projection carries the
			// SAME attention flag as the order read paths (BillingPage's
			// anomaly suffix reads it); the purchase state axis itself stays
			// the closed five-state set — attention rides on the order
			// sub-object only.
			if ok, aerr := s.orders.orders.HasUnresolvedPaymentAnomaly(ctx, row.ID); aerr == nil && ok {
				ov.PaymentAttention = true
			}
			out.Order = &ov
			// (#82 D3) paid_awaiting_activation is a COORDINATOR-COMPOSED
			// product state: a locally PAID order while the authority has
			// not yet been observed active. It never comes from an authority
			// read — only this synthesis produces it.
			if p.State == domain.PurchaseStateAwaitingPayment &&
				row.State == domain.OrderStatePaid {
				out.State = domain.PurchaseStatePaidAwaitingActivation
			}
		}
	}
	return out, nil
}

// quoteBoughtPlan reads the plan key a quote's frozen snapshot carries (""
// when unreadable — an unmatchable value, so the caller never projects the
// order). The read is tenant-guarded like every quote access.
func quoteBoughtPlan(ctx context.Context, orders *OrderService, tenantID uint64, quoteID string) string {
	_, snap, err := orders.QuoteSnapshotForTenant(ctx, tenantID, quoteID)
	if err != nil {
		return ""
	}
	return snap.PlanKey
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
		// (R1-V18) A definition that does not deserialize is a data problem
		// (it was written by domain.PlanVersion at publish time) — it has
		// nothing to do with a quote predating the freeze, so it must not
		// tell the caller to re-quote (the same corruption would fail every
		// fresh quote identically).
		return ErrPurchasePlanInvalid
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
// fabricated state (the #78 outage posture). GET PurchaseStatus keeps this
// state view; the POST surface answers ErrPurchaseUnavailable instead.
func (s *PurchaseService) pendingView(err error) PurchaseView {
	return PurchaseView{State: domain.PurchaseStateAbsent, Reason: platformReason(err)}
}

// platformSentinel re-opens a closed reason token onto its domain sentinel —
// the exact inverse of platformReason — so a failure that crossed the
// billing-account boundary keeps its token through the purchase-unavailable
// error chain. Unknown tokens map to ErrPlatformUnsupported (the closed
// "unsupported" posture).
func platformSentinel(reason string) error {
	switch reason {
	case "unconfigured":
		return domain.ErrPlatformUnconfigured
	case "unreachable":
		return domain.ErrPlatformUnreachable
	case "invalid_response":
		return domain.ErrPlatformInvalidResponse
	default:
		return domain.ErrPlatformUnsupported
	}
}

// PurchaseUnavailableReason extracts the closed reason token
// (unconfigured|unreachable|invalid_response|unsupported) from an
// ErrPurchaseUnavailable chain — the only failure vocabulary the wire may
// carry.
func PurchaseUnavailableReason(err error) string {
	return platformReason(err)
}

// orderViewFromRow projects a stored order row (the OrderService keeps this
// mapping private to its list paths; the purchase replay needs it too).
// CheckoutURL rides along (R1-35): the replayed POST answers the persisted
// channel link verbatim instead of dropping the customer's payment entry.
func orderViewFromRow(r repocommercial.OrderRow) OrderView {
	return OrderView{ID: r.ID, QuoteID: r.QuoteID, State: r.State,
		AmountFen: r.AmountFen, Currency: r.Currency, CheckoutURL: r.CheckoutURL,
		Version: r.Version}
}
