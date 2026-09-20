package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// fakeTenantExpertMarket is the interfaces.TenantExpertMarketService seam.
type fakeTenantExpertMarket struct {
	publishTenant uint64
	publishID     string
	publishBy     string
	publishReq    interfaces.PublishAgentExpertRequest
	publishErr    error
	publish       *interfaces.PublishedExpertView

	unpublishTenant uint64
	unpublishID     string
	unpublishErr    error

	listTenant uint64
	listErr    error
	list       *interfaces.PublishedExpertIndex

	installTenant uint64
	installID     string
	installReq    interfaces.InstantiateRequest
	installErr    error
	install       *interfaces.InstantiateResult
}

func (f *fakeTenantExpertMarket) PublishAgentExpert(
	_ context.Context, tenantID uint64, agentID, publishedBy string, req interfaces.PublishAgentExpertRequest,
) (*interfaces.PublishedExpertView, error) {
	f.publishTenant = tenantID
	f.publishID = agentID
	f.publishBy = publishedBy
	f.publishReq = req
	return f.publish, f.publishErr
}

func (f *fakeTenantExpertMarket) UnpublishExpert(_ context.Context, tenantID uint64, publishedID string) error {
	f.unpublishTenant = tenantID
	f.unpublishID = publishedID
	return f.unpublishErr
}

func (f *fakeTenantExpertMarket) ListPublishedExperts(
	_ context.Context, tenantID uint64,
) (*interfaces.PublishedExpertIndex, error) {
	f.listTenant = tenantID
	return f.list, f.listErr
}

func (f *fakeTenantExpertMarket) InstallPublishedExpert(
	_ context.Context, tenantID uint64, publishedID string, req interfaces.InstantiateRequest,
) (*interfaces.InstantiateResult, error) {
	f.installTenant = tenantID
	f.installID = publishedID
	f.installReq = req
	return f.install, f.installErr
}

// ---------------------------------------------------------------------------

func TestTenantExpertPublishDecodesStrictBodyAndAccepts(t *testing.T) {
	market := &fakeTenantExpertMarket{publish: &interfaces.PublishedExpertView{ID: "pe-1"}}
	r := newTenantMarketRouter(true, true)
	r.POST("/agents/:id/publish-expert", NewTenantExpertMarketHandler(market).PublishAgentExpert)

	for _, body := range []string{"", `{"name":"市场名","description":"市场描述"}`} {
		w := doTenantMarketRequest(r, http.MethodPost, "/agents/agent-1/publish-expert", body, "application/json")
		require.Equal(t, http.StatusCreated, w.Code, "body %q: %s", body, w.Body.String())
	}
	require.Equal(t, "agent-1", market.publishID)
	require.Equal(t, testSkillTenantID, market.publishTenant)
	require.Equal(t, "user-1", market.publishBy)
	require.Equal(t, "市场名", market.publishReq.Name)
	require.Equal(t, "市场描述", market.publishReq.Description)

	// Strict: unknown fields, non-object bodies and trailing input are 400s.
	for _, body := range []string{
		`{"name":"x","extra":true}`,
		`{"name":"x"}{"name":"y"}`,
		`null`,
		`[]`,
	} {
		w := doTenantMarketRequest(r, http.MethodPost, "/agents/agent-1/publish-expert", body, "application/json")
		require.Equal(t, http.StatusBadRequest, w.Code, "body %q must be refused", body)
	}

	// The 201 body carries the row.
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	w := doTenantMarketRequest(r, http.MethodPost, "/agents/agent-1/publish-expert", "", "application/json")
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, "pe-1", payload.Data.ID)
}

func TestTenantExpertPublishCapsBodySizeAndFieldLengths(t *testing.T) {
	market := &fakeTenantExpertMarket{publish: &interfaces.PublishedExpertView{ID: "pe-1"}}
	r := newTenantMarketRouter(true, true)
	r.POST("/agents/:id/publish-expert", NewTenantExpertMarketHandler(market).PublishAgentExpert)

	big := strings.Repeat("a", skillSourceJSONMaxBytes+1)
	w := doTenantMarketRequest(r, http.MethodPost, "/agents/agent-1/publish-expert", big, "text/plain")
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "too large")

	w = doTenantMarketRequest(r, http.MethodPost, "/agents/agent-1/publish-expert",
		`{"name":"`+strings.Repeat("名", 256)+`"}`, "application/json")
	require.Equal(t, http.StatusBadRequest, w.Code, "name beyond 255 runes must be refused")

	w = doTenantMarketRequest(r, http.MethodPost, "/agents/agent-1/publish-expert",
		`{"description":"`+strings.Repeat("述", 2001)+`"}`, "application/json")
	require.Equal(t, http.StatusBadRequest, w.Code, "description beyond 2000 runes must be refused")
}

func TestTenantExpertPublishMapsErrors(t *testing.T) {
	market := &fakeTenantExpertMarket{publishErr: apperrors.NewNotFoundError("agent not found")}
	r := newTenantMarketRouter(true, true)
	r.POST("/agents/:id/publish-expert", NewTenantExpertMarketHandler(market).PublishAgentExpert)

	w := doTenantMarketRequest(r, http.MethodPost, "/agents/nope/publish-expert", "", "application/json")
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestTenantExpertUnpublishRespondsAndMapsErrors(t *testing.T) {
	market := &fakeTenantExpertMarket{}
	r := newTenantMarketRouter(true, true)
	r.DELETE("/market/tenant/experts/:id", NewTenantExpertMarketHandler(market).UnpublishExpert)

	w := doTenantMarketRequest(r, http.MethodDelete, "/market/tenant/experts/pe-1", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, testSkillTenantID, market.unpublishTenant)
	require.Equal(t, "pe-1", market.unpublishID)

	market.unpublishErr = apperrors.NewNotFoundError("expert not found")
	w = doTenantMarketRequest(r, http.MethodDelete, "/market/tenant/experts/pe-1", "", "")
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestTenantExpertListServesPublishedExperts(t *testing.T) {
	market := &fakeTenantExpertMarket{list: &interfaces.PublishedExpertIndex{
		Experts: []interfaces.PublishedExpertEntry{{
			ID: "pe-1", Name: "合同审查助手", Description: "审查合同",
			PublisherName: "alice", Installed: true,
		}},
	}}
	r := newTenantMarketRouter(true, false)
	r.GET("/market/tenant/experts", NewTenantExpertMarketHandler(market).ListPublishedExperts)

	w := doTenantMarketRequest(r, http.MethodGet, "/market/tenant/experts", "", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, testSkillTenantID, market.listTenant)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Experts []interfaces.PublishedExpertEntry `json:"experts"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Len(t, body.Data.Experts, 1)
	require.Equal(t, "合同审查助手", body.Data.Experts[0].Name)
	require.True(t, body.Data.Experts[0].Installed)
}

func TestTenantExpertInstallDecodesStrictBodyAndAccepts(t *testing.T) {
	market := &fakeTenantExpertMarket{install: &interfaces.InstantiateResult{
		Agent:           &types.CustomAgent{ID: "agent-copy", Name: "副本"},
		PendingSkills:   []string{"pdf-extract"},
		SkillInstallIDs: []string{},
	}}
	r := newTenantMarketRouter(true, false)
	r.POST("/market/tenant/experts/:id/install", NewTenantExpertMarketHandler(market).InstallPublishedExpert)

	w := doTenantMarketRequest(r, http.MethodPost, "/market/tenant/experts/pe-1/install",
		`{"agent_name":"副本","sandbox_config_id":"cfg-1"}`, "application/json")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Equal(t, testSkillTenantID, market.installTenant)
	require.Equal(t, "pe-1", market.installID)
	require.Equal(t, "副本", market.installReq.AgentName)
	require.Equal(t, "cfg-1", market.installReq.SandboxConfigID)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Agent          *types.CustomAgent `json:"agent"`
			PendingSkills  []string           `json:"pending_skills"`
			SkillInstallID []string           `json:"skill_install_ids"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "agent-copy", body.Data.Agent.ID)
	require.Equal(t, []string{"pdf-extract"}, body.Data.PendingSkills)

	// Strict: unknown fields, trailing input and oversized bodies are 400s.
	for _, raw := range []string{
		`{"agent_name":"x","extra":true}`,
		`{"agent_name":"x"}{}`,
	} {
		w = doTenantMarketRequest(r, http.MethodPost, "/market/tenant/experts/pe-1/install", raw, "application/json")
		require.Equal(t, http.StatusBadRequest, w.Code, "body %q must be refused", raw)
	}
	big := strings.Repeat(" ", skillSourceJSONMaxBytes+1)
	w = doTenantMarketRequest(r, http.MethodPost, "/market/tenant/experts/pe-1/install", big, "application/json")
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "too large")

	// agent_name beyond the column cap is a 400 (the instantiate precedent).
	w = doTenantMarketRequest(r, http.MethodPost, "/market/tenant/experts/pe-1/install",
		`{"agent_name":"`+strings.Repeat("名", 256)+`"}`, "application/json")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTenantExpertInstallMapsServiceErrors(t *testing.T) {
	market := &fakeTenantExpertMarket{installErr: apperrors.NewNotFoundError("expert is not published")}
	r := newTenantMarketRouter(true, false)
	r.POST("/market/tenant/experts/:id/install", NewTenantExpertMarketHandler(market).InstallPublishedExpert)

	w := doTenantMarketRequest(r, http.MethodPost, "/market/tenant/experts/nope/install", "", "application/json")
	require.Equal(t, http.StatusNotFound, w.Code)
}
