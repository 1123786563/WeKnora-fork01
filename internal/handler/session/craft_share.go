package session

// T11 (#128): the restricted-share consent HTTP surface. The feature
// registers through the T00 constrained registry at central assembly time.
// The service owns every authority check; these routes only translate.

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/gin-gonic/gin"
)

// CraftShareAPI is the T11 consent surface. Its implementation owns all
// current Task and evidence checks; route guards alone are not ACLs.
type CraftShareAPI interface {
	ShareView(context.Context, craft.Scope, string) (service.CraftShareView, error)
	DecideShare(context.Context, craft.Scope, string, craft.DecisionStatus, string) (service.CraftShareView, error)
	RevokeShare(context.Context, craft.Scope, string) (service.CraftShareView, error)
}

type CraftShareHandler struct{ svc CraftShareAPI }

func NewCraftShareHandler(svc CraftShareAPI) *CraftShareHandler {
	return &CraftShareHandler{svc: svc}
}

// RegisterCraftShareFeature binds the share-consent surface to one router
// assembly's feature registry. Duplicate and post-mount registrations fail
// on that registry without retaining process-global handler state.
func RegisterCraftShareFeature(routes *CraftFeatureRoutes, share CraftShareAPI) error {
	if routes == nil || share == nil {
		return errors.New("Craft share feature unavailable")
	}
	return routes.Register("share", func(group CraftRouteGroup) {
		RegisterCraftShareRoutes(group, NewCraftShareHandler(share))
	})
}

// MountCraftShareRoutes is called through the T00 constrained feature
// registry at central assembly time.
func (h *CraftShareHandler) MountCraftShareRoutes(group CraftRouteGroup) {
	RegisterCraftShareRoutes(group, h)
}

// RegisterCraftShareRoutes mounts the T11 routes onto one constrained
// feature group.
func RegisterCraftShareRoutes(group CraftRouteGroup, h *CraftShareHandler) {
	group.GET("/:id/craft/versions/:version_id/share", h.GetCraftShare)
	group.POST("/:session_id/craft/versions/:version_id/share/decision", h.DecideCraftShare)
	group.POST("/:session_id/craft/versions/:version_id/share/revocation", h.RevokeCraftShare)
}

// craftShareBody is the wire projection: typed consent facts only — the
// exact version, the restricted flag, the evidence digest, the state, the
// bound decision and its expiry. No source material ever travels here.
func craftShareBody(view service.CraftShareView) gin.H {
	body := gin.H{
		"version_id":      view.Contribution.VersionID,
		"restricted":      view.Contribution.Restricted,
		"evidence_digest": view.Contribution.EvidenceDigest,
		"status":          string(view.State),
	}
	if view.Decision != nil {
		body["decision"] = gin.H{
			"version_id":      view.Decision.VersionID,
			"evidence_digest": view.Decision.EvidenceDigest,
			"owner_id":        view.Decision.OwnerID,
			"decision":        string(view.Decision.Decision),
		}
	} else {
		body["decision"] = nil
	}
	if view.ExpiresAt != nil {
		body["expires_at"] = view.ExpiresAt
	}
	return body
}

func (h *CraftShareHandler) GetCraftShare(c *gin.Context) {
	scope, ok := craftScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if h == nil || h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	view, err := h.svc.ShareView(c.Request.Context(), scope, c.Param("version_id"))
	if err != nil {
		craftShareHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, craftShareBody(view))
}

type craftShareDecisionRequest struct {
	Decision       string `json:"decision"`
	EvidenceDigest string `json:"evidence_digest"`
}

func (h *CraftShareHandler) DecideCraftShare(c *gin.Context) {
	scope, ok := craftScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if h == nil || h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	var body craftShareDecisionRequest
	// The decision body goes through the same strict decoder as every craft
	// body: 1MiB ceiling and unknown-field rejection (no injected authority
	// fields ride a consent submission).
	if !decodeCraftBody(c, &body) {
		return
	}
	decision := craft.DecisionStatus(strings.TrimSpace(body.Decision))
	view, err := h.svc.DecideShare(c.Request.Context(), scope, c.Param("version_id"), decision, body.EvidenceDigest)
	if err != nil {
		craftShareHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, craftShareBody(view))
}

func (h *CraftShareHandler) RevokeCraftShare(c *gin.Context) {
	scope, ok := craftScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if h == nil || h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	view, err := h.svc.RevokeShare(c.Request.Context(), scope, c.Param("version_id"))
	if err != nil {
		craftShareHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, craftShareBody(view))
}

// craftShareHTTPError keeps denials stable and non-leaking: the body is the
// generic status text and never says why.
func craftShareHTTPError(c *gin.Context, err error) {
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
