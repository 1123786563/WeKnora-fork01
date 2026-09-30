package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type publicAdoptionConflictService struct {
	interfaces.PublicMarketplaceService
}

func (publicAdoptionConflictService) AdoptPublicListing(context.Context, uint64, string, string, string) (interfaces.PublicAdoptionResult, bool, error) {
	return interfaces.PublicAdoptionResult{}, false, repository.ErrAgentAdoptionTransition
}

func TestAdoptPublicListingMapsEndedAdoptionToHTTPConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	handler := NewPublicMarketplaceHandler(publicAdoptionConflictService{})
	r.POST("/marketplace/public/listings/:id/adopt", func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(2))
		ctx := context.WithValue(c.Request.Context(), types.UserIDContextKey, "adopter-admin")
		c.Request = c.Request.WithContext(ctx)
		handler.AdoptPublicListing(c)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/marketplace/public/listings/public-listing/adopt", strings.NewReader(`{}`))
	r.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusConflict, recorder.Code, recorder.Body.String())
}
