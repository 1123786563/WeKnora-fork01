# T18：Notion 文档发布端到端闭环（Issue #48）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 从确定版本的 Task Artifact 形成 Notion 发布 Action Plan，走既有 A03 审批闭环后创建/更新外部文档——发布前读取外部当前版本并检测冲突（AC1）、超时/未知先核对远端绝不盲重试（AC2）、端到端行为在生产迁移库 + 真实 HTTP 处理器链路上以最高稳定 Interface 验证并保存外部版本与回执（AC3）。

**Architecture:** 仓库已有冻结的 A03 审批管线（`internal/modules/appconnector/service/appconnector/action.go:160` 的 `ActionService`：Prepare→Approve→Execute→ResolveUnknown，store 为权威）与未接线的 Notion create 适配器（`internal/modules/appconnector/notion_create.go:248`）。本计划补四块：①root 包新增 **Notion 更新适配器**（`notion_update.go`：四字段快照含 `expected_version`、执行前 `GET /v1/pages/{id}` 版本预读 + 冲突拒绝、多步写 progress 落库、unknown 语义与 create 一致、Query 远端对账）；②`repository/appconnector` 新增 **发布回执表** `app_publications`（计划行 planned→published/failed/unknown 两阶段写入，同时承载 NO-03 progress；不动冻结的 `ActionRow`）；③新包 `internal/modules/appconnector/publish`：**NotionBridge**（把适配器族桥接为 `ActionDispatcher`+`UnknownResolver`，按快照形状路由 create/update，凭据/出站策略/审批作用域三个端口注入）与 **NotionPublishService**（Artifact 不可变版本→Notion 段落块→快照→`ActionService.Prepare`；Execute/Reconcile 后按行动行权威状态 settle 回执）；④HTTP 面 `POST/GET /apps/notion-publish/*` + 容器接线（发布链使用**第二个 `ActionService` 实例**，同一 store 为权威，不改冻结的 `newOCArmedActionService`——B4 并行 #52 同域避让）。审批复用既有 `POST /apps/actions/:id/approve`。

**Tech Stack:** Go 1.26（gin + gorm + golang-migrate + testify + httptest），单模块 `github.com/Tencent/WeKnora`，全部命令在 worktree 根执行。本计划作者已在本环境实跑基线（2026-09-24）：`go test ./internal/modules/appconnector/... -count=1` 全部 ok（5 个包）；`NOTION_TOKEN=x NOTION_PARENT_PAGE_ID=xxxx-skip go test ./internal/modules/appconnector/ -run TestNotionRealControlledCreate -count=1 -v` 输出 `--- SKIP: TestNotionRealControlledCreate`（**skip is not a pass**——真实 Notion 验收在本环境 blocked-env，见 Task 9）；`go test ./internal/application/repository/ -run 'TestWorkbenchNotificationsTableExistsAfterMigrations' -count=1` **FAIL（预存在损坏，非本计划引入）**：`duplicate migration file: 000112_task_grants.down.sql`——`git ls-files` 实证 `migrations/sqlite/000112` 与 `migrations/versioned/000191` 各被 `task_grants`（#42，ca9b66ee1 先落）与 `agent_adoption_variants`（#59，a3132eaa0 后落）两个 feature 双占，golang-migrate 在加载源目录时即失败，**Task 8 的全部 E2E 与 Task 3 的迁移冒烟在当前 HEAD 不可运行**。Task 0 先修复该链（重编号后落位：adoptions → versioned 000192 / sqlite 000113），本计划自身迁移暂用 **versioned 000193 / sqlite 000114**——同批 plan-t43（task_compliance）/plan-t60（public marketplace 六表）/plan-t67（mobile_device_app）亦声明该组号，集成时按各计划一致的顺延约定仲裁（见差异记录第 5 条）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-48.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（User Story 34；Implementation Decisions；Testing Decisions——尤其「Go integration tests verify authorization, Tenant/Owner/Collaborator predicates, revision and digest CAS, idempotency, partial external success, unknown outcomes and durable checkpoints.」；Out of Scope 的双向同步排除）
- 领域术语：`CONTEXT.md`——「**外部发布（External Publication）**：通过获准的连接，把确定版本的任务产物创建或更新到外部办公系统的操作，并保存获批版本、外部目标、目标版本与操作结果作为回执。发布后外部文档是后续协作的权威版本；再次更新前必须读取外部当前版本并形成新的候选变更。」、「操作计划（Action Plan）」、「任务产物（Task Artifact）」（不可原地覆盖）、「任务协作者」（不授予批准外部副作用权限）
- ADR（按需）：`docs/adr/0004-task-is-session.md`（Task=Session）、`docs/adr/0009-cloud-data-trust-boundary.md`（凭据/密文边界）、`docs/adr/0014-commercial-platform-single-deep-seam.md`（U05 预算门）
- Parent：Issue #30；Blocked by：#38（B3 已交付——`ApproveAction` 审批谓词 `internal/handler/app_connector_action.go:211`）、#46（B3 已交付——不可变版本 `internal/application/repository/artifact_version.go:63` 与 `ReadableArtifactVersion:261`、workbench 材料列表 `internal/handler/session/workbench_artifacts.go`，均在当前 HEAD 亲眼核实）
- 下游（本计划 Produces 供其消费）：#49 飞书发布、#50 Confluence 发布（复用 publish 包 seam：PublicationStore/NotionBridge 形态/HTTP 端点范式）、#51 多操作 Action Plan（复用 app_publications 行为单操作计划记录）

## Global Constraints

以下为批准 Spec / CONTEXT.md / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「发布前读取外部当前版本并检测冲突。」（Issue #48 验收标准 1 原文）
- 「超时和未知结果先核对远端，不盲重试。」（Issue #48 验收标准 2 原文）
- 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」（Issue #48 验收标准 3 原文）
- 「Internal Artifact versions coexist with external office documents. After publication, the external document is the collaboration authority; later updates read its current version first.」（mobile-ai-office-design.md · Implementation Decisions）
- 「The first office write Adapters are Feishu, Notion and Confluence. Existing read/sync capability does not imply write permission.」（同上）
- 「Lead Agent Version, Artifact versions, Action Plans and candidate code commits are immutable approval anchors. Changes invalidate prior approvals.」（同上）——发布快照（含 `expected_version`）在 Prepare 时 Normalize+Digest，任何变化产生新 digest，旧批准失效。
- 「Go integration tests verify authorization, Tenant/Owner/Collaborator predicates, revision and digest CAS, idempotency, partial external success, unknown outcomes and durable checkpoints.」（同上 · Testing Decisions）
- 「Tests target observable behavior at the highest stable Interface.」（同上）
- 「Offline mode permits approved reads, drafts and annotations. It prohibits Run commands, approval, budget expansion and external Actions.」（同上）——本计划纯服务端，不引入任何移动端离线面。
- Out of Scope 逐字：「Bidirectional real-time synchronization between WeKnora Artifacts and external office documents.」（同上）与 CONTEXT.md「_避免_：内置 Office 编辑器、未经审批的自动同步、WeKnora 与外部文档双向实时同步、把外部写入成功等同于任务产物本身。」——本计划只做单向外发 + 回执，不做同步。
- CONTEXT.md「任务产物（Task Artifact）」：「……但不能原地覆盖已存在或已审批的版本。」——发布不修改内部 Artifact 版本行；回执只追加。
- CONTEXT.md「任务协作者」：「协作访问必须显式授予……不会授予使用任务所有者个人连接或批准其外部副作用的权限。」与「任务所有者」：「负责涉及其个人连接……或其他外部副作用的授权。」——发布计划形成沿用 `PrepareAction` 的个人连接 owner 谓词（`internal/handler/app_connector_action.go:168-172`），审批沿用 `ApproveAction` 谓词（`:226-238`）。
- 安全约束（Mimosa，与本需求相关者视为验收条件）：服务端出站请求仅 http/https 且发请求前校验 host、拒绝 localhost/环回/私网/保留地址——既有 `HTTPPolicy.PublicAddress`（`internal/modules/appconnector/http_policy.go:20`）+ `AuthorizedNetworks` 管理员授权通道承担，测试契约双打通过 127.0.0.0/8 测试钩子（`http_policy.go:63-66` 注释明示的 documented test hook）；数据库查询全部参数绑定（本计划新查询一律 gorm `Where("col = ?", v)` 绑定，无字符串拼接 SQL）；凭据只从环境变量读取（`NOTION_TOKEN`/`NOTION_PARENT_PAGE_ID` 经环境变量注入真实测试，源码与测试不写入可用凭据字面量；生产 token 走 `CredentialResolver`→`MCPOAuthBindingStore.LoadCredential`，不在任何响应/日志出现）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计；skip 不是 pass——凡 blocked-env 证据必须显式标注。

**Issue #48 验收标准原文（docs/plans/issue30-sweep/issues/issue-48.md）：**

1. 「发布前读取外部当前版本并检测冲突。」
2. 「超时和未知结果先核对远端，不盲重试。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

**验收标准 3 的本地可验证性说明（blocked-env 声明）：** 真实 Notion API 验收（真实凭据 + 真实远端页面 + 真实版本推进）需要 `NOTION_TOKEN`/`NOTION_PARENT_PAGE_ID`（artifacts/connector-real/notion.env），本环境不存在——本计划作者实跑既有 `TestNotionRealControlledCreate` 确认为 SKIP。因此：**Task 9 的真实 Provider 证据为 blocked-env**（有凭据的运行自动执行，本地 SKIP 且不得伪造）；本地最高稳定 Interface 替代证据 = **Task 8 端到端集成测试**：生产 sqlite 迁移库（golang-migrate 全量 `migrations/sqlite`）+ 真实 `ActionService`/`PublicationStore`/`NotionBridge`/gin 处理器/既有审批端点 + 本地契约双打 Notion HTTP 服务（httptest 实现官方 `GET/POST/PATCH /v1/pages`、`PATCH/GET /v1/blocks/{id}/children` 契约形状）——只有 Notion 网络端点被替换，其余全真；双打在测试注释中明示「NOT the real-provider acceptance」。这与既有 NO-04 证据纪律一致（`notion_create_real_test.go:16`「Gated on NOTION_TOKEN/NOTION_PARENT_PAGE_ID; a skip is never a pass」）。

**与调查结论的差异记录（以代码现状为准）：**

1. 调查称「前置依赖 #38、#46 本身均为 open（issue index）」——issue index 状态滞后于批次事实：B3 完成报告（`docs/plans/issue30-sweep/FINAL-REPORT.md` §1.2「#48←#38✅#46✅」）与本计划作者亲眼核实的当前 HEAD 代码（#38 的 `ApproveAction` 审批谓词与 `run_id` inbox、#46 的 `artifact_versions` 不可变版本 store 与 workbench artifacts 端点）证明两者产出已集成，本计划直接 Consumes。
2. 调查建议回执落在 `ActionRow` 加「外部文档版本字段」——本计划不采纳：`ActionRow`（`internal/modules/appconnector/repository/appconnector/action.go:31`）是被 OC 派发/恢复链共用的冻结投影，且 B4 并行 #52 同动 appconnector 域；回执独立为新表 `app_publications`（两阶段：计划形成写 planned，行动行终态后 settle），同时承载 NO-03 progress，冻结面零改动。
3. 调查称「NotionCreateAdapter 未接线到生产」属实（作者 rg 复核：仅 `notion_create.go` 定义 + 两个测试文件引用）。接线方式：不为通用 `/apps/actions/:id/execute` 改造冻结构造 `NewOCArmedActionService`（`internal/container/open_connector.go:606`，其 dispatcher 只能经冻结构造器注入且 nil=显式拒绝语义），而是发布链持有第二个 `ActionService` 实例（`NewActionService(store, guard, gate, bridge, bridge)`，同一 `ActionStore` 为权威——`action.go:158-160` 明示「this store, not any memory map, is the authority」）。边界如实声明：通用 execute 端点对原生 Notion 动作保持现状（OC 未启用→503 未消费任何额度；OC 启用→`OCDispatcher.Dispatch` 因 `snap.OC == nil` 以 `ErrDispatchNotStarted` 判 failed，`oc_dispatcher.go:138-141`，预发送拒绝、零外发）。Notion 发布动作的执行/对账只经本计划发布端点。
4. 调查称「无更新已有外部文档前的版本读取与冲突检测」属实：全仓库 `notion_update`/`expected_version`/`last_edited_time` 零命中（作者 rg 复核）。Task 1/2 补齐；create 路径保持既有 NO-01/NO-03 契约不动（`notion_create.go` 全文件零修改，B4 并行避让）。
5. **独立计划审查发现的预存在阻塞（本计划作者已实跑复核）**：迁移链在 worktree HEAD 已损坏——`go test ./internal/application/repository/ -run 'TestWorkbenchNotificationsTableExistsAfterMigrations' -count=1` FAIL，报错 `duplicate migration file: 000112_task_grants.down.sql`。根因：`migrations/sqlite/000112` 与 `migrations/versioned/000191` 各被 `task_grants`（#42，commit ca9b66ee1，先落）与 `agent_adoption_variants`（#59，commit a3132eaa0，后落）双占（4 个文件均已提交，非脏文件；`migrations/mysql/`、`migrations/paradedb/` 无此编号，不受影响）。后果：golang-migrate 加载源目录即失败，`internal/application/repository` 全部迁移依赖测试（含 `TestTaskCollaborationEndToEnd`）与本计划 Task 8 E2E 在当前 HEAD 全红。处置：**Task 0 重编号后落的 `agent_adoption_variants` 到 versioned 000192 / sqlite 000113**（保持两 feature 的原始落库相对顺序；已核实 adoptions 迁移仅依赖 `agent_marketplace_listings`/`agent_releases`，不依赖 task_grants，无顺序风险；唯一自引用为 sqlite up 文件第 1 行注释，随迁移一并修正）；本计划 `app_publications` 暂用 **versioned 000193 / sqlite 000114**。重命名对既有环境无脏状态风险：双占期间 golang-migrate 无法加载该目录，任何环境都不可能存在「已应用 191=adoptions」的中间状态（作者已用 /tmp 迁移副本模拟重编号实跑验证：golang-migrate 全量 Up 通过、task_grants 与 adoptions 表均建成，随后清理模拟产物）。

**编号协调（B4 批次事实，作者已核对兄弟计划原文）**：plan-t37/plan-t43 等各自携带与本计划 Task 0 完全相同的重编号任务（同组 git mv + 同一注释修正），集成时去重即可；而 **versioned 000193 / sqlite 000114 被四个计划同时声明**——本计划（app_publications）、plan-t43（task_compliance 两表）、plan-t60（public marketplace 六表）、plan-t67（mobile_device_app 两表）。四方计划原文已写明同一顺延约定：「若集成时编号已被占用，整体顺延为下一个可用编号（内容不变），不得挤占他人编号」。本计划遵守同一约定：集成时若 193/114 已被先合入者占用，Task 3 的四个迁移文件（含测试内若有路径引用）整体改号为下一可用编号，DDL 与测试断言零变化。

## Review Focus

Spec/领域定义隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **审批窗口内的外部版本漂移（TOCTOU）**：成员批准时看到版本 V0，执行前外部协作者已编辑（V1）。合理行为：执行时重读远端版本，不一致→确定性拒绝且**零写请求**（不是 unknown，更不是覆盖写入）。——Task 2 `TestNotionUpdateConflictZeroWrites`（断言 fake 仅收到 GET、PATCH 计数为 0）+ Task 8 E2E 409 分支。
2. **传输失败/5xx 被误判为 failed 触发盲重试**：一次 PATCH 超时后本地判「失败」再执行一次=重复写入。合理行为：写步骤一切不可观察结局→unknown 停车，只有远端读回能定论。——Task 2 `TestNotionUpdateUnknownOnLostWriteReply`（丢响应→ActionUnknown）+ Task 6 `TestPublishReconcileResolvesUnknownWithoutRedispatch`（断言 fake 写调用数不增长）。
3. **unknown 被重新排队/二次执行（重复页面/重复块）**：合理行为：unknown 状态结构性拒绝再次 dispatch（`ClaimDispatch` 仅从 authorized 起），对账先行。——Task 8 E2E「第二次 publish 409 且 fake 写计数不变」断言 + Task 2 恢复语义（progress 恢复只补剩余块）。
4. **回执漂移或丢失**：回执先于行动行落地、或行动行已终态而回执仍是 planned。合理行为：行动行是权威，回执是其后置投影；settle 失败可重入且不倒改行动行。——Task 3 `TestPublicationSettleTransitions`（终态不可倒改、幂等）+ Task 6 settle 顺序（先行动行后回执）。
5. **越权发布**：非 owner 成员用他人个人连接形成计划、viewer 角色调写端点、跨租户 action id 探测。合理行为：个人连接 owner 谓词 403、写门 403（`CanDriveActionWrites` 仅 owner/admin，`internal/modules/appconnector/access.go:43-45`）、跨租户与不存在统一 404 不泄漏存在性。——Task 7 `TestNotionPublishPlanGates`（NOT_CONNECTION_OWNER/viewer 403/404）+ `TestNotionPublishActionLookupIsTenantScoped`（跨租户 404）。

（快照层面的 approve-then-rewrite——四字段外多一字段被拒——由 Task 1 `TestParseNotionUpdateSnapshotRejectsExtraField` 覆盖，属第 6 类已覆盖项，不占前五。）

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 0 | 前置：修复预存在迁移链损坏（重编号双占的 000191/000112） | `agent_adoption_variants` 迁移重命名至 versioned 000192 / sqlite 000113 + 注释修正；迁移加载测试由红转绿 |
| 1 | Go root 包：更新快照/版本冲突/回执解析纯逻辑 | `notion_update.go`（Part A）+ 测试 |
| 2 | Go root 包：`NotionUpdateAdapter`（预读版本→冲突拒绝→多步写→unknown→Query 对账） | `notion_update.go`（Part B）+ 契约双打测试 |
| 3 | Go：`app_publications` 回执存储 + 双轨迁移 000193/000114 | `publication.go` + 4 个迁移文件 + 测试 |
| 4 | publish 包：Artifact 文本→Notion 段落块纯投影 | `blocks.go` + 测试 |
| 5 | publish 包：`NotionBridge`（Dispatcher+Resolver，create/update 路由，progress→回执行） | `dispatcher.go` + 测试 |
| 6 | publish 包：`NotionPublishService`（FormPlan 版本预读/Execute/Reconcile/回执 settle） | `plan.go` + 测试 |
| 7 | HTTP：`/apps/notion-publish/*` 端点 + 路由 + 容器接线 | `app_connector_notion_publish.go` 等三处新文件 + router.go/container.go 最小修改 + 谓词测试 |
| 8 | E2E：最高稳定 Interface 证据（生产迁移 + 全链 + 迁移对齐断言） | `app_connector_notion_publish_e2e_test.go` |
| 9 | 真实受控集成证据（blocked-env，opt-in） | `notion_publish_real_test.go` |

**并行批次注意（本计划与同批 10 个计划并行实施，#52 同动 appconnector 审批接线与 Go 路由注册）：** Task 0 重命名 4 个既有迁移文件（`agent_adoption_variants`，#59 产物；plan-t37/plan-t43 携带完全相同的重命名，集成去重）并修正两处过时测试注释；本计划新增文件全部为本计划独有（Task 1–9 的 Create 项共 20 个）。共享文件修改仅两处、均为最小且位置明确：`internal/router/router.go`（`RouterParams` 结构体 `AppActionHandler` 字段后加 1 个字段 + `RegisterAppConnectorRoutes(...)` 调用后加 1 行注册调用）；`internal/container/container.go`（`SetOCConnectionService` invoke 块后加 1 个 Provide 块）。`notion_create.go`、`open_connector.go`、`routes_app_connectors.go`、`app_connector_action.go` 等 #52 高概率触碰文件**零修改**。迁移编号：本计划 Task 0 占用 versioned 000192 / sqlite 000113（与兄弟计划一致）；自身迁移暂占 versioned 000193 / sqlite 000114，**与 plan-t43/plan-t60/plan-t67 声明撞号，集成时按四方一致的顺延约定仲裁**（差异记录第 5 条）。

---

### Task 0: 前置——修复预存在的迁移链双占（重编号 `agent_adoption_variants`）

**Files:**
- Modify（重命名）: `migrations/versioned/000191_agent_adoption_variants.up.sql` → `migrations/versioned/000192_agent_adoption_variants.up.sql`
- Modify（重命名）: `migrations/versioned/000191_agent_adoption_variants.down.sql` → `migrations/versioned/000192_agent_adoption_variants.down.sql`
- Modify（重命名 + 首行注释修正）: `migrations/sqlite/000112_agent_adoption_variants.up.sql` → `migrations/sqlite/000113_agent_adoption_variants.up.sql`
- Modify（重命名）: `migrations/sqlite/000112_agent_adoption_variants.down.sql` → `migrations/sqlite/000113_agent_adoption_variants.down.sql`
- Modify: `internal/application/repository/agent_adoption_test.go:245-247`（注释提及「000112 duplicate-number conflict」，重编号后已过时）
- Modify: `internal/application/repository/task_grant_store_test.go:139-141`（同上）

**Interfaces:**
- Consumes: 无（纯迁移文件重编号；不改任何 DDL——golang-migrate 按文件名版本号发现迁移）。
- 并行一致性：同批 B4 的 plan-t37/plan-t43（及按其声明追随的计划）各自携带**本任务的同款重编号**（adoptions → versioned 000192 / sqlite 000113）——集成时为同一组 git mv + 同一行注释修正的完全相同变更，编排层去重即可，无语义冲突。
- Produces: 可加载的生产迁移双轨（`migrations/sqlite` 与 `migrations/versioned` 版本号唯一）——Task 3 的迁移落位（000193/000114）、Task 8 的 `openNotionPublishE2EDB`（golang-migrate 全量加载）与 `internal/application/repository` 全部迁移依赖测试由此解锁。事实依据（作者已实证）：`git log --follow` 显示 `task_grants`（ca9b66ee1，#42）先落、`agent_adoption_variants`（a3132eaa0，#59）后落，故移动后者保持相对顺序；adoptions 迁移仅依赖 `agent_marketplace_listings`/`agent_releases`（更早迁移），不依赖 task_grants；`migrations/mysql/`、`migrations/paradedb/` 无 000191/000112，不受影响。

- [ ] **Step 1: 确认预存在失败（RED——该红非本计划引入，是修复对象本身）**

Run: `go test ./internal/application/repository/ -run 'TestWorkbenchNotificationsTableExistsAfterMigrations' -count=1`
Expected: FAIL，错误含 `duplicate migration file: 000112_task_grants.down.sql`（作者在计划撰写时实跑复核；这是 golang-migrate 在打开 `migrations/sqlite` 源目录时的加载失败，修复前 `internal/application/repository` 的任何迁移依赖测试都无法运行）。

- [ ] **Step 2: 重命名四个迁移文件（versioned 与 sqlite 双轨同步）**

```bash
git mv migrations/versioned/000191_agent_adoption_variants.up.sql migrations/versioned/000192_agent_adoption_variants.up.sql
git mv migrations/versioned/000191_agent_adoption_variants.down.sql migrations/versioned/000192_agent_adoption_variants.down.sql
git mv migrations/sqlite/000112_agent_adoption_variants.up.sql migrations/sqlite/000113_agent_adoption_variants.up.sql
git mv migrations/sqlite/000112_agent_adoption_variants.down.sql migrations/sqlite/000113_agent_adoption_variants.down.sql
```

- [ ] **Step 3: 修正 sqlite up 文件的首行自引用注释**

`migrations/sqlite/000113_agent_adoption_variants.up.sql` 第 1 行由：

```sql
-- SQLite twin of versioned migration 000191.
```

改为：

```sql
-- SQLite twin of versioned migration 000192.
```

（作者已核实这是四个迁移文件中唯一的编号自引用；两个 down 文件与 versioned up 文件内容不含自身编号，零改动。）

随后更新两处过时的 Go 测试注释（B3 批次明示「000112 双号冲突超范围」的让步说明，重编号后不再成立）：

`internal/application/repository/agent_adoption_test.go` 第 245-247 行的注释改为：

```go
// touches (gorm AutoMigrate of the entities). The B3-F87 CAS guard under
// test is store-level SQL semantics, so the case stays runnable independently
// of the migration track (the B4 renumber resolved the former 000112 duplicate).
```

`internal/application/repository/task_grant_store_test.go` 第 139-141 行的注释改为：

```go
// directly. The B3-F74/F75 behaviors under test are store-level SQL semantics
// (NULL scan, upsert conflict), not migration-track properties, so these cases
// stay runnable independently of the migration track (the B4 renumber resolved
// the former 000112 duplicate-number conflict).
```

- [ ] **Step 4: 运行验证（GREEN）**

Run: `go test ./internal/application/repository/ -run 'TestWorkbenchNotificationsTableExistsAfterMigrations|TestTaskCollaborationEndToEnd' -count=1`
Expected: PASS（迁移目录可加载，测试全量跑完 sqlite 迁移链）。

Run（双轨唯一性 shell 断言，versioned 轨无本地 PG 加载测试、以目录级检查为证据；每条迁移本有 up/down 两个文件，故健康状态=任何版本号出现次数 ≤2）:

```bash
test -z "$(ls migrations/sqlite | grep -oE '^[0-9]{6}' | sort | uniq -c | awk '$1 > 2')" && test -z "$(ls migrations/versioned | grep -oE '^[0-9]{6}' | sort | uniq -c | awk '$1 > 2')" && echo "migration versions unique on both tracks"
```

Expected: 输出 `migration versions unique on both tracks`。（作者已在计划撰写时实跑该断言的检测形态：当前 HEAD 下 `ls migrations/sqlite | grep -oE '^[0-9]{6}' | sort | uniq -c | awk '$1 > 2'` 输出 `   4 000112`、versioned 轨输出 `   4 000191`——正是两处双占；Task 0 重命名后两者必须为空。）

- [ ] **Step 5: 提交**

```bash
git add migrations/ internal/application/repository/agent_adoption_test.go internal/application/repository/task_grant_store_test.go
git commit -m "fix(migrations): renumber agent_adoption_variants to 000192/000113 - resolve duplicate-version chain breakage with task_grants (B4 #48 prerequisite)"
```

---

### Task 1: Notion 更新快照、版本冲突与回执解析（纯逻辑）

**Files:**
- Create: `internal/modules/appconnector/notion_update.go`
- Test: `internal/modules/appconnector/notion_update_test.go`

**Interfaces:**
- Consumes: `NormalizeArgs`（`internal/modules/appconnector/action.go:97`）、`ErrNotionSnapshotInvalid`/`ErrNotionOutcomeUnknown`/`Action`/`json.RawMessage`（同包既有）。
- Produces（Task 2/5/6 依赖，签名逐字）:
  - `var ErrNotionVersionConflict = errors.New("notion_version_conflict")`
  - `type NotionUpdateSnapshot struct { PageID string; ExpectedVersion string; Title string; Blocks []json.RawMessage }`
  - `func ParseNotionUpdateSnapshot(args json.RawMessage) (NotionUpdateSnapshot, error)`
  - `func IsNotionUpdateArgs(args json.RawMessage) bool`
  - `func DetectNotionVersionConflict(expected, actual string) error`
  - `type NotionPageVersion struct { PageID string; LastEditedTime string }`
  - `func ParseNotionPageVersion(raw []byte) (NotionPageVersion, error)`
  - `type NotionPageReceipt struct { ExternalID string; ExternalVersion string }`
  - `func ParseNotionPageReceipt(raw []byte) (NotionPageReceipt, error)`

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/appconnector/notion_update_test.go`：

```go
package appconnector

import (
	"encoding/json"
	"errors"
	"testing"
)

func updateArgs(pageID, version, title string, blocks []any) json.RawMessage {
	if blocks == nil {
		blocks = []any{paragraphBlock("hello")}
	}
	raw, _ := json.Marshal(map[string]any{
		"page_id":          pageID,
		"expected_version": version,
		"title":            title,
		"blocks":           blocks,
	})
	return raw
}

func paragraphBlock(text string) map[string]any {
	return map[string]any{
		"object": "block",
		"type":   "paragraph",
		"paragraph": map[string]any{
			"rich_text": []any{map[string]any{
				"type": "text",
				"text": map[string]string{"content": text},
			}},
		},
	}
}

func TestParseNotionUpdateSnapshotAcceptsExactFourFields(t *testing.T) {
	snap, err := ParseNotionUpdateSnapshot(updateArgs("page-1", "2026-09-24T10:00:00.000Z", "T", nil))
	if err != nil {
		t.Fatal(err)
	}
	if snap.PageID != "page-1" || snap.ExpectedVersion != "2026-09-24T10:00:00.000Z" || snap.Title != "T" || len(snap.Blocks) != 1 {
		t.Fatalf("snapshot fields drift: %+v", snap)
	}
}

func TestParseNotionUpdateSnapshotRejectsExtraField(t *testing.T) {
	raw := append([]byte{}, updateArgs("page-1", "v", "T", nil)...)
	raw = raw[:len(raw)-1]
	raw = append(raw, []byte(`,"evil":"x"}`)...)
	if _, err := ParseNotionUpdateSnapshot(raw); !errors.Is(err, ErrNotionSnapshotInvalid) {
		t.Fatalf("approve-then-rewrite must be refused, got %v", err)
	}
}

func TestParseNotionUpdateSnapshotRejectsMissingOrEmpty(t *testing.T) {
	cases := map[string]json.RawMessage{
		"missing page":     []byte(`{"expected_version":"v","title":"T","blocks":[]}`),
		"missing version":  []byte(`{"page_id":"p","title":"T","blocks":[]}`),
		"empty page":       updateArgs("", "v", "T", nil),
		"empty version":    updateArgs("p", "", "T", nil),
		"empty title":      updateArgs("p", "v", "", nil),
		"not an object":    []byte(`["page_id"]`),
		"three fields":     []byte(`{"page_id":"p","title":"T","blocks":[]}`),
	}
	for name, raw := range cases {
		if _, err := ParseNotionUpdateSnapshot(raw); !errors.Is(err, ErrNotionSnapshotInvalid) {
			t.Fatalf("%s: want ErrNotionSnapshotInvalid, got %v", name, err)
		}
	}
}

func TestIsNotionUpdateArgs(t *testing.T) {
	if !IsNotionUpdateArgs(updateArgs("p", "v", "T", nil)) {
		t.Fatal("update-shaped args must be detected")
	}
	create, _ := json.Marshal(map[string]any{"parent": "pp", "title": "T", "blocks": []any{}})
	if IsNotionUpdateArgs(create) {
		t.Fatal("create-shaped args must not be treated as update")
	}
	if IsNotionUpdateArgs([]byte(`not json`)) {
		t.Fatal("garbage must not be update")
	}
}

func TestDetectNotionVersionConflict(t *testing.T) {
	if err := DetectNotionVersionConflict("v1", "v1"); err != nil {
		t.Fatalf("matching version must pass: %v", err)
	}
	if err := DetectNotionVersionConflict("v1", "v2"); !errors.Is(err, ErrNotionVersionConflict) {
		t.Fatalf("drift must conflict, got %v", err)
	}
	if err := DetectNotionVersionConflict("", "v2"); !errors.Is(err, ErrNotionVersionConflict) {
		t.Fatalf("empty expected must fail closed, got %v", err)
	}
	if err := DetectNotionVersionConflict("v1", ""); !errors.Is(err, ErrNotionVersionConflict) {
		t.Fatalf("unreadable remote must fail closed, got %v", err)
	}
}

func TestParseNotionPageVersion(t *testing.T) {
	v, err := ParseNotionPageVersion([]byte(`{"object":"page","id":"p1","last_edited_time":"2026-09-24T10:00:00.000Z","parent":{"type":"page_id","page_id":"pp"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if v.PageID != "p1" || v.LastEditedTime != "2026-09-24T10:00:00.000Z" {
		t.Fatalf("version fields drift: %+v", v)
	}
	if _, err := ParseNotionPageVersion([]byte(`{"object":"page","id":"p1"}`)); err == nil {
		t.Fatal("missing last_edited_time must be an error, never a fabricated version")
	}
	if _, err := ParseNotionPageVersion([]byte(`{"object":"page","last_edited_time":"v"}`)); err == nil {
		t.Fatal("missing id must be an error")
	}
}

func TestParseNotionPageReceipt(t *testing.T) {
	r, err := ParseNotionPageReceipt([]byte(`{"object":"page","id":"p1","last_edited_time":"v9"}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.ExternalID != "p1" || r.ExternalVersion != "v9" {
		t.Fatalf("receipt fields drift: %+v", r)
	}
	if _, err := ParseNotionPageReceipt([]byte(`{"object":"page","id":""}`)); err == nil {
		t.Fatal("no real page id must refuse a receipt")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/appconnector/ -run 'ParseNotionUpdateSnapshot|IsNotionUpdateArgs|DetectNotionVersionConflict|ParseNotionPageVersion|ParseNotionPageReceipt' -count=1`
Expected: FAIL（`undefined: ParseNotionUpdateSnapshot` 等编译错误）。

- [ ] **Step 3: 最小实现（notion_update.go Part A）**

创建 `internal/modules/appconnector/notion_update.go`：

```go
package appconnector

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrNotionVersionConflict: the external page's current version (Notion
// last_edited_time) no longer equals the version the approved update plan
// was formed against. The update is refused BEFORE any write request —
// per CONTEXT.md「外部发布」: 再次更新前必须读取外部当前版本并形成新的
// 候选变更.
var ErrNotionVersionConflict = errors.New("notion_version_conflict")

// NotionUpdateSnapshot is the A03-approved argument snapshot for updating
// ONE existing external page: exactly page_id, expected_version (the
// last_edited_time the plan read before approval — an immutable approval
// anchor), title and blocks. Every dispatched request is built FROM these
// fields; nothing is added, rewritten or re-derived after approval.
type NotionUpdateSnapshot struct {
	PageID          string
	ExpectedVersion string
	Title           string
	Blocks          []json.RawMessage
}

// ParseNotionUpdateSnapshot validates that args are EXACTLY the approved
// four-field update snapshot: no extra fields (approve-then-rewrite), no
// missing fields, non-empty page id / expected version / title, and blocks
// that are each valid JSON (normalized so the wire bytes are the approved
// bytes). Blocks may be empty (a title-only update).
func ParseNotionUpdateSnapshot(args json.RawMessage) (NotionUpdateSnapshot, error) {
	var s NotionUpdateSnapshot
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return s, fmt.Errorf("%w: %v", ErrNotionSnapshotInvalid, err)
	}
	if len(raw) != 4 {
		return s, fmt.Errorf("%w: snapshot must be exactly page_id, expected_version, title, blocks", ErrNotionSnapshotInvalid)
	}
	if err := json.Unmarshal(raw["page_id"], &s.PageID); err != nil {
		return s, fmt.Errorf("%w: page_id: %v", ErrNotionSnapshotInvalid, err)
	}
	if err := json.Unmarshal(raw["expected_version"], &s.ExpectedVersion); err != nil {
		return s, fmt.Errorf("%w: expected_version: %v", ErrNotionSnapshotInvalid, err)
	}
	if err := json.Unmarshal(raw["title"], &s.Title); err != nil {
		return s, fmt.Errorf("%w: title: %v", ErrNotionSnapshotInvalid, err)
	}
	if s.PageID == "" {
		return s, fmt.Errorf("%w: empty page_id", ErrNotionSnapshotInvalid)
	}
	if s.ExpectedVersion == "" {
		return s, fmt.Errorf("%w: empty expected_version", ErrNotionSnapshotInvalid)
	}
	if s.Title == "" {
		return s, fmt.Errorf("%w: empty title", ErrNotionSnapshotInvalid)
	}
	var blocks []json.RawMessage
	if err := json.Unmarshal(raw["blocks"], &blocks); err != nil {
		return s, fmt.Errorf("%w: blocks: %v", ErrNotionSnapshotInvalid, err)
	}
	s.Blocks = make([]json.RawMessage, 0, len(blocks))
	for i, b := range blocks {
		n, err := NormalizeArgs(b)
		if err != nil {
			return s, fmt.Errorf("%w: block %d: %v", ErrNotionSnapshotInvalid, i, err)
		}
		s.Blocks = append(s.Blocks, n)
	}
	return s, nil
}

// IsNotionUpdateArgs reports whether args carry the update snapshot's
// distinguishing key pair (page_id AND expected_version). It never parses
// the full snapshot — the adapter family uses it only to route an approved
// action to the update adapter; full validation still happens in
// ParseNotionUpdateSnapshot.
func IsNotionUpdateArgs(args json.RawMessage) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return false
	}
	_, hasPage := raw["page_id"]
	_, hasVersion := raw["expected_version"]
	return hasPage && hasVersion
}

// DetectNotionVersionConflict compares the approved expected version with
// the version just read from the provider. Anything but an exact match —
// including an unreadable empty side — is a conflict; an unobservable
// remote state must never authorize an overwrite.
func DetectNotionVersionConflict(expected, actual string) error {
	if expected == "" || actual == "" || expected != actual {
		return fmt.Errorf("%w: approved %q but remote has %q", ErrNotionVersionConflict, expected, actual)
	}
	return nil
}

// NotionPageVersion is the reliable read shape of GET /v1/pages/{id} for
// version purposes: the exact page id and its last_edited_time. Notion's
// page object carries last_edited_time as an ISO-8601 string that changes
// on every content/property edit — the external collaboration authority's
// version token (spec: "After publication, the external document is the
// collaboration authority").
type NotionPageVersion struct {
	PageID         string
	LastEditedTime string
}

type notionPageVersionObject struct {
	Object         string `json:"object"`
	ID             string `json:"id"`
	LastEditedTime string `json:"last_edited_time"`
}

// ParseNotionPageVersion extracts the page identity + current version from
// a GET /v1/pages/{id} reply. A reply without a real id or a real
// last_edited_time is an error — a fabricated version is never a basis for
// conflict detection or a receipt.
func ParseNotionPageVersion(raw []byte) (NotionPageVersion, error) {
	var p notionPageVersionObject
	if err := json.Unmarshal(raw, &p); err != nil {
		return NotionPageVersion{}, fmt.Errorf("notion_page_version_unparseable: %v", err)
	}
	if p.ID == "" || p.LastEditedTime == "" {
		return NotionPageVersion{}, fmt.Errorf("notion_page_version_unparseable: no id or last_edited_time")
	}
	return NotionPageVersion{PageID: p.ID, LastEditedTime: p.LastEditedTime}, nil
}

// NotionPageReceipt is the persisted external receipt of one publish: the
// provider page id and the version the publish itself produced (read back
// from the provider's own reply — never fabricated locally).
type NotionPageReceipt struct {
	ExternalID      string
	ExternalVersion string
}

// ParseNotionPageReceipt extracts the receipt fields from a provider page
// payload (the create/update reply the adapter recorded as the action's
// output evidence).
func ParseNotionPageReceipt(raw []byte) (NotionPageReceipt, error) {
	v, err := ParseNotionPageVersion(raw)
	if err != nil {
		return NotionPageReceipt{}, err
	}
	return NotionPageReceipt{ExternalID: v.PageID, ExternalVersion: v.LastEditedTime}, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/appconnector/ -run 'ParseNotionUpdateSnapshot|IsNotionUpdateArgs|DetectNotionVersionConflict|ParseNotionPageVersion|ParseNotionPageReceipt' -count=1`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/appconnector/notion_update.go internal/modules/appconnector/notion_update_test.go
git commit -m "feat(appconnector): notion update snapshot, version conflict and receipt parsing (T18 #48)"
```

---

### Task 2: `NotionUpdateAdapter`——预读版本、冲突拒绝、多步写与 Query 对账

**Files:**
- Modify: `internal/modules/appconnector/notion_update.go`（追加 Part B）
- Test: `internal/modules/appconnector/notion_update_test.go`（追加契约双打与适配器测试）

**Interfaces:**
- Consumes: `HTTPPolicy`（`http_policy.go:67`，`NewClient()` 逐跳校验）、`NotionAPIVersion`/`NotionPageFormat`/`NotionAppendChildrenFormat`/`NotionAppendBatchLimit`/`NotionCapabilityInsert`/`ErrNotionOutcomeUnknown`/`ErrNotionApprovalRevoked`/`ErrNotionMissingCapability`/`ErrNotionNotConfigured`（`notion_create.go:50-68` 常量与哨兵）、`NotionPageProgress`/`Action`/`ActionResult`/`Adapter`。
- Produces: `type NotionUpdateAdapter struct`（字段见下）+ `func (m *NotionUpdateAdapter) Execute(ctx context.Context, a Action) (ActionResult, error)` + `func (m *NotionUpdateAdapter) Query(ctx context.Context, a Action) (ActionResult, error)`，`var _ Adapter = (*NotionUpdateAdapter)(nil)`——Task 5 的 bridge 按此路由。

- [ ] **Step 1: 写失败测试（先写契约双打，再写适配器行为测试）**

向 `internal/modules/appconnector/notion_update_test.go` 追加（文件头 import 增加 `"context"`、`"fmt"`、`"io"`、`"net"`、`"net/http"`、`"net/http/httptest"`、`"sync"`、`"time"`）：

```go
// fakeNotionDocs is a LOCAL contract double of the official Notion page
// endpoints (developers.notion.com): POST/GET/PATCH /v1/pages and
// PATCH/GET /v1/blocks/{id}/children, carrying last_edited_time exactly
// like the real page object. It is NOT the real-provider acceptance — that
// stays NOTION_TOKEN-gated (Task 9, notion_publish_real_test.go).
type fakeNotionDocs struct {
	mu       sync.Mutex
	token    string
	nextID   int
	pages    map[string]*fakeNotionPage
	patchCalls int // page PATCHes (title updates)
	appendCalls int
	pageGets  int
	titlePatches []string
	// knobs: apply the effect then lose the reply (unknown outcome).
	dropNextTitlePatch bool
	dropNextAppend     bool
}

type fakeNotionPage struct {
	id, parent, title, lastEdited string
	children                      []json.RawMessage
}

func newFakeNotionDocs(token string) *fakeNotionDocs {
	return &fakeNotionDocs{token: token, pages: map[string]*fakeNotionPage{}}
}

func (f *fakeNotionDocs) addPage(id, parent, title string) *fakeNotionPage {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := &fakeNotionPage{id: id, parent: parent, title: title, lastEdited: "2026-09-24T08:00:00.000Z"}
	f.pages[id] = p
	return p
}

// touch bumps a page's last_edited_time — an external collaborator's edit.
func (f *fakeNotionDocs) touch(id, when string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.pages[id]; ok {
		p.lastEdited = when
	}
}

func (f *fakeNotionDocs) stats() (patch, appends, gets int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.patchCalls, f.appendCalls, f.pageGets
}

func (f *fakeNotionDocs) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
	mux.HandleFunc("/v1/pages", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+f.token {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		if r.Method != http.MethodPost {
			writeJSON(w, 405, `{"object":"error","status":405,"code":"method_not_allowed","message":"x"}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Parent struct {
				PageID string `json:"page_id"`
			} `json:"parent"`
			Properties struct {
				Title struct {
					Title []struct {
						Text struct {
							Content string `json:"content"`
						} `json:"text"`
					} `json:"title"`
				} `json:"title"`
			} `json:"properties"`
		}
		_ = json.Unmarshal(body, &req)
		f.mu.Lock()
		f.nextID++
		id := fmt.Sprintf("page-%d", f.nextID)
		f.pages[id] = &fakeNotionPage{id: id, parent: req.Parent.PageID, title: req.Properties.Title.Title[0].Text.Content, lastEdited: "2026-09-24T09:00:00.000Z"}
		f.mu.Unlock()
		writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":"2026-09-24T09:00:00.000Z","parent":{"type":"page_id","page_id":%q}}`, id, req.Parent.PageID))
	})
	mux.HandleFunc("/v1/pages/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+f.token {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		id := r.URL.Path[len("/v1/pages/"):]
		f.mu.Lock()
		p, ok := f.pages[id]
		f.mu.Unlock()
		if !ok {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		switch r.Method {
		case http.MethodGet:
			f.mu.Lock()
			f.pageGets++
			edited := p.lastEdited
			f.mu.Unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":%q,"parent":{"type":"page_id","page_id":%q}}`, p.id, edited, p.parent))
		case http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Properties struct {
					Title struct {
						Title []struct {
							Text struct {
								Content string `json:"content"`
							} `json:"text"`
						} `json:"title"`
					} `json:"title"`
				} `json:"properties"`
			}
			_ = json.Unmarshal(body, &req)
			f.mu.Lock()
			f.patchCalls++
			f.titlePatches = append(f.titlePatches, req.Properties.Title.Title[0].Text.Content)
			p.title = req.Properties.Title.Title[0].Text.Content
			p.lastEdited = "2026-09-24T11:00:00.000Z"
			drop := f.dropNextTitlePatch
			if drop {
				f.dropNextTitlePatch = false
			}
			f.mu.Unlock()
			if drop {
				// Effect applied, response lost: the client cannot observe
				// the outcome.
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":%q,"parent":{"type":"page_id","page_id":%q}}`, p.id, p.lastEdited, p.parent))
		default:
			writeJSON(w, 405, `{"object":"error","status":405,"code":"method_not_allowed","message":"x"}`)
		}
	})
	mux.HandleFunc("/v1/blocks/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+f.token {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		rest := r.URL.Path[len("/v1/blocks/"):]
		var id string
		if suffix := "/children"; len(rest) > len(suffix) && rest[len(rest)-len(suffix):] == suffix {
			id = rest[:len(rest)-len(suffix)]
		} else {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		f.mu.Lock()
		p, ok := f.pages[id]
		f.mu.Unlock()
		if !ok {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		switch r.Method {
		case http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Children []json.RawMessage `json:"children"`
			}
			_ = json.Unmarshal(body, &req)
			f.mu.Lock()
			f.appendCalls++
			p.children = append(p.children, req.Children...)
			p.lastEdited = "2026-09-24T12:00:00.000Z"
			drop := f.dropNextAppend
			if drop {
				f.dropNextAppend = false
			}
			f.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			f.mu.Lock()
			results := append([]json.RawMessage(nil), p.children...)
			f.mu.Unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"list","results":[%s]}`, joinRaw(results)))
		case http.MethodGet:
			f.mu.Lock()
			results := append([]json.RawMessage(nil), p.children...)
			f.mu.Unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"list","results":[%s]}`, joinRaw(results)))
		default:
			writeJSON(w, 405, `{"object":"error","status":405,"code":"method_not_allowed","message":"x"}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func joinRaw(items []json.RawMessage) string {
	out := ""
	for i, it := range items {
		if i > 0 {
			out += ","
		}
		out += string(it)
	}
	return out
}

// loopbackPolicy mirrors the documented test hook (http_policy.go:63-66):
// the reviewed policy shape for the Notion contract, with 127.0.0.0/8
// authorized so the httptest double is reachable.
func loopbackPolicy(host, port string) HTTPPolicy {
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	return HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PATCH"}, PathPrefix: "/v1/",
		AuthorizedNetworks: []*net.IPNet{network},
		Timeout:            10 * time.Second,
	}
}

func policyOfServer(srv *httptest.Server) HTTPPolicy {
	u := srv.URL // http://127.0.0.1:port
	host, port, _ := net.SplitHostPort(u[len("http://"):])
	return loopbackPolicy(host, port)
}

func newUpdateAdapter(srv *httptest.Server, progress map[string]NotionPageProgress) *NotionUpdateAdapter {
	return &NotionUpdateAdapter{
		Policy:                  policyOfServer(srv),
		Token:                   func(ctx context.Context) (string, error) { return "secret_test_token", nil },
		ConnectionCapabilities:  func(ctx context.Context, a Action) ([]string, error) { return []string{NotionCapabilityInsert}, nil },
		LoadProgress:            func(a Action) NotionPageProgress { return progress[a.ID] },
		SaveProgress:            func(a Action, p NotionPageProgress) error { progress[a.ID] = p; return nil },
	}
}

func updateAction(args json.RawMessage) Action {
	return Action{ID: "act_upd_1", TenantID: 7, ActorID: "u1", ConnectionID: "conn-notion",
		Version: "notion/v1", Target: "page-9", Risk: RiskWrite, Args: args}
}

func TestNotionUpdatePublishesTitleAndBlocksHappyPath(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	srv := fake.server(t)
	progress := map[string]NotionPageProgress{}
	ad := newUpdateAdapter(srv, progress)

	out, err := ad.Execute(context.Background(), updateAction(updateArgs("page-9", "2026-09-24T08:00:00.000Z", "new title", nil)))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("update: state=%s err=%v", out.State, err)
	}
	if out.ExternalID != "page-9" {
		t.Fatalf("external id: %+v", out)
	}
	rcpt, rerr := ParseNotionPageReceipt(out.Output)
	if rerr != nil || rcpt.ExternalID != "page-9" || rcpt.ExternalVersion == "" {
		t.Fatalf("output must be a receipt-bearing page payload: %+v %v", rcpt, rerr)
	}
	patch, appends, _ := fake.stats()
	if patch != 1 || appends != 1 {
		t.Fatalf("wire counts: patch=%d appends=%d", patch, appends)
	}
	if progress["act_upd_1"].PageID != "page-9" || progress["act_upd_1"].BlocksDone != 1 {
		t.Fatalf("progress must record the resume point: %+v", progress["act_upd_1"])
	}
}

func TestNotionUpdateConflictZeroWrites(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	srv := fake.server(t)
	ad := newUpdateAdapter(srv, map[string]NotionPageProgress{})
	// External collaborator edited AFTER the plan was formed.
	fake.touch("page-9", "2026-09-24T10:30:00.000Z")

	out, err := ad.Execute(context.Background(), updateAction(updateArgs("page-9", "2026-09-24T08:00:00.000Z", "new title", nil)))
	if !errors.Is(err, ErrNotionVersionConflict) || out.State != ActionFailed {
		t.Fatalf("version drift must fail definitively: state=%s err=%v", out.State, err)
	}
	patch, appends, gets := fake.stats()
	if patch != 0 || appends != 0 {
		t.Fatalf("conflict must leave ZERO write requests, got patch=%d appends=%d", patch, appends)
	}
	if gets == 0 {
		t.Fatal("the pre-read version GET must have run")
	}
}

func TestNotionUpdatePreReadFailureIsFailedNotUnknown(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	srv := fake.server(t)
	ad := newUpdateAdapter(srv, map[string]NotionPageProgress{})
	// page-missing: the pre-read 404s — nothing was written, so this is a
	// definitive failure, never an unknown.
	out, err := ad.Execute(context.Background(), updateAction(updateArgs("page-missing", "v", "T", nil)))
	if out.State != ActionFailed || err == nil {
		t.Fatalf("missing page must fail definitively: state=%s err=%v", out.State, err)
	}
	patch, appends, _ := fake.stats()
	if patch != 0 || appends != 0 {
		t.Fatalf("no write may follow an unreadable pre-read: %d/%d", patch, appends)
	}
}

func TestNotionUpdateUnknownOnLostWriteReply(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	srv := fake.server(t)
	ad := newUpdateAdapter(srv, map[string]NotionPageProgress{})
	fake.mu.Lock()
	fake.dropNextTitlePatch = true
	fake.mu.Unlock()

	out, err := ad.Execute(context.Background(), updateAction(updateArgs("page-9", "2026-09-24T08:00:00.000Z", "new title", nil)))
	if out.State != ActionUnknown || !errors.Is(err, ErrNotionOutcomeUnknown) {
		t.Fatalf("lost write reply must park unknown, got state=%s err=%v", out.State, err)
	}
}

func TestNotionUpdateQueryResolvesAfterDroppedAppend(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	srv := fake.server(t)
	progress := map[string]NotionPageProgress{}
	ad := newUpdateAdapter(srv, progress)
	fake.mu.Lock()
	fake.dropNextAppend = true
	fake.mu.Unlock()

	act := updateAction(updateArgs("page-9", "2026-09-24T08:00:00.000Z", "new title", nil))
	out, err := ad.Execute(context.Background(), act)
	if out.State != ActionUnknown {
		t.Fatalf("dropped append reply must park unknown, got %s (%v)", out.State, err)
	}
	// AC2: reconcile by READING the remote first — the effect applied, so
	// the query must confirm success without any new write.
	_, appendsBefore, _ := fake.stats()
	q, qerr := ad.Query(context.Background(), act)
	if qerr != nil || q.State != ActionSucceeded || q.ExternalID != "page-9" {
		t.Fatalf("query must resolve the unknown from the remote state: %+v %v", q, qerr)
	}
	_, appendsAfter, _ := fake.stats()
	if appendsAfter != appendsBefore {
		t.Fatalf("query must not re-send: appends %d -> %d", appendsBefore, appendsAfter)
	}
}

func TestNotionUpdateQueryStaysUnknownWhenBlocksMissing(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	srv := fake.server(t)
	ad := newUpdateAdapter(srv, map[string]NotionPageProgress{})
	// The page still has ZERO children: not provably complete — honest
	// unknown, never fabricated success or failure.
	q, _ := ad.Query(context.Background(), updateAction(updateArgs("page-9", "v", "T", nil)))
	if q.State != ActionUnknown {
		t.Fatalf("unprovable state must stay unknown, got %s", q.State)
	}
}

func TestNotionUpdateQueryVerifiesContainedRun(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	srv := fake.server(t)
	// Earlier foreign blocks precede ours: our approved blocks must be
	// found as a contiguous run, not only as the tail.
	fake.mu.Lock()
	if p := fake.pages["page-9"]; p != nil {
		p.children = []json.RawMessage{mustJSONBlock("earlier foreign"), mustJSONBlock("hello")}
	}
	fake.mu.Unlock()
	ad := newUpdateAdapter(srv, map[string]NotionPageProgress{})
	q, err := ad.Query(context.Background(), updateAction(updateArgs("page-9", "v", "T", nil)))
	if err != nil || q.State != ActionSucceeded {
		t.Fatalf("contained run must verify: %+v %v", q, err)
	}
}

func mustJSONBlock(text string) json.RawMessage {
	b, _ := json.Marshal(paragraphBlock(text))
	return b
}

func TestNotionUpdateResumesOnlyRemainingBlocks(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	// Faithful crash simulation: the FIRST block was delivered to the
	// provider before the crash (progress persisted BlocksDone=1 AFTER a
	// successful append), so the remote page already carries it.
	fake.mu.Lock()
	fake.pages["page-9"].children = []json.RawMessage{mustJSONBlock("one")}
	fake.mu.Unlock()
	srv := fake.server(t)
	progress := map[string]NotionPageProgress{"act_upd_1": {PageID: "page-9", BlocksDone: 1}}
	ad := newUpdateAdapter(srv, progress)
	args, _ := json.Marshal(map[string]any{
		"page_id": "page-9", "expected_version": "2026-09-24T08:00:00.000Z",
		"title": "T", "blocks": []any{paragraphBlock("one"), paragraphBlock("two")},
	})
	out, err := ad.Execute(context.Background(), updateAction(args))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("resume: state=%s err=%v", out.State, err)
	}
	fake.mu.Lock()
	appended := len(fake.pages["page-9"].children)
	fake.mu.Unlock()
	// Correct resume appends ONLY blocks[1:2] → children = {one, two} = 2.
	// A restart-from-zero bug re-appends both → 3; a no-op resume → 1.
	if appended != 2 {
		t.Fatalf("resume must not duplicate the first block: children=%d", appended)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/appconnector/ -run 'NotionUpdate' -count=1`
Expected: FAIL（`undefined: NotionUpdateAdapter`）。

- [ ] **Step 3: 最小实现（notion_update.go 追加 Part B）**

向 `internal/modules/appconnector/notion_update.go` 追加（import 增加 `"io"`、`"net"`、`"net/http"`、`"net/url"`）：

```go
// notionUpdatePageRequest is the wire body of the title PATCH.
type notionUpdatePageRequest struct {
	Properties notionCreatePageProperties `json:"properties"`
}

// NotionUpdateAdapter executes ONE approved page update against the same
// NO-01 reviewed contract family as NotionCreateAdapter: the update is a
// multi-step write — read the current version, PATCH the title, append the
// approved blocks in batches — persisting the block range between steps.
// The version pre-read is the publish conflict gate: the remote
// last_edited_time must still equal the snapshot's approved
// expected_version, otherwise the update is refused with
// ErrNotionVersionConflict and ZERO write requests leave the process.
//
// Outcome semantics (identical to create):
//   - pre-read failures are definitive FAILED (a GET cannot have produced
//     the write; nothing left the process);
//   - every write-step transport failure / 5xx / unparseable reply is
//     ErrNotionOutcomeUnknown — the effect may exist remotely;
//   - Query reconciles ONLY via the reliable page read + children read.
type NotionUpdateAdapter struct {
	// Policy is the admin-reviewed outbound contract (A04).
	Policy HTTPPolicy
	// Token returns the Notion integration token (Authorization: Bearer).
	Token func(ctx context.Context) (string, error)
	// ConnectionCapabilities reports the connection's granted
	// capabilities; the reviewed insert-content capability is required.
	ConnectionCapabilities func(ctx context.Context, a Action) ([]string, error)
	// Recheck re-validates the A03 approval right before the outbound
	// call; a revocation between approval and execute blocks the update.
	Recheck func(ctx context.Context, a Action) error
	// LoadProgress / SaveProgress persist the multi-step recovery record
	// (the same NO-03 record shape as create: real page id + completed
	// block range).
	LoadProgress func(a Action) NotionPageProgress
	SaveProgress func(a Action, p NotionPageProgress) error
	// MaxBatch caps the blocks per append request (test hook; 0 = the
	// documented NotionAppendBatchLimit, never above it).
	MaxBatch int
}

var _ Adapter = (*NotionUpdateAdapter)(nil)

func (m *NotionUpdateAdapter) configError() error {
	if m.Policy.Host == "" || m.Policy.Scheme == "" {
		return fmt.Errorf("%w: no reviewed outbound policy", ErrNotionNotConfigured)
	}
	if m.Token == nil {
		return fmt.Errorf("%w: no token source", ErrNotionNotConfigured)
	}
	return nil
}

func (m *NotionUpdateAdapter) requireInsertCapability(ctx context.Context, a Action) error {
	if m.ConnectionCapabilities == nil {
		return fmt.Errorf("%w: no capability source", ErrNotionMissingCapability)
	}
	caps, err := m.ConnectionCapabilities(ctx, a)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotionMissingCapability, err)
	}
	for _, c := range caps {
		if c == NotionCapabilityInsert {
			return nil
		}
	}
	return fmt.Errorf("%w: connection lacks %s", ErrNotionMissingCapability, NotionCapabilityInsert)
}

func (m *NotionUpdateAdapter) loadProgress(a Action) NotionPageProgress {
	if m.LoadProgress == nil {
		return NotionPageProgress{}
	}
	return m.LoadProgress(a)
}

func (m *NotionUpdateAdapter) storeProgress(a Action, p NotionPageProgress) error {
	if m.SaveProgress == nil {
		return nil
	}
	if err := m.SaveProgress(a, p); err != nil {
		return fmt.Errorf("%w: persisting progress: %v", ErrNotionOutcomeUnknown, err)
	}
	return nil
}

func (m *NotionUpdateAdapter) batchSize() int {
	if m.MaxBatch > 0 && m.MaxBatch < NotionAppendBatchLimit {
		return m.MaxBatch
	}
	return NotionAppendBatchLimit
}

func (m *NotionUpdateAdapter) targetURL(path string) *url.URL {
	host := m.Policy.Host
	if m.Policy.Port != "" {
		host = net.JoinHostPort(host, m.Policy.Port)
	}
	return &url.URL{Scheme: m.Policy.Scheme, Host: host, Path: path}
}

// do performs ONE policy-validated request through the A04 client — the
// same request/redirect re-validation as the create adapter. Transport
// failures wrap ErrNotionOutcomeUnknown.
func (m *NotionUpdateAdapter) do(ctx context.Context, method string, u *url.URL, body []byte) (int, []byte, error) {
	if err := m.Policy.ValidateRequest(method, u); err != nil {
		return 0, nil, err
	}
	tok, err := m.Token(ctx)
	if err != nil {
		return 0, nil, err
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Notion-Version", NotionAPIVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := m.Policy.NewClient().Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %v", ErrNotionOutcomeUnknown, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("%w: %v", ErrNotionOutcomeUnknown, err)
	}
	return resp.StatusCode, raw, nil
}

// readPageVersion performs the version pre-read (GET /v1/pages/{id}).
func (m *NotionUpdateAdapter) readPageVersion(ctx context.Context, pageID string) (NotionPageVersion, error) {
	u := m.targetURL(fmt.Sprintf(NotionPageFormat, url.PathEscape(pageID)))
	status, raw, err := m.do(ctx, http.MethodGet, u, nil)
	if err != nil {
		return NotionPageVersion{}, err
	}
	if status != http.StatusOK {
		e := parseNotionError(raw)
		return NotionPageVersion{}, fmt.Errorf("notion_provider_error: status=%d code=%s message=%s", status, e.Code, e.Message)
	}
	return ParseNotionPageVersion(raw)
}

// patchTitle performs the idempotent title step (PATCH /v1/pages/{id}).
func (m *NotionUpdateAdapter) patchTitle(ctx context.Context, pageID, title string) (json.RawMessage, error) {
	body, err := json.Marshal(notionUpdatePageRequest{
		Properties: notionCreatePageProperties{Title: notionTitleProperty{Title: []notionRichText{{Text: notionText{Content: title}}}}},
	})
	if err != nil {
		return nil, err
	}
	u := m.targetURL(fmt.Sprintf(NotionPageFormat, url.PathEscape(pageID)))
	status, raw, err := m.do(ctx, http.MethodPatch, u, body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		e := parseNotionError(raw)
		if status >= 500 {
			return nil, fmt.Errorf("%w: status=%d code=%s", ErrNotionOutcomeUnknown, status, e.Code)
		}
		return nil, fmt.Errorf("notion_provider_error: status=%d code=%s message=%s", status, e.Code, e.Message)
	}
	return raw, nil
}

// appendChildren performs one recoverable append step of the REMAINING
// blocks.
func (m *NotionUpdateAdapter) appendChildren(ctx context.Context, pageID string, blocks []json.RawMessage) (json.RawMessage, error) {
	body, err := json.Marshal(notionAppendChildrenRequest{Children: blocks})
	if err != nil {
		return nil, err
	}
	u := m.targetURL(fmt.Sprintf(NotionAppendChildrenFormat, url.PathEscape(pageID)))
	status, raw, err := m.do(ctx, http.MethodPatch, u, body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		e := parseNotionError(raw)
		if status >= 500 {
			return nil, fmt.Errorf("%w: status=%d code=%s", ErrNotionOutcomeUnknown, status, e.Code)
		}
		return nil, fmt.Errorf("notion_provider_error: status=%d code=%s message=%s", status, e.Code, e.Message)
	}
	return raw, nil
}

func (m *NotionUpdateAdapter) readChildren(ctx context.Context, pageID string) ([]json.RawMessage, error) {
	u := m.targetURL(fmt.Sprintf(NotionAppendChildrenFormat, url.PathEscape(pageID)))
	status, raw, err := m.do(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("notion_query_unverifiable: status=%d", status)
	}
	var list notionBlockList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("notion_query_unverifiable: %v", err)
	}
	return list.Results, nil
}

// notionBlocksContained reports whether the approved blocks appear in the
// page's children as one contiguous run (external collaborators may have
// appended their own blocks before or after ours).
func notionBlocksContained(children, blocks []json.RawMessage) bool {
	if len(blocks) == 0 {
		return true
	}
	if len(children) < len(blocks) {
		return false
	}
	for start := 0; start+len(blocks) <= len(children); start++ {
		match := true
		for i := range blocks {
			want, werr := NormalizeArgs(blocks[i])
			got, rerr := NormalizeArgs(children[start+i])
			if werr != nil || rerr != nil || !bytes.Equal(want, got) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Execute performs the approved update. Order: A03 recheck, snapshot
// validation, capability check, version PRE-READ + conflict detection —
// all BEFORE any network write — then the multi-step write with progress
// persisted between steps, ending with a reliable read-back whose payload
// is the action's output evidence (the receipt basis).
func (m *NotionUpdateAdapter) Execute(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if m.Recheck != nil {
		if err := m.Recheck(ctx, a); err != nil {
			return ActionResult{State: ActionAwaitingApproval}, fmt.Errorf("%w: %v", ErrNotionApprovalRevoked, err)
		}
	}
	snap, err := ParseNotionUpdateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if err := m.requireInsertCapability(ctx, a); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	// AC1: read the external current version FIRST. A read can never have
	// produced the write, so an unreadable pre-read is a definitive
	// failure — zero write requests leave the process.
	ver, gerr := m.readPageVersion(ctx, snap.PageID)
	if gerr != nil {
		return ActionResult{State: ActionFailed}, gerr
	}
	if cerr := DetectNotionVersionConflict(snap.ExpectedVersion, ver.LastEditedTime); cerr != nil {
		return ActionResult{State: ActionFailed}, cerr
	}
	pageID := snap.PageID
	progress := m.loadProgress(a)
	done := progress.PageID == pageID && progress.BlocksDone >= 0 && progress.BlocksDone <= len(snap.Blocks)
	blocksDone := 0
	if done {
		blocksDone = progress.BlocksDone
	}
	if blocksDone == 0 {
		if _, terr := m.patchTitle(ctx, pageID, snap.Title); terr != nil {
			state := ActionFailed
			if errors.Is(terr, ErrNotionOutcomeUnknown) {
				state = ActionUnknown
			}
			return ActionResult{State: state, ExternalID: pageID}, terr
		}
		if serr := m.storeProgress(a, NotionPageProgress{PageID: pageID, BlocksDone: 0}); serr != nil {
			return ActionResult{State: ActionUnknown, ExternalID: pageID}, serr
		}
	}
	for blocksDone < len(snap.Blocks) {
		end := blocksDone + m.batchSize()
		if end > len(snap.Blocks) {
			end = len(snap.Blocks)
		}
		if _, aerr := m.appendChildren(ctx, pageID, snap.Blocks[blocksDone:end]); aerr != nil {
			state := ActionFailed
			if errors.Is(aerr, ErrNotionOutcomeUnknown) {
				state = ActionUnknown
			}
			return ActionResult{State: state, ExternalID: pageID}, aerr
		}
		blocksDone = end
		if serr := m.storeProgress(a, NotionPageProgress{PageID: pageID, BlocksDone: blocksDone}); serr != nil {
			return ActionResult{State: ActionUnknown, ExternalID: pageID}, serr
		}
	}
	// Reliable read-back: the payload carrying the page id + the version
	// THIS publish produced stays the action's output evidence (the
	// receipt basis). A lost read-back cannot flip the writes to failed —
	// the effect exists; the honest state is unknown for Query to resolve.
	final, ferr := m.readPageVersion(ctx, pageID)
	if ferr != nil {
		if errors.Is(ferr, ErrNotionOutcomeUnknown) {
			return ActionResult{State: ActionUnknown, ExternalID: pageID}, ferr
		}
		return ActionResult{State: ActionUnknown, ExternalID: pageID}, ferr
	}
	raw, _ := json.Marshal(map[string]string{"object": "page", "id": final.PageID, "last_edited_time": final.LastEditedTime})
	return ActionResult{State: ActionSucceeded, ExternalID: pageID, Output: raw}, nil
}

// Query is the reconciliation entry point for an update parked in
// unknown: it reconciles ONLY via the reliable page read + children read.
// The approved blocks must be present as a contiguous run before success
// is claimed; anything less stays the honest unknown.
func (m *NotionUpdateAdapter) Query(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	snap, err := ParseNotionUpdateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	ver, gerr := m.readPageVersion(ctx, snap.PageID)
	if gerr != nil {
		return ActionResult{State: ActionUnknown}, gerr
	}
	kids, kerr := m.readChildren(ctx, snap.PageID)
	if kerr != nil {
		return ActionResult{State: ActionUnknown}, kerr
	}
	if !notionBlocksContained(kids, snap.Blocks) {
		return ActionResult{State: ActionUnknown}, fmt.Errorf("notion_query_unverifiable: approved blocks not present as a contiguous run")
	}
	raw, _ := json.Marshal(map[string]string{"object": "page", "id": ver.PageID, "last_edited_time": ver.LastEditedTime})
	return ActionResult{State: ActionSucceeded, ExternalID: snap.PageID, Output: raw}, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/appconnector/ -run 'NotionUpdate|NotionVersionConflict|ParseNotionPage' -count=1`
Expected: PASS（全部新测试 + Task 1 测试不回归）。

- [ ] **Step 5: 回归既有 create 套件并提交**

Run: `go test ./internal/modules/appconnector/ -count=1`
Expected: PASS（含既有 `TestNotionPartialSuccessDoesNotCreateAnotherPage` 等；`TestNotionRealControlledCreate` 无凭据 SKIP 属预期）。

```bash
git add internal/modules/appconnector/notion_update.go internal/modules/appconnector/notion_update_test.go
git commit -m "feat(appconnector): notion update adapter with version pre-read, conflict refusal and remote-first reconciliation (T18 #48 AC1/AC2)"
```

---

### Task 3: `app_publications` 发布回执存储与双轨迁移

**前置：Task 0 必须先完成**——当前 HEAD 迁移双占使本任务 Step 5 的迁移冒烟（及一切迁移依赖测试）不可运行；Task 0 重编号后本任务迁移落位 000193/000114。**编号顺延约定**：000193/000114 与 plan-t43/plan-t60/plan-t67 撞号（四方计划原文一致约定）——集成时若已被先合入者占用，本任务四个迁移文件整体改号为下一可用编号，DDL 内容与测试断言零变化。

**Files:**
- Create: `internal/modules/appconnector/repository/appconnector/publication.go`
- Create: `migrations/versioned/000193_app_publications.up.sql`
- Create: `migrations/versioned/000193_app_publications.down.sql`
- Create: `migrations/sqlite/000114_app_publications.up.sql`
- Create: `migrations/sqlite/000114_app_publications.down.sql`
- Test: `internal/modules/appconnector/repository/appconnector/publication_test.go`

**Interfaces:**
- Consumes: 包内既有 gorm 约定（`action.go` 的 ActionRow 同款风格；`install_test.go` 的内存 sqlite + AutoMigrate 测试约定）。
- Produces（Task 5/6/7/8 依赖）:
  - `var ErrPublicationNotFound = errors.New("publication_not_found")`、`var ErrPublicationConflict = errors.New("publication_conflict")`
  - `type PublicationRow struct`（列：tenant_id/action_id 复合 PK、connection_id、provider、mode、destination、expected_version、artifact_version_id、artifact_digest、state、external_id、external_version、receipt_json、progress_json、created_at/updated_at；`TableName() == "app_publications"`；state 词汇 `PublicationPlanned/Published/Failed/Unknown` 常量）
  - `func NewPublicationStore(db *gorm.DB) *PublicationStore`
  - `func (s *PublicationStore) CreatePublication(ctx context.Context, row PublicationRow) error`
  - `func (s *PublicationStore) FindByAction(ctx context.Context, tenantID uint64, actionID string) (PublicationRow, error)`
  - `func (s *PublicationStore) SaveProgress(ctx context.Context, tenantID uint64, actionID, progressJSON string) error`
  - `func (s *PublicationStore) SettlePublication(ctx context.Context, tenantID uint64, actionID, state, externalID, externalVersion, receiptJSON string) error`
  - `func (s *PublicationStore) LatestPublishedByDestination(ctx context.Context, tenantID uint64, connectionID, externalID string) (PublicationRow, error)`

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/appconnector/repository/appconnector/publication_test.go`：

```go
package appconnector

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openPublicationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&PublicationRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func plannedRow(actionID string) PublicationRow {
	return PublicationRow{
		TenantID: 7, ActionID: actionID, ConnectionID: "conn-notion", Provider: "notion",
		Mode: "create", Destination: "parent-1", ExpectedVersion: "2026-09-24T08:00:00.000Z",
		ArtifactVersionID: "ver-1", ArtifactDigest: "d1", State: PublicationPlanned,
	}
}

func TestPublicationCreateFindRoundTrip(t *testing.T) {
	db := openPublicationDB(t)
	store := NewPublicationStore(db)
	row := plannedRow("act-1")
	if err := store.CreatePublication(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	got, err := store.FindByAction(context.Background(), 7, "act-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "notion" || got.Mode != "create" || got.Destination != "parent-1" ||
		got.ExpectedVersion != "2026-09-24T08:00:00.000Z" || got.ArtifactVersionID != "ver-1" {
		t.Fatalf("round trip drift: %+v", got)
	}
	// Duplicate plan for the same action is a conflict, never a silent
	// overwrite of the approval binding.
	if err := store.CreatePublication(context.Background(), row); !errors.Is(err, ErrPublicationConflict) {
		t.Fatalf("duplicate create must conflict, got %v", err)
	}
	if _, err := store.FindByAction(context.Background(), 8, "act-1"); !errors.Is(err, ErrPublicationNotFound) {
		t.Fatalf("cross-tenant lookup must be not-found, got %v", err)
	}
}

func TestPublicationSettleTransitions(t *testing.T) {
	db := openPublicationDB(t)
	store := NewPublicationStore(db)
	ctx := context.Background()
	if err := store.CreatePublication(ctx, plannedRow("act-1")); err != nil {
		t.Fatal(err)
	}
	// planned -> unknown (parked for provider query) -> published.
	if err := store.SettlePublication(ctx, 7, "act-1", PublicationUnknown, "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SettlePublication(ctx, 7, "act-1", PublicationPublished, "page-9", "2026-09-24T12:00:00.000Z", `{"object":"page"}`); err != nil {
		t.Fatal(err)
	}
	got, _ := store.FindByAction(ctx, 7, "act-1")
	if got.State != PublicationPublished || got.ExternalID != "page-9" ||
		got.ExternalVersion != "2026-09-24T12:00:00.000Z" || got.ReceiptJSON == "" {
		t.Fatalf("settle must record the external version + receipt: %+v", got)
	}
	// Terminal states never move again — a late settle cannot rewrite the
	// recorded receipt.
	if err := store.SettlePublication(ctx, 7, "act-1", PublicationFailed, "", "", ""); !errors.Is(err, ErrPublicationConflict) {
		t.Fatalf("published is terminal, got %v", err)
	}
	// planned -> failed is legal.
	if err := store.CreatePublication(ctx, plannedRow("act-2")); err != nil {
		t.Fatal(err)
	}
	if err := store.SettlePublication(ctx, 7, "act-2", PublicationFailed, "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SettlePublication(ctx, 7, "act-2", PublicationPublished, "x", "y", ""); !errors.Is(err, ErrPublicationConflict) {
		t.Fatalf("failed is terminal, got %v", err)
	}
}

func TestPublicationSaveProgressIdempotent(t *testing.T) {
	db := openPublicationDB(t)
	store := NewPublicationStore(db)
	ctx := context.Background()
	if err := store.CreatePublication(ctx, plannedRow("act-1")); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveProgress(ctx, 7, "act-1", `{"page_id":"page-9","blocks_done":2}`); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveProgress(ctx, 7, "act-1", `{"page_id":"page-9","blocks_done":4}`); err != nil {
		t.Fatal(err)
	}
	got, _ := store.FindByAction(ctx, 7, "act-1")
	if got.ProgressJSON != `{"page_id":"page-9","blocks_done":4}` {
		t.Fatalf("progress must be last-write-wins durable checkpoint: %q", got.ProgressJSON)
	}
	// Progress writes never touch the state columns.
	if got.State != PublicationPlanned {
		t.Fatalf("progress must not settle: %s", got.State)
	}
}

func TestPublicationLatestPublishedByDestination(t *testing.T) {
	db := openPublicationDB(t)
	store := NewPublicationStore(db)
	ctx := context.Background()
	row := plannedRow("act-1")
	row.Mode = "update"
	row.Destination = "page-9"
	if err := store.CreatePublication(ctx, row); err != nil {
		t.Fatal(err)
	}
	if err := store.SettlePublication(ctx, 7, "act-1", PublicationPublished, "page-9", "v1", "{}"); err != nil {
		t.Fatal(err)
	}
	got, err := store.LatestPublishedByDestination(ctx, 7, "conn-notion", "page-9")
	if err != nil || got.ActionID != "act-1" {
		t.Fatalf("published receipt must be discoverable: %+v %v", got, err)
	}
	if _, err := store.LatestPublishedByDestination(ctx, 7, "conn-notion", "page-other"); !errors.Is(err, ErrPublicationNotFound) {
		t.Fatalf("unpublished destination must be not-found, got %v", err)
	}
	if _, err := store.LatestPublishedByDestination(ctx, 8, "conn-notion", "page-9"); !errors.Is(err, ErrPublicationNotFound) {
		t.Fatalf("cross-tenant destination must be not-found, got %v", err)
	}
	_ = time.Now
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/appconnector/repository/appconnector/ -run 'Publication' -count=1`
Expected: FAIL（`undefined: PublicationRow`）。

- [ ] **Step 3: 最小实现**

创建 `internal/modules/appconnector/repository/appconnector/publication.go`：

```go
package appconnector

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	// ErrPublicationNotFound: no publication row for the (tenant, action)
	// pair — indistinguishable from a foreign tenant's row by design.
	ErrPublicationNotFound = errors.New("publication_not_found")
	// ErrPublicationConflict: invalid publication lifecycle transition or
	// a duplicate plan for one action.
	ErrPublicationConflict = errors.New("publication_conflict")
)

// Publication lifecycle states. planned is written at plan formation (the
// Action Plan record binding artifact version + external destination +
// expected external version); published/failed/unknown are settled from
// the ACTION row's authoritative terminal outcome — the receipt is a
// projection of the action, never the reverse.
const (
	PublicationPlanned   = "planned"
	PublicationPublished = "published"
	PublicationFailed    = "failed"
	PublicationUnknown   = "unknown"
)

// PublicationRow is the durable 外部发布 record (CONTEXT.md): the approved
// artifact version, the external target, the target's version, and the
// operation result as a receipt. ProgressJSON carries the NO-03 multi-step
// recovery checkpoint between dispatch and settlement.
type PublicationRow struct {
	TenantID  uint64 `gorm:"primaryKey;column:tenant_id"`
	ActionID  string `gorm:"primaryKey;column:action_id"`
	CreatedAt time.Time
	UpdatedAt time.Time

	ConnectionID string `gorm:"column:connection_id;not null"`
	Provider     string `gorm:"column:provider;not null"`  // "notion" (#48); feishu #49, confluence #50
	Mode         string `gorm:"column:mode;not null"`      // create | update
	Destination  string `gorm:"column:destination;not null"` // parent page id (create) / page id (update)
	// ExpectedVersion is the external current version READ BEFORE the plan
	// was formed (AC1 baseline; the update snapshot's expected_version).
	ExpectedVersion string `gorm:"column:expected_version;not null;default:''"`
	// The immutable internal artifact version this publish carries.
	ArtifactVersionID string `gorm:"column:artifact_version_id;not null;default:''"`
	ArtifactDigest    string `gorm:"column:artifact_digest;not null;default:''"`

	State          string `gorm:"column:state;not null"`
	ExternalID     string `gorm:"column:external_id;not null;default:''"`
	ExternalVersion string `gorm:"column:external_version;not null;default:''"`
	ReceiptJSON    string `gorm:"column:receipt_json;not null;default:''"`
	ProgressJSON   string `gorm:"column:progress_json;not null;default:''"`
}

func (PublicationRow) TableName() string { return "app_publications" }

// PublicationStore persists the publish plan records and their receipts.
type PublicationStore struct{ db *gorm.DB }

// NewPublicationStore builds a PublicationStore over a gorm DB.
func NewPublicationStore(db *gorm.DB) *PublicationStore { return &PublicationStore{db: db} }

// CreatePublication inserts one planned publication; a second plan for the
// same action is a conflict (the approval binding is never silently
// rewritten). The existence pre-check and the INSERT run in one
// transaction — gorm's driver-level duplicate-key translation is not
// enabled in this codebase, so the guarded insert is the portable form.
func (s *PublicationStore) CreatePublication(ctx context.Context, row PublicationRow) error {
	if row.TenantID == 0 || row.ActionID == "" || row.Provider == "" || row.Mode == "" || row.Destination == "" || row.State != PublicationPlanned {
		return ErrPublicationConflict
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing PublicationRow
		err := tx.Where("tenant_id = ? AND action_id = ?", row.TenantID, row.ActionID).First(&existing).Error
		if err == nil {
			return ErrPublicationConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(&row).Error
	})
}

// FindByAction loads the publication row scoped to the tenant.
func (s *PublicationStore) FindByAction(ctx context.Context, tenantID uint64, actionID string) (PublicationRow, error) {
	var row PublicationRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND action_id = ?", tenantID, actionID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PublicationRow{}, ErrPublicationNotFound
	}
	return row, err
}

// SaveProgress durably records the NO-03 checkpoint without touching the
// lifecycle columns.
func (s *PublicationStore) SaveProgress(ctx context.Context, tenantID uint64, actionID, progressJSON string) error {
	res := s.db.WithContext(ctx).Model(&PublicationRow{}).
		Where("tenant_id = ? AND action_id = ?", tenantID, actionID).
		Update("progress_json", progressJSON)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPublicationNotFound
	}
	return nil
}

func publicationTransitionAllowed(from, to string) bool {
	switch from {
	case PublicationPlanned:
		return to == PublicationPublished || to == PublicationFailed || to == PublicationUnknown
	case PublicationUnknown:
		return to == PublicationPublished || to == PublicationFailed
	default:
		return false
	}
}

// SettlePublication moves the receipt to a state derived from the ACTION
// row's authoritative outcome. Terminal states never move again; the
// published settle records the external id + the version the publish
// produced + the raw provider receipt payload.
func (s *PublicationStore) SettlePublication(ctx context.Context, tenantID uint64, actionID, state, externalID, externalVersion, receiptJSON string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row PublicationRow
		if err := tx.Where("tenant_id = ? AND action_id = ?", tenantID, actionID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPublicationNotFound
			}
			return err
		}
		if !publicationTransitionAllowed(row.State, state) {
			return ErrPublicationConflict
		}
		return tx.Model(&PublicationRow{}).
			Where("tenant_id = ? AND action_id = ?", tenantID, actionID).
			Updates(map[string]interface{}{
				"state": state, "external_id": externalID,
				"external_version": externalVersion, "receipt_json": receiptJSON,
			}).Error
	})
}

// LatestPublishedByDestination returns the most recent published receipt
// for one external destination — the authority rule for update plans: a
// page this tenant+connection published before is ours to update; anything
// else must fail closed.
func (s *PublicationStore) LatestPublishedByDestination(ctx context.Context, tenantID uint64, connectionID, externalID string) (PublicationRow, error) {
	var row PublicationRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND connection_id = ? AND external_id = ? AND state = ?", tenantID, connectionID, externalID, PublicationPublished).
		Order("updated_at DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PublicationRow{}, ErrPublicationNotFound
	}
	return row, err
}
```

- [ ] **Step 4: 写迁移文件**

`migrations/versioned/000193_app_publications.up.sql`：

```sql
-- External publication receipts (T18, #48): the durable record of an
-- Action Plan that publishes a confirmed artifact version to an external
-- office system. planned rows are written at plan formation; terminal
-- states are settled from the authoritative app_actions outcome. The NO-03
-- multi-step progress checkpoint rides progress_json. Provider column
-- starts with notion; feishu (#49) and confluence (#50) join later.
CREATE TABLE app_publications (
    tenant_id INTEGER NOT NULL,
    action_id VARCHAR(64) NOT NULL,
    connection_id VARCHAR(64) NOT NULL,
    provider VARCHAR(32) NOT NULL,
    mode VARCHAR(16) NOT NULL,
    destination VARCHAR(128) NOT NULL,
    expected_version VARCHAR(64) NOT NULL DEFAULT '',
    artifact_version_id VARCHAR(64) NOT NULL DEFAULT '',
    artifact_digest VARCHAR(64) NOT NULL DEFAULT '',
    state VARCHAR(16) NOT NULL,
    external_id VARCHAR(128) NOT NULL DEFAULT '',
    external_version VARCHAR(64) NOT NULL DEFAULT '',
    receipt_json TEXT NOT NULL DEFAULT '',
    progress_json TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, action_id)
);

CREATE INDEX idx_app_publications_destination ON app_publications (tenant_id, connection_id, external_id, state);
```

`migrations/versioned/000193_app_publications.down.sql`：

```sql
DROP INDEX IF EXISTS idx_app_publications_destination;
DROP TABLE IF EXISTS app_publications;
```

`migrations/sqlite/000114_app_publications.up.sql`：

```sql
-- External publication receipts (T18, #48) — sqlite track. Same shape as
-- the versioned migration.
CREATE TABLE app_publications (
    tenant_id INTEGER NOT NULL,
    action_id TEXT NOT NULL,
    connection_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    mode TEXT NOT NULL,
    destination TEXT NOT NULL,
    expected_version TEXT NOT NULL DEFAULT '',
    artifact_version_id TEXT NOT NULL DEFAULT '',
    artifact_digest TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL,
    external_id TEXT NOT NULL DEFAULT '',
    external_version TEXT NOT NULL DEFAULT '',
    receipt_json TEXT NOT NULL DEFAULT '',
    progress_json TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, action_id)
);

CREATE INDEX idx_app_publications_destination ON app_publications (tenant_id, connection_id, external_id, state);
```

`migrations/sqlite/000114_app_publications.down.sql`：

```sql
DROP INDEX IF EXISTS idx_app_publications_destination;
DROP TABLE IF EXISTS app_publications;
```

- [ ] **Step 5: 运行测试确认通过 + 迁移全量冒烟**

Run: `go test ./internal/modules/appconnector/repository/appconnector/ -count=1`
Expected: PASS。

Run: `go test ./internal/application/repository/ -run 'TestWorkbenchNotificationsTableExistsAfterMigrations|TestTaskCollaborationEndToEnd' -count=1`
Expected: PASS（生产迁移链全量执行无冲突——前置 Task 0 已解除双占，adoptions 落位 versioned 000192 / sqlite 000113，本任务占用其后的 000193/000114，均未被他者占用）。

- [ ] **Step 6: 提交**

```bash
git add internal/modules/appconnector/repository/appconnector/publication.go internal/modules/appconnector/repository/appconnector/publication_test.go migrations/versioned/000193_app_publications.up.sql migrations/versioned/000193_app_publications.down.sql migrations/sqlite/000114_app_publications.up.sql migrations/sqlite/000114_app_publications.down.sql
git commit -m "feat(appconnector): app_publications receipt store + migrations 000193/000114 (T18 #48)"
```

---

### Task 4: publish 包——Artifact 文本到 Notion 段落块的纯投影

**Files:**
- Create: `internal/modules/appconnector/publish/blocks.go`
- Test: `internal/modules/appconnector/publish/blocks_test.go`

**Interfaces:**
- Consumes: 无（纯函数）。
- Produces（Task 6 依赖）:
  - `var ErrPublishContentTooLarge = errors.New("publish_content_too_large")`、`var ErrPublishEmptyContent = errors.New("publish_empty_content")`、`var ErrPublishUnsupportedArtifact = errors.New("publish_unsupported_artifact")`（此文件仅前两个；`ErrPublishUnsupportedArtifact` 在 `plan.go` 定义，见 Task 6）
  - `const MaxPublishBlocks = 500`
  - `func NotionParagraphBlocks(text string) ([]json.RawMessage, error)`

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/appconnector/publish/blocks_test.go`：

```go
package publish

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNotionParagraphBlocksSplitsParagraphs(t *testing.T) {
	blocks, err := NotionParagraphBlocks("First paragraph.\n\nSecond paragraph.\n\n\nThird.")
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 3 {
		t.Fatalf("want 3 paragraph blocks, got %d", len(blocks))
	}
	var first struct {
		Object string `json:"object"`
		Type   string `json:"type"`
		Paragraph struct {
			RichText []struct {
				Type string `json:"type"`
				Text struct {
					Content string `json:"content"`
				} `json:"text"`
			} `json:"rich_text"`
		} `json:"paragraph"`
	}
	if err := json.Unmarshal(blocks[0], &first); err != nil {
		t.Fatal(err)
	}
	if first.Object != "block" || first.Type != "paragraph" || len(first.Paragraph.RichText) != 1 ||
		first.Paragraph.RichText[0].Type != "text" || first.Paragraph.RichText[0].Text.Content != "First paragraph." {
		t.Fatalf("block shape drift: %s", blocks[0])
	}
}

func TestNotionParagraphBlocksNormalizesLineEndingsAndWhitespace(t *testing.T) {
	blocks, err := NotionParagraphBlocks("  a  \r\n\r\n \r\n\r\nb  \n")
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 {
		t.Fatalf("whitespace-only paragraphs must be dropped, got %d", len(blocks))
	}
	var probe struct {
		Paragraph struct {
			RichText []struct {
				Text struct {
					Content string `json:"content"`
				} `json:"text"`
			} `json:"rich_text"`
		} `json:"paragraph"`
	}
	_ = json.Unmarshal(blocks[0], &probe)
	if probe.Paragraph.RichText[0].Text.Content != "a" {
		t.Fatalf("paragraph must be trimmed: %q", probe.Paragraph.RichText[0].Text.Content)
	}
}

func TestNotionParagraphBlocksChunksLongParagraphs(t *testing.T) {
	// One paragraph of 3x the 1900-rune chunk limit: ONE block carrying a
	// rich_text array of 3 text objects (Notion caps each text object; a
	// block may carry several).
	long := strings.Repeat("字", 1900*3)
	blocks, err := NotionParagraphBlocks(long)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 {
		t.Fatalf("long paragraph stays one block, got %d", len(blocks))
	}
	var probe struct {
		Paragraph struct {
			RichText []json.RawMessage `json:"rich_text"`
		} `json:"paragraph"`
	}
	_ = json.Unmarshal(blocks[0], &probe)
	if len(probe.Paragraph.RichText) != 3 {
		t.Fatalf("want 3 rich_text chunks, got %d", len(probe.Paragraph.RichText))
	}
}

func TestNotionParagraphBlocksRejectsEmpty(t *testing.T) {
	if _, err := NotionParagraphBlocks("   \n\n  \n"); !errors.Is(err, ErrPublishEmptyContent) {
		t.Fatalf("empty content must be refused, got %v", err)
	}
}

func TestNotionParagraphBlocksRejectsTooManyBlocks(t *testing.T) {
	paragraphs := make([]string, MaxPublishBlocks+1)
	for i := range paragraphs {
		paragraphs[i] = "p"
	}
	if _, err := NotionParagraphBlocks(strings.Join(paragraphs, "\n\n")); !errors.Is(err, ErrPublishContentTooLarge) {
		t.Fatalf("block cap must be enforced, got %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/appconnector/publish/ -count=1`
Expected: FAIL（包不存在/`undefined: NotionParagraphBlocks`）。

- [ ] **Step 3: 最小实现**

创建 `internal/modules/appconnector/publish/blocks.go`：

```go
// Package publish owns the external publication seam (T18 #48): forming
// an approved Notion publish plan from a confirmed artifact version,
// bridging the A03 action pipeline to the Notion adapter family, and
// settling the durable receipt. Downstream providers (#49 feishu, #50
// confluence) join this seam; #51 extends the single-action plan record.
package publish

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Publish content bounds. MaxPublishBlocks bounds the derived block count
// (Notion append batches carry ≤100 blocks per request; the adapter batches
// beyond that — the cap bounds plan size, not wire size).
const (
	MaxPublishBlocks = 500
	// notionRichTextChunk is below Notion's documented 2000-character cap
	// per rich text text object.
	notionRichTextChunk = 1900
)

var (
	// ErrPublishContentTooLarge: the derived plan exceeds the publish
	// bounds — refused at plan formation, never truncated silently.
	ErrPublishContentTooLarge = errors.New("publish_content_too_large")
	// ErrPublishEmptyContent: the artifact carries no publishable text.
	ErrPublishEmptyContent = errors.New("publish_empty_content")
)

type notionRichTextItem struct {
	Type string `json:"type"`
	Text struct {
		Content string `json:"content"`
	} `json:"text"`
}

type notionParagraphBlock struct {
	Object    string `json:"object"`
	Type      string `json:"type"`
	Paragraph struct {
		RichText []notionRichTextItem `json:"rich_text"`
	} `json:"paragraph"`
}

// NotionParagraphBlocks derives Notion paragraph blocks from plain text:
// paragraphs split on blank lines, each trimmed, long paragraphs chunked
// into ≤1900-rune rich_text text objects (≤100 per block). The output is
// deterministic pure derivation — the same artifact bytes always produce
// the same blocks, so the approval digest pins exactly what will be sent.
func NotionParagraphBlocks(text string) ([]json.RawMessage, error) {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	parts := strings.Split(normalized, "\n\n")
	paragraphs := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			paragraphs = append(paragraphs, trimmed)
		}
	}
	if len(paragraphs) == 0 {
		return nil, ErrPublishEmptyContent
	}
	if len(paragraphs) > MaxPublishBlocks {
		return nil, fmt.Errorf("%w: %d paragraphs exceed %d blocks", ErrPublishContentTooLarge, len(paragraphs), MaxPublishBlocks)
	}
	out := make([]json.RawMessage, 0, len(paragraphs))
	for _, p := range paragraphs {
		runes := []rune(p)
		chunks := make([]string, 0, len(runes)/notionRichTextChunk+1)
		for start := 0; start < len(runes); start += notionRichTextChunk {
			end := start + notionRichTextChunk
			if end > len(runes) {
				end = len(runes)
			}
			chunks = append(chunks, string(runes[start:end]))
		}
		if len(chunks) > 100 {
			return nil, fmt.Errorf("%w: one paragraph needs %d rich_text objects", ErrPublishContentTooLarge, len(chunks))
		}
		block := notionParagraphBlock{Object: "block", Type: "paragraph"}
		for _, c := range chunks {
			item := notionRichTextItem{Type: "text"}
			item.Text.Content = c
			block.Paragraph.RichText = append(block.Paragraph.RichText, item)
		}
		raw, err := json.Marshal(block)
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}
```

- [ ] **Step 4: 运行测试确认通过并提交**

Run: `go test ./internal/modules/appconnector/publish/ -count=1`
Expected: PASS。

```bash
git add internal/modules/appconnector/publish/blocks.go internal/modules/appconnector/publish/blocks_test.go
git commit -m "feat(publish): deterministic artifact text to notion paragraph blocks projection (T18 #48)"
```

---

### Task 5: publish 包——`NotionBridge`（Dispatcher + UnknownResolver + 作用域/凭据/策略端口）

**Files:**
- Create: `internal/modules/appconnector/publish/dispatcher.go`
- Test: `internal/modules/appconnector/publish/dispatcher_test.go`

**Interfaces:**
- Consumes: `appconnectorsvc.ActionDispatcher`/`UnknownResolver`/`ActionSnapshot`/`DispatchOutcome`/`ErrDispatchNotStarted`（`service/appconnector/action.go:110-126`、`:49`）；`appconn.NotionCreateAdapter`/`NotionUpdateAdapter`/`IsNotionUpdateArgs`/`ParseNotionPageReceipt`/`HTTPPolicy`/`Action`/`ActionSucceeded/Failed/Unknown`/`NotionAPIHost`；`repoappconn.PublicationRow`/`PublicationStore`（Task 3）。
- Produces（Task 6/7/8 依赖，签名逐字）:
  - `type NotionConnectionScope struct { AppID string; ConnectionKind string; OwnerID string; AuthVersion int64; ApprovedParents []string; InsertCapability bool }`
  - `type NotionScopeSource interface { NotionScope(ctx context.Context, connectionID string) (NotionConnectionScope, error) }` + `func NewDBNotionScopeSource(db *gorm.DB) NotionScopeSource`（连接→installation→app_versions.schema_json 的 `{"scopes":[...],"approved_parents":[...]}` 解析，全参数绑定）
  - `type NotionPolicyProvider interface { PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error) }` + `func NewConstantNotionPolicyProvider() NotionPolicyProvider`（https api.notion.com /v1/、GET/POST/PATCH、30s——NO-01 复审常量）
  - `type NotionTokenSource interface { Token(ctx context.Context, connectionID string, expectedVersion int64) (string, error) }` + `func NewCredentialTokenSource(resolver appconnectorsvc.CredentialResolver) NotionTokenSource`
  - `type PublicationSource interface { FindByAction(ctx context.Context, tenantID uint64, actionID string) (repoappconn.PublicationRow, error); SaveProgress(ctx context.Context, tenantID uint64, actionID, progressJSON string) error }`
  - `type NotionBridge struct` + `func NewNotionBridge(scopes NotionScopeSource, policies NotionPolicyProvider, tokens NotionTokenSource, pubs PublicationSource) *NotionBridge`
  - `func (b *NotionBridge) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error)`
  - `func (b *NotionBridge) QueryProvider(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error)`
  - `func (b *NotionBridge) ReadPageVersion(ctx context.Context, connectionID, pageID string) (string, error)`（Task 6 计划形成的 AC1 预读；经 scope 的 AuthVersion 取 token）
  - `const PublishVersionConflictResult = "notion_version_conflict"`（bridge 把 `ErrNotionVersionConflict` 归一为该前缀的 ProviderResult，服务层据此映射 409）

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/appconnector/publish/dispatcher_test.go`：

```go
package publish

import (
	"context"
	"encoding/json"
	"net"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---- in-memory ports (module-level scenario doubles for the seams the
// production wiring supplies from the database / credential store) ----

type fakeScopes struct {
	scope  NotionConnectionScope
	err    error
	calls  int
}

func (f *fakeScopes) NotionScope(ctx context.Context, connectionID string) (NotionConnectionScope, error) {
	f.calls++
	return f.scope, f.err
}

type fakePolicies struct{ pol appconn.HTTPPolicy }

func (f *fakePolicies) PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error) {
	return f.pol, nil
}

type fakeTokens struct{ tok string }

func (f *fakeTokens) Token(ctx context.Context, connectionID string, expectedVersion int64) (string, error) {
	return f.tok, nil
}

func openBridgeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repoappconn.PublicationRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func bridgeTestStack(t *testing.T, fake *bridgeFakeNotion) (*NotionBridge, *repoappconn.PublicationStore) {
	t.Helper()
	db := openBridgeDB(t)
	pubs := repoappconn.NewPublicationStore(db)
	srv := fake.server(t)
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	pol := appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PATCH"}, PathPrefix: "/v1/",
		AuthorizedNetworks: []*net.IPNet{network}, Timeout: 10 * time.Second,
	}
	bridge := NewNotionBridge(
		&fakeScopes{scope: NotionConnectionScope{
			AppID: "notion", ConnectionKind: "personal", OwnerID: "u1", AuthVersion: 1,
			ApprovedParents: []string{"parent-1"}, InsertCapability: true,
		}},
		&fakePolicies{pol: pol},
		&fakeTokens{tok: "secret_test_token"},
		pubs,
	)
	return bridge, pubs
}

func createSnapshotArgs(parent, title string) appconnectorsvc.ActionSnapshot {
	blocks, _ := json.Marshal([]any{map[string]any{
		"object": "block", "type": "paragraph",
		"paragraph": map[string]any{"rich_text": []any{map[string]any{
			"type": "text", "text": map[string]string{"content": "hello"},
		}}},
	}})
	args, _ := json.Marshal(map[string]any{"parent": parent, "title": title, "blocks": []any{json.RawMessage(blocks)}})
	return appconnectorsvc.ActionSnapshot{
		ID: "act-b1", TenantID: 7, ActorID: "u1", ConnectionID: "conn-notion",
		Version: "notion/v1", Target: parent, Risk: "write",
		AuthVersion: 1, Args: args,
	}
}

func TestBridgeDispatchCreateRoutesToCreateAdapter(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	bridge, pubs := bridgeTestStack(t, fake)
	if err := pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-b1", ConnectionID: "conn-notion", Provider: "notion",
		Mode: "create", Destination: "parent-1", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	out, err := bridge.Dispatch(context.Background(), createSnapshotArgs("parent-1", "T"), "conn-notion")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != appconn.ActionSucceeded || out.ExecutionID == "" {
		t.Fatalf("create dispatch outcome: %+v", out)
	}
	rcpt, rerr := appconn.ParseNotionPageReceipt([]byte(out.ProviderResult))
	if rerr != nil || rcpt.ExternalID != out.ExecutionID || rcpt.ExternalVersion == "" {
		t.Fatalf("ProviderResult must be the receipt-bearing page payload: %+v %v", rcpt, rerr)
	}
	row, perr := pubs.FindByAction(context.Background(), 7, "act-b1")
	if perr != nil || row.ProgressJSON == "" {
		t.Fatalf("progress must be durably recorded on the publication row: %+v %v", row, perr)
	}
}

func TestBridgeDispatchUpdateConflictIsDefinitiveFailure(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("page-9", "parent-1", "old")
	fake.touch("page-9", "2026-09-24T10:30:00.000Z") // external drift
	bridge, pubs := bridgeTestStack(t, fake)
	if err := pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-b2", ConnectionID: "conn-notion", Provider: "notion",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	blocks, _ := json.Marshal([]any{map[string]any{
		"object": "block", "type": "paragraph",
		"paragraph": map[string]any{"rich_text": []any{map[string]any{
			"type": "text", "text": map[string]string{"content": "hello"},
		}}},
	}})
	args, _ := json.Marshal(map[string]any{
		"page_id": "page-9", "expected_version": "2026-09-24T08:00:00.000Z",
		"title": "T", "blocks": []any{json.RawMessage(blocks)},
	})
	snap := appconnectorsvc.ActionSnapshot{ID: "act-b2", TenantID: 7, ActorID: "u1",
		ConnectionID: "conn-notion", Version: "notion/v1", Target: "page-9", Risk: "write",
		AuthVersion: 1, Args: args}
	out, err := bridge.Dispatch(context.Background(), snap, "conn-notion")
	if err != nil {
		t.Fatalf("a definitive provider refusal must be an outcome, not an error: %v", err)
	}
	if out.Status != appconn.ActionFailed || !strings.HasPrefix(out.ProviderResult, PublishVersionConflictResult) {
		t.Fatalf("conflict must map to a failed outcome with the conflict marker: %+v", out)
	}
	if fake.patchCalls != 0 {
		t.Fatalf("conflict must write nothing, got %d patches", fake.patchCalls)
	}
}

func TestBridgeDispatchUnknownOutcomeParks(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("page-9", "parent-1", "old")
	fake.mu.Lock()
	fake.dropNextAppend = true
	fake.mu.Unlock()
	bridge, pubs := bridgeTestStack(t, fake)
	if err := pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-b3", ConnectionID: "conn-notion", Provider: "notion",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	blocks, _ := json.Marshal([]any{map[string]any{
		"object": "block", "type": "paragraph",
		"paragraph": map[string]any{"rich_text": []any{map[string]any{
			"type": "text", "text": map[string]string{"content": "hello"},
		}}},
	}})
	args, _ := json.Marshal(map[string]any{
		"page_id": "page-9", "expected_version": "2026-09-24T08:00:00.000Z",
		"title": "T", "blocks": []any{json.RawMessage(blocks)},
	})
	snap := appconnectorsvc.ActionSnapshot{ID: "act-b3", TenantID: 7, ActorID: "u1",
		ConnectionID: "conn-notion", Version: "notion/v1", Target: "page-9", Risk: "write",
		AuthVersion: 1, Args: args}
	out, err := bridge.Dispatch(context.Background(), snap, "conn-notion")
	if err != nil || out.Status != appconn.ActionUnknown {
		t.Fatalf("unobservable outcome must park unknown: %+v %v", out, err)
	}
	// AC2: reconcile by remote query FIRST — the effect applied, so the
	// query confirms; no re-dispatch ever happens here.
	q, qerr := bridge.QueryProvider(context.Background(), snap, "conn-notion")
	if qerr != nil || q.Status != appconn.ActionSucceeded {
		t.Fatalf("query must resolve from the remote state: %+v %v", q, qerr)
	}
	if fake.appendCalls != 1 {
		t.Fatalf("no re-dispatch: appends=%d", fake.appendCalls)
	}
}

func TestBridgeDispatchNonNotionConnectionFailsClosedPreSend(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	bridge, _ := bridgeTestStackWithScopes(t, fake, &fakeScopes{scope: NotionConnectionScope{AppID: "feishu"}})
	out, err := bridge.Dispatch(context.Background(), createSnapshotArgs("parent-1", "T"), "conn-feishu")
	if err == nil || out.Status != "" {
		t.Fatalf("non-notion connection must be a pre-send refusal, got %+v %v", out, err)
	}
	if !strings.Contains(err.Error(), "not a notion connection") {
		t.Fatalf("refusal must name the wiring gap: %v", err)
	}
	if fake.creates != 0 {
		t.Fatalf("nothing may be sent: creates=%d", fake.creates)
	}
}

func TestBridgeReadPageVersion(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("page-9", "parent-1", "old")
	bridge, _ := bridgeTestStack(t, fake)
	v, err := bridge.ReadPageVersion(context.Background(), "conn-notion", "page-9")
	if err != nil || v != "2026-09-24T08:00:00.000Z" {
		t.Fatalf("version pre-read: %q %v", v, err)
	}
}

// bridgeFakeNotion: the same contract double as Task 2's fakeNotionDocs,
// duplicated here so the publish package's tests stay self-contained.
type bridgeFakeNotion struct {
	mu       sync.Mutex
	token    string
	nextID   int
	creates  int
	patchCalls int
	appendCalls int
	pages    map[string]*bridgeFakePage
	dropNextAppend bool
}

type bridgeFakePage struct {
	id, parent, title, lastEdited string
	children                      []json.RawMessage
}

func newBridgeFakeNotion(token string) *bridgeFakeNotion {
	return &bridgeFakeNotion{token: token, pages: map[string]*bridgeFakePage{}}
}

func (f *bridgeFakeNotion) addPage(id, parent, title string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pages[id] = &bridgeFakePage{id: id, parent: parent, title: title, lastEdited: "2026-09-24T08:00:00.000Z"}
}

func (f *bridgeFakeNotion) touch(id, when string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.pages[id]; ok {
		p.lastEdited = when
	}
}

func (f *bridgeFakeNotion) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
	auth := func(r *http.Request) bool { return r.Header.Get("Authorization") == "Bearer "+f.token }
	mux.HandleFunc("/v1/pages", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Parent struct {
				PageID string `json:"page_id"`
			} `json:"parent"`
			Properties struct {
				Title struct {
					Title []struct {
						Text struct {
							Content string `json:"content"`
						} `json:"text"`
					} `json:"title"`
				} `json:"title"`
			} `json:"properties"`
		}
		_ = json.Unmarshal(body, &req)
		f.mu.Lock()
		f.creates++
		f.nextID++
		id := fmt.Sprintf("page-%d", f.nextID)
		f.pages[id] = &bridgeFakePage{id: id, parent: req.Parent.PageID, title: req.Properties.Title.Title[0].Text.Content, lastEdited: "2026-09-24T09:00:00.000Z"}
		f.mu.Unlock()
		writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":"2026-09-24T09:00:00.000Z","parent":{"type":"page_id","page_id":%q}}`, id, req.Parent.PageID))
	})
	mux.HandleFunc("/v1/pages/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		id := r.URL.Path[len("/v1/pages/"):]
		f.mu.Lock()
		p, ok := f.pages[id]
		f.mu.Unlock()
		if !ok {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		switch r.Method {
		case http.MethodGet:
			f.mu.Lock()
			le := p.lastEdited
			f.mu.Unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":%q,"parent":{"type":"page_id","page_id":%q}}`, p.id, le, p.parent))
		case http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Properties struct {
					Title struct {
						Title []struct {
							Text struct {
								Content string `json:"content"`
							} `json:"text"`
						} `json:"title"`
					} `json:"title"`
				} `json:"properties"`
			}
			_ = json.Unmarshal(body, &req)
			f.mu.Lock()
			f.patchCalls++
			p.title = req.Properties.Title.Title[0].Text.Content
			p.lastEdited = "2026-09-24T11:00:00.000Z"
			f.mu.Unlock()
			f.mu.Lock()
			le := p.lastEdited
			f.mu.Unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":%q,"parent":{"type":"page_id","page_id":%q}}`, p.id, le, p.parent))
		default:
			writeJSON(w, 405, `{"object":"error","status":405,"code":"method_not_allowed","message":"x"}`)
		}
	})
	mux.HandleFunc("/v1/blocks/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		rest := r.URL.Path[len("/v1/blocks/"):]
		var id string
		if suffix := "/children"; strings.HasSuffix(rest, suffix) {
			id = strings.TrimSuffix(rest, suffix)
		} else {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		f.mu.Lock()
		p, ok := f.pages[id]
		f.mu.Unlock()
		if !ok {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		switch r.Method {
		case http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Children []json.RawMessage `json:"children"`
			}
			_ = json.Unmarshal(body, &req)
			f.mu.Lock()
			f.appendCalls++
			p.children = append(p.children, req.Children...)
			p.lastEdited = "2026-09-24T12:00:00.000Z"
			drop := f.dropNextAppend
			if drop {
				f.dropNextAppend = false
			}
			results := append([]json.RawMessage(nil), p.children...)
			f.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, fmt.Sprintf(`{"object":"list","results":[%s]}`, joinBridgeRaw(results)))
		case http.MethodGet:
			f.mu.Lock()
			results := append([]json.RawMessage(nil), p.children...)
			f.mu.Unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"list","results":[%s]}`, joinBridgeRaw(results)))
		default:
			writeJSON(w, 405, `{"object":"error","status":405,"code":"method_not_allowed","message":"x"}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func joinBridgeRaw(items []json.RawMessage) string {
	out := ""
	for i, it := range items {
		if i > 0 {
			out += ","
		}
		out += string(it)
	}
	return out
}

// bridgeTestStackWithScopes lets a test inject its own scope source.
func bridgeTestStackWithScopes(t *testing.T, fake *bridgeFakeNotion, scopes NotionScopeSource) (*NotionBridge, *repoappconn.PublicationStore) {
	t.Helper()
	db := openBridgeDB(t)
	pubs := repoappconn.NewPublicationStore(db)
	srv := fake.server(t)
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	pol := appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PATCH"}, PathPrefix: "/v1/",
		AuthorizedNetworks: []*net.IPNet{network}, Timeout: 10 * time.Second,
	}
	return NewNotionBridge(scopes, &fakePolicies{pol: pol}, &fakeTokens{tok: "secret_test_token"}, pubs), pubs
}
```

（该测试文件 import 还需 `"fmt"`、`"io"`、`"net/http"`——`bridgeFakeNotion.server` 使用。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/appconnector/publish/ -count=1`
Expected: FAIL（`undefined: NewNotionBridge`）。

- [ ] **Step 3: 最小实现**

创建 `internal/modules/appconnector/publish/dispatcher.go`：

```go
package publish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"

	"gorm.io/gorm"
)

// PublishVersionConflictResult is the ProviderResult prefix the bridge
// records for a definitive version-conflict refusal; the service layer
// maps it onto the 409 PUBLISH_VERSION_CONFLICT response.
const PublishVersionConflictResult = "notion_version_conflict"

// NotionConnectionScope is everything the publish seam needs to know
// about one connection before any Notion call: the installation's app
// identity (routing), the reviewed destination scope (approved parent
// pages), the reviewed insert-content capability, and the connection's
// auth generation (strict credential binding).
type NotionConnectionScope struct {
	AppID            string
	ConnectionKind   string
	OwnerID          string
	AuthVersion      int64
	ApprovedParents  []string
	InsertCapability bool
}

// NotionScopeSource resolves the scope of one connection from the
// authoritative rows. The production implementation is
// NewDBNotionScopeSource (connections → installations → app_versions).
type NotionScopeSource interface {
	NotionScope(ctx context.Context, connectionID string) (NotionConnectionScope, error)
}

// NotionPolicyProvider returns the reviewed outbound HTTP policy for one
// connection's Notion calls (A04). Production uses the pinned Notion
// contract; tests inject the loopback-authorized policy.
type NotionPolicyProvider interface {
	PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error)
}

// NotionTokenSource returns the decrypted Notion token for exactly one
// connection at one auth generation — the A02 credential resolution AFTER
// the permission guard has passed.
type NotionTokenSource interface {
	Token(ctx context.Context, connectionID string, expectedVersion int64) (string, error)
}

// PublicationSource is the receipt-store subset the bridge needs (the
// NO-03 progress checkpoint rides the publication row).
type PublicationSource interface {
	FindByAction(ctx context.Context, tenantID uint64, actionID string) (repoappconn.PublicationRow, error)
	SaveProgress(ctx context.Context, tenantID uint64, actionID, progressJSON string) error
}

// NotionBridge adapts the Notion adapter family to the A03 pipeline's
// dispatch boundary. It routes by the approved snapshot's shape
// (page_id+expected_version → update; parent → create) and maps adapter
// outcomes onto DispatchOutcome:
//
//   - adapter FAILED (provider-refused, incl. version conflict) → a
//     definitive failed outcome, nil error — the service settles failed;
//   - adapter UNKNOWN (transport failure / 5xx / unprovable) → an unknown
//     outcome, nil error — the action parks for a provider query;
//   - wiring gaps (scope/token/policy unavailable, non-notion connection)
//     → an ErrDispatchNotStarted error: provably pre-send, nothing left
//     the process.
type NotionBridge struct {
	scopes   NotionScopeSource
	policies NotionPolicyProvider
	tokens   NotionTokenSource
	pubs     PublicationSource
}

// NewNotionBridge builds the bridge over its four ports.
func NewNotionBridge(scopes NotionScopeSource, policies NotionPolicyProvider, tokens NotionTokenSource, pubs PublicationSource) *NotionBridge {
	return &NotionBridge{scopes: scopes, policies: policies, tokens: tokens, pubs: pubs}
}

var _ appconnectorsvc.ActionDispatcher = (*NotionBridge)(nil)
var _ appconnectorsvc.UnknownResolver = (*NotionBridge)(nil)

// appIDNotion is the installation app identity the first write adapter
// family serves (appOAuthDefaults key, internal/handler/app_connector_oauth.go).
const appIDNotion = "notion"

// adapterFor builds the adapter instance for one dispatch/query. Token and
// policy are resolved fresh per call so a revoked connection (auth version
// bump) can never be reached with a stale secret.
func (b *NotionBridge) adapterFor(ctx context.Context, snap appconnectorsvc.ActionSnapshot) (appconn.Adapter, error) {
	scope, err := b.scopes.NotionScope(ctx, snap.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("%w: scope for connection %q: %v", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	if scope.AppID != appIDNotion {
		return nil, fmt.Errorf("%w: connection %q is %q, not a notion connection", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, scope.AppID)
	}
	pol, err := b.policies.PolicyFor(ctx, snap.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("%w: policy for connection %q: %v", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	tok, err := b.tokens.Token(ctx, snap.ConnectionID, snap.AuthVersion)
	if err != nil {
		return nil, fmt.Errorf("%w: token for connection %q: %v", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	caps := func(ctx context.Context, a appconn.Action) ([]string, error) {
		if !scope.InsertCapability {
			return nil, fmt.Errorf("connection lacks %s", appconn.NotionCapabilityInsert)
		}
		return []string{appconn.NotionCapabilityInsert}, nil
	}
	progress := newPublicationProgress(b.pubs, snap.TenantID, snap.ID)
	action := appconn.Action{
		ID: snap.ID, TenantID: snap.TenantID, ActorID: snap.ActorID,
		ConnectionID: snap.ConnectionID, Version: snap.Version,
		Target: snap.Target, Risk: snap.Risk, AuthVersion: snap.AuthVersion,
		Args: json.RawMessage(snap.Args),
	}
	if appconn.IsNotionUpdateArgs(action.Args) {
		up := &appconn.NotionUpdateAdapter{
			Policy: pol,
			Token:  func(ctx context.Context) (string, error) { return tok, nil },
			ConnectionCapabilities: caps,
			LoadProgress:           progress.load,
			SaveProgress:           progress.save,
		}
		return up, nil
	}
	cr := &appconn.NotionCreateAdapter{
		Policy:           pol,
		Token:            func(ctx context.Context) (string, error) { return tok, nil },
		ApprovedParents:  scope.ApprovedParents,
		ConnectionCapabilities: caps,
		LoadProgress:     progress.load,
		SaveProgress:     progress.save,
	}
	return cr, nil
}

// publicationProgress adapts the publication row's progress_json onto the
// adapters' LoadProgress/SaveProgress hooks.
type publicationProgress struct {
	pubs     PublicationSource
	tenantID uint64
	actionID string
}

func newPublicationProgress(pubs PublicationSource, tenantID uint64, actionID string) *publicationProgress {
	return &publicationProgress{pubs: pubs, tenantID: tenantID, actionID: actionID}
}

func (p *publicationProgress) load(a appconn.Action) appconn.NotionPageProgress {
	row, err := p.pubs.FindByAction(context.Background(), p.tenantID, p.actionID)
	if err != nil {
		return appconn.NotionPageProgress{}
	}
	var prog appconn.NotionPageProgress
	if json.Unmarshal([]byte(row.ProgressJSON), &prog) != nil {
		return appconn.NotionPageProgress{}
	}
	return prog
}

func (p *publicationProgress) save(a appconn.Action, prog appconn.NotionPageProgress) error {
	raw, err := json.Marshal(prog)
	if err != nil {
		return err
	}
	return p.pubs.SaveProgress(context.Background(), p.tenantID, p.actionID, string(raw))
}

func (b *NotionBridge) run(ctx context.Context, snap appconnectorsvc.ActionSnapshot, query bool) (appconnectorsvc.DispatchOutcome, error) {
	adapter, err := b.adapterFor(ctx, snap)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	action := appconn.Action{
		ID: snap.ID, TenantID: snap.TenantID, ActorID: snap.ActorID,
		ConnectionID: snap.ConnectionID, Version: snap.Version,
		Target: snap.Target, Risk: snap.Risk, AuthVersion: snap.AuthVersion,
		Args: json.RawMessage(snap.Args),
	}
	var res appconn.ActionResult
	var aerr error
	if query {
		res, aerr = adapter.Query(ctx, action)
	} else {
		res, aerr = adapter.Execute(ctx, action)
	}
	if aerr != nil {
		switch res.State {
		case appconn.ActionFailed:
			reason := aerr.Error()
			if errors.Is(aerr, appconn.ErrNotionVersionConflict) {
				reason = PublishVersionConflictResult + ": remote version moved since approval"
			}
			return appconnectorsvc.DispatchOutcome{Status: appconn.ActionFailed, ProviderResult: reason, ExecutionID: res.ExternalID}, nil
		case appconn.ActionUnknown:
			return appconnectorsvc.DispatchOutcome{Status: appconn.ActionUnknown, ProviderResult: aerr.Error(), ExecutionID: res.ExternalID}, nil
		case appconn.ActionAwaitingApproval:
			return appconnectorsvc.DispatchOutcome{Status: appconn.ActionFailed, ProviderResult: "approval revoked before send", ExecutionID: res.ExternalID}, nil
		default:
			return appconnectorsvc.DispatchOutcome{}, aerr
		}
	}
	return appconnectorsvc.DispatchOutcome{Status: res.State, ProviderResult: string(res.Output), ExecutionID: res.ExternalID}, nil
}

// Dispatch implements appconnectorsvc.ActionDispatcher.
func (b *NotionBridge) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	return b.run(ctx, snap, false)
}

// QueryProvider implements appconnectorsvc.UnknownResolver: the provider
// (never the local queue) answers what became of an unknown dispatch.
func (b *NotionBridge) QueryProvider(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	return b.run(ctx, snap, true)
}

// ReadPageVersion is the plan-formation pre-read (AC1): the external
// current version of one page, read through the same policy/token ports.
func (b *NotionBridge) ReadPageVersion(ctx context.Context, connectionID, pageID string) (string, error) {
	scope, err := b.scopes.NotionScope(ctx, connectionID)
	if err != nil {
		return "", err
	}
	if scope.AppID != appIDNotion {
		return "", fmt.Errorf("connection %q is not a notion connection", connectionID)
	}
	pol, err := b.policies.PolicyFor(ctx, connectionID)
	if err != nil {
		return "", err
	}
	tok, err := b.tokens.Token(ctx, connectionID, scope.AuthVersion)
	if err != nil {
		return "", err
	}
	reader := &appconn.NotionUpdateAdapter{
		Policy: pol,
		Token:  func(ctx context.Context) (string, error) { return tok, nil },
	}
	return appconn.ReadNotionPageVersion(ctx, reader.Policy, reader.Token, pageID)
}

// ---- production port implementations ----

// dbNotionScopeSource resolves a connection's publish scope from the
// authoritative rows: connections → installations (app id) →
// app_versions.schema_json (reviewed scopes + approved destination pages).
type dbNotionScopeSource struct{ db *gorm.DB }

// NewDBNotionScopeSource builds the production scope source.
func NewDBNotionScopeSource(db *gorm.DB) NotionScopeSource { return &dbNotionScopeSource{db: db} }

func (s *dbNotionScopeSource) NotionScope(ctx context.Context, connectionID string) (NotionConnectionScope, error) {
	var conn repoappconn.ConnectionRow
	if err := s.db.WithContext(ctx).Where("id = ?", connectionID).First(&conn).Error; err != nil {
		return NotionConnectionScope{}, err
	}
	var inst repoappconn.InstallationRow
	if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", conn.InstallationID, conn.TenantID).First(&inst).Error; err != nil {
		return NotionConnectionScope{}, err
	}
	var ver repoappconn.AppVersion
	if err := s.db.WithContext(ctx).Where("app_id = ? AND version = ?", inst.AppID, inst.AppVersion).First(&ver).Error; err != nil {
		return NotionConnectionScope{}, err
	}
	var parsed struct {
		Scopes           []string `json:"scopes"`
		ApprovedParents  []string `json:"approved_parents"`
	}
	if err := json.Unmarshal([]byte(ver.SchemaJSON), &parsed); err != nil {
		return NotionConnectionScope{}, err
	}
	scope := NotionConnectionScope{
		AppID:           inst.AppID,
		ConnectionKind:  conn.Kind,
		OwnerID:         conn.OwnerID,
		AuthVersion:     conn.AuthVersion,
		ApprovedParents: parsed.ApprovedParents,
	}
	for _, sc := range parsed.Scopes {
		if sc == appconn.NotionCapabilityInsert {
			scope.InsertCapability = true
		}
	}
	return scope, nil
}

// constantNotionPolicyProvider pins the reviewed NO-01 outbound contract.
type constantNotionPolicyProvider struct{}

// NewConstantNotionPolicyProvider returns the production policy provider:
// HTTPS to the pinned Notion API host, GET/POST/PATCH under /v1/, 30s
// per-request timeout (inside the pipeline's 30s dispatch deadline).
func NewConstantNotionPolicyProvider() NotionPolicyProvider { return constantNotionPolicyProvider{} }

func (constantNotionPolicyProvider) PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error) {
	return appconn.HTTPPolicy{
		Scheme:     "https",
		Host:       appconn.NotionAPIHost,
		Methods:    []string{"GET", "POST", "PATCH"},
		PathPrefix: "/v1/",
		Timeout:    30 * time.Second,
	}, nil
}

// credentialTokenSource resolves the Notion token through the A02
// credential resolver (revoked / stale-version connections never resolve).
type credentialTokenSource struct{ resolver appconnectorsvc.CredentialResolver }

// NewCredentialTokenSource adapts the internal credential resolver onto
// the bridge's token port.
func NewCredentialTokenSource(resolver appconnectorsvc.CredentialResolver) NotionTokenSource {
	return &credentialTokenSource{resolver: resolver}
}

func (s *credentialTokenSource) Token(ctx context.Context, connectionID string, expectedVersion int64) (string, error) {
	raw, err := s.resolver.Resolve(ctx, connectionID, expectedVersion)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
```

同时向 `internal/modules/appconnector/notion_update.go` 追加一个小型公开读助手（`ReadNotionPageVersion`，Task 5 的 `ReadPageVersion` 依赖；不改动既有函数）：

```go
// ReadNotionPageVersion performs a one-off version read (GET
// /v1/pages/{id}) through the given reviewed policy and token source —
// the plan-formation pre-read shared by the publish seam. It performs no
// write of any kind.
func ReadNotionPageVersion(ctx context.Context, pol HTTPPolicy, token func(ctx context.Context) (string, error), pageID string) (string, error) {
	m := &NotionUpdateAdapter{Policy: pol, Token: token}
	ver, err := m.readPageVersion(ctx, pageID)
	if err != nil {
		return "", err
	}
	return ver.LastEditedTime, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/appconnector/publish/ ./internal/modules/appconnector/ -count=1`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/appconnector/publish/dispatcher.go internal/modules/appconnector/publish/dispatcher_test.go internal/modules/appconnector/notion_update.go
git commit -m "feat(publish): notion bridge dispatcher/resolver with scope, policy and credential ports (T18 #48)"
```

---

### Task 6: publish 包——`NotionPublishService`（计划形成 / 执行 / 对账 / 回执 settle）

**Files:**
- Create: `internal/modules/appconnector/publish/plan.go`
- Test: `internal/modules/appconnector/publish/plan_test.go`

**Interfaces:**
- Consumes: `appconnectorsvc.ActionService`（Prepare/Approve/Execute/ResolveUnknown/ErrDispatchUnknown，`service/appconnector/action.go:192-318/:593`）、`appconnectorsvc.ActionStoreSource.FindAction`、`repoappconn.PublicationStore`（Task 3）、`NotionBridge`（Task 5）、`NotionParagraphBlocks`（Task 4）、`repository.ArtifactVersion`/`ReadableArtifactVersion`（`internal/application/repository/artifact_version.go:51/:261`）。
- Produces（Task 7/8 依赖，签名逐字）:
  - 哨兵错误：`ErrPublishInvalidInput`、`ErrPublishArtifactNotReady`、`ErrPublishUnsupportedArtifact`、`ErrPublishDestinationOutOfScope`、`ErrPublishDestinationUnreadable`、`ErrPublishUpdateTargetNotPublished`（均 `errors.New(...)`）
  - `const MaxPublishArtifactBytes = 1 << 20`
  - `type ArtifactVersionReader interface { ReadableArtifactVersion(ctx context.Context, tenantID uint64, sessionID, versionID string) (repository.ArtifactVersion, error) }`
  - `type ArtifactContentReader interface { ReadArtifactContent(ctx context.Context, tenantID uint64, version repository.ArtifactVersion) ([]byte, error) }`
  - `type NotionRemoteReader interface { ReadPageVersion(ctx context.Context, connectionID, pageID string) (string, error) }`
  - `type PublishPlanInput struct { TenantID uint64; ActorID, ConnectionID, SessionID, ArtifactVersionID, Title, ParentPageID, PageID string }`
  - `type PublishPlanView struct { ActionID, Digest, State string; Fence int64; Mode, Destination, ExpectedExternalVersion, Title string; Artifact PublishArtifactView }`、`type PublishArtifactView struct { VersionID, Digest, MIME string; Size int64 }`
  - `type PublishReceiptView struct { ActionID, State, ExternalID, ExternalVersion, ArtifactVersionID, Destination, Mode string }`
  - `type PublishExecuteOutcome struct { ActionState string; Conflict bool; Receipt PublishReceiptView }`
  - `func NewNotionPublishService(actions *appconnectorsvc.ActionService, store appconnectorsvc.ActionStoreSource, pubs *repoappconn.PublicationStore, artifacts ArtifactVersionReader, content ArtifactContentReader, remote NotionRemoteReader, scopes NotionScopeSource) *NotionPublishService`
  - `func (s *NotionPublishService) FormPlan(ctx context.Context, in PublishPlanInput) (PublishPlanView, error)`
  - `func (s *NotionPublishService) Execute(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error)`
  - `func (s *NotionPublishService) Reconcile(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error)`
  - `func (s *NotionPublishService) Receipt(ctx context.Context, tenantID uint64, actionID string) (PublishReceiptView, error)`

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/appconnector/publish/plan_test.go`：

```go
package publish

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---- scenario doubles for the two non-DB ports ----

type fakeArtifacts struct {
	version repository.ArtifactVersion
	err     error
}

func (f *fakeArtifacts) ReadableArtifactVersion(ctx context.Context, tenantID uint64, sessionID, versionID string) (repository.ArtifactVersion, error) {
	return f.version, f.err
}

type fakeContent struct{ data []byte }

func (f *fakeContent) ReadArtifactContent(ctx context.Context, tenantID uint64, version repository.ArtifactVersion) ([]byte, error) {
	return f.data, nil
}

type fakeRemote struct {
	versions map[string]string
	err      error
}

func (f *fakeRemote) ReadPageVersion(ctx context.Context, connectionID, pageID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.versions[pageID], nil
}

type passGuard struct{}

func (passGuard) Check(ctx context.Context, subject appconn.OCSubject, connectionID string, expectedVersion int64) error {
	return nil
}

func openPlanDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repoappconn.ActionRow{}, &repoappconn.ApprovalRow{}, &repoappconn.PreAuthorizationRow{}, &repoappconn.PublicationRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

type planEnv struct {
	svc   *NotionPublishService
	pubs  *repoappconn.PublicationStore
	store *repoappconn.ActionStore
	fake  *bridgeFakeNotion
}

func newPlanEnv(t *testing.T, fake *bridgeFakeNotion, remote *fakeRemote) *planEnv {
	t.Helper()
	db := openPlanDB(t)
	store := repoappconn.NewActionStore(db)
	pubs := repoappconn.NewPublicationStore(db)
	// The dedicated publish ActionService: the bridge is BOTH dispatcher
	// and unknown resolver — the production composition (container wires
	// the same shape).
	pol := loopbackPolicyFor(t, fake)
	bridge := NewNotionBridge(
		&fakeScopes{scope: NotionConnectionScope{
			AppID: "notion", ConnectionKind: "personal", OwnerID: "u1", AuthVersion: 1,
			ApprovedParents: []string{"parent-1"}, InsertCapability: true,
		}},
		&fakePolicies{pol: pol},
		&fakeTokens{tok: "secret_test_token"},
		pubs,
	)
	actions := appconnectorsvc.NewActionService(store, passGuard{}, nil, bridge, bridge)
	artifacts := &fakeArtifacts{version: repository.ArtifactVersion{
		TenantID: 7, ID: "ver-1", RunID: "run-1", SessionID: "sess-1",
		Digest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ObjectKey: "artifact-versions/7/run-1/d", MIME: "text/plain", ScanState: repository.ArtifactScanReady, Size: 12,
	}}
	svc := NewNotionPublishService(actions, store, pubs, artifacts, &fakeContent{data: []byte("hello\n\nworld")}, remote,
		&fakeScopes{scope: NotionConnectionScope{
			AppID: "notion", ConnectionKind: "personal", OwnerID: "u1", AuthVersion: 1,
			ApprovedParents: []string{"parent-1"}, InsertCapability: true,
		}})
	return &planEnv{svc: svc, pubs: pubs, store: store, fake: fake}
}

func loopbackPolicyFor(t *testing.T, fake *bridgeFakeNotion) appconn.HTTPPolicy {
	t.Helper()
	srv := fake.server(t)
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	return appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PATCH"}, PathPrefix: "/v1/",
		AuthorizedNetworks: []*net.IPNet{network},
	}
}

func planInput() PublishPlanInput {
	return PublishPlanInput{
		TenantID: 7, ActorID: "u1", ConnectionID: "conn-notion",
		SessionID: "sess-1", ArtifactVersionID: "ver-1", Title: "Report",
		ParentPageID: "parent-1",
	}
}

func approvePlan(t *testing.T, env *planEnv, view PublishPlanView) {
	t.Helper()
	if err := env.svc.actions.Approve(context.Background(), view.ActionID, "u1", view.Digest); err != nil {
		t.Fatal(err)
	}
}

func TestFormPlanCreateBindsBaselineVersion(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("parent-1", "root", "Parent")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{"parent-1": "2026-09-24T08:00:00.000Z"}})
	view, err := env.svc.FormPlan(context.Background(), planInput())
	if err != nil {
		t.Fatal(err)
	}
	if view.Mode != "create" || view.Destination != "parent-1" {
		t.Fatalf("plan shape: %+v", view)
	}
	// AC1 baseline: the plan records the external version it read.
	if view.ExpectedExternalVersion != "2026-09-24T08:00:00.000Z" {
		t.Fatalf("baseline version must be bound: %+v", view)
	}
	row, err := env.store.FindAction(context.Background(), view.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if row.State != appconn.ActionAwaitingApproval || row.Risk != appconn.RiskWrite {
		t.Fatalf("prepared action row: %+v", row)
	}
	var snap struct {
		Parent string `json:"parent"`
	}
	if err := json.Unmarshal([]byte(row.ArgsSnapshot), &snap); err != nil || snap.Parent != "parent-1" {
		t.Fatalf("create snapshot must carry the approved parent: %s (%v)", row.ArgsSnapshot, err)
	}
	pub, perr := env.pubs.FindByAction(context.Background(), 7, view.ActionID)
	if perr != nil || pub.State != repoappconn.PublicationPlanned || pub.ArtifactVersionID != "ver-1" ||
		pub.ExpectedVersion != "2026-09-24T08:00:00.000Z" {
		t.Fatalf("planned publication must bind artifact + destination + version: %+v %v", pub, perr)
	}
}

func TestFormPlanCreateParentOutOfScopeFailsClosed(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{}})
	in := planInput()
	in.ParentPageID = "parent-unlisted"
	if _, err := env.svc.FormPlan(context.Background(), in); !errors.Is(err, ErrPublishDestinationOutOfScope) {
		t.Fatalf("unlisted parent must fail closed, got %v", err)
	}
}

func TestFormPlanDestinationUnreadable(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	env := newPlanEnv(t, fake, &fakeRemote{err: errors.New("boom")})
	if _, err := env.svc.FormPlan(context.Background(), planInput()); !errors.Is(err, ErrPublishDestinationUnreadable) {
		t.Fatalf("unreadable destination must refuse plan formation, got %v", err)
	}
}

func TestFormPlanUpdateRequiresPriorPublishedReceipt(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("page-9", "parent-1", "old")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{"page-9": "v"}})
	in := planInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	if _, err := env.svc.FormPlan(context.Background(), in); !errors.Is(err, ErrPublishUpdateTargetNotPublished) {
		t.Fatalf("update target must chain from a prior receipt, got %v", err)
	}
}

func TestFormPlanUpdateBindsExpectedVersion(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("page-9", "parent-1", "old")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{"page-9": "2026-09-24T08:00:00.000Z"}})
	// The prior published receipt that authorizes this target.
	if err := env.pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-prev", ConnectionID: "conn-notion", Provider: "notion",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.pubs.SettlePublication(context.Background(), 7, "act-prev", repoappconn.PublicationPublished, "page-9", "v0", "{}"); err != nil {
		t.Fatal(err)
	}
	in := planInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	view, err := env.svc.FormPlan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if view.Mode != "update" || view.ExpectedExternalVersion != "2026-09-24T08:00:00.000Z" {
		t.Fatalf("update plan must bind the read version: %+v", view)
	}
	row, _ := env.store.FindAction(context.Background(), view.ActionID)
	var snap struct {
		PageID          string `json:"page_id"`
		ExpectedVersion string `json:"expected_version"`
	}
	if err := json.Unmarshal([]byte(row.ArgsSnapshot), &snap); err != nil || snap.PageID != "page-9" || snap.ExpectedVersion != "2026-09-24T08:00:00.000Z" {
		t.Fatalf("update snapshot must carry page_id + expected_version: %s (%v)", row.ArgsSnapshot, err)
	}
}

func TestExecuteCreatePublishesAndSettlesReceipt(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("parent-1", "root", "Parent")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{"parent-1": "2026-09-24T09:00:00.000Z"}})
	view, err := env.svc.FormPlan(context.Background(), planInput())
	if err != nil {
		t.Fatal(err)
	}
	approvePlan(t, env, view)
	out, err := env.svc.Execute(context.Background(), 7, view.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if out.ActionState != appconn.ActionSucceeded || out.Receipt.State != repoappconn.PublicationPublished {
		t.Fatalf("publish outcome: %+v", out)
	}
	if out.Receipt.ExternalID == "" || out.Receipt.ExternalVersion == "" {
		t.Fatalf("receipt must carry external id + version: %+v", out.Receipt)
	}
}

func TestExecuteUpdateConflictSettlesFailedReceipt(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("page-9", "parent-1", "old")
	remote := &fakeRemote{versions: map[string]string{"page-9": "2026-09-24T08:00:00.000Z"}}
	env := newPlanEnv(t, fake, remote)
	if err := env.pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-prev", ConnectionID: "conn-notion", Provider: "notion",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.pubs.SettlePublication(context.Background(), 7, "act-prev", repoappconn.PublicationPublished, "page-9", "v0", "{}"); err != nil {
		t.Fatal(err)
	}
	in := planInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	view, err := env.svc.FormPlan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	approvePlan(t, env, view)
	// External collaborator edits between approval and execute — the
	// bridge's pre-read (through the REAL fake server) observes the drift.
	fake.touch("page-9", "2026-09-24T10:30:00.000Z")
	out, err := env.svc.Execute(context.Background(), 7, view.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Conflict || out.ActionState != appconn.ActionFailed {
		t.Fatalf("conflict outcome: %+v", out)
	}
	if out.Receipt.State != repoappconn.PublicationFailed {
		t.Fatalf("receipt must settle failed: %+v", out.Receipt)
	}
	if fake.patchCalls != 0 {
		t.Fatalf("conflict must write nothing: %d patches", fake.patchCalls)
	}
}

func TestPublishReconcileResolvesUnknownWithoutRedispatch(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("page-9", "parent-1", "old")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{"page-9": "2026-09-24T08:00:00.000Z"}})
	if err := env.pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-prev", ConnectionID: "conn-notion", Provider: "notion",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.pubs.SettlePublication(context.Background(), 7, "act-prev", repoappconn.PublicationPublished, "page-9", "v0", "{}"); err != nil {
		t.Fatal(err)
	}
	in := planInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	view, err := env.svc.FormPlan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	approvePlan(t, env, view)
	// The append's reply is lost after the effect applied → unknown.
	fake.mu.Lock()
	fake.dropNextAppend = true
	fake.mu.Unlock()
	out, err := env.svc.Execute(context.Background(), 7, view.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if out.ActionState != appconn.ActionUnknown {
		t.Fatalf("lost reply must park unknown, got %s", out.ActionState)
	}
	// Blind re-publish is structurally refused (the store only claims
	// from authorized).
	if _, err := env.svc.Execute(context.Background(), 7, view.ActionID); !errors.Is(err, appconnectorsvc.ErrActionState) {
		t.Fatalf("re-publish while unknown must be refused, got %v", err)
	}
	appendsBefore := fake.appendCalls
	rec, rerr := env.svc.Reconcile(context.Background(), 7, view.ActionID)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if rec.ActionState != appconn.ActionSucceeded || rec.Receipt.State != repoappconn.PublicationPublished {
		t.Fatalf("reconcile must resolve from the remote: %+v", rec)
	}
	if fake.appendCalls != appendsBefore {
		t.Fatalf("reconcile must not re-send: %d -> %d", appendsBefore, fake.appendCalls)
	}
}

func TestFormPlanRejectsBadInputs(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{}})
	cases := map[string]PublishPlanInput{
		"no title":      func() PublishPlanInput { in := planInput(); in.Title = ""; return in }(),
		"both targets":  func() PublishPlanInput { in := planInput(); in.PageID = "p"; return in }(),
		"no target":     func() PublishPlanInput { in := planInput(); in.ParentPageID = ""; return in }(),
		"no version id": func() PublishPlanInput { in := planInput(); in.ArtifactVersionID = ""; return in }(),
	}
	for name, in := range cases {
		if _, err := env.svc.FormPlan(context.Background(), in); !errors.Is(err, ErrPublishInvalidInput) {
			t.Fatalf("%s: want ErrPublishInvalidInput, got %v", name, err)
		}
	}
}

func TestFormPlanRejectsUnsupportedArtifact(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{"parent-1": "v"}})
	env.svc.artifacts = &fakeArtifacts{version: repository.ArtifactVersion{
		TenantID: 7, ID: "ver-2", SessionID: "sess-1", MIME: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		ScanState: repository.ArtifactScanReady, Size: 10, Digest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}}
	if _, err := env.svc.FormPlan(context.Background(), planInput()); !errors.Is(err, ErrPublishUnsupportedArtifact) {
		t.Fatalf("binary artifact must be refused, got %v", err)
	}
}

func TestFormPlanRejectsNotReadyArtifact(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	env := newPlanEnv(t, fake, &fakeRemote{versions: map[string]string{"parent-1": "v"}})
	env.svc.artifacts = &fakeArtifacts{err: errors.New("not found")}
	if _, err := env.svc.FormPlan(context.Background(), planInput()); !errors.Is(err, ErrPublishArtifactNotReady) {
		t.Fatalf("unreadable version must be refused, got %v", err)
	}
}
```

注意：`approvePlan` 直接访问 `env.svc.actions`（非导出字段）——同包测试合法。`TestFormPlanRejectsUnsupportedArtifact`/`NotReady` 中替换 `env.svc.artifacts` 同理。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/appconnector/publish/ -count=1`
Expected: FAIL（`undefined: NewNotionPublishService`）。

- [ ] **Step 3: 最小实现**

创建 `internal/modules/appconnector/publish/plan.go`：

```go
package publish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
)

// Publish sentinel errors. None of them ever carries credential material.
var (
	ErrPublishInvalidInput            = errors.New("publish_invalid_input")
	ErrPublishArtifactNotReady        = errors.New("publish_artifact_not_ready")
	ErrPublishUnsupportedArtifact     = errors.New("publish_unsupported_artifact")
	ErrPublishDestinationOutOfScope   = errors.New("publish_destination_out_of_scope")
	ErrPublishDestinationUnreadable   = errors.New("publish_destination_unreadable")
	ErrPublishUpdateTargetNotPublished = errors.New("publish_update_target_not_published")
)

// MaxPublishArtifactBytes bounds the artifact bytes a plan may read.
const MaxPublishArtifactBytes = 1 << 20

// ArtifactVersionReader reads immutable, published (ready) artifact
// versions — the「确定 Artifact」anchor. Satisfied by
// *repository.ArtifactVersionStore.
type ArtifactVersionReader interface {
	ReadableArtifactVersion(ctx context.Context, tenantID uint64, sessionID, versionID string) (repository.ArtifactVersion, error)
}

// ArtifactContentReader reads one version's bytes from tenant storage.
type ArtifactContentReader interface {
	ReadArtifactContent(ctx context.Context, tenantID uint64, version repository.ArtifactVersion) ([]byte, error)
}

// NotionRemoteReader is the plan-formation pre-read port (satisfied by
// *NotionBridge.ReadPageVersion).
type NotionRemoteReader interface {
	ReadPageVersion(ctx context.Context, connectionID, pageID string) (string, error)
}

// PublishPlanInput forms one publish plan: exactly one destination shape
// (ParentPageID for create, PageID for update).
type PublishPlanInput struct {
	TenantID          uint64
	ActorID           string
	ConnectionID      string
	SessionID         string
	ArtifactVersionID string
	Title             string
	ParentPageID      string
	PageID            string
}

// PublishArtifactView is the immutable internal artifact anchor carried by
// the plan and the receipt.
type PublishArtifactView struct {
	VersionID string `json:"version_id"`
	Digest    string `json:"digest"`
	MIME      string `json:"mime"`
	Size      int64  `json:"size"`
}

// PublishPlanView is the formed Action Plan as returned to the caller: the
// prepared action (digest + fence are what an approval binds) plus the
// destination and the external version the plan read.
type PublishPlanView struct {
	ActionID                string              `json:"action_id"`
	Digest                  string              `json:"digest"`
	State                   string              `json:"state"`
	Fence                   int64               `json:"expected_version"`
	Mode                    string              `json:"mode"`
	Destination             string              `json:"destination"`
	ExpectedExternalVersion string              `json:"expected_external_version"`
	Title                   string              `json:"title"`
	Artifact                PublishArtifactView `json:"artifact"`
}

// PublishReceiptView is the durable 外部发布 record's projection.
type PublishReceiptView struct {
	ActionID          string `json:"action_id"`
	State             string `json:"state"`
	Mode              string `json:"mode"`
	Destination       string `json:"destination"`
	ExternalID        string `json:"external_id,omitempty"`
	ExternalVersion   string `json:"external_version,omitempty"`
	ArtifactVersionID string `json:"artifact_version_id"`
}

// PublishExecuteOutcome is the result of Execute/Reconcile: the ACTION
// row's authoritative state, the conflict marker, and the receipt
// projection.
type PublishExecuteOutcome struct {
	ActionState string            `json:"action_state"`
	Conflict    bool              `json:"conflict"`
	Receipt     PublishReceiptView `json:"publication"`
}

// NotionPublishService forms approved publish plans from confirmed
// artifact versions, executes them through the dedicated A03 ActionService
// (whose dispatcher/resolver is the Notion bridge), and settles the
// publication receipt from the action row's authoritative outcome — the
// receipt is always a projection of the action, never the reverse.
type NotionPublishService struct {
	actions   *appconnectorsvc.ActionService
	store     appconnectorsvc.ActionStoreSource
	pubs      *repoappconn.PublicationStore
	artifacts ArtifactVersionReader
	content   ArtifactContentReader
	remote    NotionRemoteReader
	scopes    NotionScopeSource
}

// NewNotionPublishService builds the publish service.
func NewNotionPublishService(
	actions *appconnectorsvc.ActionService,
	store appconnectorsvc.ActionStoreSource,
	pubs *repoappconn.PublicationStore,
	artifacts ArtifactVersionReader,
	content ArtifactContentReader,
	remote NotionRemoteReader,
	scopes NotionScopeSource,
) *NotionPublishService {
	return &NotionPublishService{actions: actions, store: store, pubs: pubs, artifacts: artifacts, content: content, remote: remote, scopes: scopes}
}

func publishableMIME(mime string) bool {
	m := strings.ToLower(strings.TrimSpace(mime))
	return strings.HasPrefix(m, "text/") || m == "application/json"
}

// FormPlan forms one publish Action Plan (CONTEXT.md「操作计划/外部发布」):
// resolve the confirmed artifact version, derive the Notion blocks
// deterministically, READ the external current version (AC1 baseline; the
// update snapshot binds it as expected_version), then Prepare the A03
// action whose normalized bytes + digest the approval binds, and record
// the planned publication row.
func (s *NotionPublishService) FormPlan(ctx context.Context, in PublishPlanInput) (PublishPlanView, error) {
	if in.TenantID == 0 || in.ActorID == "" || in.ConnectionID == "" || in.SessionID == "" ||
		in.ArtifactVersionID == "" || strings.TrimSpace(in.Title) == "" {
		return PublishPlanView{}, fmt.Errorf("%w: tenant, actor, connection, session, artifact version and title are required", ErrPublishInvalidInput)
	}
	if (in.ParentPageID == "") == (in.PageID == "") {
		return PublishPlanView{}, fmt.Errorf("%w: exactly one of parent_page_id (create) or page_id (update)", ErrPublishInvalidInput)
	}
	scope, err := s.scopes.NotionScope(ctx, in.ConnectionID)
	if err != nil {
		return PublishPlanView{}, fmt.Errorf("%w: connection scope: %v", ErrPublishInvalidInput, err)
	}
	if scope.AppID != "notion" {
		return PublishPlanView{}, fmt.Errorf("%w: connection is %q, not notion", ErrPublishInvalidInput, scope.AppID)
	}
	mode, destination := "create", in.ParentPageID
	if in.PageID != "" {
		mode, destination = "update", in.PageID
		// Authority rule: a page this tenant+connection published before
		// is ours to update; anything else fails closed.
		if _, perr := s.pubs.LatestPublishedByDestination(ctx, in.TenantID, in.ConnectionID, in.PageID); perr != nil {
			return PublishPlanView{}, fmt.Errorf("%w: %s has no prior receipt through this connection", ErrPublishUpdateTargetNotPublished, in.PageID)
		}
	} else {
		approved := false
		for _, p := range scope.ApprovedParents {
			if p == in.ParentPageID {
				approved = true
				break
			}
		}
		if !approved {
			return PublishPlanView{}, fmt.Errorf("%w: parent %q is not in the reviewed destination scope", ErrPublishDestinationOutOfScope, in.ParentPageID)
		}
	}
	version, verr := s.artifacts.ReadableArtifactVersion(ctx, in.TenantID, in.SessionID, in.ArtifactVersionID)
	if verr != nil {
		return PublishPlanView{}, fmt.Errorf("%w: %v", ErrPublishArtifactNotReady, verr)
	}
	if !publishableMIME(version.MIME) {
		return PublishPlanView{}, fmt.Errorf("%w: mime %q", ErrPublishUnsupportedArtifact, version.MIME)
	}
	content, cerr := s.content.ReadArtifactContent(ctx, in.TenantID, version)
	if cerr != nil {
		return PublishPlanView{}, fmt.Errorf("%w: %v", ErrPublishArtifactNotReady, cerr)
	}
	if len(content) > MaxPublishArtifactBytes {
		return PublishPlanView{}, fmt.Errorf("%w: %d bytes", ErrPublishContentTooLarge, len(content))
	}
	blocks, berr := NotionParagraphBlocks(string(content))
	if berr != nil {
		return PublishPlanView{}, berr
	}
	// AC1: 发布前读取外部当前版本 —— the plan-time pre-read. For update
	// this value is bound into the approved snapshot; for create it is the
	// destination's recorded baseline.
	remoteVersion, rerr := s.remote.ReadPageVersion(ctx, in.ConnectionID, destination)
	if rerr != nil || remoteVersion == "" {
		return PublishPlanView{}, fmt.Errorf("%w: %v", ErrPublishDestinationUnreadable, rerr)
	}
	var args []byte
	if mode == "update" {
		args, err = json.Marshal(map[string]any{
			"page_id": in.PageID, "expected_version": remoteVersion,
			"title": in.Title, "blocks": blocks,
		})
	} else {
		args, err = json.Marshal(map[string]any{
			"parent": in.ParentPageID, "title": in.Title, "blocks": blocks,
		})
	}
	if err != nil {
		return PublishPlanView{}, err
	}
	actionID, perr := s.actions.Prepare(ctx, appconn.Action{
		ID: "", TenantID: in.TenantID, ActorID: in.ActorID, ConnectionID: in.ConnectionID,
		Version: "notion/v1", Target: destination, Risk: appconn.RiskWrite,
		AuthVersion: scope.AuthVersion, Args: args,
	})
	if perr != nil {
		return PublishPlanView{}, perr
	}
	if uerr := s.pubs.CreatePublication(ctx, repoappconn.PublicationRow{
		TenantID: in.TenantID, ActionID: actionID, ConnectionID: in.ConnectionID,
		Provider: "notion", Mode: mode, Destination: destination,
		ExpectedVersion: remoteVersion, ArtifactVersionID: version.ID,
		ArtifactDigest: version.Digest, State: repoappconn.PublicationPlanned,
	}); uerr != nil {
		// The prepared action stays awaiting_approval (never approved,
		// never dispatched) — an orphan plan row is harmless, a fabricated
		// receipt is not.
		return PublishPlanView{}, uerr
	}
	row, aerr := s.store.FindAction(ctx, actionID)
	if aerr != nil {
		return PublishPlanView{}, aerr
	}
	return PublishPlanView{
		ActionID: actionID, Digest: row.ArgsDigest, State: row.State, Fence: row.Fence,
		Mode: mode, Destination: destination, ExpectedExternalVersion: remoteVersion,
		Title: in.Title,
		Artifact: PublishArtifactView{VersionID: version.ID, Digest: version.Digest, MIME: version.MIME, Size: version.Size},
	}, nil
}

// Execute runs the approved plan through the dedicated ActionService and
// settles the receipt from the action row's authoritative outcome. ErrDispatchUnknown
// is NOT an error here: the outcome (unknown + receipt projection) reports
// the honest parked state for reconciliation.
func (s *NotionPublishService) Execute(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error) {
	err := s.actions.Execute(ctx, actionID)
	if err != nil && !errors.Is(err, appconnectorsvc.ErrDispatchUnknown) {
		return PublishExecuteOutcome{}, err
	}
	return s.project(ctx, tenantID, actionID)
}

// Reconcile resolves an unknown outcome by querying the PROVIDER first —
// never a re-dispatch — then settles the receipt from the action row.
func (s *NotionPublishService) Reconcile(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error) {
	err := s.actions.ResolveUnknown(ctx, actionID)
	if err != nil && !errors.Is(err, appconnectorsvc.ErrDispatchUnknown) {
		return PublishExecuteOutcome{}, err
	}
	return s.project(ctx, tenantID, actionID)
}

// Receipt returns the publication record's current projection.
func (s *NotionPublishService) Receipt(ctx context.Context, tenantID uint64, actionID string) (PublishReceiptView, error) {
	pub, err := s.pubs.FindByAction(ctx, tenantID, actionID)
	if err != nil {
		return PublishReceiptView{}, err
	}
	return publicationView(pub), nil
}

func publicationView(pub repoappconn.PublicationRow) PublishReceiptView {
	return PublishReceiptView{
		ActionID: pub.ActionID, State: pub.State, Mode: pub.Mode,
		Destination: pub.Destination, ExternalID: pub.ExternalID,
		ExternalVersion: pub.ExternalVersion, ArtifactVersionID: pub.ArtifactVersionID,
	}
}

// project settles the receipt FROM the action row's authoritative state.
// Order matters: the action row is written by the ActionService first;
// only then does the receipt follow. A settle failure surfaces while the
// action state stays durable (the receipt can be re-settled).
func (s *NotionPublishService) project(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error) {
	row, err := s.store.FindAction(ctx, actionID)
	if err != nil {
		return PublishExecuteOutcome{}, err
	}
	if row.TenantID != tenantID {
		return PublishExecuteOutcome{}, repoappconn.ErrActionNotFound
	}
	out := PublishExecuteOutcome{ActionState: row.State, Conflict: strings.HasPrefix(row.ProviderResult, PublishVersionConflictResult)}
	switch row.State {
	case appconn.ActionSucceeded:
		rcpt, rerr := appconn.ParseNotionPageReceipt([]byte(row.ProviderResult))
		if rerr == nil {
			if serr := s.pubs.SettlePublication(ctx, tenantID, actionID, repoappconn.PublicationPublished, rcpt.ExternalID, rcpt.ExternalVersion, row.ProviderResult); serr != nil && !errors.Is(serr, repoappconn.ErrPublicationConflict) {
				return out, serr
			}
		}
	case appconn.ActionFailed:
		if serr := s.pubs.SettlePublication(ctx, tenantID, actionID, repoappconn.PublicationFailed, "", "", row.ProviderResult); serr != nil && !errors.Is(serr, repoappconn.ErrPublicationConflict) {
			return out, serr
		}
	case appconn.ActionUnknown:
		if serr := s.pubs.SettlePublication(ctx, tenantID, actionID, repoappconn.PublicationUnknown, "", "", ""); serr != nil && !errors.Is(serr, repoappconn.ErrPublicationConflict) {
			return out, serr
		}
	}
	pub, perr := s.pubs.FindByAction(ctx, tenantID, actionID)
	if perr != nil {
		return out, perr
	}
	out.Receipt = publicationView(pub)
	return out, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/appconnector/publish/ -count=1`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/appconnector/publish/plan.go internal/modules/appconnector/publish/plan_test.go
git commit -m "feat(publish): notion publish service - plan formation with external version pre-read, execute, remote-first reconcile, receipt settle (T18 #48)"
```

---

### Task 7: HTTP 端点、路由注册与容器接线

**Files:**
- Create: `internal/handler/app_connector_notion_publish.go`
- Create: `internal/router/routes_app_notion_publish.go`
- Create: `internal/container/notion_publish.go`
- Modify: `internal/router/router.go`（`RouterParams` 结构体 `AppActionHandler` 字段后加 1 字段；`RegisterAppConnectorRoutes(...)` 调用后加 1 行）
- Modify: `internal/container/container.go`（`SetOCConnectionService` invoke 块后加 1 个 Provide）
- Test: `internal/handler/app_connector_notion_publish_test.go`

**Interfaces:**
- Consumes: `publish.NotionPublishService`（Task 6 全部导出面）、`appFail/appOK/appTenantScope/appRequireWriteCapability`（`internal/handler/app_connector.go:28-81`）、`appconnector.CanDriveActionWrites`、`appconnectorrepo.ConnectionRow/ActionRow`、既有 `POST /apps/actions/:id/approve`（审批走既有端点，不重复实现）。
- Produces（Task 8 与 #49/#50 依赖）:
  - `type AppNotionPublishHandler struct` + `func NewAppNotionPublishHandler(db *gorm.DB) *AppNotionPublishHandler` + `func (h *AppNotionPublishHandler) SetNotionPublishService(s *publish.NotionPublishService)`
  - 端点（路由组 `r.Group("/apps/notion-publish", h.RequireActionCapabilityForWrites())`，均在认证 `/api/v1` 组内、不进 API-key 授权表——机器密钥默认拒绝，与 `routes_app_connectors.go:10-16` 同姿态）:
    - `POST /apps/notion-publish/plans` → `FormNotionPublishPlan`
    - `POST /apps/notion-publish/actions/:id/publish` → `PublishNotionAction`
    - `POST /apps/notion-publish/actions/:id/reconcile` → `ReconcileNotionAction`
    - `GET /apps/notion-publish/actions/:id` → `GetNotionPublication`
  - `func RegisterAppNotionPublishRoutes(r *gin.RouterGroup, h *handler.AppNotionPublishHandler)`（`internal/router/routes_app_notion_publish.go`）
  - 容器构造器 `newNotionPublishHandler(...)`（`internal/container/notion_publish.go`，dig `Provide`）

- [ ] **Step 1: 写失败测试**

创建 `internal/handler/app_connector_notion_publish_test.go`：

```go
package handler

// Notion publish endpoint gates (T18 #48). The publish pipeline itself is
// service-level (publish package) and end-to-end (Task 8); these pin the
// HTTP predicates: role gate, personal-connection owner predicate,
// fail-closed unconfigured service, malformed input, tenant-scoped
// lookups.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// publishTestContext injects the authenticated identity exactly like the
// OC engine middleware (app_connector_oc_test.go:275-298).
func publishTestContext(ctx context.Context, tenant uint64, role, user string) context.Context {
	ctx = context.WithValue(ctx, types.TenantIDContextKey, tenant)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRole(role))
	ctx = context.WithValue(ctx, types.UserIDContextKey, user)
	return ctx
}

func newNotionPublishTestEngine(t *testing.T) (*gin.Engine, *AppNotionPublishHandler, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(&appconnectorrepo.ConnectionRow{}, &appconnectorrepo.InstallationRow{}, &appconnectorrepo.AppVersion{}, &appconnectorrepo.ActionRow{}, &appconnectorrepo.ApprovalRow{}, &appconnectorrepo.PreAuthorizationRow{}, &appconnectorrepo.PublicationRow{}))
	// conn-notion is user-a's PERSONAL notion connection.
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-notion", TenantID: 7, AppID: "notion", AppVersion: "v1", State: "active"}).Error)
	require.NoError(t, db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('notion', 'v1', '{"scopes":["insert_content"],"approved_parents":["parent-1"]}', '{}')`).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 7, ID: "conn-notion", InstallationID: "inst-notion", Kind: "personal", OwnerID: "user-a", CredentialRef: "mcp_oauth_token:notion", State: "active", AuthVersion: 1}).Error)

	h := NewAppNotionPublishHandler(db)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		tenant := uint64(7)
		role := "admin" // CanDriveActionWrites admits owner/admin only (access.go:43-45)
		user := "user-a"
		if c.GetHeader("X-Test-User") != "" {
			user = c.GetHeader("X-Test-User")
		}
		if c.GetHeader("X-Test-Role") != "" {
			role = c.GetHeader("X-Test-Role")
		}
		if c.GetHeader("X-Test-Tenant") == "8" {
			tenant = 8
		}
		c.Request = c.Request.WithContext(publishTestContext(c.Request.Context(), tenant, role, user))
		c.Next()
	})
	g := engine.Group("/api/v1/apps/notion-publish", h.RequireActionCapabilityForWrites())
	g.POST("/plans", h.FormNotionPublishPlan)
	g.POST("/actions/:id/publish", h.PublishNotionAction)
	g.POST("/actions/:id/reconcile", h.ReconcileNotionAction)
	g.GET("/actions/:id", h.GetNotionPublication)
	return engine, h, db
}

func notionPublishDo(t *testing.T, engine *gin.Engine, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestNotionPublishPlanGates(t *testing.T) {
	engine, h, _ := newNotionPublishTestEngine(t)
	body := `{"connection_id":"conn-notion","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"parent-1"}`

	// Service not wired: fail closed 501 (mirroring the frozen
	// ACTION_PIPELINE_NOT_CONFIGURED), nothing formed.
	w := notionPublishDo(t, engine, http.MethodPost, "/api/v1/apps/notion-publish/plans", body)
	require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "PUBLISH_PIPELINE_NOT_CONFIGURED")
	require.NotNil(t, h)

	// Viewer role: the write gate refuses.
	w = notionPublishDo(t, engine, http.MethodPost, "/api/v1/apps/notion-publish/plans", body, "X-Test-Role", "viewer")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// Another member may not use user-a's PERSONAL connection (same
	// predicate as PrepareAction).
	w = notionPublishDo(t, engine, http.MethodPost, "/api/v1/apps/notion-publish/plans", body, "X-Test-User", "user-b", "X-Test-Role", "admin")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "NOT_CONNECTION_OWNER")

	// Malformed input: both destination shapes at once.
	w = notionPublishDo(t, engine, http.MethodPost, "/api/v1/apps/notion-publish/plans",
		`{"connection_id":"conn-notion","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"p","page_id":"q"}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "INVALID_REQUEST")

	// Unknown connection: 404 without leaking existence details.
	w = notionPublishDo(t, engine, http.MethodPost, "/api/v1/apps/notion-publish/plans",
		`{"connection_id":"conn-nope","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"parent-1"}`)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestNotionPublishActionLookupIsTenantScoped(t *testing.T) {
	engine, h, db := newNotionPublishTestEngine(t)
	h.SetNotionPublishService(nil) // stays unconfigured; lookup gates first
	require.NoError(t, db.Create(&appconnectorrepo.ActionRow{ID: "act-x", TenantID: 7, ConnectionID: "conn-notion",
		AppVersion: "notion/v1", Target: "parent-1", Risk: "write", ArgsSnapshot: "{}", ArgsDigest: "d",
		State: "authorized"}).Error)
	// Same tenant: reaches the unconfigured refusal (501).
	w := notionPublishDo(t, engine, http.MethodPost, "/api/v1/apps/notion-publish/actions/act-x/publish", "")
	require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
	// Foreign tenant id is indistinguishable from missing: 404.
	w = notionPublishDo(t, engine, http.MethodPost, "/api/v1/apps/notion-publish/actions/act-x/publish", "", "X-Test-Tenant", "8")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = notionPublishDo(t, engine, http.MethodGet, "/api/v1/apps/notion-publish/actions/act-x", "", "X-Test-Tenant", "8")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/handler/ -run 'TestNotionPublishPlanGates|TestNotionPublishActionLookupIsTenantScoped' -count=1`
Expected: FAIL（`undefined: NewAppNotionPublishHandler`）。

- [ ] **Step 3: 最小实现（handler）**

创建 `internal/handler/app_connector_notion_publish.go`：

```go
package handler

import (
	"errors"
	"net/http"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AppNotionPublishHandler serves the T18 Notion publish closed loop under
// /api/v1/apps/notion-publish. Plan formation derives the approved
// snapshot SERVER-SIDE from a confirmed artifact version + destination;
// approval stays on the existing POST /apps/actions/:id/approve endpoint
// (its predicate owns approval authority); execution/reconciliation run
// through the publish service whose ActionService holds the Notion bridge
// dispatcher. The handler is nil-service fail-closed like its siblings.
type AppNotionPublishHandler struct {
	db     *gorm.DB
	publish *publish.NotionPublishService
}

// NewAppNotionPublishHandler constructs the handler over the business DB.
func NewAppNotionPublishHandler(db *gorm.DB) *AppNotionPublishHandler {
	return &AppNotionPublishHandler{db: db}
}

// SetNotionPublishService wires the publish service (container injection
// point). Until called every endpoint fails closed with 501
// PUBLISH_PIPELINE_NOT_CONFIGURED (mirroring the frozen action pipeline).
func (h *AppNotionPublishHandler) SetNotionPublishService(s *publish.NotionPublishService) { h.publish = s }

// RequireActionCapabilityForWrites mirrors the action write gate
// (CanDriveActionWrites): plan formation and execution are action writes.
func (h *AppNotionPublishHandler) RequireActionCapabilityForWrites() gin.HandlerFunc {
	return appRequireWriteCapability(appconnector.CanDriveActionWrites,
		"FORBIDDEN_ACTION_WRITE",
		"publish writes require owner or admin role")
}

type notionPublishPlanInput struct {
	ConnectionID      string `json:"connection_id"`
	SessionID         string `json:"session_id"`
	ArtifactVersionID string `json:"artifact_version_id"`
	Title             string `json:"title"`
	ParentPageID      string `json:"parent_page_id"`
	PageID            string `json:"page_id"`
}

// notionActionByID resolves an action inside the tenant — a cross-tenant
// id and a missing one are both 404, never a 403 that leaks existence.
func (h *AppNotionPublishHandler) notionActionByID(c *gin.Context, tenantID uint64, id string) (appconnectorrepo.ActionRow, bool) {
	var row appconnectorrepo.ActionRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
		appFail(c, http.StatusNotFound, "ACTION_NOT_FOUND", "action not found")
		return row, false
	}
	return row, true
}

func (h *AppNotionPublishHandler) notionConnection(c *gin.Context, tenantID uint64, connectionID, userID string) (appconnectorrepo.ConnectionRow, bool) {
	var conn appconnectorrepo.ConnectionRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, connectionID).First(&conn).Error; err != nil {
		appFail(c, http.StatusNotFound, "CONNECTION_NOT_FOUND", "connection not found")
		return conn, false
	}
	// A personal connection is its owner's identity (same predicate as
	// PrepareAction / #42).
	if conn.Kind == appconnector.ConnectionKindPersonal && conn.OwnerID != userID {
		appFail(c, http.StatusForbidden, "NOT_CONNECTION_OWNER",
			"a personal connection may only be used by its owner")
		return conn, false
	}
	return conn, true
}

// FormNotionPublishPlan POST /apps/notion-publish/plans — derives the
// approved publish snapshot server-side (artifact version + destination),
// reads the external current version, prepares the A03 action and records
// the planned publication. The response carries the digest + fence an
// approval binds.
func (h *AppNotionPublishHandler) FormNotionPublishPlan(c *gin.Context) {
	tenantID, _, userID, ok := appTenantScope(c)
	if !ok {
		return
	}
	var input notionPublishPlanInput
	if err := c.ShouldBindJSON(&input); err != nil || input.ConnectionID == "" || input.SessionID == "" ||
		input.ArtifactVersionID == "" || input.Title == "" ||
		(input.ParentPageID == "") == (input.PageID == "") {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST",
			"connection_id, session_id, artifact_version_id, title and exactly one of parent_page_id / page_id are required")
		return
	}
	if _, ok := h.notionConnection(c, tenantID, input.ConnectionID, userID); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Notion publish pipeline is not wired in this environment; refusing to form a plan")
		return
	}
	view, err := h.publish.FormPlan(c.Request.Context(), publish.PublishPlanInput{
		TenantID: tenantID, ActorID: userID, ConnectionID: input.ConnectionID,
		SessionID: input.SessionID, ArtifactVersionID: input.ArtifactVersionID,
		Title: input.Title, ParentPageID: input.ParentPageID, PageID: input.PageID,
	})
	if err != nil {
		h.failPublish(c, err)
		return
	}
	appOK(c, http.StatusCreated, view)
}

// PublishNotionAction POST /apps/notion-publish/actions/:id/publish —
// executes the APPROVED plan through the publish service. An unobservable
// provider outcome returns 200 with the parked unknown state (mirror of
// ExecuteAction); reconciliation is the only resolution path.
func (h *AppNotionPublishHandler) PublishNotionAction(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	if _, ok := h.notionActionByID(c, tenantID, c.Param("id")); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Notion publish pipeline is not wired in this environment; refusing to dispatch")
		return
	}
	outcome, err := h.publish.Execute(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		h.failPublishExecute(c, err)
		return
	}
	if outcome.Conflict {
		appFail(c, http.StatusConflict, "PUBLISH_VERSION_CONFLICT",
			"the external document changed since approval; form a new plan from its current version")
		return
	}
	appOK(c, http.StatusOK, outcome)
}

// ReconcileNotionAction POST /apps/notion-publish/actions/:id/reconcile —
// resolves an unknown outcome by querying the provider first (AC2).
func (h *AppNotionPublishHandler) ReconcileNotionAction(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	if _, ok := h.notionActionByID(c, tenantID, c.Param("id")); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Notion publish pipeline is not wired in this environment; refusing to reconcile")
		return
	}
	outcome, err := h.publish.Reconcile(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		h.failPublishExecute(c, err)
		return
	}
	appOK(c, http.StatusOK, outcome)
}

// GetNotionPublication GET /apps/notion-publish/actions/:id — the plan +
// receipt view (计划/回执查询).
func (h *AppNotionPublishHandler) GetNotionPublication(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	if _, ok := h.notionActionByID(c, tenantID, c.Param("id")); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Notion publish pipeline is not wired in this environment")
		return
	}
	view, err := h.publish.Receipt(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		h.failPublish(c, err)
		return
	}
	appOK(c, http.StatusOK, view)
}

func (h *AppNotionPublishHandler) failPublish(c *gin.Context, err error) {
	switch {
	case errors.Is(err, publish.ErrPublishInvalidInput):
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid publish plan input")
	case errors.Is(err, publish.ErrPublishArtifactNotReady):
		appFail(c, http.StatusNotFound, "ARTIFACT_VERSION_NOT_ACCESSIBLE", "the artifact version is not readable for this session")
	case errors.Is(err, publish.ErrPublishUnsupportedArtifact):
		appFail(c, http.StatusUnsupportedMediaType, "PUBLISH_UNSUPPORTED_ARTIFACT", "only text artifacts are publishable in this version")
	case errors.Is(err, publish.ErrPublishContentTooLarge):
		appFail(c, http.StatusRequestEntityTooLarge, "PUBLISH_CONTENT_TOO_LARGE", "the artifact exceeds the publish bounds")
	case errors.Is(err, publish.ErrPublishEmptyContent):
		appFail(c, http.StatusBadRequest, "PUBLISH_EMPTY_CONTENT", "the artifact carries no publishable text")
	case errors.Is(err, publish.ErrPublishDestinationOutOfScope):
		appFail(c, http.StatusForbidden, "PUBLISH_DESTINATION_OUT_OF_SCOPE", "the destination is not in the reviewed scope for this connection")
	case errors.Is(err, publish.ErrPublishDestinationUnreadable):
		appFail(c, http.StatusBadGateway, "PUBLISH_DESTINATION_UNREADABLE", "the external destination could not be read; no plan was formed")
	case errors.Is(err, publish.ErrPublishUpdateTargetNotPublished):
		appFail(c, http.StatusConflict, "PUBLISH_UPDATE_TARGET_NOT_PUBLISHED", "only pages this workspace published through this connection may be updated")
	default:
		appFail(c, http.StatusInternalServerError, "PUBLISH_PLAN_FAILED", "failed to form the publish plan")
	}
}

func (h *AppNotionPublishHandler) failPublishExecute(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appconnectorsvc.ErrActionState):
		appFail(c, http.StatusConflict, "ACTION_STATE_CONFLICT", "action is not in the state this operation requires")
	case errors.Is(err, appconnectorsvc.ErrNoDispatcher):
		appFail(c, http.StatusServiceUnavailable, "OC_DISPATCH_NOT_CONFIGURED",
			"no outbound dispatcher is configured in this deployment; refusing to fabricate a dispatch")
	default:
		appFail(c, http.StatusInternalServerError, "PUBLISH_EXECUTE_FAILED", "publish execution failed")
	}
}
```

- [ ] **Step 4: 路由与容器接线**

创建 `internal/router/routes_app_notion_publish.go`：

```go
package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterAppNotionPublishRoutes registers the T18 Notion publish closed
// loop under /api/v1/apps/notion-publish. Like the app-connector routes,
// these are intentionally NOT declared in the API-key route authorizer:
// the /api/v1 gate default-denies every X-API-Key principal. The group
// carries the action write gate; approval authority stays on the existing
// POST /apps/actions/:id/approve predicate.
func RegisterAppNotionPublishRoutes(r *gin.RouterGroup, h *handler.AppNotionPublishHandler) {
	if h == nil {
		return
	}
	g := r.Group("/apps/notion-publish", h.RequireActionCapabilityForWrites())
	{
		g.POST("/plans", h.FormNotionPublishPlan)
		g.POST("/actions/:id/publish", h.PublishNotionAction)
		g.POST("/actions/:id/reconcile", h.ReconcileNotionAction)
		g.GET("/actions/:id", h.GetNotionPublication)
	}
}
```

修改 `internal/router/router.go`：在 `AppActionHandler       *handler.AppActionHandler`（:138）之后追加：

```go
	// T18 (#48): the Notion publish closed loop (plan formation through
	// receipt) — its own handler so the frozen action lifecycle handlers
	// stay untouched.
	AppNotionPublishHandler *handler.AppNotionPublishHandler
```

并在 `RegisterAppConnectorRoutes(v1, ...)` 调用（:421-425）之后追加一行：

```go
		RegisterAppNotionPublishRoutes(v1, params.AppNotionPublishHandler)
```

创建 `internal/container/notion_publish.go`：

```go
package container

import (
	"context"
	"fmt"
	"io"

	"github.com/Tencent/WeKnora/internal/application/repository"
	filesvc "github.com/Tencent/WeKnora/internal/application/service/file"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/modules/airesource/storageurl"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"

	"gorm.io/gorm"
)

// newNotionPublishHandler is the T18 (#48) dig constructor: it builds the
// dedicated publish ActionService (same ActionStore authority, same A02
// guard, same U05 gate; its dispatcher AND unknown resolver are the
// Notion bridge), the publish service, and the HTTP handler. The frozen
// OC-armed ActionService is untouched — the two services share the store,
// which the design names as the authority (action.go:158-160).
func newNotionPublishHandler(
	db *gorm.DB,
	store appconnectorsvc.ActionStoreSource,
	guard appconnectorsvc.A02Guard,
	gate domain.ExecutionGate,
	creds appconnectorsvc.ConnectionCredentialSource,
	files interfaces.FileService,
	tenants interfaces.TenantService,
	storage interfaces.StorageBackendResolver,
	versions *repository.ArtifactVersionStore,
) (*handler.AppNotionPublishHandler, error) {
	pubs := repoappconn.NewPublicationStore(db)
	bridge := publish.NewNotionBridge(
		publish.NewDBNotionScopeSource(db),
		publish.NewConstantNotionPolicyProvider(),
		publish.NewCredentialTokenSource(appconnectorsvc.NewCredentialResolver(creds)),
		pubs,
	)
	actions := appconnectorsvc.NewActionService(store, guard, gate, bridge, bridge)
	svc := publish.NewNotionPublishService(actions, store, pubs, versions,
		&tenantStorageArtifactContent{files: files, tenants: tenants, storage: storage}, bridge,
		publish.NewDBNotionScopeSource(db))
	h := handler.NewAppNotionPublishHandler(db)
	h.SetNotionPublishService(svc)
	return h, nil
}

// tenantStorageArtifactContent reads one artifact version's bytes through
// the same tenant storage resolution the versioned download uses
// (artifact_download.go:556-586): tenant backend resolution first, the
// global service as fallback. v1 publishes text artifacts only.
type tenantStorageArtifactContent struct {
	files   interfaces.FileService
	tenants interfaces.TenantService
	storage interfaces.StorageBackendResolver
}

func (a *tenantStorageArtifactContent) ReadArtifactContent(ctx context.Context, tenantID uint64, version repository.ArtifactVersion) ([]byte, error) {
	fileService := a.files
	if a.tenants != nil {
		tenant, err := a.tenants.GetTenantByID(ctx, tenantID)
		if err != nil || tenant == nil {
			return nil, fmt.Errorf("artifact workspace unavailable")
		}
		backendID, providerPath, scoped := types.ParseStorageBackendPath(version.ObjectKey)
		if !scoped {
			providerPath = version.ObjectKey
		}
		var ok bool
		fileService, _, ok = filesvc.ResolveTenantFileServiceWithFallback(
			ctx, "notion publish", tenant, backendID,
			types.ParseProviderScheme(providerPath), storageurl.LocalStorageBaseDir(), a.storage, a.files,
		)
		if !ok {
			return nil, fmt.Errorf("artifact storage unavailable")
		}
	}
	reader, err := fileService.GetFile(ctx, version.ObjectKey)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, publish.MaxPublishArtifactBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > publish.MaxPublishArtifactBytes {
		return nil, fmt.Errorf("%w", publish.ErrPublishContentTooLarge)
	}
	return data, nil
}
```

修改 `internal/container/container.go`：在 `must(container.Invoke(func(h *handler.AppActionHandler, s *appconnectorsvc.OCConnectionService) { h.SetOCConnectionService(s) }))`（:996-998）之后追加：

```go
	// T18 (#48): the Notion publish closed loop — dedicated ActionService
	// (bridge dispatcher + resolver), publish service and HTTP handler.
	// The frozen OC-armed action service above is untouched.
	must(container.Provide(newNotionPublishHandler))
```

- [ ] **Step 5: 运行测试确认通过 + 编译全仓**

Run: `go test ./internal/handler/ -run 'TestNotionPublishPlanGates|TestNotionPublishActionLookupIsTenantScoped' -count=1`
Expected: PASS。

Run: `go build ./...`
Expected: 无输出（router/container 接线编译通过；dig 经 `router.NewRouter` 的 `RouterParams` 自动注入 `*handler.AppNotionPublishHandler`）。

- [ ] **Step 6: 提交**

```bash
git add internal/handler/app_connector_notion_publish.go internal/handler/app_connector_notion_publish_test.go internal/router/routes_app_notion_publish.go internal/router/router.go internal/container/notion_publish.go internal/container/container.go
git commit -m "feat(http/container): /apps/notion-publish endpoints and wiring for the notion publish closed loop (T18 #48)"
```

---

### Task 8: E2E——最高稳定 Interface 证据（生产迁移 + 全链）

**前置：Task 0 必须先完成**——`openNotionPublishE2EDB` 经 golang-migrate 全量加载 `migrations/sqlite`；Task 0 修复前该加载在 HEAD 即失败（`duplicate migration file`）。

**Files:**
- Test: `internal/handler/app_connector_notion_publish_e2e_test.go`

**Interfaces:**
- Consumes: Task 3–7 全部产出 + 既有 `POST /apps/actions/:id/approve`（`app_connector_action.go:211`）+ 生产迁移链（`migrations/sqlite` 全量，`openTaskGrantDB` 同款模式，`task_grant_store_test.go:27-46`）+ `types.MCPOAuthToken`/`MCPOAuthBindingStore`（token 凭据解析）+ `file.NewLocalFileService`（真实本地文件服务读取 artifact 字节）。
- Produces: `TestNotionPublishEndToEnd*` 三条端到端证据（AC1 冲突、AC2 unknown→远端对账、AC3 全链回执）+ `TestAppPublicationsTableExistsAfterMigrations` 迁移↔投影对齐断言。

- [ ] **Step 1: 写测试（本任务只有测试——实现已由 Task 1–7 完成；若此处失败即上层缺陷）**

创建 `internal/handler/app_connector_notion_publish_e2e_test.go`：

```go
package handler

// End-to-end evidence for T18 (#48). Everything here runs on a FULLY
// MIGRATED sqlite database (the production migrations/sqlite track) with
// the REAL ActionService / PublicationStore / NotionBridge / handlers and
// the REAL approval endpoint. The ONLY replaced piece is the Notion wire
// endpoint: a local contract double (bridgeFakeNotion's handler shape,
// duplicated below as e2eNotion) implementing the official page-object
// contract with last_edited_time. This is the highest stable Interface
// evidence available without provider credentials — it is NOT the
// real-provider acceptance, which stays NOTION_TOKEN-gated (Task 9) and
// honestly SKIPs in this environment.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service/file"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---- production-migrated database ----

func openNotionPublishE2EDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "notion-publish.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance("file://"+filepath.Join(root, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

// TestAppPublicationsTableExistsAfterMigrations: the receipt table must
// be created by the PRODUCTION migrations (columns aligned with
// PublicationRow — the workbench-notifications precedent).
func TestAppPublicationsTableExistsAfterMigrations(t *testing.T) {
	db := openNotionPublishE2EDB(t)
	require.True(t, db.Migrator().HasTable("app_publications"),
		"app_publications must be created by the production migrations")
	for _, column := range []string{"tenant_id", "action_id", "connection_id", "provider", "mode",
		"destination", "expected_version", "artifact_version_id", "artifact_digest", "state",
		"external_id", "external_version", "receipt_json", "progress_json"} {
		require.True(t, db.Migrator().HasColumn("app_publications", column),
			"app_publications.%s must exist (aligned with PublicationRow)", column)
	}
}

// ---- the contract double for the Notion wire (same shape as Task 5's
// bridgeFakeNotion, self-contained here for the HTTP-level flow) ----

type e2eNotion struct {
	mu             sync.Mutex
	token          string
	pages          map[string]*e2ePage
	nextID         int
	patchCalls     int
	appendCalls    int
	dropNextAppend bool
}

type e2ePage struct {
	id, parent, title, lastEdited string
	children                      []json.RawMessage
}

func newE2ENotion(token string) *e2eNotion {
	return &e2eNotion{token: token, pages: map[string]*e2ePage{}}
}

func (e *e2eNotion) lock()   { e.mu.Lock() }
func (e *e2eNotion) unlock() { e.mu.Unlock() }

func (e *e2eNotion) addPage(id, parent, title string) {
	e.lock()
	e.pages[id] = &e2ePage{id: id, parent: parent, title: title, lastEdited: "2026-09-24T08:00:00.000Z"}
	e.unlock()
}

func (e *e2eNotion) touch(id, when string) {
	e.lock()
	if p, ok := e.pages[id]; ok {
		p.lastEdited = when
	}
	e.unlock()
}

func (e *e2eNotion) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
	auth := func(r *http.Request) bool { return r.Header.Get("Authorization") == "Bearer "+e.token }
	mux.HandleFunc("/v1/pages", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Parent struct {
				PageID string `json:"page_id"`
			} `json:"parent"`
			Properties struct {
				Title struct {
					Title []struct {
						Text struct {
							Content string `json:"content"`
						} `json:"text"`
					} `json:"title"`
				} `json:"title"`
			} `json:"properties"`
		}
		_ = json.Unmarshal(body, &req)
		e.lock()
		e.nextID++
		id := fmt.Sprintf("page-%d", e.nextID)
		e.pages[id] = &e2ePage{id: id, parent: req.Parent.PageID, title: req.Properties.Title.Title[0].Text.Content, lastEdited: "2026-09-24T09:00:00.000Z"}
		e.unlock()
		writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":"2026-09-24T09:00:00.000Z","parent":{"type":"page_id","page_id":%q}}`, id, req.Parent.PageID))
	})
	mux.HandleFunc("/v1/pages/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		id := r.URL.Path[len("/v1/pages/"):]
		e.lock()
		p, ok := e.pages[id]
		le := ""
		if ok {
			le = p.lastEdited
		}
		e.unlock()
		if !ok {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		switch r.Method {
		case http.MethodGet:
			e.lock()
			le = p.lastEdited
			e.unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":%q,"parent":{"type":"page_id","page_id":%q}}`, p.id, le, p.parent))
		case http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Properties struct {
					Title struct {
						Title []struct {
							Text struct {
								Content string `json:"content"`
							} `json:"text"`
						} `json:"title"`
					} `json:"title"`
				} `json:"properties"`
			}
			_ = json.Unmarshal(body, &req)
			e.lock()
			e.patchCalls++
			p.title = req.Properties.Title.Title[0].Text.Content
			p.lastEdited = "2026-09-24T11:00:00.000Z"
			le = p.lastEdited
			e.unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":%q,"parent":{"type":"page_id","page_id":%q}}`, p.id, le, p.parent))
		default:
			writeJSON(w, 405, `{"object":"error","status":405,"code":"method_not_allowed","message":"x"}`)
		}
	})
	mux.HandleFunc("/v1/blocks/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		rest := r.URL.Path[len("/v1/blocks/"):]
		if !strings.HasSuffix(rest, "/children") {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		id := strings.TrimSuffix(rest, "/children")
		e.lock()
		p, ok := e.pages[id]
		e.unlock()
		if !ok {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		switch r.Method {
		case http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Children []json.RawMessage `json:"children"`
			}
			_ = json.Unmarshal(body, &req)
			e.lock()
			e.appendCalls++
			p.children = append(p.children, req.Children...)
			p.lastEdited = "2026-09-24T12:00:00.000Z"
			drop := e.dropNextAppend
			if drop {
				e.dropNextAppend = false
			}
			results := append([]json.RawMessage(nil), p.children...)
			e.unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, fmt.Sprintf(`{"object":"list","results":[%s]`, strings.Join(rawStrings(results), ","))+"]}")
		case http.MethodGet:
			e.lock()
			results := append([]json.RawMessage(nil), p.children...)
			e.unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"list","results":[%s]`, strings.Join(rawStrings(results), ","))+"]}")
		default:
			writeJSON(w, 405, `{"object":"error","status":405,"code":"method_not_allowed","message":"x"}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func rawStrings(items []json.RawMessage) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = string(it)
	}
	return out
}

// ---- the full-stack environment ----

type notionE2EEnv struct {
	engine *gin.Engine
	db     *gorm.DB
	fake   *e2eNotion
}

func newNotionPublishE2E(t *testing.T) *notionE2EEnv {
	t.Helper()
	db := openNotionPublishE2EDB(t)
	// Tenants / users / membership (column sets verified against the
	// sqlite migration track: tenants/users/sessions mirror the
	// task-grant fixtures; tenant_members columns per 000000_init.up.sql).
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (7, 't7', 'test')`).Error)
	for _, u := range []string{"user-a", "user-b"} {
		require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, 'x', 7)`, u, u, u+"@example.test").Error)
		require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES (7, ?, 'contributor', 'active')`, u).Error)
	}
	// Task = Session (ADR-0004) + an admitted run for the artifact version
	// binding (agent_runs has many NOT NULL columns — seed through the
	// real store, exactly like the #42 collaboration fixtures).
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('sess-1', 7, 'task-1', 'user-a', 'trpc')`).Error)
	_, err := repository.NewAgentRunStore(db).Admit(context.Background(), agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 7, RunID: "run-1"}, SessionID: "sess-1", UserID: "user-a",
		RequestID: "q1", AssistantMessageID: "a1", RequestHash: "hash-1",
		Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	// The confirmed artifact version (columns per 000067_artifact_versions)
	// + its real bytes on a REAL local file service (the production
	// content-reader path minus tenant backend routing).
	baseDir := t.TempDir()
	digest := strings.Repeat("a", 64)
	objectKey := fmt.Sprintf("artifact-versions/7/run-1/%s", digest)
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, filepath.Dir(objectKey)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, objectKey), []byte("第一段。\n\n第二段。"), 0o644))
	require.NoError(t, db.Exec(`INSERT INTO artifact_versions (tenant_id, id, run_id, session_id, digest, object_key, mime, scan_state, size)
		VALUES (7, 'ver-1', 'run-1', 'sess-1', ?, ?, 'text/plain', 'ready', 15)`, digest, objectKey).Error)
	// Notion installation + reviewed app version (scope + approved
	// parents) + user-a's personal connection + its token row (principal
	// columns per 000011_principal_model).
	require.NoError(t, db.Exec(`INSERT INTO installations (id, tenant_id, app_id, app_version, state, version) VALUES ('inst-notion', 7, 'notion', 'v1', 'active', 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('notion', 'v1', '{"scopes":["insert_content"],"approved_parents":["parent-1"]}', '{}')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO connections (tenant_id, id, installation_id, kind, owner_id, credential_ref, state, auth_version)
		VALUES (7, 'conn-notion', 'inst-notion', 'personal', 'user-a', 'mcp_oauth_token:notion', 'active', 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mcp_oauth_tokens (id, tenant_id, user_id, principal_type, principal_id, service_id, access_token, token_type, expires_at)
		VALUES ('tok-1', 7, 'user-a', 'web_user', 'user-a', 'notion', 'secret_test_token', 'bearer', '2099-01-01 00:00:00')`).Error)
	// The Notion double: the pre-approved parent page exists remotely.
	fake := newE2ENotion("secret_test_token")
	fake.addPage("parent-1", "root", "Workspace")
	srv := fake.server(t)

	// The production composition, with the loopback policy provider
	// (the documented test hook) replacing the pinned api.notion.com one.
	pubs := repoappconn.NewPublicationStore(db)
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	_, loopback, _ := net.ParseCIDR("127.0.0.0/8")
	pol := appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PATCH"}, PathPrefix: "/v1/",
		AuthorizedNetworks: []*net.IPNet{loopback}, Timeout: 10 * time.Second,
	}
	scopeSrc := publish.NewDBNotionScopeSource(db)
	bridge := publish.NewNotionBridge(scopeSrc, &e2ePolicyProvider{pol: pol},
		publish.NewCredentialTokenSource(appconnectorsvc.NewCredentialResolver(repository.NewMCPOAuthBindingStore(db))), pubs)
	store := repoappconn.NewActionStore(db)
	publishActions := appconnectorsvc.NewActionService(store, &e2ePassGuard{}, nil, bridge, bridge)
	content := &e2eLocalContent{svc: file.NewLocalFileService(baseDir, "")}
	svc := publish.NewNotionPublishService(publishActions, store, pubs, repository.NewArtifactVersionStore(db), content, bridge, scopeSrc)

	publishHandler := NewAppNotionPublishHandler(db)
	publishHandler.SetNotionPublishService(svc)
	actionHandler := NewAppActionHandler(db)
	actionHandler.SetActionService(publishActions)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		ctx := c.Request.Context()
		user := "user-a"
		if c.GetHeader("X-Test-User") != "" {
			user = c.GetHeader("X-Test-User")
		}
		role := types.TenantRoleAdmin
		if c.GetHeader("X-Test-Role") != "" {
			role = types.TenantRole(c.GetHeader("X-Test-Role"))
		}
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, user)
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	v1 := engine.Group("/api/v1")
	// The EXISTING approval endpoint — approval authority stays there.
	v1.POST("/apps/actions/:id/approve", actionHandler.ApproveAction)
	// Mirror of RegisterAppNotionPublishRoutes (internal/router) — the
	// router package is not importable from this handler-package test.
	g := v1.Group("/apps/notion-publish", publishHandler.RequireActionCapabilityForWrites())
	{
		g.POST("/plans", publishHandler.FormNotionPublishPlan)
		g.POST("/actions/:id/publish", publishHandler.PublishNotionAction)
		g.POST("/actions/:id/reconcile", publishHandler.ReconcileNotionAction)
		g.GET("/actions/:id", publishHandler.GetNotionPublication)
	}
	return &notionE2EEnv{engine: engine, db: db, fake: fake}
}

type e2ePolicyProvider struct{ pol appconn.HTTPPolicy }

func (p *e2ePolicyProvider) PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error) {
	return p.pol, nil
}

type e2ePassGuard struct{}

func (e2ePassGuard) Check(ctx context.Context, subject appconn.OCSubject, connectionID string, expectedVersion int64) error {
	return nil
}

type e2eLocalContent struct{ svc interfaces.FileService }

func (e *e2eLocalContent) ReadArtifactContent(ctx context.Context, tenantID uint64, version repository.ArtifactVersion) ([]byte, error) {
	reader, err := e.svc.GetFile(ctx, version.ObjectKey)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (e *notionE2EEnv) do(t *testing.T, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader = strings.NewReader("")
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func (e *notionE2EEnv) formPlan(t *testing.T, body string) map[string]any {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/apps/notion-publish/plans", body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var parsed struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parsed))
	return parsed.Data
}

func (e *notionE2EEnv) approve(t *testing.T, plan map[string]any) {
	t.Helper()
	// Route through the EXISTING approval endpoint: approval authority
	// stays on the frozen ApproveAction predicate.
	body := fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, plan["digest"], int(plan["expected_version"].(float64)))
	w := e.do(t, http.MethodPost, "/api/v1/apps/actions/"+plan["action_id"].(string)+"/approve", body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// TestNotionPublishEndToEndCreateApprovePublishReceipt: the full AC3
// chain — form (artifact version + destination, server-derived snapshot,
// external baseline read) → approve on the EXISTING endpoint → publish →
// receipt with external id + version; the migration-aligned publication
// row lands 'published'.
func TestNotionPublishEndToEndCreateApprovePublishReceipt(t *testing.T) {
	env := newNotionPublishE2E(t)
	plan := env.formPlan(t, `{"connection_id":"conn-notion","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report","parent_page_id":"parent-1"}`)
	require.Equal(t, "create", plan["mode"])
	require.Equal(t, "parent-1", plan["destination"])
	require.NotEmpty(t, plan["expected_external_version"], "AC1 baseline: the plan read the external version")
	require.NotEmpty(t, plan["digest"])

	env.approve(t, plan)

	w := env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+plan["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Data struct {
			ActionState string `json:"action_state"`
			Publication struct {
				State           string `json:"state"`
				ExternalID      string `json:"external_id"`
				ExternalVersion string `json:"external_version"`
				ArtifactVersionID string `json:"artifact_version_id"`
			} `json:"publication"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "succeeded", out.Data.ActionState)
	require.Equal(t, "published", out.Data.Publication.State)
	require.NotEmpty(t, out.Data.Publication.ExternalID)
	require.NotEmpty(t, out.Data.Publication.ExternalVersion, "the receipt must save the external version")
	require.Equal(t, "ver-1", out.Data.Publication.ArtifactVersionID)

	// The receipt is durable and queryable through the GET endpoint.
	w = env.do(t, http.MethodGet, "/api/v1/apps/notion-publish/actions/"+plan["action_id"].(string), "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"external_version"`)

	// The published page exists remotely with the derived blocks.
	env.fake.lock()
	created := env.fake.pages[out.Data.Publication.ExternalID]
	env.fake.unlock()
	require.NotNil(t, created)
	require.Len(t, created.children, 2, "two paragraphs derived from the artifact text")
}

// TestNotionPublishEndToEndUpdateConflict: AC1 — an external edit between
// plan formation and publish is detected at execute time and the publish
// is refused with 409 and ZERO write requests.
func TestNotionPublishEndToEndUpdateConflict(t *testing.T) {
	env := newNotionPublishE2E(t)
	// First publish to mint a receipt-backed target page.
	plan1 := env.formPlan(t, `{"connection_id":"conn-notion","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report","parent_page_id":"parent-1"}`)
	env.approve(t, plan1)
	w := env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+plan1["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out1 struct {
		Data struct {
			Publication struct {
				ExternalID string `json:"external_id"`
			} `json:"publication"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out1))
	pageID := out1.Data.Publication.ExternalID

	// The update plan reads the page's CURRENT version.
	plan2 := env.formPlan(t, fmt.Sprintf(`{"connection_id":"conn-notion","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report v2","page_id":%q}`, pageID))
	require.Equal(t, "update", plan2["mode"])
	env.approve(t, plan2)

	// External collaborator edits between approval and publish.
	env.fake.lock()
	patchesBefore := env.fake.patchCalls
	appendsBefore := env.fake.appendCalls
	env.fake.unlock()
	env.fake.touch(pageID, "2026-09-24T10:30:00.000Z")

	w = env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "PUBLISH_VERSION_CONFLICT")
	env.fake.lock()
	patchesAfter := env.fake.patchCalls
	appendsAfter := env.fake.appendCalls
	env.fake.unlock()
	require.Equal(t, patchesBefore, patchesAfter, "AC1: conflict leaves ZERO page writes")
	require.Equal(t, appendsBefore, appendsAfter, "AC1: conflict leaves ZERO block writes")
}

// TestNotionPublishEndToEndUnknownReconcilesRemoteFirst: AC2 — a lost
// write reply parks the action unknown; a second publish is structurally
// refused; reconcile reads the REMOTE first and settles the receipt, with
// no re-dispatch ever.
func TestNotionPublishEndToEndUnknownReconcilesRemoteFirst(t *testing.T) {
	env := newNotionPublishE2E(t)
	plan1 := env.formPlan(t, `{"connection_id":"conn-notion","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report","parent_page_id":"parent-1"}`)
	env.approve(t, plan1)
	w := env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+plan1["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out1 struct {
		Data struct {
			Publication struct {
				ExternalID string `json:"external_id"`
			} `json:"publication"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out1))
	pageID := out1.Data.Publication.ExternalID

	// The update plan; the append's reply is lost after the effect applied.
	plan2 := env.formPlan(t, fmt.Sprintf(`{"connection_id":"conn-notion","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report v2","page_id":%q}`, pageID))
	env.approve(t, plan2)
	env.fake.lock()
	env.fake.dropNextAppend = true
	appendsBefore := env.fake.appendCalls
	env.fake.unlock()

	w = env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"unknown"`, "an unobservable outcome must park unknown, never fabricate: %s", w.Body.String())

	// Blind re-publish is refused by the store (claim only from
	// authorized); the double's write count stays put.
	w = env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	env.fake.lock()
	require.Equal(t, appendsBefore+1, env.fake.appendCalls, "the dropped append counted once; NO re-dispatch happened")
	appendsAfterUnknown := env.fake.appendCalls
	env.fake.unlock()

	// Reconcile: provider query FIRST — the effect applied remotely, so
	// the query settles success and the receipt lands published.
	w = env.do(t, http.MethodPost, "/api/v1/apps/notion-publish/actions/"+plan2["action_id"].(string)+"/reconcile", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"succeeded"`, w.Body.String())
	require.Contains(t, w.Body.String(), `"state":"published"`, w.Body.String())
	env.fake.lock()
	require.Equal(t, appendsAfterUnknown, env.fake.appendCalls, "reconcile must not re-send")
	env.fake.unlock()
}
```

- [ ] **Step 2: 运行测试确认通过（E2E 即验证；若失败按失败点回溯对应任务的实现）**

Run: `go test ./internal/handler/ -run 'TestNotionPublishEndToEnd|TestAppPublicationsTableExistsAfterMigrations' -count=1 -v`
Expected: PASS（4 个测试）。

- [ ] **Step 3: 全量回归**

Run: `go test ./internal/handler/ -run 'NotionPublish' -count=1 && go test ./internal/modules/appconnector/... -count=1`
Expected: PASS。

- [ ] **Step 4: 提交**

```bash
git add internal/handler/app_connector_notion_publish_e2e_test.go
git commit -m "test(http): notion publish end-to-end evidence on production migrations - conflict, unknown-reconcile, receipt (T18 #48 AC1-AC3)"
```

---

### Task 9: 真实受控集成证据（blocked-env，opt-in）

**Files:**
- Create: `internal/modules/appconnector/notion_publish_real_test.go`

**Interfaces:**
- Consumes: `NotionCreateAdapter`/`NotionUpdateAdapter`/`ParseNotionCreateSnapshot` 快照构造（既有）+ Task 1/2 产出（`NotionUpdateSnapshot`/`ParseNotionPageReceipt`/`DetectNotionVersionConflict`）。
- Produces: `TestNotionRealPublishLoop`（NOTION_TOKEN/NOTION_PARENT_PAGE_ID 门控；无凭据 SKIP 且明示 blocked-env——与 `notion_create_real_test.go:16-22` 同纪律）。

- [ ] **Step 1: 写测试**

创建 `internal/modules/appconnector/notion_publish_real_test.go`：

```go
package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// Real-provider controlled evidence for the publish loop (NO-04
// discipline): the user designates a test parent page (shared with a
// one-off integration) and its token. The REAL adapters run the full
// closed loop against api.notion.com: create (baseline version read) →
// receipt → update against the SAME page (version read + conflict-free
// write) → a stale-version update must be REFUSED with zero writes.
// Gated on NOTION_TOKEN/NOTION_PARENT_PAGE_ID; a skip is never a pass.
func TestNotionRealPublishLoop(t *testing.T) {
	token := os.Getenv("NOTION_TOKEN")
	parent := os.Getenv("NOTION_PARENT_PAGE_ID")
	if token == "" || parent == "" || strings.HasPrefix(parent, "xxxx") {
		t.Skip("notion real credentials not configured (NOTION_TOKEN/NOTION_PARENT_PAGE_ID in artifacts/connector-real/notion.env); skip is not a pass — T18 real-provider evidence stays blocked-env")
	}
	progress := map[string]NotionPageProgress{}
	caps := func(ctx context.Context, a Action) ([]string, error) { return []string{NotionCapabilityInsert}, nil }
	pol := HTTPPolicy{
		Scheme:     "https",
		Host:       NotionAPIHost,
		Methods:    []string{"GET", "POST", "PATCH"},
		PathPrefix: "/v1/",
		Timeout:    30 * time.Second,
	}
	create := &NotionCreateAdapter{
		Policy: pol,
		Token:  func(ctx context.Context) (string, error) { return token, nil },
		ApprovedParents:        []string{parent},
		ConnectionCapabilities: caps,
		LoadProgress:           func(a Action) NotionPageProgress { return progress[a.ID] },
		SaveProgress:           func(a Action, p NotionPageProgress) error { progress[a.ID] = p; return nil },
	}
	stamp := time.Now().Format("15:04:05")
	block := map[string]any{
		"object": "block", "type": "paragraph",
		"paragraph": map[string]any{"rich_text": []any{map[string]any{
			"type": "text", "text": map[string]string{"content": "t18 real publish " + stamp},
		}}},
	}
	createArgs, _ := json.Marshal(map[string]any{"parent": parent, "title": "T18 real " + stamp, "blocks": []any{block}})
	createAction := Action{ID: "act_t18_real_1", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t18",
		Version: "notion/v1", Target: parent, Risk: RiskWrite, Args: createArgs}

	out, err := create.Execute(context.Background(), createAction)
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("real create: state=%s err=%v", out.State, err)
	}
	pageID := out.ExternalID
	rcpt, rerr := ParseNotionPageReceipt(out.Output)
	if rerr != nil || rcpt.ExternalID != pageID || rcpt.ExternalVersion == "" {
		t.Fatalf("real create receipt: %+v %v", rcpt, rerr)
	}
	t.Logf("REAL page created: id=%s version=%s", pageID, rcpt.ExternalVersion)

	// Update against the SAME page: read its current version, publish a
	// new block under it.
	update := &NotionUpdateAdapter{
		Policy: pol,
		Token:  func(ctx context.Context) (string, error) { return token, nil },
		ConnectionCapabilities: caps,
		LoadProgress:           func(a Action) NotionPageProgress { return progress[a.ID] },
		SaveProgress:           func(a Action, p NotionPageProgress) error { progress[a.ID] = p; return nil },
	}
	current := rcpt.ExternalVersion
	// The create's own append bumped the version; re-read the live value
	// instead of assuming.
	reader := &NotionUpdateAdapter{Policy: pol, Token: func(ctx context.Context) (string, error) { return token, nil }}
	live, lerr := ReadNotionPageVersion(context.Background(), reader.Policy, reader.Token, pageID)
	if lerr != nil {
		t.Fatalf("real version pre-read: %v", lerr)
	}
	current = live
	updateBlock := map[string]any{
		"object": "block", "type": "paragraph",
		"paragraph": map[string]any{"rich_text": []any{map[string]any{
			"type": "text", "text": map[string]string{"content": "t18 real update " + stamp},
		}}},
	}
	updateArgs, _ := json.Marshal(map[string]any{
		"page_id": pageID, "expected_version": current,
		"title": "T18 real " + stamp, "blocks": []any{updateBlock},
	})
	updateAction := Action{ID: "act_t18_real_2", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t18",
		Version: "notion/v1", Target: pageID, Risk: RiskWrite, Args: updateArgs}
	uout, uerr := update.Execute(context.Background(), updateAction)
	if uerr != nil || uout.State != ActionSucceeded {
		t.Fatalf("real update: state=%s err=%v", uout.State, uerr)
	}
	urcpt, urerr := ParseNotionPageReceipt(uout.Output)
	if urerr != nil || urcpt.ExternalID != pageID || urcpt.ExternalVersion == current {
		t.Fatalf("real update receipt must carry a NEW version: %+v (had %s)", urcpt, current)
	}
	t.Logf("REAL page updated: id=%s version %s -> %s", pageID, current, urcpt.ExternalVersion)

	// Stale-version update: a conflict must be refused BEFORE any write.
	staleArgs, _ := json.Marshal(map[string]any{
		"page_id": pageID, "expected_version": current, // superseded by the update above
		"title": "T18 stale " + stamp, "blocks": []any{updateBlock},
	})
	staleAction := Action{ID: "act_t18_real_3", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t18",
		Version: "notion/v1", Target: pageID, Risk: RiskWrite, Args: staleArgs}
	sout, serr := update.Execute(context.Background(), staleAction)
	if !errors.Is(serr, ErrNotionVersionConflict) || sout.State != ActionFailed {
		t.Fatalf("real stale update must conflict: state=%s err=%v", sout.State, serr)
	}
	t.Logf("REAL stale update refused with zero writes (conflict on %s)", current)
}
```

- [ ] **Step 2: 本地实跑（必须 SKIP，且 SKIP 文案明示 blocked-env）**

Run: `NOTION_TOKEN=x NOTION_PARENT_PAGE_ID=xxxx-skip go test ./internal/modules/appconnector/ -run TestNotionRealPublishLoop -count=1 -v`
Expected: `--- SKIP: TestNotionRealPublishLoop`（文案含 blocked-env；**不得**为通过而放宽门控）。无凭据直接运行同样 SKIP。

- [ ] **Step 3: 计划级验证**

Run（worktree 根，覆盖本计划全部测试，含 Task 0 的迁移链修复证据）:

```bash
go build ./... && go test ./internal/application/repository/ -run 'TestWorkbenchNotificationsTableExistsAfterMigrations|TestTaskCollaborationEndToEnd' -count=1 && go test ./internal/modules/appconnector/... -count=1 && go test ./internal/handler/ -run 'NotionPublish|AppPublications' -count=1
```

Expected: `go build` 无输出；三个 `go test` 全部 ok（`TestNotionRealControlledCreate`/`TestNotionRealPublishLoop` 无凭据 SKIP 属预期——skip is not a pass，真实验收 blocked-env）。

- [ ] **Step 4: 提交**

```bash
git add internal/modules/appconnector/notion_publish_real_test.go
git commit -m "test(appconnector): real-provider publish loop evidence, credential-gated (blocked-env without NOTION_TOKEN) (T18 #48)"
```

---

## 验收标准覆盖对照

| 验收标准 | 覆盖任务 | 证据 |
|---|---|---|
| 1. 发布前读取外部当前版本并检测冲突 | Task 1（`DetectNotionVersionConflict`/`ParseNotionPageVersion`）、Task 2（执行前预读+零写拒绝）、Task 6（计划形成绑定 expected_version）、Task 8（E2E 409 + 零写断言）、Task 9（真实 stale 拒绝，blocked-env） |
| 2. 超时和未知结果先核对远端，不盲重试 | Task 2（丢响应→unknown；Query 远端对账）、Task 5（bridge unknown 映射 + QueryProvider）、Task 6（`Reconcile` 先查 Provider；重执行被 store 结构性拒绝）、Task 8（E2E：二次 publish 409 + 写计数不增 + reconcile 成功）、Task 9 |
| 3. 端到端行为通过最高稳定 Interface 验证；mock 不冒充真实集成证据 | Task 0（预存在迁移链修复——Task 8 全量迁移加载的前置）+ Task 8（生产迁移库 + 真实 ActionService/Store/Bridge/handler + 既有审批端点全链，仅 Notion 网点为明示契约双打）；Task 9（真实 Provider 证据，blocked-env 如实 SKIP）；Task 3（迁移↔投影对齐断言）；「与调查结论的差异记录」与 Task 8 文件头明示双打不冒充真实验收 |

## 附：Consumes-Produces 摘要（供 #49/#50/#51 与审查者）

- **Consumes（前置批次产出，当前 HEAD 亲眼核实）**：#46 的 `repository.ArtifactVersionStore.ReadableArtifactVersion`（`artifact_version.go:261`，tenant+session+ready 谓词）与不可变版本行；#38 的 `POST /apps/actions/:id/approve` 审批谓词（`app_connector_action.go:211`）；既有 A03 管线 `ActionService`/`ActionStore`（`service/appconnector/action.go`、`repository/appconnector/action.go`）；既有 NO-01/NO-03 契约 `NotionCreateAdapter`（`notion_create.go`，本计划零修改）；既有 A04 `HTTPPolicy`（`http_policy.go`）与 A02 凭据解析 `CredentialResolver`/`MCPOAuthBindingStore`（`credentials.go:79`、`mcp_oauth.go:494-531`）。
- **Produces**：`internal/modules/appconnector/notion_update.go`（`NotionUpdateSnapshot`/`ParseNotionUpdateSnapshot`/`IsNotionUpdateArgs`/`DetectNotionVersionConflict`/`NotionPageVersion`/`ParseNotionPageVersion`/`NotionPageReceipt`/`ParseNotionPageReceipt`/`NotionUpdateAdapter`/`ReadNotionPageVersion`）；`repository/appconnector/publication.go`（`PublicationRow`/`PublicationStore`：Create/FindByAction/SaveProgress/SettlePublication/LatestPublishedByDestination，状态机 planned→{published,failed,unknown}、unknown→{published,failed}）；迁移 000193（versioned）/000114（sqlite）`app_publications`；`internal/modules/appconnector/publish`（`NotionParagraphBlocks`、`NotionBridge`（ActionDispatcher+UnknownResolver+ReadPageVersion）、`NewDBNotionScopeSource`/`NewConstantNotionPolicyProvider`/`NewCredentialTokenSource`、`NotionPublishService.FormPlan/Execute/Reconcile/Receipt` 及全部哨兵错误与视图类型）；HTTP `POST /apps/notion-publish/plans`、`POST .../actions/:id/publish`、`POST .../actions/:id/reconcile`、`GET .../actions/:id`；容器 `newNotionPublishHandler`。
