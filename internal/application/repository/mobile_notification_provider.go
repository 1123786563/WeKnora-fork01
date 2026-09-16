package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// NotificationProviderState is durable operational state for one configured
// push provider. It deliberately contains no credentials or token material.
type NotificationProviderState struct {
	ProviderKey string     `gorm:"column:provider_key;primaryKey"`
	Paused      bool       `gorm:"column:paused"`
	Reason      string     `gorm:"column:reason"`
	AlertCount  int64      `gorm:"column:alert_count"`
	PausedAt    *time.Time `gorm:"column:paused_at"`
	RecoveredAt *time.Time `gorm:"column:recovered_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (NotificationProviderState) TableName() string { return "mobile_notification_provider_state" }

type NotificationProviderStateStore struct{ db *gorm.DB }

func NewNotificationProviderStateStore(db *gorm.DB) *NotificationProviderStateStore {
	return &NotificationProviderStateStore{db: db}
}

func providerKeyValid(key string) bool { return strings.TrimSpace(key) != "" }

// IsPaused is fail-closed for a missing/invalid store, but treats an absent
// row as healthy so existing installations can start before the first alert.
func (s *NotificationProviderStateStore) IsPaused(ctx context.Context, key string) (bool, error) {
	if s == nil || s.db == nil || !providerKeyValid(key) {
		return false, errors.New("invalid_notification_provider_state")
	}
	var row NotificationProviderState
	err := s.db.WithContext(ctx).Where("provider_key = ?", key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return row.Paused, err
}

// Pause is idempotent and increments alert_count only when transitioning from
// healthy to paused. This gives operators a durable alert/audit signal while
// avoiding a new alert on every retry tick.
func (s *NotificationProviderStateStore) Pause(ctx context.Context, key, reason string) error {
	if s == nil || s.db == nil || !providerKeyValid(key) {
		return errors.New("invalid_notification_provider_state")
	}
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row NotificationProviderState
		err := tx.Where("provider_key = ?", key).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&NotificationProviderState{ProviderKey: key, Paused: true, Reason: reason, AlertCount: 1, PausedAt: &now, UpdatedAt: now}).Error
		}
		if err != nil {
			return err
		}
		updates := map[string]any{"paused": true, "reason": reason, "updated_at": now}
		if !row.Paused {
			updates["alert_count"] = gorm.Expr("alert_count + ?", 1)
			updates["paused_at"] = now
		}
		return tx.Model(&NotificationProviderState{}).Where("provider_key = ?", key).Updates(updates).Error
	})
}

// Recover clears a durable pause after the provider is configured and a
// subsequent worker pass proves the provider is usable again.
func (s *NotificationProviderStateStore) Recover(ctx context.Context, key string) error {
	if s == nil || s.db == nil || !providerKeyValid(key) {
		return errors.New("invalid_notification_provider_state")
	}
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&NotificationProviderState{}).Where("provider_key = ?", key).Updates(map[string]any{
		"paused": false, "reason": "", "recovered_at": now, "updated_at": now,
	}).Error
}

func (s *NotificationProviderStateStore) State(ctx context.Context, key string) (NotificationProviderState, error) {
	if s == nil || s.db == nil || !providerKeyValid(key) {
		return NotificationProviderState{}, errors.New("invalid_notification_provider_state")
	}
	var row NotificationProviderState
	err := s.db.WithContext(ctx).Where("provider_key = ?", key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NotificationProviderState{ProviderKey: key}, nil
	}
	return row, err
}
