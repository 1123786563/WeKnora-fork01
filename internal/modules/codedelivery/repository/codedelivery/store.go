// Package codedelivery persists the delivery traceability rows (T22 #52):
// task/run identity, the A03 approval anchor, and the remote receipts
// (commit sha, draft PR, actual remote identity).
package codedelivery

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	ErrDeliveryNotFound      = errors.New("code_delivery_not_found")
	ErrDeliveryStateConflict = errors.New("code_delivery_state_conflict")
)

// DeliveryRow is one code delivery bound to a task (session) and a run.
type DeliveryRow struct {
	ID           string `gorm:"primaryKey;column:id"`
	TenantID     uint64 `gorm:"column:tenant_id;not null"`
	TaskID       string `gorm:"column:task_id;not null"`
	RunID        string `gorm:"column:run_id;not null"`
	OwnerID      string `gorm:"column:owner_id;not null;default:''"`
	ActionID     string `gorm:"column:action_id;not null;default:''"`
	ConnectionID string `gorm:"column:connection_id;not null;default:''"`
	Repo         string `gorm:"column:repo;not null;default:''"`
	BaselineSHA  string `gorm:"column:baseline_sha;not null;default:''"`
	Branch       string `gorm:"column:branch;not null;default:''"`
	CommitSHA    string `gorm:"column:commit_sha;not null;default:''"`
	PRNumber     int64  `gorm:"column:pr_number;not null;default:0"`
	PRURL        string `gorm:"column:pr_url;not null;default:''"`
	RemoteLogin  string `gorm:"column:remote_login;not null;default:''"`
	State        string `gorm:"column:state;not null;default:'prepared'"`
	Failure      string `gorm:"column:failure;not null;default:''"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (DeliveryRow) TableName() string { return "code_deliveries" }

// ReceiptUpdate carries remote receipts; zero fields are left untouched.
type ReceiptUpdate struct {
	CommitSHA   string
	PRNumber    int64
	PRURL       string
	RemoteLogin string
}

// DeliveryStore is the persistence surface of the delivery module.
type DeliveryStore struct{ db *gorm.DB }

func NewDeliveryStore(db *gorm.DB) *DeliveryStore { return &DeliveryStore{db: db} }

// DB exposes the underlying handle for read-face joins (same package only).
func (s *DeliveryStore) DB() *gorm.DB { return s.db }

func (s *DeliveryStore) CreateDelivery(ctx context.Context, row DeliveryRow) error {
	return s.db.WithContext(ctx).Create(&row).Error
}

func (s *DeliveryStore) GetDelivery(ctx context.Context, tenantID uint64, id string) (DeliveryRow, error) {
	var row DeliveryRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DeliveryRow{}, ErrDeliveryNotFound
	}
	return row, err
}

func (s *DeliveryStore) LatestForRun(ctx context.Context, tenantID uint64, runID string) (DeliveryRow, error) {
	var row DeliveryRow
	// created_at 并列（sqlite DATETIME 秒级精度下可能发生）时以 id DESC
	// 决出全序——单一 created_at 排序的并列行次序未定，同一 run 并发多次
	// prepare 的极端场景读面可能取错行。
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND run_id = ?", tenantID, runID).
		Order("created_at DESC").Order("id DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DeliveryRow{}, ErrDeliveryNotFound
	}
	return row, err
}

// TransitionState is the CAS state machine write: the row must currently sit
// in one of `from`, otherwise ErrDeliveryStateConflict (nothing is written).
func (s *DeliveryStore) TransitionState(ctx context.Context, tenantID uint64, id string, from []string, to, failure string) error {
	if len(from) == 0 {
		return ErrDeliveryStateConflict
	}
	res := s.db.WithContext(ctx).Model(&DeliveryRow{}).
		Where("tenant_id = ? AND id = ? AND state IN ?", tenantID, id, from).
		Updates(map[string]any{"state": to, "failure": failure})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDeliveryStateConflict
	}
	return nil
}

// TransitionStateWithReceipts atomically records remote facts and settles the
// state under the same source-state CAS. A rejected CAS writes neither.
func (s *DeliveryStore) TransitionStateWithReceipts(ctx context.Context, tenantID uint64, id string, from []string, to, failure string, update ReceiptUpdate) error {
	if len(from) == 0 {
		return ErrDeliveryStateConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		patch := map[string]any{"state": to, "failure": failure}
		if update.CommitSHA != "" {
			patch["commit_sha"] = update.CommitSHA
		}
		if update.PRNumber != 0 {
			patch["pr_number"] = update.PRNumber
		}
		if update.PRURL != "" {
			patch["pr_url"] = update.PRURL
		}
		if update.RemoteLogin != "" {
			patch["remote_login"] = update.RemoteLogin
		}
		res := tx.Model(&DeliveryRow{}).Where("tenant_id = ? AND id = ? AND state IN ?", tenantID, id, from).Updates(patch)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrDeliveryStateConflict
		}
		return nil
	})
}

// RecordReceipts writes remote receipts; only non-zero fields update.
func (s *DeliveryStore) RecordReceipts(ctx context.Context, tenantID uint64, id string, update ReceiptUpdate) error {
	patch := map[string]any{}
	if update.CommitSHA != "" {
		patch["commit_sha"] = update.CommitSHA
	}
	if update.PRNumber != 0 {
		patch["pr_number"] = update.PRNumber
	}
	if update.PRURL != "" {
		patch["pr_url"] = update.PRURL
	}
	if update.RemoteLogin != "" {
		patch["remote_login"] = update.RemoteLogin
	}
	if len(patch) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&DeliveryRow{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).Updates(patch).Error
}

// LatestApproverForAction reads the most recent approval bound to the A03
// action (approval content traceability). Parameter-bound join on
// app_action_approvals.
func (s *DeliveryStore) LatestApproverForAction(ctx context.Context, actionID string) (string, bool, error) {
	var row struct {
		Actor string `gorm:"column:actor"`
	}
	err := s.db.WithContext(ctx).Table("app_action_approvals").
		Select("actor").
		Where("action_id = ?", actionID).
		Order("expiry DESC").Limit(1).Scan(&row).Error
	if err != nil {
		return "", false, err
	}
	return row.Actor, row.Actor != "", nil
}

// State constants mirror the module-level DeliveryState vocabulary. They are
// string literals ON PURPOSE: the repository package must NOT import its
// parent (the parent's service imports this store — importing the parent
// back is an import cycle; T22 #52 task 5). The values are pinned equal to
// the parent's DeliveryState constants by the parent's service_prepare_test
// alignment assertions.
const (
	StatePrepared   = "prepared"
	StateDispatched = "dispatched"
	StatePushed     = "pushed"
	StateDelivered  = "delivered"
	StateFailed     = "failed"
	StateUnknown    = "unknown"
)
