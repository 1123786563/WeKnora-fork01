package craft

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/commercial"
)

// TestUsageKeyCountsPhysicalAttempts is the O01 RED-first contract test: the
// usage dedup key must count one physical attempt each. A real retry of the
// same logical call is a NEW attempt (distinct attempt id) and must never
// collapse into the failed attempt it replaces; the same attempt redelivered
// (at-least-once observation) must key identically so it is counted once;
// and two tenants reusing the same call id must never mix ledgers.
func TestUsageKeyCountsPhysicalAttempts(t *testing.T) {
	if UsageKey(1, "call", "a1") == UsageKey(1, "call", "a2") {
		t.Fatal("retry lost")
	}
	if UsageKey(1, "call", "a1") == UsageKey(2, "call", "a1") {
		t.Fatal("tenant collision")
	}
	if UsageKey(1, "call", "a1") != UsageKey(1, "call", "a1") {
		t.Fatal("redelivery counted twice")
	}
}

func usageFactFixture() UsageFact {
	return UsageFact{
		RunID:     "run-1",
		CallID:    "call-1",
		AttemptID: "att-1",
		Runtime:   RuntimeMain,
		ModelID:   "gpt-test",
		Funding:   commercial.FundingPlatform,
		Status:    UsageStatusReported,
		TenantID:  1,
		Input:     100,
		Output:    40,
		Cached:    10,
	}
}

// TestUsageFactValidateEnforcesPhysicalIdentity pins the fact invariants:
// identity is complete, the runtime is one of the two physical planes, an OC
// child attempt always names its delegation, the status is a known
// observation status, token counts are non-negative, and the fixed provider
// contract holds — cached tokens are the cache-hit portion of input, never
// additive. An unknown fact must not carry invented numbers.
func TestUsageFactValidateEnforcesPhysicalIdentity(t *testing.T) {
	if err := usageFactFixture().Validate(); err != nil {
		t.Fatalf("valid fact rejected: %v", err)
	}

	noTenant := usageFactFixture()
	noTenant.TenantID = 0
	if err := noTenant.Validate(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("missing tenant: got %v", err)
	}

	noCall := usageFactFixture()
	noCall.CallID = ""
	if err := noCall.Validate(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("missing call id: got %v", err)
	}

	badRuntime := usageFactFixture()
	badRuntime.Runtime = "sidecar"
	if err := badRuntime.Validate(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown runtime: got %v", err)
	}

	orphanChild := usageFactFixture()
	orphanChild.Runtime = RuntimeOC
	if err := orphanChild.Validate(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("oc attempt without delegation: got %v", err)
	}
	orphanChild.DelegationID = "dlg-1"
	if err := orphanChild.Validate(); err != nil {
		t.Fatalf("oc attempt with delegation rejected: %v", err)
	}

	badStatus := usageFactFixture()
	badStatus.Status = "final-ish"
	if err := badStatus.Validate(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown status: got %v", err)
	}

	negative := usageFactFixture()
	negative.Output = -1
	if err := negative.Validate(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("negative tokens: got %v", err)
	}

	additiveCache := usageFactFixture()
	additiveCache.Cached = additiveCache.Input + 1
	if err := additiveCache.Validate(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("cached tokens counted on top of input: %v", err)
	}

	fabricatedUnknown := usageFactFixture()
	fabricatedUnknown.Status = UsageStatusUnknown
	if err := fabricatedUnknown.Validate(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown fact carrying numbers: %v", err)
	}
}

// TestUsageFactResolvedIDBindsIdentity: the fact id IS the usage key of its
// own identity, so a spoofed id cannot alias another tenant's or another
// attempt's ledger row; an empty id is derived, not rejected.
func TestUsageFactResolvedIDBindsIdentity(t *testing.T) {
	f := usageFactFixture()
	key := UsageKey(f.TenantID, f.CallID, f.AttemptID)
	resolved, err := f.ResolvedID()
	if err != nil {
		t.Fatalf("derive id: %v", err)
	}
	if resolved != key {
		t.Fatalf("derived id %q != usage key %q", resolved, key)
	}

	f.ID = key
	if resolved, err = f.ResolvedID(); err != nil || resolved != key {
		t.Fatalf("bound id rejected: %v", err)
	}

	f.ID = UsageKey(2, f.CallID, f.AttemptID)
	if _, err = f.ResolvedID(); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("spoofed id accepted: %v", err)
	}
}

// TestDeriveCallIdentityBindsServerSideBinding: the call identity the model
// gateway derives BEFORE forwarding is bound to the server-resolved
// tenant/run/delegation/model/funding, so changing any binding (a different
// funding source must never adopt another ledger's call) yields a different
// call, and a real retry yields a new physical attempt id.
func TestDeriveCallIdentityBindsServerSideBinding(t *testing.T) {
	call := DeriveCallID(1, "run-1", "", "gpt-test", commercial.FundingPlatform, 1)
	if call == "" {
		t.Fatal("empty call id")
	}
	if call != DeriveCallID(1, "run-1", "", "gpt-test", commercial.FundingPlatform, 1) {
		t.Fatal("call id not stable for the same binding")
	}
	if call == DeriveCallID(1, "run-1", "", "gpt-test", commercial.FundingBYOK, 1) {
		t.Fatal("funding not bound to the call identity")
	}
	if call == DeriveCallID(1, "run-1", "", "gpt-byok", commercial.FundingPlatform, 1) {
		t.Fatal("model not bound to the call identity")
	}
	childCall := DeriveCallID(1, "run-1", "dlg-1", "gpt-test", commercial.FundingPlatform, 1)
	if childCall == call {
		t.Fatal("delegation not bound to the call identity")
	}

	first := DeriveAttemptID(call, 1)
	if first == "" || first == DeriveAttemptID(call, 2) {
		t.Fatal("retry must be a new physical attempt id")
	}
}

// TestUsageTotalsExcludesUnknown: unknown facts stay a separate line item —
// they never contribute zeros (or guesses) to the observed totals.
func TestUsageTotalsExcludesUnknown(t *testing.T) {
	reported := usageFactFixture()
	unknown := usageFactFixture()
	unknown.CallID = "call-2"
	unknown.Status = UsageStatusUnknown

	var totals UsageTotals
	totals.Add(reported)
	totals.Add(unknown)
	if totals.Input != 100 || totals.Output != 40 || totals.Cached != 10 {
		t.Fatalf("reported tokens not summed: %+v", totals)
	}
	if totals.Facts != 1 {
		t.Fatalf("unknown fact counted as observed: %+v", totals)
	}
	if totals.Unknown != 1 {
		t.Fatalf("unknown fact not listed separately: %+v", totals)
	}
}

// TestUsageSinkContract documents the sink contract through the interface:
// appending the same physical fact twice is idempotent, and a different fact
// under the same id is a conflict that must travel through Correct, never an
// overwrite. The durable sink is the repository; this in-memory fake pins the
// same semantics the repository tests assert against the migrated database.
func TestUsageSinkContract(t *testing.T) {
	sink := newFakeUsageSink()
	f := usageFactFixture()
	if err := sink.Append(context.Background(), f); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := sink.Append(context.Background(), f); err != nil {
		t.Fatalf("redelivery must be idempotent: %v", err)
	}
	changed := f
	changed.Output = 99
	if err := sink.Append(context.Background(), changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed fact under the same id must conflict: %v", err)
	}
	unrecorded := f
	unrecorded.CallID = "call-never"
	if err := sink.Correct(context.Background(), unrecorded); !errors.Is(err, ErrUsageNotRecorded) {
		t.Fatalf("correct unrecorded attempt must fail: %v", err)
	}
	late := f
	late.Input, late.Output, late.Cached = 200, 80, 20
	late.Status = UsageStatusCorrected
	if err := sink.Correct(context.Background(), late); err != nil {
		t.Fatalf("correct: %v", err)
	}
	current, revisions := sink.lookup(f.TenantID, f.CallID, f.AttemptID)
	if len(revisions) != 2 {
		t.Fatalf("correction must append a revision, got %d", len(revisions))
	}
	if current.Output != 80 {
		t.Fatalf("current version must be the correction: %+v", current)
	}
	if revisions[0].Output != 40 {
		t.Fatalf("original revision overwritten: %+v", revisions[0])
	}
}

// fakeUsageSink is the in-memory UsageSink: same idempotency, conflict and
// revision semantics as the durable repository, used by domain and service
// tests without a database.
type fakeUsageSink struct {
	mu        sync.Mutex
	current   map[string]UsageFact
	revisions map[string][]UsageFact
}

func newFakeUsageSink() *fakeUsageSink {
	return &fakeUsageSink{
		current:   map[string]UsageFact{},
		revisions: map[string][]UsageFact{},
	}
}

func (s *fakeUsageSink) Append(_ context.Context, f UsageFact) error {
	id, err := f.ResolvedID()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if stored, ok := s.current[id]; ok {
		if sameUsageContent(stored, f) {
			return nil
		}
		return fmt.Errorf("%w: usage fact %s changed", ErrConflict, id)
	}
	stored := f
	stored.ID = id
	s.current[id] = stored
	s.revisions[id] = append(s.revisions[id], stored)
	return nil
}

func (s *fakeUsageSink) Correct(_ context.Context, f UsageFact) error {
	id, err := f.ResolvedID()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.current[id]; !ok {
		return ErrUsageNotRecorded
	}
	stored := f
	stored.ID = id
	stored.Status = UsageStatusCorrected
	s.current[id] = stored
	s.revisions[id] = append(s.revisions[id], stored)
	return nil
}

func (s *fakeUsageSink) lookup(tenant uint64, callID, attemptID string) (UsageFact, []UsageFact) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := UsageKey(tenant, callID, attemptID)
	return s.current[id], append([]UsageFact(nil), s.revisions[id]...)
}

// sameUsageContent compares the observation content of two facts of the same
// identity; the derived ID is not content.
func sameUsageContent(a, b UsageFact) bool {
	return a.RunID == b.RunID && a.DelegationID == b.DelegationID &&
		a.Runtime == b.Runtime && a.ModelID == b.ModelID &&
		a.Funding == b.Funding && a.Status == b.Status &&
		a.TenantID == b.TenantID && a.Input == b.Input &&
		a.Output == b.Output && a.Cached == b.Cached
}
