package handler

import (
	"errors"
	"net/http"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	"github.com/Tencent/WeKnora/internal/appconnector/publish"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AppNotionPublishHandler serves the T18 Notion publish closed loop under
// /api/v1/apps/notion-publish. Plan formation derives the approved
// snapshot SERVER-SIDE from a confirmed artifact version + destination;
// approval stays on the existing POST /apps/actions/:id/approve endpoint
// (its predicate owns approval authority); execution/reconciliation run
// through the publish service whose ActionService holds the Notion bridge
// dispatcher. The handler is nil-service fail-closed like its siblings.
type AppNotionPublishHandler struct {
	db      *gorm.DB
	publish *publish.NotionPublishService
}

// NewAppNotionPublishHandler constructs the handler over the business DB.
func NewAppNotionPublishHandler(db *gorm.DB) *AppNotionPublishHandler {
	return &AppNotionPublishHandler{db: db}
}

// SetNotionPublishService wires the publish service (container injection
// point). Until called every endpoint fails closed with 501
// PUBLISH_PIPELINE_NOT_CONFIGURED (mirroring the frozen action pipeline).
func (h *AppNotionPublishHandler) SetNotionPublishService(s *publish.NotionPublishService) {
	h.publish = s
}

// RequireActionCapabilityForWrites mirrors the action write gate
// (CanDriveActionWrites): plan formation and execution are action writes.
func (h *AppNotionPublishHandler) RequireActionCapabilityForWrites() gin.HandlerFunc {
	return appRequireWriteCapability(appconnector.CanDriveActionWrites,
		"FORBIDDEN_ACTION_WRITE",
		"publish writes require owner or admin role")
}

type notionPublishPlanInput struct {
	ConnectionID      string `json:"connection_id"`
	SessionID         string `json:"session_id"`
	ArtifactVersionID string `json:"artifact_version_id"`
	Title             string `json:"title"`
	ParentPageID      string `json:"parent_page_id"`
	PageID            string `json:"page_id"`
}

// notionActionByID resolves an action inside the tenant AND inside the
// notion publish family (feishu/confluence share the same stores): a
// cross-family id is the uniform 404 and never consumes the other
// pipeline's approved action (B5-F42/F62).
func (h *AppNotionPublishHandler) notionActionByID(c *gin.Context, tenantID uint64, id string) (appconnectorrepo.ActionRow, bool) {
	var row appconnectorrepo.ActionRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
		appFail(c, http.StatusNotFound, "ACTION_NOT_FOUND", "action not found")
		return row, false
	}
	if !publishActionOfFamily(c, h.db, tenantID, id, "notion") {
		return row, false
	}
	return row, true
}

func (h *AppNotionPublishHandler) notionConnection(c *gin.Context, tenantID uint64, connectionID, userID string) (appconnectorrepo.ConnectionRow, bool) {
	var conn appconnectorrepo.ConnectionRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, connectionID).First(&conn).Error; err != nil {
		appFail(c, http.StatusNotFound, "CONNECTION_NOT_FOUND", "connection not found")
		return conn, false
	}
	// A personal connection is its owner's identity (same predicate as
	// PrepareAction / #42).
	if conn.Kind == appconnector.ConnectionKindPersonal && conn.OwnerID != userID {
		appFail(c, http.StatusForbidden, "NOT_CONNECTION_OWNER",
			"a personal connection may only be used by its owner")
		return conn, false
	}
	return conn, true
}

// FormNotionPublishPlan POST /apps/notion-publish/plans — derives the
// approved publish snapshot server-side (artifact version + destination),
// reads the external current version, prepares the A03 action and records
// the planned publication. The response carries the digest + fence an
// approval binds.
func (h *AppNotionPublishHandler) FormNotionPublishPlan(c *gin.Context) {
	tenantID, _, userID, ok := appTenantScope(c)
	if !ok {
		return
	}
	var input notionPublishPlanInput
	if err := c.ShouldBindJSON(&input); err != nil || input.ConnectionID == "" || input.SessionID == "" ||
		input.ArtifactVersionID == "" || input.Title == "" ||
		(input.ParentPageID == "") == (input.PageID == "") {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST",
			"connection_id, session_id, artifact_version_id, title and exactly one of parent_page_id / page_id are required")
		return
	}
	if _, ok := h.notionConnection(c, tenantID, input.ConnectionID, userID); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Notion publish pipeline is not wired in this environment; refusing to form a plan")
		return
	}
	view, err := h.publish.FormPlan(c.Request.Context(), publish.PublishPlanInput{
		TenantID: tenantID, ActorID: userID, ConnectionID: input.ConnectionID,
		SessionID: input.SessionID, ArtifactVersionID: input.ArtifactVersionID,
		Title: input.Title, ParentPageID: input.ParentPageID, PageID: input.PageID,
	})
	if err != nil {
		h.failPublish(c, err)
		return
	}
	appOK(c, http.StatusCreated, view)
}

// PublishNotionAction POST /apps/notion-publish/actions/:id/publish —
// executes the APPROVED plan through the publish service. An unobservable
// provider outcome returns 200 with the parked unknown state (mirror of
// ExecuteAction); reconciliation is the only resolution path.
func (h *AppNotionPublishHandler) PublishNotionAction(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	if _, ok := h.notionActionByID(c, tenantID, c.Param("id")); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Notion publish pipeline is not wired in this environment; refusing to dispatch")
		return
	}
	outcome, err := h.publish.Execute(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		h.failPublishExecute(c, err)
		return
	}
	if outcome.Conflict {
		appFail(c, http.StatusConflict, "PUBLISH_VERSION_CONFLICT",
			"the external document changed since approval; form a new plan from its current version")
		return
	}
	appOK(c, http.StatusOK, outcome)
}

// ReconcileNotionAction POST /apps/notion-publish/actions/:id/reconcile —
// resolves an unknown outcome by querying the provider first (AC2).
func (h *AppNotionPublishHandler) ReconcileNotionAction(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	if _, ok := h.notionActionByID(c, tenantID, c.Param("id")); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Notion publish pipeline is not wired in this environment; refusing to reconcile")
		return
	}
	outcome, err := h.publish.Reconcile(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		h.failPublishExecute(c, err)
		return
	}
	appOK(c, http.StatusOK, outcome)
}

// GetNotionPublication GET /apps/notion-publish/actions/:id — the plan +
// receipt view (计划/回执查询).
func (h *AppNotionPublishHandler) GetNotionPublication(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	if _, ok := h.notionActionByID(c, tenantID, c.Param("id")); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Notion publish pipeline is not wired in this environment")
		return
	}
	view, err := h.publish.Receipt(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		h.failPublish(c, err)
		return
	}
	appOK(c, http.StatusOK, view)
}

func (h *AppNotionPublishHandler) failPublish(c *gin.Context, err error) {
	switch {
	case errors.Is(err, publish.ErrPublishInvalidInput):
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid publish plan input")
	case errors.Is(err, publish.ErrPublishArtifactNotReady):
		appFail(c, http.StatusNotFound, "ARTIFACT_VERSION_NOT_ACCESSIBLE", "the artifact version is not readable for this session")
	case errors.Is(err, publish.ErrPublishUnsupportedArtifact):
		appFail(c, http.StatusUnsupportedMediaType, "PUBLISH_UNSUPPORTED_ARTIFACT", "only text artifacts are publishable in this version")
	case errors.Is(err, publish.ErrPublishContentTooLarge):
		appFail(c, http.StatusRequestEntityTooLarge, "PUBLISH_CONTENT_TOO_LARGE", "the artifact exceeds the publish bounds")
	case errors.Is(err, publish.ErrPublishEmptyContent):
		appFail(c, http.StatusBadRequest, "PUBLISH_EMPTY_CONTENT", "the artifact carries no publishable text")
	case errors.Is(err, publish.ErrPublishDestinationOutOfScope):
		appFail(c, http.StatusForbidden, "PUBLISH_DESTINATION_OUT_OF_SCOPE", "the destination is not in the reviewed scope for this connection")
	case errors.Is(err, publish.ErrPublishDestinationUnreadable):
		appFail(c, http.StatusBadGateway, "PUBLISH_DESTINATION_UNREADABLE", "the external destination could not be read; no plan was formed")
	case errors.Is(err, publish.ErrPublishUpdateTargetNotPublished):
		appFail(c, http.StatusConflict, "PUBLISH_UPDATE_TARGET_NOT_PUBLISHED", "only pages this workspace published through this connection may be updated")
	default:
		appFail(c, http.StatusInternalServerError, "PUBLISH_PLAN_FAILED", "failed to form the publish plan")
	}
}

func (h *AppNotionPublishHandler) failPublishExecute(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appconnectorsvc.ErrActionState):
		appFail(c, http.StatusConflict, "ACTION_STATE_CONFLICT", "action is not in the state this operation requires")
	case errors.Is(err, appconnectorsvc.ErrNoDispatcher):
		appFail(c, http.StatusServiceUnavailable, "OC_DISPATCH_NOT_CONFIGURED",
			"no outbound dispatcher is configured in this deployment; refusing to fabricate a dispatch")
	default:
		appFail(c, http.StatusInternalServerError, "PUBLISH_EXECUTE_FAILED", "publish execution failed")
	}
}
