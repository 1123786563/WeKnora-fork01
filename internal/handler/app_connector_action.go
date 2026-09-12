package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/application/service/appconnector"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AppActionHandler serves the W05 external action approval endpoints
// (A03 pipeline) under /api/v1/apps/actions.
type AppActionHandler struct {
	db *gorm.DB
	// actions is the A03 approval pipeline; nil until the container wires
	// it, in which case every action WRITE endpoint fails closed.
	actions *appconnectorsvc.ActionService
}

func NewAppActionHandler(db *gorm.DB) *AppActionHandler {
	return &AppActionHandler{db: db}
}

// SetActionService wires the A03 ActionService (injection point for the
// container). Until it is called, prepare/approve/execute fail closed with
// 501 ACTION_PIPELINE_NOT_CONFIGURED — the HTTP edge never fabricates an
// approval or a dispatch.
func (h *AppActionHandler) SetActionService(s *appconnectorsvc.ActionService) { h.actions = s }

// RequireActionCapabilityForWrites gates every non-read method on the
// action routes (prepare/approve/execute) with
// appconnector.CanDriveActionWrites. Action approval is its own lifecycle
// (B12/B13: approvals bind the actual target and parameters at the service
// layer); the HTTP write gate stays declared separately from installation
// authority so action policy can follow the resource-permission matrix
// independently.
func (h *AppActionHandler) RequireActionCapabilityForWrites() gin.HandlerFunc {
	return appRequireWriteCapability(appconnector.CanDriveActionWrites,
		"FORBIDDEN_ACTION_WRITE",
		"action writes require owner or admin role")
}

// appActionStates is the fixed A03 lifecycle vocabulary
// (internal/appconnector/action.go:30-36). A persisted state outside it
// fails closed instead of being guessed into a display state.
var appActionStates = map[string]bool{
	appconnector.ActionAwaitingApproval: true,
	appconnector.ActionAuthorized:       true,
	appconnector.ActionQueued:           true,
	appconnector.ActionDispatched:       true,
	appconnector.ActionSucceeded:        true,
	appconnector.ActionFailed:           true,
	appconnector.ActionUnknown:          true,
}

// appActionRisks mirrors the A03 risk categories: reads are safe, writes
// may be grant-covered, send/delete always need explicit approval.
var appActionRisks = map[string]bool{
	appconnector.RiskRead: true, appconnector.RiskWrite: true,
	appconnector.RiskSend: true, appconnector.RiskDelete: true,
}

// appActionView is the wire projection of one action: state (A03
// vocabulary), the digest an approval is bound to, the exact target and
// content snapshot, and a display connection name. It is owner-free: no
// actor/owner identity crosses the wire.
type appActionView struct {
	ID             string `json:"id"`
	State          string `json:"state"`
	Digest         string `json:"digest"`
	Target         string `json:"target"`
	Content        string `json:"content"`
	ConnectionName string `json:"connection_name"`
}

type appActionDetailView struct {
	Action          appActionView `json:"action"`
	ExpectedVersion int64         `json:"expected_version"`
}

// appActionByID resolves an action inside the tenant. A cross-tenant id
// and a missing one are both 404 — never a 403 that leaks existence.
func (h *AppActionHandler) appActionByID(c *gin.Context, tenantID uint64, id string) (appconnectorrepo.ActionRow, bool) {
	var row appconnectorrepo.ActionRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
		appFail(c, http.StatusNotFound, "ACTION_NOT_FOUND", "action not found")
		return row, false
	}
	return row, true
}

// actionDetailFor projects a persisted row, deriving a display connection
// name from the tenant-scoped connection row (kind:id) with the raw
// connection id as fallback when the row is gone.
func (h *AppActionHandler) actionDetailFor(c *gin.Context, tenantID uint64, row appconnectorrepo.ActionRow) appActionDetailView {
	name := row.ConnectionID
	var conn appconnectorrepo.ConnectionRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, row.ConnectionID).First(&conn).Error; err == nil {
		name = conn.Kind + ":" + conn.ID
	}
	return appActionDetailView{
		Action: appActionView{
			ID: row.ID, State: row.State, Digest: row.ArgsDigest,
			Target: row.Target, Content: row.ArgsSnapshot, ConnectionName: name,
		},
		ExpectedVersion: row.Fence,
	}
}

type appActionPrepareInput struct {
	ConnectionID string `json:"connection_id"`
	Target       string `json:"target"`
	Risk         string `json:"risk"`
	Content      string `json:"content"`
	AppVersion   string `json:"app_version"`
}

// PrepareAction POST /apps/actions/prepare — snapshots the exact call and
// parks it in awaiting_approval with a fresh digest. The connection must
// exist in this tenant; its CURRENT auth_version is bound into the digest
// so a later permission bump invalidates reuse. Fails closed (501) while
// the A03 service is not wired.
func (h *AppActionHandler) PrepareAction(c *gin.Context) {
	tenantID, _, userID, ok := appTenantScope(c)
	if !ok {
		return
	}
	var input appActionPrepareInput
	if err := c.ShouldBindJSON(&input); err != nil || input.ConnectionID == "" || input.Target == "" ||
		!appActionRisks[input.Risk] || input.Content == "" {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST",
			"connection_id, target, a known risk (read/write/send/delete) and non-empty content are required")
		return
	}
	var conn appconnectorrepo.ConnectionRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, input.ConnectionID).First(&conn).Error; err != nil {
		appFail(c, http.StatusNotFound, "CONNECTION_NOT_FOUND", "connection not found")
		return
	}
	if h.actions == nil {
		appFail(c, http.StatusNotImplemented, "ACTION_PIPELINE_NOT_CONFIGURED",
			"the A03 action approval pipeline is not wired in this environment; refusing to fabricate an approval")
		return
	}
	id, err := h.actions.Prepare(c.Request.Context(), appconnector.Action{
		TenantID: tenantID, ActorID: userID, ConnectionID: input.ConnectionID,
		Version: input.AppVersion, Target: input.Target, Risk: input.Risk,
		AuthVersion: conn.AuthVersion, Args: json.RawMessage(input.Content),
	})
	if err != nil {
		if errors.Is(err, appconnectorsvc.ErrInvalidAction) {
			appFail(c, http.StatusBadRequest, "INVALID_ACTION", "invalid action payload")
			return
		}
		if errors.Is(err, appconnector.ErrInvalidArgs) {
			appFail(c, http.StatusBadRequest, "INVALID_CONTENT", "content is not valid JSON")
			return
		}
		appFail(c, http.StatusInternalServerError, "ACTION_PREPARE_FAILED", "failed to prepare action")
		return
	}
	row, ok := h.appActionByID(c, tenantID, id)
	if !ok {
		return
	}
	appOK(c, http.StatusCreated, h.actionDetailFor(c, tenantID, row))
}

type appActionApproveInput struct {
	Digest          string `json:"digest"`
	ExpectedVersion *int64 `json:"expected_version"`
}

// ApproveAction POST /apps/actions/:id/approve — binds a human decision to
// the CURRENT snapshot: a stale fence (expected_version) or a digest issued
// for older (edited) content is refused, never migrated. Fails closed
// (501) while the A03 service is not wired.
func (h *AppActionHandler) ApproveAction(c *gin.Context) {
	tenantID, _, userID, ok := appTenantScope(c)
	if !ok {
		return
	}
	var input appActionApproveInput
	if err := c.ShouldBindJSON(&input); err != nil || input.Digest == "" || input.ExpectedVersion == nil {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "digest and expected_version are required")
		return
	}
	row, ok := h.appActionByID(c, tenantID, c.Param("id"))
	if !ok {
		return
	}
	if row.Fence != *input.ExpectedVersion {
		appFail(c, http.StatusConflict, "ACTION_VERSION_CONFLICT", "stale expected_version")
		return
	}
	if row.ArgsDigest != input.Digest {
		appFail(c, http.StatusConflict, "ACTION_DIGEST_MISMATCH",
			"the approval was issued for different content; prepare the new content again")
		return
	}
	if row.State != appconnector.ActionAwaitingApproval && row.State != appconnector.ActionAuthorized {
		appFail(c, http.StatusConflict, "ACTION_STATE_CONFLICT", "action cannot be approved from state "+row.State)
		return
	}
	if h.actions == nil {
		appFail(c, http.StatusNotImplemented, "ACTION_PIPELINE_NOT_CONFIGURED",
			"the A03 action approval pipeline is not wired in this environment; refusing to fabricate an approval")
		return
	}
	if err := h.actions.Approve(c.Request.Context(), row.ID, userID, input.Digest); err != nil {
		if errors.Is(err, appconnectorsvc.ErrActionDigestMismatch) {
			appFail(c, http.StatusConflict, "ACTION_DIGEST_MISMATCH",
				"the approval was issued for different content; prepare the new content again")
			return
		}
		if errors.Is(err, appconnectorsvc.ErrActionState) {
			appFail(c, http.StatusConflict, "ACTION_STATE_CONFLICT", "invalid lifecycle transition")
			return
		}
		appFail(c, http.StatusInternalServerError, "ACTION_APPROVE_FAILED", "failed to record approval")
		return
	}
	after, ok := h.appActionByID(c, tenantID, row.ID)
	if !ok {
		return
	}
	appOK(c, http.StatusOK, h.actionDetailFor(c, tenantID, after))
}

// ExecuteAction POST /apps/actions/:id/execute — consumes the approval and
// dispatches through the A03 pipeline (never a fabricated dispatch at this
// edge: 501 while the service is not wired). An unobservable provider
// outcome parks the action in unknown and the response still carries the
// snapshot; the ONLY resolution is a provider query, never a resend.
func (h *AppActionHandler) ExecuteAction(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	row, ok := h.appActionByID(c, tenantID, c.Param("id"))
	if !ok {
		return
	}
	if !appActionStates[row.State] {
		appFail(c, http.StatusConflict, "ACTION_STATE_CONFLICT", "action state is outside the lifecycle vocabulary")
		return
	}
	if h.actions == nil {
		appFail(c, http.StatusNotImplemented, "ACTION_PIPELINE_NOT_CONFIGURED",
			"the A03 action approval pipeline is not wired in this environment; refusing to fabricate a dispatch")
		return
	}
	err := h.actions.Execute(c.Request.Context(), row.ID)
	if err != nil && !errors.Is(err, appconnectorsvc.ErrDispatchUnknown) {
		if errors.Is(err, appconnectorsvc.ErrActionState) {
			appFail(c, http.StatusConflict, "ACTION_STATE_CONFLICT", "action is not authorized for dispatch")
			return
		}
		appFail(c, http.StatusInternalServerError, "ACTION_EXECUTE_FAILED", "dispatch failed")
		return
	}
	after, ok := h.appActionByID(c, tenantID, row.ID)
	if !ok {
		return
	}
	appOK(c, http.StatusOK, h.actionDetailFor(c, tenantID, after))
}

// GetAction GET /apps/actions/:id — the SERVER snapshot of the exact
// target and content, tenant-scoped. Reconnect recovery reads this, not
// any client-side pending state. A persisted state outside the A03
// vocabulary fails closed (500) instead of being guessed.
func (h *AppActionHandler) GetAction(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	row, ok := h.appActionByID(c, tenantID, c.Param("id"))
	if !ok {
		return
	}
	if !appActionStates[row.State] {
		appFail(c, http.StatusInternalServerError, "ACTION_STATE_INVALID",
			"persisted action state is outside the lifecycle vocabulary")
		return
	}
	appOK(c, http.StatusOK, h.actionDetailFor(c, tenantID, row))
}
