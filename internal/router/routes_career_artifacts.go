package router

import (
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/gin-gonic/gin"
)

// RegisterCareerArtifactRoutes mounts only authenticated grant issuance.
// Download is mounted before global Auth because the signed capability is its
// complete credential.
func RegisterCareerArtifactRoutes(v1 *gin.RouterGroup, h *session.CareerArtifactHandler) {
	if v1 == nil || h == nil {
		return
	}
	v1.POST("/career/resources/:resource_id/versions/:version_id/signed-url", h.Issue)
}

// RegisterCareerArtifactDownloadRoute mounts the signed capability endpoint
// on the pre-auth router; the token is the complete download credential.
func RegisterCareerArtifactDownloadRoute(r *gin.Engine, h *session.CareerArtifactHandler) {
	if r == nil || h == nil {
		return
	}
	r.GET("/api/v1/career/artifacts/download", h.Download)
}
