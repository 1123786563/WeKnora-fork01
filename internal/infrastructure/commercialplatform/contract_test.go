package commercialplatform

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/commercial"
)

// runPlatformContract is the ONE shared contract suite every adapter behind
// the frozen CommercialPlatform seam must satisfy — the fake and the Lago
// adapter run the exact same table (acceptance criterion: "fake adapter 与
// Lago adapter 通过同一接口契约"). A behavior only one adapter has is a
// defect.
//
// wantReady reports whether the adapter, as primed by the caller, is expected
// to answer the readiness snapshot as ready; the contract then additionally
// demands the closed ready state and an empty reason. ensureTenant is the
// tenant the W3 legs (ensure_customer command + account snapshot) run
// against; both adapters must prove the identical idempotency-by-identity
// contract for it.
func runPlatformContract(t *testing.T, name string, p commercial.CommercialPlatform, wantReady func() bool, ensureTenant uint64) {
	t.Helper()

	t.Run(name+"/readiness snapshot is a closed product answer", func(t *testing.T) {
		snap, err := p.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
			Kind: commercial.SnapshotKindReadiness,
		})
		if err != nil {
			t.Fatalf("readiness snapshot failed: %v", err)
		}
		if snap.Kind != commercial.SnapshotKindReadiness {
			t.Fatalf("snapshot kind = %q, want %q", snap.Kind, commercial.SnapshotKindReadiness)
		}
		if snap.Readiness == nil {
			t.Fatalf("readiness snapshot must carry a Readiness section")
		}
		switch snap.Readiness.State {
		case commercial.ReadinessReady, commercial.ReadinessDegraded, commercial.ReadinessUnavailable:
		default:
			t.Fatalf("state %q is outside the closed enum ready/degraded/unavailable", snap.Readiness.State)
		}
		if snap.Readiness.CheckedAt.IsZero() {
			t.Fatalf("CheckedAt must be set, got the zero time")
		}
		if wantReady() {
			if snap.Readiness.State != commercial.ReadinessReady {
				t.Fatalf("primed-ready adapter must answer ready, got %q", snap.Readiness.State)
			}
			if snap.Readiness.Reason != "" {
				t.Fatalf("ready snapshot must carry an empty reason, got %q", snap.Readiness.Reason)
			}
		}
	})

	t.Run(name+"/unknown snapshot kind fails closed unsupported", func(t *testing.T) {
		_, err := p.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
			Kind: commercial.SnapshotKind("no_such_kind"),
		})
		if !errors.Is(err, commercial.ErrPlatformUnsupported) {
			t.Fatalf("unknown snapshot kind must fail closed with ErrPlatformUnsupported, got %v", err)
		}
	})

	t.Run(name+"/unknown command kind fails closed unsupported", func(t *testing.T) {
		_, err := p.SubmitCommand(context.Background(), commercial.Command{
			Kind: commercial.CommandKind("no_such_kind"),
			Key:  "contract-1",
		})
		if !errors.Is(err, commercial.ErrPlatformUnsupported) {
			t.Fatalf("an unknown command kind must fail closed with ErrPlatformUnsupported, got %v", err)
		}
	})

	// W3 leg (#78): ensure_customer is idempotent BY IDENTITY — the same Key
	// replayed answers the same external id, and the authority holds exactly
	// that identity afterwards.
	t.Run(name+"/ensure_customer is idempotent by identity", func(t *testing.T) {
		ext := commercial.ExternalCustomerID(ensureTenant)
		key := "ensure_customer:" + ext
		cmd := commercial.Command{
			Kind:    commercial.CommandKindEnsureCustomer,
			Key:     key,
			Actor:   "contract",
			Reason:  "first_billing_access",
			Payload: commercial.EnsureCustomerPayload{TenantID: ensureTenant, ExternalCustomerID: ext, DisplayName: "Contract Space"},
		}
		first, err := p.SubmitCommand(context.Background(), cmd)
		if err != nil {
			t.Fatalf("first ensure_customer: %v", err)
		}
		second, err := p.SubmitCommand(context.Background(), cmd)
		if err != nil {
			t.Fatalf("replay ensure_customer: %v", err)
		}
		if first.ExternalID != ext || second.ExternalID != ext {
			t.Fatalf("both receipts must carry the deterministic identity %q, got %+v / %+v", ext, first, second)
		}
		if first.Key != key || second.Key != key {
			t.Fatalf("receipts must echo the command Key, got %+v / %+v", first, second)
		}
	})

	// W3 leg (#78): the account snapshot answers the closed authority truth
	// for the ensured tenant and stays honest (absent) for an untouched one.
	t.Run(name+"/account snapshot answers closed truth", func(t *testing.T) {
		snap, err := p.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
			Kind: commercial.SnapshotKindAccount, TenantID: ensureTenant,
		})
		if err != nil {
			t.Fatalf("account snapshot: %v", err)
		}
		if snap.Account == nil {
			t.Fatalf("account snapshot must carry the Account section")
		}
		if snap.Account.State != commercial.AccountStateLinked || snap.Account.TenantID != ensureTenant {
			t.Fatalf("ensured tenant must answer linked, got %+v", snap.Account)
		}
		other, err := p.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
			Kind: commercial.SnapshotKindAccount, TenantID: ensureTenant + 1,
		})
		if err != nil {
			t.Fatalf("other-tenant account snapshot: %v", err)
		}
		if other.Account == nil || other.Account.State != commercial.AccountStateAbsent {
			t.Fatalf("an untouched tenant must answer absent, got %+v", other.Account)
		}
	})

	t.Run(name+"/any reconcile fails closed unsupported", func(t *testing.T) {
		_, err := p.Reconcile(context.Background(), commercial.ReconciliationCursor{
			Stream: "billing", Value: "0",
		})
		if !errors.Is(err, commercial.ErrPlatformUnsupported) {
			t.Fatalf("Reconcile must fail closed with ErrPlatformUnsupported in T05, got %v", err)
		}
	})
}

// TestFakeAdapterContract registers the fake leg of the shared contract
// table (the Lago leg joins in lago_test.go once the adapter exists).
func TestFakeAdapterContract(t *testing.T) {
	ready := NewFakeAdapter()
	ready.SetReadiness(commercial.ReadinessSnapshot{
		State:     commercial.ReadinessReady,
		Release:   "v1.53.0",
		CheckedAt: time.Now().UTC(),
	})
	runPlatformContract(t, "fake-ready", ready, func() bool { return true }, 101)

	unavailable := NewFakeAdapter()
	unavailable.SetReadiness(commercial.ReadinessSnapshot{
		State:     commercial.ReadinessUnavailable,
		CheckedAt: time.Now().UTC(),
		Reason:    "unconfigured",
	})
	runPlatformContract(t, "fake-unavailable", unavailable, func() bool { return false }, 102)
}

// TestLagoAdapterContract registers the stub-backed Lago leg of the SAME
// contract table — acceptance criterion: fake and Lago adapter pass the
// identical interface contract. The customers stub answers absent on the
// identity GET (404) and created on the POST (200), so the W3 legs run the
// read-before-create path.
func TestLagoAdapterContract(t *testing.T) {
	stub := newCustomersStub(t, http.StatusNotFound, http.StatusOK)
	runPlatformContract(t, "lago", NewLagoAdapter(lagoTestConfig(stub.url())), func() bool { return true }, 103)
}

// TestFakeAdapterUnprimedFailsClosed: a fake that was never primed has no
// authoritative readiness to report — it must fail closed instead of
// fabricating a state.
func TestFakeAdapterUnprimedFailsClosed(t *testing.T) {
	_, err := NewFakeAdapter().ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindReadiness,
	})
	if !errors.Is(err, commercial.ErrPlatformUnconfigured) {
		t.Fatalf("unprimed fake must fail closed with ErrPlatformUnconfigured, got %v", err)
	}
}

// TestFakeAdapterReturnsStoredSnapshotVerbatim: SetReadiness stores,
// ReadSnapshot copies back verbatim (including release and reason).
func TestFakeAdapterReturnsStoredSnapshotVerbatim(t *testing.T) {
	primed := commercial.ReadinessSnapshot{
		State:     commercial.ReadinessDegraded,
		Release:   "v1.53.0",
		CheckedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
		Reason:    "unreachable",
	}
	fake := NewFakeAdapter()
	fake.SetReadiness(primed)
	snap, err := fake.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
		Kind: commercial.SnapshotKindReadiness,
	})
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if snap.Readiness == nil || *snap.Readiness != primed {
		t.Fatalf("snapshot must be copied back verbatim, got %+v", snap.Readiness)
	}
}
