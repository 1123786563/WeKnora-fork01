//go:build integration

// T13（plan 06 Task 13）：Jira 纵向案例端到端集成测试——fake Jira（按
// 凭据分账本）+ Jira 形替身（复刻 T04 契约，工具经真实 HTTP 调 fake
// Jira）+ 真实安装（PreviewFromManifest → ConfirmInstallation 物化）→
// 成员个人 OAuth 授权 → 对话调用（call_mcp_tool 全链：快照守卫 + 审批
// 链 + per-principal token）。
//
// 环境契约与 T11 相同：PLUGIN_TEST_DATABASE_URL 缺失 → blocked-env Fatal。
//
// 包名遵守总索引「测试包名约定」：import plugintest 的测试一律外部测试包
// plugins_test。本文件仅供测试使用。
package plugins_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	internalmcp "github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/modules/plugins/plugintest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// 夹具：随机 fake 凭据、本周日期、真 PG + Jira 形替身 stack
// ---------------------------------------------------------------------------

// jiraE2ECredential 随机生成一名成员的 fake Jira 凭据（邮箱 + API token，
// "tok-"+随机十六进制）。凭据仅对本次测试的 fake Jira 账本有意义，每次
// 运行随机生成——源码不留任何可用凭据字面量（全局约束 + Brief）。
func jiraE2ECredential(t *testing.T) (email, apiToken string) {
	t.Helper()
	buf := make([]byte, 16)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	suffix := hex.EncodeToString(buf)
	return "member-" + suffix + "@example.test", "tok-" + suffix
}

// jiraE2EWeekday 返回本周（以本地时区周一为一周之始）周一偏移 offset 天
// 的日期串（YYYY-MM-DD）。fake Jira 的固定 JQL 语义：due 落在
// [本周一, 下周一) 内才算本周事项——测试数据据此落在本周内。
func jiraE2EWeekday(offset int) string {
	now := time.Now()
	wd := int(now.Weekday())
	if wd == 0 {
		wd = 7 // Go 的 Sunday=0 归一为 7，使周一恒为偏移 0
	}
	monday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, -(wd - 1))
	return monday.AddDate(0, 0, offset).Format("2006-01-02")
}

// jiraE2EStack 是 T13 的应用边界 stack：真实 PG（openPluginDB）+ fake
// Jira + Jira 形替身 + 真实 repo/service/manager/oauthManager + 已确认
// 安装（物化服务）。对话调用面按「成员的一轮对话请求」即时注册
// （newConversationRegistry）：生产每个成员请求各持自己的 MCP 目录
// （mcp_catalog.go authorize 按注册时的 principal 绑定）。
type jiraE2EStack struct {
	tenantID       uint64
	svc            interfaces.PluginService
	manager        *internalmcp.MCPManager
	oauthManager   *internalmcp.OAuthManager
	pluginRepo     interfaces.PluginRepository
	gate           *agentITGate
	service        *types.MCPService // 安装物化的服务行
	installationID string
	remote         *plugintest.Server
	jira           *plugintest.FakeJira
}

// newJiraE2EStack 组装全链并落一笔确认安装：管理员粘贴替身清单 URL →
// PreviewFromManifest → ConfirmInstallation（与 T11 newPluginStack 同构，
// 远端换成 Jira 形替身；对话调用面接真实的 RegisterMCPTools）。
func newJiraE2EStack(
	t *testing.T, db *gorm.DB, tenantID uint64, jira *plugintest.FakeJira, opts ...plugintest.JiraTodoOption,
) *jiraE2EStack {
	t.Helper()

	remote := plugintest.NewJiraTodoPlugin(t, jira, opts...)
	remote.Start(t)

	pluginRepo := repository.NewPluginRepository(db)
	mcpRepo := repository.NewMCPServiceRepository(db)
	approvalRepo := repository.NewMCPToolApprovalRepository(db)
	oauthRepo := repository.NewMCPOAuthRepository(db)
	manager := internalmcp.NewMCPManager(oauthRepo)
	t.Cleanup(manager.Shutdown)
	mcpSvcService := service.NewMCPServiceService(mcpRepo, manager, oauthRepo)
	approvalSvc := service.NewMCPToolApprovalService(approvalRepo, mcpRepo)

	// Lister over the real manager（与 T11 同构：核验用一次性 nonce 客户端）。
	lister := func(ctx context.Context, transportType, endpointURL string) ([]*types.MCPTool, error) {
		return jiraE2EListTools(ctx, manager, transportType, endpointURL)
	}
	closer := service.MCPClientCloser(func(serviceID string) { _ = manager.CloseClient(serviceID) })
	svc := service.NewPluginService(pluginRepo, mcpSvcService, mcpRepo, approvalSvc, lister, closer, oauthRepo)
	oauthManager := internalmcp.NewOAuthManager(oauthRepo, mcpRepo, nil)

	ctx := context.Background()
	preview, err := svc.PreviewFromManifest(ctx, tenantID, "admin-1", remote.ManifestURL())
	require.NoError(t, err)
	result, err := svc.ConfirmInstallation(ctx, tenantID, "admin-1", preview.PreviewID)
	require.NoError(t, err)
	require.NotEmpty(t, result.ServiceID)
	serviceRow, err := mcpRepo.GetByID(ctx, tenantID, result.ServiceID)
	require.NoError(t, err)
	require.NotNil(t, serviceRow)

	return &jiraE2EStack{
		tenantID:       tenantID,
		svc:            svc,
		manager:        manager,
		oauthManager:   oauthManager,
		pluginRepo:     pluginRepo,
		gate:           &agentITGate{},
		service:        serviceRow,
		installationID: result.InstallationID,
		remote:         remote,
		jira:           jira,
	}
}

// newConversationRegistry 为成员的一轮对话请求注册对话调用面：生产快照
// 守卫 PluginSnapshotLookup（按租户反查安装快照）+ 自动批准审批链（T10
// agentITGate——插件工具调用仍必须走 approval 链，GAP-5）。注册 ctx 携带
// 成员身份，与生产每请求各持目录的形态一致。
func (st *jiraE2EStack) newConversationRegistry(t *testing.T, principalID string) *tools.ToolRegistry {
	t.Helper()
	regCtx := agentITContext(st.tenantID, principalID)
	guard := service.PluginSnapshotLookup(st.pluginRepo)
	registry := tools.NewToolRegistry()
	_, err := tools.RegisterMCPTools(
		regCtx, registry, []*types.MCPService{st.service}, st.manager, st.gate, 0, nil, nil, guard,
	)
	require.NoError(t, err)
	return registry
}

// jiraE2EListTools 是核验 EndpointLister 的本地 seam（与 T11 同构：internal
// /modules 不得互相 import 容器装配）。
func jiraE2EListTools(
	ctx context.Context, manager *internalmcp.MCPManager, transportType, endpointURL string,
) ([]*types.MCPTool, error) {
	var nonce [4]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("generate verification nonce: %w", err)
	}
	sum := sha256.Sum256([]byte(endpointURL))
	serviceID := "plugin-verify-" + hex.EncodeToString(sum[:8]) + "-" + hex.EncodeToString(nonce[:])
	verify := &types.MCPService{
		ID: serviceID, TenantID: 0, Name: "plugin-verify", Enabled: true,
		TransportType: types.MCPTransportType(transportType), URL: &endpointURL,
	}
	client, err := manager.GetOrCreateClient(ctx, verify)
	if err != nil {
		_ = manager.CloseClient(verify.ID)
		return nil, err
	}
	defer func() {
		_ = client.Disconnect()
		_ = manager.CloseClient(verify.ID)
	}()
	return client.ListTools(ctx)
}

// authorizeJiraMember 驱动真实成员 OAuth 流（与 T11 authorizeMember 同构，
// 凭据换成成员的 Jira 凭据：POST username=email、password=apiToken——替身
// 经 fake Jira /myself 真实验证后才发一次性 code）。返回 error 而非内部
// require：供并发测试在 goroutine 中安全调用（t.FailNow 只能在测试
// goroutine 里执行）。
func (st *jiraE2EStack) authorizeJiraMember(principalID, email, apiToken string) error {
	principal := memberPrincipal(principalID)
	ctx := context.Background()
	authURL, _, err := st.oauthManager.StartAuthorization(
		ctx, st.service, st.tenantID, principal, "http://127.0.0.1:1/cb", "/")
	if err != nil {
		return fmt.Errorf("start authorization: %w", err)
	}

	u, err := url.Parse(authURL)
	if err != nil {
		return fmt.Errorf("parse authorize url: %w", err)
	}
	state := u.Query().Get("state")
	if state == "" {
		return fmt.Errorf("authorize url carries no state")
	}
	if u.Query().Get("code_challenge") == "" {
		return fmt.Errorf("PKCE code_challenge required")
	}

	noRedirect := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	page, err := noRedirect.Get(u.String())
	if err != nil {
		return fmt.Errorf("get authorize page: %w", err)
	}
	if page.StatusCode != http.StatusOK {
		return fmt.Errorf("authorize page must render, got HTTP %d", page.StatusCode)
	}

	form := url.Values{"state": {state}, "username": {email}, "password": {apiToken}}
	resp, err := noRedirect.PostForm(u.String(), form)
	if err != nil {
		return fmt.Errorf("submit credentials: %w", err)
	}
	if resp.StatusCode != http.StatusFound {
		return fmt.Errorf("member consent must 302 back with a code, got HTTP %d", resp.StatusCode)
	}
	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		return fmt.Errorf("parse consent redirect: %w", err)
	}
	code := location.Query().Get("code")
	if code == "" {
		return fmt.Errorf("successful authorization must issue a code")
	}
	if got := location.Query().Get("state"); got != state {
		return fmt.Errorf("state must round-trip verbatim, got %q", got)
	}

	if _, _, err := st.oauthManager.CompleteAuthorization(ctx, state, code); err != nil {
		return fmt.Errorf("complete authorization: %w", err)
	}
	return nil
}

// askJiraTool 是成员在对话中问待办的完整动作：发现目录 → describe 拿
// tool_ref → call_mcp_tool（arguments 传给远端工具）。任一步失败即原样
// 返回该步的对话可见 ToolResult——未授权成员的对话内引导正来自第一步
// （目录加载的 OAuth authorization required 错误，mcp_tool.go
// loadPluginDirectory → OAuthReauthorizationRequiredError）。
func (st *jiraE2EStack) askJiraTool(t *testing.T, principalID string, arguments map[string]any) *types.ToolResult {
	t.Helper()
	// OAuth 插件服务的目录加载要求调用 ctx 携带 ToolExecContext
	//（mcp_tool.go loadPluginDirectory：成员在对话中执行工具的常态）。
	ctx := agentITCallCtx(agentITContext(st.tenantID, principalID))
	registry := st.newConversationRegistry(t, principalID)

	page := agentITDiscoverResult(ctx, t, registry, map[string]any{
		"mode": "list_tools", "server_id": st.service.ID,
	})
	if !page.Success {
		return page
	}
	var toolPage agentITPage
	require.NoError(t, json.Unmarshal([]byte(page.Output), &toolPage))
	require.Equal(t, []string{plugintest.JiraTodoToolName}, agentITToolNames(toolPage),
		"the installed plugin must expose exactly the T04-contract tool")

	describe := agentITDiscoverResult(ctx, t, registry, map[string]any{
		"mode": "describe", "server_id": st.service.ID, "tool_name": plugintest.JiraTodoToolName,
	})
	if !describe.Success {
		return describe
	}
	var definition agentITDefinition
	require.NoError(t, json.Unmarshal([]byte(describe.Output), &definition))
	require.NotEmpty(t, definition.ToolRef)

	raw, err := json.Marshal(arguments)
	require.NoError(t, err)
	return agentITCall(ctx, t, registry, definition.ToolRef, json.RawMessage(raw))
}

// ---------------------------------------------------------------------------
// 场景 1：纵向主链——未授权对话内引导 → 授权 → 重试成功；A/B 数据隔离；
// 撤销后再引导。
// ---------------------------------------------------------------------------

func TestJiraTodoVerticalEndToEnd(t *testing.T) {
	db := openPluginDB(t)
	tenantID := uint64(1)
	jira := plugintest.NewFakeJira(t)

	emailA, tokenA := jiraE2ECredential(t)
	emailB, tokenB := jiraE2ECredential(t)
	friday, wednesday := jiraE2EWeekday(4), jiraE2EWeekday(2)
	jira.AddAccount(t, emailA, tokenA,
		plugintest.JiraIssue{Key: "A-101", Summary: "成员A的本周任务", Status: "进行中", Due: friday},
		plugintest.JiraIssue{Key: "A-102", Summary: "A的代码评审", Status: "待办", Due: wednesday},
	)
	jira.AddAccount(t, emailB, tokenB,
		plugintest.JiraIssue{Key: "B-201", Summary: "成员B的本周任务", Status: "进行中", Due: friday},
	)
	st := newJiraE2EStack(t, db, tenantID, jira)

	// 1) 成员 A 未授权直接问待办：对话内授权引导——工具目录加载时无 token
	//    → OAuthRequired → catalog 折叠为 needs_auth 状态文案（mcp_catalog.go
	//    snapshot：retry discovery after resolving its authentication）；同时
	//    连接状态 unauthorized + 授权入口路径（T11 面板引导）。
	denied := st.askJiraTool(t, "user-a", map[string]any{})
	require.False(t, denied.Success, "an unauthorized member must not receive issue data")
	require.Contains(t, denied.Error, "needs_auth",
		"the in-conversation guidance must point the member at authorizing")
	require.Contains(t, denied.Error, "authentication")
	status, err := st.svc.GetMyConnectionStatus(context.Background(), tenantID, st.installationID, memberPrincipal("user-a"))
	require.NoError(t, err)
	require.Equal(t, types.PluginConnectionUnauthorized, status.State)
	require.Equal(t, "/api/v1/mcp-services/"+st.service.ID+"/oauth/authorize-url", status.AuthorizeURLPath)

	// 2) A 完成真实 OAuth（提交 Jira 凭据，替身经 fake Jira /myself 验证）
	//    → 对话内重试 → 成功且只含本人本周事项与 /browse/<KEY> 来源链接。
	require.NoError(t, st.authorizeJiraMember("user-a", emailA, tokenA))
	granted := st.askJiraTool(t, "user-a", map[string]any{})
	require.True(t, granted.Success, granted.Error)
	// 行格式与示例服务 formatIssues 逐段一致：[KEY] 标题 · 状态 X · 截止 Y · URL
	require.Contains(t, granted.Output,
		"[A-101] 成员A的本周任务 · 状态 进行中 · 截止 "+friday+" · "+jira.BaseURL()+"/browse/A-101")
	require.Contains(t, granted.Output,
		"[A-102] A的代码评审 · 状态 待办 · 截止 "+wednesday+" · "+jira.BaseURL()+"/browse/A-102")
	require.NotContains(t, granted.Output, "B-201", "member A must never see member B's issues")
	require.Contains(t, granted.Output, fmt.Sprintf("[MCP tool result from %q", st.service.Name),
		"the conversation result must carry the untrusted-source prefix")

	// 3) 成员 B 授权后：只见 B-201；A 的结果不受影响（两成员数据隔离）。
	require.NoError(t, st.authorizeJiraMember("user-b", emailB, tokenB))
	outB := st.askJiraTool(t, "user-b", map[string]any{})
	require.True(t, outB.Success, outB.Error)
	require.Contains(t, outB.Output,
		"[B-201] 成员B的本周任务 · 状态 进行中 · 截止 "+friday+" · "+jira.BaseURL()+"/browse/B-201")
	require.NotContains(t, outB.Output, "A-101", "member B must never see member A's issues")
	require.NotContains(t, outB.Output, "A-102")
	outA := st.askJiraTool(t, "user-a", map[string]any{})
	require.True(t, outA.Success, outA.Error)
	require.NotContains(t, outA.Output, "B-201")

	// 4) A 撤销授权（Revoke + 连接回收）→ 再问 → 再次进入对话内授权引导。
	require.NoError(t, st.oauthManager.Revoke(context.Background(), tenantID, memberPrincipal("user-a"), st.service.ID))
	require.NoError(t, st.manager.CloseClient(st.service.ID))
	afterRevoke := st.askJiraTool(t, "user-a", map[string]any{})
	require.False(t, afterRevoke.Success)
	require.Contains(t, afterRevoke.Error, "needs_auth",
		"revocation must re-enter the in-conversation authorization guidance")
	require.NotContains(t, afterRevoke.Error, "A-101", "no issue data may leak after revocation")
	// B 不受 A 撤销影响。
	outB2 := st.askJiraTool(t, "user-b", map[string]any{})
	require.True(t, outB2.Success, outB2.Error)
	require.Contains(t, outB2.Output, "B-201")
}

// ---------------------------------------------------------------------------
// 场景 2：403 / 超时 / token 失效三态如实报错，结果不含事项行（不虚构）。
// ---------------------------------------------------------------------------

func TestJiraFailureModesDoNotFabricate(t *testing.T) {
	// 各子测试独立组装（注入互不干扰）。
	newAuthorized := func(t *testing.T, inject func(jira *plugintest.FakeJira, email string), opts ...plugintest.JiraTodoOption) (*jiraE2EStack, string) {
		db := openPluginDB(t)
		jira := plugintest.NewFakeJira(t)
		email, apiToken := jiraE2ECredential(t)
		jira.AddAccount(t, email, apiToken,
			plugintest.JiraIssue{Key: "A-101", Summary: "成员A的本周任务", Status: "进行中", Due: jiraE2EWeekday(4)},
		)
		if inject != nil {
			inject(jira, email)
		}
		st := newJiraE2EStack(t, db, 1, jira, opts...)
		require.NoError(t, st.authorizeJiraMember("user-a", email, apiToken))
		return st, email
	}

	t.Run("jira-403", func(t *testing.T) {
		st, _ := newAuthorized(t, func(jira *plugintest.FakeJira, email string) {
			jira.DenySearch(email, http.StatusForbidden)
		})
		result := st.askJiraTool(t, "user-a", map[string]any{})
		require.False(t, result.Success, "a Jira 403 must surface as a failure, never an empty success")
		require.Contains(t, result.Error, "403")
		require.NotContains(t, result.Error, "A-101", "a failure must not fabricate issue rows")
	})

	t.Run("jira-timeout", func(t *testing.T) {
		// 替身工具 HTTP 客户端 500ms 超时；fake Jira search 延迟 3s。
		st, _ := newAuthorized(t, nil,
			plugintest.WithJiraToolTimeout(500*time.Millisecond),
		)
		st.jira.SetSearchDelay(3 * time.Second)
		start := time.Now()
		result := st.askJiraTool(t, "user-a", map[string]any{})
		require.False(t, result.Success, "a Jira timeout must surface as a failure, never an empty success")
		require.Contains(t, result.Error, "timeout")
		require.NotContains(t, result.Error, "A-101")
		require.Less(t, time.Since(start), 2*time.Second,
			"the stand-in's configurable client timeout must cut the slow upstream short")
	})

	t.Run("jira-token-invalid", func(t *testing.T) {
		// 授权完成后 Jira 侧凭据失效（如 API token 被吊销）→ 401 如实报错。
		st, email := newAuthorized(t, nil)
		st.jira.InvalidateAccount(email)
		result := st.askJiraTool(t, "user-a", map[string]any{})
		require.False(t, result.Success, "an invalidated Jira credential must surface as a failure")
		require.Contains(t, result.Error, "401")
		require.NotContains(t, result.Error, "A-101")
	})
}

// ---------------------------------------------------------------------------
// 场景 3：空周是空成功（不虚构条目，也不报错）。
// ---------------------------------------------------------------------------

func TestJiraEmptyWeekIsEmpty(t *testing.T) {
	db := openPluginDB(t)
	jira := plugintest.NewFakeJira(t)
	email, apiToken := jiraE2ECredential(t)
	jira.AddAccount(t, email, apiToken) // 本周无任何事项
	st := newJiraE2EStack(t, db, 1, jira)
	require.NoError(t, st.authorizeJiraMember("user-a", email, apiToken))

	result := st.askJiraTool(t, "user-a", map[string]any{})
	require.True(t, result.Success, result.Error, "an empty week is a legal success, not an error")
	require.Contains(t, result.Output, "(no text output)",
		"the empty result must be presented as no output, not a fabricated list")
	require.NotContains(t, result.Output, "[A-", "no fabricated issue rows")
	// 空成功来自真实的空响应（search 确实到达过 fake Jira），而非没发请求。
	require.GreaterOrEqual(t, st.jira.Calls(), int64(1),
		"emptiness must come from a real empty response")
}

// ---------------------------------------------------------------------------
// 场景 4：模型注入账号参数（token/jql/url/user_id）被服务端 schema 拒绝，
// 且该次拒绝调用不发起任何 Jira 请求（fake 计数增量 0）。
// ---------------------------------------------------------------------------

func TestModelCannotInjectAccountParameters(t *testing.T) {
	db := openPluginDB(t)
	jira := plugintest.NewFakeJira(t)
	email, apiToken := jiraE2ECredential(t)
	jira.AddAccount(t, email, apiToken,
		plugintest.JiraIssue{Key: "A-101", Summary: "成员A的本周任务", Status: "进行中", Due: jiraE2EWeekday(4)},
	)
	st := newJiraE2EStack(t, db, 1, jira)
	require.NoError(t, st.authorizeJiraMember("user-a", email, apiToken))
	before := jira.Calls()

	result := st.askJiraTool(t, "user-a", map[string]any{
		"token":   "tok-attacker-injected",
		"jql":     "assignee = someone-else()",
		"url":     "http://evil.example",
		"user_id": "42",
	})
	// 拒绝形态二者取一：JSON-RPC 错误（err 面 → Execute 包装 Error）或工具
	// 执行错误面（SEP-1303）；但绝不能是成功结果。
	require.False(t, result.Success,
		"extraneous arguments must be rejected by additionalProperties:false, not executed")
	require.NotContains(t, result.Error, "A-101", "a rejected call must not leak issue data")
	require.NotContains(t, result.Output, "A-101")
	// 该次调用不得发起任何 Jira 请求。
	require.Equal(t, before, jira.Calls(),
		"a schema-rejected call must not reach Jira at all")
}

// ---------------------------------------------------------------------------
// OCR 修复轮（T13-OCR1-F4）：慢凭据验证不得串行化替身的其他 OAuth/鉴权链。
// ---------------------------------------------------------------------------

// TestJiraSlowCredentialCheckDoesNotBlockOtherMembers 钉住替身 OAuth 的锁
// 纪律：submitCredentials 的凭据验证（credentialCheck——对 fake Jira
// /myself 的真实 HTTP 出站）必须在 oauthStub.mu 之外执行，否则一次慢验证
// 会把 lookupMember（每个 /mcp 请求与 tools/call 鉴权）、startAuthorization
// （GET /authorize）、exchangeCode/refresh（/token）、sessionCredential
// （每次工具调用）全部串行化。场景：A 的 /myself 延迟 1.5s 期间，B 的完整
// 授权（GET /authorize → POST 凭据 → /token 交换）必须远早于 A 的验证
// 完成——B 授权耗时 < 1s 即为不阻塞（持锁实现下 B 至少要等 A 剩余的
// ~1.3s）。
func TestJiraSlowCredentialCheckDoesNotBlockOtherMembers(t *testing.T) {
	db := openPluginDB(t)
	jira := plugintest.NewFakeJira(t)
	emailA, tokenA := jiraE2ECredential(t)
	emailB, tokenB := jiraE2ECredential(t)
	jira.AddAccount(t, emailA, tokenA,
		plugintest.JiraIssue{Key: "A-101", Summary: "成员A的本周任务", Status: "进行中", Due: jiraE2EWeekday(4)})
	jira.AddAccount(t, emailB, tokenB,
		plugintest.JiraIssue{Key: "B-201", Summary: "成员B的本周任务", Status: "进行中", Due: jiraE2EWeekday(4)})
	jira.SetMyselfDelay(emailA, 1500*time.Millisecond) // A 的凭据验证慢
	st := newJiraE2EStack(t, db, 1, jira)

	// A 在后台发起授权：POST /authorize 触发 credentialCheck → 慢 /myself。
	errA := make(chan error, 1)
	go func() { errA <- st.authorizeJiraMember("user-a", emailA, tokenA) }()
	// 等 A 的慢验证已经占住出站（若实现持锁，此刻 o.mu 被 A 占着）。
	time.Sleep(200 * time.Millisecond)

	// B 的完整授权必须不被 A 的慢验证阻塞。
	bStart := time.Now()
	require.NoError(t, st.authorizeJiraMember("user-b", emailB, tokenB),
		"member B's authorization must complete while A's slow verification is in flight")
	require.Less(t, time.Since(bStart), time.Second,
		"a slow credential check must not serialize other members' OAuth traffic (lock held across egress)")

	// A 的授权最终也成功（慢但正确）。
	require.NoError(t, <-errA)

	// 两成员数据隔离不受并发影响：各自能查到本人事项。
	outA := st.askJiraTool(t, "user-a", map[string]any{})
	require.True(t, outA.Success, outA.Error)
	require.Contains(t, outA.Output, "A-101")
	require.NotContains(t, outA.Output, "B-201")
}
