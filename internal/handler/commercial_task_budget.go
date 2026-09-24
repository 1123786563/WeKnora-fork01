package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
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
	appOK(c, http.StatusOK, gin.H{
		"task_id":            runID,
		"additional_credits": *input.AdditionalCredits,
	})
}

// taskRunOwner resolves the business owner of one run (agent_runs.owner_id)
// inside the tenant. found=false covers unknown and cross-tenant runs alike.
func (h *CommercialHandler) taskRunOwner(ctx context.Context, tenantID uint64, runID string) (string, bool, error) {
	if h == nil || h.db == nil || tenantID == 0 || runID == "" {
		return "", false, nil
	}
	var ownerID string
	err := h.db.WithContext(ctx).Table("agent_runs").
		Where("tenant_id = ? AND run_id = ?", tenantID, runID).
		Select("owner_id").Scan(&ownerID).Error
	if err != nil {
		return "", false, err
	}
	if ownerID == "" {
		return "", false, nil
	}
	return ownerID, true, nil
}
