package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AppInstallationHandler serves the installation lifecycle under
// /api/v1/apps/installations (W04): list, install, upgrade, disable.
type AppInstallationHandler struct {
	db            *gorm.DB
	installations *appconnectorrepo.InstallationStore
}

func NewAppInstallationHandler(db *gorm.DB) *AppInstallationHandler {
	return &AppInstallationHandler{db: db, installations: appconnectorrepo.NewInstallationStore(db)}
}

// RequireInstallCapabilityForWrites gates every non-read method on the
// installation routes with appconnector.CanInstallInstallation: owner/admin
// may act, every other role gets the request-an-installation path instead
// of a fake success.
func (h *AppInstallationHandler) RequireInstallCapabilityForWrites() gin.HandlerFunc {
	return appRequireWriteCapability(appconnector.CanInstallInstallation,
		"MEMBER_MUST_REQUEST_INSTALLATION",
		"member role cannot install or upgrade apps; submit an installation request instead")
}

// appInstallationView is the wire projection of one installed app version.
// scopes come from the catalog entry schema_json {"scopes":[...]}.
type appInstallationView struct {
	ID      string   `json:"id"`
	AppKey  string   `json:"app_key"`
	Version string   `json:"version"`
	State   string   `json:"state"`
	Scopes  []string `json:"scopes"`
}

// scopesForVersion reads the published scope list of an app version. An
// unreadable or missing catalog entry yields an empty list - the handler
// never invents scopes that were not published.
func (h *AppInstallationHandler) scopesForVersion(c *gin.Context, app, version string) []string {
	var row appconnectorrepo.AppVersion
	if err := h.db.WithContext(c.Request.Context()).
		Where("app_id = ? AND version = ?", app, version).First(&row).Error; err != nil {
		return []string{}
	}
	var parsed struct {
		Scopes []string `json:"scopes"`
	}
	if err := json.Unmarshal([]byte(row.SchemaJSON), &parsed); err != nil || parsed.Scopes == nil {
		return []string{}
	}
	return parsed.Scopes
}

func (h *AppInstallationHandler) appInstallationViewFor(c *gin.Context, id, app, version, state string) appInstallationView {
	return appInstallationView{
		ID: id, AppKey: app, Version: version, State: state,
		Scopes: h.scopesForVersion(c, app, version),
	}
}

// ListInstallations GET /apps/installations
func (h *AppInstallationHandler) ListInstallations(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	var rows []appconnectorrepo.InstallationRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tenantID).Find(&rows).Error; err != nil {
		appFail(c, http.StatusInternalServerError, "LIST_INSTALLATIONS_FAILED", "failed to list installations")
		return
	}
	views := make([]appInstallationView, 0, len(rows))
	for _, row := range rows {
		views = append(views, h.appInstallationViewFor(c, row.ID, row.AppID, row.AppVersion, row.State))
	}
	appOK(c, http.StatusOK, views)
}

type installationWriteInput struct {
	AppKey          string `json:"app_key"`
	Version         string `json:"version"`
	ExpectedVersion *int64 `json:"expected_version"`
}

// installationByID resolves an installation id inside the tenant. A
// cross-tenant id and a missing one are both 404 - never a 403 that leaks
// existence.
func (h *AppInstallationHandler) installationByID(c *gin.Context, tenantID uint64, id string) (appconnectorrepo.InstallationRow, bool) {
	var row appconnectorrepo.InstallationRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error; err != nil {
		appFail(c, http.StatusNotFound, "INSTALLATION_NOT_FOUND", "installation not found")
		return row, false
	}
	return row, true
}

func (h *AppInstallationHandler) applyInstallation(c *gin.Context, tenantID uint64, row appconnectorrepo.InstallationRow, targetVersion, targetState string, expected int64) (appInstallationView, bool) {
	inst := appconnector.Installation{
		ID: row.ID, AppID: row.AppID, Version: targetVersion, State: targetState, TenantID: tenantID,
	}
	err := h.installations.ApplyInstallation(c.Request.Context(), inst, expected)
	if err != nil {
		if errors.Is(err, appconnectorrepo.ErrReauthorizationRequired) {
			appFail(c, http.StatusConflict, "REAUTHORIZATION_REQUIRED",
				"upgrade expands the permission scope; re-authorization is required")
			return appInstallationView{}, false
		}
		if errors.Is(err, appconnectorrepo.ErrInstallationConflict) {
			appFail(c, http.StatusConflict, "INSTALLATION_CONFLICT", "stale expected_version or invalid transition")
			return appInstallationView{}, false
		}
		appFail(c, http.StatusInternalServerError, "INSTALLATION_WRITE_FAILED", "failed to persist installation")
		return appInstallationView{}, false
	}
	after, _, err := h.installations.GetInstallation(c.Request.Context(), tenantID, row.AppID)
	if err != nil {
		appFail(c, http.StatusInternalServerError, "INSTALLATION_READBACK_FAILED", "failed to read back installation")
		return appInstallationView{}, false
	}
	return h.appInstallationViewFor(c, after.ID, after.AppID, after.Version, after.State), true
}

// CreateInstallation POST /apps/installations
func (h *AppInstallationHandler) CreateInstallation(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	var input installationWriteInput
	if err := c.ShouldBindJSON(&input); err != nil || input.AppKey == "" || input.Version == "" || input.ExpectedVersion == nil {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "app_key, version and expected_version are required")
		return
	}
	var existing appconnectorrepo.InstallationRow
	err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND app_id = ?", tenantID, input.AppKey).First(&existing).Error
	if err == nil {
		appFail(c, http.StatusConflict, "INSTALLATION_EXISTS", "app already installed; use upgrade")
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		appFail(c, http.StatusInternalServerError, "INSTALLATION_LOOKUP_FAILED", "failed to look up installation")
		return
	}
	if *input.ExpectedVersion != 0 {
		appFail(c, http.StatusConflict, "INSTALLATION_CONFLICT", "expected_version must be 0 for a fresh install")
		return
	}
	row := appconnectorrepo.InstallationRow{ID: newAppID("inst_"), TenantID: tenantID, AppID: input.AppKey, AppVersion: input.Version, State: appconnector.InstallationActive}
	if view, ok := h.applyInstallation(c, tenantID, row, input.Version, appconnector.InstallationActive, 0); ok {
		appOK(c, http.StatusCreated, view)
	}
}

// UpgradeInstallation POST /apps/installations/:id/upgrade
// A scope-expanding upgrade NEVER lands active: the store refuses an active
// target with ErrReauthorizationRequired and the endpoint surfaces 409
// REAUTHORIZATION_REQUIRED so the UI must prompt for re-consent.
func (h *AppInstallationHandler) UpgradeInstallation(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	var input installationWriteInput
	if err := c.ShouldBindJSON(&input); err != nil || input.Version == "" || input.ExpectedVersion == nil {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "version and expected_version are required")
		return
	}
	row, ok := h.installationByID(c, tenantID, c.Param("id"))
	if !ok {
		return
	}
	if view, ok := h.applyInstallation(c, tenantID, row, input.Version, appconnector.InstallationActive, *input.ExpectedVersion); ok {
		appOK(c, http.StatusOK, view)
	}
}

// DisableInstallation POST /apps/installations/:id/disable
func (h *AppInstallationHandler) DisableInstallation(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	var input installationWriteInput
	if err := c.ShouldBindJSON(&input); err != nil || input.ExpectedVersion == nil {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "expected_version is required")
		return
	}
	row, ok := h.installationByID(c, tenantID, c.Param("id"))
	if !ok {
		return
	}
	if view, ok := h.applyInstallation(c, tenantID, row, row.AppVersion, appconnector.InstallationDisabled, *input.ExpectedVersion); ok {
		appOK(c, http.StatusOK, view)
	}
}
