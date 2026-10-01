package container

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appservice "github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWorkbenchAdmissionRejectsRetiredAgentFromProductionProvider(t *testing.T) {
	db := wiringTestDB(t)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{
		ID: "adoption-retired", TenantID: 1, ListingID: "listing-retired",
		AcceptedReleaseID: "release-retired", State: "active",
	}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{
		ID: "variant-retired", TenantID: 1, AdoptionID: "adoption-retired",
		ReleaseID: "release-retired", Name: "Retired agent", State: "retired",
		LocalAgentID: "retired-agent",
	}).Error)

	coordinator := NewWorkbenchAdmissionCoordinator(
		&config.Config{}, db, repository.NewAgentRunStore(db),
		repository.NewExecutionTargetStore(db), repository.NewAgentAdoptionRepository(db),
		appservice.NewAgentSecurityService(repository.NewAgentSecurityStore(db), repository.NewAgentRunStore(db)),
	)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/workbench/executions", func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Set(types.UserIDContextKey.String(), "u-wiring")
		c.Next()
	}, session.NewWorkbenchStartHandler(coordinator).Start)

	requestID := "req-retired-agent"
	body := `{"session_id":"s-wiring","agent_id":"retired-agent","target_id":"platform","request_id":"` + requestID + `","text":"start work","budget_upper":100}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/executions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusConflict, resp.Code, resp.Body.String())
	var requests, runs int64
	require.NoError(t, db.Table("workbench_requests").Where("tenant_id = ? AND request_id = ?", 1, requestID).Count(&requests).Error)
	require.Zero(t, requests, "denied retired-agent start must not create a durable request")
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND request_id = ?", 1, requestID).Count(&runs).Error)
	require.Zero(t, runs, "denied retired-agent start must not create a run")
}
