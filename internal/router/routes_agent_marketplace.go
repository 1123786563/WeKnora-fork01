package router

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
)

// RegisterAgentMarketplaceRoutes mounts the authenticated Tenant Release
// workflow beside the legacy expert market. Submission uses the frozen
// version's source Agent for OwnedAgentOrAdmin authorization; that ID is
// resolved server-side before applying the shared guard.
func RegisterAgentMarketplaceRoutes(r *gin.RouterGroup, marketHandler *handler.AgentMarketplaceHandler, g *rbacGuards) {
	if marketHandler == nil {
		return
	}
	base := r.Group("/marketplace/tenant")
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/release-submissions",
		apiKeyManageAgents(apiKeyFullAccess()), submissionSourceAgent(marketHandler), g.OwnedAgentOrAdmin(), marketHandler.SubmitRelease)
	g.apiKeyRoute(r, http.MethodGet, "/marketplace/tenant/release-submissions/review-queue",
		apiKeyFullAccess(), g.Admin(), marketHandler.ListReviewQueue)
	g.apiKeyRoute(r, http.MethodPost, "/marketplace/tenant/release-submissions/:id/review",
		apiKeyManageAgents(apiKeyFullAccess()), g.Admin(), marketHandler.ReviewSubmission)
	tenantCatalog := g.apiKeyGroup(base.Group("/catalog"), apiKeyFullAccess())
	tenantCatalog.GET("", g.Viewer(), marketHandler.ListTenantCatalog)
}

type submissionVersionBody struct {
	AgentVersionID string `json:"agent_version_id"`
}

func submissionSourceAgent(marketHandler *handler.AgentMarketplaceHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
		if err != nil {
			_ = c.Error(apperrors.NewBadRequestError("invalid marketplace request body"))
			c.Abort()
			return
		}
		var body submissionVersionBody
		dec := json.NewDecoder(bytes.NewReader(raw))
		if err := dec.Decode(&body); err != nil {
			// Let the strict handler return the structured 400 response.
			c.Request.Body = io.NopCloser(bytes.NewReader(raw))
			c.Next()
			return
		}
		if strings.TrimSpace(body.AgentVersionID) == "" {
			c.Request.Body = io.NopCloser(bytes.NewReader(raw))
			c.Next()
			return
		}
		agentID, err := marketHandler.AgentIDForVersion(c.Request.Context(), tenantIDFromRequest(c), body.AgentVersionID)
		if err != nil {
			_ = c.Error(err)
			c.Abort()
			return
		}
		// OwnedAgentOrAdmin consumes :id. The route itself has no agent URL
		// segment, so populate it only from the tenant-scoped immutable source.
		c.Params = append(c.Params, gin.Param{Key: "id", Value: agentID})
		c.Request.Body = io.NopCloser(bytes.NewReader(raw))
		c.Next()
	}
}

func tenantIDFromRequest(c *gin.Context) uint64 {
	return c.GetUint64(types.TenantIDContextKey.String())
}
