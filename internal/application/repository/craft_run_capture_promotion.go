package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CraftRunCapturePromotionCandidate is a bounded promotion scan projection.
type CraftRunCapturePromotionCandidate struct {
	Scope         craft.Scope
	WorkspaceID   string
	RunID         string
	State         string
	DraftRevision *int64
	UpdatedAt     time.Time
}

type craftRunCapturePromotionCursorRow struct {
	ID          uint64     `gorm:"column:id;primaryKey"`
	UpdatedAt   *time.Time `gorm:"column:updated_at"`
	TenantID    *uint64    `gorm:"column:tenant_id"`
	WorkspaceID *string    `gorm:"column:workspace_id"`
	RunID       *string    `gorm:"column:run_id"`
}

func (craftRunCapturePromotionCursorRow) TableName() string {
	return "craft_run_capture_promotion_cursor"
}

type craftRunCapturePromotionAttemptRow struct {
	TenantID    uint64    `gorm:"column:tenant_id;primaryKey"`
	WorkspaceID string    `gorm:"column:workspace_id;primaryKey"`
	RunID       string    `gorm:"column:run_id;primaryKey"`
	RetryAfter  time.Time `gorm:"column:retry_after"`
	Completed   bool      `gorm:"column:completed"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

type craftPromotionReceiptRow struct {
	TenantID      uint64    `gorm:"column:tenant_id"`
	WorkspaceID   string    `gorm:"column:workspace_id"`
	RunID         string    `gorm:"column:run_id"`
	OwnerID       string    `gorm:"column:owner_id"`
	SessionID     string    `gorm:"column:session_id"`
	State         string    `gorm:"column:state"`
	DraftRevision *int64    `gorm:"column:draft_revision"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (craftRunCapturePromotionAttemptRow) TableName() string {
	return "craft_run_capture_promotion_attempts"
}

// ClaimPromotionBatch reserves one bounded global page before external probes
// run. Cursor and retry reservations commit together so another process or a
// restart continues from the next page; crashes release claimed receipts when
// their cooldown expires.
func (s *CraftRunCaptureStore) ClaimPromotionBatch(ctx context.Context, limit int, cooldown time.Duration) ([]CraftRunCapturePromotionCandidate, error) {
	return s.claimPromotionBatch(ctx, limit, cooldown, nil)
}

// ClaimPromotionForRun reserves only the target Run's eligible receipts and
// deliberately leaves the global scan cursor unchanged.
func (s *CraftRunCaptureStore) ClaimPromotionForRun(ctx context.Context, tenantID uint64, runID string, cooldown time.Duration) ([]CraftRunCapturePromotionCandidate, error) {
	return s.claimPromotionBatch(ctx, 32, cooldown, &craftPromotionReceiptFilter{TenantID: tenantID, RunID: runID})
}

type craftPromotionReceiptFilter struct {
	TenantID uint64
	RunID    string
}

func (s *CraftRunCaptureStore) claimPromotionBatch(ctx context.Context, limit int, cooldown time.Duration, filter *craftPromotionReceiptFilter) ([]CraftRunCapturePromotionCandidate, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("%w: capture promotion store is unavailable", craft.ErrUnsupported)
	}
	if limit <= 0 {
		limit = 32
	}
	if limit > 32 {
		limit = 32
	}
	if cooldown <= 0 {
		return nil, fmt.Errorf("%w: promotion retry cooldown must be positive", craft.ErrInvalidInput)
	}
	now := time.Now().UTC()
	var claimed []CraftRunCapturePromotionCandidate
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// All claim paths take the same singleton lock. The targeted per-Run
		// claimant does not read or move the cursor, but it must serialize with
		// a global claim so one receipt cannot be probed by both paths.
		locked := tx.Model(&craftRunCapturePromotionCursorRow{}).
			Where("id = 1").UpdateColumn("id", gorm.Expr("id"))
		if locked.Error != nil {
			return locked.Error
		}
		if locked.RowsAffected != 1 {
			return fmt.Errorf("%w: promotion scan cursor row is missing", craft.ErrInvalidInput)
		}
		var cursor craftRunCapturePromotionCursorRow
		if filter == nil {
			// SQLite's no-op write above also takes its writer slot, while this
			// row lock documents the corresponding PostgreSQL serialization.
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = 1").Take(&cursor).Error; err != nil {
				return err
			}
		}

		base := func() *gorm.DB {
			q := tx.Table("craft_run_captures AS c").
				Select("c.tenant_id, c.workspace_id, c.run_id, c.owner_id, c.session_id, c.state, c.draft_revision, c.updated_at").
				Where("c.state IN ('sealed','advanced') AND c.draft_revision IS NOT NULL")
			if filter != nil {
				q = q.Where("c.tenant_id = ? AND c.run_id = ?", filter.TenantID, filter.RunID)
			}
			return q
		}
		order := "c.updated_at ASC, c.tenant_id ASC, c.workspace_id ASC, c.run_id ASC"
		var rows, freshRows, scannedFreshRows []craftPromotionReceiptRow
		if filter != nil {
			q := base().Where(`NOT EXISTS (
				SELECT 1 FROM craft_run_capture_promotion_attempts AS a
				WHERE a.tenant_id=c.tenant_id AND a.workspace_id=c.workspace_id AND a.run_id=c.run_id
				AND (a.completed = ? OR a.retry_after > ?)
			)`, true, now).Order(order)
			if err := q.Limit(limit).Scan(&rows).Error; err != nil {
				return err
			}
		} else {
			// Due retries get at most three quarters of the page. Reserving fresh
			// capacity on every tick means a permanently failing due backlog can
			// never pin the global cursor in place.
			freshQuota := limit / 4
			if freshQuota == 0 {
				freshQuota = 1
			}
			dueLimit := limit - freshQuota
			if dueLimit > 0 {
				dueQuery := duePromotionQuery(base(), now, dueLimit)
				if err := dueQuery.Limit(dueLimit).Scan(&rows).Error; err != nil {
					return err
				}
			}
			freshScanLimit := limit - len(rows)
			if freshScanLimit > 0 {
				freshPageQuery := freshPromotionPageQuery(base(), cursor, freshScanLimit)
				if err := freshPageQuery.Scan(&scannedFreshRows).Error; err != nil {
					return err
				}
				var filterErr error
				freshRows, filterErr = filterUnattemptedPromotionRows(tx, scannedFreshRows)
				if filterErr != nil {
					return filterErr
				}
				rows = append(rows, freshRows...)
				if len(scannedFreshRows) > 0 {
					last := scannedFreshRows[len(scannedFreshRows)-1]
					if err := updatePromotionCursor(tx, &last); err != nil {
						return err
					}
				}
				if len(scannedFreshRows) < freshScanLimit && cursor.UpdatedAt != nil {
					if err := updatePromotionCursor(tx, nil); err != nil {
						return err
					}
				}
			}
		}
		if len(rows) == 0 {
			return nil
		}

		attempts := make([]craftRunCapturePromotionAttemptRow, 0, len(rows))
		claimed = make([]CraftRunCapturePromotionCandidate, 0, len(rows))
		for _, row := range rows {
			attempts = append(attempts, craftRunCapturePromotionAttemptRow{
				TenantID: row.TenantID, WorkspaceID: row.WorkspaceID, RunID: row.RunID,
				RetryAfter: now.Add(cooldown), Completed: false, UpdatedAt: now,
			})
			claimed = append(claimed, CraftRunCapturePromotionCandidate{
				Scope:       craft.Scope{TenantID: row.TenantID, UserID: row.OwnerID, SessionID: row.SessionID},
				WorkspaceID: row.WorkspaceID, RunID: row.RunID, State: row.State,
				DraftRevision: row.DraftRevision, UpdatedAt: row.UpdatedAt,
			})
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "workspace_id"}, {Name: "run_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"retry_after", "updated_at"}),
		}).Create(&attempts).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

func duePromotionQuery(base *gorm.DB, now time.Time, limit int) *gorm.DB {
	return base.Joins(`JOIN craft_run_capture_promotion_attempts AS a
		ON a.tenant_id=c.tenant_id AND a.workspace_id=c.workspace_id AND a.run_id=c.run_id`).
		Where("a.completed = ? AND a.retry_after <= ?", false, now).
		Order("a.retry_after ASC, a.tenant_id ASC, a.workspace_id ASC, a.run_id ASC").
		Limit(limit)
}

func freshPromotionPageQuery(base *gorm.DB, cursor craftRunCapturePromotionCursorRow, limit int) *gorm.DB {
	q := base
	if base.Dialector.Name() == "sqlite" {
		// SQLite otherwise prefers the broader state/updated_at recovery index
		// and sorts the complete matching history before applying LIMIT. This
		// fixed index name matches the migration and guarantees keyset order.
		q = q.Table("craft_run_captures AS c INDEXED BY idx_craft_run_captures_promotion_scan")
	}
	if cursor.UpdatedAt != nil {
		q = q.Where("(c.updated_at, c.tenant_id, c.workspace_id, c.run_id) > (?, ?, ?, ?)",
			*cursor.UpdatedAt, *cursor.TenantID, *cursor.WorkspaceID, *cursor.RunID)
	}
	return q.Order("c.updated_at ASC, c.tenant_id ASC, c.workspace_id ASC, c.run_id ASC").Limit(limit)
}

func filterUnattemptedPromotionRows(tx *gorm.DB, scanned []craftPromotionReceiptRow) ([]craftPromotionReceiptRow, error) {
	if len(scanned) == 0 {
		return nil, nil
	}
	query := tx.Table("craft_run_capture_promotion_attempts").Select("tenant_id, workspace_id, run_id")
	for i, row := range scanned {
		condition := "tenant_id = ? AND workspace_id = ? AND run_id = ?"
		args := []any{row.TenantID, row.WorkspaceID, row.RunID}
		if i == 0 {
			query = query.Where(condition, args...)
		} else {
			query = query.Or(condition, args...)
		}
	}
	var attempted []craftRunCapturePromotionAttemptRow
	if err := query.Scan(&attempted).Error; err != nil {
		return nil, err
	}
	claimed := make(map[string]struct{}, len(attempted))
	for _, row := range attempted {
		claimed[promotionReceiptKey(row.TenantID, row.WorkspaceID, row.RunID)] = struct{}{}
	}
	fresh := make([]craftPromotionReceiptRow, 0, len(scanned))
	for _, row := range scanned {
		if _, exists := claimed[promotionReceiptKey(row.TenantID, row.WorkspaceID, row.RunID)]; !exists {
			fresh = append(fresh, row)
		}
	}
	return fresh, nil
}

func promotionReceiptKey(tenantID uint64, workspaceID, runID string) string {
	return fmt.Sprintf("%d\x00%s\x00%s", tenantID, workspaceID, runID)
}

func updatePromotionCursor(tx *gorm.DB, row *craftPromotionReceiptRow) error {
	values := map[string]any{"updated_at": nil, "tenant_id": nil, "workspace_id": nil, "run_id": nil}
	if row != nil {
		values["updated_at"], values["tenant_id"] = row.UpdatedAt, row.TenantID
		values["workspace_id"], values["run_id"] = row.WorkspaceID, row.RunID
	}
	return tx.Model(&craftRunCapturePromotionCursorRow{}).Where("id = 1").UpdateColumns(values).Error
}

// CompletePromotion permanently removes a successfully handled or terminally
// ineligible receipt from future promotion pages.
func (s *CraftRunCaptureStore) CompletePromotion(ctx context.Context, tenantID uint64, workspaceID, runID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("%w: capture promotion store is unavailable", craft.ErrUnsupported)
	}
	result := s.db.WithContext(ctx).Model(&craftRunCapturePromotionAttemptRow{}).
		Where("tenant_id = ? AND workspace_id = ? AND run_id = ?", tenantID, workspaceID, runID).
		Updates(map[string]any{"completed": true, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("craft promotion receipt was not claimed")
	}
	return nil
}
