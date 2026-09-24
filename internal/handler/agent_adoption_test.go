package handler

// Agent-adoption handler tests (B3 batch): the publish response envelope must
// match its sibling Variant endpoints — data IS the variant body, never a
// double-wrapped {variant: {...}} (B3-F86).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubAdoptionService struct {
	interfaces.AgentAdoptionService
	publishResult interfaces.PublishVariantResult
}

func (s *stubAdoptionService) PublishVariant(_ context.Context, _ uint64, _, _ string) (interfaces.PublishVariantResult, error) {
	return s.publishResult, nil
}

func adoptionHandlerContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/marketplace/tenant/variants/v1/publish", nil)
	c.Params = gin.Params{{Key: "id", Value: "v1"}}
	return c, recorder
}

func TestPublishVariantReturnsVariantBodyDirectly(t *testing.T) {
	h := NewAgentAdoptionHandler(&stubAdoptionService{publishResult: interfaces.PublishVariantResult{
		Variant: interfaces.AdoptionVariantView{
			AgentAdoptionVariantEntity: types.AgentAdoptionVariantEntity{
				TenantID: 1, ID: "v1", AdoptionID: "a1", ReleaseID: "r1", State: "published", Name: "V",
			},
		},
	}})
	c, rec := adoptionHandlerContext(t)
	h.PublishVariant(c)

	require.Equal(t, http.StatusOK, c.Writer.Status())
	var body struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "v1", body.Data["id"], "data 直接是 variant 本体（id 在顶层）")
	require.Equal(t, "published", body.Data["state"])
	_, wrapped := body.Data["variant"]
	require.False(t, wrapped, "发布响应不得双层包裹——与同文件其余 Variant 端点一致（B3-F86）")
}
