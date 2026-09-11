package handler

import (
	"crypto/subtle"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
)

// SemanticInternalHandler exposes scope resolution to the semantic service
// over an internal, service-identity-authenticated route. It never serves
// knowledge content - only authorization snapshots.
type SemanticInternalHandler struct {
	resolveToken string
	scopeService *service.SemanticScopeService
}

// NewSemanticInternalHandler returns nil when the internal resolve token is
// not configured: no token, no endpoint (fail closed).
func NewSemanticInternalHandler(resolveToken string, scopeService *service.SemanticScopeService) *SemanticInternalHandler {
	if resolveToken == "" || scopeService == nil {
		return nil
	}
	return &SemanticInternalHandler{resolveToken: resolveToken, scopeService: scopeService}
}

// RegisterSemanticInternalRoutes mounts the semantic internal endpoints.
// Every request must carry the approved service identity header.
func RegisterSemanticInternalRoutes(r *gin.RouterGroup, resolveToken string, scopeService *service.SemanticScopeService) {
	if resolveToken == "" || scopeService == nil {
		return // fail closed: without a configured identity no route exists
	}
	h := &SemanticInternalHandler{resolveToken: resolveToken, scopeService: scopeService}
	group := r.Group("/internal/semantic")
	group.Use(serviceIdentityMiddleware(resolveToken))
	group.POST("/scope/resolve", h.ResolveScope)
}

func serviceIdentityMiddleware(resolveToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		presented := c.GetHeader("X-Semantic-Internal-Token")
		if presented == "" || subtle.ConstantTimeCompare([]byte(presented), []byte(resolveToken)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "semantic service identity required"})
			return
		}
		c.Next()
	}
}

// ResolveToken exposes the configured service identity (for route mounting).
func (h *SemanticInternalHandler) ResolveToken() string { return h.resolveToken }

// ScopeService exposes the backing scope service (for route mounting).
func (h *SemanticInternalHandler) ScopeService() *service.SemanticScopeService {
	return h.scopeService
}

type resolveScopeRequest struct {
	ScopeRef string `json:"scope_ref"`
}

// ResolveScope turns a signed scope reference into the authorization
// snapshot the semantic service computes against. Recognizable failures:
// 401 (identity), 403 (invalid scope), 409 (authorization changed - the
// caller must discard everything derived under the old scope).
func (h *SemanticInternalHandler) ResolveScope(c *gin.Context) {
	var request resolveScopeRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.ScopeRef == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope_ref is required"})
		return
	}
	snapshot, err := h.scopeService.Resolve(c.Request.Context(), request.ScopeRef)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, snapshot)
	case errors.Is(err, service.ErrSemanticScopeChanged):
		c.JSON(http.StatusConflict, gin.H{"error": "authorization changed", "detail": err.Error()})
	case errors.Is(err, service.ErrSemanticScopeInvalid):
		c.JSON(http.StatusForbidden, gin.H{"error": "scope invalid", "detail": err.Error()})
	default:
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": apperrors.NewServiceUnavailableError("scope resolution failed").Error()})
	}
}
