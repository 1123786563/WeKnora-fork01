package commercialplatform

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	commercial "github.com/Tencent/WeKnora/internal/modules/commercial"
)

// contractGrantPeriod is a safely-future calendar period for the grant legs
// (payload validation requires the exclusive period end to be ahead).
const contractGrantPeriod = "2099-01"

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

// runPublishContract is the shared publish_plan_version leg every adapter
// must satisfy identically (#79): publish returns a receipt echoing the
// deterministic plan code, a replay of the same Key NEVER creates a second
// external object, and the same Key with DIFFERENT content is a definitive
// conflict. creates() reports how many external creates the adapter issued
// (fake: recorded commands; Lago stub: POST /api/v1/plans count).
func runPublishContract(t *testing.T, name string, p commercial.CommercialPlatform, creates func() int) {
	t.Helper()
	payload := commercial.PublishPlanVersionPayload{
		PlanKey:              "contract-plan",
		Version:              1,
		PlanCode:             commercial.DeterministicPlanCode("contract-plan", 1),
		Name:                 "Contract Plan",
		Interval:             "monthly",
		AmountFen:            9900,
		Currency:             "CNY",
		PayInAdvance:         true,
		IncludedCreditsMicro: 9_900_000,
	}
	cmd := func(payload commercial.PublishPlanVersionPayload) commercial.Command {
		return commercial.Command{
			Kind:    commercial.CommandKindPublishPlanVersion,
			Key:     commercial.PublishCommandKey(payload.PlanKey, payload.Version),
			Actor:   "contract",
			Reason:  "shared contract",
			Payload: payload,
		}
	}

	t.Run(name+"/publish returns a receipt echoing the plan code", func(t *testing.T) {
		receipt, err := p.SubmitCommand(context.Background(), cmd(payload))
		if err != nil {
			t.Fatalf("publish: %v", err)
		}
		if receipt.Key != cmd(payload).Key {
			t.Fatalf("receipt key = %q", receipt.Key)
		}
		if receipt.ExternalID != payload.PlanCode {
			t.Fatalf("receipt ExternalID = %q, want %q", receipt.ExternalID, payload.PlanCode)
		}
		if receipt.RecordedAt.IsZero() {
			t.Fatal("receipt RecordedAt must be set")
		}
	})

	t.Run(name+"/replay of the same key never creates twice", func(t *testing.T) {
		before := creates()
		receipt, err := p.SubmitCommand(context.Background(), cmd(payload))
		if err != nil {
			t.Fatalf("replay: %v", err)
		}
		if receipt.ExternalID != payload.PlanCode {
			t.Fatalf("replay receipt ExternalID = %q", receipt.ExternalID)
		}
		if after := creates(); after != before {
			t.Fatalf("replay must not create a second external object: %d -> %d", before, after)
		}
	})

	t.Run(name+"/same key with different content is a conflict", func(t *testing.T) {
		different := payload
		different.AmountFen = 19_900
		_, err := p.SubmitCommand(context.Background(), cmd(different))
		if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
			t.Fatalf("content conflict must be ErrPlatformInvalidResponse, got %v", err)
		}
		if after := creates(); after != 1 {
			t.Fatalf("the conflict must not create anything new, creates = %d", after)
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

	// The publish leg against the fake: the fake exposes Commands() so the
	// contract can observe that a replay creates nothing.
	publishFake := NewFakeAdapter()
	runPublishContract(t, "fake", publishFake, func() int { return len(publishFake.Commands()) })
}

// contractGrantCommand builds the shared grant command for (tenant, period).
func contractGrantCommand(tenant uint64, period string, credits int64) commercial.Command {
	end := time.Date(2099, 2, 1, 0, 0, 0, 0, time.UTC)
	if period != contractGrantPeriod {
		var err error
		end, err = commercial.PeriodEnd(period)
		if err != nil {
			panic(err)
		}
	}
	return commercial.Command{
		Kind:  commercial.CommandKindGrantIncludedCredits,
		Key:   commercial.GrantCreditsCommandKey(commercial.ExternalCustomerID(tenant), period),
		Actor: "contract", Reason: "monthly_included_credits",
		Payload: commercial.GrantIncludedCreditsPayload{
			TenantID:           tenant,
			ExternalCustomerID: commercial.ExternalCustomerID(tenant),
			Period:             period,
			CreditsMicro:       credits,
			ExpiresAt:          end,
		},
	}
}

// contractEnsureSubCommand builds the shared ensure_subscription command.
func contractEnsureSubCommand(tenant uint64, planCode string) commercial.Command {
	return commercial.Command{
		Kind:  commercial.CommandKindEnsureSubscription,
		Key:   commercial.EnsureSubscriptionCommandKey(commercial.ExternalSubscriptionID(tenant)),
		Actor: "contract", Reason: "first_billing_access",
		Payload: commercial.EnsureSubscriptionPayload{
			TenantID:               tenant,
			ExternalCustomerID:     commercial.ExternalCustomerID(tenant),
			ExternalSubscriptionID: commercial.ExternalSubscriptionID(tenant),
			PlanCode:               planCode,
		},
	}
}

// runSubscriptionContract is the shared ensure_subscription leg every
// adapter must satisfy identically (#80): the ensure is idempotent BY
// IDENTITY — a replay answers the same receipt and the authority holds
// exactly one subscription; a held subscription on a DIFFERENT plan code is
// a definitive conflict, never a parallel subscription.
// subscriptionCount() reports the authority-side count for the tenant.
func runSubscriptionContract(t *testing.T, name string, p commercial.CommercialPlatform, tenant uint64, planCode string, subscriptionCount func() int) {
	t.Helper()
	cmd := contractEnsureSubCommand(tenant, planCode)
	t.Run(name+"/ensure_subscription is idempotent by identity", func(t *testing.T) {
		first, err := p.SubmitCommand(context.Background(), cmd)
		if err != nil {
			t.Fatalf("first ensure_subscription: %v", err)
		}
		if first.ExternalID != commercial.ExternalSubscriptionID(tenant) {
			t.Fatalf("receipt must carry the deterministic identity, got %q", first.ExternalID)
		}
		second, err := p.SubmitCommand(context.Background(), cmd)
		if err != nil {
			t.Fatalf("replay ensure_subscription: %v", err)
		}
		if second.ExternalID != first.ExternalID {
			t.Fatalf("replay must answer the same identity, %q vs %q", second.ExternalID, first.ExternalID)
		}
		if got := subscriptionCount(); got != 1 {
			t.Fatalf("the authority must hold exactly ONE subscription, got %d", got)
		}
	})
	t.Run(name+"/different plan code is a definitive conflict", func(t *testing.T) {
		_, err := p.SubmitCommand(context.Background(), contractEnsureSubCommand(tenant, "weknora-pro-v1"))
		if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
			t.Fatalf("a held subscription on another plan must conflict with ErrPlatformInvalidResponse, got %v", err)
		}
		if got := subscriptionCount(); got != 1 {
			t.Fatalf("a conflict must never mint a parallel subscription, got %d", got)
		}
	})
}

// runGrantContract is the shared grant_included_credits leg every adapter
// must satisfy identically (#80, the T03 E3 highest-severity hole): a grant
// replay NEVER doubles the balance — walletCount/balanceMicro observe the
// authority state on both adapters — and the same Key with different content
// is a definitive conflict.
func runGrantContract(t *testing.T, name string, p commercial.CommercialPlatform, tenant uint64, walletCount func() int, balanceMicro func() (int64, error)) {
	t.Helper()
	const credits = 1_000_000
	cmd := contractGrantCommand(tenant, contractGrantPeriod, credits)
	t.Run(name+"/grant issues one batch with one month's balance", func(t *testing.T) {
		receipt, err := p.SubmitCommand(context.Background(), cmd)
		if err != nil {
			t.Fatalf("grant: %v", err)
		}
		if receipt.ExternalID != commercial.MonthlyWalletName(tenant, contractGrantPeriod) {
			t.Fatalf("receipt must carry the deterministic wallet name, got %q", receipt.ExternalID)
		}
		if got := walletCount(); got != 1 {
			t.Fatalf("exactly one wallet expected, got %d", got)
		}
	})
	t.Run(name+"/grant replay never doubles the balance", func(t *testing.T) {
		before, err := balanceMicro()
		if err != nil {
			t.Fatalf("observe balance: %v", err)
		}
		if before != credits {
			t.Fatalf("balance after one grant = %d, want %d", before, credits)
		}
		if _, err := p.SubmitCommand(context.Background(), cmd); err != nil {
			t.Fatalf("replay: %v", err)
		}
		after, err := balanceMicro()
		if err != nil {
			t.Fatalf("observe balance: %v", err)
		}
		if after != before {
			t.Fatalf("a replayed grant DOUBLED the balance (E3 anti-pattern): %d -> %d", before, after)
		}
		if got := walletCount(); got != 1 {
			t.Fatalf("a replay must not create a second wallet, got %d", got)
		}
	})
	t.Run(name+"/same key different content is a conflict", func(t *testing.T) {
		_, err := p.SubmitCommand(context.Background(), contractGrantCommand(tenant, contractGrantPeriod, 2_000_000))
		if !errors.Is(err, commercial.ErrPlatformInvalidResponse) {
			t.Fatalf("same key with different credits must be ErrPlatformInvalidResponse, got %v", err)
		}
		if got := walletCount(); got != 1 {
			t.Fatalf("the conflict must not create anything, got %d wallets", got)
		}
	})
}

// runBenefitsContract is the shared benefits snapshot leg: the ensured
// tenant answers the closed truth (active subscription, plan code, feature
// map, one month's balance); an untouched tenant answers honest absence
// (pending state, zero balance) — never a fabricated answer.
func runBenefitsContract(t *testing.T, name string, p commercial.CommercialPlatform, tenant, untouched uint64, planCode string) {
	t.Helper()
	t.Run(name+"/benefits snapshot answers the closed truth", func(t *testing.T) {
		snap, err := p.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
			Kind: commercial.SnapshotKindBenefits, TenantID: tenant,
		})
		if err != nil {
			t.Fatalf("benefits snapshot: %v", err)
		}
		if snap.Benefits == nil {
			t.Fatalf("benefits snapshot must carry the Benefits section")
		}
		if snap.Benefits.SubscriptionState != commercial.SubscriptionStateActive {
			t.Fatalf("ensured tenant must answer active, got %q", snap.Benefits.SubscriptionState)
		}
		if snap.Benefits.PlanCode != planCode {
			t.Fatalf("plan code = %q, want %q", snap.Benefits.PlanCode, planCode)
		}
		if !snap.Benefits.Features["api_access"] {
			t.Fatalf("features must carry the primed entitlement, got %+v", snap.Benefits.Features)
		}
		if snap.Benefits.BalanceMicro != 1_000_000 {
			t.Fatalf("balance = %d micro, want one month's 1000000", snap.Benefits.BalanceMicro)
		}
		if len(snap.Benefits.Batches) != 1 {
			t.Fatalf("one batch expected, got %+v", snap.Benefits.Batches)
		}
	})
	t.Run(name+"/untouched tenant answers honest absence", func(t *testing.T) {
		snap, err := p.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
			Kind: commercial.SnapshotKindBenefits, TenantID: untouched,
		})
		if err != nil {
			t.Fatalf("benefits snapshot: %v", err)
		}
		if snap.Benefits == nil || snap.Benefits.SubscriptionState != commercial.SubscriptionStatePending ||
			snap.Benefits.BalanceMicro != 0 || len(snap.Benefits.Batches) != 0 || snap.Benefits.PlanCode != "" {
			t.Fatalf("untouched tenant must answer pending/zero/absent honestly, got %+v", snap.Benefits)
		}
	})
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

// TestLagoAdapterPublishContract registers the stub-backed Lago leg of the
// shared publish contract: the same legs the fake runs, observed through
// the stub's POST /api/v1/plans count.
func TestLagoAdapterPublishContract(t *testing.T) {
	stub := newPlanStub(t)
	stub.readbackAmountCen = 9900
	stub.createStatuses = []int{http.StatusCreated, http.StatusUnprocessableEntity}
	runPublishContract(t, "lago", NewLagoAdapter(lagoTestConfig(stub.url())), stub.countCreatePosts)
}

// TestFakeAdapterSubscriptionContract registers the fake legs of the T08
// shared contract table (#80): ensure_subscription, grant_included_credits
// and the benefits snapshot, observed through the fake's state hooks.
func TestFakeAdapterSubscriptionContract(t *testing.T) {
	fake := NewFakeAdapter()
	fake.SetBasePlanFeatures(map[string]bool{"api_access": true})
	runSubscriptionContract(t, "fake", fake, 111, "weknora-base-v1", func() int {
		return len(fake.Subscriptions())
	})
	runGrantContract(t, "fake", fake, 111,
		func() int { return len(fake.Wallets()) },
		func() (int64, error) {
			var cents int64
			for _, w := range fake.Wallets() {
				cents += w.BalanceCents
			}
			return commercial.CentsToMicro(cents), nil
		})
	runBenefitsContract(t, "fake", fake, 111, 112, "weknora-base-v1")
}

// TestLagoAdapterSubscriptionContract registers the stub-backed Lago legs
// of the SAME T08 contract table — identical legs for both adapters. The
// wallet balance is observed through the stub wallet GET (the settle-poll
// path), the way the runtime observes it.
func TestLagoAdapterSubscriptionContract(t *testing.T) {
	stub := newCombinedStub(t)
	stub.wallets.entitlementCustomer = commercial.ExternalCustomerID(113)
	p := NewLagoAdapter(lagoTestConfig(stub.url()))
	runSubscriptionContract(t, "lago", p, 113, "weknora-base-v1", func() int {
		return stub.subs.countCreates()
	})
	runGrantContract(t, "lago", p, 113,
		func() int { return stub.wallets.countCreates() },
		func() (int64, error) {
			snap, err := p.ReadSnapshot(context.Background(), commercial.SnapshotQuery{
				Kind: commercial.SnapshotKindBenefits, TenantID: 113,
			})
			if err != nil {
				return 0, err
			}
			return snap.Benefits.BalanceMicro, nil
		})
	runBenefitsContract(t, "lago", p, 113, 114, "weknora-base-v1")
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
