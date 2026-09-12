package handler

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	repocommercialmcp "github.com/Tencent/WeKnora/internal/application/repository"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AppConnectionHandler serves the connection lifecycle under
// /api/v1/apps/connections (A02): projection listing, the OAuth authorize
// start, the public provider callback that materializes a connection, and
// revocation.
type AppConnectionHandler struct {
	db            *gorm.DB
	installations *appconnectorrepo.InstallationStore
	// bindings issues/consumes the A02 one-time OAuth binding states.
	bindings *repocommercialmcp.MCPOAuthBindingStore
	// oauthMu guards the (container-injected) provider registrations.
	oauthMu        sync.Mutex
	oauthProviders map[string]AppOAuthProviderConfig
}

func NewAppConnectionHandler(db *gorm.DB) *AppConnectionHandler {
	return &AppConnectionHandler{
		db:             db,
		installations:  appconnectorrepo.NewInstallationStore(db),
		bindings:       repocommercialmcp.NewMCPOAuthBindingStore(db),
		oauthProviders: map[string]AppOAuthProviderConfig{},
	}
}

// RequireConnectionCapabilityForWrites gates every non-read method on the
// connection routes with appconnector.CanManageConnections. Connection
// management is a SEPARATE capability from installation management (the
// permission matrix keeps 管理空间连接与授权 as its own row); the
// predicates coincide today, but declaring the gate here lets connection
// policy evolve without touching installation writes.
func (h *AppConnectionHandler) RequireConnectionCapabilityForWrites() gin.HandlerFunc {
	return appRequireWriteCapability(appconnector.CanManageConnections,
		"FORBIDDEN_CONNECTION_MANAGEMENT",
		"connection management requires owner or admin role")
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

func appConnectionViewFor(row appconnectorrepo.ConnectionRow) appConnectionView {
	var owner *string
	if row.OwnerID != "" {
		owner = &row.OwnerID
	}
	return appConnectionView{ID: row.ID, Kind: row.Kind, State: row.State, OwnerID: owner}
}

// ListConnections GET /apps/connections - projections only, no credentials.
func (h *AppConnectionHandler) ListConnections(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
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
	// RedirectURI is the backend OAuth callback registered with the
	// provider; the authorize URL points the browser back to it.
	RedirectURI string `json:"redirect_uri"`
}

// CreateConnection POST /apps/connections - starts the A02 OAuth authorize
// flow: the installation is verified IN this tenant at the expected
// version, a one-time binding state is minted from crypto/rand and
// persisted, and the provider authorize URL is returned. No connection is
// minted here and no credential is ever seen by this endpoint; the
// connection materializes only when the public callback redeems the state
// (ConnectionOAuthCallback). An app whose OAuth application is not
// registered in this deployment fails closed (501).
func (h *AppConnectionHandler) CreateConnection(c *gin.Context) {
	tenantID, _, userID, ok := appTenantScope(c)
	if !ok {
		return
	}
	var input connectionCreateInput
	if err := c.ShouldBindJSON(&input); err != nil || input.InstallationID == "" ||
		(input.Kind != appconnector.ConnectionKindPersonal && input.Kind != appconnector.ConnectionKindSpace) ||
		input.ExpectedVersion == nil {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "installation_id, kind and expected_version are required")
		return
	}
	redirectURI := strings.TrimSpace(input.RedirectURI)
	if redirectURI == "" {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "redirect_uri is required")
		return
	}
	var inst appconnectorrepo.InstallationRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, input.InstallationID).First(&inst).Error; err != nil {
		appFail(c, http.StatusNotFound, "INSTALLATION_NOT_FOUND", "installation not found")
		return
	}
	if inst.State != appconnector.InstallationActive {
		appFail(c, http.StatusConflict, "INSTALLATION_NOT_ACTIVE", "installation is not active")
		return
	}
	if inst.Version != *input.ExpectedVersion {
		appFail(c, http.StatusConflict, "VERSION_CONFLICT", "stale expected_version")
		return
	}
	state, authorizeURL, expires, err := h.createConnectionOAuth(c, tenantID, userID, inst, redirectURI)
	if err != nil {
		var cfgErr *appOAuthConfigError
		if errors.As(err, &cfgErr) {
			if cfgErr.known {
				appFail(c, http.StatusNotImplemented, "OAUTH_NOT_CONFIGURED",
					"the provider application is not registered in this deployment")
			} else {
				appFail(c, http.StatusBadRequest, "UNKNOWN_APP", "this app has no OAuth flow")
			}
			return
		}
		appFail(c, http.StatusInternalServerError, "OAUTH_START_FAILED", "failed to start the authorization flow")
		return
	}
	appOK(c, http.StatusCreated, gin.H{
		"authorization_state": state,
		"authorize_url":       authorizeURL,
		"expires_at":          expires.UTC().Format(time.RFC3339),
		"installation_id":     inst.ID,
		"kind":                input.Kind,
	})
}

// RevokeConnection POST /apps/connections/:id/revoke - CAS on auth_version.
func (h *AppConnectionHandler) RevokeConnection(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
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
