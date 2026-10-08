package handler

import (
	"errors"
	"net/http"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	"github.com/Tencent/WeKnora/internal/appconnector/plan"
	"github.com/Tencent/WeKnora/internal/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AppActionPlanHandler serves the T21 (#51) multi-action Action Plan
// endpoints under /api/v1/apps/action-plans: formation (server-derived
// per-item snapshots through the #48 publish seam), whole-plan approval
// with 排除单项, ordered partial-success execution and the per-item
// result projection. The handler is nil-service fail-closed like its
// siblings.
type AppActionPlanHandler struct {
	db    *gorm.DB
	plans *plan.Service
}

// NewAppActionPlanHandler constructs the handler over the business DB.
func NewAppActionPlanHandler(db *gorm.DB) *AppActionPlanHandler {
	return &AppActionPlanHandler{db: db}
}

// SetActionPlanService wires the plan service (container injection
// point). Until called every endpoint fails closed with 501
// ACTION_PLAN_PIPELINE_NOT_CONFIGURED.
func (h *AppActionPlanHandler) SetActionPlanService(s *plan.Service) { h.plans = s }

// RequireActionCapabilityForWrites mirrors the action write gate
// (CanDriveActionWrites): plan formation, approval and execution are
// action writes.
func (h *AppActionPlanHandler) RequireActionCapabilityForWrites() gin.HandlerFunc {
	return appRequireWriteCapability(appconnector.CanDriveActionWrites,
		"FORBIDDEN_ACTION_WRITE",
		"plan writes require owner or admin role")
}

type actionPlanItemInput struct {
	ConnectionID      string `json:"connection_id"`
	SessionID         string `json:"session_id"`
	ArtifactVersionID string `json:"artifact_version_id"`
	Title             string `json:"title"`
	ParentPageID      string `json:"parent_page_id"`
	PageID            string `json:"page_id"`
}

type actionPlanFormInput struct {
	Items []actionPlanItemInput `json:"items"`
}

type actionPlanApproveInput struct {
	Digest      string `json:"digest"`
	ExcludeSeqs []int  `json:"exclude_seqs"`
}

type actionPlanExecuteInput struct {
	Digest string `json:"digest"`
}

// planByID resolves a plan inside the tenant — a cross-tenant id and a
// missing one are both 404, never a 403 that leaks existence.
func (h *AppActionPlanHandler) planByID(c *gin.Context, tenantID uint64, id string) (repoappconn.ActionPlanRow, bool) {
	var row repoappconn.ActionPlanRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
		appFail(c, http.StatusNotFound, "ACTION_PLAN_NOT_FOUND", "action plan not found")
		return row, false
	}
	return row, true
}

// FormActionPlan POST /apps/action-plans — forms the multi-action plan
// through the #48 per-item formation; the response carries the plan
// digest an approval binds.
func (h *AppActionPlanHandler) FormActionPlan(c *gin.Context) {
	tenantID, _, userID, ok := appTenantScope(c)
	if !ok {
		return
	}
	var input actionPlanFormInput
	if err := c.ShouldBindJSON(&input); err != nil || len(input.Items) == 0 {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "at least one plan item is required")
		return
	}
	if h.plans == nil {
		appFail(c, http.StatusNotImplemented, "ACTION_PLAN_PIPELINE_NOT_CONFIGURED",
			"the action plan pipeline is not wired in this environment; refusing to fabricate a plan")
		return
	}
	items := make([]plan.ItemInput, 0, len(input.Items))
	for _, it := range input.Items {
		items = append(items, plan.ItemInput{
			ConnectionID: it.ConnectionID, SessionID: it.SessionID,
			ArtifactVersionID: it.ArtifactVersionID, Title: it.Title,
			ParentPageID: it.ParentPageID, PageID: it.PageID,
		})
	}
	view, err := h.plans.FormPlan(c.Request.Context(), plan.FormInput{
		TenantID: tenantID, ActorID: userID, Items: items})
	if err != nil {
		h.failForm(c, err)
		return
	}
	appOK(c, http.StatusCreated, view)
}

// ApproveActionPlan POST /apps/action-plans/:id/approve — the owner's
// whole-plan decision: approve everything or exclude individual items
// (排除单项). Approval authority: the plan's initiator or a tenant
// owner/admin — the initiator necessarily owns every personal connection
// the plan uses (the formation connection check enforces it,
// app_connector_notion_publish.go:77), and sharing a task never
// delegates approval of its owner's side effects (CONTEXT.md 任务协作者).
func (h *AppActionPlanHandler) ApproveActionPlan(c *gin.Context) {
	tenantID, role, userID, ok := appTenantScope(c)
	if !ok {
		return
	}
	row, ok := h.planByID(c, tenantID, c.Param("id"))
	if !ok {
		return
	}
	var input actionPlanApproveInput
	if err := c.ShouldBindJSON(&input); err != nil || input.Digest == "" {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "digest is required")
		return
	}
	if row.ActorID != userID && !appconnector.CanDriveActionWrites(role) {
		appFail(c, http.StatusForbidden, "ACTION_APPROVAL_FORBIDDEN",
			"approving this plan requires its initiator or tenant owner/admin")
		return
	}
	if h.plans == nil {
		appFail(c, http.StatusNotImplemented, "ACTION_PLAN_PIPELINE_NOT_CONFIGURED",
			"the action plan pipeline is not wired in this environment; refusing to fabricate an approval")
		return
	}
	view, err := h.plans.Approve(c.Request.Context(), tenantID, row.ID, userID,
		plan.ApproveInput{Digest: input.Digest, ExcludeSeqs: input.ExcludeSeqs})
	if err != nil {
		h.failPlan(c, err)
		return
	}
	appOK(c, http.StatusOK, view)
}

// ExecuteActionPlan POST /apps/action-plans/:id/execute — one ordered
// pass over the approved plan; the presented digest must match the plan
// content (AC1 at execute time). Per-item outcomes ride the 200 payload.
func (h *AppActionPlanHandler) ExecuteActionPlan(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	row, ok := h.planByID(c, tenantID, c.Param("id"))
	if !ok {
		return
	}
	var input actionPlanExecuteInput
	if err := c.ShouldBindJSON(&input); err != nil || input.Digest == "" {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "digest is required")
		return
	}
	if h.plans == nil {
		appFail(c, http.StatusNotImplemented, "ACTION_PLAN_PIPELINE_NOT_CONFIGURED",
			"the action plan pipeline is not wired in this environment; refusing to dispatch")
		return
	}
	out, err := h.plans.Execute(c.Request.Context(), tenantID, row.ID, input.Digest)
	if err != nil {
		h.failPlan(c, err)
		return
	}
	appOK(c, http.StatusOK, out)
}

// GetActionPlan GET /apps/action-plans/:id — the durable plan projection
// with per-item authoritative states and receipts.
func (h *AppActionPlanHandler) GetActionPlan(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	row, ok := h.planByID(c, tenantID, c.Param("id"))
	if !ok {
		return
	}
	if h.plans == nil {
		appFail(c, http.StatusNotImplemented, "ACTION_PLAN_PIPELINE_NOT_CONFIGURED",
			"the action plan pipeline is not wired in this environment")
		return
	}
	st, err := h.plans.Status(c.Request.Context(), tenantID, row.ID)
	if err != nil {
		h.failPlan(c, err)
		return
	}
	appOK(c, http.StatusOK, st)
}

// failForm maps one plan-formation refusal onto the fixed code table —
// the per-item publish sentinels keep their #48 wire meanings.
func (h *AppActionPlanHandler) failForm(c *gin.Context, err error) {
	switch {
	case errors.Is(err, plan.ErrPlanInvalidInput):
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid action plan input")
	case errors.Is(err, publish.ErrPublishInvalidInput):
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid publish plan input in one of the items")
	case errors.Is(err, publish.ErrPublishArtifactNotReady):
		appFail(c, http.StatusNotFound, "ARTIFACT_VERSION_NOT_ACCESSIBLE", "an artifact version is not readable for this session")
	case errors.Is(err, publish.ErrPublishUnsupportedArtifact):
		appFail(c, http.StatusUnsupportedMediaType, "PUBLISH_UNSUPPORTED_ARTIFACT", "only text artifacts are publishable in this version")
	case errors.Is(err, publish.ErrPublishContentTooLarge):
		appFail(c, http.StatusRequestEntityTooLarge, "PUBLISH_CONTENT_TOO_LARGE", "an item's artifact exceeds the publish bounds")
	case errors.Is(err, publish.ErrPublishEmptyContent):
		appFail(c, http.StatusBadRequest, "PUBLISH_EMPTY_CONTENT", "an item's artifact carries no publishable text")
	case errors.Is(err, publish.ErrPublishDestinationOutOfScope):
		appFail(c, http.StatusForbidden, "PUBLISH_DESTINATION_OUT_OF_SCOPE", "a destination is not in the reviewed scope for its connection")
	case errors.Is(err, publish.ErrPublishDestinationUnreadable):
		appFail(c, http.StatusBadGateway, "PUBLISH_DESTINATION_UNREADABLE", "an external destination could not be read; no plan was formed")
	case errors.Is(err, publish.ErrPublishUpdateTargetNotPublished):
		appFail(c, http.StatusConflict, "PUBLISH_UPDATE_TARGET_NOT_PUBLISHED", "only pages published through this connection may be updated")
	default:
		appFail(c, http.StatusInternalServerError, "ACTION_PLAN_FORM_FAILED", "failed to form the action plan")
	}
}

// failPlan maps one approve/execute/read refusal onto the fixed code
// table. Messages are static on purpose: no upstream error text or
// provider detail ever crosses the wire.
func (h *AppActionPlanHandler) failPlan(c *gin.Context, err error) {
	switch {
	case errors.Is(err, plan.ErrPlanInvalidInput):
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid action plan input")
	case errors.Is(err, repoappconn.ErrPlanNotFound):
		appFail(c, http.StatusNotFound, "ACTION_PLAN_NOT_FOUND", "action plan not found")
	case errors.Is(err, plan.ErrPlanDigestMismatch):
		// AC1 on the wire: an approval/request issued for different plan
		// content is a conflict with its OWN code, never a generic 500.
		appFail(c, http.StatusConflict, "ACTION_PLAN_DIGEST_MISMATCH",
			"the digest was issued for different plan content; form and approve the plan again")
	case errors.Is(err, plan.ErrPlanState):
		appFail(c, http.StatusConflict, "ACTION_PLAN_STATE_CONFLICT", "action plan is not in the state this operation requires")
	case errors.Is(err, appconnectorsvc.ErrActionDigestMismatch):
		appFail(c, http.StatusConflict, "ACTION_DIGEST_MISMATCH", "an item's approval was issued for different content")
	case errors.Is(err, appconnectorsvc.ErrActionState):
		appFail(c, http.StatusConflict, "ACTION_STATE_CONFLICT", "an item is not in the state this operation requires")
	case errors.Is(err, appconnectorsvc.ErrNoDispatcher):
		appFail(c, http.StatusServiceUnavailable, "OC_DISPATCH_NOT_CONFIGURED",
			"no outbound dispatcher is configured in this deployment; refusing to fabricate a dispatch")
	default:
		appFail(c, http.StatusInternalServerError, "ACTION_PLAN_EXECUTE_FAILED", "action plan operation failed")
	}
}
