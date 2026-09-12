package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
)

var (
	ErrInvalidPlanRow         = errors.New("invalid_plan_row")
	ErrPlanNotFound           = errors.New("plan_not_found")
	ErrPublishedPlanImmutable = errors.New("published_plan_immutable")
	ErrInvalidQuoteRow        = errors.New("invalid_quote_row")
	ErrQuoteNotFound          = errors.New("quote_not_found")
	ErrQuoteExpired           = errors.New("quote_expired")
	ErrQuoteVersionConflict   = errors.New("quote_version_conflict")
	ErrQuoteAlreadyUsed       = errors.New("quote_already_used")
	ErrInvalidQuoteUsage      = errors.New("invalid_quote_usage")
)

// PlanRow stores an immutable versioned plan definition. The pair
// (plan_key, version) is the primary key; definition_json holds the encoded
// commercial.PlanVersion and external_id ties the row to the external billing
// catalog. Once state reaches published the definition is frozen.
type PlanRow struct {
	PlanKey        string `gorm:"primaryKey;column:plan_key"`
	Version        int64  `gorm:"primaryKey;column:version"`
	DefinitionJSON string `gorm:"column:definition_json;not null"`
	ExternalID     string `gorm:"column:external_id;uniqueIndex;not null"`
	State          string `gorm:"column:state;not null"`
}

func (PlanRow) TableName() string { return "commercial_plan_catalog" }

// QuoteRow stores a quoted remaining paid interval: the tenant, the
// subscription version it was cut against, the quote snapshot, its expiry,
// and the order that consumed it (unique once set).
type QuoteRow struct {
	ID                  string    `gorm:"primaryKey;column:id"`
	TenantID            uint64    `gorm:"column:tenant_id;not null"`
	SubscriptionVersion int64     `gorm:"column:subscription_version;not null"`
	SnapshotJSON        string    `gorm:"column:snapshot_json;not null"`
	ExpiresAt           time.Time `gorm:"column:expires_at;not null"`
	UsedOrderID         *string   `gorm:"column:used_order_id;uniqueIndex"`
}

func (QuoteRow) TableName() string { return "commercial_quotes" }

type CatalogStore struct{ db *gorm.DB }

func NewCatalogStore(db *gorm.DB) *CatalogStore { return &CatalogStore{db: db} }

// SaveDefinition inserts or updates a non-published definition. Published
// definitions are immutable: any UPDATE touching a published row is rejected
// so a price already sold can never be rewritten in place.
func (s *CatalogStore) SaveDefinition(ctx context.Context, row PlanRow) error {
	if row.PlanKey == "" || row.Version <= 0 || row.DefinitionJSON == "" || row.ExternalID == "" {
		return ErrInvalidPlanRow
	}
	if err := domain.ValidatePlanState(row.State); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing PlanRow
		err := tx.Where("plan_key = ? AND version = ?", row.PlanKey, row.Version).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&row).Error
		}
		if err != nil {
			return err
		}
		if existing.State == domain.PlanStatePublished {
			return ErrPublishedPlanImmutable
		}
		return tx.Model(&PlanRow{}).
			Where("plan_key = ? AND version = ?", row.PlanKey, row.Version).
			Updates(map[string]interface{}{
				"definition_json": row.DefinitionJSON,
				"external_id":     row.ExternalID,
				"state":           row.State,
			}).Error
	})
}

// Publish flips a draft or publishing row to published after validating the
// definition: an unknown price point or a missing base tier never publishes.
func (s *CatalogStore) Publish(ctx context.Context, planKey string, version int64) (PlanRow, error) {
	var out PlanRow
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plan_key = ? AND version = ?", planKey, version).First(&out).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPlanNotFound
			}
			return err
		}
		if out.State == domain.PlanStatePublished {
			return nil // idempotent: definition already frozen
		}
		if out.State != domain.PlanStateDraft && out.State != domain.PlanStatePublishing {
			return domain.ErrInvalidPlanState
		}
		var plan domain.PlanVersion
		if err := json.Unmarshal([]byte(out.DefinitionJSON), &plan); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidPlanRow, err)
		}
		if err := plan.ValidateForPublish(); err != nil {
			return err
		}
		if err := tx.Model(&PlanRow{}).
			Where("plan_key = ? AND version = ?", planKey, version).
			Update("state", domain.PlanStatePublished).Error; err != nil {
			return err
		}
		out.State = domain.PlanStatePublished
		return nil
	})
	if err != nil {
		return PlanRow{}, err
	}
	return out, nil
}

// LatestPublishedPlan returns the NEWEST published definition of a plan;
// quoting always prices from this immutable row, never a draft.
func (s *CatalogStore) LatestPublishedPlan(ctx context.Context, planKey string) (PlanRow, error) {
	var row PlanRow
	err := s.db.WithContext(ctx).
		Where("plan_key = ? AND state = ?", planKey, domain.PlanStatePublished).
		Order("version DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PlanRow{}, ErrPlanNotFound
	}
	return row, err
}

// GetPlan returns the stored definition row for a plan version.
func (s *CatalogStore) GetPlan(ctx context.Context, planKey string, version int64) (PlanRow, error) {
	var row PlanRow
	err := s.db.WithContext(ctx).Where("plan_key = ? AND version = ?", planKey, version).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PlanRow{}, ErrPlanNotFound
	}
	return row, err
}

// CreateQuote stores a new quote offer. Quotes are append-only; consumption
// happens exclusively through ConsumeQuote.
func (s *CatalogStore) CreateQuote(ctx context.Context, q QuoteRow) error {
	if q.ID == "" || q.TenantID == 0 || q.SnapshotJSON == "" || q.ExpiresAt.IsZero() || q.UsedOrderID != nil {
		return ErrInvalidQuoteRow
	}
	return s.db.WithContext(ctx).Create(&q).Error
}

// GetQuote returns a quote by ID.
func (s *CatalogStore) GetQuote(ctx context.Context, id string) (QuoteRow, error) {
	var row QuoteRow
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return QuoteRow{}, ErrQuoteNotFound
	}
	return row, err
}

// ConsumeQuote validates and marks a quote used in a single transaction. The
// guarded UPDATE checks the subscription version, the expiry, and
// used_order_id IS NULL atomically, so concurrent consumers cannot both win:
// exactly one sets used_order_id, and the unique index keeps that ID single
// across quotes. Expired or superseded quotes are rejected without marks.
func (s *CatalogStore) ConsumeQuote(ctx context.Context, id string, subscriptionVersion int64, orderID string, now time.Time) (QuoteRow, error) {
	if id == "" || orderID == "" || now.IsZero() {
		return QuoteRow{}, ErrInvalidQuoteUsage
	}
	var out QuoteRow
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		out, err = consumeQuoteTx(tx, id, subscriptionVersion, orderID, now)
		return err
	})
	if err != nil {
		return QuoteRow{}, err
	}
	return out, nil
}

// consumeQuoteTx is the transaction-scoped core of ConsumeQuote: it may run
// inside a LARGER unit of work (order creation, plan-change scheduling) so
// the guarded consumption, the order row and the attempt row commit or roll
// back together.
func consumeQuoteTx(tx *gorm.DB, id string, subscriptionVersion int64, orderID string, now time.Time) (QuoteRow, error) {
	var out QuoteRow
	{
		res := tx.Model(&QuoteRow{}).
			Where("id = ? AND used_order_id IS NULL AND subscription_version = ? AND expires_at > ?", id, subscriptionVersion, now).
			Update("used_order_id", orderID)
		if res.Error != nil {
			return QuoteRow{}, res.Error
		}
		if err := tx.Where("id = ?", id).First(&out).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return QuoteRow{}, ErrQuoteNotFound
			}
			return QuoteRow{}, err
		}
		if res.RowsAffected == 1 {
			used := orderID
			out.UsedOrderID = &used
			return out, nil
		}
		// Diagnose why the guarded update missed.
		if out.UsedOrderID != nil {
			return QuoteRow{}, ErrQuoteAlreadyUsed
		}
		if !out.ExpiresAt.After(now) {
			return QuoteRow{}, ErrQuoteExpired
		}
		if out.SubscriptionVersion != subscriptionVersion {
			return QuoteRow{}, ErrQuoteVersionConflict
		}
		return QuoteRow{}, ErrInvalidQuoteUsage
	}
}
