package commercial

import (
	"testing"
	"time"
)

func TestProrateDoesNotRefillMonth(t *testing.T) {
	v, err := Prorate(9000000, 10, 30, false)
	if err != nil || v != 3000000 {
		t.Fatalf("%d %v", v, err)
	}
	cents, err := Prorate(101, 1, 2, true)
	if err != nil || cents != 51 {
		t.Fatalf("%d %v", cents, err)
	}
}

func TestProrateRejectsInvalidIntervals(t *testing.T) {
	for _, tc := range [][3]int64{
		{-1, 10, 30},   // negative delta
		{100, -1, 30},  // negative remaining
		{100, 31, 30},  // remaining beyond the paid period
		{100, 10, 0},   // zero duration
		{100, 10, -30}, // negative duration
	} {
		v, err := Prorate(tc[0], tc[1], tc[2], true)
		if err == nil || v != 0 {
			t.Fatalf("Prorate(%d,%d,%d) = %d, %v; want invalid_quote_interval", tc[0], tc[1], tc[2], v, err)
		}
	}
}

func TestProrateMonthEndBoundary(t *testing.T) {
	// A fully remaining period charges the whole delta: the month-end
	// boundary must not manufacture an extra day.
	full, err := Prorate(3100000, 31, 31, true)
	if err != nil || full != 3100000 {
		t.Fatalf("%d %v", full, err)
	}
	// Jan 31 clamps to Feb 28, so remaining never exceeds the paid duration.
	jan := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	feb := MonthBoundary(jan, 1)
	if feb.Month() != time.February || feb.Day() != 28 {
		t.Fatalf("clamped boundary %v", feb)
	}
	edge, err := Prorate(2800000, int64(feb.Day()), 28, false)
	if err != nil || edge != 2800000 {
		t.Fatalf("%d %v", edge, err)
	}
	// One unit left in the period is still a chargeable interval.
	tail, err := Prorate(3000000, 1, 30, true)
	if err != nil || tail != 100000 {
		t.Fatalf("%d %v", tail, err)
	}
}

func TestQuoteAmountCurrentMonthUpgrade(t *testing.T) {
	// 当前月补差: 30-day month, 10 days remaining on a 9,000,000 fen delta.
	got, err := QuoteAmount([]Segment{{Delta: 9000000, Remaining: 10, Duration: 30}}, false)
	if err != nil || got != 3000000 {
		t.Fatalf("%d %v", got, err)
	}
	// Truncation applies unless roundUp is requested.
	down, err := QuoteAmount([]Segment{{Delta: 101, Remaining: 1, Duration: 3}}, false)
	if err != nil || down != 33 {
		t.Fatalf("%d %v", down, err)
	}
}

func TestQuoteAmountRoundsOnceAfterSummingSegments(t *testing.T) {
	// Two half-period segments of 101 fen are exactly 101 in total; rounding
	// each segment up first would overcharge by one fen (51+51=102).
	exact, err := QuoteAmount([]Segment{
		{Delta: 101, Remaining: 1, Duration: 2},
		{Delta: 101, Remaining: 1, Duration: 2},
	}, true)
	if err != nil || exact != 101 {
		t.Fatalf("%d %v", exact, err)
	}
	// Three thirds of 100 fen stay exactly 100, not 34*3=102.
	thirds, err := QuoteAmount([]Segment{
		{Delta: 100, Remaining: 1, Duration: 3},
		{Delta: 100, Remaining: 1, Duration: 3},
		{Delta: 100, Remaining: 1, Duration: 3},
	}, true)
	if err != nil || thirds != 100 {
		t.Fatalf("%d %v", thirds, err)
	}
}

func TestQuoteConsecutiveUpgrades(t *testing.T) {
	// 连续升级: A→B then B→C in the same paid month. The second quote prices
	// C against B, the version actually paid for, never re-charging A→B.
	amount, err := QuoteAmount([]Segment{{Delta: 7000000, Remaining: 10, Duration: 30}}, false)
	if err != nil || amount != 2333333 {
		t.Fatalf("%d %v", amount, err)
	}
	// Stacking both upgrades into one rational invoice keeps the sum exact:
	// (5000000+7000000)*10/30 = 4000000 exactly.
	stacked, err := QuoteAmount([]Segment{
		{Delta: 5000000, Remaining: 10, Duration: 30},
		{Delta: 7000000, Remaining: 10, Duration: 30},
	}, true)
	if err != nil || stacked != 4000000 {
		t.Fatalf("%d %v", stacked, err)
	}
}

func TestQuoteFuturePaidIntervalNotChargedNow(t *testing.T) {
	// 未来已付区间: a delta that only starts next month contributes nothing
	// to the current month's invoice.
	now, err := QuoteAmount([]Segment{{Delta: 9000000, Remaining: 0, Duration: 30}}, true)
	if err != nil || now != 0 {
		t.Fatalf("%d %v", now, err)
	}
	// ...and it is charged in full inside its own prepaid period.
	future, err := QuoteAmount([]Segment{{Delta: 9000000, Remaining: 30, Duration: 30}}, true)
	if err != nil || future != 9000000 {
		t.Fatalf("%d %v", future, err)
	}
}

func TestQuoteAmountRejectsInvalidAndOverflow(t *testing.T) {
	if _, err := QuoteAmount(nil, true); err == nil {
		t.Fatal("empty segments must be rejected")
	}
	if _, err := QuoteAmount([]Segment{{Delta: 100, Remaining: 31, Duration: 30}}, false); err == nil {
		t.Fatal("remaining beyond duration must be rejected")
	}
	// 溢出: the rational sum exceeds int64 fen.
	if v, err := QuoteAmount([]Segment{
		{Delta: 9223372036854775807, Remaining: 30, Duration: 30},
		{Delta: 9223372036854775807, Remaining: 30, Duration: 30},
	}, false); err == nil || v != 0 {
		t.Fatalf("expected quote_overflow, got %d %v", v, err)
	}
}

func TestQuoteValidateForUse(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	q := Quote{
		ID: "q_1", TenantID: 7, SubscriptionVersion: 3,
		Amount: 3000000, CreditDelta: Credits(1200000),
		ExpiresAt: now.Add(time.Hour),
	}
	if err := q.ValidateForUse(3, now); err != nil {
		t.Fatal(err)
	}
	// 过期报价拒绝: an expired quote never settles.
	expired := q
	expired.ExpiresAt = now.Add(-time.Minute)
	if err := expired.ValidateForUse(3, now); err != ErrQuoteExpired {
		t.Fatalf("got %v", err)
	}
	// 并发变更冲突: the subscription moved on after the quote was cut.
	if err := q.ValidateForUse(4, now); err != ErrQuoteVersionConflict {
		t.Fatalf("got %v", err)
	}
}

func TestCatalogPlanStateValidation(t *testing.T) {
	for _, state := range []string{PlanStateDraft, PlanStatePublishing, PlanStatePublished, PlanStateArchived} {
		if err := ValidatePlanState(state); err != nil {
			t.Fatalf("state %q rejected: %v", state, err)
		}
	}
	for _, state := range []string{"", "live", "published ", "Draft"} {
		if err := ValidatePlanState(state); err == nil {
			t.Fatalf("state %q accepted", state)
		}
	}
}

func TestCatalogPublishRequiresKnownPriceAndBaseTier(t *testing.T) {
	base := PlanVersion{
		Key: "pro", Version: 2, Price: 990000,
		Monthly:  Credits(30000000),
		Features: map[string]bool{"priority_support": true},
		Limits:   map[string]int64{"seats": 0},
	}
	if err := base.ValidateForPublish(); err != nil {
		t.Fatal(err)
	}
	// 未知价格: an unconfigured price point must not publish.
	unknown := base
	unknown.Price = 0
	if err := unknown.ValidateForPublish(); err != ErrUnknownPrice {
		t.Fatalf("got %v", err)
	}
	// 基础档配置缺失: no base credit tier must not publish.
	noTier := base
	noTier.Monthly = 0
	if err := noTier.ValidateForPublish(); err != ErrMissingBaseTier {
		t.Fatalf("got %v", err)
	}
	if err := (PlanVersion{}).ValidateForPublish(); err != ErrMissingBaseTier {
		t.Fatalf("got %v", err)
	}
}

func TestCatalogZeroLimitDistinctFromUnlimited(t *testing.T) {
	p := PlanVersion{Limits: map[string]int64{"seats": 0}}
	// A configured 0 is a hard zero...
	if v, ok := p.Limit("seats"); !ok || v != 0 {
		t.Fatalf("configured hard zero lost: %d %v", v, ok)
	}
	// ...while an absent key means unlimited, never zero.
	if _, ok := p.Limit("projects"); ok {
		t.Fatal("absent key must mean unlimited, not zero")
	}
	// Identity configured but no price point: still unpublishable.
	noPrice := PlanVersion{Key: "pro", Version: 1, Monthly: Credits(1000000), Limits: p.Limits}
	if err := noPrice.ValidateForPublish(); err != ErrUnknownPrice {
		t.Fatalf("got %v", err)
	}
	// Negative limits are nonsensical and rejected.
	neg := PlanVersion{Key: "pro", Version: 1, Price: 100, Monthly: 1, Limits: map[string]int64{"seats": -1}}
	if err := neg.ValidateForPublish(); err != ErrInvalidPlanLimit {
		t.Fatalf("got %v", err)
	}
}
