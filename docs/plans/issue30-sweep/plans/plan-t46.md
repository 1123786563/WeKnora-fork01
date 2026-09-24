# T16：Task Material——Artifact、引用、预览与分享（Issue #46）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 移动端通过 Task Material 深模块 Interface 列出并打开一个 Task 的材料（Artifact/Files/Diff/测试报告/Evidence 引用/只读 Terminal），经短时效签名授权下载与系统分享，并使「版本不可原地覆盖、签名 URL 不长期缓存、不支持/大文件/grant 过期/终端输入尝试正确处理」全部在该最高稳定 Interface 上可验证。

**Architecture:** 后端已具备不可变版本（`internal/application/repository/artifact_version.go:63-65`，Insert 永不 upsert）与 15 分钟 HMAC 签名授权（`internal/modules/workbench/artifact_signing.go:33` + `internal/handler/session/workbench_artifacts.go:123-193`），本计划只补三块 Go 缺口：①run 事件日志上的**只读终端日志分页端点**（`tool.terminal` 事件稀疏子序列，复用 owner 谓词）；②workbench 材料列表的**真实版本身份**（替换硬编码 `"1"`）；③列表信封声明终端可用性。客户端按 module-seams §7 新建 `packages/mobile-core/src/material/` 深模块：`open({lease}) → TaskMaterialHandle`（`index/open/act/subscribe/close`），MIME/大小预览判定、unified diff 解析、签名 URL 铸造纪律（每次新鲜铸造、origin 钉住部署、版本键 blob 缓存）、grant 过期映射与终端输入拒绝全部藏在模块后；`packages/api-client` 新增 `createMobileMaterialRemote`（走 `MobileRuntime.authorizedRequest` 授权通道，不新建传输）；apps/mobile 新增材料屏、`/tasks/materials` 路由、composition 接线与 opt-in 真实 HTTP 集成证据。

**Tech Stack:** Go 1.26（gin + gorm + testify，`go test`）、TypeScript（`packages/mobile-core`、`packages/api-client`、`apps/mobile` Expo RN）、node:test + tsx（TS 测试运行器，与 `task-office.test.ts` 一致）。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置 `pnpm install` 已就绪。本计划作者已实跑以下基线，全部绿色：`go test ./internal/modules/workbench/ -count=1` ok；`go test ./internal/handler/session/ -run 'TestListWorkbenchArtifacts|TestCreateWorkbenchArtifactSignedURL|TestDownloadWorkbenchArtifactGrant' -count=1` ok；`pnpm exec tsx --test packages/mobile-core/src/task-office/task-office.test.ts` 8 pass；`pnpm exec tsx --test packages/api-client/src/mobile/task-office.test.ts` 9 pass；`pnpm --filter @weknora/mobile test` 72 pass；`pnpm --filter @weknora/mobile typecheck` 无输出（通过）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-46.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（User Stories 18/19/26/30/31/32/33、Implementation Decisions、Testing Decisions——尤其「Task Material owns evidence, Artifact versions, Files, Diff, tests, read-only Terminal, download, share and annotation.」与「Task Material Interface tests cover immutable versions, citation provenance, grant expiry, supported/unsupported previews, Diff, terminal read-only behavior, download and system share.」）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§7 Task Material Module——所有权/Interface/seam；§3 依赖方向；§10 App Shell 禁止事项；§13 Interface 测试面：「Task Material：版本固定、grant 过期、下载/分享、Diff 与只读 Terminal」）
- ADR：`docs/adr/0004-task-is-session.md`（taskId = sessionId）、`docs/adr/0006-mobile-transport-by-semantics.md`（REST 取 Snapshot、推送仅同步提示）、`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`（「移动业务逻辑放在深 Module 后，App 只负责组装与呈现」）
- 领域术语：`CONTEXT.md`（「任务产物（Task Artifact）」：不可变版本化结果……**不能原地覆盖已存在或已审批的版本**；「任务时间线（Task Timeline）」；「证据（Evidence）」相关条目）
- Parent：Issue #30；Blocked by：#35（T05，已合并——`TaskHandle`/`authorizedEventStream`/`/tasks/detail` 路由均在当前 HEAD 亲眼核实）
- 前序批次产出（本计划 Consumes，全部在当前 HEAD 亲眼核实）：#34 的 `MobileRuntime.authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown>` 与 `MobileRuntimePorts.authorizedTransport?: (deploymentOrigin: string) => AuthorizedTransport`（`packages/mobile-core/src/runtime/types.ts:31-37/:55`、`ports.ts:89-109`）；#35 的 `/tasks/detail` 路由与 `TaskDetailScreen`（`apps/mobile/src/app/tasks/detail.tsx`、`apps/mobile/src/screens/TaskDetailScreen.tsx`）；#32 的 `RuntimeScopeLease`/`leaseActive`/`leaseScopeOf`（`packages/mobile-core/src/runtime/scope-lease.ts`，包内可见不入公共导出）；#33 的 shelf 模块结构范式（`packages/mobile-core/src/shelf/resource-shelf.ts`——`open({lease})` 句柄 + guard + 代次/迟到拒绝）；`apps/mobile/src/composition.ts:100-144` 的 `taskOfficeFor`/`activeTaskOffice` 记忆化工厂范式与 `apps/mobile/src/task-detail-integration-smoke.ts` 的 opt-in 集成证据范式（`disallowedDeploymentHost` 主机防线自 `runtime-integration-smoke.ts:118` 导入）。

## Global Constraints

以下为批准 Spec / ADR / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「Task Material owns evidence, Artifact versions, Files, Diff, tests, read-only Terminal, download, share and annotation.」（mobile-ai-office-design.md · Implementation Decisions）——本计划交付 evidence/版本/Files/Diff/测试报告/只读 Terminal/下载/分享；**annotation（批注）与基于版本请求修改属 #47**（本 Issue 的下游阻塞 Issue，验收标准为「批注/修改生成新版本，已审批版本保持不变」），本计划在 `MaterialIntent` 边界明确不含 annotate。
- 「Lead Agent Version, Artifact versions, Action Plans and candidate code commits are immutable approval anchors. Changes invalidate prior approvals.」（同上）
- 「REST submits commands and loads authoritative Snapshots. Cursored SSE carries durable Task/Run events. WebSocket or WebRTC is reserved for real-time voice. Push is a synchronization hint.」（同上；ADR-0006 同义）——材料读走授权 REST（`authorizedRequest`），终端日志是事件日志的只读分页投影，**不新增任何 WebSocket**。
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）
- 「Cache identity includes Deployment, user, Tenant, Task and Run where applicable. Scope changes invalidate subscriptions and reject late responses.」（同上）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（同上 · Testing Decisions）
- 「Task Material Interface tests cover immutable versions, citation provenance, grant expiry, supported/unsupported previews, Diff, terminal read-only behavior, download and system share.」（同上）
- 「True external dependencies such as APNs, FCM, WebRTC, system audio and system share use mock or scripted Adapters at the Port and real-device acceptance separately.」（同上）
- 「Screen 不调用 start、lookup、snapshot、events、interaction、command 等多个 wire 方法。Module 内部决定顺序、幂等、重连、revision 和错误呈现。」（mobile-module-seams.md §5.2，同义适用于 §7）
- 「禁止：Screen 直接导入 packages/contracts 或 packages/api-client；Screen 自己维护 request_id、cursor、revision、scope generation；每个 Screen 建独立 query cache 或 token refresh」（mobile-module-seams.md §10）
- 「Interface 不暴露 token、query key、generation number 或 SecureStore key。Scope Lease 是不透明、可撤销的能力对象，子 Module 每次异步提交前检查其有效性。」（mobile-module-seams.md §4.2）
- 「MIME、大小限制、signed URL、Diff parser、终端日志分页和 native share 都隐藏在 Module 后。」（mobile-module-seams.md §7.2）
- 「任务产物（Task Artifact）：任务生成并由 WeKnora 保存的不可变版本化结果……成员可在移动端预览、比较、批注、下载、分享，或要求 Agent 基于指定版本生成新版本，但不能原地覆盖已存在或已审批的版本。」（CONTEXT.md）
- 「移动 AI Office 将现有 WeKnora Session 呈现为 Task，并保持 `taskId = sessionId`，不新增第二个 Task 聚合身份。」（ADR-0004）——材料 wire 按 run 寻址（`/workbench/executions/:run_id/artifacts`），taskId 可由列表行派生，模块不另造身份。
- 安全约束（会话注入）：服务端 SQL 一律参数绑定（本计划新查询为 `?` 占位 + gorm 绑定，不拼接外部输入）；凭据只从环境变量读取，源码与测试不写入可用凭据字面量（签名测试密钥逐测试随机生成，沿用 `internal/modules/workbench/artifact_signing_test.go:16-22` 模式）；blob 抓取适配器仅允许 http/https 且模块层把 grant URL origin 钉在活动部署 origin 上（拒绝任何私网/换源重定向面）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。

**Issue #46 验收标准原文（docs/plans/issue30-sweep/issues/issue-46.md）：**

1. 「Artifact 版本不可原地覆盖，签名 URL 不长期缓存。」
2. 「不支持、大文件、grant 过期和 Terminal 输入尝试均正确处理。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明（blocked-env 声明）：真实端到端（生产 JSON transport + 授权通道 + 具体 Remote Adapter + Task Material 编排 + 真后端材料列表/签名授权/免凭据下载）沿用 T01–T05 已合并的 opt-in 真实 HTTP 模式，需要「一个真实 WeKnora Deployment（HTTPS origin，且已配置 `WEKNORA_ARTIFACT_SIGNING_KEY`）+ 一个测试账号」（`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` 环境变量）。本地无此环境时 Task 9 的真实 HTTP 用例以 `t.skip` 跳过（**不得伪造通过**）。本地替代证据：Task Material Interface 级场景测试（Task 6，真实模块编排 + in-memory scenario Adapter，覆盖 AC1/AC2 全部分支）+ Go 集成测试（真实 sqlite 迁移库上的终端日志分页与版本身份派生，Task 1–3）+ api-client wire 契约测试（真实序列化字节，Task 7）+ 控制器/屏渲染测试（Task 8）。凡具备环境的运行都自动产出端到端证据。真机系统分享面板与原生下载管理器属真机验收门槛（spec Testing Decisions：「real-device acceptance separately」），本地以 scripted SharePort 为证据，不伪造真机结论。

**与调查结论的差异记录（以代码现状为准）：**

1. 调查称「消息索引寻址与版本 ID 寻址（artifact_download.go:513）两套并存」。亲眼核实：版本 ID 寻址是 `internal/handler/session/artifact_download.go` 中 `ArtifactVersionDownloadHandler.DownloadArtifactVersion`（W26 路由 `GET /sessions/:session_id/artifact-versions/:version_id/download`），其数据源 `artifact_versions` 表**只由 `Import` 生产路径填充**（容器 `registerArtifactVersionHTTPHandlers`，paseo 桥场景）；普通会话消息工件（`types.MessageArtifact`）没有版本行。强行 join 会为不存在的行编造数据。因此统一方式是：workbench 列表的版本身份从 `MessageArtifact.ContentHash`（SHA-256，有则取前 16 hex）派生，无 hash 时以 `(message_id, index)` 绑定地址兜底——两者都在内容变化时改变、都不可原地改写（消息重发产生新 message id → 新地址；版本表 Insert 永不 upsert）。
2. 调查引用的预览票据测试在 `internal/handler/artifact_preview_test.go`（非 session 包），属 sessions API 的 Web 隔离 origin 渲染面。移动端预览判定在本计划模块内以 MIME + 大小实现（spec §7.2「MIME、大小限制……隐藏在 Module 后」），不复用 Web preview ticket。
3. 调查称「批注（annotation）全仓无实现」——属实。批注→新版本是 #47 的验收标准，本计划不实现（见 Global Constraints 第一条的边界声明）。
4. 调查称版本字段硬编码 `"1"` 在 `internal/handler/session/workbench_artifacts.go:74`——亲眼核实为该文件 `artifactListItemFromRef` 中 `Version: "1",`（当前 HEAD 行号 74）。Task 3 替换。
5. 终端日志无既有生产者：`agent_run_events` 现无 `tool.terminal` 事件写入，但 `observationType`（`internal/application/repository/execution_observation.go:77`）已接受 `tool.` 前缀——paseo 桥经 `POST /workbench/executions/:run_id/source-events` 上报 `{"type":"tool.terminal","payload":{"stream":"stdout","text":"…"}}` 即入库，无需摄入端改动。本计划交付的是**读面**（分页端点 + 移动模块），不虚构已有日志内容。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **被重放的篡改/过期签名链接**：泄漏的下载 URL 被换绑到别的 message/index/租户或在过期后重放。服务端常量时间 HMAC 已拒（`workbench_artifacts_test.go` 既有 `TestDownloadWorkbenchArtifactGrantRejectsTamperedAndExpired`）；移动面必须把它映射为「重新授权」信号而不是死链。——Task 6 测试「a grant that expired during a blob fetch maps to MATERIAL_GRANT_EXPIRED, notifies subscribers and never poisons the cache」+ Task 9 集成证据断言 `grantTtlSeconds <= 900`。
2. **服务端铸造的 grant URL 指向非活动部署 origin**（代理误配 X-Forwarded-Proto/Host 或被篡改的授权响应）：若直接抓取会把凭据免流量导向陌生主机。——Task 6 测试「a grant url whose origin differs from the active deployment fails closed before any fetch」（断言 blob 零调用）。
3. **版本身份变化后旧缓存字节被复用**：同名材料重新生成（新版本身份）后，若 blob 缓存按名字键控会呈现服务端从未确认的旧内容。——Task 6 测试「cached bytes are keyed by (materialId, version); a version change forces a fresh grant and fetch」。
4. **终端日志载荷畸形**（非对象 JSON、缺 text/stream 字段的事件行）：不得伪装成输出、不得让分页卡死在同一游标。——Task 2 Go 测试 `TestGetWorkbenchTerminalLogSkipsMalformedPayloadAndAdvancesCursor`。
5. **不支持 MIME/超大文件误触发预览抓取**：流量与内存放大（移动计费场景）。unsupported 判定必须发生在任何网络调用之前。——Task 6 测试「an unsupported mime or oversized material never mints a grant and never fetches」（断言 grant/blob 零调用）。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | Go：终端日志仓储读 | `AgentRunSnapshotRepository.ReadRunTerminalEvents`（稀疏子序列分页，全参数绑定） |
| 2 | Go：只读终端日志端点 | `WorkbenchReadHandler.GetWorkbenchTerminalLog` + 路由一行 + owner 谓词/501 fail-closed/畸形载荷测试 |
| 3 | Go：材料列表版本身份 | `artifactVersionOf`（ContentHash 派生）+ `digest` 字段 + `terminal.available` 信封标志 |
| 4 | mobile-core：纯投影 | `material-kinds.ts`（kind/预览判定）+ `diff.ts`（unified diff 解析）+ `evidence.ts`（引用投影） |
| 5 | mobile-core：端口与场景 Adapter | `ports.ts`（MaterialBackend/BlobFetch/Share Port）+ `in-memory-material-remote.ts` |
| 6 | mobile-core：`createTaskMaterial` 深模块 | `open({lease}) → TaskMaterialHandle`（index/open/act/subscribe/close 全行为）+ index.ts 导出 |
| 7 | api-client：`createMobileMaterialRemote` | list/signedUrl/terminalLog/events wire 适配 + `./mobile/materials` exports |
| 8 | apps/mobile：材料屏与接线 | `MaterialsScreen` + `materials-view` 控制器 + material 适配器 + `/tasks/materials` 路由 + composition 工厂 + 详情屏入口 + app-smoke 断言 |
| 9 | apps/mobile：真实 HTTP 集成证据 | `material-integration-smoke.ts`（opt-in，AC3）+ 证据契约测试 |

**并行批次注意（本计划与同批 6 个计划并行实施）**：新增文件全部为本计划独有（上表 Create 项）。共享文件修改清单与位置：`internal/router/routes_workbench.go`（读组内加 1 行路由）；`internal/handler/session/workbench_artifacts.go`（`artifactListItemFromRef` 与列表信封两处小改）+ `workbench_artifacts_test.go`（改 1 处期望 + 加 2 个测试）；`packages/mobile-core/src/index.ts`（末尾追加 material 导出块）；`packages/api-client/package.json`（exports 加 1 行）；`apps/mobile/src/composition.ts`（`taskOfficeFor` 之后追加 material 工厂与 `activeTaskMaterial`）；`apps/mobile/src/screens/TaskDetailScreen.tsx`（加 1 个可选 prop + 1 个按钮）；`apps/mobile/src/app/tasks/detail.tsx`（透传 `onOpenMaterials` 并 router.push）；`apps/mobile/src/app-smoke.test.tsx`（react-native stub 加 `Image` + 末尾追加 2 个测试）。均为最小、位置明确的追加，便于合并。

---

### Task 1: Go——`ReadRunTerminalEvents` 仓储读（稀疏子序列分页）

**Files:**
- Create: `internal/application/repository/agent_run_terminal.go`
- Test: `internal/application/repository/agent_run_terminal_test.go`

**Interfaces:**
- Consumes: `AgentRunSnapshotRepository`（`internal/application/repository/agent_run_snapshot.go:51` 构造器）、包内 `snapshotEventRow`（`:70-80`，表 `agent_run_events`，列 `event_type`）、包内 `executionEventFor`（`execution_observation.go:233`）、既有测试助手 `openRunTestDB`（`agent_run_test.go:29`，真实 sqlite 迁移库）与 `AgentRunStore.Admit/Claim/AppendEvent`。
- Produces: `TerminalLogEventType = "tool.terminal"`（包级常量）；`func (s *AgentRunSnapshotRepository) ReadRunTerminalEvents(ctx context.Context, key agentruntime.RunKey, after int64, limit int) ([]workbench.ExecutionEvent, error)`——Task 2 的 `TerminalLogReader` seam 依赖此签名。

- [ ] **Step 1: 写失败测试**

创建 `internal/application/repository/agent_run_terminal_test.go`：

```go
package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
)

// seedTerminalRun 写入交错的事件序列：终端块稀疏地夹在普通事件之间，
// 分页读必须只返回 tool.terminal 子序列且按 seq 升序。
func seedTerminalRun(t *testing.T, count int) (agentruntime.RunKey, *AgentRunSnapshotRepository) {
	t.Helper()
	db := openRunTestDB(t)
	store := NewAgentRunStore(db)
	snapshots := NewAgentRunSnapshotRepository(db)
	ctx := context.Background()
	key := agentruntime.RunKey{TenantID: 1, RunID: "term-r1"}
	user, _ := json.Marshal(map[string]any{"role": "user", "content": "q"})
	assistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	_, err := store.Admit(ctx, agentruntime.Admission{
		Key: key, SessionID: "s1", UserID: "u1", RequestID: "term-q1",
		AssistantMessageID: "term-a1", RequestHash: "term-h1",
		Snapshot:    json.RawMessage(`{"version":1}`),
		UserMessage: user, AssistantMessage: assistant,
		Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	fence, err := store.Claim(ctx, key, "w", time.Minute)
	require.NoError(t, err)
	for i := 0; i < count; i++ {
		payload, _ := json.Marshal(map[string]any{"stream": "stdout", "text": "$ echo " + string(rune('a'+i))})
		_, err = store.AppendEvent(ctx, fence, agentruntime.RunEvent{Type: "tool.terminal", Payload: payload})
		require.NoError(t, err)
		noise, _ := json.Marshal(map[string]any{"n": i})
		_, err = store.AppendEvent(ctx, fence, agentruntime.RunEvent{Type: "text.delta", Payload: noise})
		require.NoError(t, err)
	}
	return key, snapshots
}

func TestReadRunTerminalEventsPagesSparseSubsequence(t *testing.T) {
	key, snapshots := seedTerminalRun(t, 3)

	first, err := snapshots.ReadRunTerminalEvents(context.Background(), key, 0, 2)
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.EqualValues(t, 1, first[0].Seq)
	require.EqualValues(t, 3, first[1].Seq) // seq 2 是 text.delta，不得出现
	for _, event := range first {
		require.Equal(t, TerminalLogEventType, event.Type)
		require.NotEmpty(t, event.OccurredAt)
	}

	second, err := snapshots.ReadRunTerminalEvents(context.Background(), key, first[len(first)-1].Seq, 2)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.EqualValues(t, 5, second[0].Seq)

	third, err := snapshots.ReadRunTerminalEvents(context.Background(), key, 5, 2)
	require.NoError(t, err)
	require.Empty(t, third)
}

func TestReadRunTerminalEventsScopesByTenantAndRun(t *testing.T) {
	key, snapshots := seedTerminalRun(t, 1)

	foreignTenant := agentruntime.RunKey{TenantID: 2, RunID: key.RunID}
	empty, err := snapshots.ReadRunTerminalEvents(context.Background(), foreignTenant, 0, 10)
	require.NoError(t, err)
	require.Empty(t, empty)

	foreignRun := agentruntime.RunKey{TenantID: key.TenantID, RunID: "other-run"}
	emptyTwo, err := snapshots.ReadRunTerminalEvents(context.Background(), foreignRun, 0, 10)
	require.NoError(t, err)
	require.Empty(t, emptyTwo)
}

func TestReadRunTerminalEventsRejectsInvalidInput(t *testing.T) {
	key, snapshots := seedTerminalRun(t, 1)
	_, err := snapshots.ReadRunTerminalEvents(context.Background(), agentruntime.RunKey{}, 0, 10)
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
	_, err = snapshots.ReadRunTerminalEvents(context.Background(), key, -1, 10)
	require.ErrorIs(t, err, agentruntime.ErrNotFound)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/repository/ -run 'TestReadRunTerminalEvents' -count=1`
Expected: FAIL——`undefined: TerminalLogEventType`（编译错误，测试未过编译即为 RED 证据）。

- [ ] **Step 3: 最小实现**

创建 `internal/application/repository/agent_run_terminal.go`：

```go
package repository

import (
	"context"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/workbench"
)

// TerminalLogEventType is the product event type that carries one chunk of
// read-only terminal output. Producers (the Paseo bridge) persist it through
// the authenticated source-event callback; observationType already admits the
// "tool." prefix, so ingestion needs no change.
const TerminalLogEventType = "tool.terminal"

const maxTerminalLogPageSize = 256

// ReadRunTerminalEvents pages a run's read-only terminal log: the sparse
// subsequence of TerminalLogEventType events ordered by product seq. Unlike
// ReadRunEvents, gaps between terminal chunks are expected (other event
// types interleave), so contiguity is NOT enforced and there is no
// cursor-expiry error — `after` simply resumes from the last delivered seq.
// The read is fully parameter-bound; tenant and run always come from the
// caller's ownership predicate, never from the request path alone.
func (s *AgentRunSnapshotRepository) ReadRunTerminalEvents(ctx context.Context, key agentruntime.RunKey, after int64, limit int) ([]workbench.ExecutionEvent, error) {
	if s == nil || s.db == nil || key.TenantID == 0 || key.RunID == "" || after < 0 {
		return nil, agentruntime.ErrNotFound
	}
	if limit <= 0 || limit > maxTerminalLogPageSize {
		limit = maxTerminalLogPageSize
	}
	var rows []snapshotEventRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND run_id = ? AND event_type = ? AND seq > ?", key.TenantID, key.RunID, TerminalLogEventType, after).
		Order("seq ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	events := make([]workbench.ExecutionEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, executionEventFor(row.RunID, row.AttemptID, row.Seq, row.EventType, row.Payload, row.CreatedAt))
	}
	return events, nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/application/repository/ -run 'TestReadRunTerminalEvents' -count=1`
Expected: PASS（3 个测试全绿）。

- [ ] **Step 5: REFACTOR 检查**

`ReadRunTerminalEvents` 与 `ReadRunEvents`（`agent_run_snapshot.go:231`）共享行→事件转换，已复用 `executionEventFor`；无重复可再收敛。确认 `go vet ./internal/application/repository/` 无输出。

- [ ] **Step 6: 提交**

```bash
git add internal/application/repository/agent_run_terminal.go internal/application/repository/agent_run_terminal_test.go
git commit -m "feat(workbench): page the read-only terminal subsequence of a run's event log"
```

---

### Task 2: Go——`GET /workbench/executions/:run_id/terminal-log` 只读端点

**Files:**
- Create: `internal/handler/session/workbench_terminal_log.go`
- Test: `internal/handler/session/workbench_terminal_log_test.go`
- Modify: `internal/router/routes_workbench.go:47`（读组内、`/:run_id/events` 之后加 1 行）

**Interfaces:**
- Consumes: Task 1 的 `repository.TerminalLogEventType` 与 `ReadRunTerminalEvents`（生产装配中 `h.snapshots` 即 `*repository.AgentRunSnapshotRepository`，`internal/container/workbench.go:20-27`，天然实现新方法——**不改容器**）；`resolveOwnedRun`/`OwnedRunReader`（`workbench_read.go:123-155`，owner 谓词共享入口）；`writeWorkbenchError`（`workbench_read.go:365`）；测试助手 `workbenchRunReaderStub`（`workbench_read_test.go:27-39`，owner 谓词硬编码 tenant 1/user u1）。
- Produces: `GET /api/v1/workbench/executions/:run_id/terminal-log?after=&limit=` → `{"success":true,"data":{"lines":[{"seq","occurred_at","stream","text"}],"next_cursor":<int>}}`；404（非本人 run/不存在）、400（畸形 after/limit）、401（未认证）、501 `terminal_log_unavailable`（snapshots 未实现终端 seam——旧装配 fail closed）。Task 7 的 remote `terminalLog` 依赖此 wire。

- [ ] **Step 1: 写失败测试**

创建 `internal/handler/session/workbench_terminal_log_test.go`：

```go
package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/workbench"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type terminalReaderStub struct {
	terminal []workbench.ExecutionEvent
	calls    int
	lastAfter int64
	lastLimit int
}

func (s *terminalReaderStub) ReadRunSnapshot(context.Context, agentruntime.RunKey) (workbench.ExecutionSnapshot, error) {
	return workbench.ExecutionSnapshot{}, nil
}

func (s *terminalReaderStub) ReadRunEvents(context.Context, agentruntime.RunKey, int64, int) ([]workbench.ExecutionEvent, int64, error) {
	return nil, 0, nil
}

func (s *terminalReaderStub) ReadRunTerminalEvents(_ context.Context, _ agentruntime.RunKey, after int64, limit int) ([]workbench.ExecutionEvent, error) {
	s.calls++
	s.lastAfter = after
	s.lastLimit = limit
	return s.terminal, nil
}

func terminalEvents() []workbench.ExecutionEvent {
	return []workbench.ExecutionEvent{
		{SchemaVersion: 1, RunID: "run-1", AttemptID: "a1", Seq: 3, Type: "tool.terminal", OccurredAt: "2026-09-24T00:00:03Z", Payload: json.RawMessage(`{"stream":"stdout","text":"$ cargo test\n"}`)},
		{SchemaVersion: 1, RunID: "run-1", AttemptID: "a1", Seq: 7, Type: "tool.terminal", OccurredAt: "2026-09-24T00:00:07Z", Payload: json.RawMessage(`{"stream":"stderr","text":"warning: unused import\n"}`)},
	}
}

func terminalLogContext(query string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/workbench/executions/run-1/terminal-log"+query, nil)
	ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	c.Request = c.Request.WithContext(ctx)
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	return c, recorder
}

func TestGetWorkbenchTerminalLogProjectsReadOnlyLines(t *testing.T) {
	reader := &terminalReaderStub{terminal: terminalEvents()}
	h := NewWorkbenchReadHandler(artifactRunStub(), reader)
	c, rec := terminalLogContext("")
	h.GetWorkbenchTerminalLog(c)

	require.Equal(t, http.StatusOK, c.Writer.Status())
	require.EqualValues(t, 0, reader.lastAfter)
	require.Equal(t, 200, reader.lastLimit) // 默认页大小
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Lines []struct {
				Seq        int64  `json:"seq"`
				OccurredAt string `json:"occurred_at"`
				Stream     string `json:"stream"`
				Text       string `json:"text"`
			} `json:"lines"`
			NextCursor int64 `json:"next_cursor"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Len(t, body.Data.Lines, 2)
	require.EqualValues(t, 3, body.Data.Lines[0].Seq)
	require.Equal(t, "stdout", body.Data.Lines[0].Stream)
	require.Equal(t, "$ cargo test\n", body.Data.Lines[0].Text)
	require.Equal(t, "stderr", body.Data.Lines[1].Stream)
	require.EqualValues(t, 7, body.Data.NextCursor)
}

func TestGetWorkbenchTerminalLogResumesAndClamps(t *testing.T) {
	reader := &terminalReaderStub{terminal: terminalEvents()[1:]}
	h := NewWorkbenchReadHandler(artifactRunStub(), reader)
	c, _ := terminalLogContext("?after=3&limit=9999")
	h.GetWorkbenchTerminalLog(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	require.EqualValues(t, 3, reader.lastAfter)
	require.Equal(t, 256, reader.lastLimit) // 超限钳到上限

	bad, _ := terminalLogContext("?after=-1")
	h.GetWorkbenchTerminalLog(bad)
	require.Equal(t, http.StatusBadRequest, bad.Writer.Status())

	badLimit, _ := terminalLogContext("?limit=0")
	h.GetWorkbenchTerminalLog(badLimit)
	require.Equal(t, http.StatusBadRequest, badLimit.Writer.Status())
}

func TestGetWorkbenchTerminalLogSkipsMalformedPayloadAndAdvancesCursor(t *testing.T) {
	events := append(terminalEvents(), workbench.ExecutionEvent{
		SchemaVersion: 1, RunID: "run-1", AttemptID: "a1", Seq: 9, Type: "tool.terminal",
		OccurredAt: "2026-09-24T00:00:09Z", Payload: json.RawMessage(`"not an object"`),
	})
	reader := &terminalReaderStub{terminal: events}
	h := NewWorkbenchReadHandler(artifactRunStub(), reader)
	c, rec := terminalLogContext("")
	h.GetWorkbenchTerminalLog(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	var body struct {
		Data struct {
			Lines      []json.RawMessage `json:"lines"`
			NextCursor int64             `json:"next_cursor"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data.Lines, 2, "malformed payload must not be fabricated into output")
	require.EqualValues(t, 9, body.Data.NextCursor, "cursor must advance past the skipped chunk so paging cannot stall")
}

func TestGetWorkbenchTerminalLogIsOwnerScopedAndFailsClosed(t *testing.T) {
	reader := &terminalReaderStub{terminal: terminalEvents()}
	h := NewWorkbenchReadHandler(artifactRunStub(), reader)

	// 跨租户：run 读谓词先拒，终端读零调用。
	foreign, _ := terminalLogContext("")
	ctx := context.WithValue(foreign.Request.Context(), types.TenantIDContextKey, uint64(2))
	foreign.Request = foreign.Request.WithContext(ctx)
	h.GetWorkbenchTerminalLog(foreign)
	require.Equal(t, http.StatusNotFound, foreign.Writer.Status())
	require.Equal(t, 0, reader.calls)

	// snapshots 未实现终端 seam：诚实 501，不降级为空日志。
	h2 := NewWorkbenchReadHandler(artifactRunStub(), &workbenchSnapshotReaderStub{})
	c2, rec2 := terminalLogContext("")
	h2.GetWorkbenchTerminalLog(c2)
	require.Equal(t, http.StatusNotImplemented, c2.Writer.Status())
	require.Contains(t, rec2.Body.String(), "terminal_log_unavailable")
}
```

注意：`workbenchSnapshotReaderStub`（`workbench_read_test.go:41-44`）不实现 `ReadRunTerminalEvents`，正好充当「旧装配」的 501 用例。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/handler/session/ -run 'TestGetWorkbenchTerminalLog' -count=1`
Expected: FAIL——`h.GetWorkbenchTerminalLog undefined (type *session.WorkbenchReadHandler has no field or method GetWorkbenchTerminalLog)`。

- [ ] **Step 3: 最小实现**

创建 `internal/handler/session/workbench_terminal_log.go`：

```go
package session

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/workbench"
	"github.com/gin-gonic/gin"
)

// TerminalLogReader pages the read-only terminal projection of a run's
// durable event log. It is deliberately its own seam: the subsequence is
// sparse, so the contiguous ReadRunEvents contract does not apply. The
// production repository implements it, so the handler resolves the seam by
// assertion at request time — an assembly without it fails closed (501)
// instead of degrading to a fabricated empty log.
type TerminalLogReader interface {
	ReadRunTerminalEvents(ctx context.Context, key agentruntime.RunKey, after int64, limit int) ([]workbench.ExecutionEvent, error)
}

const (
	workbenchTerminalLogDefaultLimit = 200
	workbenchTerminalLogMaxLimit     = 256
)

// workbenchTerminalLine is the wire shape of one read-only terminal chunk.
type workbenchTerminalLine struct {
	Seq        int64  `json:"seq"`
	OccurredAt string `json:"occurred_at"`
	Stream     string `json:"stream"`
	Text       string `json:"text"`
}

// GetWorkbenchTerminalLog godoc
// @Summary      分页读取只读终端日志
// @Description  按 run 归属分页返回只读终端输出（事件类型 tool.terminal 的稀疏子序列，按 seq 升序）；无任何输入通道——移动面终端只读，交互式 PTY 仅存在于 Web 沙箱面
// @Tags         工作台
// @Produce      json
// @Param        run_id  path  string  true  "执行ID"
// @Param        after   query int     false "上一页最后一条 seq（默认 0）"
// @Param        limit   query int     false "页大小（默认 200，上限 256）"
// @Success      200  {object}  map[string]interface{}
// @Failure      401  {object}  errors.AppError
// @Failure      400  {object}  errors.AppError
// @Failure      404  {object}  errors.AppError
// @Failure      501  {object}  errors.AppError
// @Security     Bearer
// @Router       /workbench/executions/{run_id}/terminal-log [get]
func (h *WorkbenchReadHandler) GetWorkbenchTerminalLog(c *gin.Context) {
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	reader, supported := h.snapshots.(TerminalLogReader)
	if !supported || reader == nil {
		c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{
			"success": false,
			"code":    "terminal_log_unavailable",
			"error":   "terminal log projection not wired",
		})
		return
	}
	after := int64(0)
	if raw := strings.TrimSpace(c.Query("after")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		after = parsed
	}
	limit := workbenchTerminalLogDefaultLimit
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	if limit > workbenchTerminalLogMaxLimit {
		limit = workbenchTerminalLogMaxLimit
	}
	events, err := reader.ReadRunTerminalEvents(c.Request.Context(), run.Key, after, limit)
	if err != nil {
		writeWorkbenchError(c, err)
		return
	}
	lines := make([]workbenchTerminalLine, 0, len(events))
	next := after
	for _, event := range events {
		var payload struct {
			Stream string `json:"stream"`
			Text   string `json:"text"`
		}
		jsonErr := json.Unmarshal(event.Payload, &payload)
		if jsonErr != nil || (payload.Text == "" && payload.Stream == "") {
			// A payload that is not a terminal-chunk object is never
			// fabricated into output, but the cursor still advances past it
			// so paging cannot stall on the same malformed seq.
			next = event.Seq
			continue
		}
		if payload.Stream != "stdout" && payload.Stream != "stderr" {
			payload.Stream = "stdout"
		}
		lines = append(lines, workbenchTerminalLine{
			Seq:        event.Seq,
			OccurredAt: event.OccurredAt,
			Stream:     payload.Stream,
			Text:       payload.Text,
		})
		next = event.Seq
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"lines": lines, "next_cursor": next}})
}
```

修改 `internal/router/routes_workbench.go`——在 `if h != nil` 读组内、`workbench.GET("/:run_id/events", h.StreamWorkbenchEvents)` 之后加：

```go
		// T16: read-only terminal log paging — no input lane exists here; the
		// interactive PTY stays on the web sandbox surface only.
		workbench.GET("/:run_id/terminal-log", h.GetWorkbenchTerminalLog)
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/handler/session/ -run 'TestGetWorkbenchTerminalLog' -count=1`
Expected: PASS（4 个测试全绿）。随后跑回归：`go test ./internal/handler/session/ -run 'TestListWorkbenchArtifacts|TestCreateWorkbenchArtifactSignedURL|TestDownloadWorkbenchArtifactGrant' -count=1` 仍 ok。

- [ ] **Step 5: REFACTOR 检查**

`after/limit` 解析与 `workbench_commands.go` 等处 query 解析风格一致（TrimSpace + ParseInt + 显式 400），保持内联——抽出共享 helper 会触碰共享文件，收益不足。`go vet ./internal/handler/session/` 无输出。

- [ ] **Step 6: 提交**

```bash
git add internal/handler/session/workbench_terminal_log.go internal/handler/session/workbench_terminal_log_test.go internal/router/routes_workbench.go
git commit -m "feat(workbench): read-only terminal log endpoint paged by product seq"
```

---

### Task 3: Go——材料列表真实版本身份、digest 与终端可用性标志

**Files:**
- Modify: `internal/handler/session/workbench_artifacts.go:47-107`（`workbenchArtifactItem` 结构、`artifactListItemFromRef`、`ListWorkbenchArtifacts` 信封）
- Test: `internal/handler/session/workbench_artifacts_test.go`（改 1 处期望 + 追加 2 个测试）

**Interfaces:**
- Consumes: `types.SessionArtifactRef`/`types.MessageArtifact`（`internal/types/message.go:234-256`，`ContentHash string json:"content_hash,omitempty"` 为 SHA-256 hex）；既有测试助手 `artifactRefs`/`artifactContext`/`artifactRunStub`（`workbench_artifacts_test.go:47-96`）。
- Produces: 列表项新增 `digest`（omitempty，ContentHash 原文）；`version` 从 `"1"` 变为 ContentHash 前 16 hex（无 hash 时 `"<message_id>:<index>"`）；列表信封新增 `"terminal": {"available": true}`。Task 7 的 remote `list` 依赖此 wire。

- [ ] **Step 1: 写失败测试**

在 `internal/handler/session/workbench_artifacts_test.go` 中：

(a) 将 `TestListWorkbenchArtifactsProjectsMessageBoundRefs` 内的 `require.Equal(t, "1", first.Version)` 改为：

```go
	require.Equal(t, "msg-1:0", first.Version) // 无 ContentHash 时以 (message, index) 绑定地址为版本身份
```

(b) 追加两个测试（放在 `TestListWorkbenchArtifactsScopesByOwner` 之后）：

```go
func TestListWorkbenchArtifactsDerivesVersionFromDigest(t *testing.T) {
	digest := "aaaaaaaaaaaabbbbbbbbbbccccccccccccdddddddddddd"
	refs := artifactRefReaderStub{refs: []types.SessionArtifactRef{
		{MessageID: "msg-9", Index: 0, Artifact: types.MessageArtifact{
			URL: "local://tenant/1/report.md", FileName: "report.md", FileType: ".md", FileSize: 10,
			ContentHash: digest,
			CreatedAt:   time.Unix(1_700_000_000, 0),
		}},
	}}
	h := NewWorkbenchArtifactHandler(artifactRunStub(), &refs)
	c, rec := artifactContext()
	h.ListWorkbenchArtifacts(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	var body struct {
		Data struct {
			Items []workbenchArtifactItem `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data.Items, 1)
	// 内容寻址：版本取 digest 前缀，digest 原文随行下发——内容变则两者变，不可原地改写。
	require.Equal(t, digest[:16], body.Data.Items[0].Version)
	require.Equal(t, digest, body.Data.Items[0].Digest)
}

func TestListWorkbenchArtifactsDeclaresTerminalAvailability(t *testing.T) {
	refs := artifactRefReaderStub{refs: artifactRefs()}
	h := NewWorkbenchArtifactHandler(artifactRunStub(), &refs)
	c, rec := artifactContext()
	h.ListWorkbenchArtifacts(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	var body struct {
		Data struct {
			Terminal struct {
				Available bool `json:"available"`
			} `json:"terminal"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Data.Terminal.Available, "this server version mounts the terminal-log endpoint; older deployments omit the flag")
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/handler/session/ -run 'TestListWorkbenchArtifacts' -count=1`
Expected: FAIL——`TestListWorkbenchArtifactsProjectsMessageBoundRefs`（version 实际仍为 `"1"`）、`TestListWorkbenchArtifactsDerivesVersionFromDigest`（version/digest 不符）、`TestListWorkbenchArtifactsDeclaresTerminalAvailability`（terminal.available 为 false）。

- [ ] **Step 3: 最小实现**

修改 `internal/handler/session/workbench_artifacts.go`：

(a) `workbenchArtifactItem` 结构加 `Digest` 字段（`Version` 之后）：

```go
	Version   string `json:"version"`
	// Digest is the persisted SHA-256 of the artifact bytes when the
	// producer recorded one; the content-addressed proof behind Version.
	Digest    string `json:"digest,omitempty"`
	Size      int64  `json:"size"`
```

(b) 在 `artifactListItemFromRef` 之前加版本派生函数，并替换 `Version: "1",`：

```go
// artifactVersionOf derives the immutable version identity of a
// message-bound artifact. Content-addressed when the persisted bytes carry
// a SHA-256 digest, otherwise the (message, index) binding itself: both
// change when the content changes, and neither can be rewritten in place —
// artifact versions insert without upsert (repository.ErrArtifactVersion-
// Conflict on any difference) and a re-produced file lands under a new
// message binding.
func artifactVersionOf(ref types.SessionArtifactRef) string {
	digest := strings.TrimSpace(ref.Artifact.ContentHash)
	if len(digest) >= 16 {
		return digest[:16]
	}
	if digest != "" {
		return digest
	}
	return ref.MessageID + ":" + strconv.Itoa(ref.Index)
}
```

`artifactListItemFromRef` 内：

```go
		Version:   artifactVersionOf(ref),
		Digest:    strings.TrimSpace(ref.Artifact.ContentHash),
```

(c) `ListWorkbenchArtifacts` 的响应信封：

```go
	// terminal declares whether this server mounts the read-only terminal
	// log endpoint; older deployments omit the flag and clients degrade.
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": items, "terminal": gin.H{"available": true}}})
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/handler/session/ -run 'TestListWorkbenchArtifacts|TestCreateWorkbenchArtifactSignedURL|TestDownloadWorkbenchArtifactGrant' -count=1`
Expected: PASS（含既有签名/下载测试回归）。

- [ ] **Step 5: REFACTOR 检查**

`CreateWorkbenchArtifactSignedURL` 响应中的 `artifact` 回显复用 `artifactListItemFromRef`，版本身份自动一致，无需第二处改动。`go vet ./internal/handler/session/` 无输出。

- [ ] **Step 6: 提交**

```bash
git add internal/handler/session/workbench_artifacts.go internal/handler/session/workbench_artifacts_test.go
git commit -m "feat(workbench): derive real artifact version identity and declare terminal availability"
```

---

### Task 4: mobile-core——材料纯投影（kind/预览判定、unified diff、evidence 引用）

**Files:**
- Create: `packages/mobile-core/src/material/material-kinds.ts`
- Create: `packages/mobile-core/src/material/diff.ts`
- Create: `packages/mobile-core/src/material/evidence.ts`
- Create: `packages/mobile-core/src/material/types.ts`（本任务先落纯投影所需的最小类型；Task 6 补全其余）
- Test: `packages/mobile-core/src/material/material-kinds.test.ts`
- Test: `packages/mobile-core/src/material/diff.test.ts`
- Test: `packages/mobile-core/src/material/evidence.test.ts`

**Interfaces:**
- Consumes: 无外部依赖（纯函数，domain-free；`MaterialEntryKind`/`PreviewVerdict`/`EvidenceCitation` 在本任务定义）。
- Produces（Task 5/6 依赖的精确签名）:
  - `type MaterialEntryKind = 'artifact' | 'diff' | 'test-report'`
  - `type PreviewVerdict = { state: 'supported' } | { state: 'unsupported'; reason: 'mime' | 'size' }`
  - `const PREVIEW_MAX_BYTES = 2 * 1024 * 1024`
  - `function materialKindOf(name: string, mime: string): MaterialEntryKind`
  - `function previewVerdictOf(mime: string, size: number): PreviewVerdict`
  - `function isInlineImageMime(mime: string): boolean`
  - `interface DiffLine { origin: 'context' | 'add' | 'remove'; text: string }`、`interface DiffHunk { header: string; oldStart: number; newStart: number; lines: DiffLine[] }`、`function parseUnifiedDiff(text: string): { hunks: DiffHunk[]; malformed: boolean }`
  - `interface EvidenceCitation { seq: number; occurredAt: string; type: string; source?: string; detail: string }`、`function projectCitations(events: ReadonlyArray<{ seq: number; type: string; occurredAt: string; payload: Record<string, unknown> }>): EvidenceCitation[]`

- [ ] **Step 1: 写失败测试**

创建 `packages/mobile-core/src/material/material-kinds.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { PREVIEW_MAX_BYTES, isInlineImageMime, materialKindOf, previewVerdictOf } from './material-kinds.ts';

test('material kinds derive from name and mime without guessing', () => {
  assert.equal(materialKindOf('changes.diff', 'text/x-diff'), 'diff');
  assert.equal(materialKindOf('feature.patch', 'text/plain'), 'diff');
  assert.equal(materialKindOf('app.patch', 'application/x-diff'), 'diff');
  assert.equal(materialKindOf('test-report.json', 'application/json'), 'test-report');
  assert.equal(materialKindOf('junit.xml', 'application/xml'), 'test-report');
  assert.equal(materialKindOf('TEST-RESULTS.txt', 'text/plain'), 'test-report');
  assert.equal(materialKindOf('report.md', 'text/markdown'), 'artifact');
  assert.equal(materialKindOf('data.csv', 'text/csv'), 'artifact');
  assert.equal(materialKindOf('monthly.tests.csv', 'text/csv'), 'artifact', '仅 test-report/junit/test-results 模式判定测试报告，普通文件名不误判');
});

test('preview verdicts gate inline previews by mime and size', () => {
  assert.deepEqual(previewVerdictOf('text/plain', 100), { state: 'supported' });
  assert.deepEqual(previewVerdictOf('text/markdown', PREVIEW_MAX_BYTES), { state: 'supported' }, '恰好在上限处仍可预览');
  assert.deepEqual(previewVerdictOf('text/plain', PREVIEW_MAX_BYTES + 1), { state: 'unsupported', reason: 'size' });
  assert.deepEqual(previewVerdictOf('application/pdf', 100), { state: 'unsupported', reason: 'mime' }, '首版无内联 PDF 渲染器：按不支持处理，走下载/分享路径');
  assert.deepEqual(previewVerdictOf('application/zip', 5), { state: 'unsupported', reason: 'mime' });
  assert.deepEqual(previewVerdictOf('image/png', 1000), { state: 'supported' });
  assert.equal(isInlineImageMime('image/png'), true);
  assert.equal(isInlineImageMime('image/svg+xml'), false, 'SVG 不做内联渲染（脚本面）');
});
```

创建 `packages/mobile-core/src/material/diff.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseUnifiedDiff } from './diff.ts';

const SAMPLE = `--- a/src/app.ts
+++ b/src/app.ts
@@ -1,5 +1,6 @@
 import { createApp } from './app.ts';
+import { logger } from './logger.ts';

 function main() {
-  createApp();
+  createApp({ log: logger });
 }
@@ -10,2 +11,3 @@
 export function helper() {
-  return 1;
+  return 2;
+}
`;

test('a unified diff parses into hunks with add/remove/context lines', () => {
  const { hunks, malformed } = parseUnifiedDiff(SAMPLE);
  assert.equal(malformed, false);
  assert.equal(hunks.length, 2);
  assert.equal(hunks[0]!.oldStart, 1);
  assert.equal(hunks[0]!.newStart, 1);
  // 第 3 行是真空行（编辑器常剥掉 context 行的前导空格）——按 context 解析，不判畸形。
  assert.deepEqual(hunks[0]!.lines.map((line) => line.origin), ['context', 'add', 'context', 'context', 'remove', 'add', 'context']);
  assert.equal(hunks[0]!.lines[1]!.text, "import { logger } from './logger.ts';");
});

test('malformed content is reported instead of half-parsed hunks', () => {
  const broken = parseUnifiedDiff('this is not\na diff at all\n');
  assert.equal(broken.malformed, true);
  assert.deepEqual(broken.hunks, []);

  // + 行出现在任何 hunk 之前：无法归类，整体 malformed（hunk 内的 + 行是合法 add，二者靠位置区分）。
  const dangling = parseUnifiedDiff('--- a/x\n+++ b/x\n+ dangling add before any hunk\n');
  assert.equal(dangling.malformed, true);
  assert.deepEqual(dangling.hunks, [], '一旦发现畸形就整体回退原始文本，不呈现半解析结果');
});

test('an empty diff is a valid empty diff, not malformed input', () => {
  assert.deepEqual(parseUnifiedDiff(''), { hunks: [], malformed: false });
  assert.deepEqual(parseUnifiedDiff('--- a/x\n+++ b/x\n'), { hunks: [], malformed: false });
});

test('the no-newline marker is tolerated inside a hunk', () => {
  const { hunks, malformed } = parseUnifiedDiff('@@ -1 +1 @@\n-a\n\\ No newline at end of file\n+b\n');
  assert.equal(malformed, false);
  assert.deepEqual(hunks[0]!.lines.map((line) => line.origin), ['remove', 'add']);
});
```

创建 `packages/mobile-core/src/material/evidence.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { projectCitations } from './evidence.ts';

test('citations project tool and artifact events with their source binding', () => {
  const citations = projectCitations([
    { seq: 5, type: 'tool.started', occurredAt: '2026-09-24T00:00:05Z', payload: { tool: 'knowledge_search', knowledge_base_id: 'kb-7', query: 'handbook' } },
    { seq: 6, type: 'text.delta', occurredAt: '2026-09-24T00:00:06Z', payload: { text: 'partial' } },
    { seq: 7, type: 'artifact.available', occurredAt: '2026-09-24T00:00:07Z', payload: { file_name: 'report.md' } },
  ]);
  assert.deepEqual(citations.map((citation) => citation.seq), [5, 7], '只有 tool./artifact. 事件是可追溯来源；text 流不混入');
  assert.equal(citations[0]!.type, 'tool.started');
  assert.equal(citations[0]!.source, 'kb-7', 'source 取载荷中第一个已知的来源键');
  assert.ok(citations[0]!.detail.includes('knowledge_search'));
  assert.equal(citations[1]!.source, 'report.md', 'file_name 也是来源键');
});

test('citations never invent a source and always carry the raw detail', () => {
  const citations = projectCitations([
    { seq: 2, type: 'tool.completed', occurredAt: '2026-09-24T00:00:02Z', payload: { elapsed_ms: 42 } },
  ]);
  assert.equal(citations.length, 1);
  assert.equal('source' in citations[0]!, false, '没有已知来源键时不编造 source');
  assert.ok(citations[0]!.detail.includes('42'));
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/material/material-kinds.test.ts packages/mobile-core/src/material/diff.test.ts packages/mobile-core/src/material/evidence.test.ts`
Expected: FAIL——`Cannot find module './material-kinds.ts'`（三个文件均不存在，RED 证据）。

- [ ] **Step 3: 最小实现**

创建 `packages/mobile-core/src/material/types.ts`（本任务所需最小类型区）：

```ts
/** Task Material 深模块（module-seams §7）类型合同。Task 5/6 补全 Port 与句柄类型。 */

export type MaterialEntryKind = 'artifact' | 'diff' | 'test-report';

export type PreviewVerdict =
  | { state: 'supported' }
  | { state: 'unsupported'; reason: 'mime' | 'size' };

export interface EvidenceCitation {
  seq: number;
  occurredAt: string;
  type: string;
  source?: string;
  detail: string;
}
```

创建 `packages/mobile-core/src/material/material-kinds.ts`：

```ts
import type { MaterialEntryKind, PreviewVerdict } from './types.ts';

/** 内联预览的移动端大小上限：超过即判 size 不支持，走下载/分享路径（spec User Story 32）。 */
export const PREVIEW_MAX_BYTES = 2 * 1024 * 1024;

const DIFF_NAME = /\.(diff|patch)$/i;
const DIFF_MIME = new Set(['text/x-diff', 'text/x-patch', 'application/x-diff', 'application/x-patch']);
/** 测试报告仅按明确的文件名模式判定；普通名字不猜测。 */
const REPORT_NAME = /(^|[^a-z])(test[-_]?reports?|test[-_]?results?|junit)([^a-z]|$)/i;
const INLINE_TEXT_MIME = new Set([
  'application/json', 'application/x-ndjson', 'text/csv', 'text/markdown', 'text/x-markdown',
  'text/x-diff', 'text/x-patch',
]);
const INLINE_IMAGE_MIME = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp']);

export function materialKindOf(name: string, mime: string): MaterialEntryKind {
  const trimmedName = typeof name === 'string' ? name.trim() : '';
  const normalizedMime = typeof mime === 'string' ? mime.trim().toLowerCase() : '';
  if (DIFF_NAME.test(trimmedName) || DIFF_MIME.has(normalizedMime)) return 'diff';
  if (REPORT_NAME.test(trimmedName)) return 'test-report';
  return 'artifact';
}

export function previewVerdictOf(mime: string, size: number): PreviewVerdict {
  const normalizedMime = typeof mime === 'string' ? mime.trim().toLowerCase() : '';
  const previewable = normalizedMime.startsWith('text/') || INLINE_TEXT_MIME.has(normalizedMime) || INLINE_IMAGE_MIME.has(normalizedMime);
  if (!previewable) return { state: 'unsupported', reason: 'mime' };
  if (typeof size !== 'number' || !Number.isFinite(size) || size < 0 || size > PREVIEW_MAX_BYTES) {
    return { state: 'unsupported', reason: 'size' };
  }
  return { state: 'supported' };
}

export function isInlineImageMime(mime: string): boolean {
  return INLINE_IMAGE_MIME.has(mime.trim().toLowerCase());
}
```

创建 `packages/mobile-core/src/material/diff.ts`：

```ts
export interface DiffLine {
  origin: 'context' | 'add' | 'remove';
  text: string;
}

export interface DiffHunk {
  header: string;
  oldStart: number;
  newStart: number;
  lines: DiffLine[];
}

const HUNK_HEADER = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/;

/**
 * 解析 unified diff。纪律：文件头（---/+++）之前只允许空行与头行；hunk 内的真空行
 * （编辑器常剥掉 context 空行的前导空格）按 context 解析；"\ No newline at end of
 * file" 标记行忽略；一旦出现无法归类的行即整体 malformed——呈现层回退到原始文本，
 * 绝不展示半解析结果。空文本与只有文件头的输入是合法的空 diff。
 * 注意：hunk 头声明的行数不参与校验（真实工具产出的头计数时有出入，宽松读入）；
 * 畸形判定只依赖行首前缀与位置。
 */
export function parseUnifiedDiff(text: string): { hunks: DiffHunk[]; malformed: boolean } {
  if (typeof text !== 'string' || text.trim() === '') return { hunks: [], malformed: false };
  const lines = text.split('\n');
  // 末尾空行是换行符产物，不是内容。
  if (lines.length > 0 && lines[lines.length - 1] === '') lines.pop();
  const hunks: DiffHunk[] = [];
  let current: DiffHunk | undefined;
  for (const line of lines) {
    if (line.startsWith('--- ') || line.startsWith('+++ ') || line.startsWith('diff --git ')) {
      if (current !== undefined) return { hunks: [], malformed: true }; // 文件头只出现在 hunk 之外
      continue;
    }
    const header = HUNK_HEADER.exec(line);
    if (header !== null) {
      current = { header: line, oldStart: Number(header[1]), newStart: Number(header[2]), lines: [] };
      hunks.push(current);
      continue;
    }
    if (current === undefined) {
      if (line.trim() === '') continue;
      return { hunks: [], malformed: true };
    }
    if (line === '') current.lines.push({ origin: 'context', text: '' }); // 被剥掉前导空格的空 context 行
    else if (line.startsWith(' ')) current.lines.push({ origin: 'context', text: line.slice(1) });
    else if (line.startsWith('+')) current.lines.push({ origin: 'add', text: line.slice(1) });
    else if (line.startsWith('-')) current.lines.push({ origin: 'remove', text: line.slice(1) });
    else if (line.startsWith('\\')) continue; // "\ No newline at end of file"
    else return { hunks: [], malformed: true };
  }
  return { hunks, malformed: false };
}
```

创建 `packages/mobile-core/src/material/evidence.ts`：

```ts
import type { EvidenceCitation } from './types.ts';

/** 已知来源键（取第一个命中的字符串值）；不命中则不编造 source。 */
const SOURCE_KEYS = ['knowledge_base_id', 'document_id', 'file_name', 'url', 'source', 'tool'] as const;

/**
 * 把 run 事件投影为 Evidence 引用：tool.* / artifact.* 事件是可追溯的来源
 * 事实（module-seams §7.1「Evidence Citation 和来源」）。detail 携带原始载荷
 * 摘要——与 Task 时间线「原始证据按需展开」同一纪律，不丢事实也不猜结构。
 */
export function projectCitations(
  events: ReadonlyArray<{ seq: number; type: string; occurredAt: string; payload: Record<string, unknown> }>,
): EvidenceCitation[] {
  const citations: EvidenceCitation[] = [];
  for (const event of events) {
    if (!event.type.startsWith('tool.') && !event.type.startsWith('artifact.')) continue;
    let source: string | undefined;
    for (const key of SOURCE_KEYS) {
      const value = event.payload?.[key];
      if (typeof value === 'string' && value.trim() !== '') {
        source = value;
        break;
      }
    }
    let detail: string;
    try {
      detail = JSON.stringify(event.payload ?? {});
    } catch {
      detail = '[unserializable]';
    }
    citations.push({ seq: event.seq, occurredAt: event.occurredAt, type: event.type, ...(source === undefined ? {} : { source }), detail });
  }
  return citations.sort((a, b) => a.seq - b.seq);
}
```

- [ ] **Step 4: 运行确认通过**

Run: `pnpm exec tsx --test packages/mobile-core/src/material/material-kinds.test.ts packages/mobile-core/src/material/diff.test.ts packages/mobile-core/src/material/evidence.test.ts`
Expected: PASS（全部用例绿。判定规则已实跑核验：`REPORT_NAME` 的三个词根 `test-report(s)/test-result(s)/junit` 均不匹配裸词 `tests`，故 `monthly.tests.csv` 判为 `artifact` 不误判；空行/悬垂 `+` 行/no-newline 标记的 diff 行为已按修复后逻辑以 node 逐一实跑验证——见下方 REFACTOR）。

- [ ] **Step 5: REFACTOR 检查**

`parseUnifiedDiff` 与 `projectCitations` 均为无依赖纯函数，无重复可收敛；确认 `HUNK_HEADER` 正则只在此文件出现。若实现时对空行/dangling 行为有任何调整，必须同步实跑 `node` 单文件验证四类输入（含真空行的 SAMPLE、hunk 前悬垂 `+` 行、空文本、no-newline 标记）后再继续——本计划作者已按上述实现逻辑实跑全部用例通过。

- [ ] **Step 6: 提交**

```bash
git add packages/mobile-core/src/material/material-kinds.ts packages/mobile-core/src/material/diff.ts packages/mobile-core/src/material/evidence.ts packages/mobile-core/src/material/types.ts packages/mobile-core/src/material/material-kinds.test.ts packages/mobile-core/src/material/diff.test.ts packages/mobile-core/src/material/evidence.test.ts
git commit -m "feat(mobile-core/material): pure projections for material kinds, preview verdicts, unified diff and evidence citations"
```

---

### Task 5: mobile-core——Material 端口与 in-memory 场景 Adapter

**Files:**
- Create: `packages/mobile-core/src/material/material-errors.ts`
- Create: `packages/mobile-core/src/material/ports.ts`
- Create: `packages/mobile-core/src/material/in-memory-material-remote.ts`
- Modify: `packages/mobile-core/src/material/types.ts`（追加视图/意图/事件/句柄类型）
- Test: `packages/mobile-core/src/material/in-memory-material-remote.test.ts`

**Interfaces:**
- Consumes: Task 4 的 `MaterialEntryKind`/`PreviewVerdict`/`EvidenceCitation`（`types.ts`）；`ScopeLease`（`../runtime/types.ts`，仅类型引用）。
- Produces（Task 6/8 依赖的精确签名）:

```ts
// material-errors.ts
export type MaterialErrorCode =
  | 'MATERIAL_SCOPE_CHANGED' | 'MATERIAL_CLOSED' | 'MATERIAL_INVALID_INPUT' | 'MATERIAL_NOT_FOUND'
  | 'MATERIAL_GRANT_EXPIRED' | 'MATERIAL_GRANT_INVALID' | 'MATERIAL_SIGNING_DISABLED'
  | 'MATERIAL_TERMINAL_READ_ONLY' | 'MATERIAL_GRANT_ORIGIN' | 'MATERIAL_SHARE_UNAVAILABLE' | 'MATERIAL_BACKEND';
export class MaterialError extends Error {
  constructor(readonly code: MaterialErrorCode, options?: { cause?: unknown });
}

// ports.ts
export interface MaterialBackendArtifact {
  id: string; index: number; name: string; mime: string; version: string; size: number;
  sourceRun: string; createdAt?: string; digest?: string;
}
export interface MaterialBackendList { runId: string; artifacts: MaterialBackendArtifact[]; terminalAvailable: boolean }
export interface MaterialBackendGrant { url: string; expiresAt: string; artifact: MaterialBackendArtifact }
export interface MaterialBackendTerminalLine { seq: number; occurredAt: string; stream: 'stdout' | 'stderr'; text: string }
export interface MaterialBackendTerminalPage { lines: MaterialBackendTerminalLine[]; nextCursor: number }
export interface MaterialBackendEvent { seq: number; type: string; occurredAt: string; payload: Record<string, unknown> }
export interface MaterialBackendPort {
  list(runId: string): Promise<MaterialBackendList>;
  signedUrl(input: { runId: string; index: number }): Promise<MaterialBackendGrant>;
  terminalLog(input: { runId: string; after: number; limit: number }): Promise<MaterialBackendTerminalPage>;
  events(runId: string): Promise<MaterialBackendEvent[]>;
}
export interface BlobFetchPort { fetch(url: string): Promise<{ bytes: Uint8Array; mime: string }> }
export interface SharePort { share(input: { url: string; name: string }): Promise<void> }
export interface TaskMaterialPorts { remote: MaterialBackendPort; blob: BlobFetchPort; share?: SharePort }

// types.ts 追加
export interface MaterialEntry {
  materialId: string; index: number; kind: MaterialEntryKind; name: string; mime: string;
  size: number; version: string; sourceRun: string; createdAt?: string; digest?: string;
}
export interface MaterialIndex { runId: string; materials: MaterialEntry[]; terminal: { available: boolean } }
export type MaterialRef =
  | { kind: MaterialEntryKind; runId: string; materialId: string }
  | { kind: 'evidence'; runId: string }
  | { kind: 'terminal'; runId: string; cursor?: number };
export interface TerminalLine { seq: number; occurredAt: string; stream: 'stdout' | 'stderr'; text: string }
export type MaterialView =
  | { kind: 'artifact' | 'test-report'; entry: MaterialEntry; preview: PreviewVerdict; text?: string; bytes?: Uint8Array }
  | { kind: 'diff'; entry: MaterialEntry; preview: PreviewVerdict; hunks: DiffHunk[]; malformed: boolean; raw?: string }
  | { kind: 'evidence'; citations: EvidenceCitation[] }
  | { kind: 'terminal'; lines: TerminalLine[]; nextCursor?: number; readOnly: true };
export type MaterialIntent =
  | { kind: 'download'; runId: string; materialId: string }
  | { kind: 'share'; runId: string; materialId: string }
  | { kind: 'terminal-input'; text: string };
export type MaterialActResult =
  | { kind: 'grant'; materialId: string; url: string; expiresAt: string }
  | { kind: 'shared'; materialId: string };
export interface MaterialEvent { type: 'grant-expired' | 'scope-closed'; materialId?: string; reason?: string }
export interface TaskMaterialHandle {
  index(input: { runId: string }): Promise<MaterialIndex>;
  open(ref: MaterialRef): Promise<MaterialView>;
  act(intent: MaterialIntent): Promise<MaterialActResult>;
  subscribe(listener: (event: MaterialEvent) => void): () => void;
  close(reason: string): void;
}
export interface TaskMaterial { open(input: { lease: ScopeLease }): TaskMaterialHandle }
```

- [ ] **Step 1: 写失败测试**

创建 `packages/mobile-core/src/material/in-memory-material-remote.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createRecordingSharePort, createScenarioMaterialRemote, createScriptedBlobFetch } from './in-memory-material-remote.ts';

const artifact = { id: 'msg-1:0', index: 0, name: 'report.md', mime: 'text/markdown', version: 'aaaaaaaaaaaaaaaa', size: 12, sourceRun: 'run-1' };

test('the scenario remote records calls and serves scripted answers', async () => {
  const remote = createScenarioMaterialRemote({
    list: () => ({ runId: 'run-1', artifacts: [artifact], terminalAvailable: true }),
    signedUrl: ({ index }) => ({ url: `https://weknora.example.test/api/v1/workbench/artifacts/download?index=${index}`, expiresAt: '2026-09-24T01:00:00Z', artifact }),
    terminalLog: ({ after }) => ({ lines: [{ seq: after + 1, occurredAt: '2026-09-24T00:00:01Z', stream: 'stdout', text: '$ ls\n' }], nextCursor: after + 1 }),
    events: () => [{ seq: 5, type: 'tool.started', occurredAt: '2026-09-24T00:00:05Z', payload: { tool: 'shell' } }],
  });
  const list = await remote.list('run-1');
  assert.equal(list.terminalAvailable, true);
  const grant = await remote.signedUrl({ runId: 'run-1', index: 0 });
  assert.match(grant.url, /index=0$/);
  const page = await remote.terminalLog({ runId: 'run-1', after: 0, limit: 200 });
  assert.equal(page.lines.length, 1);
  assert.equal((await remote.events('run-1'))[0]!.type, 'tool.started');
  assert.deepEqual(remote.listCalls, ['run-1']);
  assert.deepEqual(remote.grantCalls, [{ runId: 'run-1', index: 0 }]);
  assert.deepEqual(remote.terminalCalls, [{ runId: 'run-1', after: 0, limit: 200 }]);
  assert.deepEqual(remote.eventCalls, ['run-1']);
});

test('unscripted calls fail loudly instead of answering empty', async () => {
  const remote = createScenarioMaterialRemote({});
  await assert.rejects(() => remote.list('run-1'), /not scripted/);
});

test('the scripted blob fetch and recording share port observe usage', async () => {
  const blob = createScriptedBlobFetch({ 'https://weknora.example.test/blob': { bytes: new Uint8Array([104, 105]), mime: 'text/plain' } });
  const fetched = await blob.fetch('https://weknora.example.test/blob');
  assert.equal(new TextDecoder().decode(fetched.bytes), 'hi');
  await assert.rejects(() => blob.fetch('https://weknora.example.test/missing'), /not scripted/);
  assert.deepEqual(blob.calls, ['https://weknora.example.test/blob', 'https://weknora.example.test/missing']);

  const share = createRecordingSharePort();
  await share.share({ url: 'https://weknora.example.test/blob', name: 'report.md' });
  assert.deepEqual(share.shared, [{ url: 'https://weknora.example.test/blob', name: 'report.md' }]);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/material/in-memory-material-remote.test.ts`
Expected: FAIL——`Cannot find module './in-memory-material-remote.ts'`。

- [ ] **Step 3: 最小实现**

创建 `packages/mobile-core/src/material/material-errors.ts`：

```ts
/**
 * Task Material 共享错误合同（独立于实现文件，避免循环导入；镜像
 * task-office-errors.ts 的模式）。message 即裸错误码，呈现层查文案表。
 */
export type MaterialErrorCode =
  | 'MATERIAL_SCOPE_CHANGED'
  | 'MATERIAL_CLOSED'
  | 'MATERIAL_INVALID_INPUT'
  | 'MATERIAL_NOT_FOUND'
  | 'MATERIAL_GRANT_EXPIRED'
  | 'MATERIAL_GRANT_INVALID'
  | 'MATERIAL_SIGNING_DISABLED'
  | 'MATERIAL_TERMINAL_READ_ONLY'
  | 'MATERIAL_GRANT_ORIGIN'
  | 'MATERIAL_SHARE_UNAVAILABLE'
  | 'MATERIAL_BACKEND';

export class MaterialError extends Error {
  constructor(readonly code: MaterialErrorCode, options?: { cause?: unknown }) {
    super(code, options);
    this.name = 'MaterialError';
  }
}
```

创建 `packages/mobile-core/src/material/ports.ts`：

```ts
import type {
  EvidenceCitation, MaterialActResult, MaterialEntry, MaterialEntryKind, MaterialEvent, MaterialIndex,
  MaterialIntent, MaterialRef, MaterialView, PreviewVerdict, TaskMaterialHandle, TerminalLine,
} from './types.ts';

// ── re-export：句柄与视图类型与 Port 同文件可见，方便组合根单点导入 ──
export type {
  EvidenceCitation, MaterialActResult, MaterialEntry, MaterialEntryKind, MaterialEvent, MaterialIndex,
  MaterialIntent, MaterialRef, MaterialView, PreviewVerdict, TaskMaterialHandle, TerminalLine,
};

/** wire 行（api-client Adapter 产出）：消息绑定工件 + 版本身份 + 终端可用性。 */
export interface MaterialBackendArtifact {
  id: string;
  index: number;
  name: string;
  mime: string;
  version: string;
  size: number;
  sourceRun: string;
  createdAt?: string;
  digest?: string;
}

export interface MaterialBackendList {
  runId: string;
  artifacts: MaterialBackendArtifact[];
  terminalAvailable: boolean;
}

export interface MaterialBackendGrant {
  url: string;
  expiresAt: string;
  artifact: MaterialBackendArtifact;
}

export interface MaterialBackendTerminalLine {
  seq: number;
  occurredAt: string;
  stream: 'stdout' | 'stderr';
  text: string;
}

export interface MaterialBackendTerminalPage {
  lines: MaterialBackendTerminalLine[];
  nextCursor: number;
}

export interface MaterialBackendEvent {
  seq: number;
  type: string;
  occurredAt: string;
  payload: Record<string, unknown>;
}

/** Material Backend Port（module-seams §7.3）：WeKnora artifact/terminal-log/events Adapter 或 in-memory 场景 Adapter。 */
export interface MaterialBackendPort {
  list(runId: string): Promise<MaterialBackendList>;
  signedUrl(input: { runId: string; index: number }): Promise<MaterialBackendGrant>;
  terminalLog(input: { runId: string; after: number; limit: number }): Promise<MaterialBackendTerminalPage>;
  events(runId: string): Promise<MaterialBackendEvent[]>;
}

/** 免凭据字节抓取（签名链接的兑现通道）。Adapter 必须只接受 http/https。 */
export interface BlobFetchPort {
  fetch(url: string): Promise<{ bytes: Uint8Array; mime: string }>;
}

/** 系统分享 seam（module-seams §7.3 Preview / Share Port）：native Adapter 或 scripted test Adapter。 */
export interface SharePort {
  share(input: { url: string; name: string }): Promise<void>;
}

export interface TaskMaterialPorts {
  remote: MaterialBackendPort;
  blob: BlobFetchPort;
  /** 缺省 = 无系统分享通道：act(share) fail closed（MATERIAL_SHARE_UNAVAILABLE）。 */
  share?: SharePort;
}
```

（注：`types.ts` 需在文件顶部加 `import type { ScopeLease } from '../runtime/types.ts';` 与 `import type { DiffHunk } from './diff.ts';`——分别供 `TaskMaterial` 与 diff 视图变体引用；均为纯类型导入，不产生运行时依赖。）

在 `packages/mobile-core/src/material/types.ts` 追加（保留 Task 4 已有内容）：

```ts
import type { ScopeLease } from '../runtime/types.ts';
import type { DiffHunk } from './diff.ts';

/** （Task 4 已定义 MaterialEntryKind / PreviewVerdict / EvidenceCitation） */

export interface MaterialEntry {
  materialId: string;
  index: number;
  kind: MaterialEntryKind;
  name: string;
  mime: string;
  size: number;
  version: string;
  sourceRun: string;
  createdAt?: string;
  digest?: string;
}

export interface MaterialIndex {
  runId: string;
  materials: MaterialEntry[];
  terminal: { available: boolean };
}

export type MaterialRef =
  | { kind: MaterialEntryKind; runId: string; materialId: string }
  | { kind: 'evidence'; runId: string }
  | { kind: 'terminal'; runId: string; cursor?: number };

export interface TerminalLine {
  seq: number;
  occurredAt: string;
  stream: 'stdout' | 'stderr';
  text: string;
}

export type MaterialView =
  | { kind: 'artifact' | 'test-report'; entry: MaterialEntry; preview: PreviewVerdict; text?: string; bytes?: Uint8Array }
  | { kind: 'diff'; entry: MaterialEntry; preview: PreviewVerdict; hunks: DiffHunk[]; malformed: boolean; raw?: string }
  | { kind: 'evidence'; citations: EvidenceCitation[] }
  | { kind: 'terminal'; lines: TerminalLine[]; nextCursor?: number; readOnly: true };

export type MaterialIntent =
  | { kind: 'download'; runId: string; materialId: string }
  | { kind: 'share'; runId: string; materialId: string }
  | { kind: 'terminal-input'; text: string };

export type MaterialActResult =
  | { kind: 'grant'; materialId: string; url: string; expiresAt: string }
  | { kind: 'shared'; materialId: string };

export interface MaterialEvent {
  type: 'grant-expired' | 'scope-closed';
  materialId?: string;
  reason?: string;
}

export interface TaskMaterialHandle {
  index(input: { runId: string }): Promise<MaterialIndex>;
  open(ref: MaterialRef): Promise<MaterialView>;
  act(intent: MaterialIntent): Promise<MaterialActResult>;
  subscribe(listener: (event: MaterialEvent) => void): () => void;
  close(reason: string): void;
}

export interface TaskMaterial {
  open(input: { lease: ScopeLease }): TaskMaterialHandle;
}
```

创建 `packages/mobile-core/src/material/in-memory-material-remote.ts`：

```ts
import type { MaterialBackendPort, SharePort, BlobFetchPort, MaterialBackendList, MaterialBackendGrant, MaterialBackendTerminalPage, MaterialBackendEvent } from './ports.ts';

export interface ScenarioMaterialHandlers {
  list?(runId: string): Promise<MaterialBackendList> | MaterialBackendList;
  signedUrl?(input: { runId: string; index: number }): Promise<MaterialBackendGrant> | MaterialBackendGrant;
  terminalLog?(input: { runId: string; after: number; limit: number }): Promise<MaterialBackendTerminalPage> | MaterialBackendTerminalPage;
  events?(runId: string): Promise<MaterialBackendEvent[]> | MaterialBackendEvent[];
}

/** in-memory 场景 Adapter（module-seams §12「remote but owned」的测试面）；未编排的调用大声失败，不静默答空。 */
export function createScenarioMaterialRemote(handlers: ScenarioMaterialHandlers = {}): MaterialBackendPort & {
  listCalls: string[];
  grantCalls: Array<{ runId: string; index: number }>;
  terminalCalls: Array<{ runId: string; after: number; limit: number }>;
  eventCalls: string[];
} {
  const listCalls: string[] = [];
  const grantCalls: Array<{ runId: string; index: number }> = [];
  const terminalCalls: Array<{ runId: string; after: number; limit: number }> = [];
  const eventCalls: string[] = [];
  return {
    listCalls, grantCalls, terminalCalls, eventCalls,
    async list(runId) {
      listCalls.push(runId);
      if (handlers.list === undefined) throw new Error(`scenario material list not scripted for ${runId}`);
      return handlers.list(runId);
    },
    async signedUrl(input) {
      grantCalls.push(input);
      if (handlers.signedUrl === undefined) throw new Error(`scenario material grant not scripted for ${input.runId}#${input.index}`);
      return handlers.signedUrl(input);
    },
    async terminalLog(input) {
      terminalCalls.push(input);
      if (handlers.terminalLog === undefined) throw new Error(`scenario terminal log not scripted for ${input.runId}`);
      return handlers.terminalLog(input);
    },
    async events(runId) {
      eventCalls.push(runId);
      if (handlers.events === undefined) throw new Error(`scenario material events not scripted for ${runId}`);
      return handlers.events(runId);
    },
  };
}

/** 按完整 URL 编排的字节源：未编排的 URL 拒绝（不编造字节）。 */
export function createScriptedBlobFetch(script: Record<string, { bytes: Uint8Array; mime: string } | { error: unknown }>): BlobFetchPort & { calls: string[] } {
  const calls: string[] = [];
  return {
    calls,
    async fetch(url) {
      calls.push(url);
      const entry = script[url];
      if (entry === undefined) throw new Error(`scenario blob not scripted for ${url}`);
      if ('error' in entry) throw entry.error;
      return entry;
    },
  };
}

/** 记录型分享 Adapter（系统分享 seam 的 scripted test Adapter）。 */
export function createRecordingSharePort(): SharePort & { shared: Array<{ url: string; name: string }> } {
  const shared: Array<{ url: string; name: string }> = [];
  return { shared, async share(input) { shared.push(input); } };
}
```

- [ ] **Step 4: 运行确认通过**

Run: `pnpm exec tsx --test packages/mobile-core/src/material/in-memory-material-remote.test.ts packages/mobile-core/src/material/material-kinds.test.ts packages/mobile-core/src/material/diff.test.ts packages/mobile-core/src/material/evidence.test.ts`
Expected: PASS（Task 4 回归仍绿）。

- [ ] **Step 5: REFACTOR 检查**

`types.ts` 与 `ports.ts` 之间的 re-export 关系收敛为一处（ports re-export types，反向不导出），避免同名双定义。无运行时循环导入（全部 `import type`）。

- [ ] **Step 6: 提交**

```bash
git add packages/mobile-core/src/material/material-errors.ts packages/mobile-core/src/material/ports.ts packages/mobile-core/src/material/in-memory-material-remote.ts packages/mobile-core/src/material/types.ts packages/mobile-core/src/material/in-memory-material-remote.test.ts
git commit -m "feat(mobile-core/material): material ports, error contract and in-memory scenario adapters"
```

---

### Task 6: mobile-core——`createTaskMaterial` 深模块（AC1/AC2 的 Interface 级证据）

**Files:**
- Create: `packages/mobile-core/src/material/task-material.ts`
- Test: `packages/mobile-core/src/material/task-material.test.ts`
- Modify: `packages/mobile-core/src/index.ts`（末尾追加 material 导出块）

**Interfaces:**
- Consumes: Task 4 的 `materialKindOf`/`previewVerdictOf`/`isInlineImageMime`/`parseUnifiedDiff`/`projectCitations`；Task 5 的 `MaterialError`/`TaskMaterialPorts` 及全部类型；`RuntimeScopeLease`/`leaseActive`/`leaseScopeOf`（`../runtime/scope-lease.ts`，包内直导）。
- Produces: `createTaskMaterial(ports: TaskMaterialPorts): TaskMaterial`——公共导出（`packages/mobile-core` index）。Task 8 的 composition 与控制器依赖它。行为合同（全部有测试钉住）：
  - `open({lease})` 绑定 scope；lease 撤销/句柄关闭后一切调用按 `MATERIAL_SCOPE_CHANGED`/`MATERIAL_CLOSED` 拒绝，在途结果丢弃；
  - `index`：kind/版本身份投影；失败清空旧索引不回填；
  - `open`（材料）：unsupported（mime/size）判定发生在任何网络调用之前（零 grant、零 blob）；supported 文本经「新鲜 grant → origin 钉住 → blob 抓取 → 版本键缓存」；diff 视图解析 hunk、malformed 回退 raw；图片以 bytes 上抛；
  - `open`（evidence/terminal）：引用投影 / 只读分页（`readOnly: true`）；
  - `act(download|share)`：每次新鲜铸造 grant（绝不复用 URL——AC1「签名 URL 不长期缓存」）；share 经 SharePort，无端口 fail closed；
  - `act(terminal-input)`：一律 `MATERIAL_TERMINAL_READ_ONLY`（AC2）；
  - 错误映射：501 `artifact_signing_disabled` → `MATERIAL_SIGNING_DISABLED`；401 `artifact_grant_expired` → `MATERIAL_GRANT_EXPIRED`（并通知 `grant-expired` 事件）；401 其他 → `MATERIAL_GRANT_INVALID`；grant URL origin ≠ 活动部署 → `MATERIAL_GRANT_ORIGIN`（blob 零调用）；其余 → `MATERIAL_BACKEND`。

- [ ] **Step 1: 写失败测试**

创建 `packages/mobile-core/src/material/task-material.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import { MaterialError } from './material-errors.ts';
import { createRecordingSharePort, createScenarioMaterialRemote, createScriptedBlobFetch } from './in-memory-material-remote.ts';
import { createTaskMaterial } from './task-material.ts';
import type { MaterialBackendArtifact, TaskMaterialPorts } from './ports.ts';

const SCOPE = { deploymentOrigin: 'https://weknora.example.test', userId: 'member-1', tenantId: 'tenant-1' };
const GRANT_URL = 'https://weknora.example.test/api/v1/workbench/artifacts/download?index=0';

function artifactRow(overrides: Partial<MaterialBackendArtifact> = {}): MaterialBackendArtifact {
  return { id: 'msg-1:0', index: 0, name: 'report.md', mime: 'text/markdown', version: 'aaaaaaaaaaaaaaaa', size: 12, sourceRun: 'run-1', ...overrides };
}

function openModule(script: {
  artifacts?: MaterialBackendArtifact[];
  terminalAvailable?: boolean;
  grantUrl?: string;
  grantError?: unknown;
  blobs?: Record<string, { bytes: Uint8Array; mime: string } | { error: unknown }>;
}) {
  const remote = createScenarioMaterialRemote({
    list: (runId) => ({ runId, artifacts: script.artifacts ?? [artifactRow()], terminalAvailable: script.terminalAvailable ?? true }),
    signedUrl: ({ index }) => {
      if (script.grantError !== undefined) throw script.grantError;
      return { url: script.grantUrl ?? GRANT_URL, expiresAt: '2026-09-24T01:00:00Z', artifact: artifactRow() };
    },
    terminalLog: ({ after }) => ({ lines: [{ seq: after + 1, occurredAt: '2026-09-24T00:00:01Z', stream: 'stdout', text: '$ ls\n' }], nextCursor: after + 1 }),
    events: () => [{ seq: 5, type: 'tool.started', occurredAt: '2026-09-24T00:00:05Z', payload: { tool: 'knowledge_search', knowledge_base_id: 'kb-7' } }],
  });
  const blob = createScriptedBlobFetch(script.blobs ?? { [GRANT_URL]: { bytes: new TextEncoder().encode('# report\n'), mime: 'text/markdown' } });
  const share = createRecordingSharePort();
  const ports: TaskMaterialPorts = { remote, blob, share };
  const lease = new RuntimeScopeLease(SCOPE);
  const handle = createTaskMaterial(ports).open({ lease: lease.asScopeLease() });
  return { handle, remote, blob, share, lease };
}

test('index projects kinds, immutable version identities and terminal availability', async () => {
  const { handle } = openModule({
    artifacts: [
      artifactRow(),
      artifactRow({ id: 'msg-1:1', index: 1, name: 'changes.diff', mime: 'text/x-diff', version: 'bbbbbbbbbbbbbbbb' }),
      artifactRow({ id: 'msg-2:0', index: 2, name: 'junit.xml', mime: 'application/xml', version: 'cccccccccccccccc' }),
    ],
  });
  const index = await handle.index({ runId: 'run-1' });
  assert.equal(index.terminal.available, true);
  assert.deepEqual(index.materials.map((entry) => entry.kind), ['artifact', 'diff', 'test-report']);
  assert.ok(index.materials.every((entry) => entry.version !== ''), '每项材料都有非空不可变版本身份（AC1 的 Interface 面）');
});

test('open previewable text fetches a fresh grant, pins origin and caches bytes by version', async () => {
  const { handle, remote, blob } = openModule({});
  const first = await handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(first.kind, 'artifact');
  assert.equal(first.kind === 'artifact' && first.text, '# report\n');
  const second = await handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(second.kind === 'artifact' && second.text, '# report\n');
  assert.equal(remote.grantCalls.length, 1, '字节按 (materialId, version) 缓存——缓存命中不再铸造 grant');
  assert.equal(blob.calls.length, 1);
});

test('cached bytes are keyed by (materialId, version); a version change forces a fresh grant and fetch', async () => {
  // 同一句柄内：服务端对同一 materialId 发布新版本身份（内容变化 → 版本变化，AC1 的 Interface 面）。
  let version = 'aaaaaaaaaaaaaaaa';
  const remote = createScenarioMaterialRemote({
    list: (runId) => ({ runId, artifacts: [artifactRow({ version })], terminalAvailable: true }),
    signedUrl: ({ index }) => ({ url: GRANT_URL, expiresAt: '2026-09-24T01:00:00Z', artifact: artifactRow({ version, index }) }),
    terminalLog: ({ after }) => ({ lines: [], nextCursor: after }),
    events: () => [],
  });
  const blob = createScriptedBlobFetch({ [GRANT_URL]: { bytes: new TextEncoder().encode('# report\n'), mime: 'text/markdown' } });
  const lease = new RuntimeScopeLease(SCOPE);
  const handle = createTaskMaterial({ remote, blob, share: createRecordingSharePort() }).open({ lease: lease.asScopeLease() });

  const first = await handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(first.kind === 'artifact' && first.text, '# report\n');
  assert.equal(remote.grantCalls.length, 1);
  assert.equal(blob.calls.length, 1);

  version = 'dddddddddddddddd';
  await handle.index({ runId: 'run-1' }); // 重新载入索引 → 新版本身份进入条目
  const second = await handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(second.kind === 'artifact' && second.entry.version, 'dddddddddddddddd');
  assert.equal(remote.grantCalls.length, 2, '版本身份变化必须重新铸造 grant 并抓取（不得复用旧字节）');
  assert.equal(blob.calls.length, 2);
});

test('an unsupported mime or oversized material never mints a grant and never fetches', async () => {
  const oversized = openModule({ artifacts: [artifactRow({ name: 'big.csv', mime: 'text/csv', size: 3 * 1024 * 1024 })] });
  const bySize = await oversized.handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-1:0' });
  assert.deepEqual(bySize.kind === 'artifact' && bySize.preview, { state: 'unsupported', reason: 'size' });
  const byMime = openModule({ artifacts: [artifactRow({ name: 'bundle.zip', mime: 'application/zip', size: 5 })] });
  const view = await byMime.handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-1:0' });
  assert.deepEqual(view.kind === 'artifact' && view.preview, { state: 'unsupported', reason: 'mime' });
  assert.equal(bySize.remote.grantCalls.length + byMime.remote.grantCalls.length, 0, 'unsupported 判定先于一切网络调用（Review Focus 5）');
  assert.equal(bySize.blob.calls.length + byMime.blob.calls.length, 0);
});

test('open parses unified diff hunks and falls back to raw on malformed', async () => {
  const diffBytes = new TextEncoder().encode('--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n');
  const good = openModule({
    artifacts: [artifactRow({ name: 'changes.diff', mime: 'text/x-diff' })],
    blobs: { [GRANT_URL]: { bytes: diffBytes, mime: 'text/x-diff' } },
  });
  const view = await good.handle.open({ kind: 'diff', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(view.kind, 'diff');
  assert.equal(view.malformed, false);
  assert.deepEqual(view.hunks[0]!.lines.map((line) => line.origin), ['remove', 'add']);

  const broken = openModule({
    artifacts: [artifactRow({ name: 'weird.diff', mime: 'text/x-diff' })],
    blobs: { [GRANT_URL]: { bytes: new TextEncoder().encode('not a diff\n'), mime: 'text/x-diff' } },
  });
  const raw = await broken.handle.open({ kind: 'diff', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(raw.kind, 'diff');
  assert.equal(raw.kind === 'diff' && raw.malformed, true);
  assert.equal(raw.kind === 'diff' && raw.raw, 'not a diff\n');
});

test('open evidence projects citations and open terminal pages read-only lines', async () => {
  const { handle } = openModule({});
  const evidence = await handle.open({ kind: 'evidence', runId: 'run-1' });
  assert.equal(evidence.kind, 'evidence');
  assert.equal(evidence.citations[0]!.source, 'kb-7');

  const terminal = await handle.open({ kind: 'terminal', runId: 'run-1' });
  assert.equal(terminal.kind === 'terminal' && terminal.readOnly, true, '终端视图恒只读（AC2）');
  assert.equal(terminal.kind === 'terminal' && terminal.lines.length, 1);
  const page2 = await handle.open({ kind: 'terminal', runId: 'run-1', cursor: terminal.kind === 'terminal' ? terminal.nextCursor : 0 });
  assert.equal(page2.kind === 'terminal' && page2.lines.length, 1, '从 nextCursor 续页');
});

test('act download mints a fresh grant every time and act share goes through the share port', async () => {
  const { handle, remote, share } = openModule({});
  const one = await handle.act({ kind: 'download', runId: 'run-1', materialId: 'msg-1:0' });
  const two = await handle.act({ kind: 'download', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(one.kind === 'grant' && one.url, GRANT_URL);
  assert.equal(one.kind === 'grant' && one.expiresAt, '2026-09-24T01:00:00Z');
  assert.equal(remote.grantCalls.length, 2, '每次 act 都新鲜铸造——签名 URL 绝不缓存复用（AC1）');
  const shared = await handle.act({ kind: 'share', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(shared.kind, 'shared');
  assert.deepEqual(share.shared, [{ url: GRANT_URL, name: 'report.md' }]);
  assert.equal(remote.grantCalls.length, 3, '分享同样走新鲜 grant');
});

test('act terminal-input is always rejected with MATERIAL_TERMINAL_READ_ONLY (AC2)', async () => {
  const { handle } = openModule({});
  await assert.rejects(
    () => handle.act({ kind: 'terminal-input', text: 'rm -rf /' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_TERMINAL_READ_ONLY',
  );
});

test('a grant that expired during a blob fetch maps to MATERIAL_GRANT_EXPIRED and notifies subscribers (AC2)', async () => {
  const expired = new Error('401') as Error & { status?: number; code?: string };
  expired.status = 401;
  expired.code = 'artifact_grant_expired';
  const { handle } = openModule({ blobs: { [GRANT_URL]: { error: expired } } });
  const events: Array<{ type: string; materialId?: string }> = [];
  handle.subscribe((event) => events.push(event));
  await assert.rejects(
    () => handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-1:0' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_GRANT_EXPIRED',
  );
  assert.deepEqual(events, [{ type: 'grant-expired', materialId: 'msg-1:0' }], '订阅者收到失效事件以驱动重新授权');
});

test('signing disabled and invalid grants map to dedicated codes', async () => {
  const disabled = new Error('501') as Error & { status?: number; code?: string };
  disabled.status = 501;
  disabled.code = 'artifact_signing_disabled';
  const off = openModule({ grantError: disabled });
  await assert.rejects(
    () => off.handle.act({ kind: 'download', runId: 'run-1', materialId: 'msg-1:0' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_SIGNING_DISABLED',
  );

  const invalid = new Error('401') as Error & { status?: number; code?: string };
  invalid.status = 401;
  invalid.code = 'artifact_grant_invalid';
  const tampered = openModule({ grantError: invalid });
  await assert.rejects(
    () => tampered.handle.act({ kind: 'download', runId: 'run-1', materialId: 'msg-1:0' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_GRANT_INVALID',
  );
});

test('a grant url whose origin differs from the active deployment fails closed before any fetch', async () => {
  const foreign = openModule({ grantUrl: 'https://evil.example.test/api/v1/workbench/artifacts/download?index=0' });
  await assert.rejects(
    () => foreign.handle.act({ kind: 'download', runId: 'run-1', materialId: 'msg-1:0' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_GRANT_ORIGIN',
  );
  assert.equal(foreign.blob.calls.length, 0, 'origin 不一致时零字节流量（Review Focus 2）');
});

test('a revoked lease rejects subsequent calls and drops late results', async () => {
  const { handle, lease } = openModule({});
  lease.revoke();
  await assert.rejects(
    () => handle.index({ runId: 'run-1' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_SCOPE_CHANGED',
  );
  await assert.rejects(
    () => handle.act({ kind: 'download', runId: 'run-1', materialId: 'msg-1:0' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_SCOPE_CHANGED',
  );
});

test('unknown material ids answer MATERIAL_NOT_FOUND and close answers MATERIAL_CLOSED', async () => {
  const { handle } = openModule({});
  await assert.rejects(
    () => handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-9:9' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_NOT_FOUND',
  );
  handle.close('test-done');
  await assert.rejects(
    () => handle.index({ runId: 'run-1' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_CLOSED',
  );
});

test('a failed index load clears the previous projection instead of serving stale entries', async () => {
  const remote = createScenarioMaterialRemote({
    list: (() => {
      let failed = false;
      return (runId: string) => {
        if (failed) throw new Error('backend down');
        failed = true;
        return { runId, artifacts: [artifactRow()], terminalAvailable: true };
      };
    })(),
  });
  const lease = new RuntimeScopeLease(SCOPE);
  const handle = createTaskMaterial({ remote, blob: createScriptedBlobFetch({}), share: createRecordingSharePort() }).open({ lease: lease.asScopeLease() });
  const first = await handle.index({ runId: 'run-1' });
  assert.equal(first.materials.length, 1);
  await assert.rejects(() => handle.index({ runId: 'run-1' }), /MATERIAL_BACKEND/);
});
```

（实现说明：`openModule` 的 scenario handler 均为同步返回；`let version` 闭包使「同一句柄内服务端换版本身份」可被测试驱动。判别联合的收窄用 `view.kind === 'x' && view.field` 表达式保持，断言语义不变。）

- [ ] **Step 2: 运行确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/material/task-material.test.ts`
Expected: FAIL——`Cannot find module './task-material.ts'`。

- [ ] **Step 3: 最小实现**

创建 `packages/mobile-core/src/material/task-material.ts`：

```ts
import { leaseActive, leaseScopeOf } from '../runtime/scope-lease.ts';
import { MaterialError } from './material-errors.ts';
import { isInlineImageMime, materialKindOf, previewVerdictOf } from './material-kinds.ts';
import { parseUnifiedDiff } from './diff.ts';
import { projectCitations } from './evidence.ts';
import type {
  MaterialBackendArtifact, MaterialBackendGrant, MaterialBackendPort, TaskMaterialPorts,
} from './ports.ts';
import type {
  MaterialActResult, MaterialEntry, MaterialEvent, MaterialIndex, MaterialIntent, MaterialRef,
  MaterialView, TaskMaterial, TaskMaterialHandle,
} from './types.ts';

const TERMINAL_PAGE_LIMIT = 200;

/** 结构化读取（不读 message 字面量）：与 shelf 的 httpStatus 同一纪律。 */
function errorStatus(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null) return undefined;
  const status = (error as { status?: unknown }).status;
  return typeof status === 'number' ? status : undefined;
}

function errorCode(error: unknown): string | undefined {
  if (typeof error !== 'object' || error === null) return undefined;
  const code = (error as { code?: unknown }).code;
  return typeof code === 'string' && code !== '' ? code : undefined;
}

function decodeUtf8(bytes: Uint8Array): string {
  if (typeof TextDecoder !== 'undefined') return new TextDecoder('utf-8').decode(bytes);
  let out = '';
  const CHUNK = 0x8000;
  for (let index = 0; index < bytes.length; index += CHUNK) {
    out += String.fromCharCode(...bytes.subarray(index, index + CHUNK));
  }
  return out;
}

function toEntry(row: MaterialBackendArtifact): MaterialEntry {
  const version = typeof row.version === 'string' && row.version.trim() !== '' ? row.version : row.id;
  return {
    materialId: row.id,
    index: row.index,
    kind: materialKindOf(row.name, row.mime),
    name: row.name,
    mime: row.mime,
    size: row.size,
    version,
    sourceRun: row.sourceRun,
    ...(row.createdAt === undefined || row.createdAt === '' ? {} : { createdAt: row.createdAt }),
    ...(row.digest === undefined || row.digest === '' ? {} : { digest: row.digest }),
  };
}

/**
 * Task Material 深模块（module-seams §7）。MIME/大小判定、签名 grant 铸造纪律
 * （每次新鲜铸造、origin 钉住活动部署、TTL 内不复用 URL）、版本键字节缓存、
 * diff 解析、终端日志分页与系统分享全部藏在句柄后（§7.2）。
 * 批注/基于版本请求修改属 #47，不在本模块意图集内。
 */
export function createTaskMaterial(ports: TaskMaterialPorts): TaskMaterial {
  return {
    open({ lease }) {
      const scope = leaseScopeOf(lease);
      if (!scope || !leaseActive(lease)) throw new MaterialError('MATERIAL_SCOPE_CHANGED');
      const listeners = new Set<(event: MaterialEvent) => void>();
      const blobs = new Map<string, Uint8Array>(); // key: `${materialId}@${version}`
      let closed = false;
      let lastIndex: MaterialIndex | undefined;

      const notify = (event: MaterialEvent): void => {
        for (const listener of [...listeners]) listener(event);
      };
      const guard = (): void => {
        if (closed) throw new MaterialError('MATERIAL_CLOSED');
        if (!leaseActive(lease)) throw new MaterialError('MATERIAL_SCOPE_CHANGED');
      };
      const mapError = (error: unknown): MaterialError => {
        if (error instanceof MaterialError) return error;
        const status = errorStatus(error);
        const code = errorCode(error);
        if (status === 501 && code === 'artifact_signing_disabled') return new MaterialError('MATERIAL_SIGNING_DISABLED', { cause: error });
        if (status === 401 && code === 'artifact_grant_expired') return new MaterialError('MATERIAL_GRANT_EXPIRED', { cause: error });
        if (status === 401) return new MaterialError('MATERIAL_GRANT_INVALID', { cause: error });
        return new MaterialError('MATERIAL_BACKEND', { cause: error });
      };
      const callRemote = async <T>(action: () => Promise<T>): Promise<T> => {
        try {
          return await action();
        } catch (error) {
          throw mapError(error);
        }
      };
      /** grant 铸造 + origin 钉住：URL 只在当次调用内存活，永不缓存（AC1）。 */
      const mintGrant = async (runId: string, entry: MaterialEntry): Promise<MaterialBackendGrant> => {
        const grant = await callRemote(() => ports.remote.signedUrl({ runId, index: entry.index }));
        let origin: string;
        try {
          origin = new URL(grant.url).origin;
        } catch {
          throw new MaterialError('MATERIAL_GRANT_ORIGIN', { cause: new Error(`unparsable grant url: ${grant.url}`) });
        }
        if (origin !== scope.deploymentOrigin) {
          throw new MaterialError('MATERIAL_GRANT_ORIGIN', { cause: new Error(`grant origin ${origin} is not the active deployment`) });
        }
        return grant;
      };
      const fetchBytes = async (runId: string, entry: MaterialEntry): Promise<Uint8Array> => {
        const cacheKey = `${entry.materialId}@${entry.version}`;
        const cached = blobs.get(cacheKey);
        if (cached !== undefined) return cached;
        const grant = await mintGrant(runId, entry);
        let fetched: { bytes: Uint8Array; mime: string };
        try {
          fetched = await ports.blob.fetch(grant.url);
        } catch (error) {
          const mapped = mapError(error);
          if (mapped.code === 'MATERIAL_GRANT_EXPIRED') notify({ type: 'grant-expired', materialId: entry.materialId });
          throw mapped;
        }
        blobs.set(cacheKey, fetched.bytes);
        return fetched.bytes;
      };
      const entryFor = async (runId: string, materialId: string): Promise<MaterialEntry> => {
        let index = lastIndex;
        if (index === undefined || index.runId !== runId) index = await loadIndex(runId);
        const entry = index.materials.find((candidate) => candidate.materialId === materialId);
        if (entry === undefined) throw new MaterialError('MATERIAL_NOT_FOUND');
        return entry;
      };
      const loadIndex = async (runId: string): Promise<MaterialIndex> => {
        const list = await callRemote(() => ports.remote.list(runId));
        guard();
        const next: MaterialIndex = { runId, materials: list.artifacts.map(toEntry), terminal: { available: list.terminalAvailable } };
        lastIndex = next;
        return next;
      };

      const handle: TaskMaterialHandle = {
        async index(input): Promise<MaterialIndex> {
          guard();
          const runId = input.runId.trim();
          if (runId === '') throw new MaterialError('MATERIAL_INVALID_INPUT');
          try {
            return await loadIndex(runId);
          } catch (error) {
            // 失败清空旧投影：撤权/故障后不回填旧材料（AC 与 shelf 冻结规则同源）。
            lastIndex = undefined;
            throw error;
          }
        },
        async open(ref): Promise<MaterialView> {
          guard();
          const runId = ref.runId.trim();
          if (runId === '') throw new MaterialError('MATERIAL_INVALID_INPUT');
          if (ref.kind === 'evidence') {
            const events = await callRemote(() => ports.remote.events(runId));
            guard();
            return { kind: 'evidence', citations: projectCitations(events) };
          }
          if (ref.kind === 'terminal') {
            const after = typeof ref.cursor === 'number' && Number.isSafeInteger(ref.cursor) && ref.cursor >= 0 ? ref.cursor : 0;
            const page = await callRemote(() => ports.remote.terminalLog({ runId, after, limit: TERMINAL_PAGE_LIMIT }));
            guard();
            return {
              kind: 'terminal',
              lines: page.lines.map((line) => ({ seq: line.seq, occurredAt: line.occurredAt, stream: line.stream, text: line.text })),
              ...(page.nextCursor === undefined ? {} : { nextCursor: page.nextCursor }),
              readOnly: true,
            };
          }
          const materialId = ref.materialId.trim();
          if (materialId === '') throw new MaterialError('MATERIAL_INVALID_INPUT');
          const entry = await entryFor(runId, materialId);
          const verdict = previewVerdictOf(entry.mime, entry.size);
          if (verdict.state === 'unsupported') {
            // 判定先于一切网络调用（Review Focus 5）。
            return entry.kind === 'diff'
              ? { kind: 'diff', entry, preview: verdict, hunks: [], malformed: false }
              : { kind: entry.kind, entry, preview: verdict };
          }
          const bytes = await fetchBytes(runId, entry);
          guard();
          if (entry.kind === 'diff') {
            const raw = decodeUtf8(bytes);
            const parsed = parseUnifiedDiff(raw);
            return { kind: 'diff', entry, preview: verdict, hunks: parsed.hunks, malformed: parsed.malformed, raw };
          }
          if (isInlineImageMime(entry.mime)) return { kind: entry.kind, entry, preview: verdict, bytes };
          return { kind: entry.kind, entry, preview: verdict, text: decodeUtf8(bytes) };
        },
        async act(intent): Promise<MaterialActResult> {
          guard();
          if (intent.kind === 'terminal-input') {
            // 只读终端：任何输入尝试一律拒绝（AC2）；移动面不存在 PTY 写通道。
            throw new MaterialError('MATERIAL_TERMINAL_READ_ONLY');
          }
          const runId = intent.runId.trim();
          const materialId = intent.materialId.trim();
          if (runId === '' || materialId === '') throw new MaterialError('MATERIAL_INVALID_INPUT');
          const entry = await entryFor(runId, materialId);
          const grant = await mintGrant(runId, entry); // 每次 act 新鲜铸造，不复用任何旧 URL（AC1）
          guard();
          if (intent.kind === 'download') {
            return { kind: 'grant', materialId: entry.materialId, url: grant.url, expiresAt: grant.expiresAt };
          }
          if (ports.share === undefined) throw new MaterialError('MATERIAL_SHARE_UNAVAILABLE');
          try {
            await ports.share.share({ url: grant.url, name: entry.name });
          } catch (error) {
            throw mapError(error);
          }
          guard();
          return { kind: 'shared', materialId: entry.materialId };
        },
        subscribe(listener) {
          listeners.add(listener);
          return () => { listeners.delete(listener); };
        },
        close(reason) {
          if (closed) return;
          closed = true;
          blobs.clear();
          lastIndex = undefined;
          notify({ type: 'scope-closed', reason });
        },
      };
      return handle;
    },
  };
}
```

在 `packages/mobile-core/src/index.ts` 末尾追加：

```ts
export { createTaskMaterial } from './material/task-material.ts';
export { MaterialError } from './material/material-errors.ts';
export type { MaterialErrorCode } from './material/material-errors.ts';
export { materialKindOf, previewVerdictOf, isInlineImageMime, PREVIEW_MAX_BYTES } from './material/material-kinds.ts';
export { parseUnifiedDiff } from './material/diff.ts';
export type { DiffHunk, DiffLine } from './material/diff.ts';
export { projectCitations } from './material/evidence.ts';
export { createScenarioMaterialRemote, createScriptedBlobFetch, createRecordingSharePort } from './material/in-memory-material-remote.ts';
export type { ScenarioMaterialHandlers } from './material/in-memory-material-remote.ts';
export type {
  BlobFetchPort, MaterialBackendArtifact, MaterialBackendEvent, MaterialBackendGrant, MaterialBackendList,
  MaterialBackendPort, MaterialBackendTerminalLine, MaterialBackendTerminalPage, SharePort, TaskMaterialPorts,
} from './material/ports.ts';
export type {
  EvidenceCitation, MaterialActResult, MaterialEntry, MaterialEntryKind, MaterialEvent, MaterialIndex,
  MaterialIntent, MaterialRef, MaterialView, PreviewVerdict, TaskMaterial, TaskMaterialHandle, TerminalLine,
} from './material/types.ts';
```

- [ ] **Step 4: 运行确认通过**

Run: `pnpm exec tsx --test packages/mobile-core/src/material/*.test.ts`
Expected: PASS（Task 4/5/6 全部材料测试绿）。回归：`pnpm exec tsx --test packages/mobile-core/src/shelf/resource-shelf.test.ts packages/mobile-core/src/task-office/task-office.test.ts` 仍绿。

- [ ] **Step 5: REFACTOR 检查**

确认 `mintGrant` 同时被 `fetchBytes` 与 `act` 复用且无第二处 origin 检查；`mapError` 是唯一错误翻译点；`index.ts` 追加块不与既有导出重名（`MaterialError`/`PreviewVerdict` 等均首次导出）。

- [ ] **Step 6: 提交**

```bash
git add packages/mobile-core/src/material/task-material.ts packages/mobile-core/src/material/task-material.test.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core/material): Task Material deep module with grant discipline and read-only terminal"
```

---

### Task 7: api-client——`createMobileMaterialRemote` wire 适配

**Files:**
- Create: `packages/api-client/src/mobile/materials.ts`
- Test: `packages/api-client/src/mobile/materials.test.ts`
- Modify: `packages/api-client/package.json`（exports 块 `"./mobile/task-office"` 之后加 1 行）

**Interfaces:**
- Consumes: `ClientRequest`（`../client.ts:38-46`）与 `(input: ClientRequest) => Promise<unknown>` request 形态（`createTaskOfficeRemote` 同款——实参为 `MobileRuntime.authorizedRequest`，本适配器不新建传输、不持有 token）；Task 2/3 的 wire（artifacts 列表 + signed-url + terminal-log + 既有 snapshot events 端点）。
- Produces: `createMobileMaterialRemote(options: { origin: string; request: (input: ClientRequest) => Promise<unknown> }): MaterialRemote`，其中 `MaterialRemote` 结构与 mobile-core `MaterialBackendPort` **逐字一致**（`list/signedUrl/terminalLog/events`，结构可赋值由 apps/mobile typecheck 证明）：

```ts
export interface MaterialRemote {
  list(runId: string): Promise<{ runId: string; artifacts: Array<{ id: string; index: number; name: string; mime: string; version: string; size: number; sourceRun: string; createdAt?: string; digest?: string }>; terminalAvailable: boolean }>;
  signedUrl(input: { runId: string; index: number }): Promise<{ url: string; expiresAt: string; artifact: MaterialRemoteArtifact }>;
  terminalLog(input: { runId: string; after: number; limit: number }): Promise<{ lines: Array<{ seq: number; occurredAt: string; stream: 'stdout' | 'stderr'; text: string }>; nextCursor: number }>;
  events(runId: string): Promise<Array<{ seq: number; type: string; occurredAt: string; payload: Record<string, unknown> }>>;
}
```

- [ ] **Step 1: 写失败测试**

创建 `packages/api-client/src/mobile/materials.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import type { ClientRequest } from '../client.ts';
import { createMobileMaterialRemote } from './materials.ts';

const listData = {
  items: [
    { index: 0, id: 'msg-1:0', name: 'report.md', mime: 'text/markdown', version: 'aaaaaaaaaaaaaaaa', digest: 'aaaaaaaaaaaabbbbbbbbbbccccccccccccdddddddddddd', size: 12, source_run: 'run-1', created_at: '2026-09-24T00:00:00Z' },
    { index: 1, id: 'msg-1:1', name: 'changes.diff', mime: 'text/x-diff', version: 'msg-1:1', size: 40, source_run: 'run-1' },
  ],
  terminal: { available: true },
};

test('material remote maps the artifact list wire and degrades without the terminal flag', async () => {
  const remote = createMobileMaterialRemote({ origin: 'https://weknora.example.test', request: async () => ({ success: true, data: listData }) });
  const list = await remote.list('run-1');
  assert.equal(list.runId, 'run-1');
  assert.equal(list.terminalAvailable, true);
  assert.deepEqual(list.artifacts.map((row) => row.version), ['aaaaaaaaaaaaaaaa', 'msg-1:1']);
  assert.equal(list.artifacts[0]!.digest, 'aaaaaaaaaaaabbbbbbbbbbccccccccccccdddddddddddd');
  assert.equal('digest' in list.artifacts[1]!, false, '无 digest 的行不下发该字段');

  // 旧服务端：terminal 标志缺失 → false（能力降级，不猜成 true）。
  const legacy = createMobileMaterialRemote({ origin: 'https://weknora.example.test', request: async () => ({ success: true, data: { items: [] } }) });
  assert.equal((await legacy.list('run-1')).terminalAvailable, false);
});

test('material remote posts signed-url with the numeric index and maps the grant', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileMaterialRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return {
        success: true,
        data: {
          url: 'https://weknora.example.test/api/v1/workbench/artifacts/download?index=1',
          expires_at: '2026-09-24T01:00:00Z',
          artifact: listData.items[1],
        },
      };
    },
  });
  const grant = await remote.signedUrl({ runId: 'run-1', index: 1 });
  assert.equal(requests[0]!.method, 'POST');
  assert.equal(requests[0]!.path, '/api/v1/workbench/executions/run-1/artifacts/1/signed-url');
  assert.equal(grant.expiresAt, '2026-09-24T01:00:00Z');
  assert.equal(grant.artifact.id, 'msg-1:1');
  assert.equal(grant.artifact.version, 'msg-1:1');
});

test('material remote pages the terminal log with after/limit query facets', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileMaterialRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return { success: true, data: { lines: [{ seq: 3, occurred_at: '2026-09-24T00:00:03Z', stream: 'stdout', text: '$ ls\n' }], next_cursor: 3 } };
    },
  });
  const page = await remote.terminalLog({ runId: 'run-1', after: 0, limit: 200 });
  assert.equal(requests[0]!.method, 'GET');
  assert.equal(requests[0]!.path, '/api/v1/workbench/executions/run-1/terminal-log?after=0&limit=200');
  assert.equal(page.lines[0]!.stream, 'stdout');
  assert.equal(page.nextCursor, 3);
});

test('material remote projects evidence events from the run snapshot', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileMaterialRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return {
        success: true,
        data: {
          execution: { schema_version: 1, run_id: 'run-1', session_id: 'task-1', revision: 1, driver: 'platform', run_status: 'succeeded', execution_status: 'succeeded', settlement_status: 'settled', seq: 2, capabilities: {} },
          watermark: 5,
          incomplete: false,
          confirmed_watermark: 5,
          events: [
            { schema_version: 1, run_id: 'run-1', attempt_id: 'a1', seq: 5, type: 'tool.started', occurred_at: '2026-09-24T00:00:05Z', payload: { tool: 'shell' } },
          ],
        },
      };
    },
  });
  const events = await remote.events('run-1');
  assert.equal(requests[0]!.path, '/api/v1/workbench/executions/run-1/snapshot');
  assert.deepEqual(events, [{ seq: 5, type: 'tool.started', occurredAt: '2026-09-24T00:00:05Z', payload: { tool: 'shell' } }]);
});

test('material remote requires a credential-free absolute https origin', async () => {
  const request = async () => ({ success: true, data: { items: [] } });
  assert.throws(() => createMobileMaterialRemote({ origin: 'http://weknora.example.test', request }), /HTTPS/);
  assert.throws(() => createMobileMaterialRemote({ origin: 'https://user:pass@weknora.example.test', request }), /user info/);
  assert.throws(() => createMobileMaterialRemote({ origin: 'not-a-url', request }), /absolute URL/);
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/materials.test.ts`
Expected: FAIL——`Cannot find module './materials.ts'`。

- [ ] **Step 3: 最小实现**

创建 `packages/api-client/src/mobile/materials.ts`：

```ts
import type { ClientRequest } from '../client.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface MaterialRemoteOptions {
  /** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输、不持有 token。 */
  request: Request;
}

export interface MaterialRemoteArtifact {
  id: string; index: number; name: string; mime: string; version: string; size: number;
  sourceRun: string; createdAt?: string; digest?: string;
}
export interface MaterialRemoteList { runId: string; artifacts: MaterialRemoteArtifact[]; terminalAvailable: boolean }
export interface MaterialRemoteGrant { url: string; expiresAt: string; artifact: MaterialRemoteArtifact }
export interface MaterialRemoteTerminalLine { seq: number; occurredAt: string; stream: 'stdout' | 'stderr'; text: string }
export interface MaterialRemoteTerminalPage { lines: MaterialRemoteTerminalLine[]; nextCursor: number }
export interface MaterialRemoteEvent { seq: number; type: string; occurredAt: string; payload: Record<string, unknown> }

/** 与 mobile-core MaterialBackendPort 结构逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface MaterialRemote {
  list(runId: string): Promise<MaterialRemoteList>;
  signedUrl(input: { runId: string; index: number }): Promise<MaterialRemoteGrant>;
  terminalLog(input: { runId: string; after: number; limit: number }): Promise<MaterialRemoteTerminalPage>;
  events(runId: string): Promise<MaterialRemoteEvent[]>;
}

function requireDeploymentOrigin(origin: string): string {
  let parsed: URL;
  if (typeof origin !== 'string' || origin.trim() === '') throw new Error('deployment origin is required');
  try { parsed = new URL(origin); } catch { throw new Error(`deployment origin must be an absolute URL: ${origin}`); }
  if (parsed.protocol !== 'https:') throw new Error('deployment origin must use HTTPS');
  if (parsed.username !== '' || parsed.password !== '') throw new Error('deployment origin must not embed user info');
  if (parsed.pathname !== '/') throw new Error('deployment origin must not include a path');
  if (parsed.search !== '' || parsed.hash !== '') throw new Error('deployment origin must not include a query or fragment');
  return parsed.origin;
}

function unwrap(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error('material response must be a success envelope');
  const envelope = value as { success?: unknown; data?: unknown };
  if (envelope.success !== true || !Object.prototype.hasOwnProperty.call(envelope, 'data')) {
    throw new Error('material response.success must be true with data');
  }
  const data = envelope.data;
  if (typeof data !== 'object' || data === null || Array.isArray(data)) throw new Error('material response data must be an object');
  return data as Record<string, unknown>;
}

function stringRow(row: unknown, at: string): Record<string, unknown> {
  if (typeof row !== 'object' || row === null) throw new Error(`${at} must be an object`);
  return row as Record<string, unknown>;
}

function optionalString(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined;
}

export function createMobileMaterialRemote(options: MaterialRemoteOptions): MaterialRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;

  const artifactFrom = (row: unknown): MaterialRemoteArtifact => {
    const r = stringRow(row, 'artifact');
    return {
      id: String(r.id ?? ''),
      index: typeof r.index === 'number' ? r.index : Number(r.index ?? 0),
      name: String(r.name ?? ''),
      mime: String(r.mime ?? ''),
      version: String(r.version ?? ''),
      size: typeof r.size === 'number' ? r.size : Number(r.size ?? 0),
      sourceRun: String(r.source_run ?? ''),
      ...(optionalString(r.created_at) === undefined ? {} : { createdAt: optionalString(r.created_at) }),
      ...(optionalString(r.digest) === undefined ? {} : { digest: optionalString(r.digest) }),
    };
  };

  return {
    async list(runId) {
      const data = unwrap(await request({ method: 'GET', path: `/api/v1/workbench/executions/${encodeURIComponent(runId)}/artifacts` }));
      const items = Array.isArray(data.items) ? data.items : [];
      const terminal = typeof data.terminal === 'object' && data.terminal !== null
        ? (data.terminal as { available?: unknown }).available === true
        : false;
      return { runId, artifacts: items.map(artifactFrom), terminalAvailable: terminal };
    },
    async signedUrl({ runId, index }) {
      const data = unwrap(await request({ method: 'POST', path: `/api/v1/workbench/executions/${encodeURIComponent(runId)}/artifacts/${index}/signed-url` }));
      return {
        url: String(data.url ?? ''),
        expiresAt: String(data.expires_at ?? ''),
        artifact: artifactFrom(data.artifact),
      };
    },
    async terminalLog({ runId, after, limit }) {
      const data = unwrap(await request({
        method: 'GET',
        path: `/api/v1/workbench/executions/${encodeURIComponent(runId)}/terminal-log?after=${after}&limit=${limit}`,
      }));
      const lines = Array.isArray(data.lines) ? data.lines : [];
      return {
        lines: lines.map((row) => {
          const r = stringRow(row, 'terminal line');
          const stream = r.stream === 'stderr' ? 'stderr' : 'stdout';
          return { seq: typeof r.seq === 'number' ? r.seq : Number(r.seq ?? 0), occurredAt: String(r.occurred_at ?? ''), stream, text: String(r.text ?? '') };
        }),
        nextCursor: typeof data.next_cursor === 'number' ? data.next_cursor : Number(data.next_cursor ?? 0),
      };
    },
    async events(runId) {
      // Evidence 引用复用既有 snapshot 端点（ADR-0006：REST 取权威 Snapshot）。
      const data = unwrap(await request({ method: 'GET', path: `/api/v1/workbench/executions/${encodeURIComponent(runId)}/snapshot` }));
      const events = Array.isArray(data.events) ? data.events : [];
      return events.map((row) => {
        const r = stringRow(row, 'event');
        return {
          seq: typeof r.seq === 'number' ? r.seq : Number(r.seq ?? 0),
          type: String(r.type ?? ''),
          occurredAt: String(r.occurred_at ?? ''),
          payload: (typeof r.payload === 'object' && r.payload !== null ? r.payload : {}) as Record<string, unknown>,
        };
      });
    },
  };
}
```

修改 `packages/api-client/package.json` exports 块（`"./mobile/task-office"` 之后加一行——插入点选在既有 mobile 子路径相邻处便于审阅；现状 exports 本就非字母序，不引入排序约定）：

```json
    "./mobile/materials": "./src/mobile/materials.ts",
```

- [ ] **Step 4: 运行确认通过**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/materials.test.ts`
Expected: PASS（5 个测试绿）。回归：`pnpm exec tsx --test packages/api-client/src/mobile/task-office.test.ts` 仍 9 pass。

- [ ] **Step 5: REFACTOR 检查**

`requireDeploymentOrigin`/`unwrap` 与 `task-office.ts` 中同名函数语义相同——刻意不跨文件抽取（该文件属并行批次共享面，复制 12 行优于触碰共享文件；两处行为有各自测试钉住）。确认 `list` 对 `terminal` 缺失的降级路径有测试。

- [ ] **Step 6: 提交**

```bash
git add packages/api-client/src/mobile/materials.ts packages/api-client/src/mobile/materials.test.ts packages/api-client/package.json
git commit -m "feat(api-client/mobile): material remote adapter over the authorized workbench wire"
```

---

### Task 8: apps/mobile——材料屏、控制器、路由与 composition 接线

**Files:**
- Create: `apps/mobile/src/materials-view.ts`
- Create: `apps/mobile/src/screens/MaterialsScreen.tsx`
- Create: `apps/mobile/src/app/tasks/materials.tsx`
- Create: `apps/mobile/src/adapters/material-adapters.ts`
- Test: `apps/mobile/src/materials-view.test.ts`
- Modify: `apps/mobile/src/composition.ts`（`activeTaskOffice` 之后追加 material 工厂与 `activeTaskMaterial`）
- Modify: `apps/mobile/src/screens/TaskDetailScreen.tsx`（props 加 `onOpenMaterials?: () => void` + 时间线之后 1 个按钮）
- Modify: `apps/mobile/src/app/tasks/detail.tsx`（`TaskDetailRouteLifecycle` 透传 `onOpenMaterials`；默认导出用 `router.push` 接线）
- Modify: `apps/mobile/src/app-smoke.test.tsx`（react-native stub 的导出表加 `Image: 'Image'`；文件末尾追加 2 个测试）

**Interfaces:**
- Consumes: Task 6 的 `createTaskMaterial`/`MaterialError`/`TaskMaterial`/`TaskMaterialHandle` 及全部类型（`@weknora/mobile-core`）；Task 7 的 `createMobileMaterialRemote`（`@weknora/api-client/mobile/materials`，仅在 composition.ts 出现——Screen 禁止导入 wire Adapter，module-seams §10）；`bytesToBase64`（`@weknora/mobile-core`，`vault/base64.ts`）；composition 的 `runtime()`/`activeMobileRuntime()`/`snapshot()` 范式；task-detail 路由的 lifecycle 宿主范式。
- Produces:
  - `createMaterialsController(handle: TaskMaterialHandle, input: { runId: string }): MaterialsController`（`state/subscribe/load/openMaterial/openTerminal/openEvidence/download/share/dispose`）与 `MATERIAL_ERROR_COPY`；
  - `MaterialsScreen`（props：`index/view/loading/error/grant/onOpenMaterial/onOpenTerminal/onOpenEvidence/onDownload/onShare/onRefresh/onBack`）；
  - `createFetchBlobAdapter(): BlobFetchPort` 与 `createNativeSharePortIfAvailable(): SharePort | undefined`（`apps/mobile/src/adapters/material-adapters.ts`）；
  - composition：`taskMaterialFor(runtime, origin): TaskMaterial`（按 origin 记忆化）与 `activeTaskMaterial(): TaskMaterial | undefined`；
  - 路由 `/tasks/materials?runId=..`。

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/materials-view.test.ts`（纯 TS 控制器测试——接口级，不经 RN；句柄用符合 `TaskMaterialHandle` 合同的测试替身，**不**导入 mobile-core 包私有的 `RuntimeScopeLease`（#32 约定不入公共导出），真实模块行为已在 Task 6 钉住）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { MaterialError } from '@weknora/mobile-core';
import type { MaterialIndex, MaterialRef, MaterialIntent, TaskMaterialHandle } from '@weknora/mobile-core';
import { createMaterialsController, MATERIAL_ERROR_COPY } from './materials-view.ts';

const GRANT_URL = 'https://weknora.example.test/api/v1/workbench/artifacts/download?index=0';

function fakeHandle(script: { grantError?: unknown } = {}): TaskMaterialHandle {
  const index: MaterialIndex = {
    runId: 'run-1',
    materials: [
      { materialId: 'msg-1:0', index: 0, kind: 'artifact', name: 'report.md', mime: 'text/markdown', size: 9, version: 'aaaaaaaaaaaaaaaa', sourceRun: 'run-1' },
      { materialId: 'msg-1:1', index: 1, kind: 'diff', name: 'changes.diff', mime: 'text/x-diff', size: 30, version: 'bbbbbbbbbbbbbbbb', sourceRun: 'run-1' },
    ],
    terminal: { available: true },
  };
  return {
    async index() { return index; },
    async open(ref: MaterialRef) {
      if (ref.kind === 'terminal') {
        return { kind: 'terminal', lines: [{ seq: 1, occurredAt: '2026-09-24T00:00:01Z', stream: 'stdout', text: '$ ls\n' }], nextCursor: 1, readOnly: true };
      }
      if (ref.kind === 'evidence') {
        return { kind: 'evidence', citations: [{ seq: 5, occurredAt: '2026-09-24T00:00:05Z', type: 'tool.started', detail: '{"tool":"shell"}' }] };
      }
      if (ref.materialId === 'msg-1:9') throw new MaterialError('MATERIAL_NOT_FOUND');
      const entry = index.materials[0]!;
      return { kind: 'artifact', entry, preview: { state: 'supported' }, text: '# report\n' };
    },
    async act(intent: MaterialIntent) {
      if (intent.kind === 'terminal-input') throw new MaterialError('MATERIAL_TERMINAL_READ_ONLY');
      // 模块合同（Task 6 mapError）：传输错误在模块边界翻译为 MaterialError 上抛——
      // 测试替身据此只抛 MaterialError，不模拟原始传输错误形态。
      if (script.grantError !== undefined) throw script.grantError;
      return { kind: 'grant', materialId: 'msg-1:0', url: GRANT_URL, expiresAt: '2026-09-24T01:00:00Z' };
    },
    subscribe() { return () => {}; },
    close() {},
  };
}

test('the controller loads the index, opens views and records grants', async () => {
  const controller = createMaterialsController(fakeHandle(), { runId: 'run-1' });
  await controller.load();
  assert.equal(controller.state().index?.materials.length, 2);
  await controller.openMaterial('msg-1:0');
  assert.equal(controller.state().view?.kind, 'artifact');
  await controller.openTerminal();
  assert.equal(controller.state().view?.kind, 'terminal');
  await controller.openEvidence();
  assert.equal(controller.state().view?.kind, 'evidence');
  await controller.download('msg-1:0');
  assert.equal(controller.state().grant?.url, GRANT_URL);
  assert.equal(controller.state().grant?.name, 'report.md');
  await controller.share('msg-1:0');
  assert.equal(controller.state().error, undefined);
});

test('material error codes map to user copy, never raw internals', async () => {
  assert.equal(MATERIAL_ERROR_COPY.MATERIAL_GRANT_EXPIRED, '下载授权已过期，请重新获取。');
  assert.equal(MATERIAL_ERROR_COPY.MATERIAL_TERMINAL_READ_ONLY, '终端为只读，不能输入。');
  // 501 artifact_signing_disabled 由模块 mapError 翻译为 MATERIAL_SIGNING_DISABLED；
  // 控制器只映射 MaterialError（messageOf），故替身按模块合同抛 MaterialError。
  const controller = createMaterialsController(fakeHandle({ grantError: new MaterialError('MATERIAL_SIGNING_DISABLED') }), { runId: 'run-1' });
  await controller.load();
  await controller.download('msg-1:0');
  assert.equal(controller.state().error, '此部署未配置签名密钥，暂无法下载（请联系管理员）。');
  await controller.openMaterial('msg-1:9');
  assert.equal(controller.state().error, '该材料已不存在，请刷新列表。');
});

test('dispose closes the handle exactly once', async () => {
  const controller = createMaterialsController(fakeHandle(), { runId: 'run-1' });
  await controller.load();
  controller.dispose();
  controller.dispose(); // 幂等
  assert.equal(controller.state().loading, false);
});
```

在 `apps/mobile/src/app-smoke.test.tsx` 末尾追加（并将 NATIVE_MODULE_STUBS 中 `'react-native'` 的导出表字符串补上 `Image: 'Image',`）：

```tsx
test('the task detail screen exposes the materials entry point', async () => {
  const screen = await import('./screens/TaskDetailScreen.tsx');
  const offline = screen.TaskDetailScreen({ view: undefined, loading: false, error: 'x', onRefresh: () => {}, onOpenMaterials: () => {} });
  assert.ok(JSON.stringify(offline).includes('重试'), 'offline fallback still renders');
  const view: import('@weknora/mobile-core').TaskDetailView = {
    taskId: 'task-1', runId: 'run-1', title: '报告', lifecycle: 'active', runStatus: 'succeeded', attention: 'none',
    executionStatus: 'succeeded', settlementStatus: 'settled', revision: 1, cursor: 2, incomplete: false, connection: 'drained',
    timeline: [], duplicateSeqs: [],
  };
  const withEntry = screen.TaskDetailScreen({ view, loading: false, onRefresh: () => {}, onOpenMaterials: () => {} });
  assert.ok(JSON.stringify(withEntry).includes('任务材料'), 'the materials entry renders when the callback is provided');
  const withoutEntry = screen.TaskDetailScreen({ view, loading: false, onRefresh: () => {} });
  assert.ok(!JSON.stringify(withoutEntry).includes('任务材料'), 'no entry without the callback (older callers compile unchanged)');
});

test('the materials screen and route consume the Task Material interface only', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(composition, /createTaskMaterial\(/, 'composition must instantiate the Task Material module');
  assert.match(composition, /createMobileMaterialRemote/, 'composition must bind the remote adapter to the module');
  for (const relative of ['screens/MaterialsScreen.tsx', 'materials-view.ts', 'app/tasks/materials.tsx']) {
    const source = readFileSync(join(here, relative), 'utf8');
    assert.equal(/@weknora\/(api-client|contracts)/.test(source), false, `${relative} must consume the Task Material Interface only`);
  }
  const route = await import('./app/tasks/materials.tsx');
  assert.equal(typeof route.default, 'function', 'src/app/tasks/materials.tsx must default-export the materials route');
  const screen = await import('./screens/MaterialsScreen.tsx');
  assert.equal(typeof screen.MaterialsScreen, 'function');
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——`Cannot find module './materials-view.ts'`（及新屏/路由缺失；app-smoke 新测试失败于文件不存在）。

- [ ] **Step 3: 最小实现**

创建 `apps/mobile/src/materials-view.ts`：

```ts
import { MaterialError } from '@weknora/mobile-core';
import type { MaterialActResult, MaterialIndex, MaterialView, TaskMaterialHandle } from '@weknora/mobile-core';

export interface MaterialsViewState {
  index?: MaterialIndex;
  view?: MaterialView;
  loading: boolean;
  error?: string;
  grant?: { name: string; url: string; expiresAt: string };
}

/** MaterialError 错误码 → 用户文案（message 即裸错误码）。 */
export const MATERIAL_ERROR_COPY: Record<string, string> = {
  MATERIAL_SCOPE_CHANGED: '登录状态或活动空间已变化，请重新进入。',
  MATERIAL_CLOSED: '材料视图已关闭。',
  MATERIAL_INVALID_INPUT: '材料参数缺失，请从任务详情重新进入。',
  MATERIAL_NOT_FOUND: '该材料已不存在，请刷新列表。',
  MATERIAL_GRANT_EXPIRED: '下载授权已过期，请重新获取。',
  MATERIAL_GRANT_INVALID: '下载授权校验失败，请重新获取。',
  MATERIAL_SIGNING_DISABLED: '此部署未配置签名密钥，暂无法下载（请联系管理员）。',
  MATERIAL_TERMINAL_READ_ONLY: '终端为只读，不能输入。',
  MATERIAL_GRANT_ORIGIN: '下载链接与当前部署不一致，已拒绝。',
  MATERIAL_SHARE_UNAVAILABLE: '此设备暂无系统分享通道。',
  MATERIAL_BACKEND: '服务端暂时不可用，请稍后重试。',
};

const messageOf = (failure: unknown): string => {
  if (failure instanceof MaterialError) return MATERIAL_ERROR_COPY[failure.code] ?? failure.code;
  return failure instanceof Error ? failure.message : String(failure);
};

export interface MaterialsController {
  state(): MaterialsViewState;
  subscribe(listener: (state: MaterialsViewState) => void): () => void;
  load(): Promise<void>;
  openMaterial(materialId: string): Promise<void>;
  openTerminal(): Promise<void>;
  openEvidence(): Promise<void>;
  download(materialId: string): Promise<void>;
  share(materialId: string): Promise<void>;
  dispose(): void;
}

/** 材料页控制器：load 驱动索引，open* 驱动视图，download/share 驱动 act；dispose 关闭句柄。 */
export function createMaterialsController(handle: TaskMaterialHandle, input: { runId: string }): MaterialsController {
  let state: MaterialsViewState = { loading: false };
  let disposed = false;
  const listeners = new Set<(state: MaterialsViewState) => void>();
  const publish = (next: MaterialsViewState): void => {
    state = next;
    for (const listener of [...listeners]) listener(state);
  };
  const run = async (action: () => Promise<MaterialsViewState>): Promise<void> => {
    publish({ ...state, loading: true, error: undefined });
    try {
      if (!disposed) publish(await action());
    } catch (failure) {
      if (!disposed) publish({ ...state, loading: false, error: messageOf(failure) });
    }
  };
  const applyGrant = (result: MaterialActResult): MaterialsViewState => {
    if (result.kind !== 'grant') return { ...state, loading: false };
    const entry = state.index?.materials.find((candidate) => candidate.materialId === result.materialId);
    return { ...state, loading: false, grant: { name: entry?.name ?? result.materialId, url: result.url, expiresAt: result.expiresAt } };
  };
  return {
    state: () => state,
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    load(): Promise<void> {
      return run(async () => ({ loading: false, view: undefined, grant: undefined, index: await handle.index({ runId: input.runId }) }));
    },
    openMaterial(materialId): Promise<void> {
      return run(async () => ({ ...state, loading: false, grant: undefined, view: await handle.open({ kind: 'artifact', runId: input.runId, materialId }) }));
    },
    openTerminal(): Promise<void> {
      return run(async () => ({ ...state, loading: false, grant: undefined, view: await handle.open({ kind: 'terminal', runId: input.runId }) }));
    },
    openEvidence(): Promise<void> {
      return run(async () => ({ ...state, loading: false, grant: undefined, view: await handle.open({ kind: 'evidence', runId: input.runId }) }));
    },
    download(materialId): Promise<void> {
      return run(async () => applyGrant(await handle.act({ kind: 'download', runId: input.runId, materialId })));
    },
    share(materialId): Promise<void> {
      return run(async () => applyGrant(await handle.act({ kind: 'share', runId: input.runId, materialId })));
    },
    dispose() {
      if (disposed) return;
      disposed = true;
      listeners.clear();
      handle.close('controller-disposed');
    },
  };
}
```

注意：`openMaterial` 传 `kind: 'artifact'` 是 MaterialRef 的判别值——模块内部按 entry 实际 kind（diff/test-report）投影视图，屏不预判类型。

创建 `apps/mobile/src/adapters/material-adapters.ts`：

```ts
import type { BlobFetchPort, SharePort } from '@weknora/mobile-core';

/** 免凭据字节抓取（签名链接兑现通道）：仅 http/https；非 2xx 以 {status, code} 形态拒绝（模块据此映射 GRANT_*）。 */
export function createFetchBlobAdapter(): BlobFetchPort {
  return {
    async fetch(url) {
      let parsed: URL;
      try { parsed = new URL(url); } catch { throw new Error(`blob url must be absolute: ${url}`); }
      if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') throw new Error('blob url must use http(s)');
      const response = await fetch(url);
      if (!response.ok) {
        let code = '';
        try { code = String(((await response.json()) as { code?: unknown }).code ?? ''); } catch { code = ''; }
        const error = new Error(`blob fetch failed: ${response.status}`) as Error & { status?: number; code?: string };
        error.status = response.status;
        error.code = code;
        throw error;
      }
      const bytes = new Uint8Array(await response.arrayBuffer());
      return { bytes, mime: response.headers.get('content-type') ?? 'application/octet-stream' };
    },
  };
}

/** 系统分享 native Adapter（module-seams §7.3 Preview / Share Port）。解析失败返回 undefined（fail closed）。 */
export function createNativeSharePortIfAvailable(): SharePort | undefined {
  try {
    const shareLike = (require('react-native') as { Share?: { share(input: { url?: string; message?: string }): Promise<unknown> } }).Share;
    if (shareLike === undefined) return undefined;
    return {
      async share({ url, name }) {
        await shareLike.share({ url, message: name });
      },
    };
  } catch {
    return undefined;
  }
}
```

创建 `apps/mobile/src/screens/MaterialsScreen.tsx`：

```tsx
import { Button, Image, ScrollView, Text, View } from 'react-native';
import { bytesToBase64 } from '@weknora/mobile-core';
import type { MaterialIndex, MaterialView } from '@weknora/mobile-core';

export interface MaterialsScreenProps {
  index?: MaterialIndex;
  view?: MaterialView;
  loading: boolean;
  error?: string;
  grant?: { name: string; url: string; expiresAt: string };
  onOpenMaterial(materialId: string): void;
  onOpenTerminal(): void;
  onOpenEvidence(): void;
  onDownload(materialId: string): void;
  onShare(materialId: string): void;
  onRefresh(): void;
  onBack(): void;
}

const KIND_LABELS = { artifact: '产物', diff: '变更', 'test-report': '测试报告' } as const;

/** 材料屏：索引列表 + 视图面板。终端恒只读（无输入控件）；不支持/大文件给下载与分享路径。 */
export function MaterialsScreen({ index, view, loading, error, grant, onOpenMaterial, onOpenTerminal, onOpenEvidence, onDownload, onShare, onRefresh, onBack }: MaterialsScreenProps) {
  return (
    <ScrollView>
      <Button title="返回任务" onPress={onBack} />
      <Text>任务材料</Text>
      <Button title="刷新材料" onPress={onRefresh} disabled={loading} />
      {index === undefined
        ? <Text>{loading ? '正在读取材料索引…' : '尚无材料索引'}</Text>
        : (
          <View>
            {index.materials.map((entry) => (
              <View key={entry.materialId}>
                <Text>{entry.name}</Text>
                <Text>{KIND_LABELS[entry.kind]} · {entry.mime} · {entry.size} B · 版本 {entry.version}</Text>
                <Button title="打开" onPress={() => onOpenMaterial(entry.materialId)} />
                <Button title="下载" onPress={() => onDownload(entry.materialId)} />
                <Button title="分享" onPress={() => onShare(entry.materialId)} />
              </View>
            ))}
            {index.terminal.available && <Button title="只读终端" onPress={onOpenTerminal} />}
            <Button title="证据引用" onPress={onOpenEvidence} />
          </View>
        )}
      {grant !== undefined && (
        <View>
          <Text>下载授权（短时效，请尽快使用）</Text>
          <Text>{grant.name} · 有效至 {grant.expiresAt.replace('T', ' ').replace('Z', ' UTC')}</Text>
          <Text>{grant.url}</Text>
        </View>
      )}
      {view !== undefined && <MaterialViewPane view={view} />}
      {error !== undefined && <Text>{error}</Text>}
    </ScrollView>
  );
}

function MaterialViewPane({ view }: { view: MaterialView }) {
  if (view.kind === 'terminal') {
    return (
      <View>
        <Text>终端（只读）</Text>
        {view.lines.map((line) => (
          <Text key={line.seq}>{line.stream === 'stderr' ? '[stderr] ' : ''}{line.text}</Text>
        ))}
        {view.nextCursor !== undefined && <Text>可继续加载（游标 {view.nextCursor}）</Text>}
      </View>
    );
  }
  if (view.kind === 'evidence') {
    return (
      <View>
        <Text>证据引用</Text>
        {view.citations.map((citation) => (
          <View key={citation.seq}>
            <Text>#{citation.seq} · {citation.type}{citation.source === undefined ? '' : ` · 来源 ${citation.source}`}</Text>
            <Text>{citation.occurredAt.replace('T', ' ').replace('Z', ' UTC')}</Text>
          </View>
        ))}
        {view.citations.length === 0 && <Text>该任务暂无可追溯的工具与产物来源。</Text>}
      </View>
    );
  }
  if (view.kind === 'diff') {
    if (view.preview.state === 'unsupported') return <Text>该变更文件{view.preview.reason === 'size' ? '过大' : '类型不支持'}内联展示，请下载后查看。</Text>;
    if (view.malformed) return <Text>无法解析为标准 diff，以下为原始内容：</Text>;
    return (
      <View>
        {view.hunks.map((hunk) => (
          <View key={hunk.header}>
            <Text>{hunk.header}</Text>
            {hunk.lines.map((line, position) => (
              <Text key={position}>{line.origin === 'add' ? '+' : line.origin === 'remove' ? '-' : ' '}{line.text}</Text>
            ))}
          </View>
        ))}
      </View>
    );
  }
  if (view.preview.state === 'unsupported') {
    return <Text>该材料{view.preview.reason === 'size' ? '过大' : '类型不支持'}移动端内联预览，请使用下载或分享。</Text>;
  }
  if (view.bytes !== undefined && view.entry.mime.startsWith('image/')) {
    return <Image source={{ uri: `data:${view.entry.mime};base64,${bytesToBase64(view.bytes)}` }} style={{ width: 320, height: 240 }} />;
  }
  return <Text>{view.text}</Text>;
}
```

创建 `apps/mobile/src/app/tasks/materials.tsx`：

```tsx
import { useEffect, useState } from 'react';
import { router, useLocalSearchParams } from 'expo-router';
import { activeMobileRuntime, activeTaskMaterial } from '../../../composition.ts';
import { createMaterialsController, type MaterialsController, type MaterialsViewState } from '../../../materials-view.ts';
import { MaterialsScreen } from '../../../screens/MaterialsScreen.tsx';

/** /tasks/materials 挂载生命周期宿主：handle 在 effect 内开、卸载即 close——与 /tasks/detail 同一模式。 */
export function MaterialsRouteLifecycle({ runId }: { runId: string }) {
  const [state, setState] = useState<MaterialsViewState>({ loading: true });
  const [controller, setController] = useState<MaterialsController | undefined>(undefined);
  useEffect(() => {
    const runtime = activeMobileRuntime();
    const material = activeTaskMaterial();
    const lease = runtime.scopeLease();
    if (!material || !lease || runId.trim() === '') {
      setState({ loading: false, error: '请先登录并激活空间，再查看任务材料。' });
      return;
    }
    let next: MaterialsController | undefined;
    try {
      next = createMaterialsController(material.open({ lease }), { runId });
    } catch {
      setState({ loading: false, error: '登录状态或活动空间已变化，请重新进入。' });
      return;
    }
    setController(next);
    setState(next.state());
    const unsubscribe = next.subscribe(setState);
    void next.load();
    return () => {
      unsubscribe();
      next?.dispose();
    };
  }, [runId]);
  return (
    <MaterialsScreen
      index={state.index}
      view={state.view}
      loading={state.loading}
      error={state.error}
      grant={state.grant}
      onOpenMaterial={(materialId) => { void controller?.openMaterial(materialId); }}
      onOpenTerminal={() => { void controller?.openTerminal(); }}
      onOpenEvidence={() => { void controller?.openEvidence(); }}
      onDownload={(materialId) => { void controller?.download(materialId); }}
      onShare={(materialId) => { void controller?.share(materialId); }}
      onRefresh={() => { void controller?.load(); }}
      onBack={() => router.back()}
    />
  );
}

/** Expo Router 文件路由：/tasks/materials?runId=..。只消费 Task Material Interface。 */
export default function TaskMaterialsRoute() {
  const params = useLocalSearchParams<{ runId?: string }>();
  return <MaterialsRouteLifecycle runId={String(params.runId ?? '')} />;
}
```

修改 `apps/mobile/src/composition.ts`——在 `activeTaskOffice` 函数之后追加：

```ts
import { createTaskMaterial } from '@weknora/mobile-core';
import type { TaskMaterial } from '@weknora/mobile-core';
import { createMobileMaterialRemote } from '@weknora/api-client/mobile/materials';
import { createFetchBlobAdapter, createNativeSharePortIfAvailable } from './adapters/material-adapters.ts';

const taskMaterials = new Map<string, TaskMaterial>();

/** Task Material 按 deployment origin 记忆化；lease 由 Runtime 提供，切租户即 fail closed（module-seams §7）。 */
function taskMaterialFor(activeRuntime: MobileRuntime, origin: string): TaskMaterial {
  let material = taskMaterials.get(origin);
  if (!material) {
    const remote = createMobileMaterialRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) });
    material = createTaskMaterial({ remote, blob: createFetchBlobAdapter(), share: createNativeSharePortIfAvailable() });
    taskMaterials.set(origin, material);
  }
  return material;
}

/** 详情/材料路由经此取当前授权 scope 的 Task Material（无授权面返回 undefined）。 */
export function activeTaskMaterial(): TaskMaterial | undefined {
  const activeRuntime = runtime();
  const snapshot = activeRuntime.snapshot();
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return undefined;
  return taskMaterialFor(activeRuntime, snapshot.deployment.origin);
}
```

（import 语句按该文件既有分组风格并入顶部 import 区：`@weknora/mobile-core` 组与 `@weknora/api-client/mobile/*` 组各加行；`./adapters/material-adapters.ts` 并入本地 adapter import 区。）

修改 `apps/mobile/src/screens/TaskDetailScreen.tsx`：`TaskDetailScreenProps` 增加 `onOpenMaterials?: () => void;`；组件参数解构加入 `onOpenMaterials`；在「任务时间线」区块之后、`重新同步快照` 按钮之前加：

```tsx
      {onOpenMaterials !== undefined && <Button title="任务材料" onPress={onOpenMaterials} />}
```

修改 `apps/mobile/src/app/tasks/detail.tsx`：`TaskDetailRouteLifecycle` 的 props 增加 `onOpenMaterials?: () => void` 并透传给 `TaskDetailScreen`；默认导出改为：

```tsx
import { router, useLocalSearchParams } from 'expo-router';
// …（既有 import 保持不变）…
export default function TaskDetailRoute() {
  const params = useLocalSearchParams<{ taskId?: string; runId?: string }>();
  return (
    <TaskDetailRouteLifecycle
      taskId={String(params.taskId ?? '')}
      runId={String(params.runId ?? '')}
      onOpenMaterials={() => { router.push({ pathname: '/tasks/materials', params: { runId: String(params.runId ?? '') } }); }}
    />
  );
}
```

- [ ] **Step 4: 运行确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: 测试 PASS（原 72 + 新增用例全绿；typecheck 无输出）。typecheck 同时证明 `createMobileMaterialRemote` 返回结构与 mobile-core `MaterialBackendPort` 结构可赋值（composition 直接把 remote 传入 `createTaskMaterial({ remote, ... })`，strict tsc 编译通过即为证据）。

- [ ] **Step 5: REFACTOR 检查**

`materials-view.ts` 的 publish/run 骨架与 `task-detail-view.ts` 相似但错误表/动作集不同——保持独立（两个 controller 属不同模块面，合并会造出浅层共享）。确认 `MaterialsScreen` 无任何 wire import（app-smoke 断言钉住）。

- [ ] **Step 6: 提交**

```bash
git add apps/mobile/src/materials-view.ts apps/mobile/src/materials-view.test.ts apps/mobile/src/screens/MaterialsScreen.tsx apps/mobile/src/app/tasks/materials.tsx apps/mobile/src/adapters/material-adapters.ts apps/mobile/src/composition.ts apps/mobile/src/screens/TaskDetailScreen.tsx apps/mobile/src/app/tasks/detail.tsx apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): materials screen, controller, route and composition wiring for Task Material"
```

---

### Task 9: apps/mobile——真实 HTTP 集成证据（AC3，opt-in）

**Files:**
- Create: `apps/mobile/src/material-integration-smoke.ts`
- Test: `apps/mobile/src/material-integration-smoke.test.ts`

**Interfaces:**
- Consumes: Task 6 的 `createTaskMaterial`；Task 7 的 `createMobileMaterialRemote`；Task 8 的 `createFetchBlobAdapter`；`task-detail-integration-smoke.ts` 的范式（`disallowedDeploymentHost` 自 `./runtime-integration-smoke.ts:118` 导入）；`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` opt-in 变量集合。
- Produces: `materialIntegrationConfig(env)`（与 T04/T05 同一 opt-in 语义，自包含）；`runMaterialIntegration(config): Promise<MaterialIntegrationEvidence>`（total 契约：任何步骤异常 → `listed:'failed'` + `failure` 摘要，从不 reject，单 finally 清理）；`MaterialIntegrationEvidence`（无凭据字段）：

```ts
export interface MaterialIntegrationEvidence {
  deploymentOrigin: string;
  listed: 'listed' | 'no-tasks' | 'no-materials' | 'failed';
  materialCount?: number;
  kindCounts?: { artifact: number; diff: number; 'test-report': number };
  grant?: 'minted' | 'failed';
  grantTtlSeconds?: number;   // 断言 ≤ 900（AC1「签名 URL 不长期缓存」的端到端面）
  downloaded?: 'fetched' | 'failed' | 'skipped';
  downloadedBytes?: number;
  terminal?: 'paged' | 'empty' | 'unavailable' | 'failed';
  citations?: number;
  failure?: string;
  commandTimestamp: string;
}
```

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/material-integration-smoke.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { materialIntegrationConfig } from './material-integration-smoke.ts';

const here = dirname(fileURLToPath(import.meta.url));

test('the config rejects loopback, private and reserved deployment hosts as invalid', () => {
  const vars = { WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://127.0.0.1:8080', WEKNORA_MOBILE_TEST_EMAIL: 'user@example.test', WEKNORA_MOBILE_TEST_PASSWORD: 'pw' };
  for (const host of ['https://localhost', 'https://10.0.0.5', 'https://192.168.1.4', 'https://[fe80::1]', 'https://169.254.1.1',
    'https://100.64.0.1', 'https://198.18.0.1', 'https://192.0.2.1', 'https://203.0.113.1']) {
    const rejected = materialIntegrationConfig({ ...vars, WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: host });
    assert.equal(rejected.enabled, false, host);
    assert.equal(rejected.enabled === false && rejected.disposition, 'invalid', host);
  }
});

test('a public https origin still enables the integration config', () => {
  const verdict = materialIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.org', WEKNORA_MOBILE_TEST_EMAIL: 'user@example.test', WEKNORA_MOBILE_TEST_PASSWORD: 'pw' });
  assert.equal(verdict.enabled, true);
});

test('missing credentials skip instead of failing', () => {
  const verdict = materialIntegrationConfig({});
  assert.equal(verdict.enabled, false);
  assert.equal(verdict.enabled === false && verdict.disposition, 'skip');
});

test('runMaterialIntegration is total and pins the AC1 ttl assertion', () => {
  const source = readFileSync(join(here, 'material-integration-smoke.ts'), 'utf8');
  assert.match(source, /disallowedDeploymentHost/, 'config 校验必须复用主机防线');
  const runBody = source.slice(source.indexOf('export async function runMaterialIntegration'));
  assert.match(runBody, /finally\s*\{/, '主流程必须有 finally 收口');
  assert.match(runBody, /grantTtlSeconds/, '证据必须记录 grant TTL');
  assert.match(runBody, /900/, '并断言 TTL 不超过 900 秒（AC1）');
});

test('runMaterialIntegration executes against a real deployment when credentials are provided', { skip: Object.keys(process.env).some((key) => key === 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL') === false }, async () => {
  const { runMaterialIntegration } = await import('./material-integration-smoke.ts');
  const config = materialIntegrationConfig(process.env as Record<string, string | undefined>);
  if (config.enabled === false) { assert.ok(true, '凭据不完整时如实跳过，不伪造通过'); return; }
  const evidence = await runMaterialIntegration(config);
  assert.equal(typeof evidence.commandTimestamp, 'string');
  if (evidence.listed === 'listed') {
    if (evidence.grantTtlSeconds !== undefined) assert.ok(evidence.grantTtlSeconds <= 900, `grant TTL ${evidence.grantTtlSeconds}s 超过 900s 上限`);
  }
});
```

- [ ] **Step 2: 运行确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL——`Cannot find module './material-integration-smoke.ts'`。

- [ ] **Step 3: 最小实现**

创建 `apps/mobile/src/material-integration-smoke.ts`：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileMaterialRemote } from '@weknora/api-client/mobile/materials';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskMaterial, createTaskOffice } from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';
import { createFetchBlobAdapter } from './adapters/material-adapters.ts';

export type MaterialIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface MaterialIntegrationEvidence {
  deploymentOrigin: string;
  listed: 'listed' | 'no-tasks' | 'no-materials' | 'failed';
  materialCount?: number;
  kindCounts?: { artifact: number; diff: number; 'test-report': number };
  grant?: 'minted' | 'failed';
  grantTtlSeconds?: number;
  downloaded?: 'fetched' | 'failed' | 'skipped';
  downloadedBytes?: number;
  terminal?: 'paged' | 'empty' | 'unavailable' | 'failed';
  citations?: number;
  failure?: string;
  commandTimestamp: string;
}

/** 与 T04/T05 相同的 opt-in 语义（自包含，不跨计划 import 凭据逻辑）。 */
export function materialIntegrationConfig(env: Record<string, string | undefined>): MaterialIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    return { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' };
  }
  let parsed: URL;
  try { parsed = new URL(deploymentOrigin); } catch {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

/**
 * 真实 JSON transport + 授权通道 + 具体 Remote Adapter + Task Material 编排 + 免凭据
 * 签名下载。账号无任务/无材料时如实记 'no-tasks'/'no-materials'；任何步骤异常 →
 * listed:'failed' + failure 摘要（无凭据字段），从不 reject；所有路径经同一 finally 清理。
 */
export async function runMaterialIntegration(config: Extract<MaterialIntegrationConfig, { enabled: true }>): Promise<MaterialIntegrationEvidence> {
  const evidence: MaterialIntegrationEvidence = { deploymentOrigin: config.deploymentOrigin, listed: 'failed', commandTimestamp: new Date().toISOString() };
  const fetcher: FetchLike = (input, init) => fetch(input, init as RequestInit);
  const runtime = createMobileRuntime({
    credentialStore: createInMemoryCredentialStore(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
    },
  });
  try {
    const snapshot = await runtime.signIn({ deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' }, email: config.email, password: config.password });
    if (snapshot.surface !== 'authorized' || !snapshot.deployment) return evidence;

    const office = createTaskOffice({
      backend: createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
      lease: () => runtime.scopeLease(),
    });
    const page = await office.tasks({});
    if (page.items.length === 0) { evidence.listed = 'no-tasks'; return evidence; }
    const runId = page.items[0]!.runId;

    const material = createTaskMaterial({
      remote: createMobileMaterialRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
      blob: createFetchBlobAdapter(),
    });
    const lease = runtime.scopeLease();
    if (lease === undefined) return evidence;
    const handle = material.open({ lease });
    try {
      const index = await handle.index({ runId });
      evidence.terminal = index.terminal.available ? 'paged' : 'unavailable';
      if (index.materials.length === 0) { evidence.listed = 'no-materials'; evidence.materialCount = 0; return evidence; }
      evidence.listed = 'listed';
      evidence.materialCount = index.materials.length;
      evidence.kindCounts = {
        artifact: index.materials.filter((entry) => entry.kind === 'artifact').length,
        diff: index.materials.filter((entry) => entry.kind === 'diff').length,
        'test-report': index.materials.filter((entry) => entry.kind === 'test-report').length,
      };
      const target = index.materials[0]!;

      const grant = await handle.act({ kind: 'download', runId, materialId: target.materialId });
      if (grant.kind === 'grant') {
        evidence.grant = 'minted';
        const expiresAt = Date.parse(grant.expiresAt);
        const ttlSeconds = Number.isFinite(expiresAt) ? Math.round((expiresAt - Date.now()) / 1000) : -1;
        evidence.grantTtlSeconds = ttlSeconds;
        if (ttlSeconds < 0 || ttlSeconds > 900) {
          evidence.failure = `grant ttl ${ttlSeconds}s exceeds the 900s cap`;
        } else {
          // 真实免凭据下载：签名链接本身不带登录态，走全局 fetch。
          const response = await fetch(grant.url);
          if (response.ok) {
            evidence.downloaded = 'fetched';
            evidence.downloadedBytes = (await response.arrayBuffer()).byteLength;
          } else {
            evidence.downloaded = 'failed';
          }
        }
      } else {
        evidence.grant = 'failed';
      }

      const citationsView = await handle.open({ kind: 'evidence', runId });
      if (citationsView.kind === 'evidence') evidence.citations = citationsView.citations.length;

      if (evidence.terminal === 'paged') {
        const terminalView = await handle.open({ kind: 'terminal', runId });
        if (terminalView.kind === 'terminal' && terminalView.lines.length === 0) evidence.terminal = 'empty';
      }
    } finally {
      try { handle.close('integration-complete'); } catch { /* 清理不得再抛 */ }
    }
  } catch (error) {
    evidence.listed = 'failed';
    evidence.failure = error instanceof Error ? error.message : String(error);
  } finally {
    runtime.dispose();
  }
  return evidence;
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitMaterialIntegrationEvidence(evidence: MaterialIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
```

- [ ] **Step 4: 运行确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: 本地无 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL` 时——config 校验/total 契约测试 PASS，真实集成用例按 skip 条件跳过（如实跳过，**不伪造通过**）。具备真实部署与账号（且服务端配置了 `WEKNORA_ARTIFACT_SIGNING_KEY`）时——`runMaterialIntegration` 用例 PASS 且产出 `listed:'listed'`、`grant:'minted'`、`grantTtlSeconds ≤ 900`、`downloaded:'fetched'` 的端到端证据（AC3）。

- [ ] **Step 5: REFACTOR 检查**

确认 evidence 契约无凭据字段（只有 origin/计数/状态/TTL/字节数）；`finally` 双层收口（handle.close + runtime.dispose）；`grantTtlSeconds` 为负或超 900 时记 failure 而非静默。

- [ ] **Step 6: 提交**

```bash
git add apps/mobile/src/material-integration-smoke.ts apps/mobile/src/material-integration-smoke.test.ts
git commit -m "test(mobile): opt-in real-http material integration evidence (list, grant ttl, credential-free download)"
```

---

## 计划级验证

全部任务完成后在 worktree 根执行（覆盖本计划全部测试面，定向到受影响包/目录，避开全量 flaky 套件）：

```bash
go test ./internal/application/repository/ -run 'TestReadRunTerminalEvents' -count=1 && go test ./internal/handler/session/ -run 'TestGetWorkbenchTerminalLog|TestListWorkbenchArtifacts|TestCreateWorkbenchArtifactSignedURL|TestDownloadWorkbenchArtifactGrant' -count=1 && go test ./internal/modules/workbench/ -count=1 && pnpm exec tsx --test packages/mobile-core/src/material/*.test.ts && pnpm exec tsx --test packages/api-client/src/mobile/materials.test.ts packages/api-client/src/mobile/task-office.test.ts && pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck
```

（本计划作者已在当前 HEAD 实跑其中的既有基线部分：两个 Go 定向命令 ok、`task-office.test.ts` 9 pass、`@weknora/mobile` test 72 pass + typecheck 干净；新增用例由执行者按各任务步骤实跑。）

## Consumes-Produces 摘要（供同批其余计划与 #47/#48/#52/#68 集成）

**Consumes（前置接口，均已在当前 HEAD 亲眼核实）**
- `MobileRuntime.authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown>`（#34）；`runtime.scopeLease(): ScopeLease | undefined`（#32）；`RuntimeScopeLease`/`leaseActive`/`leaseScopeOf`（#32，包内）；`taskOfficeFor`/`activeTaskOffice`/`activeMobileRuntime` 组合根范式（#34/#35）；`/tasks/detail` 路由（#35）；`disallowedDeploymentHost`（#32 B2）；Go 侧 `resolveOwnedRun`/`OwnedRunReader`/`WorkbenchSnapshotReader`/`agent_run_events`/`executionEventFor`。

**Produces（本计划对外接口，供后续计划 Consumes）**
- Go：`repository.ReadRunTerminalEvents(ctx, key, after, limit)` 与 `repository.TerminalLogEventType = "tool.terminal"`；`GET /api/v1/workbench/executions/:run_id/terminal-log`；workbench artifacts 列表项 `version`（内容寻址身份）/`digest?` 与信封 `terminal.available`。
- mobile-core：`createTaskMaterial(ports: TaskMaterialPorts): TaskMaterial`（`open({lease}) → index/open/act/subscribe/close`）、`MaterialError` + 11 个错误码、`materialKindOf`/`previewVerdictOf`/`isInlineImageMime`/`PREVIEW_MAX_BYTES`、`parseUnifiedDiff`、`projectCitations`、`createScenarioMaterialRemote`/`createScriptedBlobFetch`/`createRecordingSharePort`、`BlobFetchPort`/`SharePort`/`MaterialBackendPort`。
- api-client：`createMobileMaterialRemote({ origin, request })`（exports `@weknora/api-client/mobile/materials`）。
- apps/mobile：`activeTaskMaterial()`、`taskMaterialFor`、`createMaterialsController`/`MATERIAL_ERROR_COPY`、`MaterialsScreen`、`/tasks/materials` 路由、`createFetchBlobAdapter`/`createNativeSharePortIfAvailable`、`MaterialIntegrationEvidence` + `materialIntegrationConfig`/`runMaterialIntegration`。
- **边界声明**：`MaterialIntent` 不含 annotate/request-revision（#47）；office 外部发布（#48）与代码交付 Diff/PR（#52）不经本模块；跨进程加密 blob 持久化属 #40（本计划 blob 缓存为句柄内存态）；真机系统分享/下载管理器属真机验收门槛。
