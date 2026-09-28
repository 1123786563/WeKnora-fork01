//go:build integration

// T19（plan 11 Task 19 Step 1）：真 PostgreSQL 应用边界集成测试——写工具
// 成员审批闭环的真实验收：真实 approval.Gate（Adapter 直读 PG 策略行，
// container.go:470-473 生产装配同款）+ MCPTool.Execute 的
// NeedsApproval→RequestAndWait 链 + handler.ResolveToolApproval 的 HTTP
// 语义直调（gin 路由 + 身份注入，等效 POST /agent/tool-approvals/:pending_id）。
//
// 验收四条件（Brief）：
//  1. 批准 → 调用成功且 WriteCalls()==1；同 pending 二次 Resolve 被拒
//     （一次性派发）且计数仍 1；
//  2. 拒绝 / 超时（ToolApprovalTimeoutSeconds=2 不 Resolve）/ 停用工具 /
//     停用插件 → 零派发零写入；
//  3. 审批卡 PendingRequest 断言 ServiceName / MCPToolName / Args（参数
//     原文）/ ToolCallID 非空（确定操作目标，无「批准整个未来操作」语义）；
//  4. 未配置审批的只读工具无 pending 噪声。
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
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/approval"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	internalmcp "github.com/Tencent/WeKnora/internal/modules/airesource/mcp"
	"github.com/Tencent/WeKnora/internal/modules/plugins/plugintest"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 写工具 w 带必填参数 title：审批卡的「参数原文」断言需要非空输入。
const (
	writeApprovalWSchema = `{"type":"object","properties":{"title":{"type":"string"}},"required":["title"],"additionalProperties":false}`
	writeApprovalWArgs   = `{"title":"T19-external-write"}`
	writeApprovalTimeout = 2 * time.Second
)

// approvalPGStack：plugintest 单远端（只读 r + 写工具 w）+ 真实 PG 仓储/服务/
// manager + 已确认 v1 安装 + 真实 approval.Gate（container.go:470-473 生产
// 装配同款：NewGate(cfg, &approval.Adapter{Svc: approvalService}, nil)——
// rdb nil 单实例，集成测试单进程正合适）。pending 事件经同步 EventBus
// 捕获进 channel——RequestAndWait 的 Emit 在 waiter 注册之后同步发生，
// handler 只记录不 Resolve（在 Emit 内 Resolve 虽不死锁，但先记录后断言
// 才能核对审批卡原文，二次 Resolve 断言也需要 handler 已返回）。
type approvalPGStack struct {
	tenantID       uint64
	svc            interfaces.PluginService
	pluginRepo     interfaces.PluginRepository
	mcpRepo        interfaces.MCPServiceRepository
	approvalSvc    interfaces.MCPToolApprovalService
	manager        *internalmcp.MCPManager
	remote         *plugintest.Server
	installationID string
	service        *types.MCPService
	gate           *approval.Gate
	bus            *event.EventBus
	pendingCh      chan event.ToolApprovalRequiredData
}

// newApprovalPGStack assembles the stack and lands ONE confirmed v1
// installation with the no-account read tool r and the WRITE tool w (title
// parameter). The write tool stays policy-disabled until the test arms it
// (admin enabled+require_approval) — the T18 default-off semantics.
func newApprovalPGStack(t *testing.T, db *gorm.DB, tenantID uint64, pluginID string) *approvalPGStack {
	t.Helper()

	remote := plugintest.New()
	remote.PluginID = pluginID
	remote.Version = "1.0.0"
	remote.SetTools([]plugintest.Tool{
		{Name: "r", Description: "v1 read tool", ReadOnly: true, InputSchema: agentITNoArgSchema},
		{Name: "w", Description: "v1 write tool", ReadOnly: false, InputSchema: writeApprovalWSchema,
			Call: func(member string) (string, error) {
				return "written:" + member, nil
			}},
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

	ctx := context.Background()
	preview, err := svc.PreviewFromManifest(ctx, tenantID, "admin-1", remote.ManifestURL())
	require.NoError(t, err)
	confirmed, err := svc.ConfirmInstallation(ctx, tenantID, "admin-1", preview.PreviewID)
	require.NoError(t, err)
	require.NotEmpty(t, confirmed.ServiceID)
	serviceRow, err := mcpRepo.GetByID(ctx, tenantID, confirmed.ServiceID)
	require.NoError(t, err)
	require.NotNil(t, serviceRow)

	bus := event.NewEventBus()
	pendingCh := make(chan event.ToolApprovalRequiredData, 8)
	bus.On(event.EventToolApprovalRequired, func(_ context.Context, e event.Event) error {
		if data, ok := e.Data.(event.ToolApprovalRequiredData); ok {
			pendingCh <- data
		}
		return nil
	})

	// 真实 Gate：审批策略行直读 PG（生产装配同款 Adapter）。超时 2s 让
	// 超时场景的等待窗口可承受，批准/拒绝场景 Resolve 先到不受影响。
	gate := approval.NewGate(
		&config.Config{Agent: &config.AgentConfig{ToolApprovalTimeoutSeconds: 2}},
		&approval.Adapter{Svc: approvalSvc}, nil)

	return &approvalPGStack{
		tenantID:       tenantID,
		svc:            svc,
		pluginRepo:     pluginRepo,
		mcpRepo:        mcpRepo,
		approvalSvc:    approvalSvc,
		manager:        manager,
		remote:         remote,
		installationID: confirmed.InstallationID,
		service:        serviceRow,
		gate:           gate,
		bus:            bus,
		pendingCh:      pendingCh,
	}
}

// armWriteTool 是管理员的 T19 治理动作：启用写工具并配置调用时成员审批。
func (st *approvalPGStack) armWriteTool(t *testing.T) {
	t.Helper()
	enabled, requireApproval := true, true
	rows, err := st.svc.SetInstallationToolPolicy(
		context.Background(), st.tenantID, st.installationID, "w", &enabled, &requireApproval)
	require.NoError(t, err)
	for _, row := range rows {
		if row.Name == "w" {
			require.True(t, row.Enabled)
			require.True(t, row.RequireApproval)
		}
	}
}

// memberSession assembles ONE member conversation's tool surface over the
// REAL gate（approval.Gate 实现 approval.MCPApproval）与生产形态运行守卫。
func (st *approvalPGStack) memberSession(t *testing.T, memberID string) *tools.ToolRegistry {
	t.Helper()
	services, err := st.mcpRepo.ListEnabled(context.Background(), st.tenantID)
	require.NoError(t, err)
	registry := tools.NewToolRegistry()
	guard := service.PluginSnapshotLookupWithDriftMarking(st.pluginRepo, newManagerLister(st.manager))
	_, err = tools.RegisterMCPTools(
		agentITContext(st.tenantID, memberID), registry, services, st.manager, st.gate, 0, nil, nil, guard)
	require.NoError(t, err)
	return registry
}

// approvalCallOutcome 是一次异步工具调用的终态。
type approvalCallOutcome struct {
	result *types.ToolResult
	err    error
}

// callToolAsync 在 goroutine 里执行 ToolCallMCPTool：审批等待会阻塞到
// Resolve/超时，主测试线程先核对审批卡再决策。ToolExecContext 携带本
// stack 的捕获 EventBus（agentITCallCtx 每次新建 bus 的形态不能复用——
// pending 事件必须落到可断言的 bus 上）与注入的 ToolCallID。
func (st *approvalPGStack) callToolAsync(
	ctx context.Context, registry *tools.ToolRegistry, toolRef string, arguments json.RawMessage, toolCallID string,
) <-chan approvalCallOutcome {
	callCtx := tools.WithToolExecContext(ctx, &tools.ToolExecContext{
		EventBus:           st.bus,
		ToolCallID:         toolCallID,
		SessionID:          "t19-approval-session",
		AssistantMessageID: "t19-approval-message",
		RequestID:          "t19-approval-request",
		ApprovalCtx:        ctx,
	})
	done := make(chan approvalCallOutcome, 1)
	raw, err := json.Marshal(map[string]any{"tool_ref": toolRef, "arguments": arguments})
	if err != nil {
		done <- approvalCallOutcome{err: err}
		return done
	}
	go func() {
		result, err := registry.ExecuteTool(callCtx, tools.ToolCallMCPTool, raw)
		done <- approvalCallOutcome{result: result, err: err}
	}()
	return done
}

// resolveViaHandler 直调 handler.ResolveToolApproval 的 HTTP 语义（计划
// Interfaces 的既定方式：gin 路由 + 身份注入中间件直调，等效生产
// POST /agent/tool-approvals/:pending_id + Viewer 会话鉴权）。返回 HTTP
// 状态码供「同 pending 二次 Resolve 被拒」的 400 断言。
func (st *approvalPGStack) resolveViaHandler(
	t *testing.T, member types.Principal, pendingID, decision, reason string,
) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// 生产错误面（router.go:173 同款）：ResolveToolApproval 的 4xx 经
	// c.Error 挂错——没有 ErrorHandler 时状态码停在 200，HTTP 语义失真。
	router.Use(middleware.ErrorHandler())
	router.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), st.tenantID)
		c.Request = c.Request.WithContext(types.WithPrincipal(c.Request.Context(), member))
		c.Next()
	})
	h := handler.NewMCPServiceHandler(nil, nil, st.gate, nil)
	router.POST("/agent/tool-approvals/:pending_id", h.ResolveToolApproval)

	body := map[string]any{"decision": decision}
	if reason != "" {
		body["reason"] = reason
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/agent/tool-approvals/"+pendingID, strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code
}

// waitPending 等待一个审批事件到达（Emit 同步发生，正常应在调用发起后
// 立刻可读；3s 上限防御调度抖动——远短于 gate 的 2s 超时会让失败场景
// 变成超时而非挂死）。
func (st *approvalPGStack) waitPending(t *testing.T) event.ToolApprovalRequiredData {
	t.Helper()
	select {
	case data := <-st.pendingCh:
		return data
	case <-time.After(3 * time.Second):
		t.Fatal("no tool_approval_required event arrived — the call did not reach the approval gate")
		return event.ToolApprovalRequiredData{}
	}
}

// awaitCall 等待异步调用终态（上限 = gate 超时 + 执行余量）。
func awaitCall(t *testing.T, done <-chan approvalCallOutcome) approvalCallOutcome {
	t.Helper()
	select {
	case outcome := <-done:
		return outcome
	case <-time.After(writeApprovalTimeout + 8*time.Second):
		t.Fatal("the tool call never returned from the approval wait")
		return approvalCallOutcome{}
	}
}

// TestWriteApprovalApproveDispatchesOnce（Brief 条件 1+3）：管理员启用写
// 工具并配置成员审批 → 成员调用阻塞等待；审批卡携带确定目标（服务名/
// 工具名/参数原文/ToolCallID）；经 ResolveToolApproval 批准 → 调用成功、
// 受控服务 WriteCalls()==1（恰一次派发）；同 pending 二次 Resolve 被拒
// （一次性——ErrAlreadyResolved 的 HTTP 面）且写计数仍为 1。
func TestWriteApprovalApproveDispatchesOnce(t *testing.T) {
	db := openPluginDB(t)
	st := newApprovalPGStack(t, db, 1, "com.example.write-approval")
	st.armWriteTool(t)
	ctx := agentITContext(1, "member-1")
	member := memberPrincipal("member-1")

	session := st.memberSession(t, "member-1")
	defW := agentITDescribe(ctx, t, session, st.service.ID, "w")

	callDone := st.callToolAsync(ctx, session, defW.ToolRef, json.RawMessage(writeApprovalWArgs), "t19-call-approve-1")

	// 审批卡断言：确定操作目标与参数原文（Brief 条件 3）。
	card := st.waitPending(t)
	require.Equal(t, st.service.ID, card.ServiceID, "the approval card names the materialized service")
	require.Equal(t, st.service.Name, card.ServiceName, "the approval card carries the service name")
	require.NotEmpty(t, card.ServiceName)
	require.Equal(t, "w", card.MCPToolName, "the approval card names the exact MCP tool")
	require.Equal(t, writeApprovalWArgs, card.ArgsJSON, "the approval card carries the RAW argument text — approving ONE determined operation, not future ones")
	require.Equal(t, "t19-call-approve-1", card.ToolCallID, "the approval card ties back to the tool call id")
	require.NotEmpty(t, card.PendingID)

	// 批准（成员本人 = 会话 owner 的 Resolve 授权语义）。
	require.Equal(t, http.StatusOK,
		st.resolveViaHandler(t, member, card.PendingID, "approve", ""),
		"the session owner's approve resolves 200")

	outcome := awaitCall(t, callDone)
	require.NoError(t, outcome.err)
	require.NotNil(t, outcome.result)
	require.True(t, outcome.result.Success, outcome.result.Error)
	require.Equal(t, int64(1), st.remote.WriteCalls(), "exactly ONE external write after approval")

	// 同 pending 二次 Resolve：一次性派发（ADR-0014 pending 一次性终结）。
	// awaitCall 已保证 RequestAndWait 返回——defer 移除了 waiter，二次
	// Resolve 确定性落 ErrPendingNotFound 的 404「not found or already
	// completed」面（若在删除前竞态窗口内则为 400 already-resolved；两者
	// 都是拒绝，这里断言确定性时序的 404）。
	require.Equal(t, http.StatusNotFound,
		st.resolveViaHandler(t, member, card.PendingID, "approve", ""),
		"a second Resolve of the SAME pending must be rejected (one-shot dispatch)")
	require.Equal(t, int64(1), st.remote.WriteCalls(), "the rejected double-resolve must not add external writes")
}

// TestWriteApprovalRejectTimeoutDisableZeroWrites（Brief 条件 2）：拒绝 /
// 超时 / 停用三场景零派发零写入。三场景各自全新安装态（互不污染策略行）。
func TestWriteApprovalRejectTimeoutDisableZeroWrites(t *testing.T) {
	t.Run("reject", func(t *testing.T) {
		db := openPluginDB(t)
		st := newApprovalPGStack(t, db, 2, "com.example.write-approval-reject")
		st.armWriteTool(t)
		ctx := agentITContext(2, "member-1")
		member := memberPrincipal("member-1")

		session := st.memberSession(t, "member-1")
		defW := agentITDescribe(ctx, t, session, st.service.ID, "w")
		callDone := st.callToolAsync(ctx, session, defW.ToolRef, json.RawMessage(writeApprovalWArgs), "t19-call-reject-1")

		card := st.waitPending(t)
		require.Equal(t, http.StatusOK,
			st.resolveViaHandler(t, member, card.PendingID, "reject", "member declined this write"))

		outcome := awaitCall(t, callDone)
		require.NoError(t, outcome.err)
		require.NotNil(t, outcome.result)
		require.False(t, outcome.result.Success, "a rejected approval must fail the call")
		require.Contains(t, outcome.result.Error, "member declined this write",
			"the rejection reason surfaces to the conversation")
		require.Zero(t, st.remote.WriteCalls(), "a rejected approval never dispatches")
	})

	t.Run("timeout", func(t *testing.T) {
		db := openPluginDB(t)
		st := newApprovalPGStack(t, db, 3, "com.example.write-approval-timeout")
		st.armWriteTool(t)
		ctx := agentITContext(3, "member-1")

		session := st.memberSession(t, "member-1")
		defW := agentITDescribe(ctx, t, session, st.service.ID, "w")
		callDone := st.callToolAsync(ctx, session, defW.ToolRef, json.RawMessage(writeApprovalWArgs), "t19-call-timeout-1")

		// 不 Resolve：gate 的 2s 超时决策必须自己落零派发。
		card := st.waitPending(t)
		require.NotEmpty(t, card.PendingID)
		started := time.Now()
		outcome := awaitCall(t, callDone)
		require.GreaterOrEqual(t, time.Since(started), writeApprovalTimeout,
			"the call actually waited for the configured approval timeout")
		require.NoError(t, outcome.err)
		require.NotNil(t, outcome.result)
		require.False(t, outcome.result.Success, "a timed-out approval must fail the call")
		require.Contains(t, outcome.result.Error, "approval timeout", "the timeout decision surfaces to the conversation")
		require.Zero(t, st.remote.WriteCalls(), "a timed-out approval never dispatches")
	})

	t.Run("disable-tool-then-plugin", func(t *testing.T) {
		db := openPluginDB(t)
		st := newApprovalPGStack(t, db, 4, "com.example.write-approval-disable")
		st.armWriteTool(t)
		ctx := agentITContext(4, "member-1")
		bg := context.Background()

		session := st.memberSession(t, "member-1")
		defW := agentITDescribe(ctx, t, session, st.service.ID, "w")

		// 停用工具：调用时策略重查直接拒绝——无审批卡、零派发。
		disable := false
		_, err := st.svc.SetInstallationToolPolicy(bg, st.tenantID, st.installationID, "w", &disable, nil)
		require.NoError(t, err)
		callDone := st.callToolAsync(ctx, session, defW.ToolRef, json.RawMessage(writeApprovalWArgs), "t19-call-disabled-1")
		outcome := awaitCall(t, callDone)
		require.NoError(t, outcome.err)
		require.NotNil(t, outcome.result)
		require.False(t, outcome.result.Success, "a disabled tool must not run")
		// 文案口径：tool_ref 解析层先于 MCPTool.Execute 拒绝（"no longer
		// available or enabled; rediscover..."）——两者都含 "enabled"，与
		// T18 describe 拒绝断言同口径。
		require.Contains(t, outcome.result.Error, "enabled")
		require.Zero(t, len(st.pendingCh), "no approval noise — the disabled tool never reaches the approval gate")
		require.Zero(t, st.remote.WriteCalls(), "a disabled tool never dispatches")

		// 停用插件（治理终点）：新成员会话的目录整体不可见——服务不再
		// Enabled（ListEnabled 不返回），旧 ref 在新会话不可达，零派发。
		_, err = st.svc.SetInstallationState(bg, st.tenantID, st.installationID, types.PluginInstallationDisabled)
		require.NoError(t, err)
		enabledServices, err := st.mcpRepo.ListEnabled(bg, st.tenantID)
		require.NoError(t, err)
		require.Empty(t, enabledServices, "a disabled installation's service is invisible to new member sessions")
		afterDisable := st.memberSession(t, "member-1")
		staleRaw, err := json.Marshal(map[string]any{
			"tool_ref": defW.ToolRef, "arguments": json.RawMessage(writeApprovalWArgs)})
		require.NoError(t, err)
		_, staleErr := afterDisable.ExecuteTool(ctx, tools.ToolCallMCPTool, staleRaw)
		require.Error(t, staleErr, "the stale ref is unreachable in a fresh session — the disabled installation never dispatches")
		require.Zero(t, st.remote.WriteCalls(), "a disabled installation never dispatches")
	})
}

// TestReadOnlyToolsDoNotPromptApproval（Brief 条件 4）：未配置审批的只读
// 工具调用不产生任何 pending（无审批噪声），直接成功。
func TestReadOnlyToolsDoNotPromptApproval(t *testing.T) {
	db := openPluginDB(t)
	st := newApprovalPGStack(t, db, 5, "com.example.write-approval-readonly")
	st.armWriteTool(t) // w 审批开启——与 r 无关：噪声回归必须在「审批已配置」的安装态下观察
	ctx := agentITContext(5, "member-1")

	session := st.memberSession(t, "member-1")
	defR := agentITDescribe(ctx, t, session, st.service.ID, "r")
	callDone := st.callToolAsync(ctx, session, defR.ToolRef, json.RawMessage(`{}`), "t19-call-readonly-1")

	outcome := awaitCall(t, callDone)
	require.NoError(t, outcome.err)
	require.NotNil(t, outcome.result)
	require.True(t, outcome.result.Success, outcome.result.Error)
	require.Zero(t, len(st.pendingCh),
		"a read tool without approval configured must not produce a pending request (no approval noise)")
	require.Zero(t, st.remote.WriteCalls(), "a read call is never a write dispatch")
}
