// Package commercialplatform implements the adapters behind the frozen
// CommercialPlatform seam (internal/commercial/platform.go): a deterministic
// fake for tests and dev, and a Lago adapter that reads the authority's
// health signal with server-side env config and fails closed. Both adapters
// run the same shared contract suite (contract_test.go); a behavior only one
// adapter has is a defect. W3 (#78) enables the first command kind —
// ensure_customer, idempotent by deterministic identity — and the account
// snapshot kind; every other command kind and the reconcile family stay
// frozen and fail closed with commercial.ErrPlatformUnsupported.
package commercialplatform

import (
	"context"
	"sort"
	"sync"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// FakeCustomer is one stored authority-side customer in the fake: the
// deterministic identity plus the ADVISORY display name at last ensure.
type FakeCustomer struct {
	ExternalID string
	Name       string
}

// FakeAdapter is the deterministic in-process CommercialPlatform used by
// tests and dev environments. SetReadiness stores one readiness snapshot;
// ReadSnapshot copies it back verbatim. The W3 customer surface is a real
// in-memory authority: customers are stored keyed by external id (upsert —
// never a second entry), submits are idempotent per Command.Key, and the
// account snapshot derives from the store honestly — an absent customer is
// absent, never a fabricated linked. The store mutates now, so every access
// is mutex-guarded.
type FakeAdapter struct {
	mu          sync.Mutex
	primed      bool
	readiness   commercial.ReadinessSnapshot
	customers   map[string]FakeCustomer
	receipts    map[string]commercial.CommandReceipt
	failSubmits error
}

// NewFakeAdapter builds the fake with no readiness primed: reading readiness
// before SetReadiness fails closed with ErrPlatformUnconfigured — the fake
// never fabricates platform state either.
func NewFakeAdapter() *FakeAdapter {
	return &FakeAdapter{
		customers: map[string]FakeCustomer{},
		receipts:  map[string]commercial.CommandReceipt{},
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

// ReadSnapshot answers the primed readiness snapshot verbatim and the
// account truth from the customer store; unknown kinds fail closed
// unsupported.
func (f *FakeAdapter) ReadSnapshot(_ context.Context, query commercial.SnapshotQuery) (commercial.Snapshot, error) {
	switch query.Kind {
	case commercial.SnapshotKindReadiness:
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
	default:
		return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
	}
}

// SubmitCommand implements ensure_customer (the W3 first enabled kind,
// #78): idempotent per Command.Key — a replay returns the ORIGINAL receipt
// with unchanged RecordedAt — and upserting per external id, so a different
// Key addressing the same identity refreshes advisory metadata and never
// creates a second customer. Every other kind fails closed unsupported.
func (f *FakeAdapter) SubmitCommand(_ context.Context, cmd commercial.Command) (commercial.CommandReceipt, error) {
	if cmd.Kind != commercial.CommandKindEnsureCustomer {
		return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
	}
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
		return receipt, nil // the ORIGINAL receipt, unchanged RecordedAt
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
}

// Reconcile stays frozen and disabled: fail closed.
func (f *FakeAdapter) Reconcile(_ context.Context, _ commercial.ReconciliationCursor) (commercial.ReconciliationPage, error) {
	return commercial.ReconciliationPage{}, commercial.ErrPlatformUnsupported
}
