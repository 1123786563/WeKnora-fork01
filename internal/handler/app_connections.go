package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// appConnectionBindingTTL bounds how long a one-time OAuth state stays
// redeemable after it is issued.
const appConnectionBindingTTL = 10 * time.Minute

// AppConnectionStore is the narrow persistence surface the HTTP layer needs
// for binding and revocation (implemented by
// repository.MCPOAuthBindingStore).
type AppConnectionStore interface {
	IssueBindingState(ctx context.Context, b appconn.OAuthBinding, serviceID string) error
	CompleteBinding(ctx context.Context, tenant uint64, state, actor string, token *types.MCPOAuthToken) (appconn.Connection, error)
	RevokeConnection(ctx context.Context, tenantID uint64, actor, connectionID string) error
}

// OAuthCodeExchanger performs the provider-side authorization-code exchange.
// It is only invoked from the verified callback path, with the
// provider-issued code and the one-time state the server itself minted;
// implementations must enforce the provider's PKCE rules. The production
// implementation is wired by the container; tests inject a stub. When no
// exchanger is configured the callback stays fail-closed.
type OAuthCodeExchanger interface {
	Exchange(ctx context.Context, tenant uint64, serviceID, code, state string) (*types.MCPOAuthToken, error)
}

// AppConnectionHandler exposes the binding/revocation endpoints. Tenant and
// actor always come from the request context (set by auth middleware), never
// from the request body, and no response ever carries credential material —
// responses go through the redacted view projection only.
type AppConnectionHandler struct {
	store     AppConnectionStore
	exchanger OAuthCodeExchanger
}

// NewAppConnectionHandler builds the handler. exchanger may be nil, in which
// case the callback endpoint answers 503 rather than trusting unverified
// tokens.
func NewAppConnectionHandler(store AppConnectionStore, exchanger OAuthCodeExchanger) *AppConnectionHandler {
	return &AppConnectionHandler{store: store, exchanger: exchanger}
}

// bindingStateFromContext extracts the authenticated tenant and user.
func bindingStateFromContext(c *gin.Context) (uint64, string, bool) {
	tenantID, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenantID == 0 {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "missing tenant scope"})
		return 0, "", false
	}
	userID, ok := types.UserIDFromContext(c.Request.Context())
	if !ok || userID == "" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "missing authenticated user"})
		return 0, "", false
	}
	return tenantID, userID, true
}

// StartBinding issues a one-time OAuth state for the authenticated member.
// The state is minted server-side from crypto/rand — a client-provided state
// is never accepted — and expires after appConnectionBindingTTL.
func (h *AppConnectionHandler) StartBinding(c *gin.Context) {
	tenantID, userID, ok := bindingStateFromContext(c)
	if !ok {
		return
	}
	installationID := c.Param("installation_id")
	var req struct {
		ServiceID string `json:"service_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ServiceID == "" || installationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service_id and installation_id are required"})
		return
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mint state"})
		return
	}
	state := hex.EncodeToString(raw)
	binding := appconn.OAuthBinding{
		State:          state,
		InstallationID: installationID,
		ActorID:        userID,
		TenantID:       tenantID,
		ExpiresAt:      time.Now().Add(appConnectionBindingTTL),
	}
	if err := h.store.IssueBindingState(c.Request.Context(), binding, req.ServiceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"state": state, "expires_at": binding.ExpiresAt})
}

// Callback completes the OAuth binding: the provider-verified code is
// exchanged strictly per the provider's callback/PKCE rules by the injected
// exchanger, and only then is the one-time state consumed and the
// connection bound in a single transaction. On any validation failure the
// exchanged token is discarded and nothing is persisted.
func (h *AppConnectionHandler) Callback(c *gin.Context) {
	tenantID, userID, ok := bindingStateFromContext(c)
	if !ok {
		return
	}
	if h.exchanger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "oauth exchange not configured"})
		return
	}
	var req struct {
		ServiceID string `json:"service_id"`
		Code      string `json:"code"`
		State     string `json:"state"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Code == "" || req.State == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code and state are required"})
		return
	}
	token, err := h.exchanger.Exchange(c.Request.Context(), tenantID, req.ServiceID, req.Code, req.State)
	if err != nil || token == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "oauth exchange rejected"})
		return
	}
	conn, err := h.store.CompleteBinding(c.Request.Context(), tenantID, req.State, userID, token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"connection": connectionView(conn)})
}

// Revoke revokes a connection. The auth_version bump invalidates every
// outstanding credential reference immediately; revoked connections never
// refresh and never dispatch again.
func (h *AppConnectionHandler) Revoke(c *gin.Context) {
	tenantID, userID, ok := bindingStateFromContext(c)
	if !ok {
		return
	}
	if err := h.store.RevokeConnection(c.Request.Context(), tenantID, userID, c.Param("connection_id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": true})
}

// connectionView is the HTTP-safe projection: identifiers and lifecycle
// state only — no credential material, not even the credential reference.
func connectionView(c appconn.Connection) gin.H {
	return gin.H{
		"id":              c.ID,
		"installation_id": c.InstallationID,
		"kind":            c.Kind,
		"owner_id":        c.OwnerID,
		"state":           c.State,
		"tenant_id":       c.TenantID,
		"auth_version":    c.AuthVersion,
	}
}
