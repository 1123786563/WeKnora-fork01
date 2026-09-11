package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testUsageStore(t *testing.T) *UsageStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&UsageRow{}, &UsageCurrentRow{}, &OutboxEvent{}); err != nil {
		t.Fatal(err)
	}
	return NewUsageStore(db).WithRates(func(version string) (domain.PriceVersionRates, error) {
		return domain.PriceVersionRates{Version: version, Rates: map[string]domain.DimensionRate{
			domain.DimensionModel: {RateMicro: 1500, Units: 1000},
		}}, nil
	})
}

func repoUsageFact(tenant uint64, call, attempt, status string, revision int64, tokens int64) domain.UsageFact {
	return domain.UsageFact{
		TenantID:     tenant,
		RunID:        "run_" + call,
		DelegationID: "",
		CallID:       call,
		AttemptID:    attempt,
		Funding:      domain.FundingPlatform,
		Service:      "chat",
		PriceVersion: "pv-1",
		Revision:     revision,
		OccurredAt:   time.Now(),
		Dimensions:   map[string]int64{domain.DimensionModel: tokens},
		Status:       status,
	}
}

func usageSettlementCount(t *testing.T, s *UsageStore) int64 {
	t.Helper()
	var n int64
	if err := s.db.Model(&OutboxEvent{}).Where("kind = ?", OutboxKindUsageSettlement).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func usageRowCount(t *testing.T, s *UsageStore) int64 {
	t.Helper()
	var n int64
	if err := s.db.Model(&UsageRow{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func usageCurrentRevision(t *testing.T, s *UsageStore, tenant uint64, call, attempt string) int64 {
	t.Helper()
	var cur UsageCurrentRow
	if err := s.db.Where("tenant_id = ? AND call_id = ? AND attempt_id = ?", tenant, call, attempt).First(&cur).Error; err != nil {
		t.Fatal(err)
	}
	return cur.Revision
}

func TestUsageRecordDuplicateFinalIsIdempotent(t *testing.T) {
	s := testUsageStore(t)
	ctx := context.Background()
	fact := repoUsageFact(7, "call_dup", "attempt_1", domain.UsageStatusFinal, 1, 1000)
	if err := s.Record(ctx, fact); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(ctx, fact); err != nil {
		t.Fatal(err)
	}
	if got := usageRowCount(t, s); got != 1 {
		t.Fatalf("fact rows=%d want 1", got)
	}
	if got := usageSettlementCount(t, s); got != 1 {
		t.Fatalf("settlement events=%d want 1", got)
	}
}

func TestUsageRecordPartialsStoreObservationsFinalContributesOneDelta(t *testing.T) {
	s := testUsageStore(t)
	ctx := context.Background()
	for i, tokens := range []int64{50, 120, 180} {
		if err := s.Record(ctx, repoUsageFact(7, "call_stream", "attempt_1", domain.UsageStatusPartial, int64(i+1), tokens)); err != nil {
			t.Fatal(err)
		}
	}
	if got := usageSettlementCount(t, s); got != 0 {
		t.Fatalf("partial emitted settlement: %d", got)
	}
	final := repoUsageFact(7, "call_stream", "attempt_1", domain.UsageStatusFinal, 4, 200)
	if err := s.Record(ctx, final); err != nil {
		t.Fatal(err)
	}
	if got := usageSettlementCount(t, s); got != 1 {
		t.Fatalf("settlement events=%d want exactly one final delta", got)
	}
	var ev OutboxEvent
	if err := s.db.Where("kind = ?", OutboxKindUsageSettlement).First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	var payload usageSettlementPayload
	if err := json.Unmarshal([]byte(ev.PayloadJSON), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ChargeMicro != 300 {
		t.Fatalf("charge_micro=%d want 300 for 200 tokens at 1500/1000", payload.ChargeMicro)
	}
}

func TestUsageRecordParentAggregateDisplayOnlyNeverSettles(t *testing.T) {
	s := testUsageStore(t)
	ctx := context.Background()
	parent := repoUsageFact(7, "call_parent", "attempt_1", domain.UsageStatusDisplayOnly, 1, 5000)
	parent.Service = "agent_run"
	if err := s.Record(ctx, parent); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(ctx, repoUsageFact(7, "call_child", "attempt_1", domain.UsageStatusFinal, 1, 1000)); err != nil {
		t.Fatal(err)
	}
	if got := usageRowCount(t, s); got != 2 {
		t.Fatalf("fact rows=%d want parent+child", got)
	}
	if got := usageSettlementCount(t, s); got != 1 {
		t.Fatalf("settlement events=%d want child only, no parent double count", got)
	}
}

func TestUsageRecordRetryRecordsNewAttemptSeparately(t *testing.T) {
	s := testUsageStore(t)
	ctx := context.Background()
	if err := s.Record(ctx, repoUsageFact(7, "call_retry", "attempt_1", domain.UsageStatusFinal, 1, 1000)); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(ctx, repoUsageFact(7, "call_retry", "attempt_2", domain.UsageStatusFinal, 1, 900)); err != nil {
		t.Fatal(err)
	}
	if got := usageRowCount(t, s); got != 2 {
		t.Fatalf("fact rows=%d want one per attempt", got)
	}
	if got := usageSettlementCount(t, s); got != 2 {
		t.Fatalf("settlement events=%d want one per attempt", got)
	}
	if usageCurrentRevision(t, s, 7, "call_retry", "attempt_1") != 1 ||
		usageCurrentRevision(t, s, 7, "call_retry", "attempt_2") != 1 {
		t.Fatal("current pointers must exist per attempt")
	}
}

func TestUsageRecordSameRevisionDifferentContentConflicts(t *testing.T) {
	s := testUsageStore(t)
	ctx := context.Background()
	if err := s.Record(ctx, repoUsageFact(7, "call_conflict", "attempt_1", domain.UsageStatusFinal, 1, 1000)); err != nil {
		t.Fatal(err)
	}
	conflicting := repoUsageFact(7, "call_conflict", "attempt_1", domain.UsageStatusFinal, 1, 2000)
	if err := s.Record(ctx, conflicting); !errors.Is(err, ErrUsageRevisionConflict) {
		t.Fatalf("got %v want ErrUsageRevisionConflict", err)
	}
	if got := usageRowCount(t, s); got != 1 {
		t.Fatalf("conflict overwrote fact: rows=%d", got)
	}
	if got := usageSettlementCount(t, s); got != 1 {
		t.Fatalf("settlement events=%d want original only", got)
	}
}

func TestUsageRecordLateCorrectionNewRevisionSupersedesCurrent(t *testing.T) {
	s := testUsageStore(t)
	ctx := context.Background()
	if err := s.Record(ctx, repoUsageFact(7, "call_fix", "attempt_1", domain.UsageStatusFinal, 1, 1000)); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(ctx, repoUsageFact(7, "call_fix", "attempt_1", domain.UsageStatusFinal, 2, 3000)); err != nil {
		t.Fatal(err)
	}
	if got := usageCurrentRevision(t, s, 7, "call_fix", "attempt_1"); got != 2 {
		t.Fatalf("current revision=%d want 2", got)
	}
	if got := usageRowCount(t, s); got != 2 {
		t.Fatalf("fact rows=%d want both revisions kept", got)
	}
	// Each final revision emits its own settlement event; U03 nets them
	// against the current-version pointer.
	if got := usageSettlementCount(t, s); got != 2 {
		t.Fatalf("settlement events=%d want one per final revision", got)
	}
}

func TestUsageRecordUnknownStatusEmitsNoZeroSettlement(t *testing.T) {
	s := testUsageStore(t)
	ctx := context.Background()
	if err := s.Record(ctx, repoUsageFact(7, "call_unknown", "attempt_1", domain.UsageStatusUnknown, 1, 1000)); err != nil {
		t.Fatal(err)
	}
	if got := usageRowCount(t, s); got != 1 {
		t.Fatalf("fact rows=%d want observation stored", got)
	}
	if got := usageSettlementCount(t, s); got != 0 {
		t.Fatalf("settlement events=%d want none for status=unknown", got)
	}
}

func TestUsageRecordFinalWithoutRatesIsRejected(t *testing.T) {
	s := testUsageStore(t)
	ctx := context.Background()
	bare := NewUsageStore(s.db)
	if err := bare.Record(ctx, repoUsageFact(7, "call_norates", "attempt_1", domain.UsageStatusFinal, 1, 1000)); !errors.Is(err, ErrUsageRatesUnavailable) {
		t.Fatalf("got %v want ErrUsageRatesUnavailable", err)
	}
	if got := usageSettlementCount(t, s); got != 0 {
		t.Fatalf("settlement events=%d want none", got)
	}
}
