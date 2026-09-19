package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// FeedbackHandler serves the message feedback API: submit (like/dislike),
// remove, and the caller's own per-session feedback list used by the chat
// UI to restore button state.
type FeedbackHandler struct {
	FeedbackService interfaces.FeedbackService
}

// NewFeedbackHandler creates a feedback handler with its service.
func NewFeedbackHandler(feedbackService interfaces.FeedbackService) *FeedbackHandler {
	return &FeedbackHandler{FeedbackService: feedbackService}
}

type feedbackRequest struct {
	Rating  string `json:"rating"`
	Comment string `json:"comment"`
}

// paramMessageID resolves the message id from either wildcard name. The POST
// feedback route registers :message_id, but the DELETE route must reuse :id
// because gin keeps one radix tree per verb and the existing
// DELETE /messages/:session_id/:id already owns that wildcard position (same
// pattern as the sessions pin/artifact routes).
func paramMessageID(c *gin.Context) string {
	if v := c.Param("message_id"); v != "" {
		return v
	}
	return c.Param("id")
}

// SubmitFeedback upserts the caller's rating on one message.
// POST /api/v1/messages/:session_id/:message_id/feedback
func (h *FeedbackHandler) SubmitFeedback(c *gin.Context) {
	ctx := c.Request.Context()
	var req feedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewBadRequestError("invalid request body"))
		return
	}
	err := h.FeedbackService.SubmitFeedback(
		ctx, types.CallerFromContext(ctx),
		c.Param("session_id"), paramMessageID(c), req.Rating, req.Comment,
	)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{}})
}

// RemoveFeedback deletes the caller's rating on one message.
// DELETE /api/v1/messages/:session_id/:message_id/feedback
func (h *FeedbackHandler) RemoveFeedback(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.FeedbackService.RemoveFeedback(
		ctx, types.CallerFromContext(ctx), c.Param("session_id"), paramMessageID(c),
	); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{}})
}

// ListMyFeedback returns the caller's own feedback rows of a session.
// GET /api/v1/messages/:session_id/feedback/mine
func (h *FeedbackHandler) ListMyFeedback(c *gin.Context) {
	ctx := c.Request.Context()
	list, err := h.FeedbackService.ListMyFeedback(
		ctx, types.CallerFromContext(ctx), c.Param("session_id"),
	)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}
