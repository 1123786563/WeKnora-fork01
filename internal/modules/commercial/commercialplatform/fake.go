// Package commercialplatform implements the adapters behind the frozen
// CommercialPlatform seam (internal/commercial/platform.go): a deterministic
// fake for tests and dev, and a Lago adapter that reads the authority's
// health signal with server-side env config and fails closed. Both adapters
// run the same shared contract suite (contract_test.go); a behavior only one
// adapter has is a defect. W3 (#78) enables ensure_customer — idempotent by
// deterministic identity — and the account snapshot kind; T07 (#79) enables
// publish_plan_version on BOTH adapters; every other command kind and the
// reconcile family stay frozen and fail closed with
// commercial.ErrPlatformUnsupported.
package commercialplatform

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
)

// FakeCustomer is one stored authority-side customer in the fake: the
// deterministic identity plus the ADVISORY display name at last ensure.
type FakeCustomer struct {
	ExternalID string
	Name       string
}

// fakeSubscription is one stored authority-side subscription (T08): the
// deterministic identity, the customer it belongs to and the plan code
// attached — upserted by identity, never a second entry per tenant.
// Terminated is the #102 closure disposal: a terminated subscription stays
// (financial history) but never reports active again.
type fakeSubscription struct {
	ExternalID       string
	ExternalCustomer string
	PlanCode         string
	Terminated       bool
}

// fakeWallet is one stored authority-side wallet (T08): the deterministic
// name, the customer it belongs to, the granted amount and the short TTL.
type fakeWallet struct {
	Name         string
	Customer     string
	GrantedCents int64
	ExpiresAt    time.Time
	Terminated   bool
	CreatedAt    time.Time
	Priority     int
}

// FakeWallet is the observable wallet state for tests: the VISIBLE
// (settled) balance — the a1 settle lag is modeled by SetWalletSettleLag.
type FakeWallet struct {
	Name         string
	Customer     string
	GrantedCents int64
	BalanceCents int64
	ExpiresAt    time.Time
	Terminated   bool
	Priority     int
}

// FakeSubscription is the observable subscription state for tests.
type FakeSubscription struct {
	ExternalID       string
	ExternalCustomer string
	PlanCode         string
}

// fakePurchase is one stored authority-side PAYMENT-GATED purchase
// subscription (#81): created incomplete by create_purchase_subscription,
// advanced to active only by the (test) activation hook.
type fakePurchase struct {
	ExternalID           string
	ExternalCustomer     string
	PlanCode             string
	AmountFen            int64
	Currency             string
	Status               string // "incomplete"|"active"|"canceled"
	InvoiceFees          []commercial.InvoiceLineSnapshot
	InvoicePaymentStatus string // finalized-stage payment_status (D6' review input)
}

// FakePurchaseSubscription is the observable purchase-subscription state for
// tests (#81): a fresh create is always incomplete.
type FakePurchaseSubscription struct {
	ExternalID       string
	ExternalCustomer string
	PlanCode         string
	Status           string // "incomplete"|"active"|"canceled" —— fake 建即 incomplete
}

// FakeAdapter is the deterministic in-process CommercialPlatform used by
// tests and dev environments. SetReadiness stores one readiness snapshot;
// ReadSnapshot copies it back verbatim. The W3 customer surface is a real
// in-memory authority: customers are stored keyed by external id (upsert —
// never a second entry), ensure_customer submits are idempotent per
// Command.Key, and the account snapshot derives from the store honestly —
// an absent customer is absent, never a fabricated linked. SubmitCommand
// also implements the publish_plan_version kind with coordinator-owned
// idempotency: the same Key with byte-equal payload replays the same
// receipt, the same Key with different content is a definitive conflict
// (spec story 59). The store mutates now, so every access is
// mutex-guarded — concurrent publish tests are legal.
type FakeAdapter struct {
	mu          sync.Mutex
	primed      bool
	readiness   commercial.ReadinessSnapshot
	customers   map[string]FakeCustomer
	receipts    map[string]commercial.CommandReceipt
	failSubmits error
	// failReadSnapshots (A-31/A-32 test hook): when set, purchase-kind
	// snapshot reads answer this error — the transient/definitive error
	// classification the fulfiller tests drive.
	failReadSnapshots error
	commands          map[string]fakeCommand
	creates           []commercial.Command
	// T08 state (#80): a real in-memory subscription + wallet authority.
	subs    map[string]fakeSubscription
	wallets []fakeWallet
	// T09 state (#81): a real in-memory payment-gated purchase authority —
	// purchase subscriptions keyed by external id, and the provider binding
	// per external customer (external customer id → provider customer id).
	purchaseSubs     map[string]fakePurchase
	providerBindings map[string]string
	// #96 state: the credit-note ledger — append-only, keyed by the seam
	// command identity (one note per refund).
	creditNotes []fakeCreditNote
	// now is the injectable clock (expiry/settle determinism); nil = real
	// time. baseFeatures primes the benefits feature map. walletSettleLag
	// models the a1 after-commit settlement lag (default 0 — the fake's
	// authority is idealized; tests advance the clock to expose the lag).
	// rejectWalletCreates models a wallet_limit_reached window.
	now                 func() time.Time
	baseFeatures        map[string]bool
	walletSettleLag     time.Duration
	rejectWalletCreates error
}

// fakeCommand is one recorded publish command: its exact payload and the
// receipt issued for it.
type fakeCommand struct {
	payload commercial.PublishPlanVersionPayload
	receipt commercial.CommandReceipt
}

// fakeCreditNote is one recorded credit-note correction (#96): the exact
// payload it was issued under and the receipt answered for it.
type fakeCreditNote struct {
	Key     string
	Payload commercial.IssueCreditNotePayload
	Receipt commercial.CommandReceipt
}

// NewFakeAdapter builds the fake with no readiness primed: reading readiness
// before SetReadiness fails closed with ErrPlatformUnconfigured — the fake
// never fabricates platform state either.
func NewFakeAdapter() *FakeAdapter {
	return &FakeAdapter{
		customers:        map[string]FakeCustomer{},
		receipts:         map[string]commercial.CommandReceipt{},
		commands:         map[string]fakeCommand{},
		subs:             map[string]fakeSubscription{},
		purchaseSubs:     map[string]fakePurchase{},
		providerBindings: map[string]string{},
	}
}

// nowUTC returns the injected clock (or real time), truncated to UTC.
// Caller must hold f.mu.
func (f *FakeAdapter) nowUTC() time.Time {
	if f.now != nil {
		return f.now().UTC()
	}
	return time.Now().UTC()
}

// SetNow injects the clock (deterministic expiry and settle-lag tests).
func (f *FakeAdapter) SetNow(fn func() time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = fn
}

// SetBasePlanFeatures primes the entitlement feature map the benefits
// snapshot passes through (the runtime reads entitlements; the fake is
// primed by the test).
func (f *FakeAdapter) SetBasePlanFeatures(features map[string]bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.baseFeatures = features
}

// SetWalletSettleLag models the a1 after-commit settlement lag: a wallet's
// granted balance becomes visible only at observations at or after
// createdAt+lag. Default 0.
func (f *FakeAdapter) SetWalletSettleLag(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.walletSettleLag = d
}

// RejectWalletCreatesWith models a wallet_limit_reached window: while set,
// the grant for an ABSENT wallet answers the injected error WITHOUT
// creating anything (a refused create — nothing persisted); a replay for an
// EXISTING wallet still answers the receipt. nil clears the knob.
func (f *FakeAdapter) RejectWalletCreatesWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rejectWalletCreates = err
}

// SeedTopUpWallet seeds a TOP-UP shaped wallet directly into the authority
// store (#86): a non-grant-family name, a mid-month expiry the grant
// command's own validation can never express (grants expire at period ends
// only). This is the #85 payment-confirmed batch shape tests model — the
// name must NOT parse as a monthly/purchase deterministic name.
func (f *FakeAdapter) SeedTopUpWallet(name, customer string, grantedCents int64, expiresAt, grantedAt time.Time, priority int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.wallets = append(f.wallets, fakeWallet{
		Name: name, Customer: customer,
		GrantedCents: grantedCents, ExpiresAt: expiresAt,
		CreatedAt: grantedAt, Priority: priority,
	})
}

// TerminateWallet flips one stored wallet to terminated (the observation
// knob for the authority's termination tick): a terminated wallet leaves
// the benefits snapshot's batch list — the post-lazy-termination steady
// state cross-month tests model.
func (f *FakeAdapter) TerminateWallet(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.wallets {
		if f.wallets[i].Name == name {
			f.wallets[i].Terminated = true
			return
		}
	}
}

// Wallets returns the observable wallet state (VISIBLE balance — settled
// per the settle lag, terminated excluded from balance but listed).
func (f *FakeAdapter) Wallets() []FakeWallet {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.nowUTC()
	out := make([]FakeWallet, 0, len(f.wallets))
	for _, w := range f.wallets {
		visible := int64(0)
		if !w.Terminated && !now.Before(w.CreatedAt.Add(f.walletSettleLag)) {
			visible = w.GrantedCents
		}
		out = append(out, FakeWallet{
			Name: w.Name, Customer: w.Customer, GrantedCents: w.GrantedCents,
			BalanceCents: visible, ExpiresAt: w.ExpiresAt, Terminated: w.Terminated,
			Priority: w.Priority,
		})
	}
	return out
}

// Subscriptions returns the stored authority-side subscriptions sorted by
// external id (the identity assertion surface: count and identity must be
// exactly one per tenant).
func (f *FakeAdapter) Subscriptions() []FakeSubscription {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]FakeSubscription, 0, len(f.subs))
	for _, s := range f.subs {
		out = append(out, FakeSubscription{
			ExternalID: s.ExternalID, ExternalCustomer: s.ExternalCustomer, PlanCode: s.PlanCode,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExternalID < out[j].ExternalID })
	return out
}

// PurchaseSubscriptions returns the stored payment-gated purchase
// subscriptions sorted by external id (#81) — the identity assertion
// surface: count must be exactly one per tenant.
func (f *FakeAdapter) PurchaseSubscriptions() []FakePurchaseSubscription {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]FakePurchaseSubscription, 0, len(f.purchaseSubs))
	for _, s := range f.purchaseSubs {
		out = append(out, FakePurchaseSubscription{
			ExternalID: s.ExternalID, ExternalCustomer: s.ExternalCustomer,
			PlanCode: s.PlanCode, Status: s.Status,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExternalID < out[j].ExternalID })
	return out
}

// ProviderBindings returns the stored provider bindings (external customer
// id → provider customer id) — the #81 binding assertion surface.
func (f *FakeAdapter) ProviderBindings() map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]string, len(f.providerBindings))
	for k, v := range f.providerBindings {
		out[k] = v
	}
	return out
}

// ActivatePurchase advances the purchase subscription to active (#82 前的
// 手动推进钩子)：真实环境由 provider 收款驱动（t02 F9）。
func (f *FakeAdapter) ActivatePurchase(extPurchaseSubscriptionID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.purchaseSubs[extPurchaseSubscriptionID]; ok {
		s.Status = "active"
		f.purchaseSubs[extPurchaseSubscriptionID] = s
	}
}

// SetPurchaseInvoiceFees injects the invoice line fees the purchase snapshot
// answers (#81 D2 condition 3): the finalized-stage interface #82/#84
// consume — the open stage always answers EMPTY regardless of this knob
// being unset (nothing is fabricated).
func (f *FakeAdapter) SetPurchaseInvoiceFees(extPurchaseSubscriptionID string, fees []commercial.InvoiceLineSnapshot) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.purchaseSubs[extPurchaseSubscriptionID]; ok {
		s.InvoiceFees = append([]commercial.InvoiceLineSnapshot(nil), fees...)
		if s.InvoicePaymentStatus == "" {
			// A fee injection models the finalized stage; the D6' review
			// reads the invoice payment_status alongside the lines.
			s.InvoicePaymentStatus = "succeeded"
		}
		f.purchaseSubs[extPurchaseSubscriptionID] = s
	}
}

// CancelPurchase advances the purchase subscription to canceled (the test
// observation knob for the timeout/cancel boundary).
func (f *FakeAdapter) CancelPurchase(extPurchaseSubscriptionID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.purchaseSubs[extPurchaseSubscriptionID]; ok {
		s.Status = "canceled"
		f.purchaseSubs[extPurchaseSubscriptionID] = s
	}
}

// SetReadiness primes the readiness snapshot returned by ReadSnapshot.
func (f *FakeAdapter) SetReadiness(s commercial.ReadinessSnapshot) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.readiness = s
	f.primed = true
}

// FailSubmitsWith installs a fault-injection knob for the recovery tests:
// while set, every ensure_customer submit STILL applies the authority state
// (the customer is stored — the #73 persisted-but-response-lost case) and
// then answers with the injected sentinel instead of a receipt. nil clears
// the knob.
func (f *FakeAdapter) FailSubmitsWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failSubmits = err
}

// FailPurchaseSnapshotsWith makes every PURCHASE-kind snapshot read answer
// the injected error (nil clears it) — the fulfiller's error-classification
// test seam (unreachable vs invalid response).
func (f *FakeAdapter) FailPurchaseSnapshotsWith(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failReadSnapshots = err
}

// Customers returns the stored authority-side customers sorted by external
// id — the identity assertion surface for concurrency tests (count and
// identity must be exactly one per tenant).
func (f *FakeAdapter) Customers() []FakeCustomer {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]FakeCustomer, 0, len(f.customers))
	for _, c := range f.customers {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExternalID < out[j].ExternalID })
	return out
}

// Commands returns the publish commands that CREATED state, in order — the
// test observation point proving a replay creates nothing (each create
// appends exactly once).
func (f *FakeAdapter) Commands() []commercial.Command {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]commercial.Command(nil), f.creates...)
}

// ReadSnapshot answers the primed readiness snapshot verbatim and the
// account truth from the customer store; unknown kinds fail closed
// unsupported.
func (f *FakeAdapter) ReadSnapshot(_ context.Context, query commercial.SnapshotQuery) (commercial.Snapshot, error) {
	if query.Kind == commercial.SnapshotKindPurchase {
		f.mu.Lock()
		injected := f.failReadSnapshots
		f.mu.Unlock()
		if injected != nil {
			return commercial.Snapshot{}, injected
		}
	}
	switch query.Kind {
	case commercial.SnapshotKindReadiness:
		f.mu.Lock()
		defer f.mu.Unlock()
		if !f.primed {
			return commercial.Snapshot{}, commercial.ErrPlatformUnconfigured
		}
		snapshot := f.readiness // copy back verbatim
		return commercial.Snapshot{
			Kind:      commercial.SnapshotKindReadiness,
			Readiness: &snapshot,
		}, nil
	case commercial.SnapshotKindAccount:
		if query.TenantID == 0 {
			return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
		}
		f.mu.Lock()
		_, present := f.customers[commercial.ExternalCustomerID(query.TenantID)]
		f.mu.Unlock()
		state := commercial.AccountStateAbsent
		if present {
			state = commercial.AccountStateLinked
		}
		return commercial.Snapshot{
			Kind: commercial.SnapshotKindAccount,
			Account: &commercial.AccountSnapshot{
				TenantID:  query.TenantID,
				State:     state,
				CheckedAt: time.Now().UTC(),
			},
		}, nil
	case commercial.SnapshotKindBenefits:
		if query.TenantID == 0 {
			return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		extCustomer := commercial.ExternalCustomerID(query.TenantID)
		extSub := commercial.ExternalSubscriptionID(query.TenantID)
		b := &commercial.BenefitsSnapshot{
			TenantID:          query.TenantID,
			SubscriptionState: commercial.SubscriptionStatePending,
			CheckedAt:         f.nowUTC(),
		}
		if sub, held := f.subs[extSub]; held {
			if sub.Terminated {
				b.SubscriptionState = commercial.SubscriptionStateTerminated
			} else {
				b.SubscriptionState = commercial.SubscriptionStateActive
			}
			b.PlanCode = sub.PlanCode
		}
		if f.baseFeatures != nil {
			features := make(map[string]bool, len(f.baseFeatures))
			for k, v := range f.baseFeatures {
				features[k] = v
			}
			b.Features = features
		}
		// Raw authority truth: TERMINATED wallets are excluded; expired but
		// not-yet-terminated ones are INCLUDED (the coordinator overlays
		// registry expiry). Batch families (#86 Task 1): a deterministic-name
		// wallet is the monthly family; any OTHER wallet of this customer is
		// a top-up batch (the #85 payment-confirmed shape — the fake models
		// it by name, exactly how tests seed it).
		now := f.nowUTC()
		for _, w := range f.wallets {
			if w.Customer != extCustomer || w.Terminated {
				continue
			}
			visible := int64(0)
			if !now.Before(w.CreatedAt.Add(f.walletSettleLag)) {
				visible = w.GrantedCents
			}
			b.BalanceMicro += commercial.CentsToMicro(visible)
			if period, ok := fakeWalletPeriod(extCustomer, w.Name); ok {
				b.Batches = append(b.Batches, commercial.CreditBatchSnapshot{
					Period:       period,
					BalanceMicro: commercial.CentsToMicro(visible),
					ExpiresAt:    w.ExpiresAt,
					Source:       commercial.BatchSourceMonthly,
					GrantedAt:    w.CreatedAt,
					WalletRef:    w.Name,
				})
			} else {
				b.Batches = append(b.Batches, commercial.CreditBatchSnapshot{
					BalanceMicro: commercial.CentsToMicro(visible),
					ExpiresAt:    w.ExpiresAt,
					Source:       commercial.BatchSourceTopUp,
					GrantedAt:    w.CreatedAt,
					WalletRef:    w.Name,
				})
			}
		}
		return commercial.Snapshot{Kind: commercial.SnapshotKindBenefits, Benefits: b}, nil
	case commercial.SnapshotKindPurchase:
		if query.TenantID == 0 {
			return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		p := &commercial.PurchaseSnapshot{
			TenantID:  query.TenantID,
			State:     commercial.PurchaseStateAbsent,
			CheckedAt: f.nowUTC(),
		}
		if sub, held := f.purchaseSubs[commercial.ExternalPurchaseSubscriptionID(query.TenantID)]; held {
			switch sub.Status {
			case "incomplete":
				p.State = commercial.PurchaseStateAwaitingPayment
			case "active":
				p.State = commercial.PurchaseStateActive
			case "canceled":
				p.State = commercial.PurchaseStateCanceled
			default:
				return commercial.Snapshot{}, fmt.Errorf("%w: unknown purchase status", commercial.ErrPlatformInvalidResponse)
			}
			p.PlanCode = sub.PlanCode
			p.AmountFen = sub.AmountFen
			p.Currency = sub.Currency
			// Open-stage lines are never fabricated (F3-F5); the injected
			// fees model the finalized stage only.
			if sub.Status == "active" && len(sub.InvoiceFees) > 0 {
				p.InvoiceFees = append([]commercial.InvoiceLineSnapshot(nil), sub.InvoiceFees...)
				p.InvoicePaymentStatus = sub.InvoicePaymentStatus
			}
		}
		return commercial.Snapshot{Kind: commercial.SnapshotKindPurchase, Purchase: p}, nil
	default:
		return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
	}
}

// fakeWalletPeriod extracts the calendar period from a deterministic wallet
// name — BOTH grant families (F-4): "<ext-customer>-<YYYY-MM>" (monthly)
// and "<ext-customer>-purchase-<YYYY-MM>" (the #82 D4 purchase first-period
// batch). Non-WeKnora-named wallets — future top-ups — carry no period.
func fakeWalletPeriod(extCustomer, name string) (string, bool) {
	for _, prefix := range []string{extCustomer + "-purchase-", extCustomer + "-"} {
		suffix, ok := strings.CutPrefix(name, prefix)
		if !ok || len(suffix) != 7 {
			continue
		}
		if _, err := commercial.PeriodEnd(suffix); err != nil {
			continue
		}
		return suffix, true
	}
	return "", false
}

// SubmitCommand applies the enabled command families. ensure_customer (the
// W3 first enabled kind, #78) is idempotent per Command.Key — a replay
// returns the ORIGINAL receipt with unchanged RecordedAt — and upserting
// per external id, so a different Key addressing the same identity refreshes
// advisory metadata and never creates a second customer. publish_plan_version
// (T07, #79) validates the command and payload, records under the
// coordinator Key and answers a receipt whose ExternalID echoes the
// payload's deterministic plan code; a replay with equal payload returns the
// SAME receipt without a second record, the same Key with different payload
// is a conflict. Every other kind fails closed unsupported.
func (f *FakeAdapter) SubmitCommand(_ context.Context, cmd commercial.Command) (commercial.CommandReceipt, error) {
	if err := cmd.Validate(); err != nil {
		return commercial.CommandReceipt{}, err
	}
	switch cmd.Kind {
	case commercial.CommandKindEnsureCustomer:
		payload, ok := cmd.Payload.(commercial.EnsureCustomerPayload)
		if !ok || payload.TenantID == 0 || payload.ExternalCustomerID == "" ||
			payload.ExternalCustomerID != commercial.ExternalCustomerID(payload.TenantID) {
			return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
		}

		f.mu.Lock()
		defer f.mu.Unlock()
		apply := func() {
			if f.customers == nil {
				f.customers = map[string]FakeCustomer{}
			}
			f.customers[payload.ExternalCustomerID] = FakeCustomer{
				ExternalID: payload.ExternalCustomerID,
				Name:       payload.DisplayName,
			}
		}
		if f.failSubmits != nil {
			// Persisted-but-response-lost: the authority state applies, the
			// caller observes the injected failure.
			apply()
			return commercial.CommandReceipt{}, f.failSubmits
		}
		if receipt, ok := f.receipts[cmd.Key]; ok {
			// US-59 note: a replay under the same Key with a different display
			// name legitimately updates ADVISORY metadata (identity immutable) —
			// so the name applies while the ORIGINAL receipt (unchanged
			// RecordedAt) answers.
			apply()
			return receipt, nil
		}
		receipt := commercial.CommandReceipt{
			Key:        cmd.Key,
			ExternalID: payload.ExternalCustomerID,
			RecordedAt: time.Now().UTC(),
		}
		if f.receipts == nil {
			f.receipts = map[string]commercial.CommandReceipt{}
		}
		f.receipts[cmd.Key] = receipt
		apply()
		return receipt, nil

	case commercial.CommandKindPublishPlanVersion:
		payload, ok := cmd.Payload.(commercial.PublishPlanVersionPayload)
		if !ok {
			return commercial.CommandReceipt{}, fmt.Errorf("%w: publish payload has the wrong type", commercial.ErrPlatformInvalidResponse)
		}
		if err := payload.Validate(); err != nil {
			return commercial.CommandReceipt{}, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, err)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if existing, ok := f.commands[cmd.Key]; ok {
			if reflect.DeepEqual(existing.payload, payload) {
				return existing.receipt, nil // same receipt, no second record
			}
			return commercial.CommandReceipt{}, fmt.Errorf("%w: publish command conflict", commercial.ErrPlatformInvalidResponse)
		}
		receipt := commercial.CommandReceipt{
			Key:        cmd.Key,
			ExternalID: payload.PlanCode,
			RecordedAt: time.Now().UTC(),
		}
		f.commands[cmd.Key] = fakeCommand{payload: payload, receipt: receipt}
		f.creates = append(f.creates, cmd)
		return receipt, nil

	case commercial.CommandKindEnsureSubscription:
		payload, ok := cmd.Payload.(commercial.EnsureSubscriptionPayload)
		if !ok {
			return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
		}
		if err := payload.Validate(); err != nil {
			return commercial.CommandReceipt{}, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, err)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		apply := func() {
			if f.subs == nil {
				f.subs = map[string]fakeSubscription{}
			}
			f.subs[payload.ExternalSubscriptionID] = fakeSubscription{
				ExternalID:       payload.ExternalSubscriptionID,
				ExternalCustomer: payload.ExternalCustomerID,
				PlanCode:         payload.PlanCode,
			}
		}
		if existing, held := f.subs[payload.ExternalSubscriptionID]; held && existing.PlanCode != payload.PlanCode {
			// The no-parallel-subscription rule: a held subscription on a
			// different plan code is a definitive conflict.
			return commercial.CommandReceipt{}, fmt.Errorf("%w: subscription plan conflict", commercial.ErrPlatformInvalidResponse)
		}
		if f.failSubmits != nil {
			// Persisted-but-response-lost: the authority state applies, the
			// caller observes the injected failure.
			apply()
			return commercial.CommandReceipt{}, f.failSubmits
		}
		if receipt, ok := f.receipts[cmd.Key]; ok {
			apply() // advisory refresh only
			return receipt, nil
		}
		receipt := commercial.CommandReceipt{
			Key:        cmd.Key,
			ExternalID: payload.ExternalSubscriptionID,
			RecordedAt: f.nowUTC(),
		}
		f.receipts[cmd.Key] = receipt
		apply()
		return receipt, nil

	case commercial.CommandKindGrantIncludedCredits:
		payload, ok := cmd.Payload.(commercial.GrantIncludedCreditsPayload)
		if !ok {
			return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
		}
		if err := payload.Validate(); err != nil {
			return commercial.CommandReceipt{}, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, err)
		}
		// D4: an empty WalletName keeps the Base monthly batch identity; a
		// purchase first-period grant names its own wallet so the two grant
		// families never collide on the by-name idempotency match.
		walletName := payload.WalletName
		if walletName == "" {
			walletName = commercial.MonthlyWalletName(payload.TenantID, payload.Period)
		}
		wantCents := payload.CreditsMicro / 10_000
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, w := range f.wallets {
			if w.Customer != payload.ExternalCustomerID || w.Name != walletName {
				continue
			}
			// Read-before-create match: the same key with DIFFERENT content
			// is a conflict; an equal match is a replay (NO balance change —
			// the E3 anti-pattern must be impossible here).
			if w.GrantedCents != wantCents {
				return commercial.CommandReceipt{}, fmt.Errorf("%w: grant content conflict", commercial.ErrPlatformInvalidResponse)
			}
			if f.failSubmits != nil {
				return commercial.CommandReceipt{}, f.failSubmits
			}
			return commercial.CommandReceipt{
				Key:        cmd.Key,
				ExternalID: walletName,
				RecordedAt: f.nowUTC(),
			}, nil
		}
		if f.failSubmits != nil {
			// Persisted-but-response-lost: the wallet IS created; the next
			// grant resolves by name.
			f.wallets = append(f.wallets, fakeWallet{
				Name: walletName, Customer: payload.ExternalCustomerID,
				GrantedCents: wantCents, ExpiresAt: payload.ExpiresAt, CreatedAt: f.nowUTC(),
				Priority: payload.Priority,
			})
			return commercial.CommandReceipt{}, f.failSubmits
		}
		if f.rejectWalletCreates != nil {
			// A refused create (wallet_limit_reached): nothing persisted —
			// the caller replays by identity later.
			return commercial.CommandReceipt{}, f.rejectWalletCreates
		}
		f.wallets = append(f.wallets, fakeWallet{
			Name: walletName, Customer: payload.ExternalCustomerID,
			GrantedCents: wantCents, ExpiresAt: payload.ExpiresAt, CreatedAt: f.nowUTC(),
			Priority: payload.Priority,
		})
		return commercial.CommandReceipt{
			Key:        cmd.Key,
			ExternalID: walletName,
			RecordedAt: f.nowUTC(),
		}, nil

	case commercial.CommandKindCreatePurchaseSubscription:
		payload, ok := cmd.Payload.(commercial.CreatePurchaseSubscriptionPayload)
		if !ok {
			return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
		}
		if err := payload.Validate(); err != nil {
			return commercial.CommandReceipt{}, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, err)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		// ensureProviderBinding semantics: the create command guarantees the
		// customer carries a provider binding (D3) — recorded by the fake at
		// first create, never duplicated.
		if f.providerBindings == nil {
			f.providerBindings = map[string]string{}
		}
		if _, bound := f.providerBindings[payload.ExternalCustomerID]; !bound {
			f.providerBindings[payload.ExternalCustomerID] = "fake-provider-" + payload.ExternalCustomerID
		}
		if existing, held := f.purchaseSubs[payload.ExternalPurchaseSubscriptionID]; held {
			if existing.PlanCode != payload.PlanCode {
				// Concurrent plan change (AC4): a held purchase on a different
				// plan code is a definitive conflict — no second subscription,
				// the late caller re-quotes.
				return commercial.CommandReceipt{}, fmt.Errorf("%w: purchase plan conflict", commercial.ErrPlatformInvalidResponse)
			}
			// Identity replay: same purchase identity + same plan → the same
			// receipt, never a second subscription or a second gating invoice
			// (F7: never re-POST).
			return commercial.CommandReceipt{
				Key:        cmd.Key,
				ExternalID: payload.ExternalPurchaseSubscriptionID,
				RecordedAt: f.nowUTC(),
			}, nil
		}
		if f.purchaseSubs == nil {
			f.purchaseSubs = map[string]fakePurchase{}
		}
		f.purchaseSubs[payload.ExternalPurchaseSubscriptionID] = fakePurchase{
			ExternalID:       payload.ExternalPurchaseSubscriptionID,
			ExternalCustomer: payload.ExternalCustomerID,
			PlanCode:         payload.PlanCode,
			AmountFen:        payload.AmountFen,
			Currency:         payload.Currency,
			Status:           "incomplete",
		}
		return commercial.CommandReceipt{
			Key:        cmd.Key,
			ExternalID: payload.ExternalPurchaseSubscriptionID,
			RecordedAt: f.nowUTC(),
		}, nil

	case commercial.CommandKindSettlePurchasePayment:
		payload, ok := cmd.Payload.(commercial.SettlePurchasePaymentPayload)
		if !ok {
			return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
		}
		if err := payload.Validate(); err != nil {
			return commercial.CommandReceipt{}, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, err)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		sub, held := f.purchaseSubs[payload.ExternalPurchaseSubscriptionID]
		if !held {
			// D2'(c): settling an unknown purchase is a definitive integrity
			// violation — never a fabricated activation.
			return commercial.CommandReceipt{}, fmt.Errorf("%w: settle target purchase does not exist", commercial.ErrPlatformInvalidResponse)
		}
		if sub.Status == "canceled" || sub.Status == "terminated" {
			// A terminal purchase can never be settled (the authority's own
			// truth refuses; the Lago rails would fail the same way).
			return commercial.CommandReceipt{}, fmt.Errorf("%w: settle target purchase is terminal", commercial.ErrPlatformInvalidResponse)
		}
		if receipt, ok := f.receipts[cmd.Key]; ok {
			return receipt, nil // already settled under this channel identity
		}
		receipt := commercial.CommandReceipt{
			Key:        cmd.Key,
			ExternalID: payload.ExternalPurchaseSubscriptionID,
			RecordedAt: f.nowUTC(),
		}
		f.receipts[cmd.Key] = receipt
		sub.Status = "active"
		f.purchaseSubs[payload.ExternalPurchaseSubscriptionID] = sub
		return receipt, nil

	case commercial.CommandKindRebalanceCreditsOrder:
		payload, ok := cmd.Payload.(commercial.RebalanceCreditsOrderPayload)
		if !ok {
			return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
		}
		if err := payload.Validate(); err != nil {
			return commercial.CommandReceipt{}, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, err)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		// The same convergent calibration the Lago adapter runs: rank the
		// customer's non-terminated wallets by (expires_at, created_at) and
		// align each stored priority — aligned wallets stay untouched.
		inputs := make([]commercial.WalletRankInput, 0, len(f.wallets))
		for _, w := range f.wallets {
			if w.Customer != payload.ExternalCustomerID || w.Terminated {
				continue
			}
			inputs = append(inputs, commercial.WalletRankInput{
				WalletRef: w.Name, ExpiresAt: w.ExpiresAt, GrantedAt: w.CreatedAt,
			})
		}
		ranks, rerr := commercial.WalletRank(inputs)
		if rerr != nil {
			return commercial.CommandReceipt{}, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, rerr)
		}
		for i := range f.wallets {
			if f.wallets[i].Customer != payload.ExternalCustomerID || f.wallets[i].Terminated {
				continue
			}
			if rank, ranked := ranks[f.wallets[i].Name]; ranked {
				f.wallets[i].Priority = rank
			}
		}
		return commercial.CommandReceipt{
			Key:        cmd.Key,
			ExternalID: payload.ExternalCustomerID,
			RecordedAt: f.nowUTC(),
		}, nil

	case commercial.CommandKindIssueCreditNote:
		payload, ok := cmd.Payload.(commercial.IssueCreditNotePayload)
		if !ok {
			return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
		}
		if err := payload.Validate(); err != nil {
			return commercial.CommandReceipt{}, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, err)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, n := range f.creditNotes {
			if n.Key == cmd.Key {
				if n.Payload != payload {
					// Same refund identity, different correction: a
					// definitive conflict, never a second note.
					return commercial.CommandReceipt{}, fmt.Errorf("%w: credit note replay conflict", commercial.ErrPlatformInvalidResponse)
				}
				return n.Receipt, nil
			}
		}
		receipt := commercial.CommandReceipt{
			Key:        cmd.Key,
			ExternalID: "lago_cn_" + payload.RefundID,
			RecordedAt: f.nowUTC(),
		}
		f.creditNotes = append(f.creditNotes, fakeCreditNote{
			Key: cmd.Key, Payload: payload, Receipt: receipt,
		})
		return receipt, nil

	case commercial.CommandKindCloseWorkspace:
		payload, ok := cmd.Payload.(commercial.CloseWorkspacePayload)
		if !ok {
			return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
		}
		if err := payload.Validate(); err != nil {
			return commercial.CommandReceipt{}, fmt.Errorf("%w: %v", commercial.ErrPlatformInvalidResponse, err)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.failSubmits != nil {
			// Unreachable authority: nothing applies, the caller observes
			// the injected failure, and a replay after the outage converges.
			return commercial.CommandReceipt{}, f.failSubmits
		}
		if receipt, ok := f.receipts[cmd.Key]; ok {
			return receipt, nil // closure already disposed; a replay is a no-op
		}
		// Dispose the customer's objects: every subscription (standard and
		// purchase) is terminated, every wallet terminated, and the display
		// name rewritten to the de-identified form. Financial history rows
		// (credit notes, invoices) are never touched.
		for id, sub := range f.subs {
			if sub.ExternalCustomer == payload.ExternalCustomerID {
				sub.Terminated = true
				f.subs[id] = sub
			}
		}
		for id, p := range f.purchaseSubs {
			if p.ExternalCustomer == payload.ExternalCustomerID && p.Status != "canceled" {
				p.Status = "canceled"
				f.purchaseSubs[id] = p
			}
		}
		for i, w := range f.wallets {
			if w.Customer == payload.ExternalCustomerID {
				w.Terminated = true
				f.wallets[i] = w
			}
		}
		if f.customers != nil {
			if c, held := f.customers[payload.ExternalCustomerID]; held {
				c.Name = payload.DisplayName
				f.customers[payload.ExternalCustomerID] = c
			}
		}
		receipt := commercial.CommandReceipt{
			Key:        cmd.Key,
			ExternalID: payload.ExternalCustomerID,
			RecordedAt: f.nowUTC(),
		}
		if f.receipts == nil {
			f.receipts = map[string]commercial.CommandReceipt{}
		}
		f.receipts[cmd.Key] = receipt
		return receipt, nil

	default:
		return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
	}
}

// CreditNoteCount answers how many credit notes the fake authority holds —
// append-only: a replay never adds one, a refused divergent replay never
// adds one.
func (f *FakeAdapter) CreditNoteCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.creditNotes)
}

// Reconcile stays frozen and disabled: fail closed.
func (f *FakeAdapter) Reconcile(_ context.Context, _ commercial.ReconciliationCursor) (commercial.ReconciliationPage, error) {
	return commercial.ReconciliationPage{}, commercial.ErrPlatformUnsupported
}
