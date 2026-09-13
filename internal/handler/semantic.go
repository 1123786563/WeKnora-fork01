package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/types"
)

// SemanticQueryFacade is the W01 handler seam over the Q04 facade.
type SemanticQueryFacade interface {
	Status(c *gin.Context, kbID, documentID string) (types.SemanticDocumentStatusWire, error)
	Search(c *gin.Context, kbID, query string) (types.SemanticSearchResponseWire, error)
	Reason(c *gin.Context, kbID, query, mode string) (types.SemanticReasonResponseWire, error)
}

// SemanticHandler serves the user-facing semantic API (W01). A nil
// facade fails every route closed - the deployment has no semantic
// pipeline and no result is ever fabricated.
type SemanticHandler struct {
	facade SemanticQueryFacade
}

func NewSemanticHandler(facade SemanticQueryFacade) *SemanticHandler {
	return &SemanticHandler{facade: facade}
}

func (h *SemanticHandler) unavailable(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"error": "semantic indexing is not enabled for this deployment",
		"code":  "semantic_unavailable",
	})
}

func (h *SemanticHandler) Status(c *gin.Context) {
	if h == nil || h.facade == nil {
		h.unavailable(c)
		return
	}
	status, err := h.facade.Status(c, c.Param("id"), c.Query("document_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "semantic status failed"})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *SemanticHandler) Search(c *gin.Context) {
	if h == nil || h.facade == nil {
		h.unavailable(c)
		return
	}
	var body struct {
		Query string `json:"query"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
		return
	}
	result, err := h.facade.Search(c, c.Param("id"), body.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "semantic search failed"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *SemanticHandler) Reason(c *gin.Context) {
	if h == nil || h.facade == nil {
		h.unavailable(c)
		return
	}
	var body struct {
		Query string `json:"query"`
		Mode  string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
		return
	}
	if body.Mode != "rules" && body.Mode != "model" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be rules or model"})
		return
	}
	result, err := h.facade.Reason(c, c.Param("id"), body.Query, body.Mode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "semantic reason failed"})
		return
	}
	c.JSON(http.StatusOK, result)
}
