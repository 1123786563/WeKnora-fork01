package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidUsageRow       = errors.New("invalid_usage_row")
	ErrUsageRevisionConflict = errors.New("usage_revision_conflict")
	ErrUsageRatesUnavailable = errors.New("usage_rates_unavailable")
)

// OutboxKindUsageSettlement is the event kind emitted once per final usage
// revision; U03 nets the events of a call against its current-version
// pointer.
const OutboxKindUsageSettlement = "usage_settlement"

// UsageRow stores one physical usage fact revision. (tenant_id, call_id,
// attempt_id, revision) is UNIQUE: replaying the same revision with the
// same content is an idempotent no-op, while the same revision with
// different content is a conflict — corrections must arrive as NEW
// revisions.
type UsageRow struct {
	ID             string    `gorm:"primaryKey;column:id"`
	TenantID       uint64    `gorm:"column:tenant_id;not null;uniqueIndex:uq_commercial_usage_revision,priority:1;index:idx_commercial_usage_attempt,priority:1"`
	CallID         string    `gorm:"column:call_id;not null;uniqueIndex:uq_commercial_usage_revision,priority:2;index:idx_commercial_usage_attempt,priority:2"`
	AttemptID      string    `gorm:"column:attempt_id;not null;uniqueIndex:uq_commercial_usage_revision,priority:3;index:idx_commercial_usage_attempt,priority:3"`
	Revision       int64     `gorm:"column:revision;not null;uniqueIndex:uq_commercial_usage_revision,priority:4"`
	RunID          string    `gorm:"column:run_id;not null;default:''"`
	DelegationID   string    `gorm:"column:delegation_id;not null;default:''"`
	Funding        string    `gorm:"column:funding;not null"`
	Service        string    `gorm:"column:service;not null"`
	PriceVersion   string    `gorm:"column:price_version;not null"`
	OccurredAt     time.Time `gorm:"column:occurred_at;not null"`
	DimensionsJSON string    `gorm:"column:dimensions_json;not null"`
	Status         string    `gorm:"column:status;not null"`
	ChargeMicro    int64     `gorm:"column:charge_micro;not null;default:0"`
}

func (UsageRow) TableName() string { return "commercial_usage_facts" }

// UsageCurrentRow is the current-version pointer for one attempt of one
// call: (tenant_id, call_id, attempt_id) PRIMARY KEY pointing at the
// revision that currently represents the attempt. A late correction moves
// the pointer to its new revision.
type UsageCurrentRow struct {
	TenantID  uint64    `gorm:"column:tenant_id;primaryKey;autoIncrement:false"`
	CallID    string    `gorm:"column:call_id;primaryKey"`
	AttemptID string    `gorm:"column:attempt_id;primaryKey"`
	Revision  int64     `gorm:"column:revision;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (UsageCurrentRow) TableName() string { return "commercial_usage_current" }

// usageSettlementPayload is the outbox payload of one final usage revision.
// ChargeMicro was rounded exactly once, at the end of the physical call, by
// the price version's fixed-point rates (see PriceVersionRates).
type usageSettlementPayload struct {
	TenantID     uint64           `json:"tenant_id"`
	RunID        string           `json:"run_id,omitempty"`
	DelegationID string           `json:"delegation_id,omitempty"`
	CallID       string           `json:"call_id"`
	AttemptID    string           `json:"attempt_id"`
	Funding      string           `json:"funding"`
	Service      string           `json:"service"`
	PriceVersion string           `json:"price_version"`
	Revision     int64            `json:"revision"`
	ChargeMicro  int64            `json:"charge_micro"`
	Dimensions   map[string]int64 `json:"dimensions"`
}

// RateResolver loads the immutable fixed-point rates of a price version.
// A final fact without resolvable rates is rejected, never zero-charged.
type RateResolver func(priceVersion string) (domain.PriceVersionRates, error)

// UsageStore records physical usage facts and, in the same transaction,
// the settlement outbox event for final revisions.
type UsageStore struct {
	db    *gorm.DB
	rates RateResolver
}

func NewUsageStore(db *gorm.DB) *UsageStore { return &UsageStore{db: db} }

// WithRates attaches the price-version rate resolver.
func (s *UsageStore) WithRates(r RateResolver) *UsageStore { return &UsageStore{db: s.db, rates: r} }

// Record persists one usage fact revision atomically with its settlement
// outbox event (C01 outbox pattern). Semantics:
//   - identical replay of (tenant, call, attempt, revision) → idempotent success;
//   - same revision with different content → ErrUsageRevisionConflict;
//   - display_only (parent aggregates), partial (streaming observations)
//     and unknown statuses are stored but NEVER generate a settlement —
//     status=unknown does not emit a zero settlement either;
//   - final contributes exactly ONE delta, rounded once at the end of the
//     physical call;
//   - every recorded revision moves the current-version pointer, so a late
//     correction arriving as a new revision supersedes the current version.
func (s *UsageStore) Record(ctx context.Context, fact domain.UsageFact) error {
	if err := fact.Validate(); err != nil {
		return err
	}
	dims, err := json.Marshal(fact.Dimensions)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidUsageRow, err)
	}
	row := UsageRow{
		ID:             fmt.Sprintf("%d:%s:%s:%d", fact.TenantID, fact.CallID, fact.AttemptID, fact.Revision),
		TenantID:       fact.TenantID,
		CallID:         fact.CallID,
		AttemptID:      fact.AttemptID,
		Revision:       fact.Revision,
		RunID:          fact.RunID,
		DelegationID:   fact.DelegationID,
		Funding:        fact.Funding,
		Service:        fact.Service,
		PriceVersion:   fact.PriceVersion,
		OccurredAt:     fact.OccurredAt,
		DimensionsJSON: string(dims),
		Status:         fact.Status,
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing UsageRow
		err := tx.Where("tenant_id = ? AND call_id = ? AND attempt_id = ? AND revision = ?",
			row.TenantID, row.CallID, row.AttemptID, row.Revision).First(&existing).Error
		if err == nil {
			if existing.contentEqual(row) {
				return nil // identical replay: idempotent success
			}
			return ErrUsageRevisionConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// Only a final fact settles. display_only/partial/unknown stay
		// observations — never a settlement, never a zero settlement.
		settles := fact.Status == domain.UsageStatusFinal
		if settles {
			charge, err := s.chargeFor(fact)
			if err != nil {
				return err
			}
			row.ChargeMicro = int64(charge)
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}

		// Current-version pointer moves to the newest revision seen.
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "call_id"}, {Name: "attempt_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"revision", "updated_at"}),
		}).Create(&UsageCurrentRow{
			TenantID:  row.TenantID,
			CallID:    row.CallID,
			AttemptID: row.AttemptID,
			Revision:  row.Revision,
			UpdatedAt: time.Now(),
		}).Error; err != nil {
			return err
		}

		if !settles {
			return nil
		}
		payload, err := json.Marshal(usageSettlementPayload{
			TenantID:     fact.TenantID,
			RunID:        fact.RunID,
			DelegationID: fact.DelegationID,
			CallID:       fact.CallID,
			AttemptID:    fact.AttemptID,
			Funding:      fact.Funding,
			Service:      fact.Service,
			PriceVersion: fact.PriceVersion,
			Revision:     fact.Revision,
			ChargeMicro:  row.ChargeMicro,
			Dimensions:   fact.Dimensions,
		})
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidOutboxEvent, err)
		}
		if err := insertOutboxEvent(tx, OutboxEvent{
			EventKey:    fmt.Sprintf("%s:%d:%s:%s:%d", OutboxKindUsageSettlement, fact.TenantID, fact.CallID, fact.AttemptID, fact.Revision),
			TenantID:    fact.TenantID,
			Kind:        OutboxKindUsageSettlement,
			PayloadJSON: string(payload),
		}); err != nil {
			if errors.Is(err, ErrOutboxEventDuplicate) {
				return nil // the fact is already settled; replay is success
			}
			return err
		}
		return nil
	})
}

// contentEqual compares the logical content of two revisions of the same
// attempt. OccurredAt is metadata, not content.
func (r UsageRow) contentEqual(other UsageRow) bool {
	return r.RunID == other.RunID &&
		r.DelegationID == other.DelegationID &&
		r.Funding == other.Funding &&
		r.Service == other.Service &&
		r.PriceVersion == other.PriceVersion &&
		r.Status == other.Status &&
		r.DimensionsJSON == other.DimensionsJSON
}

// chargeFor prices the fact with its immutable price version's fixed-point
// rates. Missing rates are an error: an unpriced final call is rejected
// rather than silently recorded as free.
func (s *UsageStore) chargeFor(fact domain.UsageFact) (domain.Credits, error) {
	if s.rates == nil {
		return 0, ErrUsageRatesUnavailable
	}
	rates, err := s.rates(fact.PriceVersion)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrUsageRatesUnavailable, err)
	}
	charge, err := rates.ChargeForCall(fact)
	if err != nil {
		return 0, err
	}
	if charge < 0 {
		return 0, ErrInvalidUsageRow
	}
	return charge, nil
}
