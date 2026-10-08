package commercial

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// WebhookInboxRow is the durable dedupe face for commercial webhooks
// (#98 / Lago 26): (provider, event_id) is the unique key the spec's
// "unique key 验证" requires — a redelivery is a receipt no-op, never a
// second projection event.
type WebhookInboxRow struct {
	ID         uint64    `gorm:"primaryKey"`
	Provider   string    `gorm:"uniqueIndex:uniq_webhook_event"`
	EventID    string    `gorm:"uniqueIndex:uniq_webhook_event"`
	Kind       string
	ExternalID string
	TenantID   uint64
	ReceivedAt time.Time
}

func (WebhookInboxRow) TableName() string { return "commercial_webhook_inbox" }

// ProjectionAuditRow records the auditable convergence trail (#98 / US43-45,
// gate 19): every divergence, repair and watermark move, from either the
// webhook path or the reconciliation pass.
type ProjectionAuditRow struct {
	ID         uint64 `gorm:"primaryKey"`
	Source     string // webhook | reconcile
	Action     string // received | repaired | diverged | watermark
	Kind       string
	ExternalID string
	TenantID   uint64
	Detail     string
	CreatedAt  time.Time
}

func (ProjectionAuditRow) TableName() string { return "commercial_projection_audit" }

// ReconciliationStateRow persists the reconciliation watermark per stream
// (#98): the cursor only ever advances on a successful pass.
type ReconciliationStateRow struct {
	Stream      string    `gorm:"primaryKey"`
	CursorValue string
	UpdatedAt   time.Time
}

func (ReconciliationStateRow) TableName() string { return "commercial_reconciliation_state" }

// ProjectionStore is the inbox/audit/cursor persistence face.
type ProjectionStore struct {
	db *gorm.DB
}

func NewProjectionStore(db *gorm.DB) *ProjectionStore { return &ProjectionStore{db: db} }

// RecordWebhook inserts the inbox receipt; a unique conflict reports false
// (duplicate delivery) without error.
func (s *ProjectionStore) RecordWebhook(ctx context.Context, row *WebhookInboxRow) (bool, error) {
	err := s.db.WithContext(ctx).Create(row).Error
	if err != nil {
		if isUniqueViolation(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Audit appends one audit trail row.
func (s *ProjectionStore) Audit(ctx context.Context, row *ProjectionAuditRow) error {
	return s.db.WithContext(ctx).Create(row).Error
}

// LoadCursor returns the stored watermark for the stream ("" when none).
func (s *ProjectionStore) LoadCursor(ctx context.Context, stream string) (string, error) {
	var row ReconciliationStateRow
	err := s.db.WithContext(ctx).Where("stream = ?", stream).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return row.CursorValue, nil
}

// SaveCursor advances the watermark; a backward value is ignored so the
// cursor is monotonic by construction.
func (s *ProjectionStore) SaveCursor(ctx context.Context, stream, value string) error {
	if value == "" {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row ReconciliationStateRow
		err := tx.Where("stream = ?", stream).First(&row).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return tx.Create(&ReconciliationStateRow{Stream: stream, CursorValue: value, UpdatedAt: time.Now().UTC()}).Error
		}
		if value == row.CursorValue {
			return nil
		}
		return tx.Model(&ReconciliationStateRow{}).Where("stream = ?", stream).
			Update("cursor_value", value).Error
	})
}
