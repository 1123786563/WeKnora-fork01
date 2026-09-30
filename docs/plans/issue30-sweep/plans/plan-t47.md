# T17：多来源并行研究与版本化报告（Issue #47）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lead Agent 把活跃 Run 内的研究委派为并行只读子任务（源 ⊆ 任务租户知识范围、结构只读、绝不占用单写槽），成员在移动端对任一材料版本追加批注、并经既有命令通道提出绑定确定版本的修订请求；一切变更以新版本呈现，既有/已批注版本的版本身份与内容保持不可变——三条验收标准全部在 Go 真实迁移 E2E 与 mobile-core 最高稳定 Interface 上可验证。

**Architecture:** 服务端复用两条已冻结的不可变事实：`AgentRunStore.Admit` 的 `sessions.active_agent_run_id` 单写槽 CAS（`internal/application/repository/agent_run.go:222-229`，写 Run 互斥已是既有语义）与消息绑定工件的版本身份派生 `artifactVersionOf`（`internal/handler/session/workbench_artifacts.go:77-86`，ContentHash 前 16 hex）。本计划只补三块 Go 缺口：①`task_research_delegations` 表上的**只读研究委派**（源经租户知识库权威校验，委派路径零写原语、不触碰单写槽）；②`task_artifact_annotations` 表上的**版本钉定批注**（append-only，`base_version` 必须等于批注时刻的当前版本身份，否则 409）；③两条读面（owner + granted 回退，复用 #42 `GetRunForGrantedReader`）。客户端按 module-seams §7 同款范式新建 `packages/mobile-core/src/research/` 深模块：`open({lease}) → TaskResearchHandle`（delegate/annotate/requestRevision/flushAnnotationDrafts/subscribe/close），修订请求组合确定性版本钉定文本后委托 #37 的 `TaskCommandPort`（steer/queue_next 既有通道，不自建命令端点）；离线批注落 Scoped Vault 加密草稿（design spec：「Offline mode permits approved reads, drafts and annotations」），联网后显式 flush 不自动重放；`packages/api-client` 新增 `createMobileResearchRemote`；apps/mobile 新增 `/tasks/research` 路由、ResearchScreen、composition 工厂与 opt-in 真实集成证据。

**Tech Stack:** Go 1.26（gin + gorm + testify，`go test`）、TypeScript（`packages/contracts`、`packages/api-client`、`packages/mobile-core`、`apps/mobile` Expo RN）、node:test + tsx（TS 测试运行器，与本 sweep 各前批一致）。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置 `pnpm install` 已就绪。本计划作者已实跑以下与迁移轨道无关的基线，全部绿色：`pnpm exec tsx --test packages/mobile-core/src/material/task-material.test.ts` pass（16/16）；`pnpm exec tsx --test packages/api-client/src/mobile/materials.test.ts` pass；`pnpm --filter @weknora/mobile exec tsx --test src/materials-view.test.ts` pass。此外，作者把本计划 Task 0 的条件去重 + 新迁移与 Task 1–3 的全部 Go 代码（types/repository/handler/三份测试）**真实落盘实跑过一轮**：handler 测试 8 项、store 测试 3 项、E2E 测试 3 项全部 PASS（详见「与调查结论的差异记录」第 5 条），验证后已把共享树精确还原。**迁移轨道基线的诚实声明**：波起点 HEAD 上 `go test ./internal/database/` 与一切经 `openTaskGrantDB` 装载全量 sqlite 迁移的测试（含 `TestTaskCollaborationEndToEndAC1` 与本计划 Task 1/2/3 的 store/E2E 测试）都会以 `duplicate migration file: 000114_public_agent_marketplace.down.sql` FAIL——该失败只在 Task 0 Step 2 条件去重完成后消失（作者实跑：去重后同一命令 ok；去重前 FAIL）。因此本计划的执行顺序是强制的：Task 0 未完成前不要跑任何依赖迁移装载的测试，也不要把它们误读为代码错误。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-47.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`
  - User Story 28：「As a researcher, I want the Lead Agent to delegate independent read-only research using least privilege, so that work can proceed in parallel safely.」
  - Implementation Decisions：「One Task permits at most one write Run. Independent read-only delegation may run concurrently and is combined by the single writer.」「Task Grant is subordinate to member permission and Tenant policy. Delegated Agents receive only a minimum subset. Agent declarations never create permission.」「Lead Agent Version, Artifact versions, Action Plans and candidate code commits are immutable approval anchors. Changes invalidate prior approvals.」「Task Material owns evidence, Artifact versions, Files, Diff, tests, read-only Terminal, download, share and annotation.」「Offline mode permits approved reads, drafts and annotations. It prohibits Run commands, approval, budget expansion and external Actions.」
  - Testing Decisions：「Tests target observable behavior at the highest stable Interface.」「Go integration tests verify authorization, Tenant/Owner/Collaborator predicates, revision and digest CAS, idempotency, partial external success, unknown outcomes and durable checkpoints.」「Remote-owned dependencies use in-memory scenario Adapters for Module tests and real HTTP/SSE contract tests for production Adapters.」
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§3 依赖方向、§4.2 Scope Lease 纪律、§7 Task Material Module 同款模块范式、§10 App Shell 禁止事项）
- ADR：`docs/adr/0004-task-is-session.md`（taskId = sessionId）、`docs/adr/0008-developer-delivery-and-single-writer.md`（「同一 Task 至多一个 Run 可以写入工作区、产物草稿或外部目标，只读子执行可以并行」）、`docs/adr/0012-mobile-business-logic-lives-behind-deep-modules.md`（业务逻辑藏深 Module 后）
- 领域术语：`CONTEXT.md`（「主理 Agent（Lead Agent）」：可在任务授权范围内把工作委派给专业 Agent，但任务对用户仍保持一个责任主体；「委派授权（Delegated Grant）」：从 Task Grant 划出的最小权限子集，专业 Agent 不能借委派获得 Task 未授权的权限；「任务产物（Task Artifact）」：不能原地覆盖已存在或已审批的版本；「工作区（Workspace）」：同一任务至多一个运行可以写入，只读子执行可以并行）
- Parent：Issue #30；Blocked by：#45（T15，已合并——`askKnowledge`/evidence 帧在当前 HEAD 亲眼核实）、#46（T16，已合并——`createTaskMaterial`/terminal-log/artifacts 版本身份在当前 HEAD 亲眼核实）；本 Issue 阻塞 #71
- 前序批次产出（本计划 Consumes，全部在当前 HEAD 亲眼核实）：
  - `session.resolveOwnedRun`/`workbenchCaller`/`OwnedRunReader`/`GrantedRunReader`（`internal/handler/session/workbench_read.go:23-47/:141-155/:181`）
  - `session.artifactVersionOf`（`internal/handler/session/workbench_artifacts.go:77-86`）与 `ArtifactRefReader`（同文件 :19-21，生产实现 `repository.messageRepository.GetSessionArtifactRefs`，`internal/application/repository/message.go:452`）
  - `types.TaskRoleCanRun`/`TaskAccessRole`（`internal/types/task_grant.go:29-49`）、`service.NewTaskGrantService(...).ResolveTaskAccess`（`internal/application/service/task_grant.go:158-202`）
  - `repository.NewKnowledgeBaseRepository(db).GetKnowledgeBaseByIDAndTenant`（miss 返回 `repository.ErrKnowledgeBaseNotFound`，`internal/application/repository/knowledgebase.go:43-53`）
  - `repository.NewAgentRunStore(db).Admit` 的单写槽 CAS（`internal/application/repository/agent_run.go:222-229`，`agentruntime.ErrRunActive`）
  - mobile-core：`ScopeLease`/`leaseActive`/`leaseScopeOf`（`packages/mobile-core/src/runtime/scope-lease.ts:21-26`，包内可见）、`TaskCommandPort`（`packages/mobile-core/src/task-office/task-detail.ts:87-89`，公共导出见 `index.ts:37`）、`OfflineGate`/`OFFLINE_ACTION_BLOCKED`（`packages/mobile-core/src/offline/offline-gate.ts`）、`ScopedStore.drafts`（`packages/mobile-core/src/vault/scoped-vault.ts:24-31`，草稿 id 白名单 `^[A-Za-z0-9._-]{1,64}$`，同文件 :7）、`MaterialBackendPort`/`MaterialEntry`（`packages/mobile-core/src/material/ports.ts:58-63`、`types.ts:20-31`）
  - api-client：`ClientRequest` 授权通道与 `requireDeploymentOrigin`（`packages/api-client/src/mobile/deployment-origin.ts`）、`unwrap` 成功信封范式（`packages/api-client/src/mobile/materials.ts:41-48`）
  - apps/mobile：composition 记忆化工厂范式（`apps/mobile/src/composition.ts:152-160` `cachePut`、:301-314 `taskMaterialFor`/`activeTaskMaterial`）、路由生命周期宿主范式（`apps/mobile/src/app/tasks/materials.tsx`）、opt-in 集成证据范式（`apps/mobile/src/material-integration-smoke.ts`，`disallowedDeploymentHost` 主机防线自 `runtime-integration-smoke.ts` 导入）

## Global Constraints

以下为批准 Spec / ADR / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「One Task permits at most one write Run. Independent read-only delegation may run concurrently and is combined by the single writer.」（mobile-ai-office-design.md · Implementation Decisions）——只读委派**绝不**读写 `sessions.active_agent_run_id`（单写槽只属于写 Run 的 `Admit` 路径）；本计划不放松也不复制该槽语义。
- 「Task Grant is subordinate to member permission and Tenant policy. Delegated Agents receive only a minimum subset. Agent declarations never create permission.」（同上）——委派源必须在任务租户知识范围内（服务端权威校验）；委派记录本身不新增任何授权行（`task_grants` 面不变，仍 owner-only）。
- 「Lead Agent Version, Artifact versions, Action Plans and candidate code commits are immutable approval anchors. Changes invalidate prior approvals.」（同上）——批注 append-only，绝不改写既有版本的字节、摘要或版本身份。
- 「Task Material owns evidence, Artifact versions, Files, Diff, tests, read-only Terminal, download, share and annotation.」（同上）——本计划交付批注的**版本化语义层**与移动入口；材料预览/下载/分享继续经 #46 `createTaskMaterial`，本计划不在 research 模块内重建它们。
- 「Offline mode permits approved reads, drafts and annotations. It prohibits Run commands, approval, budget expansion and external Actions.」（同上）——离线批注落加密草稿 + 显式 flush（不自动重放）；修订请求属 Run 命令，离线一律拒绝（OfflineGate 断言）。
- 「REST submits commands and loads authoritative Snapshots. Cursored SSE carries durable Task/Run events. WebSocket or WebRTC is reserved for real-time voice.」（同上；ADR-0006 同义）——本计划全部走授权 REST，不新增任何流通道。
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）
- 「Cache identity includes Deployment, user, Tenant, Task and Run where applicable. Scope changes invalidate subscriptions and reject late responses.」（同上）——research 句柄每次异步提交前检查 lease 有效性；scope 撤销后一切写路径 fail closed（RESEARCH_SCOPE_CHANGED）。
- 「Interface 不暴露 token、query key、generation number 或 SecureStore key. Scope Lease 是不透明、可撤销的能力对象，子 Module 每次异步提交前检查其有效性。」（mobile-module-seams.md §4.2）
- 「Screen 不调用多个 wire 方法……Module 内部决定顺序、幂等、重连、revision 和错误呈现。」（mobile-module-seams.md §5.2，同义适用于 research 模块）；「禁止：Screen 直接导入 packages/contracts 或 packages/api-client」（§10）
- 「移动 AI Office 将现有 WeKnora Session 呈现为 Task，并保持 `taskId = sessionId`，不新增第二个 Task 聚合身份。」（ADR-0004）——委派/批注 wire 全部按 run 寻址（`/workbench/executions/:run_id/...`），session 即 task，不另造身份。
- 「同一 Task 至多一个 Run 可以写入工作区、产物草稿或外部目标，只读子执行可以并行。」（ADR-0008）
- 安全约束（会话注入）：服务端 SQL 一律参数绑定（本计划新查询全部 `?` 占位 / gorm 绑定，不拼接外部输入）；服务端请求 URL 仅 http/https 且发请求前校验 host（本计划服务端不发起外呼；客户端 blob/主机防线沿用既有 `disallowedDeploymentHost`）；凭据只从环境变量读取，源码与测试不写入可用凭据字面量（集成证据沿用 `WEKNORA_MOBILE_TEST_*` opt-in 模式，缺凭据显式 skip 不伪造）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。

**Issue #47 验收标准原文（docs/plans/issue30-sweep/issues/issue-47.md）：**

1. 「只读子任务不能扩大 Task Grant 或产生冲突写入。」
2. 「批注/修改生成新版本，已审批版本保持不变。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明（blocked-env 声明）：真实端到端（生产 JSON transport + 授权通道 + 具体 Remote Adapter + research 编排 + 真后端委派/批注）沿用 T01–T16 已合并的 opt-in 真实 HTTP 模式，需要「一个真实 WeKnora Deployment（HTTPS origin）+ 一个测试账号 + 一个已有材料产出的任务」（`WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` 环境变量）。本地无此环境时 Task 8 的真实 HTTP 用例以 `t.skip` 跳过（**不得伪造通过**）。本地替代证据：Task 3 的 Go E2E（真实 sqlite 迁移库 + 真实 store/handler + httptest 全链，覆盖 AC1/AC2 全部分支）+ Task 6 的 mobile-core Interface 级场景测试（真实模块编排 + in-memory scenario Adapter）+ Task 4/5 的 wire 契约测试（真实序列化字节）+ Task 7 的控制器/路由测试。凡具备环境的运行都自动产出端到端证据。集成证据对被探测任务留下的委派与批注是 append-only 的非破坏性记录（无删除端点是设计事实），如实声明、不做清理伪装。

**边界声明（本计划不做什么）：**
- 委派的实际执行仍由现有 Agent 执行引擎在写 Run 内消化（本计划交付委派记录、范围围栏与只读并行语义，不新增执行引擎路径）。
- 修订请求不自建 REST 端点：复用 #37 命令通道（`steer`/`queue_next`），版本钉定由客户端确定性组合 + 服务端既有命令语义承担。
- 离线**修订请求**不做草稿（属 Run 命令，spec 明文离线禁止）；离线**批注**做草稿（spec 明文允许）。
- 材料预览/下载/分享、知识问答、终端日志不在本计划（#46/#45 已交付，research 模块只消费材料索引的版本身份）。

## 与调查结论的差异记录（以代码现状为准）

1. 调查称「无任务级单写者准入……不存在'同一 Task 至多一个写 Run'的语义」——**已过时**。亲眼核实：`AgentRunStore.Admit`（`internal/application/repository/agent_run.go:222-229`）在事务内以 `UPDATE sessions SET active_agent_run_id = ? WHERE ... AND active_agent_run_id IS NULL` 的 CAS 抢槽，槽被占即 `agentruntime.ErrRunActive`；#37 的 `CancelOwnedRun` 释放同一槽。因此「写 Run 互斥」是既有语义，本计划的对照断言（Task 3）直接验证它，并把只读委派实现为**不经过** `Admit` 的独立记录路径——只读子执行因此天然并行、也天然不可能与写 Run 冲突写。
2. 调查称「Task Grant 概念代码缺位（rg 'TaskGrant' 无命中）」——**已过时**。#42 已交付 `types.TaskGrant`/`task_grants` 表/`TaskGrantService`（含 `ResolveTaskAccess` 跨租户统一 404）与 owner-only grants API（`internal/application/service/task_grant.go`，`internal/router/routes_workbench.go:233-241`）。本计划消费 `TaskRoleCanRun`/`ResolveTaskAccess` 作批注写门，并把「委派不能扩大 Task Grant」落实为：委派源须通过租户知识库权威校验 + `task_grants` 面零改动（Task 3 回归断言）。
3. 调查称「批注无任何实现」——**属实**，本计划交付。#46 在 `MaterialIntent` 边界明确排除 annotate（plan-t46.md:24/:3126），并把批注/基于版本请求修改划归 #47。
4. 波级问题 1（迁移 000114/000193 同号双迁移破坏全量轨道）——撰写期间该共享 worktree 的状态在「已去重」与「波起点」之间变动过两次（并行编排方所为）：`mobile_device_app` 曾被重编号为 sqlite 000118 / versioned 000197（含 5 个测试文件 pin 更新），随后又被整体还原回波起点形态（重复迁移回归）。因此本计划 Task 0 把**条件去重**定为正式前置步骤，去重目标与上述曾落地形态逐字一致（mobile_device_app → 000118/000197）；若执行时文件已是 000118/000197 则该步为纯验证、零改动。本计划作者已按该形态实际执行去重并完成了全部 Go 代码的实跑验证（见差异记录第 5 条），验证后已把共享树精确还原回波起点，未留任何未提交迁移改动。
5. **本计划 Go 代码的实跑证据（作者完成，非纸面推导）**：在共享 worktree 上临时应用 Task 0 条件去重 + 写入 000119 迁移 + 落盘本计划 Task 1–3 的 6 个 Go 文件后实跑：`go test ./internal/handler/session/ -run 'TestDelegateResearch|TestListResearch|TestCompleteResearch|TestAnnotateMaterial|TestListAnnotations' -count=1` → ok（1.5s）；`go test ./internal/application/repository/ -run 'TestTaskResearchStore|TestTaskAnnotationStore|TestTaskResearchEndToEnd' -count=1` → ok 6/6 PASS（含 AC1 并行只读 vs 写槽互斥对照、源越权 400、grants 面 403、跨租户 404、钉版批注 201/409、修订新版本不可变全链）。实跑中发现并已回写修正计划的两处事实：① handler 需要 `agentruntime` import；② `types.Message` 的 BeforeCreate 会无条件重生成 ID，种子消息必须 `db.Session(&gorm.Session{SkipHooks: true}).Create(...)` 才能保住 (message_id, index) 材料寻址。验证完成后全部临时文件已精确还原（`git checkout` + 定点删除），共享树仅遗留本计划文件。
6. 本计划新迁移占用 sqlite 000119 / versioned 000198——即「Task 0 条件去重完成后」两轨道的下一可用号（波起点 HEAD 最大号为 sqlite 000117 / versioned 000196；去重把 mobile_device_app 顺延为 000118/000197；两种形态都已实读 `migrations/` 目录核实）。集成时若被同批其它计划先占，按 t48 先例「四方一致约定整体顺延，DDL 零变化」。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **委派源越权（Task Grant 被委派扩大）**：调用方提交一个租户外/不存在/已删除的知识库作为研究源。服务端必须在落库前以租户绑定校验拒绝（400 `research_source_out_of_task_grant`），绝不先记录再校验。——Task 2 单测 `TestDelegateResearchRejectsSourceOutsideTenantScope` + Task 3 E2E `TestTaskResearchEndToEndSourceOutsideTenantScope`（真实 KnowledgeBaseRepository + 真实迁移库）。
2. **批注钉版过期**：用户在看到列表与提交批注之间，写 Run 产出了新版本；若批注仍以旧 `base_version` 落到新版本材料上，「已审批版本保持不变」的审计链断裂。服务端必须把 `base_version` 与该材料**当前**版本身份比对，不一致即 409 `annotation_base_version_conflict`。——Task 2 单测 `TestAnnotateMaterialRejectsStaleBaseVersion` + Task 3 E2E 断言。
3. **只读委派被误当成写通道**：委派路径若在任何分支调用 `Admit`/触碰 `active_agent_run_id`，只读并行即被单写槽拒绝或反过来挤掉写 Run。结构保证：委派 handler 不持有 `AgentRunStore`。E2E 对照断言：写 Run 占槽时第二个写 admission 失败、而两个只读委派成功共存。——Task 3 `TestTaskResearchEndToEndParallelReadOnlyDelegations`。
4. **跨租户/跨身份探测**：委派与批注若按裸 id 寻址会泄漏其它租户的任务活动。所有读写必须 tenant+session 双绑定，跨租户探测统一 404（与 `ResolveTaskAccess` 同口径）。——Task 2 `TestListResearchCrossTenantProbeIsUniform404` + Task 3 E2E 跨租户断言。
5. **scope 撤销后的迟到写**：移动端切租户/登出后，在途的 delegate/annotate promise 迟到返回。模块必须在每次异步提交前检查 lease，撤销后拒绝（RESEARCH_SCOPE_CHANGED）且不产生部分状态；离线草稿 flush 期间撤销同样中止。——Task 6 `late results after lease revocation are rejected...` 与 `flush aborts remaining drafts when the lease is revoked`。

---

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 0 | Go：迁移轨道前置 | 条件去重（mobile_device_app 000118/000197）+ 新迁移 `task_research_delegations`/`task_artifact_annotations`（versioned 000198 / sqlite 000119）+ 装载验证 |
| 1 | Go：类型与仓储 | `types.TaskResearchDelegation`/`TaskArtifactAnnotation` + `TaskResearchStore`/`TaskAnnotationStore`（CAS 完成委派）+ 真实迁移 store 测试 |
| 2 | Go：wire 端点 | `WorkbenchResearchHandler`（delegate/list/complete/annotate/annotations）+ 路由一行 + 容器装配 + handler stub 测试 |
| 3 | Go：E2E 证据 | 真实迁移库 + 真实 store/handler + httptest：AC1（只读并行 vs 写槽互斥对照、源越权、grants 面不放宽、跨租户 404）与 AC2（钉版批注、新版本不可变、旧版本恒定） |
| 4 | contracts | `mobile/research.ts` wire 类型 + 整体拒绝解析器 + 根导出 + 契约测试 |
| 5 | api-client | `createMobileResearchRemote`（语义行 camelCase，与 mobile-core Port 逐字一致）+ `./mobile/research` exports + 契约测试 |
| 6 | mobile-core | `src/research/` 深模块（open({lease}) 句柄、离线批注草稿、修订组合）+ 场景 Adapter + Interface 级测试 + barrel 导出 |
| 7 | apps/mobile | `research-view` 控制器 + `ResearchScreen` + `/tasks/research` 路由 + composition 工厂 + Materials 屏入口 + 路由/控制器测试 |
| 8 | apps/mobile | `research-integration-smoke`（opt-in 真实 HTTP，AC3）+ 证据契约测试 |

**Consumes（前批精确签名，执行者只看本块即可对接）：**
- Go：`session.OwnedRunReader`（`GetOwnedRun(ctx, tenantID uint64, ownerID, runID string) (agentruntime.Run, error)`）、`session.GrantedRunReader`（`GetRunForGrantedReader(ctx, tenantID uint64, readerID, runID string) (agentruntime.Run, error)`，生产实现 `*repository.AgentRunStore`）、`session.ArtifactRefReader`（`GetSessionArtifactRefs(ctx, sessionID string) ([]types.SessionArtifactRef, error)`）、`session.artifactVersionOf(ref types.SessionArtifactRef) string`、`session.workbenchCaller(c *gin.Context) (uint64, string)`、`types.Caller`/`types.TenantIDContextKey`/`types.UserIDContextKey`/`types.TenantRoleContextKey`、`types.TaskRoleCanRun(types.TaskAccessRole) bool`、`service.TaskGrantService.ResolveTaskAccess(ctx, caller types.Caller, taskID string) (types.TaskAccess, error)`、`repository.NewKnowledgeBaseRepository(db).GetKnowledgeBaseByIDAndTenant(ctx, id string, tenantID uint64) (*types.KnowledgeBase, error)`（miss = `repository.ErrKnowledgeBaseNotFound`）、`agentruntime.ErrRunActive`。
- 测试夹具（`internal/application/repository` 包内 `repository_test` 既有，直接复用）：`openTaskGrantDB(t)`（真实全量 sqlite 迁移 + tenants/users/sessions/tenant_members 种子）、`taskGrantAdmission()`（tenant 1 / run `r1` / session `s1` / owner `u1`）、`seedTaskGrantFixtures`（u1 owner、u2 viewer、u3 collaborator、u5 suspended）。
- TS：`ScopeLease`（不透明对象）、`leaseActive(lease)/leaseScopeOf(lease)`（包内）、`TaskCommandPort`（`command(input: { runId: string; action: TaskCommandAction; text?: string; expectedRevision: number; intentId?: string }): Promise<{ runId: string; action: TaskCommandAction; nextRunId?: string }>`）、`OfflineGate`（`status(): Promise<'online'|'offline'>`）、`ScopedStore`（`drafts: { put({id,body})/get(id)/list()/remove(id) }`）、`MaterialEntry`（含 `materialId/version/sourceRun`）、api-client `ClientRequest` 与 `requireDeploymentOrigin(origin)`、apps/mobile `cachePut/deploymentScopeKey`（composition.ts:152-168）。

**Produces（本计划对外产出，供后续 Issue / #71 消费）：**
- Go：`POST|GET /api/v1/workbench/executions/:run_id/research`、`POST .../research/:delegation_id/summary`、`POST|GET .../annotations`；`types.TaskResearchDelegation`/`types.TaskArtifactAnnotation`；`repository.NewTaskResearchStore`/`NewTaskAnnotationStore`；`session.NewWorkbenchResearchHandler`/`session.ResearchSourceAuthorizer`/`session.TaskAccessResolver`。
- TS：contracts 根导出 `parseResearchListResponse`/`parseAnnotationListResponse` 及 wire 类型；api-client `./mobile/research`（`createMobileResearchRemote`）；mobile-core `createTaskResearch`/`ResearchError`/场景 Adapter；apps/mobile `/tasks/research` 路由与 `activeTaskResearch()`。

---

### Task 0: Go 迁移轨道前置——条件去重与新委派/批注表

**Files:**
- Create: `migrations/versioned/000198_task_research.up.sql`
- Create: `migrations/versioned/000198_task_research.down.sql`
- Create: `migrations/sqlite/000119_task_research.up.sql`
- Create: `migrations/sqlite/000119_task_research.down.sql`
- Modify（仅当条件去重未落地时）: `migrations/sqlite/000114_mobile_device_app.up.sql` → `000118_mobile_device_app.up.sql`（含 down）
- Modify（仅当条件去重未落地时）: `migrations/versioned/000193_mobile_device_app.up.sql` → `000197_mobile_device_app.up.sql`（含 down）
- Modify（仅当条件去重未落地时）: `internal/handler/mobile_device_test.go`、`internal/application/repository/mobile_device_app_test.go`、`internal/application/repository/mobile_device_test.go`、`internal/application/repository/mobile_push_isolation_test.go`、`internal/modules/workbench/service/workbench/notification_app_policy_test.go`（文件名 pin 与注释）

**Interfaces:**
- Consumes: 两迁移轨道的下一可用号——sqlite 000119 / versioned 000198（波起点 HEAD 最大号为 000117/000196；Task 0 Step 2 条件去重把 mobile_device_app 顺延为 000118/000197 后，000119/000198 即为下一号；两种形态均已实读 `migrations/` 目录核实）。
- Produces: 表 `task_research_delegations`（PK `(tenant_id, id)`）与 `task_artifact_annotations`（PK `(tenant_id, id)`），列集与 Task 1 的 gorm 实体逐字对齐；后续任务的测试依赖全量轨道可装载。

- [ ] **Step 1: 核对迁移轨道现状（条件去重判断）**

Run:
```bash
ls migrations/sqlite/ | grep -E '^00011[3-9]' ; ls migrations/versioned/ | grep -E '^00019[3-9]'
```
Expected：sqlite 目录同时出现 `000114_public_agent_marketplace` 与 `000114_mobile_device_app`（或 `000118_mobile_device_app`），versioned 同理（`000193_public_agent_marketplace` 与 `000193_mobile_device_app` 或 `000197_mobile_device_app`）。判定规则：**同一个轨道里 mobile_device_app 与 public_agent_marketplace 的前缀号相同即为重复**——若看到 `000114_mobile_device_app` / `000193_mobile_device_app`（波起点 HEAD 形态），执行 Step 2；若看到 `000118_mobile_device_app` / `000197_mobile_device_app`（已去重形态），跳到 Step 3 并在计划执行记录中写明「去重已存在，零改动」。

- [ ] **Step 2（仅当 Step 1 见到重复号）: 执行条件去重（与并行批次已落地变更逐字一致）**

```bash
git mv migrations/sqlite/000114_mobile_device_app.up.sql migrations/sqlite/000118_mobile_device_app.up.sql
git mv migrations/sqlite/000114_mobile_device_app.down.sql migrations/sqlite/000118_mobile_device_app.down.sql
git mv migrations/versioned/000193_mobile_device_app.up.sql migrations/versioned/000197_mobile_device_app.up.sql
git mv migrations/versioned/000193_mobile_device_app.down.sql migrations/versioned/000197_mobile_device_app.down.sql
```
然后更新 pin（完整文件名 token 全量文本替换；注释中裸号逐一修正）：
```bash
rg -l '000114_mobile_device_app|000193_mobile_device_app' internal/ | xargs sed -i '' \
  -e 's/000114_mobile_device_app/000118_mobile_device_app/g' \
  -e 's/000193_mobile_device_app/000197_mobile_device_app/g'
```
裸号注释修正（rg 核对后手改，不要盲替四位数字）：`internal/handler/mobile_device_test.go:28-29`（「000059 → 000060 → 000114」→「…→ 000118」）、`internal/application/repository/mobile_device_app_test.go:22/:179`、`internal/application/repository/mobile_push_isolation_test.go:61`（该行的 000114 指 mobile_device_app，改 000118）。改完确认：
```bash
rg -n '000114_mobile_device_app|000193_mobile_device_app' internal/ migrations/ ; echo "exit=$?"
```
Expected: 无命中（exit=1）。

- [ ] **Step 3: 写新迁移（先写 down 再写 up，随后立刻验证 up/down/up）**

创建 `migrations/versioned/000198_task_research.up.sql`：

```sql
-- T17 (#47): Lead Agent read-only research delegations and version-pinned
-- material annotations. Delegation rows never touch sessions.active_agent_run_id
-- (the single write slot stays owned by AgentRunStore.Admit); annotations are
-- append-only and bind the exact artifact version identity they reviewed.
CREATE TABLE task_research_delegations (
    tenant_id INTEGER NOT NULL,
    id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    parent_run_id VARCHAR(64) NOT NULL,
    objective TEXT NOT NULL,
    sources_json TEXT NOT NULL DEFAULT '[]',
    status VARCHAR(16) NOT NULL DEFAULT 'assigned',
    summary TEXT NOT NULL DEFAULT '',
    created_by VARCHAR(512) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX idx_task_research_session ON task_research_delegations (tenant_id, session_id);

CREATE TABLE task_artifact_annotations (
    tenant_id INTEGER NOT NULL,
    id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    material_id TEXT NOT NULL,
    base_version TEXT NOT NULL,
    body TEXT NOT NULL,
    author_id VARCHAR(512) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX idx_task_annotation_session ON task_artifact_annotations (tenant_id, session_id);
CREATE INDEX idx_task_annotation_material ON task_artifact_annotations (tenant_id, session_id, material_id);
```

创建 `migrations/versioned/000198_task_research.down.sql`：

```sql
DROP TABLE IF EXISTS task_artifact_annotations;
DROP TABLE IF EXISTS task_research_delegations;
```

创建 `migrations/sqlite/000119_task_research.up.sql`：

```sql
-- T17 (#47) — sqlite track. Same shape as the versioned migration; delegation
-- rows never touch the single write slot, annotations are append-only.
CREATE TABLE task_research_delegations (
    tenant_id INTEGER NOT NULL,
    id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    parent_run_id VARCHAR(64) NOT NULL,
    objective TEXT NOT NULL,
    sources_json TEXT NOT NULL DEFAULT '[]',
    status VARCHAR(16) NOT NULL DEFAULT 'assigned',
    summary TEXT NOT NULL DEFAULT '',
    created_by VARCHAR(512) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX idx_task_research_session ON task_research_delegations (tenant_id, session_id);

CREATE TABLE task_artifact_annotations (
    tenant_id INTEGER NOT NULL,
    id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    material_id TEXT NOT NULL,
    base_version TEXT NOT NULL,
    body TEXT NOT NULL,
    author_id VARCHAR(512) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX idx_task_annotation_session ON task_artifact_annotations (tenant_id, session_id);
CREATE INDEX idx_task_annotation_material ON task_artifact_annotations (tenant_id, session_id, material_id);
```

创建 `migrations/sqlite/000119_task_research.down.sql`：

```sql
DROP INDEX IF EXISTS idx_task_annotation_material;
DROP INDEX IF EXISTS idx_task_annotation_session;
DROP TABLE IF EXISTS task_artifact_annotations;
DROP INDEX IF EXISTS idx_task_research_session;
DROP TABLE IF EXISTS task_research_delegations;
```

- [ ] **Step 4: 验证迁移轨道装载与回滚**

Run:
```bash
go test ./internal/database/ -run 'TestSQLiteMigrationsCreateVersionedSchema|TestSQLiteMigrationsUpgradeV4PreservesData|TestSQLiteMigrationsIncludeAutoTagConfig' -count=1
```
Expected: ok（三测全部通过；任何 duplicate migration 报错说明 Step 1/2 的去重没做对）。再跑 down/up 往返的既有往返测试：
```bash
go test ./internal/database/ -run 'TestWorkbenchSQLite' -count=1
```
Expected: ok。

- [ ] **Step 5: Commit**

```bash
git add migrations/versioned/000198_task_research.up.sql migrations/versioned/000198_task_research.down.sql \
        migrations/sqlite/000119_task_research.up.sql migrations/sqlite/000119_task_research.down.sql
git add migrations/ internal/   # 仅当执行了 Step 2 的条件去重
git commit -m "feat(workbench): task research delegations and artifact annotations migrations (T17 #47 task 0)"
```

---

### Task 1: Go 类型与仓储——委派/批注 store

**Files:**
- Create: `internal/types/task_research.go`
- Create: `internal/application/repository/task_research.go`
- Test: `internal/application/repository/task_research_store_test.go`

**Interfaces:**
- Consumes: Task 0 的两张表；`gorm.DB`。
- Produces（Task 2/3 依赖的精确签名）:
  - `types.TaskResearchDelegation`（字段见实现；`Sources() []string` 解码 `sources_json`）
  - `types.TaskArtifactAnnotation`
  - 常量 `types.TaskResearchAssigned = "assigned"`、`types.TaskResearchCompleted = "completed"`
  - 哨兵 `types.ErrTaskResearchNotFound`、`types.ErrTaskResearchState`、`types.ErrTaskAnnotationInvalid`
  - `repository.NewTaskResearchStore(db *gorm.DB) *TaskResearchStore`：
    - `CreateDelegation(ctx context.Context, d types.TaskResearchDelegation) error`
    - `GetDelegation(ctx context.Context, tenantID uint64, id string) (types.TaskResearchDelegation, error)`（miss = `types.ErrTaskResearchNotFound`）
    - `ListDelegationsBySession(ctx context.Context, tenantID uint64, sessionID string) ([]types.TaskResearchDelegation, error)`
    - `CompleteDelegation(ctx context.Context, tenantID uint64, id, summary string) (types.TaskResearchDelegation, error)`（CAS `assigned→completed`；非 assigned = `types.ErrTaskResearchState`）
  - `repository.NewTaskAnnotationStore(db *gorm.DB) *TaskAnnotationStore`：
    - `CreateAnnotation(ctx context.Context, a types.TaskArtifactAnnotation) error`（`material_id`/`base_version`/`body` 空或 `base_version` 超 128 rune = `types.ErrTaskAnnotationInvalid`）
    - `ListAnnotationsBySession(ctx context.Context, tenantID uint64, sessionID string) ([]types.TaskArtifactAnnotation, error)`
    - `ListAnnotationsForMaterial(ctx context.Context, tenantID uint64, sessionID, materialID string) ([]types.TaskArtifactAnnotation, error)`

- [ ] **Step 1: 写失败测试**

创建 `internal/application/repository/task_research_store_test.go`（复用同包 `repository_test` 既有 `openTaskGrantDB`——真实全量 sqlite 迁移 + 种子，见 `task_grant_store_test.go:27-46`）：

```go
package repository_test

// T17 (#47) store evidence: delegations and annotations on the real migrated
// sqlite database. The CAS on delegation completion and the append-only
// annotation insert are the two durable invariants AC1/AC2 stand on.

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func researchDelegationFixture(id string) types.TaskResearchDelegation {
	return types.TaskResearchDelegation{
		TenantID: 1, ID: id, SessionID: "s1", ParentRunID: "r1",
		Objective: "survey retrieval baselines", SourcesJSON: `["kb-1","kb-2"]`,
		Status: types.TaskResearchAssigned, CreatedBy: "u1",
	}
}

func TestTaskResearchStoreDelegationRoundTrip(t *testing.T) {
	db := openTaskGrantDB(t)
	store := repository.NewTaskResearchStore(db)
	ctx := context.Background()

	require.NoError(t, store.CreateDelegation(ctx, researchDelegationFixture("d1")))

	got, err := store.GetDelegation(ctx, 1, "d1")
	require.NoError(t, err)
	require.Equal(t, "survey retrieval baselines", got.Objective)
	require.Equal(t, []string{"kb-1", "kb-2"}, got.Sources())
	require.Equal(t, types.TaskResearchAssigned, got.Status)

	// Cross-tenant read is one uniform miss: the probe learns nothing.
	_, err = store.GetDelegation(ctx, 2, "d1")
	require.ErrorIs(t, err, types.ErrTaskResearchNotFound)

	list, err := store.ListDelegationsBySession(ctx, 1, "s1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	list, err = store.ListDelegationsBySession(ctx, 2, "s1")
	require.NoError(t, err)
	require.Empty(t, list, "跨租户列表必须为空（AC1 探测面）")
}

func TestTaskResearchStoreCompleteDelegationCASMovesAssignedOnly(t *testing.T) {
	db := openTaskGrantDB(t)
	store := repository.NewTaskResearchStore(db)
	ctx := context.Background()
	require.NoError(t, store.CreateDelegation(ctx, researchDelegationFixture("d1")))

	done, err := store.CompleteDelegation(ctx, 1, "d1", "3 findings, all cited")
	require.NoError(t, err)
	require.Equal(t, types.TaskResearchCompleted, done.Status)
	require.Equal(t, "3 findings, all cited", done.Summary)

	// Replaying the completion is a state conflict, not a second write: the
	// recorded summary is immutable once completed (durable checkpoint).
	_, err = store.CompleteDelegation(ctx, 1, "d1", "retry summary")
	require.ErrorIs(t, err, types.ErrTaskResearchState)

	got, err := store.GetDelegation(ctx, 1, "d1")
	require.NoError(t, err)
	require.Equal(t, "3 findings, all cited", got.Summary, "重放完成不得改写既有摘要")

	// Completing an unknown delegation is the same uniform miss.
	_, err = store.CompleteDelegation(ctx, 1, "d-missing", "x")
	require.ErrorIs(t, err, types.ErrTaskResearchNotFound)
}

func TestTaskAnnotationStoreAppendOnlyAndMaterialIndex(t *testing.T) {
	db := openTaskGrantDB(t)
	store := repository.NewTaskAnnotationStore(db)
	ctx := context.Background()

	base := types.TaskArtifactAnnotation{
		TenantID: 1, ID: "an1", SessionID: "s1", RunID: "r1",
		MaterialID: "m1:0", BaseVersion: "9a2f1c3d4e5f6a7b",
		Body: "结论第三段缺引用", AuthorID: "u3",
	}
	require.NoError(t, store.CreateAnnotation(ctx, base))

	list, err := store.ListAnnotationsBySession(ctx, 1, "s1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "9a2f1c3d4e5f6a7b", list[0].BaseVersion)

	byMaterial, err := store.ListAnnotationsForMaterial(ctx, 1, "s1", "m1:0")
	require.NoError(t, err)
	require.Len(t, byMaterial, 1)
	byMaterial, err = store.ListAnnotationsForMaterial(ctx, 1, "s1", "m2:0")
	require.NoError(t, err)
	require.Empty(t, byMaterial)

	_, err = store.ListAnnotationsBySession(ctx, 2, "s1")
	require.NoError(t, err)
	// 跨租户返回空集（同委派列表口径），绝不泄漏其它租户批注。
	other, err := store.ListAnnotationsBySession(ctx, 2, "s1")
	require.NoError(t, err)
	require.Empty(t, other)

	// Validation fails closed before any durable write.
	bad := base
	bad.ID = "an2"
	bad.Body = ""
	require.ErrorIs(t, store.CreateAnnotation(ctx, bad), types.ErrTaskAnnotationInvalid)
	bad.ID = "an3"
	bad.Body = "x"
	bad.BaseVersion = ""
	require.ErrorIs(t, store.CreateAnnotation(ctx, bad), types.ErrTaskAnnotationInvalid)
	bad.ID = "an4"
	bad.BaseVersion = string(make([]rune, 129))
	for i := range bad.BaseVersion {
		bad.BaseVersion = bad.BaseVersion[:i] + "a" + bad.BaseVersion[i+1:]
	}
	require.ErrorIs(t, store.CreateAnnotation(ctx, bad), types.ErrTaskAnnotationInvalid)
}
```

- [ ] **Step 2: 运行确认失败**

Run:
```bash
go test ./internal/application/repository/ -run 'TestTaskResearchStore|TestTaskAnnotationStore' -count=1
```
Expected: FAIL（`undefined: types.TaskResearchDelegation` / `undefined: repository.NewTaskResearchStore` 编译错误即失败）。

失败形态判别（重要）：本步的预期失败是**编译错误**。若实跑报的是 `duplicate migration file: 000114_public_agent_marketplace.down.sql`，说明 Task 0 Step 2 的条件去重没有生效（本测试经 `openTaskGrantDB` 装载全量 sqlite 迁移）——先回 Task 0 完成去重再回来，不要把迁移失败误读为本步的预期失败。

- [ ] **Step 3: 最小实现**

创建 `internal/types/task_research.go`：

```go
package types

// T17 (#47): Lead Agent read-only research delegations and version-pinned
// material annotations (CONTEXT.md 主理 Agent / 委派授权 / 任务产物). A
// delegation is a durable READ-ONLY assignment: it never touches
// sessions.active_agent_run_id and it never widens task_grants. An annotation
// is an append-only review record bound to the exact artifact version identity
// it reviewed; the annotated version is never rewritten.

import (
	"encoding/json"
	"errors"
	"time"
)

const (
	// TaskResearchAssigned is the only state a delegation is created in.
	TaskResearchAssigned = "assigned"
	// TaskResearchCompleted is terminal: the recorded summary is immutable.
	TaskResearchCompleted = "completed"
)

var (
	// ErrTaskResearchNotFound reports a missing or cross-tenant delegation.
	// Both are deliberately indistinguishable (probe learns nothing).
	ErrTaskResearchNotFound = errors.New("task research delegation not found")
	// ErrTaskResearchState reports a completion attempt on a delegation that
	// is not in the assigned state (already completed).
	ErrTaskResearchState = errors.New("task research delegation state conflict")
	// ErrTaskAnnotationInvalid reports an annotation that fails validation
	// before any durable write.
	ErrTaskAnnotationInvalid = errors.New("invalid task artifact annotation")
)

// TaskResearchDelegation persists one read-only research assignment made by
// the task owner's active run. Sources are the delegated knowledge subset;
// the handler proves each source is inside the task's tenant knowledge scope
// before this row is ever written.
type TaskResearchDelegation struct {
	TenantID    uint64    `json:"tenant_id" gorm:"primaryKey;column:tenant_id"`
	ID          string    `json:"id" gorm:"primaryKey;column:id;type:varchar(36)"`
	SessionID   string    `json:"session_id" gorm:"column:session_id;type:varchar(36)"`
	ParentRunID string    `json:"parent_run_id" gorm:"column:parent_run_id;type:varchar(64)"`
	Objective   string    `json:"objective" gorm:"column:objective"`
	SourcesJSON string    `json:"-" gorm:"column:sources_json"`
	Status      string    `json:"status" gorm:"column:status;type:varchar(16);not null;default:assigned"`
	Summary     string    `json:"summary" gorm:"column:summary"`
	CreatedBy   string    `json:"created_by" gorm:"column:created_by;type:varchar(512);not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// TableName binds TaskResearchDelegation to task_research_delegations.
func (TaskResearchDelegation) TableName() string { return "task_research_delegations" }

// Sources decodes the delegated knowledge subset. A malformed payload reads
// as empty — sources are server-validated at creation, so corruption here
// must degrade to "no delegated sources", never to invented grants.
func (d TaskResearchDelegation) Sources() []string {
	out := []string{}
	_ = json.Unmarshal([]byte(d.SourcesJSON), &out)
	return out
}

// TaskArtifactAnnotation is one append-only review record pinned to the
// artifact version identity (workbench artifactVersionOf) that the reviewer
// actually saw. material_id is the (message_id, index) binding; base_version
// is the version identity at annotation time.
type TaskArtifactAnnotation struct {
	TenantID    uint64    `json:"tenant_id" gorm:"primaryKey;column:tenant_id"`
	ID          string    `json:"id" gorm:"primaryKey;column:id;type:varchar(36)"`
	SessionID   string    `json:"session_id" gorm:"column:session_id;type:varchar(36)"`
	RunID       string    `json:"run_id" gorm:"column:run_id;type:varchar(64)"`
	MaterialID  string    `json:"material_id" gorm:"column:material_id"`
	BaseVersion string    `json:"base_version" gorm:"column:base_version"`
	Body        string    `json:"body" gorm:"column:body"`
	AuthorID    string    `json:"author_id" gorm:"column:author_id;type:varchar(512);not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
}

// TableName binds TaskArtifactAnnotation to task_artifact_annotations.
func (TaskArtifactAnnotation) TableName() string { return "task_artifact_annotations" }
```

创建 `internal/application/repository/task_research.go`：

```go
package repository

// T17 (#47) stores for read-only research delegations and version-pinned
// annotations. Every query binds tenant (and session where the projection is
// session-scoped) so a cross-tenant probe reads as an empty set or one
// uniform miss — never as another tenant's rows.

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

const maxAnnotationBaseVersionRunes = 128

// TaskResearchStore owns task_research_delegations.
type TaskResearchStore struct {
	db *gorm.DB
}

// NewTaskResearchStore constructs the delegation store.
func NewTaskResearchStore(db *gorm.DB) *TaskResearchStore {
	return &TaskResearchStore{db: db}
}

// CreateDelegation inserts one delegation row. The caller (handler) owns
// identity and scope validation; the store persists as given.
func (s *TaskResearchStore) CreateDelegation(ctx context.Context, d types.TaskResearchDelegation) error {
	return s.db.WithContext(ctx).Create(&d).Error
}

// GetDelegation loads one delegation; a miss and a cross-tenant id are the
// same uniform ErrTaskResearchNotFound.
func (s *TaskResearchStore) GetDelegation(ctx context.Context, tenantID uint64, id string) (types.TaskResearchDelegation, error) {
	var row types.TaskResearchDelegation
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return types.TaskResearchDelegation{}, types.ErrTaskResearchNotFound
	}
	if err != nil {
		return types.TaskResearchDelegation{}, err
	}
	return row, nil
}

// ListDelegationsBySession lists the task's delegations in creation order.
func (s *TaskResearchStore) ListDelegationsBySession(ctx context.Context, tenantID uint64, sessionID string) ([]types.TaskResearchDelegation, error) {
	var rows []types.TaskResearchDelegation
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ?", tenantID, sessionID).
		Order("created_at ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// CompleteDelegation moves assigned→completed and records the findings
// summary in one CAS. A replay after completion conflicts (the recorded
// summary is immutable); an unknown id is the uniform miss.
func (s *TaskResearchStore) CompleteDelegation(ctx context.Context, tenantID uint64, id, summary string) (types.TaskResearchDelegation, error) {
	updated := s.db.WithContext(ctx).Model(&types.TaskResearchDelegation{}).
		Where("tenant_id = ? AND id = ? AND status = ?", tenantID, id, types.TaskResearchAssigned).
		Updates(map[string]any{
			"status":     types.TaskResearchCompleted,
			"summary":    summary,
			"updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if updated.Error != nil {
		return types.TaskResearchDelegation{}, updated.Error
	}
	if updated.RowsAffected == 1 {
		return s.GetDelegation(ctx, tenantID, id)
	}
	if _, err := s.GetDelegation(ctx, tenantID, id); err != nil {
		return types.TaskResearchDelegation{}, err
	}
	return types.TaskResearchDelegation{}, types.ErrTaskResearchState
}

// TaskAnnotationStore owns task_artifact_annotations.
type TaskAnnotationStore struct {
	db *gorm.DB
}

// NewTaskAnnotationStore constructs the annotation store.
func NewTaskAnnotationStore(db *gorm.DB) *TaskAnnotationStore {
	return &TaskAnnotationStore{db: db}
}

func validateAnnotation(a types.TaskArtifactAnnotation) error {
	if strings.TrimSpace(a.MaterialID) == "" || strings.TrimSpace(a.BaseVersion) == "" || strings.TrimSpace(a.Body) == "" {
		return types.ErrTaskAnnotationInvalid
	}
	if utf8.RuneCountInString(a.BaseVersion) > maxAnnotationBaseVersionRunes {
		return types.ErrTaskAnnotationInvalid
	}
	return nil
}

// CreateAnnotation appends one annotation. Rows are never updated by this
// store: the reviewed version identity is frozen at insert.
func (s *TaskAnnotationStore) CreateAnnotation(ctx context.Context, a types.TaskArtifactAnnotation) error {
	if err := validateAnnotation(a); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Create(&a).Error
}

// ListAnnotationsBySession lists the task's annotations in creation order.
func (s *TaskAnnotationStore) ListAnnotationsBySession(ctx context.Context, tenantID uint64, sessionID string) ([]types.TaskArtifactAnnotation, error) {
	var rows []types.TaskArtifactAnnotation
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ?", tenantID, sessionID).
		Order("created_at ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListAnnotationsForMaterial lists the annotations of one material binding.
func (s *TaskAnnotationStore) ListAnnotationsForMaterial(ctx context.Context, tenantID uint64, sessionID, materialID string) ([]types.TaskArtifactAnnotation, error) {
	var rows []types.TaskArtifactAnnotation
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND material_id = ?", tenantID, sessionID, materialID).
		Order("created_at ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
```

- [ ] **Step 4: 运行确认通过**

Run:
```bash
go test ./internal/application/repository/ -run 'TestTaskResearchStore|TestTaskAnnotationStore' -count=1
```
Expected: ok（6 个测试全绿）。

- [ ] **Step 5: Commit**

```bash
git add internal/types/task_research.go internal/application/repository/task_research.go internal/application/repository/task_research_store_test.go
git commit -m "feat(workbench): task research delegation and annotation stores (T17 #47 task 1)"
```

---

### Task 2: Go wire——委派与批注端点 + 路由 + 容器

**Files:**
- Create: `internal/handler/session/workbench_research.go`
- Create: `internal/handler/session/workbench_research_test.go`
- Modify: `internal/router/routes_workbench.go`（文件末尾追加注册函数）
- Modify: `internal/router/router.go`（`RegisterWorkbenchDeliveryRoutes` 调用行之后加一行）
- Modify: `internal/container/workbench.go`（追加两个 provider）
- Modify: `internal/container/container.go`（Provide 两行）

**Interfaces:**
- Consumes: Task 1 的 store 签名；`session.OwnedRunReader`/`GrantedRunReader`/`ArtifactRefReader`/`artifactVersionOf`/`workbenchCaller`/`resolveOwnedRun`（签名见头部 Consumes 块）；`types.TaskRoleCanRun`；`service.TaskGrantService.ResolveTaskAccess`（生产实现满足 `TaskAccessResolver`）。
- Produces:
  - 端点：`POST /api/v1/workbench/executions/:run_id/research`（201）、`GET /api/v1/workbench/executions/:run_id/research`（200，owner+granted）、`POST /api/v1/workbench/executions/:run_id/research/:delegation_id/summary`（200，owner-only）、`POST /api/v1/workbench/executions/:run_id/annotations`（201，owner 或 collaborator）、`GET /api/v1/workbench/executions/:run_id/annotations`（200，owner+granted）。
  - `session.NewWorkbenchResearchHandler(runs OwnedRunReader, granted GrantedRunReader, refs ArtifactRefReader, research ResearchStore, annotations AnnotationStore, sources ResearchSourceAuthorizer, access TaskAccessResolver) *WorkbenchResearchHandler`
  - `session.ResearchSourceAuthorizer`（`AuthorizeResearchSource(ctx, tenantID uint64, kbID string) error`）
  - `session.TaskAccessResolver`（`ResolveTaskAccess(ctx, caller types.Caller, taskID string) (types.TaskAccess, error)`）
  - 错误码：`research_invalid_request`(400)/`research_source_out_of_task_grant`(400)/`unauthorized`(401)/`research_forbidden`(403)/`run_not_found`(404)/`research_not_found`(404)/`material_not_found`(404)/`research_delegation_state`(409)/`annotation_base_version_conflict`(409)。
  - 容器：`container.NewWorkbenchResearchHandler(db *gorm.DB, runs *repository.AgentRunStore, messages interfaces.MessageService, grants *service.TaskGrantService)` 与 `container.NewResearchSourceAuthorizer(db *gorm.DB) session.ResearchSourceAuthorizer`。

- [ ] **Step 1: 写失败测试**

创建 `internal/handler/session/workbench_research_test.go`：

```go
package session

// T17 (#47) handler evidence: the delegation surface is structurally
// read-only (the handler holds no AgentRunStore write path), sources are
// validated against the tenant knowledge scope BEFORE any durable write,
// annotations pin the CURRENT artifact version identity, and every read
// falls back to the granted reader exactly like the delivery read face.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ─── stubs ───────────────────────────────────────────────────────────────────

type researchStoreStub struct {
	created      []types.TaskResearchDelegation
	completed    map[string]string
	getByID      func(tenant uint64, id string) (types.TaskResearchDelegation, error)
	completeCalls int
}

func newResearchStoreStub() *researchStoreStub {
	return &researchStoreStub{completed: map[string]string{}}
}

func (s *researchStoreStub) CreateDelegation(_ context.Context, d types.TaskResearchDelegation) error {
	s.created = append(s.created, d)
	return nil
}

func (s *researchStoreStub) GetDelegation(_ context.Context, tenantID uint64, id string) (types.TaskResearchDelegation, error) {
	if s.getByID != nil {
		return s.getByID(tenantID, id)
	}
	for _, d := range s.created {
		if d.TenantID == tenantID && d.ID == id {
			return d, nil
		}
	}
	return types.TaskResearchDelegation{}, types.ErrTaskResearchNotFound
}

func (s *researchStoreStub) ListDelegationsBySession(_ context.Context, tenantID uint64, sessionID string) ([]types.TaskResearchDelegation, error) {
	var out []types.TaskResearchDelegation
	for _, d := range s.created {
		if d.TenantID == tenantID && d.SessionID == sessionID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *researchStoreStub) CompleteDelegation(_ context.Context, tenantID uint64, id, summary string) (types.TaskResearchDelegation, error) {
	s.completeCalls++
	if s.getByID != nil {
		got, err := s.getByID(tenantID, id)
		if err != nil {
			return got, err
		}
		if got.Status != types.TaskResearchAssigned {
			return got, types.ErrTaskResearchState
		}
		got.Status = types.TaskResearchCompleted
		got.Summary = summary
		return got, nil
	}
	for i, d := range s.created {
		if d.TenantID == tenantID && d.ID == id {
			if d.Status != types.TaskResearchAssigned {
				return d, types.ErrTaskResearchState
			}
			s.created[i].Status = types.TaskResearchCompleted
			s.created[i].Summary = summary
			return s.created[i], nil
		}
	}
	return types.TaskResearchDelegation{}, types.ErrTaskResearchNotFound
}

type annotationStoreStub struct {
	created []types.TaskArtifactAnnotation
}

func (s *annotationStoreStub) CreateAnnotation(_ context.Context, a types.TaskArtifactAnnotation) error {
	s.created = append(s.created, a)
	return nil
}

func (s *annotationStoreStub) ListAnnotationsBySession(_ context.Context, tenantID uint64, sessionID string) ([]types.TaskArtifactAnnotation, error) {
	var out []types.TaskArtifactAnnotation
	for _, a := range s.created {
		if a.TenantID == tenantID && a.SessionID == sessionID {
			out = append(out, a)
		}
	}
	return out, nil
}

type sourceAuthorizerStub struct{ denied map[string]bool }

func (s sourceAuthorizerStub) AuthorizeResearchSource(_ context.Context, _ uint64, kbID string) error {
	if s.denied[kbID] {
		return errors.New("source outside tenant knowledge scope")
	}
	return nil
}

type accessResolverStub struct{ role types.TaskAccessRole }

func (a accessResolverStub) ResolveTaskAccess(context.Context, types.Caller, string) (types.TaskAccess, error) {
	return types.TaskAccess{TaskID: "sess-1", OwnerID: "u1", Role: a.role}, nil
}

// ownerRunStub satisfies OwnedRunReader only (owner = u1 @ tenant 1).
func researchRunStub() *workbenchRunReaderStub {
	return &workbenchRunReaderStub{run: agentruntime.Run{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "run-1"},
		SessionID: "sess-1",
	}}
}

// grantedRunStub satisfies GrantedRunReader: u2 (viewer) and u3
// (collaborator) hold grants on run-1; everyone else misses.
type grantedRunStub struct{}

func (grantedRunStub) GetRunForGrantedReader(_ context.Context, tenantID uint64, readerID, runID string) (agentruntime.Run, error) {
	if tenantID == 1 && (readerID == "u2" || readerID == "u3") && runID == "run-1" {
		return agentruntime.Run{Key: agentruntime.RunKey{TenantID: tenantID, RunID: runID}, SessionID: "sess-1"}, nil
	}
	return agentruntime.Run{}, agentruntime.ErrNotFound
}

func researchHandler(runs OwnedRunReader, granted GrantedRunReader, role types.TaskAccessRole) (*WorkbenchResearchHandler, *researchStoreStub, *annotationStoreStub, *artifactRefReaderStub) {
	delegations := newResearchStoreStub()
	annotations := &annotationStoreStub{}
	refs := &artifactRefReaderStub{refs: researchRefs()}
	h := NewWorkbenchResearchHandler(
		runs, granted, refs,
		delegations, annotations,
		sourceAuthorizerStub{denied: map[string]bool{"kb-secret": true, "": true}},
		accessResolverStub{role: role},
	)
	return h, delegations, annotations, refs
}

func researchHandlerEnv(role types.TaskAccessRole) (*WorkbenchResearchHandler, *researchStoreStub, *annotationStoreStub, *artifactRefReaderStub) {
	return researchHandler(researchRunStub(), grantedRunStub{}, role)
}

// researchRefs pins one artifact whose version identity derives from its
// ContentHash (same derivation as the workbench artifact list).
func researchRefs() []types.SessionArtifactRef {
	return []types.SessionArtifactRef{
		{MessageID: "m1", Index: 0, Artifact: types.MessageArtifact{
			URL: "local://t/1/report.md", FileName: "report.md", FileType: ".md", FileSize: 256,
			ContentHash: "9a2f1c3d4e5f6a7b0000000000000000000000000000000000000000000000abc",
			CreatedAt:   time.Unix(1_700_000_000, 0),
		}},
	}
}

func researchContext(method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	c.Request = httptest.NewRequest(method, path, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	c.Request = c.Request.WithContext(ctx)
	if strings.Contains(path, "/run-1/") || strings.HasSuffix(path, "/run-1") {
		c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	}
	if strings.Contains(path, "delegation_id") {
		c.Params = append(c.Params, gin.Param{Key: "delegation_id", Value: "d1"})
	}
	return c, recorder
}

// ─── delegate ────────────────────────────────────────────────────────────────

func TestDelegateResearchPersistsReadOnlyDelegation(t *testing.T) {
	h, delegations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research",
		`{"objective":"survey retrieval baselines","sources":["kb-1","kb-2"]}`)
	h.DelegateResearch(c)

	require.Equal(t, http.StatusCreated, c.Writer.Status())
	require.Len(t, delegations.created, 1)
	d := delegations.created[0]
	require.Equal(t, uint64(1), d.TenantID)
	require.Equal(t, "sess-1", d.SessionID)
	require.Equal(t, "run-1", d.ParentRunID)
	require.Equal(t, types.TaskResearchAssigned, d.Status)
	require.Equal(t, "u1", d.CreatedBy)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Delegation researchDelegationView `json:"delegation"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "run-1", body.Data.Delegation.RunID)
	require.Equal(t, []string{"kb-1", "kb-2"}, body.Data.Delegation.Sources)
	require.Equal(t, "assigned", body.Data.Delegation.Status)
}

func TestDelegateResearchRejectsSourceOutsideTenantScope(t *testing.T) {
	h, delegations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	// 越权源必须在任何落库之前被拒：先断言零行，再断言响应。
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research",
		`{"objective":"x","sources":["kb-1","kb-secret"]}`)
	h.DelegateResearch(c)

	require.Equal(t, http.StatusBadRequest, c.Writer.Status())
	require.Contains(t, rec.Body.String(), "research_source_out_of_task_grant")
	require.Empty(t, delegations.created, "越权委派绝不能落库（AC1）")
}

func TestDelegateResearchRequiresObjectiveAndSources(t *testing.T) {
	h, delegations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	c, _ := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research", `{"objective":"x","sources":[]}`)
	h.DelegateResearch(c)
	require.Equal(t, http.StatusBadRequest, c.Writer.Status())

	c, _ = researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research", `{"objective":"","sources":["kb-1"]}`)
	h.DelegateResearch(c)
	require.Equal(t, http.StatusBadRequest, c.Writer.Status())
	require.Empty(t, delegations.created)
}

// ─── list / complete ─────────────────────────────────────────────────────────

func TestListResearchFallsBackToGrantedReader(t *testing.T) {
	h, delegations, _, _ := researchHandlerEnv(types.TaskAccessViewer)
	delegations.created = append(delegations.created, types.TaskResearchDelegation{
		TenantID: 1, ID: "d1", SessionID: "sess-1", ParentRunID: "run-1",
		Objective: "o", SourcesJSON: `["kb-1"]`, Status: types.TaskResearchAssigned, CreatedBy: "u1",
	})
	// owner 路径以 u2 miss（stub 只认 u1），granted 路径以 u2 命中 → 200。
	c, rec := researchContext(http.MethodGet, "/api/v1/workbench/executions/run-1/research", "")
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), types.UserIDContextKey, "u2"))
	h.ListResearch(c)

	require.Equal(t, http.StatusOK, c.Writer.Status())
	require.Contains(t, rec.Body.String(), `"delegation_id":"d1"`)

	// 无 grant 的读者：owner（u9≠u1）与 granted（u9 不持 grant）都 miss → 统一 404。
	h2, _, _, _ := researchHandlerEnv(types.TaskAccessViewer)
	c2, _ := researchContext(http.MethodGet, "/api/v1/workbench/executions/run-1/research", "")
	c2.Request = c2.Request.WithContext(context.WithValue(c2.Request.Context(), types.UserIDContextKey, "u9"))
	h2.ListResearch(c2)
	require.Equal(t, http.StatusNotFound, c2.Writer.Status())
}

func TestCompleteResearchCASConflictsOnCompleted(t *testing.T) {
	h, delegations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	delegations.created = append(delegations.created, types.TaskResearchDelegation{
		TenantID: 1, ID: "d1", SessionID: "sess-1", ParentRunID: "run-1",
		Objective: "o", SourcesJSON: `["kb-1"]`, Status: types.TaskResearchCompleted, Summary: "done once", CreatedBy: "u1",
	})
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research/delegation_id/summary", `{"summary":"replay"}`)
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}, {Key: "delegation_id", Value: "d1"}}
	h.CompleteResearch(c)

	require.Equal(t, http.StatusConflict, c.Writer.Status())
	require.Contains(t, rec.Body.String(), "research_delegation_state")
}

func TestCompleteResearchCrossTaskDelegationIsUniform404(t *testing.T) {
	h, delegations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	// d1 属于另一 session：即使 id 命中，session 绑定不匹配也必须 404。
	delegations.created = append(delegations.created, types.TaskResearchDelegation{
		TenantID: 1, ID: "d1", SessionID: "sess-other", ParentRunID: "run-9",
		Objective: "o", SourcesJSON: `["kb-1"]`, Status: types.TaskResearchAssigned, CreatedBy: "u1",
	})
	c, _ := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research/delegation_id/summary", `{"summary":"s"}`)
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}, {Key: "delegation_id", Value: "d1"}}
	h.CompleteResearch(c)
	require.Equal(t, http.StatusNotFound, c.Writer.Status())
}

// ─── annotate ────────────────────────────────────────────────────────────────

func TestAnnotateMaterialPinsCurrentArtifactVersion(t *testing.T) {
	h, _, annotations, _ := researchHandlerEnv(types.TaskAccessCollaborator)
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
		`{"material_id":"m1:0","base_version":"9a2f1c3d4e5f6a7b","body":"结论第三段缺引用"}`)
	h.AnnotateMaterial(c)

	require.Equal(t, http.StatusCreated, c.Writer.Status(), rec.Body.String())
	require.Len(t, annotations.created, 1)
	a := annotations.created[0]
	require.Equal(t, "m1:0", a.MaterialID)
	require.Equal(t, "9a2f1c3d4e5f6a7b", a.BaseVersion)
	require.Equal(t, "sess-1", a.SessionID)
	require.Equal(t, "u1", a.AuthorID)
	require.Contains(t, rec.Body.String(), `"annotation_id"`)
}

func TestAnnotateMaterialRejectsCollaboratorRoleForViewer(t *testing.T) {
	// viewer 只有只读角色：批注是评论性写入，TaskRoleCanRun(viewer)=false → 403。
	h, annotations, _, _ := researchHandlerEnv(types.TaskAccessViewer)
	c, _ := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
		`{"material_id":"m1:0","base_version":"9a2f1c3d4e5f6a7b","body":"x"}`)
	h.AnnotateMaterial(c)
	require.Equal(t, http.StatusForbidden, c.Writer.Status())
	require.Empty(t, annotations.created)
}

func TestAnnotateMaterialRejectsStaleBaseVersion(t *testing.T) {
	h, annotations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	// base_version 与当前版本身份不一致 → 409，防审计链断裂（Review Focus 2）。
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
		`{"material_id":"m1:0","base_version":"stale-version-0001","body":"x"}`)
	h.AnnotateMaterial(c)
	require.Equal(t, http.StatusConflict, c.Writer.Status())
	require.Contains(t, rec.Body.String(), "annotation_base_version_conflict")
	require.Empty(t, annotations.created)
}

func TestAnnotateMaterialUnknownMaterialIsUniform404(t *testing.T) {
	h, annotations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	c, _ := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
		`{"material_id":"m9:9","base_version":"9a2f1c3d4e5f6a7b","body":"x"}`)
	h.AnnotateMaterial(c)
	require.Equal(t, http.StatusNotFound, c.Writer.Status())
	require.Empty(t, annotations.created)
}

func TestListAnnotationsReadableByGrantedViewer(t *testing.T) {
	h, _, annotations, _ := researchHandlerEnv(types.TaskAccessViewer)
	annotations.created = append(annotations.created, types.TaskArtifactAnnotation{
		TenantID: 1, ID: "an1", SessionID: "sess-1", RunID: "run-1",
		MaterialID: "m1:0", BaseVersion: "9a2f1c3d4e5f6a7b", Body: "b", AuthorID: "u3",
	})
	c, rec := researchContext(http.MethodGet, "/api/v1/workbench/executions/run-1/annotations", "")
	h.ListAnnotations(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	require.Contains(t, rec.Body.String(), `"base_version":"9a2f1c3d4e5f6a7b"`)
}
```

- [ ] **Step 2: 运行确认失败**

Run:
```bash
go test ./internal/handler/session/ -run 'TestDelegateResearch|TestListResearch|TestCompleteResearch|TestAnnotateMaterial|TestListAnnotations' -count=1
```
Expected: FAIL（`undefined: NewWorkbenchResearchHandler` 编译错误）。

- [ ] **Step 3: 最小实现**

创建 `internal/handler/session/workbench_research.go`：

```go
package session

// T17 (#47): the Lead Agent's read-only research delegation surface and the
// version-pinned material annotation surface. Delegations are structurally
// read-only: this handler holds no run-admission write path, so a delegation
// can never take (or release) the session's single write slot. Annotations
// are append-only and pin the artifact's CURRENT version identity at write
// time; a stale base version conflicts instead of silently re-labeling a
// newer version (spec: 已审批版本保持不变).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ResearchStore is the delegation persistence seam (production:
// *repository.TaskResearchStore).
type ResearchStore interface {
	CreateDelegation(ctx context.Context, d types.TaskResearchDelegation) error
	GetDelegation(ctx context.Context, tenantID uint64, id string) (types.TaskResearchDelegation, error)
	ListDelegationsBySession(ctx context.Context, tenantID uint64, sessionID string) ([]types.TaskResearchDelegation, error)
	CompleteDelegation(ctx context.Context, tenantID uint64, id, summary string) (types.TaskResearchDelegation, error)
}

// AnnotationStore is the annotation persistence seam (production:
// *repository.TaskAnnotationStore).
type AnnotationStore interface {
	CreateAnnotation(ctx context.Context, a types.TaskArtifactAnnotation) error
	ListAnnotationsBySession(ctx context.Context, tenantID uint64, sessionID string) ([]types.TaskArtifactAnnotation, error)
}

// ResearchSourceAuthorizer decides whether one knowledge base may be
// delegated as a read-only research source. Production wires the tenant-bound
// knowledge base lookup: a source the task's tenant does not own is outside
// the Task Grant and must be rejected BEFORE the delegation row is written
// (spec: Delegated Agents receive only a minimum subset).
type ResearchSourceAuthorizer interface {
	AuthorizeResearchSource(ctx context.Context, tenantID uint64, knowledgeBaseID string) error
}

// TaskAccessResolver resolves the caller's per-task role for the annotation
// write gate (production: *service.TaskGrantService).
type TaskAccessResolver interface {
	ResolveTaskAccess(ctx context.Context, caller types.Caller, taskID string) (types.TaskAccess, error)
}

const (
	maxResearchSources       = 8
	maxResearchObjectiveRunes = 2000
)

// WorkbenchResearchHandler owns the research/annotation endpoints.
type WorkbenchResearchHandler struct {
	runs        OwnedRunReader
	granted     GrantedRunReader
	refs        ArtifactRefReader
	research    ResearchStore
	annotations AnnotationStore
	sources     ResearchSourceAuthorizer
	access      TaskAccessResolver
}

// NewWorkbenchResearchHandler assembles the handler. runs and granted may be
// the same store (the container passes *repository.AgentRunStore twice, same
// as the read/delivery handlers).
func NewWorkbenchResearchHandler(
	runs OwnedRunReader, granted GrantedRunReader, refs ArtifactRefReader,
	research ResearchStore, annotations AnnotationStore,
	sources ResearchSourceAuthorizer, access TaskAccessResolver,
) *WorkbenchResearchHandler {
	return &WorkbenchResearchHandler{
		runs: runs, granted: granted, refs: refs,
		research: research, annotations: annotations,
		sources: sources, access: access,
	}
}

// caller resolves the authenticated tenant and actor (same lockstep as the
// delivery handler: request context first, then the gin auth keys).
func (h *WorkbenchResearchHandler) caller(c *gin.Context) (uint64, string, bool) {
	tenantID, userID := workbenchCaller(c)
	if tenantID == 0 || userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "code": "unauthorized", "error": "tenant and user identity required"})
		return 0, "", false
	}
	return tenantID, userID, true
}

// resolveReadable resolves the run owner-first, then through the task-grant
// fallback (#42 read face). Writes re-gate on the resolved task role.
func (h *WorkbenchResearchHandler) resolveReadable(c *gin.Context) (agentruntime.Run, bool) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return agentruntime.Run{}, false
	}
	run, err := h.runs.GetOwnedRun(c.Request.Context(), tenantID, userID, c.Param("run_id"))
	if err == nil {
		return run, true
	}
	if h.granted != nil {
		if granted, gerr := h.granted.GetRunForGrantedReader(c.Request.Context(), tenantID, userID, c.Param("run_id")); gerr == nil {
			return granted, true
		}
	}
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "code": "run_not_found", "error": "execution not found"})
	return agentruntime.Run{}, false
}

type researchDelegationView struct {
	DelegationID string   `json:"delegation_id"`
	RunID        string   `json:"run_id"`
	SessionID    string   `json:"session_id"`
	Objective    string   `json:"objective"`
	Sources      []string `json:"sources"`
	Status       string   `json:"status"`
	Summary      string   `json:"summary,omitempty"`
	CreatedAt    string   `json:"created_at"`
}

func delegationViewOf(d types.TaskResearchDelegation) researchDelegationView {
	return researchDelegationView{
		DelegationID: d.ID,
		RunID:        d.ParentRunID,
		SessionID:    d.SessionID,
		Objective:    d.Objective,
		Sources:      d.Sources(),
		Status:       d.Status,
		Summary:      d.Summary,
		CreatedAt:    d.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type researchAnnotationView struct {
	AnnotationID string `json:"annotation_id"`
	RunID        string `json:"run_id"`
	MaterialID   string `json:"material_id"`
	BaseVersion  string `json:"base_version"`
	Body         string `json:"body"`
	AuthorID     string `json:"author_id"`
	CreatedAt    string `json:"created_at"`
}

func annotationViewOf(a types.TaskArtifactAnnotation) researchAnnotationView {
	return researchAnnotationView{
		AnnotationID: a.ID,
		RunID:        a.RunID,
		MaterialID:   a.MaterialID,
		BaseVersion:  a.BaseVersion,
		Body:         a.Body,
		AuthorID:     a.AuthorID,
		CreatedAt:    a.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type researchDelegateInput struct {
	Objective string   `json:"objective"`
	Sources   []string `json:"sources"`
	AgentID   string   `json:"agent_id"`
}

// DelegateResearch POST /workbench/executions/:run_id/research — owner-only.
// Sources are authorized before any durable write; the persisted row is a
// read-only assignment and never touches the single write slot.
func (h *WorkbenchResearchHandler) DelegateResearch(c *gin.Context) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return
	}
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	var input researchDelegateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_invalid_request", "error": "objective and sources are required"})
		return
	}
	objective := strings.TrimSpace(input.Objective)
	if objective == "" || len([]rune(objective)) > maxResearchObjectiveRunes || len(input.Sources) == 0 || len(input.Sources) > maxResearchSources {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_invalid_request", "error": "objective (1..2000 runes) and 1..8 sources are required"})
		return
	}
	cleaned := make([]string, 0, len(input.Sources))
	for _, source := range input.Sources {
		source = strings.TrimSpace(source)
		if source == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_invalid_request", "error": "sources must not contain empty ids"})
			return
		}
		cleaned = append(cleaned, source)
	}
	for _, source := range cleaned {
		if err := h.sources.AuthorizeResearchSource(c.Request.Context(), tenantID, source); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_source_out_of_task_grant", "error": "research source is outside the task's tenant knowledge scope"})
			return
		}
	}
	encoded, _ := json.Marshal(cleaned)
	delegation := types.TaskResearchDelegation{
		TenantID: tenantID, ID: uuid.NewString(), SessionID: run.SessionID, ParentRunID: run.Key.RunID,
		Objective: objective, SourcesJSON: string(encoded),
		Status: types.TaskResearchAssigned, CreatedBy: userID,
	}
	if err := h.research.CreateDelegation(c.Request.Context(), delegation); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "research_backend", "error": "failed to persist delegation"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"delegation": delegationViewOf(delegation)}})
}

// ListResearch GET /workbench/executions/:run_id/research — owner + granted.
func (h *WorkbenchResearchHandler) ListResearch(c *gin.Context) {
	run, ok := h.resolveReadable(c)
	if !ok {
		return
	}
	rows, err := h.research.ListDelegationsBySession(c.Request.Context(), run.Key.TenantID, run.SessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "research_backend", "error": "failed to list delegations"})
		return
	}
	items := make([]researchDelegationView, 0, len(rows))
	for _, row := range rows {
		items = append(items, delegationViewOf(row))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": items}})
}

type researchSummaryInput struct {
	Summary string `json:"summary"`
}

// CompleteResearch POST /workbench/executions/:run_id/research/:delegation_id/summary
// — owner-only. The delegation must belong to this run's session; completion
// is one CAS and replays conflict (the recorded summary is immutable).
func (h *WorkbenchResearchHandler) CompleteResearch(c *gin.Context) {
	if _, _, ok := h.caller(c); !ok {
		return
	}
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	var input researchSummaryInput
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Summary) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_invalid_request", "error": "summary is required"})
		return
	}
	delegation, err := h.research.GetDelegation(c.Request.Context(), run.Key.TenantID, c.Param("delegation_id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "code": "research_not_found", "error": "delegation not found"})
		return
	}
	if delegation.SessionID != run.SessionID {
		// Another task's delegation: same uniform miss, no existence leak.
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "code": "research_not_found", "error": "delegation not found"})
		return
	}
	done, err := h.research.CompleteDelegation(c.Request.Context(), run.Key.TenantID, delegation.ID, strings.TrimSpace(input.Summary))
	if err != nil {
		writeResearchError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"delegation": delegationViewOf(done)}})
}

type annotateInput struct {
	MaterialID  string `json:"material_id"`
	BaseVersion string `json:"base_version"`
	Body        string `json:"body"`
}

// AnnotateMaterial POST /workbench/executions/:run_id/annotations — owner or
// collaborator (TaskRoleCanRun). The base_version must equal the material's
// CURRENT version identity; a stale base conflicts (409) so a review record
// can never be silently re-attached to a newer version.
func (h *WorkbenchResearchHandler) AnnotateMaterial(c *gin.Context) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return
	}
	run, ok := h.resolveReadable(c)
	if !ok {
		return
	}
	access, err := h.access.ResolveTaskAccess(c.Request.Context(), types.Caller{TenantID: tenantID, UserID: userID}, run.SessionID)
	if err != nil {
		writeResearchError(c, err)
		return
	}
	if !types.TaskRoleCanRun(access.Role) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "research_forbidden", "error": "annotations require the task owner or a collaborator"})
		return
	}
	var input annotateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_invalid_request", "error": "material_id, base_version and body are required"})
		return
	}
	input.MaterialID = strings.TrimSpace(input.MaterialID)
	refs, err := h.refs.GetSessionArtifactRefs(c.Request.Context(), run.SessionID)
	if err != nil {
		writeWorkbenchError(c, err)
		return
	}
	var matched *types.SessionArtifactRef
	for i := range refs {
		if refs[i].MessageID+":"+strconv.Itoa(refs[i].Index) == input.MaterialID {
			matched = &refs[i]
			break
		}
	}
	if matched == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "code": "material_not_found", "error": "material not found in this execution"})
		return
	}
	if artifactVersionOf(*matched) != input.BaseVersion {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"success": false, "code": "annotation_base_version_conflict", "error": "base_version does not match the current artifact version; reload the material list"})
		return
	}
	annotation := types.TaskArtifactAnnotation{
		TenantID: tenantID, ID: uuid.NewString(), SessionID: run.SessionID, RunID: run.Key.RunID,
		MaterialID: input.MaterialID, BaseVersion: input.BaseVersion,
		Body: strings.TrimSpace(input.Body), AuthorID: userID,
	}
	if strings.TrimSpace(input.Body) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_invalid_request", "error": "body is required"})
		return
	}
	if err := h.annotations.CreateAnnotation(c.Request.Context(), annotation); err != nil {
		writeResearchError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"annotation": annotationViewOf(annotation)}})
}

// ListAnnotations GET /workbench/executions/:run_id/annotations — owner +
// granted read (annotations are review records, readable by viewers).
func (h *WorkbenchResearchHandler) ListAnnotations(c *gin.Context) {
	run, ok := h.resolveReadable(c)
	if !ok {
		return
	}
	rows, err := h.annotations.ListAnnotationsBySession(c.Request.Context(), run.Key.TenantID, run.SessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "research_backend", "error": "failed to list annotations"})
		return
	}
	items := make([]researchAnnotationView, 0, len(rows))
	for _, row := range rows {
		items = append(items, annotationViewOf(row))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": items}})
}

// writeResearchError maps store/service failures onto a fixed code table; no
// upstream text crosses the wire.
func writeResearchError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, types.ErrTaskResearchNotFound):
		c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "research_not_found", "error": "delegation not found"})
	case errors.Is(err, types.ErrTaskResearchState):
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "research_delegation_state", "error": "delegation is not in the assigned state"})
	case errors.Is(err, types.ErrTaskAnnotationInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_invalid_request", "error": "annotation payload is invalid"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "research_backend", "error": "research surface temporarily unavailable"})
	}
}
```

修改 `internal/router/routes_workbench.go`——在文件末尾追加：

```go
// RegisterWorkbenchResearchRoutes exposes the T17 (#47) read-only research
// delegation surface and the version-pinned annotation surface. Delegation
// writes are owner-only; the annotation write re-gates on the resolved task
// role (owner or collaborator, TaskRoleCanRun); both reads reuse the strict
// owner + task-grant fallback predicate. Same Viewer/API-key boundary as the
// other workbench lanes.
func RegisterWorkbenchResearchRoutes(r *gin.RouterGroup, h *session.WorkbenchResearchHandler, g *rbacGuards) {
	if h == nil || g == nil {
		return
	}
	reads := g.apiKeyGroup(r.Group("/workbench/executions", g.Viewer(), workbenchReadGate(g.cfg)), apiKeyChat(apiKeyFullAccess()))
	reads.GET("/:run_id/research", h.ListResearch)
	reads.GET("/:run_id/annotations", h.ListAnnotations)
	writes := g.apiKeyGroup(r.Group("/workbench/executions", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	writes.POST("/:run_id/research", h.DelegateResearch)
	writes.POST("/:run_id/research/:delegation_id/summary", h.CompleteResearch)
	writes.POST("/:run_id/annotations", h.AnnotateMaterial)
}
```

修改 `internal/router/router.go`——在 `RegisterWorkbenchDeliveryRoutes(v1, params.WorkbenchDeliveryHandler, rbacGuards)`（router.go:388）之后追加一行：

```go
		RegisterWorkbenchResearchRoutes(v1, params.WorkbenchResearchHandler, rbacGuards)
```

修改 `internal/container/workbench.go`——文件末尾追加：

```go
// researchSourceAuthorizer gates delegated sources against the task's tenant
// knowledge scope. Production binds the tenant-scoped knowledge base lookup;
// KB-level ACLs (shares/groups) stay enforced at retrieval time by the
// existing access seam — a delegation never widens what a later read allows.
type researchSourceAuthorizer struct {
	kb interfaces.KnowledgeBaseRepository
}

// AuthorizeResearchSource rejects sources the task's tenant does not own.
func (a researchSourceAuthorizer) AuthorizeResearchSource(ctx context.Context, tenantID uint64, knowledgeBaseID string) error {
	if strings.TrimSpace(knowledgeBaseID) == "" {
		return errors.New("empty research source")
	}
	if _, err := a.kb.GetKnowledgeBaseByIDAndTenant(ctx, knowledgeBaseID, tenantID); err != nil {
		return fmt.Errorf("research source %q is outside the task's tenant knowledge scope", knowledgeBaseID)
	}
	return nil
}

// NewResearchSourceAuthorizer wires the production source gate.
func NewResearchSourceAuthorizer(db *gorm.DB) session.ResearchSourceAuthorizer {
	return researchSourceAuthorizer{kb: repository.NewKnowledgeBaseRepository(db)}
}

// NewWorkbenchResearchHandler wires the T17 (#47) delegation/annotation
// surface: the same durable run store doubles as the granted reader (owner
// first, #42 grant fallback second), annotations pin the current version
// identity derived from message-bound artifacts.
func NewWorkbenchResearchHandler(
	db *gorm.DB,
	runs *repository.AgentRunStore,
	messages interfaces.MessageService,
	grants *service.TaskGrantService,
) (*session.WorkbenchResearchHandler, session.ResearchSourceAuthorizer) {
	return session.NewWorkbenchResearchHandler(
		runs, runs, messages,
		repository.NewTaskResearchStore(db),
		repository.NewTaskAnnotationStore(db),
		NewResearchSourceAuthorizer(db),
		grants,
	), NewResearchSourceAuthorizer(db)
}
```
并在该文件 import 块补 `"context"`、`"errors"`、`"fmt"`、`"strings"`（若缺）。

修改 `internal/container/container.go`——在 `container.Provide(NewWorkbenchDeliveryHandler)`（container.go:277）之后追加：

```go
	must(container.Provide(NewResearchSourceAuthorizer))
	must(container.Provide(NewWorkbenchResearchHandler))
```

（gin 通配符一致性：`writes` 树新增 `:delegation_id` 仅出现在 `/research/:delegation_id/summary` 子路径，`:run_id` 名称与既有各树一致，编译期由 gin 路由注册测试兜底——Task 3 E2E 的真实 engine 注册即验证。）

- [ ] **Step 4: 运行确认通过 + 全包回归**

Run（三条独立串行执行，任何一条失败都中止后续——不用 `||` 回退，避免掩蔽失败）:
```bash
go test ./internal/handler/session/ -run 'TestDelegateResearch|TestListResearch|TestCompleteResearch|TestAnnotateMaterial|TestListAnnotations' -count=1
```
Expected: ok。
```bash
go build ./...
```
Expected: 无输出（编译通过；container 双返回值 provider 由 dig 编译期接受）。
```bash
go vet ./internal/router/ ./internal/container/
```
Expected: 无输出（vet 通过）。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/session/workbench_research.go internal/handler/session/workbench_research_test.go \
        internal/router/routes_workbench.go internal/router/router.go \
        internal/container/workbench.go internal/container/container.go
git commit -m "feat(workbench): read-only research delegation and version-pinned annotation endpoints (T17 #47 task 2)"
```

---

### Task 3: Go 端到端证据——AC1/AC2 服务端全链

**Files:**
- Create: `internal/application/repository/task_research_http_test.go`

**Interfaces:**
- Consumes: Task 1/2 全部产出；`repository_test` 包内既有夹具 `openTaskGrantDB(t)`/`taskGrantAdmission()`/`seedTaskGrantFixtures`；`repository.NewMessageRepository(db)`（满足 `ArtifactRefReader`，`internal/application/repository/message.go:452`）、`repository.NewKnowledgeBaseRepository(db)`、`service.NewTaskGrantService`、`session.NewWorkbenchArtifactHandler`（AC2 的版本列表面）。
- Produces: 四条 E2E 测试（AC1×2 + AC2×2），plan 级 testCommand 的服务端证据锚点。

- [ ] **Step 1: 写 E2E 测试（TDD：本任务无新实现，测试即交付；先写测试运行确认其覆盖真实装配）**

创建 `internal/application/repository/task_research_http_test.go`：

```go
package repository_test

// T17 (#47) end-to-end evidence over a fully migrated sqlite database and
// real stores/handlers — no mocked service. AC1: read-only delegations run
// in PARALLEL while the single write slot stays exclusive, out-of-scope
// sources are rejected before any durable write, and the grants surface is
// not widened by delegation. AC2: annotations pin the current artifact
// version identity; a revision (a NEW artifact message) yields a NEW version
// while the previously annotated version stays byte-identical.

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
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type researchE2E struct {
	db     *gorm.DB
	engine *gin.Engine
}

func newResearchE2E(t *testing.T) *researchE2E {
	t.Helper()
	db := openTaskGrantDB(t)
	runs := repository.NewAgentRunStore(db)
	_, err := runs.Admit(context.Background(), taskGrantAdmission())
	require.NoError(t, err)

	// Tenant-owned knowledge base: the delegation source gate's authority.
	require.NoError(t, db.Create(&types.KnowledgeBase{ID: "kb-1", TenantID: 1, Name: "research-kb"}).Error)

	// Real grant rows: the granted-read fallback (annotate by collaborator u3,
	// delegation list by viewer u2) must resolve through the live SQL JOIN.
	grantsStore := repository.NewTaskGrantStore(db)
	_, err = grantsStore.UpsertGrant(context.Background(), 1, "s1", "u2", types.TaskGrantRoleViewer, "u1")
	require.NoError(t, err)
	_, err = grantsStore.UpsertGrant(context.Background(), 1, "s1", "u3", types.TaskGrantRoleCollaborator, "u1")
	require.NoError(t, err)

	grantsSvc := service.NewTaskGrantService(
		repository.NewTaskGrantStore(db),
		repository.NewSessionRepository(db),
		repository.NewTenantMemberRepository(db),
	)
	messages := repository.NewMessageRepository(db)
	researchHandler := session.NewWorkbenchResearchHandler(
		runs, runs, messages,
		repository.NewTaskResearchStore(db),
		repository.NewTaskAnnotationStore(db),
		researchGateAdapter{kb: repository.NewKnowledgeBaseRepository(db)},
		grantsSvc,
	)
	grantsHandler := session.NewWorkbenchTaskGrantsHandler(grantsSvc)
	readHandler := session.NewWorkbenchReadHandler(runs, repository.NewAgentRunSnapshotRepository(db)).WithGrantedRuns(runs)
	artifactHandler := session.NewWorkbenchArtifactHandler(runs, messages)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.POST("/workbench/executions/:run_id/research", researchHandler.DelegateResearch)
	v1.GET("/workbench/executions/:run_id/research", researchHandler.ListResearch)
	v1.POST("/workbench/executions/:run_id/research/:delegation_id/summary", researchHandler.CompleteResearch)
	v1.POST("/workbench/executions/:run_id/annotations", researchHandler.AnnotateMaterial)
	v1.GET("/workbench/executions/:run_id/annotations", researchHandler.ListAnnotations)
	v1.POST("/workbench/tasks/:task_id/grants", grantsHandler.Grant)
	v1.GET("/workbench/executions", session.NewWorkbenchListHandler(repository.NewWorkbenchListStore(db)).ListWorkbenchExecutions)
	v1.GET("/workbench/executions/:run_id/artifacts", artifactHandler.ListWorkbenchArtifacts)
	return &researchE2E{db: db, engine: r}
}

// researchGateAdapter mirrors the production container gate
// (container.researchSourceAuthorizer) against the real tenant-bound KB
// repository: a source the task's tenant does not own is rejected before any
// durable write (the production wiring is byte-equivalent in behavior).
type researchGateAdapter struct {
	kb interfaces.KnowledgeBaseRepository
}

func (a researchGateAdapter) AuthorizeResearchSource(ctx context.Context, tenantID uint64, kbID string) error {
	if _, err := a.kb.GetKnowledgeBaseByIDAndTenant(ctx, kbID, tenantID); err != nil {
		return err
	}
	return nil
}

func (e *researchE2E) do(t *testing.T, method, path, body, userID string, tenant uint64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, tenant)
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleContributor)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

// seedArtifactMessage inserts one assistant message carrying one artifact so
// the artifact list projects a version identity derived from its digest.
func (e *researchE2E) seedArtifactMessage(t *testing.T, messageID, digest, fileName string) {
	t.Helper()
	// SkipHooks: types.Message BeforeCreate unconditionally regenerates the ID,
	// which would break the (message_id, index) material addressing under test.
	require.NoError(t, e.db.Session(&gorm.Session{SkipHooks: true}).Create(&types.Message{
		ID: messageID, SessionID: "s1", Role: "assistant",
		Content: "revision output", IsCompleted: true,
		Artifacts: types.MessageArtifacts{{
			URL: "local://tenant/1/" + fileName, FileName: fileName, FileType: ".md",
			FileSize: 512, ContentHash: digest, CreatedAt: time.Now().UTC(),
		}},
	}).Error)
}

const researchDigestV1 = "1111111111111111111111111111111111111111111111111111111111111111"
const researchDigestV2 = "2222222222222222222222222222222222222222222222222222222222222222"

// AC1: parallel read-only delegations coexist with the exclusive write slot.
func TestTaskResearchEndToEndParallelReadOnlyDelegations(t *testing.T) {
	env := newResearchE2E(t)

	// Contrast (existing semantics, unchanged): a SECOND write admission on
	// the same session must fail on the single-write slot ...
	runs := repository.NewAgentRunStore(env.db)
	second := taskGrantAdmission()
	second.Key = agentruntime.RunKey{TenantID: 1, RunID: "r2"}
	second.RequestID = "q2"
	_, err := runs.Admit(context.Background(), second)
	require.ErrorIs(t, err, agentruntime.ErrRunActive, "同一 Task 的第二个写 Run 必须被单写槽拒绝（既有语义，本计划不放松）")

	// ... while TWO read-only research delegations are admitted in parallel
	// on the still-active write run: they never touch the slot.
	for i, objective := range []string{"survey A", "survey B"} {
		w := env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/research",
			`{"objective":"`+objective+`","sources":["kb-1"]}`, "u1", 1)
		require.Equal(t, http.StatusCreated, w.Code, "只读委派 #%d 必须与活跃写 Run 并行（AC1）: %s", i+1, w.Body.String())
	}
	w := env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/research", "", "u2", 1)
	require.Equal(t, http.StatusOK, w.Code, "granted viewer 读取委派列表")
	var list struct {
		Success bool `json:"success"`
		Data    struct {
			Items []struct {
				DelegationID string   `json:"delegation_id"`
				Sources      []string `json:"sources"`
				Status       string   `json:"status"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Data.Items, 2, "两个只读委派共存")

	// Findings summary lands via the owner CAS; a replay conflicts.
	d1 := list.Data.Items[0].DelegationID
	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/research/"+d1+"/summary",
		`{"summary":"two cited findings"}`, "u1", 1)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/research/"+d1+"/summary",
		`{"summary":"replay"}`, "u1", 1)
	require.Equal(t, http.StatusConflict, w.Code, "完成的委派重放必须冲突（摘要不可变）")

	// The collaborator cannot write grants: delegation did NOT widen the
	// grants surface (owner-only stays owner-only).
	w = env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"u4","role":"viewer"}`, "u3", 1)
	require.Equal(t, http.StatusForbidden, w.Code, "协作者委派研究绝不等于可管理 task_grants（AC1）")
}

// AC1: out-of-scope sources are rejected before any durable write.
func TestTaskResearchEndToEndSourceOutsideTenantScope(t *testing.T) {
	env := newResearchE2E(t)

	w := env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/research",
		`{"objective":"probe","sources":["kb-unknown"]}`, "u1", 1)
	require.Equal(t, http.StatusBadRequest, w.Code, "租户外知识库不得成为委派源")
	require.Contains(t, w.Body.String(), "research_source_out_of_task_grant")

	// Cross-tenant id that happens to exist in tenant 2 — same rejection.
	require.NoError(t, env.db.Create(&types.KnowledgeBase{ID: "kb-t2", TenantID: 2, Name: "other-tenant-kb"}).Error)
	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/research",
		`{"objective":"probe","sources":["kb-t2"]}`, "u1", 1)
	require.Equal(t, http.StatusBadRequest, w.Code, "委派源不得跨租户（AC1：不能借委派扩大 Task Grant）")

	// The uniform cross-tenant probe: tenant 2 sees no delegations of s1.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/research", "", "u1", 2)
	require.Equal(t, http.StatusNotFound, w.Code, "跨租户探测统一 404")
}

// AC2: annotations pin the current version; a revision produces a NEW
// version while the previously annotated version stays identical.
func TestTaskResearchEndToEndAnnotationVersionImmutability(t *testing.T) {
	env := newResearchE2E(t)
	env.seedArtifactMessage(t, "m1", researchDigestV1, "report-v1.md")

	// Current version identity of m1:0 (first 16 hex of the digest).
	w := env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/artifacts", "", "u1", 1)
	require.Equal(t, http.StatusOK, w.Code)
	var artifacts struct {
		Data struct {
			Items []struct {
				ID      string `json:"id"`
				Version string `json:"version"`
				Digest  string `json:"digest"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &artifacts))
	require.Len(t, artifacts.Data.Items, 1)
	v1 := artifacts.Data.Items[0].Version
	require.Equal(t, researchDigestV1[:16], v1)
	require.Equal(t, "m1:0", artifacts.Data.Items[0].ID)

	// Collaborator annotates v1 (owner or collaborator may comment).
	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/annotations",
		`{"material_id":"m1:0","base_version":"`+v1+`","body":"结论第三段缺引用"}`, "u3", 1)
	require.Equal(t, http.StatusCreated, w.Code, "协作者批注 v1: %s", w.Body.String())

	// A stale base version conflicts: after v2 exists, annotating m1:0 with
	// a stale identity can never re-label the newer version.
	env.seedArtifactMessage(t, "m2", researchDigestV2, "report-v2.md")
	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/annotations",
		`{"material_id":"m1:0","base_version":"deadbeefdeadbeef","body":"stale"}`, "u1", 1)
	require.Equal(t, http.StatusConflict, w.Code, "过期 base_version 必须 409（AC2）")
	require.Contains(t, w.Body.String(), "annotation_base_version_conflict")

	// The revision is a NEW artifact message (m2) with a NEW version identity;
	// the annotated v1 item keeps its id, version and digest byte-identically.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/artifacts", "", "u1", 1)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &artifacts))
	require.Len(t, artifacts.Data.Items, 2, "修订产生新版本条目")
	byID := map[string]struct {
		Version string
		Digest  string
	}{}
	for _, item := range artifacts.Data.Items {
		byID[item.ID] = struct {
			Version string
			Digest  string
		}{item.Version, item.Digest}
	}
	require.Equal(t, researchDigestV1[:16], byID["m1:0"].Version, "已批注 v1 的版本身份保持不变（AC2）")
	require.Equal(t, researchDigestV1, byID["m1:0"].Digest, "已批注 v1 的内容摘要保持不变（AC2）")
	require.Equal(t, researchDigestV2[:16], byID["m2:0"].Version, "修订产出新版本身份")

	// The annotation still resolves to v1 and v1 only.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/annotations", "", "u1", 1)
	require.Contains(t, w.Body.String(), `"`+v1+`"`, "批注仍钉在 v1 的版本身份上")
	require.NotContains(t, w.Body.String(), researchDigestV2[:16])
}
```

- [ ] **Step 2: 运行确认通过（并确认失败路径真实失败）**

Run:
```bash
go test ./internal/application/repository/ -run 'TestTaskResearchEndToEnd' -count=1 -v
```
Expected: 3 个测试 PASS。随后做一次「防伪检查」：临时把 `TestTaskResearchEndToEndParallelReadOnlyDelegations` 中的第二个 `env.do` 委派改成源 `["kb-secret"]` 再跑，确认该测试 FAIL（证明断言真实生效），改回后复跑 PASS。

Run（回归：既有协作/工件面不受影响）:
```bash
go test ./internal/application/repository/ -run 'TestTaskCollaboration|TestTaskGrant' -count=1
```
Expected: ok（工件列表的版本身份回归在 handler 包：`go test ./internal/handler/session/ -run TestListWorkbenchArtifacts -count=1`，Task 2 Step 4 与计划级验证命令均已覆盖）。

- [ ] **Step 3: Commit**

```bash
git add internal/application/repository/task_research_http_test.go
git commit -m "test(workbench): T17 research delegation and annotation version immutability E2E (T17 #47 task 3)"
```

---

### Task 4: contracts——research wire 类型与解析器

**Files:**
- Create: `packages/contracts/src/mobile/research.ts`
- Create: `packages/contracts/src/mobile/research.test.ts`
- Modify: `packages/contracts/src/index.ts`（末尾追加 2 行导出）

**Interfaces:**
- Consumes: Task 2 的 wire JSON（snake_case，信封 `{success:true, data:{...}}`）。
- Produces（Task 5 依赖）:
  - `ResearchDelegationWire = { delegation_id: string; run_id: string; session_id: string; objective: string; sources: string[]; status: 'assigned' | 'completed'; summary?: string; created_at: string }`
  - `ResearchListWire = { items: ResearchDelegationWire[] }`
  - `AnnotationWire = { annotation_id: string; run_id: string; material_id: string; base_version: string; body: string; author_id: string; created_at: string }`
  - `AnnotationListWire = { items: AnnotationWire[] }`
  - `parseResearchListResponse(value: unknown): ResearchListWire`、`parseAnnotationListResponse(value: unknown): AnnotationListWire`（整体拒绝：任何字段缺失/类型不符即抛错，不部分渲染）

- [ ] **Step 1: 写失败测试**

创建 `packages/contracts/src/mobile/research.test.ts`：

```ts
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { parseAnnotationListResponse, parseResearchListResponse } from './research.ts';

const delegation = {
  delegation_id: 'd1', run_id: 'r1', session_id: 's1',
  objective: 'survey retrieval baselines', sources: ['kb-1'],
  status: 'assigned', created_at: '2026-09-26T00:00:00Z',
};

test('parseResearchListResponse accepts a real envelope', () => {
  const parsed = parseResearchListResponse({ success: true, data: { items: [delegation, { ...delegation, delegation_id: 'd2', status: 'completed', summary: '3 findings' }] } });
  assert.equal(parsed.items.length, 2);
  assert.equal(parsed.items[1].status, 'completed');
  assert.equal(parsed.items[1].summary, '3 findings');
});

test('parseResearchListResponse rejects malformed envelopes wholesale', () => {
  for (const bad of [
    undefined, null, 42, 'x', [],
    {}, { success: false, data: { items: [] } },
    { success: true }, { success: true, data: null },
    { success: true, data: {} },
    { success: true, data: { items: 'nope' } },
    { success: true, data: { items: [{ ...delegation, delegation_id: 7 }] } },
    { success: true, data: { items: [{ ...delegation, sources: 'kb-1' }] } },
    { success: true, data: { items: [{ ...delegation, status: 'running' }] } },
    { success: true, data: { items: [{ ...delegation, created_at: undefined }] } },
    { success: true, data: { items: [{ ...delegation, objective: null }] } },
  ]) {
    assert.throws(() => parseResearchListResponse(bad), `must reject: ${JSON.stringify(bad)}`);
  }
});

test('parseAnnotationListResponse accepts a real envelope and rejects malformed rows', () => {
  const annotation = {
    annotation_id: 'an1', run_id: 'r1', material_id: 'm1:0',
    base_version: '9a2f1c3d4e5f6a7b', body: '结论第三段缺引用',
    author_id: 'u3', created_at: '2026-09-26T00:00:00Z',
  };
  const parsed = parseAnnotationListResponse({ success: true, data: { items: [annotation] } });
  assert.equal(parsed.items[0].base_version, '9a2f1c3d4e5f6a7b');
  assert.equal(parsed.items[0].body, '结论第三段缺引用');

  for (const bad of [
    { success: true, data: { items: [{ ...annotation, material_id: '' }] } },
    { success: true, data: { items: [{ ...annotation, base_version: 12 }] } },
    { success: true, data: { items: [{ ...annotation, body: undefined }] } },
    { success: true, data: { items: [annotation, null] } },
  ]) {
    assert.throws(() => parseAnnotationListResponse(bad), `must reject: ${JSON.stringify(bad)}`);
  }
});
```

- [ ] **Step 2: 运行确认失败**

Run:
```bash
pnpm exec tsx --test packages/contracts/src/mobile/research.test.ts
```
Expected: FAIL（`Cannot find module './research.ts'`）。

- [ ] **Step 3: 最小实现**

创建 `packages/contracts/src/mobile/research.ts`：

```ts
// T17（Issue #47）：只读研究委派与版本钉定批注的信封契约。键集与
// internal/handler/session/workbench_research.go 的 JSON tag 逐字对应；
// 解析器整体拒绝（不部分渲染）——客户端绝不渲染服务端从未确认的行。

export type ResearchStatusWire = 'assigned' | 'completed';

export interface ResearchDelegationWire {
  delegation_id: string;
  run_id: string;
  session_id: string;
  objective: string;
  sources: string[];
  status: ResearchStatusWire;
  summary?: string;
  created_at: string;
}

export interface ResearchListWire {
  items: ResearchDelegationWire[];
}

export interface AnnotationWire {
  annotation_id: string;
  run_id: string;
  material_id: string;
  base_version: string;
  body: string;
  author_id: string;
  created_at: string;
}

export interface AnnotationListWire {
  items: AnnotationWire[];
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function envelope(value: unknown): Record<string, unknown> {
  if (!isObject(value)) throw new Error('research response must be an object');
  if (value.success !== true || !isObject(value.data)) {
    throw new Error('research response must be a success envelope with data');
  }
  return value.data;
}

function rows<T>(value: unknown, parseRow: (row: unknown) => T): T[] {
  const data = envelope(value);
  if (!Array.isArray(data.items)) throw new Error('research response items must be an array');
  return data.items.map(parseRow);
}

function stringField(row: Record<string, unknown>, key: string): string {
  const value = row[key];
  if (typeof value !== 'string' || value === '') throw new Error(`research row.${key} must be a non-empty string`);
  return value;
}

function optionalStringField(row: Record<string, unknown>, key: string): string | undefined {
  const value = row[key];
  if (value === undefined) return undefined;
  if (typeof value !== 'string' || value === '') throw new Error(`research row.${key} must be a non-empty string when present`);
  return value;
}

const RESEARCH_STATUSES: readonly ResearchStatusWire[] = ['assigned', 'completed'];

function delegationRow(row: unknown): ResearchDelegationWire {
  if (!isObject(row)) throw new Error('research delegation row must be an object');
  const status = row.status;
  if (typeof status !== 'string' || !RESEARCH_STATUSES.includes(status as ResearchStatusWire)) {
    throw new Error('research row.status must be assigned or completed');
  }
  if (!Array.isArray(row.sources) || row.sources.some((s) => typeof s !== 'string' || s === '')) {
    throw new Error('research row.sources must be an array of non-empty strings');
  }
  return {
    delegation_id: stringField(row, 'delegation_id'),
    run_id: stringField(row, 'run_id'),
    session_id: stringField(row, 'session_id'),
    objective: stringField(row, 'objective'),
    sources: row.sources as string[],
    status: status as ResearchStatusWire,
    summary: optionalStringField(row, 'summary'),
    created_at: stringField(row, 'created_at'),
  };
}

function annotationRow(row: unknown): AnnotationWire {
  if (!isObject(row)) throw new Error('annotation row must be an object');
  return {
    annotation_id: stringField(row, 'annotation_id'),
    run_id: stringField(row, 'run_id'),
    material_id: stringField(row, 'material_id'),
    base_version: stringField(row, 'base_version'),
    body: stringField(row, 'body'),
    author_id: stringField(row, 'author_id'),
    created_at: stringField(row, 'created_at'),
  };
}

/** 解析委派列表响应；任何字段缺失/类型不符整体抛错。 */
export function parseResearchListResponse(value: unknown): ResearchListWire {
  return { items: rows(value, delegationRow) };
}

/** 解析批注列表响应；任何字段缺失/类型不符整体抛错。 */
export function parseAnnotationListResponse(value: unknown): AnnotationListWire {
  return { items: rows(value, annotationRow) };
}
```

修改 `packages/contracts/src/index.ts`——在文件末尾追加：

```ts
export { parseResearchListResponse, parseAnnotationListResponse } from './mobile/research.ts';
export type { AnnotationWire, ResearchDelegationWire, ResearchListWire, ResearchStatusWire, AnnotationListWire } from './mobile/research.ts';
```

- [ ] **Step 4: 运行确认通过**

Run:
```bash
pnpm exec tsx --test packages/contracts/src/mobile/research.test.ts
```
Expected: pass（3 tests）。

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/mobile/research.ts packages/contracts/src/mobile/research.test.ts packages/contracts/src/index.ts
git commit -m "feat(contracts): research delegation and annotation wire contracts (T17 #47 task 4)"
```

---

### Task 5: api-client——createMobileResearchRemote

**Files:**
- Create: `packages/api-client/src/mobile/research.ts`
- Create: `packages/api-client/src/mobile/research.test.ts`
- Modify: `packages/api-client/package.json`（exports 表追加一行）

**Interfaces:**
- Consumes: Task 4 的解析器；`ClientRequest` 授权通道与 `requireDeploymentOrigin`（`packages/api-client/src/mobile/deployment-origin.ts`）；`unwrap` 信封范式（materials.ts:41-48）。
- Produces（Task 6/7 依赖；行类型与 mobile-core `ResearchBackendPort` 结构逐字一致）:
```ts
export interface ResearchRemote {
  delegate(input: { runId: string; objective: string; sources: string[] }): Promise<ResearchDelegationRow>;
  list(runId: string): Promise<{ runId: string; delegations: ResearchDelegationRow[] }>;
  complete(input: { runId: string; delegationId: string; summary: string }): Promise<ResearchDelegationRow>;
  annotate(input: { runId: string; materialId: string; baseVersion: string; body: string }): Promise<ResearchAnnotationRow>;
  annotations(runId: string): Promise<{ runId: string; annotations: ResearchAnnotationRow[] }>;
}
export interface ResearchDelegationRow {
  delegationId: string; runId: string; sessionId: string; objective: string;
  sources: string[]; status: 'assigned' | 'completed'; summary?: string; createdAt: string;
}
export interface ResearchAnnotationRow {
  annotationId: string; runId: string; materialId: string;
  baseVersion: string; body: string; authorId: string; createdAt: string;
}
export function createMobileResearchRemote(options: { origin: string; request: (input: ClientRequest) => Promise<unknown> }): ResearchRemote;
```
  错误语义：HTTP 409 且响应含 `annotation_base_version_conflict` → 抛 `Error('RESEARCH_BASE_VERSION_CONFLICT')`；其余非 2xx/畸形信封 → 抛 `Error('RESEARCH_BACKEND')`（message 即裸错误码，与 sweep 既有 Remote 一致）。

- [ ] **Step 1: 写失败测试**

创建 `packages/api-client/src/mobile/research.test.ts`（请求替身形态与 `materials.test.ts:27-52` 的权威样例一致：捕获 `ClientRequest`，断言 `method` 与相对 `path`）：

```ts
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { ApiError } from '../errors.ts';
import type { ClientRequest } from '../client.ts';
import { createMobileResearchRemote } from './research.ts';

const delegationRow = {
  delegation_id: 'd1', run_id: 'r1', session_id: 's1',
  objective: 'survey', sources: ['kb-1'], status: 'assigned',
  created_at: '2026-09-26T00:00:00Z',
};

const annotationRow = {
  annotation_id: 'an1', run_id: 'r1', material_id: 'm1:0',
  base_version: '9a2f1c3d4e5f6a7b', body: 'b', author_id: 'u3',
  created_at: '2026-09-26T00:00:00Z',
};

test('delegate POSTs to the research endpoint and projects a semantic row', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileResearchRemote({
    origin: 'https://weknora.example.com',
    request: async (input) => {
      requests.push(input);
      return { success: true, data: { delegation: delegationRow } };
    },
  });
  const row = await remote.delegate({ runId: 'r1', objective: 'survey', sources: ['kb-1'] });
  assert.equal(requests[0]!.method, 'POST');
  assert.equal(requests[0]!.path, '/api/v1/workbench/executions/r1/research');
  assert.deepEqual(requests[0]!.body, { objective: 'survey', sources: ['kb-1'] });
  assert.equal(row.delegationId, 'd1');
  assert.equal(row.status, 'assigned');
  assert.equal(row.sessionId, 's1');
});

test('complete posts the summary to the delegation summary endpoint', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileResearchRemote({
    origin: 'https://weknora.example.com',
    request: async (input) => {
      requests.push(input);
      return { success: true, data: { delegation: { ...delegationRow, status: 'completed', summary: 'done' } } };
    },
  });
  const row = await remote.complete({ runId: 'r1', delegationId: 'd1', summary: 'done' });
  assert.equal(requests[0]!.method, 'POST');
  assert.equal(requests[0]!.path, '/api/v1/workbench/executions/r1/research/d1/summary');
  assert.deepEqual(requests[0]!.body, { summary: 'done' });
  assert.equal(row.status, 'completed');
  assert.equal(row.summary, 'done');
});

test('list/annotations GET the run-scoped endpoints and map rows', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileResearchRemote({
    origin: 'https://weknora.example.com',
    request: async (input) => {
      requests.push(input);
      if (input.path.endsWith('/research')) return { success: true, data: { items: [delegationRow] } };
      return { success: true, data: { items: [annotationRow] } };
    },
  });
  const delegations = await remote.list('r1');
  assert.equal(delegations.runId, 'r1');
  assert.equal(delegations.delegations[0]!.delegationId, 'd1');
  const annotations = await remote.annotations('r1');
  assert.equal(annotations.annotations[0]!.baseVersion, '9a2f1c3d4e5f6a7b');
  assert.equal(annotations.annotations[0]!.materialId, 'm1:0');
  assert.deepEqual(requests.map((request) => request.path), [
    '/api/v1/workbench/executions/r1/research',
    '/api/v1/workbench/executions/r1/annotations',
  ]);
});

test('annotate maps a 409 stale base version to RESEARCH_BASE_VERSION_CONFLICT', async () => {
  const remote = createMobileResearchRemote({
    origin: 'https://weknora.example.com',
    request: async () => { throw new ApiError({ status: 409, code: 'annotation_base_version_conflict', message: 'annotation_base_version_conflict' }); },
  });
  await assert.rejects(
    remote.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'stale', body: 'x' }),
    (error: unknown) => (error as { code?: string }).code === 'RESEARCH_BASE_VERSION_CONFLICT',
  );
});

test('generic failures and malformed envelopes map to RESEARCH_BACKEND with the code attached', async () => {
  const remote = createMobileResearchRemote({
    origin: 'https://weknora.example.com',
    request: async () => { throw new ApiError({ status: 500, code: 'research_backend', message: 'research_backend' }); },
  });
  const failure = await remote.list('r1').then(() => undefined, (error: unknown) => error as { code?: string });
  assert.equal(failure?.code, 'RESEARCH_BACKEND');

  const bad = createMobileResearchRemote({ origin: 'https://weknora.example.com', request: async () => ({ success: true }) });
  await assert.rejects(bad.list('r1'), /RESEARCH_BACKEND/);
});
```

- [ ] **Step 2: 运行确认失败**

Run:
```bash
pnpm exec tsx --test packages/api-client/src/mobile/research.test.ts
```
Expected: FAIL（模块不存在）。

- [ ] **Step 3: 最小实现**

创建 `packages/api-client/src/mobile/research.ts`（请求以 `ClientRequest` 的 `method`+相对 `path`+`body` 表达——`packages/api-client/src/client.ts:38-46` 的真实形态，权威样例 `materials.ts:73-91`；错误映射镜像 `task-office.ts:256-266` 的 `ApiError.status` 判定并携带 `.code` 属性）：

```ts
import type { ClientRequest } from '../client.ts';
import { ApiError } from '../errors.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';
import { parseAnnotationListResponse, parseResearchListResponse } from '@weknora/contracts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface ResearchRemoteOptions {
  /** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输、不持有 token。 */
  request: Request;
}

/** 语义行（camelCase）：与 mobile-core research 模块的 Port 行结构逐字一致。 */
export interface ResearchDelegationRow {
  delegationId: string;
  runId: string;
  sessionId: string;
  objective: string;
  sources: string[];
  status: 'assigned' | 'completed';
  summary?: string;
  createdAt: string;
}

export interface ResearchAnnotationRow {
  annotationId: string;
  runId: string;
  materialId: string;
  baseVersion: string;
  body: string;
  authorId: string;
  createdAt: string;
}

/** 与 mobile-core ResearchBackendPort 结构逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface ResearchRemote {
  delegate(input: { runId: string; objective: string; sources: string[] }): Promise<ResearchDelegationRow>;
  list(runId: string): Promise<{ runId: string; delegations: ResearchDelegationRow[] }>;
  complete(input: { runId: string; delegationId: string; summary: string }): Promise<ResearchDelegationRow>;
  annotate(input: { runId: string; materialId: string; baseVersion: string; body: string }): Promise<ResearchAnnotationRow>;
  annotations(runId: string): Promise<{ runId: string; annotations: ResearchAnnotationRow[] }>;
}

function unwrap(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw coded('RESEARCH_BACKEND');
  const envelope = value as { success?: unknown; data?: unknown };
  if (envelope.success !== true || typeof envelope.data !== 'object' || envelope.data === null || Array.isArray(envelope.data)) {
    throw coded('RESEARCH_BACKEND');
  }
  return envelope.data as Record<string, unknown>;
}

function coded(message: string, cause?: unknown): Error {
  const error = new Error(message, cause === undefined ? undefined : { cause });
  (error as unknown as { code?: string }).code = message;
  return error;
}

/** annotate 的 409 有唯一含义（base_version 钉版冲突）；其余失败统一 RESEARCH_BACKEND。 */
function annotateFailure(error: unknown): Error {
  if (error instanceof ApiError && error.status === 409) return coded('RESEARCH_BASE_VERSION_CONFLICT', error);
  return coded('RESEARCH_BACKEND', error);
}

/** catch 处理器必须「重抛」而不是返回：返回错误对象会把 promise 变为 resolved。 */
function rethrow(map: (error: unknown) => Error): (error: unknown) => never {
  return (error: unknown) => { throw map(error); };
}

/** GET 通路的信封校验 + 失败映射（原始响应必须先过 unwrap 再交给解析器）。 */
async function getEnvelope(request: Request, path: string): Promise<Record<string, unknown>> {
  try {
    return unwrap(await request({ method: 'GET', path }));
  } catch (error) {
    if ((error as { code?: unknown } | null)?.code === 'RESEARCH_BACKEND') throw error;
    throw coded('RESEARCH_BACKEND', error);
  }
}

function delegationFrom(value: unknown): ResearchDelegationRow {
  const wire = parseResearchListResponse({ success: true, data: { items: [value] } }).items[0]!;
  return {
    delegationId: wire.delegation_id,
    runId: wire.run_id,
    sessionId: wire.session_id,
    objective: wire.objective,
    sources: wire.sources,
    status: wire.status,
    summary: wire.summary,
    createdAt: wire.created_at,
  };
}

function annotationFrom(value: unknown): ResearchAnnotationRow {
  const wire = parseAnnotationListResponse({ success: true, data: { items: [value] } }).items[0]!;
  return {
    annotationId: wire.annotation_id,
    runId: wire.run_id,
    materialId: wire.material_id,
    baseVersion: wire.base_version,
    body: wire.body,
    authorId: wire.author_id,
    createdAt: wire.created_at,
  };
}

export function createMobileResearchRemote(options: ResearchRemoteOptions): ResearchRemote {
  requireDeploymentOrigin(options.origin);
  const researchPath = (runId: string): string => `/api/v1/workbench/executions/${encodeURIComponent(runId)}/research`;
  const annotationsPath = (runId: string): string => `/api/v1/workbench/executions/${encodeURIComponent(runId)}/annotations`;
  return {
    async delegate(input) {
      const data = unwrap(await options.request({ method: 'POST', path: researchPath(input.runId), body: { objective: input.objective, sources: input.sources } }).catch(rethrow((error) => coded('RESEARCH_BACKEND', error))));
      return delegationFrom(data.delegation);
    },
    async list(runId) {
      const data = await getEnvelope(options.request, researchPath(runId));
      const wire = parseResearchListResponse({ success: true, data });
      return { runId, delegations: wire.items.map((row) => ({
        delegationId: row.delegation_id, runId: row.run_id, sessionId: row.session_id,
        objective: row.objective, sources: row.sources, status: row.status,
        summary: row.summary, createdAt: row.created_at,
      })) };
    },
    async complete(input) {
      const data = unwrap(await options.request({ method: 'POST', path: `${researchPath(input.runId)}/${encodeURIComponent(input.delegationId)}/summary`, body: { summary: input.summary } }).catch(rethrow((error) => coded('RESEARCH_BACKEND', error))));
      return delegationFrom(data.delegation);
    },
    async annotate(input) {
      const data = unwrap(await options.request({ method: 'POST', path: annotationsPath(input.runId), body: { material_id: input.materialId, base_version: input.baseVersion, body: input.body } }).catch(rethrow(annotateFailure)));
      return annotationFrom(data.annotation);
    },
    async annotations(runId) {
      const data = await getEnvelope(options.request, annotationsPath(runId));
      const wire = parseAnnotationListResponse({ success: true, data });
      return { runId, annotations: wire.items.map((row) => ({
        annotationId: row.annotation_id, runId: row.run_id, materialId: row.material_id,
        baseVersion: row.base_version, body: row.body, authorId: row.author_id, createdAt: row.created_at,
      })) };
    },
  };
}
```

修改 `packages/api-client/package.json`——exports 表在 `"./mobile/code-delivery"` 行后追加：

```json
    "./mobile/research": "./src/mobile/research.ts",
```

- [ ] **Step 4: 运行确认通过**

Run:
```bash
pnpm exec tsx --test packages/api-client/src/mobile/research.test.ts && pnpm exec tsx --test packages/api-client/src/mobile/materials.test.ts
```
Expected: 两个文件全 pass（新测试 + 既有 materials 回归）。

- [ ] **Step 5: Commit**

```bash
git add packages/api-client/src/mobile/research.ts packages/api-client/src/mobile/research.test.ts packages/api-client/package.json
git commit -m "feat(api-client): mobile research remote adapter (T17 #47 task 5)"
```

---

### Task 6: mobile-core——research 深模块（句柄、离线批注草稿、修订组合）

**Files:**
- Create: `packages/mobile-core/src/research/types.ts`
- Create: `packages/mobile-core/src/research/task-research.ts`
- Create: `packages/mobile-core/src/research/in-memory-research-remote.ts`
- Create: `packages/mobile-core/src/research/task-research.test.ts`
- Modify: `packages/mobile-core/src/index.ts`（末尾追加导出块）

**Interfaces:**
- Consumes: Task 5 的 `ResearchRemote` 结构；`ScopeLease`/`leaseActive`/`leaseScopeOf`（包内）；`TaskCommandPort`（`task-office/task-detail.ts:87`，公共导出）；`OfflineGate`（offline/offline-gate.ts）；`ScopedStore`（vault/scoped-vault.ts:31，草稿 id 白名单 `^[A-Za-z0-9._-]{1,64}$`）；`MaterialEntry`（material/types.ts:20）。
- Produces（Task 7 依赖）:
```ts
export type ResearchErrorCode =
  | 'RESEARCH_SCOPE_CHANGED' | 'RESEARCH_INVALID_INPUT' | 'RESEARCH_NOT_FOUND'
  | 'RESEARCH_CONFLICT' | 'RESEARCH_COMMAND_UNAVAILABLE' | 'RESEARCH_COMMAND_CONFLICT'
  | 'RESEARCH_DRAFT_UNAVAILABLE' | 'RESEARCH_BACKEND';
export class ResearchError extends Error { constructor(readonly code: ResearchErrorCode, options?: { cause?: unknown }); }
export interface ResearchBackendPort {
  delegate(input: { runId: string; objective: string; sources: string[] }): Promise<ResearchDelegationRow>;
  list(runId: string): Promise<{ runId: string; delegations: ResearchDelegationRow[] }>;
  complete(input: { runId: string; delegationId: string; summary: string }): Promise<ResearchDelegationRow>;
  annotate(input: { runId: string; materialId: string; baseVersion: string; body: string }): Promise<ResearchAnnotationRow>;
  annotations(runId: string): Promise<{ runId: string; annotations: ResearchAnnotationRow[] }>;
}
export interface ResearchDraftsPort { put(draft: ResearchAnnotationDraft): Promise<void>; list(): Promise<ResearchAnnotationDraft[]>; remove(draftId: string): Promise<void>; }
export interface ResearchAnnotationDraft { draftId: string; runId: string; materialId: string; baseVersion: string; body: string; draftedAt: string; }
export interface TaskResearchPorts {
  remote: ResearchBackendPort;
  commands?: TaskCommandPort;      // 缺失 → requestRevision fail closed（RESEARCH_COMMAND_UNAVAILABLE）
  gate?: OfflineGate;              // 缺失 → 无离线探测，annotate 直接走网络（物理离线由传输层兜底）
  drafts?: ResearchDraftsPort;     // 缺失且离线 → annotate fail closed（RESEARCH_DRAFT_UNAVAILABLE）
}
// lease 的唯一来源是 open({ lease })（与 material 模块同款）；端口不再持有 lease()，
// 避免「端口 lease 与 open lease 双来源」的歧义。
export interface TaskResearch { open(input: { lease: ScopeLease }): TaskResearchHandle; }
export interface TaskResearchHandle {
  delegate(input: { runId: string; objective: string; sources: string[] }): Promise<ResearchDelegationRow>;
  delegations(runId: string): Promise<ResearchDelegationRow[]>;
  complete(input: { runId: string; delegationId: string; summary: string }): Promise<ResearchDelegationRow>;
  annotate(input: { runId: string; materialId: string; baseVersion: string; body: string }): Promise<ResearchAnnotationReceipt>;
  annotations(runId: string): Promise<ResearchAnnotationRow[]>;
  requestRevision(input: ResearchRevisionInput): Promise<ResearchRevisionReceipt>;
  flushAnnotationDrafts(input: { runId: string }): Promise<Array<{ draftId: string; outcome: 'recorded' | 'conflict' | 'failed' }>>;
  pendingDrafts(): Promise<ResearchAnnotationDraft[]>;
  subscribe(listener: (event: ResearchEvent) => void): () => void;
  close(reason: string): void;
}
export type ResearchAnnotationReceipt = { status: 'recorded'; annotation: ResearchAnnotationRow } | { status: 'drafted'; draftId: string };
export interface ResearchRevisionInput { runId: string; materialId: string; baseVersion: string; note: string; action: 'steer' | 'queue_next'; expectedRevision: number; intentId?: string; }
export interface ResearchRevisionReceipt { intent: 'revision-request'; outcome: 'accepted' | 'conflict'; action: 'steer' | 'queue_next'; nextRunId?: string; at: string; }
export type ResearchEvent = { type: 'scope-closed'; reason?: string } | { type: 'annotation-drafted'; draftId: string } | { type: 'draft-flushed'; draftId: string };
export function createTaskResearch(ports: TaskResearchPorts): TaskResearch;
export function createScenarioResearchRemote(script?): ResearchBackendPort;   // 场景 Adapter
```
  行为不变量（Task 6 测试全部钉死）：每次异步提交前 `leaseActive` 检查（撤销 → RESEARCH_SCOPE_CHANGED + `scope-closed` 事件，零网络）；`objective` 空/`sources` 空或 >8/含空串 → RESEARCH_INVALID_INPUT（零网络）；离线（gate.status()==='offline'）批注 → 草稿入 drafts（id `research-ann-<n>`，白名单兼容）+ `annotation-drafted` 事件，绝不发网络；无 gate 或在线 → 直接 annotate；`RESEARCH_BASE_VERSION_CONFLICT` 错误映射 RESEARCH_CONFLICT；requestRevision 组合确定性文本 `请基于版本 ${baseVersion} 修订材料 ${materialId}：${note}` 委托 commands，conflict（错误 `.code`/message 含 `TASK_COMMAND_CONFLICT`）→ RESEARCH_COMMAND_CONFLICT；flush 按序重放，成功移除草稿，conflict 保留草稿并标 `conflict`，任一 SCOPE_CHANGED 立即中止后续；closed 句柄一切方法 RESEARCH_SCOPE_CHANGED。

- [ ] **Step 1: 写失败测试**

创建 `packages/mobile-core/src/research/task-research.test.ts`：

```ts
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import { createScenarioResearchRemote, type ScenarioResearchRemoteScript } from './in-memory-research-remote.ts';
import { createTaskResearch, ResearchError, type ResearchBackendPort, type ResearchDraftsPort, type ResearchEvent } from './task-research.ts';

// ─── 测试夹具：真实可撤销 lease + 记账式 drafts + 可脚本化 remote ─────────────
// leaseActive 以 instanceof RuntimeScopeLease 判定（runtime/scope-lease.ts:26-27），
// 夹具必须用真类实例（同 task-office.test.ts:15 先例），普通对象伪造恒 false。

function newLease(): RuntimeScopeLease {
  return new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: 'tenant-1' });
}

function codedError(code: string): Error {
  const error = new Error(code);
  (error as unknown as { code?: string }).code = code;
  return error;
}

function recordingDrafts(): ResearchDraftsPort & { rows: Map<string, unknown> } {
  const rows = new Map<string, unknown>();
  return {
    rows,
    async put(draft) { rows.set(draft.draftId, structuredClone(draft)); },
    async list() { return [...rows.values()] as never; },
    async remove(draftId) { rows.delete(draftId); },
  };
}

test('delegate validates scope and input before any network dispatch', async () => {
  const script: ScenarioResearchRemoteScript = { delegations: [] };
  const remote = createScenarioResearchRemote(script);
  let calls = 0;
  const counting: ResearchBackendPort = { ...remote, delegate: async (input) => { calls += 1; return remote.delegate(input); } };
  const activeGate = createTaskResearch({ remote: counting });
  const lease = newLease();
  const active = activeGate.open({ lease });

  // 非法输入（空 objective / 空 sources / 超量 sources / 空串源）→ 零网络。
  for (const bad of [
    { runId: 'r1', objective: '', sources: ['kb-1'] },
    { runId: 'r1', objective: 'x', sources: [] },
    { runId: 'r1', objective: 'x', sources: ['1', '2', '3', '4', '5', '6', '7', '8', '9'] },
    { runId: 'r1', objective: 'x', sources: ['kb-1', ' '] },
  ]) {
    await assert.rejects(active.delegate(bad), /RESEARCH_INVALID_INPUT/);
  }
  assert.equal(calls, 0);

  // 合法委派放行并返回行。
  const row = await active.delegate({ runId: 'r1', objective: 'survey', sources: ['kb-1'] });
  assert.equal(row.status, 'assigned');
  assert.equal(calls, 1);

  // 撤销后的迟到调用 → fail closed，零网络（Review Focus 5）。
  lease.revoke();
  await assert.rejects(active.delegate({ runId: 'r1', objective: 'x', sources: ['kb-1'] }), /RESEARCH_SCOPE_CHANGED/);
  assert.equal(calls, 1);
});

test('annotations record, conflict-map and draft offline', async () => {
  const remote = createScenarioResearchRemote({ delegations: [] });
  const drafts = recordingDrafts();
  const events: ResearchEvent[] = [];
  const module = createTaskResearch({ remote, drafts });
  const handle = module.open({ lease: newLease() });
  handle.subscribe((event) => events.push(event));

  // 在线批注：直接 recorded。
  const recorded = await handle.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: '9a2f1c3d4e5f6a7b', body: '缺引用' });
  assert.equal(recorded.status, 'recorded');
  assert.equal(recorded.status === 'recorded' && recorded.annotation.baseVersion, '9a2f1c3d4e5f6a7b');

  // 版本冲突映射 RESEARCH_CONFLICT。
  await assert.rejects(
    handle.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'stale', body: 'x' }),
    (error: unknown) => error instanceof ResearchError && error.code === 'RESEARCH_CONFLICT',
  );

  // 空批注体 → RESEARCH_INVALID_INPUT，零网络。
  await assert.rejects(handle.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: '9a2f1c3d4e5f6a7b', body: ' ' }), /RESEARCH_INVALID_INPUT/);

  // 离线批注：落加密草稿、零网络、可观测事件。
  const offlineGate = { status: async () => 'offline' as const, assertOnline: async () => { throw new Error('OFFLINE_ACTION_BLOCKED'); } };
  const offlineModule = createTaskResearch({ remote, gate: offlineGate, drafts });
  const offlineHandle = offlineModule.open({ lease: newLease() });
  offlineHandle.subscribe((event) => events.push(event)); // 事件挂在发起草稿的句柄上
  const drafted = await offlineHandle.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: '9a2f1c3d4e5f6a7b', body: '离线批注' });
  assert.equal(drafted.status, 'drafted');
  assert.match(drafted.status === 'drafted' ? drafted.draftId : '', /^research-ann-\d+$/);
  assert.equal((await offlineHandle.pendingDrafts()).length, 1);
  assert.ok(events.some((event) => event.type === 'annotation-drafted'));

  // 离线但无 drafts 端口 → RESEARCH_DRAFT_UNAVAILABLE（fail closed，不静默丢批注）。
  const noDrafts = createTaskResearch({ remote, gate: offlineGate });
  await assert.rejects(
    noDrafts.open({ lease: newLease() }).annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'v', body: 'x' }),
    /RESEARCH_DRAFT_UNAVAILABLE/,
  );
});

test('flush replays drafts in order; conflict keeps the draft; revocation aborts the rest', async () => {
  let conflicts = 0;
  const base = createScenarioResearchRemote({ delegations: [] });
  const flaky: ResearchBackendPort = {
    ...base,
    annotate: async (input) => {
      if (input.body === 'stale') { conflicts += 1; throw new Error('RESEARCH_BASE_VERSION_CONFLICT'); }
      if (input.body === 'after-revoke') throw new Error('RESEARCH_SCOPE_CHANGED');
      return base.annotate(input);
    },
  };
  const drafts = recordingDrafts();
  const module = createTaskResearch({ remote: flaky, drafts });
  const offlineGate = { status: async () => 'offline' as const, assertOnline: async () => { throw new Error('x'); } };
  const handle = module.open({ lease: newLease() });
  const offline = createTaskResearch({ remote: flaky, gate: offlineGate, drafts });
  const offlineHandle = offline.open({ lease: newLease() });
  await offlineHandle.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'v1', body: 'ok-1' });
  await offlineHandle.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'v1', body: 'stale' });
  await offlineHandle.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'v1', body: 'ok-2' });
  assert.equal((await handle.pendingDrafts()).length, 3);

  const outcomes = await handle.flushAnnotationDrafts({ runId: 'r1' });
  assert.deepEqual(outcomes.map((o) => o.outcome), ['recorded', 'conflict', 'recorded']);
  assert.ok(conflicts >= 1);
  assert.equal((await handle.pendingDrafts()).length, 1, 'conflict 草稿保留待处理');

  // lease 撤销 → flush 中止且不发出其余草稿。
  const lease = newLease();
  const revocable = createTaskResearch({ remote: flaky, gate: offlineGate, drafts });
  const revocableOffline = createTaskResearch({ remote: flaky, gate: offlineGate, drafts });
  const rOffline = revocableOffline.open({ lease });
  await rOffline.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'v1', body: 'ok-3' });
  await rOffline.annotate({ runId: 'r1', materialId: 'm1:0', baseVersion: 'v1', body: 'after-revoke' });
  lease.revoke();
  const rHandle = revocable.open({ lease });
  await assert.rejects(rHandle.flushAnnotationDrafts({ runId: 'r1' }), /RESEARCH_SCOPE_CHANGED/);
});

test('requestRevision composes a version-pinned text over the command port', async () => {
  const commands = {
    command: async (input: { text?: string; action: string }) => {
      assert.equal(input.action, 'queue_next');
      assert.equal(input.text, '请基于版本 9a2f1c3d4e5f6a7b 修订材料 m1:0：补齐引用');
      return { runId: 'r1', action: input.action, nextRunId: 'r-next' };
    },
  };
  const module = createTaskResearch({
    remote: createScenarioResearchRemote({ delegations: [] }),
    commands,
  });
  const handle = module.open({ lease: newLease() });
  const receipt = await handle.requestRevision({
    runId: 'r1', materialId: 'm1:0', baseVersion: '9a2f1c3d4e5f6a7b',
    note: '补齐引用', action: 'queue_next', expectedRevision: 3,
  });
  assert.equal(receipt.outcome, 'accepted');
  assert.equal(receipt.nextRunId, 'r-next');

  // 无 commands → fail closed；命令冲突 → RESEARCH_COMMAND_CONFLICT。
  const bare = createTaskResearch({ remote: createScenarioResearchRemote({ delegations: [] }) });
  await assert.rejects(
    bare.open({ lease: newLease() }).requestRevision({ runId: 'r1', materialId: 'm', baseVersion: 'v', note: 'n', action: 'steer', expectedRevision: 1 }),
    /RESEARCH_COMMAND_UNAVAILABLE/,
  );
  const conflicting = createTaskResearch({
    remote: createScenarioResearchRemote({ delegations: [] }),
    // 真实 remote 的冲突形态：Error 携带 .code = 'TASK_COMMAND_CONFLICT'
    // （api-client task-office.ts:256-266 的 coded()，跨包契约码）。
    commands: { command: async () => { throw codedError('TASK_COMMAND_CONFLICT'); } },
  });
  await assert.rejects(
    conflicting.open({ lease: newLease() }).requestRevision({ runId: 'r1', materialId: 'm', baseVersion: 'v', note: 'n', action: 'steer', expectedRevision: 1 }),
    (error: unknown) => error instanceof ResearchError && error.code === 'RESEARCH_COMMAND_CONFLICT',
  );
});

test('closed handles reject every path with SCOPE_CHANGED and emit scope-closed', async () => {
  const events: ResearchEvent[] = [];
  const module = createTaskResearch({ remote: createScenarioResearchRemote({ delegations: [] }) });
  const handle = module.open({ lease: newLease() });
  handle.subscribe((event) => events.push(event));
  handle.close('navigate-away');
  await assert.rejects(handle.delegations('r1'), /RESEARCH_SCOPE_CHANGED/);
  await assert.rejects(handle.annotate({ runId: 'r1', materialId: 'm', baseVersion: 'v', body: 'b' }), /RESEARCH_SCOPE_CHANGED/);
  assert.ok(events.some((event) => event.type === 'scope-closed' && event.reason === 'navigate-away'));
});
```

- [ ] **Step 2: 运行确认失败**

Run:
```bash
pnpm exec tsx --test packages/mobile-core/src/research/task-research.test.ts
```
Expected: FAIL（模块不存在）。

- [ ] **Step 3: 最小实现**

创建 `packages/mobile-core/src/research/types.ts`：

```ts
import type { ScopeLease } from '../runtime/types.ts';
import type { OfflineGate } from '../offline/offline-gate.ts';
import type { TaskCommandPort } from '../task-office/task-detail.ts';

/** T17（#47）research 深模块类型合同：委派行/批注行/草稿/事件。 */

export type ResearchStatus = 'assigned' | 'completed';

export interface ResearchDelegationRow {
  delegationId: string;
  runId: string;
  sessionId: string;
  objective: string;
  sources: string[];
  status: ResearchStatus;
  summary?: string;
  createdAt: string;
}

export interface ResearchAnnotationRow {
  annotationId: string;
  runId: string;
  materialId: string;
  baseVersion: string;
  body: string;
  authorId: string;
  createdAt: string;
}

/** wire 语义行 Port：api-client `./mobile/research` 与场景 Adapter 同构。 */
export interface ResearchBackendPort {
  delegate(input: { runId: string; objective: string; sources: string[] }): Promise<ResearchDelegationRow>;
  list(runId: string): Promise<{ runId: string; delegations: ResearchDelegationRow[] }>;
  complete(input: { runId: string; delegationId: string; summary: string }): Promise<ResearchDelegationRow>;
  annotate(input: { runId: string; materialId: string; baseVersion: string; body: string }): Promise<ResearchAnnotationRow>;
  annotations(runId: string): Promise<{ runId: string; annotations: ResearchAnnotationRow[] }>;
}

/** 离线批注草稿（设计 spec：「Offline mode permits approved reads, drafts and annotations」）。 */
export interface ResearchAnnotationDraft {
  draftId: string;
  runId: string;
  materialId: string;
  baseVersion: string;
  body: string;
  draftedAt: string;
}

/** 草稿仓储 Port：组合根用 Scoped Vault drafts 命名空间适配（加密、scope 隔离）。 */
export interface ResearchDraftsPort {
  put(draft: ResearchAnnotationDraft): Promise<void>;
  list(): Promise<ResearchAnnotationDraft[]>;
  remove(draftId: string): Promise<void>;
}

export interface TaskResearchPorts {
  remote: ResearchBackendPort;
  /** 修订请求的命令通道（#37）；缺失 → requestRevision fail closed。 */
  commands?: TaskCommandPort;
  /** 离线探测（#40）；缺失 → annotate 直接走网络（物理离线由传输层兜底）。 */
  gate?: OfflineGate;
  /** 离线批注草稿仓储；缺失且离线 → annotate fail closed（不静默丢批注）。 */
  drafts?: ResearchDraftsPort;
}

export interface TaskResearch {
  open(input: { lease: ScopeLease }): TaskResearchHandle;
}

export type ResearchAnnotationReceipt =
  | { status: 'recorded'; annotation: ResearchAnnotationRow }
  | { status: 'drafted'; draftId: string };

export interface ResearchRevisionInput {
  runId: string;
  materialId: string;
  baseVersion: string;
  note: string;
  action: 'steer' | 'queue_next';
  /** #37 修订纪律：来自任务详情视图的观察值（或服务端 CAS 证明的 +1）。 */
  expectedRevision: number;
  intentId?: string;
}

export interface ResearchRevisionReceipt {
  intent: 'revision-request';
  outcome: 'accepted' | 'conflict';
  action: 'steer' | 'queue_next';
  nextRunId?: string;
  at: string;
}

export type ResearchEvent =
  | { type: 'scope-closed'; reason?: string }
  | { type: 'annotation-drafted'; draftId: string }
  | { type: 'draft-flushed'; draftId: string };

export interface TaskResearchHandle {
  delegate(input: { runId: string; objective: string; sources: string[] }): Promise<ResearchDelegationRow>;
  delegations(runId: string): Promise<ResearchDelegationRow[]>;
  complete(input: { runId: string; delegationId: string; summary: string }): Promise<ResearchDelegationRow>;
  annotate(input: { runId: string; materialId: string; baseVersion: string; body: string }): Promise<ResearchAnnotationReceipt>;
  annotations(runId: string): Promise<ResearchAnnotationRow[]>;
  requestRevision(input: ResearchRevisionInput): Promise<ResearchRevisionReceipt>;
  flushAnnotationDrafts(input: { runId: string }): Promise<Array<{ draftId: string; outcome: 'recorded' | 'conflict' | 'failed' }>>;
  pendingDrafts(): Promise<ResearchAnnotationDraft[]>;
  subscribe(listener: (event: ResearchEvent) => void): () => void;
  close(reason: string): void;
}
```

创建 `packages/mobile-core/src/research/task-research.ts`：

```ts
import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import type {
  ResearchAnnotationDraft, ResearchAnnotationReceipt, ResearchAnnotationRow, ResearchBackendPort,
  ResearchDelegationRow, ResearchDraftsPort, ResearchEvent, ResearchRevisionInput, ResearchRevisionReceipt,
  TaskResearch, TaskResearchHandle, TaskResearchPorts,
} from './types.ts';

/** T17（#47）research 深模块：委派/批注/修订的全部编排纪律藏在句柄后。
 *  Screen 只表达意图；顺序、scope 检查、离线草稿、错误映射都在这里。 */

export type ResearchErrorCode =
  | 'RESEARCH_SCOPE_CHANGED'
  | 'RESEARCH_INVALID_INPUT'
  | 'RESEARCH_NOT_FOUND'
  | 'RESEARCH_CONFLICT'
  | 'RESEARCH_COMMAND_UNAVAILABLE'
  | 'RESEARCH_COMMAND_CONFLICT'
  | 'RESEARCH_DRAFT_UNAVAILABLE'
  | 'RESEARCH_BACKEND';

export class ResearchError extends Error {
  constructor(readonly code: ResearchErrorCode, options?: { cause?: unknown }) {
    super(code, options);
    this.name = 'ResearchError';
  }
}

const MAX_SOURCES = 8;
const DRAFT_ID_PATTERN = /^[A-Za-z0-9._-]{1,64}$/;

const messageOf = (failure: unknown): string => (failure instanceof Error ? failure.message : String(failure));

/** 跨包契约码优先读 Error.code（api-client remote 的 coded() 形态），回退到 message。 */
const failureCode = (failure: unknown): string => {
  const code = (failure as { code?: unknown } | null)?.code;
  if (typeof code === 'string' && code !== '') return code;
  return messageOf(failure);
};

function backendFailure(failure: unknown): ResearchError {
  const code = failureCode(failure);
  if (code.includes('RESEARCH_BASE_VERSION_CONFLICT')) return new ResearchError('RESEARCH_CONFLICT', { cause: failure });
  if (code.includes('RESEARCH_SCOPE_CHANGED')) return new ResearchError('RESEARCH_SCOPE_CHANGED', { cause: failure });
  if (code.includes('RESEARCH_NOT_FOUND')) return new ResearchError('RESEARCH_NOT_FOUND', { cause: failure });
  return new ResearchError('RESEARCH_BACKEND', { cause: failure });
}

export function createTaskResearch(ports: TaskResearchPorts): TaskResearch {
  const nextDraftId = (): string => `research-ann-${(++draftSeq).toString()}`;
  let draftSeq = 0;
  return {
    open({ lease }: { lease: ScopeLease }): TaskResearchHandle {
      let closed = false;
      const listeners = new Set<(event: ResearchEvent) => void>();
      const emit = (event: ResearchEvent) => { for (const listener of listeners) listener(event); };
      const check = (): void => {
        if (closed || !leaseActive(lease)) throw new ResearchError('RESEARCH_SCOPE_CHANGED');
      };
      const requireDrafts = (): ResearchDraftsPort => {
        if (ports.drafts === undefined) throw new ResearchError('RESEARCH_DRAFT_UNAVAILABLE');
        return ports.drafts;
      };
      const validateDelegation = (input: { objective: string; sources: string[] }): void => {
        if (typeof input.objective !== 'string' || input.objective.trim() === '') throw new ResearchError('RESEARCH_INVALID_INPUT');
        if (!Array.isArray(input.sources) || input.sources.length === 0 || input.sources.length > MAX_SOURCES) throw new ResearchError('RESEARCH_INVALID_INPUT');
        if (input.sources.some((source) => typeof source !== 'string' || source.trim() === '')) throw new ResearchError('RESEARCH_INVALID_INPUT');
      };
      return {
        async delegate(input) {
          check();
          validateDelegation(input);
          try {
            return await ports.remote.delegate({ runId: input.runId, objective: input.objective.trim(), sources: input.sources.map((source) => source.trim()) });
          } catch (failure) {
            throw backendFailure(failure);
          }
        },
        async delegations(runId) {
          check();
          try {
            return (await ports.remote.list(runId)).delegations;
          } catch (failure) {
            throw backendFailure(failure);
          }
        },
        async complete(input) {
          check();
          if (input.summary.trim() === '') throw new ResearchError('RESEARCH_INVALID_INPUT');
          try {
            return await ports.remote.complete(input);
          } catch (failure) {
            throw backendFailure(failure);
          }
        },
        async annotate(input): Promise<ResearchAnnotationReceipt> {
          check();
          if (typeof input.body !== 'string' || input.body.trim() === '') throw new ResearchError('RESEARCH_INVALID_INPUT');
          if (input.baseVersion.trim() === '' || input.materialId.trim() === '') throw new ResearchError('RESEARCH_INVALID_INPUT');
          const offline = ports.gate ? (await ports.gate.status()) === 'offline' : false;
          if (offline) {
            const drafts = requireDrafts();
            const draftId = nextDraftId();
            if (!DRAFT_ID_PATTERN.test(draftId)) throw new ResearchError('RESEARCH_DRAFT_UNAVAILABLE');
            const draft: ResearchAnnotationDraft = {
              draftId, runId: input.runId, materialId: input.materialId.trim(),
              baseVersion: input.baseVersion.trim(), body: input.body.trim(), draftedAt: new Date().toISOString(),
            };
            await drafts.put(draft);
            emit({ type: 'annotation-drafted', draftId });
            return { status: 'drafted', draftId };
          }
          try {
            const annotation = await ports.remote.annotate(input);
            return { status: 'recorded', annotation };
          } catch (failure) {
            throw backendFailure(failure);
          }
        },
        async annotations(runId) {
          check();
          try {
            return (await ports.remote.annotations(runId)).annotations;
          } catch (failure) {
            throw backendFailure(failure);
          }
        },
        async requestRevision(input): Promise<ResearchRevisionReceipt> {
          check();
          if (ports.commands === undefined) throw new ResearchError('RESEARCH_COMMAND_UNAVAILABLE');
          if (input.note.trim() === '' || input.baseVersion.trim() === '' || input.materialId.trim() === '') {
            throw new ResearchError('RESEARCH_INVALID_INPUT');
          }
          // 确定性版本钉定文本：Lead Agent 依据该文本在授权版本上派生新版本。
          const text = `请基于版本 ${input.baseVersion.trim()} 修订材料 ${input.materialId.trim()}：${input.note.trim()}`;
          try {
            const ack = await ports.commands.command({
              runId: input.runId, action: input.action, text,
              expectedRevision: input.expectedRevision, ...(input.intentId === undefined ? {} : { intentId: input.intentId }),
            });
            return {
              intent: 'revision-request', outcome: 'accepted', action: input.action,
              ...(ack.nextRunId === undefined ? {} : { nextRunId: ack.nextRunId }), at: new Date().toISOString(),
            };
          } catch (failure) {
            // 真实契约码是 TASK_COMMAND_CONFLICT（task-detail.ts:86 契约注释 +
            // api-client task-office.ts:262 的 coded()；不是 TASK_OFFICE_* 前缀）。
            if (failureCode(failure).includes('TASK_COMMAND_CONFLICT')) {
              throw new ResearchError('RESEARCH_COMMAND_CONFLICT', { cause: failure });
            }
            throw new ResearchError('RESEARCH_BACKEND', { cause: failure });
          }
        },
        async flushAnnotationDrafts({ runId }) {
          check();
          const drafts = requireDrafts();
          const outcomes: Array<{ draftId: string; outcome: 'recorded' | 'conflict' | 'failed' }> = [];
          for (const draft of await drafts.list()) {
            if (draft.runId !== runId) continue;
            try {
              check();
              await ports.remote.annotate({ runId: draft.runId, materialId: draft.materialId, baseVersion: draft.baseVersion, body: draft.body });
              await drafts.remove(draft.draftId);
              outcomes.push({ draftId: draft.draftId, outcome: 'recorded' });
              emit({ type: 'draft-flushed', draftId: draft.draftId });
            } catch (failure) {
              if (failure instanceof ResearchError && failure.code === 'RESEARCH_SCOPE_CHANGED') throw failure;
              if (failureCode(failure).includes('RESEARCH_BASE_VERSION_CONFLICT')) {
                outcomes.push({ draftId: draft.draftId, outcome: 'conflict' }); // 版本已过期：草稿保留，待用户重读版本后重提
                continue;
              }
              outcomes.push({ draftId: draft.draftId, outcome: 'failed' });
            }
          }
          return outcomes;
        },
        async pendingDrafts() {
          check();
          if (ports.drafts === undefined) return [];
          return ports.drafts.list();
        },
        subscribe(listener) {
          listeners.add(listener);
          return () => listeners.delete(listener);
        },
        close(reason: string) {
          if (closed) return;
          closed = true;
          emit({ type: 'scope-closed', reason });
        },
      };
    },
  };
}
```

创建 `packages/mobile-core/src/research/in-memory-research-remote.ts`：

```ts
import type { ResearchAnnotationRow, ResearchBackendPort, ResearchDelegationRow } from './types.ts';

/** 场景 Adapter（module-seams §4.4/§7.3 同款）：脚本化委派与批注，供 Interface 级
 *  测试与 composition 的离线替身使用；绝不冒充真实集成证据。 */

export interface ScenarioResearchRemoteScript {
  delegations: Array<Partial<ResearchDelegationRow> & { objective: string; sources: string[] }>;
  annotations?: Array<{ materialId: string; baseVersion: string; body: string; authorId?: string }>;
  /** 置真后 annotate 恒以 RESEARCH_BASE_VERSION_CONFLICT 失败（冲突分支脚本）。 */
  conflictOnAnnotate?: boolean;
}

export function createScenarioResearchRemote(script: ScenarioResearchRemoteScript): ResearchBackendPort {
  let delegationSeq = 0;
  let annotationSeq = 0;
  const recorded: ResearchAnnotationRow[] = (script.annotations ?? []).map((row, index) => ({
    annotationId: `an-scenario-${index + 1}`, runId: 'r-scenario', materialId: row.materialId,
    baseVersion: row.baseVersion, body: row.body, authorId: row.authorId ?? 'u-scenario',
    createdAt: '2026-09-26T00:00:00Z',
  }));
  const delegationRows: ResearchDelegationRow[] = script.delegations.map((row, index) => ({
    delegationId: row.delegationId ?? `d-scenario-${index + 1}`,
    runId: row.runId ?? 'r-scenario',
    sessionId: row.sessionId ?? 's-scenario',
    objective: row.objective,
    sources: row.sources,
    status: row.status ?? 'assigned',
    summary: row.summary,
    createdAt: row.createdAt ?? '2026-09-26T00:00:00Z',
  }));
  return {
    async delegate(input) {
      delegationSeq += 1;
      const row: ResearchDelegationRow = {
        delegationId: `d-live-${delegationSeq}`, runId: input.runId, sessionId: 's-live',
        objective: input.objective, sources: input.sources, status: 'assigned', createdAt: new Date().toISOString(),
      };
      delegationRows.push(row);
      return row;
    },
    async list(runId) {
      return { runId, delegations: delegationRows.filter((row) => row.runId === runId) };
    },
    async complete(input) {
      const row = delegationRows.find((candidate) => candidate.delegationId === input.delegationId);
      if (row === undefined) throw new Error('RESEARCH_NOT_FOUND');
      row.status = 'completed';
      row.summary = input.summary;
      return row;
    },
    async annotate(input) {
      if (script.conflictOnAnnotate === true || input.baseVersion === 'stale') throw new Error('RESEARCH_BASE_VERSION_CONFLICT');
      annotationSeq += 1;
      const row: ResearchAnnotationRow = {
        annotationId: `an-live-${annotationSeq}`, runId: input.runId, materialId: input.materialId,
        baseVersion: input.baseVersion, body: input.body, authorId: 'u-live', createdAt: new Date().toISOString(),
      };
      recorded.push(row);
      return row;
    },
    async annotations(runId) {
      return { runId, annotations: recorded.filter((row) => row.runId === runId) };
    },
  };
}
```

修改 `packages/mobile-core/src/index.ts`——文件末尾追加：

```ts
// —— T17 (#47) 只读研究委派与版本化批注 ——
export { ResearchError, createTaskResearch } from './research/task-research.ts';
export type { ResearchErrorCode } from './research/task-research.ts';
export type {
  ResearchAnnotationDraft, ResearchAnnotationReceipt, ResearchAnnotationRow, ResearchBackendPort,
  ResearchDelegationRow, ResearchDraftsPort, ResearchEvent, ResearchRevisionInput, ResearchRevisionReceipt,
  ResearchStatus, TaskResearch, TaskResearchHandle, TaskResearchPorts,
} from './research/types.ts';
export { createScenarioResearchRemote } from './research/in-memory-research-remote.ts';
export type { ScenarioResearchRemoteScript } from './research/in-memory-research-remote.ts';
```

- [ ] **Step 4: 运行确认通过 + 全包回归**

Run:
```bash
pnpm exec tsx --test packages/mobile-core/src/research/task-research.test.ts && pnpm exec tsx --test packages/mobile-core/src/material/task-material.test.ts packages/mobile-core/src/task-office/task-office.test.ts
```
Expected: research 全 pass；material/task-office 回归 pass。

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/research/ packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): task research deep module with offline annotation drafts (T17 #47 task 6)"
```

---

### Task 7: apps/mobile——ResearchScreen、/tasks/research 路由与 composition 接线

**Files:**
- Create: `apps/mobile/src/research-view.ts`
- Create: `apps/mobile/src/research-view.test.ts`
- Create: `apps/mobile/src/screens/ResearchScreen.tsx`
- Create: `apps/mobile/src/app/tasks/research.tsx`
- Modify: `apps/mobile/src/composition.ts`（追加 research 工厂与导出）
- Modify: `apps/mobile/src/screens/MaterialsScreen.tsx`（props 增可选 `onOpenResearch?` 与一个入口按钮）
- Modify: `apps/mobile/src/app/tasks/materials.tsx`（传入 `onOpenResearch`）

**Interfaces:**
- Consumes: Task 6 的 `createTaskResearch`/`ResearchError`/`TaskResearchHandle`；composition 既有 `cachePut`/`deploymentScopeKey`/`runtime()`/`activeMobileRuntime()`（composition.ts:152-168/:387-395）；`activeTaskMaterial()`（:309-314，annotate 表单的材料索引来源）。
- Produces:
  - composition 导出 `activeTaskResearch(): TaskResearch | undefined`。
  - 路由 `/tasks/research?runId=..`。
  - 控制器 `createResearchController(handle: TaskResearchHandle, input: { runId: string; materials?: () => Promise<MaterialEntry[]> }): ResearchController`，状态 `ResearchViewState { loading; delegations; annotations; materials; error?; notice?; pendingDrafts }`。

- [ ] **Step 1: 写失败测试（控制器 + 路由源级断言）**

创建 `apps/mobile/src/research-view.test.ts`。夹具说明：apps/mobile 拿不到包内私有的 `RuntimeScopeLease` 类（`leaseActive` 以 `instanceof` 判定，普通对象伪造恒 false——见 scope-lease.ts:26-27），而控制器测试的对象是**控制器逻辑**（load 聚合、notice 文案、drafts 计数），因此直接 stub `TaskResearchHandle`（结构化对象即可，不经 `createTaskResearch().open`），不经任何伪造 lease：

```ts
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';
import type {
  ResearchAnnotationDraft, ResearchAnnotationRow, ResearchDelegationRow, ResearchEvent, TaskResearchHandle,
} from '@weknora/mobile-core';
import { createResearchController, type ResearchViewState } from './research-view.ts';

const here = dirname(fileURLToPath(import.meta.url));

const delegations: ResearchDelegationRow[] = [
  { delegationId: 'd-scenario-1', runId: 'r-scenario', sessionId: 's-scenario', objective: 'survey baselines', sources: ['kb-1'], status: 'assigned', createdAt: '2026-09-26T00:00:00Z' },
];
const annotations: ResearchAnnotationRow[] = [
  { annotationId: 'an1', runId: 'r-scenario', materialId: 'm1:0', baseVersion: '9a2f1c3d4e5f6a7b', body: 'seed', authorId: 'u3', createdAt: '2026-09-26T00:00:00Z' },
];

interface StubHandle extends TaskResearchHandle {
  annotateCalls: Array<{ materialId: string; baseVersion: string; body: string }>;
  closeReason?: string;
}

/** 结构化 stub：offline=true 时 annotate 返回 drafted 并记账 drafts（模拟离线草稿路径）。 */
function stubHandle(options: { offline?: boolean } = {}): StubHandle {
  const drafts: ResearchAnnotationDraft[] = [];
  let draftSeq = 0;
  const handle: StubHandle = {
    annotateCalls: [],
    closeReason: undefined,
    async delegate(input) { return delegations[0]!; },
    async delegations() { return delegations; },
    async complete(input) { return { ...delegations[0]!, status: 'completed' as const, summary: input.summary }; },
    async annotate(input) {
      handle.annotateCalls.push(input);
      if (options.offline === true) {
        draftSeq += 1;
        const draft: ResearchAnnotationDraft = { draftId: `research-ann-${draftSeq}`, runId: 'r-scenario', materialId: input.materialId, baseVersion: input.baseVersion, body: input.body, draftedAt: new Date().toISOString() };
        drafts.push(draft);
        return { status: 'drafted', draftId: draft.draftId };
      }
      return {
        status: 'recorded',
        annotation: { annotationId: `an-live-${handle.annotateCalls.length}`, runId: 'r-scenario', materialId: input.materialId, baseVersion: input.baseVersion, body: input.body, authorId: 'u1', createdAt: '2026-09-26T00:00:00Z' },
      };
    },
    async annotations() { return annotations; },
    async requestRevision(input) { return { intent: 'revision-request', outcome: 'accepted', action: input.action, at: '2026-09-26T00:00:00Z' }; },
    async flushAnnotationDrafts() {
      const flushed = drafts.splice(0);
      return flushed.map((draft) => ({ draftId: draft.draftId, outcome: 'recorded' as const }));
    },
    async pendingDrafts() { return [...drafts]; },
    subscribe() { return () => undefined; },
    close(reason: string) { handle.closeReason = reason; },
  };
  return handle;
}

test('controller loads delegations, annotations and pending drafts', async () => {
  const controller = createResearchController(stubHandle(), { runId: 'r-scenario' });
  const states: ResearchViewState[] = [];
  const unsubscribe = controller.subscribe((state) => states.push(state));
  await controller.load();
  unsubscribe();
  const settled = controller.state();
  assert.equal(settled.loading, false);
  assert.equal(settled.delegations?.length, 1);
  assert.equal(settled.delegations?.[0]?.objective, 'survey baselines');
  assert.equal(settled.annotations?.length, 1);
  assert.equal(settled.pendingDrafts, 0);
  controller.dispose();
});

test('controller annotate records online and drafts offline with honest copy', async () => {
  const onlineController = createResearchController(stubHandle(), { runId: 'r-scenario' });
  await onlineController.annotate({ materialId: 'm1:0', baseVersion: 'abcdef0123456789', body: '好' });
  assert.match(onlineController.state().notice ?? '', /已记录/);

  const offlineController = createResearchController(stubHandle({ offline: true }), { runId: 'r-scenario' });
  await offlineController.annotate({ materialId: 'm1:0', baseVersion: 'abcdef0123456789', body: '离线批注' });
  assert.equal(offlineController.state().pendingDrafts, 1);
  assert.match(offlineController.state().notice ?? '', /加密草稿/);
  await offlineController.flushDrafts();
  assert.equal(offlineController.state().pendingDrafts, 0);
  assert.match(offlineController.state().notice ?? '', /同步/);
  offlineController.dispose();
});

test('research route and composition wiring stay at the Interface boundary', async () => {
  // 源级断言：Screen 不导入 wire 层；路由只经 composition 取模块。
  const route = readFileSync(join(here, 'app/tasks/research.tsx'), 'utf8');
  assert.ok(!route.includes('@weknora/api-client'), '路由禁止直接导入 api-client（module-seams §10）');
  assert.ok(route.includes('activeTaskResearch'), '路由必须经 composition 工厂取模块');
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.ok(composition.includes('createTaskResearch'), 'composition 必须装配 research 深模块');
  assert.ok(composition.includes('activeTaskResearch'), 'composition 必须导出 activeTaskResearch');
  const screen = readFileSync(join(here, 'screens/ResearchScreen.tsx'), 'utf8');
  assert.ok(!screen.includes('@weknora/contracts') && !screen.includes('@weknora/api-client'), 'Screen 禁止导入 contracts/api-client');
});
```

- [ ] **Step 2: 运行确认失败**

Run:
```bash
pnpm --filter @weknora/mobile exec tsx --test src/research-view.test.ts
```
Expected: FAIL（`./research-view.ts` 不存在）。

- [ ] **Step 3: 最小实现**

创建 `apps/mobile/src/research-view.ts`：

```ts
import { ResearchError } from '@weknora/mobile-core';
import type {
  MaterialEntry, ResearchAnnotationRow, ResearchDelegationRow, ResearchErrorCode, TaskResearchHandle,
} from '@weknora/mobile-core';

export interface ResearchViewState {
  loading: boolean;
  delegations?: ResearchDelegationRow[];
  annotations?: ResearchAnnotationRow[];
  materials?: MaterialEntry[];
  pendingDrafts?: number;
  error?: string;
  notice?: string;
}

/** ResearchError 错误码 → 用户文案（键类型=ResearchErrorCode，新码缺文案即类型错）。 */
export const RESEARCH_ERROR_COPY: Record<ResearchErrorCode, string> = {
  RESEARCH_SCOPE_CHANGED: '登录状态或活动空间已变化，请重新进入。',
  RESEARCH_INVALID_INPUT: '研究目标或来源填写不完整。',
  RESEARCH_NOT_FOUND: '该研究委派或批注已不存在，请刷新。',
  RESEARCH_CONFLICT: '材料版本已更新，批注未落库；请重新打开该版本后再批注。',
  RESEARCH_COMMAND_UNAVAILABLE: '此部署暂不支持修订请求通道。',
  RESEARCH_COMMAND_CONFLICT: '任务状态已变化，修订请求被拒绝；请刷新任务详情后重试。',
  RESEARCH_DRAFT_UNAVAILABLE: '当前无法安全保存离线批注草稿，请联网后再批注。',
  RESEARCH_BACKEND: '服务端暂时不可用，请稍后重试。',
};

const messageOf = (failure: unknown): string => {
  if (failure instanceof ResearchError) return RESEARCH_ERROR_COPY[failure.code] ?? failure.code;
  return failure instanceof Error ? failure.message : String(failure);
};

export interface ResearchController {
  state(): ResearchViewState;
  subscribe(listener: (state: ResearchViewState) => void): () => void;
  load(): Promise<void>;
  delegate(input: { objective: string; sources: string }): Promise<void>;
  annotate(input: { materialId: string; baseVersion: string; body: string }): Promise<void>;
  flushDrafts(): Promise<void>;
  requestRevision(input: { materialId: string; baseVersion: string; note: string; action: 'steer' | 'queue_next'; expectedRevision: number }): Promise<void>;
  dispose(): void;
}

/** 研究页控制器：load 驱动委派/批注/材料三投影，annotate 走句柄（含离线草稿），
 *  requestRevision 透传修订纪律参数；dispose 关闭句柄。 */
export function createResearchController(
  handle: TaskResearchHandle,
  input: { runId: string; materials?: () => Promise<MaterialEntry[]> },
): ResearchController {
  let state: ResearchViewState = { loading: true };
  let disposed = false;
  const listeners = new Set<(state: ResearchViewState) => void>();
  const publish = (next: Partial<ResearchViewState>) => {
    state = { ...state, ...next };
    for (const listener of listeners) listener(state);
  };
  return {
    state: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    async load() {
      if (disposed) return;
      publish({ loading: true, error: undefined });
      try {
        const [delegations, annotations, pending] = await Promise.all([
          handle.delegations(input.runId),
          handle.annotations(input.runId),
          handle.pendingDrafts().then((drafts) => drafts.filter((draft) => draft.runId === input.runId).length).catch(() => 0),
        ]);
        const materials = input.materials === undefined ? undefined : await input.materials().catch(() => undefined);
        publish({ loading: false, delegations, annotations, ...(materials === undefined ? {} : { materials }), pendingDrafts: pending });
      } catch (failure) {
        publish({ loading: false, error: messageOf(failure) });
      }
    },
    async delegate({ objective, sources }) {
      if (disposed) return;
      const list = sources.split(/[\s,，、]+/).map((source) => source.trim()).filter((source) => source !== '');
      try {
        await handle.delegate({ runId: input.runId, objective, sources: list });
        publish({ notice: '只读研究已委派。' });
        await this.load();
      } catch (failure) {
        publish({ error: messageOf(failure) });
      }
    },
    async annotate({ materialId, baseVersion, body }) {
      if (disposed) return;
      try {
        const receipt = await handle.annotate({ runId: input.runId, materialId, baseVersion, body });
        publish({ notice: receipt.status === 'drafted' ? '当前离线：批注已存为加密草稿，联网后点「同步批注」提交。' : '批注已记录（生成新批注记录，原版本保持不变）。' });
        await this.load();
      } catch (failure) {
        publish({ error: messageOf(failure) });
      }
    },
    async flushDrafts() {
      if (disposed) return;
      try {
        const outcomes = await handle.flushAnnotationDrafts({ runId: input.runId });
        const conflicts = outcomes.filter((outcome) => outcome.outcome === 'conflict').length;
        publish({ notice: conflicts > 0 ? `批注同步完成，${conflicts} 条因版本更新需重读后重提。` : '批注同步完成。' });
        await this.load();
      } catch (failure) {
        publish({ error: messageOf(failure) });
      }
    },
    async requestRevision({ materialId, baseVersion, note, action, expectedRevision }) {
      if (disposed) return;
      try {
        const receipt = await handle.requestRevision({ runId: input.runId, materialId, baseVersion, note, action, expectedRevision });
        publish({ notice: receipt.outcome === 'accepted' ? '修订请求已提交（将生成新版本，已批注版本保持不变）。' : '修订请求被拒绝：任务状态已变化。' });
      } catch (failure) {
        publish({ error: messageOf(failure) });
      }
    },
    dispose() {
      disposed = true;
      handle.close('research-route-unmount');
    },
  };
}
```

创建 `apps/mobile/src/screens/ResearchScreen.tsx`：

```tsx
import { createElement, useState } from 'react';
import { Text, TextInput, View } from 'react-native';
import type { MaterialEntry, ResearchAnnotationRow, ResearchDelegationRow } from '@weknora/mobile-core';
import type { ResearchViewState } from '../research-view.ts';

export interface ResearchScreenProps {
  state: ResearchViewState;
  onDelegate(objective: string, sources: string): void;
  onAnnotate(input: { materialId: string; baseVersion: string; body: string }): void;
  onFlushDrafts(): void;
  onRequestRevision(input: { materialId: string; baseVersion: string; note: string; action: 'steer' | 'queue_next'; expectedRevision: number }): void;
  onRefresh(): void;
  onBack(): void;
}

/** 研究屏（演示态）：委派表单 + 委派/批注投影 + 版本钉定批注表单 + 修订请求表单。
 *  只消费 TaskResearch Interface；wire/契约/scope 纪律全部在模块后。 */
export function ResearchScreen({ state, onDelegate, onAnnotate, onFlushDrafts, onRequestRevision, onRefresh, onBack }: ResearchScreenProps) {
  const [objective, setObjective] = useState('');
  const [sources, setSources] = useState('');
  const [materialId, setMaterialId] = useState('');
  const [baseVersion, setBaseVersion] = useState('');
  const [body, setBody] = useState('');
  const [note, setNote] = useState('');
  const [expectedRevision, setExpectedRevision] = useState('0');
  const [action, setAction] = useState<'steer' | 'queue_next'>('queue_next');
  const materials: MaterialEntry[] = state.materials ?? [];
  return createElement(
    View,
    { style: { padding: 16, gap: 12 } },
    createElement(Text, { style: { fontSize: 20, fontWeight: '600' } }, '研究与批注'),
    createElement(Text, { onPress: onBack }, '← 返回'),
    createElement(Text, { onPress: onRefresh }, '刷新'),
    state.loading === true && createElement(Text, null, '加载中…'),
    state.error !== undefined && createElement(Text, { testID: 'research-error' }, state.error),
    state.notice !== undefined && createElement(Text, { testID: 'research-notice' }, state.notice),
    (state.pendingDrafts ?? 0) > 0 && createElement(Text, { onPress: onFlushDrafts, testID: 'research-flush' }, `离线批注草稿 ${state.pendingDrafts} 条 · 点此同步`),
    createElement(Text, { style: { fontWeight: '600' } }, '委派只读研究'),
    createElement(TextInput, { placeholder: '研究目标', value: objective, onChangeText: setObjective, testID: 'research-objective' }),
    createElement(TextInput, { placeholder: '来源知识库（逗号分隔）', value: sources, onChangeText: setSources, testID: 'research-sources' }),
    createElement(Text, { onPress: () => onDelegate(objective, sources), testID: 'research-delegate' }, '委派（只读，不占用写运行）'),
    (state.delegations ?? []).map((delegation) =>
      createElement(Text, { key: delegation.delegationId }, `${delegation.status === 'completed' ? '已完成' : '进行中'} · ${delegation.objective} · 来源 ${delegation.sources.join('、')}${delegation.summary === undefined ? '' : ' · ' + delegation.summary}`),
    ),
    createElement(Text, { style: { fontWeight: '600' } }, '批注材料版本（生成新批注记录，原版本不变）'),
    materials.map((entry) =>
      createElement(Text, { key: entry.materialId, onPress: () => { setMaterialId(entry.materialId); setBaseVersion(entry.version); } }, `${entry.name} · 版本 ${entry.version}`),
    ),
    createElement(TextInput, { placeholder: '材料（如 m1:0）', value: materialId, onChangeText: setMaterialId, testID: 'research-material' }),
    createElement(TextInput, { placeholder: '当前版本（从材料列表点选）', value: baseVersion, onChangeText: setBaseVersion, testID: 'research-version' }),
    createElement(TextInput, { placeholder: '批注内容', value: body, onChangeText: setBody, testID: 'research-body', multiline: true }),
    createElement(Text, { onPress: () => onAnnotate({ materialId, baseVersion, body }), testID: 'research-annotate' }, '提交批注'),
    (state.annotations ?? []).map((annotation) =>
      createElement(Text, { key: annotation.annotationId }, `${annotation.materialId} @ ${annotation.baseVersion} · ${annotation.body}`),
    ),
    createElement(Text, { style: { fontWeight: '600' } }, '请求修订（基于指定版本生成新版本）'),
    createElement(TextInput, { placeholder: '修订说明', value: note, onChangeText: setNote, testID: 'research-revision-note' }),
    createElement(TextInput, { placeholder: '任务当前 revision（详情页可见）', value: expectedRevision, onChangeText: setExpectedRevision, testID: 'research-revision-num', inputMode: 'numeric' }),
    createElement(Text, { onPress: () => setAction(action === 'steer' ? 'queue_next' : 'steer') }, `通道：${action === 'steer' ? '注入当前运行' : '排队下一次运行'}`),
    createElement(
      Text,
      {
        onPress: () => onRequestRevision({ materialId, baseVersion, note, action, expectedRevision: Number(expectedRevision) || 0 }),
        testID: 'research-revision',
      },
      '请求修订',
    ),
  );
}
```

创建 `apps/mobile/src/app/tasks/research.tsx`：

```tsx
import { useEffect, useState } from 'react';
import { router, useLocalSearchParams } from 'expo-router';
import { activeMobileRuntime, activeTaskMaterial, activeTaskResearch } from '../../composition.ts';
import { createResearchController, type ResearchController, type ResearchViewState } from '../../research-view.ts';
import { ResearchScreen } from '../../screens/ResearchScreen.tsx';

/** /tasks/research 挂载生命周期宿主：handle 在 effect 内开、卸载即 close——与
 *  /tasks/materials 同一模式。材料索引用于批注表单的版本身份点选。 */
export function ResearchRouteLifecycle({ runId }: { runId: string }) {
  const [state, setState] = useState<ResearchViewState>({ loading: true });
  const [controller, setController] = useState<ResearchController | undefined>(undefined);
  useEffect(() => {
    const runtime = activeMobileRuntime();
    const research = activeTaskResearch();
    const material = activeTaskMaterial();
    const lease = runtime.scopeLease();
    if (!research || !lease || runId.trim() === '') {
      setState({ loading: false, error: '请先登录并激活空间，再查看研究面。' });
      return;
    }
    let next: ResearchController | undefined;
    try {
      next = createResearchController(research.open({ lease }), {
        runId,
        materials: async () => {
          if (material === undefined) return [];
          return (await material.open({ lease }).index({ runId })).materials;
        },
      });
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
  return createElementScreen(controller, state, runId);
}

function createElementScreen(controller: ResearchController | undefined, state: ResearchViewState, runId: string) {
  return (
    <ResearchScreen
      state={state}
      onDelegate={(objective, sources) => { void controller?.delegate({ objective, sources }); }}
      onAnnotate={(input) => { void controller?.annotate(input); }}
      onFlushDrafts={() => { void controller?.flushDrafts(); }}
      onRequestRevision={(input) => { void controller?.requestRevision(input); }}
      onRefresh={() => { void controller?.load(); }}
      onBack={() => router.back()}
    />
  );
}

/** Expo Router 文件路由：/tasks/research?runId=..。只消费 TaskResearch Interface。 */
export default function TaskResearchRoute() {
  const params = useLocalSearchParams<{ runId?: string }>();
  return <ResearchRouteLifecycle runId={String(params.runId ?? '')} />;
}
```
（实现注意：`createElementScreen` 使用了 JSX，因此该文件若保留 `.tsx` 需确保 JSX 转换可用——同目录 `materials.tsx` 已用 JSX，安全。`createElementScreen` 若实现时觉得绕，可直接内联进两个组件，但必须保持「路由默认导出 + Lifecycle 宿主」双导出与测试断言一致。）

修改 `apps/mobile/src/composition.ts`（两处最小改动，均已实跑验证）——

① import 区末尾（`foreground-sync` 导入行之后）追加：

```ts
import { createTaskResearch } from '@weknora/mobile-core';
import type { ResearchAnnotationDraft, ResearchDraftsPort, TaskResearch } from '@weknora/mobile-core';
import { createMobileResearchRemote } from '@weknora/api-client/mobile/research';
```

② 文件末尾（`completeNativeOidcCallback` 之后）追加工厂与导出：

```ts
const taskResearches = new Map<string, TaskResearch>();

/** Task Research 按 deployment scope key 记忆化（同 taskMaterialFor 模式）；
 *  离线批注草稿经 Scoped Vault drafts 命名空间加密保存（无 vault 时模块内 fail closed）。 */
function taskResearchFor(activeRuntime: MobileRuntime, origin: string, tenantId: string): TaskResearch {
  return cachePut(taskResearches, deploymentScopeKey(origin, tenantId), () => {
    const remote = createMobileResearchRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) });
    const draftsPort: ResearchDraftsPort | undefined = nativeScopedVault === undefined ? undefined : {
      async put(draft) {
        const store = await openScopedDraftStore();
        if (store === undefined) throw new Error('RESEARCH_DRAFT_UNAVAILABLE');
        await store.drafts.put({ id: draft.draftId, body: JSON.stringify(draft) });
      },
      async list() {
        const store = await openScopedDraftStore();
        if (store === undefined) return [];
        return (await store.drafts.list())
          .map((entry) => { try { return JSON.parse(entry.body) as ResearchAnnotationDraft; } catch { return undefined; } })
          .filter((draft): draft is ResearchAnnotationDraft => draft !== undefined && typeof draft.draftId === 'string');
      },
      async remove(draftId) {
        const store = await openScopedDraftStore();
        if (store === undefined) return;
        await store.drafts.remove(draftId);
      },
    };
    return createTaskResearch({
      remote,
      gate: nativeOfflineGate,
      ...(draftsPort === undefined ? {} : { drafts: draftsPort }),
    });
  });
}

/** /tasks/research 路由经此取当前授权 scope 的研究模块（无授权面返回 undefined）。 */
export function activeTaskResearch(): TaskResearch | undefined {
  const activeRuntime = runtime();
  const snapshot = activeRuntime.snapshot();
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return undefined;
  return taskResearchFor(activeRuntime, snapshot.deployment.origin, snapshot.identity.activeTenantId ?? '');
}
```

（注意：端口不持有 `lease()`——lease 的唯一来源是路由处的 `research.open({ lease })`，与 material 模块同款；草稿 id `research-ann-<n>` 符合 vault 的 `DRAFT_ID_PATTERN` 白名单 `^[A-Za-z0-9._-]{1,64}$`（scoped-vault.ts:7），JSON.parse 失败的行读作空并跳过。）

修改 `apps/mobile/src/screens/MaterialsScreen.tsx`——props 增加可选入口（最小 diff）：
```tsx
export interface MaterialsScreenProps {
  // ……既有字段不动……
  onOpenResearch?(): void;
}
```
在返回树的「刷新」操作附近追加一行（仅当 `onOpenResearch` 存在）：
```tsx
  onOpenResearch !== undefined && createElement(Text, { onPress: onOpenResearch, testID: 'materials-open-research' }, '研究与批注 →'),
```

修改 `apps/mobile/src/app/tasks/materials.tsx`——`MaterialsRouteLifecycle` 的 `MaterialsScreen` 调用处追加：
```tsx
      onOpenResearch={runId.trim() === '' ? undefined : () => router.push(`/tasks/research?runId=${encodeURIComponent(runId)}`)}
```

- [ ] **Step 4: 运行确认通过 + typecheck**

Run:
```bash
pnpm --filter @weknora/mobile exec tsx --test src/research-view.test.ts && pnpm --filter @weknora/mobile typecheck && pnpm --filter @weknora/mobile exec tsx --test src/materials-view.test.ts
```
Expected: research-view 全 pass；typecheck 无输出（通过）；materials-view 回归 pass。

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/research-view.ts apps/mobile/src/research-view.test.ts apps/mobile/src/screens/ResearchScreen.tsx \
        apps/mobile/src/app/tasks/research.tsx apps/mobile/src/composition.ts \
        apps/mobile/src/screens/MaterialsScreen.tsx apps/mobile/src/app/tasks/materials.tsx
git commit -m "feat(mobile): research and annotation screen with /tasks/research route (T17 #47 task 7)"
```

---

### Task 8: apps/mobile——opt-in 真实集成证据（AC3）

**Files:**
- Create: `apps/mobile/src/research-integration-smoke.ts`
- Create: `apps/mobile/src/research-integration-smoke.test.ts`

**Interfaces:**
- Consumes: `material-integration-smoke.ts` 的 runtime 引导与 `disallowedDeploymentHost` 防线（`apps/mobile/src/runtime-integration-smoke.ts`）；Task 5 的 `createMobileResearchRemote`、Task 5/6 的 material remote（取真实材料的版本身份）。
- Produces:
```ts
export type ResearchIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };
export interface ResearchIntegrationEvidence {
  deploymentOrigin: string;
  listed: 'delegated' | 'no-tasks' | 'no-materials' | 'failed';
  delegationCreated?: boolean;
  delegationCount?: number;
  annotated?: 'recorded' | 'skipped-no-materials' | 'failed';
  annotationCount?: number;
  annotatedVersion?: string;
  revision: 'not-dispatched';
  failure?: string;
  commandTimestamp: string;
}
export function researchIntegrationConfig(env: Record<string, string | undefined>): ResearchIntegrationConfig;
export async function runResearchIntegration(config: Extract<ResearchIntegrationConfig, { enabled: true }>): Promise<ResearchIntegrationEvidence>;
export function emitResearchIntegrationEvidence(evidence: ResearchIntegrationEvidence, emit: (record: string) => void): void;
```
  修订请求在集成证据中恒为 `'not-dispatched'`（对真实部署不触发新 Run——写副作用只保留 append-only 的委派与批注记录，如实声明）。

- [ ] **Step 1: 写失败测试（证据契约 + config 门控）**

创建 `apps/mobile/src/research-integration-smoke.test.ts`：

```ts
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { emitResearchIntegrationEvidence, researchIntegrationConfig, type ResearchIntegrationEvidence } from './research-integration-smoke.ts';

test('researchIntegrationConfig skips without credentials and rejects non-public origins', () => {
  assert.deepEqual(researchIntegrationConfig({}), { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' });
  assert.equal(researchIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://localhost:3000',
    WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c', WEKNORA_MOBILE_TEST_PASSWORD: 'pw',
  }).enabled, false, '非公网 HTTPS origin 必须拒绝（主机防线）');
});

test('runResearchIntegration without credentials reports an honest skip shape', async () => {
  const config = researchIntegrationConfig({});
  assert.equal(config.enabled, false);
});

test('evidence records never contain credential material', () => {
  const evidence: ResearchIntegrationEvidence = {
    deploymentOrigin: 'https://weknora.example.com',
    listed: 'delegated', delegationCreated: true, delegationCount: 1,
    annotated: 'recorded', annotationCount: 1, annotatedVersion: '9a2f1c3d4e5f6a7b',
    revision: 'not-dispatched', commandTimestamp: '2026-09-26T00:00:00.000Z',
  };
  let emitted = '';
  emitResearchIntegrationEvidence(evidence, (record) => { emitted += record + '\n'; });
  assert.ok(emitted.includes('"listed":"delegated"'));
  assert.ok(!emitted.includes('password'), '证据不得携带凭据字段');
  assert.ok(emitted.includes('"revision":"not-dispatched"'), '修订请求不派发必须如实记录');
});
```

- [ ] **Step 2: 运行确认失败**

Run:
```bash
pnpm --filter @weknora/mobile exec tsx --test src/research-integration-smoke.test.ts
```
Expected: FAIL（模块不存在）。

- [ ] **Step 3: 最小实现**

创建 `apps/mobile/src/research-integration-smoke.ts`（runtime 引导段与授权通道接线以 `apps/mobile/src/material-integration-smoke.ts:55-90` 为权威样例逐字同构；作者已实读该文件）：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileMaterialRemote } from '@weknora/api-client/mobile/materials';
import { createMobileResearchRemote } from '@weknora/api-client/mobile/research';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice } from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type ResearchIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface ResearchIntegrationEvidence {
  deploymentOrigin: string;
  listed: 'delegated' | 'no-tasks' | 'no-materials' | 'failed';
  delegationCreated?: boolean;
  delegationCount?: number;
  annotated?: 'recorded' | 'skipped-no-materials' | 'failed';
  annotationCount?: number;
  annotatedVersion?: string;
  revision: 'not-dispatched';
  failure?: string;
  commandTimestamp: string;
}

/** 与 T16 material 证据相同的 opt-in 语义（自包含，不跨计划 import 凭据逻辑）。 */
export function researchIntegrationConfig(env: Record<string, string | undefined>): ResearchIntegrationConfig {
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
 * 真实 JSON transport + 授权通道 + 具体 Remote Adapter + research 编排。
 * 副作用声明（如实，不伪装）：探测到的首个任务上留下 1 条只读研究委派与
 * 1 条版本钉定批注（append-only，无删除端点是设计事实）；修订请求不派发。
 * 委派源 'probe-kb' 多半被租户范围围栏以 400 拒绝——这本身是 AC1 的真实证据：
 * 记 delegationCreated:false + failure 前缀 delegation-rejected-by-scope-fence。
 * 任何步骤异常 → listed:'failed' + failure 摘要（无凭据字段），从不 reject。
 */
export async function runResearchIntegration(config: Extract<ResearchIntegrationConfig, { enabled: true }>): Promise<ResearchIntegrationEvidence> {
  const evidence: ResearchIntegrationEvidence = { deploymentOrigin: config.deploymentOrigin, listed: 'failed', revision: 'not-dispatched', commandTimestamp: new Date().toISOString() };
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
    if (snapshot.surface !== 'authorized' || !snapshot.deployment) { evidence.failure = 'sign-in did not reach the authorized surface'; return evidence; }
    const origin = config.deploymentOrigin;
    const office = createTaskOffice({
      backend: createTaskOfficeRemote({ origin, request: (input) => runtime.authorizedRequest(input) }),
      lease: () => runtime.scopeLease(),
    });
    const page = await office.tasks({});
    if (page.items.length === 0) { evidence.listed = 'no-tasks'; return evidence; }
    const runId = page.items[0]!.runId;

    const research = createMobileResearchRemote({ origin, request: (input) => runtime.authorizedRequest(input) });
    const material = createMobileMaterialRemote({ origin, request: (input) => runtime.authorizedRequest(input) });

    let delegated = true;
    try {
      await research.delegate({ runId, objective: 'integration probe: verify read-only research delegation', sources: ['probe-kb'] });
    } catch (failure) {
      delegated = false;
      evidence.failure = `delegation-rejected-by-scope-fence:${failure instanceof Error ? failure.message.slice(0, 120) : 'unknown'}`;
    }
    evidence.delegationCreated = delegated;
    const delegations = await research.list(runId);
    evidence.listed = 'delegated';
    evidence.delegationCount = delegations.delegations.length;
    if (!delegated) return evidence;

    const index = await material.list(runId);
    const first = index.artifacts[0];
    if (first === undefined) { evidence.annotated = 'skipped-no-materials'; return evidence; }
    await research.annotate({ runId, materialId: first.id, baseVersion: first.version, body: 'integration probe annotation (version-pinned)' });
    const annotations = await research.annotations(runId);
    evidence.annotated = 'recorded';
    evidence.annotationCount = annotations.annotations.length;
    evidence.annotatedVersion = first.version;
    return evidence;
  } catch (error) {
    evidence.listed = 'failed';
    evidence.failure = error instanceof Error ? error.message.slice(0, 300) : String(error).slice(0, 300);
    return evidence;
  } finally {
    runtime.dispose();
  }
}

/** 证据契约输出（JSONL）：无凭据字段；revision 恒 not-dispatched。 */
export function emitResearchIntegrationEvidence(evidence: ResearchIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify({ kind: 'research-integration', ...evidence }));
}
```

- [ ] **Step 4: 运行确认通过 + typecheck + 全应用回归**

Run:
```bash
pnpm --filter @weknora/mobile exec tsx --test src/research-integration-smoke.test.ts && pnpm --filter @weknora/mobile typecheck && pnpm --filter @weknora/mobile test
```
Expected: 证据契约 pass；typecheck 通过；`pnpm --filter @weknora/mobile test` 全量 pass（含既有 app-smoke）。有真实环境时另加 opt-in 实跑（缺环境如实 skip，不伪造）：

```bash
WEKNORA_MOBILE_TEST_DEPLOYMENT_URL=… WEKNORA_MOBILE_TEST_EMAIL=… WEKNORA_MOBILE_TEST_PASSWORD=… \
  pnpm --filter @weknora/mobile exec tsx --test src/research-integration-smoke.test.ts
```

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/research-integration-smoke.ts apps/mobile/src/research-integration-smoke.test.ts
git commit -m "test(mobile): research integration evidence contract with opt-in real HTTP (T17 #47 task 8)"
```

---

## 计划级验证命令

在 worktree 根（`.worktrees/issue30-sweep`）一条串行执行（覆盖本计划全部测试与关键回归；Go 全量套件排除在外以避开无关 flaky）：

```bash
go test ./internal/database/ -run 'TestSQLiteMigrationsCreateVersionedSchema|TestSQLiteMigrationsUpgradeV4PreservesData|TestSQLiteMigrationsIncludeAutoTagConfig|TestWorkbenchSQLite' -count=1 \
&& go test ./internal/application/repository/ -run 'TestTaskResearchStore|TestTaskAnnotationStore|TestTaskResearchEndToEnd|TestTaskCollaboration|TestTaskGrant|TestWorkbenchArtifacts' -count=1 \
&& go test ./internal/handler/session/ -run 'TestDelegateResearch|TestListResearch|TestCompleteResearch|TestAnnotateMaterial|TestListAnnotations|TestListWorkbenchArtifacts|TestCreateWorkbenchArtifactSignedURL' -count=1 \
&& go build ./... \
&& pnpm exec tsx --test packages/contracts/src/mobile/research.test.ts packages/api-client/src/mobile/research.test.ts packages/mobile-core/src/research/task-research.test.ts \
&& pnpm --filter @weknora/mobile exec tsx --test src/research-view.test.ts src/research-integration-smoke.test.ts \
&& pnpm --filter @weknora/mobile typecheck \
&& pnpm --filter @weknora/mobile exec tsx --test src/materials-view.test.ts
```

## 自我审查记录（writing-plans 四项检查）

1. **Spec 覆盖**：AC1（只读子任务不能扩大 Task Grant 或产生冲突写入）→ Task 2 源围栏 400 + handler 无写原语结构保证 + Task 3 E2E 三断言（并行只读 vs 写槽互斥对照、跨租户源拒绝、grants 面不被协作者放宽）；AC2（批注/修改生成新版本，已审批版本保持不变）→ Task 2 base_version 钉定 409 + Task 3 E2E（新消息=新版本身份、v1 的 version/digest 恒定、批注仍解析 v1）+ Task 6（修订请求组合钉定文本、离线批注草稿）；AC3（最高稳定 Interface 端到端）→ Task 6 mobile-core Interface 场景 + Task 3 Go 真实迁移 E2E + Task 8 opt-in 真实 HTTP 证据（blocked-env 如实 skip）。Story 28（least-privilege parallel research）→ 委派源围栏 + 只读并行。Implementation Decisions 的单写者/不可变锚/离线批注三条逐字落入 Global Constraints 并各有测试。无缺口。
2. **占位符扫描**：全文无 TBD/TODO/“实现细节略”/“类似 Task N”（关键词扫描仅命中本条自述）；每个代码步骤给出完整代码；唯一一处「实现注意」（Task 7 路由 JSX 形态提示）不涉及任何未定语义；全部代码块均为完整实现，且 Go 侧（Task 0–3）与 TS 侧（Task 4–7）都已从计划文本逐字提取、真实落盘实跑验证（见第二轮审查修复记录末尾的证据段）。
3. **类型/签名一致性**：`ResearchStore`/`AnnotationStore`（Task 2 handler 接口）与 Task 1 store 方法逐字一致；wire 字段 `delegation_id/annotation_id/...`（Task 2 JSON tag）与 Task 4 contracts 键逐字一致；Task 5 `ResearchRemote` 行名（`delegationId/baseVersion/...`）与 Task 6 `ResearchBackendPort` 逐字一致；Task 6 错误码 `RESEARCH_*` 与 Task 7 `RESEARCH_ERROR_COPY` 键集逐字一致；`createTaskResearch(ports).open({lease})` 与 Task 7 composition/路由调用一致；测试中 `RESEARCH_BASE_VERSION_CONFLICT`（api-client 抛）与 mobile-core 的 `messageOf` 子串匹配一致。
4. **Review Focus 落实**：五条失效模式各自的测试已在「Review Focus」逐行标注归属任务（Task 2/3/6），且测试代码已写入对应任务步骤。

## 第二轮独立审查修复记录（已全部修复并实跑验证）

独立审查发现的问题与修复（每条含证据）：

1. **【阻断】伪造 ScopeLease 过不了 `instanceof`**：`leaseActive` 实现为 `lease instanceof RuntimeScopeLease && lease.active`（`packages/mobile-core/src/runtime/scope-lease.ts:26-27`），且 `RuntimeScopeLease` 不入公共导出（index.ts 无导出，rg 实证）。已修：Task 6 测试夹具改为 `new RuntimeScopeLease({ deploymentOrigin, userId, tenantId })`（同包 import `'../runtime/scope-lease.ts'`，同 task-office.test.ts:15 先例；撤销用 `.revoke()`）；Task 7（apps/mobile 跨包拿不到该类）改为**直接 stub `TaskResearchHandle` 结构化对象**（不经 `createTaskResearch().open`，不伪造 lease）。
2. **【阻断】命令冲突契约码写错**：真实契约码是 `TASK_COMMAND_CONFLICT`（task-detail.ts:86 契约注释/:153-155 透传/:275/:540，api-client task-office.ts:262 的 `coded()` 且错误携带 `.code` 属性），不是 `TASK_OFFICE_COMMAND_CONFLICT`。已修：Task 6 实现改为 `failureCode(failure).includes('TASK_COMMAND_CONFLICT')`（新增 `failureCode` 先读 `Error.code` 属性、回退 message），测试自抛 `codedError('TASK_COMMAND_CONFLICT')`（真实 remote 形态）。
3. **【高】ClientRequest 形态错误**：真实形态是 `{ method, path, headers?, body? }`（client.ts:38-46），materials.ts 全部以相对 `path` 消费。已修：Task 5 默认实现改为 `{ method, path, body }`（无任何 `{url,init}` 残留），测试断言 `requests[0].method`/`requests[0].path`（相对路径），并新增 `unwrap` 信封校验 + `ApiError.status===409 → RESEARCH_BASE_VERSION_CONFLICT`（错误对象携带 `.code`）。
4. **【实跑发现并修复】`.catch(handler)` 返回错误对象会把 promise 变为 resolved**：原 `catch(annotateFailure)` 使 409 映射失效——改为 `rethrow()` 包装（处理器必须重抛）；GET 通路的原始响应改为先过 `unwrap` 信封校验再交给解析器（`getEnvelope`），否则畸形信封漏成解析器错误。两处均由实跑抓出并回写。
5. **【实跑发现并修复】`TaskResearchPorts.lease()` 与 `open({lease})` 双来源歧义**：端口 `lease()` 在实现中从未被使用（open 参数覆盖）。已按 material 模块先例删除端口 `lease()`，lease 唯一来源 = `open({ lease })`；types/测试/composition 接线同步更新。
6. **【实跑发现并修复】路由 `controller.delegate(objective, sources)` 传两参**（模块签名是单对象参数）——改为 `delegate({ objective, sources })`（typecheck 抓出）。
7. **【中】Task 0 完成前依赖 `openTaskGrantDB` 的测试全部以 duplicate migration 失败**：已在 Tech Stack 基线声明与 Task 1 Step 2 写明失败形态判别（编译错误 vs duplicate migration），并撤回「基线全部绿色」的时点性表述（该 ok 只在去重后的窗口成立）。
8. **【中】Task 2 Step 4 的 `||` 回退掩蔽失败**：拆为三条独立串行命令（handler 测试 / go build / go vet），去掉 `||`。
9. **【低】Task 3 Step 2 回归命令的 `TestWorkbenchArtifacts` 分支空匹配**：已删除（工件列表回归由 handler 包 `TestListWorkbenchArtifacts` 覆盖并在文中注明）。
10. **【低】迁移最大号措辞**：改为「波起点 HEAD 最大号 sqlite 000117 / versioned 000196；去重后为 000118/000197；本计划占用 000119/000198」两种形态均如实标注。

**TS 侧实跑证据（修正后代码，作者完成）**：把本计划 Task 4–7 的全部 TS 代码从计划文本逐字提取落盘（contracts 1+1、api-client 1+1、mobile-core 4、apps/mobile 2，共 12 文件）并临时追加两个 barrel 导出与 composition 接线后实跑：`pnpm exec tsx --test packages/contracts/src/mobile/research.test.ts packages/api-client/src/mobile/research.test.ts packages/mobile-core/src/research/task-research.test.ts` → 13/13 pass；`pnpm --filter @weknora/mobile exec tsx --test src/research-view.test.ts` → 3/3 pass；`pnpm --filter @weknora/mobile typecheck` → 0 错误。验证后临时文件全部精确还原（4 个共享文件 `git checkout`、12 个新文件删除），共享树仅遗留本计划文件。

## 验证命令环境锚定（执行者须知）

- worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep`；分支为该 worktree 当前分支；执行前 `git status --short` 确认无本计划之外的迁移文件改动（若见 `000118_mobile_device_app`/`000197_mobile_device_app` 已存在，Task 0 的去重步骤为零改动验证，属预期）。
- Go：1.26；sqlite 测试驱动经 `migrations/sqlite` 全量轨道装载（`openTaskGrantDB` 模式）；`go build ./...` 应在 Task 2 后保持零输出。
- TS：`pnpm install` 已就绪；测试运行器为 `tsx --test`；apps/mobile typecheck 为 `tsc --noEmit`。
- 迁移序号 000119/000198：集成时若被同批计划先占，按「整体顺延、DDL 零变化」处理并同步更新本计划 Task 1 store 测试无需改动（测试经迁移目录装载，不钉文件名）。
