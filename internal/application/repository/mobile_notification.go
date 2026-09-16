package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NotificationIntent is the durable, device-scoped notification identity
// derived from an immutable agent run event. Delivery is intentionally
// at-least-once: event_id plus owner/device/environment is the idempotency key.
type NotificationIntent struct {
	TenantID    uint64
	EventID     string
	OwnerID     string
	DeviceID    string
	Environment string
	Kind        string
	RunID       string
	ExpiresAt   time.Time
}

// NotificationDelivery is a leased delivery attempt. Fence changes every time
// a lease is acquired, so a late worker cannot acknowledge a newer attempt.
type NotificationDelivery struct {
	ID         string
	Intent     NotificationIntent
	Attempt    int64
	Fence      int64
	LeaseUntil time.Time
}

type mobileNotificationRow struct {
	ID          string `gorm:"column:id"`
	TenantID    uint64
	EventID     string
	OwnerID     string
	DeviceID    string
	Environment string
	Kind        string
	RunID       string
	ExpiresAt   time.Time
	State       string
	Attempt     int64
	LeaseOwner  string
	LeaseUntil  *time.Time
	Fence       int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (mobileNotificationRow) TableName() string { return "mobile_notification_intents" }

type NotificationStore struct{ db *gorm.DB }

func NewNotificationStore(db *gorm.DB) *NotificationStore { return &NotificationStore{db: db} }

func validateNotificationIntent(in NotificationIntent) error {
	if in.EventID == "" || in.OwnerID == "" || in.DeviceID == "" || in.Environment == "" ||
		in.Kind == "" || in.RunID == "" || in.ExpiresAt.IsZero() {
		return errors.New("invalid_notification_intent")
	}
	switch in.Kind {
	case "completed", "failed", "interaction_requested", "budget_exhausted":
	default:
		return errors.New("invalid_notification_kind")
	}
	return nil
}

func notificationID(in NotificationIntent) string {
	// The unique constraint is authoritative. A deterministic ID makes logs,
	// retries and support tooling refer to the same intent without token data.
	return fmt.Sprintf("%d:%s:%s:%s:%s", in.TenantID, in.EventID, in.OwnerID, in.DeviceID, in.Environment)
}

// Enqueue inserts an intent idempotently. Replaying the same event after a
// crash is a no-op, including when a prior worker has already claimed it.
func (s *NotificationStore) Enqueue(ctx context.Context, in NotificationIntent) error {
	if s == nil || s.db == nil {
		return errors.New("notification_store_unavailable")
	}
	if err := validateNotificationIntent(in); err != nil {
		return err
	}
	if in.TenantID == 0 {
		// Keep the public intent shape compatible with callers that already
		// resolve ownership through the device binding, while persisting the
		// resolved tenant so subsequent claims remain tenant-scoped.
		var binding struct{ TenantID uint64 }
		err := s.db.WithContext(ctx).Table("mobile_devices").Select("tenant_id").
			Where("owner_id = ? AND device_id = ? AND environment = ? AND revoked_at IS NULL", in.OwnerID, in.DeviceID, in.Environment).
			Take(&binding).Error
		if err != nil {
			return err
		}
		in.TenantID = binding.TenantID
	}
	row := mobileNotificationRow{
		ID: notificationID(in), TenantID: in.TenantID, EventID: in.EventID,
		OwnerID: in.OwnerID, DeviceID: in.DeviceID, Environment: in.Environment,
		Kind: in.Kind, RunID: in.RunID, ExpiresAt: in.ExpiresAt.UTC(), State: "pending",
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

// Claim obtains up to limit unexpired intents. Expired leases are reclaimable;
// revoked devices are filtered in the same SQL selection so no provider call
// should be made for a device that was revoked while a worker was asleep.
func (s *NotificationStore) Claim(ctx context.Context, worker string, limit int, lease time.Duration) ([]NotificationDelivery, error) {
	if s == nil || s.db == nil || worker == "" || limit <= 0 || lease <= 0 {
		return nil, errors.New("invalid_notification_claim")
	}
	now := time.Now().UTC()
	until := now.Add(lease)
	var out []NotificationDelivery
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidates []mobileNotificationRow
		q := tx.Where(`expires_at > ? AND
			(state = 'pending' OR (state = 'in_flight' AND (lease_until IS NULL OR lease_until <= ?))) AND
			EXISTS (SELECT 1 FROM mobile_devices d
				WHERE d.tenant_id = mobile_notification_intents.tenant_id
				  AND d.owner_id = mobile_notification_intents.owner_id
				  AND d.device_id = mobile_notification_intents.device_id
				  AND d.environment = mobile_notification_intents.environment
				  AND d.revoked_at IS NULL) AND
			EXISTS (SELECT 1 FROM agent_runs r
				WHERE r.tenant_id = mobile_notification_intents.tenant_id
				  AND r.run_id = mobile_notification_intents.run_id
				  AND r.owner_id = mobile_notification_intents.owner_id)`, now, now).
			Order("created_at ASC, id ASC").Limit(limit).Find(&candidates)
		if q.Error != nil {
			return q.Error
		}
		for _, candidate := range candidates {
			newFence := candidate.Fence + 1
			update := tx.Model(&mobileNotificationRow{}).
				Where("id = ? AND expires_at > ? AND (state = 'pending' OR (state = 'in_flight' AND (lease_until IS NULL OR lease_until <= ?)))", candidate.ID, now, now).
				Updates(map[string]interface{}{
					"state": "in_flight", "attempt": gorm.Expr("attempt + 1"),
					"lease_owner": worker, "lease_until": until, "fence": newFence,
					"updated_at": now,
				})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				continue
			}
			out = append(out, NotificationDelivery{
				ID:      candidate.ID,
				Intent:  NotificationIntent{TenantID: candidate.TenantID, EventID: candidate.EventID, OwnerID: candidate.OwnerID, DeviceID: candidate.DeviceID, Environment: candidate.Environment, Kind: candidate.Kind, RunID: candidate.RunID, ExpiresAt: candidate.ExpiresAt},
				Attempt: candidate.Attempt + 1, Fence: newFence, LeaseUntil: until,
			})
		}
		return nil
	})
	return out, err
}

// Ack marks one delivery sent only if the worker still owns the exact lease
// fence. A false result is a stale worker or an already completed delivery.
func (s *NotificationStore) Ack(ctx context.Context, id, worker string, fence int64) bool {
	if s == nil || s.db == nil || id == "" || worker == "" || fence <= 0 {
		return false
	}
	result := s.db.WithContext(ctx).Model(&mobileNotificationRow{}).
		Where("id = ? AND state = 'in_flight' AND lease_owner = ? AND fence = ? AND (lease_until IS NULL OR lease_until > ?)", id, worker, fence, time.Now().UTC()).
		Updates(map[string]interface{}{"state": "sent", "lease_owner": "", "lease_until": nil, "updated_at": time.Now().UTC()})
	return result.Error == nil && result.RowsAffected == 1
}

// Retry releases a leased intent for another attempt. The fence is checked so
// a stale worker cannot resurrect an intent claimed by a newer worker.
func (s *NotificationStore) Retry(ctx context.Context, id, worker string, fence int64) bool {
	if s == nil || s.db == nil || id == "" || worker == "" || fence <= 0 {
		return false
	}
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Model(&mobileNotificationRow{}).
		Where("id = ? AND state = 'in_flight' AND lease_owner = ? AND fence = ?", id, worker, fence).
		Updates(map[string]interface{}{"state": "pending", "lease_owner": "", "lease_until": nil, "updated_at": now})
	return result.Error == nil && result.RowsAffected == 1
}

// RunNotificationEvent is the minimal authenticated event identity used by
// the projector. Payloads stay in agent_run_events and are never copied into
// a push intent.
type RunNotificationEvent struct {
	TenantID uint64
	OwnerID  string
	RunID    string
	Seq      int64
	Type     string
}

// ProjectEvent fans one terminal/interaction/budget event out to the current
// active devices for its owner. It is safe to rerun after a crash because each
// event/device/environment has a unique intent key.
func (s *NotificationStore) ProjectEvent(ctx context.Context, evt RunNotificationEvent) error {
	if s == nil || s.db == nil || evt.TenantID == 0 || evt.OwnerID == "" || evt.RunID == "" || evt.Seq <= 0 {
		return errors.New("invalid_notification_event")
	}
	kind, ok := NotificationEventKind(evt.Type)
	if !ok {
		return nil
	}
	var devices []struct {
		DeviceID    string
		Environment string
	}
	if err := s.db.WithContext(ctx).Table("mobile_devices").Select("device_id, environment").
		Where("tenant_id = ? AND owner_id = ? AND revoked_at IS NULL", evt.TenantID, evt.OwnerID).
		Find(&devices).Error; err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, device := range devices {
			in := NotificationIntent{
				TenantID: evt.TenantID, EventID: eventIdentity(evt.TenantID, evt.RunID, evt.Seq),
				OwnerID: evt.OwnerID, DeviceID: device.DeviceID, Environment: device.Environment,
				Kind: kind, RunID: evt.RunID, ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
			}
			if err := enqueueNotificationTx(tx, in); err != nil {
				return err
			}
		}
		return nil
	})
}

func enqueueNotificationTx(tx *gorm.DB, in NotificationIntent) error {
	if err := validateNotificationIntent(in); err != nil {
		return err
	}
	row := mobileNotificationRow{
		ID: notificationID(in), TenantID: in.TenantID, EventID: in.EventID,
		OwnerID: in.OwnerID, DeviceID: in.DeviceID, Environment: in.Environment,
		Kind: in.Kind, RunID: in.RunID, ExpiresAt: in.ExpiresAt.UTC(), State: "pending",
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

func eventIdentity(tenantID uint64, runID string, seq int64) string {
	return fmt.Sprintf("%d:%s:%d", tenantID, runID, seq)
}
