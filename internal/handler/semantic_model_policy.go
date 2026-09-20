package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	policy "github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

type SemanticModelPolicyHandler struct {
	service *policy.SemanticModelPolicyService
}

func NewSemanticModelPolicyHandler(service *policy.SemanticModelPolicyService) *SemanticModelPolicyHandler {
	return &SemanticModelPolicyHandler{service: service}
}
func (h *SemanticModelPolicyHandler) Get(c *gin.Context) {
	scope, err := semanticModelPolicyScope(c)
	if err != nil {
		c.Error(err)
		return
	}
	p, err := h.service.Get(c.Request.Context(), scope)
	if err != nil {
		c.Error(policyHTTPError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": p})
}
func (h *SemanticModelPolicyHandler) Put(c *gin.Context) {
	var input policy.SemanticModelPolicyInput
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid semantic model policy"))
		return
	}
	if err := ensureJSONEOF(dec); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid semantic model policy"))
		return
	}
	scope, err := semanticModelPolicyScope(c)
	if err != nil {
		c.Error(err)
		return
	}
	if h == nil || h.service == nil {
		c.Error(apperrors.NewServiceUnavailableError("semantic model policy is unavailable"))
		return
	}
	p, err := h.service.Put(c.Request.Context(), scope, input)
	if err != nil {
		c.Error(policyHTTPError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": p})
}
func ensureJSONEOF(dec *json.Decoder) error {
	var extra any
	err := dec.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return errors.New("trailing json")
}
func semanticModelPolicyScope(c *gin.Context) (policy.SemanticModelPolicyScope, error) {
	tenant, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenant == 0 {
		return policy.SemanticModelPolicyScope{}, apperrors.NewForbiddenError("semantic model policy requires tenant scope")
	}
	actor, ok := types.UserIDFromContext(c.Request.Context())
	if !ok || actor == "" {
		return policy.SemanticModelPolicyScope{}, apperrors.NewForbiddenError("semantic model policy requires actor")
	}
	return policy.SemanticModelPolicyScope{TenantID: tenant, KBID: c.Param("id"), ActorID: actor}, nil
}
func policyHTTPError(err error) error {
	switch {
	case errors.Is(err, policy.ErrSemanticModelPricingUnavailable):
		return apperrors.NewServiceUnavailableError("semantic model pricing is unavailable")
	case errors.Is(err, policy.ErrSemanticModelPolicyDisabled):
		return apperrors.NewNotFoundError("semantic model policy is disabled")
	default:
		return apperrors.NewBadRequestError("invalid semantic model policy")
	}
}
