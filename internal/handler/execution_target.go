package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/execution"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

type ExecutionTargetHandler struct {
	store repository.ExecutionTargetStore
}

func NewExecutionTargetHandler(store repository.ExecutionTargetStore) *ExecutionTargetHandler {
	return &ExecutionTargetHandler{store: store}
}

func targetCaller(c *gin.Context) (uint64, string, error) {
	tenant, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenant == 0 {
		return 0, "", apperrors.NewUnauthorizedError("Unauthorized")
	}
	actor, ok := types.UserIDFromContext(c.Request.Context())
	if !ok || strings.TrimSpace(actor) == "" {
		return 0, "", apperrors.NewUnauthorizedError("Unauthorized")
	}
	return tenant, actor, nil
}

func (h *ExecutionTargetHandler) List(c *gin.Context) {
	tenant, actor, err := targetCaller(c)
	if err != nil {
		c.Error(err)
		return
	}
	items, err := h.store.ListOwnedTargets(c.Request.Context(), tenant, actor)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func (h *ExecutionTargetHandler) Get(c *gin.Context) {
	tenant, actor, err := targetCaller(c)
	if err != nil {
		c.Error(err)
		return
	}
	item, err := h.store.GetOwnedTarget(c.Request.Context(), tenant, actor, c.Param("id"))
	if errors.Is(err, repository.ErrExecutionTargetNotFound) {
		c.Error(apperrors.NewNotFoundError("execution target not found"))
		return
	}
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

type executionTargetRequest struct {
	ID                string `json:"id" binding:"required"`
	Kind              string `json:"kind" binding:"required"`
	RuntimeID         string `json:"runtime_id" binding:"required"`
	ExternalTargetID  string `json:"external_target_id" binding:"required"`
	CredentialVersion int64  `json:"credential_version" binding:"required"`
}

func (h *ExecutionTargetHandler) Create(c *gin.Context) {
	tenant, actor, err := targetCaller(c)
	if err != nil {
		c.Error(err)
		return
	}
	var req executionTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	if req.CredentialVersion <= 0 || strings.TrimSpace(req.ID) == "" || strings.TrimSpace(req.RuntimeID) == "" || strings.TrimSpace(req.ExternalTargetID) == "" {
		c.Error(apperrors.NewBadRequestError("invalid execution target"))
		return
	}
	// root_ref is resolved by the node registration layer and is intentionally
	// absent from this client-facing request.
	target := execution.Target{ID: req.ID, TenantID: tenant, OwnerID: actor, Kind: req.Kind, State: "active", CredentialVersion: req.CredentialVersion, RuntimeID: req.RuntimeID, ExternalTargetID: req.ExternalTargetID}
	if err := h.store.CreateTarget(c.Request.Context(), target, ""); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": target})
}

func (h *ExecutionTargetHandler) Revoke(c *gin.Context) {
	tenant, actor, err := targetCaller(c)
	if err != nil {
		c.Error(err)
		return
	}
	if err := h.store.RevokeTarget(c.Request.Context(), tenant, actor, c.Param("id")); errors.Is(err, repository.ErrExecutionTargetNotFound) {
		c.Error(apperrors.NewNotFoundError("execution target not found"))
		return
	} else if err != nil {
		c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GetWorkspace resolves only a server-created workspace binding. The client
// supplies an opaque identifier and never a path or URL to probe.
func (h *ExecutionTargetHandler) GetWorkspace(c *gin.Context) {
	tenant, actor, err := targetCaller(c)
	if err != nil {
		c.Error(err)
		return
	}
	workspace, err := h.store.GetOwnedWorkspace(c.Request.Context(), tenant, actor, c.Param("id"))
	if errors.Is(err, repository.ErrExecutionTargetNotFound) {
		c.Error(apperrors.NewNotFoundError("execution workspace not found"))
		return
	}
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": workspace})
}
