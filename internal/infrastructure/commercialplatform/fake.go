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
	commands    map[string]fakeCommand
	creates     []commercial.Command
}

// fakeCommand is one recorded publish command: its exact payload and the
// receipt issued for it.
type fakeCommand struct {
	payload commercial.PublishPlanVersionPayload
	receipt commercial.CommandReceipt
}

// NewFakeAdapter builds the fake with no readiness primed: reading readiness
// before SetReadiness fails closed with ErrPlatformUnconfigured — the fake
// never fabricates platform state either.
func NewFakeAdapter() *FakeAdapter {
	return &FakeAdapter{
		customers: map[string]FakeCustomer{},
		receipts:  map[string]commercial.CommandReceipt{},
		commands:  map[string]fakeCommand{},
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
	default:
		return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
	}
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

	default:
		return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
	}
}

// Reconcile stays frozen and disabled: fail closed.
func (f *FakeAdapter) Reconcile(_ context.Context, _ commercial.ReconciliationCursor) (commercial.ReconciliationPage, error) {
	return commercial.ReconciliationPage{}, commercial.ErrPlatformUnsupported
}
