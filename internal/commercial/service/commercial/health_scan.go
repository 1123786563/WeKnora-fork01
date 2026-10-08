package commercial

import (
	"context"
	"errors"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"

	"gorm.io/gorm"
)

// BillingHealthService scans the operator-facing billing observability
// faces (#103 / Lago 31, AC3): pricing-lag queue depth (the #90
// settlement face), webhook inbox backlog (dead receipts — deliveries
// whose projection audit never landed), negative available balances, and
// commercial timeouts (settlements parked in the indeterminate state).
// Every face is a count plus the oldest age in Go-computed seconds — no
// dialect-specific SQL time math — the shape any alerting pipe consumes.
// The Prometheus exporter itself is a production-deployment concern gated
// on the AGPL production gate
// (docs/upstream-parity/lago-agpl-production-gate.md).
type BillingHealthService struct {
	db *gorm.DB
}

// NewBillingHealthService builds the scan over the commercial database.
func NewBillingHealthService(db *gorm.DB) (*BillingHealthService, error) {
	if db == nil {
		return nil, errors.New("billing health service requires a database")
	}
	return &BillingHealthService{db: db}, nil
}

// BillingHealthFace is one observability category's snapshot. Category is
// the closed set pricing_lag | commercial_timeout | webhook_backlog |
// negative_balance; OldestSeconds is the oldest member's age (0 when
// empty; negative_balance is a stock, not an event, so it carries 0).
type BillingHealthFace struct {
	Category      string
	Count         int64
	OldestSeconds int64
}

// BillingHealth is the full scan answer. Ready=false iff any face is
// non-empty; per-face alert thresholds ride the alerting pipe, not this
// scan (the #90 thresholds are surfaced for the consumer).
type BillingHealth struct {
	ScannedAt    time.Time
	Faces        []BillingHealthFace
	LagPauseSecs int64
	LagAlertSecs int64
	Ready        bool
}

// scanFace runs one count + oldest-timestamp face. table absent → empty
// face (the stand-down posture of the closure/lag guards).
func (s *BillingHealthService) scanFace(ctx context.Context, category, table, where, oldestColumn string, now time.Time) (BillingHealthFace, error) {
	face := BillingHealthFace{Category: category}
	if !s.db.Migrator().HasTable(table) {
		return face, nil
	}
	if err := s.db.WithContext(ctx).Table(table).Where(where).Count(&face.Count).Error; err != nil {
		return face, err
	}
	if face.Count == 0 || oldestColumn == "" {
		return face, nil
	}
	var oldest *time.Time
	if err := s.db.WithContext(ctx).Table(table).Where(where).
		Select(oldestColumn).Order(oldestColumn + " ASC").Limit(1).Scan(&oldest).Error; err != nil {
		return face, err
	}
	if oldest != nil {
		face.OldestSeconds = int64(now.Sub(*oldest).Seconds())
		if face.OldestSeconds < 0 {
			face.OldestSeconds = 0
		}
	}
	return face, nil
}

// ScanBillingHealth runs the four faces.
func (s *BillingHealthService) ScanBillingHealth(ctx context.Context) (BillingHealth, error) {
	now := time.Now().UTC()
	out := BillingHealth{
		ScannedAt:    now,
		LagPauseSecs: int64(domain.SettlementLagPauseThreshold.Seconds()),
		LagAlertSecs: int64(domain.SettlementLagAlertThreshold.Seconds()),
		Ready:        true,
	}
	type spec struct{ category, table, where, oldest string }
	for _, f := range []spec{
		{"pricing_lag", "commercial_settlement_records", "state NOT IN ('confirmed', 'unknown')", "updated_at"},
		{"commercial_timeout", "commercial_settlement_records", "state = 'unknown'", "updated_at"},
		{"webhook_backlog", "commercial_webhook_inbox",
			"NOT EXISTS (SELECT 1 FROM commercial_projection_audit a WHERE a.source = 'webhook' AND a.kind = commercial_webhook_inbox.kind AND a.external_id = commercial_webhook_inbox.external_id)",
			"received_at"},
		{"negative_balance", "commercial_budget_accounts",
			"verified_micro - unreflected_micro - held_micro - refund_locked_micro < 0", ""},
	} {
		face, err := s.scanFace(ctx, f.category, f.table, f.where, f.oldest, now)
		if err != nil {
			return BillingHealth{}, err
		}
		out.Faces = append(out.Faces, face)
		if face.Count > 0 {
			out.Ready = false
		}
	}
	return out, nil
}
