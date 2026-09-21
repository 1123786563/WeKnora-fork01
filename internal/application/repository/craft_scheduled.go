package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SP3 (C-23): the craft scheduled-task store. One user-owned recipe row per
// task plus the append-only fire ledger; the 30s dispatcher claims due
// recipes through the CAS below (Ruling P-1). Every owner-facing read is
// scoped tenant+owner+live in THIS layer, so a wrong-owner, deleted or
// missing task is one indistinguishable not-found — the API never leaks
// which one it was.

// craftScheduledDefaultListCap and craftScheduledMaxListCap bound the run
// history page (spec §3: limit 1..100, default 50). A non-positive limit
// falls back to the default instead of returning everything.
const (
	craftScheduledDefaultListCap = 50
	craftScheduledMaxListCap     = 100
	craftScheduledDefaultBatch   = 50
)

// ClaimedTask is one due fire this caller's sweep won. The CAS UPDATE in
// ClaimDueTasks is the win itself; this struct reports both sides of the
// swap so the dispatcher can label the run with the fired time without a
// re-read.
type ClaimedTask struct {
	// Task is the recipe snapshot AFTER the claim: NextRunAt already holds
	// the following fire.
	Task types.CraftScheduledTask
	// PrevNextRunAt is the due time that just fired — the OLD next_run_at
	// the CAS matched. The claim also pins it into last_run_at.
	PrevNextRunAt time.Time
}

// CraftScheduledTaskRepository persists the scheduled-task recipe table and
// its run ledger, and owns the due-fire claim.
type CraftScheduledTaskRepository interface {
	// Create stores one recipe. The cron expression must be canonical
	// five-field (types.ValidateCronExpression); an empty ID is minted.
	Create(ctx context.Context, task *types.CraftScheduledTask) error
	// ListByOwner returns the owner's live recipes newest-first (created_at
	// DESC). No pagination — V1 volumes are small, the Onyx shape.
	ListByOwner(ctx context.Context, tenantID uint64, ownerID string) ([]types.CraftScheduledTask, error)
	// GetByID returns one live recipe in the owner's scope. Another owner's
	// or tenant's recipe, a deleted recipe and a missing id all answer
	// craft.ErrNotFound — indistinguishable on purpose.
	GetByID(ctx context.Context, tenantID uint64, ownerID, id string) (*types.CraftScheduledTask, error)
	// Update rewrites the mutable fields (name, prompt, cron, editor_mode,
	// status, next_run_at, last_run_at) scoped to the owner's live row; a
	// scoped miss is craft.ErrNotFound.
	Update(ctx context.Context, task *types.CraftScheduledTask) error
	// SoftDelete tombstones one recipe. Idempotent: a row that is already
	// gone (deleted, missing or another owner's) is a no-op success — the
	// desired end state holds and nothing is leaked.
	SoftDelete(ctx context.Context, tenantID uint64, ownerID, id string) error
	// ClaimDueTasks sweeps at most batch due fires (status active, live,
	// next_run_at <= now, oldest first) and claims EACH row with an
	// independent compare-and-swap UPDATE — next_run_at = following fire
	// WHERE the row still carries the old ticket and is active and live.
	// Only rows whose CAS affected one row are returned: a row another
	// instance already claimed (or that changed underneath the sweep) is
	// silently skipped. This is the portable SKIP LOCKED equivalent that
	// keeps multi-instance dispatches from double-firing (Ruling P-1).
	ClaimDueTasks(ctx context.Context, now time.Time, batch int) ([]ClaimedTask, error)
	// InsertRun appends one fire ledger row (queued, skipped or manual). An
	// empty ID is minted and a zero StartedAt is stamped with the insert
	// time — the fire time is the run-history pagination anchor, so it is
	// never NULL.
	InsertRun(ctx context.Context, run *types.CraftScheduledTaskRun) error
	// UpdateRunStatus moves one run to its next status, writing the error
	// fields and summary (empty strings clear to NULL) and finished_at when
	// the caller supplies it. A missing run id answers craft.ErrNotFound.
	UpdateRunStatus(ctx context.Context, id, status, errorClass, detail, summary string, finishedAt *time.Time) error
	// ListRunsByTask pages one task's run history newest-first by started_at
	// with the strict-before keyset cursor (before is exclusive) and the
	// clamped limit. Terminal and in-flight rows are both listed.
	ListRunsByTask(ctx context.Context, taskID string, before *time.Time, limit int) ([]types.CraftScheduledTaskRun, error)
	// HasInFlightRun reports whether the task holds a queued or running run
	// — the SKIP_IF_RUNNING overlap policy's probe.
	HasInFlightRun(ctx context.Context, taskID string) (bool, error)
}

type craftScheduledTaskRepository struct{ db *gorm.DB }

var _ CraftScheduledTaskRepository = (*craftScheduledTaskRepository)(nil)

// NewCraftScheduledTaskRepository returns the GORM-backed implementation
// over the migrated business database.
func NewCraftScheduledTaskRepository(db *gorm.DB) CraftScheduledTaskRepository {
	return &craftScheduledTaskRepository{db: db}
}

// craftScheduledDefaults fills the row-key and column defaults a caller may
// legitimately omit; the cron stays the caller's contract to state.
func craftScheduledDefaults(task *types.CraftScheduledTask) {
	if task.ID == "" {
		task.ID = uuid.NewString()
	}
	if task.EditorMode == "" {
		task.EditorMode = types.CraftScheduledEditorModeAdvanced
	}
	if task.Status == "" {
		task.Status = types.CraftScheduledTaskStatusActive
	}
}

func (r *craftScheduledTaskRepository) Create(ctx context.Context, task *types.CraftScheduledTask) error {
	if err := types.ValidateCronExpression(task.CronExpression); err != nil {
		return fmt.Errorf("%w: %v", craft.ErrInvalidInput, err)
	}
	if task.TenantID == 0 || task.OwnerID == "" {
		return fmt.Errorf("%w: scheduled task requires tenant and owner", craft.ErrInvalidInput)
	}
	craftScheduledDefaults(task)
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *craftScheduledTaskRepository) ListByOwner(
	ctx context.Context, tenantID uint64, ownerID string,
) ([]types.CraftScheduledTask, error) {
	var rows []types.CraftScheduledTask
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND owner_id = ?", tenantID, ownerID).
		Order("created_at DESC, id DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *craftScheduledTaskRepository) GetByID(
	ctx context.Context, tenantID uint64, ownerID, id string,
) (*types.CraftScheduledTask, error) {
	var row types.CraftScheduledTask
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND owner_id = ? AND id = ?", tenantID, ownerID, id).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, craft.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *craftScheduledTaskRepository) Update(ctx context.Context, task *types.CraftScheduledTask) error {
	if err := types.ValidateCronExpression(task.CronExpression); err != nil {
		return fmt.Errorf("%w: %v", craft.ErrInvalidInput, err)
	}
	updated := r.db.WithContext(ctx).
		Model(&types.CraftScheduledTask{}).
		Where("tenant_id = ? AND owner_id = ? AND id = ?", task.TenantID, task.OwnerID, task.ID).
		Updates(map[string]any{
			"name":            task.Name,
			"prompt":          task.Prompt,
			"cron_expression": task.CronExpression,
			"editor_mode":     task.EditorMode,
			"status":          task.Status,
			"next_run_at":     task.NextRunAt,
			"last_run_at":     task.LastRunAt,
			"updated_at":      gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return fmt.Errorf("%w: scheduled task %s", craft.ErrNotFound, task.ID)
	}
	return nil
}

func (r *craftScheduledTaskRepository) SoftDelete(
	ctx context.Context, tenantID uint64, ownerID, id string,
) error {
	// gorm's soft-delete plugin writes deleted_at on the scoped row; the
	// scoped predicate (plus its automatic deleted_at IS NULL filter) makes
	// every re-delete or wrong-owner delete a zero-row, error-free no-op.
	res := r.db.WithContext(ctx).
		Where("tenant_id = ? AND owner_id = ? AND id = ?", tenantID, ownerID, id).
		Delete(&types.CraftScheduledTask{})
	return res.Error
}

func (r *craftScheduledTaskRepository) ClaimDueTasks(
	ctx context.Context, now time.Time, batch int,
) ([]ClaimedTask, error) {
	if batch <= 0 {
		batch = craftScheduledDefaultBatch
	}
	var due []types.CraftScheduledTask
	err := r.db.WithContext(ctx).
		Where("status = ? AND next_run_at <= ?", types.CraftScheduledTaskStatusActive, now).
		Order("next_run_at ASC, id ASC").
		Limit(batch).
		Find(&due).Error
	if err != nil {
		return nil, err
	}
	claims := make([]ClaimedTask, 0, len(due))
	for i := range due {
		task := due[i]
		if task.NextRunAt == nil {
			continue
		}
		// Advance the ticket BEFORE the swap: the new value rides in the
		// same UPDATE, so the row never shows a due (or missing) ticket to
		// the next sweep between read and write.
		next, err := types.NextCronFire(task.CronExpression, now)
		if err != nil {
			// Every write entrance validates the expression, so an
			// unparseable stored cron is corrupted state: fail the sweep
			// visibly instead of silently re-reading the row every 30s.
			return nil, fmt.Errorf("craft scheduled task %s: %w", task.ID, err)
		}
		// The CAS (Ruling P-1): the row is claimed only if it still carries
		// the ticket this sweep read — active and live (gorm's soft-delete
		// filter contributes the deleted_at IS NULL arm). Zero rows means
		// another instance claimed it, it was paused or deleted, or it was
		// rescheduled: not ours, skip.
		claimed := r.db.WithContext(ctx).
			Model(&types.CraftScheduledTask{}).
			Where("id = ? AND tenant_id = ? AND next_run_at = ? AND status = ?",
				task.ID, task.TenantID, task.NextRunAt, types.CraftScheduledTaskStatusActive).
			Updates(map[string]any{
				"next_run_at": next,
				"last_run_at": task.NextRunAt,
				"updated_at":  gorm.Expr("CURRENT_TIMESTAMP"),
			})
		if claimed.Error != nil {
			return nil, claimed.Error
		}
		if claimed.RowsAffected != 1 {
			continue
		}
		task.NextRunAt = &next
		claims = append(claims, ClaimedTask{Task: task, PrevNextRunAt: *due[i].NextRunAt})
	}
	return claims, nil
}

func (r *craftScheduledTaskRepository) InsertRun(ctx context.Context, run *types.CraftScheduledTaskRun) error {
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	if run.StartedAt == nil {
		at := time.Now().UTC()
		run.StartedAt = &at
	}
	return r.db.WithContext(ctx).Create(run).Error
}

// craftNullableText maps the empty string to SQL NULL: absent terminal
// fields store NULL rather than empty placeholders.
func craftNullableText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (r *craftScheduledTaskRepository) UpdateRunStatus(
	ctx context.Context, id, status, errorClass, detail, summary string, finishedAt *time.Time,
) error {
	updates := map[string]any{
		"status":       status,
		"error_class":  craftNullableText(errorClass),
		"error_detail": craftNullableText(detail),
		"summary":      craftNullableText(summary),
		"updated_at":   gorm.Expr("CURRENT_TIMESTAMP"),
	}
	if finishedAt != nil {
		updates["finished_at"] = finishedAt
	}
	res := r.db.WithContext(ctx).
		Model(&types.CraftScheduledTaskRun{}).
		Where("id = ?", id).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return fmt.Errorf("%w: scheduled run %s", craft.ErrNotFound, id)
	}
	return nil
}

func (r *craftScheduledTaskRepository) ListRunsByTask(
	ctx context.Context, taskID string, before *time.Time, limit int,
) ([]types.CraftScheduledTaskRun, error) {
	if limit <= 0 {
		limit = craftScheduledDefaultListCap
	}
	if limit > craftScheduledMaxListCap {
		limit = craftScheduledMaxListCap
	}
	query := r.db.WithContext(ctx).Where("task_id = ?", taskID)
	if before != nil {
		query = query.Where("started_at < ?", *before)
	}
	var rows []types.CraftScheduledTaskRun
	err := query.
		Order("started_at DESC, id DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *craftScheduledTaskRepository) HasInFlightRun(ctx context.Context, taskID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&types.CraftScheduledTaskRun{}).
		Where("task_id = ? AND status IN ?", taskID, []string{
			types.CraftScheduledRunStatusQueued,
			types.CraftScheduledRunStatusRunning,
		}).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
