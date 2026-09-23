package career

import (
	"context"
	"errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"strconv"
)

type Handler struct{ office *Office }

func NewHandler(db *gorm.DB) (*Handler, error) {
	o, e := NewOffice(db)
	if e != nil {
		return nil, e
	}
	return &Handler{o}, nil
}
func (h *Handler) scope(c *gin.Context) (context.Context, bool) {
	ctx := c.Request.Context()
	uid, uok := types.UserIDFromContext(ctx)
	tid, tok := types.TenantIDFromContext(ctx)
	if !uok || !tok {
		c.JSON(401, gin.H{"error": gin.H{"code": "unauthorized", "message": ErrUnauthorized.Error()}})
		return nil, false
	}
	return WithScope(ctx, Scope{UserID: uid, TenantID: tid}), true
}
func writeError(c *gin.Context, e error) {
	status, code := 500, "internal"
	switch {
	case errors.Is(e, ErrUnauthorized):
		status = 401
		code = "unauthorized"
	case errors.Is(e, ErrRevisionConflict):
		status = 409
		code = "revision_conflict"
	case errors.Is(e, ErrIdempotencyConflict):
		status = 409
		code = "idempotency_conflict"
	case errors.Is(e, ErrInvalidRequest):
		status = 400
		code = "invalid_request"
	case errors.Is(e, ErrReceiptNotFound):
		status = 404
		code = "not_found"
	case errors.Is(e, gorm.ErrRecordNotFound):
		status = 404
		code = "not_found"
	}
	body := gin.H{"code": code, "message": e.Error()}
	if errors.Is(e, ErrRevisionConflict) {
		var conflict *RevisionConflictError
		if errors.As(e, &conflict) {
			body["currentRevision"] = conflict.CurrentRevision
		}
	}
	c.JSON(status, gin.H{"error": body})
}
func (h *Handler) Open(c *gin.Context) {
	ctx, ok := h.scope(c)
	if !ok {
		return
	}
	v, e := h.office.Open(ctx)
	if e != nil {
		writeError(c, e)
		return
	}
	c.JSON(200, v)
}
func (h *Handler) Act(c *gin.Context) {
	ctx, ok := h.scope(c)
	if !ok {
		return
	}
	var req struct {
		Key              string `json:"key"`
		Value            string `json:"value"`
		RequestID        string `json:"requestId"`
		ExpectedRevision uint64 `json:"expectedRevision"`
		Confirmed        bool   `json:"confirmed"`
	}
	if e := c.ShouldBindJSON(&req); e != nil {
		writeError(c, ErrInvalidRequest)
		return
	}
	var r Receipt
	var e error
	if req.Confirmed {
		r, e = h.office.Confirm(ctx, req.Key, req.Value, req.RequestID, req.ExpectedRevision)
	} else {
		r, e = h.office.Propose(ctx, req.Key, req.Value, req.RequestID, req.ExpectedRevision)
	}
	if e != nil {
		writeError(c, e)
		return
	}
	c.JSON(200, r)
}
func (h *Handler) Receipt(c *gin.Context) {
	ctx, ok := h.scope(c)
	if !ok {
		return
	}
	r, e := h.office.Receipt(ctx, c.Query("requestId"))
	if e != nil {
		writeError(c, e)
		return
	}
	c.JSON(200, r)
}
func (h *Handler) Changes(c *gin.Context) {
	ctx, ok := h.scope(c)
	if !ok {
		return
	}
	v, e := h.office.Open(ctx)
	if e != nil {
		writeError(c, e)
		return
	}
	since, _ := strconv.ParseUint(c.Query("since"), 10, 64)
	changes := []Fact{}
	for _, f := range v.Facts {
		if f.Revision > since {
			changes = append(changes, f)
		}
	}
	c.JSON(200, gin.H{"revision": v.Revision, "changes": changes})
}

// List returns confirmed facts and pending proposals from the authenticated scope.
func (h *Handler) List(c *gin.Context) { h.Open(c) }
