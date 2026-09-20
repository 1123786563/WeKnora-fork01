// Package commercialplatform implements the adapters behind the frozen
// CommercialPlatform seam (internal/commercial/platform.go): a deterministic
// fake for tests and dev, and a Lago adapter that reads the authority's
// health signal with server-side env config and fails closed. Both adapters
// run the same shared contract suite (contract_test.go); a behavior only one
// adapter has is a defect. The command and reconcile families stay frozen
// and disabled in T05 — both adapters answer them with
// commercial.ErrPlatformUnsupported.
package commercialplatform

import (
	"context"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// FakeAdapter is the deterministic in-process CommercialPlatform used by
// tests and dev environments. SetReadiness stores one readiness snapshot;
// ReadSnapshot copies it back verbatim. Mutex-free reads are fine for the
// test/dev usage this adapter is built for.
type FakeAdapter struct {
	primed    bool
	readiness commercial.ReadinessSnapshot
}

// NewFakeAdapter builds the fake with no readiness primed: reading readiness
// before SetReadiness fails closed with ErrPlatformUnconfigured — the fake
// never fabricates platform state either.
func NewFakeAdapter() *FakeAdapter {
	return &FakeAdapter{}
}

// SetReadiness primes the readiness snapshot returned by ReadSnapshot.
func (f *FakeAdapter) SetReadiness(s commercial.ReadinessSnapshot) {
	f.readiness = s
	f.primed = true
}

// ReadSnapshot answers the primed readiness snapshot verbatim; unknown kinds
// fail closed unsupported.
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
	default:
		return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
	}
}

// SubmitCommand is frozen and disabled in T05: fail closed.
func (f *FakeAdapter) SubmitCommand(_ context.Context, _ commercial.Command) (commercial.CommandReceipt, error) {
	return commercial.CommandReceipt{}, commercial.ErrPlatformUnsupported
}

// Reconcile is frozen and disabled in T05: fail closed.
func (f *FakeAdapter) Reconcile(_ context.Context, _ commercial.ReconciliationCursor) (commercial.ReconciliationPage, error) {
	return commercial.ReconciliationPage{}, commercial.ErrPlatformUnsupported
}
