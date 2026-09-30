# T20：Confluence 页面发布端到端闭环（Issue #50）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 #48 为 Notion 建立的发布 seam 复用到 Confluence：从确定版本的 Task Artifact 形成 Confluence 页面发布 Action Plan，走既有 A03 审批闭环后在租户自己的 Confluence 实例（Cloud v2 / Server REST 双版式）创建/更新页面——执行前重读外部 `version.number` 并检测冲突（AC1：旧 Artifact 不静默覆盖外部新版本）、失败与未知结果只经远端对账绝不盲重试（AC2：失败和重试行为与统一 Action 语义一致）、端到端行为在生产迁移库 + 真实 HTTP 处理器链路上以最高稳定 Interface 验证并保存外部版本与回执（AC3）。

**Architecture:** 仓库已有冻结的 A03 审批管线（`internal/modules/appconnector/service/appconnector/action.go:192` 的 `ActionService`：Prepare→Approve→Execute→ResolveUnknown，store 为权威）与 #48 落地的发布 seam：`internal/modules/appconnector/publish/`（blocks/dispatcher/plan 三文件）、`app_publications` 回执表（`internal/modules/appconnector/repository/appconnector/publication.go:66`）、Notion HTTP 面（`internal/handler/app_connector_notion_publish.go`、`internal/router/routes_app_notion_publish.go`、`internal/container/notion_publish.go`）与第二个 `ActionService` 实例的装配模式（`internal/container/container.go:1013`）。现状缺口（作者逐文件核实）：知识库 Confluence 连接器（`internal/modules/datasource/connector/confluence/client.go:51`）只有 `GET`（ping/spaces/pages/body/paginate），`rg "MethodPost|MethodPut|MethodPatch" internal/modules/datasource/connector/confluence/` 零命中——**写能力不存在**。本计划补五块，全部为新增文件（`notion_create.go`/`notion_update.go`/`publish/plan.go` 等既有文件零修改）：①`appconnector` 包新增 **Confluence 适配器族**（`confluence_common.go`：Basic 凭据 JSON、双版式端点解析、页面版本/回执解析、冲突判定；`confluence_create.go`：三字段快照 + 单步创建适配器；`confluence_update.go`：四字段快照（含 `expected_version`）+ 版本预读→冲突拒绝→单步 PUT→Query 对账）；②`publish` 包新增 **ConfluenceBridge**（把适配器族桥接为 `ActionDispatcher`+`UnknownResolver`，按快照形状路由 create/update，凭据/出站策略/审批作用域三个端口注入）与 **ConfluencePublishService**（复用 `PublishPlanInput`/`PublishPlanView`/`PublishReceiptView`；Artifact 不可变版本→Confluence storage 正文→快照→`ActionService.Prepare`；Execute/Reconcile 后按行动行权威状态 settle 回执）；③HTTP 面 `/api/v1/apps/confluence-publish/*` + 容器接线（发布链使用**第三个 `ActionService` 实例**，同一 `ActionStore` 为权威，不改冻结的 `newOCArmedActionService` 与 #48 的 Notion 发布实例）；④审批复用既有 `POST /apps/actions/:id/approve`；⑤真实受控集成证据（blocked-env，opt-in）。**本计划零迁移、零 TS 改动**（`app_publications.provider` 是字符串列，直接落 `"confluence"`；移动端对发布 Action Plan 的组合消费属 #51）。

**Tech Stack:** Go 1.26（gin + gorm + golang-migrate + testify + httptest），单模块 `github.com/Tencent/WeKnora`（go.mod:1），全部命令在 worktree 根执行。作者实跑基线（2026-09-26，本 worktree HEAD `d52270a0f`）：`go build ./...` 通过；`go test ./internal/modules/appconnector/... -count=1` 6 个包全部 ok；`go test ./internal/handler/ -run 'TestNotionPublishEndToEnd|TestAppPublicationsTable' -count=1` ok（生产 sqlite 迁移全量可装载）。**迁移轨道预警**：作者在会话开始时实跑该 E2E 曾 FAIL（`duplicate migration file: 000114_public_agent_marketplace.down.sql`——sqlite `000114` 与 versioned `000193` 被 `public_agent_marketplace`（#60）与 `mobile_device_app`（#67）双占）；撰写期间该双占已由并行会话在 worktree 内以「`mobile_device_app` 后落顺延至 sqlite 000118 / versioned 000197（+5 个钉住路径的测试文件同步改串）」修复（工作区未提交状态，`git status` 显示 4 个 R 重命名 + 5 个测试文件 M）。执行工作树若从含修复的基线切出则 Task 0 为纯验证；若从双占基线切出则 Task 0 给出的就是这套已验证的重编配方（见差异记录第 1 条与 Task 0）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-50.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（User Story 34；Implementation Decisions；Testing Decisions——尤其「Go integration tests verify authorization, Tenant/Owner/Collaborator predicates, revision and digest CAS, idempotency, partial external success, unknown outcomes and durable checkpoints.」；Out of Scope 的双向同步排除）
- 领域术语：`CONTEXT.md`（342 行）——「**外部发布（External Publication）**：通过获准的连接，把确定版本的任务产物创建或更新到外部办公系统的操作，并保存获批版本、外部目标、目标版本与操作结果作为回执。发布后外部文档是后续协作的权威版本；再次更新前必须读取外部当前版本并形成新的候选变更。」、「操作计划（Action Plan）」、「任务产物（Task Artifact）」（不可原地覆盖）、「任务协作者」（不授予批准外部副作用权限）
- ADR（按需）：`docs/adr/0004-task-is-session.md`（Task=Session）、`docs/adr/0009-cloud-data-trust-boundary.md`（凭据/密文边界）、`docs/adr/0014-commercial-platform-single-deep-seam.md`（U05 预算门）
- Parent：Issue #30；Blocked by：#48（**已交付并在当前 HEAD 亲眼核实**：`publish.NotionPublishService`/`NotionBridge`、`PublicationStore`、`/apps/notion-publish` 路由与容器装配、E2E 三测全绿）
- 下游（本计划 Produces 供其消费）：#51 多操作 Action Plan（复用单操作计划记录与 confluence 回执行）、#71 首版发布证据（本计划的 blocked-env 声明如实进入其证据汇总）

## Global Constraints

以下为批准 Spec / CONTEXT.md / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「旧 Artifact 不静默覆盖外部新版本。」（Issue #50 验收标准 1 原文）
- 「失败和重试行为与统一 Action 语义一致。」（Issue #50 验收标准 2 原文）
- 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」（Issue #50 验收标准 3 原文）
- 「Internal Artifact versions coexist with external office documents. After publication, the external document is the collaboration authority; later updates read its current version first.」（mobile-ai-office-design.md · Implementation Decisions）
- 「The first office write Adapters are Feishu, Notion and Confluence. Existing read/sync capability does not imply write permission.」（同上）——知识库只读连接器（`internal/modules/datasource/connector/confluence/`）的存在不授予任何写权限；本计划的写适配器族独立于该连接器，且不改动它的任何文件。
- 「Lead Agent Version, Artifact versions, Action Plans and candidate code commits are immutable approval anchors. Changes invalidate prior approvals.」（同上）——发布快照（含 `expected_version`）在 Prepare 时 Normalize+Digest（`internal/modules/appconnector/action.go:104` 的 `NormalizeArgs`、`:141` 的 `ActionDigest`），任何变化产生新 digest，旧批准失效。
- 「Go integration tests verify authorization, Tenant/Owner/Collaborator predicates, revision and digest CAS, idempotency, partial external success, unknown outcomes and durable checkpoints.」（同上 · Testing Decisions）
- 「Tests target observable behavior at the highest stable Interface.」（同上）
- 「Offline mode permits approved reads, drafts and annotations. It prohibits Run commands, approval, budget expansion and external Actions.」（同上）——本计划纯服务端，不引入任何移动端离线面。
- Out of Scope 逐字：「Bidirectional real-time synchronization between WeKnora Artifacts and external office documents.」（同上）与 CONTEXT.md「_避免_：内置 Office 编辑器、未经审批的自动同步、WeKnora 与外部文档双向实时同步、把外部写入成功等同于任务产物本身。」——本计划只做单向外发 + 回执，不做同步；「后续修改以外部页面为协作权威」（Issue #50 正文）落实为：每次更新都必须从新的外部当前版本重新形成计划（旧 `expected_version` 一律失效）。
- CONTEXT.md「任务产物（Task Artifact）」：「……但不能原地覆盖已存在或已审批的版本。」——发布不修改内部 Artifact 版本行；回执只追加。
- CONTEXT.md「任务协作者」：「协作访问必须显式授予……不会授予使用任务所有者个人连接或批准其外部副作用的权限。」与「任务所有者」：「负责涉及其个人连接……或其他外部副作用的授权。」——发布计划形成沿用 `PrepareAction` 的个人连接 owner 谓词（`internal/handler/app_connector_notion_publish.go:68-83` 同形），审批沿用既有 `ApproveAction` 谓词（`internal/modules/appconnector/service/appconnector/action.go:284`）。
- 安全约束（Mimosa，与本需求相关者视为验收条件）：服务端出站请求仅 http/https 且发请求前校验 host、拒绝 localhost/环回/私网/保留地址——既有 `HTTPPolicy.PublicAddress`（`internal/modules/appconnector/http_policy.go:20`）+ `AuthorizedNetworks` 管理员授权通道承担（逐跳重校验 + DNS 全地址检查 + 校验后 IP 直拨，`http_policy.go:146-192`），测试契约双打通过 127.0.0.0/8 测试钩子（`http_policy.go:63-66` 注释明示的 documented test hook）；`ParseConfluenceBaseURL` 拒绝非 http/https scheme 与空 host（本计划 Task 1）；数据库查询全部参数绑定（本计划新查询一律 gorm `Where("col = ?", v)` 绑定，无字符串拼接 SQL）；凭据只从环境变量读取（`CONFLUENCE_BASE_URL`/`CONFLUENCE_EMAIL`/`CONFLUENCE_API_TOKEN`/`CONFLUENCE_PARENT_PAGE_ID` 经环境变量注入真实测试，源码与测试不写入可用凭据字面量；生产凭据走 `CredentialResolver`→`MCPOAuthBindingStore.LoadCredential`，以 JSON 形态 `{"username","secret"}` 存取，不在任何响应/日志出现）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计；skip 不是 pass——凡 blocked-env 证据必须显式标注。

**Issue #50 验收标准原文（docs/plans/issue30-sweep/issues/issue-50.md）：**

1. 「旧 Artifact 不静默覆盖外部新版本。」
2. 「失败和重试行为与统一 Action 语义一致。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

**验收标准 3 的本地可验证性说明（blocked-env 声明）：** 真实 Confluence 验收（真实站点 + 真实 API token + 真实远端页面 + 真实 version.number 推进）需要 `CONFLUENCE_BASE_URL`/`CONFLUENCE_EMAIL`/`CONFLUENCE_API_TOKEN`/`CONFLUENCE_PARENT_PAGE_ID`，本环境不存在（#48 的同类先例：`TestNotionRealPublishLoop` 在本环境 SKIP，`internal/modules/appconnector/notion_publish_real_test.go:24`「a skip is never a pass」）。因此：**Task 9 的真实 Provider 证据为 blocked-env**（有凭据的运行自动执行，本地 SKIP 且不得伪造）；本地最高稳定 Interface 替代证据 = **Task 8 端到端集成测试**：生产 sqlite 迁移库（golang-migrate 全量 `migrations/sqlite`）+ 真实 `ActionService`/`PublicationStore`/`ConfluenceBridge`/`ConfluencePublishService`/gin 处理器/既有审批端点/生产 `DBConfluenceScopeSource` + 本地契约双打 Confluence HTTP 服务（httptest 实现官方 Cloud v2 `GET/POST/PUT /api/v2/pages` 契约形状，含 Basic 认证与 version.number 单调门）——只有 Confluence 网络端点被替换，其余全真；双打在测试注释中明示「NOT the real-provider acceptance」。版本令牌语义与真实 Provider 一致：Confluence Cloud/Server 的页面版本均为单调递增整数 `version.number`（知识库连接器 `types.go:281-289` 的 `pageVersion` 同源佐证），本计划全程以十进制字符串承载。

**与调查结论的差异记录（以代码现状为准）：**

1. 调查称「前置依赖 #48（Notion 发布闭环）为 partial，Confluence 发布被其阻塞」——**批次事实已变**：B4 已交付并集成 #48（`git log` 见 merge 提交；本计划作者在当前 HEAD 亲眼核实 `internal/modules/appconnector/publish/plan.go:109` 的 `NotionPublishService`、`internal/modules/appconnector/repository/appconnector/publication.go:66` 的 `PublicationStore`、`internal/handler/app_connector_notion_publish.go` 全链与 E2E 三测实跑通过），阻塞解除，本计划直接 Consumes。
2. 调查称「无外部页面版本读取与冲突检测」「无 Confluence Adapter 接入统一 Action 管道」——属实且本计划补齐；但接线方式沿 #48 差异记录第 3 条的既定裁决：不为通用 `/apps/actions/:id/execute` 改造冻结的 OC-armed ActionService，而是发布链持有**自己的第二个/第三个 ActionService 实例**（`NewActionService(store, guard, gate, bridge, bridge)`，同一 `ActionStore` 为权威——`action.go:158-160` 明示「this store, not any memory map, is the authority」）。Confluence 发布动作的执行/对账只经本计划发布端点。
3. 调查称「现有 connector 是知识库只读导入，无创建/更新页面 API 调用」——属实（作者 rg 复核：`internal/modules/datasource/connector/confluence/` 全包零 `MethodPost/MethodPut/MethodPatch`）。处置：写适配器族**不寄生**于该包（包边界是 datasource 同步域），新建于 `internal/modules/appconnector/`（与 Notion/Feishu 写适配器同族）；该包零修改，只读导入能力不受影响。调查建议的「在 confluence 包加写方法」不采纳。
4. **Wave 级迁移双占（作者实锤 + 已见修复）**：作者会话开始时实跑 `go test ./internal/handler/ -run TestNotionPublishEndToEndCreateApprovePublishReceipt -count=1` FAIL（`duplicate migration file: 000114_public_agent_marketplace.down.sql`）；根因与波级指引一致——`000114`（sqlite）/`000193`（versioned）被 #60 `public_agent_marketplace`（commit 34565aa41，先落）与 #67 `mobile_device_app`（commit fb9037710，后落）双占。撰写期间 worktree 内已出现未提交修复：`mobile_device_app` 顺延至 **sqlite 000118 / versioned 000197**（`git status`：4 个 R + 5 个钉住文件 M；作者已核实 5 个测试文件的引用行号并逐行列入 Task 0）。Task 0 写成幂等：轨道健康则纯验证，双占则执行该配方（含 5 个测试文件的精确改串清单）。本计划自身**零新迁移**（`app_publications` 已存在，`provider` 为字符串列）。
5. 调查缺口清单含「端到端验证与移动端 UI 均不存在」——端到端验证由 Task 8 补齐；**移动端 UI 不属于本 Issue**：#48（同类发布闭环）同样零 TS 面，外部发布在移动端的 Action Plan 组合/审批消费属 #51（正文「一个 Task 可组合多个有序外部副作用，Owner 可整体批准或排除单项」）。本计划 filesTouched 不含任何 packages/*/apps 文件。

**编号协调与并行批次注意（B5 批次）**：本计划与同批其余计划并行实施（独立 worktree 后合并）。本计划新增文件全部为本计划独有（Task 1–9 的 Create 项共 19 个）。共享文件修改仅三处、均为最小且位置明确：`internal/router/router.go`（`RouterParams` 结构体 `AppNotionPublishHandler` 字段后加 1 个字段 + `RegisterAppNotionPublishRoutes(...)` 调用后加 1 行注册调用）；`internal/container/container.go`（Notion Provide 块后加 1 行 `must(container.Provide(newConfluencePublishHandler))`）；Task 0 的条件性迁移重编（若执行基线已含修复则为零改动）。`notion_create.go`、`notion_update.go`、`publish/plan.go`、`publish/dispatcher.go`、`publish/blocks.go`、`routes_app_notion_publish.go`、`container/notion_publish.go` 等 #48 产物**零修改**——#49（飞书发布）作者若按同一 seam 落位，与本计划的文件集完全不相交，仅在 router.go/container.go 两处各有 1 行相邻插入，合并冲突可机械消解。迁移编号：本计划不新增迁移，不占用任何新序号；Task 0 的条件重编若执行，占用 sqlite 000118 / versioned 000197（当前轨道最大号 000117/000196 之后的下一个可用号，与 worktree 内已见的修复一致；若集成时已被兄弟计划占用，按四方一致约定整体顺延为下一可用编号，文件内容零变化）。

## Review Focus

Spec/领域定义隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **审批窗口内的外部版本漂移（TOCTOU）**：成员批准时页面在 `version.number=3`，执行前外部协作者已编辑（`version.number=4`）。合理行为：执行时重读远端版本，不一致→确定性拒绝（409 `PUBLISH_VERSION_CONFLICT`）且**零写请求**（不是 unknown，更不是覆盖写入）——这是 AC1 的字面要求「旧 Artifact 不静默覆盖外部新版本」。——Task 3 `TestConfluenceUpdateConflictZeroWrites`（断言 fake 仅收到 GET、PUT 计数为 0）+ Task 6 `TestConfluenceExecuteUpdateConflictSettlesFailedReceipt` + Task 8 E2E 409 分支（fake PUT 计数不变）。
2. **版本令牌缺失/伪造被当作可写**：远端返回无 `version.number`（或 `<1`）的页面载荷、或计划期的预读返回空版本。合理行为：解析拒绝（`confluence_page_version_unparseable`）、预读失败→`PUBLISH_DESTINATION_UNREADABLE` 拒绝形成计划、执行期预读不可读→确定性 failed——任何「读不到版本」都绝不能落成「可以覆盖」。——Task 1 `TestParseConfluencePageVersion`（缺 id/number<1 拒绝）+ Task 6 `TestConfluenceFormPlanDestinationUnreadable` + Task 3 `TestConfluenceUpdatePreReadFailureIsFailedNotUnknown`。
3. **传输失败/5xx 被误判为 failed 触发盲重试**：一次 PUT 超时后本地判「失败」再执行一次=重复写入（Confluence PUT 按 `version.number+1` 推进，重复 PUT 会被远端 409 挡住，但重试语义仍必须走统一 Action 语义）。合理行为：写步骤一切不可观察结局→unknown 停车，只有远端读回能定论；unknown 状态结构性拒绝再次 dispatch，对账（reconcile）只读远端不重发。——Task 3 `TestConfluenceUpdateUnknownOnLostWriteReply`（丢响应→ActionUnknown）+ Task 6 `TestConfluenceReconcileResolvesUnknownWithoutRedispatch`（断言 fake 写调用数不增长）+ Task 8 E2E「第二次 publish 409 且 fake PUT 计数不变」。
4. **审批后内容被改写（approve-then-rewrite）**：快照四字段（update）/三字段（create）之外多一字段、或缺字段、或 storage 为空——旧批准绝不能授权新参数。合理行为：快照精确解析拒绝（`ErrConfluenceSnapshotInvalid`）+ digest 绑定使任何变化都是新 Action。——Task 1/2/3 的 `TestParseConfluence*RejectsExtraField` 系列与本计划依赖的既有 digest 机制（`action.go:141`）。
5. **越权与错接**：非 owner 成员用他人个人连接形成计划、viewer 角色调写端点、跨租户 action id 探测、把 Notion 连接接到 confluence-publish 端点（`scope.AppID != "confluence"`）。合理行为：个人连接 owner 谓词 403、写门 403（`CanDriveActionWrites` 仅 owner/admin，`internal/modules/appconnector/access.go:43-45`）、跨租户与不存在统一 404 不泄漏存在性、非 Confluence 连接一律 400 `INVALID_REQUEST` 拒绝形成计划。——Task 7 `TestConfluencePublishPlanGates` + `TestConfluencePublishActionLookupIsTenantScoped` + Task 5 `TestConfluenceBridgeRefusesNonConfluenceConnection`。

（create 目的地不在审阅过的 `approved_parents` 白名单——由 Task 6 `TestConfluenceFormPlanCreateParentOutOfScopeFailsClosed` 与 Task 2 `TestConfluenceCreateParentOutOfScope` 双层覆盖，属第 5 类已覆盖项，不占前五。）

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 0 | 前置：迁移轨道装载验证（双占基线时执行已验证的重编配方） | 幂等 Task 0——健康轨道零改动；双占轨道 4 个 git mv + 5 个测试文件改串 |
| 1 | Go appconnector 包：Confluence 公共层（端点解析/凭据/页面版本与回执解析/冲突判定） | `confluence_common.go` + 测试 |
| 2 | Go appconnector 包：`ConfluenceCreateAdapter`（三字段快照、单步创建、Query 诚实 unknown） | `confluence_create.go` + 契约双打测试（Cloud+Server 双版式） |
| 3 | Go appconnector 包：`ConfluenceUpdateAdapter`（版本预读→冲突拒绝→单步 PUT→Query 对账） | `confluence_update.go` + 契约双打测试 |
| 4 | publish 包：Artifact 文本→Confluence storage 正文纯投影 | `confluence_blocks.go` + 测试 |
| 5 | publish 包：`ConfluenceBridge`（Dispatcher+Resolver+计划期预读）+ 生产 DB scope source / policy / 凭据端口 | `confluence_bridge.go` + 测试 |
| 6 | publish 包：`ConfluencePublishService`（FormPlan 版本预读/Execute/Reconcile/回执 settle） | `confluence.go` + 测试 |
| 7 | HTTP：`/apps/confluence-publish/*` 端点 + 路由 + 容器接线 + 谓词测试 | `app_connector_confluence_publish.go` 等三处新文件 + router.go/container.go 各 1 处最小修改 |
| 8 | E2E：最高稳定 Interface 证据（生产迁移 + 全链 + 双打 Confluence） | `app_connector_confluence_publish_e2e_test.go` |
| 9 | 真实受控集成证据（blocked-env，opt-in） | `confluence_publish_real_test.go` |

---

### Task 0: 前置——迁移轨道装载验证（双占基线时执行重编配方）

**Files:**
- Modify（条件，重命名）: `migrations/sqlite/000114_mobile_device_app.up.sql` → `migrations/sqlite/000118_mobile_device_app.up.sql`（仅当存在时）
- Modify（条件，重命名）: `migrations/sqlite/000114_mobile_device_app.down.sql` → `migrations/sqlite/000118_mobile_device_app.down.sql`（仅当存在时）
- Modify（条件，重命名）: `migrations/versioned/000193_mobile_device_app.up.sql` → `migrations/versioned/000197_mobile_device_app.up.sql`（仅当存在时）
- Modify（条件，重命名）: `migrations/versioned/000193_mobile_device_app.down.sql` → `migrations/versioned/000197_mobile_device_app.down.sql`（仅当存在时）
- Modify（条件，改串）: `internal/application/repository/mobile_device_app_test.go:37,188,189`（路径钉住）与 `:22,179`（注释）
- Modify（条件，改串）: `internal/application/repository/mobile_push_isolation_test.go:67`（路径）与 `:61`（注释）
- Modify（条件，改串）: `internal/application/repository/mobile_device_test.go:31`（路径）与 `:27`（注释）
- Modify（条件，改串）: `internal/handler/mobile_device_test.go:33`（路径）与 `:28-29`（注释）
- Modify（条件，改串）: `internal/modules/workbench/service/workbench/notification_app_policy_test.go:50`（路径）与 `:36`（注释）

**Interfaces:**
- Consumes: 无（纯既有迁移文件条件重编号；不改任何 DDL——golang-migrate 按文件名版本号发现迁移）。
- Produces: 可加载的生产迁移双轨（`migrations/sqlite` 与 `migrations/versioned` 版本号唯一）——Task 8 的 `openConfluencePublishE2EDB`（golang-migrate 全量加载）由此解锁。事实依据（作者已核实）：`mobile_device_app` 只重建/改列 `mobile_devices` 与 `mobile_notification_intents` 两表（建表迁移远早于 000114），`public_agent_marketplace` 只建 marketplace 五表+台账——两迁移无任何相互依赖，把后落的 `mobile_device_app` 移到轨道末尾安全；本计划自身零新迁移。
- 幂等性：本任务在健康轨道上是 no-op（只有验证步骤）。

- [ ] **Step 1: 检查执行基线的轨道状态**

```bash
test -f migrations/sqlite/000114_mobile_device_app.up.sql && echo "DUPLICATE-PRESENT" || echo "TRACK-HEALTHY"
```

Expected: `TRACK-HEALTHY`（作者撰写时 worktree 已含并行会话的未提交修复：`mobile_device_app` 已在 sqlite 000118 / versioned 000197，`git status` 为 R+M 状态）或 `DUPLICATE-PRESENT`（执行工作树从双占基线切出）。**若输出 `TRACK-HEALTHY`，跳过 Step 2/3，直接做 Step 4 验证与 Step 5（无改动则无提交）。**

- [ ] **Step 2（仅 DUPLICATE-PRESENT）: 重命名四个迁移文件（sqlite 与 versioned 双轨同步）**

```bash
git mv migrations/sqlite/000114_mobile_device_app.up.sql migrations/sqlite/000118_mobile_device_app.up.sql
git mv migrations/sqlite/000114_mobile_device_app.down.sql migrations/sqlite/000118_mobile_device_app.down.sql
git mv migrations/versioned/000193_mobile_device_app.up.sql migrations/versioned/000197_mobile_device_app.up.sql
git mv migrations/versioned/000193_mobile_device_app.down.sql migrations/versioned/000197_mobile_device_app.down.sql
```

（占用 sqlite 000118 / versioned 000197：当前轨道最大号 000117/000196 之后的下一个可用号。若集成时该号已被兄弟计划占用，整体顺延为下一个可用编号——文件内容零变化。重命名对既有环境无脏状态风险：双占期间 golang-migrate 无法加载目录，任何环境都不可能存在「已应用 114=mobile_device_app」的中间状态；作者已用 `/tmp` 迁移副本模拟重编号实跑验证 golang-migrate 全量 Up 可通过。）

- [ ] **Step 3（仅 DUPLICATE-PRESENT）: 同步修正 5 个测试文件中的钉住路径与注释**

对以下 5 个文件执行精确字符串替换 `000114_mobile_device_app` → `000118_mobile_device_app`、`000193_mobile_device_app` → `000197_mobile_device_app`（作者已逐行核实，这是全部出现点；均为路径字符串与注释，无逻辑改动）：

- `internal/application/repository/mobile_device_app_test.go`：第 22、37、179、188、189 行
- `internal/application/repository/mobile_push_isolation_test.go`：第 61、67 行
- `internal/application/repository/mobile_device_test.go`：第 27、31 行
- `internal/handler/mobile_device_test.go`：第 28、29、33 行
- `internal/modules/workbench/service/workbench/notification_app_policy_test.go`：第 36、50 行

- [ ] **Step 4: 运行验证（GREEN）**

Run: `go test ./internal/handler/ -run 'TestNotionPublishEndToEndCreateApprovePublishReceipt' -count=1`
Expected: PASS（该既有测试经 golang-migrate 全量加载 `migrations/sqlite`——迁移轨道可装载的直接证据；作者在修复后的 worktree 实跑通过）。

Run（双轨唯一性 shell 断言；每条迁移本有 up/down 两个文件，故健康状态=任何版本号出现次数 ≤2）:

```bash
test -z "$(ls migrations/sqlite | grep -oE '^[0-9]{6}' | sort | uniq -c | awk '$1 > 2')" && test -z "$(ls migrations/versioned | grep -oE '^[0-9]{6}' | sort | uniq -c | awk '$1 > 2')" && echo "migration versions unique on both tracks"
```

Expected: 输出 `migration versions unique on both tracks`。（作者在双占基线实跑该断言检测形态时，sqlite 轨输出 `4 000114`、versioned 轨输出 `4 000193`——正是两处双占；修复后两者必须为空。）

- [ ] **Step 5: 提交（仅当有改动）**

```bash
git add migrations/ internal/application/repository/mobile_device_app_test.go internal/application/repository/mobile_push_isolation_test.go internal/application/repository/mobile_device_test.go internal/handler/mobile_device_test.go internal/modules/workbench/service/workbench/notification_app_policy_test.go
git commit -m "fix(migrations): renumber mobile_device_app to 000118/000197 - resolve duplicate-version chain breakage with public_agent_marketplace (B5 #50 prerequisite)"
```

（无改动时跳过——不允许空提交。）

---

### Task 1: Confluence 公共层——端点解析、Basic 凭据、页面版本/回执解析、冲突判定（纯逻辑）

**Files:**
- Create: `internal/modules/appconnector/confluence_common.go`
- Test: `internal/modules/appconnector/confluence_common_test.go`

**Interfaces:**
- Consumes: 无新依赖（标准库 `encoding/json`/`errors`/`fmt`/`net/url`/`strconv`/`strings`）。
- Produces（Task 2/3/5/6/9 依赖，签名逐字）:
  - `var ErrConfluenceVersionConflict = errors.New("confluence_version_conflict")`
  - `var ErrConfluenceSnapshotInvalid = errors.New("confluence_snapshot_invalid")`
  - `var ErrConfluenceParentOutOfScope = errors.New("confluence_parent_out_of_scope")`
  - `var ErrConfluenceMissingCapability = errors.New("confluence_missing_capability")`
  - `var ErrConfluenceOutcomeUnknown = errors.New("confluence_outcome_unknown")`
  - `var ErrConfluenceNotConfigured = errors.New("confluence_adapter_not_configured")`
  - `var ErrConfluenceApprovalRevoked = errors.New("confluence_approval_revoked")`
  - `var ErrConfluenceCredentialInvalid = errors.New("confluence_credential_invalid")`
  - `const ConfluenceCapabilityWrite = "write_content"`；`const EditionCloud = "cloud"`；`const EditionServer = "server"`
  - `type ConfluenceCredential struct { Username string; Secret string }`
  - `func ParseConfluenceCredential(raw []byte) (ConfluenceCredential, error)`
  - `type ConfluenceEndpoint struct { Scheme, Host, Port, APIBasePath, Edition string }`
  - `func ParseConfluenceBaseURL(raw, explicitEdition string) (ConfluenceEndpoint, error)`
  - `type ConfluencePageVersion struct { PageID, Title, SpaceID, SpaceKey, VersionNumber, BodyStorage string }`
  - `func ParseConfluencePageVersion(raw []byte) (ConfluencePageVersion, error)`
  - `type ConfluencePageReceipt struct { ExternalID string; ExternalVersion string }`
  - `func ParseConfluencePageReceipt(raw []byte) (ConfluencePageReceipt, error)`
  - `func DetectConfluenceVersionConflict(expected, actual string) error`

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/appconnector/confluence_common_test.go`：

```go
package appconnector

import (
	"errors"
	"testing"
)

func TestParseConfluenceCredential(t *testing.T) {
	c, err := ParseConfluenceCredential([]byte(`{"username":"u@example.test","secret":"s3cret"}`))
	if err != nil || c.Username != "u@example.test" || c.Secret != "s3cret" {
		t.Fatalf("credential parse drift: %+v %v", c, err)
	}
	for name, raw := range map[string][]byte{
		"not json":       []byte(`nope`),
		"empty username": []byte(`{"username":"","secret":"s"}`),
		"blank username": []byte(`{"username":"  ","secret":"s"}`),
		"empty secret":   []byte(`{"username":"u","secret":""}`),
		"missing secret": []byte(`{"username":"u"}`),
	} {
		if _, err := ParseConfluenceCredential(raw); !errors.Is(err, ErrConfluenceCredentialInvalid) {
			t.Fatalf("%s: want ErrConfluenceCredentialInvalid, got %v", name, err)
		}
	}
}

func TestParseConfluenceBaseURL(t *testing.T) {
	// Atlassian Cloud: /wiki context path is filled in when absent.
	ep, err := ParseConfluenceBaseURL("https://acme.atlassian.net", "")
	if err != nil || ep.Edition != EditionCloud || ep.APIBasePath != "/wiki" ||
		ep.Scheme != "https" || ep.Host != "acme.atlassian.net" || ep.Port != "" {
		t.Fatalf("cloud endpoint drift: %+v %v", ep, err)
	}
	// An explicit path is kept as-is.
	ep, err = ParseConfluenceBaseURL("https://acme.atlassian.net/wiki/", "")
	if err != nil || ep.APIBasePath != "/wiki" {
		t.Fatalf("cloud explicit context path drift: %+v %v", ep, err)
	}
	// Server/DC at the root: empty base path, server edition.
	ep, err = ParseConfluenceBaseURL("https://confluence.corp.example", "")
	if err != nil || ep.Edition != EditionServer || ep.APIBasePath != "" || ep.Host != "confluence.corp.example" {
		t.Fatalf("server root drift: %+v %v", ep, err)
	}
	// Server/DC behind a context path keeps it.
	ep, err = ParseConfluenceBaseURL("https://corp.example/confluence", "")
	if err != nil || ep.Edition != EditionServer || ep.APIBasePath != "/confluence" {
		t.Fatalf("server context path drift: %+v %v", ep, err)
	}
	// An explicit edition overrides host-based derivation (loopback test
	// doubles are cloud-shaped but not *.atlassian.net).
	ep, err = ParseConfluenceBaseURL("http://127.0.0.1:8989/wiki", "cloud")
	if err != nil || ep.Edition != EditionCloud || ep.APIBasePath != "/wiki" || ep.Port != "8989" {
		t.Fatalf("explicit edition override drift: %+v %v", ep, err)
	}
	for name, raw := range map[string]string{
		"empty":        "   ",
		"no scheme":    "confluence.corp.example",
		"bad scheme":   "ftp://confluence.corp.example",
		"no host":      "https:///wiki",
		"parse error":  "https://exa mple.test",
	} {
		if _, err := ParseConfluenceBaseURL(raw, ""); err == nil {
			t.Fatalf("%s: must be refused", name)
		}
	}
	// Unknown explicit edition fails closed.
	if _, err := ParseConfluenceBaseURL("https://confluence.corp.example", "hyper"); err == nil {
		t.Fatal("unknown explicit edition must be refused")
	}
}

func TestParseConfluencePageVersion(t *testing.T) {
	// Cloud v2 page shape.
	v, err := ParseConfluencePageVersion([]byte(`{"id":"9","status":"current","title":"P","spaceId":"42","version":{"number":3,"createdAt":"2026-09-25T08:00:00Z"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if v.PageID != "9" || v.Title != "P" || v.SpaceID != "42" || v.VersionNumber != "3" || v.SpaceKey != "" {
		t.Fatalf("cloud page version drift: %+v", v)
	}
	// Server/DC content shape.
	v, err = ParseConfluencePageVersion([]byte(`{"id":"9","type":"page","title":"P","space":{"key":"ENG","id":"42"},"version":{"number":7},"body":{"storage":{"value":"<p>x</p>"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if v.SpaceKey != "ENG" || v.VersionNumber != "7" || v.BodyStorage != "<p>x</p>" {
		t.Fatalf("server page version drift: %+v", v)
	}
	for name, raw := range map[string][]byte{
		"no id":          []byte(`{"version":{"number":2}}`),
		"no version":     []byte(`{"id":"9"}`),
		"zero version":   []byte(`{"id":"9","version":{"number":0}}`),
		"negative":       []byte(`{"id":"9","version":{"number":-1}}`),
		"not an object":  []byte(`["page"]`),
	} {
		if _, err := ParseConfluencePageVersion(raw); err == nil {
			t.Fatalf("%s: a fabricated version must never parse", name)
		}
	}
}

func TestParseConfluencePageReceipt(t *testing.T) {
	r, err := ParseConfluencePageReceipt([]byte(`{"id":"p1","version":{"number":9}}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.ExternalID != "p1" || r.ExternalVersion != "9" {
		t.Fatalf("receipt fields drift: %+v", r)
	}
	if _, err := ParseConfluencePageReceipt([]byte(`{"id":"","version":{"number":1}}`)); err == nil {
		t.Fatal("no real page id must refuse a receipt")
	}
}

func TestDetectConfluenceVersionConflict(t *testing.T) {
	if err := DetectConfluenceVersionConflict("3", "3"); err != nil {
		t.Fatalf("matching version must pass: %v", err)
	}
	if err := DetectConfluenceVersionConflict("3", "4"); !errors.Is(err, ErrConfluenceVersionConflict) {
		t.Fatalf("drift must conflict, got %v", err)
	}
	if err := DetectConfluenceVersionConflict("", "4"); !errors.Is(err, ErrConfluenceVersionConflict) {
		t.Fatalf("empty expected must fail closed, got %v", err)
	}
	if err := DetectConfluenceVersionConflict("3", ""); !errors.Is(err, ErrConfluenceVersionConflict) {
		t.Fatalf("unreadable remote must fail closed, got %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/appconnector/ -run 'TestParseConfluenceCredential|TestParseConfluenceBaseURL|TestParseConfluencePageVersion|TestParseConfluencePageReceipt|TestDetectConfluenceVersionConflict' -count=1`
Expected: FAIL（`undefined: ParseConfluenceCredential` 等编译错误）。

- [ ] **Step 3: 最小实现**

创建 `internal/modules/appconnector/confluence_common.go`：

```go
package appconnector

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Confluence write rejections. Every denial names which rule was violated;
// none of them ever carries credential material.
var (
	// ErrConfluenceVersionConflict: the external page's current
	// version.number no longer equals the version the approved update plan
	// was formed against. The update is refused BEFORE any write request —
	// per CONTEXT.md「外部发布」: 再次更新前必须读取外部当前版本并形成新的
	// 候选变更.
	ErrConfluenceVersionConflict = errors.New("confluence_version_conflict")
	// ErrConfluenceSnapshotInvalid: the action arguments are not exactly the
	// approved snapshot shape — an extra field (approve-then-rewrite), a
	// missing field, or an empty required field is refused rather than
	// silently forwarded.
	ErrConfluenceSnapshotInvalid = errors.New("confluence_snapshot_invalid")
	// ErrConfluenceParentOutOfScope: the snapshot's parent page is not
	// inside the approval scope for this connection. The check fails
	// closed: an empty scope list rejects every parent.
	ErrConfluenceParentOutOfScope = errors.New("confluence_parent_out_of_scope")
	// ErrConfluenceMissingCapability: the connection does not carry the
	// reviewed write capability.
	ErrConfluenceMissingCapability = errors.New("confluence_missing_capability")
	// ErrConfluenceOutcomeUnknown: the request may or may not have produced
	// its remote effect (response lost, 5xx, unparseable reply, or a
	// provider reply without a real page id/version). The outcome stays
	// unknown and resolves ONLY via Query's reliable page read.
	ErrConfluenceOutcomeUnknown = errors.New("confluence_outcome_unknown")
	// ErrConfluenceNotConfigured: the adapter is missing a reviewed outbound
	// policy or a credential source — fail closed, never dial by invention.
	ErrConfluenceNotConfigured = errors.New("confluence_adapter_not_configured")
	// ErrConfluenceApprovalRevoked: the A03 approval was revoked between
	// approval and execute; the action stays parked awaiting approval.
	ErrConfluenceApprovalRevoked = errors.New("confluence_approval_revoked")
	// ErrConfluenceCredentialInvalid: the resolved credential bytes are not
	// the reviewed JSON shape.
	ErrConfluenceCredentialInvalid = errors.New("confluence_credential_invalid")
)

// Confluence capability + editions. ConfluenceCapabilityWrite is the
// reviewed schema_json scope this adapter family requires
// ({"scopes":["write_content"],...}); EditionCloud selects the
// {base}/api/v2 wire, EditionServer the {base}/rest/api wire.
const (
	ConfluenceCapabilityWrite = "write_content"
	EditionCloud              = "cloud"
	EditionServer             = "server"
)

// ConfluenceCredential is the decrypted connection credential: Confluence
// authenticates with HTTP Basic over (username, secret) on BOTH editions
// (Cloud: email + API token; Server/DC: username + password). The
// connection credential store holds it as JSON
// {"username": "...", "secret": "..."} — never as a header string.
type ConfluenceCredential struct {
	Username string
	Secret   string
}

// ParseConfluenceCredential validates the resolved credential bytes.
func ParseConfluenceCredential(raw []byte) (ConfluenceCredential, error) {
	var c struct {
		Username string `json:"username"`
		Secret   string `json:"secret"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return ConfluenceCredential{}, fmt.Errorf("%w: %v", ErrConfluenceCredentialInvalid, err)
	}
	if strings.TrimSpace(c.Username) == "" || c.Secret == "" {
		return ConfluenceCredential{}, fmt.Errorf("%w: username and secret are required", ErrConfluenceCredentialInvalid)
	}
	return ConfluenceCredential{Username: c.Username, Secret: c.Secret}, nil
}

// ConfluenceEndpoint is the reviewed outbound anchor derived from the
// installation's config_json.base_url: exactly one scheme/host/port plus
// the API base path (context path) and the resolved edition.
type ConfluenceEndpoint struct {
	Scheme string
	Host   string
	Port   string
	// APIBasePath is "" or a "/context" path (e.g. "/wiki").
	APIBasePath string
	Edition     string
}

// ParseConfluenceBaseURL derives the endpoint from the reviewed base URL.
// Edition defaults from the host (*.atlassian.net → cloud) and may be
// overridden explicitly (an enterprise test double on a loopback host is
// cloud-shaped but not an atlassian host). Only http/https is accepted —
// dial-time public-address enforcement stays with HTTPPolicy.
func ParseConfluenceBaseURL(raw, explicitEdition string) (ConfluenceEndpoint, error) {
	raw = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if raw == "" || !strings.Contains(raw, "://") {
		return ConfluenceEndpoint{}, fmt.Errorf("confluence_base_url_invalid: %q", raw)
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return ConfluenceEndpoint{}, fmt.Errorf("confluence_base_url_invalid: %q", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ConfluenceEndpoint{}, fmt.Errorf("confluence_base_url_invalid: scheme %q", parsed.Scheme)
	}
	host := parsed.Hostname()
	ep := ConfluenceEndpoint{Scheme: parsed.Scheme, Host: host, Port: parsed.Port(), Edition: EditionServer}
	if host == "atlassian.net" || strings.HasSuffix(host, ".atlassian.net") {
		ep.Edition = EditionCloud
	}
	if explicitEdition != "" {
		if explicitEdition != EditionCloud && explicitEdition != EditionServer {
			return ConfluenceEndpoint{}, fmt.Errorf("confluence_base_url_invalid: edition %q", explicitEdition)
		}
		ep.Edition = explicitEdition
	}
	if path := strings.TrimRight(parsed.Path, "/"); path != "" {
		ep.APIBasePath = path
	} else if ep.Edition == EditionCloud {
		ep.APIBasePath = "/wiki"
	}
	return ep, nil
}

// ConfluencePageVersion is the reliable read shape of one page across both
// editions. VersionNumber is the external collaboration authority's version
// token — Confluence's monotonic integer version.number, carried as its
// decimal string. BodyStorage is present when the read asked for the
// storage body.
type ConfluencePageVersion struct {
	PageID        string
	Title         string
	SpaceID       string
	SpaceKey      string
	VersionNumber string
	BodyStorage   string
}

type confluencePageWire struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	SpaceID string `json:"spaceId"`
	Space   struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	} `json:"space"`
	Version struct {
		Number    int    `json:"number"`
		When      string `json:"when"`
		CreatedAt string `json:"createdAt"`
	} `json:"version"`
	Body struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
}

// ParseConfluencePageVersion accepts the Cloud v2 page object (spaceId) and
// the Server/DC content object (space.key) in one shape. A reply without a
// real id or a real version.number (>= 1) is an error — a fabricated
// version is never a basis for conflict detection or a receipt.
func ParseConfluencePageVersion(raw []byte) (ConfluencePageVersion, error) {
	var w confluencePageWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return ConfluencePageVersion{}, fmt.Errorf("confluence_page_version_unparseable: %v", err)
	}
	if w.ID == "" || w.Version.Number < 1 {
		return ConfluencePageVersion{}, fmt.Errorf("confluence_page_version_unparseable: no id or version.number")
	}
	return ConfluencePageVersion{
		PageID: w.ID, Title: w.Title, SpaceID: w.SpaceID, SpaceKey: w.Space.Key,
		VersionNumber: strconv.Itoa(w.Version.Number), BodyStorage: w.Body.Storage.Value,
	}, nil
}

// ConfluencePageReceipt is the persisted external receipt of one publish:
// the provider page id and the version the publish itself produced (read
// from the provider's own reply — never fabricated locally).
type ConfluencePageReceipt struct {
	ExternalID      string
	ExternalVersion string
}

// ParseConfluencePageReceipt extracts the receipt fields from a provider
// page payload (the create/update reply the adapter recorded as the
// action's output evidence).
func ParseConfluencePageReceipt(raw []byte) (ConfluencePageReceipt, error) {
	v, err := ParseConfluencePageVersion(raw)
	if err != nil {
		return ConfluencePageReceipt{}, err
	}
	return ConfluencePageReceipt{ExternalID: v.PageID, ExternalVersion: v.VersionNumber}, nil
}

// DetectConfluenceVersionConflict compares the approved expected version
// with the version just read from the provider. Anything but an exact
// match — including an unreadable empty side — is a conflict; an
// unobservable remote state must never authorize an overwrite.
func DetectConfluenceVersionConflict(expected, actual string) error {
	if expected == "" || actual == "" || expected != actual {
		return fmt.Errorf("%w: approved %q but remote has %q", ErrConfluenceVersionConflict, expected, actual)
	}
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/appconnector/ -run 'TestParseConfluenceCredential|TestParseConfluenceBaseURL|TestParseConfluencePageVersion|TestParseConfluencePageReceipt|TestDetectConfluenceVersionConflict' -count=1`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/appconnector/confluence_common.go internal/modules/appconnector/confluence_common_test.go
git commit -m "feat(appconnector): confluence endpoint/credential/page-version parsing (T20 #50)"
```

---

### Task 2: `ConfluenceCreateAdapter`——三字段快照、单步创建、诚实 unknown

**Files:**
- Create: `internal/modules/appconnector/confluence_create.go`
- Test: `internal/modules/appconnector/confluence_create_test.go`

**Interfaces:**
- Consumes: Task 1 全部公共层符号；`HTTPPolicy`（`internal/modules/appconnector/http_policy.go:67`，`NewClient()` 逐跳校验 + 公网地址强制）；`Action`/`ActionResult`/`Adapter`/`ActionFailed`/`ActionUnknown`/`ActionAwaitingApproval`/`ActionSucceeded`/`RiskWrite`（同包既有）。
- Produces（Task 3/5/8 依赖，签名逐字）:
  - `type ConfluenceCreateSnapshot struct { Parent string; Title string; Storage string }`
  - `func ParseConfluenceCreateSnapshot(args json.RawMessage) (ConfluenceCreateSnapshot, error)`
  - `func IsConfluenceCreateArgs(args json.RawMessage) bool`
  - `const ConfluenceCloudPagesPath = "/api/v2/pages"`；`const ConfluenceServerContentPath = "/rest/api/content"`
  - `type ConfluenceCreateAdapter struct { Policy HTTPPolicy; Credential func(ctx context.Context) (ConfluenceCredential, error); Edition string; APIBasePath string; ApprovedParents []string; ConnectionCapabilities func(ctx context.Context, a Action) ([]string, error); Recheck func(ctx context.Context, a Action) error }`
  - `func (m *ConfluenceCreateAdapter) Execute(ctx context.Context, a Action) (ActionResult, error)`；`func (m *ConfluenceCreateAdapter) Query(ctx context.Context, a Action) (ActionResult, error)`；`var _ Adapter = (*ConfluenceCreateAdapter)(nil)`
  - 包内共享管线（Task 3 复用）：`func confluenceTargetURL(pol HTTPPolicy, apiBasePath, path, query string) *url.URL`、`func confluenceDo(ctx context.Context, pol HTTPPolicy, cred func(context.Context) (ConfluenceCredential, error), method string, u *url.URL, body []byte) (int, []byte, error)`、`func confluenceReadPage(ctx context.Context, pol HTTPPolicy, cred func(context.Context) (ConfluenceCredential, error), edition, apiBasePath, pageID string) (ConfluencePageVersion, error)`

设计要点（与 Notion 家族的差异都有因）：create 的目的地是**审阅过的父页面**（`approved_parents`，与 Notion 完全同形）；但 Confluence create 必须携带目的空间（Cloud `space_id` / Server `space key`），所以适配器在写之前先 GET 父页面解析空间——该读在证明目的地可读的同时给出空间，仍是「读永不产生写」，读失败一律确定性 failed。写是**单步** POST（storage 正文一次携带全部内容，无需 Notion 的 100 块分批），故无 progress 持久化端口；create 回复丢失→unknown 且**无任何已确认页面 id 可供 Query 核对**（Confluence create 无幂等键），Query 诚实返回 unknown 交人工对账（与 Notion create 的「标题搜索仅供参考、永不作为唯一证据」同一纪律，且更保守：连参考搜索都不发）。

- [ ] **Step 1: 写失败测试（契约双打 + 适配器行为）**

创建 `internal/modules/appconnector/confluence_create_test.go`：

```go
package appconnector

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeConfluence is a LOCAL contract double of the official Confluence
// REST wire for BOTH editions (developer.atlassian.com): Cloud v2
// GET/POST/PUT {base}/api/v2/pages and Server/DC GET/POST/PUT
// {base}/rest/api/content, with HTTP Basic auth and the monotonic
// version.number gate a real instance enforces on update. It is NOT the
// real-provider acceptance — that stays CONFLUENCE_*-gated (Task 9,
// confluence_publish_real_test.go).
type fakeConfluence struct {
	mu       sync.Mutex
	username string
	secret   string
	pages    map[string]*cfPage
	nextID   int
	// counters
	postCalls, putCalls, pageGets int
	// knob: apply the write effect, then lose the reply (unknown outcome).
	dropNextWrite bool
}

type cfPage struct {
	id, spaceID, spaceKey, title, storage string
	version                               int
}

func newFakeConfluence(username, secret string) *fakeConfluence {
	return &fakeConfluence{username: username, secret: secret, pages: map[string]*cfPage{}}
}

func (f *fakeConfluence) addPage(id, spaceID, spaceKey, title string, version int) *cfPage {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := &cfPage{id: id, spaceID: spaceID, spaceKey: spaceKey, title: title, version: version}
	f.pages[id] = p
	return p
}

// bump applies an EXTERNAL collaborator's edit: the version advances and
// (when storage is non-empty) the content changes.
func (f *fakeConfluence) bump(id, storage string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.pages[id]; ok {
		p.version++
		if storage != "" {
			p.storage = storage
		}
	}
}

func (f *fakeConfluence) stats() (posts, puts, gets int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.postCalls, f.putCalls, f.pageGets
}

func cfAuthorized(r *http.Request, username, secret string) bool {
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+secret))
	return r.Header.Get("Authorization") == want
}

func cfCloudJSON(p *cfPage, withBody bool) string {
	storage := ""
	if withBody {
		raw, _ := json.Marshal(p.storage)
		storage = fmt.Sprintf(`,"body":{"storage":{"value":%s}}`, raw)
	}
	return fmt.Sprintf(`{"id":%q,"status":"current","title":%q,"spaceId":%q,"version":{"number":%d,"createdAt":"2026-09-25T08:00:00Z"}%s}`,
		p.id, p.title, p.spaceID, p.version, storage)
}

func cfServerJSON(p *cfPage, withBody bool) string {
	storage := ""
	if withBody {
		raw, _ := json.Marshal(p.storage)
		storage = fmt.Sprintf(`,"body":{"storage":{"value":%s,"representation":"storage"}}`, raw)
	}
	return fmt.Sprintf(`{"id":%q,"type":"page","title":%q,"space":{"key":%q},"version":{"number":%d}%s}`,
		p.id, p.title, p.spaceKey, p.version, storage)
}

func (f *fakeConfluence) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !cfAuthorized(r, f.username, f.secret) {
			writeJSON(w, 401, `{"message":"unauthorized"}`)
			return
		}
		// The /wiki context path (Atlassian Cloud) is stripped before
		// routing so both bare and /wiki-prefixed doubles share one mux.
		path := strings.TrimPrefix(r.URL.Path, "/wiki")
		switch {
		case path == "/api/v2/pages" && r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				SpaceID  string `json:"space_id"`
				ParentID string `json:"parent_id"`
				Status   string `json:"status"`
				Title    string `json:"title"`
				Body     struct {
					Representation string `json:"representation"`
					Value          string `json:"value"`
				} `json:"body"`
			}
			_ = json.Unmarshal(body, &req)
			f.mu.Lock()
			parent, ok := f.pages[req.ParentID]
			f.mu.Unlock()
			if !ok || req.SpaceID == "" || req.Title == "" || req.Body.Value == "" {
				writeJSON(w, 400, `{"errors":[{"title":"invalid create request"}]}`)
				return
			}
			f.mu.Lock()
			f.nextID++
			f.postCalls++
			np := &cfPage{id: fmt.Sprintf("cf-%d", f.nextID), spaceID: req.SpaceID, spaceKey: parent.spaceKey,
				title: req.Title, storage: req.Body.Value, version: 1}
			f.pages[np.id] = np
			drop := f.dropNextWrite
			if drop {
				f.dropNextWrite = false
			}
			f.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, cfCloudJSON(np, false))
		case strings.HasPrefix(path, "/api/v2/pages/") && r.Method == http.MethodGet:
			id := strings.TrimPrefix(path, "/api/v2/pages/")
			f.mu.Lock()
			f.pageGets++
			p, ok := f.pages[id]
			f.mu.Unlock()
			if !ok {
				writeJSON(w, 404, `{"errors":[{"title":"not found"}]}`)
				return
			}
			writeJSON(w, 200, cfCloudJSON(p, true))
		case strings.HasPrefix(path, "/api/v2/pages/") && r.Method == http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				ID     string `json:"id"`
				Status string `json:"status"`
				Title  string `json:"title"`
				Body   struct {
					Representation string `json:"representation"`
					Value          string `json:"value"`
				} `json:"body"`
				Version struct {
					Number int `json:"number"`
				} `json:"version"`
			}
			_ = json.Unmarshal(body, &req)
			id := strings.TrimPrefix(path, "/api/v2/pages/")
			f.mu.Lock()
			p, ok := f.pages[id]
			f.mu.Unlock()
			if !ok {
				writeJSON(w, 404, `{"errors":[{"title":"not found"}]}`)
				return
			}
			if req.Version.Number != p.version+1 {
				writeJSON(w, 409, `{"errors":[{"title":"version conflict"}]}`)
				return
			}
			f.mu.Lock()
			f.putCalls++
			p.title, p.storage, p.version = req.Title, req.Body.Value, req.Version.Number
			drop := f.dropNextWrite
			if drop {
				f.dropNextWrite = false
			}
			f.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, cfCloudJSON(p, false))
		case path == "/rest/api/content" && r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Type      string `json:"type"`
				Space     struct {
					Key string `json:"key"`
				} `json:"space"`
				Ancestors []struct {
					ID string `json:"id"`
				} `json:"ancestors"`
				Title string `json:"title"`
				Body  struct {
					Storage struct {
						Value          string `json:"value"`
						Representation string `json:"representation"`
					} `json:"storage"`
				} `json:"body"`
			}
			_ = json.Unmarshal(body, &req)
			parentID := ""
			if len(req.Ancestors) > 0 {
				parentID = req.Ancestors[0].ID
			}
			f.mu.Lock()
			parent, ok := f.pages[parentID]
			f.mu.Unlock()
			if !ok || req.Space.Key == "" || req.Title == "" || req.Body.Storage.Value == "" {
				writeJSON(w, 400, `{"message":"invalid create request"}`)
				return
			}
			f.mu.Lock()
			f.nextID++
			f.postCalls++
			np := &cfPage{id: fmt.Sprintf("cf-%d", f.nextID), spaceID: parent.spaceID, spaceKey: req.Space.Key,
				title: req.Title, storage: req.Body.Storage.Value, version: 1}
			f.pages[np.id] = np
			drop := f.dropNextWrite
			if drop {
				f.dropNextWrite = false
			}
			f.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, cfServerJSON(np, false))
		case strings.HasPrefix(path, "/rest/api/content/") && r.Method == http.MethodGet:
			id := strings.TrimPrefix(path, "/rest/api/content/")
			f.mu.Lock()
			f.pageGets++
			p, ok := f.pages[id]
			f.mu.Unlock()
			if !ok {
				writeJSON(w, 404, `{"message":"not found"}`)
				return
			}
			writeJSON(w, 200, cfServerJSON(p, true))
		case strings.HasPrefix(path, "/rest/api/content/") && r.Method == http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Type  string `json:"type"`
				ID    string `json:"id"`
				Title string `json:"title"`
				Body  struct {
					Storage struct {
						Value          string `json:"value"`
						Representation string `json:"representation"`
					} `json:"storage"`
				} `json:"body"`
				Version struct {
					Number int `json:"number"`
				} `json:"version"`
			}
			_ = json.Unmarshal(body, &req)
			id := strings.TrimPrefix(path, "/rest/api/content/")
			f.mu.Lock()
			p, ok := f.pages[id]
			f.mu.Unlock()
			if !ok {
				writeJSON(w, 404, `{"message":"not found"}`)
				return
			}
			if req.Version.Number != p.version+1 {
				writeJSON(w, 409, `{"message":"version conflict"}`)
				return
			}
			f.mu.Lock()
			f.putCalls++
			p.title, p.storage, p.version = req.Title, req.Body.Storage.Value, req.Version.Number
			drop := f.dropNextWrite
			if drop {
				f.dropNextWrite = false
			}
			f.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, cfServerJSON(p, false))
		default:
			writeJSON(w, 404, `{"message":"no route"}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// cfPolicyOf mirrors the documented loopback test hook (http_policy.go:63-66).
// It takes no *testing.T on purpose: several tests build adapters through
// helpers that have no t in scope.
func cfPolicyOf(srv *httptest.Server, apiBasePath string) HTTPPolicy {
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	prefix := "/"
	if apiBasePath != "" {
		prefix = apiBasePath + "/"
	}
	return HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PUT"}, PathPrefix: prefix,
		AuthorizedNetworks: []*net.IPNet{network}, Timeout: 10 * time.Second,
	}
}

func cfCredential() (ConfluenceCredential, error) {
	return ConfluenceCredential{Username: "cf-user@example.test", Secret: "secret_cf_token"}, nil
}

func cfCaps(caps ...string) func(ctx context.Context, a Action) ([]string, error) {
	return func(ctx context.Context, a Action) ([]string, error) { return caps, nil }
}

func cfCreateAdapter(srv *httptest.Server, edition, apiBasePath string) *ConfluenceCreateAdapter {
	return &ConfluenceCreateAdapter{
		Policy:                 cfPolicyOf(srv, apiBasePath),
		Credential:             cfCredential,
		Edition:                edition,
		APIBasePath:            apiBasePath,
		ApprovedParents:        []string{"parent-1"},
		ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
}

func cfCreateArgs(parent, title, storage string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"parent": parent, "title": title, "storage": storage})
	return raw
}

func cfCreateAction(args json.RawMessage) Action {
	return Action{ID: "act_cf_c1", TenantID: 7, ActorID: "u1", ConnectionID: "conn-cf",
		Version: "confluence/v1", Target: "parent-1", Risk: RiskWrite, Args: args}
}

func TestParseConfluenceCreateSnapshot(t *testing.T) {
	snap, err := ParseConfluenceCreateSnapshot(cfCreateArgs("parent-1", "T", "<p>x</p>"))
	if err != nil || snap.Parent != "parent-1" || snap.Title != "T" || snap.Storage != "<p>x</p>" {
		t.Fatalf("snapshot fields drift: %+v %v", snap, err)
	}
	raw := append([]byte{}, cfCreateArgs("p", "T", "s")...)
	raw = raw[:len(raw)-1]
	raw = append(raw, []byte(`,"evil":"x"}`)...)
	if _, err := ParseConfluenceCreateSnapshot(raw); !errors.Is(err, ErrConfluenceSnapshotInvalid) {
		t.Fatalf("approve-then-rewrite must be refused, got %v", err)
	}
	for name, raw := range map[string]json.RawMessage{
		"missing parent": []byte(`{"title":"T","storage":"s"}`),
		"missing title":  []byte(`{"parent":"p","storage":"s"}`),
		"empty storage":  cfCreateArgs("p", "T", ""),
		"empty parent":   cfCreateArgs("", "T", "s"),
		"two fields":     []byte(`{"parent":"p","title":"T"}`),
		"not an object":  []byte(`["parent"]`),
	} {
		if _, err := ParseConfluenceCreateSnapshot(raw); !errors.Is(err, ErrConfluenceSnapshotInvalid) {
			t.Fatalf("%s: want ErrConfluenceSnapshotInvalid, got %v", name, err)
		}
	}
}

func TestIsConfluenceCreateArgs(t *testing.T) {
	if !IsConfluenceCreateArgs(cfCreateArgs("p", "T", "s")) {
		t.Fatal("create-shaped args must be detected")
	}
	update, _ := json.Marshal(map[string]any{"page_id": "p", "expected_version": "3", "title": "T", "storage": "s"})
	if IsConfluenceCreateArgs(update) {
		t.Fatal("update-shaped args must not be treated as create")
	}
	if IsConfluenceCreateArgs([]byte(`not json`)) {
		t.Fatal("garbage must not be create")
	}
}

func TestConfluenceCreateCloudHappyPath(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "ENG", "Parent", 7)
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "")

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "Report", "<p>hello</p>")))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("create: state=%s err=%v", out.State, err)
	}
	if out.ExternalID == "" || out.ExternalID == "parent-1" {
		t.Fatalf("external id must be the NEW page: %+v", out)
	}
	rcpt, rerr := ParseConfluencePageReceipt(out.Output)
	if rerr != nil || rcpt.ExternalID != out.ExternalID || rcpt.ExternalVersion != "1" {
		t.Fatalf("output must be a receipt-bearing page payload: %+v %v", rcpt, rerr)
	}
	posts, puts, gets := fake.stats()
	if posts != 1 || puts != 0 || gets != 1 {
		t.Fatalf("wire counts: posts=%d puts=%d gets=%d (the parent read must run)", posts, puts, gets)
	}
	fake.mu.Lock()
	created := fake.pages[out.ExternalID]
	fake.mu.Unlock()
	if created == nil || created.spaceID != "sp-1" || created.title != "Report" || created.storage != "<p>hello</p>" {
		t.Fatalf("created page drift: %+v", created)
	}
}

func TestConfluenceCreateCloudWikiContextPath(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "ENG", "Parent", 7)
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "/wiki")

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "Report", "<p>x</p>")))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("create under /wiki context path: state=%s err=%v", out.State, err)
	}
	posts, _, _ := fake.stats()
	if posts != 1 {
		t.Fatalf("posts=%d", posts)
	}
}

func TestConfluenceCreateServerHappyPath(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "42", "ENG", "Parent", 3)
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionServer, "")

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "Report", "<p>srv</p>")))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("server create: state=%s err=%v", out.State, err)
	}
	posts, puts, gets := fake.stats()
	if posts != 1 || puts != 0 || gets != 1 {
		t.Fatalf("wire counts: posts=%d puts=%d gets=%d", posts, puts, gets)
	}
	fake.mu.Lock()
	created := fake.pages[out.ExternalID]
	fake.mu.Unlock()
	if created == nil || created.spaceKey != "ENG" || created.storage != "<p>srv</p>" {
		t.Fatalf("created page drift: %+v", created)
	}
}

func TestConfluenceCreateParentOutOfScope(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-unlisted", "sp-9", "OPS", "Other", 1)
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "")

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-unlisted", "T", "s")))
	if !errors.Is(err, ErrConfluenceParentOutOfScope) || out.State != ActionFailed {
		t.Fatalf("unlisted parent must fail closed: state=%s err=%v", out.State, err)
	}
	posts, _, _ := fake.stats()
	if posts != 0 {
		t.Fatalf("zero writes after scope refusal, got %d posts", posts)
	}
}

func TestConfluenceCreateCapabilityMissing(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "ENG", "Parent", 7)
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "")
	ad.ConnectionCapabilities = cfCaps()

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "T", "s")))
	if !errors.Is(err, ErrConfluenceMissingCapability) || out.State != ActionFailed {
		t.Fatalf("missing capability must fail closed: state=%s err=%v", out.State, err)
	}
	posts, _, gets := fake.stats()
	if posts != 0 || gets != 0 {
		t.Fatalf("no request may leave without the capability: posts=%d gets=%d", posts, gets)
	}
}

func TestConfluenceCreateUnreadableParentIsFailedNotUnknown(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "")

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-missing", "T", "s")))
	if out.State != ActionFailed || err == nil {
		t.Fatalf("missing parent (a GET that never wrote) must fail definitively: state=%s err=%v", out.State, err)
	}
	posts, _, _ := fake.stats()
	if posts != 0 {
		t.Fatalf("no write may follow an unreadable parent: %d", posts)
	}
}

func TestConfluenceCreateUnknownOnLostCreateReply(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "ENG", "Parent", 7)
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "")
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "T", "s")))
	if out.State != ActionUnknown || !errors.Is(err, ErrConfluenceOutcomeUnknown) {
		t.Fatalf("lost create reply must park unknown, got state=%s err=%v", out.State, err)
	}
	// The effect DID land (the fake applies then drops): a blind second
	// create would duplicate the page — which is exactly why this state is
	// unknown and resolves only via human reconciliation.
	posts, _, _ := fake.stats()
	if posts != 1 {
		t.Fatalf("the dropped post counted once: %d", posts)
	}
}

func TestConfluenceCreateQueryStaysUnknown(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "")

	q, _ := ad.Query(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "T", "s")))
	if q.State != ActionUnknown {
		t.Fatalf("a create without a confirmed page id can never be proven — must stay unknown, got %s", q.State)
	}
}

func TestConfluenceCreateRejectsBadEdition(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, "gopher", "")

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "T", "s")))
	if out.State != ActionFailed || err == nil {
		t.Fatalf("unknown edition must fail closed pre-send: state=%s err=%v", out.State, err)
	}
	posts, _, gets := fake.stats()
	if posts != 0 || gets != 0 {
		t.Fatalf("no request may leave: posts=%d gets=%d", posts, gets)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/appconnector/ -run 'ConfluenceCreate|ParseConfluenceCreateSnapshot|IsConfluenceCreateArgs' -count=1`
Expected: FAIL（`undefined: ConfluenceCreateAdapter`）。

- [ ] **Step 3: 最小实现**

创建 `internal/modules/appconnector/confluence_create.go`：

```go
package appconnector

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
)

// NO-CF fixed contract, validated against the official Confluence REST
// documents (developer.atlassian.com): Cloud v2 pages under
// {base}/api/v2, Server/DC content under {base}/rest/api. These are the
// reviewed paths the adapter family is allowed to use; nothing here is
// derived from model output.
const (
	ConfluenceCloudPagesPath    = "/api/v2/pages"
	ConfluenceServerContentPath = "/rest/api/content"
)

// ConfluenceCreateSnapshot is the A03-approved argument snapshot for ONE
// page creation: exactly parent (the reviewed parent page id), title and
// storage (the deterministic Confluence storage-format body derived from
// the artifact by the publish seam). Every dispatched request is built
// FROM these fields; nothing is added, rewritten or re-derived after
// approval.
type ConfluenceCreateSnapshot struct {
	Parent  string
	Title   string
	Storage string
}

// ParseConfluenceCreateSnapshot validates that args are EXACTLY the
// approved three-field create snapshot.
func ParseConfluenceCreateSnapshot(args json.RawMessage) (ConfluenceCreateSnapshot, error) {
	var s ConfluenceCreateSnapshot
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return s, fmt.Errorf("%w: %v", ErrConfluenceSnapshotInvalid, err)
	}
	if len(raw) != 3 {
		return s, fmt.Errorf("%w: snapshot must be exactly parent, title, storage", ErrConfluenceSnapshotInvalid)
	}
	for _, f := range []struct {
		name string
		dst  *string
	}{{"parent", &s.Parent}, {"title", &s.Title}, {"storage", &s.Storage}} {
		if err := json.Unmarshal(raw[f.name], f.dst); err != nil {
			return s, fmt.Errorf("%w: %s: %v", ErrConfluenceSnapshotInvalid, f.name, err)
		}
	}
	if s.Parent == "" || s.Title == "" || s.Storage == "" {
		return s, fmt.Errorf("%w: parent, title and storage must be non-empty", ErrConfluenceSnapshotInvalid)
	}
	return s, nil
}

// IsConfluenceCreateArgs reports whether args carry the create snapshot's
// distinguishing key pair (parent AND storage, with no page_id). Routing
// only — full validation happens in ParseConfluenceCreateSnapshot.
func IsConfluenceCreateArgs(args json.RawMessage) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return false
	}
	_, hasParent := raw["parent"]
	_, hasStorage := raw["storage"]
	_, hasPage := raw["page_id"]
	return hasParent && hasStorage && !hasPage
}

// Wire bodies. Cloud v2 create carries space_id + optional parent_id; the
// Server/DC create carries the space key + ancestor list. Content travels
// as Confluence storage format (XHTML) in both.
type confluenceStorageBody struct {
	Representation string `json:"representation"`
	Value          string `json:"value"`
}

type confluenceCloudCreateRequest struct {
	SpaceID  string                `json:"space_id"`
	ParentID string                `json:"parent_id,omitempty"`
	Status   string                `json:"status"`
	Title    string                `json:"title"`
	Body     confluenceStorageBody `json:"body"`
}

type confluenceSpaceKeyRef struct {
	Key string `json:"key"`
}

type confluenceIDRef struct {
	ID string `json:"id"`
}

type confluenceServerStorageBody struct {
	Value          string `json:"value"`
	Representation string `json:"representation"`
}

type confluenceServerCreateRequest struct {
	Type      string                      `json:"type"`
	Space     confluenceSpaceKeyRef       `json:"space"`
	Ancestors []confluenceIDRef           `json:"ancestors"`
	Title     string                      `json:"title"`
	Body      confluenceServerStorageBody `json:"body"`
}

// confluenceTargetURL builds one request URL under the reviewed policy's
// origin and the configured context path.
func confluenceTargetURL(pol HTTPPolicy, apiBasePath, path, query string) *url.URL {
	host := pol.Host
	if pol.Port != "" {
		host = net.JoinHostPort(host, pol.Port)
	}
	return &url.URL{Scheme: pol.Scheme, Host: host, Path: apiBasePath + path, RawQuery: query}
}

// confluenceDo performs ONE policy-validated request with HTTP Basic auth
// — the same request/redirect re-validation as the Notion family (A04).
// Transport failures wrap ErrConfluenceOutcomeUnknown: the effect may
// already exist remotely, so it is unknown, never failed.
func confluenceDo(ctx context.Context, pol HTTPPolicy, cred func(context.Context) (ConfluenceCredential, error), method string, u *url.URL, body []byte) (int, []byte, error) {
	if err := pol.ValidateRequest(method, u); err != nil {
		return 0, nil, err // policy denial: the request never leaves
	}
	c, err := cred(ctx)
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
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.Username+":"+c.Secret)))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := pol.NewClient().Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %v", ErrConfluenceOutcomeUnknown, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("%w: %v", ErrConfluenceOutcomeUnknown, err)
	}
	return resp.StatusCode, raw, nil
}

// confluenceReadPage reads ONE page across both editions, always asking
// for the storage body (the update reconciliation compares it). Read
// failures are returned verbatim: a GET can never have produced a write,
// so callers may treat them as definitive.
func confluenceReadPage(ctx context.Context, pol HTTPPolicy, cred func(context.Context) (ConfluenceCredential, error), edition, apiBasePath, pageID string) (ConfluencePageVersion, error) {
	var u *url.URL
	if edition == EditionServer {
		u = confluenceTargetURL(pol, apiBasePath, ConfluenceServerContentPath+"/"+url.PathEscape(pageID), "expand=body.storage,version,space")
	} else {
		u = confluenceTargetURL(pol, apiBasePath, ConfluenceCloudPagesPath+"/"+url.PathEscape(pageID), "body-format=storage")
	}
	status, raw, err := confluenceDo(ctx, pol, cred, http.MethodGet, u, nil)
	if err != nil {
		return ConfluencePageVersion{}, err
	}
	if status != http.StatusOK {
		return ConfluencePageVersion{}, fmt.Errorf("confluence_provider_error: status=%d", status)
	}
	return ParseConfluencePageVersion(raw)
}

// ConfluenceCreateAdapter is the Confluence member of the A04 Adapter
// family. It executes ONE approved page creation against the reviewed
// contract as a SINGLE write: read the parent page (existence proof +
// destination space resolution), then POST the storage body. There is no
// multi-step progress to persist — a lost create reply parks unknown, and
// no confirmed created-page id exists anywhere to resume from.
type ConfluenceCreateAdapter struct {
	// Policy is the admin-reviewed outbound contract (A04).
	Policy HTTPPolicy
	// Credential returns the connection's Basic credential after the
	// permission guard has passed.
	Credential func(ctx context.Context) (ConfluenceCredential, error)
	// Edition selects the wire: "" / "cloud" → {base}/api/v2, "server" →
	// {base}/rest/api. Anything else fails closed pre-send.
	Edition string
	// APIBasePath is "" or a "/context" path (e.g. "/wiki").
	APIBasePath string
	// ApprovedParents is the approval / pre-authorization scope: the page
	// ids a creation may be parented under. Empty fails closed for every
	// parent.
	ApprovedParents []string
	// ConnectionCapabilities reports the connection's granted capabilities;
	// the reviewed write capability is required before any request. Nil
	// fails closed.
	ConnectionCapabilities func(ctx context.Context, a Action) ([]string, error)
	// Recheck re-validates the A03 approval right before any request; a
	// revocation between approval and execute blocks the creation.
	Recheck func(ctx context.Context, a Action) error
}

var _ Adapter = (*ConfluenceCreateAdapter)(nil)

func (m *ConfluenceCreateAdapter) configError() error {
	if m.Policy.Host == "" || m.Policy.Scheme == "" {
		return fmt.Errorf("%w: no reviewed outbound policy", ErrConfluenceNotConfigured)
	}
	if m.Credential == nil {
		return fmt.Errorf("%w: no credential source", ErrConfluenceNotConfigured)
	}
	return nil
}

func (m *ConfluenceCreateAdapter) editionName() (string, error) {
	switch m.Edition {
	case "", EditionCloud:
		return EditionCloud, nil
	case EditionServer:
		return EditionServer, nil
	default:
		return "", fmt.Errorf("%w: edition %q", ErrConfluenceNotConfigured, m.Edition)
	}
}

// parentApproved fails closed: only a parent explicitly listed in the
// approved scope may receive the new page.
func (m *ConfluenceCreateAdapter) parentApproved(parent string) bool {
	for _, p := range m.ApprovedParents {
		if p == parent {
			return true
		}
	}
	return false
}

func (m *ConfluenceCreateAdapter) requireWriteCapability(ctx context.Context, a Action) error {
	if m.ConnectionCapabilities == nil {
		return fmt.Errorf("%w: no capability source", ErrConfluenceMissingCapability)
	}
	caps, err := m.ConnectionCapabilities(ctx, a)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConfluenceMissingCapability, err)
	}
	for _, c := range caps {
		if c == ConfluenceCapabilityWrite {
			return nil
		}
	}
	return fmt.Errorf("%w: connection lacks %s", ErrConfluenceMissingCapability, ConfluenceCapabilityWrite)
}

// createPage performs the single create step. The body is built FROM the
// approved snapshot field-by-field plus the parent's resolved space —
// nothing else. A reply without a provable receipt (real id + version)
// is an unknown, never a success.
func (m *ConfluenceCreateAdapter) createPage(ctx context.Context, edition string, snap ConfluenceCreateSnapshot, parent ConfluencePageVersion) ([]byte, error) {
	var body []byte
	var path string
	var err error
	if edition == EditionServer {
		body, err = json.Marshal(confluenceServerCreateRequest{
			Type:      "page",
			Space:     confluenceSpaceKeyRef{Key: parent.SpaceKey},
			Ancestors: []confluenceIDRef{{ID: parent.PageID}},
			Title:     snap.Title,
			Body:      confluenceServerStorageBody{Value: snap.Storage, Representation: "storage"},
		})
		path = ConfluenceServerContentPath
	} else {
		body, err = json.Marshal(confluenceCloudCreateRequest{
			SpaceID:  parent.SpaceID,
			ParentID: parent.PageID,
			Status:   "current",
			Title:    snap.Title,
			Body:     confluenceStorageBody{Representation: "storage", Value: snap.Storage},
		})
		path = ConfluenceCloudPagesPath
	}
	if err != nil {
		return nil, err
	}
	u := confluenceTargetURL(m.Policy, m.APIBasePath, path, "")
	status, raw, derr := confluenceDo(ctx, m.Policy, m.Credential, http.MethodPost, u, body)
	if derr != nil {
		return nil, derr
	}
	if status >= 400 {
		if status >= 500 {
			// A 5xx leaves the create ambiguous: the page may exist.
			return nil, fmt.Errorf("%w: status=%d", ErrConfluenceOutcomeUnknown, status)
		}
		return nil, fmt.Errorf("confluence_provider_error: status=%d", status)
	}
	return raw, nil
}

// Execute performs the approved creation. Order: A03 recheck, snapshot
// validation, parent-scope check, capability check, parent read (existence
// + space resolution — a GET that can never write) — all BEFORE the single
// network write.
func (m *ConfluenceCreateAdapter) Execute(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	edition, ederr := m.editionName()
	if ederr != nil {
		return ActionResult{State: ActionFailed}, ederr
	}
	if m.Recheck != nil {
		if err := m.Recheck(ctx, a); err != nil {
			return ActionResult{State: ActionAwaitingApproval}, fmt.Errorf("%w: %v", ErrConfluenceApprovalRevoked, err)
		}
	}
	snap, err := ParseConfluenceCreateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if !m.parentApproved(snap.Parent) {
		return ActionResult{State: ActionFailed}, fmt.Errorf("%w: parent %q outside the approved page scope", ErrConfluenceParentOutOfScope, snap.Parent)
	}
	if err := m.requireWriteCapability(ctx, a); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	parent, rerr := confluenceReadPage(ctx, m.Policy, m.Credential, edition, m.APIBasePath, snap.Parent)
	if rerr != nil {
		// A read can never have produced the write: definitive failure.
		return ActionResult{State: ActionFailed}, rerr
	}
	raw, cerr := m.createPage(ctx, edition, snap, parent)
	if cerr != nil {
		state := ActionFailed
		if errors.Is(cerr, ErrConfluenceOutcomeUnknown) {
			state = ActionUnknown
		}
		return ActionResult{State: state}, cerr
	}
	rcpt, rerr := ParseConfluencePageReceipt(raw)
	if rerr != nil {
		// The POST may have applied while the reply proves nothing: the
		// honest state is unknown.
		return ActionResult{State: ActionUnknown}, rerr
	}
	return ActionResult{State: ActionSucceeded, ExternalID: rcpt.ExternalID, Output: json.RawMessage(raw)}, nil
}

// Query is the reconciliation entry point for a creation parked in
// unknown. A single-step create that lost its reply leaves NO confirmed
// created-page id anywhere in the durable record (Confluence create has
// no idempotency key), so no read can prove WHICH page — if any — this
// action produced. The honest outcome is unknown; resolution is human
// reconciliation (the Notion family's advisory title search is equally
// non-proving and is therefore not even attempted here).
func (m *ConfluenceCreateAdapter) Query(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if _, err := ParseConfluenceCreateSnapshot(a.Args); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	return ActionResult{State: ActionUnknown}, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/appconnector/ -run 'ConfluenceCreate|ParseConfluenceCreateSnapshot|IsConfluenceCreateArgs' -count=1`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/appconnector/confluence_create.go internal/modules/appconnector/confluence_create_test.go
git commit -m "feat(appconnector): confluence create adapter - single-step page create with parent scope gate (T20 #50)"
```

---

### Task 3: `ConfluenceUpdateAdapter`——版本预读、冲突拒绝、单步 PUT 与 Query 对账

**Files:**
- Create: `internal/modules/appconnector/confluence_update.go`
- Test: `internal/modules/appconnector/confluence_update_test.go`

**Interfaces:**
- Consumes: Task 1 全部公共层符号 + Task 2 的 `confluenceTargetURL`/`confluenceDo`/`confluenceReadPage`/`ConfluenceCloudPagesPath`/`ConfluenceServerContentPath`。
- Produces（Task 5/8/9 依赖，签名逐字）:
  - `type ConfluenceUpdateSnapshot struct { PageID string; ExpectedVersion string; Title string; Storage string }`
  - `func ParseConfluenceUpdateSnapshot(args json.RawMessage) (ConfluenceUpdateSnapshot, error)`
  - `func IsConfluenceUpdateArgs(args json.RawMessage) bool`
  - `type ConfluenceUpdateAdapter struct { Policy HTTPPolicy; Credential func(ctx context.Context) (ConfluenceCredential, error); Edition string; APIBasePath string; ConnectionCapabilities func(ctx context.Context, a Action) ([]string, error); Recheck func(ctx context.Context, a Action) error }`
  - `func (m *ConfluenceUpdateAdapter) Execute(ctx context.Context, a Action) (ActionResult, error)`；`func (m *ConfluenceUpdateAdapter) Query(ctx context.Context, a Action) (ActionResult, error)`；`var _ Adapter = (*ConfluenceUpdateAdapter)(nil)`
  - `func ReadConfluencePageVersion(ctx context.Context, pol HTTPPolicy, cred func(context.Context) (ConfluenceCredential, error), edition, apiBasePath, pageID string) (string, error)`

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/appconnector/confluence_update_test.go`：

```go
package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func cfUpdateArgs(pageID, expectedVersion, title, storage string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"page_id": pageID, "expected_version": expectedVersion,
		"title": title, "storage": storage,
	})
	return raw
}

func cfUpdateAction(args json.RawMessage) Action {
	return Action{ID: "act_cf_u1", TenantID: 7, ActorID: "u1", ConnectionID: "conn-cf",
		Version: "confluence/v1", Target: "page-9", Risk: RiskWrite, Args: args}
}

func TestParseConfluenceUpdateSnapshot(t *testing.T) {
	snap, err := ParseConfluenceUpdateSnapshot(cfUpdateArgs("page-9", "3", "T", "<p>x</p>"))
	if err != nil || snap.PageID != "page-9" || snap.ExpectedVersion != "3" || snap.Title != "T" || snap.Storage != "<p>x</p>" {
		t.Fatalf("snapshot fields drift: %+v %v", snap, err)
	}
	raw := append([]byte{}, cfUpdateArgs("p", "3", "T", "s")...)
	raw = raw[:len(raw)-1]
	raw = append(raw, []byte(`,"evil":"x"}`)...)
	if _, err := ParseConfluenceUpdateSnapshot(raw); !errors.Is(err, ErrConfluenceSnapshotInvalid) {
		t.Fatalf("approve-then-rewrite must be refused, got %v", err)
	}
	for name, raw := range map[string]json.RawMessage{
		"missing page":    []byte(`{"expected_version":"3","title":"T","storage":"s"}`),
		"missing version": []byte(`{"page_id":"p","title":"T","storage":"s"}`),
		"empty version":   cfUpdateArgs("p", "", "T", "s"),
		"empty page":      cfUpdateArgs("", "3", "T", "s"),
		"empty storage":   cfUpdateArgs("p", "3", "T", ""),
		"three fields":    []byte(`{"page_id":"p","title":"T","storage":"s"}`),
		"not an object":   []byte(`["page_id"]`),
	} {
		if _, err := ParseConfluenceUpdateSnapshot(raw); !errors.Is(err, ErrConfluenceSnapshotInvalid) {
			t.Fatalf("%s: want ErrConfluenceSnapshotInvalid, got %v", name, err)
		}
	}
}

func TestIsConfluenceUpdateArgs(t *testing.T) {
	if !IsConfluenceUpdateArgs(cfUpdateArgs("p", "3", "T", "s")) {
		t.Fatal("update-shaped args must be detected")
	}
	// The Notion update shape (page_id + expected_version but NO storage)
	// must never be treated as a Confluence update — the storage key is the
	// edition-bridging discriminator.
	notion, _ := json.Marshal(map[string]any{"page_id": "p", "expected_version": "v", "title": "T", "blocks": []any{}})
	if IsConfluenceUpdateArgs(notion) {
		t.Fatal("notion-shaped args must not be confluence updates")
	}
	if IsConfluenceUpdateArgs(cfCreateArgs("p", "T", "s")) {
		t.Fatal("create-shaped args must not be updates")
	}
	if IsConfluenceUpdateArgs([]byte(`garbage`)) {
		t.Fatal("garbage must not be update")
	}
}

func TestConfluenceUpdateCloudHappyPath(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}

	out, err := ad.Execute(context.Background(), cfUpdateAction(cfUpdateArgs("page-9", "3", "new title", "<p>next</p>")))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("update: state=%s err=%v", out.State, err)
	}
	if out.ExternalID != "page-9" {
		t.Fatalf("external id: %+v", out)
	}
	rcpt, rerr := ParseConfluencePageReceipt(out.Output)
	if rerr != nil || rcpt.ExternalID != "page-9" || rcpt.ExternalVersion != "4" {
		t.Fatalf("receipt must carry the version THIS publish produced: %+v %v", rcpt, rerr)
	}
	posts, puts, gets := fake.stats()
	if puts != 1 || gets < 1 || posts != 0 {
		t.Fatalf("wire counts: posts=%d puts=%d gets=%d (pre-read GET must run)", posts, puts, gets)
	}
	fake.mu.Lock()
	p := fake.pages["page-9"]
	fake.mu.Unlock()
	if p.title != "new title" || p.storage != "<p>next</p>" || p.version != 4 {
		t.Fatalf("remote page drift: %+v", p)
	}
}

func TestConfluenceUpdateServerHappyPath(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "42", "ENG", "old title", 5)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionServer, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}

	out, err := ad.Execute(context.Background(), cfUpdateAction(cfUpdateArgs("page-9", "5", "v2", "<p>srv</p>")))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("server update: state=%s err=%v", out.State, err)
	}
	rcpt, rerr := ParseConfluencePageReceipt(out.Output)
	if rerr != nil || rcpt.ExternalVersion != "6" {
		t.Fatalf("server receipt drift: %+v %v", rcpt, rerr)
	}
	_, puts, _ := fake.stats()
	if puts != 1 {
		t.Fatalf("puts=%d", puts)
	}
}

func TestConfluenceUpdateConflictZeroWrites(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
	// External collaborator edited AFTER the plan was formed: version 3 → 4.
	fake.bump("page-9", "<p>external edit</p>")

	out, err := ad.Execute(context.Background(), cfUpdateAction(cfUpdateArgs("page-9", "3", "new title", "<p>mine</p>")))
	if !errors.Is(err, ErrConfluenceVersionConflict) || out.State != ActionFailed {
		t.Fatalf("version drift must fail definitively: state=%s err=%v", out.State, err)
	}
	posts, puts, gets := fake.stats()
	if posts != 0 || puts != 0 {
		t.Fatalf("AC1: conflict must leave ZERO write requests, got posts=%d puts=%d", posts, puts)
	}
	if gets == 0 {
		t.Fatal("the pre-read version GET must have run")
	}
	fake.mu.Lock()
	p := fake.pages["page-9"]
	fake.mu.Unlock()
	if p.storage != "<p>external edit</p>" || p.version != 4 {
		t.Fatalf("the external edit must be untouched: %+v", p)
	}
}

func TestConfluenceUpdatePreReadFailureIsFailedNotUnknown(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}

	out, err := ad.Execute(context.Background(), cfUpdateAction(cfUpdateArgs("page-missing", "3", "T", "s")))
	if out.State != ActionFailed || err == nil {
		t.Fatalf("missing page (a GET that never wrote) must fail definitively: state=%s err=%v", out.State, err)
	}
	_, puts, _ := fake.stats()
	if puts != 0 {
		t.Fatalf("no write may follow an unreadable pre-read: %d", puts)
	}
}

func TestConfluenceUpdateUnknownOnLostWriteReply(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()

	out, err := ad.Execute(context.Background(), cfUpdateAction(cfUpdateArgs("page-9", "3", "new title", "<p>next</p>")))
	if out.State != ActionUnknown || !errors.Is(err, ErrConfluenceOutcomeUnknown) {
		t.Fatalf("lost write reply must park unknown, got state=%s err=%v", out.State, err)
	}
	fake.mu.Lock()
	p := fake.pages["page-9"]
	fake.mu.Unlock()
	if p.version != 4 || p.storage != "<p>next</p>" {
		t.Fatalf("the effect DID apply (the fake drops the reply after): %+v", p)
	}
}

func TestConfluenceUpdateQueryResolvesAfterDroppedReply(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()
	act := cfUpdateAction(cfUpdateArgs("page-9", "3", "new title", "<p>next</p>"))
	out, err := ad.Execute(context.Background(), act)
	if out.State != ActionUnknown {
		t.Fatalf("dropped reply must park unknown, got %s (%v)", out.State, err)
	}
	_, putsBefore, _ := fake.stats()
	// AC2: reconcile by READING the remote first — version advanced by
	// exactly one AND the content is ours, so the query must confirm
	// success without any new write.
	q, qerr := ad.Query(context.Background(), act)
	if qerr != nil || q.State != ActionSucceeded || q.ExternalID != "page-9" {
		t.Fatalf("query must resolve the unknown from the remote state: %+v %v", q, qerr)
	}
	rcpt, rerr := ParseConfluencePageReceipt(q.Output)
	if rerr != nil || rcpt.ExternalVersion != "4" {
		t.Fatalf("query receipt drift: %+v %v", rcpt, rerr)
	}
	_, putsAfter, _ := fake.stats()
	if putsAfter != putsBefore {
		t.Fatalf("query must not re-send: puts %d -> %d", putsBefore, putsAfter)
	}
}

func TestConfluenceUpdateQueryStaysUnknownWhenContentDrifted(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()
	act := cfUpdateAction(cfUpdateArgs("page-9", "3", "new title", "<p>next</p>"))
	if out, _ := ad.Execute(context.Background(), act); out.State != ActionUnknown {
		t.Fatalf("setup: unknown expected, got %s", out.State)
	}
	// A SECOND external edit lands on top of ours: version moved past
	// expected+1 AND content no longer matches — not provably ours anymore.
	fake.bump("page-9", "<p>someone else</p>")
	q, _ := ad.Query(context.Background(), act)
	if q.State != ActionUnknown {
		t.Fatalf("drifted content must stay unknown, got %s", q.State)
	}
}

func TestConfluenceUpdateQueryStaysUnknownWhenVersionUnmoved(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
	// No write ever happened: version still 3 — the lost PUT provably did
	// not apply, but the ADAPTER's Query still reports unknown (definitive
	// failed settlement is the pipeline's business, not the query's).
	q, _ := ad.Query(context.Background(), cfUpdateAction(cfUpdateArgs("page-9", "3", "new title", "<p>next</p>")))
	if q.State != ActionUnknown {
		t.Fatalf("unmoved version must stay unknown at adapter level, got %s", q.State)
	}
}

func TestReadConfluencePageVersion(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "P", 4)
	srv := fake.server(t)
	v, err := ReadConfluencePageVersion(context.Background(), cfPolicyOf(srv, ""), cfCredential, EditionCloud, "", "page-9")
	if err != nil || v != "4" {
		t.Fatalf("pre-read drift: %q %v", v, err)
	}
	if _, err := ReadConfluencePageVersion(context.Background(), cfPolicyOf(srv, ""), cfCredential, EditionCloud, "", "missing"); err == nil {
		t.Fatal("missing page must error, never fabricate a version")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/appconnector/ -run 'ConfluenceUpdate|TestReadConfluencePageVersion' -count=1`
Expected: FAIL（`undefined: ConfluenceUpdateAdapter`）。

- [ ] **Step 3: 最小实现**

创建 `internal/modules/appconnector/confluence_update.go`：

```go
package appconnector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// ConfluenceUpdateSnapshot is the A03-approved argument snapshot for
// updating ONE existing external page: exactly page_id, expected_version
// (the version.number the plan read before approval — an immutable
// approval anchor), title and storage. Every dispatched request is built
// FROM these fields; nothing is added, rewritten or re-derived after
// approval.
type ConfluenceUpdateSnapshot struct {
	PageID          string
	ExpectedVersion string
	Title           string
	Storage         string
}

// ParseConfluenceUpdateSnapshot validates that args are EXACTLY the
// approved four-field update snapshot.
func ParseConfluenceUpdateSnapshot(args json.RawMessage) (ConfluenceUpdateSnapshot, error) {
	var s ConfluenceUpdateSnapshot
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return s, fmt.Errorf("%w: %v", ErrConfluenceSnapshotInvalid, err)
	}
	if len(raw) != 4 {
		return s, fmt.Errorf("%w: snapshot must be exactly page_id, expected_version, title, storage", ErrConfluenceSnapshotInvalid)
	}
	for _, f := range []struct {
		name string
		dst  *string
	}{{"page_id", &s.PageID}, {"expected_version", &s.ExpectedVersion}, {"title", &s.Title}, {"storage", &s.Storage}} {
		if err := json.Unmarshal(raw[f.name], f.dst); err != nil {
			return s, fmt.Errorf("%w: %s: %v", ErrConfluenceSnapshotInvalid, f.name, err)
		}
	}
	if s.PageID == "" || s.ExpectedVersion == "" || s.Title == "" || s.Storage == "" {
		return s, fmt.Errorf("%w: page_id, expected_version, title and storage must be non-empty", ErrConfluenceSnapshotInvalid)
	}
	return s, nil
}

// IsConfluenceUpdateArgs reports whether args carry the update snapshot's
// distinguishing keys: page_id AND expected_version AND storage. The
// storage key is what separates a Confluence update from a Notion update
// (page_id + expected_version) if bytes ever cross bridges; full
// validation still happens in ParseConfluenceUpdateSnapshot.
func IsConfluenceUpdateArgs(args json.RawMessage) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return false
	}
	_, hasPage := raw["page_id"]
	_, hasVersion := raw["expected_version"]
	_, hasStorage := raw["storage"]
	return hasPage && hasVersion && hasStorage
}

type confluenceVersionRef struct {
	Number  int    `json:"number"`
	Message string `json:"message,omitempty"`
}

type confluenceCloudUpdateRequest struct {
	ID      string                `json:"id"`
	Status  string                `json:"status"`
	Title   string                `json:"title"`
	Body    confluenceStorageBody `json:"body"`
	Version confluenceVersionRef  `json:"version"`
}

type confluenceServerUpdateRequest struct {
	Type    string                      `json:"type"`
	ID      string                      `json:"id"`
	Title   string                      `json:"title"`
	Body    confluenceServerStorageBody `json:"body"`
	Version confluenceVersionRef        `json:"version"`
}

// confluenceNextVersion parses the approved expected version and returns
// the NEXT number a real instance demands on update (Confluence rejects a
// PUT whose version.number is not current+1).
func confluenceNextVersion(expected string) (int, error) {
	n, err := strconv.Atoi(expected)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%w: expected_version %q is not a real page version", ErrConfluenceSnapshotInvalid, expected)
	}
	return n + 1, nil
}

// ConfluenceUpdateAdapter executes ONE approved page update against the
// reviewed contract family: the update is a TWO-request sequence — read
// the current version (the conflict gate), then PUT the storage body with
// version.number+1. The version pre-read is the publish conflict gate:
// the remote version.number must still equal the snapshot's approved
// expected_version, otherwise the update is refused with
// ErrConfluenceVersionConflict and ZERO write requests leave the process.
//
// Outcome semantics (identical to the create adapter):
//   - pre-read failures are definitive FAILED (a GET cannot have produced
//     the write; nothing left the process);
//   - the PUT's transport failure / 5xx / unparseable reply is
//     ErrConfluenceOutcomeUnknown — the effect may exist remotely;
//   - Query reconciles ONLY via the reliable page read: version moved by
//     exactly one AND title AND storage all still ours → success.
type ConfluenceUpdateAdapter struct {
	Policy      HTTPPolicy
	Credential  func(ctx context.Context) (ConfluenceCredential, error)
	Edition     string
	APIBasePath string
	ConnectionCapabilities func(ctx context.Context, a Action) ([]string, error)
	Recheck     func(ctx context.Context, a Action) error
}

var _ Adapter = (*ConfluenceUpdateAdapter)(nil)

func (m *ConfluenceUpdateAdapter) configError() error {
	if m.Policy.Host == "" || m.Policy.Scheme == "" {
		return fmt.Errorf("%w: no reviewed outbound policy", ErrConfluenceNotConfigured)
	}
	if m.Credential == nil {
		return fmt.Errorf("%w: no credential source", ErrConfluenceNotConfigured)
	}
	return nil
}

func (m *ConfluenceUpdateAdapter) editionName() (string, error) {
	switch m.Edition {
	case "", EditionCloud:
		return EditionCloud, nil
	case EditionServer:
		return EditionServer, nil
	default:
		return "", fmt.Errorf("%w: edition %q", ErrConfluenceNotConfigured, m.Edition)
	}
}

func (m *ConfluenceUpdateAdapter) requireWriteCapability(ctx context.Context, a Action) error {
	if m.ConnectionCapabilities == nil {
		return fmt.Errorf("%w: no capability source", ErrConfluenceMissingCapability)
	}
	caps, err := m.ConnectionCapabilities(ctx, a)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConfluenceMissingCapability, err)
	}
	for _, c := range caps {
		if c == ConfluenceCapabilityWrite {
			return nil
		}
	}
	return fmt.Errorf("%w: connection lacks %s", ErrConfluenceMissingCapability, ConfluenceCapabilityWrite)
}

// putPage performs the single update step with version.number+1.
func (m *ConfluenceUpdateAdapter) putPage(ctx context.Context, edition string, snap ConfluenceUpdateSnapshot) ([]byte, error) {
	next, nerr := confluenceNextVersion(snap.ExpectedVersion)
	if nerr != nil {
		return nil, nerr
	}
	var body []byte
	var u *url.URL
	var err error
	if edition == EditionServer {
		body, err = json.Marshal(confluenceServerUpdateRequest{
			Type: "page", ID: snap.PageID, Title: snap.Title,
			Body:    confluenceServerStorageBody{Value: snap.Storage, Representation: "storage"},
			Version: confluenceVersionRef{Number: next},
		})
		u = confluenceTargetURL(m.Policy, m.APIBasePath, ConfluenceServerContentPath+"/"+url.PathEscape(snap.PageID), "")
	} else {
		body, err = json.Marshal(confluenceCloudUpdateRequest{
			ID: snap.PageID, Status: "current", Title: snap.Title,
			Body:    confluenceStorageBody{Representation: "storage", Value: snap.Storage},
			Version: confluenceVersionRef{Number: next},
		})
		u = confluenceTargetURL(m.Policy, m.APIBasePath, ConfluenceCloudPagesPath+"/"+url.PathEscape(snap.PageID), "")
	}
	if err != nil {
		return nil, err
	}
	status, raw, derr := confluenceDo(ctx, m.Policy, m.Credential, http.MethodPut, u, body)
	if derr != nil {
		return nil, derr
	}
	if status >= 400 {
		if status >= 500 {
			return nil, fmt.Errorf("%w: status=%d", ErrConfluenceOutcomeUnknown, status)
		}
		return nil, fmt.Errorf("confluence_provider_error: status=%d", status)
	}
	return raw, nil
}

// Execute performs the approved update. Order: A03 recheck, snapshot
// validation, capability check, version PRE-READ + conflict detection —
// all BEFORE any network write — then the single PUT whose reply payload
// is the action's output evidence (the receipt basis).
func (m *ConfluenceUpdateAdapter) Execute(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	edition, ederr := m.editionName()
	if ederr != nil {
		return ActionResult{State: ActionFailed}, ederr
	}
	if m.Recheck != nil {
		if err := m.Recheck(ctx, a); err != nil {
			return ActionResult{State: ActionAwaitingApproval}, fmt.Errorf("%w: %v", ErrConfluenceApprovalRevoked, err)
		}
	}
	snap, err := ParseConfluenceUpdateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if err := m.requireWriteCapability(ctx, a); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	// AC1: read the external current version FIRST. A read can never have
	// produced the write, so an unreadable pre-read is a definitive
	// failure — zero write requests leave the process.
	cur, rerr := confluenceReadPage(ctx, m.Policy, m.Credential, edition, m.APIBasePath, snap.PageID)
	if rerr != nil {
		return ActionResult{State: ActionFailed}, rerr
	}
	if cerr := DetectConfluenceVersionConflict(snap.ExpectedVersion, cur.VersionNumber); cerr != nil {
		return ActionResult{State: ActionFailed}, cerr
	}
	raw, uerr := m.putPage(ctx, edition, snap)
	if uerr != nil {
		state := ActionFailed
		if errors.Is(uerr, ErrConfluenceOutcomeUnknown) {
			state = ActionUnknown
		}
		return ActionResult{State: state, ExternalID: snap.PageID}, uerr
	}
	rcpt, rerr := ParseConfluencePageReceipt(raw)
	if rerr != nil {
		// The PUT may have applied while the reply proves nothing: the
		// honest state is unknown.
		return ActionResult{State: ActionUnknown, ExternalID: snap.PageID}, rerr
	}
	return ActionResult{State: ActionSucceeded, ExternalID: rcpt.ExternalID, Output: json.RawMessage(raw)}, nil
}

// Query is the reconciliation entry point for an update parked in
// unknown: it reconciles ONLY via the reliable page read. Success is
// claimed only when the version advanced by EXACTLY one from the approved
// baseline AND the title AND the storage body all still match the
// approved snapshot; anything less stays the honest unknown.
func (m *ConfluenceUpdateAdapter) Query(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	snap, err := ParseConfluenceUpdateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	edition, ederr := m.editionName()
	if ederr != nil {
		return ActionResult{State: ActionUnknown}, ederr
	}
	next, nerr := confluenceNextVersion(snap.ExpectedVersion)
	if nerr != nil {
		return ActionResult{State: ActionUnknown}, nerr
	}
	cur, rerr := confluenceReadPage(ctx, m.Policy, m.Credential, edition, m.APIBasePath, snap.PageID)
	if rerr != nil {
		return ActionResult{State: ActionUnknown}, rerr
	}
	if cur.VersionNumber != strconv.Itoa(next) {
		return ActionResult{State: ActionUnknown}, fmt.Errorf("confluence_query_unverifiable: remote version %q, expected ours at %q", cur.VersionNumber, strconv.Itoa(next))
	}
	if cur.Title != snap.Title || cur.BodyStorage != snap.Storage {
		return ActionResult{State: ActionUnknown}, fmt.Errorf("confluence_query_unverifiable: remote content drifted from the approved snapshot")
	}
	raw, _ := json.Marshal(map[string]any{"id": cur.PageID, "version": map[string]int{"number": next}})
	return ActionResult{State: ActionSucceeded, ExternalID: cur.PageID, Output: json.RawMessage(raw)}, nil
}

// ReadConfluencePageVersion performs a one-off version read through the
// given reviewed policy and credential source — the plan-formation
// pre-read shared by the publish seam. It performs no write of any kind.
func ReadConfluencePageVersion(ctx context.Context, pol HTTPPolicy, cred func(context.Context) (ConfluenceCredential, error), edition, apiBasePath, pageID string) (string, error) {
	ver, err := confluenceReadPage(ctx, pol, cred, edition, apiBasePath, pageID)
	if err != nil {
		return "", err
	}
	return ver.VersionNumber, nil
}
```

**实现注意**：`errors.Is`（Execute 的 unknown 分支）来自 `errors`，`bytes.NewReader` 来自 `bytes`——两 import 已在上块；实现者以编译器为准做一次 `gofmt` 校对即可，语义以代码为准。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/appconnector/ -run 'ConfluenceUpdate|TestReadConfluencePageVersion' -count=1`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/appconnector/confluence_update.go internal/modules/appconnector/confluence_update_test.go
git commit -m "feat(appconnector): confluence update adapter - version pre-read conflict gate + single PUT (T20 #50)"
```

---

### Task 4: publish 包——Artifact 文本→Confluence storage 正文纯投影

**Files:**
- Create: `internal/modules/appconnector/publish/confluence_blocks.go`
- Test: `internal/modules/appconnector/publish/confluence_blocks_test.go`

**Interfaces:**
- Consumes: 同包既有 `MaxPublishBlocks = 500`、`ErrPublishContentTooLarge`、`ErrPublishEmptyContent`（`internal/modules/appconnector/publish/blocks.go:18-31`，零修改复用）。
- Produces（Task 6 依赖，签名逐字）: `func ConfluenceStorageBody(text string) (string, error)`

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/appconnector/publish/confluence_blocks_test.go`：

```go
package publish

import (
	"errors"
	"strings"
	"testing"
)

func TestConfluenceStorageBodyParagraphs(t *testing.T) {
	got, err := ConfluenceStorageBody("第一段。\n\n第二段。")
	if err != nil {
		t.Fatal(err)
	}
	if want := "<p>第一段。</p>\n<p>第二段。</p>"; got != want {
		t.Fatalf("storage body drift:\n got %q\nwant %q", got, want)
	}
}

func TestConfluenceStorageBodyNormalizesCRLF(t *testing.T) {
	got, err := ConfluenceStorageBody("a\r\n\r\nb")
	if err != nil || got != "<p>a</p>\n<p>b</p>" {
		t.Fatalf("CRLF normalization drift: %q %v", got, err)
	}
}

func TestConfluenceStorageBodyEscapesHTML(t *testing.T) {
	got, err := ConfluenceStorageBody(`<b>bold</b> & "quoted" 'single'`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "<b>") || !strings.Contains(got, "&lt;b&gt;") || !strings.Contains(got, "&amp;") {
		t.Fatalf("storage body must be HTML-escaped: %q", got)
	}
}

func TestConfluenceStorageBodyKeepsSingleNewlineInsideParagraph(t *testing.T) {
	got, err := ConfluenceStorageBody("line one\nline two")
	if err != nil || got != "<p>line one\nline two</p>" {
		t.Fatalf("single newline stays inside the paragraph (renders as whitespace): %q %v", got, err)
	}
}

func TestConfluenceStorageBodyEmpty(t *testing.T) {
	for _, in := range []string{"", "  \n \n  ", "\n\n"} {
		if _, err := ConfluenceStorageBody(in); !errors.Is(err, ErrPublishEmptyContent) {
			t.Fatalf("%q: want ErrPublishEmptyContent, got %v", in, err)
		}
	}
}

func TestConfluenceStorageBodyTooLarge(t *testing.T) {
	text := strings.Repeat("p\n\n", MaxPublishBlocks) // MaxPublishBlocks+1 paragraphs
	if _, err := ConfluenceStorageBody(text); !errors.Is(err, ErrPublishContentTooLarge) {
		t.Fatalf("want ErrPublishContentTooLarge, got %v", err)
	}
}

func TestConfluenceStorageBodyDeterministic(t *testing.T) {
	a, _ := ConfluenceStorageBody("x\n\ny\n\nz")
	b, _ := ConfluenceStorageBody("x\n\ny\n\nz")
	if a != b {
		t.Fatalf("the same artifact bytes must always produce the same storage string: %q vs %q", a, b)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/appconnector/publish/ -run 'TestConfluenceStorageBody' -count=1`
Expected: FAIL（`undefined: ConfluenceStorageBody`）。

- [ ] **Step 3: 最小实现**

创建 `internal/modules/appconnector/publish/confluence_blocks.go`：

```go
package publish

import (
	"errors"
	"fmt"
	"html"
	"strings"
)

// ConfluenceStorageBody derives the Confluence storage-format (XHTML)
// body from plain text: paragraphs split on blank lines, each trimmed and
// HTML-escaped, wrapped in <p> elements separated by newlines (a single
// newline inside a paragraph stays in place — Confluence renders it as
// whitespace, which matches the plain-text semantics). The output is
// deterministic pure derivation — the same artifact bytes always produce
// the same storage string, so the approval digest pins exactly what will
// be sent.
func ConfluenceStorageBody(text string) (string, error) {
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
		return "", ErrPublishEmptyContent
	}
	if len(paragraphs) > MaxPublishBlocks {
		return "", fmt.Errorf("%w: %d paragraphs exceed %d blocks", ErrPublishContentTooLarge, len(paragraphs), MaxPublishBlocks)
	}
	var b strings.Builder
	for i, p := range paragraphs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("<p>")
		b.WriteString(html.EscapeString(p))
		b.WriteString("</p>")
	}
	return b.String(), nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/appconnector/publish/ -run 'TestConfluenceStorageBody' -count=1`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/appconnector/publish/confluence_blocks.go internal/modules/appconnector/publish/confluence_blocks_test.go
git commit -m "feat(publish): confluence storage body pure projection (T20 #50)"
```

---

### Task 5: publish 包——`ConfluenceBridge`（Dispatcher+Resolver+预读）与生产端口

**Files:**
- Create: `internal/modules/appconnector/publish/confluence_bridge.go`
- Test: `internal/modules/appconnector/publish/confluence_bridge_test.go`

**Interfaces:**
- Consumes: Task 1/2/3 的适配器族符号（`appconn.ConfluenceCreateAdapter`/`appconn.ConfluenceUpdateAdapter`/`appconn.IsConfluenceUpdateArgs`/`appconn.IsConfluenceCreateArgs`/`appconn.ConfluenceCredential`/`appconn.ParseConfluenceCredential`/`appconn.ParseConfluenceBaseURL`/`appconn.ConfluenceCapabilityWrite`/`appconn.ReadConfluencePageVersion`/`appconn.ErrConfluenceVersionConflict`）；`appconnectorsvc.ActionDispatcher`/`UnknownResolver`/`ActionSnapshot`/`DispatchOutcome`/`ErrDispatchNotStarted`（`internal/modules/appconnector/service/appconnector/action.go:116-126,49`）；`repoappconn.ConnectionRow`/`InstallationRow`（`config_json`）/`AppVersion`（`internal/modules/appconnector/repository/appconnector/install.go:24,37,51`）；`appconnectorsvc.CredentialResolver`（`internal/modules/appconnector/service/appconnector/credentials.go:21`）。
- Produces（Task 6/7/8 依赖，签名逐字）:
  - `const ConfluenceVersionConflictResult = "confluence_version_conflict"`
  - `type ConfluenceConnectionScope struct { AppID string; ConnectionKind string; OwnerID string; AuthVersion int64; ApprovedParents []string; WriteCapability bool; Scheme string; Host string; Port string; APIBasePath string; Edition string }`
  - `type ConfluenceScopeSource interface { ConfluenceScope(ctx context.Context, connectionID string) (ConfluenceConnectionScope, error) }`
  - `type ConfluencePolicyProvider interface { PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error) }`
  - `type ConfluenceTokenSource interface { Token(ctx context.Context, connectionID string, expectedVersion int64) (appconn.ConfluenceCredential, error) }`
  - `type ConfluenceBridge struct`（三端口）+ `func NewConfluenceBridge(scopes ConfluenceScopeSource, policies ConfluencePolicyProvider, tokens ConfluenceTokenSource) *ConfluenceBridge`；`var _ appconnectorsvc.ActionDispatcher = (*ConfluenceBridge)(nil)`、`var _ appconnectorsvc.UnknownResolver = (*ConfluenceBridge)(nil)`
  - `func (b *ConfluenceBridge) ReadConfluencePageVersion(ctx context.Context, connectionID, pageID string) (string, error)`
  - `func NewDBConfluenceScopeSource(db *gorm.DB) ConfluenceScopeSource`
  - `func NewConfluencePolicyProvider(scopes ConfluenceScopeSource) ConfluencePolicyProvider`
  - `func NewConfluenceCredentialTokenSource(resolver appconnectorsvc.CredentialResolver) ConfluenceTokenSource`

- [ ] **Step 1: 写失败测试（云版式线缆双打 + 桥接行为 + 生产 scope source）**

创建 `internal/modules/appconnector/publish/confluence_bridge_test.go`：

```go
package publish

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// cfWireFake is the Cloud-v2-only wire double for bridge/service tests
// (the both-editions double lives in the appconnector package tests; this
// one stays minimal because routing/edition logic is already covered
// there). It is NOT the real-provider acceptance (Task 9).
type cfWireFake struct {
	mu       sync.Mutex
	username string
	secret   string
	pages    map[string]*cfWirePage
	nextID   int
	posts    int
	puts     int
	gets     int
	// knob: apply the write effect, then lose the reply.
	dropNextWrite bool
}

type cfWirePage struct {
	id, spaceID, title, storage string
	version                     int
}

func newCfWireFake(username, secret string) *cfWireFake {
	return &cfWireFake{username: username, secret: secret, pages: map[string]*cfWirePage{}}
}

func (f *cfWireFake) addPage(id, spaceID, title string, version int) *cfWirePage {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := &cfWirePage{id: id, spaceID: spaceID, title: title, version: version}
	f.pages[id] = p
	return p
}

func (f *cfWireFake) bump(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.pages[id]; ok {
		p.version++
	}
}

func (f *cfWireFake) page(id string) *cfWirePage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pages[id]
}

func cfWireJSON(p *cfWirePage, withBody bool) string {
	storage := ""
	if withBody {
		raw, _ := json.Marshal(p.storage)
		storage = fmt.Sprintf(`,"body":{"storage":{"value":%s}}`, raw)
	}
	return fmt.Sprintf(`{"id":%q,"status":"current","title":%q,"spaceId":%q,"version":{"number":%d,"createdAt":"2026-09-25T08:00:00Z"}%s}`,
		p.id, p.title, p.spaceID, p.version, storage)
}

func (f *cfWireFake) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
	auth := func(r *http.Request) bool {
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte(f.username+":"+f.secret))
		return r.Header.Get("Authorization") == want
	}
	mux.HandleFunc("/api/v2/pages", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			write(w, 401, `{}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			SpaceID  string `json:"space_id"`
			ParentID string `json:"parent_id"`
			Title    string `json:"title"`
			Body     struct {
				Value string `json:"value"`
			} `json:"body"`
		}
		_ = json.Unmarshal(body, &req)
		f.mu.Lock()
		parent, ok := f.pages[req.ParentID]
		f.mu.Unlock()
		if !ok || req.SpaceID == "" || req.Title == "" || req.Body.Value == "" {
			write(w, 400, `{"errors":[{"title":"invalid"}]}`)
			return
		}
		f.mu.Lock()
		f.nextID++
		f.posts++
		np := &cfWirePage{id: fmt.Sprintf("cf-%d", f.nextID), spaceID: req.SpaceID, title: req.Title, storage: req.Body.Value, version: 1}
		f.pages[np.id] = np
		drop := f.dropNextWrite
		if drop {
			f.dropNextWrite = false
		}
		f.mu.Unlock()
		if drop {
			panic(http.ErrAbortHandler)
		}
		write(w, 200, cfWireJSON(np, false))
	})
	mux.HandleFunc("/api/v2/pages/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			write(w, 401, `{}`)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/v2/pages/")
		f.mu.Lock()
		p, ok := f.pages[id]
		f.mu.Unlock()
		if !ok {
			write(w, 404, `{"errors":[{"title":"not found"}]}`)
			return
		}
		switch r.Method {
		case http.MethodGet:
			f.mu.Lock()
			f.gets++
			f.mu.Unlock()
			write(w, 200, cfWireJSON(p, true))
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Title string `json:"title"`
				Body  struct {
					Value string `json:"value"`
				} `json:"body"`
				Version struct {
					Number int `json:"number"`
				} `json:"version"`
			}
			_ = json.Unmarshal(body, &req)
			if req.Version.Number != p.version+1 {
				write(w, 409, `{"errors":[{"title":"version conflict"}]}`)
				return
			}
			f.mu.Lock()
			f.puts++
			p.title, p.storage, p.version = req.Title, req.Body.Value, req.Version.Number
			drop := f.dropNextWrite
			if drop {
				f.dropNextWrite = false
			}
			f.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			write(w, 200, cfWireJSON(p, false))
		default:
			write(w, 405, `{}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func cfWirePolicy(t *testing.T, srv *httptest.Server) appconn.HTTPPolicy {
	t.Helper()
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	return appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PUT"}, PathPrefix: "/",
		AuthorizedNetworks: []*net.IPNet{network},
	}
}

// ---- port fakes ----

type cfFakeScopes struct{ scope ConfluenceConnectionScope }

func (f *cfFakeScopes) ConfluenceScope(ctx context.Context, connectionID string) (ConfluenceConnectionScope, error) {
	return f.scope, nil
}

type cfFakeTokens struct{ cred appconn.ConfluenceCredential }

func (f *cfFakeTokens) Token(ctx context.Context, connectionID string, expectedVersion int64) (appconn.ConfluenceCredential, error) {
	return f.cred, nil
}

func cfScope(policy appconn.HTTPPolicy) ConfluenceConnectionScope {
	return ConfluenceConnectionScope{
		AppID: "confluence", ConnectionKind: "personal", OwnerID: "u1", AuthVersion: 1,
		ApprovedParents: []string{"parent-1"}, WriteCapability: true,
		Scheme: policy.Scheme, Host: policy.Host, Port: policy.Port,
		Edition: "cloud",
	}
}

func cfCred() appconn.ConfluenceCredential {
	return appconn.ConfluenceCredential{Username: "cf-user@example.test", Secret: "secret_cf_token"}
}

func cfSnapshot(id string, args json.RawMessage) appconnectorsvc.ActionSnapshot {
	return appconnectorsvc.ActionSnapshot{ID: id, TenantID: 7, ActorID: "u1", ConnectionID: "conn-cf",
		Version: "confluence/v1", Target: "parent-1", Risk: appconn.RiskWrite, Args: args, AuthVersion: 1}
}

func TestConfluenceBridgeDispatchesCreateAndUpdate(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 7)
	srv := fake.server(t)
	bridge := NewConfluenceBridge(
		&cfFakeScopes{scope: cfScope(cfWirePolicy(t, srv))},
		&fakePolicies{pol: cfWirePolicy(t, srv)},
		&cfFakeTokens{cred: cfCred()},
	)
	createArgs, _ := json.Marshal(map[string]any{"parent": "parent-1", "title": "Report", "storage": "<p>x</p>"})
	out, err := bridge.Dispatch(context.Background(), cfSnapshot("act-1", createArgs), "conn-cf")
	if err != nil || out.Status != appconn.ActionSucceeded {
		t.Fatalf("create dispatch: %+v %v", out, err)
	}
	pageID := out.ExecutionID
	if pageID == "" {
		t.Fatalf("execution id must carry the new page: %+v", out)
	}
	updateArgs, _ := json.Marshal(map[string]any{"page_id": pageID, "expected_version": "1", "title": "Report v2", "storage": "<p>y</p>"})
	out, err = bridge.Dispatch(context.Background(), cfSnapshot("act-2", updateArgs), "conn-cf")
	if err != nil || out.Status != appconn.ActionSucceeded {
		t.Fatalf("update dispatch: %+v %v", out, err)
	}
	fake.mu.Lock()
	p := fake.pages[pageID]
	fake.mu.Unlock()
	if p.version != 2 || p.storage != "<p>y</p>" {
		t.Fatalf("remote page drift: %+v", p)
	}
}

func TestConfluenceBridgeConflictPrefix(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 7)
	fake.addPage("page-9", "sp-1", "Old", 3)
	srv := fake.server(t)
	bridge := NewConfluenceBridge(
		&cfFakeScopes{scope: cfScope(cfWirePolicy(t, srv))},
		&fakePolicies{pol: cfWirePolicy(t, srv)},
		&cfFakeTokens{cred: cfCred()},
	)
	fake.bump("page-9") // external edit: version 3 → 4
	args, _ := json.Marshal(map[string]any{"page_id": "page-9", "expected_version": "3", "title": "T", "storage": "<p>s</p>"})
	out, err := bridge.Dispatch(context.Background(), cfSnapshot("act-3", args), "conn-cf")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != appconn.ActionFailed || !strings.HasPrefix(out.ProviderResult, ConfluenceVersionConflictResult) {
		t.Fatalf("conflict must fail with the confluence prefix: %+v", out)
	}
	_, puts, _ := func() (int, int, int) { fake.mu.Lock(); defer fake.mu.Unlock(); return fake.posts, fake.puts, fake.gets }()
	if puts != 0 {
		t.Fatalf("zero writes on conflict, got %d", puts)
	}
}

func TestConfluenceBridgeRefusesNonConfluenceConnection(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	srv := fake.server(t)
	scope := cfScope(cfWirePolicy(t, srv))
	scope.AppID = "notion"
	bridge := NewConfluenceBridge(&cfFakeScopes{scope: scope}, &fakePolicies{pol: cfWirePolicy(t, srv)}, &cfFakeTokens{cred: cfCred()})
	args, _ := json.Marshal(map[string]any{"parent": "parent-1", "title": "T", "storage": "s"})
	_, err := bridge.Dispatch(context.Background(), cfSnapshot("act-4", args), "conn-cf")
	if !errors.Is(err, appconnectorsvc.ErrDispatchNotStarted) {
		t.Fatalf("a non-confluence connection must be a provable pre-send refusal, got %v", err)
	}
	posts, puts, gets := func() (int, int, int) { fake.mu.Lock(); defer fake.mu.Unlock(); return fake.posts, fake.puts, fake.gets }()
	if posts != 0 || puts != 0 || gets != 0 {
		t.Fatalf("no request may leave: %d/%d/%d", posts, puts, gets)
	}
}

func TestConfluenceBridgeQueryProviderReadsRemoteFirst(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 7)
	srv := fake.server(t)
	bridge := NewConfluenceBridge(
		&cfFakeScopes{scope: cfScope(cfWirePolicy(t, srv))},
		&fakePolicies{pol: cfWirePolicy(t, srv)},
		&cfFakeTokens{cred: cfCred()},
	)
	// A lost update reply after the effect applied.
	updateArgs, _ := json.Marshal(map[string]any{"page_id": "page-9", "expected_version": "3", "title": "T2", "storage": "<p>q</p>"})
	fake.addPage("page-9", "sp-1", "Old", 3)
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()
	out, err := bridge.Dispatch(context.Background(), cfSnapshot("act-5", updateArgs), "conn-cf")
	if err != nil || out.Status != appconn.ActionUnknown {
		t.Fatalf("lost reply must park unknown: %+v %v", out, err)
	}
	fake.mu.Lock()
	putsBefore := fake.puts
	fake.mu.Unlock()
	resolved, err := bridge.QueryProvider(context.Background(), cfSnapshot("act-5", updateArgs), "conn-cf")
	if err != nil || resolved.Status != appconn.ActionSucceeded {
		t.Fatalf("provider query must resolve from the remote: %+v %v", resolved, err)
	}
	fake.mu.Lock()
	putsAfter := fake.puts
	fake.mu.Unlock()
	if putsAfter != putsBefore {
		t.Fatalf("query must not re-send: %d -> %d", putsBefore, putsAfter)
	}
}

func TestConfluenceBridgeReadPageVersion(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 9)
	srv := fake.server(t)
	bridge := NewConfluenceBridge(
		&cfFakeScopes{scope: cfScope(cfWirePolicy(t, srv))},
		&fakePolicies{pol: cfWirePolicy(t, srv)},
		&cfFakeTokens{cred: cfCred()},
	)
	v, err := bridge.ReadConfluencePageVersion(context.Background(), "conn-cf", "parent-1")
	if err != nil || v != "9" {
		t.Fatalf("plan-time pre-read drift: %q %v", v, err)
	}
}

func TestDBConfluenceScopeSource(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repoappconn.ConnectionRow{}, &repoappconn.InstallationRow{}, &repoappconn.AppVersion{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repoappconn.InstallationRow{ID: "inst-cf", TenantID: 7, AppID: "confluence", AppVersion: "v1", State: "active",
		ConfigJSON: `{"base_url":"https://acme.atlassian.net"}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('confluence', 'v1', '{"scopes":["write_content"],"approved_parents":["parent-1"]}', '{}')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repoappconn.ConnectionRow{TenantID: 7, ID: "conn-cf", InstallationID: "inst-cf", Kind: "personal",
		OwnerID: "u1", State: "active", AuthVersion: 3}).Error; err != nil {
		t.Fatal(err)
	}
	src := NewDBConfluenceScopeSource(db)
	scope, err := src.ConfluenceScope(context.Background(), "conn-cf")
	if err != nil {
		t.Fatal(err)
	}
	if scope.AppID != "confluence" || !scope.WriteCapability || scope.AuthVersion != 3 ||
		len(scope.ApprovedParents) != 1 || scope.ApprovedParents[0] != "parent-1" ||
		scope.Edition != "cloud" || scope.APIBasePath != "/wiki" || scope.Scheme != "https" || scope.Host != "acme.atlassian.net" {
		t.Fatalf("production scope drift: %+v", scope)
	}
	// Missing base_url fails closed.
	if err := db.Create(&repoappconn.InstallationRow{ID: "inst-bad", TenantID: 7, AppID: "confluence", AppVersion: "v1", State: "active", ConfigJSON: `{}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repoappconn.ConnectionRow{TenantID: 7, ID: "conn-bad", InstallationID: "inst-bad", Kind: "personal", OwnerID: "u1", State: "active", AuthVersion: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := src.ConfluenceScope(context.Background(), "conn-bad"); err == nil {
		t.Fatal("a connection without a reviewed base_url must fail scope resolution")
	}
	// Unknown connection is a lookup miss.
	if _, err := src.ConfluenceScope(context.Background(), "conn-nope"); err == nil {
		t.Fatal("unknown connection must error")
	}
}

func TestConfluencePolicyProviderFromScope(t *testing.T) {
	scope := ConfluenceConnectionScope{Scheme: "https", Host: "acme.atlassian.net", APIBasePath: "/wiki"}
	pol, err := NewConfluencePolicyProvider(&cfFakeScopes{scope: scope}).PolicyFor(context.Background(), "conn-cf")
	if err != nil {
		t.Fatal(err)
	}
	if pol.Scheme != "https" || pol.Host != "acme.atlassian.net" || pol.PathPrefix != "/wiki/" {
		t.Fatalf("policy drift: %+v", pol)
	}
	if len(pol.Methods) != 3 || pol.Methods[0] != "GET" || pol.Methods[1] != "POST" || pol.Methods[2] != "PUT" {
		t.Fatalf("policy methods drift: %+v", pol.Methods)
	}
	pol, err = NewConfluencePolicyProvider(&cfFakeScopes{scope: ConfluenceConnectionScope{Scheme: "https", Host: "h"}}).PolicyFor(context.Background(), "c")
	if err != nil || pol.PathPrefix != "/" {
		t.Fatalf("root prefix drift: %+v %v", pol, err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/appconnector/publish/ -run 'TestConfluenceBridge|TestDBConfluenceScopeSource|TestConfluencePolicyProviderFromScope' -count=1`
Expected: FAIL（`undefined: ConfluenceBridge`）。

- [ ] **Step 3: 最小实现**

创建 `internal/modules/appconnector/publish/confluence_bridge.go`：

```go
package publish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"

	"gorm.io/gorm"
)

// ConfluenceVersionConflictResult is the ProviderResult prefix the bridge
// records for a definitive version-conflict refusal; the service layer
// maps it onto the 409 PUBLISH_VERSION_CONFLICT response.
const ConfluenceVersionConflictResult = "confluence_version_conflict"

// appIDConfluence is the installation app identity the Confluence write
// adapter family serves.
const appIDConfluence = "confluence"

// ConfluenceConnectionScope is everything the publish seam needs to know
// about one connection before any Confluence call: the installation's app
// identity (routing), the reviewed destination scope (approved parent
// pages), the reviewed write capability, the connection's auth generation
// (strict credential binding) and the reviewed outbound endpoint derived
// from the installation's config_json.base_url.
type ConfluenceConnectionScope struct {
	AppID           string
	ConnectionKind  string
	OwnerID         string
	AuthVersion     int64
	ApprovedParents []string
	WriteCapability bool
	Scheme          string
	Host            string
	Port            string
	APIBasePath     string
	Edition         string
}

// ConfluenceScopeSource resolves the scope of one connection from the
// authoritative rows. The production implementation is
// NewDBConfluenceScopeSource (connections → installations → app_versions).
type ConfluenceScopeSource interface {
	ConfluenceScope(ctx context.Context, connectionID string) (ConfluenceConnectionScope, error)
}

// ConfluencePolicyProvider returns the reviewed outbound HTTP policy for
// one connection's Confluence calls (A04). Production derives it from the
// same scope rows (NewConfluencePolicyProvider); tests inject the
// loopback-authorized policy.
type ConfluencePolicyProvider interface {
	PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error)
}

// ConfluenceTokenSource returns the decrypted Basic credential for
// exactly one connection at one auth generation — the A02 credential
// resolution AFTER the permission guard has passed.
type ConfluenceTokenSource interface {
	Token(ctx context.Context, connectionID string, expectedVersion int64) (appconn.ConfluenceCredential, error)
}

// ConfluenceBridge adapts the Confluence adapter family to the A03
// pipeline's dispatch boundary. It routes by the approved snapshot's
// shape (page_id+expected_version+storage → update; parent+storage →
// create) and maps adapter outcomes onto DispatchOutcome:
//
//   - adapter FAILED (provider-refused, incl. version conflict) → a
//     definitive failed outcome, nil error — the service settles failed;
//   - adapter UNKNOWN (transport failure / 5xx / unprovable) → an unknown
//     outcome, nil error — the action parks for a provider query;
//   - wiring gaps (scope/token/policy unavailable, non-confluence
//     connection) → an ErrDispatchNotStarted error: provably pre-send,
//     nothing left the process.
type ConfluenceBridge struct {
	scopes   ConfluenceScopeSource
	policies ConfluencePolicyProvider
	tokens   ConfluenceTokenSource
}

// NewConfluenceBridge builds the bridge over its three ports (the
// single-step adapters carry no progress record, so unlike the Notion
// bridge there is no publication-progress port here).
func NewConfluenceBridge(scopes ConfluenceScopeSource, policies ConfluencePolicyProvider, tokens ConfluenceTokenSource) *ConfluenceBridge {
	return &ConfluenceBridge{scopes: scopes, policies: policies, tokens: tokens}
}

var _ appconnectorsvc.ActionDispatcher = (*ConfluenceBridge)(nil)
var _ appconnectorsvc.UnknownResolver = (*ConfluenceBridge)(nil)

// adapterFor builds the adapter instance for one dispatch/query. Scope,
// policy and credential are resolved fresh per call so a revoked
// connection (auth version bump) can never be reached with a stale
// secret.
func (b *ConfluenceBridge) adapterFor(ctx context.Context, snap appconnectorsvc.ActionSnapshot) (appconn.Adapter, error) {
	scope, err := b.scopes.ConfluenceScope(ctx, snap.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("%w: scope for connection %q: %v", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	if scope.AppID != appIDConfluence {
		return nil, fmt.Errorf("%w: connection %q is %q, not a confluence connection", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, scope.AppID)
	}
	pol, err := b.policies.PolicyFor(ctx, snap.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("%w: policy for connection %q: %v", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	cred, err := b.tokens.Token(ctx, snap.ConnectionID, snap.AuthVersion)
	if err != nil {
		return nil, fmt.Errorf("%w: credential for connection %q: %v", appconnectorsvc.ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	caps := func(ctx context.Context, a appconn.Action) ([]string, error) {
		if !scope.WriteCapability {
			return nil, fmt.Errorf("connection lacks %s", appconn.ConfluenceCapabilityWrite)
		}
		return []string{appconn.ConfluenceCapabilityWrite}, nil
	}
	credFn := func(ctx context.Context) (appconn.ConfluenceCredential, error) { return cred, nil }
	if appconn.IsConfluenceUpdateArgs(json.RawMessage(snap.Args)) {
		return &appconn.ConfluenceUpdateAdapter{
			Policy: pol, Credential: credFn, Edition: scope.Edition,
			APIBasePath: scope.APIBasePath, ConnectionCapabilities: caps,
		}, nil
	}
	return &appconn.ConfluenceCreateAdapter{
		Policy: pol, Credential: credFn, Edition: scope.Edition,
		APIBasePath: scope.APIBasePath, ApprovedParents: scope.ApprovedParents,
		ConnectionCapabilities: caps,
	}, nil
}

func (b *ConfluenceBridge) run(ctx context.Context, snap appconnectorsvc.ActionSnapshot, query bool) (appconnectorsvc.DispatchOutcome, error) {
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
			if errors.Is(aerr, appconn.ErrConfluenceVersionConflict) {
				reason = ConfluenceVersionConflictResult + ": remote version moved since approval"
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
func (b *ConfluenceBridge) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	return b.run(ctx, snap, false)
}

// QueryProvider implements appconnectorsvc.UnknownResolver: the provider
// (never the local queue) answers what became of an unknown dispatch.
func (b *ConfluenceBridge) QueryProvider(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	return b.run(ctx, snap, true)
}

// ReadConfluencePageVersion is the plan-formation pre-read (AC1): the
// external current version of one page, read through the same
// policy/credential ports. It performs no write of any kind.
func (b *ConfluenceBridge) ReadConfluencePageVersion(ctx context.Context, connectionID, pageID string) (string, error) {
	scope, err := b.scopes.ConfluenceScope(ctx, connectionID)
	if err != nil {
		return "", err
	}
	if scope.AppID != appIDConfluence {
		return "", fmt.Errorf("connection %q is not a confluence connection", connectionID)
	}
	pol, err := b.policies.PolicyFor(ctx, connectionID)
	if err != nil {
		return "", err
	}
	cred, err := b.tokens.Token(ctx, connectionID, scope.AuthVersion)
	if err != nil {
		return "", err
	}
	return appconn.ReadConfluencePageVersion(ctx, pol, func(context.Context) (appconn.ConfluenceCredential, error) { return cred, nil },
		scope.Edition, scope.APIBasePath, pageID)
}

// ---- production port implementations ----

// dbConfluenceScopeSource resolves a connection's publish scope from the
// authoritative rows: connections → installations (app id + reviewed
// base_url config) → app_versions.schema_json (reviewed scopes + approved
// destination pages).
type dbConfluenceScopeSource struct{ db *gorm.DB }

// NewDBConfluenceScopeSource builds the production scope source.
func NewDBConfluenceScopeSource(db *gorm.DB) ConfluenceScopeSource { return &dbConfluenceScopeSource{db: db} }

// ConfluenceScope's first lookup keys on connection id ALONE — no tenant
// predicate. Tenant isolation here is a caller contract, not defense this
// query provides: every reachable path tenant-checks BEFORE calling
// (FormPlan's handler 404s cross-tenant connection ids at
// app_connector_confluence_publish.go; dispatch acts only on the snapshot
// of an action row whose tenant was bound and validated at Prepare), and
// the resolved scope never crosses back out to an external caller. This
// "tenant check first, then ConfluenceScope" order is inherited from the
// Notion bridge (dispatcher.go NotionScope) and MUST be kept by reusers.
func (s *dbConfluenceScopeSource) ConfluenceScope(ctx context.Context, connectionID string) (ConfluenceConnectionScope, error) {
	var conn repoappconn.ConnectionRow
	if err := s.db.WithContext(ctx).Where("id = ?", connectionID).First(&conn).Error; err != nil {
		return ConfluenceConnectionScope{}, err
	}
	var inst repoappconn.InstallationRow
	if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", conn.InstallationID, conn.TenantID).First(&inst).Error; err != nil {
		return ConfluenceConnectionScope{}, err
	}
	var ver repoappconn.AppVersion
	if err := s.db.WithContext(ctx).Where("app_id = ? AND version = ?", inst.AppID, inst.AppVersion).First(&ver).Error; err != nil {
		return ConfluenceConnectionScope{}, err
	}
	var schema struct {
		Scopes          []string `json:"scopes"`
		ApprovedParents []string `json:"approved_parents"`
	}
	if err := json.Unmarshal([]byte(ver.SchemaJSON), &schema); err != nil {
		return ConfluenceConnectionScope{}, err
	}
	var cfg struct {
		BaseURL string `json:"base_url"`
		Edition string `json:"edition"`
	}
	if err := json.Unmarshal([]byte(inst.ConfigJSON), &cfg); err != nil {
		return ConfluenceConnectionScope{}, fmt.Errorf("installation %q config_json: %v", inst.ID, err)
	}
	ep, err := appconn.ParseConfluenceBaseURL(cfg.BaseURL, cfg.Edition)
	if err != nil {
		// A connection whose reviewed endpoint does not parse fails closed:
		// no scope, no dispatch.
		return ConfluenceConnectionScope{}, fmt.Errorf("installation %q base_url: %v", inst.ID, err)
	}
	scope := ConfluenceConnectionScope{
		AppID: inst.AppID, ConnectionKind: conn.Kind, OwnerID: conn.OwnerID,
		AuthVersion: conn.AuthVersion, ApprovedParents: schema.ApprovedParents,
		Scheme: ep.Scheme, Host: ep.Host, Port: ep.Port,
		APIBasePath: ep.APIBasePath, Edition: ep.Edition,
	}
	for _, sc := range schema.Scopes {
		if sc == appconn.ConfluenceCapabilityWrite {
			scope.WriteCapability = true
		}
	}
	return scope, nil
}

// scopePolicyProvider derives the reviewed outbound policy from the same
// scope the bridge already trusts: HTTPS-or-HTTP to the reviewed origin,
// exactly GET/POST/PUT under the configured context path, 30s per-request
// timeout (inside the pipeline's 30s dispatch deadline). Dial-time
// public-address enforcement stays with HTTPPolicy.
type scopePolicyProvider struct{ scopes ConfluenceScopeSource }

// NewConfluencePolicyProvider builds the production policy provider over
// a scope source.
func NewConfluencePolicyProvider(scopes ConfluenceScopeSource) ConfluencePolicyProvider {
	return scopePolicyProvider{scopes: scopes}
}

func (p scopePolicyProvider) PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error) {
	scope, err := p.scopes.ConfluenceScope(ctx, connectionID)
	if err != nil {
		return appconn.HTTPPolicy{}, err
	}
	prefix := "/"
	if scope.APIBasePath != "" {
		prefix = scope.APIBasePath + "/"
	}
	return appconn.HTTPPolicy{
		Scheme: scope.Scheme, Host: scope.Host, Port: scope.Port,
		Methods:    []string{"GET", "POST", "PUT"},
		PathPrefix: prefix,
		Timeout:    30 * time.Second,
	}, nil
}

// confluenceCredentialTokenSource resolves the connection credential
// through the A02 credential resolver (revoked / stale-version
// connections never resolve) and validates the reviewed JSON shape.
type confluenceCredentialTokenSource struct {
	resolver appconnectorsvc.CredentialResolver
}

// NewConfluenceCredentialTokenSource adapts the internal credential
// resolver onto the bridge's token port.
func NewConfluenceCredentialTokenSource(resolver appconnectorsvc.CredentialResolver) ConfluenceTokenSource {
	return &confluenceCredentialTokenSource{resolver: resolver}
}

func (s *confluenceCredentialTokenSource) Token(ctx context.Context, connectionID string, expectedVersion int64) (appconn.ConfluenceCredential, error) {
	raw, err := s.resolver.Resolve(ctx, connectionID, expectedVersion)
	if err != nil {
		return appconn.ConfluenceCredential{}, err
	}
	return appconn.ParseConfluenceCredential(raw)
}
```

**实现注意**：本文件 import 块为 `context`/`encoding/json`/`errors`/`fmt`/`time` + 三个 appconnector 包 + `gorm.io/gorm`（`errors` 用于 `errors.Is`，`json` 用于快照形状路由与 schema/config 解析）；不需要 `strings`。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/modules/appconnector/publish/ -run 'TestConfluenceBridge|TestDBConfluenceScopeSource|TestConfluencePolicyProviderFromScope' -count=1`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/appconnector/publish/confluence_bridge.go internal/modules/appconnector/publish/confluence_bridge_test.go
git commit -m "feat(publish): confluence bridge - dispatcher/resolver routing + production scope/policy/credential ports (T20 #50)"
```

---

### Task 6: publish 包——`ConfluencePublishService`（FormPlan/Execute/Reconcile/回执 settle）

**Files:**
- Create: `internal/modules/appconnector/publish/confluence.go`
- Test: `internal/modules/appconnector/publish/confluence_test.go`

**Interfaces:**
- Consumes: 同包既有 `PublishPlanInput`/`PublishPlanView`/`PublishArtifactView`/`PublishExecuteOutcome`/`PublishReceiptView`/`publicationView`/`publishableMIME`/`MaxPublishArtifactBytes`/`ArtifactVersionReader`/`ArtifactContentReader` 与全部 `ErrPublish*` 哨兵（`internal/modules/appconnector/publish/plan.go:18-135`，零修改复用）；Task 4 `ConfluenceStorageBody`；Task 5 全部桥接符号；`repoappconn.PublicationStore`（`CreatePublication:73`/`FindByAction:91`/`SettlePublication:130`/`LatestPublishedByDestination:155`）；`appconn.ParseConfluencePageReceipt`。
- Produces（Task 7/8 依赖，签名逐字）:
  - `type ConfluenceRemoteReader interface { ReadConfluencePageVersion(ctx context.Context, connectionID, pageID string) (string, error) }`
  - `type ConfluencePublishService struct` + `func NewConfluencePublishService(actions *appconnectorsvc.ActionService, store appconnectorsvc.ActionStoreSource, pubs *repoappconn.PublicationStore, artifacts ArtifactVersionReader, content ArtifactContentReader, remote ConfluenceRemoteReader, scopes ConfluenceScopeSource) *ConfluencePublishService`
  - `func (s *ConfluencePublishService) FormPlan(ctx context.Context, in PublishPlanInput) (PublishPlanView, error)`
  - `func (s *ConfluencePublishService) Execute(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error)`
  - `func (s *ConfluencePublishService) Reconcile(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error)`
  - `func (s *ConfluencePublishService) Receipt(ctx context.Context, tenantID uint64, actionID string) (PublishReceiptView, error)`

- [ ] **Step 1: 写失败测试**

创建 `internal/modules/appconnector/publish/confluence_test.go`：

```go
package publish

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
)

type cfRemote struct {
	versions map[string]string
	err      error
}

func (f *cfRemote) ReadConfluencePageVersion(ctx context.Context, connectionID, pageID string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.versions[pageID], nil
}

func cfScopeOK() ConfluenceConnectionScope {
	return ConfluenceConnectionScope{
		AppID: "confluence", ConnectionKind: "personal", OwnerID: "u1", AuthVersion: 1,
		ApprovedParents: []string{"parent-1"}, WriteCapability: true,
		Edition: "cloud",
	}
}

type cfPlanEnv struct {
	svc   *ConfluencePublishService
	pubs  *repoappconn.PublicationStore
	store *repoappconn.ActionStore
	fake  *cfWireFake
}

func newCfPlanEnv(t *testing.T, fake *cfWireFake, remote *cfRemote) *cfPlanEnv {
	t.Helper()
	db := openPlanDB(t)
	store := repoappconn.NewActionStore(db)
	pubs := repoappconn.NewPublicationStore(db)
	srv := fake.server(t)
	pol := cfWirePolicy(t, srv)
	bridge := NewConfluenceBridge(
		&cfFakeScopes{scope: cfScope(pol)},
		&fakePolicies{pol: pol},
		&cfFakeTokens{cred: cfCred()},
	)
	actions := appconnectorsvc.NewActionService(store, passGuard{}, nil, bridge, bridge)
	artifacts := &fakeArtifacts{version: repository.ArtifactVersion{
		TenantID: 7, ID: "ver-1", RunID: "run-1", SessionID: "sess-1",
		Digest:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ObjectKey: "artifact-versions/7/run-1/d", MIME: "text/plain", ScanState: repository.ArtifactScanReady, Size: 13,
	}}
	svc := NewConfluencePublishService(actions, store, pubs, artifacts, &fakeContent{data: []byte("hello\n\nworld")}, remote,
		&cfFakeScopes{scope: cfScopeOK()})
	return &cfPlanEnv{svc: svc, pubs: pubs, store: store, fake: fake}
}

func cfPlanInput() PublishPlanInput {
	return PublishPlanInput{
		TenantID: 7, ActorID: "u1", ConnectionID: "conn-cf",
		SessionID: "sess-1", ArtifactVersionID: "ver-1", Title: "Report",
		ParentPageID: "parent-1",
	}
}

func cfApprove(t *testing.T, env *cfPlanEnv, view PublishPlanView) {
	t.Helper()
	if err := env.svc.actions.Approve(context.Background(), view.ActionID, "u1", view.Digest); err != nil {
		t.Fatal(err)
	}
}

func TestConfluenceFormPlanCreateBindsBaselineVersion(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 7)
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"parent-1": "7"}})
	view, err := env.svc.FormPlan(context.Background(), cfPlanInput())
	if err != nil {
		t.Fatal(err)
	}
	if view.Mode != "create" || view.Destination != "parent-1" || view.ExpectedExternalVersion != "7" {
		t.Fatalf("plan shape: %+v", view)
	}
	row, err := env.store.FindAction(context.Background(), view.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if row.State != appconn.ActionAwaitingApproval || row.Risk != appconn.RiskWrite {
		t.Fatalf("prepared action row: %+v", row)
	}
	var snap struct {
		Parent  string `json:"parent"`
		Title   string `json:"title"`
		Storage string `json:"storage"`
	}
	if err := json.Unmarshal([]byte(row.ArgsSnapshot), &snap); err != nil || snap.Parent != "parent-1" {
		t.Fatalf("create snapshot must carry the approved parent: %s (%v)", row.ArgsSnapshot, err)
	}
	if snap.Storage != "<p>hello</p>\n<p>world</p>" {
		t.Fatalf("the snapshot must carry the derived storage body, not raw text: %s", snap.Storage)
	}
	pub, perr := env.pubs.FindByAction(context.Background(), 7, view.ActionID)
	if perr != nil || pub.State != repoappconn.PublicationPlanned || pub.Provider != "confluence" ||
		pub.ArtifactVersionID != "ver-1" || pub.ExpectedVersion != "7" {
		t.Fatalf("planned publication must bind artifact + destination + version: %+v %v", pub, perr)
	}
}

func TestConfluenceFormPlanCreateParentOutOfScopeFailsClosed(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{}})
	in := cfPlanInput()
	in.ParentPageID = "parent-unlisted"
	if _, err := env.svc.FormPlan(context.Background(), in); !errors.Is(err, ErrPublishDestinationOutOfScope) {
		t.Fatalf("unlisted parent must fail closed, got %v", err)
	}
}

func TestConfluenceFormPlanDestinationUnreadable(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	env := newCfPlanEnv(t, fake, &cfRemote{err: errors.New("boom")})
	if _, err := env.svc.FormPlan(context.Background(), cfPlanInput()); !errors.Is(err, ErrPublishDestinationUnreadable) {
		t.Fatalf("unreadable destination must refuse plan formation, got %v", err)
	}
}

func TestConfluenceFormPlanUpdateRequiresPriorPublishedReceipt(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "old", 3)
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"page-9": "3"}})
	in := cfPlanInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	if _, err := env.svc.FormPlan(context.Background(), in); !errors.Is(err, ErrPublishUpdateTargetNotPublished) {
		t.Fatalf("update target must chain from a prior receipt, got %v", err)
	}
}

func TestConfluenceFormPlanUpdateBindsExpectedVersion(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "old", 3)
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"page-9": "3"}})
	if err := env.pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-prev", ConnectionID: "conn-cf", Provider: "confluence",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.pubs.SettlePublication(context.Background(), 7, "act-prev", repoappconn.PublicationPublished, "page-9", "1", "{}"); err != nil {
		t.Fatal(err)
	}
	in := cfPlanInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	view, err := env.svc.FormPlan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if view.Mode != "update" || view.ExpectedExternalVersion != "3" {
		t.Fatalf("update plan must bind the read version: %+v", view)
	}
	row, _ := env.store.FindAction(context.Background(), view.ActionID)
	var snap struct {
		PageID          string `json:"page_id"`
		ExpectedVersion string `json:"expected_version"`
	}
	if err := json.Unmarshal([]byte(row.ArgsSnapshot), &snap); err != nil || snap.PageID != "page-9" || snap.ExpectedVersion != "3" {
		t.Fatalf("update snapshot must carry page_id + expected_version: %s (%v)", row.ArgsSnapshot, err)
	}
}

func TestConfluenceFormPlanRejectsNotConfluenceConnection(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"parent-1": "7"}})
	env.svc.scopes = &cfFakeScopes{scope: func() ConfluenceConnectionScope {
		s := cfScopeOK()
		s.AppID = "notion"
		return s
	}()}
	if _, err := env.svc.FormPlan(context.Background(), cfPlanInput()); !errors.Is(err, ErrPublishInvalidInput) {
		t.Fatalf("a notion connection must be refused at the confluence endpoint, got %v", err)
	}
}

func TestConfluenceFormPlanRejectsBadInputs(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{}})
	cases := map[string]PublishPlanInput{
		"no title":      func() PublishPlanInput { in := cfPlanInput(); in.Title = ""; return in }(),
		"both targets":  func() PublishPlanInput { in := cfPlanInput(); in.PageID = "p"; return in }(),
		"no target":     func() PublishPlanInput { in := cfPlanInput(); in.ParentPageID = ""; return in }(),
		"no version id": func() PublishPlanInput { in := cfPlanInput(); in.ArtifactVersionID = ""; return in }(),
	}
	for name, in := range cases {
		if _, err := env.svc.FormPlan(context.Background(), in); !errors.Is(err, ErrPublishInvalidInput) {
			t.Fatalf("%s: want ErrPublishInvalidInput, got %v", name, err)
		}
	}
}

func TestConfluenceFormPlanRejectsEmptyArtifact(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"parent-1": "7"}})
	env.svc.content = &fakeContent{data: []byte("  \n ")}
	if _, err := env.svc.FormPlan(context.Background(), cfPlanInput()); !errors.Is(err, ErrPublishEmptyContent) {
		t.Fatalf("an artifact with no publishable text must be refused, got %v", err)
	}
}

func TestConfluenceExecuteCreatePublishesAndSettlesReceipt(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 7)
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"parent-1": "7"}})
	view, err := env.svc.FormPlan(context.Background(), cfPlanInput())
	if err != nil {
		t.Fatal(err)
	}
	cfApprove(t, env, view)
	out, err := env.svc.Execute(context.Background(), 7, view.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	if out.ActionState != appconn.ActionSucceeded || out.Receipt.State != repoappconn.PublicationPublished {
		t.Fatalf("publish outcome: %+v", out)
	}
	if out.Receipt.ExternalID == "" || out.Receipt.ExternalVersion != "1" {
		t.Fatalf("receipt must carry external id + the version the publish produced: %+v", out.Receipt)
	}
}

func TestConfluenceExecuteUpdateConflictSettlesFailedReceipt(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 7)
	fake.addPage("page-9", "sp-1", "old", 3)
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"page-9": "3"}})
	if err := env.pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-prev", ConnectionID: "conn-cf", Provider: "confluence",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.pubs.SettlePublication(context.Background(), 7, "act-prev", repoappconn.PublicationPublished, "page-9", "1", "{}"); err != nil {
		t.Fatal(err)
	}
	in := cfPlanInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	view, err := env.svc.FormPlan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	cfApprove(t, env, view)
	// External collaborator edits between approval and execute.
	fake.bump("page-9")
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
	fake.mu.Lock()
	puts := fake.puts
	fake.mu.Unlock()
	if puts != 0 {
		t.Fatalf("conflict must write nothing: %d puts", puts)
	}
}

func TestConfluenceReconcileResolvesUnknownWithoutRedispatch(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 7)
	fake.addPage("page-9", "sp-1", "old", 3)
	env := newCfPlanEnv(t, fake, &cfRemote{versions: map[string]string{"page-9": "3"}})
	if err := env.pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-prev", ConnectionID: "conn-cf", Provider: "confluence",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	if err := env.pubs.SettlePublication(context.Background(), 7, "act-prev", repoappconn.PublicationPublished, "page-9", "1", "{}"); err != nil {
		t.Fatal(err)
	}
	in := cfPlanInput()
	in.ParentPageID = ""
	in.PageID = "page-9"
	view, err := env.svc.FormPlan(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	cfApprove(t, env, view)
	// The PUT's reply is lost after the effect applied → unknown.
	fake.mu.Lock()
	fake.dropNextWrite = true
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
	fake.mu.Lock()
	putsBefore := fake.puts
	fake.mu.Unlock()
	rec, rerr := env.svc.Reconcile(context.Background(), 7, view.ActionID)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if rec.ActionState != appconn.ActionSucceeded || rec.Receipt.State != repoappconn.PublicationPublished {
		t.Fatalf("reconcile must resolve from the remote: %+v", rec)
	}
	fake.mu.Lock()
	putsAfter := fake.puts
	fake.mu.Unlock()
	if putsAfter != putsBefore {
		t.Fatalf("reconcile must not re-send: %d -> %d", putsBefore, putsAfter)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/modules/appconnector/publish/ -run 'TestConfluenceFormPlan|TestConfluenceExecute|TestConfluenceReconcile' -count=1`
Expected: FAIL（`undefined: NewConfluencePublishService`）。

- [ ] **Step 3: 最小实现**

创建 `internal/modules/appconnector/publish/confluence.go`：

```go
package publish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
)

// ConfluenceRemoteReader is the plan-formation pre-read port (satisfied
// by *ConfluenceBridge.ReadConfluencePageVersion).
type ConfluenceRemoteReader interface {
	ReadConfluencePageVersion(ctx context.Context, connectionID, pageID string) (string, error)
}

// ConfluencePublishService forms approved publish plans from confirmed
// artifact versions, executes them through the dedicated A03 ActionService
// (whose dispatcher/resolver is the Confluence bridge), and settles the
// publication receipt from the action row's authoritative outcome — the
// receipt is always a projection of the action, never the reverse. The
// plan/receipt views, the publication store and the error sentinels are
// shared with the Notion publish service verbatim; only the destination
// scope source, the remote pre-read, the content projection and the
// receipt parser are provider-specific.
type ConfluencePublishService struct {
	actions   *appconnectorsvc.ActionService
	store     appconnectorsvc.ActionStoreSource
	pubs      *repoappconn.PublicationStore
	artifacts ArtifactVersionReader
	content   ArtifactContentReader
	remote    ConfluenceRemoteReader
	scopes    ConfluenceScopeSource
}

// NewConfluencePublishService builds the publish service.
func NewConfluencePublishService(
	actions *appconnectorsvc.ActionService,
	store appconnectorsvc.ActionStoreSource,
	pubs *repoappconn.PublicationStore,
	artifacts ArtifactVersionReader,
	content ArtifactContentReader,
	remote ConfluenceRemoteReader,
	scopes ConfluenceScopeSource,
) *ConfluencePublishService {
	return &ConfluencePublishService{actions: actions, store: store, pubs: pubs, artifacts: artifacts, content: content, remote: remote, scopes: scopes}
}

// FormPlan forms one publish Action Plan (CONTEXT.md「操作计划/外部发布」):
// resolve the confirmed artifact version, derive the Confluence storage
// body deterministically, READ the external current version (AC1
// baseline; the update snapshot binds it as expected_version), then
// Prepare the A03 action whose normalized bytes + digest the approval
// binds, and record the planned publication row. For create the pre-read
// is the PARENT page's current version — a recorded baseline of the
// reviewed destination (nothing existing is overwritten, so it is never
// enforced); for update it is the authoritative conflict anchor.
func (s *ConfluencePublishService) FormPlan(ctx context.Context, in PublishPlanInput) (PublishPlanView, error) {
	if in.TenantID == 0 || in.ActorID == "" || in.ConnectionID == "" || in.SessionID == "" ||
		in.ArtifactVersionID == "" || strings.TrimSpace(in.Title) == "" {
		return PublishPlanView{}, fmt.Errorf("%w: tenant, actor, connection, session, artifact version and title are required", ErrPublishInvalidInput)
	}
	if (in.ParentPageID == "") == (in.PageID == "") {
		return PublishPlanView{}, fmt.Errorf("%w: exactly one of parent_page_id (create) or page_id (update)", ErrPublishInvalidInput)
	}
	scope, err := s.scopes.ConfluenceScope(ctx, in.ConnectionID)
	if err != nil {
		return PublishPlanView{}, fmt.Errorf("%w: connection scope: %v", ErrPublishInvalidInput, err)
	}
	if scope.AppID != appIDConfluence {
		return PublishPlanView{}, fmt.Errorf("%w: connection is %q, not confluence", ErrPublishInvalidInput, scope.AppID)
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
	storage, berr := ConfluenceStorageBody(string(content))
	if berr != nil {
		return PublishPlanView{}, berr
	}
	// AC1: 发布前读取外部当前版本 —— the plan-time pre-read. For update
	// this value is bound into the approved snapshot; for create it is the
	// destination's recorded baseline.
	remoteVersion, rerr := s.remote.ReadConfluencePageVersion(ctx, in.ConnectionID, destination)
	if rerr != nil || remoteVersion == "" {
		return PublishPlanView{}, fmt.Errorf("%w: %v", ErrPublishDestinationUnreadable, rerr)
	}
	var args []byte
	if mode == "update" {
		args, err = json.Marshal(map[string]any{
			"page_id": in.PageID, "expected_version": remoteVersion,
			"title": in.Title, "storage": storage,
		})
	} else {
		args, err = json.Marshal(map[string]any{
			"parent": in.ParentPageID, "title": in.Title, "storage": storage,
		})
	}
	if err != nil {
		return PublishPlanView{}, err
	}
	actionID, perr := s.actions.Prepare(ctx, appconn.Action{
		ID: "", TenantID: in.TenantID, ActorID: in.ActorID, ConnectionID: in.ConnectionID,
		Version: "confluence/v1", Target: destination, Risk: appconn.RiskWrite,
		AuthVersion: scope.AuthVersion, Args: args,
	})
	if perr != nil {
		return PublishPlanView{}, perr
	}
	if uerr := s.pubs.CreatePublication(ctx, repoappconn.PublicationRow{
		TenantID: in.TenantID, ActionID: actionID, ConnectionID: in.ConnectionID,
		Provider: "confluence", Mode: mode, Destination: destination,
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
		Title:    in.Title,
		Artifact: PublishArtifactView{VersionID: version.ID, Digest: version.Digest, MIME: version.MIME, Size: version.Size},
	}, nil
}

// Execute runs the approved plan through the dedicated ActionService and
// settles the receipt from the action row's authoritative outcome.
// ErrDispatchUnknown is NOT an error here: the outcome (unknown + receipt
// projection) reports the honest parked state for reconciliation.
func (s *ConfluencePublishService) Execute(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error) {
	err := s.actions.Execute(ctx, actionID)
	if err != nil && !errors.Is(err, appconnectorsvc.ErrDispatchUnknown) {
		return PublishExecuteOutcome{}, err
	}
	return s.project(ctx, tenantID, actionID)
}

// Reconcile resolves an unknown outcome by querying the PROVIDER first —
// never a re-dispatch — then settles the receipt from the action row.
func (s *ConfluencePublishService) Reconcile(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error) {
	err := s.actions.ResolveUnknown(ctx, actionID)
	if err != nil && !errors.Is(err, appconnectorsvc.ErrDispatchUnknown) {
		return PublishExecuteOutcome{}, err
	}
	return s.project(ctx, tenantID, actionID)
}

// Receipt returns the publication record's current projection.
func (s *ConfluencePublishService) Receipt(ctx context.Context, tenantID uint64, actionID string) (PublishReceiptView, error) {
	pub, err := s.pubs.FindByAction(ctx, tenantID, actionID)
	if err != nil {
		return PublishReceiptView{}, err
	}
	return publicationView(pub), nil
}

// project settles the receipt FROM the action row's authoritative state.
// Order matters: the action row is written by the ActionService first;
// only then does the receipt follow. A settle failure surfaces while the
// action state stays durable (the receipt can be re-settled).
func (s *ConfluencePublishService) project(ctx context.Context, tenantID uint64, actionID string) (PublishExecuteOutcome, error) {
	row, err := s.store.FindAction(ctx, actionID)
	if err != nil {
		return PublishExecuteOutcome{}, err
	}
	if row.TenantID != tenantID {
		return PublishExecuteOutcome{}, repoappconn.ErrActionNotFound
	}
	out := PublishExecuteOutcome{ActionState: row.State, Conflict: strings.HasPrefix(row.ProviderResult, ConfluenceVersionConflictResult)}
	switch row.State {
	case appconn.ActionSucceeded:
		rcpt, rerr := appconn.ParseConfluencePageReceipt([]byte(row.ProviderResult))
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

Run: `go test ./internal/modules/appconnector/publish/ -run 'TestConfluenceFormPlan|TestConfluenceExecute|TestConfluenceReconcile' -count=1`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/modules/appconnector/publish/confluence.go internal/modules/appconnector/publish/confluence_test.go
git commit -m "feat(publish): confluence publish service - form/execute/reconcile/settle (T20 #50)"
```

---

### Task 7: HTTP 面——`/apps/confluence-publish/*` 端点、路由注册与容器接线

**Files:**
- Create: `internal/handler/app_connector_confluence_publish.go`
- Create: `internal/router/routes_app_confluence_publish.go`
- Create: `internal/container/confluence_publish.go`
- Modify: `internal/router/router.go:145`（`RouterParams` 增 1 字段）与 `:436`（增 1 行注册调用）
- Modify: `internal/container/container.go:1013`（Notion Provide 块后增 1 行）
- Test: `internal/handler/app_connector_confluence_publish_test.go`

**Interfaces:**
- Consumes: Task 6 `publish.ConfluencePublishService`；既有 HTTP 面范式 `AppNotionPublishHandler`（`internal/handler/app_connector_notion_publish.go`——`appTenantScope`/`appFail`/`appOK`/`appRequireWriteCapability` 同包复用，零修改）；`appconnector.CanDriveActionWrites`（`access.go:43`）；容器装配范式 `newNotionPublishHandler`（`internal/container/notion_publish.go`——`tenantStorageArtifactContent` 同包复用，零修改）；dig 自动装配（`container.Provide` 后 dig 按 `RouterParams` 字段类型注入，`internal/router/router.go:28`）。
- Produces（Task 8 依赖）:
  - `type AppConfluencePublishHandler struct` + `func NewAppConfluencePublishHandler(db *gorm.DB) *AppConfluencePublishHandler` + `func (h *AppConfluencePublishHandler) SetConfluencePublishService(s *publish.ConfluencePublishService)` + `func (h *AppConfluencePublishHandler) RequireActionCapabilityForWrites() gin.HandlerFunc`
  - `func (h *AppConfluencePublishHandler) FormConfluencePublishPlan(c *gin.Context)`（POST /apps/confluence-publish/plans，201）
  - `func (h *AppConfluencePublishHandler) PublishConfluenceAction(c *gin.Context)`（POST /apps/confluence-publish/actions/:id/publish；冲突→409 `PUBLISH_VERSION_CONFLICT`）
  - `func (h *AppConfluencePublishHandler) ReconcileConfluenceAction(c *gin.Context)`（POST …/actions/:id/reconcile）
  - `func (h *AppConfluencePublishHandler) GetConfluencePublication(c *gin.Context)`（GET …/actions/:id）
  - `func RegisterAppConfluencePublishRoutes(r *gin.RouterGroup, h *handler.AppConfluencePublishHandler)`

- [ ] **Step 1: 写失败测试（谓词门）**

创建 `internal/handler/app_connector_confluence_publish_test.go`：

```go
package handler

// Confluence publish endpoint gates (T20 #50). The publish pipeline
// itself is service-level (publish package) and end-to-end (Task 8);
// these pin the HTTP predicates: role gate, personal-connection owner
// predicate, fail-closed unconfigured service, malformed input,
// tenant-scoped lookups.

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

func newConfluencePublishTestEngine(t *testing.T) (*gin.Engine, *AppConfluencePublishHandler, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(&appconnectorrepo.ConnectionRow{}, &appconnectorrepo.InstallationRow{}, &appconnectorrepo.AppVersion{}, &appconnectorrepo.ActionRow{}, &appconnectorrepo.ApprovalRow{}, &appconnectorrepo.PreAuthorizationRow{}, &appconnectorrepo.PublicationRow{}))
	// conn-cf is user-a's PERSONAL confluence connection.
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-cf", TenantID: 7, AppID: "confluence", AppVersion: "v1", State: "active"}).Error)
	require.NoError(t, db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('confluence', 'v1', '{"scopes":["write_content"],"approved_parents":["parent-1"]}', '{}')`).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 7, ID: "conn-cf", InstallationID: "inst-cf", Kind: "personal", OwnerID: "user-a", CredentialRef: "mcp_oauth_token:confluence", State: "active", AuthVersion: 1}).Error)

	h := NewAppConfluencePublishHandler(db)
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
	g := engine.Group("/api/v1/apps/confluence-publish", h.RequireActionCapabilityForWrites())
	g.POST("/plans", h.FormConfluencePublishPlan)
	g.POST("/actions/:id/publish", h.PublishConfluenceAction)
	g.POST("/actions/:id/reconcile", h.ReconcileConfluenceAction)
	g.GET("/actions/:id", h.GetConfluencePublication)
	return engine, h, db
}

func confluencePublishDo(t *testing.T, engine *gin.Engine, method, path, body string, headers ...string) *httptest.ResponseRecorder {
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

func TestConfluencePublishPlanGates(t *testing.T) {
	engine, h, _ := newConfluencePublishTestEngine(t)
	body := `{"connection_id":"conn-cf","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"parent-1"}`

	// Service not wired: fail closed 501, nothing formed.
	w := confluencePublishDo(t, engine, http.MethodPost, "/api/v1/apps/confluence-publish/plans", body)
	require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "PUBLISH_PIPELINE_NOT_CONFIGURED")
	require.NotNil(t, h)

	// Viewer role: the write gate refuses.
	w = confluencePublishDo(t, engine, http.MethodPost, "/api/v1/apps/confluence-publish/plans", body, "X-Test-Role", "viewer")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// Another member may not use user-a's PERSONAL connection (same
	// predicate as PrepareAction).
	w = confluencePublishDo(t, engine, http.MethodPost, "/api/v1/apps/confluence-publish/plans", body, "X-Test-User", "user-b", "X-Test-Role", "admin")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "NOT_CONNECTION_OWNER")

	// Malformed input: both destination shapes at once.
	w = confluencePublishDo(t, engine, http.MethodPost, "/api/v1/apps/confluence-publish/plans",
		`{"connection_id":"conn-cf","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"p","page_id":"q"}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "INVALID_REQUEST")

	// Unknown connection: 404 without leaking existence details.
	w = confluencePublishDo(t, engine, http.MethodPost, "/api/v1/apps/confluence-publish/plans",
		`{"connection_id":"conn-nope","session_id":"s1","artifact_version_id":"v1","title":"T","parent_page_id":"parent-1"}`)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

func TestConfluencePublishActionLookupIsTenantScoped(t *testing.T) {
	engine, h, db := newConfluencePublishTestEngine(t)
	h.SetConfluencePublishService(nil) // stays unconfigured; lookup gates first
	require.NoError(t, db.Create(&appconnectorrepo.ActionRow{ID: "act-x", TenantID: 7, ConnectionID: "conn-cf",
		AppVersion: "confluence/v1", Target: "parent-1", Risk: "write", ArgsSnapshot: "{}", ArgsDigest: "d",
		State: "authorized"}).Error)
	// Same tenant: reaches the unconfigured refusal (501).
	w := confluencePublishDo(t, engine, http.MethodPost, "/api/v1/apps/confluence-publish/actions/act-x/publish", "")
	require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
	// Foreign tenant id is indistinguishable from missing: 404.
	w = confluencePublishDo(t, engine, http.MethodPost, "/api/v1/apps/confluence-publish/actions/act-x/publish", "", "X-Test-Tenant", "8")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = confluencePublishDo(t, engine, http.MethodGet, "/api/v1/apps/confluence-publish/actions/act-x", "", "X-Test-Tenant", "8")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}
```

（`publishTestContext` 复用同包既有测试助手 `app_connector_notion_publish_test.go:26`，零修改。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/handler/ -run 'TestConfluencePublishPlanGates|TestConfluencePublishActionLookupIsTenantScoped' -count=1`
Expected: FAIL（`undefined: NewAppConfluencePublishHandler`）。

- [ ] **Step 3: 最小实现（三个新文件 + 两处单行接线）**

创建 `internal/handler/app_connector_confluence_publish.go`：

```go
package handler

import (
	"errors"
	"net/http"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AppConfluencePublishHandler serves the T20 Confluence publish closed
// loop under /api/v1/apps/confluence-publish. Plan formation derives the
// approved snapshot SERVER-SIDE from a confirmed artifact version +
// destination; approval stays on the existing POST
// /apps/actions/:id/approve endpoint (its predicate owns approval
// authority); execution/reconciliation run through the publish service
// whose ActionService holds the Confluence bridge dispatcher. The handler
// is nil-service fail-closed like its siblings.
type AppConfluencePublishHandler struct {
	db      *gorm.DB
	publish *publish.ConfluencePublishService
}

// NewAppConfluencePublishHandler constructs the handler over the business DB.
func NewAppConfluencePublishHandler(db *gorm.DB) *AppConfluencePublishHandler {
	return &AppConfluencePublishHandler{db: db}
}

// SetConfluencePublishService wires the publish service (container
// injection point). Until called every endpoint fails closed with 501
// PUBLISH_PIPELINE_NOT_CONFIGURED (mirroring the frozen action pipeline).
func (h *AppConfluencePublishHandler) SetConfluencePublishService(s *publish.ConfluencePublishService) {
	h.publish = s
}

// RequireActionCapabilityForWrites mirrors the action write gate
// (CanDriveActionWrites): plan formation and execution are action writes.
func (h *AppConfluencePublishHandler) RequireActionCapabilityForWrites() gin.HandlerFunc {
	return appRequireWriteCapability(appconnector.CanDriveActionWrites,
		"FORBIDDEN_ACTION_WRITE",
		"publish writes require owner or admin role")
}

type confluencePublishPlanInput struct {
	ConnectionID      string `json:"connection_id"`
	SessionID         string `json:"session_id"`
	ArtifactVersionID string `json:"artifact_version_id"`
	Title             string `json:"title"`
	ParentPageID      string `json:"parent_page_id"`
	PageID            string `json:"page_id"`
}

// confluenceActionByID resolves an action inside the tenant — a
// cross-tenant id and a missing one are both 404, never a 403 that leaks
// existence.
func (h *AppConfluencePublishHandler) confluenceActionByID(c *gin.Context, tenantID uint64, id string) (appconnectorrepo.ActionRow, bool) {
	var row appconnectorrepo.ActionRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
		appFail(c, http.StatusNotFound, "ACTION_NOT_FOUND", "action not found")
		return row, false
	}
	return row, true
}

func (h *AppConfluencePublishHandler) confluenceConnection(c *gin.Context, tenantID uint64, connectionID, userID string) (appconnectorrepo.ConnectionRow, bool) {
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

// FormConfluencePublishPlan POST /apps/confluence-publish/plans — derives
// the approved publish snapshot server-side (artifact version +
// destination), reads the external current version, prepares the A03
// action and records the planned publication. The response carries the
// digest + fence an approval binds.
func (h *AppConfluencePublishHandler) FormConfluencePublishPlan(c *gin.Context) {
	tenantID, _, userID, ok := appTenantScope(c)
	if !ok {
		return
	}
	var input confluencePublishPlanInput
	if err := c.ShouldBindJSON(&input); err != nil || input.ConnectionID == "" || input.SessionID == "" ||
		input.ArtifactVersionID == "" || input.Title == "" ||
		(input.ParentPageID == "") == (input.PageID == "") {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST",
			"connection_id, session_id, artifact_version_id, title and exactly one of parent_page_id / page_id are required")
		return
	}
	if _, ok := h.confluenceConnection(c, tenantID, input.ConnectionID, userID); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Confluence publish pipeline is not wired in this environment; refusing to form a plan")
		return
	}
	view, err := h.publish.FormPlan(c.Request.Context(), publish.PublishPlanInput{
		TenantID: tenantID, ActorID: userID, ConnectionID: input.ConnectionID,
		SessionID: input.SessionID, ArtifactVersionID: input.ArtifactVersionID,
		Title: input.Title, ParentPageID: input.ParentPageID, PageID: input.PageID,
	})
	if err != nil {
		h.failConfluence(c, err)
		return
	}
	appOK(c, http.StatusCreated, view)
}

// PublishConfluenceAction POST /apps/confluence-publish/actions/:id/publish —
// executes the APPROVED plan through the publish service. An unobservable
// provider outcome returns 200 with the parked unknown state (mirror of
// ExecuteAction); reconciliation is the only resolution path.
func (h *AppConfluencePublishHandler) PublishConfluenceAction(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	if _, ok := h.confluenceActionByID(c, tenantID, c.Param("id")); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Confluence publish pipeline is not wired in this environment; refusing to dispatch")
		return
	}
	outcome, err := h.publish.Execute(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		h.failConfluenceExecute(c, err)
		return
	}
	if outcome.Conflict {
		appFail(c, http.StatusConflict, "PUBLISH_VERSION_CONFLICT",
			"the external document changed since approval; form a new plan from its current version")
		return
	}
	appOK(c, http.StatusOK, outcome)
}

// ReconcileConfluenceAction POST
// /apps/confluence-publish/actions/:id/reconcile — resolves an unknown
// outcome by querying the provider first (AC2).
func (h *AppConfluencePublishHandler) ReconcileConfluenceAction(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	if _, ok := h.confluenceActionByID(c, tenantID, c.Param("id")); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Confluence publish pipeline is not wired in this environment; refusing to reconcile")
		return
	}
	outcome, err := h.publish.Reconcile(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		h.failConfluenceExecute(c, err)
		return
	}
	appOK(c, http.StatusOK, outcome)
}

// GetConfluencePublication GET /apps/confluence-publish/actions/:id — the
// plan + receipt view (计划/回执查询).
func (h *AppConfluencePublishHandler) GetConfluencePublication(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	if _, ok := h.confluenceActionByID(c, tenantID, c.Param("id")); !ok {
		return
	}
	if h.publish == nil {
		appFail(c, http.StatusNotImplemented, "PUBLISH_PIPELINE_NOT_CONFIGURED",
			"the Confluence publish pipeline is not wired in this environment")
		return
	}
	view, err := h.publish.Receipt(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		h.failConfluence(c, err)
		return
	}
	appOK(c, http.StatusOK, view)
}

func (h *AppConfluencePublishHandler) failConfluence(c *gin.Context, err error) {
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
		appFail(c, http.StatusConflict, "PUBLISH_UPDATE_TARGET_NOT_PUBLISHED", "only pages this site published through this connection may be updated")
	default:
		appFail(c, http.StatusInternalServerError, "PUBLISH_PLAN_FAILED", "failed to form the publish plan")
	}
}

func (h *AppConfluencePublishHandler) failConfluenceExecute(c *gin.Context, err error) {
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

创建 `internal/router/routes_app_confluence_publish.go`：

```go
package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterAppConfluencePublishRoutes registers the T20 Confluence publish
// closed loop under /api/v1/apps/confluence-publish. Like the
// app-connector routes, these are intentionally NOT declared in the
// API-key route authorizer: the /api/v1 gate default-denies every
// X-API-Key principal. The group carries the action write gate; approval
// authority stays on the existing POST /apps/actions/:id/approve predicate.
func RegisterAppConfluencePublishRoutes(r *gin.RouterGroup, h *handler.AppConfluencePublishHandler) {
	if h == nil {
		return
	}
	g := r.Group("/apps/confluence-publish", h.RequireActionCapabilityForWrites())
	{
		g.POST("/plans", h.FormConfluencePublishPlan)
		g.POST("/actions/:id/publish", h.PublishConfluenceAction)
		g.POST("/actions/:id/reconcile", h.ReconcileConfluenceAction)
		g.GET("/actions/:id", h.GetConfluencePublication)
	}
}
```

创建 `internal/container/confluence_publish.go`：

```go
package container

import (
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/publish"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/types/interfaces"

	"gorm.io/gorm"
)

// newConfluencePublishHandler is the T20 (#50) dig constructor: it builds
// the dedicated confluence publish ActionService (same ActionStore
// authority, same A02 guard, same U05 gate; its dispatcher AND unknown
// resolver are the Confluence bridge), the publish service, and the HTTP
// handler. The frozen OC-armed ActionService and the #48 Notion publish
// instance are untouched — the three services share the store, which the
// design names as the authority (action.go:158-160).
func newConfluencePublishHandler(
	db *gorm.DB,
	store appconnectorsvc.ActionStoreSource,
	guard appconnectorsvc.A02Guard,
	gate domain.ExecutionGate,
	creds appconnectorsvc.ConnectionCredentialSource,
	files interfaces.FileService,
	tenants interfaces.TenantService,
	storage interfaces.StorageBackendResolver,
	versions *repository.ArtifactVersionStore,
) (*handler.AppConfluencePublishHandler, error) {
	pubs := repoappconn.NewPublicationStore(db)
	scopeSrc := publish.NewDBConfluenceScopeSource(db)
	bridge := publish.NewConfluenceBridge(
		scopeSrc,
		publish.NewConfluencePolicyProvider(scopeSrc),
		publish.NewConfluenceCredentialTokenSource(appconnectorsvc.NewCredentialResolver(creds)),
	)
	actions := appconnectorsvc.NewActionService(store, guard, gate, bridge, bridge)
	svc := publish.NewConfluencePublishService(actions, store, pubs, versions,
		&tenantStorageArtifactContent{files: files, tenants: tenants, storage: storage}, bridge,
		scopeSrc)
	h := handler.NewAppConfluencePublishHandler(db)
	h.SetConfluencePublishService(svc)
	return h, nil
}
```

修改 `internal/router/router.go` 第 145 行 `AppNotionPublishHandler *handler.AppNotionPublishHandler` 之后插入一行：

```go
	AppConfluencePublishHandler *handler.AppConfluencePublishHandler
```

第 436 行 `RegisterAppNotionPublishRoutes(v1, params.AppNotionPublishHandler)` 之后插入一行：

```go
		RegisterAppConfluencePublishRoutes(v1, params.AppConfluencePublishHandler)
```

修改 `internal/container/container.go` 第 1013 行 `must(container.Provide(newNotionPublishHandler))` 之后插入：

```go
	// T20 (#50): the Confluence publish closed loop — same shape as the
	// Notion publish wiring, its own dedicated ActionService instance
	// (bridge dispatcher + resolver), frozen services untouched.
	must(container.Provide(newConfluencePublishHandler))
```

- [ ] **Step 4: 运行测试确认通过（含编译接线验证）**

Run: `go build ./... && go test ./internal/handler/ -run 'TestConfluencePublishPlanGates|TestConfluencePublishActionLookupIsTenantScoped|TestNotionPublishPlanGates' -count=1`
Expected: PASS（`go build` 证明 dig 构造器签名与注入依赖全部可解析；Notion 门测试复跑证明零回归）。

- [ ] **Step 5: 提交**

```bash
git add internal/handler/app_connector_confluence_publish.go internal/handler/app_connector_confluence_publish_test.go internal/router/routes_app_confluence_publish.go internal/container/confluence_publish.go internal/router/router.go internal/container/container.go
git commit -m "feat(appconnector): /apps/confluence-publish endpoints, routes and container wiring (T20 #50)"
```

---

### Task 8: E2E——最高稳定 Interface 证据（生产迁移 + 全链 + 契约双打）

**Files:**
- Create: `internal/handler/app_connector_confluence_publish_e2e_test.go`

**Interfaces:**
- Consumes: Task 1–7 全部产出；既有 E2E 基建 `openNotionPublishE2EDB` 的迁移加载范式（`internal/handler/app_connector_notion_publish_e2e_test.go:53-71`）、可复用助手 `e2ePolicyProvider`/`e2ePassGuard`/`e2eLocalContent`（同包测试文件，零修改复用）；`repository.NewAgentRunStore(db).Admit`（run 种子，`internal/application/repository/agent_run.go`）；`file.NewLocalFileService`。
- Produces: AC3 的本地最高稳定 Interface 证据——除 Confluence 网络端点被本地契约双打替换外全真（生产 sqlite 迁移、真实 ActionService/PublicationStore/ConfluenceBridge/ConfluencePublishService/生产 DBConfluenceScopeSource/gin 处理器/既有审批端点）。

- [ ] **Step 1: 写 E2E 测试（先跑确认编译失败 = RED）**

创建 `internal/handler/app_connector_confluence_publish_e2e_test.go`：

```go
package handler

// End-to-end evidence for T20 (#50). Everything here runs on a FULLY
// MIGRATED sqlite database (the production migrations/sqlite track) with
// the REAL ActionService / PublicationStore / ConfluenceBridge /
// ConfluencePublishService / DBConfluenceScopeSource / handlers and the
// REAL approval endpoint. The ONLY replaced piece is the Confluence wire
// endpoint: a local contract double (e2eConfluence) implementing the
// official Cloud v2 page contract (GET/POST/PUT /wiki/api/v2/pages) with
// HTTP Basic auth and the monotonic version.number gate. This is the
// highest stable Interface evidence available without provider
// credentials — it is NOT the real-provider acceptance, which stays
// CONFLUENCE_*-gated (Task 9) and honestly SKIPs in this environment.

import (
	"context"
	"database/sql"
	"encoding/base64"
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
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---- production-migrated database ----

func openConfluencePublishE2EDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "confluence-publish.db") + "?_foreign_keys=on&_busy_timeout=5000"
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

// ---- the contract double for the Confluence wire (Cloud v2, /wiki
// context path; NOT the real-provider acceptance) ----

type e2eConfluence struct {
	mu       sync.Mutex
	username string
	secret   string
	pages    map[string]*e2eCfPage
	nextID   int
	postCalls int
	putCalls  int
	dropNextPut bool
}

type e2eCfPage struct {
	id, spaceID, title, storage string
	version                     int
}

func newE2EConfluence(username, secret string) *e2eConfluence {
	return &e2eConfluence{username: username, secret: secret, pages: map[string]*e2eCfPage{}}
}

func (e *e2eConfluence) addPage(id, spaceID, title string, version int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pages[id] = &e2eCfPage{id: id, spaceID: spaceID, title: title, version: version}
}

// bump applies an EXTERNAL collaborator's edit between approval and
// publish (the AC1 window).
func (e *e2eConfluence) bump(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if p, ok := e.pages[id]; ok {
		p.version++
	}
}

func e2eCfJSON(p *e2eCfPage, withBody bool) string {
	storage := ""
	if withBody {
		raw, _ := json.Marshal(p.storage)
		storage = fmt.Sprintf(`,"body":{"storage":{"value":%s}}`, raw)
	}
	return fmt.Sprintf(`{"id":%q,"status":"current","title":%q,"spaceId":%q,"version":{"number":%d,"createdAt":"2026-09-26T08:00:00Z"}%s}`,
		p.id, p.title, p.spaceID, p.version, storage)
}

func (e *e2eConfluence) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
	auth := func(r *http.Request) bool {
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte(e.username+":"+e.secret))
		return r.Header.Get("Authorization") == want
	}
	mux.HandleFunc("/wiki/api/v2/pages", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"errors":[{"title":"unauthorized"}]}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			SpaceID  string `json:"space_id"`
			ParentID string `json:"parent_id"`
			Title    string `json:"title"`
			Body     struct {
				Value string `json:"value"`
			} `json:"body"`
		}
		_ = json.Unmarshal(body, &req)
		e.mu.Lock()
		parent, ok := e.pages[req.ParentID]
		e.mu.Unlock()
		if !ok || req.SpaceID == "" || req.Title == "" || req.Body.Value == "" {
			writeJSON(w, 400, `{"errors":[{"title":"invalid create request"}]}`)
			return
		}
		e.mu.Lock()
		e.nextID++
		e.postCalls++
		np := &e2eCfPage{id: fmt.Sprintf("cf-%d", e.nextID), spaceID: req.SpaceID, title: req.Title, storage: req.Body.Value, version: 1}
		e.pages[np.id] = np
		e.mu.Unlock()
		writeJSON(w, 200, e2eCfJSON(np, false))
	})
	mux.HandleFunc("/wiki/api/v2/pages/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"errors":[{"title":"unauthorized"}]}`)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/wiki/api/v2/pages/")
		e.mu.Lock()
		p, ok := e.pages[id]
		e.mu.Unlock()
		if !ok {
			writeJSON(w, 404, `{"errors":[{"title":"not found"}]}`)
			return
		}
		switch r.Method {
		case http.MethodGet:
			e.mu.Lock()
			e.mu.Unlock()
			writeJSON(w, 200, e2eCfJSON(p, true))
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Title string `json:"title"`
				Body  struct {
					Value string `json:"value"`
				} `json:"body"`
				Version struct {
					Number int `json:"number"`
				} `json:"version"`
			}
			_ = json.Unmarshal(body, &req)
			if req.Version.Number != p.version+1 {
				writeJSON(w, 409, `{"errors":[{"title":"version conflict"}]}`)
				return
			}
			e.mu.Lock()
			e.putCalls++
			p.title, p.storage, p.version = req.Title, req.Body.Value, req.Version.Number
			drop := e.dropNextPut
			if drop {
				e.dropNextPut = false
			}
			e.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, e2eCfJSON(p, false))
		default:
			writeJSON(w, 405, `{"errors":[{"title":"method not allowed"}]}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// ---- the full-stack environment ----

type confluenceE2EEnv struct {
	engine *gin.Engine
	db     *gorm.DB
	fake   *e2eConfluence
}

func newConfluencePublishE2E(t *testing.T) *confluenceE2EEnv {
	t.Helper()
	db := openConfluencePublishE2EDB(t)
	// Tenants / users / membership (column sets verified against the
	// sqlite migration track — same seeds as the #48 E2E).
	require.NoError(t, db.Exec(`INSERT INTO tenants (id, name, business) VALUES (7, 't7', 'test')`).Error)
	for _, u := range []string{"user-a", "user-b"} {
		require.NoError(t, db.Exec(`INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, 'x', 7)`, u, u, u+"@example.test").Error)
		require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id, user_id, role, status) VALUES (7, ?, 'contributor', 'active')`, u).Error)
	}
	// Task = Session (ADR-0004) + an admitted run for the artifact version
	// binding.
	require.NoError(t, db.Exec(`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('sess-1', 7, 'task-1', 'user-a', 'trpc')`).Error)
	_, err := repository.NewAgentRunStore(db).Admit(context.Background(), agentruntime.Admission{
		Key: agentruntime.RunKey{TenantID: 7, RunID: "run-1"}, SessionID: "sess-1", UserID: "user-a",
		RequestID: "q1", AssistantMessageID: "a1", RequestHash: "hash-1",
		Snapshot: json.RawMessage(`{"version":1}`), UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
		AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	// The confirmed artifact version + its real bytes on a REAL local file
	// service (the production content-reader path minus tenant backend
	// routing).
	baseDir := t.TempDir()
	artifactText := "first para.\n\nsecond para."
	digest := strings.Repeat("b", 64)
	objectKey := fmt.Sprintf("artifact-versions/7/run-1/%s", digest)
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, filepath.Dir(objectKey)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, objectKey), []byte(artifactText), 0o644))
	require.NoError(t, db.Exec(`INSERT INTO artifact_versions (tenant_id, id, run_id, session_id, digest, object_key, mime, scan_state, size)
		VALUES (7, 'ver-1', 'run-1', 'sess-1', ?, ?, 'text/plain', 'ready', ?)`, digest, objectKey, len(artifactText)).Error)
	// The Confluence double FIRST: its loopback URL seeds the installation's
	// reviewed base_url (production scope resolution reads it back).
	fake := newE2EConfluence("user-a@test.example", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Workspace", 7)
	srv := fake.server(t)
	// Confluence installation + reviewed app version (write scope +
	// approved parents) + user-a's personal connection + its credential
	// row (the reviewed JSON credential shape, Basic username+secret).
	require.NoError(t, db.Exec(`INSERT INTO installations (id, tenant_id, app_id, app_version, state, version, config_json)
		VALUES ('inst-cf', 7, 'confluence', 'v1', 'active', 1, ?)`, fmt.Sprintf(`{"base_url":%q,"edition":"cloud"}`, srv.URL+"/wiki")).Error)
	require.NoError(t, db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('confluence', 'v1', '{"scopes":["write_content"],"approved_parents":["parent-1"]}', '{}')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO connections (tenant_id, id, installation_id, kind, owner_id, credential_ref, state, auth_version)
		VALUES (7, 'conn-cf', 'inst-cf', 'personal', 'user-a', 'mcp_oauth_token:confluence', 'active', 1)`).Error)
	// mcp_oauth_tokens.service_id is a FOREIGN KEY to mcp_services(id) —
	// the service row must exist first.
	require.NoError(t, db.Exec(`INSERT INTO mcp_services (id, tenant_id, name, transport_type) VALUES ('confluence', 7, 'confluence', 'http')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO mcp_oauth_tokens (id, tenant_id, user_id, principal_type, principal_id, service_id, access_token, token_type, expires_at)
		VALUES ('tok-cf', 7, 'user-a', 'web_user', 'user-a', 'confluence', '{"username":"user-a@test.example","secret":"secret_cf_token"}', 'bearer', '2099-01-01 00:00:00')`).Error)

	// The production composition: REAL DBConfluenceScopeSource (base_url +
	// schema above) and REAL credential token source; only the outbound
	// POLICY is the injected loopback-authorized variant — the documented
	// HTTPPolicy test hook (http_policy.go:63-66) that lets the public-
	// address dial gate accept 127.0.0.0/8 for the httptest double.
	pubs := repoappconn.NewPublicationStore(db)
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	_, loopback, _ := net.ParseCIDR("127.0.0.0/8")
	pol := appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PUT"}, PathPrefix: "/wiki/",
		AuthorizedNetworks: []*net.IPNet{loopback}, Timeout: 10 * time.Second,
	}
	scopeSrc := publish.NewDBConfluenceScopeSource(db)
	bridge := publish.NewConfluenceBridge(scopeSrc, &e2ePolicyProvider{pol: pol},
		publish.NewConfluenceCredentialTokenSource(appconnectorsvc.NewCredentialResolver(repository.NewMCPOAuthBindingStore(db))))
	store := repoappconn.NewActionStore(db)
	publishActions := appconnectorsvc.NewActionService(store, &e2ePassGuard{}, nil, bridge, bridge)
	content := &e2eLocalContent{svc: file.NewLocalFileService(baseDir, "")}
	svc := publish.NewConfluencePublishService(publishActions, store, pubs, repository.NewArtifactVersionStore(db), content, bridge, scopeSrc)

	publishHandler := NewAppConfluencePublishHandler(db)
	publishHandler.SetConfluencePublishService(svc)
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
	// Mirror of RegisterAppConfluencePublishRoutes (internal/router) — the
	// router package is not importable from this handler-package test.
	g := v1.Group("/apps/confluence-publish", publishHandler.RequireActionCapabilityForWrites())
	{
		g.POST("/plans", publishHandler.FormConfluencePublishPlan)
		g.POST("/actions/:id/publish", publishHandler.PublishConfluenceAction)
		g.POST("/actions/:id/reconcile", publishHandler.ReconcileConfluenceAction)
		g.GET("/actions/:id", publishHandler.GetConfluencePublication)
	}
	return &confluenceE2EEnv{engine: engine, db: db, fake: fake}
}

func (e *confluenceE2EEnv) do(t *testing.T, method, path, body string, headers ...string) *httptest.ResponseRecorder {
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

func (e *confluenceE2EEnv) formPlan(t *testing.T, body string) map[string]any {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/plans", body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var parsed struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parsed))
	return parsed.Data
}

func (e *confluenceE2EEnv) approve(t *testing.T, plan map[string]any) {
	t.Helper()
	// Route through the EXISTING approval endpoint: approval authority
	// stays on the frozen ApproveAction predicate.
	body := fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, plan["digest"], int(plan["expected_version"].(float64)))
	w := e.do(t, http.MethodPost, "/api/v1/apps/actions/"+plan["action_id"].(string)+"/approve", body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// TestConfluencePublishEndToEndCreateApprovePublishReceipt: the full AC3
// chain — form (artifact version + destination, server-derived snapshot,
// external baseline read through the PRODUCTION scope source) → approve
// on the EXISTING endpoint → publish → receipt with external id +
// version; the publication row lands 'published'.
func TestConfluencePublishEndToEndCreateApprovePublishReceipt(t *testing.T) {
	env := newConfluencePublishE2E(t)
	plan := env.formPlan(t, `{"connection_id":"conn-cf","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report","parent_page_id":"parent-1"}`)
	require.Equal(t, "create", plan["mode"])
	require.Equal(t, "parent-1", plan["destination"])
	require.Equal(t, "7", plan["expected_external_version"], "AC1 baseline: the plan read the parent page's current version")
	require.NotEmpty(t, plan["digest"])

	env.approve(t, plan)

	w := env.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/actions/"+plan["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Data struct {
			ActionState string `json:"action_state"`
			Publication struct {
				State             string `json:"state"`
				ExternalID        string `json:"external_id"`
				ExternalVersion   string `json:"external_version"`
				ArtifactVersionID string `json:"artifact_version_id"`
			} `json:"publication"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "succeeded", out.Data.ActionState)
	require.Equal(t, "published", out.Data.Publication.State)
	require.NotEmpty(t, out.Data.Publication.ExternalID)
	require.Equal(t, "1", out.Data.Publication.ExternalVersion, "the receipt must save the version the publish produced")
	require.Equal(t, "ver-1", out.Data.Publication.ArtifactVersionID)

	// The receipt is durable and queryable through the GET endpoint.
	w = env.do(t, http.MethodGet, "/api/v1/apps/confluence-publish/actions/"+plan["action_id"].(string), "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"external_version"`)

	// The published page exists remotely with the derived storage body.
	env.fake.mu.Lock()
	created := env.fake.pages[out.Data.Publication.ExternalID]
	env.fake.mu.Unlock()
	require.NotNil(t, created)
	require.Equal(t, "<p>first para.</p>\n<p>second para.</p>", created.storage, "two paragraphs derived from the artifact text")
}

// TestConfluencePublishEndToEndUpdateConflict: AC1 — an external edit
// between plan formation and publish is detected at execute time and the
// publish is refused with 409 and ZERO write requests; the update is only
// reachable through a page this connection published before.
func TestConfluencePublishEndToEndUpdateConflict(t *testing.T) {
	env := newConfluencePublishE2E(t)
	// First publish to mint a receipt-backed target page.
	plan1 := env.formPlan(t, `{"connection_id":"conn-cf","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report","parent_page_id":"parent-1"}`)
	env.approve(t, plan1)
	w := env.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/actions/"+plan1["action_id"].(string)+"/publish", "")
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
	plan2 := env.formPlan(t, fmt.Sprintf(`{"connection_id":"conn-cf","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report v2","page_id":%q}`, pageID))
	require.Equal(t, "update", plan2["mode"])
	require.Equal(t, "1", plan2["expected_external_version"])
	env.approve(t, plan2)

	// External collaborator edits between approval and publish: version 1 → 2.
	env.fake.mu.Lock()
	putsBefore := env.fake.putCalls
	env.fake.mu.Unlock()
	env.fake.bump(pageID)

	w = env.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "PUBLISH_VERSION_CONFLICT")
	env.fake.mu.Lock()
	putsAfter := env.fake.putCalls
	env.fake.mu.Unlock()
	require.Equal(t, putsBefore, putsAfter, "AC1: conflict leaves ZERO page writes")
}

// TestConfluencePublishEndToEndUnknownReconcilesRemoteFirst: AC2 — a lost
// write reply parks the action unknown; a second publish is structurally
// refused; reconcile reads the REMOTE first and settles the receipt, with
// no re-dispatch ever.
func TestConfluencePublishEndToEndUnknownReconcilesRemoteFirst(t *testing.T) {
	env := newConfluencePublishE2E(t)
	plan1 := env.formPlan(t, `{"connection_id":"conn-cf","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report","parent_page_id":"parent-1"}`)
	env.approve(t, plan1)
	w := env.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/actions/"+plan1["action_id"].(string)+"/publish", "")
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

	// The update plan; the PUT's reply is lost after the effect applied.
	plan2 := env.formPlan(t, fmt.Sprintf(`{"connection_id":"conn-cf","session_id":"sess-1","artifact_version_id":"ver-1","title":"Report v2","page_id":%q}`, pageID))
	env.approve(t, plan2)
	env.fake.mu.Lock()
	env.fake.dropNextPut = true
	putsBefore := env.fake.putCalls
	env.fake.mu.Unlock()

	w = env.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"unknown"`, "an unobservable outcome must park unknown, never fabricate: %s", w.Body.String())

	// Blind re-publish is refused by the store (claim only from
	// authorized); the double's write count stays put.
	w = env.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/actions/"+plan2["action_id"].(string)+"/publish", "")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	env.fake.mu.Lock()
	require.Equal(t, putsBefore+1, env.fake.putCalls, "the dropped put counted once; NO re-dispatch happened")
	putsAfterUnknown := env.fake.putCalls
	env.fake.mu.Unlock()

	// Reconcile: provider query FIRST — the effect applied remotely, so
	// the query settles success and the receipt lands published.
	w = env.do(t, http.MethodPost, "/api/v1/apps/confluence-publish/actions/"+plan2["action_id"].(string)+"/reconcile", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"action_state":"succeeded"`, w.Body.String())
	require.Contains(t, w.Body.String(), `"state":"published"`, w.Body.String())
	env.fake.mu.Lock()
	require.Equal(t, putsAfterUnknown, env.fake.putCalls, "reconcile must not re-send")
	env.fake.mu.Unlock()
}
```

- [ ] **Step 2: 运行确认 RED（文件先于实现不存在时为编译失败；Task 7 完成后本步为直接 GREEN 验证）**

Run: `go test ./internal/handler/ -run 'TestConfluencePublishEndToEnd' -count=1`
Expected: PASS（若 Task 1–7 已全部完成；任何 FAIL 都指向具体链路，按 TestMain 输出定位）。

- [ ] **Step 3: 全链复跑（含 #48 回归，证明发布 seam 双实例互不干扰）**

Run: `go test ./internal/handler/ -run 'TestConfluencePublishEndToEnd|TestNotionPublishEndToEnd|TestAppPublicationsTable' -count=1`
Expected: PASS（Notion 与 Confluence 两套发布闭环在同一包内并存全绿）。

- [ ] **Step 4: 提交**

```bash
git add internal/handler/app_connector_confluence_publish_e2e_test.go
git commit -m "test(appconnector): confluence publish end-to-end evidence on production migrations (T20 #50)"
```

---

### Task 9: 真实受控集成证据（blocked-env，opt-in）

**Files:**
- Create: `internal/modules/appconnector/confluence_publish_real_test.go`

**Interfaces:**
- Consumes: Task 1–3 的真实适配器（`ConfluenceCreateAdapter`/`ConfluenceUpdateAdapter`/`ReadConfluencePageVersion`/`ParseConfluenceBaseURL`/`ParseConfluencePageReceipt`/`ErrConfluenceVersionConflict`）；`HTTPPolicy`。
- Produces: 真实 Provider 证据 `TestConfluenceRealPublishLoop`——**仅当环境注入真实凭据时执行**；本环境无凭据，SKIP 且 skip 不是 pass（与 `notion_publish_real_test.go:24` 同款纪律）。

- [ ] **Step 1: 写测试**

创建 `internal/modules/appconnector/confluence_publish_real_test.go`：

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

// Real-provider controlled evidence for the Confluence publish loop
// (NO-04 discipline): the REAL adapters run the full closed loop against
// the configured Confluence site: create under the designated test parent
// page (baseline version read) → receipt → update against the SAME page
// (version read + conflict-free write) → a stale-version update must be
// REFUSED with zero writes. Gated on CONFLUENCE_BASE_URL /
// CONFLUENCE_EMAIL / CONFLUENCE_API_TOKEN / CONFLUENCE_PARENT_PAGE_ID; a
// skip is never a pass — T20 real-provider evidence stays blocked-env in
// environments without these.
func TestConfluenceRealPublishLoop(t *testing.T) {
	baseURL := os.Getenv("CONFLUENCE_BASE_URL")
	email := os.Getenv("CONFLUENCE_EMAIL")
	token := os.Getenv("CONFLUENCE_API_TOKEN")
	parent := os.Getenv("CONFLUENCE_PARENT_PAGE_ID")
	if baseURL == "" || email == "" || token == "" || parent == "" || strings.HasPrefix(parent, "xxxx") {
		t.Skip("confluence real credentials not configured (CONFLUENCE_BASE_URL/CONFLUENCE_EMAIL/CONFLUENCE_API_TOKEN/CONFLUENCE_PARENT_PAGE_ID); skip is not a pass — T20 real-provider evidence stays blocked-env")
	}
	ep, err := ParseConfluenceBaseURL(baseURL, "")
	if err != nil {
		t.Fatalf("CONFLUENCE_BASE_URL: %v", err)
	}
	pol := HTTPPolicy{
		Scheme: ep.Scheme, Host: ep.Host, Port: ep.Port,
		Methods:    []string{"GET", "POST", "PUT"},
		PathPrefix: ep.APIBasePath + "/",
		Timeout:    30 * time.Second,
	}
	cred := func(ctx context.Context) (ConfluenceCredential, error) {
		return ConfluenceCredential{Username: email, Secret: token}, nil
	}
	caps := func(ctx context.Context, a Action) ([]string, error) { return []string{ConfluenceCapabilityWrite}, nil }
	stamp := time.Now().Format("150405")

	create := &ConfluenceCreateAdapter{
		Policy: pol, Credential: cred, Edition: ep.Edition, APIBasePath: ep.APIBasePath,
		ApprovedParents:        []string{parent},
		ConnectionCapabilities: caps,
	}
	createArgs, _ := json.Marshal(map[string]any{
		"parent": parent, "title": "T20 real " + stamp,
		"storage": "<p>t20 real publish " + stamp + "</p>",
	})
	createAction := Action{ID: "act_t20_real_1", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t20",
		Version: "confluence/v1", Target: parent, Risk: RiskWrite, Args: createArgs}
	out, err := create.Execute(context.Background(), createAction)
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("real create: state=%s err=%v", out.State, err)
	}
	pageID := out.ExternalID
	rcpt, rerr := ParseConfluencePageReceipt(out.Output)
	if rerr != nil || rcpt.ExternalID != pageID || rcpt.ExternalVersion == "" {
		t.Fatalf("real create receipt: %+v %v", rcpt, rerr)
	}
	t.Logf("REAL page created: id=%s version=%s", pageID, rcpt.ExternalVersion)

	// Update against the SAME page: read its CURRENT version live (the
	// create bumped the parent space's page inventory; never assume).
	live, lerr := ReadConfluencePageVersion(context.Background(), pol, cred, ep.Edition, ep.APIBasePath, pageID)
	if lerr != nil {
		t.Fatalf("real version pre-read: %v", lerr)
	}
	update := &ConfluenceUpdateAdapter{
		Policy: pol, Credential: cred, Edition: ep.Edition, APIBasePath: ep.APIBasePath,
		ConnectionCapabilities: caps,
	}
	updateArgs, _ := json.Marshal(map[string]any{
		"page_id": pageID, "expected_version": live,
		"title": "T20 real " + stamp,
		"storage": "<p>t20 real update " + stamp + "</p>",
	})
	updateAction := Action{ID: "act_t20_real_2", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t20",
		Version: "confluence/v1", Target: pageID, Risk: RiskWrite, Args: updateArgs}
	uout, uerr := update.Execute(context.Background(), updateAction)
	if uerr != nil || uout.State != ActionSucceeded {
		t.Fatalf("real update: state=%s err=%v", uout.State, uerr)
	}
	urcpt, urerr := ParseConfluencePageReceipt(uout.Output)
	if urerr != nil || urcpt.ExternalVersion == live {
		t.Fatalf("real update receipt must carry a NEW version: %+v (had %s)", urcpt, live)
	}
	t.Logf("REAL page updated: id=%s version %s -> %s", pageID, live, urcpt.ExternalVersion)

	// Stale-version update: a conflict must be refused BEFORE any write.
	staleArgs, _ := json.Marshal(map[string]any{
		"page_id": pageID, "expected_version": live, // superseded by the update above
		"title": "T20 stale " + stamp,
		"storage": "<p>t20 stale " + stamp + "</p>",
	})
	staleAction := Action{ID: "act_t20_real_3", TenantID: 7, ActorID: "user_real", ConnectionID: "conn_t20",
		Version: "confluence/v1", Target: pageID, Risk: RiskWrite, Args: staleArgs}
	sout, serr := update.Execute(context.Background(), staleAction)
	if !errors.Is(serr, ErrConfluenceVersionConflict) || sout.State != ActionFailed {
		t.Fatalf("real stale update must conflict: state=%s err=%v", sout.State, serr)
	}
	t.Logf("REAL stale update refused with zero writes (conflict on %s)", live)
}
```

- [ ] **Step 2: 本环境确认 SKIP（skip 不是 pass，如实记录 blocked-env）**

Run: `CONFLUENCE_BASE_URL=x CONFLUENCE_EMAIL=x CONFLUENCE_API_TOKEN=x CONFLUENCE_PARENT_PAGE_ID=xxxx-skip go test ./internal/modules/appconnector/ -run TestConfluenceRealPublishLoop -count=1 -v`
Expected: `--- SKIP: TestConfluenceRealPublishLoop`（与 Task 9 前置说明一致；作者在计划撰写时以同款形态实跑过 Notion 先例 `TestNotionRealControlledCreate` 确认该纪律在 CI 中成立）。有凭据的环境（`artifacts/connector-real/confluence.env` 形态）自动执行完整闭环。

- [ ] **Step 3: 提交**

```bash
git add internal/modules/appconnector/confluence_publish_real_test.go
git commit -m "test(appconnector): gated real confluence publish loop evidence (T20 #50, blocked-env)"
```

---

## 计划级验证命令

在 worktree 根执行（覆盖本计划全部测试域：appconnector 适配器族、publish 桥接与服务、HTTP 门、E2E 全链；`go vet` 覆盖全部新包路径；不触碰全量 flaky 套件）：

```bash
go build ./... && go vet ./internal/modules/appconnector/... ./internal/handler/ ./internal/router/ ./internal/container/ && go test ./internal/modules/appconnector/ -run 'Confluence' -count=1 && go test ./internal/modules/appconnector/publish/ -run 'Confluence' -count=1 && go test ./internal/handler/ -run 'TestConfluencePublish' -count=1
```

## 自我审查记录

**1. Spec 覆盖（逐条验收标准 → 任务）：**
- AC1「旧 Artifact 不静默覆盖外部新版本」：Task 3 `TestConfluenceUpdateConflictZeroWrites`（执行期重读 version.number、不一致确定性 failed、零写请求）+ Task 6 `TestConfluenceExecuteUpdateConflictSettlesFailedReceipt`（回执 settle failed + 409 冲突标记）+ Task 8 `TestConfluencePublishEndToEndUpdateConflict`（409 `PUBLISH_VERSION_CONFLICT` + fake PUT 计数不变）；「不静默」的另一半——更新目标必须由本连接先发布过（`LatestPublishedByDestination` 权威规则）由 Task 6 `TestConfluenceFormPlanUpdateRequiresPriorPublishedReceipt` 覆盖；版本令牌不可伪造由 Task 1 `TestParseConfluencePageVersion`（number<1 拒绝）覆盖。
- AC2「失败和重试行为与统一 Action 语义一致」：失败=4xx/冲突→确定性 failed（Task 3）；不可观察=transport/5xx/回执不可解析→unknown（Task 3 `TestConfluenceUpdateUnknownOnLostWriteReply`）；unknown 结构性拒绝重发（ActionStore.ClaimDispatch 仅从 authorized 起，Task 6 `TestConfluenceReconcileResolvesUnknownWithoutRedispatch` 断言 re-publish 报 `ErrActionState`）；对账只读远端不重发（Task 5 `TestConfluenceBridgeQueryProviderReadsRemoteFirst` + Task 6 + Task 8 E2E 三处 fake 写计数断言）。全部走既有 A03 管线语义，未引入任何平行的重试路径。
- AC3「端到端行为通过最高稳定 Interface 验证」：Task 8（生产迁移 + 真实服务/处理器/审批端点 + 生产 scope source，仅 Confluence 网线为契约双打，注释明示 NOT the real-provider acceptance）；真实 Provider 证据为 Task 9 的 blocked-env opt-in，本地 SKIP 且明确「skip is not a pass」——不冒充。
- 正文「后续修改以外部页面为协作权威」：每次更新绑定批准时读取的外部当前版本（`expected_version`），任何漂移即 409 并要求从新的当前版本重新形成计划——AC1 测试即此语义。
- 调查缺口逐项：写能力不存在→Task 1–3；版本读取与冲突检测→Task 1/3/6；Adapter 接入统一 Action 管道→Task 5（`ActionDispatcher`+`UnknownResolver`）+ Task 7（容器第二实例）；端到端验证→Task 8；移动端 UI→差异记录第 5 条（属 #51，非本 Issue）。

**2. 占位符扫描**：全文无 TBD/TODO/「类似 Task N」/「补充错误处理」。两处曾出现的草稿锚（Task 5 的 `strings` 假引用、Task 6 的 `mustCfArtifactVersion` 草稿）已在成稿中删除并以正确代码单一版本呈现，仅保留指向性的「实现注意」注记（不承载语义）。测试代码全部完整可编译级呈现；唯一开放项是 import 排序类机械差异（已注明以编译器/gofmt 为准）。

**3. 类型/签名一致性**（跨任务核对）：
- `ConfluenceCredential`（Task 1）= Task 2/3 适配器 `Credential` 字段返回类型 = Task 5 `ConfluenceTokenSource.Token` 返回类型 ✓
- `ConfluencePageVersion.VersionNumber string` ↔ `DetectConfluenceVersionConflict(expected, actual string)` ↔ 快照 `ExpectedVersion string` ↔ 回执 `ExternalVersion string` ↔ `PublishReceiptView.ExternalVersion string`（plan.go 既有）——全链同一字符串令牌 ✓
- 快照 JSON 键：create `{parent,title,storage}` / update `{page_id,expected_version,title,storage}` 在 Parse（Task 2/3）、FormPlan marshal（Task 6）、bridge 路由 `IsConfluence*Args`（Task 5）、E2E 断言（Task 8）四处逐字一致 ✓
- `ConfluenceConnectionScope` 字段（Task 5）↔ `dbConfluenceScopeSource` 填充 ↔ `adapterFor` 消费（Edition/APIBasePath/ApprovedParents/WriteCapability/AuthVersion）↔ Task 8 测试断言 ✓
- `NewConfluencePublishService` 参数序（actions, store, pubs, artifacts, content, remote, scopes）在 Task 6 定义、Task 7 容器与 Task 8 E2E 装配两处调用一致 ✓
- Task 8 复用的同包助手 `e2ePolicyProvider`/`e2ePassGuard`/`e2eLocalContent`（notion e2e 已定义）未重定义；`fakeArtifacts`/`fakeContent`/`passGuard`/`openPlanDB`/`openBridgeDB`/`fakePolicies`（publish 包既有测试）未重定义 ✓
- `PublishPlanInput` 复用（字段与 #48 完全一致，`Version` 常量由服务内写死 `confluence/v1`）✓

**4. Review Focus 落实**：五条逐条映射——①TOCTOU→Task 3/6/8 三层（见各条目内测试名）；②版本令牌缺失/伪造→Task 1 parse 拒绝 + Task 6 预读失败→`ErrPublishDestinationUnreadable` + Task 3 预读失败→failed；③盲重试→Task 3 unknown + Task 6 reconcile 不重发 + Task 8 re-publish 409 且 PUT 计数不变；④approve-then-rewrite→Task 2/3 快照精确字段拒绝（extra/missing/empty 各 case）+ 既有 digest 机制（approval 绑定 digest，Plan 变化=新 digest）；⑤越权/错接→Task 7 双门测试（501/viewer 403/owner 403/400/404/跨租户 404）+ Task 5 非 confluence 连接 `ErrDispatchNotStarted` + Task 6 非 confluence 连接 FormPlan 400。无空缺。

**与 #48 计划自审的差异化核对**：本计划 `ConfluencePublishService.project` 的 succeeded 分支保留了 Notion 版同款防御（回执解析失败→跳过 settle，注释同义）；`project` 对 `row.TenantID != tenantID` 统一映射 `ErrActionNotFound`（跨租户 404 不泄漏）——与 Notion 实现逐字同形，无行为漂移。

**遗留如实声明**：(a) Task 9 真实 Confluence 证据在本环境 blocked-env（SKIP），Task 8 为本地最高稳定 Interface 替代证据；(b) Task 0 在健康基线上是纯验证（可能零改动零提交）；(c) Confluence storage 正文采用最小确定性投影（段落→`<p>`，单换行保留段内），富文本（表格/宏/图片）不在本 Issue 范围（与 #48 的「仅 text/plain 可发布」同界，`publishableMIME` 共用）。
