# OCR 增量修复批次 3（issue30-sweep）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 OCR 第三批增量发现（B3 报告，44 项主发现 + 38 项 lowWorth）中全部 6 项 high（contracts 类型拓宽击穿 typecheck:shared 门禁、task-office start() 的 intentLog.remove 击穿幂等重放/伪装失败、跨重启未回填 intentRequestId、SecureStore 意图预算漏算、New 屏初始化 agents() 无兜底致草稿丢失）与 38 项 medium 中可精确落地的部分，按根因聚合为 15 个任务，同根因共用一组回归测试，不逐条开流。

**Architecture:** 按**根因**聚合成 15 个任务：1 项 TS contracts 类型修复（P0，先解锁 typecheck 门禁）+ 3 个 Go 任务（task_grant 授权语义 / workbench 只读面授权与能力位 / agent-adoption 治理链）+ 11 个 TS 任务（domain submission 重发终态 → task-office start/恢复错误语义 → 持久化适配防御 → new-task 控制器生命周期 → inbox/device 投影一致性 → api-client 错误翻译与 origin 提取 → material core 缓存与凭据回验 → material 远端与视图 → legacy 域 → composition/请求 ID → task-start smoke 收口）。共享文件（`task-office.ts` 被 Task 6→13 串行修改、`NewTaskScreen.tsx` 在 Task 8、`composition.ts` 仅 Task 14）按任务号顺序串行执行；Go 三个任务相互独立可并行。所有修复维持既有架构约束：mobile-core 不依赖 RN、D5「绝不换 ID 重建任务」、SecureStore 单值 2048B 预算、fail-closed 不放松。

**Tech Stack:** Go（`internal/handler`、`internal/handler/session`、`internal/application/service`、`internal/application/repository`、`internal/router`，go test + testify）、TypeScript（`packages/contracts`、`packages/domain`、`packages/mobile-core`、`packages/api-client`、`apps/mobile`，node:test + tsx）。测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行。本计划作者已实跑下列基线（2026-09-24，当前 HEAD）：
- `pnpm exec tsx --test packages/mobile-core/src/task-office/task-office-start.test.ts packages/mobile-core/src/task-office/task-office.test.ts packages/domain/src/mobile/submission.test.ts apps/mobile/src/new-task-view.test.ts apps/mobile/src/new-task-drafts.test.ts apps/mobile/src/adapters/intent-log.test.ts` → **48 pass / 0 fail**。
- `pnpm exec tsx --test packages/mobile-core/src/inbox/notification-inbox.test.ts packages/mobile-core/src/device/device-registry.test.ts packages/mobile-core/src/material/task-material.test.ts apps/mobile/src/legacy-tasks-view.test.ts apps/mobile/src/materials-view.test.ts packages/api-client/src/mobile/legacy-tasks.test.ts packages/api-client/src/mobile/materials.test.ts packages/contracts/src/marketplace/agent-adoption.test.ts apps/mobile/src/task-start-integration-smoke.test.ts` → **57 tests / 56 pass / 1 skip**（skip 为 opt-in integration，语义正常）。
- `go test ./internal/application/service/ ./internal/application/repository/ ./internal/router/ ./internal/handler/` → 全部 **ok**。
- `go test ./internal/handler/session/ -run 'TestGetWorkbenchTerminalLog|TestGetWorkbenchArtifact'` → **PASS**（terminal-log/artifacts 既有用例全绿）。
- `npx tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions packages/contracts/src/marketplace/agent-adoption.ts` → **FAIL**：`agent-adoption.ts(89,5): error TS2322`（B3-F85 复现）。
- `pnpm run typecheck:shared` → **FAIL**，基线失败集合 = `packages/contracts/src/marketplace/agent-adoption.ts` ×1（本批次 Task 1 修复）+ `packages/views/src/chat/mermaid.ts` ×2（**范围外预存在**，本批次不动，验收口径见 Global Constraints）。

**Spec:**
- 发现来源：B3 增量发现清单（本计划的 ask 材料；编号 B3-F*，全部经本计划作者按当前 HEAD 读码复核，行号以复核为准；差异见「差异记录」）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（Implementation Decisions / Testing Decisions）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§4/§5/§7/§10）
- ADR：`docs/adr/0007-registered-devices-and-encrypted-cache.md`、`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`、ADR-0004（owner 权威，B3-F83 引用）
- 领域术语：`CONTEXT.md`
- 前序批次：`docs/plans/issue30-sweep/plans/ocr-fix-round-1.md`、`ocr-fix-increment-batch2.md`（本批次沿用其纪律与验收口径）
- Parent：Issue #30（issue30-sweep）

## Global Constraints

以下为批准 Spec / ADR / 报告隐含的项目级约束，所有任务隐含遵守：

- **D5「绝不换 ID 重建任务」**（`packages/domain/src/mobile/submission.ts` resume 注释原文：「仅当服务端明确 unknown……才以同一 request_id 重发——绝不换 ID 重建任务」）——Task 5/6/8 的一切改动不得引入换 ID 路径。
- 「Mobile core does not depend on React Native, DOM or concrete transport.」（mobile-ai-office-design.md）——Task 5/6/9/11 对 domain/mobile-core 的修改不得引入 RN/平台依赖。
- 「Tests target observable behavior at the highest stable Interface.」——回归测试落在 Interface 行为（coordinator/office/handler HTTP 输出），不测内部实现细节。
- Android SecureStore 单值约 2048 字节；intent log 安全线 1536B（`apps/mobile/src/adapters/intent-log.ts:15`）——Task 7 的预算拦截必须保持「save 失败 = 零 Start 派发」语义。
- intentLog 端口契约：记录留存幂等无害（remove 失败可吞）；但「重入时记录缺失绝不能换 session」（B3-F78）——Task 6 的两个修复方向受这对约束共同限定。
- 测试凭据仅用 `*.example.test` 保留域形态；真实 HTTP 证据仅公网 HTTPS 主机（Task 15 沿用既有 opt-in 语义，不得伪造通过）。
- 严格 RED→GREEN→REFACTOR：每个任务先写失败测试、实跑确认失败、最小实现、通过、提交；实现与已批准 Spec 冲突时升级处理。
- 发现驱动的最小修复：lowWorth 项仅按「附录 A」映射折叠；结构化重构（B3-F4/F8/F70/F71/F72 等）明确延期并记录理由。
- **已知预存在失败（非本批次引入，验收口径为「失败集合不扩大」）**：`go test ./internal/handler/session/` 中 `TestWorkbenchStartHTTPIntegrationAndIdentityIsolation` 因 `migrations/sqlite/` 下 `000112_task_grants.*` 与 `000112_agent_adoption_variants.*` 序号冲突而失败（merge d58675a67 引入，已在本 worktree 实跑确认）；`pnpm run typecheck:shared` 中 `packages/views/src/chat/mermaid.ts` ×2 类型错误为范围外预存在。两者均不阻塞本批次任务级验收（用 `-run` 过滤 / 窄版 tsc），但批次验收（附录 B）必须复核失败集合未扩大。migration 序号重编涉及已发布迁移历史，应升级为独立决策，不在本批次顺手修改。

## Review Focus

Spec/报告隐含但任务测试需钉住、最可能咬到真实用户的五类失效模式（每行后标注 owning 任务）：

1. **跨重启后用户重试一个已 bound 的意图（intentLog 记录已被 remove）** —— 不得 createSession 换 session、不得抛 SUBMISSION_CONFLICT、必须零网络返回原 bound 回执；remove 本身失败也不得把成功提交伪装成失败。——Task 6 测试（`replaying a bound intent after its log record was removed...` ×2）。
2. **服务端确定性 4xx 拒绝（预算不足/参数拒绝）** —— 不得永远停留在 awaiting_reconciliation 形成「重入即重发」循环；必须可达 rejected 终态，让上层（Task 8）停止同 ID 重试引导。——Task 5 测试 ×2 + Task 8 测试（rejected receipt 不再驱动同 ID 重试）。
3. **离线/慢网络首次打开 New 屏（agents() 失败、初始化秒级窗口内用户已输入）** —— 已保存草稿不得被 emptyDraft 覆盖；用户在 loading 窗口内的输入不得被 `stored ?? emptyDraft()` 无条件回写丢失。——Task 8 测试 ×3（agents 失败保草稿、慢初始化输入保留、初始化前 submit 不以旧状态裁决）。
4. **SecureStore 2048B 硬限与并发写** —— 合法 goal（500 字 + knowledgeIds + attachments）单条超限必须显式失败（可行动错误码）而不是 setItemAsync 底层抛错；两个并发 start 的 save 不得后写覆盖前写丢意图记录。——Task 7 测试 ×2。
5. **被授权 Viewer/Collaborator 的只读面与授权数据源** —— terminal-log 不得 owner-only 404（snapshot 同源数据已可读）；停用成员的 grant 不得继续放行；数据库故障不得伪装成 task not found。——Task 3 测试 ×1 + Task 2 测试 ×3。

---

## 任务结构与文件地图

| # | 任务 | 根因分组（发现编号） | 主要文件 | 优先级 |
|---|---|---|---|---|
| 1 | contracts：agent-adoption 类型拓宽 | B3-F85 | `packages/contracts/src/marketplace/agent-adoption.ts` | **P0/门禁** |
| 2 | Go：task_grant 授权语义 | B3-F82、F84、F83；顺手 F74、F75 | `internal/application/service/task_grant.go`、`internal/handler/commercial_task_budget.go`、`internal/application/repository/task_grant_store.go` | high |
| 3 | Go：workbench 只读面授权与能力位 | B3-F63、F76 | `internal/handler/session/workbench_terminal_log.go`、`workbench_artifacts.go`、`internal/router/routes_workbench.go` | medium |
| 4 | Go：agent-adoption 治理链 | B3-F69、F86、F87；顺手 F73（F70/F71 延期，见附录 A） | `internal/router/routes_agent_adoption.go`、`internal/handler/agent_adoption.go`、`internal/application/repository/agent_adoption.go` | medium |
| 5 | domain：submission 重发终态 | B3-F44；顺手 F62 | `packages/domain/src/mobile/submission.ts` | high |
| 6 | mobile-core：task-office start/恢复错误语义 | B3-F78、F64、F79、F65；顺手 F68 | `packages/mobile-core/src/task-office/task-office.ts` | **high** |
| 7 | apps/mobile：意图/草稿持久化防御 | B3-F51、F38、F42 | `apps/mobile/src/adapters/intent-log.ts`、`apps/mobile/src/new-task-drafts.ts` | high |
| 8 | apps/mobile：new-task 控制器生命周期 | B3-F37、F41、F40、F59、F39；顺手 F46 | `apps/mobile/src/new-task-view.ts`、`apps/mobile/src/screens/NewTaskScreen.tsx` | **high** |
| 9 | mobile-core：inbox/device 投影一致性 | B3-F1、F5；顺手 F2、F6、F50 | `packages/mobile-core/src/inbox/notification-inbox.ts`、`packages/mobile-core/src/device/device-registry.ts` | medium |
| 10 | api-client：错误翻译与 origin 提取 | B3-F43、F11 | `packages/api-client/src/mobile/task-office.ts`、新建 `deployment-origin.ts` + 7 处替换 | medium |
| 11 | mobile-core：material 缓存与凭据回验 | B3-F31、F58；顺手 F32、F33、F34、F35、F55 | `packages/mobile-core/src/material/task-material.ts`、`evidence.ts`、`apps/mobile/src/materials-view.ts`（F55） | medium |
| 12 | api-client/app：material 末页与视图 | B3-F52、F54、F57；顺手 F10、F18、F36 | `packages/api-client/src/mobile/materials.ts`、`apps/mobile/src/materials-view.ts`、`adapters/material-adapters.ts`、`resources-view.ts`、`packages/mobile-core/src/material/ports.ts` | medium |
| 13 | legacy 域：卡片隔离与历史通道 | B3-F21、F22、F23、F24；顺手 F14、F66 | `apps/mobile/src/screens/LegacyTasksScreen.tsx`、`legacy-tasks-view.ts`、`packages/mobile-core/src/task-office/task-office.ts`（legacyHistory）、`packages/api-client/src/mobile/legacy-tasks.ts` | medium |
| 14 | composition/请求 ID/设备身份 | B3-F26、F28、F29、F27、F49；顺手 F48 | `apps/mobile/src/composition.ts`、`adapters/request-id.ts`、`adapters/device-identity.ts`、`apps/mobile/package.json` | medium |
| 15 | apps/mobile：task-start smoke 证据链 | B3-F45、F60 | `apps/mobile/src/task-start-integration-smoke.ts` | medium |

执行顺序：Task 1 最先（解锁门禁）；Task 5 → Task 6 → Task 15（domain 语义 → office 行为 → smoke 断言）；Task 6 → Task 13（`task-office.ts` 串行）；Task 7 → Task 8（意图持久化先稳，控制器后改）；Task 11 → Task 12（material 域，F53 依赖 F52 的游标收敛）。Task 2/3/4（Go）与 Task 9/10/14 相互独立，可并行。共享文件：`task-office.ts`（Task 6→13）、`NewTaskScreen.tsx`（Task 8）、`materials-view.ts`（Task 11 的 F55 → Task 12）、`composition.ts`（仅 Task 14）。

## 差异记录（报告 vs 代码现状，以代码现状为准）

1. B3-F22 引用的对照模式 `attention.tsx:13-15` 实际位于 `apps/mobile/src/app/attention.tsx:13-16`（`InboxRouteLifecycle` 的显式空态）；LegacyTasksScreen 本身无此模式。
2. B3-F11 称 7 份 `requireDeploymentOrigin`「逐字拷贝」——实际已轻微漂移：`packages/api-client/src/mobile/inbox.ts:36-46` 比 `materials.ts:30-39` 多一条 `parsed.hostname === ''` 检查。Task 10 提取时以**最严格版**（含 hostname 检查）合并，这正是 F11 指出的漂移风险的实证。
3. B3-F13（signIn 非 authorized 面 failure 摘要为空）：对 `task-start-integration-smoke.ts` **不成立**（`task-start-integration-smoke.ts:73-75` 已写 `evidence.errorReason = \`surface ${snapshot.surface}\``）；对其余 5 个 integration smoke 成立，见附录 A 延期记录。
4. B3-F53 的「可继续加载（游标 X）」提示：作者在 `apps/mobile` 全目录 grep 未定位到该文案的渲染点（`MaterialsScreen.tsx` 仅有「只读终端」入口）。F52 修复后 `nextCursor` 信号本身已正确收敛（`task-material.ts:168` 以 undefined 判末页），提示的交互承载延期（附录 A）。
5. B3-F47（inbox 硬编码 limit 50）：作者复核 `packages/api-client/src/mobile/inbox.ts:70-72` 的 `inbox(cursor?)` 不携带 limit 参数，也未在 TS 侧发现 50 的硬编码点（报告描述被截断）；延期并记录复核入口（附录 A）。
6. `pnpm run typecheck:shared` 基线除 B3-F85 外还有 `packages/views/src/chat/mermaid.ts` ×2 预存在失败（本作者实跑确认）；Task 1 的验收以「agent-adoption 错误消失 + 失败集合不扩大」为准，不以全量门禁转绿为准。
7. B3-F45 的「第二次同 ID 重入必抛 TASK_OFFICE_SUBMISSION_CONFLICT」前提在 Task 6 修复 B3-F78 后**不再成立**（重入将零网络返回 bound 回执）——Task 15 的断言按修复后语义设计，但仍保留 try/catch 证据保护（失败也产出 evidence）。

---

### Task 1: [P0] contracts：agent-adoption 状态数组类型拓宽（B3-F85）

**Files:**
- Modify: `packages/contracts/src/marketplace/agent-adoption.ts:41-42`

**Interfaces:**
- Consumes: `enumValue<T extends string>(row, key, allowed: readonly T[], path): T`（本文件 :58-63，已要求 `readonly T[]`）。
- Produces: `VARIANT_STATES`/`CAPABILITY_STATES` 类型为 `readonly ['draft','mapped','tested','published']` / `readonly ['supported','unavailable','forbidden']`；`parseVariantRow` 的 `state` 赋值恢复字面量联合，`agent-adoption.ts(89,5)` 的 TS2322 消失；`pnpm run typecheck:shared` 失败集合中本文件清零。无运行时行为变化（`as const` 仅类型层）。

**根因与修复说明：** `const VARIANT_STATES = ['draft', 'mapped', 'tested', 'published']` 缺 `as const`，推断为 `string[]`；传入 `enumValue` 时 `T` 被拓宽为 `string`，返回值赋给 `AgentAdoptionVariant['state']` 的字面量联合即 TS2322（作者已实跑窄版 tsc 复现，见 Tech Stack）。`packages/contracts/src/index.ts:729-730` re-export 本模块，错误进入 `typecheck:shared` 门禁（根 package.json:19 确认含 `packages/contracts/src/index.ts`）。

- [ ] **Step 1: 实跑确认当前失败（本任务的 check 即门禁本身）**

Run: `npx tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions packages/contracts/src/marketplace/agent-adoption.ts`
Expected: FAIL —— 输出含 `agent-adoption.ts(89,5): error TS2322: Type 'string' is not assignable to type '"draft" | "mapped" | "tested" | "published"'`（作者基线实跑已复现）。

- [ ] **Step 2: 最小实现**

```ts
// agent-adoption.ts:41-42 —— 两行各追加 as const
const VARIANT_STATES = ['draft', 'mapped', 'tested', 'published'] as const;
const CAPABILITY_STATES = ['supported', 'unavailable', 'forbidden'] as const;
```

- [ ] **Step 3: 实跑确认通过**

Run: 同 Step 1 命令。
Expected: 无输出（exit 0）。
Run: `pnpm exec tsx --test packages/contracts/src/marketplace/agent-adoption.test.ts`
Expected: PASS（既有解析器用例全绿，证明 `as const` 无运行时影响）。

- [ ] **Step 4: 门禁失败集合复核**

Run: `pnpm run typecheck:shared 2>&1 | grep -E "^packages/" | sed 's/(.*//' | sort | uniq -c`
Expected: 仅剩 `packages/views/src/chat/mermaid.ts`（×2，预存在、范围外）；`agent-adoption.ts` 不再出现（对照差异记录 6）。

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/marketplace/agent-adoption.ts
git commit -m "fix(contracts): tighten agent-adoption state arrays with as const (typecheck:shared gate)"
```

---

### Task 2: Go：task_grant 授权语义（B3-F82、F84、F83；顺手 B3-F74、F75）

**Files:**
- Modify: `internal/application/service/task_grant.go:60-63`（loadTaskForGrantManagement 错误折叠）、`:162-165`（ResolveTaskAccess 同型折叠）、`:171-181`（成员 active 校验）
- Modify: `internal/handler/commercial_task_budget.go:99-113`（taskRunOwner 的 owner 权威改 sessions.user_id）
- Modify: `internal/application/repository/task_grant_store.go:98-105`（顺手 F74：TaskOwnerID NULL 扫描）、UpsertGrant（顺手 F75：created_at 一致性）
- Test: `internal/application/service/task_grant_test.go`、`internal/handler/commercial_task_budget_test.go`、`internal/application/repository/task_grant_store_test.go`

**Interfaces:**
- Consumes: `apperrors.ErrSessionNotFound`（既有 not-found 哨兵，`stubTaskGrantSessions.GetByID` 于 task_grant_test.go:71-76 已按此返回）；`TaskMemberLookupPort.Get(ctx, userID, tenantID)`（task_grant.go:80 已用于 grantee 校验，miss 返回 `(nil, nil)`）；既有 `activeMember` 判定（task_grant.go:75-90 区域）。
- Produces: `loadTaskForGrantManagement`/`ResolveTaskAccess` 对 `errors.Is(err, apperrors.ErrSessionNotFound)` 返回 `NewNotFoundError`，**其余错误原样透传**（5xx 不再伪装 404）；`ResolveTaskAccess` 在 grant 命中后经成员 active 校验（停用成员 → `TaskAccessNone`，与 `GetRunForGrantedReader` 的实时 JOIN 收敛）；`CommercialHandler.taskRunOwner` 的 owner 谓词改为 `agent_runs JOIN sessions` 取 `sessions.user_id`（ADR-0004 权威）；`TaskGrantStore.TaskOwnerID` 对 SQL NULL 的 `sessions.user_id` 不再扫入普通 string。

**根因与修复说明：** 三处同根因——授权数据源把「查询失败」与「不存在」折叠、把「grant 行存在」当「授权有效」、把运行属主当任务属主：(a) 两处 `if err != nil { return NewNotFoundError }` 把基础设施故障折叠成 404（F82，报告对照 `agent_run.go:130-141` 的 GetRunForGrantedReader 惯例：NotFound→404、其余透传 500）；(b) `ResolveTaskAccess` 只凭 grant 行返回 viewer/collaborator，不校验成员 active（F84，`s.members` 已注入却只在 grantee 校验用——task_grant.go:80）；(c) `taskRunOwner` 从 `agent_runs.Select("owner_id")` 解析（commercial_task_budget.go:103-106），而 `task_grant_store.go:95-98` 注释明确 owner 权威是 `sessions.user_id`——协作者建 run 后能过门禁扩额、真实 owner 反被 403（F83，`TaskRoleCanRun(collaborator)=true` 已交付）。顺手：F74（TaskOwnerID 把 SQL NULL 扫进普通 string 触发 gorm「unsupported data」类错误——改 `sql.NullString` 或 `*string` 扫描）；F75（UpsertGrant 冲突更新路径 DB 行保留原 created_at，返回结构体却带本次构造的 now——冲突路径重读 DB 行返回）。

- [ ] **Step 1: 写失败测试（service 层 3 个 + handler 层 1 个）**

`internal/application/service/task_grant_test.go` 追加（沿用本文件既有 `stubTaskGrantRepo`/`stubTaskGrantSessions`/`stubTaskGrantMembers`/`newTaskGrantServiceForTest` 夹具）：

```go
// stubTaskGrantSessions 的可注入失败变体：GetByID 返回非 NotFound 的基础设施错误
type explodingTaskGrantSessions struct {
	interfaces.SessionRepository
}

func (r *explodingTaskGrantSessions) GetByID(_ context.Context, _ uint64, _ string) (*types.Session, error) {
	return nil, errors.New("db connection refused")
}

func TestGrantTaskAccessSurfacesInfrastructureErrorsInsteadOf404(t *testing.T) {
	ctx := context.Background()
	svc := NewTaskGrantService(&stubTaskGrantRepo{}, &explodingTaskGrantSessions{}, &stubTaskGrantMembers{})
	_, err := svc.GrantTaskAccess(ctx, taskGrantOwner(), "s1", "member-2", types.TaskGrantRoleViewer)
	require.Error(t, err)
	require.False(t, apperrors.IsNotFound(err), "数据库故障不得伪装成 task not found（B3-F82）")
	require.Contains(t, err.Error(), "db connection refused")
}

func TestResolveTaskAccessSurfacesInfrastructureErrorsInsteadOf404(t *testing.T) {
	ctx := context.Background()
	svc := NewTaskGrantService(&stubTaskGrantRepo{}, &explodingTaskGrantSessions{}, &stubTaskGrantMembers{})
	_, err := svc.ResolveTaskAccess(ctx, types.Caller{TenantID: 1, UserID: "owner-1"}, "s1")
	require.Error(t, err)
	require.False(t, apperrors.IsNotFound(err))
}

func TestResolveTaskAccessDeactivatesStaleGrants(t *testing.T) {
	ctx := context.Background()
	grants := &stubTaskGrantRepo{grants: map[string]types.TaskGrant{
		taskGrantKey("s1", "member-2"): {TenantID: 1, TaskID: "s1", GranteeID: "member-2", Role: types.TaskGrantRoleViewer},
	}}
	// 成员已停用：grant 行仍在，但实时成员状态不再 active
	members := &stubTaskGrantMembers{members: map[string]*types.TenantMember{
		"member-2": {UserID: "member-2", TenantID: 1, Status: types.TenantMemberStatusDisabled},
	}}
	svc := newTaskGrantServiceForTest(grants, &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}, members)
	access, err := svc.ResolveTaskAccess(ctx, types.Caller{TenantID: 1, UserID: "member-2"}, "s1")
	require.NoError(t, err)
	require.Equal(t, types.TaskAccessNone, access.Role, "被停用成员的 grant 必须收敛（B3-F84，与 GetRunForGrantedReader 的实时 JOIN 一致）")
}
```

注：`apperrors.IsNotFound` 若该包无此谓词，用 `require.NotErrorIs(t, err, apperrors.ErrSessionNotFound)` 加 `require.False(t, errors.Is(err, apperrors.ErrNotFound))` 的等价组合（以 `internal/errors` 包实际导出为准；stub 沿用既有内嵌 `interfaces.SessionRepository` 的形态补齐空方法）。`types.TenantMemberStatusDisabled` 以 `internal/types` 实际枚举名为准（既有 activeMember 判定 :75-90 引用同一枚举）。

`internal/handler/commercial_task_budget_test.go` 追加（沿用本文件 `newBudgetGateDB` 的真实 sqlite 夹具——它只建了 `agent_runs(tenant_id, run_id, owner_id)`，本用例需补 sessions 表）：

```go
func TestBudgetExtensionGateUsesSessionOwnerAsAuthority(t *testing.T) {
	// 协作者创建了 run（agent_runs.owner_id = collaborator），任务真实 owner 是 sessions.user_id。
	// ADR-0004/TaskGrantStore 的 owner 权威：扩额门禁必须认 sessions.user_id（B3-F83）。
	db := newBudgetGateDB(t)
	require.NoError(t, db.Exec(`CREATE TABLE sessions (
		id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, user_id TEXT NOT NULL)`).Error)
	require.NoError(t, db.Exec(`ALTER TABLE agent_runs ADD COLUMN session_id TEXT NOT NULL DEFAULT 's1'`).Error)
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, user_id) VALUES ('s1', 7, 'real-owner')`).Error)
	require.NoError(t, db.Exec(`UPDATE agent_runs SET owner_id = 'collaborator-1' WHERE run_id = 'r1'`).Error)
	h := newBudgetGateHandlerForTest(t, db) // 沿用本文件既有 handler 构造 helper（按实际名引用）

	// 真实 owner（sessions.user_id）通过门禁
	w := performBudgetGateRequest(t, h, "real-owner", "r1")
	require.Equal(t, http.StatusOK, w.Code)

	// 协作者（agent_runs.owner_id）被拒——不得通过扩额门禁
	w2 := performBudgetGateRequest(t, h, "collaborator-1", "r1")
	require.Equal(t, http.StatusForbidden, w2.Code)
}
```

注：`newBudgetGateHandlerForTest`/`performBudgetGateRequest` 以本文件既有夹具的实际命名沿用（本文件已有「real owner 通过 / collaborator 拒绝」型用例，扩展点仅是 sessions 表与新断言方向）。若既有夹具的 agent_runs 建表语句在本用例中需含 `session_id` 列，直接在本用例内重建两表（CREATE TABLE IF NOT EXISTS 语义下避免污染共享 helper）。

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/application/service/ -run 'TestGrantTaskAccessSurfaces|TestResolveTaskAccess' -v`
Expected: FAIL（当前两处 `if err != nil` 一律 NewNotFoundError——IsNotFound 断言失败；停用成员当前拿到 TaskAccessViewer）。
Run: `go test ./internal/handler/ -run TestBudgetExtensionGateUsesSessionOwnerAsAuthority -v`
Expected: FAIL（当前 owner 谓词取 agent_runs.owner_id——real-owner 被 403/404、collaborator-1 通过）。

- [ ] **Step 3: 最小实现**

```go
// task_grant.go loadTaskForGrantManagement（:60-63）与 ResolveTaskAccess（:162-165）同型修改：
session, err := s.sessions.GetByID(ctx, caller.TenantID, taskID)
if err != nil {
	if errors.Is(err, apperrors.ErrSessionNotFound) {
		return nil, apperrors.NewNotFoundError("task not found")
	}
	return nil, err // 基础设施故障透传（仓库惯例：区分 miss 与 5xx）
}

// task_grant.go ResolveTaskAccess grant 命中分支（:171-181 区域）：
} else if found {
	access.GrantRole = role
	// grant 行存在 ≠ 授权有效：grantee 必须仍是 active 同租户成员
	// （与 GetRunForGrantedReader 的实时 tenant_members JOIN 收敛，B3-F84）。
	if member, mErr := s.members.Get(ctx, caller.UserID, caller.TenantID); mErr == nil && member != nil && member.Status == types.TenantMemberStatusActive {
		switch role {
		case types.TaskGrantRoleCollaborator:
			access.Role = types.TaskAccessCollaborator
		case types.TaskGrantRoleViewer:
			access.Role = types.TaskAccessViewer
		}
	}
}

// commercial_task_budget.go taskRunOwner —— owner 权威改 sessions.user_id（B3-F83）：
func (h *CommercialHandler) taskRunOwner(ctx context.Context, tenantID uint64, runID string) (string, bool, error) {
	if h == nil || h.db == nil || tenantID == 0 || runID == "" {
		return "", false, nil
	}
	var ownerID sql.NullString
	err := h.db.WithContext(ctx).Table("agent_runs").
		Select("sessions.user_id").
		Joins("JOIN sessions ON sessions.id = agent_runs.session_id AND sessions.tenant_id = ?", tenantID).
		Where("agent_runs.tenant_id = ? AND agent_runs.run_id = ?", tenantID, runID).
		Scan(&ownerID).Error
	if err != nil {
		return "", false, err
	}
	if !ownerID.Valid || strings.TrimSpace(ownerID.String) == "" {
		return "", false, nil
	}
	return ownerID.String, true, nil
}
```

顺手项（同文件，随本任务实现与验收）：
- F74：`task_grant_store.go:98-105` 的 `TaskOwnerID` 把 `Select("user_id")` 结果扫进 `sql.NullString`，NULL 时返回 `("", nil)`（ownerless 语义），不再扫进普通 `string`。
- F75：`UpsertGrant` 冲突更新路径返回前重读 DB 行（或 RETURNING），返回的 `created_at` 与 DB 一致；测试在 `task_grant_store_test.go` 追加「conflict path returns the persisted created_at」断言。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/application/service/ -run 'TestGrantTask|TestResolveTaskAccess' -v && go test ./internal/handler/ -run 'TestBudgetExtensionGate' -v`
Expected: PASS（新旧用例全绿）。
Run: `go test ./internal/application/service/ ./internal/application/repository/ ./internal/handler/`
Expected: ok（三包全绿）。

- [ ] **Step 5: Commit**

```bash
git add internal/application/service/task_grant.go internal/application/service/task_grant_test.go internal/handler/commercial_task_budget.go internal/handler/commercial_task_budget_test.go internal/application/repository/task_grant_store.go internal/application/repository/task_grant_store_test.go
git commit -m "fix(grants): surface infra errors, require active members, and gate budget by session owner"
```

---

### Task 3: Go：workbench 只读面授权与能力位（B3-F63、F76）

**Files:**
- Modify: `internal/handler/session/workbench_terminal_log.go:54`
- Modify: `internal/handler/session/workbench_artifacts.go:27-45`（构造期能力位）、`:128-130`（terminal.available）
- Modify: `internal/router/routes_workbench.go:145-152`（artifacts 装配传入 read 侧 reader，能力位同源）
- Test: `internal/handler/session/workbench_terminal_log_test.go`、`internal/handler/session/workbench_artifacts_test.go`

**Interfaces:**
- Consumes: `WorkbenchReadHandler.resolveReadableRun(c)`（workbench_read.go:206-234，owner 先行 + task-grant 回退，`WithGrantedRuns(reader)` 注入 `h.granted`）；`TerminalLogReader`（workbench_terminal_log.go:58-66 以 `h.snapshots.(TerminalLogReader)` 判定 501）。
- Produces: `GetWorkbenchTerminalLog` 改走 `h.resolveReadableRun(c)`——被授权 Viewer/Collaborator 在 snapshot 可读时同样拿到 200（无 granted 注入时保持 owner-only fail-closed，语义与 `TestGetWorkbenchTerminalLogIsOwnerScopedAndFailsClosed` 兼容）；`NewWorkbenchArtifactHandler` 增加可选构造参数 `terminal TerminalLogReader`，`terminal.available` = `h.terminal != nil`（构造期真值，不再硬编码 true）。

**根因与修复说明：** 同根因——只读面的授权与能力声明不诚实：(a) terminal-log 是纯 GET 只读面却用包级 `resolveOwnedRun(c, h.runs)`（terminal_log.go:54），同组 `GetWorkbenchExecution`/`GetWorkbenchSnapshot` 已迁移 `h.resolveReadableRun`（workbench_read.go:241/254）；snapshot 返回全部事件类型含完整 tool.terminal payload，owner-only 无保密收益只造成功能缺口（F63）。(b) `workbench_artifacts.go:130` 硬编码 `"available": true`，但 artifact handler 可在 read handler 未装配时独立挂载（`router.go:370-371` 两次独立装配、参数各自 optional——routes_workbench.go:42/149 均有 nil 判断），且 snapshots 未实现 `TerminalLogReader` 时 terminal-log 返回 501（terminal_log.go:58-66）——声明误导客户端展示必然失败的终端入口（F76）。

- [ ] **Step 1: 写失败测试**

`workbench_terminal_log_test.go` 追加（沿用本文件 `terminalLogContext(query)` 直调夹具与 `workbenchRunReaderStub` 形态；granted stub 参照 workbench_read.go 的 `GrantedRunReader` 接口 `GetRunForGrantedReader(ctx, tenantID, readerID, runID) (agentruntime.Run, error)`）：

```go
func TestGetWorkbenchTerminalLogAdmitsGrantedReaders(t *testing.T) {
	runs := &workbenchRunReaderStub{run: agentruntime.Run{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "run-1"},
		UserID:    "owner-1", // 业务属主；调用方是 viewer
		SessionID: "s1",
	}}
	granted := &stubGrantedRunReader{run: agentruntime.Run{ // owner 判 miss 后的 task-grant 回退命中
		Key: agentruntime.RunKey{TenantID: 1, RunID: "run-1"}, UserID: "viewer-1", SessionID: "s1",
	}}
	h := NewWorkbenchReadHandler(runs, &terminalReaderStub{terminal: terminalEvents()}).WithGrantedRuns(granted)

	c, w := terminalLogContext("")
	// 调用方身份是 viewer（非 owner）：以 workbench_read_test 的 caller 注入方式设置 user id
	setWorkbenchCaller(c, "viewer-1")
	h.GetWorkbenchTerminalLog(c)

	require.Equal(t, http.StatusOK, w.Code, "被授权 viewer 经 task-grant 回退必须能读 terminal-log（B3-F63）")
}
```

注：`stubGrantedRunReader`/`setWorkbenchCaller` 以 `workbench_read_test.go` 既有 granted stub 与 caller 注入 helper 的实际命名沿用（batch2 Task 1 已在 read 侧建立同型用例）；若无现成 helper，按 `workbench_read.go:207` 的 `workbenchCaller(c)` 读取的 context key 直接 `c.Request = c.Request.WithContext(context.WithValue(...))` 注入。

`workbench_artifacts_test.go` 追加：

```go
func TestListWorkbenchArtifactsDeclaresTerminalAvailabilityFromWiring(t *testing.T) {
	refs := &artifactRefReaderStub{refs: []workbench.ArtifactRef{ /* 沿用本文件既有 ref 夹具形态 */ }}
	runs := &workbenchRunReaderStub{run: agentruntime.Run{Key: agentruntime.RunKey{TenantID: 1, RunID: "run-1"}, UserID: "owner-1", SessionID: "s1"}}

	// 未接线 TerminalLogReader 的装配：terminal.available 必须是 false（B3-F76）
	h := NewWorkbenchArtifactHandler(runs, refs)
	c, w := artifactsListContext("owner-1") // 沿用本文件既有直调夹具
	h.ListWorkbenchArtifacts(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"terminal":{"available":false}`, "未接线时能力位必须如实为 false")

	// 接线后：available 为 true
	h2 := NewWorkbenchArtifactHandler(runs, refs).WithTerminalLog(&terminalReaderStub{})
	c2, w2 := artifactsListContext("owner-1")
	h2.ListWorkbenchArtifacts(c2)
	require.Equal(t, http.StatusOK, w2.Code)
	require.Contains(t, w2.Body.String(), `"terminal":{"available":true}`)
}
```

注：`artifactRefReaderStub`/`artifactsListContext` 以本文件既有夹具实际命名沿用；夹具注释处 `/* 沿用本文件既有 ref 夹具形态 */` 在执行时替换为至少一条真实 ref（使 items 非空，断言含 items 序列化）。

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/handler/session/ -run 'TestGetWorkbenchTerminalLogAdmits|TestListWorkbenchArtifactsDeclaresTerminal' -v`
Expected: FAIL（viewer 当前 404；`WithTerminalLog` 未定义导致编译失败也算预期 RED——先补 stub 方法使编译过、断言红）。

- [ ] **Step 3: 最小实现**

```go
// workbench_terminal_log.go:54 —— owner-only 改可读解析（与 GetWorkbenchExecution/Snapshot 同规则）
func (h *WorkbenchReadHandler) GetWorkbenchTerminalLog(c *gin.Context) {
	run, ok := h.resolveReadableRun(c)
	if !ok {
		return
	}
	// ... 其余保持（含 h.snapshots.(TerminalLogReader) 的 501 fail-closed）
}

// workbench_artifacts.go —— 构造期能力位：
type WorkbenchArtifactHandler struct {
	runs     OwnedRunReader
	refs     ArtifactRefReader
	terminal TerminalLogReader // nil = 本装配未提供只读终端（B3-F76）
	signingKey func() string
	ttl      time.Duration
}

func NewWorkbenchArtifactHandler(runs OwnedRunReader, refs ArtifactRefReader) *WorkbenchArtifactHandler {
	return &WorkbenchArtifactHandler{runs: runs, refs: refs, signingKey: workbench.ArtifactSigningKeyFromEnv, ttl: workbench.MaxArtifactGrantTTL}
}

// WithTerminalLog attaches the read-side terminal reader; the availability
// flag in ListWorkbenchArtifacts mirrors the actual wiring (nil = false).
func (h *WorkbenchArtifactHandler) WithTerminalLog(reader TerminalLogReader) *WorkbenchArtifactHandler {
	h.terminal = reader
	return h
}

// ListWorkbenchArtifacts 内 :128-130 ——
c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
	"items": items,
	"terminal": gin.H{"available": h.terminal != nil},
}})

// routes_workbench.go RegisterWorkbenchArtifactRoutes —— 装配处传入 read 侧 snapshots（与 terminal-log 501 判定同源）：
if artifacts != nil {
	executions := r.Group("/workbench/executions", g.Viewer())
	workbench := g.apiKeyGroup(executions, apiKeyChat(apiKeyFullAccess()))
	if readSnapshots != nil {
		artifacts = artifacts.WithTerminalLog(readSnapshots) // read handler 的装配参数（nil 判断已有先例）
	}
	workbench.GET("/:run_id/artifacts", artifacts.ListWorkbenchArtifacts)
	...
}
```

注：`RegisterWorkbenchArtifactRoutes` 现签名 `(r, artifacts, sessionHandler, g)`（routes_workbench.go:145）不含 read 侧 snapshots——装配点在 `internal/router/router.go:370-371`（两次独立装配调用处）把 read handler 的 snapshots 传入（增加一个参数或改在 router.go 装配后链式调用 `WithTerminalLog`； snapshots 同时实现了 `TerminalLogReader` 与否由类型断言天然裁决——用 `if tr, ok := snapshots.(session.TerminalLogReader); ok { artifacts = artifacts.WithTerminalLog(tr) }` 保持「未实现 TerminalLogReader → false」的诚实语义）。执行者按 router.go 现行装配代码选择链式注入点，行为断言（Step 1 的两个用例）不因注入方式而变。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/handler/session/ -run 'TestGetWorkbenchTerminalLog|TestListWorkbenchArtifacts|TestGetWorkbenchSnapshot|TestGetWorkbenchExecution' -v`
Expected: PASS（含既有 owner-scoped fail-closed 用例——无 granted 注入时 resolveReadableRun 仍 404）。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/session/workbench_terminal_log.go internal/handler/session/workbench_terminal_log_test.go internal/handler/session/workbench_artifacts.go internal/handler/session/workbench_artifacts_test.go internal/router/routes_workbench.go internal/router/router.go
git commit -m "fix(workbench): terminal-log admits granted readers and artifacts declares real terminal availability"
```

---

### Task 4: Go：agent-adoption 治理链（B3-F69、F86、F87；顺手 B3-F73）

**Files:**
- Modify: `internal/router/routes_agent_adoption.go:29`（available-agents 的 API-key 地板）
- Modify: `internal/handler/agent_adoption.go:231`（发布响应信封对齐）
- Modify: `internal/application/repository/agent_adoption.go:184-192`（ReplaceCapabilityMappings 的 state 守卫）、`decodeIDList`（顺手 F73）
- Test: `internal/router/routes_agent_adoption_test.go`、`internal/application/repository/agent_adoption_test.go`、`internal/handler/agent_adoption_test.go`（若无则新建，见 Step 1 注）

**Interfaces:**
- Consumes: `apiKeyReadAgents(apiKeyManageAgents(apiKeyChat(apiKeyFullAccess())))`（routes_agent.go:27 的镜像端点 OR 叠加模式）；`mustLookupAPIKeyPolicy(t, g, method, path)`（router_api_key_capabilities_test.go:664 的既有策略断言 helper，routes_agent_adoption_test.go:206 已在用）；`adoptionVariantDTO(view)`（handler 既有 DTO 映射）。
- Produces: `GET /marketplace/tenant/available-agents` 的 API-key 地板与 `GET /api/v1/agents` 一致（read_agents OR manage_agents OR chat OR full-access）；`PublishVariant` 响应 `data` 直接是 variant 本体（与 Adopt/CreateVariant/TestVariant/UpdateCapabilityMapping 四个端点一致）；`ReplaceCapabilityMappings` 的状态回写 UPDATE 带 `AND state IN ('draft','mapped','tested')` 守卫，RowsAffected≠1 时返回 `ErrAgentAdoptionStateConflict`（并发 CAS 落地后不再被静默打回）；`decodeIDList` 解析失败记 log 且返回空列表 + 错误信号（不再 `_ =` 静默吞）。

**根因与修复说明：** 同根因——adoption 治理链的端点契约与并发守卫不完整：(a) available-agents 注释自称「only the available-agent read model is Viewer+ (spec §2 mobile read-only surface)」（routes_agent_adoption.go:16-17），但 key 地板用 `admin`（= `apiKeyFullAccess()`，:29），持 read_agents/chat 能力的集成 key 被 403，与镜像端点 `GET /api/v1/agents`（routes_agent.go:27/51 的叠加 OR）不一致（F69）；(b) `PublishVariant` 的 data 是 `gin.H{"variant": adoptionVariantDTO(result.Variant)}` 双层信封（agent_adoption.go:231），同文件其余 Variant 端点直接返回本体（:211/:221），契约侧无匹配解析器（F86）；(c) `ReplaceCapabilityMappings` 的 UPDATE 仅按 tenant+id 匹配无条件覆盖 state（repository agent_adoption.go:184-186），并发 TestVariant/PublishVariant 的 CAS 落地后被静默打回 draft/mapped，published 戳残留导致 available-agents 读模型丢 Agent——违背 `UpdateVariantState` 的「fails loudly instead of double-applying」语义（F87，服务层前置检查 service:164-168 在事务外先读，TOCTOU）。顺手 F73：`decodeIDList` 用 `_ = json.Unmarshal` 忽略解析错误。F70/F71（资源存在性校验与 verdict）延期，见附录 A。

- [ ] **Step 1: 写失败测试**

`routes_agent_adoption_test.go` 追加（沿用 `newAgentAdoptionTestApp` + `mustLookupAPIKeyPolicy`）：

```go
func TestAvailableAgentsAPIKeyFloorMatchesAgentList(t *testing.T) {
	_, g, _ := newAgentAdoptionTestApp(t)
	policy := mustLookupAPIKeyPolicy(t, g, http.MethodGet, "/api/v1/marketplace/tenant/available-agents")
	// 镜像端点的 OR 语义：read_agents / manage_agents / chat / full-access 任一即可读（B3-F69）
	require.True(t, policy.Allows("read_agents"), "持 read_agents 能力的集成 key 必须可读移动只读面")
	require.True(t, policy.Allows("chat"))
	require.False(t, policy.Allows("knowledge:write"), "无关能力不得放行")
}
```

注：`policy.Allows(capability)` 以 `mustLookupAPIKeyPolicy` 返回的实际类型的方法为准（router_api_key_capabilities_test.go:664 起的同族用例展示了断言方式，执行者按其形态写等价断言——核心是 read_agents/chat 放行、无关能力拒绝）。

`internal/application/repository/agent_adoption_test.go` 追加（沿用本文件既有 sqlite 夹具）：

```go
func TestReplaceCapabilityMappingsRefusesToDowngradeConcurrentState(t *testing.T) {
	db := newAgentAdoptionRepoTestDB(t) // 沿用本文件既有 DB 夹具（按实际名引用）
	repo := NewAgentAdoptionRepository(db)
	// 夹具：variant 处于 published（模拟并发 PublishVariant 的 CAS 恰好落地）
	variant := seedVariant(t, db, types.AgentAdoptionVariantEntity{TenantID: 1, ID: "v1", AdoptionID: "a1", ReleaseID: "r1", State: "published", Name: "V"})

	err := repo.ReplaceCapabilityMappings(context.Background(), 1, "v1",
		[]types.AgentVariantCapabilityMappingEntity{{TenantID: 1, VariantID: "v1", Capability: "rag_qa", ModelID: "m1"}}, "mapped")

	require.ErrorIs(t, err, ErrAgentAdoptionStateConflict, "并发 CAS 落地后不得被静默打回（B3-F87）")
	after, _ := repo.GetVariant(context.Background(), 1, "v1")
	require.Equal(t, "published", after.State, "published 状态必须保持")
}
```

注：`newAgentAdoptionRepoTestDB`/`seedVariant` 以本文件既有夹具实际命名沿用；若夹具无 seedVariant，直接用 `db.Create(&types.AgentAdoptionVariantEntity{...})` 建行（本文件其他用例同型）。

`internal/handler/agent_adoption_test.go`（若无则新建，package handler，沿用 commercial_task_budget_test.go 的直调 gin 夹具形态）：

```go
func TestPublishVariantReturnsVariantBodyDirectly(t *testing.T) {
	// 治理链 stub：service 返回 published variant view（构造最小 AdoptionVariantView 夹具）
	h := NewAgentAdoptionHandler(&stubAdoptionService{publishResult: AdoptionVariantView{ID: "v1", State: "published", Name: "V"}})
	c, w := adoptionHandlerContext(http.MethodPost, "/api/v1/marketplace/tenant/variants/v1/publish", "admin-1")
	h.PublishVariant(c)
	require.Equal(t, http.StatusOK, w.Code)
	// data 直接是 variant 本体：不含 {variant: {...}} 双层信封（B3-F86）
	var body struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "v1", body.Data["id"])
	_, wrapped := body.Data["variant"]
	require.False(t, wrapped, "发布响应不得双层包裹——与同文件其余 Variant 端点一致")
}
```

注：`stubAdoptionService`/`adoptionHandlerContext` 为新建 helper（service 接口以 `NewAgentAdoptionHandler` 的参数类型 `*service.AgentAdoptionService` 的最小可替代形态为准；若 handler 直接持有具体类型而非接口，则改为在真实 service + sqlite 夹具上走 publish 全链路，断言不变）。

- [ ] **Step 2: 实跑确认失败**

Run: `go test ./internal/router/ -run TestAvailableAgentsAPIKeyFloorMatchesAgentList -v`
Expected: FAIL（当前 admin=full-access 地板——`policy.Allows("read_agents")` 为 false）。
Run: `go test ./internal/application/repository/ -run TestReplaceCapabilityMappingsRefusesToDowngrade -v`
Expected: FAIL（当前无条件覆盖——err 为 nil 且 state 变 mapped）。
Run: `go test ./internal/handler/ -run TestPublishVariantReturnsVariantBodyDirectly -v`
Expected: FAIL（data 含 "variant" 包裹）。

- [ ] **Step 3: 最小实现**

```go
// routes_agent_adoption.go:29 —— 移动只读面的 OR 地板（与 GET /api/v1/agents 镜像一致）
g.apiKeyRoute(r, http.MethodGet, "/marketplace/tenant/available-agents",
	apiKeyReadAgents(apiKeyManageAgents(apiKeyChat(apiKeyFullAccess()))), g.Viewer(), adoptionHandler.ListAvailableAgents)

// handler/agent_adoption.go:231 —— 去掉双层信封
c.JSON(http.StatusOK, gin.H{"success": true, "data": adoptionVariantDTO(result.Variant)})

// repository/agent_adoption.go ReplaceCapabilityMappings（:184-186）—— state 守卫：
updated := tx.Model(&types.AgentAdoptionVariantEntity{}).
	Where("tenant_id = ? AND id = ? AND state IN ?", tenantID, strings.TrimSpace(variantID), allowedRemapStates).
	Updates(map[string]any{"state": nextState, "updated_at": now})
if updated.Error != nil {
	return updated.Error
}
if updated.RowsAffected != 1 {
	// 服务层前置检查（事务外先读）与落地之间的并发 CAS：fails loudly（B3-F87）
	return ErrAgentAdoptionStateConflict
}

// 模块级守卫常量（与服务层 :164-168 的可重映射态一致）：
// var allowedRemapStates = []string{"draft", "mapped", "tested"}

// decodeIDList —— F73：解析失败不再静默
decoded, err := json.Unmarshal(...); if err != nil { /* 记 log（repo 已有 logger 先例则用之；否则保持返回空列表但错误进入调用方可观测的 debug 日志），返回 nil */ }
```

注：`ErrAgentAdoptionStateConflict` 已存在（service:165 使用）；repo 侧返回它需要 repository 包能引用（同一 `agent_adoption.go` 文件的模块级哨兵，确认定义位置在 repository 包；若哨兵定义在 service 包，则在 repository 包新增等价哨兵并由 service 在 `adoptionClientError` 中映射为 409 类响应——以现行 `ErrAgentAdoptionStateConflict` 的实际包归属为准，测试断言相应调整 `require.ErrorIs` 的目标）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `go test ./internal/router/ -run 'TestAvailableAgents|TestAgentAdoption' -v && go test ./internal/application/repository/ -run TestReplaceCapabilityMappings -v && go test ./internal/handler/ -run 'TestPublishVariant|TestAgentAdoption' -v`
Expected: PASS。
Run: `go test ./internal/router/ ./internal/application/repository/ ./internal/application/service/ ./internal/handler/`
Expected: ok（四包全绿）。

- [ ] **Step 5: Commit**

```bash
git add internal/router/routes_agent_adoption.go internal/router/routes_agent_adoption_test.go internal/handler/agent_adoption.go internal/handler/agent_adoption_test.go internal/application/repository/agent_adoption.go internal/application/repository/agent_adoption_test.go
git commit -m "fix(adoption): viewer-or read floor, unwrapped publish envelope, and CAS-guarded mapping rewrite"
```

---

### Task 5: domain：submission 重发的确定性终态（B3-F44；顺手 B3-F62）

**Files:**
- Modify: `packages/domain/src/mobile/submission.ts:126-144`（submit 的 catch）、`:186-210`（resume 的 catch 与 lookup 复用）

**Interfaces:**
- Consumes: `SubmissionTransport.start/lookup`（:85-90）；`SubmissionEntry.phase: 'awaiting_ack' | 'awaiting_reconciliation' | 'bound' | 'rejected'`；api-client `ApiError` 的 `{ status?: number }` 形态（跨包契约，仅鸭子类型探测）。
- Produces: `submit`/`resume` 的重发失败统一裁决：(a) `isDeterministicRefusal(cause)`（错误携带 `status` 且 `typeof status === 'number' && status >= 400 && status < 500`——api-client ApiError 的跨包形态，鸭子类型探测）→ 落 **rejected** 终态；(b) 其余经私有 helper `settleFailedDispatch(): Promise<SubmitOutcome>` 复查 `transport.lookup`：`state === 'rejected'` 落 rejected（服务端已处理但响应丢失），其余落 awaiting_reconciliation（网络失败保守语义不变）；(c) resume 的 `lookup.state !== 'unknown'` 分支直接用 lookup 结果落态（消除 F62 的二次 lookup）。rejected 语义与 `retryEntry`（:209-216「仅 rejected 允许；新意图=新 request_id」）衔接。Task 6/8 依赖此语义。

**根因与修复说明：** `resume()`（:186-210）与 `submit()`（:126-144）的重发 try/catch 逐字重复且无差别落 `awaiting_reconciliation`：服务端确定性 4xx 拒绝（预算不足等）也被当作「ACK 丢失」，恒定失败意图形成「重入（lookup=unknown）→ 重发 → 4xx → awaiting_reconciliation」循环，永远无法进 rejected 终态（F44——违背方法注释「仅当服务端明确 unknown 才重发」的守卫意图的对称面：重发失败也要能明确落终态）。顺手 F62：resume 在 `lookup.state !== 'unknown'` 时丢弃刚拿到的 lookup 结果转而调 `this.reconcile()`（内部再次 lookup）——双倍 RTT。

- [ ] **Step 1: 写失败测试（追加到 submission.test.ts）**

```ts
test('a deterministic 4xx dispatch failure lands rejected (the intent can reach a terminal state)', async () => {
  const store = createInMemorySubmissionStore();
  const transport: SubmissionTransport = {
    start: async () => { throw Object.assign(new Error('budget ceiling exceeded'), { status: 422 }); },
    lookup: async () => ({ state: 'unknown' as const }),
  };
  const coordinator = createSubmissionCoordinator(store, transport);
  const outcome = await coordinator.submit(input(), scope);
  assert.equal(outcome.entry.phase, 'rejected', '确定性 4xx 拒绝必须可达 rejected 终态（B3-F44）——不得永远 awaiting_reconciliation');
  // 受控重入（resume）读取同一终态：上层得以停止同 ID 重试引导
  const resumed = await coordinator.resume(input(), scope);
  assert.equal(resumed.entry.phase, 'rejected');
  assert.equal(resumed.dispatched, false);
});

test('a lost 4xx response settles rejected via the follow-up lookup, a transport failure stays reconciling', async () => {
  const store = createInMemorySubmissionStore();
  let lookupState: 'unknown' | 'rejected' = 'unknown';
  const transport: SubmissionTransport = {
    start: async () => { throw new Error('response lost'); }, // 网络/响应丢失（无 status 形态）
    lookup: async () => ({ state: lookupState }),
  };
  const coordinator = createSubmissionCoordinator(store, transport);
  const first = await coordinator.resume(input(), scope);
  assert.equal(first.entry.phase, 'awaiting_reconciliation', '无形态网络失败保持对账语义（既有行为不回归）');
  lookupState = 'rejected'; // 服务端其实已处理为拒绝
  const second = await coordinator.resume(input(), scope);
  assert.equal(second.entry.phase, 'rejected', 'lookup 反映 rejected 时经对账落终态');
  assert.equal(second.dispatched, false);
});

test('resume reuses its lookup result instead of looking it up twice', async () => {
  const store = createInMemorySubmissionStore();
  let lookups = 0;
  const transport: SubmissionTransport = {
    start: async () => { throw new Error('not used'); },
    lookup: async () => { lookups += 1; return { state: 'pending' as const }; },
  };
  const coordinator = createSubmissionCoordinator(store, transport);
  await coordinator.submit(input(), scope); // 首次提交落 awaiting（start 抛错路径走 helper，lookup 计 1 次）
  const before = lookups;
  await coordinator.resume(input(), scope); // resume 的非 unknown 分支：lookup 1 次（不再经 reconcile 双查）
  assert.equal(lookups, before + 1, 'B3-F62：resume 不得对同一 request_id 连查两次');
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/domain/src/mobile/submission.test.ts`
Expected: FAIL（第 1 个：phase 为 `awaiting_reconciliation`；第 3 个：lookups 多计一次）。

- [ ] **Step 3: 最小实现**

```ts
// submission.ts —— coordinator 级局部函数（submit 与 resume 共用，消除逐字重复）：
/** 确定性 4xx 判定：api-client ApiError 的 { status } 跨包形态（鸭子类型探测，非类型依赖）。 */
const isDeterministicRefusal = (cause: unknown): boolean => {
  const status = (cause as { status?: unknown } | null | undefined)?.status;
  return typeof status === 'number' && status >= 400 && status < 500;
};
const rejectedEntry = (): SubmissionEntry => ({ request_id: input.request_id, input_digest: digest, scope, phase: 'rejected', updated_at: new Date().toISOString() });
const reconcilingEntry = (): SubmissionEntry => ({ request_id: input.request_id, input_digest: digest, scope, phase: 'awaiting_reconciliation', updated_at: new Date().toISOString() });
/** 失败 dispatch 的后续裁决：确定性 4xx → rejected；否则复查 lookup（rejected → rejected，其余保持对账态）。 */
const settleFailedDispatch = async (): Promise<SubmitOutcome> => {
  const lookup = await transport.lookup(input.request_id).catch(() => undefined);
  const entry = lookup?.state === 'rejected' ? rejectedEntry() : reconcilingEntry();
  store.save(entry);
  return { entry, dispatched: true };
};

// submit（:129-144）与 resume（:192-207）的重发 try/catch 均改为同一形态：
try {
  const ack = await transport.start(input);
  /* 既有 ack.request_id 校验与 bound/awaiting_reconciliation 落库逻辑逐字保留 */
} catch (cause) {
  if (isDeterministicRefusal(cause)) {
    const entry = rejectedEntry();
    store.save(entry);
    return { entry, dispatched: true };
  }
  return settleFailedDispatch(); // ACK 丢失/网络失败：复查服务端是否已处理（响应丢失的 4xx 会在 lookup 反映 rejected）
}

// resume 的非 unknown 分支（:186-188）—— F62：直接用 lookup 结果，不再二次 lookup：
if (lookup.state !== 'unknown') {
  const existing = store.load(input.request_id)!; // 上方已断言存在
  const next: SubmissionEntry = lookup.state === 'admitted' && lookup.run_id
    ? { ...existing, phase: 'bound', run_id: lookup.run_id, updated_at: new Date().toISOString() }
    : lookup.state === 'rejected'
      ? { ...existing, phase: 'rejected', updated_at: new Date().toISOString() }
      : { ...existing, phase: 'awaiting_reconciliation', updated_at: new Date().toISOString() };
  store.save(next);
  return { entry: next, dispatched: false };
}
```

注：非 unknown 分支的落态映射与 `reconcile()`（:148-170）一致——执行时可直接复用 reconcile 的映射逻辑提取的共享函数（行为断言不变）；submit 与 resume 的成功路径（ack 一致 → bound）保持既有代码不动。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test packages/domain/src/mobile/submission.test.ts`
Expected: PASS（既有用例 + 新 3 个——「lost acknowledgement reconciles」等既有 office 侧行为在 Task 6 一并回归）。
Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/task-office-start.test.ts`
Expected: PASS（office 侧对 submit/resume 的消费未受扰动；若有用例依赖「422 落 awaiting」旧行为而红，按新语义修正该用例断言并在 commit message 注明）。

- [ ] **Step 5: Commit**

```bash
git add packages/domain/src/mobile/submission.ts packages/domain/src/mobile/submission.test.ts
git commit -m "fix(domain): deterministic 4xx dispatch failures settle rejected and resume reuses its lookup"
```

---

### Task 6: mobile-core：task-office start/恢复错误语义（B3-F78、F64、F79、F65；顺手 B3-F68）

**Files:**
- Modify: `packages/mobile-core/src/task-office/task-office.ts:465-505`（start 的记录回退/remove/失效）、`:505-523`（reconcilePending 隔离）、toStartInput 调用处（F68 注释）

**Interfaces:**
- Consumes: `submissionStore.load(requestId)` 返回 `SubmissionEntry`（含 `phase`/`run_id`/`scope`——进程内权威）；`receiptOf(entry, dispatched)`（本文件既有）；followUp 成功分支的失效动作集（:559-563：`listEpoch += 1; homeEpoch += 1; legacyListEpoch += 1; accumulated = undefined; legacyAccumulated = undefined;`——已核实）。
- Produces（Task 8/13/15 依赖的精确契约）:
  - `start(goal, { requestId })` 在 `intentLog.load` 缺记录但 `submissionStore` 有该 requestId 的 entry 时：`phase === 'bound' && run_id` → **零网络**返回 bound 回执（`dispatched: false`）；非 bound entry → 抛 `TASK_OFFICE_SUBMISSION_CONFLICT`（记录缺失无法安全重放）。
  - bound 成功后 `intentLog.remove` 的失败被吞（端口契约：记录留存幂等无害）；store entry scope 与当前 lease scope 不一致按无 entry 处理（走新意图路径并在 scope 字段不匹配时维持既有 CONFLICT 语义）。
  - start bound 成功后执行与 followUp 相同的失效动作集（Task 8 依赖：创建任务返回后 TasksScreen 的在途/缓存读不再吞掉新任务）。
  - `reconcilePending()` 单条记录失败不中断整批：返回已成功的 receipts（部分成功语义）。

**根因与修复说明：** 同根因——start() 成功路径的副作用次序与错误处理击穿幂等契约：(a) bound 后 `await intentLog.remove?.(requestId)`（:497）成功执行后，同 requestId 重放走 `record === undefined` → `createSession` 换 session（:478）→ 新 session_id 进 digest（:493）→ `SubmissionConflictError`（submission.ts:177-184）→ 上抛 `TASK_OFFICE_SUBMISSION_CONFLICT` 而非幂等回执，requestId 永久毒化（F78）；(b) remove 失败时原始错误经 catch 原样上抛（:494-503），把已 bound 的成功提交伪装成失败，上层（new-task-view:155-158 catch 不设 intentRequestId）以新 requestId 重试造成重复建任务（F64）。修复：记录缺失时先查 submissionStore（进程内权威的 bound 凭据），bound 直接回执；remove 改 best-effort。(c) bound 后不作废在途读与列表累积缓存（:497-498 无失效动作，对照 followUp :558-563）——TasksScreen 只在 mount 时 reload，用户创建任务返回后看不到新任务（F79）。(d) `reconcilePending` 循环无逐记录隔离（:510-520），毒记录中断整批且丢弃已收集 receipts（F65）。顺手 F68：`toStartInput(draft, { requestID, sessionId, targetId: 'platform', workspaceRef: '' })` 的两个业务字面量补契约注释（platform 目标与会话级空 workspaceRef 是移动 Start wire 的固定语义）。

- [ ] **Step 1: 写失败测试（追加到 task-office-start.test.ts）**

```ts
test('replaying a bound intent after its log record was removed returns the bound receipt with zero network', async () => {
  const posts: string[] = [];
  const sessions: string[] = [];
  const { backend, office } = startOffice({
    createSession: async () => { sessions.push('created'); return { sessionId: 'session-original' }; },
    start: async (input) => { posts.push(input.request_id); return { run_id: 'run-original', request_id: input.request_id, status: 'queued' }; },
    lookup: async () => ({ state: 'admitted', run_id: 'run-original' }),
  }, { ids: ['req-1'] });
  const first = await office.start(goal);
  assert.equal(first.phase, 'bound');
  // in-memory intentLog 的 remove 已在 bound 后执行（task-office.ts:497）——此刻记录缺失（B3-F78 前提）
  const second = await office.start(goal, { requestId: 'req-1' });
  assert.equal(second.phase, 'bound');
  assert.equal(second.runId, 'run-original');
  assert.equal(second.dispatched, false);
  assert.equal(posts.length, 1, '幂等重放零新 POST');
  assert.equal(sessions.length, 1, '幂等重放零 createSession——绝不换 session');
});

test('a failing intentLog remove after a bound start no longer masquerades as failure', async () => {
  const log = createInMemoryIntentLog();
  const originalRemove = log.remove!;
  let failNextRemove = true;
  log.remove = async (requestId: string) => {
    if (failNextRemove) { failNextRemove = false; throw new Error('secure store busy'); }
    return originalRemove(requestId);
  };
  const { office } = startOffice({}, { log });
  const receipt = await office.start(goal); // B3-F64：remove 抛错不得把 bound 成功伪装成失败
  assert.equal(receipt.phase, 'bound');
  assert.equal(receipt.runId, 'run-1'); // scenario backend 的缺省 run id（以夹具实际返回为准）
});

test('a bound start invalidates in-flight reads and the accumulated list cache', async () => {
  const { office } = startOffice({}, {});
  const first = await office.start(goal);
  assert.equal(first.phase, 'bound');
  // F79 对照 followUp（:558-563）：bound 后在途 tasks() 必须被 SUPERSEDED，下次 tasks() 重新查询
  await assert.rejects(office.tasks({}), (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SUPERSEDED');
  const page = await office.tasks({});
  assert.ok(Array.isArray(page.items));
});

test('reconcilePending skips a poisoned record and keeps the rest of the batch', async () => {
  const log = createInMemoryIntentLog();
  await log.save({ requestId: 'req-poison', sessionId: 's-x', goal, scope, persistedAt: '2026-09-24T00:00:00Z' });
  await log.save({ requestId: 'req-good', sessionId: 's-y', goal, scope, persistedAt: '2026-09-24T00:00:01Z' });
  const { office } = startOffice({
    lookup: async (requestId: string) => {
      if (requestId === 'req-poison') throw new Error('backend 500 for this record');
      return { state: 'unknown' };
    },
  }, { log });
  const receipts = await office.reconcilePending(); // B3-F65：毒记录不得让整批恢复 reject
  assert.ok(receipts.some((receipt) => receipt.requestId === 'req-good'));
});
```

注：`startOffice`/`goal`/`scope` 为本文件既有夹具（:14-33）；scenario backend 的 `createSession`/`start`/`lookup` handler 键名以 `createScenarioTaskBackend` 的实际参数形态为准（本文件既有用例已展示 `start:`/`lookup:` 键）。`run-1` 为 scenario 缺省——若不同则按夹具实际值断言。

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/task-office-start.test.ts`
Expected: FAIL（第 1 个：重入抛 `TASK_OFFICE_SUBMISSION_CONFLICT`；第 2 个：start reject `/secure store busy/`；第 3 个：tasks() 正常 resolve 无 SUPERSEDED；第 4 个：reconcilePending 整体 reject）。

- [ ] **Step 3: 最小实现**

```ts
// task-office.ts start() —— record === undefined 分支先查进程内权威（B3-F78/F64）：
const record = await intentLog.load(requestId);
if (record === undefined) {
  const settled = submissionStore.load(requestId);
  const sameScope = settled !== undefined
    && settled.scope.origin === scope.origin && settled.scope.tenantID === scope.tenantID && settled.scope.userID === scope.userID;
  if (sameScope && settled!.phase === 'bound' && settled!.run_id !== undefined) {
    // bound 后 intentLog 记录已被 remove 的幂等重放：store 是权威凭据，零网络返回
    return receiptOf(settled!, false);
  }
  if (sameScope) {
    // 记录缺失且非 bound：sessionId 只能从 intentLog 重建，绝不能换 session（D5）
    throw new TaskOfficeError('TASK_OFFICE_SUBMISSION_CONFLICT', { cause: new Error(`intent ${requestId} has no durable session record for replay`) });
  }
  // 真新意图：createSession → intentLog.save（既有路径保持）
  ...
}

// bound 分支（:497）—— remove 改 best-effort + 写失效（B3-F64/F79）：
const outcome = await submissions.resume(input, scope);
if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
if (outcome.entry.phase === 'bound') {
  try { await intentLog.remove?.(requestId); } catch { /* 端口契约：记录留存幂等无害——remove 失败不伪装失败 */ }
  // 创建了新任务：与 followUp/archive 同规则作废在途读与累积缓存（B3-F79）
  listEpoch += 1;
  homeEpoch += 1;
  legacyListEpoch += 1;
  accumulated = undefined;
  legacyAccumulated = undefined;
}
return receiptOf(outcome.entry, outcome.dispatched);

// reconcilePending（:510-520）—— 逐记录隔离（B3-F65）：
for (const record of records) {
  try {
    // 既有 per-record 逻辑（预建 entry → reconcile → bound 后 remove）
    if (entry.phase === 'bound') { try { await intentLog.remove?.(record.requestId); } catch { /* 同上 */ } }
    receipts.push(receiptOf(entry, false));
  } catch { /* 毒记录：跳过并继续——恢复是部分成功语义，单点失败不得卡死整批 */ }
}

// toStartInput 调用处（start/reconcilePending 两处）—— F68 契约注释：
// targetId 'platform' 与 workspaceRef '' 是移动 Start wire 的固定业务语义（平台级目标、
// 无工作区限定——与 miniprogram 先例一致），非可配置参数。
```

注：`submissionStore` 变量名以本文件实际为准（startOffice 夹具的 `options.store` 注入点已存在，`:24-26` 可见 `submissionStore` 装配）；`settled.run_id` 的非空断言按文件 lint 风格改写。store 命中分支同样位于 `requireLease()` 之后（函数入口已校验），无需重复。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/task-office-start.test.ts`
Expected: PASS（既有 + 新 4 个）。
Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/`
Expected: PASS（同包全绿：task-office/legacy-office/attention-inbox 等消费侧无回归）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/task-office/task-office.ts packages/mobile-core/src/task-office/task-office-start.test.ts
git commit -m "fix(task-office): replay bound intents from the store, contain remove failures, invalidate reads, isolate poisoned reconciles"
```

---

### Task 7: apps/mobile：意图/草稿持久化防御（B3-F51、F38、F42）

**Files:**
- Modify: `apps/mobile/src/adapters/intent-log.ts:48-57`（单条超限显式失败）、`:65-85`（save/remove 串行互斥）
- Modify: `apps/mobile/src/new-task-drafts.ts:16-30`（字段级防御）
- Test: `apps/mobile/src/adapters/intent-log.test.ts`、`apps/mobile/src/new-task-drafts.test.ts`

**Interfaces:**
- Consumes: `SecureStorePort`；`serializeWithinBudget(records)`（intent-log.ts:48-57）；`TaskOfficeError`（`@weknora/mobile-core` 导出——apps/mobile 已依赖该包）。
- Produces: `createSecureIntentLog` 的 `save/remove` 经模块内串行队列（promise chain）互斥——并发 save 后写不再覆盖前写（F51）；`save` 在单条记录序列化后仍超 `INTENTS_BUDGET_BYTES` 时抛 `TaskOfficeError('TASK_OFFICE_INVALID_INPUT', { cause: new Error('intent record exceeds the SecureStore budget; reduce goal text or attachments') })` 而非把底层 `setItemAsync` 错误直接上抛（F38——office.start 的既有 catch 对 TaskOfficeError 原样上抛，上层拿到可行动错误码；Task 8 接管文案）。`createScopedNewTaskDrafts.load()` 返回字段级修复后的草稿：`text` 非字符串 → `''`；`agentId` 非 string/null → `null`；`budgetUpper` 非安全整数 → `0`；`attachments`/`knowledgeIds` 非数组 → `[]`；数组内畸形条目过滤为 `[]`（脏类型条目整组丢弃，保守 fail-closed）——脏草稿不再让 `NewTaskScreen` 整屏崩溃（F42）。

**根因与修复说明：** 同根因——SecureStore 适配层的完整性防御缺失：(a) `save`（:68-72）与 `remove`（:79-83）是无互斥的 read-modify-write，两个并发 start 的 save 后写整包覆盖前写丢意图记录，恰好击穿「崩溃后幂等恢复」核心保证；实例经 `composition.ts:41-43` 的 `intentLogOf()` 模块级单例跨全部 origin 共享（F51）。(b) `serializeWithinBudget` 仅在 `records.size > 1` 时裁剪（:50），单条超限时超限 json 直接 `setItemAsync` 抛底层错误；`NewTaskScreen.tsx:6-13` 的 maxLength=500 论证只按中文 1500B 估算，漏算 knowledgeIds（UUID 数组）与 attachments——合法组合越过 2048B 硬限后提交永久失败且无自愈路径（F38；office 侧 `task-office.ts:481` 的 `intentLog.save` 含 startGoal 全字段，save 失败 = 零 Start 派发是必须保持的语义，所以拦截点在「显式、可行动」而非「静默截断」——截断破坏 goalKeyOf digest，intent-log.ts:10-13 注释已声明）。(c) `load()` 的 `as NewTaskDraft` 断言不设防（new-task-drafts.ts:25），注释声称的 evaluateSubmitReadiness 兜底不成立——该函数直接 `draft.text.trim()`/`attachments.filter()` 且不触及 knowledgeIds（F42）。

- [ ] **Step 1: 写失败测试（追加）**

`intent-log.test.ts`：

```ts
test('concurrent saves serialize instead of overwriting each other (B3-F51)', async () => {
  const secure = memoryStore();
  const log = createSecureIntentLog(secure);
  await Promise.all([
    log.save(record),
    log.save({ ...record, requestId: 'req-2', sessionId: 'session-78' }),
  ]);
  assert.equal((await log.load('req-1'))?.sessionId, 'session-77', '先写记录不得被后写整包覆盖丢失');
  assert.equal((await log.load('req-2'))?.sessionId, 'session-78');
});

test('a single record beyond the budget fails explicitly instead of hitting the SecureStore limit (B3-F38)', async () => {
  const secure = memoryStore();
  const log = createSecureIntentLog(secure);
  const hugeGoal = {
    text: '长'.repeat(500), // 500 中文 ≈1500B
    agentId: 'a-1', budgetUpper: 200,
    knowledgeIds: Array.from({ length: 24 }, (_unused, index) => `kb-${index}-0123456789abcdef0123456789abcdef`), // 24×39 ≈ 936B
    attachments: [],
  };
  await assert.rejects(
    log.save({ requestId: 'req-big', sessionId: 'session-1', goal: hugeGoal, scope, persistedAt: '2026-09-24T00:00:00Z' }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT',
    '单条超限必须显式抛可行动错误码，而不是 setItemAsync 底层错误让提交永久失败',
  );
  assert.equal(secure.dump(), null, '超限记录不得落盘（部分状态）');
});
```

（顶部 import 追加 `import { TaskOfficeError } from '@weknora/mobile-core';`。）

`new-task-drafts.test.ts`（沿用本文件既有 memory store 夹具形态）：

```ts
test('a corrupted draft degrades field-by-field instead of crashing the screen (B3-F42)', async () => {
  const store = memoryScopedStore(); // 沿用本文件既有 { get, put } 夹具（按实际名引用）
  await store.put({ id: 'new-task', body: JSON.stringify({ text: 42, attachments: 'not-an-array', knowledgeIds: null, budgetUpper: 'x' }) });
  const drafts = createScopedNewTaskDrafts(store);
  const draft = await drafts.load();
  assert.deepEqual(draft, { text: '', agentId: null, budgetUpper: 0, attachments: [], knowledgeIds: [] }, '脏字段逐项降级，evaluateSubmitReadiness 不再抛 TypeError');
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/adapters/intent-log.test.ts apps/mobile/src/new-task-drafts.test.ts`
Expected: FAIL（并发覆盖：req-1 读回 undefined 或被 req-2 覆盖后的快照缺失——按交错而定，断言不稳绿；超大单条：当前落底层错误或成功写入超限 json——`secure.dump()` 非空；脏草稿：load 返回脏对象，deepEqual 失败）。

- [ ] **Step 3: 最小实现**

```ts
// intent-log.ts —— 串行互斥 + 单条超限显式失败：
import { TaskOfficeError } from '@weknora/mobile-core';

export function createSecureIntentLog(store: SecureStorePort): SubmissionIntentLog {
  let queue: Promise<unknown> = Promise.resolve();
  const serialized = <T>(action: () => Promise<T>): Promise<T> => {
    const run = queue.then(action, action); // 前次失败不阻塞后续
    queue = run.then(() => undefined, () => undefined);
    return run;
  };
  const readAll = async (): Promise<Map<string, SubmissionIntentRecord>> => parseRecords(await store.getItemAsync(INTENTS_KEY));
  return {
    async save(record) {
      return serialized(async () => {
        const records = await readAll();
        records.set(record.requestId, record);
        const json = serializeWithinBudget(records);
        if (new TextEncoder().encode(json).length > INTENTS_BUDGET_BYTES) {
          // 单条即超限：源头拦截（显式可行动），不截断（截断破坏 goalKeyOf digest——见 :10-13 注释）
          throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT', { cause: new Error('intent record exceeds the SecureStore budget; reduce goal text or attachments') });
        }
        await store.setItemAsync(INTENTS_KEY, json);
      });
    },
    async load(requestId) { return (await readAll()).get(requestId); },
    async listScope(scope) { return [...(await readAll()).values()].filter((record) => sameScope(record.scope, scope)); },
    async remove(requestId) {
      return serialized(async () => {
        const records = await readAll();
        if (!records.delete(requestId)) return;
        await store.setItemAsync(INTENTS_KEY, serializeWithinBudget(records));
      });
    },
  };
}
// serializeWithinBudget 改为纯函数返回 json（不再内部消化超限——由 save 判定抛错）。

// new-task-drafts.ts load() —— 字段级防御：
async load() {
  const entry = await store.get(NEW_TASK_DRAFT_ID).catch(() => undefined);
  if (!entry) return undefined;
  let value: unknown;
  try { value = JSON.parse(entry.body); } catch { return undefined; }
  if (typeof value !== 'object' || value === null) return undefined;
  const row = value as Partial<NewTaskDraft> & Record<string, unknown>;
  const cleanArray = <T>(candidate: unknown): T[] => (Array.isArray(candidate) ? candidate as T[] : []);
  const draft: NewTaskDraft = {
    text: typeof row.text === 'string' ? row.text : '',
    agentId: typeof row.agentId === 'string' ? row.agentId : null,
    budgetUpper: typeof row.budgetUpper === 'number' && Number.isSafeInteger(row.budgetUpper) && row.budgetUpper >= 0 ? row.budgetUpper : 0,
    attachments: cleanArray(row.attachments),
    knowledgeIds: cleanArray(row.knowledgeIds),
  };
  return draft;
}
```

注：`NewTaskDraft` 的字段集以 `@weknora/domain/mobile` 的实际定义为准（本计划作者读到的 `emptyDraft()` 构造含 text/agentId/budgetUpper/attachments/knowledgeIds 五字段——new-task-view.ts:43）；若类型含更多必填字段，按同型防御补齐。`load`/`listScope` 保持无互斥读（read-only，无覆盖风险）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test apps/mobile/src/adapters/intent-log.test.ts apps/mobile/src/new-task-drafts.test.ts`
Expected: PASS（既有 + 新增）。
Run: `pnpm exec tsx --test apps/mobile/src/new-task-view.test.ts packages/mobile-core/src/task-office/task-office-start.test.ts`
Expected: PASS（office→adapter 链路无回归）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/adapters/intent-log.ts apps/mobile/src/adapters/intent-log.test.ts apps/mobile/src/new-task-drafts.ts apps/mobile/src/new-task-drafts.test.ts
git commit -m "fix(mobile): serialize intent-log writes, refuse oversized records explicitly, and sanitize drafts field-by-field"
```

---

### Task 8: apps/mobile：new-task 控制器生命周期（B3-F37、F41、F40、F59、F39；顺手 B3-F46）

**Files:**
- Modify: `apps/mobile/src/new-task-view.ts:60`（intentRequestId 回填）、`:70-73`（recommendation 求值）、`:74-101`（初始化竞态与 agents 兜底）、`:126-159`（submit 的 rejected 分支）
- Modify: `apps/mobile/src/screens/NewTaskScreen.tsx:39-49`（loading 禁用 + F46 预算中间态）、`:58`（inFlight/rejected 文案）、`:6-13`（F38 预算论证注释更新）
- Test: `apps/mobile/src/new-task-view.test.ts`

**Interfaces:**
- Consumes: Task 6 的契约（bound 重放幂等、remove 失败包含）；Task 7 的契约（单条超限 → `TASK_OFFICE_INVALID_INPUT`）；`TaskStartReceipt.phase: ... | 'rejected'`；既有 `officeDouble`/`memoryDrafts`/`agents()` 夹具（new-task-view.test.ts:10-38）。
- Produces: `NewTaskViewState` 不变（`inFlight` 仅承载**未决**意图）；控制器内部：(a) `initialized` 发现 unresolved 收据时回填 `intentRequestId = unresolved.requestId`（F37——跨重启重试同 ID）；(b) `submit` 收到 `rejected` 收据时不设 inFlight/intentRequestId，发布 `error = SUBMISSION_REJECTED_COPY`（F41——终态不驱动同 ID 重试循环）；(c) `update/setAttachments/toggleKnowledge` 置 `draftDirty`，initialized 完成时 dirty 用户输入优先于 `stored ?? emptyDraft()`（F40）；(d) `ports.agents()` 失败按空目录降级，初始化不再整体失败（F59）；(e) `project()` 的 recommendation 以合并后 agents 求值（F39）。`NewTaskScreen`：loading 期间 TextInput `editable={false}`、按钮 `disabled`；预算输入本地中间态（F46——`''` 与 0 分离）；`GOAL_TEXT_MAX_LENGTH` 注释更新为含 knowledgeIds/attachments 的预算说明（F38 UI 侧）。

**根因与修复说明：** 同根因——控制器对意图生命周期与初始化时序的把控缺失：(a) `initialized`（:86-96）只写 `state.inFlight` 不设 `intentRequestId`（:60 仍 undefined），用户重试时 `submit` 走 `intentRequestId === undefined ? {} : ...`（:143）——同一意图以新 request_id 重新派发，违反 D5（F37；NewTaskScreen.tsx:58 文案承诺 retrying keeps the same request id）。(b) `submit` 的 else 分支（:150-153）不排除 `rejected`——服务端 lookup 只会维持 rejected，用户被引导进永不成功的重试循环（F41；对照 :86 的 find 已排除 rejected、submission.ts:209-216 retryEntry 注释「新意图=新 request_id」）。(c) `initialized` 需等 drafts.load+agents+knowledge+reconcilePending 多个网络调用（秒级窗口），期间 `update()` 写入的 draft 被 `draft: stored ?? emptyDraft()` 无条件覆盖（:84/89——draft 字段在 `...state` 之后），输入静默丢失；且 loading 期间输入控件全部可交互（NewTaskScreen.tsx:29 loading 仅切换标题、:39/41 TextInput 无 disabled）（F40）。(d) `Promise.all` 中唯独 `ports.agents()` 无 `.catch`（:76-80——drafts/knowledge 均有），离线时整体进 catch（:97-99），draft 停留 emptyDraft，用户后续输入经 persistDraft 覆盖 NEW_TASK_DRAFT_ID——已保存草稿永久丢失（F59）。(e) `project()` 的 `recommendLeadAgent(state.agents)`（:72）读闭包旧值而 `...patch` 已覆盖 agents 字段——refreshAgents 成功后 recommendation 不刷新，「目录刷新」承诺失效（F39）。顺手 F46：预算输入 `state.draft.budgetUpper === 0 ? '' : String(...)` 与 `!^\d+$ → 0` 双向折叠，非纯数字输入中间态被静默置 0 并渲染为空。

- [ ] **Step 1: 写失败测试（追加到 new-task-view.test.ts）**

```ts
test('a recovered unresolved intent is retried with the same request id (D5)', async () => {
  const office = officeDouble(undefined);
  office.reconcilePending = async () => [{ requestId: 'req-recovered', phase: 'awaiting_reconciliation', dispatched: false }];
  const controller = createNewTaskController({ office, agents: async () => agents(), newRequestId: () => 'req-new' });
  await controller.whenInitialized();
  assert.equal(controller.state().inFlight?.requestId, 'req-recovered');
  controller.update({ text: '整理周报' });
  controller.update({ agentId: 'a-general' });
  await controller.submit();
  const last = office.calls[office.calls.length - 1]!;
  assert.equal(last.options?.requestId, 'req-recovered', 'B3-F37：跨重启恢复的意图重试必须同 ID，绝不换 ID 重建任务');
  controller.dispose();
});

test('a rejected receipt is terminal: no inFlight, no same-id retry', async () => {
  const office = officeDouble(async () => ({ requestId: 'req-1', phase: 'rejected', dispatched: true }));
  const controller = createNewTaskController({ office, agents: async () => agents(), newRequestId: () => 'req-new' });
  await controller.whenInitialized();
  controller.update({ text: '整理周报', agentId: 'a-general' });
  const receipt = await controller.submit();
  assert.equal(receipt?.phase, 'rejected');
  const state = controller.state();
  assert.equal(state.inFlight, undefined, 'B3-F41：rejected 终态不得驱动同 ID 重试引导');
  assert.ok(state.error !== undefined && state.error.includes('拒绝'), '错误文案提示服务端拒绝');
  controller.update({ text: '修改后的目标' }); // 用户修改后重试 = 新意图 = 新 request_id
  await controller.submit();
  const last = office.calls[office.calls.length - 1]!;
  assert.equal(last.options?.requestId, undefined, 'rejected 后的新提交不携带旧 requestId');
  controller.dispose();
});

test('agents() failing offline must not lose the stored draft', async () => {
  const drafts = memoryDrafts();
  await drafts.save({ text: '离线时的重要草稿', agentId: 'a-coding', budgetUpper: 50, attachments: [], knowledgeIds: [] });
  const controller = createNewTaskController({ office: officeDouble(), agents: async () => { throw new Error('offline'); }, drafts, newRequestId: () => 'req-9' });
  await controller.whenInitialized();
  const state = controller.state();
  assert.equal(state.loading, false, 'B3-F59：agents 失败不得让初始化整体失败');
  assert.equal(state.draft.text, '离线时的重要草稿', '已保存草稿不被 emptyDraft 覆盖');
  controller.dispose();
});

test('user input during the loading window survives initialization', async () => {
  let releaseAgents: (() => void) | undefined;
  const agentsPromise = new Promise<AgentOption[]>((resolve) => { releaseAgents = () => resolve(agents()); });
  const controller = createNewTaskController({ office: officeDouble(), agents: () => agentsPromise, newRequestId: () => 'req-9' });
  controller.update({ text: '秒级窗口内输入的目标' }); // 初始化未完成
  releaseAgents?.();
  await controller.whenInitialized();
  assert.equal(controller.state().draft.text, '秒级窗口内输入的目标', 'B3-F40：初始化完成不得覆盖用户已输入内容');
  controller.dispose();
});

test('refreshAgents recomputes the recommendation from the fresh catalog', async () => {
  let current: AgentOption[] = [];
  const controller = createNewTaskController({ office: officeDouble(), agents: async () => current, newRequestId: () => 'req-9' });
  current = agents();
  await controller.refreshAgents();
  assert.deepEqual(controller.state().recommendation, recommendLeadAgent(agents()), 'B3-F39：目录刷新后推荐必须更新');
  controller.dispose();
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/new-task-view.test.ts`
Expected: FAIL（第 1 个：`options.requestId` 为 undefined；第 2 个：`inFlight` 为 rejected 收据；第 3 个：loading 停 true 且草稿被清；第 4 个：text 为空（被 `stored ?? emptyDraft()` 覆盖）；第 5 个：recommendation 仍为空目录的 no_supported_agent）。

- [ ] **Step 3: 最小实现**

```ts
// new-task-view.ts
export const SUBMISSION_REJECTED_COPY = '上次提交已被服务端拒绝；修改目标后重新提交将使用新的请求标识。';

export function createNewTaskController(ports: NewTaskControllerPorts): NewTaskController {
  let state: NewTaskViewState = { /* 既有初始值 */ };
  let intentRequestId: string | undefined;
  let draftDirty = false; // B3-F40：loading 窗口内的用户编辑
  ...
  const project = (patch: Partial<NewTaskViewState> & { draft?: NewTaskDraft }): void => {
    const draft = patch.draft ?? state.draft;
    const agents = patch.agents ?? state.agents; // B3-F39：以合并后目录求推荐
    publish({ ...state, ...patch, draft, readiness: evaluateSubmitReadiness(draft), recommendation: recommendLeadAgent(agents) });
  };
  const initialized = (async (): Promise<void> => {
    try {
      const [stored, agents, knowledge] = await Promise.all([
        ports.drafts?.load().catch(() => undefined),
        ports.agents().catch(() => [] as const), // B3-F59：目录是可降级输入（对照 knowledge 先例）
        ports.knowledge?.().catch(() => [] as const) ?? ([] as const),
      ]);
      let pending: TaskStartReceipt[] = [];
      try { pending = await ports.office.reconcilePending(); } catch { /* 恢复失败不阻塞新建流 */ }
      if (disposed) return;
      const storedDraft = stored ?? emptyDraft();
      const draft = draftDirty
        ? { ...state.draft, agentId: state.draft.agentId ?? recommendLeadAgent(agents).agent?.id ?? null } // 用户编辑优先，agentId 兜底推荐
        : (storedDraft.agentId === null ? { ...storedDraft, agentId: recommendLeadAgent(agents).agent?.id ?? null } : storedDraft);
      const unresolved = pending.find((receipt) => receipt.phase !== 'bound' && receipt.phase !== 'rejected');
      if (unresolved !== undefined) intentRequestId = unresolved.requestId; // B3-F37：回填——重试同 ID
      publish({ ...state, draft, agents, knowledge, loading: false, readiness: evaluateSubmitReadiness(draft), recommendation: recommendLeadAgent(agents),
        ...(unresolved === undefined ? {} : { inFlight: unresolved }) });
    } catch (cause) { /* 既有 catch 保持 */ }
  })();
  return {
    ...
    update(patch) { draftDirty = true; project({ draft: { ...state.draft, ...patch } }); persistDraft(); },
    setAttachments(attachments) { draftDirty = true; project({ draft: { ...state.draft, attachments: [...attachments] } }); persistDraft(); },
    toggleKnowledge(knowledgeId) { draftDirty = true; /* 既有逻辑 */ },
    ...
    async submit() {
      const readiness = evaluateSubmitReadiness(state.draft);
      if (!readiness.ready || state.submitting) { project({ readiness }); persistDraft(); return undefined; }
      publish({ ...state, submitting: true, error: undefined });
      try {
        const receipt = await ports.office.start({ /* 既有 goal 构造 */ }, intentRequestId === undefined ? {} : { requestId: intentRequestId });
        if (receipt.phase === 'bound' && receipt.runId !== undefined) {
          intentRequestId = undefined; draftDirty = false;
          /* 既有 bound 清空逻辑 */
        } else if (receipt.phase === 'rejected') {
          intentRequestId = undefined; // B3-F41：终态——新意图=新 request_id
          publish({ ...state, submitting: false, inFlight: undefined, error: SUBMISSION_REJECTED_COPY });
        } else {
          intentRequestId = receipt.requestId;
          publish({ ...state, submitting: false, inFlight: receipt });
        }
        return receipt;
      } catch (cause) { /* 既有 catch 保持（INVALID_INPUT 等） */ }
    },
  };
}
```

```tsx
// NewTaskScreen.tsx
// :6-13 注释更新（B3-F38 UI 侧）：预算论证改为「500 字中文 ≈1500B + knowledgeIds/attachments
// 逐项计入单条记录；超限时 Task 7 的存储层显式拒绝（TASK_OFFICE_INVALID_INPUT），源头双保险」。
export function NewTaskScreen({ state, onUpdate, ... }: NewTaskScreenProps) {
  const [budgetText, setBudgetText] = useState(state.draft.budgetUpper === 0 ? '' : String(state.draft.budgetUpper)); // B3-F46
  useEffect(() => { if (state.draft.budgetUpper === 0) setBudgetText(''); }, [state.draft.budgetUpper]); // bound 清空后同步
  ...
  <TextInput value={state.draft.text} onChangeText={(text) => { onUpdate({ text }); }} placeholder="今天想完成什么？" multiline maxLength={GOAL_TEXT_MAX_LENGTH} editable={!state.loading} />
  <TextInput
    value={budgetText}
    onChangeText={(text) => { setBudgetText(text); onUpdate({ budgetUpper: /^\d+$/.test(text) ? Number(text) : 0 }); }}
    placeholder="预算上限（Credits，可留空）" keyboardType="numeric" editable={!state.loading}
  />
  {/* 知识/Agent 按钮与 Submit 在 state.loading 时 disabled */}
  {state.inFlight !== undefined && <Text>{`Unresolved submission ${state.inFlight.requestId} (${state.inFlight.phase}) — retrying keeps the same request id`}</Text>}
```

注：`useState`/`useEffect` 经 react 导入（NewTaskScreen 目前未用 hooks——追加 `import { useEffect, useState } from 'react';`；app-smoke 的 react stub 已支持 hooks，batch2 Task 6 先例）。

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test apps/mobile/src/new-task-view.test.ts`
Expected: PASS（既有 + 新 5 个）。
Run: `pnpm exec tsx --test apps/mobile/src/new-task-drafts.test.ts apps/mobile/src/app-smoke.test.tsx`
Expected: PASS（app-smoke 若有 NewTaskScreen 源级/渲染断言需同步新 prop 语义——新增 editable/disabled 不破坏既有断言；红则按新行为修断言并注明）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/new-task-view.ts apps/mobile/src/new-task-view.test.ts apps/mobile/src/screens/NewTaskScreen.tsx
git commit -m "fix(mobile): retry recovered intents with the same id, treat rejected as terminal, and protect loading-window input"
```

---

### Task 9: mobile-core：inbox/device 投影一致性（B3-F1、F5；顺手 B3-F2、F6、F50）

**Files:**
- Modify: `packages/mobile-core/src/inbox/notification-inbox.ts:115-136`（merge 保持已读投影 + F2 空串游标归一）、`:150-166`（markRead 记录已读集合）
- Modify: `packages/mobile-core/src/device/device-registry.ts:129-133`（revoke 409 映射）、register 响应校验（F6）、attemptRegister 迟到成功复检（F50）
- Test: `packages/mobile-core/src/inbox/notification-inbox.test.ts`、`packages/mobile-core/src/device/device-registry.test.ts`

**Interfaces:**
- Consumes: 既有 `sequence` 票据（fetchPage :139 `++sequence`）与 `merge(page, reset)`；`createScenarioDeviceRemote`（device-registry.test.ts:15-21 夹具，`scenario.remote` 可注入 409——`conflictNextRegisters` 先例）。
- Produces: inbox 模块内维护 `markedRead: Set<string>`——`markRead` 成功后记录 id；`merge` 重建 items 时对 `markedRead` 命中的行强制 `read: true`（在途旧快照 settle 不再回退已读投影，F1）；`page.nextCursor === ''` 归一为 undefined（F2）。device：`revoke` 的 catch 对 `wireStatus(error) === 409` 抛 `DEVICE_CONFLICT`（与 register 路径 :113 对称，F5）；register 响应的 `revision`/`scope_generation` 经 `Number.isSafeInteger` 校验，脏值抛 `DEVICE_BACKEND`（F6）；`attemptRegister` 成功返回前复检 lease（F50——迟到成功不越 scope 返回）。

**根因与修复说明：** 同根因——写操作/响应在投影与错误映射上绕过既有防护：(a) `markRead`（:150-166）是模块唯一不参与 sequence 票据的路径——在途 `page()`/`more()` 的旧快照 settle 时经 `merge`（:115-136）整包重建 items，把已读行回退为未读（F1；比给 markRead 领票更根本的修复是「已读是本地投影事实」，GET 快照只能携带服务端视角，投影融合在 merge）。(b) `revoke` catch 仅映射 404（:131），服务端 `ErrMobileDeviceRevision` 返回 409（mobile_device.go:404-405，in-memory 替身复刻）——与 register 路径 :113 的 409→DEVICE_CONFLICT 不对称，调用方无法对冲突做重取重试（F5）。

- [ ] **Step 1: 写失败测试（追加）**

`notification-inbox.test.ts`（自定义延迟 remote stub——绕开 scenario 夹具的立即 resolve）：

```ts
test('an in-flight page snapshot does not roll back a read projection', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  let releaseFirst: (() => void) | undefined;
  const remote: InboxRemote = {
    inbox: () => new Promise((resolve) => { releaseFirst = () => resolve({ items: [{ notificationId: 'n-1', read: false }], unreadCount: 1 }); }),
    markRead: async () => {},
  };
  const inbox = createNotificationInbox({ remote, lease: () => leaseRef.lease });
  const pending = inbox.page();      // 在途读（慢）
  await new Promise((resolve) => setImmediate(resolve));
  await inbox.markRead('n-1');       // 已读先行落地
  releaseFirst?.();                  // 旧快照迟到 settle（B3-F1 前提）
  const view = await pending;
  assert.equal(view.items[0]!.read, true, '在途旧快照不得把已读行回退为未读');
});
```

（顶部 import 追加 `import type { InboxRemote } from './notification-inbox.ts';`——该类型已从模块导出，test 文件 :7 先例。）

`device-registry.test.ts`：

```ts
test('revoke maps a 409 revision conflict to DEVICE_CONFLICT (symmetry with register)', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const scenario = createScenarioDeviceRemote();
  scenario.failNextRevoke(409); // 夹具若无此注入器：直接传自定义 remote（status:409 形态），按本文件 stub 惯例
  const registry = createDeviceRegistry({ remote: scenario.remote, lease: () => leaseRef.lease });
  await registry.register({ deviceId: 'device-1', token: 'push-token-a', platform: 'ios' });
  await assert.rejects(
    registry.revoke({ deviceId: 'device-1', revision: 1 }),
    (error: unknown) => error instanceof DeviceError && error.code === 'DEVICE_CONFLICT',
    'B3-F5：revoke 的 409 必须映射 DEVICE_CONFLICT（与 register :113 对称）',
  );
});
```

（`failNextRevoke` 为 scenario 夹具追加的注入器——实现于 `in-memory-device-remote.ts`，让下一次 revoke 以 `errorStatus === 409` 形态拒绝；与既有 `conflictNextRegisters` 同型。）

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/inbox/notification-inbox.test.ts packages/mobile-core/src/device/device-registry.test.ts`
Expected: FAIL（已读回退：`view.items[0].read === false`；revoke：code 为 `DEVICE_BACKEND`）。

- [ ] **Step 3: 最小实现**

```ts
// notification-inbox.ts
let markedRead = new Set<string>(); // 模块闭包内（createNotificationInbox 作用域）
const merge = (page: InboxBackendPage, reset: boolean): InboxView => {
  if (reset) { seen = new Set(); duplicates = []; }
  const items: InboxItem[] = reset ? [] : [...(view?.items ?? [])];
  for (const item of page.items) {
    if (seen.has(item.notificationId)) { duplicates.push(item.notificationId); continue; }
    seen.add(item.notificationId);
    items.push({ ...item, ...(markedRead.has(item.notificationId) ? { read: true } : {}) }); // B3-F1：本地已读投影不被 GET 快照回退
  }
  cursor = page.nextCursor === '' ? undefined : page.nextCursor; // B3-F2
  return publish({ items, unreadCount: page.unreadCount, ...(cursor === undefined ? {} : { nextCursor: cursor }), duplicateNotificationIds: [...duplicates] });
};
// markRead 成功后：markedRead.add(trimmed);（既有投影逻辑保持——它本身即等价于集合语义）

// device-registry.ts revoke catch：
} catch (error) {
  if (error instanceof DeviceError) throw error;
  if (wireStatus(error) === 409) throw new DeviceError('DEVICE_CONFLICT', { cause: error }); // B3-F5
  if (wireStatus(error) === 404) throw new DeviceError('DEVICE_NOT_FOUND', { cause: error });
  throw new DeviceError('DEVICE_BACKEND', { cause: error });
}

// 顺手（随本任务实现与验收）：
// F6 —— attemptRegister 对 remote.register 响应的 revision/scope_generation 校验：
//   非 Number.isSafeInteger(revision) || revision <= 0 → throw new DeviceError('DEVICE_BACKEND', { cause: new Error('device register response carries a non-integer revision') })
//   （scope_generation 同型；服务端 dirty 值 fail-closed，不产生幻影 revision）
// F50 —— attemptRegister 成功返回前：if (!leaseActive(lease)) throw new DeviceError('DEVICE_SCOPE_CHANGED')（迟到成功不越 scope 原样返回）
//   两个顺手项的测试各追加一个用例（同型注入：revision 返回 1.5 / register 挂起期间 revoke lease）。
```

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test packages/mobile-core/src/inbox/ packages/mobile-core/src/device/`
Expected: PASS（deep-link/notification-inbox/device-registry 全绿）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/inbox/notification-inbox.ts packages/mobile-core/src/inbox/notification-inbox.test.ts packages/mobile-core/src/device/device-registry.ts packages/mobile-core/src/device/device-registry.test.ts packages/mobile-core/src/device/in-memory-device-remote.ts
git commit -m "fix(mobile-core): keep read projections across stale snapshots and map revoke conflicts symmetrically"
```

---

### Task 10: api-client：错误翻译与 origin 提取（B3-F43、F11）

**Files:**
- Create: `packages/api-client/src/mobile/deployment-origin.ts`
- Modify: `packages/api-client/src/mobile/task-office.ts:242-243`（decide 的 400/409 分流）
- Modify（替换私有拷贝为共享 import）: `devices.ts:37`、`inbox.ts:36`、`legacy-tasks.ts:21`、`materials.ts:30`、`resources.ts:25`、`runtime.ts:40`、`task-office.ts:61`（共 7 处）
- Test: `packages/api-client/src/mobile/task-office.test.ts`、新建 `packages/api-client/src/mobile/deployment-origin.test.ts`

**Interfaces:**
- Consumes: `ApiError`（`packages/api-client/src/errors.ts`）；各文件的私有 `requireDeploymentOrigin`（已核实 7 处；inbox 版含 hostname 检查为最严格）。
- Produces: `requireDeploymentOrigin(origin: string): string`（共享导出——八条校验：非空/绝对 URL/HTTPS/无 user info/**含 host**/根路径/无 query/无 fragment，返回 `parsed.origin`）；`decide()` 的翻译只对 409 → `INTERACTION_SUPERSEDED`，**400 不再翻译**（原始 `ApiError` 透传——mobile-core 的 mapError 归类为 backend 错误，action/kind 不匹配等确定性客户端错误不再被冒充为「已被处理」终态；`attention-inbox.ts:154-156` 的 superseded 归类仅覆盖 409 场景）。

**根因与修复说明：** (a) `task-office.ts:243` 的 `if (error.status === 409 || error.status === 400) coded('INTERACTION_SUPERSEDED')` 把确定性 400（后端 `workbench_commands.go:125-126` 的 `ErrInteractionActionMismatch`）与 409（`:142-143` 的 `ErrAlreadyResolved`）一并翻译——action/kind 不匹配是可修正的客户端错误，被冒充为「已被处理」终态后原始错误丢失、上层不再重试也不再提示原因（F43）。(b) `requireDeploymentOrigin` 在 `packages/api-client/src/mobile/` 下 7 份拷贝（grep 已核实：devices.ts:37、inbox.ts:37、legacy-tasks.ts:21、materials.ts:30、resources.ts:25、runtime.ts:40、task-office.ts:61），且已漂移（inbox 版多 hostname 检查——差异记录 2）——origin 校验规则演进需 7 处同步（F11）。

- [ ] **Step 1: 写失败测试**

新建 `deployment-origin.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { requireDeploymentOrigin } from './deployment-origin.ts';

test('requireDeploymentOrigin enforces the strictest merged rule set', () => {
  assert.equal(requireDeploymentOrigin('https://weknora.example.test'), 'https://weknora.example.test');
  for (const bad of ['not-a-url', 'http://weknora.example.test', 'https://user:pw@weknora.example.test',
    'https://weknora.example.test/path', 'https://weknora.example.test?q=1', 'https://weknora.example.test#f', 'https://']) {
    assert.throws(() => requireDeploymentOrigin(bad), undefined, `must reject ${bad}`); // 'https://' 覆盖 hostname 空检查
  }
});
```

`task-office.test.ts` 追加（沿用本文件既有 request 注入夹具形态）：

```ts
test('decide no longer translates a deterministic 400 as superseded', async () => {
  const remote = createTaskOfficeRemote({
    origin: 'https://weknora.example.test',
    request: async () => { const error = new (await import('../errors.ts')).ApiError({ status: 400, code: 'interaction_action_mismatch', message: 'action mismatch' }); throw error; },
  });
  await assert.rejects(
    remote.decide({ runId: 'r1', decisionId: 'd1', action: 'approve', expectedRevision: 3 }),
    (error: unknown) => {
      const coded = (error as { code?: unknown }).code;
      return coded !== 'INTERACTION_SUPERSEDED'; // 400 透传原始 ApiError——不再是 superseded 契约码
    },
    'B3-F43：400 是确定性客户端错误，不得冒充「已被处理」终态',
  );
});
```

（`decide` 的入参形态以本文件既有用例的实际字段为准——执行者按现行签名抄录参数；断言核心是错误码不等于 `INTERACTION_SUPERSEDED` 且 `status === 400` 保留。）

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/deployment-origin.test.ts packages/api-client/src/mobile/task-office.test.ts`
Expected: FAIL（新文件：模块不存在；decide：当前 400 被翻译为 `INTERACTION_SUPERSEDED`）。

- [ ] **Step 3: 最小实现**

```ts
// deployment-origin.ts（新建——最严格合并版，inbox.ts:36-46 为基线）
export function requireDeploymentOrigin(origin: string): string {
  let parsed: URL;
  if (typeof origin !== 'string' || origin.trim() === '') throw new Error('deployment origin is required');
  try { parsed = new URL(origin); } catch { throw new Error(`deployment origin must be an absolute URL: ${origin}`); }
  if (parsed.protocol !== 'https:') throw new Error('deployment origin must use HTTPS');
  if (parsed.username !== '' || parsed.password !== '') throw new Error('deployment origin must not embed user info');
  if (parsed.hostname === '') throw new Error('deployment origin must include a host');
  if (parsed.pathname !== '/') throw new Error('deployment origin must not include a path');
  if (parsed.search !== '' || parsed.hash !== '') throw new Error('deployment origin must not include a query or fragment');
  return parsed.origin;
}

// task-office.ts:242-243 —— 400 退出 superseded 翻译：
if (error.status === 409) coded('INTERACTION_SUPERSEDED');
if (error.status === 502 && error.code === 'command_recovery_unknown') coded('INTERACTION_DELIVERY_UNKNOWN');
if (error.status === 404 || error.status === 403 || error.status === 410) coded('INTERACTION_GONE');
// 400 落到既有 `throw error`（原始 ApiError 透传）

// 7 个文件 —— 删除私有 requireDeploymentOrigin，改为：
import { requireDeploymentOrigin } from './deployment-origin.ts';
// （materials.ts 等返回值消费点不变；inbox.ts 等 void 消费点忽略返回值即可）
```

- [ ] **Step 4: 实跑确认通过 + 回归（7 个模块的既有测试即提取无行为变化的证明）**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/`
Expected: PASS（全目录——devices/inbox/legacy-tasks/materials/resources/runtime/task-office + integration 按 opt-in skip）。
Run: `grep -rn "function requireDeploymentOrigin" packages/api-client/src/mobile/ | wc -l`
Expected: `0`（私有拷贝清零，共享版以 `export function` 存在于 deployment-origin.ts）。

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/mobile/deployment-origin.ts packages/api-client/src/mobile/deployment-origin.test.ts packages/api-client/src/mobile/task-office.ts packages/api-client/src/mobile/task-office.test.ts packages/api-client/src/mobile/devices.ts packages/api-client/src/mobile/inbox.ts packages/api-client/src/mobile/legacy-tasks.ts packages/api-client/src/mobile/materials.ts packages/api-client/src/mobile/resources.ts packages/api-client/src/mobile/runtime.ts
git commit -m "fix(api-client): stop translating 400s as superseded and share one requireDeploymentOrigin"
```

---

### Task 11: mobile-core：material 缓存与凭据回验（B3-F31、F58；顺手 B3-F32、F33、F34、F35、F55）

**Files:**
- Modify: `packages/mobile-core/src/material/task-material.ts:67`（blobs LRU 上限）、`:95-106`（mintGrant 身份回验）、fetchBytes（F32 字节复检）、act 分享分支（F33）、`:29-35`（decodeUtf8 fallback）
- Modify: `packages/mobile-core/src/material/evidence.ts:31`（F35 detail 截断）
- Modify: `apps/mobile/src/materials-view.ts:13`（F55 键类型收紧——本文件唯一在 apps 侧的改动，Task 12 串行接续）
- Test: `packages/mobile-core/src/material/task-material.test.ts`、`evidence.test.ts`、`apps/mobile/src/materials-view.test.ts`

**Interfaces:**
- Consumes: `MaterialBackendGrant.artifact: MaterialBackendArtifact`（ports.ts:31-35——含 `id`/`index`/`name`/`mime`/`version`，已核实）；`PREVIEW_MAX_BYTES = 2 * 1024 * 1024`（material-kinds.ts:4，已核实）；`MaterialError` 的 code 联合（task-material.ts 的错误码集合）。
- Produces: `MAX_BLOB_ENTRIES = 6`（blobs 以插入序 LRU：命中即 delete+set 重排，超限逐出最旧——F31）；`mintGrant` 回验 `grant.artifact.id === entry.materialId`（与 version 一并校验），不匹配抛新错误码 `MATERIAL_GRANT_MISMATCH`（F58——index 错位时 fail closed，不再把错材料的签名凭据外发/缓存）；`fetchBytes` 对实际字节长度复检 `PREVIEW_MAX_BYTES`，超限不入缓存并抛错（F32）；分享分支先查 `ports.share` 存在再 mintGrant（F33）；decodeUtf8 fallback 为正确的 UTF-8 解码循环（F34）；evidence citations 的 `detail` 截断至 2000 字符（F35）；`MATERIAL_ERROR_COPY: Record<MaterialErrorCode, string>`（F55——穷尽性检查，新码 `MATERIAL_GRANT_MISMATCH` 强制补条目）。

**根因与修复说明：** 同根因——material core 的资源与凭据生命周期防御缺失：(a) `blobs` 字节缓存按 `materialId@version` 无上限累积（:67，单条最高 2MB、仅 close() 清理 :223——20 个预览即 ~40MB，F31）；(b) `mintGrant` 按 `entry.index` 铸造后仅校验 URL origin（:95-106），不回验 `grant.artifact` 身份——列表重排后 entry.index 与 materialId 错位时把错材料的签名凭据外发/缓存，校验数据已在手零额外往返（F58；`entryFor` 依赖句柄内 lastIndex 缓存 :124-130，错位窗口真实存在）；(c) 预览上限只校验后端声明的 `entry.size`（previewVerdictOf），fetchBytes 实际字节入缓存前未复检（F32）；(d) 分享在铸造短时效签名 URL 之后才检查 `ports.share` 是否存在（F33）；(e) decodeUtf8 回退分支 `String.fromCharCode` 逐字节拼接是 Latin-1 而非 UTF-8（:29-35，F34）；(f) citations detail 对 payload 整包 `JSON.stringify` 无截断（evidence.ts:31，F35）；(g) `MATERIAL_ERROR_COPY` 键类型 `Record<string, string>` 丧失穷尽性（materials-view.ts:13，F55）。

- [ ] **Step 1: 写失败测试（追加）**

`task-material.test.ts`（沿用本文件既有 ports/handle 夹具形态）：

```ts
test('the blob cache evicts oldest entries beyond MAX_BLOB_ENTRIES', async () => {
  const { handle, backend } = materialFixture(); // 沿用本文件既有夹具（按实际名引用）；backend 可注入多个 artifact
  for (let index = 0; index < 8; index += 1) {
    await handle.open({ kind: 'artifact', runId: 'run-1', materialId: `mat-${index}` });
  }
  // 观测口径：backend 的 fetch/signedUrl 调用计数——再次打开最早的两个 material 必须重新铸造（被逐出）
  const signedBefore = backend.signedUrlCalls;
  await handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'mat-0' });
  assert.equal(backend.signedUrlCalls, signedBefore + 1, 'B3-F31：超限后最旧条目被逐出，重开需重新铸造');
  await handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'mat-7' });
  assert.equal(backend.signedUrlCalls, signedBefore + 1, '最近条目仍在缓存，零新铸造');
});

test('a grant whose artifact identity mismatches the requested material fails closed', async () => {
  const { handle, backend } = materialFixture();
  backend.grantForIndex = () => ({ url: 'https://weknora.example.test/f.bin', expiresAt: '...', artifact: { id: 'mat-OTHER', index: 0, name: 'other', mime: 'text/plain', version: 'v1', size: 3, sourceRun: 'run-1' } });
  await assert.rejects(
    handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'mat-1' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_GRANT_MISMATCH',
    'B3-F58：index 错位铸造出的凭据必须回验身份后拒绝，不得外发/缓存',
  );
});
```

`evidence.test.ts` 追加：

```ts
test('citation details are truncated before entering view state', () => {
  const citations = materialCitations([{ seq: 1, occurredAt: '...', type: 'tool.terminal', payload: { blob: 'x'.repeat(5000) } }]);
  assert.ok((citations[0]!.detail ?? '').length <= 2000, 'B3-F35：大载荷截断后进入视图状态');
});
```

`materials-view.test.ts` 追加（F55 穷尽性——类型层，用编译期断言式测试）：

```ts
test('MATERIAL_ERROR_COPY covers every MaterialErrorCode', async () => {
  const { MATERIAL_ERROR_COPY } = await import('./materials-view.ts');
  const { MaterialError } = await import('@weknora/mobile-core');
  // MaterialError 构造接受的所有 code 均有文案：以 mobile-core 导出的 code 联合的运行时探测（逐码构造）为准
  for (const code of ['MATERIAL_SCOPE_CHANGED', 'MATERIAL_CLOSED', 'MATERIAL_BACKEND', 'MATERIAL_SIGNING_DISABLED', 'MATERIAL_GRANT_EXPIRED', 'MATERIAL_GRANT_INVALID', 'MATERIAL_GRANT_ORIGIN', 'MATERIAL_GRANT_MISMATCH', 'MATERIAL_NOT_FOUND', 'MATERIAL_INVALID_INPUT'] as const) {
    new MaterialError(code); // 编译期穷尽（Record<MaterialErrorCode,string> 下缺键即 tsc 红）
    assert.ok(MATERIAL_ERROR_COPY[code] !== undefined, `缺少 ${code} 文案`);
  }
});
```

（code 清单以 `MaterialError` 实际联合为准——`new MaterialError(code)` 在 `as const` 数组上产生编译期检查；缺码即 RED。）

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/material/task-material.test.ts packages/mobile-core/src/material/evidence.test.ts apps/mobile/src/materials-view.test.ts`
Expected: FAIL（逐出：第二次 mat-0 打开零新铸造（缓存未满前全命中）——按夹具实际交错断言失败；mismatch：当前成功返回凭据；截断：detail 全长；穷尽：`MATERIAL_GRANT_MISMATCH` 键 undefined）。

- [ ] **Step 3: 最小实现**

```ts
// task-material.ts
const MAX_BLOB_ENTRIES = 6; // 单条上限 2MB（PREVIEW_MAX_BYTES）→ 常驻上限 ~12MB（B3-F31）
...
const cacheBlob = (key: string, bytes: Uint8Array): void => {
  blobs.delete(key); // LRU 重排
  blobs.set(key, bytes);
  while (blobs.size > MAX_BLOB_ENTRIES) blobs.delete(blobs.keys().next().value!);
};

const mintGrant = async (runId: string, entry: MaterialEntry): Promise<MaterialBackendGrant> => {
  const grant = await callRemote(() => ports.remote.signedUrl({ runId, index: entry.index }));
  if (grant.artifact.id !== entry.materialId || grant.artifact.version !== entry.version) {
    // B3-F58：signedUrl 按易变 index 铸造——回验响应自带的 artifact 身份，错位即拒绝（fail closed）
    throw new MaterialError('MATERIAL_GRANT_MISMATCH', { cause: new Error(`grant artifact ${grant.artifact.id}@${grant.artifact.version} does not match requested ${entry.materialId}@${entry.version}`) });
  }
  /* 既有 origin 校验保持 */
};

const fetchBytes = async (runId: string, entry: MaterialEntry): Promise<Uint8Array> => {
  const cacheKey = `${entry.materialId}@${entry.version}`;
  const cached = blobs.get(cacheKey);
  if (cached !== undefined) { blobs.delete(cacheKey); blobs.set(cacheKey, cached); return cached; } // LRU 命中重排
  const grant = await mintGrant(runId, entry);
  ... // 既有 fetch + GRANT_EXPIRED 处理
  if (fetched.bytes.length > PREVIEW_MAX_BYTES) throw new MaterialError('MATERIAL_INVALID_INPUT', { cause: new Error(`fetched ${fetched.bytes.length} bytes exceeds the preview budget`) }); // F32
  cacheBlob(cacheKey, fetched.bytes);
  return fetched.bytes;
};

// 分享分支（act 内）—— F33：先检查端口再铸造
if (ref.kind === 'artifact' && shareRequested) {
  if (ports.share === undefined) throw new MaterialError('MATERIAL_INVALID_INPUT', { cause: new Error('share port is not available') });
  const grant = await mintGrant(runId, entry); // 顺序对调：不再白铸
}

// decodeUtf8 fallback —— F34：正确的 UTF-8 解码（码点循环，替代 Latin-1 拼接）
// evidence.ts:31 —— F35：detail = raw.length > 2000 ? `${raw.slice(0, 2000)}…` : raw

// materials-view.ts:13 —— F55：
export const MATERIAL_ERROR_COPY: Record<MaterialErrorCode, string> = { /* 既有条目 + 'MATERIAL_GRANT_MISMATCH': '材料凭据校验失败，请刷新列表后重试。' */ };
```

（`MaterialErrorCode` 从 `@weknora/mobile-core` 导入——若该联合类型未导出，在本包补导出（type-only，无运行时影响）；`MaterialEntry.version` 字段名以 ports/material 实际定义为准。）

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test packages/mobile-core/src/material/ apps/mobile/src/materials-view.test.ts`
Expected: PASS。
Run: `pnpm --filter @weknora/mobile typecheck`（或 apps/mobile 的既有 typecheck 入口 `pnpm run typecheck:mobile`）
Expected: PASS（F55 的类型收紧无遗漏键）。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/material/task-material.ts packages/mobile-core/src/material/task-material.test.ts packages/mobile-core/src/material/evidence.ts packages/mobile-core/src/material/evidence.test.ts packages/mobile-core/src/material/material-kinds.ts apps/mobile/src/materials-view.ts apps/mobile/src/materials-view.test.ts
git commit -m "fix(material): bounded LRU blob cache, artifact-identity grant verification, and honest preview limits"
```

---

### Task 12: api-client/app：material 末页信号与视图防陈旧（B3-F52、F54、F57；顺手 B3-F10、F18、F36）

**Files:**
- Modify: `packages/api-client/src/mobile/materials.ts:100-112`（terminalLog 末页游标）
- Modify: `apps/mobile/src/materials-view.ts:53-60`（generation 令牌）
- Modify: `apps/mobile/src/adapters/material-adapters.ts`（F57 Android 分享、F10 AbortSignal）
- Modify: `apps/mobile/src/resources-view.ts`（F18 notice 重置与 alive 卫护）
- Modify: `packages/mobile-core/src/material/ports.ts`（F36 nextCursor 合同注释）
- Test: `packages/api-client/src/mobile/materials.test.ts`、`apps/mobile/src/materials-view.test.ts`、`apps/mobile/src/adapters/material-adapters` 相关测试（若无既有测试文件，断言并入 materials-view.test.ts 的源级模式）

**Interfaces:**
- Consumes: Go 端 `workbench_terminal_log.go:94/119` 的末页语义（`next_cursor === after`，游标未推进）；`resources-view.ts` 的既有 generation 范式（报告指认的同批先例）。
- Produces: `MaterialRemote.terminalLog` 的 `nextCursor` 在「游标未推进」时返回 `undefined`（`next <= after → undefined`；缺字段回退也不再是 `Number(... ?? 0)` 的 0/NaN——按 after 判定）；`createMaterialsController` 的 `run()` 以递增 generation 令牌守卫发布（晚到的旧响应不覆盖新状态）；native share 在 Android 上把 url 并入 message（RN `Share.share` 的 url 仅 iOS 生效）；`fetchBlobAdapter` 的免凭据抓取带 30s AbortController 超时；`MaterialBackendTerminalPage.nextCursor?: number` 合同注释对齐运行时语义。

**根因与修复说明：** (a) 适配器恒返回数字 `nextCursor`（materials.ts:109——缺字段回退 `Number(... ?? 0)`），而 Go 端末页语义是「游标未推进仍发数字」；`task-material.ts:168` 以 undefined 判无下一页——末页信号在转译时丢失，「可继续加载」永不消失或游标重置为 0（F52）。(b) `materials-view.ts` 的 `run()`（:53-60）仅检查 disposed，无 generation 令牌——慢 A 后到覆盖快 B 的交错成立（F54；同批 resources-view 已采用 generation 令牌为既有范式）。(c) `material-adapters.ts:32` 的 `share({ url, message: name })` 无平台分支——RN Android 实现只取 message 构建 ACTION_SEND，签名链接被静默丢弃（F57）。顺手：F10（免凭据 fetch 无超时——CDN 挂起让材料页 loading 永久挂起）；F18（resources-view refresh() 重置 error 未重置 notice + 失败回调缺 alive 卫护）；F36（ports 合同声明 nextCursor 非可选 number 与运行时 undefined 判定不一致——注释对齐）。

- [ ] **Step 1: 写失败测试**

`packages/api-client/src/mobile/materials.test.ts` 追加（沿用本文件既有 request 注入形态）：

```ts
test('terminalLog reports undefined when the cursor did not advance (end of log)', async () => {
  const remote = createMaterialRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: true, data: { lines: [{ seq: 5, occurred_at: '...', stream: 'stdout', text: 'ls' }], next_cursor: 5 } }), // after=5 末页：游标未推进
  });
  const page = await remote.terminalLog({ runId: 'r1', after: 5, limit: 200 });
  assert.equal(page.nextCursor, undefined, 'B3-F52：末页（游标未推进）必须转译为 undefined');
  const page2 = await remote.terminalLog({ runId: 'r1', after: 0, limit: 200 }); // 同一 stub：after=0 → next=5 推进
  assert.equal(page2.nextCursor, 5);
});
```

（`createMaterialRemote` 的工厂名/入参形态以本文件既有夹具为准。）

`apps/mobile/src/materials-view.test.ts` 追加（沿用本文件既有 handle stub 形态）：

```ts
test('a slower stale open does not overwrite a newer state (generation token)', async () => {
  const handle = createScriptedMaterialHandle(); // 本文件既有/新增 stub：open/index 可注入受控 promise
  const controller = createMaterialsController(handle, { runId: 'r1' });
  const slowOpen = controller.openMaterial('mat-slow'); // A 先发（挂起）
  await controller.openMaterial('mat-fast');           // B 后发先回
  assert.equal(controller.state().view?.materialId, 'mat-fast');
  handle.release('mat-slow');                          // A 迟到
  await slowOpen.catch(() => undefined);
  assert.equal(controller.state().view?.materialId, 'mat-fast', 'B3-F54：晚到的旧响应不得覆盖新状态');
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/materials.test.ts apps/mobile/src/materials-view.test.ts`
Expected: FAIL（末页：`nextCursor` 为数字 5；generation：stale 覆盖后 view 为 mat-slow）。

- [ ] **Step 3: 最小实现**

```ts
// materials.ts terminalLog（:100-112）—— 末页游标转译：
const rawCursor = typeof data.next_cursor === 'number' ? data.next_cursor : Number(data.next_cursor ?? after);
return {
  lines: /* 既有映射 */,
  ...(rawCursor > after ? { nextCursor: rawCursor } : {}), // 游标未推进 = 末页 → 省略键（undefined）
};

// materials-view.ts run() —— generation 令牌（对照 resources-view 范式）：
let generation = 0;
const run = async (action: () => Promise<MaterialsViewState>): Promise<void> => {
  const ticket = ++generation;
  publish({ ...state, loading: true, error: undefined });
  try {
    const next = await action();
    if (!disposed && ticket === generation) publish(next); // 陈旧完成丢弃
  } catch (failure) {
    if (!disposed && ticket === generation) publish({ ...state, loading: false, error: messageOf(failure) });
  }
};

// material-adapters.ts —— F57 + F10：
async share({ url, name }) {
  const os = nativeDevicePlatformOs(); // 既有 require('react-native') 的 Platform 探测先例（device-identity.ts:44-50 同型）
  await shareLike.share(os === 'android'
    ? { message: `${name} ${url}` }  // Android：url 并入 message（RN 只取 message 构建 ACTION_SEND）
    : { url, message: name });
},
// fetchBlobAdapter：
const controller = new AbortController();
const timer = setTimeout(() => controller.abort(), 30_000); // F10：免凭据抓取 30s 超时
try { response = await fetch(url, { signal: controller.signal }); } finally { clearTimeout(timer); }

// resources-view.ts —— F18：refresh() 同时重置 notice；失败回调加 alive 卫护（对照本文件既有 alive 模式）
// ports.ts —— F36 合同注释：nextCursor 省略 = 末页（游标未推进）；Go 端恒发数字，转译责任在适配器。
```

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/materials.test.ts apps/mobile/src/materials-view.test.ts apps/mobile/src/resources-view.test.ts packages/mobile-core/src/material/task-material.test.ts`
Expected: PASS（task-material 的末页判定消费 undefined 语义——既有用例加新用例全绿）。

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/mobile/materials.ts packages/api-client/src/mobile/materials.test.ts apps/mobile/src/materials-view.ts apps/mobile/src/materials-view.test.ts apps/mobile/src/adapters/material-adapters.ts apps/mobile/src/resources-view.ts packages/mobile-core/src/material/ports.ts
git commit -m "fix(material): end-of-log cursor translation, stale-response guards, and platform-honest sharing"
```

---

### Task 13: legacy 域：卡片隔离与历史通道（B3-F21、F22、F23、F24；顺手 B3-F14、F66）

**Files:**
- Modify: `apps/mobile/src/screens/LegacyTasksScreen.tsx:18`（question 按卡片隔离）、`:20-28`（无授权面显式空态）、`:51-54`（map 内受控输入改造）
- Modify: `apps/mobile/src/legacy-tasks-view.ts:70-81`（followUp 复位）
- Modify: `packages/mobile-core/src/task-office/task-office.ts` legacyHistory（F66 settle/epoch + F24 分页透传；**在 Task 6 之后串行执行**）
- Modify: `packages/api-client/src/mobile/legacy-tasks.ts:75-81`（F24 limit/before 参数化）
- Test: `apps/mobile/src/legacy-tasks-view.test.ts`、`packages/api-client/src/mobile/legacy-tasks.test.ts`、`apps/mobile/src/app-smoke.test.tsx`（Screen 层）

**Interfaces:**
- Consumes: Task 6 后的 `task-office.ts` 结构；服务端 `/messages/:id/load` 的 `limit+before_time` 分页（同包 `chat/sessions.ts` 已参数化——报告指认）；`app/attention.tsx:13-16` 的显式空态范式。
- Produces: `LegacyTasksScreen` 的 `question` 状态改为 `Record<string, string>`（按 taskId 隔离——每卡片独立输入/发送/回执，F21）；无授权面（`activeTaskOffice()` undefined）时发布显式空态文案而非停留 loading:true（F22）；`createLegacyTasksController.submitFollowUp` 的列表刷新失败也复位 `followUpState: 'idle'` + `followUpError`（F23）；`RemoteLegacy.history(taskId, options?: { limit?: number; before?: string })` 与 `TaskOffice.legacyHistory(taskId, options?)` 透传分页参数（F24——view 层 `openHistory` 请求 `limit: 100`，服务端按上限自然 clamp）；`legacyHistory` 走 settle/epoch 失效保护（F66——与同模块 legacyTasks/moreLegacyTasks 同规则）；`errorMessage` 对 `TASK_OFFICE_SUPERSEDED` 的文案映射（F14）。

**根因与修复说明：** 同根因——legacy 面把多实体状态折叠进单槽、把通道写死：(a) `question` 是整屏唯一 state 却在 `items.map` 内为每张卡片渲染受控输入（:18、:51-54）——任一卡片输入同步到所有卡片（F21）。(b) 无授权面时 useEffect 静默 `return`（:21-22），屏幕永久停留初始 `loading:true`（:17），后续授权面就绪也不重建 controller（F22；对照 `app/attention.tsx:13-16` 的显式空态范式）。(c) 追问成功后的列表刷新（:79）在 try 外，刷新失败落入 `run` 统一 catch 只发布 loading/error，`followUpState` 永久卡死 'sending'、发送按钮持续禁用且 reload 也不复位（F23）。(d) `history` 硬编码 `?limit=20` 且签名无分页参数（legacy-tasks.ts:80、:75）——超过 20 条消息的旧会话历史被静默截断，违背「历史」语义（F24）。顺手：F14（错误文案映射缺 `TASK_OFFICE_SUPERSEDED`——竞态时裸错误码直出）；F66（`legacyHistory` 未走 settle/epoch 失效保护——task-office.ts 与 legacyTasks/moreLegacyTasks 同模块不同规则）。

- [ ] **Step 1: 写失败测试**

`apps/mobile/src/legacy-tasks-view.test.ts` 追加（沿用本文件既有 controller 夹具）：

```ts
test('a failed post-follow-up refresh resets followUpState instead of sticking on sending', async () => {
  const office = legacyOfficeDouble({ // 沿用本文件既有 office stub 形态（按实际名引用）
    followUp: async () => {},
    legacyTasks: async () => { throw new Error('refresh failed'); },
  });
  const controller = createLegacyTasksController(office);
  await controller.submitFollowUp('task-1', '追问内容');
  await controller.whenSettled();
  assert.notEqual(controller.state().followUpState, 'sending', 'B3-F23：刷新失败也必须复位 followUpState');
  assert.ok(controller.state().followUpError !== undefined);
});
```

`packages/api-client/src/mobile/legacy-tasks.test.ts` 追加：

```ts
test('history passes paging parameters through to the wire', async () => {
  const paths: string[] = [];
  const remote = createLegacyTasksRemote({ // 沿用本文件既有 request 注入夹具（按实际名引用）
    origin: 'https://weknora.example.test',
    request: async (input) => { paths.push(input.path); return { success: true, data: [] }; },
  });
  await remote.history('task-9');
  await remote.history('task-9', { limit: 100, before: 'msg-42' });
  assert.equal(paths[0], '/api/v1/messages/task-9/load?limit=20');
  assert.equal(paths[1], '/api/v1/messages/task-9/load?limit=100&before_time=msg-42', 'B3-F24：翻页通道必须透传');
});
```

`app-smoke.test.tsx` 追加（react stub 组件直调 + 源级断言——batch2 先例）：

```ts
test('the legacy screen renders an explicit unauthenticated state and per-card inputs', async () => {
  const { readFileSync } = await import('node:fs');
  const source = readFileSync(join(here, 'screens/LegacyTasksScreen.tsx'), 'utf8');
  // B3-F22：无授权面必须发布显式空态文案（对照 app/attention.tsx:13-16 范式），不得停留 Loading
  assert.match(source, /请先登录/);
  // B3-F21：question 状态按 taskId 隔离（Record 槽位），不再整屏单一 useState
  assert.match(source, /Record<string, string>/);
  assert.match(source, /questions\[card\.taskId\]/);
  assert.doesNotMatch(source, /const \[question, setQuestion\] = useState\(''\)/, '整屏单一 question state 必须移除');
});

test('the legacy screen shows the explicit empty state when no office is active', async () => {
  const react = (await import('react')) as unknown as { __beginRender(): void };
  react.__beginRender();
  const { LegacyTasksScreen } = await import('./screens/LegacyTasksScreen.tsx');
  const tree = LegacyTasksScreen({}); // activeTaskOffice() 为 undefined（无授权面）
  assert.ok(JSON.stringify(tree).includes('请先登录'), 'B3-F22：显式空态文案渲染');
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/legacy-tasks-view.test.ts packages/api-client/src/mobile/legacy-tasks.test.ts apps/mobile/src/app-smoke.test.tsx`
Expected: FAIL（followUpState 卡 'sending'；paths[1] 仍是 `?limit=20`；源级断言：无 `Record<string, string>` 且整屏单一 question state 仍存在；行为级：屏幕 JSON 无『请先登录』空态）。

- [ ] **Step 3: 最小实现**

```tsx
// LegacyTasksScreen.tsx
const [view, setView] = useState<LegacyTasksViewState>({ loading: true, items: [], hasMore: false, duplicateTaskIds: [], followUpState: 'idle' });
const [questions, setQuestions] = useState<Record<string, string>>({}); // B3-F21：按 taskId 隔离
useEffect(() => {
  const office = activeTaskOffice();
  if (!office) {
    setView((prev) => ({ ...prev, loading: false, error: '请先登录并激活空间，再查看历史任务。' })); // B3-F22：显式空态（对照 app/attention.tsx:13-16）
    return;
  }
  /* 既有 controller 创建逻辑 */
}, []);
...
{view.items.map((card: LegacyTaskCard) => {
  const question = questions[card.taskId] ?? '';
  return (
    <View key={card.taskId}>
      ...
      <TextInput value={question} onChangeText={(text) => { setQuestions((prev) => ({ ...prev, [card.taskId]: text })); }} placeholder="Continue with an ordinary follow-up" />
      <Button title="Send follow-up" disabled={view.followUpState === 'sending' || question.trim() === ''} onPress={() => { void controller?.submitFollowUp(card.taskId, question); setQuestions((prev) => ({ ...prev, [card.taskId]: '' })); }} />
      ...
    </View>
  );
})}
```

```ts
// legacy-tasks-view.ts submitFollowUp —— B3-F23：刷新纳入收口
submitFollowUp: (taskId: string, question: string) => run(async () => {
  const trimmed = question.trim();
  if (trimmed === '') return { followUpState: 'idle' as const, followUpError: undefined };
  publish({ followUpState: 'sending', followUpError: undefined });
  try {
    await office.followUp({ taskId, question: trimmed });
  } catch (error) {
    return { followUpState: 'idle' as const, followUpError: errorMessage(error) };
  }
  try {
    const page = await office.legacyTasks({});
    return { followUpState: 'sent' as const, followUpError: undefined, items: page.items, hasMore: page.nextCursor !== undefined };
  } catch (error) {
    return { followUpState: 'idle' as const, followUpError: `已发送，但刷新列表失败：${errorMessage(error)}` }; // 复位 + 诚实提示
  }
}),
// errorMessage —— F14：const LEGACY_ERROR_COPY = { TASK_OFFICE_SUPERSEDED: '列表已被其他操作更新，请刷新。', ... }（TaskOfficeError 的 message 即 code，映射后兜底原样）
```

```ts
// api-client legacy-tasks.ts —— B3-F24：
async history(taskId: string, options: { limit?: number; before?: string } = {}): Promise<RemoteLegacyMessage[]> {
  const trimmed = taskId.trim();
  if (trimmed === '') throw new Error('taskId must not be empty');
  const limit = options.limit !== undefined && Number.isSafeInteger(options.limit) && options.limit > 0 ? options.limit : 20;
  const suffix = options.before !== undefined ? `?limit=${limit}&before_time=${encodeURIComponent(options.before)}` : `?limit=${limit}`;
  /* 既有 parse + 映射 */
}
// task-office.ts legacyHistory —— F24 透传 + F66 settle/epoch（对照 legacyTasks 的 ++legacyListEpoch 模式）：
async legacyHistory(taskId: string, options: { limit?: number; before?: string } = {}): Promise<LegacyMessage[]> {
  const epoch = ++legacyListEpoch;
  const lease = requireLease();
  const trimmed = taskId.trim();
  if (trimmed === '') throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
  const messages = await callBackend(() => requireLegacy().history(trimmed, options));
  settleLegacy(epoch, lease); // 与 moreLegacyTasks 同型的迟到守卫（执行时对照同模块既有 settle 调用形态）
  return messages;
}
// legacy-tasks-view.ts openHistory —— 请求 limit: 100（服务端 clamp 自然生效）
```

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm exec tsx --test apps/mobile/src/legacy-tasks-view.test.ts packages/api-client/src/mobile/legacy-tasks.test.ts packages/mobile-core/src/task-office/ apps/mobile/src/app-smoke.test.tsx`
Expected: PASS（task-office 包全绿证明 legacyHistory 签名扩展无回归——`Pick<TaskOffice, 'legacyHistory'>` 的消费方类型兼容）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/screens/LegacyTasksScreen.tsx apps/mobile/src/legacy-tasks-view.ts apps/mobile/src/legacy-tasks-view.test.ts packages/api-client/src/mobile/legacy-tasks.ts packages/api-client/src/mobile/legacy-tasks.test.ts packages/mobile-core/src/task-office/task-office.ts apps/mobile/src/app-smoke.test.tsx
git commit -m "fix(legacy): per-card question state, explicit unauthenticated state, follow-up recovery, and paged history"
```

---

### Task 14: composition/请求 ID/设备身份（B3-F26、F28、F29、F27、F49；顺手 B3-F48）

**Files:**
- Modify: `apps/mobile/src/composition.ts:130`（taskOffices 缓存键）、`:164`（deviceRegistries）、`:180`（notificationInboxes）、`:259`（taskMaterials）、`:199-204`（F29 Pick 类型）、`:345-360`（F28 注册重试）
- Modify: `apps/mobile/src/adapters/request-id.ts:9-15`（F27 CSPRNG）
- Modify: `apps/mobile/src/adapters/device-identity.ts:10-25`（F49 in-flight 去重）
- Modify: `apps/mobile/package.json`（F27 依赖 expo-crypto）
- Test: `apps/mobile/src/app-smoke.test.tsx`、`apps/mobile/src/adapters/request-id` 新测试（并入现有 app 级测试文件或新建 `request-id.test.ts`）、`device-identity` 测试（同型）

**Interfaces:**
- Consumes: `deploymentScopeKey(origin, activeTenantId)`（composition.ts:133-135 既有导出——`${origin}::${activeTenantId}`）；`expo-crypto` 的 `randomUUID`；Task 9 后的 device-registry 契约。
- Produces: 四个模块级缓存（taskOffices/deviceRegistries/notificationInboxes/taskMaterials）以 `deploymentScopeKey(origin, tenantId)` 为键——同 origin 切租户不再复用含前 scope 残留状态（submissionStore、accumulated、inbox view）的实例（F26；缓存条目超 8 个清最旧，防泄漏）；`registerActiveDeviceIfPossible` 的注册改为「失败不永久占用」——no-token/failed 不标记 registeredFor，会话内每 origin 最多重试 2 次（F28）；`openNotificationFromInbox` 的 item 参数类型改为 `Pick<InboxItem, 'notificationId' | 'deepLink'>` 并删除 `as InboxItem` 断言（F29）；`createNativeRequestId` 优先 `require('expo-crypto').randomUUID`（F27——CSPRNG 路径真实可达；require 失败回退现有 globalThis.crypto 链）；`createSecureDeviceIdentity.deviceId` 以模块内 promise 缓存去重并发首次调用（F49）。

**根因与修复说明：** 同根因——组合根与适配器的实例/身份生命周期不设防：(a) 四个缓存仅以 origin 为键（:130/164/180/259，全文件无 delete/clear——已核实），隔离完全依赖组件层 `deploymentScopeKey` remount 键（:244 含 tenantId）——remount 只换组件不换缓存实例，同 origin 切换用户/租户时复用前 scope 残留（F26）。(b) 设备注册「先标记、后尝试、永不重试」（:355-357 registeredFor.add 在调用前执行；registerActiveDeviceIfPossible 全仓库仅此一处调用——已核实报告）——iOS 首次安装未授通知权限（no-token）则整个会话无任何注册路径（F28）。(c) `openNotificationFromInbox` 声明 `Pick<InboxItem,'notificationId'>` 与实现（resolveTarget 读 deepLink）不符，`as InboxItem` 断言让类型系统无法拦截（F29；notification-inbox.ts:171-173 已核实读 item.deepLink）。(d) `createNativeRequestId` 的 CSPRNG 路径依赖 `globalThis.crypto`，但 apps/mobile 无任何 polyfill（package.json 依赖已核实无 expo-crypto/react-native-get-random-values）——Hermes 下 Math.random 兜底成为唯一实际路径，幂等关联键强度静默退化，与 mobile-core 缺省 fail-closed（task-office.ts:293-294 无 crypto.randomUUID 直接 throw——已核实）纪律不一致（F27）。(e) `deviceId()` 无 in-flight 去重——并发首次调用各自生成 uuid 后写覆盖先写，可产生收不到推送的幽灵设备记录（F49）。

- [ ] **Step 1: 写失败测试**

`apps/mobile/src/adapters/request-id.test.ts`（新建）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createNativeRequestId } from './request-id.ts';

test('request ids are v4-shaped and unique (strength floor)', () => {
  const next = createNativeRequestId();
  const seen = new Set<string>();
  for (let index = 0; index < 200; index += 1) {
    const id = next();
    assert.match(id, /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/, 'v4 形态（版本与变体位）');
    seen.add(id);
  }
  assert.equal(seen.size, 200, '幂等关联键不可碰撞');
});

test('the generator prefers the expo-crypto CSPRNG when resolvable', async () => {
  const source = await import('node:fs').then((fs) => fs.readFileSync(new URL('./request-id.ts', import.meta.url), 'utf8'));
  assert.match(source, /expo-crypto/, 'B3-F27：CSPRNG 路径必须真实可达（惰性 require expo-crypto），不依赖 Hermes 不存在的 globalThis.crypto');
});
```

`device-identity` 同型测试（并入新建 `device-identity.test.ts`）：

```ts
test('concurrent first deviceId calls resolve to one persisted identity', async () => {
  const writes: string[] = [];
  const store: SecureStorePort = {
    getItemAsync: async () => null,
    setItemAsync: async (_key, value) => { writes.push(value); },
    deleteItemAsync: async () => {},
  };
  const identity = createSecureDeviceIdentity(store);
  const [a, b, c] = await Promise.all([identity.deviceId(), identity.deviceId(), identity.deviceId()]);
  assert.equal(a, b); assert.equal(b, c);
  assert.equal(writes.length, 1, 'B3-F49：并发首次调用只落一次盘，不产生幽灵设备记录');
});
```

`app-smoke.test.tsx` 追加源级断言：

```ts
test('composition caches instances by deployment scope key and registration failures do not permanently occupy an origin', async () => {
  const { readFileSync } = await import('node:fs');
  const source = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(source, /deploymentScopeKey\(/, 'B3-F26：四个模块级缓存必须以 origin::tenant 为键');
  // B3-F28：registeredFor.add 移到成功回调之后（源级：注册调用不再先标记）
  assert.doesNotMatch(source, /registeredFor\.current\.add\(next\.deployment\.origin\);\s*\n\s*void registerActiveDeviceIfPossible/);
});
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/adapters/request-id.test.ts apps/mobile/src/adapters/device-identity.test.ts apps/mobile/src/app-smoke.test.tsx`
Expected: FAIL（新建文件：模块/断言红——expo-crypto 未引用、writes.length === 3；源级：缓存键匹配失败、先标记模式匹配命中）。

- [ ] **Step 3: 最小实现**

```ts
// composition.ts —— F26：四个缓存工厂统一走 scope key（以 taskOfficeFor 为例，其余三个同型）：
const taskOffices = new Map<string, TaskOffice>();
function taskOfficeFor(activeRuntime: MobileRuntime, origin: string, tenantId: string): TaskOffice {
  const key = deploymentScopeKey(origin, tenantId);
  let office = taskOffices.get(key);
  if (!office) {
    if (taskOffices.size >= 8) taskOffices.delete(taskOffices.keys().next().value!); // 防泄漏：清最旧
    office = createTaskOffice({ /* 既有装配 */ });
    taskOffices.set(key, office);
  }
  return office;
}
// 三个调用方（TasksScreen 宿主/activeTaskOffice/ReadOnly 宿主等）从 snapshot.identity.activeTenantId ?? '' 取 tenantId 传入——
// 调用点全部在组合根内（snapshot 在手），无 UI 层泄漏。

// F28 —— 注册失败不永久占用（会话内每 origin 最多 2 次尝试）：
const registrationAttempts = useRef(new Map<string, number>());
useEffect(() => {
  const unsubscribe = activeRuntime.subscribe((next) => {
    if (next.surface !== 'authorized' || !next.deployment) return;
    if (registeredFor.current.has(next.deployment.origin)) return;
    const attempts = registrationAttempts.current.get(next.deployment.origin) ?? 0;
    if (attempts >= 2) return; // 有界重试：no-token/failed 不无限循环（surface 发布次数有限，双保险）
    registrationAttempts.current.set(next.deployment.origin, attempts + 1);
    void registerActiveDeviceIfPossible(activeRuntime)
      .then(() => { registeredFor.current.add(next.deployment!.origin); }) // 成功才标记
      .catch(() => undefined); // 失败：下次 surface 变化再试（直至会话上限）
  });
  return unsubscribe;
}, [activeRuntime]);

// F29 —— 参数类型对齐实现：
item: Pick<InboxItem, 'notificationId' | 'deepLink'>,
...
const target = inbox.resolveTarget(item); // 删除 as InboxItem 断言

// request-id.ts —— F27：CSPRNG 真实可达
export function createNativeRequestId(): () => string {
  return (): string => {
    if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID();
    try {
      const expoCrypto = require('expo-crypto') as { randomUUID?: () => string };
      if (typeof expoCrypto.randomUUID === 'function') return expoCrypto.randomUUID(); // RN/Hermes 主路径
    } catch { /* expo-crypto 未安装（测试 stub 环境）：落既有兜底链 */ }
    /* 既有 getRandomBytes/Math.random 兜底保持（注释记录：与 mobile-core fail-closed 的权衡——
       apps 端 throw 会让提交全废，兜底强度低于 CSPRNG 但可用性优先；expo-crypto 为缺省安装项） */
  };
}
// apps/mobile/package.json —— dependencies 追加 "expo-crypto": "^14.0.0"（版本以 workspace 既有 expo 家族版本对齐为准，执行时 pnpm install 后锁定）

// device-identity.ts —— F49：
export function createSecureDeviceIdentity(store: SecureStorePort): NativeDeviceIdentity {
  let inflight: Promise<string | undefined> | undefined;
  return {
    async deviceId(): Promise<string | undefined> {
      if (inflight !== undefined) return inflight; // 并发首次调用共享同一次 read-generate-write
      inflight = (async () => { /* 既有逻辑逐字保留 */ })();
      try { return await inflight; } finally { inflight = undefined; } // 完成后允许后续直读
    },
  };
}
// 顺手 F48：registration.deviceId 空串死守卫删除（requireDeviceId 构造期已抛——报告指认；执行时定位 composition.ts 内的守卫行删除并在 commit message 注明）。
```

- [ ] **Step 4: 实跑确认通过 + 回归**

Run: `pnpm install --filter @weknora/mobile && pnpm exec tsx --test apps/mobile/src/adapters/request-id.test.ts apps/mobile/src/adapters/device-identity.test.ts apps/mobile/src/app-smoke.test.tsx apps/mobile/src/attention-inbox-view.test.ts`
Expected: PASS（expo-crypto 安装后源级与行为级断言全绿；attention-inbox-view 覆盖 F29 的调用方类型变化）。
Run: `pnpm run typecheck:mobile`
Expected: PASS（F29 的 Pick 类型收紧无其他调用方破坏——生产调用方传全量对象，报告已核实）。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/composition.ts apps/mobile/src/adapters/request-id.ts apps/mobile/src/adapters/request-id.test.ts apps/mobile/src/adapters/device-identity.ts apps/mobile/src/adapters/device-identity.test.ts apps/mobile/package.json apps/mobile/src/app-smoke.test.tsx pnpm-lock.yaml
git commit -m "fix(mobile): scope-keyed composition caches, retriable device registration, real CSPRNG ids, and deduped identity"
```

---

### Task 15: apps/mobile：task-start smoke 证据链（B3-F45、F60）

**Files:**
- Modify: `apps/mobile/src/task-start-integration-smoke.ts:66-110`（total 化收口）

**Interfaces:**
- Consumes: Task 6 后的语义（同 ID 重入 bound 意图 = 零网络幂等回执）；batch2 Task 9 的 `task-detail-integration-smoke.ts` total 化先例（try/catch/finally + `failure` 字段）。
- Produces: `runTaskStartIntegration` **total**（从不 reject）：任何步骤异常 → `evidence.start === 'failed'` + `errorReason`（错误 message，证据契约无凭据字段）；第二次同 ID 重入包 try/catch——异常时 `repeatSubmitSameRequest = 'failed'` + errorReason 而非整函数崩溃（F45；Task 6 后预期 `no-second-dispatch`，但保护保留以兑现「including failed live outcomes」契约）；`runVisibleInTasks` 探测保持既有 try/catch（F60 的 signIn/authorizedRequest/首次 start 三个裸 await 一并收口）。

**根因与修复说明：** 同根因——smoke 的异常路径绕过证据契约：`signIn`(:71)/`authorizedRequest`(:77)/首次 `office.start`(:92)/第二次重入(:101) 均无 try/catch，任一抛错整函数 reject、evidence 对象被丢弃（F60/F45）——`evidence.start='failed'` 与 `errorReason` 的设计意图即「失败仍产出证据」；对照 :104-109 的 `office.tasks({})` 有保护（已核实）。此文件在 Task 6 之后修改：重入语义已变为幂等回执，断言按新行为设计。

- [ ] **Step 1: 写失败测试（追加到 task-start-integration-smoke.test.ts）**

```ts
test('the integration runner is total: a failing transport still yields evidence, not a rejection', async () => {
  const source = readFileSync(join(here, 'task-start-integration-smoke.ts'), 'utf8');
  // 结构级断言（对照 batch2 Task 9 的先例模式）：主流程包 try/catch/finally；第二次重入不得裸 await
  assert.match(source, /finally\s*\{/);
  assert.match(source, /catch \(error\)/);
  const secondDispatch = source.indexOf('office.start(goal, { requestId: first.requestId })');
  assert.ok(secondDispatch === -1 || !/await office\.start\(goal, \{ requestId: first\.requestId \}\);\s*\n\s*evidence\.repeatSubmitSameRequest = second/.test(source), 'B3-F45：第二次重入必须有 try/catch 保护');
});
// 行为级（无需真实部署）：config 校验沿用既有用例；runner 的 total 语义经结构断言 + emit 契约锁定。
```

- [ ] **Step 2: 实跑确认失败**

Run: `pnpm exec tsx --test apps/mobile/src/task-start-integration-smoke.test.ts`
Expected: FAIL（当前无 finally；第二次重入裸 await）。

- [ ] **Step 3: 最小实现**

```ts
// 结构收口（对照 batch2 Task 9 的 task-detail-integration-smoke.ts 骨架——「原样保留」= 现行 :55-77 的装配逐字保留）：
export async function runTaskStartIntegration(config: Extract<TaskStartIntegrationConfig, { enabled: true }>): Promise<TaskStartIntegrationEvidence> {
  const evidence: TaskStartIntegrationEvidence = { deploymentOrigin: config.deploymentOrigin, start: 'failed', repeatSubmitSameRequest: 'failed', runVisibleInTasks: 'unavailable', timestamp: new Date().toISOString() };
  const fetcher: FetchLike = (input, init) => fetch(input, init as RequestInit);
  const runtime = createMobileRuntime({ /* 既有装配逐字保留 */ });
  try {
    const snapshot = await runtime.signIn({ /* 既有参数 */ });
    if (snapshot.surface !== 'authorized' || !snapshot.deployment) {
      evidence.errorReason = `surface ${snapshot.surface}`;
      return evidence;
    }
    /* :76-99 的 agents 探测 + office 装配 + 首次 start 逐字保留（evidence 字段赋值不变） */
    // 第二次同 ID 重入（B3-F45）——Task 6 后预期零网络幂等回执，但失败也产出证据：
    try {
      const second = await office.start(goal, { requestId: first.requestId });
      evidence.repeatSubmitSameRequest = second.dispatched === false && second.runId === first.runId ? 'no-second-dispatch' : 'second-dispatch';
    } catch (error) {
      evidence.repeatSubmitSameRequest = 'failed';
      evidence.errorReason = `replay rejected: ${error instanceof Error ? error.message : String(error)}`;
    }
    /* :104-109 的 tasks 探测保持既有 try/catch */
  } catch (error) {
    evidence.start = 'failed';
    evidence.errorReason = error instanceof Error ? error.message : String(error); // 失败仍产出证据（不含凭据）
  } finally {
    runtime.dispose?.(); // runtime 有 dispose 则释放（以 MobileRuntime 接口实际导出为准；无则删除此行）
  }
  return evidence;
}
```

（`runtime.dispose` 的存在性以 mobile-runtime 接口为准——batch2 Task 9 骨架调用过 `runtime.dispose()`，先例成立。）

- [ ] **Step 4: 实跑确认通过**

Run: `pnpm exec tsx --test apps/mobile/src/task-start-integration-smoke.test.ts`
Expected: PASS（本地无环境时 integration 按 opt-in skip，不得伪造通过）。
Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/task-start-integration-smoke.ts apps/mobile/src/task-start-integration-smoke.test.ts
git commit -m "test(mobile): make the task-start integration smoke total with replay protection"
```

---

## 附录 A：lowWorthFixing 项的处置映射

**折叠进任务（同文件、低风险，随所属任务实现与验收）：**

| 项 | 折叠进 | 处置 |
|---|---|---|
| B3-F62 resume 丢弃 lookup 结果转双查 | Task 5 | 直接用 lookup 结果落态 |
| B3-F68 Start wire 字面量无注释 | Task 6 | toStartInput 调用处契约注释 |
| B3-F66 legacyHistory 未走 settle/epoch | Task 13 | 对照 legacyTasks 的 epoch 模式 |
| B3-F46 预算输入 0/未设置混同 | Task 8 | NewTaskScreen 本地中间态 budgetText |
| B3-F2 merge 空串 nextCursor 未归一 | Task 9 | `'' → undefined` |
| B3-F6 register 响应 Number() 无校验 | Task 9 | revision/scope_generation 安全整数校验 + fail-closed |
| B3-F50 attemptRegister 迟到成功未复检 lease | Task 9 | 成功返回前 leaseActive 复检 |
| B3-F47 inbox limit 不可传 | —— | **延期**（见下：描述不完整，差异记录 5） |
| B3-F32 预览字节入缓存前未复检 | Task 11 | fetchBytes 复检 PREVIEW_MAX_BYTES |
| B3-F33 act 在 mintGrant 后才查 share | Task 11 | 顺序对调 |
| B3-F34 decodeUtf8 Latin-1 回退 | Task 11 | 正确 UTF-8 解码循环 |
| B3-F35 citations detail 无截断 | Task 11 | 2000 字符截断 |
| B3-F36 nextCursor 合同与运行时不一致 | Task 12 | ports.ts 合同注释对齐 |
| B3-F10 免凭据 fetch 无超时 | Task 12 | 30s AbortController |
| B3-F18 refresh 未重置 notice / 缺 alive 卫护 | Task 12 | resources-view 对齐既有 alive 模式 |
| B3-F53 可继续加载提示永不消失 | Task 12 | F52 修复后游标信号收敛；渲染点未定位（差异记录 4），交互承载延期 |
| B3-F48 deviceId 空串死守卫 | Task 14 | 删除死代码（requireDeviceId 构造期已抛） |
| B3-F14 错误文案缺 SUPERSEDED | Task 13 | LEGACY_ERROR_COPY 映射 |
| B3-F73 decodeIDList 忽略解析错误 | Task 4 | 解析失败可观测（log/空列表 + 信号） |
| B3-F74 TaskOwnerID NULL 扫描 | Task 2 | sql.NullString 扫描 |
| B3-F75 UpsertGrant created_at 不一致 | Task 2 | 冲突路径重读 DB 行返回 |

**明确延期（记录理由，不在本批次）：**

- **B3-F70 能力映射资源存在性校验 / B3-F71 capability verdict 硬编码 supported**：需要跨 model/knowledge_base/connection 的存在性读模型（repo 新查询 + 表/模型设计）与 unavailable/forbidden 判定设计；`PublishedAvailableAgents` 现行查询不 JOIN 资源表（已核实 repository/agent_adoption.go:247-289），单端修治理面而 CreateAgent 同样不校验，收益有限。应立独立任务「Agent 资源引用存在性校验与 availability verdict」统一设计。
- **B3-F4 默认 limit 50/上限 200 三层硬编码**：跨 mobile-core/api-client/Go 三层的常量统一是结构化重构，需独立任务与三层回归。
- **B3-F8 替身 intent 校验依赖单一全局 epoch**：测试替身健壮性，无生产影响。
- **B3-F16 decide 按钮在途/加载未禁用**：attention-inbox UI 增强（apps/mobile AttentionInboxScreen），与 B3 主发现不同文件域；快速连点产生的重复 receipt 文案无数据破坏（decide 幂等由服务端 revision 保证）。
- **B3-F13 其余 5 个 integration smoke 的早退无 failure 摘要**：task-start 自身已有 errorReason（差异记录 3）；attention/device-inbox/legacy/material/task-office 五个 smoke 的同型收口待各自域任务（本批次 Task 15 仅收口 task-start——不扩大范围）。
- **B3-F47 inbox limit 参数**：报告描述被截断且 TS 侧未定位到硬编码点（差异记录 5）；执行批次时可从 `workbench_inbox.go:193` 的默认页大小复核后决定是否立任务。
- **B3-F72 ListAdoptions N+1**：批量化需 repo 接口变化（ListVariantsByAdoption per-adoption → 批量 IN 查询），性能优化超出修复批次。
- **B3-F77 ApproveAction 布尔合并**：位于 `internal/handler/app_connector_action.go:207+`（app connector 域，已核实位置）——与 B3 主发现（task_grant/adoption/workbench/mobile）不同域，且「基础设施故障 vs 记录不存在」的分流涉及该端点的错误契约设计，独立处理。
- **B3-F88 ListPending expires_at 谓词与 overview 投影不一致**：读模型对齐需要先确定权威投影语义，独立复核。
- **migrations/sqlite 000112 序号冲突**（批次外新发现，见 Global Constraints）：重编已发布迁移序号有部署影响，升级为独立决策。

## 附录 B：批次验收（全部任务完成后）

1. `pnpm exec tsx --test packages/domain/src/mobile/ packages/mobile-core/src/ packages/api-client/src/mobile/ apps/mobile/src/` —— 全绿（integration 用例按 opt-in skip，不得伪造通过）。
2. `go test ./internal/application/service/ ./internal/application/repository/ ./internal/router/ ./internal/handler/` —— ok。
3. `go test ./internal/handler/session/` —— 除预存在失败 `TestWorkbenchStartHTTPIntegrationAndIdentityIsolation`（migration 000112 冲突，见 Global Constraints）外全绿；失败集合与基线一致（不扩大）。对照基线命令：`go test ./internal/handler/session/ -run 'TestGetWorkbench|TestListWorkbench' -v` 必须全 PASS。
4. `npx tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions packages/contracts/src/marketplace/agent-adoption.ts` —— exit 0；`pnpm run typecheck:shared` 失败集合较基线**收窄**（agent-adoption 清零，mermaid ×2 保持——范围外）。
5. `pnpm run typecheck:mobile` —— PASS（Task 11 的 F55 收紧与 Task 14 的依赖变更无类型遗漏）。
6. `grep -rn "function requireDeploymentOrigin" packages/api-client/src/mobile/ | wc -l` —— `0`。
7. 抽查清单（人工复核，无需自动化）：B3 报告 6 项 high 的用户可见症状逐条对上——跨重启重试同 ID（F37/F78：重入零 createSession 零 POST）；超大合法 goal 显式报错而非永久失败（F38）；离线首开 New 屏草稿不丢（F59）；bound 后 remove 失败不再伪装失败（F64）；重放不毒化 requestId（F78）；typecheck 门禁内本文件清零（F85）。
