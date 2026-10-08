package session

import (
	"context"
	"errors"
	"net/http"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/gin-gonic/gin"
)

// CraftKnowledgeAPI is the T05 read surface. Its implementation owns all
// current Task and original-resource checks; route guards alone are not ACLs.
type CraftKnowledgeAPI interface {
	Sources(context.Context, craft.Scope, string) (craft.KnowledgeRecord, error)
	AuthorizeSourceOpen(context.Context, craft.Scope, string, string) (string, error)
}

type CraftKnowledgeHandler struct{ svc CraftKnowledgeAPI }

func NewCraftKnowledgeHandler(svc CraftKnowledgeAPI) *CraftKnowledgeHandler {
	return &CraftKnowledgeHandler{svc: svc}
}

// MountCraftKnowledgeRoutes is called through the T00 constrained feature
// registry at central assembly time.
func (h *CraftKnowledgeHandler) MountCraftKnowledgeRoutes(group CraftRouteGroup) {
	group.GET("/:id/craft/runs/:run_id/sources", h.ListCraftSources)
	group.GET("/:id/craft/runs/:run_id/sources/:citation_id/open", h.OpenCraftSource)
}
func (h *CraftKnowledgeHandler) ListCraftSources(c *gin.Context) {
	scope, ok := craftScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if h == nil || h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	record, err := h.svc.Sources(c.Request.Context(), scope, c.Param("run_id"))
	if err != nil {
		craftKnowledgeHTTPError(c, err)
		return
	}
	rows := make([]gin.H, 0, len(record.Sources))
	for _, source := range record.Sources {
		rows = append(rows, gin.H{"citation_id": source.ID, "ref": source.Ref, "digest": source.Digest, "tenant_id": source.TenantID, "acquired_at": source.AcquiredAt, "excerpt_bytes": source.ExcerptBytes})
	}
	c.JSON(http.StatusOK, gin.H{"run_id": record.RunID, "empty": record.Empty, "truncated": record.Truncated, "sources": rows})
}
func (h *CraftKnowledgeHandler) OpenCraftSource(c *gin.Context) {
	scope, ok := craftScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if h == nil || h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	ref, err := h.svc.AuthorizeSourceOpen(c.Request.Context(), scope, c.Param("run_id"), c.Param("citation_id"))
	if err != nil {
		craftKnowledgeHTTPError(c, err)
		return
	}
	// A stable opaque reference is returned, never a model URL or a signed URL.
	c.JSON(http.StatusOK, gin.H{"ref": ref})
}
func craftKnowledgeHTTPError(c *gin.Context, err error) {
	code := http.StatusServiceUnavailable
	switch {
	case errors.Is(err, craft.ErrInvalidInput):
		code = http.StatusBadRequest
	case errors.Is(err, craft.ErrForbidden):
		code = http.StatusForbidden
	case errors.Is(err, craft.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, craft.ErrConflict):
		code = http.StatusConflict
	}
	c.JSON(code, gin.H{"error": http.StatusText(code)})
}
