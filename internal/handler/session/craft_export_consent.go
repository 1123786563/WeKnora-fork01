package session

// T13 (#133): the restricted derived-export consent HTTP surface. The
// feature registers through the T00 constrained registry at central
// assembly time. The service owns every authority check; these routes only
// translate. The consent-gated bundle download itself stays on the T12
// export surface: central assembly registers ConsentGatedExportService
// (service layer) where the raw export service stood, so the gate decides
// what the T12 download handler may stream.

import (
	"context"
	stderrors "errors"
	"net/http"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/gin-gonic/gin"
)

// CraftExportConsentAPI is the T13 consent surface. Its implementation
// owns all current Task, manifest and ownership checks; route guards
// alone are not ACLs.
type CraftExportConsentAPI interface {
	ExportConsentView(context.Context, craft.Scope, string) (service.CraftExportConsentView, error)
	DecideExport(context.Context, craft.Scope, string, craft.DecisionStatus, string) (service.CraftExportConsentView, error)
}

type CraftExportConsentHandler struct{ svc CraftExportConsentAPI }

func NewCraftExportConsentHandler(svc CraftExportConsentAPI) *CraftExportConsentHandler {
	return &CraftExportConsentHandler{svc: svc}
}

// RegisterCraftExportConsentFeature binds the export-consent surface to
// one router assembly's feature registry. Duplicate and post-mount
// registrations fail on that registry without retaining process-global
// handler state.
func RegisterCraftExportConsentFeature(routes *CraftFeatureRoutes, api CraftExportConsentAPI) error {
	if routes == nil || api == nil {
		return stderrors.New("Craft export consent feature unavailable")
	}
	return routes.Register("export_consent", func(group CraftRouteGroup) {
		RegisterCraftExportConsentRoutes(group, NewCraftExportConsentHandler(api))
	})
}

// MountCraftExportConsentRoutes is called through the T00 constrained
// feature registry at central assembly time.
func (h *CraftExportConsentHandler) MountCraftExportConsentRoutes(group CraftRouteGroup) {
	RegisterCraftExportConsentRoutes(group, h)
}

// RegisterCraftExportConsentRoutes mounts the T13 routes onto one
// constrained feature group. The consent view rides the GET tree beside
// the T12 export describe route; the decision rides the POST tree like
// every other craft mutation.
func RegisterCraftExportConsentRoutes(group CraftRouteGroup, h *CraftExportConsentHandler) {
	group.GET("/:id/craft/versions/:version_id/export/consent", h.GetCraftExportConsent)
	group.POST("/:session_id/craft/versions/:version_id/export/consent/decision", h.DecideCraftExport)
}

// craftExportConsentBody is the wire projection: typed consent facts only
// — the exact manifest with every member's recorded origins, the
// restricted derived classification, the state and the binding decision.
// No source byte, excerpt or provider URL ever travels here.
func craftExportConsentBody(view service.CraftExportConsentView) gin.H {
	files := make([]gin.H, 0, len(view.Manifest.Files))
	for _, f := range view.Manifest.Files {
		origins := make([]gin.H, 0, len(f.Origins))
		for _, origin := range f.Origins {
			origins = append(origins, gin.H{
				"kind": string(origin.Kind), "ref": origin.Ref,
				"sha256": origin.SHA256, "restricted": origin.Restricted,
			})
		}
		files = append(files, gin.H{
			"path": f.Path, "sha256": f.SHA256, "restricted": f.Restricted, "origins": origins,
		})
	}
	body := gin.H{
		"version_id":         view.Manifest.VersionID,
		"manifest_digest":    view.Manifest.ManifestDigest,
		"state":              string(view.State),
		"restricted_derived": view.RestrictedDerived,
		"files":              files,
		"decision":           nil,
	}
	if view.Decision != nil {
		body["decision"] = gin.H{
			"version_id":      view.Decision.VersionID,
			"manifest_digest": view.Decision.ManifestDigest,
			"owner_id":        view.Decision.OwnerID,
			"decision":        string(view.Decision.Decision),
		}
	}
	return body
}

// GetCraftExportConsent answers the consent summary for a current Task
// member: the exact files with their recorded origins, the restricted
// derived classification, the manifest digest and the current state. This
// is what the owner decides against.
//
// @Router /sessions/{session_id}/craft/versions/{version_id}/export/consent [get]
func (h *CraftExportConsentHandler) GetCraftExportConsent(c *gin.Context) {
	scope, ok := craftScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if h == nil || h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	view, err := h.svc.ExportConsentView(c.Request.Context(), scope, c.Param("version_id"))
	if err != nil {
		craftShareHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, craftExportConsentBody(view))
}

type craftExportDecisionRequest struct {
	Decision       string `json:"decision"`
	ManifestDigest string `json:"manifest_digest"`
}

// DecideCraftExport records the owner's explicit export decision. The
// decision body goes through the same strict decoder as every craft body:
// 1MiB ceiling and unknown-field rejection (no injected authority field
// rides a consent submission).
//
// @Router /sessions/{session_id}/craft/versions/{version_id}/export/consent/decision [post]
func (h *CraftExportConsentHandler) DecideCraftExport(c *gin.Context) {
	scope, ok := craftScope(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if h == nil || h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "unavailable"})
		return
	}
	var body craftExportDecisionRequest
	if !decodeCraftBody(c, &body) {
		return
	}
	decision := craft.DecisionStatus(strings.TrimSpace(body.Decision))
	view, err := h.svc.DecideExport(c.Request.Context(), scope, c.Param("version_id"), decision, body.ManifestDigest)
	if err != nil {
		craftShareHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, craftExportConsentBody(view))
}
