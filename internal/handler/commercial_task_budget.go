package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/commercial/service/commercial"
	"github.com/gin-gonic/gin"
)

// ExtendTaskBudget POST /commercial/tasks/:id/budget/extend (W05).
//
// It CALLS the U04 BudgetService.Extend semantics: entitlement pause with
// an explicit reason, exactly-once increase per idempotency key, expired
// or missing task budgets refused. The entitlement check (F02 facts) is
// not yet wired by the container, so the service is built per request
// with a nil EntitlementCheck — which U04 defines as "allow" — a disclosed
// gap, not a fabricated gate. Tenant scope always comes from the
// authenticated context; raising a task budget is a BUDGET decision only
// and never authorizes an external write (that is the separate A03
// approval on /apps/actions).
func (h *CommercialHandler) ExtendTaskBudget(c *gin.Context) {
	tenantID, role, ok := commercialTenantScope(c)
	if !ok {
		appFail(c, http.StatusForbidden, "MISSING_TENANT_SCOPE", ErrMissingTenantScope.Error())
		return
	}
	var input struct {
		AdditionalCredits *int64 `json:"additional_credits"`
		IdempotencyKey    string `json:"idempotency_key"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.AdditionalCredits == nil ||
		*input.AdditionalCredits <= 0 || input.IdempotencyKey == "" {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST",
			"additional_credits (positive integer) and idempotency_key are required")
		return
	}
	if h.db == nil {
		appFail(c, http.StatusServiceUnavailable, "BUDGET_DATABASE_MISSING", "budget storage is unavailable")
		return
	}
	svc, err := commercialsvc.NewBudgetService(h.db, nil, nil)
	if err != nil {
		appFail(c, http.StatusServiceUnavailable, "BUDGET_SERVICE_UNAVAILABLE", "budget service is unavailable")
		return
	}
	runID := c.Param("id")
	// T12 (#42): a budget raise admits the TASK OWNER or a billing-authorized
	// caller (CONTEXT.md 任务预算). The gate runs BEFORE the service so a
	// collaborator is refused ahead of any idempotency replay. found=false
	// (unknown or cross-tenant run) deliberately falls through so the service
	// keeps its explicit TASK_BUDGET_NOT_FOUND 404 contract.
	userID := commercialUserID(c)
	if !commercial.CanManageBilling(role, true, h.hasBillingGrant(c, tenantID)) {
		ownerID, found, ownerErr := h.taskRunOwner(c.Request.Context(), tenantID, runID)
		if ownerErr != nil {
			appFail(c, http.StatusInternalServerError, "BUDGET_OWNER_LOOKUP_FAILED", "failed to resolve the task owner")
			return
		}
		if found && ownerID != userID {
			appFail(c, http.StatusForbidden, "BUDGET_FORBIDDEN",
				"raising a task budget requires the task owner or billing authority")
			return
		}
	}
	if err := svc.Extend(c.Request.Context(), tenantID, runID, input.IdempotencyKey,
		commercial.Credits(*input.AdditionalCredits)); err != nil {
		switch {
		case errors.Is(err, commercialsvc.ErrBudgetUnauthorized):
			// Entitlement lost (e.g. space downgrade): an explicit pause
			// reason, never a silent success.
			appFail(c, http.StatusForbidden, "BUDGET_UNAUTHORIZED", err.Error())
		case errors.Is(err, repocommercial.ErrTaskBudgetMissing):
			appFail(c, http.StatusNotFound, "TASK_BUDGET_NOT_FOUND", "task budget not found")
		case errors.Is(err, repocommercial.ErrTaskBudgetExpired):
			appFail(c, http.StatusConflict, "TASK_BUDGET_EXPIRED", "task budget validity has expired; extension never extends the deadline")
		case errors.Is(err, repocommercial.ErrInvalidBudgetRequest):
			appFail(c, http.StatusBadRequest, "INVALID_BUDGET_REQUEST", "invalid budget extension request")
		case errors.Is(err, repocommercial.ErrBudgetAccountMissing),
			errors.Is(err, repocommercial.ErrInsufficientBudget),
			errors.Is(err, repocommercial.ErrBudgetVerificationExpired):
			appFail(c, http.StatusConflict, "BUDGET_INSUFFICIENT", "no funded budget headroom for this extension")
		default:
			appFail(c, http.StatusInternalServerError, "BUDGET_EXTEND_FAILED", "failed to extend task budget")
		}
		return
	}
	// T09 (#39): an AUTHORIZED raise also resumes the runs this budget parked
	// — the durable same-Run continuation. The requeue runs on EVERY success,
	// idempotent replays included: the guarded predicate makes the replay a
	// no-op unless a run stranded parked (healing a lost first requeue)
	// without ever raising the limit twice. A refused caller never reaches
	// here, so a collaborator can never resume anything.
	resumed := int64(0)
	if root, found, rerr := h.taskBudgetRoot(c.Request.Context(), tenantID, runID); rerr == nil && found {
		if n, qerr := repository.NewAgentRunStore(h.db).
			RequeueBudgetPausedRuns(c.Request.Context(), tenantID, root); qerr == nil {
			resumed = n
		} else {
			// T09 (#39) fix round 1: the extension already committed (200
			// below), so the swallowed requeue failure must stay OBSERVABLE —
			// a stranded park must not vanish silently. GET /budget's
			// paused_run_ids remains the operator-facing fallback.
			logger.Warnf(c.Request.Context(),
				"budget extension requeue failed: tenant=%d root=%s err=%v", tenantID, root, qerr)
		}
	} else if rerr != nil {
		logger.Warnf(c.Request.Context(),
			"budget extension root lookup failed: tenant=%d run=%s err=%v", tenantID, runID, rerr)
	}
	appOK(c, http.StatusOK, gin.H{
		"task_id":            runID,
		"additional_credits": *input.AdditionalCredits,
		"resumed_runs":       resumed,
	})
}

// taskRunOwner resolves the business owner of one run inside the tenant. The
// authority is sessions.user_id joined through agent_runs.session_id (ADR-0004,
// same as TaskGrantStore.TaskOwnerID) — agent_runs.owner_id is merely the run
// CREATOR, which a collaborator can be, so it must never gate the budget raise
// (B3-F83). found=false covers unknown and cross-tenant runs alike.
func (h *CommercialHandler) taskRunOwner(ctx context.Context, tenantID uint64, runID string) (string, bool, error) {
	if h == nil || h.db == nil || tenantID == 0 || runID == "" {
		return "", false, nil
	}
	var ownerID sql.NullString
	err := h.db.WithContext(ctx).Table("agent_runs").
		Select("sessions.user_id").
		Joins("JOIN sessions ON sessions.id = agent_runs.session_id AND sessions.tenant_id = ?", tenantID).
		Where("agent_runs.tenant_id = ? AND agent_runs.run_id = ?", tenantID, runID).
		Scan(&ownerID).Error
	if err != nil {
		return "", false, err
	}
	if !ownerID.Valid || strings.TrimSpace(ownerID.String) == "" {
		return "", false, nil
	}
	return ownerID.String, true, nil
}

// taskBudgetRoot resolves the budget ROOT of one run row: an empty
// RootRunID marks the owner row itself. found=false covers unknown and
// cross-tenant runs alike.
func (h *CommercialHandler) taskBudgetRoot(ctx context.Context, tenantID uint64, runID string) (string, bool, error) {
	if h == nil || h.db == nil || tenantID == 0 || runID == "" {
		return "", false, nil
	}
	var row struct {
		RunID     string
		RootRunID string
	}
	err := h.db.WithContext(ctx).Table("commercial_task_budgets").
		Select("run_id, root_run_id").
		Where("tenant_id = ? AND run_id = ?", tenantID, runID).Scan(&row).Error
	if err != nil {
		return "", false, err
	}
	if row.RunID == "" {
		return "", false, nil
	}
	if row.RootRunID == "" {
		return row.RunID, true, nil
	}
	return row.RootRunID, true, nil
}

// taskBudgetGrantedReader reports whether userID holds ANY #42 task grant on
// the task that owns the budget root (task_id = the root run's session) while
// being an active member. A grant never carries budget authority — it admits
// the READ only (can_extend stays false).
func (h *CommercialHandler) taskBudgetGrantedReader(ctx context.Context, tenantID uint64, rootRunID, userID string) bool {
	if h == nil || h.db == nil || tenantID == 0 || rootRunID == "" || userID == "" {
		return false
	}
	var n int64
	err := h.db.WithContext(ctx).Table("task_grants tg").
		Joins("JOIN agent_runs root ON root.tenant_id = tg.tenant_id AND root.session_id = tg.task_id").
		Joins("JOIN tenant_members tm ON tm.tenant_id = tg.tenant_id AND tm.user_id = tg.grantee_id AND tm.status = 'active' AND tm.deleted_at IS NULL").
		Where("root.tenant_id = ? AND root.run_id = ? AND tg.grantee_id = ?", tenantID, rootRunID, userID).
		Count(&n).Error
	return err == nil && n > 0
}

// GetTaskBudget GET /commercial/tasks/:id/budget (T09 #39). The four numbers
// come from the ROOT budget row — a delegated child run charges its parent's
// budget exactly once (G4 Reserve resolves the root), so the root row already
// aggregates delegated spend. Estimates never fabricate: an absent row is the
// explicit 404, never zero-filled numbers.
func (h *CommercialHandler) GetTaskBudget(c *gin.Context) {
	tenantID, role, ok := commercialTenantScope(c)
	if !ok {
		appFail(c, http.StatusForbidden, "MISSING_TENANT_SCOPE", ErrMissingTenantScope.Error())
		return
	}
	if h.db == nil {
		appFail(c, http.StatusServiceUnavailable, "BUDGET_DATABASE_MISSING", "budget storage is unavailable")
		return
	}
	runID := c.Param("id")
	root, found, err := h.taskBudgetRoot(c.Request.Context(), tenantID, runID)
	if err != nil {
		appFail(c, http.StatusInternalServerError, "BUDGET_READ_FAILED", "failed to resolve the task budget")
		return
	}
	if !found {
		appFail(c, http.StatusNotFound, "TASK_BUDGET_NOT_FOUND", "task budget not found")
		return
	}
	userID := commercialUserID(c)
	billing := commercial.CanManageBilling(role, true, h.hasBillingGrant(c, tenantID))
	owner := false
	if !billing {
		var ownerErr error
		owner, ownerErr = h.ownerMatches(c.Request.Context(), tenantID, root, userID)
		if ownerErr != nil {
			appFail(c, http.StatusInternalServerError, "BUDGET_OWNER_LOOKUP_FAILED", "failed to resolve the task owner")
			return
		}
		if !owner && !h.taskBudgetGrantedReader(c.Request.Context(), tenantID, root, userID) {
			appFail(c, http.StatusForbidden, "BUDGET_FORBIDDEN",
				"reading a task budget requires the task owner, a task grant or billing authority")
			return
		}
	}
	var row struct {
		LimitMicro int64
		SpentMicro int64
		HeldMicro  int64
		Deadline   time.Time
	}
	if err := h.db.WithContext(c.Request.Context()).Table("commercial_task_budgets").
		Select("limit_micro, spent_micro, held_micro, deadline").
		Where("tenant_id = ? AND run_id = ?", tenantID, root).Scan(&row).Error; err != nil {
		appFail(c, http.StatusInternalServerError, "BUDGET_READ_FAILED", "failed to read the task budget")
		return
	}
	var delegated []string
	if err := h.db.WithContext(c.Request.Context()).Table("commercial_task_budgets").
		Where("tenant_id = ? AND root_run_id = ?", tenantID, root).
		Order("run_id").Pluck("run_id", &delegated).Error; err != nil {
		appFail(c, http.StatusInternalServerError, "BUDGET_READ_FAILED", "failed to read delegated runs")
		return
	}
	var paused []string
	if err := h.db.WithContext(c.Request.Context()).Table("agent_runs").
		Where("tenant_id = ? AND status = ? AND wait_reason = ?", tenantID, "waiting_user", "budget_exhausted").
		Where("run_id = ? OR run_id IN (SELECT run_id FROM commercial_task_budgets WHERE tenant_id = ? AND root_run_id = ?)",
			root, tenantID, root).
		Order("run_id").Pluck("run_id", &paused).Error; err != nil {
		appFail(c, http.StatusInternalServerError, "BUDGET_READ_FAILED", "failed to read paused runs")
		return
	}
	data := gin.H{
		"task_id":           runID,
		"root_run_id":       root,
		"limit_credits":     row.LimitMicro,
		"used_credits":      row.SpentMicro,
		"held_credits":      row.HeldMicro,
		"remaining_credits": row.LimitMicro - row.SpentMicro - row.HeldMicro,
		"delegated_run_ids": delegated,
		"paused_run_ids":    paused,
		"can_extend":        billing || owner,
	}
	if !row.Deadline.IsZero() {
		data["deadline"] = row.Deadline.UTC().Format(time.RFC3339)
	}
	if delegated == nil {
		data["delegated_run_ids"] = []string{}
	}
	if paused == nil {
		data["paused_run_ids"] = []string{}
	}
	appOK(c, http.StatusOK, data)
}

// ownerMatches reports whether userID is the business owner of the run's task
// (sessions.user_id authority, ADR-0004 — taskRunOwner returning the verdict).
func (h *CommercialHandler) ownerMatches(ctx context.Context, tenantID uint64, runID, userID string) (bool, error) {
	ownerID, found, err := h.taskRunOwner(ctx, tenantID, runID)
	if err != nil {
		return false, err
	}
	return found && ownerID == userID, nil
}
