package handler

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

// MobileTokenSealer keeps token encryption at the HTTP boundary. The
// repository only accepts ciphertext and a one-way lookup hash.
type MobileTokenSealer func(token string) (string, error)

// MobileDeviceStore is the DI seam for HTTP. The concrete GORM repository is
// used in production; tests and alternative persistence backends can provide
// the same tenant/owner-scoped contract without bypassing handler checks.
type MobileDeviceStore interface {
	Bind(context.Context, repository.DeviceRegistration) error
	RevokeForTenant(context.Context, uint64, string, string, int64) error
	GetActiveForTenant(context.Context, uint64, string, string) (repository.DeviceRegistration, error)
	ListActiveForTenant(context.Context, uint64, string, string) ([]repository.DeviceRegistration, error)
	MarkPresence(context.Context, uint64, string, string, int64) error
	GetPresence(context.Context, uint64, string, string, int64) (repository.DeviceRegistration, error)
	SetPresence(context.Context, uint64, string, string, int64) (repository.DeviceRegistration, error)
	DeletePresence(context.Context, uint64, string, string, int64) error
}

type MobileDeviceHandler struct {
	store       MobileDeviceStore
	environment string
	seal        MobileTokenSealer
}

func NewMobileDeviceHandler(store MobileDeviceStore, environment string) *MobileDeviceHandler {
	return NewMobileDeviceHandlerWithSealer(store, environment, func(token string) (string, error) {
		key := utils.GetAESKey()
		if len(key) != 32 {
			return "", errors.New("mobile device token encryption is not configured")
		}
		return utils.EncryptAESGCM(token, key)
	})
}

func NewMobileDeviceHandlerWithSealer(store MobileDeviceStore, environment string, seal MobileTokenSealer) *MobileDeviceHandler {
	return &MobileDeviceHandler{store: store, environment: strings.TrimSpace(environment), seal: seal}
}

type mobileDeviceRequest struct {
	Token           string `json:"token"`
	Platform        string `json:"platform"`
	SpaceID         string `json:"space_id,omitempty"`
	Revision        int64  `json:"revision,omitempty"`
	ScopeGeneration int64  `json:"scope_generation,omitempty"`
}

func mobileCaller(c *gin.Context) (uint64, string, bool) {
	tenant, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenant == 0 {
		if value, exists := c.Get(types.TenantIDContextKey.String()); exists {
			tenant, ok = value.(uint64)
		}
	}
	owner, ownerOK := types.UserIDFromContext(c.Request.Context())
	if !ownerOK || owner == "" {
		if value, exists := c.Get(types.UserIDContextKey.String()); exists {
			owner, ownerOK = value.(string)
		}
	}
	return tenant, owner, ok && ownerOK && tenant != 0 && strings.TrimSpace(owner) != ""
}

func (h *MobileDeviceHandler) Register(c *gin.Context) {
	tenant, owner, ok := mobileCaller(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if h == nil || h.store == nil || h.seal == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	var req mobileDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	req.Platform = strings.ToLower(strings.TrimSpace(req.Platform))
	if req.Token == "" || len(req.Token) > 4096 || (req.Platform != "ios" && req.Platform != "android") || req.Revision < 0 || req.ScopeGeneration < 0 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if strings.EqualFold(h.environment, "production") && strings.Contains(strings.ToLower(req.Token), "development") {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	ciphertext, err := h.seal(req.Token)
	if err != nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	row := repository.DeviceRegistration{
		TenantID: tenant, SpaceID: strings.TrimSpace(req.SpaceID), DeviceID: strings.TrimSpace(c.Param("id")),
		OwnerID: owner, Environment: h.environment, Platform: req.Platform,
		TokenCiphertext: ciphertext, TokenHash: repository.DeviceTokenHash(req.Token),
		Revision: req.Revision, ScopeGeneration: req.ScopeGeneration,
	}
	if row.DeviceID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err := h.store.Bind(c.Request.Context(), row); err != nil {
		h.writeStoreError(c, err)
		return
	}
	active, err := h.store.GetActiveForTenant(c.Request.Context(), tenant, owner, row.DeviceID)
	if err != nil {
		h.writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"device_id": active.DeviceID, "environment": active.Environment, "platform": active.Platform,
		"scope_generation": active.ScopeGeneration, "revision": active.Revision,
	}})
}

func (h *MobileDeviceHandler) Revoke(c *gin.Context) {
	tenant, owner, ok := mobileCaller(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if h == nil || h.store == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	revision := int64(0)
	if raw := strings.TrimSpace(c.GetHeader("If-Match")); raw != "" {
		parsed, err := strconv.ParseInt(strings.Trim(raw, "\""), 10, 64)
		if err != nil || parsed <= 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		revision = parsed
	}
	if raw := strings.TrimSpace(c.Query("revision")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		revision = parsed
	}
	if err := h.store.RevokeForTenant(c.Request.Context(), tenant, owner, strings.TrimSpace(c.Param("id")), revision); err != nil {
		h.writeStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MobileDeviceHandler) Presence(c *gin.Context) {
	tenant, owner, ok := mobileCaller(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	revision := int64(0)
	if raw := strings.TrimSpace(c.Query("revision")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		revision = parsed
	}
	if err := h.store.MarkPresence(c.Request.Context(), tenant, owner, strings.TrimSpace(c.Param("id")), revision); err != nil {
		h.writeStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parsePresenceRevision(c *gin.Context) (int64, bool) {
	raw := strings.TrimSpace(c.Query("revision"))
	if raw == "" {
		raw = strings.TrimSpace(strings.Trim(c.GetHeader("If-Match"), "\""))
	}
	if raw == "" {
		return 0, true
	}
	revision, err := strconv.ParseInt(raw, 10, 64)
	return revision, err == nil && revision > 0
}

func presenceJSON(c *gin.Context, row repository.DeviceRegistration) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"device_id": row.DeviceID, "environment": row.Environment, "platform": row.Platform,
		"scope_generation": row.ScopeGeneration, "revision": row.Revision, "last_seen_at": row.LastSeenAt,
	}})
}

// GetPresence, PutPresence and DeletePresence form the complete presence
// resource. POST remains as a backwards-compatible heartbeat alias.
func (h *MobileDeviceHandler) GetPresence(c *gin.Context) {
	tenant, owner, ok := mobileCaller(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	revision, valid := parsePresenceRevision(c)
	if !valid {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	row, err := h.store.GetPresence(c.Request.Context(), tenant, owner, strings.TrimSpace(c.Param("id")), revision)
	if err != nil {
		h.writeStoreError(c, err)
		return
	}
	presenceJSON(c, row)
}

func (h *MobileDeviceHandler) PutPresence(c *gin.Context) {
	tenant, owner, ok := mobileCaller(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	revision, valid := parsePresenceRevision(c)
	if !valid {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	row, err := h.store.SetPresence(c.Request.Context(), tenant, owner, strings.TrimSpace(c.Param("id")), revision)
	if err != nil {
		h.writeStoreError(c, err)
		return
	}
	presenceJSON(c, row)
}

func (h *MobileDeviceHandler) DeletePresence(c *gin.Context) {
	tenant, owner, ok := mobileCaller(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	revision, valid := parsePresenceRevision(c)
	if !valid {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err := h.store.DeletePresence(c.Request.Context(), tenant, owner, strings.TrimSpace(c.Param("id")), revision); err != nil {
		h.writeStoreError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Explicit aliases make the HTTP contract easy to inject into routers which
// name handlers after verbs while retaining the short historical Presence
// heartbeat method.
func (h *MobileDeviceHandler) PresenceGet(c *gin.Context)    { h.GetPresence(c) }
func (h *MobileDeviceHandler) PresencePut(c *gin.Context)    { h.PutPresence(c) }
func (h *MobileDeviceHandler) PresenceDelete(c *gin.Context) { h.DeletePresence(c) }

func (h *MobileDeviceHandler) List(c *gin.Context) {
	tenant, owner, ok := mobileCaller(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	rows, err := h.store.ListActiveForTenant(c.Request.Context(), tenant, owner, h.environment)
	if err != nil {
		h.writeStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}

func (h *MobileDeviceHandler) writeStoreError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrMobileDeviceNotFound):
		c.AbortWithStatus(http.StatusNotFound)
	case errors.Is(err, repository.ErrMobileDeviceRevision):
		c.AbortWithStatus(http.StatusConflict)
	case errors.Is(err, repository.ErrMobileDeviceInvalid):
		c.AbortWithStatus(http.StatusBadRequest)
	default:
		// Do not expose DB details (which can include SQL or token hashes).
		if os.Getenv("GIN_MODE") == "test" {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}
