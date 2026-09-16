package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/execution"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// ExecutionRegistrationHandler is the authenticated control-plane entrance
// for personal nodes. It never accepts a bearer, private key, node root, or
// network address from the mobile client.
type ExecutionRegistrationHandler struct {
	service *execution.RegistrationService
}

func NewExecutionRegistrationHandler(service *execution.RegistrationService) *ExecutionRegistrationHandler {
	return &ExecutionRegistrationHandler{service: service}
}

func registrationCaller(c *gin.Context) (uint64, string, error) {
	tenant, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenant == 0 {
		return 0, "", apperrors.NewUnauthorizedError("Unauthorized")
	}
	owner, ok := types.UserIDFromContext(c.Request.Context())
	if !ok || strings.TrimSpace(owner) == "" {
		return 0, "", apperrors.NewUnauthorizedError("Unauthorized")
	}
	return tenant, owner, nil
}

func (h *ExecutionRegistrationHandler) CreateChallenge(c *gin.Context) {
	if h == nil || h.service == nil {
		c.Error(apperrors.NewServiceUnavailableError("personal node registration unavailable"))
		return
	}
	tenant, owner, err := registrationCaller(c)
	if err != nil {
		c.Error(err)
		return
	}
	var req execution.NodeChallengeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	challenge, err := h.service.CreateChallenge(c.Request.Context(), tenant, owner, req, time.Now().UTC())
	if err != nil {
		c.Error(mapRegistrationError(err))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": challenge})
}

func (h *ExecutionRegistrationHandler) Complete(c *gin.Context) {
	if h == nil || h.service == nil {
		c.Error(apperrors.NewServiceUnavailableError("personal node registration unavailable"))
		return
	}
	tenant, owner, err := registrationCaller(c)
	if err != nil {
		c.Error(err)
		return
	}
	var req execution.NodeRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	registration, grant, err := h.service.Complete(c.Request.Context(), tenant, owner, req, time.Now().UTC())
	if err != nil {
		c.Error(mapRegistrationError(err))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"registration": registration, "grant": grant}})
}

func (h *ExecutionRegistrationHandler) Revoke(c *gin.Context) {
	if h == nil || h.service == nil {
		c.Error(apperrors.NewServiceUnavailableError("personal node registration unavailable"))
		return
	}
	tenant, owner, err := registrationCaller(c)
	if err != nil {
		c.Error(err)
		return
	}
	if err := h.service.Revoke(c.Request.Context(), tenant, owner, c.Param("id"), time.Now().UTC()); err != nil {
		c.Error(mapRegistrationError(err))
		return
	}
	c.Status(http.StatusNoContent)
}

func mapRegistrationError(err error) error {
	switch {
	case errors.Is(err, execution.ErrRegistrationInvalid), errors.Is(err, execution.ErrRegistrationChallenge), errors.Is(err, execution.ErrRegistrationReplay):
		return apperrors.NewBadRequestError(err.Error())
	case errors.Is(err, execution.ErrRegistrationNotFound):
		return apperrors.NewNotFoundError("personal node registration not found")
	case errors.Is(err, execution.ErrRegistrationRevoked), errors.Is(err, execution.ErrRegistrationUnauthorized):
		return apperrors.NewForbiddenError(err.Error())
	default:
		return err
	}
}
