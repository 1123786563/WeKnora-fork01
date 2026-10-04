// #103 / Lago 31 — the billing health scan: four closed-token faces over
// the local projections (pricing lag, commercial timeouts, webhook dead
// receipts, negative balances), counts with oldest ages computed in Go,
// empty faces where a deployment lacks that traffic's tables, and
// ready=false iff any face is non-empty.
package commercial

import (
	"context"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newHealthScanDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	t.Cleanup(func() {
		if s, err := db.DB(); err == nil {
			_ = s.Close()
		}
	})
	return db
}

// TestScanBillingHealthSurfacesAllFourFaces seeds one member per face and
// one healthy member per table, then asserts each face counts exactly its
// member with a sane oldest age, and the answer is not ready.
func TestScanBillingHealthSurfacesAllFourFaces(t *testing.T) {
	db := newHealthScanDB(t)
	if err := db.AutoMigrate(&settlementRecordScanMirror{}, &repocommercial.WebhookInboxRow{},
		&repocommercial.ProjectionAuditRow{}, &repocommercial.BudgetAccountRow{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	old := time.Now().UTC().Add(-20 * time.Minute)

	// pricing_lag: one unconfirmed + old; one confirmed (healthy).
	if err := db.Create(&[]settlementRecordScanMirror{
		{Key: "lagged", TenantID: 7, State: domain.SettlementStateDispatched, UpdatedAt: old},
		{Key: "done", TenantID: 7, State: domain.SettlementStateConfirmed, UpdatedAt: old},
	}).Error; err != nil {
		t.Fatal(err)
	}
	// commercial_timeout: one parked in the indeterminate state.
	if err := db.Create(&settlementRecordScanMirror{
		Key: "timed-out", TenantID: 8, State: domain.SettlementStateUnknown, UpdatedAt: old,
	}).Error; err != nil {
		t.Fatal(err)
	}
	// webhook_backlog: one dead receipt (no audit) + one audited (healthy).
	if err := db.Create(&[]repocommercial.WebhookInboxRow{
		{Provider: "lago", EventID: "e1", Kind: "invoice.created", ExternalID: "inv-dead", ReceivedAt: old},
		{Provider: "lago", EventID: "e2", Kind: "invoice.created", ExternalID: "inv-live", ReceivedAt: old},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repocommercial.ProjectionAuditRow{
		Source: "webhook", Action: "received", Kind: "invoice.created", ExternalID: "inv-live", CreatedAt: old,
	}).Error; err != nil {
		t.Fatal(err)
	}
	// negative_balance: one account whose available credits are negative.
	if err := db.Create(&repocommercial.BudgetAccountRow{
		TenantID: 9, VerifiedMicro: 100, UnreflectedMicro: 300, Watermark: "w", Version: 1, VerifiedUntil: old,
	}).Error; err != nil {
		t.Fatal(err)
	}

	svc, err := NewBillingHealthService(db)
	if err != nil {
		t.Fatal(err)
	}
	health, err := svc.ScanBillingHealth(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if health.Ready {
		t.Fatal("a scan with non-empty faces must not answer ready")
	}
	if health.LagPauseSecs != 300 || health.LagAlertSecs != 900 {
		t.Fatalf("the #90 thresholds must ride the answer: %+v", health)
	}
	got := map[string]BillingHealthFace{}
	for _, f := range health.Faces {
		got[f.Category] = f
	}
	for _, category := range []string{"pricing_lag", "commercial_timeout", "webhook_backlog", "negative_balance"} {
		if got[category].Count != 1 {
			t.Fatalf("%s face must count exactly its member, got %+v", category, got[category])
		}
		if category != "negative_balance" && got[category].OldestSeconds < 60 {
			t.Fatalf("%s face must carry the oldest age, got %+v", category, got[category])
		}
	}
	if got["negative_balance"].OldestSeconds != 0 {
		t.Fatalf("a stock face carries no age, got %+v", got["negative_balance"])
	}
}

// TestScanBillingHealthReadyWhenEmptyAndStandsDownWithoutTables: a
// projection database with none of the traffic tables answers four EMPTY
// faces and ready=true (the stand-down posture).
func TestScanBillingHealthReadyWhenEmptyAndStandsDownWithoutTables(t *testing.T) {
	db := newHealthScanDB(t)
	svc, err := NewBillingHealthService(db)
	if err != nil {
		t.Fatal(err)
	}
	health, err := svc.ScanBillingHealth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !health.Ready || len(health.Faces) != 4 {
		t.Fatalf("empty deployment must answer ready with four faces: %+v", health)
	}
	for _, f := range health.Faces {
		if f.Count != 0 || f.OldestSeconds != 0 {
			t.Fatalf("face %+v must be empty", f)
		}
	}
}

// settlementRecordScanMirror mirrors the settlement-store row the scan
// reads via raw predicates (the owning type lives in this package's
// settlement service; only the table shape is needed).
type settlementRecordScanMirror struct {
	Key       string    `gorm:"primaryKey;column:key"`
	TenantID  uint64    `gorm:"column:tenant_id;index"`
	RunID     string    `gorm:"column:run_id;not null;default:''"`
	State     string    `gorm:"column:state;not null;default:dispatched"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (settlementRecordScanMirror) TableName() string { return "commercial_settlement_records" }
