package commercial

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ---- ensure_subscription payload ----

// validEnsureSubscriptionPayload is the canonical well-formed Base-Plan
// subscription payload; rejection cases mutate exactly one field.
func validEnsureSubscriptionPayload() EnsureSubscriptionPayload {
	return EnsureSubscriptionPayload{
		TenantID:               42,
		ExternalCustomerID:     ExternalCustomerID(42),
		ExternalSubscriptionID: ExternalSubscriptionID(42),
		PlanCode:               DeterministicPlanCode(BasePlanKey, 1),
	}
}

// TestEnsureSubscriptionPayloadValidate: the structural contract of the
// subscription ensure — derived-identity equality and a plan code to attach.
func TestEnsureSubscriptionPayloadValidate(t *testing.T) {
	rejects := []struct {
		name   string
		mutate func(*EnsureSubscriptionPayload)
	}{
		{"tenant zero", func(p *EnsureSubscriptionPayload) { p.TenantID = 0 }},
		{"mismatched customer identity", func(p *EnsureSubscriptionPayload) {
			p.ExternalCustomerID = ExternalCustomerID(43)
		}},
		{"mismatched subscription identity", func(p *EnsureSubscriptionPayload) {
			p.ExternalSubscriptionID = ExternalSubscriptionID(43)
		}},
		{"empty plan code", func(p *EnsureSubscriptionPayload) { p.PlanCode = "" }},
	}
	for _, tc := range rejects {
		t.Run("reject "+tc.name, func(t *testing.T) {
			p := validEnsureSubscriptionPayload()
			tc.mutate(&p)
			if err := p.Validate(); err == nil {
				t.Fatalf("payload must be rejected (%s), got nil", tc.name)
			}
		})
	}
	t.Run("accept fully formed base plan payload", func(t *testing.T) {
		if err := validEnsureSubscriptionPayload().Validate(); err != nil {
			t.Fatalf("fully-formed payload must be accepted, got %v", err)
		}
	})
}

// TestExternalSubscriptionIDDeterministic: the subscription continuity
// identity is a pure function of the tenant id — stable form, distinct per
// tenant.
func TestExternalSubscriptionIDDeterministic(t *testing.T) {
	if got := ExternalSubscriptionID(42); got != "weknora-tenant-42-sub" {
		t.Fatalf("ExternalSubscriptionID(42) = %q, want weknora-tenant-42-sub", got)
	}
	if ExternalSubscriptionID(42) != ExternalSubscriptionID(42) {
		t.Fatal("ExternalSubscriptionID must be pure")
	}
	if ExternalSubscriptionID(7) == ExternalSubscriptionID(8) {
		t.Fatal("distinct tenants must derive distinct subscription identities")
	}
	if !strings.HasPrefix(ExternalSubscriptionID(7), ExternalCustomerID(7)) {
		t.Fatal("the subscription identity must extend the customer identity namespace")
	}
}

// ---- grant_included_credits payload ----

// validGrantPayload is the canonical monthly grant payload for a period whose
// exclusive end is safely in the future.
func validGrantPayload(t *testing.T) GrantIncludedCreditsPayload {
	t.Helper()
	now := time.Now().UTC()
	period := MonthlyPeriod(now.AddDate(0, 1, 0)) // next month: end > now always
	end, err := PeriodEnd(period)
	if err != nil {
		t.Fatalf("PeriodEnd(%q): %v", period, err)
	}
	return GrantIncludedCreditsPayload{
		TenantID:           42,
		ExternalCustomerID: ExternalCustomerID(42),
		Period:             period,
		CreditsMicro:       1_000_000,
		ExpiresAt:          end,
		Priority:           MonthlyWalletPriority,
	}
}

// TestGrantPayloadValidate: the monthly grant contract — cent-aligned integer
// credits, a strict "YYYY-MM" UTC period, and the exclusive period end.
func TestGrantPayloadValidate(t *testing.T) {
	rejects := []struct {
		name   string
		mutate func(*GrantIncludedCreditsPayload)
	}{
		{"tenant zero", func(p *GrantIncludedCreditsPayload) { p.TenantID = 0 }},
		{"mismatched customer identity", func(p *GrantIncludedCreditsPayload) {
			p.ExternalCustomerID = ExternalCustomerID(43)
		}},
		{"credits zero", func(p *GrantIncludedCreditsPayload) { p.CreditsMicro = 0 }},
		{"credits negative", func(p *GrantIncludedCreditsPayload) { p.CreditsMicro = -1_000_000 }},
		{"credits not cent aligned", func(p *GrantIncludedCreditsPayload) { p.CreditsMicro = 1_234_567 }},
		{"period month thirteen", func(p *GrantIncludedCreditsPayload) { p.Period = "2026-13" }},
		{"period single digit month", func(p *GrantIncludedCreditsPayload) { p.Period = "2026-9" }},
		{"period garbage", func(p *GrantIncludedCreditsPayload) { p.Period = "abc" }},
		{"period empty", func(p *GrantIncludedCreditsPayload) { p.Period = "" }},
		{"expires at not the period end", func(p *GrantIncludedCreditsPayload) {
			p.ExpiresAt = p.ExpiresAt.Add(time.Hour)
		}},
		{"expires at already past", func(p *GrantIncludedCreditsPayload) {
			end, err := PeriodEnd("2020-01")
			if err != nil {
				t.Fatal(err)
			}
			p.Period, p.ExpiresAt = "2020-01", end
		}},
	}
	for _, tc := range rejects {
		t.Run("reject "+tc.name, func(t *testing.T) {
			p := validGrantPayload(t)
			tc.mutate(&p)
			if err := p.Validate(); err == nil {
				t.Fatalf("payload must be rejected (%s), got nil", tc.name)
			}
		})
	}
	t.Run("accept cent-aligned monthly payload", func(t *testing.T) {
		if err := validGrantPayload(t).Validate(); err != nil {
			t.Fatalf("monthly payload must be accepted, got %v", err)
		}
	})
	t.Run("period end rolls december into january", func(t *testing.T) {
		end, err := PeriodEnd("2026-12")
		if err != nil {
			t.Fatal(err)
		}
		if !end.Equal(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("PeriodEnd(2026-12) = %v, want 2027-01-01T00:00:00Z", end)
		}
	})
	t.Run("monthly period formats utc year month", func(t *testing.T) {
		if got := MonthlyPeriod(time.Date(2026, 9, 21, 23, 59, 0, 0, time.UTC)); got != "2026-09" {
			t.Fatalf("MonthlyPeriod = %q, want 2026-09", got)
		}
	})
}

// TestCreditsUnitHelpers: micro→decimal-string is exact integer mapping with
// trimmed trailing zeros — never binary float — and refuses non-cent-aligned
// input instead of rounding.
func TestCreditsUnitHelpers(t *testing.T) {
	table := []struct {
		micro int64
		want  string
	}{
		{9_900_000, "9.9"},
		{100_000, "0.1"},
		{1_000_000, "1"},
		{10_000, "0.01"},
		{12_300_000, "12.3"},
		{123_400_000, "123.4"},
		{0, "0"},
	}
	for _, tc := range table {
		got, err := MicroToDecimalString(tc.micro)
		if err != nil {
			t.Fatalf("MicroToDecimalString(%d): %v", tc.micro, err)
		}
		if got != tc.want {
			t.Fatalf("MicroToDecimalString(%d) = %q, want %q", tc.micro, got, tc.want)
		}
	}
	for _, bad := range []int64{1_234_567, 999, 10_001, -10_000} {
		if _, err := MicroToDecimalString(bad); err == nil {
			t.Fatalf("MicroToDecimalString(%d) must reject non-cent-aligned input", bad)
		}
	}
	if got := CentsToMicro(990); got != 9_900_000 {
		t.Fatalf("CentsToMicro(990) = %d, want 9900000", got)
	}
	if got := CentsToMicro(0); got != 0 {
		t.Fatalf("CentsToMicro(0) = %d, want 0", got)
	}
	// Round-trip property for the cent-aligned set: micro → decimal string →
	// cents → micro is the identity.
	for _, micro := range []int64{10_000, 100_000, 1_000_000, 9_900_000, 123_400_000} {
		s, err := MicroToDecimalString(micro)
		if err != nil {
			t.Fatalf("MicroToDecimalString(%d): %v", micro, err)
		}
		cents := micro / 10_000
		if back := CentsToMicro(cents); back != micro {
			t.Fatalf("round trip %d → %q → %d broke the identity", micro, s, back)
		}
	}
}

// TestCommandKeys: the seam command idempotency identities are pure
// deterministic functions.
func TestCommandKeys(t *testing.T) {
	if got := GrantCreditsCommandKey("weknora-tenant-42", "2026-09"); got != "grant_included_credits:weknora-tenant-42:2026-09" {
		t.Fatalf("GrantCreditsCommandKey = %q", got)
	}
	if GrantCreditsCommandKey("a", "2026-09") != GrantCreditsCommandKey("a", "2026-09") {
		t.Fatal("GrantCreditsCommandKey must be pure")
	}
	if GrantCreditsCommandKey("a", "2026-09") == GrantCreditsCommandKey("a", "2026-10") {
		t.Fatal("distinct periods must derive distinct grant identities")
	}
	if got := EnsureSubscriptionCommandKey("weknora-tenant-42-sub"); got != "ensure_subscription:weknora-tenant-42-sub" {
		t.Fatalf("EnsureSubscriptionCommandKey = %q", got)
	}
}

// TestMonthlyWalletIdentity: the deterministic wallet name and the E3
// recovery metadata keys are pure functions of (tenant, period).
func TestMonthlyWalletIdentity(t *testing.T) {
	if got := MonthlyWalletName(42, "2026-09"); got != "weknora-tenant-42-2026-09" {
		t.Fatalf("MonthlyWalletName = %q, want weknora-tenant-42-2026-09", got)
	}
	if MonthlyWalletName(42, "2026-09") != MonthlyWalletName(42, "2026-09") {
		t.Fatal("MonthlyWalletName must be pure")
	}
	if MonthlyWalletName(42, "2026-09") == MonthlyWalletName(43, "2026-09") {
		t.Fatal("distinct tenants must derive distinct wallet names")
	}
}

// ---- base tier ladder + zero-price publish ----

// TestBaseTierLadder: the default ladder carries base@0 at the bottom and the
// zero-clause accepts zero ONLY there.
func TestBaseTierLadder(t *testing.T) {
	ladder := DefaultPriceLadder()
	if len(ladder) == 0 || ladder[0].TierKey != BasePlanKey || ladder[0].PriceFen != 0 {
		t.Fatalf("DefaultPriceLadder must start with base@0, got %+v", ladder)
	}
	if err := ladder.Validate(); err != nil {
		t.Fatalf("default ladder must be valid: %v", err)
	}
	for i := 1; i < len(ladder); i++ {
		if ladder[i].PriceFen <= 0 || ladder[i-1].PriceFen >= ladder[i].PriceFen {
			t.Fatalf("paid rungs must be positive and strictly ascending, got %+v", ladder)
		}
	}
	// Zero at a non-zero index is rejected.
	if err := (PriceLadder{{TierKey: "lite", PriceFen: 1900}, {TierKey: "pro", PriceFen: 0}}).Validate(); err == nil {
		t.Fatal("a zero price above the bottom rung must be rejected")
	}
	if err := (PriceLadder{
		{TierKey: "base", PriceFen: 0}, {TierKey: "free2", PriceFen: 0}, {TierKey: "pro", PriceFen: 1900},
	}).Validate(); err == nil {
		t.Fatal("two zero rungs must be rejected")
	}
	// Zero at index zero of a non-base-named rung is structurally legal (the
	// publish axes decide the tier vocabulary — fail closed there).
	if err := (PriceLadder{{TierKey: "starter", PriceFen: 0}, {TierKey: "pro", PriceFen: 1900}}).Validate(); err != nil {
		t.Fatalf("zero at index 0 is structurally legal: %v", err)
	}
}

// basePlanVersion is the built-in (base, v1) definition — six-axes clean.
func basePlanVersion() PlanVersion {
	return PlanVersion{
		Key:      BasePlanKey,
		Version:  1,
		Price:    0,
		Monthly:  1_000_000,
		Currency: "CNY",
		Features: map[string]bool{"api_access": true, "advanced_models": false, "priority_support": false},
		Limits:   map[string]int64{"members": 5, "storage_gb": 10, "concurrent_tasks": 2},
	}
}

// TestBaseTierPublishValidation: the six axes accept the zero-priced base
// rung and still reject a paid tier priced zero or the base tier priced off
// its rung.
func TestBaseTierPublishValidation(t *testing.T) {
	vctx := PublishValidationContext{Ladder: DefaultPriceLadder()}
	if err := basePlanVersion().ValidateForPublishValidated(vctx); err != nil {
		t.Fatalf("base priced 0 must pass all six axes, got %v", err)
	}
	paidAtZero := basePlanVersion()
	paidAtZero.Key, paidAtZero.Price = "lite", 0
	if err := paidAtZero.ValidateForPublishValidated(vctx); !errors.Is(err, ErrPublishBasePriceTier) {
		t.Fatalf("a paid tier priced 0 must fail base_price_tier, got %v", err)
	}
	baseAtLitePrice := basePlanVersion()
	baseAtLitePrice.Price = 1900 // a ladder price point, but not the base rung's
	if err := baseAtLitePrice.ValidateForPublishValidated(vctx); !errors.Is(err, ErrPublishBasePriceTier) {
		t.Fatalf("base priced at another rung's point must fail base_price_tier, got %v", err)
	}
	// A paid tier publishes cleanly against a published base@0 neighborhood.
	paid := basePlanVersion()
	paid.Key, paid.Price = "lite", 1900
	if err := paid.ValidateForPublishValidated(PublishValidationContext{
		Ladder: DefaultPriceLadder(), PublishedPrices: map[string]int64{BasePlanKey: 0},
	}); err != nil {
		t.Fatalf("lite@1900 over a published base@0 must pass, got %v", err)
	}
}

// TestPublishPayloadZeroAmount: AmountFen 0 is STRUCTURALLY valid in the
// publish payload — zero legality is decided by the six-axis check, not here.
func TestPublishPayloadZeroAmount(t *testing.T) {
	p := validPublishPayload()
	p.PlanKey, p.PlanCode, p.AmountFen = BasePlanKey, DeterministicPlanCode(BasePlanKey, 1), 0
	if err := p.Validate(); err != nil {
		t.Fatalf("amount_fen 0 must be structurally valid (the six-axis check gates it), got %v", err)
	}
	if p.AmountFen < 0 {
		t.Fatal("test setup")
	}
	negative := p
	negative.AmountFen = -1
	if err := negative.Validate(); err == nil {
		t.Fatal("a negative amount_fen stays structurally invalid")
	}
}

// ---- #86 Task 2: consumption-order encoding (priority classes) ----

// TestMonthlyWalletPriorityYieldsToAgingTopUp: a top-up batch expiring BEFORE
// this period's end must push the monthly wallet's creation-time priority
// ABOVE the top-up class; no such aging batch keeps the monthly class 1.
func TestMonthlyWalletPriorityYieldsToAgingTopUp(t *testing.T) {
	periodEnd := time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)
	if got := MonthlyWalletPriorityFor(nil, periodEnd); got != MonthlyWalletPriority {
		t.Fatalf("no topup: got %d", got)
	}
	aging := []time.Time{time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC)} // before the period end
	if got := MonthlyWalletPriorityFor(aging, periodEnd); got != TopUpWalletPriority+1 {
		t.Fatalf("aging topup: got %d", got)
	}
	later := []time.Time{time.Date(2027, 2, 15, 0, 0, 0, 0, time.UTC)} // after the period end
	if got := MonthlyWalletPriorityFor(later, periodEnd); got != MonthlyWalletPriority {
		t.Fatalf("later topup: got %d", got)
	}
}

// TestWalletRankMixedFamilies (the r1-review High counterexample): an aging
// top-up A (expires before this month's end) + monthly M (period end) + fresh
// top-up B (expires after) coexist — the correct total order is A→M→B; no
// static priority encoding can express it (initials give A=2,B=2,M=3, which
// Lago would consume A→B→M). WalletRank must answer A=1, M=2, B=3.
func TestWalletRankMixedFamilies(t *testing.T) {
	got, err := WalletRank([]WalletRankInput{
		{WalletRef: "B", ExpiresAt: time.Date(2027, 7, 10, 0, 0, 0, 0, time.UTC), GrantedAt: time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)},
		{WalletRef: "M", ExpiresAt: time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC), GrantedAt: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)},
		{WalletRef: "A", ExpiresAt: time.Date(2027, 1, 15, 0, 0, 0, 0, time.UTC), GrantedAt: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got["A"] != 1 || got["M"] != 2 || got["B"] != 3 {
		t.Fatalf("mixed-family rank = %+v, want A=1 M=2 B=3", got)
	}
}

// TestWalletRankSameExpiryEarliestGrant: same-expiry batches break the tie
// by grant time (the spec's "earliest grant" tie-break).
func TestWalletRankSameExpiryEarliestGrant(t *testing.T) {
	exp := time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC)
	got, err := WalletRank([]WalletRankInput{
		{WalletRef: "late", ExpiresAt: exp, GrantedAt: exp.Add(-1 * time.Hour)},
		{WalletRef: "early", ExpiresAt: exp, GrantedAt: exp.Add(-2 * time.Hour)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got["early"] != 1 || got["late"] != 2 {
		t.Fatalf("same-expiry rank = %+v", got)
	}
}

// TestWalletRankFailsClosedOutsidePriorityDomain（OCR84-R1-09）：排名 1..n 无
// 上界，而同一字段在 GrantIncludedCreditsPayload.Validate 被约束为
// [1,MaxWalletPriority]——#86 十二个月 TTL 使活跃钱包累积，超界后 rebalance
// 若照写即写出文档域外优先级（权威拒绝→反复失败落 attention；接受→本地契约
// 静默失真）。重复 WalletRef（map 键，E3 恢复异常路径的同名形态）也必须显式
// 报错，不允许后写静默覆盖前写。
func TestWalletRankFailsClosedOutsidePriorityDomain(t *testing.T) {
	base := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)
	many := make([]WalletRankInput, 0, MaxWalletPriority+1)
	for i := 0; i <= MaxWalletPriority; i++ { // 51 batches → rank 51 out of domain
		many = append(many, WalletRankInput{
			WalletRef: fmt.Sprintf("w_%03d", i), ExpiresAt: base, GrantedAt: base,
		})
	}
	if _, err := WalletRank(many); err == nil {
		t.Fatalf("%d batches exceed the priority domain and must fail closed", len(many))
	}
	// 恰好在界内（50 批）仍是合法排名。
	inDomain := many[:MaxWalletPriority]
	ranks, err := WalletRank(inDomain)
	if err != nil {
		t.Fatalf("exactly %d batches must rank legally, got %v", MaxWalletPriority, err)
	}
	if len(ranks) != MaxWalletPriority || ranks["w_000"] != 1 {
		t.Fatalf("in-domain rank answer = %+v", ranks)
	}
	// 重复 WalletRef：显式报错。
	dup := []WalletRankInput{
		{WalletRef: "same", ExpiresAt: base, GrantedAt: base},
		{WalletRef: "same", ExpiresAt: base.Add(time.Hour), GrantedAt: base},
	}
	if _, err := WalletRank(dup); err == nil {
		t.Fatal("duplicate wallet refs must be refused, not silently overwritten")
	}
}

// TestGrantPayloadRequiresPriorityRange: Priority ∈ [1,50] is required —
// the consumption order must be explicitly encoded at grant time.
func TestGrantPayloadRequiresPriorityRange(t *testing.T) {
	base := GrantIncludedCreditsPayload{
		TenantID:           9,
		ExternalCustomerID: ExternalCustomerID(9),
		Period:             "2099-01",
		CreditsMicro:       1_000_000,
		ExpiresAt:          time.Date(2099, 2, 1, 0, 0, 0, 0, time.UTC),
	}
	for _, p := range []int{0, -1, 51} { // 0 = default — must be explicitly encoded
		bad := base
		bad.Priority = p
		if err := bad.Validate(); err == nil {
			t.Fatalf("priority %d must be rejected", p)
		}
	}
	ok := base
	ok.Priority = 1
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
}
