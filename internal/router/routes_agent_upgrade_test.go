package router

// Agent-upgrade route tests (T31 #61). This file hosts the governance-floor
// assertions (Task 5) and the full-lifecycle HTTP e2e (Task 6).

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAgentUpgradeRoutesRequireAdminAndFullAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 升级建议审阅是治理写面（spec §9 管理员比较/接受/驳回）：与 Adoption
	// 治理路由同款 Admin+ full-access 地板（spec §13 adopt_agent /
	// configure_variant）。
	g := &rbacGuards{}
	v1 := gin.New().Group("/api/v1")
	RegisterAgentUpgradeRoutes(v1, handler.NewAgentUpgradeHandler(nil), g)

	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals"},
		{http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals/:id"},
		{http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/:id/accept"},
		{http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/:id/dismiss"},
	}
	for _, route := range routes {
		policy := mustLookupAPIKeyPolicy(t, g, route.method, route.path)
		require.Truef(t, policy.RequireFullAccess, "%s %s 必须要求 full-access", route.method, route.path)
		require.Falsef(t, policyHasCapability(policy, types.APIKeyCapabilityIngest),
			"%s %s 不得被无关能力放行", route.method, route.path)
	}

	// nil handler fail closed：不挂载任何路由。
	bare := gin.New()
	RegisterAgentUpgradeRoutes(bare.Group("/api/v1"), nil, &rbacGuards{})
	require.Empty(t, bare.Routes())
}

// ---------- Task 6: end-to-end lifecycle over the real stack ----------

// newAgentUpgradeTestApp mounts the REAL release -> adoption -> upgrade
// stack over the real migration stream: real CustomAgentService /
// AgentVersionService / AgentMarketplaceService / AgentAdoptionService /
// AgentUpgradeService and the real agent list endpoint (GET /api/v1/agents)
// that the mobile Resource Shelf consumes. Setup mirrors
// newAgentAdoptionTestApp without modifying it (parallel-batch merge safety).
func newAgentUpgradeTestApp(t *testing.T) (*gin.Engine, *rbacGuards, *gorm.DB) {
	t.Helper()
	db := openTenantAgentMarketplaceHTTPTestDB(t)
	require.NoError(t, db.Create(&types.CustomAgent{
		ID: "agent-owned", Name: "Upgrade helper", TenantID: 1, CreatedBy: "contributor",
		Config: types.CustomAgentConfig{AgentMode: "smart-reasoning", SystemPrompt: "Be useful."},
	}).Error)

	marketRepo := repository.NewAgentMarketplaceRepository(db)
	customAgents := service.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil, nil)
	versions := service.NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	market := service.NewAgentMarketplaceService(versions, marketplaceHTTPResolver{}, marketRepo, t.TempDir())
	adoptions := service.NewAgentAdoptionService(repository.NewAgentAdoptionRepository(db), customAgents, versions)
	upgrades := service.NewAgentUpgradeService(repository.NewAgentUpgradeRepository(db))

	versionHandler := handler.NewAgentVersionHandler(versions)
	marketHandler := handler.NewAgentMarketplaceHandler(market, versions)
	adoptionHandler := handler.NewAgentAdoptionHandler(adoptions)
	upgradeHandler := handler.NewAgentUpgradeHandler(upgrades)
	agentListHandler := handler.NewCustomAgentHandler(customAgents, nil, repository.NewTenantDisabledSharedAgentRepository(db), nil, nil, service.HostSandboxManager{})

	enabled := true
	g := &rbacGuards{cfg: &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enabled}}, agentCreator: func(c *gin.Context) (string, error) {
		if c.Param("id") == "agent-owned" {
			return "contributor", nil
		}
		return "", nil
	}}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.Use(func(c *gin.Context) {
		tenantID := uint64(1)
		if c.GetHeader("X-Test-Tenant") == "2" {
			tenantID = 2
		}
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, tenantID)
		ctx = context.WithValue(ctx, types.UserIDContextKey, c.GetHeader("X-Test-Actor"))
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(c.GetHeader("X-Test-Role")))
		c.Request = c.Request.WithContext(ctx)
		c.Set(types.TenantIDContextKey.String(), tenantID)
		c.Next()
	})
	v1 := r.Group("/api/v1")
	RegisterAgentVersionRoutes(v1, versionHandler, g)
	RegisterAgentMarketplaceRoutes(v1, marketHandler, g)
	RegisterAgentAdoptionRoutes(v1, adoptionHandler, g)
	RegisterAgentUpgradeRoutes(v1, upgradeHandler, g)
	RegisterCustomAgentRoutes(v1, agentListHandler, g)
	return r, g, db
}

// freezeAndPublishUpgradeRelease drives the real HTTP release workflow for
// ONE semantic version of agent-owned: freeze a fresh AgentVersion of the
// CURRENT agent row, submit a Release with the given metadata and approve
// it. metadata 必须自带 supported_languages/use_cases/capability_requirements/
// minimum_weknora_capability/license_id（semantic_version/display_name/summary
// 由本 helper 注入）。返回 listing id 与 release id。
func freezeAndPublishUpgradeRelease(t *testing.T, r *gin.Engine, semanticVersion string, metadata map[string]any) (listingID, releaseID string) {
	t.Helper()
	frozen := adoptionCall(r, 1, http.MethodPost, "/api/v1/agents/agent-owned/versions", "contributor", "contributor", nil)
	require.Equal(t, http.StatusCreated, frozen.Code, frozen.Body.String())
	var frozenBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(frozen.Body.Bytes(), &frozenBody))
	require.NotEmpty(t, frozenBody.Data.ID)

	metadata["semantic_version"] = semanticVersion
	metadata["display_name"] = "Upgrade helper"
	metadata["summary"] = "Portable upgrade helper"
	submitted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "contributor", "contributor", map[string]any{"agent_version_id": frozenBody.Data.ID, "metadata": metadata})
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var submissionBody struct {
		Data struct {
			ID           string `json:"id"`
			ListingID    string `json:"listing_id"`
			BundleDigest string `json:"bundle_digest"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &submissionBody))

	approved := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions/"+submissionBody.Data.ID+"/review", "admin", "reviewer", map[string]any{"expected_digest": submissionBody.Data.BundleDigest, "decision": "approved"})
	require.Equal(t, http.StatusOK, approved.Code, approved.Body.String())
	var result struct {
		Data struct {
			Release *struct {
				ID string `json:"id"`
			} `json:"release"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(approved.Body.Bytes(), &result))
	require.NotNil(t, result.Data.Release)
	return submissionBody.Data.ListingID, result.Data.Release.ID
}

// publishUpgradeVariant drives the #59 variant lifecycle to published over
// real HTTP: create draft, map both capabilities, test, publish. Returns
// the variant id and the instantiated local agent id.
func publishUpgradeVariant(t *testing.T, r *gin.Engine, adoptionID, name, modelID, knowledgeID string) (variantID, localAgentID string) {
	t.Helper()
	variant := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoptionID+"/variants", "admin", "admin", map[string]any{"name": name})
	require.Equal(t, http.StatusCreated, variant.Code, variant.Body.String())
	var variantBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(variant.Body.Bytes(), &variantBody))
	localAgentID = mapTestPublishUpgradeVariant(t, r, variantBody.Data.ID, modelID, knowledgeID)
	return variantBody.Data.ID, localAgentID
}

// mapTestPublishUpgradeVariant drives the #59 既有流程（重新映射 → 测试 →
// 发布）on an EXISTING variant. The spec §9 acceptance flow proceeds on the
// draft the accept endpoint created (pinned to the proposal's to_release);
// minting another variant instead would pin to the adoption's accepted
// (OLD) release — CreateVariant defaults release_id to
// adoption.AcceptedReleaseID — so the upgrade publish must reuse the
// accepted draft's variant id.
func mapTestPublishUpgradeVariant(t *testing.T, r *gin.Engine, variantID, modelID, knowledgeID string) (localAgentID string) {
	t.Helper()
	mapping := adoptionCall(r, 1, http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantID+"/capability-mapping", "admin", "admin", map[string]any{"mappings": []map[string]any{
		{"capability": "model", "model_id": modelID},
		{"capability": "knowledge", "knowledge_base_ids": []string{knowledgeID}},
	}})
	require.Equal(t, http.StatusOK, mapping.Code, mapping.Body.String())
	tested := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantID+"/test", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, tested.Code, tested.Body.String())
	published := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantID+"/publish", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, published.Code, published.Body.String())
	var publishBody struct {
		Data struct {
			LocalAgentID string `json:"local_agent_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(published.Body.Bytes(), &publishBody))
	require.NotEmpty(t, publishBody.Data.LocalAgentID)
	return publishBody.Data.LocalAgentID
}

// agentsRowByName reads the REAL GET /api/v1/agents wire and returns the
// config of the named agent (the projection the mobile Resource Shelf maps).
func agentsRowByName(t *testing.T, r *gin.Engine, name string) (found bool, systemPrompt string, modelID string) {
	t.Helper()
	resp := adoptionCall(r, 1, http.MethodGet, "/api/v1/agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var body struct {
		Data []struct {
			Name   string `json:"name"`
			Config struct {
				SystemPrompt string `json:"system_prompt"`
				ModelID      string `json:"model_id"`
			} `json:"config"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	for _, row := range body.Data {
		if row.Name == name {
			return true, row.Config.SystemPrompt, row.Config.ModelID
		}
	}
	return false, "", ""
}

// upgradeDiffWire mirrors the four-dimension diff on the wire.
type upgradeDiffWire struct {
	Behavior []struct {
		Field string `json:"field"`
		From  string `json:"from"`
		To    string `json:"to"`
	} `json:"behavior"`
	Dependencies []struct {
		Type        string `json:"type"`
		ID          string `json:"id"`
		Change      string `json:"change"`
		FromVersion string `json:"from_version"`
		ToVersion   string `json:"to_version"`
	} `json:"dependencies"`
	Security []struct {
		Field string `json:"field"`
		From  string `json:"from"`
		To    string `json:"to"`
	} `json:"security"`
	License []struct {
		Scope string `json:"scope"`
		ID    string `json:"id"`
		From  string `json:"from"`
		To    string `json:"to"`
	} `json:"license"`
}

func TestAgentUpgradeProposalLifecycleKeepsOldVariantsAndTasks(t *testing.T) {
	r, _, db := newAgentUpgradeTestApp(t)

	// ---- v1 发布（真实 HTTP：冻结 → 提交 → 审核）----
	upgradeMetadataV1 := func() map[string]any {
		return map[string]any{
			"supported_languages": []string{"en"}, "use_cases": []string{"support"},
			"capability_requirements":    []string{"model", "knowledge"},
			"minimum_weknora_capability": "1", "license_id": "MIT",
		}
	}
	listingID, v1 := freezeAndPublishUpgradeRelease(t, r, "1.0.0", upgradeMetadataV1())

	// ---- Adoption + 第一个 Variant 到 published（走 #59 既有端点）----
	adopted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/adoptions", "admin", "admin", map[string]any{"listing_id": listingID})
	require.Equal(t, http.StatusCreated, adopted.Code, adopted.Body.String())
	var adoptionBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoptionBody))
	oldVariantID, oldAgentID := publishUpgradeVariant(t, r, adoptionBody.Data.ID, "Sales Assistant", "gpt-x", "kb-sales")

	// 记录旧世界：快照旧本地 agent 整行（v2 落地前），供下方「此刻」与
	// 「终局」两处整行逐字段比对（require.Equal 深比较，含 Config 与全部
	// 列）；wire 面不变另由真实 GET /api/v1/agents + available-agents +
	// 旧 Variant 行断言共同承载（最终审查 minor：原注释称逐字段比对但仅
	// 存在性加载，此处补齐行级直证）。
	var oldAgent types.CustomAgent
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), oldAgentID).First(&oldAgent).Error)

	// ---- 新 Release 落地前：无建议 ----
	before := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, before.Code, before.Body.String())
	require.JSONEq(t, `{"success":true,"data":[]}`, before.Body.String())

	// ---- v2 发布：真实行为差异（改源 agent 配置后重新冻结）----
	var sourceAgent types.CustomAgent
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), "agent-owned").First(&sourceAgent).Error)
	sourceAgent.Config.SystemPrompt = "Be extra useful and cite sources."
	require.NoError(t, db.Save(&sourceAgent).Error)
	metadataV2 := upgradeMetadataV1()
	metadataV2["data_categories"] = []string{"chat_content"}
	metadataV2["external_side_effects"] = []string{"web_search"}
	metadataV2["license_id"] = "Apache-2.0"
	_, v2 := freezeAndPublishUpgradeRelease(t, r, "1.1.0", metadataV2)

	// ---- AC2：建议出现，四维差异可审阅 ----
	listed := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, listed.Code, listed.Body.String())
	var listBody struct {
		Data []struct {
			ID                string          `json:"id"`
			AdoptionID        string          `json:"adoption_id"`
			ListingID         string          `json:"listing_id"`
			FromReleaseID     string          `json:"from_release_id"`
			ToReleaseID       string          `json:"to_release_id"`
			ToSemanticVersion string          `json:"to_semantic_version"`
			State             string          `json:"state"`
			Diff              upgradeDiffWire `json:"diff"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listed.Body.Bytes(), &listBody))
	require.Len(t, listBody.Data, 1)
	proposal := listBody.Data[0]
	require.Equal(t, adoptionBody.Data.ID, proposal.AdoptionID)
	require.Equal(t, listingID, proposal.ListingID)
	require.Equal(t, v1, proposal.FromReleaseID)
	require.Equal(t, v2, proposal.ToReleaseID)
	require.Equal(t, "1.1.0", proposal.ToSemanticVersion)
	require.Equal(t, "open", proposal.State)

	// 行为维：系统提示词（真实冻结→导出链产生的差异）。
	require.Len(t, proposal.Diff.Behavior, 1)
	require.Equal(t, "system_prompt", proposal.Diff.Behavior[0].Field)
	require.Equal(t, "Be useful.", proposal.Diff.Behavior[0].From)
	require.Equal(t, "Be extra useful and cite sources.", proposal.Diff.Behavior[0].To)

	// 安全维：数据类别 + 外部副作用扩大（capability_requirements 未变 → 不出现）。
	require.Len(t, proposal.Diff.Security, 2)
	require.Equal(t, "data_categories", proposal.Diff.Security[0].Field)
	require.Equal(t, "", proposal.Diff.Security[0].From)
	require.Equal(t, "chat_content", proposal.Diff.Security[0].To)
	require.Equal(t, "external_side_effects", proposal.Diff.Security[1].Field)
	require.Equal(t, "", proposal.Diff.Security[1].From)
	require.Equal(t, "web_search", proposal.Diff.Security[1].To)

	// 许可维：Release 许可 MIT → Apache-2.0（依赖段为空——生产依赖解析器
	// tenantReleaseDependencyResolver fail-closed，真实发布路径无法产生非空
	// DependencyLock；四维全量覆盖由 Task 3 纯函数测试承载）。
	require.Len(t, proposal.Diff.Dependencies, 0)
	require.Len(t, proposal.Diff.License, 1)
	require.Equal(t, "release", proposal.Diff.License[0].Scope)
	require.Equal(t, "license_id", proposal.Diff.License[0].ID)
	require.Equal(t, "MIT", proposal.Diff.License[0].From)
	require.Equal(t, "Apache-2.0", proposal.Diff.License[0].To)

	// 对账幂等（HTTP 面）：再列一次仍恰好一条。
	relisted := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, relisted.Code)
	var relistedBody struct {
		Data []json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(relisted.Body.Bytes(), &relistedBody))
	require.Len(t, relistedBody.Data, 1)

	// ---- 治理边界：viewer 403、跨租户空列表 + 404、畸形 body 400 ----
	viewerList := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "viewer", "viewer", nil)
	require.Equal(t, http.StatusForbidden, viewerList.Code)
	tenant2List := adoptionCall(r, 2, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, tenant2List.Code)
	require.JSONEq(t, `{"success":true,"data":[]}`, tenant2List.Body.String(), "他租户看不到 tenant-1 的建议")
	tenant2Get := adoptionCall(r, 2, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals/"+proposal.ID, "admin", "admin", nil)
	require.Equal(t, http.StatusNotFound, tenant2Get.Code, "跨租户枚举与不存在同形")
	missingGet := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals/missing-proposal", "admin", "admin", nil)
	require.Equal(t, http.StatusNotFound, missingGet.Code)
	emptyName := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/"+proposal.ID+"/accept", "admin", "admin", map[string]any{"name": "  "})
	require.Equal(t, http.StatusBadRequest, emptyName.Code)
	spoofed := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/"+proposal.ID+"/accept", "admin", "admin", map[string]any{"name": "X", "tenant_id": 999})
	require.Equal(t, http.StatusBadRequest, spoofed.Code, "严格解码拒绝 body 冒充 principal")

	// ---- 接受建议：只创建新 Release 上的草稿 Variant（AC1 的接受路径）----
	accepted := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/"+proposal.ID+"/accept", "admin", "admin", map[string]any{"name": "Sales Assistant v1.1"})
	require.Equal(t, http.StatusOK, accepted.Code, accepted.Body.String())
	var acceptBody struct {
		Data struct {
			Proposal struct {
				ID                string `json:"id"`
				State             string `json:"state"`
				AcceptedVariantID string `json:"accepted_variant_id"`
			} `json:"proposal"`
			Variant struct {
				ID                  string   `json:"id"`
				ReleaseID           string   `json:"release_id"`
				State               string   `json:"state"`
				MissingCapabilities []string `json:"missing_capabilities"`
			} `json:"variant"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(accepted.Body.Bytes(), &acceptBody))
	require.Equal(t, "accepted", acceptBody.Data.Proposal.State)
	require.Equal(t, acceptBody.Data.Proposal.AcceptedVariantID, acceptBody.Data.Variant.ID)
	require.Equal(t, v2, acceptBody.Data.Variant.ReleaseID, "草稿固定到新 Release")
	require.Equal(t, "draft", acceptBody.Data.Variant.State)
	require.Equal(t, []string{"knowledge", "model"}, acceptBody.Data.Variant.MissingCapabilities, "新草稿按新 Release 重算缺失能力")

	// 重复 accept：终态互斥，409。
	reAccept := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/"+proposal.ID+"/accept", "admin", "admin", map[string]any{"name": "Again"})
	require.Equal(t, http.StatusConflict, reAccept.Code)
	// draft 不得重复：同一 adoption 的草稿数 = 旧 published 1 + 新 draft 1。
	var draftCount int64
	require.NoError(t, db.Model(&types.AgentAdoptionVariantEntity{}).
		Where("tenant_id = ? AND adoption_id = ? AND state = ?", uint64(1), adoptionBody.Data.ID, "draft").Count(&draftCount).Error)
	require.Equal(t, int64(1), draftCount)

	// ---- AC1：未接受升级的此刻，旧 Variant / 旧本地 Agent / 旧冻结版本原封不动 ----
	var oldVariantRow types.AgentAdoptionVariantEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), oldVariantID).First(&oldVariantRow).Error)
	require.Equal(t, "published", oldVariantRow.State)
	require.Equal(t, v1, oldVariantRow.ReleaseID)
	found, prompt, modelID := agentsRowByName(t, r, "Sales Assistant")
	require.True(t, found)
	require.Equal(t, "Be useful.", prompt, "旧本地 agent 的行为保持旧版本")
	require.Equal(t, "gpt-x", modelID)
	availableNow := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, availableNow.Code)
	var availableBody struct {
		Data []struct {
			AgentID string `json:"agent_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(availableNow.Body.Bytes(), &availableBody))
	require.Len(t, availableBody.Data, 1, "草稿不进入移动可用面")
	require.Equal(t, oldAgentID, availableBody.Data[0].AgentID)

	// 「此刻」行级直证：accept 落地后旧 agent 整行与 v2 前快照逐字段相等。
	var oldAgentNow types.CustomAgent
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), oldAgentID).First(&oldAgentNow).Error)
	require.Equal(t, oldAgent, oldAgentNow, "旧本地 agent 行整行逐字段不变（此刻）")

	// ---- 各 Variant 可独立重新映射、测试和发布（accept 产生的 v2 草稿走
	// #59 既有流程：重新映射 → 测试 → 发布）----
	newAgentID := mapTestPublishUpgradeVariant(t, r, acceptBody.Data.Variant.ID, "gpt-new", "kb-new")
	require.NotEqual(t, oldAgentID, newAgentID, "升级发布实例化新的本地 agent，不覆盖旧的")

	// ---- AC1 终局断言：全流程后旧世界原封不动 ----
	found, prompt, modelID = agentsRowByName(t, r, "Sales Assistant")
	require.True(t, found)
	require.Equal(t, "Be useful.", prompt, "旧 agent 行为逐字节不变")
	require.Equal(t, "gpt-x", modelID)
	// 终局行级直证：升级 Variant 全流程发布后，旧 agent 整行与 v2 前快照
	// 逐字段相等（require.Equal 深比较，含 Config/时间戳/软删除位）。
	var oldAgentFinal types.CustomAgent
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), oldAgentID).First(&oldAgentFinal).Error)
	require.Equal(t, oldAgent, oldAgentFinal, "旧本地 agent 行整行逐字段不变（终局）")
	found, newPrompt, _ := agentsRowByName(t, r, "Sales Assistant v1.1")
	require.True(t, found)
	require.Equal(t, "Be extra useful and cite sources.", newPrompt, "新 agent 承载新版本行为")
	availableAfter := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/available-agents", "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, availableAfter.Code)
	var availableAfterBody struct {
		Data []struct {
			AgentID string `json:"agent_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(availableAfter.Body.Bytes(), &availableAfterBody))
	require.Len(t, availableAfterBody.Data, 2, "旧变体仍在可用面，新变体并行加入")
	require.Equal(t, oldAgentID, availableAfterBody.Data[0].AgentID, "旧变体保持在前（created_at 序）")

	// 旧冻结版本仍可经 #58 端点读取。
	var oldVariantAfter types.AgentAdoptionVariantEntity
	require.NoError(t, db.Where("tenant_id = ? AND id = ?", uint64(1), oldVariantID).First(&oldVariantAfter).Error)
	require.Equal(t, "published", oldVariantAfter.State)
	require.Equal(t, v1, oldVariantAfter.ReleaseID)
	oldVersionRead := adoptionCall(r, 1, http.MethodGet, "/api/v1/agents/"+oldAgentID+"/versions/"+oldVariantAfter.LocalAgentVersionID, "viewer", "viewer", nil)
	require.Equal(t, http.StatusOK, oldVersionRead.Code, oldVersionRead.Body.String())

	// ---- dismiss 路径：v3 → 新建议 → dismiss 终态保留、accepted 不可 dismiss ----
	metadataV3 := upgradeMetadataV1()
	_, v3 := freezeAndPublishUpgradeRelease(t, r, "1.2.0", metadataV3)
	afterV3 := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, afterV3.Code)
	var afterV3Body struct {
		Data []struct {
			ID    string `json:"id"`
			State string `json:"state"`
			To    string `json:"to_release_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(afterV3.Body.Bytes(), &afterV3Body))
	require.Len(t, afterV3Body.Data, 2)
	var openID string
	for _, row := range afterV3Body.Data {
		if row.State == "open" {
			openID = row.ID
			require.Equal(t, v3, row.To)
		}
	}
	require.NotEmpty(t, openID)
	dismissed := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/"+openID+"/dismiss", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, dismissed.Code, dismissed.Body.String())
	require.Contains(t, dismissed.Body.String(), `"state":"dismissed"`)
	require.Contains(t, dismissed.Body.String(), `"resolved_by":"admin"`)
	reDismiss := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/"+openID+"/dismiss", "admin", "admin", nil)
	require.Equal(t, http.StatusConflict, reDismiss.Code)
	acceptedDismiss := adoptionCall(r, 1, http.MethodPost, "/api/v1/marketplace/tenant/upgrade-proposals/"+proposal.ID+"/dismiss", "admin", "admin", nil)
	require.Equal(t, http.StatusConflict, acceptedDismiss.Code)
	final := adoptionCall(r, 1, http.MethodGet, "/api/v1/marketplace/tenant/upgrade-proposals", "admin", "admin", nil)
	require.Equal(t, http.StatusOK, final.Code)
	require.Contains(t, final.Body.String(), `"state":"dismissed"`)
	require.NotContains(t, final.Body.String(), `"state":"open"`, "已 resolved 的建议不复活")
}
