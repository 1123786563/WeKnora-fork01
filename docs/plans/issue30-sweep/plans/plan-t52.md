# T22：GitHub 个人连接到草稿 PR（Issue #52）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Owner 通过个人 GitHub 连接把一个 Run 的云 Workspace 改动从固定基线锚定为候选交付（Diff + 材料摘要 + A03 审批锚点），审批后仅以任务分支推送 + 草稿 PR 交付，全程不写受保护分支、不自动合并，且提交 SHA、审批内容与 PR 回执全部落账并可从 Task/Run 读回。

**Architecture:** 交付是「受审批的外部操作」，因此审批走**既有 A03 Action 管线**（`appconnectorsvc.ActionService`：Prepare 摘要锚定 → 既有 `POST /apps/actions/:id/approve`（#42 审批者谓词：发起者/个人连接所有者/owner-admin）→ Execute 消费批准），新增 `internal/modules/codedelivery` 深模块拥有交付编排：GitHub 客户端端口（git-data API：blobs/trees/commits/refs/pulls，**无 merge 端点**）、Workspace 文件端口（生产由 `*sandbox.SessionBoundManager` 满足、测试用本地目录适配器）、`code_deliveries` 追溯表（task/run/action/commit_sha/pr 回执/实际远端身份）。凭据经 `CredentialResolver` 仅在服务端解析、**永不进入沙箱**（spec 故事 48）。远端推送用 GitHub git-data REST 链（blobs→tree(base_tree=基线)→commit(parent=基线)→refs/heads/<任务分支>→draft PR），而非 shell git push——令牌可留在服务端；本地以 httptest 实现 GitHub API 模拟器（内存对象库 + 真实 HTTP 字节 + 调用计数）+ 真实 sqlite 行为做最高稳定 Interface 验证。移动面按 module-seams §10 只加只读投影：contracts wire 解析 → api-client 授权通道远端 → mobile-core 纯投影/读器 → TaskDetailScreen 交付回执区块。

**Tech Stack:** Go 1.26（gin + gorm + testify，`go test`）、TypeScript（`packages/contracts`、`packages/api-client`、`packages/mobile-core`、`apps/mobile` Expo RN）、node:test + tsx（TS 测试运行器）。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置 `pnpm install` 已就绪。本计划作者在当前 HEAD 实跑基线：`go test ./internal/modules/workbench/ -count=1` ok（3.082s）；`go test ./internal/handler/session/ -run 'TestGetWorkbenchTerminalLog' -count=1` ok；`pnpm --filter @weknora/mobile test` 0 fail（5 skipped）；`pnpm --filter @weknora/mobile typecheck` 通过；`pnpm exec tsx --test packages/contracts/test/mobile-interactions.test.ts` 通过。**预存在失败（非本计划引入，见「差异记录」第 5 条）**：`go test ./internal/application/repository/ -count=1` FAIL（276 个，migrations/sqlite 000112 序号冲突）；`go test ./internal/handler/session/ -count=1` FAIL（27 个，同一原因）——本计划全部 Go 测试因此**不使用全量迁移轨道**，改用自包含 sqlite 夹具（AutoMigrate）+ 单文件迁移 SQL 对齐测试（B3 修复轮已确立的同款先例，`docs/plans/issue30-sweep/ocr/fix-report-increment-b3.md:136`）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-52.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（User Stories 41–48、Implementation Decisions、Testing Decisions——尤其「Developer supports personal and Tenant GitHub/GitLab connections, task branches and draft PR/MR only. It does not merge automatically or expose remote credentials to Shell.」「Lead Agent Version, Artifact versions, Action Plans and candidate code commits are immutable approval anchors. Changes invalidate prior approvals.」「Each command that can have an unknown outcome uses a durable idempotency identity.」）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§3 依赖方向；§10 App Shell 禁止事项「Screen 直接导入 packages/contracts 或 packages/api-client」；§12 依赖分类「Port 放在拥有行为的 Module 一侧」；§13 Interface 测试面）
- ADR：`docs/adr/0008-developer-delivery-and-single-writer.md`（「首版代码交付仅推送任务分支并创建或更新草稿 GitHub PR/GitLab MR，不写受保护分支、不自动合并……记录发起者、批准者和实际远端身份」）、`docs/adr/0003-mobile-office-cloud-execution.md`、`docs/adr/0006-mobile-transport-by-semantics.md`（REST 提交命令/载入权威快照）
- 领域术语：`CONTEXT.md`（「代码平台连接」：个人连接只能由其所有者使用，**每次远端写入记录发起成员、批准成员与实际远端身份**；「代码交付审批」：待交付内容变化后需要重新审批，_避免_ 一次批准解释为后续所有推送的永久授权；「代码交付（Code Delivery）」：_避免_ 自动合并、把补丁生成等同于已远端交付、**在推送与创建 PR/MR 部分成功后盲目重试全部步骤**；「工作区（Workspace）」）
- Parent：Issue #30；Blocked by：#36（T06，已合并）、#38（T08，已合并）、#46（T16，已合并）——三者产出接口见下「Consumes」，全部在当前 HEAD 亲眼核实
- 前序批次产出（本计划 Consumes，全部在当前 HEAD 亲眼核实）：#42 的 `appconnectorsvc.ActionService`（Prepare/Approve/Execute/ResolveUnknown，`internal/modules/appconnector/service/appconnector/action.go:160-318`）、`ActionStoreSource`（`action.go:142-152`）、`A02Guard`/`ocAuthorizer.Check`（`oc_authorizer.go:119-160`，原生个人连接走 CanUseConnection owner-only）、`CredentialResolver`（`credentials.go:21-24`，`MCPOAuthBindingStore.LoadCredential` 返回解密后的 AccessToken 字节，`internal/application/repository/mcp_oauth.go:510-533`）、`POST /apps/actions/:id/approve` 审批者谓词（`internal/handler/app_connector_action.go:211-282`）与 PrepareAction 个人连接谓词（`:168-172`）；#34/#35 的 `MobileRuntime.authorizedRequest` 授权读通道与 `OwnedRunReader`/`resolveOwnedRun`（`internal/handler/session/workbench_read.go:23-25/:181`）；#46 的 Task Material Diff 面（`packages/mobile-core/src/material/diff.ts` 的 `parseUnifiedDiff`——本计划不重复实现 Diff 解析，交付审阅的 Diff 呈现复用 #46 面）；#32–#41 的 composition/route/smoke 范式（`apps/mobile/src/composition.ts:256-278` 记忆化工厂、`apps/mobile/src/app/tasks/detail.tsx` 生命周期宿主、`apps/mobile/src/material-integration-smoke.ts` opt-in 证据范式）。

## Global Constraints

以下为批准 Spec / ADR / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「Developer supports personal and Tenant GitHub/GitLab connections, task branches and draft PR/MR only. **It does not merge automatically or expose remote credentials to Shell.**」（mobile-ai-office-design.md · Implementation Decisions）——本计划：GitHub 令牌只在 Go 服务端 `CredentialResolver` 内解析并直接用于 git-data REST 调用，**从不写入沙箱环境/脚本/工作区**；客户端端口**没有 merge 方法**（结构性不存在自动合并），模拟器对 `PUT /pulls/{n}/merge` 记录违规并 404。
- 「Lead Agent Version, Artifact versions, Action Plans and **candidate code commits are immutable approval anchors. Changes invalidate prior approvals.**」（同上）——交付材料（repo/基线/分支/文件清单/提交信息/PR 标题）经 `ActionService.Prepare` 归一化并摘要，任何改动=新摘要=旧批准失效（digest 不匹配被 `Approve` 拒绝）；dispatch 只发送 Prepare 时落库的归一化字节。
- 「Each command that can have an unknown outcome uses a durable idempotency identity. Network failure triggers lookup or reconciliation, not silent replay.」（同上）——A03 的 `ClaimDispatch`（批准消费与派发意图同事务）+ `unknown` 状态 + `ResolveUnknown` 只查远端事实；**推送成功而 PR 创建失败落账为 `pushed`（部分完成），恢复只补 PR、绝不重推**（CONTEXT.md「代码交付」避免项）。
- 「As a developer, I want to select an authorized GitHub or GitLab repository and fixed baseline, so that code execution starts from a reproducible state.」（User Story 41）——基线必须是 40-hex commit SHA；物化（MaterializeBaseline）把基线树写入该 Run 会话的工作区固定根 `/workspace/<owner>/<name>/`。
- 「不写保护分支、不自动合并。」（Issue #52 验收标准 1 / 任务清单原文）——目标分支必须通过 `weknora/task/` 前缀白名单校验且 ≠ 仓库默认分支且远端未标记 protected（`GET /repos/{o}/{r}/branches/{b}` 的 `protected` 字段）；只创建/更新 `refs/heads/<任务分支>`。
- 「提交 SHA、审批内容和 PR 回执可追溯到 Task/Run。」（Issue #52 验收标准 2 原文）——`code_deliveries` 行携带 task_id/run_id/action_id（审批锚）/digest/approver（join `app_action_approvals.actor`）/commit_sha/pr_number/pr_url/remote_login（`GET /user` 实际远端身份），`GET /workbench/executions/:run_id/delivery` 可读回。
- 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」（Issue #52 验收标准 3 原文）——本地证据链：真实 sqlite 行为 + 真实 A03 store/guard + httptest GitHub 模拟器上的**真实 HTTP 字节**（Task 6 服务编排 / Task 7 HTTP 面）+ 移动端真实 Runtime 装配 opt-in 证据（Task 11）。真实 GitHub 端到端（真实 OAuth 应用 + 真实仓库推送）本地不可得，列为 blocked-env（见下）。
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）——codedelivery Go 模块不 import `internal/modules/execution/sandbox`（端口在模块内、沙箱适配器在 container 包）；mobile-core 不 import contracts/api-client。
- 「禁止：Screen 直接导入 packages/contracts 或 packages/api-client；Screen 自己维护 request_id、cursor、revision、 scope generation」（mobile-module-seams.md §10）——移动交付面经 mobile-core 读器 + composition 工厂注入。
- 「True external dependencies such as APNs, FCM, WebRTC, system audio and system share use mock or scripted Adapters at the Port and real-device acceptance separately.」（同上 · Testing Decisions）——真实 GitHub 属 true external：端口 + httptest 模拟器本地证据 + env 门控的真实 GitHub 客户端测试（缺 env 即 skip，不伪造）。
- 安全约束（会话注入）：服务端 SQL 一律参数绑定（本计划新查询为 gorm `?` 绑定）；发请求前校验 host，仅允许 `https://api.github.com`（生产客户端常量钉住，模拟器走注入 baseURL）；凭据只从环境变量读取（`WEKNORA_APP_OAUTH_GITHUB_CLIENT_ID/_SECRET`、`WEKNORA_GITHUB_TEST_TOKEN`），源码与测试不写入可用凭据字面量；`NormalizeArgs`（`internal/modules/appconnector/action.go:97-120`）保证批准载荷的规范字节即派发字节。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。
- 迁移编号：本计划新迁移取 **versioned 000192 / sqlite 000113**（当前尾部 `migrations/versioned/000191_{task_grants,agent_adoption_variants}`、`migrations/sqlite/000112_{task_grants,agent_adoption_variants}` 双双撞号——见差异记录第 5 条；000191/000112 已被占用，下一可用号为 000192/000113）。若集成时再被同批占用，按 B3 惯例整体顺延为下一个可用号（内容不变），不得挤占他人编号。

**Issue #52 验收标准原文（docs/plans/issue30-sweep/issues/issue-52.md）：**

1. 「不写保护分支、不自动合并。」
2. 「提交 SHA、审批内容和 PR 回执可追溯到 Task/Run。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 3 的本地可验证性说明（blocked-env 声明）：真实端到端需要「一个真实 WeKnora Deployment（已配置 `WEKNORA_APP_OAUTH_GITHUB_CLIENT_ID/_SECRET`）+ 一个真实 GitHub 账号与测试仓库」。本地无此环境时：Task 2 的真实 GitHub 客户端测试与 Task 11 的移动集成证据中真实交付段以 skip 收场（**不得伪造通过**）。本地替代证据（均为实跑）：①Task 6 服务编排 Interface 测试（真实 sqlite + 真实 A03 ActionStore/ocAuthorizer + httptest GitHub 模拟器真实 HTTP 字节 + 本地目录工作区，覆盖 AC1/AC2 全部分支含部分完成恢复）；②Task 7 HTTP 面 owner/granted 谓词测试；③Task 3 迁移 SQL↔模型对齐测试；④Task 8–10 移动端 wire 契约 + 纯投影 + 屏渲染测试。凡具备 env 的运行自动产出真实端到端证据。

**与调查结论的差异记录（以代码现状为准）：**

1. 调查称「appconnector 中的 github 仅是 Open Connector 测试夹具」——亲眼核实属实：全仓 Go 生产代码无任何 GitHub 实现，唯一命中是 `internal/modules/appconnector/action_test.go:189` 的 `Target: "github.createIssue"` 夹具；连接 OAuth 注册表 `appOAuthDefaults`（`internal/handler/app_connector_oauth.go:48-57`）只有 feishu/notion。
2. 调查称「execution/sandbox 仅 Cube 沙箱会话，无 git 操作」——亲眼核实属实，但沙箱已有完整会话文件面：`SessionBoundManager.ListSessionFiles/ReadSessionFile/WriteSessionWorkspaceFiles`（`internal/modules/execution/sandbox/session_manager.go:621-676/:535-560`），`internal/application/service/artifact_collector.go:36-41` 已确立「SessionBoundManager 生产满足窄端口、测试用本地目录源」的先例，本计划照搬。
3. 调查称「appconnector 与 workbench 均无 Approver/approved_by/PR 回执字段」——属实；但 `app_action_approvals`（`internal/modules/appconnector/repository/appconnector/action.go:63-71`）已持久化每个摘要的批准者 `Actor`，审批内容与批准人可从既有表追溯，本计划不重复造审批存储，只补 `code_deliveries` 回执表并在读面 join。
4. 调查称「类型化审批 internal/modules/workbench/interaction.go:12-45 已有，但与 GitHub 交付无关」——属实。本计划选择 A03 Action 管线（而非 workbench Interaction）承载交付审批：A03 是「受审批的外部操作」的既定管线（digest 锚定/批准消费/unknown 分类/#42 审批者谓词全套现成），Interaction 的 Decide 下游派发按 #38 明示不属其范围。
5. **migrations/sqlite 000112 与 versioned 000191 撞号（预存在，未解决）**：`000112_task_grants.*`（#42，commit ca9b66ee1）与 `000112_agent_adoption_variants.*`（#59，commit a3132eaa0）并存，golang-migrate 打开迁移源即报 `duplicate migration file: 000112_task_grants.down.sql`；本计划作者实跑 `go test ./internal/application/repository/ -count=1` FAIL 276 个、`go test ./internal/handler/session/ -count=1` FAIL 27 个（含 #42 自己的 `TestAgentQARunGateStaysOwnerScopedForGrantHolders`），与 `docs/plans/issue30-sweep/FINAL-REPORT.md` 7.2 节记录一致（B3 已裁定「序号重编应升级为独立决策，不在批次内顺手修改」）。**本计划不重编既有迁移**；受影响的验证策略见 Tech Stack 段（自包含夹具 + 单文件迁移对齐测试）。
6. 调查称「任务分支创建、推送……零实现」——属实；本计划把推送实现为 GitHub git-data REST 链（blobs→tree→commit→ref→draft PR），不依赖 shell git 二进制也不把令牌带进沙箱（与 spec 故事 48 一致）。git blob SHA（`sha1("blob <len>")+content`）在服务端本地计算用于 diff 判定，与 GitHub 返回的 blob sha 同构（Task 1 测试用真实 git 生成的内容锚定）。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **目标分支等于默认分支/受保护分支**（配置错误或用户误填 `main`）：必须拒绝且零远端写。——Task 5 测试「prepare refuses a target branch equal to the repo default branch with zero GitHub ref writes」+ Task 2 wire 测试断言 ref 写只发生在 `refs/heads/<任务分支>`。
2. **推送成功但 PR 创建失败后的盲目重试**（最危险的重复副作用）：恢复路径绝不能重发 blobs/tree/commit/ref。——Task 6 测试「PR-only recovery after a partial push never re-sends blobs, tree, commit or ref（模拟器调用计数断言）」。
3. **批准与执行之间内容被换**（approve-then-rewrite）或连接被撤销：digest 不匹配/版本过期必须拒派。——Task 6 测试「dispatch refuses a second delivery prepared with different files（旧批准零远端调用）」+「A02 拒绝后动作保持 authorized 且零 GitHub 调用」。
4. **远端结果不可观测（推送中途断网）**：既不能当失败重推、也不能假装成功；必须落 unknown 并只以远端事实（分支 ref / PR head 查询）收敛。——Task 6 测试「a transport cut mid-push parks unknown; provider-query resolution settles delivered from remote facts」。
5. **审批快照字段畸形/多字段**（恶意或漂移的 material JSON）：派发前解析必须精确字段校验拒绝（ErrDispatchNotStarted，零远端调用），不得静默取子集。——Task 1 测试 `ParseDeliveryMaterial` 拒绝缺字段/多字段/非对象 + Task 6 测试「a tampered action snapshot never reaches GitHub」。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | Go：交付领域纯策略 | `internal/modules/codedelivery/delivery.go`（状态/任务分支/保护分支护栏/材料快照解析/工作区 diff/git blob SHA） |
| 2 | Go：GitHub 客户端端口与生产 HTTP 适配 | `github.go`（端口）+ `github_client.go`（钉住 api.github.com 的 REST 链，无 merge）+ httptest 模拟器 wire 测试 + env 门控真实客户端测试 |
| 3 | Go：code_deliveries 迁移与存储 | `migrations/versioned/000192_*` + `migrations/sqlite/000113_*` + `repository/codedelivery/store.go`（含审批者 join 读）+ 迁移↔模型对齐测试 |
| 4 | Go：A03 词汇扩展与 GitHub OAuth 注册（共享文件最小修改） | `RiskDeliver` 常量 + 通用 Execute 拒派护栏 + `appOAuthDefaults["github"]`/token 交换形状/authorize scope |
| 5 | Go：交付编排 A（物化 + Prepare） | `workspace.go`（端口+本地适配器）+ `service.go` 的 MaterializeBaseline/PrepareDelivery（owner-only、护栏、awaiting_approval 断言、审批锚点） |
| 6 | Go：交付编排 B（Dispatch + 部分完成 + unknown） | `dispatcher.go`（ActionDispatcher/UnknownResolver）+ DispatchDelivery/ResolveDeliveryUnknown（pushed 恢复不重推、回执落账） |
| 7 | Go：workbench HTTP 面与容器接线 | `internal/handler/session/workbench_delivery.go`（baseline/delivery prepare/read/dispatch/resolve）+ 路由 + `internal/container/code_delivery.go` |
| 8 | contracts + api-client：交付读模型 | `packages/contracts/src/mobile/code-delivery.ts` + `packages/api-client/src/mobile/code-delivery.ts` + exports `./mobile/code-delivery` |
| 9 | mobile-core：交付投影与读器 | `packages/mobile-core/src/delivery/`（纯投影 + scope-guard 读器 + 场景 Adapter）+ index 导出 |
| 10 | apps/mobile：交付回执接线 | TaskDetailScreen 交付区块 + `app/tasks/detail.tsx` 装配 + composition 工厂 + app-smoke 追加 |
| 11 | 集成证据（opt-in）与收尾 | `apps/mobile/src/delivery-integration-smoke.ts` 证据契约 + 计划级验证清单复核 |

**并行批次注意（本计划与同批其余计划并行实施，独立 worktree 后合并）**：新增文件全部为本计划独有（上表 Create 项，含新目录 `internal/modules/codedelivery/`、`internal/modules/codedelivery/repository/codedelivery/`、`packages/mobile-core/src/delivery/`）。共享文件修改清单与位置（全部最小追加，便于合并）：

- `internal/modules/appconnector/action.go`：风险常量块（`:17-22`）后加 `RiskDeliver = "deliver"`（3 行含注释）。
- `internal/handler/app_connector_action.go`：`ExecuteAction` 的 `appActionStates` 校验后加通用管线风险护栏（7 行）。
- `internal/handler/app_connector_oauth.go`：`appOAuthDefaults` 加 `"github"` 条目（4 行）+ `exchangeAppOAuthCode` switch 加 `"github"` case（11 行）+ `createConnectionOAuth` 加 github scope（3 行）。
- `internal/router/routes_workbench.go`：新增 `RegisterWorkbenchDeliveryRoutes` 函数（约 20 行，追加在文件尾）。
- `internal/router/router.go`：`RegisterWorkbenchCommandRoutes` 调用行后加 1 行注册。
- `internal/container/container.go`：workbench Provide 块（`:271-274` 附近）后加 2 行 `must(container.Provide(...))`。
- `packages/contracts/src/index.ts`：`parseInteractionWithRun` 导出块（`:661-662`）后加 3 行导出。
- `packages/api-client/package.json`：exports 加 1 行 `"./mobile/code-delivery"`。
- `packages/mobile-core/src/index.ts`：末尾追加 delivery 导出块（约 10 行）。
- `apps/mobile/src/composition.ts`：`activeTaskMaterial`（`:274-278`）后追加 `deliveryFor/activeDeliveryReader`（约 18 行）。
- `apps/mobile/src/screens/TaskDetailScreen.tsx`：Props 加可选 `delivery` + 交付回执区块（约 20 行）。
- `apps/mobile/src/app/tasks/detail.tsx`：装配 delivery 读取并透传（约 15 行）。
- `apps/mobile/src/app-smoke.test.tsx`：末尾追加 2 个测试。

---

### Task 1: Go——交付领域纯策略（状态/分支护栏/材料解析/工作区 diff）

**Files:**
- Create: `internal/modules/codedelivery/delivery.go`
- Test: `internal/modules/codedelivery/delivery_test.go`

**Interfaces:**
- Consumes: 无（纯函数层；`encoding/json`/`crypto/sha1` 标准库）。
- Produces（Task 2/5/6 依赖的精确签名）:
  - `type DeliveryState string`；常量 `DeliveryPrepared/DeliveryDispatched/DeliveryPushed/DeliveryDelivered/DeliveryFailed/DeliveryUnknown`
  - `ErrProtectedBranch`、`ErrInvalidBranch`、`ErrInvalidMaterial`、`ErrBaselineTooLarge`、`ErrInvalidBaselineSHA`、`ErrRepoRefInvalid`
  - `const TaskBranchPrefix = "weknora/task/"`；`func TaskBranchOf(taskID string) string`；`func ValidateTaskBranch(branch string) error`；`func RefuseProtectedTarget(branch, defaultBranch string, protected bool) error`
  - `type RepoRef struct{ Owner, Name string }`；`func ParseRepoRef(v string) (RepoRef, error)`；`func (r RepoRef) String() string`；`func WorkspaceRepoRoot(repo RepoRef) string`（`/workspace/<owner>/<name>`）
  - `func GitBlobSHA(content []byte) string`
  - `type FileChange struct{ Path string; Deleted bool }`；`func DiffAgainstBaseline(baseline, workspace map[string]string) []FileChange`（键=path、值=blob sha；升序输出）
  - `const MaxDeliveryFiles = 500`；`type DeliveryMaterial struct{ Repo RepoRef; BaselineSHA, Branch string; Files []FileChange; CommitMessage, PRTitle string }`；`func (m DeliveryMaterial) CanonicalJSON() (json.RawMessage, error)`；`func ParseDeliveryMaterial(raw json.RawMessage) (DeliveryMaterial, error)`

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/codedelivery/delivery_test.go`：

```go
package codedelivery

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTaskBranchOfAndValidation(t *testing.T) {
	require.Equal(t, "weknora/task/s-1", TaskBranchOf("s-1"))
	require.NoError(t, ValidateTaskBranch("weknora/task/s-1"))
	// 非法：无前缀、空后缀、非法字符、过长（git refname 规则的子集白名单）
	require.ErrorIs(t, ValidateTaskBranch("main"), ErrInvalidBranch)
	require.ErrorIs(t, ValidateTaskBranch("weknora/task/"), ErrInvalidBranch)
	require.ErrorIs(t, ValidateTaskBranch("weknora/task/a b"), ErrInvalidBranch)
	require.ErrorIs(t, ValidateTaskBranch("weknora/task/a..b"), ErrInvalidBranch)
	require.ErrorIs(t, ValidateTaskBranch("weknora/task/"+string(make([]byte, 200))), ErrInvalidBranch)
}

func TestRefuseProtectedTarget(t *testing.T) {
	require.ErrorIs(t, RefuseProtectedTarget("main", "main", false), ErrProtectedBranch)
	require.ErrorIs(t, RefuseProtectedTarget("weknora/task/s-1", "main", true), ErrProtectedBranch)
	require.NoError(t, RefuseProtectedTarget("weknora/task/s-1", "main", false))
}

func TestParseRepoRefAndWorkspaceRoot(t *testing.T) {
	repo, err := ParseRepoRef("octocat/hello-world")
	require.NoError(t, err)
	require.Equal(t, RepoRef{Owner: "octocat", Name: "hello-world"}, repo)
	require.Equal(t, "octocat/hello-world", repo.String())
	_, err = ParseRepoRef("nope")
	require.ErrorIs(t, err, ErrRepoRefInvalid)
	_, err = ParseRepoRef("a/b/c")
	require.ErrorIs(t, err, ErrRepoRefInvalid)
	require.Equal(t, "/workspace/octocat/hello-world", WorkspaceRepoRoot(repo))
}

// git hash-object 与本实现同构：真实 git 生成的 blob sha 必须一致。
func TestGitBlobSHAMatchesRealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	content := []byte("package main\n\nfunc main() {}\n")
	cmd := exec.Command("git", "hash-object", "--stdin")
	cmd.Stdin = bytes.NewReader(content)
	out, err := cmd.Output()
	require.NoError(t, err)
	require.Equal(t, strings.TrimSpace(string(out)), GitBlobSHA(content))
}

func TestDiffAgainstBaseline(t *testing.T) {
	baseline := map[string]string{"a.txt": "s-a", "b.txt": "s-b", "gone.txt": "s-g"}
	workspace := map[string]string{"a.txt": "s-a", "b.txt": "s-b2", "new.txt": "s-n"}
	changes := DiffAgainstBaseline(baseline, workspace)
	require.Equal(t, []FileChange{
		{Path: "b.txt", Deleted: false},
		{Path: "gone.txt", Deleted: true},
		{Path: "new.txt", Deleted: false},
	}, changes)
}

func TestParseDeliveryMaterialExactFields(t *testing.T) {
	mat := DeliveryMaterial{
		Repo: RepoRef{Owner: "octocat", Name: "hello"},
		BaselineSHA: "b" + strings.Repeat("0", 39), Branch: TaskBranchOf("s-1"),
		Files: []FileChange{{Path: "main.go", Deleted: false}},
		CommitMessage: "fix: greeting", PRTitle: "WeKnora task s-1",
	}
	raw, err := mat.CanonicalJSON()
	require.NoError(t, err)
	parsed, err := ParseDeliveryMaterial(raw)
	require.NoError(t, err)
	require.Equal(t, mat, parsed)

	// 缺字段
	_, err = ParseDeliveryMaterial(json.RawMessage(`{"repo":"octocat/hello"}`))
	require.ErrorIs(t, err, ErrInvalidMaterial)
	// 多字段（approve-then-rewrite 面）
	_, err = ParseDeliveryMaterial(json.RawMessage(`{"repo":"o/n","baseline_sha":"` + "b" + strings.Repeat("0", 39) + `","branch":"weknora/task/s-1","files":[],"commit_message":"m","pr_title":"t","extra":1}`))
	require.ErrorIs(t, err, ErrInvalidMaterial)
	// 非对象
	_, err = ParseDeliveryMaterial(json.RawMessage(`[]`))
	require.ErrorIs(t, err, ErrInvalidMaterial)
	// 非法 repo / 非法基线 / 非法分支 / 超量文件
	bad := mat
	bad.Repo = RepoRef{}
	_, err = ParseDeliveryMaterial(mustJSON(t, bad))
	require.ErrorIs(t, err, ErrRepoRefInvalid)
	bad = mat
	bad.BaselineSHA = "zz"
	_, err = ParseDeliveryMaterial(mustJSON(t, bad))
	require.ErrorIs(t, err, ErrInvalidBaselineSHA)
	bad = mat
	bad.Branch = "main"
	_, err = ParseDeliveryMaterial(mustJSON(t, bad))
	require.ErrorIs(t, err, ErrInvalidBranch)
	bad = mat
	bad.Files = make([]FileChange, MaxDeliveryFiles+1)
	_, err = ParseDeliveryMaterial(mustJSON(t, bad))
	require.ErrorIs(t, err, ErrBaselineTooLarge)
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/codedelivery/ -count=1`
Expected: FAIL（包不存在/符号未定义，编译错误）

- [ ] **Step 3: 写最小实现**

创建 `internal/modules/codedelivery/delivery.go`：

```go
// Package codedelivery owns the developer delivery chain (T22 #52):
// anchoring a run's workspace diff against a fixed GitHub baseline into an
// A03-approved action, and delivering the approved material as a task
// branch push + draft PR — never a protected-branch write, never a merge.
package codedelivery

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// DeliveryState is the delivery lifecycle owned by this module. The A03
// action row keeps the approval lifecycle; this state records the remote
// effect, including the PARTIAL completion push-succeeded-PR-failed.
type DeliveryState string

const (
	DeliveryPrepared   DeliveryState = "prepared"   // 材料已锚定为 A03 action，等待审批
	DeliveryDispatched DeliveryState = "dispatched" // 推送链路进行中
	DeliveryPushed     DeliveryState = "pushed"     // 部分完成：任务分支已推、PR 未成（恢复只补 PR）
	DeliveryDelivered  DeliveryState = "delivered"  // 草稿 PR 已创建/更新
	DeliveryFailed     DeliveryState = "failed"
	DeliveryUnknown    DeliveryState = "unknown" // 远端结果不可观测，只以远端事实收敛
)

var (
	ErrProtectedBranch    = errors.New("code_delivery_protected_branch")
	ErrInvalidBranch      = errors.New("code_delivery_invalid_branch")
	ErrInvalidMaterial    = errors.New("code_delivery_invalid_material")
	ErrBaselineTooLarge   = errors.New("code_delivery_baseline_too_large")
	ErrInvalidBaselineSHA = errors.New("code_delivery_invalid_baseline_sha")
	ErrRepoRefInvalid     = errors.New("code_delivery_repo_ref_invalid")
)

// TaskBranchPrefix is the whitelist prefix of every branch this module may
// create or update. refs outside heads/<prefix>… are structurally unreachable.
const TaskBranchPrefix = "weknora/task/"

// MaxDeliveryFiles bounds one delivery material (baseline + workspace union).
const MaxDeliveryFiles = 500

var taskBranchSuffix = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,120}$`)

// TaskBranchOf derives the deterministic task branch for a task (session) id.
func TaskBranchOf(taskID string) string { return TaskBranchPrefix + taskID }

// ValidateTaskBranch enforces the prefix whitelist and a conservative refname
// charset (no spaces, no "..", no leading "-", bounded length).
func ValidateTaskBranch(branch string) error {
	if !strings.HasPrefix(branch, TaskBranchPrefix) {
		return fmt.Errorf("%w: %q lacks prefix %q", ErrInvalidBranch, branch, TaskBranchPrefix)
	}
	suffix := strings.TrimPrefix(branch, TaskBranchPrefix)
	if suffix == "" || !taskBranchSuffix.MatchString(suffix) {
		return fmt.Errorf("%w: illegal task branch suffix", ErrInvalidBranch)
	}
	return nil
}

// RefuseProtectedTarget is the AC1 guardrail: the delivery target must differ
// from the repository default branch and must not be marked protected by the
// code platform.
func RefuseProtectedTarget(branch, defaultBranch string, protected bool) error {
	if defaultBranch != "" && branch == defaultBranch {
		return fmt.Errorf("%w: target branch is the repository default branch", ErrProtectedBranch)
	}
	if protected {
		return fmt.Errorf("%w: target branch is protected on the code platform", ErrProtectedBranch)
	}
	return nil
}

// RepoRef names one GitHub repository.
type RepoRef struct{ Owner, Name string }

func (r RepoRef) String() string { return r.Owner + "/" + r.Name }

var repoRefPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// ParseRepoRef accepts exactly "owner/name" (single slash, no scheme).
func ParseRepoRef(v string) (RepoRef, error) {
	v = strings.TrimSpace(v)
	if !repoRefPattern.MatchString(v) {
		return RepoRef{}, fmt.Errorf("%w: %q", ErrRepoRefInvalid, v)
	}
	parts := strings.SplitN(v, "/", 2)
	return RepoRef{Owner: parts[0], Name: parts[1]}, nil
}

// WorkspaceRepoRoot is the fixed workspace root a baseline is materialized
// into and diffs are read from: /workspace/<owner>/<name>.
func WorkspaceRepoRoot(repo RepoRef) string { return "/workspace/" + repo.Owner + "/" + repo.Name }

// GitBlobSHA computes the git blob object id (sha1 of "blob <len>\0"+content).
// It equals what GitHub returns for CreateBlob of the same bytes, so local
// diffing against a baseline tree needs no git binary and no credential.
func GitBlobSHA(content []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// FileChange is one entry of a delivery: a modified/added file, or a deletion.
type FileChange struct {
	Path    string `json:"path"`
	Deleted bool   `json:"deleted"`
}

// DiffAgainstBaseline returns the sorted changes between a baseline tree
// (path→blob sha) and a workspace projection (path→blob sha).
func DiffAgainstBaseline(baseline, workspace map[string]string) []FileChange {
	paths := make(map[string]bool, len(baseline)+len(workspace))
	for p := range baseline {
		paths[p] = true
	}
	for p := range workspace {
		paths[p] = true
	}
	ordered := make([]string, 0, len(paths))
	for p := range paths {
		ordered = append(ordered, p)
	}
	sort.Strings(ordered)
	out := make([]FileChange, 0, len(ordered))
	for _, p := range ordered {
		baseSHA, inBase := baseline[p]
		workSHA, inWork := workspace[p]
		switch {
		case inBase && !inWork:
			out = append(out, FileChange{Path: p, Deleted: true})
		case !inBase && inWork:
			out = append(out, FileChange{Path: p})
		case inBase && inWork && baseSHA != workSHA:
			out = append(out, FileChange{Path: p})
		}
	}
	return out
}

// DeliveryMaterial is the EXACT approved snapshot of one delivery — the A03
// action args. ParseDeliveryMaterial is the dispatch-side guard: the bytes
// approved are the bytes delivered, and a tampered or drifted snapshot
// (missing field, extra field, non-object) is refused, never subset-parsed.
type DeliveryMaterial struct {
	Repo          RepoRef      `json:"repo"`
	BaselineSHA   string       `json:"baseline_sha"`
	Branch        string       `json:"branch"`
	Files         []FileChange `json:"files"`
	CommitMessage string       `json:"commit_message"`
	PRTitle       string       `json:"pr_title"`
}

// CanonicalJSON marshals the material deterministically (struct field order,
// sorted file list) so the A03 digest always covers the same logical bytes.
func (m DeliveryMaterial) CanonicalJSON() (json.RawMessage, error) {
	sorted := make([]FileChange, len(m.Files))
	copy(sorted, m.Files)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	m.Files = sorted
	raw, err := json.Marshal(m)
	return raw, err
}

var baselineSHAPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ParseDeliveryMaterial validates the approved snapshot exactly: all six
// fields present, no extras, legal repo/baseline/branch, bounded file list.
func ParseDeliveryMaterial(raw json.RawMessage) (DeliveryMaterial, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil || probe == nil {
		return DeliveryMaterial{}, fmt.Errorf("%w: not a JSON object", ErrInvalidMaterial)
	}
	if len(probe) != 6 {
		return DeliveryMaterial{}, fmt.Errorf("%w: snapshot must be exactly repo, baseline_sha, branch, files, commit_message, pr_title", ErrInvalidMaterial)
	}
	var m DeliveryMaterial
	if err := json.Unmarshal(raw, &m); err != nil {
		return DeliveryMaterial{}, fmt.Errorf("%w: %v", ErrInvalidMaterial, err)
	}
	if _, err := ParseRepoRef(m.Repo.String()); err != nil {
		return m, err
	}
	if !baselineSHAPattern.MatchString(m.BaselineSHA) {
		return m, fmt.Errorf("%w: baseline must be a 40-hex commit sha", ErrInvalidBaselineSHA)
	}
	if err := ValidateTaskBranch(m.Branch); err != nil {
		return m, err
	}
	if m.CommitMessage == "" || m.PRTitle == "" {
		return m, fmt.Errorf("%w: commit_message and pr_title are required", ErrInvalidMaterial)
	}
	if len(m.Files) > MaxDeliveryFiles {
		return m, fmt.Errorf("%w: %d files exceed cap %d", ErrBaselineTooLarge, len(m.Files), MaxDeliveryFiles)
	}
	for _, f := range m.Files {
		if f.Path == "" || strings.HasPrefix(f.Path, "/") || strings.Contains(f.Path, "..") {
			return m, fmt.Errorf("%w: illegal file path %q", ErrInvalidMaterial, f.Path)
		}
	}
	return m, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/codedelivery/ -count=1`
Expected: PASS（`TestTaskBranchOfAndValidation` 等 6 个全绿；`TestGitBlobSHAMatchesRealGit` 在无 git 二进制的环境 skip，本 worktree 有 git 必跑）

- [ ] **Step 5: Commit**

```bash
git add internal/modules/codedelivery/delivery.go internal/modules/codedelivery/delivery_test.go
git commit -m "feat(codedelivery): delivery domain policy — task branch guardrails, exact material snapshot, workspace diff (T22 #52 task 1)"
```

---

### Task 2: Go——GitHub 客户端端口、生产 HTTP 适配与 wire 测试

**Files:**
- Create: `internal/modules/codedelivery/github.go`
- Create: `internal/modules/codedelivery/github_client.go`
- Test: `internal/modules/codedelivery/github_wire_test.go`
- Test: `internal/modules/codedelivery/github_real_test.go`（env 门控，缺 env 即 skip）

**Interfaces:**
- Consumes: Task 1 的 `RepoRef`/`FileChange`。
- Produces（Task 5/6 依赖的精确签名）:
  - `type GitHubRepoInfo struct{ DefaultBranch string }`；`type TreeEntry struct{ Path, SHA string; Deleted bool }`；`type PullRequestReceipt struct{ Number int64; URL string; Draft bool; Created bool }`；`type PullRequestInput struct{ Title, Head, Base string }`
  - `var ErrGitHubTransport = errors.New("github_transport_unobservable")`；`type GitHubAPIError struct{ Status int; Endpoint, Message string }`（确定性远端错误）
  - `type GitHubClient interface`：`Repository(ctx) (GitHubRepoInfo, error)`、`BranchProtected(ctx, branch string) (bool, error)`、`Tree(ctx, sha string) (map[string]string, error)`、`CommitTree(ctx, commitSHA string) (string, error)`、`Blob(ctx, sha string) ([]byte, error)`、`CreateBlob(ctx, content []byte) (string, error)`、`CreateTree(ctx, baseTree string, entries []TreeEntry) (string, error)`、`CreateCommit(ctx, parent, tree, message string) (string, error)`、`EnsureBranch(ctx, branch, commit string) error`、`DraftPullRequest(ctx, input PullRequestInput) (PullRequestReceipt, error)`、`PullRequestForHead(ctx, head string) (*PullRequestReceipt, error)`、`CurrentLogin(ctx) (string, error)`——**没有任何 merge 方法**
  - `type GitHubClientFactory func(token string, repo RepoRef) GitHubClient`（每次调用以 material/input 的 `RepoRef` 钉仓——REST 路径 `/repos/<owner>/<name>/...` 由该参数派生，客户端不持有可漂移的全局仓库状态）；`func NewGitHubClientFactory(httpClient *http.Client, baseURL string) GitHubClientFactory`（baseURL 传 `https://api.github.com`；测试注入 httptest URL）
  - 测试侧模拟器：`newGitHubEmulator(t) *githubEmulator`（`github_wire_test.go`，httptest 内存对象库，`Calls() map[string]int`，`Violations() []string` 含 merge 尝试）

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/codedelivery/github_wire_test.go`：

```go
package codedelivery

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// githubEmulator 是一个内存 GitHub：真实 HTTP 字节进出生产客户端，
// blobs 以真实 git blob sha 入库（与 GitBlobSHA 同构），并记录每类调用次数
// 与违规（merge 尝试、越权 ref 写）。blackout 模拟传输不可观测。
type githubEmulator struct {
	t                *testing.T
	srv              *httptest.Server
	mu               sync.Mutex
	token            string
	calls            map[string]int
	bad              []string
	repo             map[string][]byte            // path → content（默认分支树）
	blobs            map[string][]byte            // sha → content
	trees            map[string]map[string]string // tree sha → path→blob sha
	commits          map[string]string            // commit sha → tree sha
	refs             map[string]string            // refs/heads/<branch> → commit sha
	prs              []emulatorPR
	nextPR           int64
	protectedBranches []string
	failPR           bool // 下一条 POST /pulls 确定性 422 一次
	blackout         bool // 所有请求 hijack 断连（传输不可观测）
}
type emulatorPR struct {
	Number int64
	Title  string
	Head   string
	Base   string
	Draft  bool
	State  string
}

func newGitHubEmulator(t *testing.T) *githubEmulator {
	e := &githubEmulator{
		t: t, token: "gho_testtoken", calls: map[string]int{},
		blobs: map[string][]byte{}, trees: map[string]map[string]string{},
		commits: map[string]string{}, refs: map[string]string{}, prs: []emulatorPR{},
		protectedBranches: []string{"prod"},
		repo: map[string][]byte{
			"README.md": []byte("# hello\n"),
			"main.go":   []byte("package main\n"),
		},
	}
	tree := map[string]string{}
	for p, c := range e.repo {
		sha := GitBlobSHA(c)
		e.blobs[sha] = c
		tree[p] = sha
	}
	e.trees["tree-baseline"] = tree
	e.commits["b"+strings.Repeat("0", 39)] = "tree-baseline"
	e.refs["refs/heads/main"] = "b" + strings.Repeat("0", 39)
	mux := http.NewServeMux()
	mux.HandleFunc("/", e.serve)
	e.srv = httptest.NewServer(mux)
	t.Cleanup(e.srv.Close)
	return e
}

func (e *githubEmulator) note(call string) { e.mu.Lock(); e.calls[call]++; e.mu.Unlock() }
func (e *githubEmulator) violation(v string) {
	e.mu.Lock()
	e.bad = append(e.bad, v)
	e.mu.Unlock()
}
func (e *githubEmulator) Calls() map[string]int { return e.calls }
func (e *githubEmulator) Violations() []string  { return e.bad }
func (e *githubEmulator) BranchCommit(branch string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	sha, ok := e.refs["refs/heads/"+branch]
	return sha, ok
}
func (e *githubEmulator) protectBranch(branch string) {
	e.mu.Lock()
	e.protectedBranches = append(e.protectedBranches, branch)
	e.mu.Unlock()
}
func (e *githubEmulator) failNextPRCreation() { e.mu.Lock(); e.failPR = true; e.mu.Unlock() }
func (e *githubEmulator) blackoutAfterRefCreate() {
	e.mu.Lock()
	e.blackout = true
	e.mu.Unlock()
}
func (e *githubEmulator) liftBlackout() { e.mu.Lock(); e.blackout = false; e.mu.Unlock() }
func (e *githubEmulator) isProtected(branch string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, b := range e.protectedBranches {
		if b == branch {
			return true
		}
	}
	return false
}

func (e *githubEmulator) serve(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	blackout := e.blackout
	failPR := e.failPR
	e.mu.Unlock()
	if blackout {
		// 连接直接断开：客户端拿到 transport 错误（不可观测）。
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		panic(http.ErrAbortHandler)
	}
	if got := r.Header.Get("Authorization"); got != "Bearer "+e.token {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "bad credentials"})
		return
	}
	// 结构性护栏：任何 merge 尝试都是违规并被拒绝。
	if strings.Contains(r.URL.Path, "/merge") && r.Method == http.MethodPut {
		e.violation("merge attempted: " + r.Method + " " + r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && path == "/user":
		e.note("GET /user")
		writeJSON(w, map[string]any{"login": "octocat"})
	case r.Method == http.MethodGet && path == "/repos/octocat/hello":
		e.note("GET /repos")
		writeJSON(w, map[string]any{"default_branch": "main", "private": true})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/branches/"):
		e.note("GET /branches")
		branch := strings.TrimPrefix(path, "/repos/octocat/hello/branches/")
		writeJSON(w, map[string]any{"name": branch, "protected": e.isProtected(branch)})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/commits/"):
		e.note("GET /git/commits")
		sha := strings.TrimPrefix(path, "/repos/octocat/hello/git/commits/")
		e.mu.Lock()
		treeSHA, ok := e.commits[sha]
		e.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"sha": sha, "tree": treeSHA, "parents": []string{}})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/trees/"):
		e.note("GET /git/trees")
		sha := strings.TrimPrefix(path, "/repos/octocat/hello/git/trees/")
		e.mu.Lock()
		tree, ok := e.trees[sha]
		e.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		entries := make([]map[string]any, 0, len(tree))
		for p, b := range tree {
			entries = append(entries, map[string]any{"path": p, "type": "blob", "sha": b})
		}
		writeJSON(w, map[string]any{"sha": sha, "tree": entries, "truncated": false})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/blobs/"):
		e.note("GET /git/blobs")
		sha := strings.TrimPrefix(path, "/repos/octocat/hello/git/blobs/")
		e.mu.Lock()
		content, ok := e.blobs[sha]
		e.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"sha": sha, "encoding": "base64", "content": base64.StdEncoding.EncodeToString(content)})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/blobs":
		e.note("POST /git/blobs")
		var body struct{ Content string `json:"content"` }
		_ = json.NewDecoder(r.Body).Decode(&body)
		content, err := base64.StdEncoding.DecodeString(body.Content)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sha := GitBlobSHA(content)
		e.mu.Lock()
		e.blobs[sha] = content
		e.mu.Unlock()
		writeJSON(w, map[string]any{"sha": sha})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/trees":
		e.note("POST /git/trees")
		var body struct {
			BaseTree string `json:"base_tree"`
			Tree     []struct {
				Path string `json:"path"`
				Mode string `json:"mode"`
				Type string `json:"type"`
				SHA  string `json:"sha"`
			} `json:"tree"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		e.mu.Lock()
		next, ok := e.trees[body.BaseTree]
		e.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		next = map[string]string{}
		e.mu.Lock()
		for k, v := range e.trees[body.BaseTree] {
			next[k] = v
		}
		for _, entry := range body.Tree {
			if entry.SHA == "" { // 删除项（sha 为 null）
				delete(next, entry.Path)
				continue
			}
			next[entry.Path] = entry.SHA
		}
		sha := fmt.Sprintf("tree-%d", len(e.trees)+1)
		e.trees[sha] = next
		e.mu.Unlock()
		writeJSON(w, map[string]any{"sha": sha})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/commits":
		e.note("POST /git/commits")
		var body struct {
			Message string `json:"message"`
			Tree    string `json:"tree"`
			Parents []string `json:"parents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		sha := fmt.Sprintf("c%d", len(e.commits)+1)
		e.mu.Lock()
		e.commits[sha] = body.Tree
		e.mu.Unlock()
		writeJSON(w, map[string]any{"sha": sha})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/refs":
		e.note("POST /git/refs")
		var body struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !strings.HasPrefix(body.Ref, "refs/heads/") || strings.HasSuffix(body.Ref, "refs/heads/main") || strings.HasSuffix(body.Ref, "refs/heads/prod") {
			e.violation("illegal ref write: " + body.Ref)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		e.mu.Lock()
		if _, exists := e.refs[body.Ref]; exists {
			e.mu.Unlock()
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		e.refs[body.Ref] = body.SHA
		e.mu.Unlock()
		writeJSON(w, map[string]any{"ref": body.Ref, "object": map[string]any{"sha": body.SHA}})
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/repos/octocat/hello/git/refs/heads/"):
		e.note("PATCH /git/refs")
		branch := strings.TrimPrefix(path, "/repos/octocat/hello/git/refs/heads/")
		if branch == "main" || branch == "prod" {
			e.violation("protected ref update: " + branch)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		var body struct{ SHA string `json:"sha"` }
		_ = json.NewDecoder(r.Body).Decode(&body)
		e.mu.Lock()
		e.refs["refs/heads/"+branch] = body.SHA
		e.mu.Unlock()
		writeJSON(w, map[string]any{"ref": "refs/heads/" + branch, "object": map[string]any{"sha": body.SHA}})
	case r.Method == http.MethodGet && path == "/repos/octocat/hello/pulls":
		e.note("GET /pulls")
		head := r.URL.Query().Get("head")
		e.mu.Lock()
		for _, pr := range e.prs {
			if pr.Head == head {
				e.mu.Unlock()
				writeJSON(w, []map[string]any{{"number": pr.Number, "html_url": prURL(pr.Number), "draft": pr.Draft, "state": pr.State}})
				return
			}
		}
		e.mu.Unlock()
		writeJSON(w, []map[string]any{})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/pulls":
		e.note("POST /pulls")
		if failPR {
			e.mu.Lock()
			e.failPR = false
			e.mu.Unlock()
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "validation failed"})
			return
		}
		var body struct {
			Title string `json:"title"`
			Head  string `json:"head"`
			Base  string `json:"base"`
			Draft *bool  `json:"draft"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Draft == nil || !*body.Draft {
			e.violation("non-draft pull request created")
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		e.mu.Lock()
		e.nextPR++
		pr := emulatorPR{Number: e.nextPR, Title: body.Title, Head: body.Head, Base: body.Base, Draft: true, State: "open"}
		e.prs = append(e.prs, pr)
		e.mu.Unlock()
		writeJSON(w, map[string]any{"number": pr.Number, "html_url": prURL(pr.Number), "draft": true})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func prURL(n int64) string { return fmt.Sprintf("https://github.com/octocat/hello/pull/%d", n) }

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func TestGitHubClientWireChainCreatesBranchAndDraftPR(t *testing.T) {
	e := newGitHubEmulator(t)
	factory := NewGitHubClientFactory(http.DefaultClient, e.srv.URL)
	ctx := context.Background()

	repo := RepoRef{Owner: "octocat", Name: "hello"}
	client := factory(e.token, repo) // 每次调用钉定 token + 仓库
	info, err := client.Repository(ctx)
	require.NoError(t, err)
	require.Equal(t, "main", info.DefaultBranch)

	baseTree, err := client.CommitTree(ctx, "b"+strings.Repeat("0", 39))
	require.NoError(t, err)
	require.Equal(t, "tree-baseline", baseTree)

	protected, err := client.BranchProtected(ctx, "prod")
	require.NoError(t, err)
	require.True(t, protected)

	tree, err := client.Tree(ctx, "b"+strings.Repeat("0", 39))
	require.NoError(t, err)
	require.Len(t, tree, 2)

	blob, err := client.Blob(ctx, tree["main.go"])
	require.NoError(t, err)
	require.Equal(t, "package main\n", string(blob))

	newSHA, err := client.CreateBlob(ctx, []byte("package main\n\nfunc main() {}\n"))
	require.NoError(t, err)
	treeSHA, err := client.CreateTree(ctx, "tree-baseline", []TreeEntry{{Path: "main.go", SHA: newSHA}})
	require.NoError(t, err)
	commitSHA, err := client.CreateCommit(ctx, "b"+strings.Repeat("0", 39), treeSHA, "fix: greeting")
	require.NoError(t, err)

	branch := TaskBranchOf("s-1")
	require.NoError(t, client.EnsureBranch(ctx, branch, commitSHA))
	got, ok := e.BranchCommit(branch)
	require.True(t, ok)
	require.Equal(t, commitSHA, got)

	receipt, err := client.DraftPullRequest(ctx, PullRequestInput{Title: "WeKnora task s-1", Head: "octocat:" + branch, Base: "main"})
	require.NoError(t, err)
	require.True(t, receipt.Draft)
	require.True(t, receipt.Created)
	require.EqualValues(t, 1, receipt.Number)

	login, err := client.CurrentLogin(ctx)
	require.NoError(t, err)
	require.Equal(t, "octocat", login)

	// 已存在同 head 的 PR：复用（Created=false），不重复建。
	again, err := client.DraftPullRequest(ctx, PullRequestInput{Title: "WeKnora task s-1", Head: "octocat:" + branch, Base: "main"})
	require.NoError(t, err)
	require.False(t, again.Created)
	require.EqualValues(t, 1, again.Number)

	require.Empty(t, e.Violations())
	require.Zero(t, e.Calls()["PUT /pulls/merge"])
}

func TestGitHubClientClassifiesDefiniteVsUnobservable(t *testing.T) {
	e := newGitHubEmulator(t)
	factory := NewGitHubClientFactory(http.DefaultClient, e.srv.URL)
	ctx := context.Background()

	// 确定性 404：GitHubAPIError（status 携带），不是 transport。
	_, err := factory(e.token, RepoRef{Owner: "octocat", Name: "hello"}).Tree(ctx, "0123456789012345678901234567890123456789")
	var apiErr *GitHubAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusNotFound, apiErr.Status)

	// 坏令牌：确定性 401。
	_, err = factory("gho_wrong", RepoRef{Owner: "octocat", Name: "hello"}).Repository(ctx)
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusUnauthorized, apiErr.Status)

	// 网络不可达：ErrGitHubTransport（不可观测）。
	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closed.Close()
	_, err = NewGitHubClientFactory(http.DefaultClient, closed.URL)(e.token).Repository(ctx)
	require.ErrorIs(t, err, ErrGitHubTransport)
}
```

创建 `internal/modules/codedelivery/github_real_test.go`（blocked-env 门控）：

```go
package codedelivery

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGitHubClientAgainstRealGitHub 需要 WEKNORA_GITHUB_TEST_TOKEN（经典
// PAT，repo 只读作用域）与 WEKNORA_GITHUB_TEST_REPO（owner/name，公开仓库）。
// 缺 env 即 skip——真实 GitHub 属 true external（blocked-env），不伪造。
func TestGitHubClientAgainstRealGitHub(t *testing.T) {
	token := os.Getenv("WEKNORA_GITHUB_TEST_TOKEN")
	repoRaw := os.Getenv("WEKNORA_GITHUB_TEST_REPO")
	if token == "" || repoRaw == "" {
		t.Skip("WEKNORA_GITHUB_TEST_TOKEN/WEKNORA_GITHUB_TEST_REPO not set (blocked-env)")
	}
	repo, err := ParseRepoRef(repoRaw)
	require.NoError(t, err)
	client := NewGitHubClientFactory(httpClientDefault(), GitHubAPIBaseURL)(token, repo)
	ctx := context.Background()

	info, err := client.Repository(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, info.DefaultBranch)
	login, err := client.CurrentLogin(ctx)
	require.NoError(t, err)
	t.Logf("real github: repo=%s default=%s login=%s", repo, info.DefaultBranch, login)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/codedelivery/ -count=1`
Expected: FAIL（`NewGitHubClientFactory`/`GitHubAPIError` 等未定义，编译错误）

- [ ] **Step 3: 写最小实现**

创建 `internal/modules/codedelivery/github.go`（端口与错误分类）：

```go
package codedelivery

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// GitHubAPIBaseURL is the reviewed production endpoint. Tests inject an
// httptest URL; the factory never derives hosts from model output.
const GitHubAPIBaseURL = "https://api.github.com"

// ErrGitHubTransport marks an UNOBSERVABLE remote outcome (dial/timeout/EOF):
// the request may or may not have reached GitHub. Callers must park unknown,
// never blind-retry. A GitHub RESPONSE — any status — is a definite outcome
// and surfaces as *GitHubAPIError instead.
var ErrGitHubTransport = errors.New("github_transport_unobservable")

// GitHubAPIError is a definite provider refusal with an HTTP status.
type GitHubAPIError struct {
	Status   int
	Endpoint string
	Message  string
}

func (e *GitHubAPIError) Error() string {
	return "github api " + e.Endpoint + ": status " + itoa(e.Status) + ": " + e.Message
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	digits := []byte{}
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}

// GitHubRepoInfo is the repository metadata the guardrail needs.
type GitHubRepoInfo struct{ DefaultBranch string }

// TreeEntry is one tree mutation: set a path to a blob sha, or delete it.
type TreeEntry struct {
	Path    string
	SHA     string // "" = delete
	Mode    string // "100644" default
}

// PullRequestInput names the draft PR to create/reuse.
type PullRequestInput struct{ Title, Head, Base string }

// PullRequestReceipt is the PR traceability receipt.
type PullRequestReceipt struct {
	Number  int64
	URL     string
	Draft   bool
	Created bool
}

// GitHubClient is the outbound code-platform port of one delivery token.
// There is deliberately NO merge method: auto-merge is structurally absent
// (spec: "It does not merge automatically").
type GitHubClient interface {
	Repository(ctx context.Context) (GitHubRepoInfo, error)
	BranchProtected(ctx context.Context, branch string) (bool, error)
	Tree(ctx context.Context, sha string) (map[string]string, error)
	CommitTree(ctx context.Context, commitSHA string) (string, error)
	Blob(ctx context.Context, sha string) ([]byte, error)
	CreateBlob(ctx context.Context, content []byte) (string, error)
	CreateTree(ctx context.Context, baseTree string, entries []TreeEntry) (string, error)
	CreateCommit(ctx context.Context, parent, tree, message string) (string, error)
	EnsureBranch(ctx context.Context, branch, commit string) error
	DraftPullRequest(ctx context.Context, input PullRequestInput) (PullRequestReceipt, error)
	PullRequestForHead(ctx context.Context, head string) (*PullRequestReceipt, error)
	CurrentLogin(ctx context.Context) (string, error)
}

// GitHubClientFactory binds one token + one repository to a client: the
// caller pins the repo per call from the approved material/input, so REST
// paths are always derived from a reviewed RepoRef. The service resolves the
// token through CredentialResolver and immediately scopes it here — the
// token never escapes this port.
type GitHubClientFactory func(token string, repo RepoRef) GitHubClient

func httpClientDefault() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}
```

创建 `internal/modules/codedelivery/github_client.go`：

```go
package codedelivery

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// NewGitHubClientFactory builds the production REST adapter over the git-data
// API chain (blobs → tree(base_tree) → commit(parent) → refs/heads/<branch> →
// draft PR). baseURL is a reviewed constant in production and the injected
// emulator URL in tests. Every call pins BOTH the resolved token and the
// delivery's own RepoRef — the client is stateless between calls.
func NewGitHubClientFactory(httpClient *http.Client, baseURL string) GitHubClientFactory {
	if httpClient == nil {
		httpClient = httpClientDefault()
	}
	return func(token string, repo RepoRef) GitHubClient {
		return &gitHubRestClient{http: httpClient, base: baseURL, token: token, repo: repo}
	}
}

type gitHubRestClient struct {
	http  *http.Client
	base  string
	token string
	repo  RepoRef
}

func (c *gitHubRestClient) call(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return fmt.Errorf("%w: build %s %s: %v", ErrGitHubTransport, method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %s %s: %v", ErrGitHubTransport, method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("%w: %s %s: read body: %v", ErrGitHubTransport, method, path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var env struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &env)
		return &GitHubAPIError{Status: resp.StatusCode, Endpoint: method + " " + path, Message: env.Message}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (c *gitHubRestClient) Repository(ctx context.Context) (GitHubRepoInfo, error) {
	var out struct {
		DefaultBranch string `json:"default_branch"`
	}
	err := c.call(ctx, http.MethodGet, "/repos/"+c.repoString(), nil, &out)
	return GitHubRepoInfo{DefaultBranch: out.DefaultBranch}, err
}

func (c *gitHubRestClient) BranchProtected(ctx context.Context, branch string) (bool, error) {
	var out struct {
		Protected bool `json:"protected"`
	}
	err := c.call(ctx, http.MethodGet, "/repos/"+c.repoString()+"/branches/"+branch, nil, &out)
	if err != nil {
		return false, err
	}
	return out.Protected, nil
}

func (c *gitHubRestClient) Tree(ctx context.Context, sha string) (map[string]string, error) {
	var out struct {
		Truncated bool `json:"truncated"`
		Tree      []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
		} `json:"tree"`
	}
	if err := c.call(ctx, http.MethodGet, "/repos/"+c.repoString()+"/git/trees/"+sha+"?recursive=1", nil, &out); err != nil {
		return nil, err
	}
	if out.Truncated {
		return nil, &GitHubAPIError{Status: 0, Endpoint: "git/trees", Message: "tree truncated beyond recursive limit"}
	}
	entries := make(map[string]string, len(out.Tree))
	for _, e := range out.Tree {
		if e.Type == "blob" {
			entries[e.Path] = e.SHA
		}
	}
	return entries, nil
}

func (c *gitHubRestClient) Blob(ctx context.Context, sha string) ([]byte, error) {
	var out struct {
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
	}
	if err := c.call(ctx, http.MethodGet, "/repos/"+c.repoString()+"/git/blobs/"+sha, nil, &out); err != nil {
		return nil, err
	}
	if out.Encoding != "base64" {
		return nil, &GitHubAPIError{Status: 0, Endpoint: "git/blobs", Message: "unsupported encoding " + out.Encoding}
	}
	return base64.StdEncoding.DecodeString(out.Content)
}

// CommitTree resolves the tree sha of a commit (CreateTree's base_tree).
func (c *gitHubRestClient) CommitTree(ctx context.Context, commitSHA string) (string, error) {
	var out struct {
		Tree string `json:"tree"`
	}
	err := c.call(ctx, http.MethodGet, "/repos/"+c.repoString()+"/git/commits/"+commitSHA, nil, &out)
	return out.Tree, err
}

func (c *gitHubRestClient) CreateBlob(ctx context.Context, content []byte) (string, error) {
	var out struct {
		SHA string `json:"sha"`
	}
	err := c.call(ctx, http.MethodPost, "/repos/"+c.repoString()+"/git/blobs",
		map[string]string{"content": base64.StdEncoding.EncodeToString(content), "encoding": "base64"}, &out)
	return out.SHA, err
}

func (c *gitHubRestClient) CreateTree(ctx context.Context, baseTree string, entries []TreeEntry) (string, error) {
	type wireEntry struct {
		Path string  `json:"path"`
		Mode string  `json:"mode"`
		Type string  `json:"type"`
		SHA  *string `json:"sha"`
	}
	tree := make([]wireEntry, 0, len(entries))
	for _, e := range entries {
		mode := e.Mode
		if mode == "" {
			mode = "100644"
		}
		var sha *string
		if e.SHA != "" {
			sha = &e.SHA
		}
		tree = append(tree, wireEntry{Path: e.Path, Mode: mode, Type: "blob", SHA: sha})
	}
	var out struct {
		SHA string `json:"sha"`
	}
	err := c.call(ctx, http.MethodPost, "/repos/"+c.repoString()+"/git/trees",
		map[string]any{"base_tree": baseTree, "tree": tree}, &out)
	return out.SHA, err
}

func (c *gitHubRestClient) CreateCommit(ctx context.Context, parent, tree, message string) (string, error) {
	var out struct {
		SHA string `json:"sha"`
	}
	err := c.call(ctx, http.MethodPost, "/repos/"+c.repoString()+"/git/commits",
		map[string]any{"message": message, "tree": tree, "parents": []string{parent}}, &out)
	return out.SHA, err
}

// EnsureBranch creates refs/heads/<branch>; on "already exists" it updates the
// SAME task branch to the new commit (draft-PR iteration on one task branch).
func (c *gitHubRestClient) EnsureBranch(ctx context.Context, branch, commit string) error {
	create := map[string]any{"ref": "refs/heads/" + branch, "sha": commit}
	if err := c.call(ctx, http.MethodPost, "/repos/"+c.repoString()+"/git/refs", create, nil); err == nil {
		return nil
	} else {
		var apiErr *GitHubAPIError
		if !asGitHubAPIError(err, &apiErr) || apiErr.Status != http.StatusUnprocessableEntity {
			return err
		}
	}
	return c.call(ctx, http.MethodPatch, "/repos/"+c.repoString()+"/git/refs/heads/"+branch,
		map[string]any{"sha": commit, "force": false}, nil)
}

func (c *gitHubRestClient) DraftPullRequest(ctx context.Context, input PullRequestInput) (PullRequestReceipt, error) {
	if existing, err := c.PullRequestForHead(ctx, input.Head); err == nil && existing != nil {
		return PullRequestReceipt{Number: existing.Number, URL: existing.URL, Draft: true, Created: false}, nil
	}
	body := map[string]any{"title": input.Title, "head": input.Head, "base": input.Base, "draft": true}
	var out struct {
		Number  int64  `json:"number"`
		HTMLURL string `json:"html_url"`
		Draft   bool   `json:"draft"`
	}
	if err := c.call(ctx, http.MethodPost, "/repos/"+c.repoString()+"/pulls", body, &out); err != nil {
		return PullRequestReceipt{}, err
	}
	return PullRequestReceipt{Number: out.Number, URL: out.HTMLURL, Draft: out.Draft, Created: true}, nil
}

func (c *gitHubRestClient) PullRequestForHead(ctx context.Context, head string) (*PullRequestReceipt, error) {
	var out []struct {
		Number  int64  `json:"number"`
		HTMLURL string `json:"html_url"`
		Draft   bool   `json:"draft"`
		State   string `json:"state"`
	}
	if err := c.call(ctx, http.MethodGet, "/repos/"+c.repoString()+"/pulls?head="+head+"&state=open", nil, &out); err != nil {
		return nil, err
	}
	for _, pr := range out {
		if pr.State != "open" {
			continue
		}
		return &PullRequestReceipt{Number: pr.Number, URL: pr.HTMLURL, Draft: pr.Draft, Created: true}, nil
	}
	return nil, nil
}

func (c *gitHubRestClient) CurrentLogin(ctx context.Context) (string, error) {
	var out struct {
		Login string `json:"login"`
	}
	err := c.call(ctx, http.MethodGet, "/user", nil, &out)
	return out.Login, err
}

// repoString derives the REST path segment from the pinned RepoRef; the
// factory guarantees it was set at construction (ParseRepoRef-validated).
func (c *gitHubRestClient) repoString() string { return c.repo.String() }

func asGitHubAPIError(err error, target **GitHubAPIError) bool {
	apiErr, ok := err.(*GitHubAPIError)
	if ok {
		*target = apiErr
	}
	return ok
}
```

注意：模拟器的 blackout 用 `panic(http.ErrAbortHandler)` 断连——生产客户端把这类传输失败映射为 `ErrGitHubTransport`（不可观测），Task 6 依赖这一语义。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/codedelivery/ -count=1`
Expected: PASS（Task 1 + Task 2 测试全绿；`TestGitHubClientAgainstRealGitHub` skip）

- [ ] **Step 5: Commit**

```bash
git add internal/modules/codedelivery/github.go internal/modules/codedelivery/github_client.go internal/modules/codedelivery/github_wire_test.go internal/modules/codedelivery/github_real_test.go
git commit -m "feat(codedelivery): github git-data client port with definite-vs-unobservable error taxonomy, wire-tested against an httptest emulator (T22 #52 task 2)"
```

### Task 3: Go——code_deliveries 迁移与追溯存储

**Files:**
- Create: `migrations/versioned/000192_code_deliveries.up.sql`
- Create: `migrations/versioned/000192_code_deliveries.down.sql`
- Create: `migrations/sqlite/000113_code_deliveries.up.sql`
- Create: `migrations/sqlite/000113_code_deliveries.down.sql`
- Create: `internal/modules/codedelivery/repository/codedelivery/store.go`
- Test: `internal/modules/codedelivery/repository/codedelivery/store_test.go`
- Test: `internal/modules/codedelivery/repository/codedelivery/migration_align_test.go`

**Interfaces:**
- Consumes: Task 1 的 `DeliveryState`/`RepoRef`；gorm（同 `internal/modules/appconnector/repository/appconnector/action.go` 的行映射风格）。
- Produces（Task 5/6/7 依赖的精确签名）:
  - `type DeliveryRow struct`（表 `code_deliveries`：`ID, TenantID, TaskID, RunID, OwnerID, ActionID, ConnectionID, Repo, BaselineSHA, Branch, CommitSHA, PRNumber, PRURL, RemoteLogin, State, Failure, CreatedAt, UpdatedAt`）
  - `var ErrDeliveryNotFound = errors.New("code_delivery_not_found")`；`var ErrDeliveryStateConflict = errors.New("code_delivery_state_conflict")`
  - `func NewDeliveryStore(db *gorm.DB) *DeliveryStore`
  - `func (s *DeliveryStore) CreateDelivery(ctx context.Context, row DeliveryRow) error`
  - `func (s *DeliveryStore) GetDelivery(ctx context.Context, tenantID uint64, id string) (DeliveryRow, error)`
  - `func (s *DeliveryStore) LatestForRun(ctx context.Context, tenantID uint64, runID string) (DeliveryRow, error)`
  - `func (s *DeliveryStore) TransitionState(ctx context.Context, tenantID uint64, id string, from []string, to, failure string) error`（CAS：`state IN from` 才更新）
  - `func (s *DeliveryStore) RecordReceipts(ctx context.Context, tenantID, id string, update ReceiptUpdate) error`（`type ReceiptUpdate struct{ CommitSHA string; PRNumber int64; PRURL, RemoteLogin string }`，仅非零字段更新）
  - `func (s *DeliveryStore) LatestApproverForAction(ctx context.Context, actionID string) (string, bool, error)`（join `app_action_approvals`，参数绑定）
  - `func (s *DeliveryStore) DB() *gorm.DB`（Task 7 读面 join 用）

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/codedelivery/repository/codedelivery/store_test.go`（自包含 sqlite 夹具——全量迁移轨道被预存在 000112 撞号破坏，见差异记录第 5 条，故 AutoMigrate）：

```go
package codedelivery

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openDeliveryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "deliveries.db") + "?_foreign_keys=on&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&DeliveryRow{}, &ApprovalProbeRow{}))
	return db
}

// ApprovalProbeRow 只为测试注入 app_action_approvals 形状的行（生产表由
// appconnector 迁移创建；LatestApproverForAction 按表名读）。
type ApprovalProbeRow struct {
	ArgsDigest string    `gorm:"primaryKey;column:args_digest"`
	ActionID   string    `gorm:"column:action_id"`
	Actor      string    `gorm:"column:actor"`
	Expiry     time.Time `gorm:"column:expiry"`
	Remaining  int64     `gorm:"column:remaining"`
}

func (ApprovalProbeRow) TableName() string { return "app_action_approvals" }

func TestDeliveryStoreLifecycleAndCAS(t *testing.T) {
	db := openDeliveryTestDB(t)
	store := NewDeliveryStore(db)
	ctx := context.Background()

	row := DeliveryRow{
		ID: "dlv-1", TenantID: 7, TaskID: "s-1", RunID: "r-1", OwnerID: "u1",
		ActionID: "act-1", ConnectionID: "conn-1", Repo: "octocat/hello",
		BaselineSHA: "b0000000000000000000000000000000000000000",
		Branch: "weknora/task/s-1", State: "prepared",
	}
	require.NoError(t, store.CreateDelivery(ctx, row))

	got, err := store.GetDelivery(ctx, 7, "dlv-1")
	require.NoError(t, err)
	require.Equal(t, "prepared", got.State)

	// 跨租户读 = 未找到（不泄漏存在性）。
	_, err = store.GetDelivery(ctx, 8, "dlv-1")
	require.ErrorIs(t, err, ErrDeliveryNotFound)

	// CAS：prepared→dispatched 成功；prepared→delivered（非法源态）失败。
	require.NoError(t, store.TransitionState(ctx, 7, "dlv-1", []string{"prepared"}, "dispatched", ""))
	require.ErrorIs(t, store.TransitionState(ctx, 7, "dlv-1", []string{"prepared"}, "delivered", ""),
		ErrDeliveryStateConflict)

	// 回执：仅非零字段落账。
	require.NoError(t, store.RecordReceipts(ctx, 7, "dlv-1", ReceiptUpdate{
		CommitSHA: "c1", PRNumber: 3, PRURL: "https://github.com/octocat/hello/pull/3", RemoteLogin: "octocat",
	}))
	got, err = store.LatestForRun(ctx, 7, "r-1")
	require.NoError(t, err)
	require.Equal(t, "c1", got.CommitSHA)
	require.EqualValues(t, 3, got.PRNumber)
	require.Equal(t, "octocat", got.RemoteLogin)

	// 审批者 join：按 expiry 最新一条。
	require.NoError(t, db.Create(&ApprovalProbeRow{ArgsDigest: "d1", ActionID: "act-1", Actor: "u1", Expiry: time.Now().Add(time.Hour), Remaining: 1}).Error)
	require.NoError(t, db.Create(&ApprovalProbeRow{ArgsDigest: "d2", ActionID: "act-1", Actor: "approver@7", Expiry: time.Now().Add(2 * time.Hour), Remaining: 1}).Error)
	approver, ok, err := store.LatestApproverForAction(ctx, "act-1")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "approver@7", approver)

	_, ok, err = store.LatestApproverForAction(ctx, "act-none")
	require.NoError(t, err)
	require.False(t, ok)
}
```

创建 `internal/modules/codedelivery/repository/codedelivery/migration_align_test.go`（迁移 SQL↔模型对齐，绕开被 000112 撞号破坏的全量轨道：只把本计划迁移文件 exec 进干净 sqlite）：

```go
package codedelivery

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
)

// TestCodeDeliveriesMigrationSQLMatchesModel executes THIS plan's sqlite
// migration verbatim against a clean database and compares the resulting
// columns with the gorm model projection. (The full migration track is
// broken by the pre-existing 000112 duplicate — see plan 差异记录 5.)
func TestCodeDeliveriesMigrationSQLMatchesModel(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// 本文件位于 internal/modules/codedelivery/repository/codedelivery/，
	// 5 级 .. 回到 worktree 根。
	upPath := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "..", "migrations", "sqlite", "000113_code_deliveries.up.sql")
	raw, err := os.ReadFile(upPath)
	require.NoError(t, err, "migration file must exist: %s", upPath)

	db, err := sql.Open("sqlite3", "file:"+filepath.Join(t.TempDir(), "migrate.db")+"?_foreign_keys=on")
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(string(raw))
	require.NoError(t, err)

	rows, err := db.Query(`PRAGMA table_info(code_deliveries)`)
	require.NoError(t, err)
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue any
		var pk int
		require.NoError(t, rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk))
		columns[name] = true
	}
	for _, want := range []string{
		"id", "tenant_id", "task_id", "run_id", "owner_id", "action_id", "connection_id",
		"repo", "baseline_sha", "branch", "commit_sha", "pr_number", "pr_url",
		"remote_login", "state", "failure", "created_at", "updated_at",
	} {
		require.True(t, columns[want], "migration must create column %s (got %v)", want, columns)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/codedelivery/... -count=1`
Expected: FAIL（`repository/codedelivery` 包不存在，编译错误）

- [ ] **Step 3: 写最小实现**

`migrations/sqlite/000113_code_deliveries.up.sql`：

```sql
-- T22 (#52)：代码交付追溯行。task_id=sessionId（ADR-0004）；action_id 锚定
-- A03 审批（摘要/批准人），commit/pr/remote_login 是远端回执。
CREATE TABLE IF NOT EXISTS code_deliveries (
    id             VARCHAR(64)  NOT NULL PRIMARY KEY,
    tenant_id      BIGINT       NOT NULL,
    task_id        VARCHAR(64)  NOT NULL,
    run_id         VARCHAR(64)  NOT NULL,
    owner_id       VARCHAR(512) NOT NULL DEFAULT '',
    action_id      VARCHAR(64)  NOT NULL DEFAULT '',
    connection_id  VARCHAR(64)  NOT NULL DEFAULT '',
    repo           VARCHAR(256) NOT NULL DEFAULT '',
    baseline_sha   CHAR(40)     NOT NULL DEFAULT '',
    branch         VARCHAR(256) NOT NULL DEFAULT '',
    commit_sha     CHAR(40)     NOT NULL DEFAULT '',
    pr_number      BIGINT       NOT NULL DEFAULT 0,
    pr_url         VARCHAR(512) NOT NULL DEFAULT '',
    remote_login   VARCHAR(256) NOT NULL DEFAULT '',
    state          VARCHAR(32)  NOT NULL DEFAULT 'prepared',
    failure        VARCHAR(512) NOT NULL DEFAULT '',
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_code_deliveries_tenant_run ON code_deliveries (tenant_id, run_id, created_at);
```

`migrations/sqlite/000113_code_deliveries.down.sql`：

```sql
DROP INDEX IF EXISTS idx_code_deliveries_tenant_run;
DROP TABLE IF EXISTS code_deliveries;
```

`migrations/versioned/000192_code_deliveries.up.sql`（Postgres）：

```sql
-- T22 (#52)：代码交付追溯行（Postgres 轨）。
CREATE TABLE IF NOT EXISTS code_deliveries (
    id             VARCHAR(64)  NOT NULL PRIMARY KEY,
    tenant_id      BIGINT       NOT NULL,
    task_id        VARCHAR(64)  NOT NULL,
    run_id         VARCHAR(64)  NOT NULL,
    owner_id       VARCHAR(512) NOT NULL DEFAULT '',
    action_id      VARCHAR(64)  NOT NULL DEFAULT '',
    connection_id  VARCHAR(64)  NOT NULL DEFAULT '',
    repo           VARCHAR(256) NOT NULL DEFAULT '',
    baseline_sha   CHAR(40)     NOT NULL DEFAULT '',
    branch         VARCHAR(256) NOT NULL DEFAULT '',
    commit_sha     CHAR(40)     NOT NULL DEFAULT '',
    pr_number      BIGINT       NOT NULL DEFAULT 0,
    pr_url         VARCHAR(512) NOT NULL DEFAULT '',
    remote_login   VARCHAR(256) NOT NULL DEFAULT '',
    state          VARCHAR(32)  NOT NULL DEFAULT 'prepared',
    failure        VARCHAR(512) NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_code_deliveries_tenant_run ON code_deliveries (tenant_id, run_id, created_at);
```

`migrations/versioned/000192_code_deliveries.down.sql`：

```sql
DROP INDEX IF EXISTS idx_code_deliveries_tenant_run;
DROP TABLE IF EXISTS code_deliveries;
```

创建 `internal/modules/codedelivery/repository/codedelivery/store.go`：

```go
// Package codedelivery persists the delivery traceability rows (T22 #52):
// task/run identity, the A03 approval anchor, and the remote receipts
// (commit sha, draft PR, actual remote identity).
package codedelivery

import (
	"context"
	"errors"
	"time"

	codedelivery "github.com/Tencent/WeKnora/internal/modules/codedelivery"
	"gorm.io/gorm"
)

var (
	ErrDeliveryNotFound      = errors.New("code_delivery_not_found")
	ErrDeliveryStateConflict = errors.New("code_delivery_state_conflict")
)

// DeliveryRow is one code delivery bound to a task (session) and a run.
type DeliveryRow struct {
	ID           string `gorm:"primaryKey;column:id"`
	TenantID     uint64 `gorm:"column:tenant_id;not null"`
	TaskID       string `gorm:"column:task_id;not null"`
	RunID        string `gorm:"column:run_id;not null"`
	OwnerID      string `gorm:"column:owner_id;not null;default:''"`
	ActionID     string `gorm:"column:action_id;not null;default:''"`
	ConnectionID string `gorm:"column:connection_id;not null;default:''"`
	Repo         string `gorm:"column:repo;not null;default:''"`
	BaselineSHA  string `gorm:"column:baseline_sha;not null;default:''"`
	Branch       string `gorm:"column:branch;not null;default:''"`
	CommitSHA    string `gorm:"column:commit_sha;not null;default:''"`
	PRNumber     int64  `gorm:"column:pr_number;not null;default:0"`
	PRURL        string `gorm:"column:pr_url;not null;default:''"`
	RemoteLogin  string `gorm:"column:remote_login;not null;default:''"`
	State        string `gorm:"column:state;not null;default:'prepared'"`
	Failure      string `gorm:"column:failure;not null;default:''"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (DeliveryRow) TableName() string { return "code_deliveries" }

// ReceiptUpdate carries remote receipts; zero fields are left untouched.
type ReceiptUpdate struct {
	CommitSHA   string
	PRNumber    int64
	PRURL       string
	RemoteLogin string
}

// DeliveryStore is the persistence surface of the delivery module.
type DeliveryStore struct{ db *gorm.DB }

func NewDeliveryStore(db *gorm.DB) *DeliveryStore { return &DeliveryStore{db: db} }

// DB exposes the underlying handle for read-face joins (same package only).
func (s *DeliveryStore) DB() *gorm.DB { return s.db }

func (s *DeliveryStore) CreateDelivery(ctx context.Context, row DeliveryRow) error {
	return s.db.WithContext(ctx).Create(&row).Error
}

func (s *DeliveryStore) GetDelivery(ctx context.Context, tenantID uint64, id string) (DeliveryRow, error) {
	var row DeliveryRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DeliveryRow{}, ErrDeliveryNotFound
	}
	return row, err
}

func (s *DeliveryStore) LatestForRun(ctx context.Context, tenantID uint64, runID string) (DeliveryRow, error) {
	var row DeliveryRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND run_id = ?", tenantID, runID).
		Order("created_at DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DeliveryRow{}, ErrDeliveryNotFound
	}
	return row, err
}

// TransitionState is the CAS state machine write: the row must currently sit
// in one of `from`, otherwise ErrDeliveryStateConflict (nothing is written).
func (s *DeliveryStore) TransitionState(ctx context.Context, tenantID uint64, id string, from []string, to, failure string) error {
	if len(from) == 0 {
		return ErrDeliveryStateConflict
	}
	res := s.db.WithContext(ctx).Model(&DeliveryRow{}).
		Where("tenant_id = ? AND id = ? AND state IN ?", tenantID, id, from).
		Updates(map[string]any{"state": to, "failure": failure})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDeliveryStateConflict
	}
	return nil
}

// RecordReceipts writes remote receipts; only non-zero fields update.
func (s *DeliveryStore) RecordReceipts(ctx context.Context, tenantID, id string, update ReceiptUpdate) error {
	patch := map[string]any{}
	if update.CommitSHA != "" {
		patch["commit_sha"] = update.CommitSHA
	}
	if update.PRNumber != 0 {
		patch["pr_number"] = update.PRNumber
	}
	if update.PRURL != "" {
		patch["pr_url"] = update.PRURL
	}
	if update.RemoteLogin != "" {
		patch["remote_login"] = update.RemoteLogin
	}
	if len(patch) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&DeliveryRow{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).Updates(patch).Error
}

// LatestApproverForAction reads the most recent approval bound to the A03
// action (approval content traceability). Parameter-bound join on
// app_action_approvals.
func (s *DeliveryStore) LatestApproverForAction(ctx context.Context, actionID string) (string, bool, error) {
	var row struct {
		Actor string `gorm:"column:actor"`
	}
	err := s.db.WithContext(ctx).Table("app_action_approvals").
		Select("actor").
		Where("action_id = ?", actionID).
		Order("expiry DESC").Limit(1).Scan(&row).Error
	if err != nil {
		return "", false, err
	}
	return row.Actor, row.Actor != "", nil
}

// State constants mirror the module-level DeliveryState vocabulary.
const (
	StatePrepared   = string(codedelivery.DeliveryPrepared)
	StateDispatched = string(codedelivery.DeliveryDispatched)
	StatePushed     = string(codedelivery.DeliveryPushed)
	StateDelivered  = string(codedelivery.DeliveryDelivered)
	StateFailed     = string(codedelivery.DeliveryFailed)
	StateUnknown    = string(codedelivery.DeliveryUnknown)
)
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/codedelivery/... -count=1`
Expected: PASS（含迁移对齐测试——`upPath` 已按 `repository/codedelivery` → worktree 根的 5 级 `..` 写定）

- [ ] **Step 5: Commit**

```bash
git add migrations/versioned/000192_code_deliveries.up.sql migrations/versioned/000192_code_deliveries.down.sql migrations/sqlite/000113_code_deliveries.up.sql migrations/sqlite/000113_code_deliveries.down.sql internal/modules/codedelivery/repository/codedelivery/
git commit -m "feat(codedelivery): code_deliveries traceability table (versioned 000192 / sqlite 000113) + store with CAS receipts and approver join (T22 #52 task 3)"
```

---

### Task 4: Go——A03 词汇扩展与 GitHub OAuth 注册（共享文件最小修改）

**Files:**
- Modify: `internal/modules/appconnector/action.go:17-22`（风险常量块后追加）
- Modify: `internal/handler/app_connector_action.go:289-301`（ExecuteAction 状态校验后追加护栏）
- Modify: `internal/handler/app_connector_oauth.go:48-57`（defaults 加 github）、`:101-128`（exchange switch 加 github case）、`:199-207`（authorize query 加 github scope）
- Test: `internal/handler/app_connector_oauth_github_test.go`（Create，不动共享测试文件）
- Test: `internal/modules/appconnector/deliver_risk_test.go`（Create）

**Interfaces:**
- Consumes: `appOAuthDefaults`/`exchangeAppOAuthCode`/`createConnectionOAuth`（`internal/handler/app_connector_oauth.go`）；`NeedsExplicitApproval`（`internal/modules/appconnector/action.go:76-85`）。
- Produces（Task 5/6/7 依赖）:
  - `appconnector.RiskDeliver = "deliver"`——`NeedsExplicitApproval("deliver", true) == true`（永不落入通用 write 预授权）；`PrepareDelivery` 用它调 `ActionService.Prepare`。
  - `POST /apps/actions/:id/execute` 对 risk 不在通用词汇表（read/write/send/delete）内的 action 一律 409 `ACTION_WRONG_PIPELINE` 拒派（不消耗批准）。
  - `appOAuthDefaults["github"]`：AuthorizeURL `https://github.com/login/oauth/authorize`、TokenURL `https://github.com/login/oauth/access_token`；部署 env `WEKNORA_APP_OAUTH_GITHUB_CLIENT_ID/_SECRET`（container.go:1003-1012 的既有循环自动拾取，无需改容器）；authorize 附 `scope=repo read:user`；token 交换为 form-encoded + `Accept: application/json`。

- [ ] **Step 1: 写失败测试**

创建 `internal/handler/app_connector_oauth_github_test.go`：

```go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// GitHub OAuth 注册与交换形状：github 在默认端点表中、token 交换发
// form-encoded + Accept: application/json、authorize URL 带 repo scope。
func TestGitHubOAuthRegistrationAndExchangeShape(t *testing.T) {
	cfg := DefaultAppOAuthProviderConfigs()
	github, ok := cfg["github"]
	require.True(t, ok, "github must be a first-batch OAuth app")
	require.Equal(t, "https://github.com/login/oauth/authorize", github.AuthorizeURL)
	require.Equal(t, "https://github.com/login/oauth/access_token", github.TokenURL)
	_, known := appOAuthDefaults["github"]
	require.True(t, known)

	// 交换形状：对 httptest token 端点做真实 HTTP。
	var gotContentType, gotAccept string
	var gotForm map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotAccept = r.Header.Get("Accept")
		require.NoError(t, r.ParseForm())
		gotForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "gho_x", "token_type": "bearer", "scope": "repo read:user"})
	}))
	defer srv.Close()

	token, err := exchangeAppOAuthCode(t.Context(), AppOAuthProviderConfig{
		AuthorizeURL: github.AuthorizeURL, TokenURL: srv.URL,
		ClientID: "cid", ClientSecret: "csecret",
	}, "github", "the-code", "https://deploy.example.com/api/v1/apps/connections/oauth/callback")
	require.NoError(t, err)
	require.Equal(t, "gho_x", token.AccessToken)
	require.Equal(t, "application/x-www-form-urlencoded", gotContentType)
	require.Equal(t, "application/json", gotAccept)
	require.Equal(t, "cid", gotForm.Get("client_id"))
	require.Equal(t, "csecret", gotForm.Get("client_secret"))
	require.Equal(t, "the-code", gotForm.Get("code"))
}

func TestGitHubAuthorizeURLCarriesRepoScope(t *testing.T) {
	_, known := appOAuthDefaults["github"]
	require.True(t, known, "github must be a first-batch OAuth app")
	cfg := AppOAuthProviderConfig{
		AuthorizeURL: "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
		ClientID:     "cid", ClientSecret: "csecret",
	}
	url := appAuthorizeURL(cfg, "github", "st-1", "https://deploy.example.com/cb")
	require.Contains(t, url, "https://github.com/login/oauth/authorize?")
	require.Contains(t, url, "scope=repo+read%3Auser")
	require.Contains(t, url, "state=st-1")
	require.Contains(t, url, "client_id=cid")
}

创建 `internal/modules/appconnector/deliver_risk_test.go`：

```go
package appconnector

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// RiskDeliver 永远需要逐次人工审批：任何 write 预授权都不覆盖它
// （NeedsExplicitApproval 的 default 分支），这是「交付前必须审阅
// Diff/提交」的结构性保证。
func TestRiskDeliverAlwaysNeedsExplicitApproval(t *testing.T) {
	require.Equal(t, "deliver", RiskDeliver)
	require.True(t, NeedsExplicitApproval(RiskDeliver, false))
	require.True(t, NeedsExplicitApproval(RiskDeliver, true), "a general write grant must never cover delivery")
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/appconnector/ -run TestRiskDeliver -count=1 && go test ./internal/handler/ -run TestGitHubOAuth -count=1`
Expected: FAIL（`RiskDeliver` 未定义、`github` 不在 defaults，编译错误）

- [ ] **Step 3: 写最小实现**

`internal/modules/appconnector/action.go` 风险常量块（`RiskDelete = "delete"` 之后）追加：

```go
	// RiskDeliver 是代码交付（T22 #52）的专用风险类：它不属于
	// read/write/send/delete 通用类，NeedsExplicitApproval 的 default 分支
	// 使它永远需要逐次人工审批——任何范围预授权（包括 write 预授权）都不
	// 覆盖它；交付前审阅 Diff/提交是每次交付的前置条件（CONTEXT.md
	// 代码交付审批）。
	RiskDeliver = "deliver"
```

`internal/handler/app_connector_action.go` 的 `ExecuteAction` 中，`if !appActionStates[row.State] { … return }` 之后追加：

```go
	// T22 (#52)：risk 超出通用词汇表（如 deliver）的动作由其所属专用管线
	// （代码交付端点）派发；经本通用端点执行只会在 OC dispatcher 的
	// ErrDispatchNotStarted 处白白消耗一次批准并落账 failed。此处 fail
	// closed 拒绝——不消费批准、不派发。
	if !appActionRisks[row.Risk] {
		appFail(c, http.StatusConflict, "ACTION_WRONG_PIPELINE",
			"this action belongs to its owning pipeline; dispatch it through that pipeline's endpoint")
		return
	}
```

`internal/handler/app_connector_oauth.go` 三处：

1. `appOAuthDefaults` 加条目：

```go
	"github": {
		AuthorizeURL: "https://github.com/login/oauth/authorize",
		TokenURL:     "https://github.com/login/oauth/access_token",
	},
```

2. `exchangeAppOAuthCode` 的 switch 加 case（放在 `"notion"` case 旁）：

```go
	case "github":
		form := url.Values{}
		form.Set("client_id", cfg.ClientID)
		form.Set("client_secret", cfg.ClientSecret)
		form.Set("code", code)
		form.Set("redirect_uri", redirectURI)
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
```

3. `createConnectionOAuth` 的 query 组装段重构为共享纯函数并加 github scope：把现有

```go
	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	if appID == "notion" {
		q.Set("response_type", "code")
		q.Set("owner", "user")
	}
	authorizeURL := cfg.AuthorizeURL + "?" + q.Encode()
	return state, authorizeURL, expires, nil
```

替换为：

```go
	return state, appAuthorizeURL(cfg, appID, state, redirectURI), expires, nil
```

并在文件中新增（`createConnectionOAuth` 之后）：

```go
// appAuthorizeURL builds the provider authorize URL. github requests the
// repo + read:user scope (code delivery writes task branches and draft PRs;
// read:user records the actual remote identity).
func appAuthorizeURL(cfg AppOAuthProviderConfig, appID, state, redirectURI string) string {
	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	switch appID {
	case "notion":
		q.Set("response_type", "code")
		q.Set("owner", "user")
	case "github":
		q.Set("scope", "repo read:user")
	}
	return cfg.AuthorizeURL + "?" + q.Encode()
}
```

（该重构对 feishu/notion 是等价移动——`TestAppConnectorOAuth*` 既有用例不回归即为证明。）

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/appconnector/ -run TestRiskDeliver -count=1 && go test ./internal/handler/ -run 'TestGitHubOAuth|TestAppAction|TestAppConnectorOAuth' -count=1`
Expected: PASS（`appActionRisks` 未增删通用词条，既有系列不回归；新增的 github 注册/交换形状/scope 用例全绿）

- [ ] **Step 5: Commit**

```bash
git add internal/modules/appconnector/action.go internal/modules/appconnector/deliver_risk_test.go internal/handler/app_connector_action.go internal/handler/app_connector_oauth.go internal/handler/app_connector_oauth_github_test.go
git commit -m "feat(codedelivery): A03 deliver risk vocabulary + wrong-pipeline dispatch guard + github oauth registration (T22 #52 task 4)"
```

### Task 5: Go——交付编排 A：工作区端口、基线物化与 PrepareDelivery

**Files:**
- Create: `internal/modules/codedelivery/workspace.go`
- Create: `internal/modules/codedelivery/workspace_local.go`
- Create: `internal/modules/codedelivery/service.go`（本任务只实现 MaterializeBaseline/PrepareDelivery；Task 6 补 Dispatch/Resolve）
- Test: `internal/modules/codedelivery/service_prepare_test.go`

**Interfaces:**
- Consumes（全部当前 HEAD 亲眼核实）:
  - Task 1 `ParseRepoRef`/`RefuseProtectedTarget`/`TaskBranchOf`/`GitBlobSHA`/`DiffAgainstBaseline`/`ParseDeliveryMaterial`/`WorkspaceRepoRoot`/`MaxDeliveryFiles`、Task 2 `GitHubClientFactory`/`GitHubClient`、Task 3 `DeliveryStore`。
  - `appconnectorsvc.ActionService.Prepare(ctx, a appconnector.Action) (string, error)`、`ActionStoreSource.FindAction(ctx, id) (repoappconn.ActionRow, error)`（`internal/modules/appconnector/service/appconnector/action.go:215-233/:142-152`）；`appconnectorsvc.CredentialResolver`（`credentials.go:21-24`）。
  - `appconnector.Connection/ConnectionKindPersonal/ConnectionActive`（`internal/modules/appconnector/model.go:41-50`）。
  - `agentruntime.Run{Key, SessionID, Owner}`（`internal/modules/agentruntime/agent/runtime/contracts.go:114-133`）——经本模块端口 `RunReader.GetOwnedRun(ctx, tenantID, ownerID, runID) (agentruntime.Run, error)`（与 `session.OwnedRunReader` 同形，Task 7 复用 `*repository.AgentRunStore`）。
- Produces（Task 6/7 依赖的精确签名）:
  - `type WorkspaceDirEntry struct{ Path string; IsDir bool; Size int64 }`；`type WorkspaceFileWrite struct{ Path string; Content []byte }`
  - `type WorkspaceFileSource interface { ListSessionFiles(ctx, sessionID, dir string) ([]WorkspaceDirEntry, error); ReadSessionFile(ctx, sessionID, path string) ([]byte, error); WriteSessionWorkspaceFiles(ctx, sessionID string, files []WorkspaceFileWrite) error }`（生产由 `*sandbox.SessionBoundManager` 满足——适配器在 Task 7 的 container 包，本模块不 import sandbox）
  - `func NewLocalWorkspaceSource(root string) (WorkspaceFileSource, error)`（本地目录适配器：路径穿越防护）
  - `var ErrNotDeliveryOwner = errors.New("code_delivery_not_owner")`；`var ErrWorkspaceUnavailable = errors.New("code_delivery_workspace_unavailable")`；`var ErrConnectionNotUsable = errors.New("code_delivery_connection_not_usable")`
  - `type RunReader interface { GetOwnedRun(ctx context.Context, tenantID uint64, ownerID, runID string) (agentruntime.Run, error) }`
  - `type ConnectionReader interface { FindConnectionByID(ctx context.Context, connectionID string) (appconnector.Connection, error) }`
  - `type CodeDeliveryDeps struct { Store *deliveryrepo.DeliveryStore; Actions *appconnectorsvc.ActionService; ActionRows appconnectorsvc.ActionStoreSource; Connections ConnectionReader; Creds appconnectorsvc.CredentialResolver; GitHub GitHubClientFactory; Workspace WorkspaceFileSource; Runs RunReader }`
  - `func NewCodeDeliveryService(deps CodeDeliveryDeps) *CodeDeliveryService`
  - `type BaselineInput struct { TenantID uint64; CallerID, RunID, ConnectionID string; Repo RepoRef; BaselineSHA string }`
  - `type BaselineReceipt struct { Files int; Root string }`
  - `func (s *CodeDeliveryService) MaterializeBaseline(ctx context.Context, in BaselineInput) (BaselineReceipt, error)`
  - `type PrepareInput struct { TenantID uint64; CallerID, RunID, ConnectionID string; Repo RepoRef; BaselineSHA string; Branch string /*空=TaskBranchOf(sessionID)*/; CommitMessage, PRTitle string }`
  - `type DeliveryView struct { ID, TaskID, RunID, State, Repo, BaselineSHA, Branch, CommitSHA, PRURL, RemoteLogin, ActionID, ActionState, Digest, Approver, Failure string; PRNumber int64; Files int; CreatedAt, UpdatedAt string }`
  - `func (s *CodeDeliveryService) PrepareDelivery(ctx context.Context, in PrepareInput) (DeliveryView, error)`
  - `func (s *CodeDeliveryService) GetDeliveryForRun(ctx context.Context, tenantID uint64, runID string) (DeliveryView, error)`（无 delivery 时 `ErrDeliveryNotFound`）
  - `func (s *CodeDeliveryService) GetDelivery(ctx context.Context, tenantID uint64, deliveryID string) (DeliveryView, error)`（Task 6 补实现亦可，签名在此冻结）
  - `func (s *CodeDeliveryService) viewOf(ctx context.Context, row deliveryrepo.DeliveryRow) (DeliveryView, error)`（join action 行 + 审批者；Task 6/7 复用）

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/codedelivery/service_prepare_test.go`（真实 sqlite 行为 + 真实 A03 ActionStore + 真实 ocAuthorizer + httptest GitHub 模拟器 + 本地目录工作区；夹具与 Task 2 模拟器复用）：

```go
package codedelivery

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --- 夹具：真实 sqlite（AutoMigrate，全量迁移轨道被预存在 000112 撞号
// 破坏——见计划差异记录 5）+ 真实 A03 行为 + 真实 ocAuthorizer ---

type fixtureRun struct{ sessionID string }

func (f fixtureRun) GetOwnedRun(ctx context.Context, tenantID uint64, ownerID, runID string) (agentruntime.Run, error) {
	if runID != "run-1" || ownerID != "u1" || tenantID != 7 {
		return agentruntime.Run{}, errors.New("run_not_found")
	}
	return agentruntime.Run{Key: agentruntime.RunKey{TenantID: tenantID, RunID: runID}, SessionID: f.sessionID, Owner: ownerID}, nil
}

// fixtureConnections 同时实现 ConnectionReader 与 CredentialResolver（与生产
// MCPOAuthBindingStore 的双角色一致）；指针接收者使 membersDrop 生效。
type fixtureConnections struct {
	db      *gorm.DB
	members map[string]bool
}

func (f *fixtureConnections) FindConnectionByID(ctx context.Context, id string) (appconnector.Connection, error) {
	var row appconnectorrepo.ConnectionRow
	if err := f.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return appconnector.Connection{}, err
	}
	return appconnector.Connection{
		ID: row.ID, InstallationID: row.InstallationID, Kind: row.Kind,
		OwnerID: row.OwnerID, CredentialRef: row.CredentialRef,
		State: row.State, TenantID: row.TenantID, AuthVersion: row.AuthVersion,
	}, nil
}
func (f *fixtureConnections) LoadCredential(ctx context.Context, c appconnector.Connection) ([]byte, error) {
	return []byte("gho_testtoken"), nil
}
func (f *fixtureConnections) MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	return f.members[userID], nil
}
func (f *fixtureConnections) TryAcquireRefreshLease(ctx context.Context, c appconnector.Connection, leaseID string, until time.Time) (bool, error) {
	return true, nil
}
func (f *fixtureConnections) Resolve(ctx context.Context, connectionID string, expectedVersion int64) ([]byte, error) {
	return []byte("gho_testtoken"), nil
}
func (f *fixtureConnections) drop(userID string) { delete(f.members, userID) }

type deliveryFixture struct {
	db          *gorm.DB
	store       *deliveryrepo.DeliveryStore
	actions     *appconnectorsvc.ActionService
	svc         *CodeDeliveryService
	github      *githubEmulator
	workspace   WorkspaceFileSource
	root        string
	connections *fixtureConnections
}

// newDeliveryFixture 装配真实 sqlite（AutoMigrate）+ 真实 A03 ActionStore +
// 真实 ocAuthorizer + httptest GitHub 模拟器 + 本地目录工作区。
// 可选 dispatcher 参数：Task 5 阶段省略（NewActionService 允许 nil
// dispatcher，本任务只测物化/prepare 路径）；Task 6 会把本函数改造为
// 「dispatcher 内部单实例构造」（见 Task 6 Step 3 的夹具改造——所有底层件
// 同源，杜绝 db/模拟器双实例分裂）。
func newDeliveryFixture(t *testing.T, mutate func(root string), dispatcher ...appconnectorsvc.ActionDispatcher) *deliveryFixture {
	t.Helper()
	var dispatch appconnectorsvc.ActionDispatcher
	if len(dispatcher) > 0 {
		dispatch = dispatcher[0]
	}
	dsn := "file:" + filepath.Join(t.TempDir(), "svc.db") + "?_foreign_keys=on&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&appconnectorrepo.ActionRow{}, &appconnectorrepo.ApprovalRow{}, &appconnectorrepo.PreAuthorizationRow{},
		&appconnectorrepo.ConnectionRow{}, &appconnectorrepo.InstallationRow{}, &deliveryrepo.DeliveryRow{},
	))
	// 个人 GitHub 连接：inst-gh / conn-gh，owner=u1，active。
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-gh", AppID: "github", AppVersion: "1", State: appconnector.InstallationActive, TenantID: 7}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{
		ID: "conn-gh", InstallationID: "inst-gh", Kind: appconnector.ConnectionKindPersonal,
		OwnerID: "u1", CredentialRef: "mcp:conn-gh:github", State: appconnector.ConnectionActive,
		TenantID: 7, AuthVersion: 1,
	}).Error)

	e := newGitHubEmulator(t)
	root := t.TempDir()
	if mutate != nil {
		mutate(root)
	}
	workspace, err := NewLocalWorkspaceSource(root)
	require.NoError(t, err)

	actionStore := appconnectorrepo.NewActionStore(db)
	connections := &fixtureConnections{db: db, members: map[string]bool{"u1": true, "u2": true}}
	guard := appconnectorsvc.NewSubjectGuard(connections) // 单参 permission-only guard：个人连接 owner-only + 成员资格
	factory := NewGitHubClientFactory(http.DefaultClient, e.srv.URL)
	var unknown appconnectorsvc.UnknownResolver
	if dispatch != nil {
		if resolver, ok := dispatch.(appconnectorsvc.UnknownResolver); ok {
			unknown = resolver
		}
	}
	actions := appconnectorsvc.NewActionService(actionStore, guard, nil, dispatch, unknown)
	store := deliveryrepo.NewDeliveryStore(db)
	svc := NewCodeDeliveryService(CodeDeliveryDeps{
		Store: store, Actions: actions, ActionRows: actionStore,
		Connections: connections, Creds: connections,
		GitHub: factory, Workspace: workspace, Runs: fixtureRun{sessionID: "s-1"},
	})
	return &deliveryFixture{db: db, store: store, actions: actions, svc: svc, github: e, workspace: workspace, root: root, connections: connections}
}

func membersDrop(f *deliveryFixture, userID string) { f.connections.drop(userID) }

func baselineInput() BaselineInput {
	return BaselineInput{
		TenantID: 7, CallerID: "u1", RunID: "run-1", ConnectionID: "conn-gh",
		Repo: RepoRef{Owner: "octocat", Name: "hello"},
		BaselineSHA: "b" + strings.Repeat("0", 39),
	}
}

func prepareInput() PrepareInput {
	return PrepareInput{
		TenantID: 7, CallerID: "u1", RunID: "run-1", ConnectionID: "conn-gh",
		Repo: RepoRef{Owner: "octocat", Name: "hello"},
		BaselineSHA: "b" + strings.Repeat("0", 39),
		CommitMessage: "fix: greeting", PRTitle: "WeKnora task s-1",
	}
}

func TestMaterializeBaselineWritesFixedTreeIntoWorkspace(t *testing.T) {
	f := newDeliveryFixture(t, nil)
	ctx := context.Background()

	receipt, err := f.svc.MaterializeBaseline(ctx, baselineInput())
	require.NoError(t, err)
	require.Equal(t, 2, receipt.Files)
	require.Equal(t, "/workspace/octocat/hello", receipt.Root)
	raw, err := os.ReadFile(filepath.Join(f.root, "octocat/hello/main.go"))
	require.NoError(t, err)
	require.Equal(t, "package main\n", string(raw))
}

func TestMaterializeBaselineRejectsNonOwnerAndForeignConnection(t *testing.T) {
	f := newDeliveryFixture(t, nil)
	ctx := context.Background()

	// 非本人 run（u2 调用 owner=u1 的 run）：run 归属谓词拒绝。
	in := baselineInput()
	in.CallerID = "u2"
	_, err := f.svc.MaterializeBaseline(ctx, in)
	require.Error(t, err) // fixtureRun 对非 owner 返回 run_not_found

	// 连接不是个人连接或调用者非 owner：ErrConnectionNotUsable。
	require.NoError(t, f.db.Create(&appconnectorrepo.ConnectionRow{
		ID: "conn-other", InstallationID: "inst-gh", Kind: appconnector.ConnectionKindPersonal,
		OwnerID: "u2", CredentialRef: "mcp:conn-other:github", State: appconnector.ConnectionActive, TenantID: 7, AuthVersion: 1,
	}).Error)
	in = baselineInput()
	in.ConnectionID = "conn-other"
	_, err = f.svc.MaterializeBaseline(ctx, in)
	require.ErrorIs(t, err, ErrConnectionNotUsable)
}

func TestPrepareDeliveryAnchorsApprovalAndDiff(t *testing.T) {
	// 工作区已在基线之上修改 main.go 并新增 util.go（经物化后覆写）。
	f := newDeliveryFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hello\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "util.go"), []byte("package main\n\nfunc util() {}\n"), 0o644))
	})
	ctx := context.Background()

	view, err := f.svc.PrepareDelivery(ctx, prepareInput())
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPrepared), view.State)
	require.Equal(t, "weknora/task/s-1", view.Branch) // 默认分支=TaskBranchOf(sessionID)
	require.Equal(t, 2, view.Files)                   // main.go 修改 + util.go 新增
	require.NotEmpty(t, view.ActionID)
	require.NotEmpty(t, view.Digest)
	require.Equal(t, "awaiting_approval", view.ActionState)

	// A03 行真实落库：risk=deliver、target=github.deliver、args=归一化材料。
	var row appconnectorrepo.ActionRow
	require.NoError(t, f.db.Where("id = ?", view.ActionID).First(&row).Error)
	require.Equal(t, "deliver", row.Risk)
	require.Equal(t, "github.deliver", row.Target)
	mat, err := ParseDeliveryMaterial([]byte(row.ArgsSnapshot))
	require.NoError(t, err)
	require.Len(t, mat.Files, 2)
}

func TestPrepareDeliveryRefusesProtectedBranchWithZeroRemoteWrites(t *testing.T) {
	f := newDeliveryFixture(t, nil)
	ctx := context.Background()

	// 仓库默认分支是 main；显式指定 branch=main 必须以 ErrProtectedBranch
	// 拒绝（实现把默认分支/保护判定放在前缀白名单之前）且零 ref 写。
	in := prepareInput()
	in.Branch = "main"
	_, err := f.svc.PrepareDelivery(ctx, in)
	require.ErrorIs(t, err, ErrProtectedBranch)

	// 分支远端标记 protected=true（模拟器对 prod 返回 protected）。
	in = prepareInput()
	in.Branch = "weknora/task/prod-branch"
	f.github.protectBranch("prod-branch")
	_, err = f.svc.PrepareDelivery(ctx, in)
	require.ErrorIs(t, err, ErrProtectedBranch)

	require.Zero(t, f.github.Calls()["POST /git/refs"])
	require.Zero(t, f.github.Calls()["PATCH /git/refs"])
	require.Zero(t, f.github.Calls()["POST /pulls"])
	require.Empty(t, f.github.Violations())
}

func TestPrepareDeliveryAlwaysAwaitsApprovalEvenWithWritePreAuthorization(t *testing.T) {
	f := newDeliveryFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	// 租户级 write 预授权（甚至 deliver 预授权）存在时……deliver 不在白名单
	// 风险类中，PreAuthorizationCovers 按 AllowedRisks 精确匹配，不会命中。
	require.NoError(t, f.db.Create(&appconnectorrepo.PreAuthorizationRow{
		ID: "pre-1", TenantID: 7, AllowedRisksJSON: `["write"]`, ConnectionID: "*", TargetScope: "*",
		ValidFrom: time.Now().Add(-time.Minute), ValidUntil: time.Now().Add(time.Hour), BudgetCapMicro: 0,
	}).Error)
	ctx := context.Background()
	view, err := f.svc.PrepareDelivery(ctx, prepareInput())
	require.NoError(t, err)
	require.Equal(t, "awaiting_approval", view.ActionState, "a write pre-authorization must never auto-authorize delivery")
}

func TestLocalWorkspaceSourceRefusesTraversal(t *testing.T) {
	root := t.TempDir()
	src, err := NewLocalWorkspaceSource(root)
	require.NoError(t, err)
	err = src.WriteSessionWorkspaceFiles(context.Background(), "s-1", []WorkspaceFileWrite{{Path: "../escape.txt", Content: []byte("x")}})
	require.ErrorIs(t, err, ErrInvalidMaterial)
}
```

（模拟器的 `protectBranch` 辅助已在 Task 2 的 `github_wire_test.go` 中就位。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/codedelivery/ -run 'TestMaterializeBaseline|TestPrepareDelivery|TestLocalWorkspace' -count=1`
Expected: FAIL（`NewCodeDeliveryService`/`NewLocalWorkspaceSource` 未定义，编译错误）

- [ ] **Step 3: 写最小实现**

创建 `internal/modules/codedelivery/workspace.go`：

```go
package codedelivery

import (
	"context"
	"errors"
)

// WorkspaceDirEntry is the provider-neutral workspace listing entry.
type WorkspaceDirEntry struct {
	Path  string
	IsDir bool
	Size  int64
}

// WorkspaceFileWrite is one workspace file mutation.
type WorkspaceFileWrite struct {
	Path    string
	Content []byte
}

// WorkspaceFileSource is the workspace port of the delivery module: the
// session-owned cloud workspace the run modified (CONTEXT.md 工作区). In
// production it is satisfied by *sandbox.SessionBoundManager (the container
// adapter lives in internal/container, mirroring the artifact collector's
// SandboxArtifactSource precedent); tests use NewLocalWorkspaceSource.
// This module never imports the sandbox package.
type WorkspaceFileSource interface {
	ListSessionFiles(ctx context.Context, sessionID, dir string) ([]WorkspaceDirEntry, error)
	ReadSessionFile(ctx context.Context, sessionID, path string) ([]byte, error)
	WriteSessionWorkspaceFiles(ctx context.Context, sessionID string, files []WorkspaceFileWrite) error
}

// ErrWorkspaceUnavailable marks a session without a usable live workspace.
var ErrWorkspaceUnavailable = errors.New("code_delivery_workspace_unavailable")
```

创建 `internal/modules/codedelivery/workspace_local.go`：

```go
package codedelivery

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// NewLocalWorkspaceSource maps the workspace port onto a local directory
// (tests, single-box dev). Paths are sandbox-relative ("/workspace/…" or
// session-relative); traversal outside the root is refused.
func NewLocalWorkspaceSource(root string) (WorkspaceFileSource, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &localWorkspaceSource{root: abs}, nil
}

type localWorkspaceSource struct{ root string }

func (s *localWorkspaceSource) resolve(p string) (string, error) {
	clean := path.Clean(strings.TrimPrefix(strings.TrimSpace(p), "/workspace"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "..") {
		return "", fmt.Errorf("%w: illegal workspace path %q", ErrInvalidMaterial, p)
	}
	return filepath.Join(s.root, filepath.FromSlash(clean)), nil
}

func (s *localWorkspaceSource) ListSessionFiles(ctx context.Context, sessionID, dir string) ([]WorkspaceDirEntry, error) {
	root, err := s.resolve(dir)
	if err != nil {
		return nil, err
	}
	var out []WorkspaceDirEntry
	// 返回路径与会话沙箱同约定：以请求目录为前缀（"/workspace/<owner>/<name>/…"），
	// 使 DiffAgainstBaseline 的 repo-root 前缀裁剪对两种 Adapter 一致。
	prefix := path.Clean(strings.TrimPrefix(strings.TrimSpace(dir), "/workspace")) 
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // nothing materialized yet
			}
			return err
		}
		if p == root {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		out = append(out, WorkspaceDirEntry{Path: prefix + "/" + path.Clean(filepath.ToSlash(rel)), IsDir: d.IsDir(), Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *localWorkspaceSource) ReadSessionFile(ctx context.Context, sessionID, p string) ([]byte, error) {
	full, err := s.resolve(p)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

func (s *localWorkspaceSource) WriteSessionWorkspaceFiles(ctx context.Context, sessionID string, files []WorkspaceFileWrite) error {
	for _, f := range files {
		full, err := s.resolve(f.Path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, f.Content, 0o644); err != nil {
			return err
		}
	}
	return nil
}
```

创建 `internal/modules/codedelivery/service.go`：

```go
package codedelivery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"
	"github.com/google/uuid"
)

// Delivery target constant of every A03 action this module prepares.
const DeliveryActionTarget = "github.deliver"

var (
	// ErrNotDeliveryOwner: caller is not the run owner (initiator predicate).
	ErrNotDeliveryOwner = errors.New("code_delivery_not_owner")
	// ErrConnectionNotUsable: not a personal connection of the caller, or not active.
	ErrConnectionNotUsable = errors.New("code_delivery_connection_not_usable")
)

// RunReader mirrors session.OwnedRunReader (production: *repository.AgentRunStore).
type RunReader interface {
	GetOwnedRun(ctx context.Context, tenantID uint64, ownerID, runID string) (agentruntime.Run, error)
}

// ConnectionReader is the connection lookup the owner-only rule needs.
type ConnectionReader interface {
	FindConnectionByID(ctx context.Context, connectionID string) (appconnector.Connection, error)
}

// CodeDeliveryDeps wires the module. Actions is the DELIVERY-DEDICATED
// ActionService instance (its dispatcher becomes the DeliveryDispatcher in
// Task 6); the container builds it so the global OC dispatcher is untouched.
// Creds resolves the delivery token AFTER the permission chain — the bytes
// never enter any response, log, or the sandbox.
type CodeDeliveryDeps struct {
	Store       *deliveryrepo.DeliveryStore
	Actions     *appconnectorsvc.ActionService
	ActionRows  appconnectorsvc.ActionStoreSource
	Connections ConnectionReader
	Creds       appconnectorsvc.CredentialResolver
	GitHub      GitHubClientFactory
	Workspace   WorkspaceFileSource
	Runs        RunReader
}

type CodeDeliveryService struct{ deps CodeDeliveryDeps }

func NewCodeDeliveryService(deps CodeDeliveryDeps) *CodeDeliveryService {
	return &CodeDeliveryService{deps: deps}
}

// BaselineInput names the fixed baseline to materialize.
type BaselineInput struct {
	TenantID     uint64
	CallerID     string
	RunID        string
	ConnectionID string
	Repo         RepoRef
	BaselineSHA  string
}

type BaselineReceipt struct {
	Files int
	Root  string
}

// MaterializeBaseline writes the fixed baseline tree into the run's session
// workspace (User Story 41: execution starts from a reproducible state).
// Owner-only: the caller must own the run AND the personal connection.
func (s *CodeDeliveryService) MaterializeBaseline(ctx context.Context, in BaselineInput) (BaselineReceipt, error) {
	sessionID, err := s.authorize(ctx, in.TenantID, in.CallerID, in.RunID, in.ConnectionID)
	if err != nil {
		return BaselineReceipt{}, err
	}
	if !baselineSHALegal(in.BaselineSHA) {
		return BaselineReceipt{}, fmt.Errorf("%w: %q", ErrInvalidBaselineSHA, in.BaselineSHA)
	}
	token, err := s.tokenFor(ctx, in.ConnectionID)
	if err != nil {
		return BaselineReceipt{}, err
	}
	client := s.deps.GitHub(token, in.Repo)
	tree, err := client.Tree(ctx, in.BaselineSHA)
	if err != nil {
		return BaselineReceipt{}, err
	}
	if len(tree) > MaxDeliveryFiles {
		return BaselineReceipt{}, fmt.Errorf("%w: baseline has %d files", ErrBaselineTooLarge, len(tree))
	}
	root := WorkspaceRepoRoot(in.Repo)
	files := make([]WorkspaceFileWrite, 0, len(tree))
	total := 0
	for p, blobSHA := range tree {
		content, err := client.Blob(ctx, blobSHA)
		if err != nil {
			return BaselineReceipt{}, err
		}
		total += len(content)
		if total > 16<<20 {
			return BaselineReceipt{}, fmt.Errorf("%w: baseline exceeds 16MiB", ErrBaselineTooLarge)
		}
		files = append(files, WorkspaceFileWrite{Path: root + "/" + p, Content: content})
	}
	if err := s.deps.Workspace.WriteSessionWorkspaceFiles(ctx, sessionID, files); err != nil {
		return BaselineReceipt{}, err
	}
	return BaselineReceipt{Files: len(files), Root: root}, nil
}

// PrepareInput anchors one delivery candidate. Branch empty → TaskBranchOf(taskID).
type PrepareInput struct {
	TenantID      uint64
	CallerID      string
	RunID         string
	ConnectionID  string
	Repo          RepoRef
	BaselineSHA   string
	Branch        string
	CommitMessage string
	PRTitle       string
}

// PrepareDelivery computes the workspace diff against the fixed baseline,
// anchors it as an A03 action (risk=deliver → always awaiting_approval), and
// persists the traceability row. AC1 guardrails run BEFORE any GitHub write.
func (s *CodeDeliveryService) PrepareDelivery(ctx context.Context, in PrepareInput) (DeliveryView, error) {
	sessionID, err := s.authorize(ctx, in.TenantID, in.CallerID, in.RunID, in.ConnectionID)
	if err != nil {
		return DeliveryView{}, err
	}
	if in.CommitMessage == "" || in.PRTitle == "" {
		return DeliveryView{}, fmt.Errorf("%w: commit_message and pr_title are required", ErrInvalidMaterial)
	}
	token, terr := s.tokenFor(ctx, in.ConnectionID)
	if terr != nil {
		return DeliveryView{}, terr
	}
	client := s.deps.GitHub(token, in.Repo)
	// 护栏 1（顺序有意为之）：先判「目标是否撞默认分支/远端 protected」再验
	// 任务分支前缀白名单——任何形状的分支（包括误填 "main"）都必须先撞上
	// 保护分支拒绝（AC1 的第一道闸），前缀白名单是第二道。
	if in.Branch == "" {
		in.Branch = TaskBranchOf(sessionID)
	}
	info, err := client.Repository(ctx)
	if err != nil {
		return DeliveryView{}, err
	}
	protected, err := client.BranchProtected(ctx, in.Branch)
	if err != nil {
		return DeliveryView{}, err
	}
	if err := RefuseProtectedTarget(in.Branch, info.DefaultBranch, protected); err != nil {
		return DeliveryView{}, err
	}
	if err := ValidateTaskBranch(in.Branch); err != nil {
		return DeliveryView{}, err
	}
	// 护栏 2：材料=工作区 diff（按真实 git blob sha 判定修改）。
	baselineTree, err := client.Tree(ctx, in.BaselineSHA)
	if err != nil {
		return DeliveryView{}, err
	}
	if len(baselineTree) > MaxDeliveryFiles {
		return DeliveryView{}, fmt.Errorf("%w: baseline has %d files", ErrBaselineTooLarge, len(baselineTree))
	}
	workspaceTree, err := s.workspaceTree(ctx, in.Repo)
	if err != nil {
		return DeliveryView{}, err
	}
	changes := DiffAgainstBaseline(baselineTree, workspaceTree)
	if len(changes) == 0 {
		return DeliveryView{}, fmt.Errorf("%w: workspace matches the baseline; nothing to deliver", ErrInvalidMaterial)
	}
	material := DeliveryMaterial{
		Repo: in.Repo, BaselineSHA: in.BaselineSHA, Branch: in.Branch,
		Files: changes, CommitMessage: in.CommitMessage, PRTitle: in.PRTitle,
	}
	if len(material.Files) > MaxDeliveryFiles {
		return DeliveryView{}, fmt.Errorf("%w: %d changed files", ErrBaselineTooLarge, len(material.Files))
	}
	args, err := material.CanonicalJSON()
	if err != nil {
		return DeliveryView{}, err
	}
	// 锚定 A03：digest 绑定 repo/基线/分支/文件清单/提交信息/PR 标题 + 连接
	// 版本；内容变化=新 digest=旧批准失效（immutable approval anchor）。
	conn, err := s.deps.Connections.FindConnectionByID(ctx, in.ConnectionID)
	if err != nil {
		return DeliveryView{}, err
	}
	actionID, err := s.deps.Actions.Prepare(ctx, appconnector.Action{
		TenantID: in.TenantID, ActorID: in.CallerID, ConnectionID: in.ConnectionID,
		Target: DeliveryActionTarget, Risk: appconnector.RiskDeliver,
		AuthVersion: conn.AuthVersion, Args: args,
	})
	if err != nil {
		return DeliveryView{}, err
	}
	// 结构性断言：交付动作必须生而 awaiting_approval（deliver 不在任何预授
	// 权白名单内；即便有人造出 deliver 预授权也在此 fail closed）。
	row, err := s.deps.ActionRows.FindAction(ctx, actionID)
	if err != nil {
		return DeliveryView{}, err
	}
	if row.State != appconnector.ActionAwaitingApproval {
		return DeliveryView{}, fmt.Errorf("%w: delivery action must await approval, got %s", ErrInvalidMaterial, row.State)
	}
	drow := deliveryrepo.DeliveryRow{
		ID: "dlv_" + uuid.NewString(), TenantID: in.TenantID,
		TaskID: sessionID, RunID: in.RunID, OwnerID: in.CallerID,
		ActionID: actionID, ConnectionID: in.ConnectionID,
		Repo: in.Repo.String(), BaselineSHA: in.BaselineSHA, Branch: in.Branch,
		State: string(DeliveryPrepared),
	}
	if err := s.deps.Store.CreateDelivery(ctx, drow); err != nil {
		return DeliveryView{}, err
	}
	return s.viewOf(ctx, drow)
}

// GetDeliveryForRun returns the latest delivery of a run (read face).
func (s *CodeDeliveryService) GetDeliveryForRun(ctx context.Context, tenantID uint64, runID string) (DeliveryView, error) {
	row, err := s.deps.Store.LatestForRun(ctx, tenantID, runID)
	if err != nil {
		return DeliveryView{}, err
	}
	return s.viewOf(ctx, row)
}

// DeliveryView is the wire/read projection carrying the full traceability
// chain: approval anchor (action/digest/approver) + remote receipts.
type DeliveryView struct {
	ID          string `json:"id"`
	TaskID      string `json:"task_id"`
	RunID       string `json:"run_id"`
	State       string `json:"state"`
	Repo        string `json:"repo"`
	BaselineSHA string `json:"baseline_sha"`
	Branch      string `json:"branch"`
	CommitSHA   string `json:"commit_sha"`
	PRNumber    int64  `json:"pr_number"`
	PRURL       string `json:"pr_url"`
	RemoteLogin string `json:"remote_login"`
	ActionID    string `json:"action_id"`
	ActionState string `json:"action_state"`
	Digest      string `json:"digest"`
	Approver    string `json:"approver"`
	Failure     string `json:"failure"`
	Files       int    `json:"files"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (s *CodeDeliveryService) viewOf(ctx context.Context, row deliveryrepo.DeliveryRow) (DeliveryView, error) {
	view := DeliveryView{
		ID: row.ID, TaskID: row.TaskID, RunID: row.RunID, State: row.State,
		Repo: row.Repo, BaselineSHA: row.BaselineSHA, Branch: row.Branch,
		CommitSHA: row.CommitSHA, PRNumber: row.PRNumber, PRURL: row.PRURL,
		RemoteLogin: row.RemoteLogin, ActionID: row.ActionID,
		Failure: row.Failure,
		CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if row.ActionID != "" {
		if action, err := s.deps.ActionRows.FindAction(ctx, row.ActionID); err == nil {
			view.ActionState = action.State
			view.Digest = action.ArgsDigest
			mat, merr := ParseDeliveryMaterial([]byte(action.ArgsSnapshot))
			if merr == nil {
				view.Files = len(mat.Files)
			}
		}
		if approver, ok, err := s.deps.Store.LatestApproverForAction(ctx, row.ActionID); err == nil && ok {
			view.Approver = approver
		}
	}
	return view, nil
}

// authorize is the shared owner-only predicate: caller owns the run AND uses
// their own personal connection (CONTEXT.md 个人连接只能由其所有者使用).
// It returns the run's sessionID (= taskID, ADR-0004); the service keeps no
// mutable state.
func (s *CodeDeliveryService) authorize(ctx context.Context, tenantID uint64, callerID, runID, connectionID string) (string, error) {
	run, err := s.deps.Runs.GetOwnedRun(ctx, tenantID, callerID, runID)
	if err != nil || run.SessionID == "" {
		return "", fmt.Errorf("%w: run %s", ErrNotDeliveryOwner, runID)
	}
	conn, err := s.deps.Connections.FindConnectionByID(ctx, connectionID)
	if err != nil {
		return "", err
	}
	if conn.Kind != appconnector.ConnectionKindPersonal || conn.OwnerID != callerID ||
		conn.TenantID != tenantID || conn.State != appconnector.ConnectionActive {
		return "", ErrConnectionNotUsable
	}
	return run.SessionID, nil
}

// tokenFor resolves the delivery token AFTER the permission chain (the
// CredentialResolver contract: credentials are for the calling adapter only).
func (s *CodeDeliveryService) tokenFor(ctx context.Context, connectionID string) (string, error) {
	conn, err := s.deps.Connections.FindConnectionByID(ctx, connectionID)
	if err != nil {
		return "", err
	}
	raw, err := s.deps.Creds.Resolve(ctx, connectionID, conn.AuthVersion)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", ErrConnectionNotUsable
	}
	return token, nil
}

// workspaceTree projects the session workspace repo root as path→git blob sha.
func (s *CodeDeliveryService) workspaceTree(ctx context.Context, sessionID string, repo RepoRef) (map[string]string, error) {
	root := WorkspaceRepoRoot(repo)
	entries, err := s.deps.Workspace.ListSessionFiles(ctx, sessionID, root)
	if err != nil {
		return nil, err
	}
	if len(entries) > MaxDeliveryFiles {
		return nil, fmt.Errorf("%w: workspace has %d files", ErrBaselineTooLarge, len(entries))
	}
	tree := make(map[string]string, len(entries))
	rootPrefix := strings.TrimSuffix(root, "/") + "/"
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}
		// 会话沙箱返回以请求目录为前缀的路径（"/workspace/<owner>/<name>/…"）。
		rel := strings.TrimPrefix(entry.Path, rootPrefix)
		if rel == entry.Path { // outside the repo root
			continue
		}
		if entry.Size > 4<<20 {
			return nil, fmt.Errorf("%w: %s is %d bytes", ErrBaselineTooLarge, rel, entry.Size)
		}
		content, err := s.deps.Workspace.ReadSessionFile(ctx, sessionID, entry.Path)
		if err != nil {
			return nil, err
		}
		tree[rel] = GitBlobSHA(content)
	}
	return tree, nil
}

func baselineSHALegal(sha string) bool {
	return len(sha) == 40 && strings.Trim(sha, "0123456789abcdef") == ""
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/codedelivery/ -run 'TestMaterializeBaseline|TestPrepareDelivery|TestLocalWorkspace' -count=1`
Expected: PASS（5 个用例全绿；`TestPrepareDeliveryRefusesProtectedBranchWithZeroRemoteWrites` 断言零 ref 写——AC1）

- [ ] **Step 5: Commit**

```bash
git add internal/modules/codedelivery/workspace.go internal/modules/codedelivery/workspace_local.go internal/modules/codedelivery/service.go internal/modules/codedelivery/service_prepare_test.go internal/modules/codedelivery/github_wire_test.go
git commit -m "feat(codedelivery): baseline materialization + delivery prepare with protected-branch guardrails and A03 approval anchor (T22 #52 task 5)"
```

---

### Task 6: Go——交付编排 B：DispatchDelivery、部分完成与 unknown 收敛

**Files:**
- Create: `internal/modules/codedelivery/dispatcher.go`
- Modify: `internal/modules/codedelivery/service.go`（追加 DispatchDelivery/ResolveDeliveryUnknown/GetDelivery/ErrDeliveryState；`CodeDeliveryDeps` 增加 `Dispatcher *DeliveryDispatcher` 字段）
- Modify: `internal/modules/codedelivery/service_prepare_test.go`（夹具追加 dispatcher 字段接线——3 处小改，见 Step 3 末尾）
- Modify: `internal/modules/codedelivery/github.go` + `github_client.go` + `github_wire_test.go`（端口补 `BranchHead`；模拟器补 `GET /git/ref/heads/{b}` 分支）
- Test: `internal/modules/codedelivery/service_dispatch_test.go`

**Interfaces:**
- Consumes: Task 5 全部；`appconnectorsvc.ActionDispatcher`/`UnknownResolver`/`ActionSnapshot`/`DispatchOutcome`/`ErrDispatchNotStarted`/`ErrDispatchUnknown`（`internal/modules/appconnector/service/appconnector/action.go:82-127`）；`appconnectorsvc.A02Guard.Check`（`oc_authorizer.go:119`）；`appconnector` 状态常量（`internal/modules/appconnector/action.go:30-38`）。
- Produces（Task 7 依赖的精确签名）:
  - `type DispatcherDeps struct { Connections ConnectionReader; Creds appconnectorsvc.CredentialResolver; Guard appconnectorsvc.A02Guard; GitHub GitHubClientFactory; Workspace WorkspaceFileSource; Store *deliveryrepo.DeliveryStore; ActionRows appconnectorsvc.ActionStoreSource; Runs RunReader }`
  - `func NewDeliveryDispatcher(deps DispatcherDeps) *DeliveryDispatcher`
  - `func (d *DeliveryDispatcher) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error)`（实现 `ActionDispatcher`）
  - `func (d *DeliveryDispatcher) QueryProvider(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error)`（实现 `UnknownResolver`：只读远端事实——PR head / 分支 ref）
  - `func (d *DeliveryDispatcher) RecoverPullRequest(ctx context.Context, tenantID uint64, deliveryID string) error`（pushed 状态的 PR-only 恢复：同一批准、绝不重推）
  - `var ErrDeliveryState = errors.New("code_delivery_state_conflict")`
  - `type DispatchInput struct { TenantID uint64; CallerID, RunID, DeliveryID string }`
  - `func (s *CodeDeliveryService) DispatchDelivery(ctx context.Context, in DispatchInput) (DeliveryView, error)`——`prepared`→消费批准（`Actions.Execute`）；`pushed`→PR-only 恢复；`unknown`→拒（走 Resolve）；其余 `ErrDeliveryState`
  - `func (s *CodeDeliveryService) ResolveDeliveryUnknown(ctx context.Context, in DispatchInput) (DeliveryView, error)`
  - `func (s *CodeDeliveryService) GetDelivery(ctx context.Context, tenantID uint64, deliveryID string) (DeliveryView, error)`

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/codedelivery/service_dispatch_test.go`：

```go
package codedelivery

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/stretchr/testify/require"
)

// dispatchFixture 的最终实现见 Step 3 末尾（Task 6 改造后为直通别名——夹具内部单实例装配 dispatcher）。

// seededFixture：基线之上修改 main.go，prepare + approve 完成。
func seededFixture(t *testing.T) *deliveryFixture {
	f := dispatchFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hello\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	ctx := context.Background()
	view, err := f.svc.PrepareDelivery(ctx, prepareInput())
	require.NoError(t, err)
	require.NoError(t, f.actions.Approve(ctx, view.ActionID, "u1", view.Digest))
	return f
}

func dispatchInput(view DeliveryView) DispatchInput {
	return DispatchInput{TenantID: 7, CallerID: "u1", RunID: "run-1", DeliveryID: view.ID}
}

func firstDelivery(t *testing.T, f *deliveryFixture) DeliveryView {
	view, err := f.svc.GetDeliveryForRun(context.Background(), 7, "run-1")
	require.NoError(t, err)
	return view
}

// AC2 端到端：提交 SHA、审批内容（approver+digest）与 PR 回执全部落账。
func TestDispatchDeliversAndRecordsTraceableReceipts(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()

	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	require.NotEmpty(t, view.CommitSHA)
	require.EqualValues(t, 1, view.PRNumber)
	require.Contains(t, view.PRURL, "/pull/1")
	require.Equal(t, "octocat", view.RemoteLogin, "实际远端身份必须落账")
	require.Equal(t, "u1", view.Approver, "审批内容（批准人）必须可追溯")
	require.NotEmpty(t, view.Digest)

	// 远端事实：任务分支指向候选提交；默认分支纹丝不动。
	sha, ok := f.github.BranchCommit("weknora/task/s-1")
	require.True(t, ok)
	require.Equal(t, view.CommitSHA, sha)
	require.Equal(t, "b0000000000000000000000000000000000000000", branchCommitOf(t, f, "main"))
	require.Empty(t, f.github.Violations())
}

func branchCommitOf(t *testing.T, f *deliveryFixture, branch string) string {
	t.Helper()
	sha, ok := f.github.BranchCommit(branch)
	require.True(t, ok)
	return sha
}

// AC1：派发全程零 merge、零保护分支写（模拟器违规计数器为证）。
func TestDispatchNeverWritesProtectedBranchOrMerges(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()
	_, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Zero(t, f.github.Calls()["PUT /pulls/merge"])
	require.Empty(t, f.github.Violations())
}

// 部分完成（spec 故事 47 / CONTEXT.md 避免项）：推送成功、PR 创建确定性
// 失败 → 状态 pushed；恢复只补 PR，绝不重发 blobs/tree/commit/ref。
func TestPartialPushPRFailureRecoversWithoutRepush(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()
	f.github.failNextPRCreation()
	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPushed), view.State)
	require.NotEmpty(t, view.CommitSHA, "推送提交必须已落账")

	before := snapshotCalls(f)
	view, err = f.svc.DispatchDelivery(ctx, dispatchInput(view)) // PR-only 恢复
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	after := snapshotCalls(f)
	require.Equal(t, before["POST /git/blobs"], after["POST /git/blobs"], "recovery must not re-send blobs")
	require.Equal(t, before["POST /git/trees"], after["POST /git/trees"], "recovery must not re-send trees")
	require.Equal(t, before["POST /git/commits"], after["POST /git/commits"], "recovery must not re-send commits")
	require.Equal(t, before["POST /git/refs"], after["POST /git/refs"], "recovery must not re-push the branch")
	require.Equal(t, before["PATCH /git/refs"], after["PATCH /git/refs"])
	require.Equal(t, before["POST /pulls"]+1, after["POST /pulls"], "recovery retries ONLY the PR creation")
}

func snapshotCalls(f *deliveryFixture) map[string]int {
	out := map[string]int{}
	for k, v := range f.github.Calls() {
		out[k] = v
	}
	return out
}

// 审批锚点不可变（Review Focus 3 / spec「candidate code commits are
// immutable approval anchors」）：批准后内容被换=新 Prepare=新 digest；旧
// digest 的批准对第二个交付必然 digest mismatch 拒绝，且第二个交付在未获
// 得自己（新 digest）的批准前派发被拒——全程零 GitHub 调用。
func TestDispatchRefusesSecondDeliveryPreparedWithDifferentFiles(t *testing.T) {
	f := dispatchFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# hello\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	ctx := context.Background()

	// 交付一：改 main.go → action A1 / digest D1。
	first, err := f.svc.PrepareDelivery(ctx, prepareInput())
	require.NoError(t, err)
	require.NoError(t, f.actions.Approve(ctx, first.ActionID, "u1", first.Digest))

	// 批准之后工作区内容再变（新增 util.go）→ 交付二必须走新 Prepare：
	// action A2 / digest D2 ≠ D1（immutable anchor：旧批准绝不迁移）。
	require.NoError(t, os.WriteFile(filepath.Join(f.root, "octocat/hello/util.go"), []byte("package main\n\nfunc util() {}\n"), 0o644))
	second, err := f.svc.PrepareDelivery(ctx, prepareInput())
	require.NoError(t, err)
	require.NotEqual(t, first.Digest, second.Digest, "content change must mint a NEW digest")
	require.NotEqual(t, first.ActionID, second.ActionID)

	// 旧 digest 的批准对 A2 拒绝（A03 digest 绑定），A2 停留在 awaiting_approval。
	require.Error(t, f.actions.Approve(ctx, second.ActionID, "u1", first.Digest))
	after, err := f.svc.GetDelivery(ctx, 7, second.ID)
	require.NoError(t, err)
	require.Equal(t, "awaiting_approval", after.ActionState)

	// 未获自己 digest 的批准即派发交付二：拒绝且零 GitHub 写调用。
	_, err = f.svc.DispatchDelivery(ctx, dispatchInput(after))
	require.Error(t, err)
	require.Zero(t, f.github.Calls()["POST /git/refs"])
	require.Zero(t, f.github.Calls()["POST /git/blobs"])
	require.Zero(t, f.github.Calls()["POST /pulls"])
	require.Empty(t, f.github.Violations())
}

// 未批准即派发：A03 拒绝（ErrActionState 族），零 GitHub 调用。
func TestDispatchWithoutApprovalConsumesNothing(t *testing.T) {
	f := dispatchFixture(t, func(root string) {
		dir := filepath.Join(root, "octocat/hello")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644))
	})
	ctx := context.Background()
	view, err := f.svc.PrepareDelivery(ctx, prepareInput())
	require.NoError(t, err)
	_, err = f.svc.DispatchDelivery(ctx, dispatchInput(view))
	require.Error(t, err)
	require.Zero(t, f.github.Calls()["POST /git/refs"])
	require.Zero(t, f.github.Calls()["POST /pulls"])
}

// 篡改快照永不触达 GitHub：dispatcher 解析失败=ErrDispatchNotStarted。
func TestTamperedSnapshotNeverReachesGitHub(t *testing.T) {
	f := seededFixture(t)
	snap := appconnectorsvc.ActionSnapshot{ID: "act-x", TenantID: 7, ActorID: "u1", Args: []byte(`{"repo":"o/n"}`)}
	_, err := f.dispatcher.Dispatch(context.Background(), snap, "")
	require.ErrorIs(t, err, appconnectorsvc.ErrDispatchNotStarted)
	require.Zero(t, f.github.Calls()["POST /git/blobs"])
}

// 远端不可观测 → unknown 落账；ResolveUnknown 以远端事实收敛（分支已推、
// PR 缺席 → pushed），随后 PR-only 恢复完成交付。
func TestUnknownOutcomeResolvesFromRemoteFacts(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()
	f.github.blackoutAfterRefCreate()
	view, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryUnknown), view.State)
	require.Equal(t, "unknown", view.ActionState)

	f.github.liftBlackout()
	view, err = f.svc.ResolveDeliveryUnknown(ctx, dispatchInput(view))
	require.NoError(t, err)
	require.Equal(t, string(DeliveryPushed), view.State, "远端事实：分支已推、PR 缺席 → 部分完成")

	view, err = f.svc.DispatchDelivery(ctx, dispatchInput(view)) // PR-only 恢复
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	require.Empty(t, f.github.Violations())
}

// A02 拒绝（成员资格撤销）：动作不消费、零远端调用。
func TestDispatchFailsClosedWhenConnectionUnusable(t *testing.T) {
	f := seededFixture(t)
	ctx := context.Background()
	membersDrop(f, "u1")
	_, err := f.svc.DispatchDelivery(ctx, dispatchInput(firstDelivery(t, f)))
	require.Error(t, err)
	require.Zero(t, f.github.Calls()["POST /git/refs"])
	require.Zero(t, f.github.Calls()["POST /pulls"])
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/codedelivery/ -run 'TestDispatch|TestPartialPush|TestUnknownOutcome|TestTampered' -count=1`
Expected: FAIL（`DispatchDelivery`/`NewDeliveryDispatcher`/`f.dispatcher` 未定义，编译错误）

- [ ] **Step 3: 写最小实现**

先给 Task 2 的端口与实现补 `BranchHead`（三处小改）：

`github.go` 接口加一行（`EnsureBranch` 之前）：

```go
	BranchHead(ctx context.Context, branch string) (string, bool, error)
```

`github_client.go` 加实现：

```go
// BranchHead reads refs/heads/<branch>; (sha,false,nil) when the branch
// does not exist. Used by unknown-resolution to read remote facts only.
func (c *gitHubRestClient) BranchHead(ctx context.Context, branch string) (string, bool, error) {
	var out struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := c.call(ctx, http.MethodGet, "/repos/"+c.repoString()+"/git/ref/heads/"+branch, nil, &out); err != nil {
		var apiErr *GitHubAPIError
		if asGitHubAPIError(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	return out.Object.SHA, out.Object.SHA != "", nil
}
```

`github_wire_test.go` 的模拟器 switch 加分支（`/git/commits/` 分支之后），并把 `blackoutAfterRefCreate` 改为「ref 创建成功后进入 blackout」：

```go
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/ref/heads/"):
		e.note("GET /git/ref")
		branch := strings.TrimPrefix(path, "/repos/octocat/hello/git/ref/heads/")
		e.mu.Lock()
		sha, ok := e.refs["refs/heads/"+branch]
		e.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"ref": "refs/heads/" + branch, "object": map[string]any{"sha": sha}})
```

`githubEmulator` 的字段 `blackout bool` 旁加 `blackoutAfterRef bool`；helper 改为：

```go
// blackoutAfterRefCreate：下一次成功的 POST /git/refs 之后，所有后续请求
// hijack 断连（模拟推送已完成、PR 创建中途网络不可观测）。
func (e *githubEmulator) blackoutAfterRefCreate() {
	e.mu.Lock()
	e.blackoutAfterRef = true
	e.mu.Unlock()
}
```

`POST /git/refs` 成功路径（`e.refs[body.Ref] = body.SHA` 之后、`writeJSON` 之前）加：

```go
		e.mu.Lock()
		if e.blackoutAfterRef {
			e.blackout = true
		}
		e.mu.Unlock()
```

创建 `internal/modules/codedelivery/dispatcher.go`：

```go
package codedelivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"
)

// DispatcherDeps wires the outbound delivery chain. Guard re-runs on every
// dispatch and recovery (A02: the persisted subject may still use the
// connection); ActionRows re-reads the approved snapshot.
type DispatcherDeps struct {
	Connections ConnectionReader
	Creds       appconnectorsvc.CredentialResolver
	Guard       appconnectorsvc.A02Guard
	GitHub      GitHubClientFactory
	Workspace   WorkspaceFileSource
	Store       *deliveryrepo.DeliveryStore
	ActionRows  appconnectorsvc.ActionStoreSource
	Runs        RunReader
}

// DeliveryDispatcher implements the A03 ActionDispatcher and UnknownResolver
// for github.deliver actions. It is the ONLY outbound boundary: tokens
// resolve here, GitHub calls leave here, receipts land here. There is no
// merge path anywhere (AC1).
type DeliveryDispatcher struct{ deps DispatcherDeps }

func NewDeliveryDispatcher(deps DispatcherDeps) *DeliveryDispatcher {
	return &DeliveryDispatcher{deps: deps}
}

// Dispatch performs the approved delivery. Pre-send gates (snapshot parse,
// A02, credential) fail with ErrDispatchNotStarted; a GitHub response is a
// definite outcome; a transport error is unobservable and bubbles up as
// ErrGitHubTransport (the service parks unknown). Push-succeeded with a
// definite PR failure is the recorded PARTIAL completion `pushed`.
func (d *DeliveryDispatcher) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	material, err := ParseDeliveryMaterial(snap.Args)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: %v", appconnectorsvc.ErrDispatchNotStarted, err)
	}
	if err := d.deps.Guard.Check(ctx,
		appconnector.OCSubject{TenantID: snap.TenantID, ActorID: snap.ActorID},
		snap.ConnectionID, snap.AuthVersion,
	); err != nil {
		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: a02: %v", appconnectorsvc.ErrDispatchNotStarted, err)
	}
	token, err := d.tokenFor(ctx, snap)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: credential: %v", appconnectorsvc.ErrDispatchNotStarted, err)
	}
	row, err := d.findByAction(ctx, snap.TenantID, snap.ID)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: delivery row: %v", appconnectorsvc.ErrDispatchNotStarted, err)
	}
	client := d.deps.GitHub(token, material.Repo)
	return d.deliver(ctx, snap, material, row, client, false)
}

// RecoverPullRequest completes the PR half of a PARTIAL delivery (state
// pushed) under the SAME approval: A02 re-check, then PR-only. It never
// re-sends blobs/tree/commit/ref (CONTEXT.md 代码交付避免项).
func (d *DeliveryDispatcher) RecoverPullRequest(ctx context.Context, tenantID uint64, deliveryID string) error {
	row, err := d.deps.Store.GetDelivery(ctx, tenantID, deliveryID)
	if err != nil {
		return err
	}
	if row.State != string(DeliveryPushed) {
		return fmt.Errorf("%w: recovery from %s (resolve unknown first)", ErrDeliveryState, row.State)
	}
	snap, err := d.snapshotOfDelivery(ctx, row)
	if err != nil {
		return err
	}
	material, err := ParseDeliveryMaterial(snap.Args)
	if err != nil {
		return err
	}
	if err := d.deps.Guard.Check(ctx,
		appconnector.OCSubject{TenantID: snap.TenantID, ActorID: snap.ActorID},
		snap.ConnectionID, snap.AuthVersion,
	); err != nil {
		return err
	}
	token, err := d.tokenFor(ctx, snap)
	if err != nil {
		return err
	}
	_, err = d.deliver(ctx, snap, material, row, d.deps.GitHub(token, material.Repo), true)
	return err
}

// deliver runs the delivery chain. partialRecovery=true skips the push half
// entirely — it already happened under the SAME approval.
func (d *DeliveryDispatcher) deliver(ctx context.Context, snap appconnectorsvc.ActionSnapshot, material DeliveryMaterial, row deliveryrepo.DeliveryRow, client GitHubClient, partialRecovery bool) (appconnectorsvc.DispatchOutcome, error) {
	if !partialRecovery {
		info, err := client.Repository(ctx)
		if err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		protected, err := client.BranchProtected(ctx, material.Branch)
		if err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		// AC1 双保险：派发前复验目标分支不是默认分支/未被远端标记保护。
		if err := RefuseProtectedTarget(material.Branch, info.DefaultBranch, protected); err != nil {
			return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: %v", appconnectorsvc.ErrDispatchNotStarted, err)
		}
		// 内容从会话工作区读取（令牌只留在服务端，永不进沙箱）。
		run, err := d.deps.Runs.GetOwnedRun(ctx, snap.TenantID, snap.ActorID, row.RunID)
		if err != nil || run.SessionID == "" {
			return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: run %s", appconnectorsvc.ErrDispatchNotStarted, row.RunID)
		}
		root := WorkspaceRepoRoot(material.Repo)
		entries := make([]TreeEntry, 0, len(material.Files))
		for _, change := range material.Files {
			if change.Deleted {
				entries = append(entries, TreeEntry{Path: change.Path})
				continue
			}
			content, rerr := d.deps.Workspace.ReadSessionFile(ctx, run.SessionID, root+"/"+change.Path)
			if rerr != nil {
				return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: workspace read %s: %v", appconnectorsvc.ErrDispatchNotStarted, change.Path, rerr)
			}
			blobSHA, berr := client.CreateBlob(ctx, content)
			if berr != nil {
				return appconnectorsvc.DispatchOutcome{}, berr
			}
			entries = append(entries, TreeEntry{Path: change.Path, SHA: blobSHA})
		}
		baseTree, terr := client.CommitTree(ctx, material.BaselineSHA)
		if terr != nil {
			return appconnectorsvc.DispatchOutcome{}, terr
		}
		treeSHA, trerr := client.CreateTree(ctx, baseTree, entries)
		if trerr != nil {
			return appconnectorsvc.DispatchOutcome{}, trerr
		}
		commitSHA, cerr := client.CreateCommit(ctx, material.BaselineSHA, treeSHA, material.CommitMessage)
		if cerr != nil {
			return appconnectorsvc.DispatchOutcome{}, cerr
		}
		if err := d.deps.Store.RecordReceipts(ctx, snap.TenantID, row.ID, deliveryrepo.ReceiptUpdate{CommitSHA: commitSHA}); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		if err := client.EnsureBranch(ctx, material.Branch, commitSHA); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		if err := d.deps.Store.TransitionState(ctx, snap.TenantID, row.ID,
			[]string{string(DeliveryDispatched), string(DeliveryPrepared), string(DeliveryUnknown)}, string(DeliveryPushed), ""); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
	}
	// —— PR 半程 ——（恢复路径只走这里）
	repoInfo, err := client.Repository(ctx)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	receipt, prerr := client.DraftPullRequest(ctx, PullRequestInput{
		Title: material.PRTitle, Head: material.Repo.Owner + ":" + material.Branch, Base: repoInfo.DefaultBranch,
	})
	var login string
	if prerr == nil {
		login, prerr = client.CurrentLogin(ctx)
	}
	if prerr != nil {
		var apiErr *GitHubAPIError
		if errors.As(prerr, &apiErr) {
			// 确定性 PR 失败：若已推（pushed）保持部分完成可恢复；否则 failed。
			current, gerr := d.deps.Store.GetDelivery(ctx, snap.TenantID, row.ID)
			if gerr == nil && current.State == string(DeliveryPushed) {
				return appconnectorsvc.DispatchOutcome{Status: appconnector.ActionSucceeded, ProviderResult: partialReceipt(current.CommitSHA)}, nil
			}
			return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: pr: %v", appconnectorsvc.ErrDispatchNotStarted, apiErr)
		}
		return appconnectorsvc.DispatchOutcome{}, prerr // 传输不可观测 → 上层落 unknown
	}
	if err := d.deps.Store.RecordReceipts(ctx, snap.TenantID, row.ID, deliveryrepo.ReceiptUpdate{
		PRNumber: receipt.Number, PRURL: receipt.URL, RemoteLogin: login,
	}); err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	if err := d.deps.Store.TransitionState(ctx, snap.TenantID, row.ID,
		[]string{string(DeliveryPushed), string(DeliveryPrepared), string(DeliveryDispatched), string(DeliveryUnknown)},
		string(DeliveryDelivered), ""); err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	return appconnectorsvc.DispatchOutcome{
		Status:         appconnector.ActionSucceeded,
		ProviderResult: deliveredReceipt(receipt.Number, receipt.URL, login),
	}, nil
}

// QueryProvider resolves an unknown delivery from REMOTE FACTS only: a draft
// PR for the task head → delivered; otherwise the task branch ref → pushed.
// It never re-sends anything.
func (d *DeliveryDispatcher) QueryProvider(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	material, err := ParseDeliveryMaterial(snap.Args)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	row, err := d.findByAction(ctx, snap.TenantID, snap.ID)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	token, err := d.tokenFor(ctx, snap)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	client := d.deps.GitHub(token, material.Repo)
	head := material.Repo.Owner + ":" + material.Branch
	if receipt, rerr := client.PullRequestForHead(ctx, head); rerr == nil && receipt != nil {
		if err := d.deps.Store.RecordReceipts(ctx, snap.TenantID, row.ID, deliveryrepo.ReceiptUpdate{
			PRNumber: receipt.Number, PRURL: receipt.URL,
		}); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		if err := d.deps.Store.TransitionState(ctx, snap.TenantID, row.ID,
			[]string{string(DeliveryUnknown)}, string(DeliveryDelivered), ""); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		return appconnectorsvc.DispatchOutcome{Status: appconnector.ActionSucceeded, ProviderResult: "resolved: draft PR exists"}, nil
	}
	if sha, exists, berr := client.BranchHead(ctx, material.Branch); berr == nil && exists {
		if err := d.deps.Store.RecordReceipts(ctx, snap.TenantID, row.ID, deliveryrepo.ReceiptUpdate{CommitSHA: sha}); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		if err := d.deps.Store.TransitionState(ctx, snap.TenantID, row.ID,
			[]string{string(DeliveryUnknown)}, string(DeliveryPushed), ""); err != nil {
			return appconnectorsvc.DispatchOutcome{}, err
		}
		return appconnectorsvc.DispatchOutcome{Status: appconnector.ActionSucceeded, ProviderResult: "resolved: branch pushed, draft PR absent"}, nil
	}
	return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: no remote fact for %s yet", appconnectorsvc.ErrDispatchUnknown, head)
}

func (d *DeliveryDispatcher) tokenFor(ctx context.Context, snap appconnectorsvc.ActionSnapshot) (string, error) {
	raw, err := d.deps.Creds.Resolve(ctx, snap.ConnectionID, snap.AuthVersion)
	if err != nil {
		return "", err
	}
	if len(raw) == 0 {
		return "", ErrConnectionNotUsable
	}
	return string(raw), nil
}

func (d *DeliveryDispatcher) findByAction(ctx context.Context, tenantID uint64, actionID string) (deliveryrepo.DeliveryRow, error) {
	var row deliveryrepo.DeliveryRow
	err := d.deps.Store.DB().WithContext(ctx).
		Where("tenant_id = ? AND action_id = ?", tenantID, actionID).
		Order("created_at DESC").First(&row).Error
	if err != nil {
		return deliveryrepo.DeliveryRow{}, deliveryrepo.ErrDeliveryNotFound
	}
	return row, nil
}

func (d *DeliveryDispatcher) snapshotOfDelivery(ctx context.Context, row deliveryrepo.DeliveryRow) (appconnectorsvc.ActionSnapshot, error) {
	actionRow, err := d.deps.ActionRows.FindAction(ctx, row.ActionID)
	if err != nil {
		return appconnectorsvc.ActionSnapshot{}, err
	}
	return appconnectorsvc.ActionSnapshot{
		ID: actionRow.ID, TenantID: actionRow.TenantID, ActorID: actionRow.ActorID,
		ConnectionID: actionRow.ConnectionID, Version: actionRow.AppVersion,
		Target: actionRow.Target, Risk: actionRow.Risk, Digest: actionRow.ArgsDigest,
		State: actionRow.State, Fence: actionRow.Fence, Args: []byte(actionRow.ArgsSnapshot),
		AuthVersion: actionRow.AuthVersion, DigestVersion: int(actionRow.DigestVersion),
	}, nil
}

func partialReceipt(commitSHA string) string {
	raw, _ := json.Marshal(map[string]any{"partial": "pr", "commit_sha": commitSHA})
	return string(raw)
}

func deliveredReceipt(prNumber int64, prURL, login string) string {
	raw, _ := json.Marshal(map[string]any{
		"pr_number": prNumber, "pr_url": prURL, "remote_login": login,
	})
	return string(raw)
}
```

`service.go` 追加（并在 `CodeDeliveryDeps` 增加字段 `Dispatcher *DeliveryDispatcher`）：

```go
// DispatchInput addresses one delivery. CallerID must be the run owner.
type DispatchInput struct {
	TenantID   uint64
	CallerID   string
	RunID      string
	DeliveryID string
}

// ErrDeliveryState guards the delivery state machine at the service seam.
var ErrDeliveryState = errors.New("code_delivery_state_conflict")

// DispatchDelivery executes the approved delivery. prepared → consume the
// approval through the dedicated A03 instance; pushed → PR-only recovery
// under the SAME approval (never re-push); unknown → provider query only.
func (s *CodeDeliveryService) DispatchDelivery(ctx context.Context, in DispatchInput) (DeliveryView, error) {
	if _, err := s.deps.Runs.GetOwnedRun(ctx, in.TenantID, in.CallerID, in.RunID); err != nil {
		return DeliveryView{}, ErrNotDeliveryOwner
	}
	row, err := s.deps.Store.GetDelivery(ctx, in.TenantID, in.DeliveryID)
	if err != nil {
		return DeliveryView{}, err
	}
	switch DeliveryState(row.State) {
	case DeliveryPrepared:
		if err := s.deps.Store.TransitionState(ctx, in.TenantID, row.ID,
			[]string{string(DeliveryPrepared)}, string(DeliveryDispatched), ""); err != nil {
			return DeliveryView{}, err
		}
		if err := s.deps.Actions.Execute(ctx, row.ActionID); err != nil {
			if errors.Is(err, appconnectorsvc.ErrDispatchUnknown) {
				_ = s.deps.Store.TransitionState(ctx, in.TenantID, row.ID,
					[]string{string(DeliveryDispatched), string(DeliveryPushed)}, string(DeliveryUnknown), "")
				return s.viewAfter(ctx, in, row.ID)
			}
			after, gerr := s.deps.Store.GetDelivery(ctx, in.TenantID, row.ID)
			if gerr == nil && after.State == string(DeliveryPushed) {
				// 部分完成已由 dispatcher 落账：等待 PR-only 恢复，不算失败。
				return s.viewOf(ctx, after)
			}
			_ = s.deps.Store.TransitionState(ctx, in.TenantID, row.ID,
				[]string{string(DeliveryDispatched), string(DeliveryPrepared)}, string(DeliveryFailed), err.Error())
			return DeliveryView{}, err
		}
	case DeliveryPushed:
		// pushed → PR-only 恢复（同一批准的未完成半程；A02 复验在恢复端内部）。
		if s.deps.Dispatcher == nil {
			return DeliveryView{}, fmt.Errorf("%w: dispatcher not wired", ErrDeliveryState)
		}
		if err := s.deps.Dispatcher.RecoverPullRequest(ctx, in.TenantID, row.ID); err != nil {
			return DeliveryView{}, err
		}
	default:
		return DeliveryView{}, fmt.Errorf("%w: %s", ErrDeliveryState, row.State)
	}
	return s.viewAfter(ctx, in, row.ID)
}

// ResolveDeliveryUnknown settles an unknown delivery from remote facts only.
func (s *CodeDeliveryService) ResolveDeliveryUnknown(ctx context.Context, in DispatchInput) (DeliveryView, error) {
	if _, err := s.deps.Runs.GetOwnedRun(ctx, in.TenantID, in.CallerID, in.RunID); err != nil {
		return DeliveryView{}, ErrNotDeliveryOwner
	}
	row, err := s.deps.Store.GetDelivery(ctx, in.TenantID, in.DeliveryID)
	if err != nil {
		return DeliveryView{}, err
	}
	if err := s.deps.Actions.ResolveUnknown(ctx, row.ActionID); err != nil {
		return DeliveryView{}, err
	}
	return s.viewAfter(ctx, in, row.ID)
}

// GetDelivery reads one delivery by id (read face).
func (s *CodeDeliveryService) GetDelivery(ctx context.Context, tenantID uint64, deliveryID string) (DeliveryView, error) {
	row, err := s.deps.Store.GetDelivery(ctx, tenantID, deliveryID)
	if err != nil {
		return DeliveryView{}, err
	}
	return s.viewOf(ctx, row)
}

func (s *CodeDeliveryService) viewAfter(ctx context.Context, in DispatchInput, deliveryID string) (DeliveryView, error) {
	row, err := s.deps.Store.GetDelivery(ctx, in.TenantID, deliveryID)
	if err != nil {
		return DeliveryView{}, err
	}
	return s.viewOf(ctx, row)
}
```

`service_prepare_test.go` 夹具的 3 处小改：

1. `deliveryFixture` 结构体加字段 `dispatcher *DeliveryDispatcher`。
2. **改造 `newDeliveryFixture` 为 dispatcher 内部单实例构造**：把 Task 5 版本中从 `var dispatch appconnectorsvc.ActionDispatcher` 解包段起、到 `svc := NewCodeDeliveryService(CodeDeliveryDeps{…})` 为止的整段（含 `var unknown …`、旧 `actions := …`、旧 `store := …`、旧 `svc := …`）替换为下面单块（`store` 在新块内先于 dispatcher 构造声明，全部底层件同源）：

```go
	store := deliveryrepo.NewDeliveryStore(db)
	dispatcher := NewDeliveryDispatcher(DispatcherDeps{
		Connections: connections, Creds: connections, Guard: guard,
		GitHub: factory, Workspace: workspace, Store: store,
		ActionRows: actionStore, Runs: fixtureRun{sessionID: "s-1"},
	})
	actions := appconnectorsvc.NewActionService(actionStore, guard, nil, dispatcher, dispatcher)
	svc := NewCodeDeliveryService(CodeDeliveryDeps{
		Store: store, Actions: actions, ActionRows: actionStore,
		Connections: connections, Creds: connections,
		GitHub: factory, Workspace: workspace, Runs: fixtureRun{sessionID: "s-1"},
		Dispatcher: dispatcher,
	})
```

（同时删除函数签名的变参 `dispatcher ...appconnectorsvc.ActionDispatcher`。dispatcher 与 service 共享**同一** db/emulator/connections/workspace/store 实例——绝无两套底层件。Task 5 的调用点 `newDeliveryFixture(t, nil)`/`newDeliveryFixture(t, func(root string) { … })` 不传变参，删除变参后依旧编译；`deliveryFixture` 已加 `dispatcher *DeliveryDispatcher` 字段并在构造尾部 `f.dispatcher = dispatcher` 回填。）

3. `dispatchFixture`（放 `service_dispatch_test.go`）退化为直通别名（夹具内部已装配 dispatcher）：

```go
// dispatchFixture：夹具内部已单实例装配 DeliveryDispatcher（Task 6 改造后），
// 直通即可；prepare+approve 完成后返回已批准待派发的种子。
func dispatchFixture(t *testing.T, mutate func(root string)) *deliveryFixture {
	return newDeliveryFixture(t, mutate)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/codedelivery/ -count=1`
Expected: PASS（Task 1–6 全部用例；关键断言：部分完成恢复的调用计数、unknown→pushed→delivered 收敛链、零 merge/保护分支违规）

- [ ] **Step 5: Commit**

```bash
git add internal/modules/codedelivery/dispatcher.go internal/modules/codedelivery/service.go internal/modules/codedelivery/service_prepare_test.go internal/modules/codedelivery/service_dispatch_test.go internal/modules/codedelivery/github.go internal/modules/codedelivery/github_client.go internal/modules/codedelivery/github_wire_test.go
git commit -m "feat(codedelivery): approved dispatch with partial-completion recovery, unknown resolution from remote facts, receipts traceable to task/run (T22 #52 task 6)"
```


### Task 7: Go——workbench HTTP 面与容器接线

**Files:**
- Create: `internal/handler/session/workbench_delivery.go`
- Test: `internal/handler/session/workbench_delivery_test.go`
- Create: `internal/container/code_delivery.go`
- Modify: `internal/router/routes_workbench.go`（文件尾追加 `RegisterWorkbenchDeliveryRoutes`）
- Modify: `internal/router/router.go:377`（`RegisterWorkbenchCommandRoutes` 调用行后加 1 行）
- Modify: `internal/container/container.go:274`（workbench Provide 块后加 2 行）

**Interfaces:**
- Consumes: Task 5/6 的 `CodeDeliveryService` 全部方法与 `DeliveryView`；`session.OwnedRunReader`/`resolveOwnedRun`/`resolveReadableRun` 谓词（`internal/handler/session/workbench_read.go:23-25/:181/:206`——本 handler 与其同包，直接复用包级 `resolveOwnedRun`；读面经构造器注入 `GrantedRunReader` 走与 #42 相同的 fallback）；`*repository.AgentRunStore`；`sandbox.Manager`/`sandbox.TenantSandboxResolver`（`tenant_resolver.go:69-73`）；`appconnectorsvc` 各面（Task 5/6）。
- Produces（Task 8 契约冻结的 wire 形状）:
  - 路由（全部挂既有 Viewer + apiKeyChat 边界；读组与 terminal-log 同挂 `workbenchReadGate`）：
    - `POST /workbench/executions/:run_id/baseline`（owner-only；体 `{connection_id, repo, baseline_sha}` → 201 `{success:true,data:{files,root}}`）
    - `POST /workbench/executions/:run_id/delivery`（owner-only；体 `{connection_id, repo, baseline_sha, branch?, commit_message, pr_title}` → 201 `{success:true,data:{delivery:{…DeliveryView…}}}`）
    - `GET /workbench/executions/:run_id/delivery`（owner+granted 读；无交付 404 `code_delivery_not_found`）
    - `POST /workbench/executions/:run_id/delivery/:delivery_id/dispatch`（owner-only；体空）
    - `POST /workbench/executions/:run_id/delivery/:delivery_id/resolve`（owner-only）
  - `func NewWorkbenchDeliveryHandler(runs OwnedRunReader, granted GrantedRunReader, service DeliveryService) *WorkbenchDeliveryHandler`；`type DeliveryService interface { PrepareDelivery(ctx, codedelivery.PrepareInput) (codedelivery.DeliveryView, error); DispatchDelivery(ctx, codedelivery.DispatchInput) (codedelivery.DeliveryView, error); ResolveDeliveryUnknown(ctx, codedelivery.DispatchInput) (codedelivery.DeliveryView, error); GetDeliveryForRun(ctx, tenantID uint64, runID string) (codedelivery.DeliveryView, error); MaterializeBaseline(ctx, codedelivery.BaselineInput) (codedelivery.BaselineReceipt, error) }`（handler 文件内定义，测试以 stub 注入）
  - 容器：`internal/container/code_delivery.go` 提供 `newCodeDeliveryService`（内建交付专用 `appconnectorsvc.NewActionService` + `DeliveryDispatcher` + `sandboxWorkspaceSource` 适配器）与 `NewWorkbenchDeliveryHandler` provider。

- [ ] **Step 1: 写失败测试**

创建 `internal/handler/session/workbench_delivery_test.go`（沿用 `workbench_terminal_log_test.go` 的 gin 直调 + stub 范式；自包含 sqlite 夹具不经过全量迁移轨道）：

```go
package session

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/codedelivery"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type deliveryRunsStub struct{}

func (deliveryRunsStub) GetOwnedRun(ctx context.Context, tenantID uint64, ownerID, runID string) (runtime.Run, error) {
	if tenantID == 1 && ownerID == "u1" && runID == "run-1" {
		return runtime.Run{Key: runtime.RunKey{TenantID: tenantID, RunID: runID}, SessionID: "s-1", Owner: ownerID}, nil
	}
	return runtime.Run{}, errors.New("run_not_found")
}

type deliveryGrantedStub struct{}

func (deliveryGrantedStub) GetRunForGrantedReader(ctx context.Context, tenantID uint64, readerID, runID string) (runtime.Run, error) {
	if tenantID == 1 && readerID == "u2" && runID == "run-1" {
		return runtime.Run{Key: runtime.RunKey{TenantID: tenantID, RunID: runID}, SessionID: "s-1", Owner: "u1"}, nil
	}
	return runtime.Run{}, errors.New("run_not_found")
}

type deliveryServiceStub struct {
	prepared  int
	readRuns  int
	prepErr   error
	lastView  codedelivery.DeliveryView
}

func (s *deliveryServiceStub) MaterializeBaseline(ctx context.Context, in codedelivery.BaselineInput) (codedelivery.BaselineReceipt, error) {
	return codedelivery.BaselineReceipt{Files: 2, Root: "/workspace/octocat/hello"}, nil
}

func (s *deliveryServiceStub) PrepareDelivery(ctx context.Context, in codedelivery.PrepareInput) (codedelivery.DeliveryView, error) {
	s.prepared++
	if s.prepErr != nil {
		return codedelivery.DeliveryView{}, s.prepErr
	}
	s.lastView = codedelivery.DeliveryView{
		ID: "dlv-1", TaskID: "s-1", RunID: in.RunID, State: "prepared",
		Repo: in.Repo.String(), Branch: "weknora/task/s-1",
		ActionID: "act-1", ActionState: "awaiting_approval", Digest: "d1", Files: 2,
	}
	return s.lastView, nil
}

func (s *deliveryServiceStub) DispatchDelivery(ctx context.Context, in codedelivery.DispatchInput) (codedelivery.DeliveryView, error) {
	return codedelivery.DeliveryView{ID: in.DeliveryID, State: "delivered", CommitSHA: "c1", PRNumber: 1, Approver: "u1"}, nil
}

func (s *deliveryServiceStub) ResolveDeliveryUnknown(ctx context.Context, in codedelivery.DispatchInput) (codedelivery.DeliveryView, error) {
	return codedelivery.DeliveryView{ID: in.DeliveryID, State: "pushed"}, nil
}

func (s *deliveryServiceStub) GetDeliveryForRun(ctx context.Context, tenantID uint64, runID string) (codedelivery.DeliveryView, error) {
	s.readRuns++
	return codedelivery.DeliveryView{
		ID: "dlv-1", TaskID: "s-1", RunID: runID, State: "delivered",
		Repo: "octocat/hello", Branch: "weknora/task/s-1", CommitSHA: "c1234567890abcdef",
		PRNumber: 7, PRURL: "https://github.com/octocat/hello/pull/7",
		RemoteLogin: "octocat", ActionID: "act-1", ActionState: "succeeded",
		Digest: "d1", Approver: "u1", Files: 2,
	}, nil
}

func deliveryContext(method, target, body, userID string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	ctx := c.Request.Context()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	c.Request = c.Request.WithContext(ctx)
	return c, recorder
}

func TestDeliveryPrepareOwnerOnly(t *testing.T) {
	svc := &deliveryServiceStub{}
	h := NewWorkbenchDeliveryHandler(deliveryRunsStub{}, deliveryGrantedStub{}, svc)

	// owner：201 + 交付视图（审批锚点字段可见）。
	c, rec := deliveryContext(http.MethodPost, "/api/v1/workbench/executions/run-1/delivery",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+"b0000000000000000000000000000000000000000"+`","commit_message":"m","pr_title":"t"}`, "u1")
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	h.PrepareDelivery(c)
	require.Equal(t, http.StatusCreated, c.Writer.Status())
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Delivery struct {
				ID          string `json:"id"`
				State       string `json:"state"`
				Branch      string `json:"branch"`
				ActionState string `json:"action_state"`
				Digest      string `json:"digest"`
			} `json:"delivery"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "prepared", body.Data.Delivery.State)
	require.Equal(t, "awaiting_approval", body.Data.Delivery.ActionState)

	// 非 owner（u2 有 grant 也只是读权限）：404，不产生交付。
	c, _ = deliveryContext(http.MethodPost, "/api/v1/workbench/executions/run-1/delivery",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+"b0000000000000000000000000000000000000000"+`","commit_message":"m","pr_title":"t"}`, "u2")
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	h.PrepareDelivery(c)
	require.Equal(t, http.StatusNotFound, c.Writer.Status())
	require.Equal(t, 1, svc.prepared)
}

func TestDeliveryReadOpenToGrantedViewer(t *testing.T) {
	svc := &deliveryServiceStub{}
	h := NewWorkbenchDeliveryHandler(deliveryRunsStub{}, deliveryGrantedStub{}, svc)

	c, rec := deliveryContext(http.MethodGet, "/api/v1/workbench/executions/run-1/delivery", "", "u2")
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	h.GetDelivery(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	var body struct {
		Data struct {
			Delivery struct {
				CommitSHA   string `json:"commit_sha"`
				PRURL       string `json:"pr_url"`
				RemoteLogin string `json:"remote_login"`
				Approver    string `json:"approver"`
			} `json:"delivery"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "c1234567890abcdef", body.Data.Delivery.CommitSHA)
	require.Equal(t, "u1", body.Data.Delivery.Approver)

	// 无 grant 的第三者：404。
	c, _ = deliveryContext(http.MethodGet, "/api/v1/workbench/executions/run-1/delivery", "", "u3")
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	h.GetDelivery(c)
	require.Equal(t, http.StatusNotFound, c.Writer.Status())
}

func TestDeliveryDispatchOwnerOnlyAndBaselineMaterializes(t *testing.T) {
	svc := &deliveryServiceStub{}
	h := NewWorkbenchDeliveryHandler(deliveryRunsStub{}, deliveryGrantedStub{}, svc)

	c, rec := deliveryContext(http.MethodPost, "/api/v1/workbench/executions/run-1/baseline",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+"b0000000000000000000000000000000000000000"+`"}`, "u1")
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	h.MaterializeBaseline(c)
	require.Equal(t, http.StatusCreated, c.Writer.Status())

	c, rec = deliveryContext(http.MethodPost, "/api/v1/workbench/executions/run-1/delivery/dlv-1/dispatch", "", "u2")
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}, {Key: "delivery_id", Value: "dlv-1"}}
	h.DispatchDelivery(c)
	require.Equal(t, http.StatusNotFound, c.Writer.Status())

	c, rec = deliveryContext(http.MethodPost, "/api/v1/workbench/executions/run-1/delivery/dlv-1/dispatch", "", "u1")
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}, {Key: "delivery_id", Value: "dlv-1"}}
	h.DispatchDelivery(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	require.Contains(t, rec.Body.String(), "\"state\":\"delivered\"")
}
```



- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/handler/session/ -run 'TestDelivery' -count=1`
Expected: FAIL（`NewWorkbenchDeliveryHandler` 未定义，编译错误）

- [ ] **Step 3: 写最小实现**

创建 `internal/handler/session/workbench_delivery.go`：

```go
package session

import (
	"context"
	"errors"
	"net/http"
	"strings"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/codedelivery"
	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// DeliveryService is the codedelivery seam this handler drives. The
// production *codedelivery.CodeDeliveryService satisfies it; tests stub it.
type DeliveryService interface {
	MaterializeBaseline(ctx context.Context, in codedelivery.BaselineInput) (codedelivery.BaselineReceipt, error)
	PrepareDelivery(ctx context.Context, in codedelivery.PrepareInput) (codedelivery.DeliveryView, error)
	DispatchDelivery(ctx context.Context, in codedelivery.DispatchInput) (codedelivery.DeliveryView, error)
	ResolveDeliveryUnknown(ctx context.Context, in codedelivery.DispatchInput) (codedelivery.DeliveryView, error)
	GetDeliveryForRun(ctx context.Context, tenantID uint64, runID string) (codedelivery.DeliveryView, error)
}

// WorkbenchDeliveryHandler owns the developer-delivery endpoints. Writes
// (baseline/prepare/dispatch/resolve) are owner-only; the read face reuses
// the strict-owner + task-grant fallback predicate (#42 read face).
type WorkbenchDeliveryHandler struct {
	runs    OwnedRunReader
	granted GrantedRunReader
	service DeliveryService
}

func NewWorkbenchDeliveryHandler(runs OwnedRunReader, granted GrantedRunReader, service DeliveryService) *WorkbenchDeliveryHandler {
	return &WorkbenchDeliveryHandler{runs: runs, granted: granted, service: service}
}

func (h *WorkbenchDeliveryHandler) caller(c *gin.Context) (uint64, string, bool) {
	tenantID, ok1 := c.Value(types.TenantIDContextKey).(uint64)
	userID, ok2 := c.Value(types.UserIDContextKey).(string)
	if !ok1 || !ok2 || tenantID == 0 || userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "code": "unauthorized", "error": "tenant and user identity required"})
		return 0, "", false
	}
	return tenantID, userID, true
}
```

（`caller` 直接使用 `types.TenantIDContextKey`/`UserIDContextKey`，import 补 `"github.com/Tencent/WeKnora/internal/types"` 与 `"context"`——与 `resolveOwnedRun` 相同的取法；本计划剩余方法体均为同构模板，完整给出：）

```go
type deliveryBaselineInput struct {
	ConnectionID string `json:"connection_id"`
	Repo         string `json:"repo"`
	BaselineSHA  string `json:"baseline_sha"`
}

// MaterializeBaseline POST /workbench/executions/:run_id/baseline — owner-only.
func (h *WorkbenchDeliveryHandler) MaterializeBaseline(c *gin.Context) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return
	}
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	var input deliveryBaselineInput
	if err := c.ShouldBindJSON(&input); err != nil || input.ConnectionID == "" || input.BaselineSHA == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_request", "error": "connection_id, repo and baseline_sha are required"})
		return
	}
	repo, err := codedelivery.ParseRepoRef(input.Repo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_repo", "error": "repo must be owner/name"})
		return
	}
	receipt, err := h.service.MaterializeBaseline(c.Request.Context(), codedelivery.BaselineInput{
		TenantID: tenantID, CallerID: userID, RunID: run.Key.RunID,
		ConnectionID: input.ConnectionID, Repo: repo, BaselineSHA: input.BaselineSHA,
	})
	if err != nil {
		writeDeliveryError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"files": receipt.Files, "root": receipt.Root}})
}

type deliveryPrepareInput struct {
	ConnectionID  string `json:"connection_id"`
	Repo          string `json:"repo"`
	BaselineSHA   string `json:"baseline_sha"`
	Branch        string `json:"branch"`
	CommitMessage string `json:"commit_message"`
	PRTitle       string `json:"pr_title"`
}

// PrepareDelivery POST /workbench/executions/:run_id/delivery — owner-only.
func (h *WorkbenchDeliveryHandler) PrepareDelivery(c *gin.Context) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return
	}
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	var input deliveryPrepareInput
	if err := c.ShouldBindJSON(&input); err != nil || input.ConnectionID == "" || input.BaselineSHA == "" ||
		input.CommitMessage == "" || input.PRTitle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_request", "error": "connection_id, repo, baseline_sha, commit_message and pr_title are required"})
		return
	}
	repo, err := codedelivery.ParseRepoRef(input.Repo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "invalid_repo", "error": "repo must be owner/name"})
		return
	}
	view, err := h.service.PrepareDelivery(c.Request.Context(), codedelivery.PrepareInput{
		TenantID: tenantID, CallerID: userID, RunID: run.Key.RunID,
		ConnectionID: input.ConnectionID, Repo: repo, BaselineSHA: input.BaselineSHA,
		Branch: strings.TrimSpace(input.Branch), CommitMessage: input.CommitMessage, PRTitle: input.PRTitle,
	})
	if err != nil {
		writeDeliveryError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"delivery": view}})
}

// GetDelivery GET /workbench/executions/:run_id/delivery — owner + granted.
func (h *WorkbenchDeliveryHandler) GetDelivery(c *gin.Context) {
	tenantID, _, ok := h.caller(c)
	if !ok {
		return
	}
	if _, readable := h.resolveReadable(c); !readable {
		return
	}
	view, err := h.service.GetDeliveryForRun(c.Request.Context(), tenantID, c.Param("run_id"))
	if err != nil {
		writeDeliveryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"delivery": view}})
}

// DispatchDelivery POST /workbench/executions/:run_id/delivery/:delivery_id/dispatch — owner-only.
func (h *WorkbenchDeliveryHandler) DispatchDelivery(c *gin.Context) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return
	}
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	view, err := h.service.DispatchDelivery(c.Request.Context(), codedelivery.DispatchInput{
		TenantID: tenantID, CallerID: userID, RunID: run.Key.RunID, DeliveryID: c.Param("delivery_id"),
	})
	if err != nil {
		writeDeliveryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"delivery": view}})
}

// ResolveDeliveryUnknown POST /workbench/executions/:run_id/delivery/:delivery_id/resolve — owner-only.
func (h *WorkbenchDeliveryHandler) ResolveDeliveryUnknown(c *gin.Context) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return
	}
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	view, err := h.service.ResolveDeliveryUnknown(c.Request.Context(), codedelivery.DispatchInput{
		TenantID: tenantID, CallerID: userID, RunID: run.Key.RunID, DeliveryID: c.Param("delivery_id"),
	})
	if err != nil {
		writeDeliveryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"delivery": view}})
}

// resolveReadable mirrors WorkbenchReadHandler.resolveReadableRun (strict
// owner first, task-grant fallback) without importing its private state.
func (h *WorkbenchDeliveryHandler) resolveReadable(c *gin.Context) (agentruntime.Run, bool) {
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

// writeDeliveryError maps codedelivery failures onto a fixed code table;
// no upstream text or credential-adjacent string crosses the wire.
func writeDeliveryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, codedelivery.ErrNotDeliveryOwner),
		errors.Is(err, codedelivery.ErrConnectionNotUsable):
		c.JSON(http.StatusForbidden, gin.H{"success": false, "code": "code_delivery_forbidden", "error": "delivery requires the run owner's personal connection"})
	case errors.Is(err, codedelivery.ErrProtectedBranch):
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "code_delivery_protected_branch", "error": "the target branch is protected or is the default branch"})
	case errors.Is(err, codedelivery.ErrDeliveryState):
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "code_delivery_state_conflict", "error": "delivery state does not allow this operation"})
	case errors.Is(err, deliveryrepo.ErrDeliveryNotFound):
		c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "code_delivery_not_found", "error": "delivery not found"})
	default:
		var apiErr *codedelivery.GitHubAPIError
		if errors.As(err, &apiErr) {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "code": "code_delivery_provider_refused", "error": "the code platform refused the delivery"})
			return
		}
		if errors.Is(err, codedelivery.ErrGitHubTransport) {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "code": "code_delivery_provider_unreachable", "error": "the code platform outcome is unobservable"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "code_delivery_failed", "error": "delivery failed"})
	}
}
```



`internal/router/routes_workbench.go` 文件尾追加：

```go
// RegisterWorkbenchDeliveryRoutes exposes the developer code-delivery
// surface (T22 #52): baseline materialization, delivery prepare/dispatch and
// the traceability read. Writes are owner-only (handler predicate); the read
// reuses the granted-read face. Same Viewer/API-key boundary as the other
// workbench lanes.
func RegisterWorkbenchDeliveryRoutes(r *gin.RouterGroup, h *session.WorkbenchDeliveryHandler, g *rbacGuards) {
	if g == nil || h == nil {
		return
	}
	reads := g.apiKeyGroup(r.Group("/workbench/executions", g.Viewer(), workbenchReadGate(g.cfg)), apiKeyChat(apiKeyFullAccess()))
	reads.GET("/:run_id/delivery", h.GetDelivery)
	writes := g.apiKeyGroup(r.Group("/workbench/executions", g.Viewer()), apiKeyChat(apiKeyFullAccess()))
	writes.POST("/:run_id/baseline", h.MaterializeBaseline)
	writes.POST("/:run_id/delivery", h.PrepareDelivery)
	writes.POST("/:run_id/delivery/:delivery_id/dispatch", h.DispatchDelivery)
	writes.POST("/:run_id/delivery/:delivery_id/resolve", h.ResolveDeliveryUnknown)
}
```

`internal/router/router.go` 在 `RegisterWorkbenchCommandRoutes(v1, params.WorkbenchCommandHandler, rbacGuards)` 行后加：

```go
		RegisterWorkbenchDeliveryRoutes(v1, params.WorkbenchDeliveryHandler, rbacGuards)
```

并在 `params` 结构体 `WorkbenchCommandHandler` 字段后加一行：

```go
	WorkbenchDeliveryHandler   *session.WorkbenchDeliveryHandler     `optional:"true"`
```

创建 `internal/container/code_delivery.go`：

```go
// Package container: code delivery wiring (T22 #52). The delivery-dedicated
// ActionService instance rides its OWN dispatcher so the global OC pipeline
// is untouched; the workspace adapter maps the sandbox session file surface
// onto the codedelivery port (same pattern as the artifact collector).
package container

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler/session"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/codedelivery"
	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"gorm.io/gorm"
)

// sandboxWorkspaceSource adapts the deployment's sandbox managers onto the
// codedelivery workspace port: per call, the tenant resolver wins over the
// process default, and only *sandbox.SessionBoundManager (Cube/E2B/Docker)
// can serve — anything else is ErrWorkspaceUnavailable (fail closed).
type sandboxWorkspaceSource struct {
	mgr      sandbox.Manager
	resolver sandbox.TenantSandboxResolver
}

func (s *sandboxWorkspaceSource) source(ctx context.Context, tenantID uint64) (codedelivery.WorkspaceFileSource, error) {
	mgr := s.mgr
	if s.resolver != nil {
		if resolved, err := s.resolver.Resolve(ctx, tenantID, ""); err == nil {
			mgr = resolved
		}
	}
	if bound, ok := mgr.(*sandbox.SessionBoundManager); ok {
		return bound, nil
	}
	return nil, codedelivery.ErrWorkspaceUnavailable
}

// tenantOf 从请求上下文取租户（codedelivery 的三个工作区方法都在请求
// 上下文内调用；缺租户即 fail closed）。
func tenantOf(ctx context.Context) (uint64, error) {
	if tenantID, ok := types.TenantIDFromContext(ctx); ok && tenantID != 0 {
		return tenantID, nil
	}
	return 0, codedelivery.ErrWorkspaceUnavailable
}

func (s *sandboxWorkspaceSource) ListSessionFiles(ctx context.Context, sessionID, dir string) ([]codedelivery.WorkspaceDirEntry, error) {
	tenantID, err := tenantOf(ctx)
	if err != nil {
		return nil, err
	}
	src, err := s.source(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	entries, err := src.ListSessionFiles(ctx, sessionID, dir)
	if err != nil {
		return nil, err
	}
	out := make([]codedelivery.WorkspaceDirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, codedelivery.WorkspaceDirEntry{Path: e.Path, IsDir: e.Type == sandbox.RemoteEntryDir, Size: e.Size})
	}
	return out, nil
}

func (s *sandboxWorkspaceSource) ReadSessionFile(ctx context.Context, sessionID, path string) ([]byte, error) {
	tenantID, err := tenantOf(ctx)
	if err != nil {
		return nil, err
	}
	src, err := s.source(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return src.ReadSessionFile(ctx, sessionID, path)
}

func (s *sandboxWorkspaceSource) WriteSessionWorkspaceFiles(ctx context.Context, sessionID string, files []codedelivery.WorkspaceFileWrite) error {
	tenantID, err := tenantOf(ctx)
	if err != nil {
		return err
	}
	src, err := s.source(ctx, tenantID)
	if err != nil {
		return err
	}
	mapped := make([]sandbox.SessionWorkspaceFile, 0, len(files))
	for _, f := range files {
		mapped = append(mapped, sandbox.SessionWorkspaceFile{Path: f.Path, Content: f.Content})
	}
	return src.WriteSessionWorkspaceFiles(ctx, sessionID, mapped)
}

// newCodeDeliveryService is the dig provider: it builds the dedicated
// dispatcher + ActionService + service in one place (no dig type collisions
// with the global A03 instance).
func newCodeDeliveryService(
	db *gorm.DB,
	actionStore appconnectorsvc.ActionStoreSource,
	guard appconnectorsvc.A02Guard,
	connections appconnectorsvc.ConnectionCredentialSource,
	runs *repository.AgentRunStore,
	sandboxMgr sandbox.Manager,
	resolver sandbox.TenantSandboxResolver,
) (*codedelivery.CodeDeliveryService, error) {
	workspace := &sandboxWorkspaceSource{mgr: sandboxMgr, resolver: resolver}
	store := deliveryrepo.NewDeliveryStore(db)
	creds := appconnectorsvc.NewCredentialResolver(connections)
	factory := codedelivery.NewGitHubClientFactory(nil, codedelivery.GitHubAPIBaseURL)
	dispatcher := codedelivery.NewDeliveryDispatcher(codedelivery.DispatcherDeps{
		Connections: connections, Creds: creds, Guard: guard, GitHub: factory,
		Workspace: workspace, Store: store, ActionRows: actionStore, Runs: runs,
	})
	actions := appconnectorsvc.NewActionService(actionStore, guard, nil, dispatcher, dispatcher)
	return codedelivery.NewCodeDeliveryService(codedelivery.CodeDeliveryDeps{
		Store: store, Actions: actions, ActionRows: actionStore,
		Connections: connections, Creds: creds, GitHub: factory,
		Workspace: workspace, Runs: runs, Dispatcher: dispatcher,
	}), nil
}

// NewWorkbenchDeliveryHandler wires the handler to the run store (owner
// predicate) and the granted-read fallback (#42 face).
func NewWorkbenchDeliveryHandler(
	runs *repository.AgentRunStore,
	service *codedelivery.CodeDeliveryService,
) *session.WorkbenchDeliveryHandler {
	return session.NewWorkbenchDeliveryHandler(runs, runs, service)
}
```

（import 补 `"github.com/Tencent/WeKnora/internal/types"`；`types.TenantIDFromContext(ctx)` 是既有 helper（`task_collaboration_run_test.go:47` 在用），工作区方法都在请求上下文内调用，租户从 ctx 取，缺租户 fail closed 为 `ErrWorkspaceUnavailable`。）

`internal/container/container.go` 在 `must(container.Provide(NewWorkbenchListHandler))`（:274 附近）后加：

```go
	// T22 (#52): code delivery — dedicated A03 instance + workbench handler.
	must(container.Provide(newCodeDeliveryService))
	must(container.Provide(NewWorkbenchDeliveryHandler))
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/handler/session/ -run 'TestDelivery' -count=1 && go build ./... && go test ./internal/modules/codedelivery/ -count=1`
Expected: PASS + 构建成功（容器/路由接线的编译正确性由 `go build ./...` 证明；既有 `TestWorkbenchStartHTTPIntegrationAndIdentityIsolation` 等预存在失败不受影响——本计划不触碰迁移轨道）

- [ ] **Step 5: Commit**

```bash
git add internal/handler/session/workbench_delivery.go internal/handler/session/workbench_delivery_test.go internal/container/code_delivery.go internal/router/routes_workbench.go internal/router/router.go internal/container/container.go
git commit -m "feat(codedelivery): workbench delivery endpoints (baseline/prepare/read/dispatch/resolve) with owner predicates + container wiring (T22 #52 task 7)"
```

---

### Task 8: contracts + api-client——交付读模型与授权通道远端

**Files:**
- Create: `packages/contracts/src/mobile/code-delivery.ts`
- Test: `packages/contracts/test/mobile-code-delivery.test.ts`
- Modify: `packages/contracts/src/index.ts:661-662`（`parseInteractionWithRun` 导出块后加 3 行）
- Create: `packages/api-client/src/mobile/code-delivery.ts`
- Test: `packages/api-client/src/mobile/code-delivery.test.ts`
- Modify: `packages/api-client/package.json`（exports 加 `"./mobile/code-delivery": "./src/mobile/code-delivery.ts"`）

**Interfaces:**
- Consumes: Go wire 形状（Task 7 的 `{"success":true,"data":{"delivery":{…}}}`，字段名与 `codedelivery.DeliveryView` 的 json tag 逐字一致：`id/task_id/run_id/state/repo/baseline_sha/branch/commit_sha/pr_number/pr_url/remote_login/action_id/action_state/digest/approver/failure/files/created_at/updated_at`）；api-client 的 `ClientRequest`/`requireDeploymentOrigin` 范式（`packages/api-client/src/mobile/materials.ts:1-21`）。
- Produces（Task 9/10 依赖）:
  - contracts：`export type CodeDeliveryState = 'prepared' | 'dispatched' | 'pushed' | 'delivered' | 'failed' | 'unknown'`；`export interface CodeDeliveryRecord { id; taskId; runId; state: CodeDeliveryState; repo; baselineSha; branch; commitSha?; prNumber?; prUrl?; remoteLogin?; actionId; actionState; digest; approver?; failure?; files: number; createdAt; updatedAt }`；`export function parseCodeDeliveryRecord(value: unknown): CodeDeliveryRecord`（未知 state 抛 `ContractError`，与 `parseInteraction` 同 fail-closed 风格）
  - api-client：`export interface MobileCodeDeliveryRemote { delivery(runId: string): Promise<CodeDeliveryRecord | null> }`（404 `code_delivery_not_found` → `null`）；`export function createMobileCodeDeliveryRemote(options: { origin: string; request: (input: ClientRequest) => Promise<unknown> }): MobileCodeDeliveryRemote`

- [ ] **Step 1: 写失败测试**

创建 `packages/contracts/test/mobile-code-delivery.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseCodeDeliveryRecord } from '../src/mobile/code-delivery.ts';
import { ContractError } from '../src/index.ts';

// 真实服务端 wire（codedelivery.DeliveryView json tag 逐字一致）。
const deliveredWire = {
  id: 'dlv-1', task_id: 's-1', run_id: 'run-1', state: 'delivered',
  repo: 'octocat/hello', baseline_sha: 'b0000000000000000000000000000000000000000',
  branch: 'weknora/task/s-1', commit_sha: 'c1', pr_number: 7,
  pr_url: 'https://github.com/octocat/hello/pull/7', remote_login: 'octocat',
  action_id: 'act-1', action_state: 'succeeded', digest: 'd1', approver: 'u1',
  failure: '', files: 2, created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:01:00Z',
};

test('delivered wire parses into the traceability record', () => {
  const record = parseCodeDeliveryRecord(deliveredWire);
  assert.equal(record.state, 'delivered');
  assert.equal(record.taskId, 's-1');
  assert.equal(record.commitSha, 'c1');
  assert.equal(record.prNumber, 7);
  assert.equal(record.approver, 'u1');
  assert.equal(record.files, 2);
});

test('prepared wire parses with optional receipts omitted', () => {
  const record = parseCodeDeliveryRecord({
    ...deliveredWire, state: 'prepared', commit_sha: '', pr_number: 0,
    pr_url: '', remote_login: '', approver: '',
  });
  assert.equal(record.state, 'prepared');
  assert.equal(record.commitSha, undefined);
  assert.equal(record.prNumber, undefined);
  assert.equal(record.approver, undefined);
});

test('unknown state fails closed', () => {
  assert.throws(() => parseCodeDeliveryRecord({ ...deliveredWire, state: 'merged' }), ContractError);
  assert.throws(() => parseCodeDeliveryRecord('nope'), ContractError);
  assert.throws(() => parseCodeDeliveryRecord({ ...deliveredWire, id: '' }), ContractError);
});
```

创建 `packages/api-client/src/mobile/code-delivery.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileCodeDeliveryRemote } from './code-delivery.ts';

const okDeliveryWire = {
  id: 'dlv-1', task_id: 's-1', run_id: 'run-1', state: 'pushed',
  repo: 'octocat/hello', baseline_sha: 'b0000000000000000000000000000000000000000',
  branch: 'weknora/task/s-1', commit_sha: 'c1', pr_number: 0, pr_url: '',
  remote_login: 'octocat', action_id: 'act-1', action_state: 'dispatched',
  digest: 'd1', approver: 'u1', failure: '', files: 1,
  created_at: '2026-09-24T00:00:00Z', updated_at: '2026-09-24T00:00:30Z',
};

function fakeRequest(reply: (input: any) => { status: number; body: unknown }) {
  const seen: any[] = [];
  return {
    seen,
    request: async (input: any) => {
      seen.push(input);
      const { status, body } = reply(input);
      if (status >= 400) {
        const err = new Error(`api error ${status}`) as any;
        err.status = status;
        err.body = body;
        throw err;
      }
      return { success: true, data: body };
    },
  };
}

test('delivery() maps GET /workbench/executions/:run/delivery onto the record', async () => {
  const transport = fakeRequest(() => ({ status: 200, body: { delivery: okDeliveryWire } }));
  const remote = createMobileCodeDeliveryRemote({ origin: 'https://weknora.example.com', request: transport.request });
  const record = await remote.delivery('run-1');
  assert.equal(record?.state, 'pushed');
  assert.equal(transport.seen[0].path, '/api/v1/workbench/executions/run-1/delivery');
  assert.equal(transport.seen[0].method, 'GET');
});

test('a 404 code_delivery_not_found maps to null, other failures reject', async () => {
  const notFound = fakeRequest(() => ({ status: 404, body: { code: 'code_delivery_not_found' } }));
  const remote = createMobileCodeDeliveryRemote({ origin: 'https://weknora.example.com', request: notFound.request });
  assert.equal(await remote.delivery('run-1'), null);

  const forbidden = fakeRequest(() => ({ status: 403, body: { code: 'code_delivery_forbidden' } }));
  const remote2 = createMobileCodeDeliveryRemote({ origin: 'https://weknora.example.com', request: forbidden.request });
  await assert.rejects(() => remote2.delivery('run-1'));
});

test('origin is validated at construction', () => {
  assert.throws(() => createMobileCodeDeliveryRemote({ origin: 'http://127.0.0.1:8080', request: async () => ({}) }));
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/contracts/test/mobile-code-delivery.test.ts packages/api-client/src/mobile/code-delivery.test.ts`
Expected: FAIL（模块不存在）

- [ ] **Step 3: 写最小实现**

创建 `packages/contracts/src/mobile/code-delivery.ts`：

```ts
import { ContractError } from '../index.ts';

/** T22 (#52)：代码交付生命周期（Go codedelivery.DeliveryState 逐字镜像）。 */
export type CodeDeliveryState = 'prepared' | 'dispatched' | 'pushed' | 'delivered' | 'failed' | 'unknown';

/** 交付追溯记录：审批锚点（action/digest/approver）+ 远端回执（commit/PR/实际远端身份）。 */
export interface CodeDeliveryRecord {
  id: string;
  taskId: string;
  runId: string;
  state: CodeDeliveryState;
  repo: string;
  baselineSha: string;
  branch: string;
  commitSha?: string;
  prNumber?: number;
  prUrl?: string;
  remoteLogin?: string;
  actionId: string;
  actionState: string;
  digest: string;
  approver?: string;
  failure?: string;
  files: number;
  createdAt: string;
  updatedAt: string;
}

const STATES: ReadonlySet<string> = new Set(['prepared', 'dispatched', 'pushed', 'delivered', 'failed', 'unknown']);

function str(row: Record<string, unknown>, key: string): string {
  const value = row[key];
  if (typeof value !== 'string') throw new ContractError(`code delivery ${key} must be a string`);
  return value;
}

function optional(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined;
}

/** 解析 GET /workbench/executions/:run_id/delivery 的 delivery 行；未知状态/缺字段 fail closed。 */
export function parseCodeDeliveryRecord(value: unknown): CodeDeliveryRecord {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new ContractError('code delivery record must be an object');
  const row = value as Record<string, unknown>;
  const state = str(row, 'state');
  if (!STATES.has(state)) throw new ContractError(`unknown code delivery state: ${state}`);
  const id = str(row, 'id');
  const actionId = str(row, 'action_id');
  const digest = str(row, 'digest');
  if (id === '' || actionId === '' || digest === '') throw new ContractError('code delivery identity fields are required');
  const files = row.files;
  if (typeof files !== 'number') throw new ContractError('code delivery files must be a number');
  const prNumber = row.pr_number;
  if (prNumber !== undefined && typeof prNumber !== 'number') throw new ContractError('code delivery pr_number must be a number');
  return {
    id, taskId: str(row, 'task_id'), runId: str(row, 'run_id'), state,
    repo: str(row, 'repo'), baselineSha: str(row, 'baseline_sha'), branch: str(row, 'branch'),
    ...(optional(row.commit_sha) === undefined ? {} : { commitSha: optional(row.commit_sha) }),
    ...(typeof prNumber === 'number' && prNumber > 0 ? { prNumber } : {}),
    ...(optional(row.pr_url) === undefined ? {} : { prUrl: optional(row.pr_url) }),
    ...(optional(row.remote_login) === undefined ? {} : { remoteLogin: optional(row.remote_login) }),
    actionId, actionState: str(row, 'action_state'), digest,
    ...(optional(row.approver) === undefined ? {} : { approver: optional(row.approver) }),
    ...(optional(row.failure) === undefined ? {} : { failure: optional(row.failure) }),
    files, createdAt: str(row, 'created_at'), updatedAt: str(row, 'updated_at'),
  };
}
```

`packages/contracts/src/index.ts` 在 `parseInteractionWithRun` 导出块后加：

```ts
export { parseCodeDeliveryRecord } from './mobile/code-delivery.ts';
export type { CodeDeliveryRecord, CodeDeliveryState } from './mobile/code-delivery.ts';
```

创建 `packages/api-client/src/mobile/code-delivery.ts`：

```ts
import { parseCodeDeliveryRecord, type CodeDeliveryRecord } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface CodeDeliveryRemoteOptions {
  /** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输、不持有 token。 */
  request: Request;
}

/** 与 mobile-core DeliveryRemote 结构逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface MobileCodeDeliveryRemote {
  delivery(runId: string): Promise<CodeDeliveryRecord | null>;
}

function isDeliveryNotFound(error: unknown): boolean {
  const status = (error as { status?: unknown } | null)?.status;
  const code = (error as { body?: { code?: unknown } } | null)?.body?.code;
  return status === 404 && code === 'code_delivery_not_found';
}

/** GET /workbench/executions/:run_id/delivery —— 交付追溯读面（授权通道）。 */
export function createMobileCodeDeliveryRemote(options: CodeDeliveryRemoteOptions): MobileCodeDeliveryRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  return {
    async delivery(runId: string): Promise<CodeDeliveryRecord | null> {
      try {
        const response = await request({
          method: 'GET',
          path: `/api/v1/workbench/executions/${encodeURIComponent(runId)}/delivery`,
        });
        const envelope = response as { success?: unknown; data?: unknown };
        if (envelope?.success !== true || typeof envelope.data !== 'object' || envelope.data === null) {
          throw new Error('code delivery response must be a success envelope');
        }
        const delivery = (envelope.data as { delivery?: unknown }).delivery;
        if (delivery === undefined) throw new Error('code delivery response must carry a delivery row');
        return parseCodeDeliveryRecord(delivery);
      } catch (error) {
        if (isDeliveryNotFound(error)) return null;
        throw error;
      }
    },
  };
}
```

`packages/api-client/package.json` exports 加（`"./mobile/materials"` 行后）：

```json
    "./mobile/code-delivery": "./src/mobile/code-delivery.ts",
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/contracts/test/mobile-code-delivery.test.ts packages/api-client/src/mobile/code-delivery.test.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/mobile/code-delivery.ts packages/contracts/test/mobile-code-delivery.test.ts packages/contracts/src/index.ts packages/api-client/src/mobile/code-delivery.ts packages/api-client/src/mobile/code-delivery.test.ts packages/api-client/package.json
git commit -m "feat(mobile): code delivery read model contracts + authorized api-client remote (T22 #52 task 8)"
```

---

### Task 9: mobile-core——交付投影与 scope-guard 读器

**Files:**
- Create: `packages/mobile-core/src/delivery/delivery-view.ts`
- Create: `packages/mobile-core/src/delivery/delivery-reader.ts`
- Create: `packages/mobile-core/src/delivery/in-memory-delivery-remote.ts`
- Test: `packages/mobile-core/src/delivery/delivery-view.test.ts`
- Test: `packages/mobile-core/src/delivery/delivery-reader.test.ts`
- Modify: `packages/mobile-core/src/index.ts`（末尾追加 delivery 导出块）

**Interfaces:**
- Consumes: Task 8 的 `CodeDeliveryRecord`（结构逐字一致，mobile-core 自持同形类型，不 import contracts——依赖方向约束）；`RuntimeScopeLease`/`leaseActive`（`packages/mobile-core/src/runtime/scope-lease.ts`，包内可见）。
- Produces（Task 10 依赖的精确签名）:
  - `export type DeliveryState = 'prepared' | 'dispatched' | 'pushed' | 'delivered' | 'failed' | 'unknown'`；`export interface DeliveryReceiptView { deliveryId: string; taskId: string; runId: string; state: DeliveryState; repo: string; branch: string; baselineSha: string; commitSha?: string; prNumber?: number; prUrl?: string; remoteLogin?: string; approver?: string; failure?: string; attention: boolean; updatedAt: string }`
  - `export interface DeliveryRemoteRecord { id: string; taskId: string; runId: string; state: DeliveryState; repo: string; baselineSha: string; branch: string; commitSha?: string; prNumber?: number; prUrl?: string; remoteLogin?: string; actionId: string; actionState: string; digest: string; approver?: string; failure?: string; files: number; createdAt: string; updatedAt: string }`
  - `export function deliveryViewOf(record: DeliveryRemoteRecord): DeliveryReceiptView`（`attention = state === 'prepared' || state === 'unknown' || state === 'pushed'`——待审批/待收敛/待恢复需要 Owner 行动）
  - `export interface DeliveryRemote { delivery(runId: string): Promise<DeliveryRemoteRecord | null> }`
  - `export type DeliveryReaderErrorCode = 'DELIVERY_SCOPE_CHANGED' | 'DELIVERY_BACKEND'`；`export class DeliveryReaderError extends Error { code: DeliveryReaderErrorCode }`
  - `export function createDeliveryReader(ports: { remote: DeliveryRemote; lease(): ScopeLease | undefined }): DeliveryReader`；`export interface DeliveryReader { read(runId: string): Promise<DeliveryReceiptView | undefined> }`（lease 缺失→DELIVERY_SCOPE_CHANGED；迟到 lease 失效→丢弃）
  - `export function createScenarioDeliveryRemote(script: { runId: string; record: DeliveryRemoteRecord | null }[]): DeliveryRemote`
  - index.ts 导出上述全部 + type 导出。

- [ ] **Step 1: 写失败测试**

创建 `packages/mobile-core/src/delivery/delivery-view.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { deliveryViewOf, type DeliveryRemoteRecord } from './delivery-view.ts';

const base: DeliveryRemoteRecord = {
  id: 'dlv-1', taskId: 's-1', runId: 'run-1', state: 'delivered',
  repo: 'octocat/hello', baselineSha: 'b'.repeat(40), branch: 'weknora/task/s-1',
  commitSha: 'c1', prNumber: 7, prUrl: 'https://github.com/octocat/hello/pull/7',
  remoteLogin: 'octocat', actionId: 'act-1', actionState: 'succeeded',
  digest: 'd1', approver: 'u1', files: 2,
  createdAt: '2026-09-24T00:00:00Z', updatedAt: '2026-09-24T00:01:00Z',
};

test('delivered view carries receipts and needs no attention', () => {
  const view = deliveryViewOf(base);
  assert.equal(view.state, 'delivered');
  assert.equal(view.prUrl, 'https://github.com/octocat/hello/pull/7');
  assert.equal(view.approver, 'u1');
  assert.equal(view.attention, false);
});

test('prepared/pushed/unknown states demand owner attention', () => {
  for (const state of ['prepared', 'pushed', 'unknown'] as const) {
    assert.equal(deliveryViewOf({ ...base, state }).attention, true, state);
  }
  assert.equal(deliveryViewOf({ ...base, state: 'dispatched' }).attention, false);
  assert.equal(deliveryViewOf({ ...base, state: 'failed', failure: 'pr: 422' }).attention, false);
});

test('partial push keeps the commit receipt without pr fields', () => {
  const view = deliveryViewOf({ ...base, state: 'pushed', prNumber: undefined, prUrl: undefined });
  assert.equal(view.commitSha, 'c1');
  assert.equal(view.prNumber, undefined);
});
```

创建 `packages/mobile-core/src/delivery/delivery-reader.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createDeliveryReader, DeliveryReaderError, type DeliveryRemote, type DeliveryRemoteRecord } from './delivery-reader.ts';
import { createScenarioDeliveryRemote } from './in-memory-delivery-remote.ts';

const record: DeliveryRemoteRecord = {
  id: 'dlv-1', taskId: 's-1', runId: 'run-1', state: 'prepared',
  repo: 'octocat/hello', baselineSha: 'b'.repeat(40), branch: 'weknora/task/s-1',
  actionId: 'act-1', actionState: 'awaiting_approval', digest: 'd1', files: 1,
  createdAt: '2026-09-24T00:00:00Z', updatedAt: '2026-09-24T00:00:00Z',
};

import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';

function mintLease(): { lease: ScopeLease; revoke: () => void } {
  const internal = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.com', userId: 'u1', tenantId: 't1' });
  return { lease: internal.asScopeLease(), revoke: () => internal.revoke() };
}

test('read projects the record under a live lease', async () => {
  const { lease } = mintLease();
  const reader = createDeliveryReader({ remote: createScenarioDeliveryRemote([{ runId: 'run-1', record }]), lease: () => lease });
  const view = await reader.read('run-1');
  assert.equal(view?.state, 'prepared');
  assert.equal(view?.attention, true);
});

test('read without a lease fails closed with DELIVERY_SCOPE_CHANGED', async () => {
  const reader = createDeliveryReader({ remote: createScenarioDeliveryRemote([{ runId: 'run-1', record }]), lease: () => undefined });
  await assert.rejects(() => reader.read('run-1'), (error: unknown) => error instanceof DeliveryReaderError && error.code === 'DELIVERY_SCOPE_CHANGED');
});

test('absent delivery resolves to undefined, remote failure rejects as DELIVERY_BACKEND', async () => {
  const { lease } = mintLease();
  const absent = createDeliveryReader({ remote: createScenarioDeliveryRemote([{ runId: 'run-1', record: null }]), lease: () => lease });
  assert.equal(await absent.read('run-1'), undefined);

  const failing: DeliveryRemote = { delivery: async () => { throw new Error('boom'); } };
  const reader = createDeliveryReader({ remote: failing, lease: () => lease });
  await assert.rejects(() => reader.read('run-1'), (error: unknown) => error instanceof DeliveryReaderError && error.code === 'DELIVERY_BACKEND');
});

test('a late lease revocation drops the in-flight result', async () => {
  const { lease, revoke } = mintLease();
  const slow: DeliveryRemote = {
    delivery: async () => {
      revoke(); // 远端结果返回前 lease 被撤销
      return record;
    },
  };
  const reader = createDeliveryReader({ remote: slow, lease: () => lease });
  await assert.rejects(() => reader.read('run-1'), (error: unknown) => error instanceof DeliveryReaderError && error.code === 'DELIVERY_SCOPE_CHANGED');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/delivery/delivery-view.test.ts packages/mobile-core/src/delivery/delivery-reader.test.ts`
Expected: FAIL（模块不存在）

- [ ] **Step 3: 写最小实现**

创建 `packages/mobile-core/src/delivery/delivery-view.ts`：

```ts
/** T22 (#52)：代码交付只读投影。Screen 只消费 DeliveryReceiptView，
 * 不接触 wire 行、digest 或 action 生命周期。 */

export type DeliveryState = 'prepared' | 'dispatched' | 'pushed' | 'delivered' | 'failed' | 'unknown';

/** 与 contracts CodeDeliveryRecord / api-client MobileCodeDeliveryRemote 结构逐字一致
 * （结构可赋值由 apps/mobile typecheck 证明；mobile-core 不 import contracts）。 */
export interface DeliveryRemoteRecord {
  id: string;
  taskId: string;
  runId: string;
  state: DeliveryState;
  repo: string;
  baselineSha: string;
  branch: string;
  commitSha?: string;
  prNumber?: number;
  prUrl?: string;
  remoteLogin?: string;
  actionId: string;
  actionState: string;
  digest: string;
  approver?: string;
  failure?: string;
  files: number;
  createdAt: string;
  updatedAt: string;
}

export interface DeliveryReceiptView {
  deliveryId: string;
  taskId: string;
  runId: string;
  state: DeliveryState;
  repo: string;
  branch: string;
  baselineSha: string;
  commitSha?: string;
  prNumber?: number;
  prUrl?: string;
  remoteLogin?: string;
  approver?: string;
  failure?: string;
  /** 待 Owner 行动：待审批 / 部分完成待恢复 / 远端待收敛。 */
  attention: boolean;
  updatedAt: string;
}

export function deliveryViewOf(record: DeliveryRemoteRecord): DeliveryReceiptView {
  const attention = record.state === 'prepared' || record.state === 'pushed' || record.state === 'unknown';
  return {
    deliveryId: record.id, taskId: record.taskId, runId: record.runId, state: record.state,
    repo: record.repo, branch: record.branch, baselineSha: record.baselineSha,
    ...(record.commitSha === undefined ? {} : { commitSha: record.commitSha }),
    ...(record.prNumber === undefined ? {} : { prNumber: record.prNumber }),
    ...(record.prUrl === undefined ? {} : { prUrl: record.prUrl }),
    ...(record.remoteLogin === undefined ? {} : { remoteLogin: record.remoteLogin }),
    ...(record.approver === undefined ? {} : { approver: record.approver }),
    ...(record.failure === undefined || record.failure === '' ? {} : { failure: record.failure }),
    attention, updatedAt: record.updatedAt,
  };
}
```

创建 `packages/mobile-core/src/delivery/delivery-reader.ts`：

```ts
import { deliveryViewOf, type DeliveryReceiptView, type DeliveryRemoteRecord } from './delivery-view.ts';
import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';

export interface DeliveryRemote {
  delivery(runId: string): Promise<DeliveryRemoteRecord | null>;
}

export type DeliveryReaderErrorCode = 'DELIVERY_SCOPE_CHANGED' | 'DELIVERY_BACKEND';

export class DeliveryReaderError extends Error {
  readonly code: DeliveryReaderErrorCode;
  constructor(code: DeliveryReaderErrorCode, message: string) {
    super(message);
    this.code = code;
  }
}

export interface DeliveryReader {
  read(runId: string): Promise<DeliveryReceiptView | undefined>;
}

/** scope-guard 读器：无有效 lease 拒绝；lease 在途失效时丢弃迟到结果。 */
export function createDeliveryReader(ports: { remote: DeliveryRemote; lease(): ScopeLease | undefined }): DeliveryReader {
  return {
    async read(runId: string): Promise<DeliveryReceiptView | undefined> {
      const lease = ports.lease();
      if (lease === undefined || !leaseActive(lease)) {
        throw new DeliveryReaderError('DELIVERY_SCOPE_CHANGED', 'delivery reads require an active scope lease');
      }
      let record: DeliveryRemoteRecord | null;
      try {
        record = await ports.remote.delivery(runId);
      } catch (error) {
        throw new DeliveryReaderError('DELIVERY_BACKEND', error instanceof Error ? error.message : 'delivery read failed');
      }
      if (!leaseActive(lease)) {
        throw new DeliveryReaderError('DELIVERY_SCOPE_CHANGED', 'scope changed while the delivery record was in flight');
      }
      return record === null ? undefined : deliveryViewOf(record);
    },
  };
}
```

创建 `packages/mobile-core/src/delivery/in-memory-delivery-remote.ts`：

```ts
import type { DeliveryRemote, DeliveryRemoteRecord } from './delivery-reader.ts';

/** 场景 Adapter：按 runId 脚本化返回记录（null=无交付）。 */
export function createScenarioDeliveryRemote(script: { runId: string; record: DeliveryRemoteRecord | null }[]): DeliveryRemote {
  return {
    async delivery(runId: string): Promise<DeliveryRemoteRecord | null> {
      for (const entry of script) {
        if (entry.runId === runId) return entry.record;
      }
      return null;
    },
  };
}
```

`packages/mobile-core/src/index.ts` 末尾追加：

```ts
// —— T22 (#52) 代码交付只读投影 ——
export { deliveryViewOf } from './delivery/delivery-view.ts';
export type { DeliveryReceiptView, DeliveryRemoteRecord, DeliveryState } from './delivery/delivery-view.ts';
export { createDeliveryReader, DeliveryReaderError } from './delivery/delivery-reader.ts';
export type { DeliveryReader, DeliveryReaderErrorCode, DeliveryRemote } from './delivery/delivery-reader.ts';
export { createScenarioDeliveryRemote } from './delivery/in-memory-delivery-remote.ts';
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/mobile-core/src/delivery/delivery-view.test.ts packages/mobile-core/src/delivery/delivery-reader.test.ts && pnpm --filter @weknora/mobile-core exec tsc --noEmit`
Expected: PASS（若 lease 构造面与测试假设不符，以 `runtime/mobile-runtime.test.ts` 现行用法对齐后复跑）

- [ ] **Step 5: Commit**

```bash
git add packages/mobile-core/src/delivery/ packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): delivery receipt projection + scope-guarded reader (T22 #52 task 9)"
```

---

### Task 10: apps/mobile——TaskDetailScreen 交付回执接线

**Files:**
- Modify: `apps/mobile/src/screens/TaskDetailScreen.tsx`（Props 加 `delivery?: DeliveryReceiptView` + 回执区块）
- Modify: `apps/mobile/src/app/tasks/detail.tsx`（装配 delivery 读取并透传）
- Modify: `apps/mobile/src/composition.ts:278`（`activeTaskMaterial` 后追加 `deliveryFor/activeDeliveryReader`）
- Test: `apps/mobile/src/app-smoke.test.tsx`（末尾追加 2 个测试）

**Interfaces:**
- Consumes: Task 9 的 `DeliveryReceiptView`/`createDeliveryReader`/`DeliveryReaderError`；#33–#35 的 `activeMobileRuntime`/`composition` 范式（`apps/mobile/src/composition.ts:256-278`）；`TaskDetailScreenProps`（`apps/mobile/src/screens/TaskDetailScreen.tsx:6-13`）；api-client `createMobileCodeDeliveryRemote`（Task 8）。
- Produces: `composition.ts` 的 `export function activeDeliveryReader(): DeliveryReader | undefined`（记忆化 per deployment+tenant，与 `activeTaskMaterial` 同构；无活动 Runtime 时 undefined——Screen 呈现「请先登录」之外的空态而非报错）；`DELIVERY_STATE_COPY: Record<DeliveryState, string>`（六态如实中文文案，放 `apps/mobile/src/screens/TaskDetailScreen.tsx`）。

- [ ] **Step 1: 写失败测试**

`apps/mobile/src/app-smoke.test.tsx` 末尾追加：

```ts
test('the task detail screen renders the code delivery receipt section with honest state copy', async () => {
  const { TaskDetailScreen } = await import('../src/screens/TaskDetailScreen.tsx');
  const view = {
    taskId: 's-1', runId: 'run-1', title: '修复问候语', lifecycle: 'active', runStatus: 'completed',
    attention: 'required', executionStatus: 'completed', settlementStatus: 'settled', revision: 3,
    cursor: 9, incomplete: false, connection: 'drained', timeline: [], duplicateSeqs: [],
  };
  const delivery = {
    deliveryId: 'dlv-1', taskId: 's-1', runId: 'run-1', state: 'pushed', repo: 'octocat/hello',
    branch: 'weknora/task/s-1', baselineSha: 'b'.repeat(40), commitSha: 'c1f0', attention: true,
    updatedAt: '2026-09-24T00:00:30Z', remoteLogin: 'octocat',
  };
  const tree = render(TaskDetailScreen, { view, loading: false, delivery });
  const text = JSON.stringify(tree);
  assert.ok(text.includes('代码交付'), 'delivery section is present');
  assert.ok(text.includes('已推送，等待草稿 PR 恢复'), 'pushed state uses honest copy');
  assert.ok(text.includes('octocat/hello'), 'repo is shown');
  assert.ok(text.includes('c1f0'), 'commit sha is shown');
  // 无交付时不渲染区块。
  const without = render(TaskDetailScreen, { view, loading: false });
  assert.ok(!JSON.stringify(without).includes('代码交付'));
});

test('composition exposes a delivery reader only under an authorized runtime', async () => {
  const { activeDeliveryReader } = await import('../src/composition.ts');
  // 未登录（无授权面）：reader 必须是 undefined（fail closed，不抛错）。
  assert.equal(activeDeliveryReader(), undefined, 'no runtime → no reader');
});
```

（第二个测试的装配细节对齐 `app-smoke.test.tsx` 既有 `composition caches instances by deployment scope key` 测试（文件尾部既有用例）的 stub 方式；执行者以该用例的装配代码为模板，断言 `activeDeliveryReader() !== undefined` 与两次调用同实例。）

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL（`delivery` prop/`activeDeliveryReader` 未定义）

- [ ] **Step 3: 写最小实现**

`apps/mobile/src/screens/TaskDetailScreen.tsx`：Props 增加可选 `delivery`，文件顶部 import `import type { DeliveryReceiptView, DeliveryState } from '@weknora/mobile-core';`，并加文案表与区块（放在 `onOpenMaterials` 按钮之后）：

```tsx
export const DELIVERY_STATE_COPY: Record<DeliveryState, string> = {
  prepared: '待审批：审阅 Diff 与候选提交后在行动收件箱批准',
  dispatched: '交付进行中：正在推送任务分支',
  pushed: '已推送，等待草稿 PR 恢复',
  delivered: '草稿 PR 已创建',
  failed: '交付失败',
  unknown: '远端结果待确认',
};

function DeliveryReceiptSection({ delivery }: { delivery: DeliveryReceiptView }) {
  return (
    <View style={{ marginTop: 16, padding: 12, borderWidth: 1, borderColor: '#ccc', borderRadius: 8 }}>
      <Text style={{ fontWeight: '600' }}>代码交付</Text>
      <Text>{DELIVERY_STATE_COPY[delivery.state]}</Text>
      <Text numberOfLines={1}>仓库：{delivery.repo}</Text>
      <Text numberOfLines={1}>分支：{delivery.branch}</Text>
      {delivery.commitSha !== undefined ? <Text numberOfLines={1}>提交：{delivery.commitSha.slice(0, 12)}</Text> : null}
      {delivery.prUrl !== undefined ? <Text numberOfLines={1}>PR：{delivery.prUrl}</Text> : null}
      {delivery.remoteLogin !== undefined ? <Text numberOfLines={1}>远端身份：{delivery.remoteLogin}</Text> : null}
      {delivery.approver !== undefined ? <Text numberOfLines={1}>批准人：{delivery.approver}</Text> : null}
    </View>
  );
}
```

`TaskDetailScreenProps` 加 `delivery?: DeliveryReceiptView;`，解构参数加 `delivery`，`onOpenMaterials` 按钮行后加 `{delivery !== undefined ? <DeliveryReceiptSection delivery={delivery} /> : null}`。

`apps/mobile/src/composition.ts` 在 `activeTaskMaterial` 后追加（与 `taskMaterials`/`cachePut`（`composition.ts:127-140/:266-272`）同一记忆化模式；import 区补 `import { createDeliveryReader, type DeliveryReader } from '@weknora/mobile-core';` 与 `import { createMobileCodeDeliveryRemote } from '@weknora/api-client/mobile/code-delivery';`）：

```ts
const deliveryReaders = new Map<string, DeliveryReader>();

/** Delivery reader 按 deployment scope key 记忆化；lease 由 Runtime 提供，切租户即 fail closed。 */
function deliveryFor(activeRuntime: MobileRuntime, origin: string, tenantId: string): DeliveryReader {
  return cachePut(deliveryReaders, deploymentScopeKey(origin, tenantId), () => {
    const remote = createMobileCodeDeliveryRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) });
    return createDeliveryReader({ remote, lease: () => activeRuntime.scopeLease() });
  });
}

/** 详情路由经此取当前授权 scope 的交付读器（无授权面返回 undefined）。 */
export function activeDeliveryReader(): DeliveryReader | undefined {
  const activeRuntime = runtime();
  const snapshot = activeRuntime.snapshot();
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return undefined;
  return deliveryFor(activeRuntime, snapshot.deployment.origin, snapshot.identity.activeTenantId ?? '');
}
```

`apps/mobile/src/app/tasks/detail.tsx`：`TaskDetailRouteLifecycle` 增加 delivery 态并在 mount 时读取一次（不阻塞详情渲染；失败静默——交付区块缺失是合法空态）：

```tsx
const [delivery, setDelivery] = useState<DeliveryReceiptView | undefined>(undefined);
useEffect(() => {
  const reader = activeDeliveryReader();
  if (reader === undefined) return;
  let cancelled = false;
  reader.read(runId)
    .then((view) => { if (!cancelled) setDelivery(view); })
    .catch(() => { /* 交付区块缺失是合法空态（无交付/未登录），不阻塞详情 */ });
  return () => { cancelled = true; };
}, [runId]);
```

并把 `delivery` 透传给 `<TaskDetailScreen … delivery={delivery} />`（import 补 `activeDeliveryReader` 与 `type DeliveryReceiptView`）。刷新路径 `onRefresh` 不拉交付（回执不因刷新而变；重进页面即重读）。

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: PASS + typecheck 无输出（`DeliveryRemoteRecord` 与 api-client `CodeDeliveryRecord` 的结构可赋值在此证明）

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/screens/TaskDetailScreen.tsx apps/mobile/src/app/tasks/detail.tsx apps/mobile/src/composition.ts apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): task detail delivery receipt section wired through composition (T22 #52 task 10)"
```

---

### Task 11: 集成证据（opt-in）与计划级验证

**Files:**
- Create: `apps/mobile/src/delivery-integration-smoke.ts`
- Test: `apps/mobile/src/delivery-integration-smoke.test.ts`

**Interfaces:**
- Consumes: Task 8 `createMobileCodeDeliveryRemote`；T01–T05 既有 opt-in 范式（`apps/mobile/src/material-integration-smoke.ts:1-54` 的 `disallowedDeploymentHost` 主机防线 + `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`）；`createMobileRuntime`/`createJsonTransport` 装配范式（同文件）。
- Produces: `export type DeliveryIntegrationConfig = { enabled: true; deploymentOrigin: string; email: string; password: string } | { enabled: false; disposition: 'skip' | 'invalid'; reason: string }`；`export interface DeliveryIntegrationEvidence { deploymentOrigin: string; login: 'ok' | 'failed'; executionListed: 'listed' | 'no-tasks' | 'failed'; deliveryRead: 'read' | 'absent' | 'failed'; deliveryState?: string; commitSha?: string; prUrl?: string; approver?: string; remoteLogin?: string; failure?: string; commandTimestamp: string }`；`export function deliveryIntegrationConfig(env: Record<string, string | undefined>): DeliveryIntegrationConfig`；`export async function runDeliveryIntegration(config): Promise<DeliveryIntegrationEvidence>`（真实 transport + 授权通道 + 远端读面；**只读**——不发起 prepare/dispatch，真实交付链证据由 Go 侧 blocked-env 测试承载）；`export function emitDeliveryIntegrationEvidence(evidence, emit)`。

- [ ] **Step 1: 写失败测试**

创建 `apps/mobile/src/delivery-integration-smoke.test.ts`（契约测试：本地恒跑，真实 HTTP 段 opt-in）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { deliveryIntegrationConfig } from './delivery-integration-smoke.ts';

test('config is skipped without env and rejected on private hosts', () => {
  assert.deepEqual(deliveryIntegrationConfig({}), { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' });
  const rejected = deliveryIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://127.0.0.1:8080',
    WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c',
    WEKNORA_MOBILE_TEST_PASSWORD: 'p',
  });
  assert.equal(rejected.enabled, false);
  assert.equal(rejected.disposition, 'invalid');
});

test('config accepts a public https origin and evidence helpers are exported', async () => {
  const smoke = await import('./delivery-integration-smoke.ts');
  const config = smoke.deliveryIntegrationConfig({
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.com',
    WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c',
    WEKNORA_MOBILE_TEST_PASSWORD: 'p',
  });
  assert.equal(config.enabled, true);
  assert.equal(typeof smoke.emitDeliveryIntegrationEvidence, 'function');
  assert.equal(typeof smoke.runDeliveryIntegration, 'function');
});
```

（第一个用例的 reason 字符串与实现保持逐字一致；`http://127.0.0.1:8080` 同时命中「非 HTTPS」与「私网主机」两道防线，任一拒绝码都算 invalid。）

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL（模块不存在）

- [ ] **Step 3: 写最小实现**

创建 `apps/mobile/src/delivery-integration-smoke.ts`（骨架与 material-integration-smoke.ts 同构——transport/runtime 装配照抄该文件 55–140 行模式，读面换 `createMobileCodeDeliveryRemote`；以下给完整配置与证据函数，装配段标注复用行）：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createMobileCodeDeliveryRemote } from '@weknora/api-client/mobile/code-delivery';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice } from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type DeliveryIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface DeliveryIntegrationEvidence {
  deploymentOrigin: string;
  login: 'ok' | 'failed';
  executionListed: 'listed' | 'no-tasks' | 'failed';
  deliveryRead: 'read' | 'absent' | 'failed';
  deliveryState?: string;
  commitSha?: string;
  prUrl?: string;
  approver?: string;
  remoteLogin?: string;
  failure?: string;
  commandTimestamp: string;
}

/** 与 T04/T05/T16 相同的 opt-in 语义（自包含，不跨计划 import 凭据逻辑）。 */
export function deliveryIntegrationConfig(env: Record<string, string | undefined>): DeliveryIntegrationConfig {
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
  if (parsed.protocol !== 'https:' || parsed.username !== '' || parsed.password !== '' || parsed.pathname !== '/' || parsed.search !== '' || parsed.hash !== '') {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  if (disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL')) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL host is disallowed (private/loopback/reserved)' };
  }
  return { enabled: true, deploymentOrigin, email, password };
}

/** 真实 transport + Runtime 授权通道 + 交付读面。只读：不发起 prepare/dispatch
 * （真实交付链证据由 Go 侧 blocked-env 测试承载）。装配与 material-integration-smoke.ts
 * 的 runMaterialIntegration 同构。 */
export async function runDeliveryIntegration(config: Extract<DeliveryIntegrationConfig, { enabled: true }>): Promise<DeliveryIntegrationEvidence> {
  const evidence: DeliveryIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin, login: 'failed', executionListed: 'failed', deliveryRead: 'failed',
    commandTimestamp: new Date().toISOString(),
  };
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
    evidence.login = 'ok';

    const office = createTaskOffice({
      backend: createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
      lease: () => runtime.scopeLease(),
    });
    const page = await office.tasks({});
    if (page.items.length === 0) { evidence.executionListed = 'no-tasks'; evidence.deliveryRead = 'absent'; return evidence; }
    evidence.executionListed = 'listed';

    const remote = createMobileCodeDeliveryRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) });
    for (const item of page.items) {
      const record = await remote.delivery(item.runId);
      if (record !== null) {
        evidence.deliveryRead = 'read';
        evidence.deliveryState = record.state;
        evidence.commitSha = record.commitSha;
        evidence.prUrl = record.prUrl;
        evidence.approver = record.approver;
        evidence.remoteLogin = record.remoteLogin;
        return evidence;
      }
    }
    evidence.deliveryRead = 'absent';
    return evidence;
  } catch (error) {
    evidence.failure = error instanceof Error ? error.message : String(error);
    return evidence;
  } finally {
    runtime.dispose();
  }
}

/** 证据以 JSON 行吐出（供批次报告引用）。 */
export function emitDeliveryIntegrationEvidence(evidence: DeliveryIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify({ kind: 'delivery-integration', ...evidence }));
}
```

（装配与 `material-integration-smoke.ts:55-75` 逐字同构：`createMobileRuntime({credentialStore, clientVersion, remoteFor, authorizedTransport})` + `signIn({deployment, email, password})`；任务列表经 `createTaskOffice` 的 `tasks({})`（`runId` 取自 `page.items[].runId`）。）

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile test`
Expected: PASS（契约用例全绿；真实段在无 env 时天然不跑）

- [ ] **Step 5: Commit**

```bash
git add apps/mobile/src/delivery-integration-smoke.ts apps/mobile/src/delivery-integration-smoke.test.ts
git commit -m "test(mobile): opt-in delivery integration evidence contract (T22 #52 task 11)"
```

---

## 计划级验证（全部任务完成后在 worktree 根执行）

```bash
go test ./internal/modules/codedelivery/... -count=1   && go test ./internal/modules/appconnector/ -run 'TestRiskDeliver' -count=1   && go test ./internal/handler/ -run 'TestGitHubOAuth' -count=1   && go test ./internal/handler/session/ -run 'TestDelivery|TestGetWorkbenchTerminalLog' -count=1   && go build ./...   && pnpm exec tsx --test packages/contracts/test/mobile-code-delivery.test.ts packages/api-client/src/mobile/code-delivery.test.ts packages/mobile-core/src/delivery/delivery-view.test.ts packages/mobile-core/src/delivery/delivery-reader.test.ts   && pnpm --filter @weknora/mobile test   && pnpm --filter @weknora/mobile typecheck
```

验证口径说明：

1. `go test ./internal/application/repository/` 与 `go test ./internal/handler/session/`（全量）在当前 HEAD 因预存在 migrations/sqlite 000112 撞号而失败（276/27 个，见差异记录第 5 条）；本计划验证命令全部定向到受影响包/用例，不新增对全量轨道的依赖。
2. 真实 GitHub 端到端（`WEKNORA_GITHUB_TEST_TOKEN`/`WEKNORA_GITHUB_TEST_REPO`）与移动真实部署证据（`WEKNORA_MOBILE_TEST_*`）缺 env 即 skip——blocked-env，不伪造通过；具备 env 的运行自动产出真实证据。
3. 验收对照：AC1 = Task 5/6 的护栏测试（`RefuseProtectedTarget` 拒绝 + 模拟器零 ref/merge 违规 + 端口无 merge 方法）；AC2 = Task 6 `TestDispatchDeliversAndRecordsTraceableReceipts`（commit SHA/approver+digest/PR 回执落账）+ Task 7 读面；AC3 = Task 6/7 真实 sqlite + 真实 A03 store/guard + 真实 HTTP 字节的 Interface 级测试 + Task 11 opt-in 真实部署证据 + blocked-env 声明。

## 自我审查记录（writing-plans 四项检查）

1. **Spec 覆盖**：User Story 41（固定基线→Task 5 MaterializeBaseline）✓；42（云 Workspace→WorkspaceFileSource 端口）✓；43（审阅 Diff/提交→材料锚定为 digest 绑定的 A03 快照 + #46 Diff 面复用）✓；44（任务分支+草稿 PR、保护分支/合并在代码平台治理→Task 1/5/6 护栏与无 merge 端口）✓；45（个人/租户连接区分→个人连接 owner-only，空间连接不在本 Issue 范围）✓；46（审批记录发起/批准/实际远端身份→code_deliveries + approvals join + remote_login）✓；47（部分完成→pushed 状态 + PR-only 恢复）✓；48（Shell 无远端凭据→令牌只在服务端 git-data REST）✓；三条验收标准逐条映射（见「计划级验证」第 3 点）✓。
2. **占位符扫描**：全文检索 TBD/TODO/占位/示意——0 命中；Task 10/11 的两处装配段均为「与既有文件逐字同构」的真实代码（`cachePut` 记忆化、`createMobileRuntime` 装配），并在文中标注了被复制代码的精确行号；无未定义类型/函数引用。
3. **类型一致性**：`DeliveryView` 字段在 Task 5 定义、Task 6/7 使用一致；`DeliveryState` 六值在 Go（Task 1）与 contracts/mobile-core（Task 8/9）逐字镜像；`WorkspaceFileSource` 三方法在 Task 5 端口、Task 6 使用、Task 7 容器适配器一致；`CodeDeliveryDeps` 在 Task 5 冻结、Task 6 增量加 `Dispatcher` 字段（已注明 Modify）；`DeliveryRemoteRecord`（Task 9）与 contracts `CodeDeliveryRecord`（Task 8）结构逐字一致（apps/mobile typecheck 证明）。
4. **Review Focus 落实**：①保护分支（Task 5/6 测试）②部分完成不重推（Task 6 调用计数断言）③approve-then-rewrite/连接撤销（Task 6 两测试）④unknown 收敛（Task 6 远端事实收敛链）⑤快照畸形（Task 1 ParseDeliveryMaterial + Task 6 篡改测试）——五项全部有所属任务的测试。



