package career

import (
	"context"
	"errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"strconv"
)

type Handler struct {
	office  *Office
	members interface {
		ListByTenant(context.Context, uint64) ([]*types.TenantMember, error)
	}
}

func NewHandler(db *gorm.DB, members interfaces.TenantMemberService) (*Handler, error) {
	o, e := NewOffice(db)
	if e != nil {
		return nil, e
	}
	return &Handler{office: o, members: members}, nil
}
func validateOwnerOnlyCareerTenant(userID string, tenantID uint64, members []*types.TenantMember) error {
	if userID == "" || tenantID == 0 || len(members) != 1 || members[0] == nil || members[0].UserID != userID || members[0].TenantID != tenantID || members[0].Role != types.TenantRoleOwner {
		return ErrUnauthorized
	}
	return nil
}
func (h *Handler) scope(c *gin.Context, claim bool) (context.Context, bool) {
	ctx := c.Request.Context()
	uid, uok := types.UserIDFromContext(ctx)
	tid, tok := types.TenantIDFromContext(ctx)
	if !uok || !tok || h.members == nil {
		writeError(c, ErrUnauthorized)
		return nil, false
	}
	members, e := h.members.ListByTenant(ctx, tid)
	if e != nil {
		c.JSON(500, gin.H{"error": gin.H{"code": "internal", "message": "failed to verify personal career workspace"}})
		return nil, false
	}
	if e = validateOwnerOnlyCareerTenant(uid, tid, members); e != nil {
		writeError(c, e)
		return nil, false
	}
	ctx = WithScope(ctx, Scope{UserID: uid, TenantID: tid})
	if claim {
		if e = h.office.ClaimSpace(ctx); e != nil {
			writeError(c, e)
			return nil, false
		}
	}
	return ctx, true
}
func writeError(c *gin.Context, e error) {
	status, code := 500, "internal"
	switch {
	case errors.Is(e, ErrUnauthorized):
		status = 403
		code = "forbidden"
	case errors.Is(e, ErrRevisionConflict):
		status = 409
		code = "revision_conflict"
	case errors.Is(e, ErrIdempotencyConflict):
		status = 409
		code = "idempotency_conflict"
	case errors.Is(e, ErrInvalidRequest):
		status = 400
		code = "invalid_request"
	case errors.Is(e, ErrReceiptNotFound), errors.Is(e, ErrProposalNotFound):
		status = 404
		code = "not_found"
	case errors.Is(e, ErrProposalResolved):
		status = 409
		code = "proposal_resolved"
	case errors.Is(e, ErrOutcomeUnknown):
		status = 504
		code = "outcome_unknown"
	}
	body := gin.H{"code": code, "message": e.Error()}
	if errors.Is(e, ErrRevisionConflict) {
		var ce *RevisionConflictError
		if errors.As(e, &ce) {
			body["currentRevision"] = ce.CurrentRevision
		}
	}
	if errors.Is(e, ErrOutcomeUnknown) {
		var unknown *OutcomeUnknownError
		if errors.As(e, &unknown) {
			body["requestId"] = unknown.RequestID
		}
	}
	c.JSON(status, gin.H{"error": body})
}
func (h *Handler) Open(c *gin.Context) {
	ctx, ok := h.scope(c, true)
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
func (h *Handler) List(c *gin.Context) {
	ctx, ok := h.scope(c, false)
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
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	var req struct {
		Action           string `json:"action"`
		ProposalID       string `json:"proposalId"`
		Key              string `json:"key"`
		Value            string `json:"value"`
		RequestID        string `json:"requestId"`
		ExpectedRevision uint64 `json:"expectedRevision"`
		Source           Source `json:"source"`
	}
	if e := c.ShouldBindJSON(&req); e != nil {
		writeError(c, ErrInvalidRequest)
		return
	}
	r, e := h.office.Act(ctx, req.Action, req.ProposalID, req.Key, req.Value, req.RequestID, req.ExpectedRevision, req.Source)
	if e != nil {
		writeError(c, e)
		return
	}
	c.JSON(200, r)
}
func (h *Handler) Receipt(c *gin.Context) {
	ctx, ok := h.scope(c, false)
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
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	since, e := strconv.ParseUint(c.Query("since"), 10, 64)
	if e != nil && c.Query("since") != "" {
		writeError(c, ErrInvalidRequest)
		return
	}
	v, e := h.office.Changes(ctx, since)
	if e != nil {
		writeError(c, e)
		return
	}
	c.JSON(200, v)
}
