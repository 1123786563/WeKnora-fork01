package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/gin-gonic/gin"
)

// CraftAccessAPI is the narrow owner-managed membership surface. Every
// method enforces the Task ACL itself; the guarded route group is additive.
type CraftAccessAPI interface {
	Grant(context.Context, craft.Scope, string, craft.TaskRole) error
	Revoke(context.Context, craft.Scope, string) error
	ListMembers(context.Context, craft.Scope) ([]craft.TaskMember, error)
}

type CraftAccessHandler struct{ svc CraftAccessAPI }

func NewCraftAccessHandler(svc CraftAccessAPI) *CraftAccessHandler {
	return &CraftAccessHandler{svc: svc}
}

// RegisterCraftAccessRoutes mounts inside the existing guarded sessions
// group. T20 calls this at central assembly; no global route is added here.
func RegisterCraftAccessRoutes(group CraftRouteGroup, h *CraftAccessHandler) {
	if group == nil || h == nil {
		return
	}
	group.GET("/:id/craft/access", h.List)
	group.POST("/:session_id/craft/access", h.Grant)
	group.POST("/:session_id/craft/access/revoke", h.Revoke)
}

func (h *CraftAccessHandler) scope(c *gin.Context) (craft.Scope, bool) {
	if h == nil || h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false})
		return craft.Scope{}, false
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false})
		return craft.Scope{}, false
	}
	return scope, true
}

func craftAccessBody(c *gin.Context, target any) bool {
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false})
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return false
	}
	return true
}

func craftAccessError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, craft.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, craft.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, craft.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, craft.ErrConflict):
		status = http.StatusConflict
	}
	c.JSON(status, gin.H{"success": false, "error": gin.H{"message": http.StatusText(status)}})
}

func (h *CraftAccessHandler) Grant(c *gin.Context) {
	scope, ok := h.scope(c)
	if !ok {
		return
	}
	var body struct {
		UserID string         `json:"user_id"`
		Role   craft.TaskRole `json:"role"`
	}
	if !craftAccessBody(c, &body) {
		return
	}
	if err := h.svc.Grant(c.Request.Context(), scope, body.UserID, body.Role); err != nil {
		craftAccessError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *CraftAccessHandler) Revoke(c *gin.Context) {
	scope, ok := h.scope(c)
	if !ok {
		return
	}
	var body struct {
		UserID string `json:"user_id"`
	}
	if !craftAccessBody(c, &body) {
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), scope, body.UserID); err != nil {
		craftAccessError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *CraftAccessHandler) List(c *gin.Context) {
	scope, ok := h.scope(c)
	if !ok {
		return
	}
	members, err := h.svc.ListMembers(c.Request.Context(), scope)
	if err != nil {
		craftAccessError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": members})
}
