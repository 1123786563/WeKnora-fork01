package handler

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

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
	CurrentScopeGeneration(context.Context, uint64, string, string) (int64, error)
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

type mobileRegistrationIntent struct {
	Tenant uint64 `json:"tenant"`
	Owner  string `json:"owner"`
	Device string `json:"device"`
	Epoch  int64  `json:"epoch"`
	Nonce  string `json:"nonce"`
	Expiry int64  `json:"expiry"`
}

func registrationIntentKey() []byte { return utils.SystemHMACKey() }

func encodeRegistrationIntent(in mobileRegistrationIntent) (string, error) {
	key := registrationIntentKey()
	if len(key) < 16 {
		return "", errors.New("mobile registration intent signing is not configured")
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + hex.EncodeToString(mac.Sum(nil)), nil
}

func verifyRegistrationIntent(value string, tenant uint64, owner, device string, current int64) (int64, error) {
	key := registrationIntentKey()
	if len(key) < 16 {
		return 0, repository.ErrMobileDeviceInvalid
	}
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return 0, repository.ErrMobileDeviceRevision
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, repository.ErrMobileDeviceRevision
	}
	sig, err := hex.DecodeString(parts[1])
	if err != nil {
		return 0, repository.ErrMobileDeviceRevision
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(raw)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return 0, repository.ErrMobileDeviceRevision
	}
	var in mobileRegistrationIntent
	if json.Unmarshal(raw, &in) != nil || in.Tenant != tenant || in.Owner != owner || in.Device != device || in.Epoch != current+1 || in.Expiry < time.Now().Unix() || in.Nonce == "" {
		return 0, repository.ErrMobileDeviceRevision
	}
	return in.Epoch, nil
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
	Token              string `json:"token"`
	Platform           string `json:"platform"`
	SpaceID            string `json:"space_id,omitempty"`
	Revision           int64  `json:"revision,omitempty"`
	ScopeGeneration    int64  `json:"scope_generation,omitempty"`
	RegistrationIntent string `json:"registration_intent"`
}

// IssueIntent creates a short-lived, server-signed epoch transition. Clients
// cannot manufacture a future epoch or replay an intent after logout moves
// the durable high-water mark.
func (h *MobileDeviceHandler) IssueIntent(c *gin.Context) {
	tenant, owner, ok := mobileCaller(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if h == nil || h.store == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	device := strings.TrimSpace(c.Param("id"))
	if device == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	current, err := h.store.CurrentScopeGeneration(c.Request.Context(), tenant, owner, device)
	if err != nil {
		h.writeStoreError(c, err)
		return
	}
	nonce := make([]byte, 24)
	if _, err := rand.Read(nonce); err != nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	intent, err := encodeRegistrationIntent(mobileRegistrationIntent{Tenant: tenant, Owner: owner, Device: device, Epoch: current + 1, Nonce: base64.RawURLEncoding.EncodeToString(nonce), Expiry: time.Now().Add(5 * time.Minute).Unix()})
	if err != nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"registration_intent": intent, "scope_generation": current + 1}})
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
	current, err := h.store.CurrentScopeGeneration(c.Request.Context(), tenant, owner, strings.TrimSpace(c.Param("id")))
	if err != nil {
		h.writeStoreError(c, err)
		return
	}
	epoch, err := verifyRegistrationIntent(strings.TrimSpace(req.RegistrationIntent), tenant, owner, strings.TrimSpace(c.Param("id")), current)
	if err != nil {
		h.writeStoreError(c, err)
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
		Revision: req.Revision, ScopeGeneration: epoch,
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
	if epoch, err := h.store.CurrentScopeGeneration(c.Request.Context(), tenant, owner, strings.TrimSpace(c.Param("id"))); err == nil {
		c.Header("X-Mobile-Scope-Generation", strconv.FormatInt(epoch, 10))
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
