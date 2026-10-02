package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WorkbenchRequest is the immutable client request and its durable admission state.
// TenantID and ActorID are always supplied by the authenticated context.
type WorkbenchRequest struct {
	TenantID       uint64
	ActorID        string
	RequestID      string
	RequestHash    string
	SessionID      string
	AgentID        string
	TargetID       string
	WorkspaceRef   string
	Text           string
	BudgetUpper    int64
	ReservationRef string
	State          string
	RunID          string
	Reason         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type workbenchRequestRow struct {
	TenantID       uint64 `gorm:"primaryKey"`
	ActorID        string `gorm:"primaryKey;size:512"`
	RequestID      string `gorm:"primaryKey;size:128"`
	RequestHash    string `gorm:"size:64;not null"`
	SessionID      string `gorm:"size:64;not null"`
	AgentID        string `gorm:"size:64;not null"`
	TargetID       string `gorm:"size:512;not null"`
	WorkspaceRef   string `gorm:"size:512;not null"`
	Text           string `gorm:"type:text;not null"`
	BudgetUpper    int64  `gorm:"not null"`
	ReservationRef string `gorm:"size:512;not null"`
	State          string `gorm:"size:32;not null"`
	RunID          string `gorm:"size:64;not null"`
	Reason         string `gorm:"size:512;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (workbenchRequestRow) TableName() string { return "workbench_requests" }

func (r workbenchRequestRow) view() WorkbenchRequest {
	return WorkbenchRequest{
		TenantID: r.TenantID, ActorID: r.ActorID, RequestID: r.RequestID,
		RequestHash: r.RequestHash, SessionID: r.SessionID, AgentID: r.AgentID,
		TargetID: r.TargetID, WorkspaceRef: r.WorkspaceRef, Text: r.Text,
		BudgetUpper: r.BudgetUpper, ReservationRef: r.ReservationRef,
		State: r.State, RunID: r.RunID, Reason: r.Reason,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

type WorkbenchRequestRepository struct{ db *gorm.DB }

func NewWorkbenchRequestRepository(db *gorm.DB) *WorkbenchRequestRepository {
	return &WorkbenchRequestRepository{db: db}
}

func (r *WorkbenchRequestRepository) CreatePending(ctx context.Context, request WorkbenchRequest) error {
	if r == nil || r.db == nil || request.TenantID == 0 || request.ActorID == "" || request.RequestID == "" || request.RequestHash == "" {
		return errors.New("invalid workbench request")
	}
	row := workbenchRequestRow{
		TenantID: request.TenantID, ActorID: request.ActorID, RequestID: request.RequestID,
		RequestHash: request.RequestHash, SessionID: request.SessionID, AgentID: request.AgentID,
		TargetID: request.TargetID, WorkspaceRef: request.WorkspaceRef, Text: request.Text,
		BudgetUpper: request.BudgetUpper, State: "pending",
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrDuplicatedKey
	}
	return nil
}

func (r *WorkbenchRequestRepository) Get(ctx context.Context, tenantID uint64, actorID, requestID string) (WorkbenchRequest, error) {
	var row workbenchRequestRow
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND actor_id = ? AND request_id = ?", tenantID, actorID, requestID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return WorkbenchRequest{}, gorm.ErrRecordNotFound
	}
	return row.view(), err
}

func (r *WorkbenchRequestRepository) UpdatePending(ctx context.Context, request WorkbenchRequest, state, reservationRef, runID, reason string) error {
	if state != "pending" && state != "dispatching" && state != "admitted" && state != "rejected" {
		return errors.New("invalid request state")
	}
	result := r.db.WithContext(ctx).Model(&workbenchRequestRow{}).
		Where("tenant_id = ? AND actor_id = ? AND request_id = ? AND request_hash = ? AND state = 'pending'", request.TenantID, request.ActorID, request.RequestID, request.RequestHash).
		Updates(map[string]any{"state": state, "reservation_ref": reservationRef, "run_id": runID, "reason": reason, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("workbench request state changed")
	}
	return nil
}

// ClaimPendingRunID atomically installs a run identity on a pending request
// whose admission was interrupted before one was persisted. Exactly one
// concurrent caller generates the identity; the others observe the winner's.
func (r *WorkbenchRequestRepository) ClaimPendingRunID(ctx context.Context, request WorkbenchRequest, runID string) (string, error) {
	result := r.db.WithContext(ctx).Model(&workbenchRequestRow{}).
		Where("tenant_id = ? AND actor_id = ? AND request_id = ? AND request_hash = ? AND state = 'pending' AND run_id = ''", request.TenantID, request.ActorID, request.RequestID, request.RequestHash).
		Updates(map[string]any{"run_id": runID, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 1 {
		return runID, nil
	}
	current, err := r.Get(ctx, request.TenantID, request.ActorID, request.RequestID)
	if err != nil {
		return "", err
	}
	return current.RunID, nil
}

// RejectPendingWithoutRun atomically rejects a pending admission only if its
// AgentRun has not committed. It takes the same session write lock as
// AgentRunStore.Admit before checking the run table, so a concurrent admission
// either wins and is preserved for replay or observes the rejected request's
// retirement denial before it can create a Run.
func (r *WorkbenchRequestRepository) RejectPendingWithoutRun(ctx context.Context, request WorkbenchRequest, reason string) (bool, error) {
	if r == nil || r.db == nil || request.TenantID == 0 || request.ActorID == "" || request.SessionID == "" || request.RequestID == "" || request.RequestHash == "" {
		return false, errors.New("invalid workbench request")
	}
	rejected := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lock := tx.Table("sessions").
			Where("tenant_id = ? AND id = ? AND user_id = ? AND deleted_at IS NULL", request.TenantID, request.SessionID, request.ActorID).
			UpdateColumn("active_agent_run_id", gorm.Expr("active_agent_run_id"))
		if lock.Error != nil {
			return lock.Error
		}
		if lock.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		var existing agentRunRow
		err := tx.Where("tenant_id = ? AND owner_id = ? AND request_id = ?", request.TenantID, request.ActorID, request.RequestID).Take(&existing).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		result := tx.Model(&workbenchRequestRow{}).
			Where("tenant_id = ? AND actor_id = ? AND request_id = ? AND request_hash = ? AND state = 'pending'", request.TenantID, request.ActorID, request.RequestID, request.RequestHash).
			Updates(map[string]any{"state": "rejected", "reason": reason, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
		if result.Error != nil {
			return result.Error
		}
		rejected = result.RowsAffected == 1
		return nil
	})
	return rejected, err
}

// UpdateFromState performs the state transition used by the dispatch outbox.
// A dispatching row is intentionally recoverable: if the process disappears
// after the durable transition, the run worker can discover the queued run
// without a second wake-up publication.
func (r *WorkbenchRequestRepository) UpdateFromState(ctx context.Context, request WorkbenchRequest, expected, state, reservationRef, runID, reason string) error {
	if expected == "" || state == "" {
		return errors.New("request states are required")
	}
	result := r.db.WithContext(ctx).Model(&workbenchRequestRow{}).
		Where("tenant_id = ? AND actor_id = ? AND request_id = ? AND request_hash = ? AND state = ?", request.TenantID, request.ActorID, request.RequestID, request.RequestHash, expected).
		Updates(map[string]any{"state": state, "reservation_ref": reservationRef, "run_id": runID, "reason": reason, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("workbench request state changed")
	}
	return nil
}
