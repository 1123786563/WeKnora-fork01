// Package commercialplatform implements the adapters behind the frozen
// CommercialPlatform seam (internal/commercial/platform.go): a deterministic
// fake for tests and dev, and a Lago adapter that reads the authority's
// health signal with server-side env config and fails closed. Both adapters
// run the same shared contract suite (contract_test.go); a behavior only one
// adapter has is a defect. T07 (#79) enables the first command kind,
// publish_plan_version, on BOTH adapters; every other command kind and the
// reconcile family stay frozen and fail closed with
// commercial.ErrPlatformUnsupported.
package commercialplatform

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// FakeAdapter is the deterministic in-process CommercialPlatform used by
// tests and dev environments. SetReadiness stores one readiness snapshot;
// ReadSnapshot copies it back verbatim. SubmitCommand implements exactly
// the publish_plan_version kind with coordinator-owned idempotency: the
// same Key with byte-equal payload replays the same receipt, the same Key
// with different content is a definitive conflict (spec story 59). Command
// state is mutex-guarded — concurrent publish tests are legal.
type FakeAdapter struct {
	mu        sync.Mutex
	primed    bool
	readiness commercial.ReadinessSnapshot
	commands  map[string]fakeCommand
	creates   []commercial.Command
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
	return &FakeAdapter{commands: map[string]fakeCommand{}}
}

// SetReadiness primes the readiness snapshot returned by ReadSnapshot.
func (f *FakeAdapter) SetReadiness(s commercial.ReadinessSnapshot) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.readiness = s
	f.primed = true
}

// ReadSnapshot answers the primed readiness snapshot verbatim; unknown kinds
// fail closed unsupported.
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
	default:
		return commercial.Snapshot{}, commercial.ErrPlatformUnsupported
	}
}

// SubmitCommand applies the enabled command families. publish_plan_version
// validates the command and payload, records under the coordinator Key and
// answers a receipt whose ExternalID echoes the payload's deterministic
// plan code. A replay with equal payload returns the SAME receipt without a
// second record; the same Key with different payload is a conflict. Every
// other kind fails closed unsupported.
func (f *FakeAdapter) SubmitCommand(_ context.Context, cmd commercial.Command) (commercial.CommandReceipt, error) {
	if err := cmd.Validate(); err != nil {
		return commercial.CommandReceipt{}, err
	}
	switch cmd.Kind {
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

// Commands returns the commands that CREATED state, in order — the test
// observation point proving a replay creates nothing (each create appends
// exactly once).
func (f *FakeAdapter) Commands() []commercial.Command {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]commercial.Command(nil), f.creates...)
}

// Reconcile stays frozen and disabled: fail closed.
func (f *FakeAdapter) Reconcile(_ context.Context, _ commercial.ReconciliationCursor) (commercial.ReconciliationPage, error) {
	return commercial.ReconciliationPage{}, commercial.ErrPlatformUnsupported
}
