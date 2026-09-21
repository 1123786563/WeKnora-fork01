package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/gin-gonic/gin"
)

type SemanticScopeResolver interface {
	Resolve(context.Context, string) (service.SemanticScopeSnapshot, error)
}
type SemanticInternalHandler struct {
	cfg      *config.SemanticServiceConfig
	resolver SemanticScopeResolver
}

func NewSemanticInternalHandler(cfg *config.Config, resolver SemanticScopeResolver) *SemanticInternalHandler {
	return &SemanticInternalHandler{cfg: cfg.Semantic, resolver: resolver}
}
func (h *SemanticInternalHandler) Resolve(c *gin.Context) {
	if h.cfg == nil || !h.cfg.Enabled || h.cfg.ServiceToken == "" || h.cfg.Audience == "" || subtle.ConstantTimeCompare([]byte(c.GetHeader("Authorization")), []byte("Bearer "+h.cfg.ServiceToken)) != 1 || c.GetHeader("X-WeKnora-Audience") != h.cfg.Audience {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var input struct {
		ScopeRef string `json:"scope_ref"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 16384))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || input.ScopeRef == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if h.resolver == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "semantic unavailable"})
		return
	}
	snapshot, err := h.resolver.Resolve(c.Request.Context(), input.ScopeRef)
	if err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, service.ErrSemanticScopeInvalid) || errors.Is(err, service.ErrSemanticScopeExpired) || errors.Is(err, service.ErrSemanticScopeChanged) {
			status = http.StatusUnauthorized
		}
		c.AbortWithStatusJSON(status, gin.H{"error": "scope unavailable"})
		return
	}
	c.JSON(http.StatusOK, snapshot)
}
