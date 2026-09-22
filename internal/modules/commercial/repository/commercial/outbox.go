package commercial

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidOutboxEvent   = errors.New("invalid_outbox_event")
	ErrOutboxEventDuplicate = errors.New("outbox_event_duplicate")
)

// Outbox event states. pending awaits dispatch; sent was delivered; dead
// exhausted its delivery attempts.
const (
	OutboxStatePending = "pending"
	OutboxStateSent    = "sent"
	OutboxStateDead    = "dead"
)

// OutboxEvent is a durable benefit event written in the same transaction as
// the payment fact it describes. event_key UNIQUE makes emission exactly-once
// at the storage layer: replays conflict and no-op instead of duplicating.
type OutboxEvent struct {
	EventKey     string    `gorm:"primaryKey;column:event_key"`
	TenantID     uint64    `gorm:"column:tenant_id;not null"`
	Kind         string    `gorm:"column:kind;not null"`
	PayloadJSON  string    `gorm:"column:payload_json;not null"`
	State        string    `gorm:"column:state;not null;default:pending"`
	LeaseToken   string    `gorm:"column:lease_token;not null;default:''"`
	LeaseUntil   time.Time `gorm:"column:lease_until;not null;default:CURRENT_TIMESTAMP"`
	AttemptCount int       `gorm:"column:attempt_count;not null;default:0"`
}

func (OutboxEvent) TableName() string { return "commercial_outbox_events" }

type OutboxStore struct{ db *gorm.DB }

func NewOutboxStore(db *gorm.DB) *OutboxStore { return &OutboxStore{db: db} }

// Enqueue inserts an event on its own; EnqueueTx inserts inside the caller's
// open transaction so the event commits atomically with the state change it
// describes. A duplicate event_key reports ErrOutboxEventDuplicate so the
// caller can decide whether a replay conflict is success or failure.
func (s *OutboxStore) Enqueue(ctx context.Context, e OutboxEvent) error {
	return insertOutboxEvent(s.db.WithContext(ctx), e)
}

func (s *OutboxStore) EnqueueTx(tx *gorm.DB, e OutboxEvent) error {
	return insertOutboxEvent(tx, e)
}

func insertOutboxEvent(db *gorm.DB, e OutboxEvent) error {
	if e.EventKey == "" || e.TenantID == 0 || e.Kind == "" || e.PayloadJSON == "" {
		return ErrInvalidOutboxEvent
	}
	if e.State == "" {
		e.State = OutboxStatePending
	}
	if e.LeaseUntil.IsZero() {
		e.LeaseUntil = time.Now()
	}
	res := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&e)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrOutboxEventDuplicate
	}
	return nil
}
