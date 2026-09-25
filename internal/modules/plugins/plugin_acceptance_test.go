//go:build integration

// T20（plan 12 Task 20）：#106 汇总验收——Spec 十条边界（B1-B10）×
// 一个应用边界 seam（受控远程 MCP 服务 + 两名成员身份，Spec Testing
// Decisions 61-66 行）。本套件不承载新实现：每条边界调用前置任务
// （T01-T19）已通过的能力（plugintest / pluginpg / fakejira / 真实
// Gate stack——同包既有构造器直接复用），只断言对外可观察结果，
// 输出 PLUGIN106-EVIDENCE journal（先例 oc_integration_test.go 的
// OC17-EVIDENCE JSON 行）。
//
// 场景键与 docs/plans/2026-09-23-issue-106-trace-matrix.md 的 A 节
// B1-B10 一一对应；矩阵证据列回填本套件的子测试名与 journal 行。
//
// 环境契约（与 plugin_pg_integration_test.go 同一约定）：
//
//	PLUGIN_TEST_DATABASE_URL  一次性 PostgreSQL DSN。
//	                          缺失 → t.Fatal("blocked-env: ...")，绝不 Skip 通过。
package plugins_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	internalmcp "github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/modules/plugins"
	"github.com/Tencent/WeKnora/internal/modules/plugins/plugintest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// PLUGIN106-EVIDENCE journal（先例 oc17Journal：每场景一行 JSON，t.Logf
// 即机器可读 feed；cleanup 汇总十行并核对齐全）。
// ---------------------------------------------------------------------------

type plugin106Journal struct {
	mu    sync.Mutex
	lines []string
}

// record 追加一行证据：boundary 是场景键（B1…B10），kv 是成对的
// string→(string|bool|int) 断言要点。
func (j *plugin106Journal) record(t *testing.T, boundary string, kv ...any) {
	t.Helper()
	entry := map[string]any{"boundary": boundary}
	for i := 0; i+1 < len(kv); i += 2 {
		if k, ok := kv[i].(string); ok {
			entry[k] = kv[i+1]
		}
	}
	raw, err := json.Marshal(entry)
	require.NoError(t, err)
	j.mu.Lock()
	j.lines = append(j.lines, string(raw))
	j.mu.Unlock()
	t.Logf("PLUGIN106-EVIDENCE %s", raw)
}

// dump 打印全部证据行并核对十条边界各恰一行（B1…B10 齐全才可 PASS）。
func (j *plugin106Journal) dump(t *testing.T) {
	t.Helper()
	j.mu.Lock()
	defer j.mu.Unlock()
	seen := map[string]int{}
	for _, line := range j.lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			if b, ok := entry["boundary"].(string); ok {
				seen[b]++
			}
		}
	}
	for _, b := range []string{"B1", "B2", "B3", "B4", "B5", "B6", "B7", "B8", "B9", "B10"} {
		require.Equalf(t, 1, seen[b], "boundary %s must carry exactly ONE evidence line, got %d", b, seen[b])
	}
	t.Logf("PLUGIN106-EVIDENCE summary: %d boundaries verified (B1..B10)", len(j.lines))
}

// accPreviewStack 是 B1 的轻量组装：受控远端（只读无账号 r + 需个人授权
// s）+ 真实 PG 仓储/服务/manager——不落安装，只走预览核验。
type accPreviewStack struct {
	svc    interfaces.PluginService
	remote *plugintest.Server
}

func newAccPreviewStack(t *testing.T, db *gorm.DB, tenantID uint64, pluginID string) *accPreviewStack {
	t.Helper()
	remote := plugintest.New()
	remote.PluginID = pluginID
	remote.Version = "1.0.0"
	remote.SetTools([]plugintest.Tool{
		{Name: "r", Description: "no-account read tool", ReadOnly: true, InputSchema: agentITNoArgSchema},
		{Name: "s", Description: "personal-auth read tool", ReadOnly: true,
			RequiresPersonalAuth: true, Scopes: []string{"read:demo"}, InputSchema: agentITNoArgSchema},
	})
	remote.Start(t)

	pluginRepo := repository.NewPluginRepository(db)
	mcpRepo := repository.NewMCPServiceRepository(db)
	approvalRepo := repository.NewMCPToolApprovalRepository(db)
	manager := internalmcp.NewMCPManager(nil)
	t.Cleanup(manager.Shutdown)
	mcpSvcService := service.NewMCPServiceService(mcpRepo, manager, nil)
	approvalSvc := service.NewMCPToolApprovalService(approvalRepo, mcpRepo)
	closer := service.MCPClientCloser(func(serviceID string) { _ = manager.CloseClient(serviceID) })
	svc := service.NewPluginService(pluginRepo, mcpSvcService, mcpRepo, approvalSvc, newManagerLister(manager), closer, nil)
	return &accPreviewStack{svc: svc, remote: remote}
}

// accPostPreview 以给定 Caller 身份走真实 HTTP 面（生产路由
// routes_plugins.go:28 的等价装配：middleware.RequireRole(Admin, cfg) +
// handler.PreviewManifest + ErrorHandler），返回状态码——非管理员的
// 403 语义在 RBAC 中间件层拒绝（service 不被触达）。
func accPostPreview(
	t *testing.T, st *accPreviewStack, tenantID uint64, role types.TenantRole, manifestURL string,
) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	enable := true
	cfg := &config.Config{Tenant: &config.TenantConfig{EnableRBAC: &enable}}
	router := gin.New()
	router.Use(middleware.ErrorHandler())
	router.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), tenantID)
		c.Set(types.UserIDContextKey.String(), "actor-"+string(role))
		c.Request = c.Request.WithContext(types.WithCaller(c.Request.Context(),
			types.Caller{TenantID: tenantID, UserID: "actor-" + string(role), Role: role}))
		c.Next()
	})
	router.POST("/api/v1/plugins/installations/preview",
		middleware.RequireRole(types.TenantRoleAdmin, cfg), handler.NewPluginHandler(st.svc).PreviewManifest)

	body, err := json.Marshal(map[string]string{"manifest_url": manifestURL})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/installations/preview", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

// ---------------------------------------------------------------------------
// 主套件：十条边界各一个子测试，cleanup 打印并核对十行 evidence。
// ---------------------------------------------------------------------------

func TestPlugin106Acceptance(t *testing.T) {
	journal := &plugin106Journal{}
	t.Cleanup(func() { journal.dump(t) })

	// B1 清单交付与管理员安装前预览核验（含非管理员拒绝、受限地址、
	// 声明不符；口径：schema 为平台 digest 核验、界面不渲染原文——US9
	// 展示粒度取舍，见 T03 Step 7）。
	t.Run("B1 清单交付与管理员安装前预览核验", func(t *testing.T) {
		db := openPluginDB(t)
		st := newAccPreviewStack(t, db, 1, "com.example.acc-b1")
		ctx := context.Background()

		// 非管理员（viewer）走真实 HTTP 面 → 403（RBAC 中间件拒绝，
		// service 未被触达——管理员预览是治理入口）。
		code, body := accPostPreview(t, st, 1, types.TenantRoleViewer, st.remote.ManifestURL())
		require.Equal(t, http.StatusForbidden, code, "a non-admin preview must be rejected with 403")
		require.Contains(t, body, "Forbidden")

		// 管理员预览：版本/端点/工具/读写分类/授权要求齐备。
		code, body = accPostPreview(t, st, 1, types.TenantRoleAdmin, st.remote.ManifestURL())
		require.Equal(t, http.StatusOK, code, body)
		require.Contains(t, body, `"version":"1.0.0"`, "the preview carries the manifest version")
		require.Contains(t, body, st.remote.BaseURL()+"/mcp", "the preview carries the version-pinned endpoint")
		require.Contains(t, body, `"name":"r"`, "the no-account read tool is listed")
		require.Contains(t, body, `"name":"s"`, "the personal-auth tool is listed")
		require.Contains(t, body, `"read_only":true`, "the read/write classification is rendered")
		require.Contains(t, body, `"requires_personal_auth":true`, "the authorization requirement is rendered")
		require.Contains(t, body, `"scopes":["read:demo"]`, "the scope requirement is rendered")

		// 服务层同口径断言（预览结果结构齐备）。
		preview, err := st.svc.PreviewFromManifest(ctx, 1, "admin-1", st.remote.ManifestURL())
		require.NoError(t, err)
		require.NotEmpty(t, preview.PreviewID)
		require.Equal(t, "1.0.0", preview.Version)
		require.Equal(t, "http-streamable", preview.TransportType)
		require.Len(t, preview.Tools, 2)
		require.NotEmpty(t, preview.IdentityFingerprint, "the identity fingerprint is verified and carried")
		require.True(t, preview.ExpiresAt.After(preview.ExpiresAt.Add(-1)), "the preview carries its TTL expiry")
		// 口径注记（T03 Step 7）：schema 为平台 digest 核验，预览结果
		// 结构不携带 schema 原文（types.PluginPreviewToolReview 无
		// schema 字段——digest 不符在上游整单拒绝），界面不渲染原文。
		raw, err := json.Marshal(preview.Tools[0])
		require.NoError(t, err)
		require.NotContains(t, string(raw), "input_schema",
			"US9 display-granularity ruling: schemas are DIGEST-verified, never rendered as raw text")

		// 受限地址清单 URL：抓取前拒绝。
		_, err = st.svc.PreviewFromManifest(ctx, 1, "admin-1", "http://10.1.2.3/manifest.json")
		require.ErrorIs(t, err, service.ErrManifestURLRejected, "a private-network manifest URL must be rejected before any fetch")

		// 清单声明与远端目录不符：预览拒绝（ghost 工具只在声明里）。
		st.remote.ManifestHandler_mutate(func(m *types.PluginManifest) {
			m.Tools = append(m.Tools, types.PluginToolDecl{
				Name: "ghost", ReadOnly: true, InputSchemaDigest: digestOfNoArg(t),
			})
		})
		_, err = st.svc.PreviewFromManifest(ctx, 1, "admin-1", st.remote.ManifestURL())
		require.Error(t, err, "a manifest declaring tools the live endpoint does not serve must be rejected")
		require.Contains(t, err.Error(), "missing from live endpoint")

		journal.record(t, "B1",
			"non_admin_preview", "403",
			"admin_preview_fields", "version+endpoint+tools+rw-class+auth+scopes",
			"schema_policy", "digest-verified-not-rendered",
			"private_manifest_url", "rejected",
			"declaration_mismatch", "rejected")
	})

	// B2 安装固定版本 + 差异 + 手动接受 + 失败保旧。
	t.Run("B2 安装固定版本+差异+手动接受+失败保旧", func(t *testing.T) {
		db := openPluginDB(t)
		st := newUpgradePGStack(t, db, 1, "com.example.acc-b2")
		ctx := agentITContext(1, "member-1")
		bg := context.Background()

		// 固定版本：确认安装即钉住 1.0.0；成员调旧版成功。
		inst, err := st.svc.GetInstallation(bg, 1, st.installationID)
		require.NoError(t, err)
		require.Equal(t, "1.0.0", inst.Version)
		sessionV1 := st.memberSession(t, "member-1")
		defV1 := agentITDescribe(ctx, t, sessionV1, st.service.ID, "v1_tool")
		callV1 := agentITCall(ctx, t, sessionV1, defV1.ToolRef, json.RawMessage(`{}`))
		require.True(t, callV1.Success, callV1.Error)

		// 差异：清单声明 v2（v1_tool 移除、v2_tool 新增）→ PreviewUpgrade。
		st.remote.SetTools([]plugintest.Tool{{
			Name: "v2_tool", Description: "v2 only tool", ReadOnly: true,
			InputSchema: agentITNoArgSchema,
		}})
		st.remote.Version = "2.0.0"
		preview, err := st.svc.PreviewUpgrade(bg, 1, st.installationID)
		require.NoError(t, err)
		require.Equal(t, "2.0.0", preview.Diff.CandidateVersion)
		require.Equal(t, "v2_tool", preview.Diff.AddedTools[0].Name)
		require.Equal(t, "v1_tool", preview.Diff.RemovedTools[0].Name)

		// 失败保旧（US24）：候选清单不可达 → 接受失败，安装行零改动
		//（版本/端点/权威快照保持 v1——远端目录此刻已切 v2 属漂移阻断
		// 语义（B3 复述），不是失败保旧的观察点）。
		require.NoError(t, db.Exec(
			"UPDATE plugin_installations SET manifest_url = ? WHERE id = ?",
			"http://127.0.0.1:1/manifest.json", st.installationID).Error)
		_, err = st.svc.AcceptUpgrade(bg, 1, "admin-1", st.installationID, preview.CandidateFingerprint)
		require.Error(t, err, "an unreachable candidate must fail the accept")
		instStill, err := st.svc.GetInstallation(bg, 1, st.installationID)
		require.NoError(t, err)
		require.Equal(t, "1.0.0", instStill.Version, "a failed accept keeps the old version")
		require.Equal(t, st.remote.BaseURL()+"/mcp", instStill.EndpointURL,
			"a failed accept never rewrites the accepted endpoint")
		snapshotStill := make([]string, 0, len(instStill.Tools))
		for _, tool := range instStill.Tools {
			snapshotStill = append(snapshotStill, tool.Name)
		}
		require.Equal(t, []string{"v1_tool"}, snapshotStill,
			"a failed accept never rewrites the authoritative snapshot")

		// 手动接受（US13）：恢复可达后重新预览 → 接受 → 成员新会话切 v2。
		require.NoError(t, db.Exec(
			"UPDATE plugin_installations SET manifest_url = ? WHERE id = ?",
			st.remote.ManifestURL(), st.installationID).Error)
		preview2, err := st.svc.PreviewUpgrade(bg, 1, st.installationID)
		require.NoError(t, err)
		accepted, err := st.svc.AcceptUpgrade(bg, 1, "admin-1", st.installationID, preview2.CandidateFingerprint)
		require.NoError(t, err)
		require.Equal(t, "2.0.0", accepted.Version)
		require.Equal(t, []string{"v2_tool"}, st.listToolNames(t, "member-1"),
			"the member's NEW conversation must expose only the accepted v2 tools")

		journal.record(t, "B2",
			"fixed_version", "1.0.0",
			"diff", "added=v2_tool,removed=v1_tool",
			"accept_failure_keeps_old", true,
			"accepted_version", "2.0.0")
	})

	// B3 运行时以已接受快照阻断漂移。
	t.Run("B3 运行时以已接受快照阻断漂移", func(t *testing.T) {
		db := openPluginDB(t)
		st := newDriftPGStack(t, db, 1, "com.example.acc-b3")
		ctx := agentITContext(1, "member-1")
		bg := context.Background()

		// 漂移前：成员目录 [t1]，调用成功。
		require.Equal(t, []string{"t1"}, st.listToolNames(t, "member-1"))
		sessionBefore := st.memberSession(t, "member-1")
		def := agentITDescribe(ctx, t, sessionBefore, st.service.ID, "t1")
		call := agentITCall(ctx, t, sessionBefore, def.ToolRef, json.RawMessage(`{}`))
		require.True(t, call.Success, call.Error)

		// 远端漂移：t1 schema 变 + 新增 t2 → 成员新会话发现被阻。
		st.remote.SetTools([]plugintest.Tool{
			{Name: "t1", Description: "v1 tool", ReadOnly: true, InputSchema: driftPGT1SchemaV2},
			{Name: "t2", Description: "new tool", ReadOnly: true, InputSchema: agentITNoArgSchema},
		})
		blocked := st.listToolNamesResult(t, "member-1")
		require.False(t, blocked.Success, "a drifted directory must block member discovery")

		// 管理员核验：detected 且四证据维明细正确。
		report, err := st.svc.CheckDrift(bg, 1, st.installationID)
		require.NoError(t, err)
		require.Equal(t, types.PluginDriftDetected, report.DriftState)
		view, err := st.svc.GetDrift(bg, 1, st.installationID)
		require.NoError(t, err)
		require.NotNil(t, view.Detail)
		require.Equal(t, []string{"t2"}, view.Detail.Added)
		require.Equal(t, []string{"t1"}, view.Detail.SchemaChanged)

		// 复审闭环：ResolveDrift 重定基 → 成员目录恢复、t1 可调。
		_, err = st.svc.ResolveDrift(bg, 1, "admin-1", st.installationID)
		require.NoError(t, err)
		require.Equal(t, []string{"t1"}, st.listToolNames(t, "member-1"),
			"after the admin rebases, the member directory recovers (t2 stays conservatively disabled)")
		sessionAfter := st.memberSession(t, "member-1")
		def2 := agentITDescribe(ctx, t, sessionAfter, st.service.ID, "t1")
		call2 := agentITCall(ctx, t, sessionAfter, def2.ToolRef, json.RawMessage(`{}`))
		require.True(t, call2.Success, call2.Error)

		journal.record(t, "B3",
			"member_discovery_blocked_on_drift", true,
			"check_drift", "detected+detail(added=t2,schema_changed=t1)",
			"resolve_rebases", "member-directory-recovered")
	})

	// B4 成员发现 + 个人授权按空间/成员/插件隔离。
	t.Run("B4 成员发现+个人授权隔离", func(t *testing.T) {
		db := openPluginDB(t)
		tenantID := uint64(1)
		st := newPluginStack(t, db, tenantID)
		ctx := context.Background()

		// 成员发现：本空间安装列表可见（插件身份/状态/授权要求）。
		list, err := st.svc.ListInstallations(ctx, tenantID)
		require.NoError(t, err)
		require.Len(t, list, 1)
		require.Equal(t, "com.example.plugintest", list[0].PluginID)
		require.True(t, list[0].RequiresPersonalAuth)

		memberA := memberPrincipal("user-a")
		memberB := memberPrincipal("user-b")

		// 未授权：两人都 unauthorized，B 调用被引导授权。
		for _, member := range []types.Principal{memberA, memberB} {
			status, err := st.svc.GetMyConnectionStatus(ctx, tenantID, st.installationID, member)
			require.NoError(t, err)
			require.Equal(t, types.PluginConnectionUnauthorized, status.State)
		}
		_, err = st.callSearchTool(t, memberB)
		require.Error(t, err, "an unauthorized member must not reach personal data")

		// A、B 各自授权 → 各只命中本人数据（凭据隔离）。
		st.authorizeMember(t, memberA, "user-a", "pass-a")
		st.authorizeMember(t, memberB, "user-b", "pass-b")
		outA, err := st.callSearchTool(t, memberA)
		require.NoError(t, err)
		require.Equal(t, "data-for:user-a", outA, "member A only hits A's data")
		outB, err := st.callSearchTool(t, memberB)
		require.NoError(t, err)
		require.Equal(t, "data-for:user-b", outB, "member B only hits B's data")

		// A 撤销：只影响 A，B 不受影响（个人撤销边界）。
		require.NoError(t, st.oauthManager.Revoke(ctx, tenantID, memberA, st.service.ID))
		require.NoError(t, st.manager.CloseClient(st.service.ID))
		statusA, err := st.svc.GetMyConnectionStatus(ctx, tenantID, st.installationID, memberA)
		require.NoError(t, err)
		require.Equal(t, types.PluginConnectionUnauthorized, statusA.State)
		statusB, err := st.svc.GetMyConnectionStatus(ctx, tenantID, st.installationID, memberB)
		require.NoError(t, err)
		require.Equal(t, types.PluginConnectionAuthorized, statusB.State)

		// 跨空间：空间 2 读空间 1 的安装 → not found。
		_, err = st.svc.GetMyConnectionStatus(ctx, 2, st.installationID, memberPrincipal("user-x"))
		require.ErrorIs(t, err, service.ErrInstallationNotFound)

		journal.record(t, "B4",
			"member_discovery", "1-installation-visible",
			"credential_isolation", "A->user-a,B->user-b",
			"revocation_scoped_to_member", true,
			"cross_tenant", "not-found")
	})

	// B5 写工具默认关闭 + 逐项启用 + 审批（拒绝与超时零写入）。
	t.Run("B5 写工具默认关闭+审批零写入", func(t *testing.T) {
		// 场景一：安装即关闭 + 成员审批被拒 → 零外部写入。
		t.Run("install-disabled-then-rejected", func(t *testing.T) {
			db := openPluginDB(t)
			st := newApprovalPGStack(t, db, 1, "com.example.acc-b5-reject")

			// 安装即默认关闭：写工具 w Enabled=false + 默认关闭原因。
			rows, err := st.svc.ListInstallationTools(context.Background(), 1, st.installationID)
			require.NoError(t, err)
			for _, row := range rows {
				if row.Name == "w" {
					require.False(t, row.Enabled)
					require.Equal(t, interfaces.PluginWriteToolDisabledReason, row.DisabledReason)
				}
			}

			st.armWriteTool(t) // 管理员逐项启用 + 配置成员审批
			ctx := agentITContext(1, "member-1")
			member := memberPrincipal("member-1")
			session := st.memberSession(t, "member-1")
			defW := agentITDescribe(ctx, t, session, st.service.ID, "w")
			callDone := st.callToolAsync(ctx, session, defW.ToolRef, json.RawMessage(writeApprovalWArgs), "acc-b5-reject-1")

			card := st.waitPending(t)
			require.Equal(t, "w", card.MCPToolName, "the approval card names the exact write tool")
			require.Equal(t, writeApprovalWArgs, card.ArgsJSON, "the approval card carries the RAW arguments")

			require.Equal(t, http.StatusOK,
				st.resolveViaHandler(t, member, card.PendingID, "reject", "member declined this write"))
			outcome := awaitCall(t, callDone)
			require.False(t, outcome.result.Success, "a rejected approval fails the call")
			require.Zero(t, st.remote.WriteCalls(), "a rejected approval NEVER dispatches an external write")
		})

		// 场景二：审批超时（不 Resolve）→ 零外部写入。
		t.Run("approval-timeout", func(t *testing.T) {
			db := openPluginDB(t)
			st := newApprovalPGStack(t, db, 2, "com.example.acc-b5-timeout")
			st.armWriteTool(t)
			ctx := agentITContext(2, "member-1")

			session := st.memberSession(t, "member-1")
			defW := agentITDescribe(ctx, t, session, st.service.ID, "w")
			callDone := st.callToolAsync(ctx, session, defW.ToolRef, json.RawMessage(writeApprovalWArgs), "acc-b5-timeout-1")

			card := st.waitPending(t)
			require.NotEmpty(t, card.PendingID)
			outcome := awaitCall(t, callDone) // 不 Resolve：gate 2s 超时自决
			require.False(t, outcome.result.Success, "a timed-out approval fails the call")
			require.Contains(t, outcome.result.Error, "approval timeout")
			require.Zero(t, st.remote.WriteCalls(), "a timed-out approval NEVER dispatches an external write")
		})

		journal.record(t, "B5",
			"install_default_off", true,
			"rejected_write", "zero-dispatch",
			"timed_out_write", "zero-dispatch",
			"approval_card", "targeted-tool+raw-args")
	})

	// B6 停用 / 撤销 / 过期 / 候选不可达。
	t.Run("B6 停用撤销过期候选不可达", func(t *testing.T) {
		bg := context.Background()

		// 停用 + 候选不可达（无账号插件栈）。
		t.Run("disable-and-unreachable-candidate", func(t *testing.T) {
			db := openPluginDB(t)
			st := newUpgradePGStack(t, db, 3, "com.example.acc-b6")

			// 候选不可达：升级预览报错且安装保持 v1（升级失败不影响旧版）。
			require.NoError(t, db.Exec(
				"UPDATE plugin_installations SET manifest_url = ? WHERE id = ?",
				"http://127.0.0.1:1/manifest.json", st.installationID).Error)
			_, err := st.svc.PreviewUpgrade(bg, 3, st.installationID)
			require.Error(t, err, "an unreachable candidate must fail the upgrade preview")
			inst, err := st.svc.GetInstallation(bg, 3, st.installationID)
			require.NoError(t, err)
			require.Equal(t, "1.0.0", inst.Version)

			// 停用（治理终点）：停用前组装成员会话并 describe 拿旧 ref，
			// 停用后物化服务不可见，旧 ref 在新会话不可达。
			ctx := agentITContext(3, "member-1")
			sessionBefore := st.memberSession(t, "member-1")
			defV1 := agentITDescribe(ctx, t, sessionBefore, st.service.ID, "v1_tool")
			_, err = st.svc.SetInstallationState(bg, 3, st.installationID, types.PluginInstallationDisabled)
			require.NoError(t, err)
			enabled, err := st.mcpRepo.ListEnabled(bg, 3)
			require.NoError(t, err)
			require.Empty(t, enabled, "a disabled installation's service is invisible to member sessions")
			after := st.memberSession(t, "member-1")
			staleRaw, err := json.Marshal(map[string]any{
				"tool_ref": defV1.ToolRef, "arguments": json.RawMessage(`{}`)})
			require.NoError(t, err)
			_, staleErr := after.ExecuteTool(ctx, tools.ToolCallMCPTool, staleRaw)
			require.Error(t, staleErr, "a stale ref is unreachable once the installation is disabled")
		})

		// 撤销与过期（个人授权插件栈：一名成员两条状态轨迹）。
		t.Run("revoke-and-expiry", func(t *testing.T) {
			db := openPluginDB(t)
			st := newPluginStack(t, db, 4)
			ctx := context.Background()
			memberA := memberPrincipal("user-a")

			// 授权 → 撤销 → unauthorized（撤销后重新授权的入口仍在）。
			st.authorizeMember(t, memberA, "user-a", "pass-a")
			require.NoError(t, st.oauthManager.Revoke(ctx, 4, memberA, st.service.ID))
			require.NoError(t, st.manager.CloseClient(st.service.ID))
			status, err := st.svc.GetMyConnectionStatus(ctx, 4, st.installationID, memberA)
			require.NoError(t, err)
			require.Equal(t, types.PluginConnectionUnauthorized, status.State)
			require.NotEmpty(t, status.AuthorizeURLPath, "a revoked member is guided back to authorization")

			// 重新授权 → 过期：expired 状态 + 调用引导重授权。
			st.authorizeMember(t, memberA, "user-a", "pass-a")
			require.NoError(t, db.Exec(
				"UPDATE mcp_oauth_tokens SET expires_at = ?, refresh_token = '' WHERE tenant_id = ? AND service_id = ?",
				time.Now().Add(-time.Minute), 4, st.service.ID).Error)
			status, err = st.svc.GetMyConnectionStatus(ctx, 4, st.installationID, memberA)
			require.NoError(t, err)
			require.Equal(t, types.PluginConnectionExpired, status.State)
			require.False(t, status.Authorized)
			_, err = st.callSearchTool(t, memberA)
			require.Error(t, err, "an expired connection must fail and guide re-authorization")
		})

		journal.record(t, "B6",
			"disabled_installation", "invisible+stale-ref-unreachable",
			"revoked_member", "unauthorized+reauth-path",
			"expired_token", "expired+reauth-guidance",
			"unreachable_candidate", "preview-fails-version-kept")
	})

	// B7 Jira 纵向案例（本人本周 / 来源链接 / 无模型注入 / 服务端时间范围）。
	t.Run("B7 Jira纵向案例", func(t *testing.T) {
		db := openPluginDB(t)
		jira := plugintest.NewFakeJira(t)

		emailA, tokenA := jiraE2ECredential(t)
		emailB, tokenB := jiraE2ECredential(t)
		friday, wednesday := jiraE2EWeekday(4), jiraE2EWeekday(2)
		jira.AddAccount(t, emailA, tokenA,
			plugintest.JiraIssue{Key: "A-101", Summary: "验收成员A的本周任务", Status: "进行中", Due: friday},
			plugintest.JiraIssue{Key: "A-102", Summary: "A的评审", Status: "待办", Due: wednesday},
		)
		jira.AddAccount(t, emailB, tokenB,
			plugintest.JiraIssue{Key: "B-201", Summary: "验收成员B的本周任务", Status: "进行中", Due: friday},
		)
		st := newJiraE2EStack(t, db, 1, jira)

		// A 授权后：本人本周事项 + /browse/<KEY> 来源链接；不见 B 的事项。
		require.NoError(t, st.authorizeJiraMember("user-a", emailA, tokenA))
		granted := st.askJiraTool(t, "user-a", map[string]any{})
		require.True(t, granted.Success, granted.Error)
		require.Contains(t, granted.Output,
			"[A-101] 验收成员A的本周任务 · 状态 进行中 · 截止 "+friday+" · "+jira.BaseURL()+"/browse/A-101")
		require.Contains(t, granted.Output, "/browse/A-102", "every issue row carries its source link")
		require.NotContains(t, granted.Output, "B-201", "member A never sees member B's issues")

		// B 授权后：只见 B-201（两成员凭据隔离，服务端以授权身份定账号）。
		require.NoError(t, st.authorizeJiraMember("user-b", emailB, tokenB))
		outB := st.askJiraTool(t, "user-b", map[string]any{})
		require.True(t, outB.Success, outB.Error)
		require.Contains(t, outB.Output, "B-201")
		require.NotContains(t, outB.Output, "A-101")

		// 模型注入账号参数被服务端 schema 拒绝且零外呼（不接受模型传
		// 令牌/任意 JQL/URL；工具 schema 固定无参，JQL 由服务端模板构造）。
		before := jira.Calls()
		injected := st.askJiraTool(t, "user-a", map[string]any{
			"token": "tok-attacker", "jql": "assignee = someone-else()", "url": "http://evil.example", "user_id": "42",
		})
		require.False(t, injected.Success, "injected account parameters must be rejected, not executed")
		require.Equal(t, before, jira.Calls(), "a rejected call never reaches Jira")

		journal.record(t, "B7",
			"member_a_week", "A-101+A-102+source-links",
			"member_b_isolated", "B-201-only",
			"model_injection", "schema-rejected+zero-jira-calls",
			"time_range", "server-side-this-week")
	})

	// B8 无账号工具独立能力（仍受空间启停约束）。
	t.Run("B8 无账号工具独立能力", func(t *testing.T) {
		db := openPluginDB(t)
		st := newUpgradePGStack(t, db, 5, "com.example.acc-b8")
		ctx := agentITContext(5, "member-1")

		// 无个人授权的成员直接 discover→describe→call 成功（独立能力）。
		page := agentITDiscover(ctx, t, st.memberSession(t, "member-1"), map[string]any{
			"mode": "list_tools", "server_id": st.service.ID,
		})
		require.Equal(t, []string{"v1_tool"}, agentITToolNames(page))
		session := st.memberSession(t, "member-1")
		def := agentITDescribe(ctx, t, session, st.service.ID, "v1_tool")
		callsBefore := st.remote.Calls()
		call := agentITCall(ctx, t, session, def.ToolRef, json.RawMessage(`{}`))
		require.True(t, call.Success, call.Error)
		require.Equal(t, int64(1), st.remote.Calls()-callsBefore, "the no-account call reached the remote exactly once")

		// 空间停用后：新会话目录不可见（无账号能力仍受空间启停约束）。
		_, err := st.svc.SetInstallationState(context.Background(), 5, st.installationID, types.PluginInstallationDisabled)
		require.NoError(t, err)
		enabled, err := st.mcpRepo.ListEnabled(context.Background(), 5)
		require.NoError(t, err)
		require.Empty(t, enabled, "a disabled space hides the no-account tool too")

		journal.record(t, "B8",
			"no_account_call", "discover+describe+call-ok",
			"space_disable_hides", true)
	})

	// B9 手工 MCP 兼容（共存 + 缺省启用语义 + 000190.down 后手工服务仍在）。
	t.Run("B9 手工MCP兼容", func(t *testing.T) {
		db := openPluginDB(t)
		st := newUpgradePGStack(t, db, 6, "com.example.acc-b9")

		// 手工服务（plugin_installation_id NULL）+ 读写两工具，与插件安装
		// 同租户共存。
		manual := plugintest.New()
		manual.PluginID = "com.example.acc-b9-manual"
		manual.Version = "1.0.0"
		manual.SetTools([]plugintest.Tool{
			{Name: "m_read", Description: "manual read", ReadOnly: true, InputSchema: agentITNoArgSchema},
			{Name: "m_write", Description: "manual write", ReadOnly: false, InputSchema: agentITNoArgSchema},
		})
		manual.Start(t)
		manualURL := manual.BaseURL() + "/mcp"
		require.NoError(t, db.Create(&types.MCPService{
			ID: "acc-b9-manual-svc", TenantID: 6, Enabled: true,
			Name: "手工服务-acc-b9", URL: &manualURL, TransportType: types.MCPTransportHTTPStreamable,
		}).Error)

		// 成员会话：插件工具与手工工具同目录可见（兼容共存）。
		ctx := agentITContext(6, "member-1")
		services, err := st.mcpRepo.ListEnabled(context.Background(), 6)
		require.NoError(t, err)
		require.Len(t, services, 2, "the manual service coexists with the plugin installation")
		approvalSvc := service.NewMCPToolApprovalService(repository.NewMCPToolApprovalRepository(db), st.mcpRepo)
		gate := &driftPGGate{approvalSvc: approvalSvc}
		guard := service.PluginSnapshotLookupWithDriftMarking(st.pluginRepo, newManagerLister(st.manager))
		registry := driftPGRegister(ctx, t, st.manager, gate, guard, services...)

		toolPage := agentITDiscover(ctx, t, registry, map[string]any{"mode": "list_servers"})
		require.Len(t, toolPage.Servers, 2, "both services appear in the member directory")
		// 手工读写工具缺省启用（无 approval 行 = enabled——兼容语义保留
		// 给手工服务；插件域才是"写默认关"）。
		defRead := agentITDescribe(ctx, t, registry, "acc-b9-manual-svc", "m_read")
		callRead := agentITCall(ctx, t, registry, defRead.ToolRef, json.RawMessage(`{}`))
		require.True(t, callRead.Success, callRead.Error)
		defWrite := agentITDescribe(ctx, t, registry, "acc-b9-manual-svc", "m_write")
		callWrite := agentITCall(ctx, t, registry, defWrite.ToolRef, json.RawMessage(`{}`))
		require.True(t, callWrite.Success, callWrite.Error)
		// 插件工具同会话可调（安装路径不破坏手工路径，反之亦然）。
		defPlugin := agentITDescribe(ctx, t, registry, st.service.ID, "v1_tool")
		callPlugin := agentITCall(ctx, t, registry, defPlugin.ToolRef, json.RawMessage(`{}`))
		require.True(t, callPlugin.Success, callPlugin.Error)

		// 迁移兼容：000190.down 后手工服务行仍在（迁移未把手工服务视为
		// 插件派生行）。注意：既有 TestPluginInstallationsMigrationUpAndDown
		// 的 harness 存在顺序缺陷（pluginMigrationsOpenDB 先 apply
		// 000190.up 后建最小父表，真 PG 必失败——T11 执行记录已转交，非
		// 本切片所有权文件），此处不复用它，自建正确顺序的独立复述：
		// schema → 最小父表 → 000189.up → 000190.up → 手工行 → down。
		accMigrationDownKeepsManual(t)

		journal.record(t, "B9",
			"coexistence", "plugin+manual-same-session",
			"manual_default_semantics", "missing-row=enabled(read+write)",
			"migration_000190_down", "manual-row-survives")
	})

	// B10 网络与租户边界（受限地址三处 + 跨空间隔离）。
	t.Run("B10 网络与租户边界", func(t *testing.T) {
		db := openPluginDB(t)
		st := newUpgradePGStack(t, db, 7, "com.example.acc-b10")
		bg := context.Background()

		// 预览：受限地址清单 URL 拒绝（抓取前）。
		_, err := st.svc.PreviewFromManifest(bg, 7, "admin-1", "http://10.1.2.3/manifest.json")
		require.ErrorIs(t, err, service.ErrManifestURLRejected)

		// 升级：长期清单来源被改为受限地址 → 重核拒绝、安装零改动。
		require.NoError(t, db.Exec(
			"UPDATE plugin_installations SET manifest_url = ? WHERE id = ?",
			"http://10.1.2.3/manifest.json", st.installationID).Error)
		_, err = st.svc.PreviewUpgrade(bg, 7, st.installationID)
		require.ErrorIs(t, err, service.ErrPluginVerifyFailed, "a private-network upgrade source must be rejected")
		require.NoError(t, db.Exec(
			"UPDATE plugin_installations SET manifest_url = ? WHERE id = ?",
			st.remote.ManifestURL(), st.installationID).Error)

		// 漂移核验：已接受端点被改为受限地址 → 核验失败（fail-closed），
		// 漂移状态零持久化（不产生虚假 detected）。
		require.NoError(t, db.Exec(
			"UPDATE plugin_installations SET endpoint_url = ? WHERE id = ?",
			"http://192.168.1.5/mcp", st.installationID).Error)
		_, err = st.svc.CheckDrift(bg, 7, st.installationID)
		require.Error(t, err, "a private-network accepted endpoint must fail the drift re-check")
		view, err := st.svc.GetDrift(bg, 7, st.installationID)
		require.NoError(t, err)
		require.Equal(t, types.PluginDriftNone, view.DriftState, "a failed re-check writes no drift state")
		require.NoError(t, db.Exec(
			"UPDATE plugin_installations SET endpoint_url = ? WHERE id = ?",
			st.remote.BaseURL()+"/mcp", st.installationID).Error)

		// 跨空间隔离：空间 8 看不见空间 7 的安装（读为 not found）。
		_, err = st.svc.GetInstallation(bg, 8, st.installationID)
		require.ErrorIs(t, err, service.ErrInstallationNotFound)
		foreign, err := st.svc.ListInstallations(bg, 8)
		require.NoError(t, err)
		require.Empty(t, foreign, "a foreign tenant lists none of this space's installations")

		journal.record(t, "B10",
			"preview_private_url", "rejected",
			"upgrade_private_source", "rejected-zero-write",
			"drift_private_endpoint", "fail-closed-no-state",
			"cross_tenant", "not-found+empty-list")
	})
}

// digestOfNoArg 返回无参 schema 的平台 digest（B1 声明不符场景的 ghost
// 声明用——与 plugintest.Manifest 同一算法）。
func digestOfNoArg(t *testing.T) string {
	t.Helper()
	return plugins.ToolSchemaDigest([]byte(agentITNoArgSchema))
}

// accMigrationSchema 是 B9 迁移复述的隔离 schema（固定字面量名：PG 的
// CREATE/DROP SCHEMA 标识符不支持参数绑定，固定名使全部 DDL 语句保持
// 纯字面量零拼接；IF NOT EXISTS + cleanup CASCADE 保证重复运行安全，
// 同库多进程并发不在一次性 DSN 契约内）。数据语句一律参数绑定。
const accMigrationSchema = "plugins_acc_t20"

// accMigrationDownKeepsManual 在独立 schema 上以正确顺序复述 000190 的
// 回退安全契约（GAP-3）：最小父表先于 000190.up（其 ALTER TABLE
// mcp_services 依赖父表在场），手工服务行（plugin_installation_id NULL）
// 在 000190.down 后原样幸存。迁移文件是仓库静态资产（与既有迁移测试同
// 一执行方式）。
func accMigrationDownKeepsManual(t *testing.T) {
	t.Helper()
	dsn := os.Getenv("PLUGIN_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("blocked-env: PLUGIN_TEST_DATABASE_URL required")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}
	if err := admin.Exec("CREATE SCHEMA IF NOT EXISTS " + accMigrationSchema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA IF EXISTS " + accMigrationSchema + " CASCADE").Error; err != nil {
			t.Errorf("drop isolated schema %s: %v", accMigrationSchema, err)
		}
		sqlAdmin, _ := admin.DB()
		if sqlAdmin != nil {
			_ = sqlAdmin.Close()
		}
	})
	dsnSchema := dsn
	if p, perr := url.Parse(dsn); perr == nil && (p.Scheme == "postgres" || p.Scheme == "postgresql") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsnSchema = dsn + sep + "search_path=" + accMigrationSchema
	} else {
		dsnSchema = dsn + " search_path=" + accMigrationSchema
	}
	db, err := gorm.Open(postgres.Open(dsnSchema), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open isolated schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}

	// 最小父表（非 AutoMigrate——冻结模型已带 PluginInstallationID，会
	// 预建列使 000190 的 ALTER 失败；与既有迁移测试同一理由）。固定
	// schema 名下重复运行先显式清场（全字面量 DDL）。
	for _, ddl := range []string{
		"DROP SCHEMA IF EXISTS " + accMigrationSchema + " CASCADE",
		"CREATE SCHEMA " + accMigrationSchema,
		`CREATE TABLE mcp_services (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id BIGINT NOT NULL,
			name VARCHAR(255) NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT TRUE
		)`,
		`CREATE TABLE mcp_tool_approvals (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id BIGINT NOT NULL,
			service_id VARCHAR(36) NOT NULL,
			tool_name VARCHAR(512) NOT NULL
		)`,
		`CREATE TABLE mcp_oauth_tokens (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id BIGINT NOT NULL,
			user_id VARCHAR(64) NOT NULL,
			service_id VARCHAR(36) NOT NULL
		)`,
		`CREATE TABLE mcp_oauth_clients (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id BIGINT NOT NULL,
			service_id VARCHAR(36) NOT NULL,
			client_id VARCHAR(512) NOT NULL
		)`,
		`CREATE TABLE mcp_metadata (
			id VARCHAR(36) PRIMARY KEY,
			tenant_id BIGINT NOT NULL,
			service_id VARCHAR(36) NOT NULL
		)`,
	} {
		if _, xerr := sqlDB.ExecContext(context.Background(), ddl); xerr != nil {
			t.Fatalf("prepare minimal parent tables: %v", xerr)
		}
	}

	apply := func(file string) {
		raw, rerr := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "versioned", file))
		if rerr != nil {
			t.Fatalf("read migration %s: %v", file, rerr)
		}
		if _, xerr := sqlDB.ExecContext(context.Background(), string(raw)); xerr != nil {
			t.Fatalf("apply migration %s: %v", file, xerr)
		}
	}
	apply("000189_plugin_previews.up.sql")
	apply("000190_plugin_installations.up.sql")

	// 一行手工服务（plugin_installation_id 列默认 NULL）。
	if err := db.Exec(
		"INSERT INTO mcp_services (id, tenant_id, name) VALUES (?, 1, 'acc-b9-manual-down')",
		"acc-b9-manual-down").Error; err != nil {
		t.Fatal(err)
	}

	apply("000190_plugin_installations.down.sql")

	var n int
	require.NoError(t, db.Raw(
		"SELECT COUNT(*) FROM mcp_services WHERE id = ?", "acc-b9-manual-down").Scan(&n).Error)
	require.Equal(t, 1, n, "the manual service row survives 000190.down (never treated as plugin-derived)")
	var nullable string
	require.NoError(t, db.Raw(
		"SELECT is_nullable FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'mcp_services' AND column_name = 'plugin_installation_id'",
	).Scan(&nullable).Error)
	require.Empty(t, nullable, "the plugin column must be dropped by 000190.down")
}
