package handler

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
)

// SemanticModelInternalHandler exposes the controlled model gateway to the
// semantic service over an internal, service-identity-authenticated route.
// No user RBAC, no raw provider URLs, no long-term keys.
type SemanticModelInternalHandler struct {
	resolveToken string
	modelService *service.SemanticModelService
}

// NewSemanticModelInternalHandler returns nil without a configured token
// (fail closed: no token, no endpoint).
func NewSemanticModelInternalHandler(resolveToken string, modelService *service.SemanticModelService) *SemanticModelInternalHandler {
	if resolveToken == "" || modelService == nil {
		return nil
	}
	return &SemanticModelInternalHandler{resolveToken: resolveToken, modelService: modelService}
}

// ResolveToken exposes the configured service identity (route mounting).
func (h *SemanticModelInternalHandler) ResolveToken() string { return h.resolveToken }

// ModelService exposes the backing gateway (route mounting).
func (h *SemanticModelInternalHandler) ModelService() *service.SemanticModelService {
	return h.modelService
}

// RegisterSemanticModelInternalRoutes mounts the internal model endpoints.
func RegisterSemanticModelInternalRoutes(r *gin.RouterGroup, resolveToken string, modelService *service.SemanticModelService) {
	if resolveToken == "" || modelService == nil {
		return
	}
	h := &SemanticModelInternalHandler{resolveToken: resolveToken, modelService: modelService}
	group := r.Group("/internal/semantic/model")
	group.Use(func(c *gin.Context) {
		presented := c.GetHeader("X-Semantic-Internal-Token")
		if presented == "" || subtle.ConstantTimeCompare([]byte(presented), []byte(resolveToken)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "semantic service identity required"})
			return
		}
		c.Next()
	})
	group.POST("/invoke", h.Invoke)
}

type semanticModelInvokeRequest struct {
	InvocationID    string                       `json:"invocation_id"`
	OperationID     string                       `json:"operation_id"`
	TenantID        uint64                       `json:"tenant_id"`
	KBID            string                       `json:"kb_id"`
	ModelProfileRef string                       `json:"model_profile_ref"`
	BudgetRef       string                       `json:"budget_ref"`
	Messages        []types.SemanticModelMessage `json:"messages"`
	MaxOutputTokens int                          `json:"max_output_tokens"`
	EstimatedTokens int                          `json:"estimated_tokens"`
}

// Invoke admits and performs one controlled model call. Recognizable
// failures: 401 identity; 400 bad payload; 409 needs-reconciliation or
// conflict; 402 budget exhausted; 502 provider failure.
func (h *SemanticModelInternalHandler) Invoke(c *gin.Context) {
	var request semanticModelInvokeRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.InvocationID == "" || len(request.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invocation_id and messages are required"})
		return
	}
	ctx, cancel := contextWithHeaderDeadline(c)
	defer cancel()
	result, err := h.modelService.Invoke(ctx, service.SemanticModelInvocation{
		InvocationID: request.InvocationID, OperationID: request.OperationID,
		TenantID: request.TenantID, KBID: request.KBID,
		ModelProfileRef: request.ModelProfileRef, BudgetRef: request.BudgetRef,
		Messages: request.Messages, MaxOutputTokens: request.MaxOutputTokens,
		EstimatedTokens: request.EstimatedTokens,
	})
	switch {
	case err == nil:
		c.JSON(http.StatusOK, result)
	case errors.Is(err, service.ErrSemanticInvocationNeedsReconciliation),
		errors.Is(err, service.ErrSemanticInvocationConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrSemanticBudgetExhausted):
		c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error()})
	case strings.Contains(err.Error(), "provider"):
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
	default:
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "model gateway failure"})
	}
}

// contextWithHeaderDeadline honors an X-Semantic-Deadline-Ms header so the
// semantic service propagates its remaining budget into the provider call.
func contextWithHeaderDeadline(c *gin.Context) (context.Context, context.CancelFunc) {
	ctx := c.Request.Context()
	if raw := c.GetHeader("X-Semantic-Deadline-Ms"); raw != "" {
		var ms int
		if _, err := fmt.Sscan(raw, &ms); err == nil && ms > 0 {
			return context.WithTimeout(ctx, time.Duration(ms)*time.Millisecond)
		}
	}
	return context.WithCancel(ctx)
}
