package commercialplatform

import (
	"context"
	"errors"
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
// demands the closed ready state and an empty reason.
func runPlatformContract(t *testing.T, name string, p commercial.CommercialPlatform, wantReady func() bool) {
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

	t.Run(name+"/any submit command fails closed unsupported", func(t *testing.T) {
		_, err := p.SubmitCommand(context.Background(), commercial.Command{
			Kind: commercial.CommandKind("ensure_customer"),
			Key:  "contract-1",
		})
		if !errors.Is(err, commercial.ErrPlatformUnsupported) {
			t.Fatalf("SubmitCommand must fail closed with ErrPlatformUnsupported in T05, got %v", err)
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
	runPlatformContract(t, "fake-ready", ready, func() bool { return true })

	unavailable := NewFakeAdapter()
	unavailable.SetReadiness(commercial.ReadinessSnapshot{
		State:     commercial.ReadinessUnavailable,
		CheckedAt: time.Now().UTC(),
		Reason:    "unconfigured",
	})
	runPlatformContract(t, "fake-unavailable", unavailable, func() bool { return false })
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
