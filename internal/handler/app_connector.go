package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AppConnectorHandler serves the /api/v1/apps endpoints (W04): the app
// catalog projections a tenant sees - installations, connections and sync
// status. Tenant scope is ALWAYS derived from the authenticated context
// (never a path parameter); every id is looked up BY tenant so a cross-tenant
// id is indistinguishable from a missing one (404).
//
// Connection responses are explicit structs. Credential material
// (access/refresh tokens, credential_ref) has no field on any view here and
// is therefore structurally unable to leak into a response.
type AppConnectorHandler struct {
	db            *gorm.DB
	installations *appconnectorrepo.InstallationStore
}

func NewAppConnectorHandler(db *gorm.DB) *AppConnectorHandler {
	return &AppConnectorHandler{db: db, installations: appconnectorrepo.NewInstallationStore(db)}
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

// appConnectionView is the wire projection of a connection. It deliberately has
// NO credential field - not even credential_ref: sharing an installation
// never implies sharing credentials.
type appConnectionView struct {
	ID      string  `json:"id"`
	Kind    string  `json:"kind"`
	State   string  `json:"state"`
	OwnerID *string `json:"owner_id"`
}

// appSyncStatusView mirrors the A07 binding state for one data source.
type appSyncStatusView struct {
	DataSourceID            string           `json:"datasource_id"`
	State                   string           `json:"state"`
	PauseReason             *string          `json:"pause_reason"`
	Binding                 *appSyncBindingView `json:"binding"`
	RequiresReauthorization bool             `json:"requires_reauthorization"`
}

type appSyncBindingView struct {
	InstallationID string `json:"installation_id"`
	ConnectionID   string `json:"connection_id"`
	AuthVersion    string `json:"auth_version"`
}

func appOK(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"success": true, "data": data})
}

func appFail(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"success": false, "error": gin.H{"code": code, "message": message}})
}

func newAppID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return prefix + hex.EncodeToString(buf)
}

// scopesForVersion reads the published scope list of an app version. An
// unreadable or missing catalog entry yields an empty list - the handler
// never invents scopes that were not published.
func (h *AppConnectorHandler) scopesForVersion(c *gin.Context, app, version string) []string {
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

func (h *AppConnectorHandler) appInstallationViewFor(c *gin.Context, id, app, version, state string) appInstallationView {
	return appInstallationView{
		ID: id, AppKey: app, Version: version, State: state,
		Scopes: h.scopesForVersion(c, app, version),
	}
}

func appConnectionViewFor(row appconnectorrepo.ConnectionRow) appConnectionView {
	var owner *string
	if row.OwnerID != "" {
		owner = &row.OwnerID
	}
	return appConnectionView{ID: row.ID, Kind: row.Kind, State: row.State, OwnerID: owner}
}

// appTenantScope reads tenant, role and user exclusively from the
// authenticated context. ok=false means the request is rejected already.
func (h *AppConnectorHandler) appTenantScope(c *gin.Context) (uint64, string, string, bool) {
	tenantID, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenantID == 0 {
		appFail(c, http.StatusForbidden, "MISSING_TENANT_SCOPE", ErrMissingTenantScope.Error())
		return 0, "", "", false
	}
	role := string(types.TenantRoleFromContext(c.Request.Context()))
	userID, _ := types.UserIDFromContext(c.Request.Context())
	return tenantID, role, userID, true
}

// RequireInstallCapabilityForWrites gates every non-read method on
// appconnector.CanInstallInstallation: owner/admin may act, every other role
// gets the request-an-installation path instead of a fake success. UI
// hiding is not authorization - this server gate is authoritative.
func (h *AppConnectorHandler) RequireInstallCapabilityForWrites() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		tenantID, ok := types.TenantIDFromContext(c.Request.Context())
		if !ok || tenantID == 0 {
			appFail(c, http.StatusForbidden, "MISSING_TENANT_SCOPE", ErrMissingTenantScope.Error())
			c.Abort()
			return
		}
		role := string(types.TenantRoleFromContext(c.Request.Context()))
		if !appconnector.CanInstallInstallation(role) {
			appFail(c, http.StatusForbidden, "MEMBER_MUST_REQUEST_INSTALLATION",
				"member role cannot install or upgrade apps; submit an installation request instead")
			c.Abort()
			return
		}
		c.Next()
	}
}

// ListInstallations GET /apps/installations
func (h *AppConnectorHandler) ListInstallations(c *gin.Context) {
	tenantID, _, _, ok := h.appTenantScope(c)
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
func (h *AppConnectorHandler) installationByID(c *gin.Context, tenantID uint64, id string) (appconnectorrepo.InstallationRow, bool) {
	var row appconnectorrepo.InstallationRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error; err != nil {
		appFail(c, http.StatusNotFound, "INSTALLATION_NOT_FOUND", "installation not found")
		return row, false
	}
	return row, true
}

func (h *AppConnectorHandler) applyInstallation(c *gin.Context, tenantID uint64, row appconnectorrepo.InstallationRow, targetVersion, targetState string, expected int64) (appInstallationView, bool) {
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
func (h *AppConnectorHandler) CreateInstallation(c *gin.Context) {
	tenantID, _, _, ok := h.appTenantScope(c)
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
func (h *AppConnectorHandler) UpgradeInstallation(c *gin.Context) {
	tenantID, _, _, ok := h.appTenantScope(c)
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
func (h *AppConnectorHandler) DisableInstallation(c *gin.Context) {
	tenantID, _, _, ok := h.appTenantScope(c)
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

// ListConnections GET /apps/connections - projections only, no credentials.
func (h *AppConnectorHandler) ListConnections(c *gin.Context) {
	tenantID, _, _, ok := h.appTenantScope(c)
	if !ok {
		return
	}
	var rows []appconnectorrepo.ConnectionRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tenantID).Find(&rows).Error; err != nil {
		appFail(c, http.StatusInternalServerError, "LIST_CONNECTIONS_FAILED", "failed to list connections")
		return
	}
	views := make([]appConnectionView, 0, len(rows))
	for _, row := range rows {
		views = append(views, appConnectionViewFor(row))
	}
	appOK(c, http.StatusOK, views)
}

type connectionCreateInput struct {
	InstallationID  string `json:"installation_id"`
	Kind            string `json:"kind"`
	ExpectedVersion *int64 `json:"expected_version"`
}

// CreateConnection POST /apps/connections - starts the A02 OAuth authorize
// flow. With no provider configured the endpoint fails closed (501): it
// never mints a connection and never echoes credentials it does not have.
func (h *AppConnectorHandler) CreateConnection(c *gin.Context) {
	if _, _, _, ok := h.appTenantScope(c); !ok {
		return
	}
	var input connectionCreateInput
	if err := c.ShouldBindJSON(&input); err != nil || input.InstallationID == "" ||
		(input.Kind != appconnector.ConnectionKindPersonal && input.Kind != appconnector.ConnectionKindSpace) ||
		input.ExpectedVersion == nil {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "installation_id, kind and expected_version are required")
		return
	}
	appFail(c, http.StatusNotImplemented, "OAUTH_NOT_CONFIGURED",
		"connection creation requires the app provider OAuth flow; no provider is configured in this environment")
}

// RevokeConnection POST /apps/connections/:id/revoke - CAS on auth_version.
func (h *AppConnectorHandler) RevokeConnection(c *gin.Context) {
	tenantID, _, _, ok := h.appTenantScope(c)
	if !ok {
		return
	}
	var input struct {
		ExpectedVersion *int64 `json:"expected_version"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.ExpectedVersion == nil {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "expected_version is required")
		return
	}
	res := h.db.WithContext(c.Request.Context()).
		Table("connections").
		Where("tenant_id = ? AND id = ? AND auth_version = ?", tenantID, c.Param("id"), *input.ExpectedVersion).
		Updates(map[string]any{"state": appconnector.ConnectionRevoked})
	if res.Error != nil {
		appFail(c, http.StatusInternalServerError, "REVOKE_FAILED", "failed to revoke connection")
		return
	}
	if res.RowsAffected == 0 {
		var count int64
		h.db.WithContext(c.Request.Context()).Table("connections").
			Where("tenant_id = ? AND id = ?", tenantID, c.Param("id")).Count(&count)
		if count == 0 {
			appFail(c, http.StatusNotFound, "CONNECTION_NOT_FOUND", "connection not found")
			return
		}
		appFail(c, http.StatusConflict, "VERSION_CONFLICT", "stale expected_version")
		return
	}
	conn, err := h.installations.GetConnection(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		appFail(c, http.StatusInternalServerError, "CONNECTION_READBACK_FAILED", "failed to read back connection")
		return
	}
	appOK(c, http.StatusOK, appConnectionViewFor(appconnectorrepo.ConnectionRow{
		TenantID: conn.TenantID, ID: conn.ID, InstallationID: conn.InstallationID,
		Kind: conn.Kind, OwnerID: conn.OwnerID, State: conn.State, AuthVersion: conn.AuthVersion,
	}))
}

// GetSyncStatus GET /apps/datasources/:id/sync-status - A07 binding plus the
// live pause reason. The data source itself is looked up BY tenant (404
// otherwise); pause_reason only ever carries the IsValidPauseReason
// vocabulary (here: permission when reauthorization is required).
func (h *AppConnectorHandler) GetSyncStatus(c *gin.Context) {
	tenantID, _, _, ok := h.appTenantScope(c)
	if !ok {
		return
	}
	dsID := c.Param("id")
	var ds struct {
		ID     string `gorm:"column:id"`
		Status string `gorm:"column:status"`
	}
	if err := h.db.WithContext(c.Request.Context()).
		Table("data_sources").Select("id", "status").
		Where("id = ? AND tenant_id = ?", dsID, tenantID).Take(&ds).Error; err != nil {
		appFail(c, http.StatusNotFound, "DATASOURCE_NOT_FOUND", "data source not found")
		return
	}
	var bindingRow struct {
		InstallationID string `gorm:"column:installation_id"`
		ConnectionID   string `gorm:"column:connection_id"`
		AuthVersion    int64  `gorm:"column:auth_version"`
	}
	err := h.db.WithContext(c.Request.Context()).
		Table("app_datasource_bindings").
		Where("tenant_id = ? AND datasource_id = ?", tenantID, dsID).Take(&bindingRow).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			appFail(c, http.StatusInternalServerError, "SYNC_STATUS_FAILED", "failed to load sync binding")
			return
		}
		// No binding row: legacy data source - no migration is forced and no
		// credential ownership is silently rewritten.
		appOK(c, http.StatusOK, appSyncStatusView{DataSourceID: dsID, State: ds.Status, PauseReason: nil, Binding: nil, RequiresReauthorization: false})
		return
	}
	stored := &appconnector.StoredSyncBinding{
		TenantID: tenantID, DataSourceID: dsID,
		InstallationID: bindingRow.InstallationID, ConnectionID: bindingRow.ConnectionID,
		AuthVersion: bindingRow.AuthVersion,
	}
	var state *appconnector.BindingState
	var conn appconnectorrepo.ConnectionRow
	if connErr := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, bindingRow.ConnectionID).First(&conn).Error; connErr == nil {
		var inst appconnectorrepo.InstallationRow
		if instErr := h.db.WithContext(c.Request.Context()).
			Where("id = ? AND tenant_id = ?", conn.InstallationID, tenantID).First(&inst).Error; instErr == nil {
			state = &appconnector.BindingState{
				ConnectionState: conn.State, ConnectionKind: conn.Kind,
				InstallationState: inst.State, ConnectionAuthVer: conn.AuthVersion,
			}
		}
	}
	// state stays nil when the live rows are gone: ResolveSyncBinding then
	// reports the binding for bookkeeping with RequiresReauthorization=true.
	binding, err := appconnector.ResolveSyncBinding(c.Request.Context(), rowStore{row: stored}, tenantID, dsID, state)
	if err != nil {
		appFail(c, http.StatusInternalServerError, "SYNC_STATUS_FAILED", "failed to resolve sync binding")
		return
	}
	view := appSyncStatusView{
		DataSourceID: dsID,
		State:        ds.Status,
		Binding: &appSyncBindingView{
			InstallationID: binding.InstallationID,
			ConnectionID:   binding.ConnectionID,
			AuthVersion:    strconv.FormatInt(binding.AuthVersion, 10),
		},
		RequiresReauthorization: binding.RequiresReauthorization,
	}
	if binding.RequiresReauthorization {
		reason := appconnector.PauseReasonPermission
		view.PauseReason = &reason
	}
	appOK(c, http.StatusOK, view)
}

// rowStore adapts an already-loaded binding row to the SyncBindingStore
// interface so ResolveSyncBinding re-reads nothing.
type rowStore struct{ row *appconnector.StoredSyncBinding }

func (s rowStore) FindSyncBinding(context.Context, uint64, string) (*appconnector.StoredSyncBinding, error) {
	return s.row, nil
}

