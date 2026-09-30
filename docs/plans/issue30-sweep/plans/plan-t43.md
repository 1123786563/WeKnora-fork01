# T13: 合规访问、Retention 与安全删除（Issue #43）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立管理员合规访问独立流程（默认仅元数据、私有内容需理由+期限+完整审计、绝不写入协作列表）、租户级 Task 保留策略（retention_days + legal_hold 驱动删除闸门与永久删除）与「内部删除不隐式删除外部文档、代码或审计」的安全删除语义，全部行为在真实迁移 sqlite + 真实 store/service/handler 的 HTTP 集成测试中验证。

**Architecture:** Task = Session（ADR-0004），任务内容权威在 `sessions`/`messages`/`agent_runs`。新增两张表（两套迁移序列）：`tenant_task_policies`（租户级保留策略，PK=tenant_id）与 `task_compliance_access`（管理员合规访问窗口——与 #42 的 `task_grants` 完全独立，service 端口层根本不持有 grant 写路径，AC1 是类型级结构性保证）。`TaskComplianceService` 承载合规面（policy 读写、元数据投影、访问窗口、内容读取）与删除闸门（`AllowsTaskDeletion`：legal_hold 拒绝并留痕）；`session.Handler` 以 O03 `craftTombstoner` 同形的 nil-safe setter 挂接闸门，覆盖单删/批删/清空消息三个删除入口。永久删除（purge）是 Admin+ 的独立检查链（legal hold → 已软删 → 保留期已满），事务内显式删除五张内部表且语句集**不含** `audit_logs`、**不触发任何外部调用**（外部资源处置归 craft 墓碑/清扫域，软删除入口已跑）。

**Tech Stack:** Go 1.26（go.mod `github.com/Tencent/WeKnora`）、gin、GORM、golang-migrate（versioned/PostgreSQL + sqlite 两套序列）、SQLite 真库集成测试（httptest + gin + 全量真实迁移，复用 `openCraftHTTPDB`/`openTaskGrantDB` 模式）、复用 #42 的 `TaskGrantService`/`AuditLogService`/`rbacGuards.Admin()`。

**Spec:** `docs/plans/issue30-sweep/issues/issue-43.md`（验收标准原文）；领域术语 `CONTEXT.md`（合规访问/任务保留策略/私有任务/任务所有者）；`docs/specs/2026-09-20-mobile-ai-office-design.md`（用户故事 54-56、Testing Decisions）；`docs/adr/0004-task-is-session.md`。

## Global Constraints

逐字引用批准需求与项目级约束（每个任务的要求都隐含本节）：

- 「合规访问不把管理员加入 Task 协作列表。」（Issue #43 验收标准 1）
- 「删除内部 Task 不隐式删除外部文档、代码或审计。」（Issue #43 验收标准 2）
- 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」（Issue #43 验收标准 3）
- 「管理员普通视图只见元数据；私有内容访问需要理由、期限和审计。归档、法律保留和永久删除遵守 Tenant policy。」（Issue #43 What to build 原文）
- 「合规访问（Compliance Access）：管理员为审计、安全响应或法律义务，在说明理由、限定期限并完整留痕后访问私有任务内容的独立流程；管理员默认只能查看任务元数据、成本、安全事件和外部操作回执。」（CONTEXT.md:206）
- 「任务保留策略（Task Retention Policy）：空间对任务、时间线、产物和操作回执的归档、保留、法律保留与永久删除规则；内部删除会清理服务端内容与设备缓存，但不会隐式删除已经发布到外部系统的内容。」（CONTEXT.md:277）
- 「54. As a compliance administrator, I want metadata available by default and content access gated by a reasoned, time-limited, audited process, so that privacy and compliance coexist.」「55. As a Tenant administrator, I want Task retention and legal hold policies, so that users cannot bypass organizational obligations.」「56. As a Task Owner, I want internal deletion not to delete external documents or code implicitly, so that destructive side effects require their own approval.」（Spec 用户故事 54-56）
- 「Task 沿用 Session 作为唯一身份，并保持 `taskId = sessionId`，不新增第二个 Task 聚合身份。」（ADR-0004）
- 「Security tests cover cross-Tenant and cross-Deployment leakage, forged cursors, revoked connections, stale approvals, shell credential isolation, dependency revocation and compliance access auditing.」「Go integration tests verify authorization, Tenant/Owner/Collaborator predicates, …」（Spec Testing Decisions）
- 安全约束（Mimosa）：所有 SQL 一律参数绑定，不得用拼接/format 组装；本计划不新增出站 HTTP 请求（purge 与合规面均为数据库操作）；不写入任何凭据字面量。
- 测试归属：Task 0 复跑既有迁移装载测试为 GREEN 证据；Task 1-6 各自的仓储/服务/handler 单测钉实现；Task 7 的 wire 级集成测试（真实 sqlite 迁移 + 真实 store/service/handler + httptest）是三条验收标准的最终证据（复用 `internal/handler/session/craft_test.go:115 openCraftHTTPDB` 与 `internal/application/repository/task_collaboration_http_test.go` 两处既有先例模式）。
- 任务结构：严格 RED → GREEN → REFACTOR；每任务以 commit 结束（若执行时由编排层接管提交，则 Commit 步骤改为「确认工作区仅含本任务文件」）。
- 迁移轨道现状与修复：**当前 HEAD 的迁移轨道已损坏**——B3 并行合并（merge d58675a67）使 `migrations/sqlite` 下 000112 同号双文件（`000112_agent_adoption_variants` + `000112_task_grants`）、`migrations/versioned` 下 000191 同样双文件，golang-migrate 装载即报 `duplicate migration file: 000112_task_grants.down.sql`（实测：`go test ./internal/handler/session/ -run TestDeleteSessionTombstonesCraftResources -v` FAIL）。本计划 **Task 0 先修复**：把 #59 的 `agent_adoption_variants` 四个文件顺延为 versioned 000192 / sqlite 000113（重命名不改内容，两组迁移建表互不依赖，任一顺延语义等价；修复后 `TestDeleteSessionTombstonesCraftResources`、`TestTaskGrant*`、`TestWorkbenchNotificationsTableExistsAfterMigrations` 全 PASS——本计划作者已在 worktree 实测验证后还原）。本计划自己的迁移随后使用 **versioned 000193 / sqlite 000114**。B4 其余并行计划若集成时占用该号，整体顺延为下一个可用编号（内容不变），不得挤占他人编号。
- 设备缓存联动清理（CONTEXT.md「内部删除会清理服务端内容与设备缓存」的设备侧半句）属移动端域：#41 设备注册/撤销面已交付撤销语义，#40（T10）持有跨进程加密缓存；本计划交付服务端删除面并在「设备缓存联动」上仅做服务端可达部分（无——服务端不触设备缓存，删除时设备缓存因失去服务端权威数据而失效）。如实列为差异记录第 5 条，不伪造。
- blocked-env 评估：本计划全部验收本地可验证（sqlite + httptest），无真机/外部凭据依赖；「外部文档、代码」指外部系统资源（Notion/GitHub/沙箱），本地以 `craft_workspaces` 记账行 + 「purge 语句集零外部调用」结构性证据替代，如实记录边界，不冒充真实外部系统证据。

## Review Focus

规格隐含、但任务测试未直接覆盖时最可能咬人的五类输入/失败模式（每条标注归属任务的测试）：

1. **管理员借道 run 读面绕过合规窗口**：Admin+ 租户角色直接 `GET /workbench/executions/:run_id` 想不经「理由+期限+审计」读私有内容。合理预期：404——run 读面只认 owner 或 `task_grants`×active member（#42 谓词），Admin+ 租户角色本身不构成 run 级授权；开合规窗口后**仍** 404（合规是独立流程，不提升协作身份）。—— Task 7 e2e `TestComplianceEndToEndAC1IndependentFlow` 的两段 404 断言。
2. **窗口过期后内容读取滞留**：`expires_at` 已过，管理员仍能 `GET .../content`。合理预期：403——窗口校验按 `now` 实时过滤，无任何缓存宽限。—— Task 2 `TestOpenAndActiveComplianceAccessWindow`（`stale` 断言：窗口过期后 `ActiveComplianceAccess` 返回 nil）+ Task 3 `TestReadTaskContentRequiresActiveWindow`。
3. **审计写入失败仍开窗口**（合规面静默降级）：audit Log 返回错误时授权照发、内容照读，「完整留痕」破洞。合理预期：fail closed——审计失败则授权/读取/策略写入全部拒绝。—— Task 3 `TestComplianceAuditFailClosed`（recordingAudit 注入 err，断言三种操作全拒且 stub store 零调用）。
4. **purge 连带删除审计或触碰外部系统**：永久删除语句悄悄带上 `audit_logs`，或 purge 内调用外部删除。合理预期：audit_logs 行一条不少（append-only 跨越一切 purge）；purge 语句集只含五张内部表、零外部调用。—— Task 6 `TestPurgeTaskDeletesInternalRowsOnlyKeepsAudit`（种历史 audit 行，purge 后计数不变 + `task.purged` 新行存在）+ Task 7 e2e AC2 段。
5. **legal hold 绕道**：单删被 409 后改走 `DELETE /sessions/batch`（含 `delete_all=true`）或 `DELETE /sessions/:id/messages`（清空消息销毁证据）绕过法律保留。合理预期：三个删除入口共用同一闸门，全部 409。—— Task 5 `TestSessionDeleteBlockedByLegalHold`（单删）、`TestBatchAndClearBlockedByLegalHold`（batch ids 分支 + delete_all 分支 + clear-messages 分支）。

---

## 与调查结论的差异记录（以代码现状为准）

1. 调查称「安全删除未按 Tenant policy 驱动，现存删除仅会话级」。代码现状：会话删除链已有三层防线——craft 墓碑（`internal/handler/session/handler.go:122 tombstoneCraftSession`，best-effort 阻止新派发、失败交由清扫）+ 运行围栏（`handler.go:579 fenceSessionRuns`）+ 软删除（`internal/application/service/session.go:662 DeleteSession`，且 `session.go:693-700` 显式注释「Skill-generated artifact blobs are intentionally NOT purged here」）。**真实缺口是租户策略层**（legal hold 拒绝 + 保留期 + 永久删除通道），本计划 Task 5/6 落地；墓碑/围栏/软删语义零改动。
2. 调查称「现存仅有审计日志保留（`internal/config/config.go:547-555 audit.retention_days`）」。属实：该配置属审计域清扫（`AuditLogService.Purge`），与 Task 保留策略无关。本计划的 `tenant_task_policies` 是独立的新面；`audit.retention_days` 零改动（审计保留继续由既有配置驱动）。
3. AC2「删除内部 Task 不隐式删除外部文档、代码或审计」部分已有证据（`internal/handler/session/session_delete_tombstone_test.go:78`）。本计划不重复交付墓碑语义，而是补齐 AC2 的另一半证据链：软删除后审计与 craft 记账行完好（e2e 断言）+ 永久删除通道同样不删审计/不触外部（Task 6/7）。
4. 「归档遵守 Tenant policy」的实现解释：归档（`POST /workbench/tasks/:task_id/archive`，#34 交付）是**非破坏性组织动作**，legal hold 不阻止归档（保留证据的组织动作恰好是 hold 期间需要的）；policy 约束的是**删除类动作**（软删除受 legal hold 闸门、purge 受 hold+保留期检查链）。该解释写入 `tenant_task_policies` 迁移注释并在 Task 7 e2e 钉住「归档不受 hold 影响」（真实 #34 archive handler 在 hold 下仍 200）。
5. 移动端（apps/mobile、mobile-core、contracts、api-client）不在本计划：三条验收标准全部是服务端行为门禁与审计语义；设备缓存联动清理的服务端可达部分（设备缓存因服务端数据消亡而失效）无需新代码，移动侧加密缓存生命周期属 #40（T10，B4 并行）、设备撤销属 #41（已交付）。本计划纯 Go 侧，最大化降低与 B4 并行计划的共享写面（#37/#39 动 workbench 命令面，本计划不碰 `workbench_commands.go`；#45 动知识域；#56 动语音域）。
6. SQLite/持久化 TaskProjectionStore 存储选型 ADR 未决（B2-F23 延期项）与本计划无关：本计划不引入任何跨进程持久化缓存，`task_compliance_access`/`tenant_task_policies` 是普通关系表（与 task_grants 同级），不触发该 ADR。
7. **迁移轨道同号损坏（独立审查发现，已实测证实）**：B3 并行合并使 sqlite 000112 与 versioned 000191 各被 `agent_adoption_variants`（#59）与 `task_grants`（#42）双文件占用，golang-migrate 全量装载直接失败——本计划全部依赖全量 sqlite 迁移的测试（Task 1/2/6/7）在此状态下不可运行。本计划 Task 0 顺延 #59 的四个迁移文件（versioned 000191→000192、sqlite 000112→000113，重命名不改内容）修复轨道；两组迁移建表互不依赖，顺延语义等价（作者已实测：重命名后 `TestDeleteSessionTombstonesCraftResources`、`TestTaskGrant*`、`TestWorkbenchNotificationsTableExistsAfterMigrations` 全 PASS）。
8. **AC3 的 sessionService 双打边界（明示）**：Task 7 e2e 的 `sqlBackedSessions` 是手写的最小 session-service 表面（真实 SQL 软删除 + 归属读，但非生产 `sessionService` 的消息清理 goroutine/建议清理/沙箱 teardown 等旁路——这些不在被测删除契约内）。该取舍沿用仓库既有先例（`session_delete_tombstone_test.go:42 deletionSessions`，B2 交付的 W33 AC 证据同款形状）；合规面本身（store/service/handler/legal-hold 闸门/审计）与迁移全真。「mock 不冒充真实集成证据」的判读：被验证的行为（删除闸门、软删除落库、审计只增、purge 语句集）全部走真实代码路径，双打仅替代删除契约之外的旁路副作用。

---

## 文件结构总览

| 文件 | 职责 | 任务 |
|---|---|---|
| `migrations/versioned/000191_agent_adoption_variants.{up,down}.sql` → `000192_agent_adoption_variants.{up,down}.sql` | 修复 versioned 轨道同号双文件（重命名，内容不变） | 0 |
| `migrations/sqlite/000112_agent_adoption_variants.{up,down}.sql` → `000113_agent_adoption_variants.{up,down}.sql` | 修复 sqlite 轨道同号双文件（重命名，内容不变） | 0 |
| `internal/types/task_compliance.go` | TenantTaskPolicy / TaskComplianceAccess / TaskMetadataFacts / TaskMessageFact / TaskMetadataView / TaskContentView / TaskPurgeReceipt | 1 |
| `migrations/versioned/000193_task_compliance.{up,down}.sql` | PostgreSQL/versioned 迁移（两表） | 1 |
| `migrations/sqlite/000114_task_compliance.{up,down}.sql` | sqlite 迁移（两表） | 1 |
| `internal/application/repository/task_compliance_store.go` | TaskComplianceStore：policy 读写 / 访问窗口 / 元数据与内容投影 / PurgeTask | 1,2,6 |
| `internal/application/repository/task_compliance_store_test.go` | store 单测（迁移对齐/往返/过期过滤/purge 语句集） | 1,2,6 |
| `internal/types/audit_log.go` | 追加 5 个 AuditAction 常量（尾部追加） | 3 |
| `internal/application/service/task_compliance.go` | TaskComplianceService：policy/metadata/access/content/guard/purge + 审计 fail-closed | 3,5,6 |
| `internal/application/service/task_compliance_test.go` | service 单测（Admin 谓词/校验/审计 fail-closed/检查链） | 3,5,6 |
| `internal/handler/session/workbench_task_compliance.go` | WorkbenchTaskComplianceHandler（五端点 + Task 6 追加 PurgeTask）+ 错误映射 | 4,6 |
| `internal/handler/session/workbench_task_compliance_test.go` | handler 单测（stub manager + 错误码映射 + Task 6 追加 purge 端点） | 4,6 |
| `internal/handler/session/handler.go` | TaskDeletionGuard 接口 + setter + 三处 nil-safe 前置检查（O03 同形） | 5 |
| `internal/handler/session/task_deletion_guard_test.go` | 闸门 handler 单测（单删/批删两分支/清消息） | 5 |
| `internal/router/routes_workbench.go` | RegisterWorkbenchTaskComplianceRoutes（尾部追加；purge 两行在 Task 6 加） | 4,6 |
| `internal/router/router.go` | RouterParams 加 1 字段 + 注册 1 行 | 4 |
| `internal/container/workbench.go` | NewWorkbenchTaskComplianceHandler（Task 4）/ NewTaskComplianceStore（Task 4）/ NewTaskComplianceService + wireTaskDeletionGuard（Task 5） | 4,5 |
| `internal/container/container.go` | Provide 三行（Task 4，`must(container.Provide(NewWorkbenchTaskGrantsHandler))` 即 container.go:286 行旁）+ Invoke 一行（Task 5，`must(container.Invoke(wireCraftSessionTombstone))` 即 container.go:1048 行旁） | 4,5 |
| `internal/container/task_compliance_wiring_test.go` | 装配登记源级断言（Provide/Invoke 行存在——optional 路由静默缺失防线） | 4,5 |
| `internal/handler/session/task_compliance_e2e_test.go` | AC1/AC2/AC3 端到端证据（真实迁移+真实装配+httptest） | 7 |

共享文件修改均为加法式最小改动（见各任务 Files 的行级定位），B4 并行合并风险低。

---

## Consumes（来自已完成批次，签名以当前 HEAD 实测为准）

- #42 `service.NewTaskGrantService(grants TaskGrantStorePort, sessions interfaces.SessionRepository, members TaskMemberLookupPort) *TaskGrantService`；`types.TaskGrantRole{viewer|collaborator}.IsValid()`；`types.TaskAccessRole{none,viewer,collaborator,owner}`；`types.Caller{TenantID uint64; UserID string; Role TenantRole}` 与 `types.CallerFromContext(ctx) Caller`（`internal/types/caller.go:9`）；`types.TenantRole.Level()`（`internal/types/tenant_member.go:53`）。
- #42 `session.NewWorkbenchTaskGrantsHandler(grants TaskGrantManager)` 与 `taskGrantCaller(c)` 身份检查模式（`internal/handler/session/workbench_task_grants.go:55`）。
- #42 `session.NewWorkbenchReadHandler(runs, snapshots).WithGrantedRuns(runs)`（grant 持有者读面；`internal/container/workbench.go:21-28`）。
- #34 `repository.NewWorkbenchTaskStateStore(db).SetTaskArchived(ctx, tenantID, ownerID, taskID, archived, now)`（归档生命周期，本计划 e2e 断言其不受 legal hold 影响）。
- 既有 `interfaces.AuditLogService.Log(ctx, *types.AuditLog) error`（`internal/types/interfaces/audit_log.go:59`）、`service.NewAuditLogService(repo interfaces.AuditLogRepository)`（`internal/application/service/audit_log.go:33`）、`repository.NewAuditLogRepository(db)`（`internal/application/repository/audit_log.go:22`）、`types.AuditLog` 列集与 `types.AuditOutcome{success,accepted,denied,failed,partial,canceled}`（`internal/types/audit_log.go:199-238`）。
- O03 闸门挂接先例：`session.Handler.craftTombstoner` 字段 + `SetCraftTombstoner` setter + `tombstoneCraftSession` nil-safe 调用（`internal/handler/session/handler.go:107-133`）与 `container.go:1048 must(container.Invoke(wireCraftSessionTombstone))`。
- 既有路由/守卫：`rbacGuards.Admin()`（`internal/router/rbac.go:203`）、`g.apiKeyGroup(group, apiKeyFullAccess())`（`internal/router/routes_workbench.go:150-151` 惯例）。

## Produces（本计划交付，供 #71 证据矩阵及后续消费）

1. `types.TenantTaskPolicy{TenantID uint64; RetentionDays int; LegalHold bool; UpdatedBy string; CreatedAt, UpdatedAt time.Time}`（TableName `tenant_task_policies`）。
2. `types.TaskComplianceAccess{ID uint64; TenantID uint64; TaskID, AdminID, Reason string; ExpiresAt, CreatedAt time.Time}`（TableName `task_compliance_access`）；`MaxComplianceAccessTTL = 7 * 24 * time.Hour`。
3. `repository.NewTaskComplianceStore(db *gorm.DB) *TaskComplianceStore`，方法：`GetTaskPolicy(ctx, tenantID) (*types.TenantTaskPolicy, error)`（未设置返回 `(nil, nil)`）、`UpsertTaskPolicy(ctx, policy) (types.TenantTaskPolicy, error)`、`TaskMetadataFacts(ctx, tenantID, taskID) (*types.TaskMetadataFacts, error)`（miss → `ErrTaskComplianceNotFound`）、`OpenComplianceAccess(ctx, access) (types.TaskComplianceAccess, error)`、`ActiveComplianceAccess(ctx, tenantID, taskID, adminID, now) (*types.TaskComplianceAccess, error)`（无有效窗口返回 `(nil, nil)`）、`ListTaskMessages(ctx, taskID, limit) ([]types.TaskMessageFact, error)`、`PurgeTask(ctx, tenantID, taskID) error`。
4. `service.NewTaskComplianceService(store TaskComplianceStorePort, audit interfaces.AuditLogService) *TaskComplianceService` 与 `service.NewTaskComplianceServiceWithClock(store, audit, now func() time.Time)`（e2e 时间推进用）。方法：`GetTaskPolicy(ctx, caller) (*types.TenantTaskPolicy, error)`、`SetTaskPolicy(ctx, caller, retentionDays int, legalHold bool) (*types.TenantTaskPolicy, error)`、`TaskMetadata(ctx, caller, taskID) (types.TaskMetadataView, error)`、`RequestContentAccess(ctx, caller, taskID, reason string, ttl time.Duration) (types.TaskComplianceAccess, error)`、`ReadTaskContent(ctx, caller, taskID) (types.TaskContentView, error)`、`AllowsTaskDeletion(ctx, tenantID uint64, actorUserID, sessionID string) error`（实现 `session.TaskDeletionGuard`）、`PurgeTask(ctx, caller, taskID) (types.TaskPurgeReceipt, error)`。sentinel：`service.ErrTaskLegalHold` / `service.ErrTaskNotSoftDeleted` / `service.ErrTaskRetentionActive`（均 409 AppError）。
5. `session.NewWorkbenchTaskComplianceHandler(compliance TaskComplianceManager) *WorkbenchTaskComplianceHandler`：`GetTaskPolicy`/`SetTaskPolicy`/`TaskMetadata`/`RequestContentAccess`/`ReadTaskContent`/`PurgeTask`。
6. 路由：`GET|PUT /api/v1/workbench/compliance/task-policy`、`GET /api/v1/workbench/compliance/tasks/:task_id`、`POST /api/v1/workbench/compliance/tasks/:task_id/access`（body `{"reason": string, "ttl_hours": int}`，1 ≤ ttl_hours ≤ 168）、`GET /api/v1/workbench/compliance/tasks/:task_id/content`、`DELETE /api/v1/workbench/tasks/:task_id`（purge，Admin+）。
7. 审计动作常量：`types.AuditActionTaskPolicyUpdated`（`task_policy.updated`）、`types.AuditActionComplianceAccessRequested`（`compliance.access_requested`）、`types.AuditActionComplianceContentRead`（`compliance.content_read`）、`types.AuditActionTaskDeleteDenied`（`task.delete_denied`）、`types.AuditActionTaskPurged`（`task.purged`）。
8. 证据：`internal/handler/session/task_compliance_e2e_test.go` 三个端到端测试（AC1 独立流程 / AC2 不级联 / AC3 真实 Interface 全链）。

---

### Task 0: 修复迁移轨道同号双文件（前置阻塞）

**Files:**
- Rename: `migrations/sqlite/000112_agent_adoption_variants.up.sql` → `migrations/sqlite/000113_agent_adoption_variants.up.sql`
- Rename: `migrations/sqlite/000112_agent_adoption_variants.down.sql` → `migrations/sqlite/000113_agent_adoption_variants.down.sql`
- Rename: `migrations/versioned/000191_agent_adoption_variants.up.sql` → `migrations/versioned/000192_agent_adoption_variants.up.sql`
- Rename: `migrations/versioned/000191_agent_adoption_variants.down.sql` → `migrations/versioned/000192_agent_adoption_variants.down.sql`
- Test: 无新测试文件——以既有迁移装载测试复跑为 GREEN 证据

**Interfaces:**
- Consumes: 无（纯基础设施修复）。
- Produces: 可装载的迁移轨道（Task 1-7 全部全量迁移测试的前置）；空出的 versioned 000192/sqlite 000113 由本重命名占用，本计划新迁移随后使用 versioned 000193 / sqlite 000114。

- [ ] **Step 1: 运行既有迁移装载测试确认失败（RED）**

Run: `cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep && go test ./internal/handler/session/ -run TestDeleteSessionTombstonesCraftResources -v`
Expected: FAIL —— `failed to open source .../migrations/sqlite: duplicate migration file: 000112_task_grants.down.sql`（B3 并行合并使 sqlite 000112 与 versioned 000191 各被两组迁移双文件占用；golang-migrate 按文件名装载，同号即损坏）。

- [ ] **Step 2: 重命名 #59 的四个迁移文件（内容零改动）**

```bash
git mv migrations/sqlite/000112_agent_adoption_variants.up.sql migrations/sqlite/000113_agent_adoption_variants.up.sql
git mv migrations/sqlite/000112_agent_adoption_variants.down.sql migrations/sqlite/000113_agent_adoption_variants.down.sql
git mv migrations/versioned/000191_agent_adoption_variants.up.sql migrations/versioned/000192_agent_adoption_variants.up.sql
git mv migrations/versioned/000191_agent_adoption_variants.down.sql migrations/versioned/000192_agent_adoption_variants.down.sql
```

选择顺延 `agent_adoption_variants`（#59）而非 `task_grants`（#42）的理由：两组迁移建表互不依赖（agent adoption 域 vs task_grants 表），任一顺延语义等价；#42 的 000191/000112 编号已被其 Produces 记录并对下游引用更可见。对已应用过 000112 的环境，golang-migrate 的 schema_migrations 按版本号推进：改名后的 000113 会被视为未应用并执行，行为正确。

- [ ] **Step 3: 复跑迁移装载测试确认通过（GREEN）**

Run: `go test ./internal/handler/session/ -run TestDeleteSessionTombstonesCraftResources -v && go test ./internal/application/repository/ -run 'TestTaskGrant|TestTaskGrantsTableExistsAfterMigrations|TestWorkbenchNotificationsTableExistsAfterMigrations' -v`
Expected: 全部 PASS（本计划作者已在本 worktree 实测此重命名方案：两组命令均 PASS 后还原，执行者按上复现）。

- [ ] **Step 4: Commit**

```bash
git add migrations/sqlite/000112_agent_adoption_variants.up.sql migrations/sqlite/000112_agent_adoption_variants.down.sql migrations/sqlite/000113_agent_adoption_variants.up.sql migrations/sqlite/000113_agent_adoption_variants.down.sql migrations/versioned/000191_agent_adoption_variants.up.sql migrations/versioned/000191_agent_adoption_variants.down.sql migrations/versioned/000192_agent_adoption_variants.up.sql migrations/versioned/000192_agent_adoption_variants.down.sql
git commit -m "fix(migrations): dedupe sqlite 000112 / versioned 000191 collision by bumping agent_adoption_variants (t13 #43 task 0)"
```

---

### Task 1: 域类型、迁移与策略存储（tenant_task_policies）

**Files:**
- Create: `internal/types/task_compliance.go`
- Create: `migrations/versioned/000193_task_compliance.up.sql`、`migrations/versioned/000193_task_compliance.down.sql`
- Create: `migrations/sqlite/000114_task_compliance.up.sql`、`migrations/sqlite/000114_task_compliance.down.sql`
- Create: `internal/application/repository/task_compliance_store.go`
- Test: `internal/application/repository/task_compliance_store_test.go`

**Interfaces:**
- Consumes: `types` 包既有约定（`TableName()` 钉表名，参照 `internal/types/task_grant.go`）；sqlite 迁移 DSN/装载模式（`internal/application/repository/task_grant_store_test.go:27-46`）。
- Produces: `types.TenantTaskPolicy`、`types.TaskComplianceAccess`、`repository.NewTaskComplianceStore` + `GetTaskPolicy/UpsertTaskPolicy`（Task 2/3/4/5/6 消费）；两表迁移（Task 7 e2e 全量迁移自动带上）。

- [ ] **Step 1: 写失败测试（迁移对齐 + policy 往返 + 缺省语义）**

创建 `internal/application/repository/task_compliance_store_test.go`：

```go
package repository_test

// T13 (#43) Task 1: the tenant task-retention policy lane. Migration↔model
// alignment first (production migrations must create both tables), then the
// upsert round-trip and the "never set → (nil, nil)" default that keeps
// every existing tenant ungated until a policy exists.

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openTaskComplianceDB opens a REAL fully-migrated sqlite database (same
// pattern as openTaskGrantDB in task_grant_store_test.go).
func openTaskComplianceDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "task-compliance.db") + "?_foreign_keys=on&_busy_timeout=5000"
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
	require.NoError(t, db.Exec(
		"INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test'), (2, 'tenant-2', 'test')").Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func TestTaskComplianceTablesExistAfterMigrations(t *testing.T) {
	db := openTaskComplianceDB(t)
	require.True(t, db.Migrator().HasTable("tenant_task_policies"),
		"tenant_task_policies must be created by the production migrations")
	require.True(t, db.Migrator().HasTable("task_compliance_access"),
		"task_compliance_access must be created by the production migrations")
	for _, column := range []string{"tenant_id", "retention_days", "legal_hold", "updated_by", "created_at", "updated_at"} {
		require.True(t, db.Migrator().HasColumn("tenant_task_policies", column),
			"tenant_task_policies must carry %s", column)
	}
	for _, column := range []string{"id", "tenant_id", "task_id", "admin_id", "reason", "expires_at", "created_at"} {
		require.True(t, db.Migrator().HasColumn("task_compliance_access", column),
			"task_compliance_access must carry %s", column)
	}
}

func TestTaskPolicyStoreMissingReturnsNil(t *testing.T) {
	store := repository.NewTaskComplianceStore(openTaskComplianceDB(t))
	policy, err := store.GetTaskPolicy(context.Background(), 1)
	require.NoError(t, err)
	require.Nil(t, policy, "a tenant that never set a policy stays ungated (nil, nil)")
}

func TestTaskPolicyStoreUpsertRoundTrip(t *testing.T) {
	store := repository.NewTaskComplianceStore(openTaskComplianceDB(t))

	saved, err := store.UpsertTaskPolicy(context.Background(), types.TenantTaskPolicy{
		TenantID: 1, RetentionDays: 30, LegalHold: true, UpdatedBy: "u9",
	})
	require.NoError(t, err)
	require.Equal(t, 30, saved.RetentionDays)
	require.True(t, saved.LegalHold)

	// Rewrite the same tenant: PK=tenant_id, so the second write updates.
	saved, err = store.UpsertTaskPolicy(context.Background(), types.TenantTaskPolicy{
		TenantID: 1, RetentionDays: 0, LegalHold: false, UpdatedBy: "u9",
	})
	require.NoError(t, err)
	require.Equal(t, 0, saved.RetentionDays)
	require.False(t, saved.LegalHold)

	policy, err := store.GetTaskPolicy(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, policy)
	require.Equal(t, 0, policy.RetentionDays)
	require.False(t, policy.LegalHold)

	// Tenant isolation: tenant 2 never set one.
	other, err := store.GetTaskPolicy(context.Background(), 2)
	require.NoError(t, err)
	require.Nil(t, other)
}
```

注意：`sql.Open` 需要 `"database/sql"` import。文件头 import 块补 `"database/sql"`（上例已含）。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep && go test ./internal/application/repository/ -run 'TestTaskComplianceTablesExistAfterMigrations|TestTaskPolicyStore' -v`
Expected: FAIL —— `types.TenantTaskPolicy` 未定义（编译错误 `undefined: types.TenantTaskPolicy`），先于任何断言。

- [ ] **Step 3: 最小实现（类型 + 迁移 + store 的 policy 部分）**

创建 `internal/types/task_compliance.go`（含 sentinel——定义在 types 而非 repository，service 层判 miss 时不引入 service→repository 反向依赖）：

```go
package types

import (
	"errors"
	"time"
)

// ErrTaskComplianceNotFound marks a compliance-lane task miss (unknown or
// cross-tenant). One uniform miss so probes learn nothing (T12 convention).
// Defined here so both repository and service reference it without a
// service→repository dependency edge.
var ErrTaskComplianceNotFound = errors.New("task compliance record not found")


// TenantTaskPolicy is the tenant-level Task retention policy (T13, #43).
// retention_days bounds how long a soft-deleted task must stay restorable
// before the permanent-delete (purge) lane admits it; legal_hold freezes
// every internal deletion lane (single delete, batch delete, message clear)
// until an authorized administrator lifts it. Archiving is NOT blocked:
// it is a non-destructive organizational action (CONTEXT.md 任务保留策略).
//
// A tenant with no row is ungated — the default keeps today's behavior.
type TenantTaskPolicy struct {
	TenantID      uint64    `json:"tenant_id" gorm:"primaryKey;column:tenant_id"`
	RetentionDays int       `json:"retention_days" gorm:"column:retention_days;not null"`
	LegalHold     bool      `json:"legal_hold" gorm:"column:legal_hold;not null"`
	UpdatedBy     string    `json:"updated_by" gorm:"column:updated_by;type:varchar(512);not null;default:''"`
	CreatedAt     time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// TableName pins the table name (same convention as TaskGrant).
func (TenantTaskPolicy) TableName() string { return "tenant_task_policies" }

// TaskComplianceAccess is one administrator's reasoned, time-limited,
// fully-audited compliance window on one task's private content. It is
// deliberately NOT a task_grants row: compliance access never joins the
// task collaboration list (CONTEXT.md 合规访问 calls it an 独立流程).
type TaskComplianceAccess struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement;column:id"`
	TenantID  uint64    `json:"tenant_id" gorm:"column:tenant_id;not null;index:idx_task_compliance_access_task,priority:1"`
	TaskID    string    `json:"task_id" gorm:"column:task_id;type:varchar(36);not null;index:idx_task_compliance_access_task,priority:2"`
	AdminID   string    `json:"admin_id" gorm:"column:admin_id;type:varchar(512);not null;index:idx_task_compliance_access_admin,priority:2"`
	Reason    string    `json:"reason" gorm:"column:reason;type:text;not null"`
	ExpiresAt time.Time `json:"expires_at" gorm:"column:expires_at;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;not null"`
}

// TableName pins the table name.
func (TaskComplianceAccess) TableName() string { return "task_compliance_access" }

// ExpiredAt reports whether the window is over at the given instant.
func (a TaskComplianceAccess) ExpiredAt(now time.Time) bool { return !a.ExpiresAt.After(now) }

// Covers reports whether the window still authorizes access at now.
func (a TaskComplianceAccess) Covers(now time.Time) bool { return !a.ExpiredAt(now) }
```

创建 `migrations/versioned/000193_task_compliance.up.sql`：

```sql
-- T13 (#43): tenant-level task retention policy + compliance access windows.
-- The policy row is tenant-scoped (PK = tenant_id); a missing row means
-- "ungated" (today's behavior). The access windows are an INDEPENDENT flow
-- from task_grants (#42): a compliance window never joins the collaboration
-- list. Archiving stays unrestricted — only deletion lanes obey the policy.
CREATE TABLE tenant_task_policies (
    tenant_id BIGINT PRIMARY KEY,
    retention_days INTEGER NOT NULL DEFAULT 0 CHECK (retention_days >= 0),
    legal_hold BOOLEAN NOT NULL DEFAULT FALSE,
    updated_by VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE task_compliance_access (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    task_id VARCHAR(36) NOT NULL,
    admin_id VARCHAR(512) NOT NULL,
    reason TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_compliance_access_task ON task_compliance_access (tenant_id, task_id);
CREATE INDEX idx_task_compliance_access_admin ON task_compliance_access (tenant_id, admin_id);
```

创建 `migrations/versioned/000193_task_compliance.down.sql`：

```sql
DROP TABLE IF EXISTS task_compliance_access;
DROP TABLE IF EXISTS tenant_task_policies;
```

创建 `migrations/sqlite/000114_task_compliance.up.sql`：

```sql
-- T13 (#43) — sqlite track. Same shape as the versioned migration; a missing
-- policy row means "ungated"; access windows are independent of task_grants.
CREATE TABLE tenant_task_policies (
    tenant_id INTEGER PRIMARY KEY,
    retention_days INTEGER NOT NULL DEFAULT 0 CHECK (retention_days >= 0),
    legal_hold INTEGER NOT NULL DEFAULT 0,
    updated_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE task_compliance_access (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    task_id TEXT NOT NULL,
    admin_id TEXT NOT NULL,
    reason TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_task_compliance_access_task ON task_compliance_access (tenant_id, task_id);
CREATE INDEX idx_task_compliance_access_admin ON task_compliance_access (tenant_id, admin_id);
```

创建 `migrations/sqlite/000114_task_compliance.down.sql`：

```sql
DROP TABLE IF EXISTS task_compliance_access;
DROP TABLE IF EXISTS tenant_task_policies;
```

创建 `internal/application/repository/task_compliance_store.go`（本任务只含 policy 两方法；Task 2/6 追加；miss sentinel 复用 `types.ErrTaskComplianceNotFound`）：

```go
package repository

// T13 (#43): the persistence lane for tenant task-retention policy,
// compliance access windows, and the compliance metadata/content
// projections. Every statement is parameter-bound.

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

// TaskComplianceStore persists the T13 lanes.
type TaskComplianceStore struct{ db *gorm.DB }

// NewTaskComplianceStore constructs the production store.
func NewTaskComplianceStore(db *gorm.DB) *TaskComplianceStore { return &TaskComplianceStore{db: db} }

// GetTaskPolicy returns the tenant's policy, or (nil, nil) when the tenant
// never set one — the ungated default that keeps every existing tenant's
// deletion flow unchanged.
func (s *TaskComplianceStore) GetTaskPolicy(ctx context.Context, tenantID uint64) (*types.TenantTaskPolicy, error) {
	if s == nil || s.db == nil || tenantID == 0 {
		return nil, errors.New("task compliance store is not assembled")
	}
	var policy types.TenantTaskPolicy
	err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Take(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

// UpsertTaskPolicy writes the tenant policy row (PK = tenant_id). The
// read-then-write transaction keeps one dialect-neutral upsert; the row is
// single-tenant config with no concurrent writers in practice.
func (s *TaskComplianceStore) UpsertTaskPolicy(ctx context.Context, policy types.TenantTaskPolicy) (types.TenantTaskPolicy, error) {
	if s == nil || s.db == nil || policy.TenantID == 0 {
		return types.TenantTaskPolicy{}, errors.New("task compliance store is not assembled")
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing types.TenantTaskPolicy
		err := tx.Where("tenant_id = ?", policy.TenantID).Take(&existing).Error
		now := time.Now().UTC()
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			policy.CreatedAt, policy.UpdatedAt = now, now
			return tx.Create(&policy).Error
		case err != nil:
			return err
		default:
			policy.CreatedAt, policy.UpdatedAt = existing.CreatedAt, now
			return tx.Model(&types.TenantTaskPolicy{}).
				Where("tenant_id = ?", policy.TenantID).
				Updates(map[string]any{
					"retention_days": policy.RetentionDays,
					"legal_hold":     policy.LegalHold,
					"updated_by":     policy.UpdatedBy,
					"updated_at":     now,
				}).Error
		}
	})
	if err != nil {
		return types.TenantTaskPolicy{}, err
	}
	return policy, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/application/repository/ -run 'TestTaskComplianceTablesExistAfterMigrations|TestTaskPolicyStore' -v`
Expected: PASS（3 个测试全绿）。

- [ ] **Step 5: Commit**

```bash
git add internal/types/task_compliance.go migrations/versioned/000193_task_compliance.up.sql migrations/versioned/000193_task_compliance.down.sql migrations/sqlite/000114_task_compliance.up.sql migrations/sqlite/000114_task_compliance.down.sql internal/application/repository/task_compliance_store.go internal/application/repository/task_compliance_store_test.go
git commit -m "feat(compliance): tenant task policy tables + policy store (t13 #43 task 1)"
```

---

### Task 2: 合规访问窗口与元数据/内容投影（store 扩展）

**Files:**
- Modify: `internal/application/repository/task_compliance_store.go`（追加方法）
- Modify: `internal/types/task_compliance.go`（追加 TaskMetadataFacts / TaskMessageFact 投影类型）
- Test: `internal/application/repository/task_compliance_store_test.go`（追加测试）

**Interfaces:**
- Consumes: Task 1 的 `TaskComplianceStore`、`types.TaskComplianceAccess`。
- Produces: `TaskMetadataFacts/TaskMessageFact/ListTaskMessages/OpenComplianceAccess/ActiveComplianceAccess`（Task 3 消费）；`ErrTaskComplianceNotFound`（Task 3/6 消费）。

- [ ] **Step 1: 写失败测试（追加到 task_compliance_store_test.go）**

投影类型（`types.TaskMetadataFacts` 等）与 store 新方法尚不存在——测试引用它们，编译失败即 RED。追加：

```go
func seedComplianceTask(t *testing.T, db *gorm.DB, tenant int, sessionID string) {
	t.Helper()
	require.NoError(t, db.Exec(
		"INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES (?, ?, ?, 'x', ?)",
		"owner-"+sessionID, "owner-"+sessionID, sessionID+"@example.test", tenant).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES (?, ?, ?, ?, 'trpc')",
		sessionID, tenant, "task-"+sessionID, "owner-"+sessionID).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO messages (id, request_id, session_id, role, content) VALUES (?, ?, ?, 'user', 'private question'), (?, ?, ?, 'assistant', 'private answer')",
		"m-"+sessionID+"-1", "req-"+sessionID, sessionID, "m-"+sessionID+"-2", "req-"+sessionID, sessionID).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, engine_type, status, snapshot) VALUES (?, ?, ?, ?, ?, ?, 'h1', 'trpc', 'succeeded', '{}')",
		tenant, "run-"+sessionID, sessionID, "owner-"+sessionID, "req-"+sessionID, "m-"+sessionID+"-2").Error)
}

func TestOpenAndActiveComplianceAccessWindow(t *testing.T) {
	db := openTaskComplianceDB(t)
	seedComplianceTask(t, db, 1, "s1")
	store := repository.NewTaskComplianceStore(db)
	ctx := context.Background()
	now := time.Now().UTC()

	opened, err := store.OpenComplianceAccess(ctx, types.TaskComplianceAccess{
		TenantID: 1, TaskID: "s1", AdminID: "u9",
		Reason: "security incident review", ExpiresAt: now.Add(2 * time.Hour),
	})
	require.NoError(t, err)
	require.NotZero(t, opened.ID)

	// Review Focus 2: the active-window lookup filters by now in real time.
	window, err := store.ActiveComplianceAccess(ctx, 1, "s1", "u9", now.Add(time.Minute))
	require.NoError(t, err)
	require.NotNil(t, window)
	require.Equal(t, opened.ID, window.ID)

	stale, err := store.ActiveComplianceAccess(ctx, 1, "s1", "u9", now.Add(3*time.Hour))
	require.NoError(t, err)
	require.Nil(t, stale, "an expired window must not authorize anything")

	other, err := store.ActiveComplianceAccess(ctx, 1, "s1", "u2", now.Add(time.Minute))
	require.NoError(t, err)
	require.Nil(t, other, "a window is bound to the requesting administrator only")
}

func TestTaskMetadataFactsProjectsMetadataOnly(t *testing.T) {
	db := openTaskComplianceDB(t)
	seedComplianceTask(t, db, 1, "s1")
	seedComplianceTask(t, db, 2, "s2")
	store := repository.NewTaskComplianceStore(db)
	ctx := context.Background()

	facts, err := store.TaskMetadataFacts(ctx, 1, "s1")
	require.NoError(t, err)
	require.Equal(t, "s1", facts.TaskID)
	require.Equal(t, "task-s1", facts.Title)
	require.Equal(t, "owner-s1", facts.OwnerID)
	require.Equal(t, int64(1), facts.RunCount)
	require.Equal(t, "succeeded", facts.LastRunState)
	require.Nil(t, facts.DeletedAt, "a live task has no deleted_at")

	// Cross-tenant probe: one uniform miss (T12 convention).
	_, err = store.TaskMetadataFacts(ctx, 2, "s1")
	require.ErrorIs(t, err, types.ErrTaskComplianceNotFound)

	// A soft-deleted task is still visible to the metadata lane (deleted_at
	// set) — the administrator must be able to see a task WAS deleted.
	require.NoError(t, db.Exec("UPDATE sessions SET deleted_at = ? WHERE id = ?", time.Now().UTC(), "s1").Error)
	facts, err = store.TaskMetadataFacts(ctx, 1, "s1")
	require.NoError(t, err)
	require.NotNil(t, facts.DeletedAt)
}

func TestListTaskMessagesScopedToSession(t *testing.T) {
	db := openTaskComplianceDB(t)
	seedComplianceTask(t, db, 1, "s1")
	store := repository.NewTaskComplianceStore(db)

	messages, err := store.ListTaskMessages(context.Background(), "s1", 10)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	require.Equal(t, "user", messages[0].Role)
	require.Equal(t, "private question", messages[0].Content)
	require.Equal(t, "assistant", messages[1].Role)

	// Another session's rows never leak in.
	none, err := store.ListTaskMessages(context.Background(), "s-unknown", 10)
	require.NoError(t, err)
	require.Empty(t, none)
}
```

（追加的测试使用 `"time"`——文件头 import 块已包含。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/application/repository/ -run 'TestOpenAndActiveComplianceAccessWindow|TestTaskMetadataFacts|TestListTaskMessages' -v`
Expected: FAIL —— `facts.TaskID` 等未定义（`undefined: facts.TaskID` / `store.OpenComplianceAccess` 未定义，编译错误）。

- [ ] **Step 3: 最小实现**

`internal/types/task_compliance.go` 追加：

```go
// TaskMetadataFacts is the metadata-only projection of one task: everything
// a compliance administrator sees by default, BEFORE any reasoned access
// window. It deliberately carries no message content (CONTEXT.md 合规访问).
type TaskMetadataFacts struct {
	TaskID       string     `json:"task_id"`
	Title        string     `json:"title"`
	OwnerID      string     `json:"owner_id"`
	CreatedAt    time.Time  `json:"created_at"`
	ArchivedAt   *time.Time `json:"archived_at,omitempty"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
	RunCount     int64      `json:"run_count"`
	LastRunState string     `json:"last_run_state,omitempty"`
}

// TaskMessageFact is one message row of the private content projection.
type TaskMessageFact struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
```

`internal/application/repository/task_compliance_store.go` 追加（import 补 `"strings"`）：

```go
// OpenComplianceAccess inserts one administrator access window.
func (s *TaskComplianceStore) OpenComplianceAccess(ctx context.Context, access types.TaskComplianceAccess) (types.TaskComplianceAccess, error) {
	if s == nil || s.db == nil {
		return types.TaskComplianceAccess{}, errors.New("task compliance store is not assembled")
	}
	if access.CreatedAt.IsZero() {
		access.CreatedAt = time.Now().UTC()
	}
	if err := s.db.WithContext(ctx).Create(&access).Error; err != nil {
		return types.TaskComplianceAccess{}, err
	}
	return access, nil
}

// ActiveComplianceAccess returns the administrator's newest unexpired window
// on the task at `now`, or (nil, nil) when none covers the instant. The
// real-time filter is the whole enforcement — no grace, no cache.
func (s *TaskComplianceStore) ActiveComplianceAccess(ctx context.Context, tenantID uint64, taskID, adminID string, now time.Time) (*types.TaskComplianceAccess, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("task compliance store is not assembled")
	}
	var window types.TaskComplianceAccess
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND task_id = ? AND admin_id = ? AND expires_at > ?", tenantID, taskID, adminID, now).
		Order("id DESC").Limit(1).Take(&window).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &window, nil
}

// TaskMetadataFacts projects ONE task's metadata (title/owner/state/run
// counts). It includes soft-deleted tasks — the administrator must see that
// a task was deleted — and carries no message content. A miss (unknown or
// cross-tenant task) is one uniform ErrTaskComplianceNotFound.
func (s *TaskComplianceStore) TaskMetadataFacts(ctx context.Context, tenantID uint64, taskID string) (*types.TaskMetadataFacts, error) {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task compliance store is not assembled")
	}
	taskID = strings.TrimSpace(taskID)
	facts := types.TaskMetadataFacts{TaskID: taskID}
	row := struct {
		Title     *string
		UserID    *string
		CreatedAt *time.Time
		ArchivedAt *time.Time
		DeletedAt *time.Time
	}{}
	err := s.db.WithContext(ctx).Table("sessions").
		Select("title, user_id, created_at, archived_at, deleted_at").
		Where("tenant_id = ? AND id = ?", tenantID, taskID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, types.ErrTaskComplianceNotFound
	}
	if err != nil {
		return nil, err
	}
	if row.Title != nil {
		facts.Title = *row.Title
	}
	if row.UserID != nil {
		facts.OwnerID = *row.UserID
	}
	if row.CreatedAt != nil {
		facts.CreatedAt = *row.CreatedAt
	}
	facts.ArchivedAt, facts.DeletedAt = row.ArchivedAt, row.DeletedAt

	if err := s.db.WithContext(ctx).Table("agent_runs").
		Where("tenant_id = ? AND session_id = ?", tenantID, taskID).
		Count(&facts.RunCount).Error; err != nil {
		return nil, err
	}
	var lastStatus *string
	if err := s.db.WithContext(ctx).Table("agent_runs").Select("status").
		Where("tenant_id = ? AND session_id = ?", tenantID, taskID).
		Order("created_at DESC").Limit(1).Scan(&lastStatus).Error; err != nil {
		return nil, err
	}
	if lastStatus != nil {
		facts.LastRunState = *lastStatus
	}
	return &facts, nil
}

// ListTaskMessages reads the task's private conversation rows (the content
// projection). Scoped by session_id; the caller proves the session's tenant
// ownership first (TaskMetadataFacts).
func (s *TaskComplianceStore) ListTaskMessages(ctx context.Context, taskID string, limit int) ([]types.TaskMessageFact, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("task compliance store is not assembled")
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var rows []types.TaskMessageFact
	err := s.db.WithContext(ctx).Table("messages").
		Select("id, role, content, created_at").
		Where("session_id = ?", taskID).
		Order("created_at ASC, id ASC").Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/application/repository/ -run 'TestTaskCompliance|TestTaskPolicyStore|TestOpenAndActive|TestTaskMetadataFacts|TestListTaskMessages' -v`
Expected: PASS（Task 1 + Task 2 全部测试绿）。

- [ ] **Step 5: Commit**

```bash
git add internal/types/task_compliance.go internal/application/repository/task_compliance_store.go internal/application/repository/task_compliance_store_test.go
git commit -m "feat(compliance): access windows + metadata/content projections (t13 #43 task 2)"
```

---

### Task 3: TaskComplianceService（合规访问流程 + 审计 fail-closed + Admin 谓词）

**Files:**
- Create: `internal/application/service/task_compliance.go`
- Modify: `internal/types/audit_log.go`（AuditAction 常量块尾部追加 5 个常量——共享文件最小改动）
- Test: `internal/application/service/task_compliance_test.go`

**Interfaces:**
- Consumes: Task 1/2 的 store 方法（以端口 `TaskComplianceStorePort` 注入）；`interfaces.AuditLogService.Log`；`types.TenantRole.Level()`。
- Produces: `NewTaskComplianceService` / `NewTaskComplianceServiceWithClock`、五方法（policy 读写 / metadata / RequestContentAccess / ReadTaskContent）、`MaxComplianceAccessTTL`、5 个 AuditAction 常量（Task 4/5/6/7 消费）。本任务**不实现** `AllowsTaskDeletion`（Task 5 追加）与 `PurgeTask`（Task 6 追加）——因此 Task 4 的 handler 接口只含五方法，`*service.TaskComplianceService` 在本任务后即满足它。

- [ ] **Step 1: 写失败测试**

创建 `internal/application/service/task_compliance_test.go`：

```go
package service

// T13 (#43) Task 3: the compliance access flow itself. The admin predicate
// (TenantRole admin+), the reason/TTL validation, the audit-first fail-closed
// rule, and the structural fact that a compliance window NEVER writes a
// task_grants row (the service holds no grant port at all — AC1 is
// type-level).

import (
	"context"
	"net/http"
	"testing"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type stubComplianceStore struct {
	policy    *types.TenantTaskPolicy
	facts     *types.TaskMetadataFacts
	factsErr  error
	opened    []types.TaskComplianceAccess
	window    *types.TaskComplianceAccess
	messages  []types.TaskMessageFact
}

func (s *stubComplianceStore) GetTaskPolicy(context.Context, uint64) (*types.TenantTaskPolicy, error) {
	return s.policy, nil
}

func (s *stubComplianceStore) UpsertTaskPolicy(_ context.Context, policy types.TenantTaskPolicy) (types.TenantTaskPolicy, error) {
	s.policy = &policy
	return policy, nil
}

func (s *stubComplianceStore) TaskMetadataFacts(context.Context, uint64, string) (*types.TaskMetadataFacts, error) {
	return s.facts, s.factsErr
}

func (s *stubComplianceStore) OpenComplianceAccess(_ context.Context, access types.TaskComplianceAccess) (types.TaskComplianceAccess, error) {
	access.ID = uint64(len(s.opened) + 1)
	s.opened = append(s.opened, access)
	return access, nil
}

func (s *stubComplianceStore) ActiveComplianceAccess(context.Context, uint64, string, string, time.Time) (*types.TaskComplianceAccess, error) {
	return s.window, nil
}

func (s *stubComplianceStore) ListTaskMessages(context.Context, string, int) ([]types.TaskMessageFact, error) {
	return s.messages, nil
}

func (s *stubComplianceStore) PurgeTask(context.Context, uint64, string) error { return nil }

type recordingAudit struct {
	interfaces.AuditLogService
	entries []types.AuditLog
	err     error
}

func (r *recordingAudit) Log(_ context.Context, entry *types.AuditLog) error {
	if r.err != nil {
		return r.err
	}
	r.entries = append(r.entries, *entry)
	return nil
}

func complianceFixtures() (*stubComplianceStore, *recordingAudit, *TaskComplianceService) {
	store := &stubComplianceStore{
		facts: &types.TaskMetadataFacts{TaskID: "s1", Title: "task-s1", OwnerID: "u1", RunCount: 1, LastRunState: "succeeded"},
		window: &types.TaskComplianceAccess{ID: 7, TenantID: 1, TaskID: "s1", AdminID: "u9",
			Reason: "security incident review", ExpiresAt: time.Now().UTC().Add(time.Hour), CreatedAt: time.Now().UTC()},
		messages: []types.TaskMessageFact{{ID: "m1", Role: "user", Content: "private question"}},
	}
	audit := &recordingAudit{}
	return store, audit, NewTaskComplianceService(store, audit)
}

func adminCaller() types.Caller {
	return types.Caller{TenantID: 1, UserID: "u9", Role: types.TenantRoleAdmin}
}

func TestSetTaskPolicyRequiresAdminAndValidates(t *testing.T) {
	_, audit, svc := complianceFixtures()
	ctx := context.Background()

	_, err := svc.SetTaskPolicy(ctx, types.Caller{TenantID: 1, UserID: "u1", Role: types.TenantRoleContributor}, 30, false)
	require.Error(t, err, "non-admin cannot touch the tenant policy")
	appErr, ok := apperrors.IsAppError(err)
	require.True(t, ok)
	require.Equal(t, http.StatusForbidden, appErr.HTTPCode)

	policy, err := svc.SetTaskPolicy(ctx, adminCaller(), -1, false)
	require.Error(t, err, "negative retention days are refused")
	require.Nil(t, policy)

	policy, err = svc.SetTaskPolicy(ctx, adminCaller(), 30, true)
	require.NoError(t, err)
	require.Equal(t, 30, policy.RetentionDays)
	require.True(t, policy.LegalHold)
	require.Len(t, audit.entries, 1, "every policy write leaves an audit row")
	require.Equal(t, types.AuditActionTaskPolicyUpdated, audit.entries[0].Action)
	require.Equal(t, string(types.TenantRoleAdmin), audit.entries[0].ActorRole)
}

func TestRequestContentAccessValidatesReasonAndTTL(t *testing.T) {
	store, audit, svc := complianceFixtures()
	ctx := context.Background()

	_, err := svc.RequestContentAccess(ctx, adminCaller(), "s1", "   ", 2*time.Hour)
	require.Error(t, err, "a blank reason is refused")

	_, err = svc.RequestContentAccess(ctx, adminCaller(), "s1", "audit", 0)
	require.Error(t, err, "zero TTL is refused")

	_, err = svc.RequestContentAccess(ctx, adminCaller(), "s1", "audit", MaxComplianceAccessTTL+time.Hour)
	require.Error(t, err, "TTL above the 7-day ceiling is refused")

	access, err := svc.RequestContentAccess(ctx, adminCaller(), "s1", "audit trail", 2*time.Hour)
	require.NoError(t, err)
	require.Equal(t, "audit trail", access.Reason)
	require.Len(t, store.opened, 1, "one window row")
	require.Len(t, audit.entries, 1, "the window is fully audited (reason + horizon)")
	require.Equal(t, types.AuditActionComplianceAccessRequested, audit.entries[0].Action)
	require.Equal(t, types.AuditOutcomeSuccess, audit.entries[0].Outcome)
	require.Equal(t, "s1", audit.entries[0].TargetID)

	// A task miss is one uniform 404 — unknown and cross-tenant probes are
	// indistinguishable (T12 convention).
	store.factsErr = types.ErrTaskComplianceNotFound
	_, err = svc.RequestContentAccess(ctx, adminCaller(), "s-missing", "audit", time.Hour)
	require.Error(t, err)
	appErr, _ := apperrors.IsAppError(err)
	require.Equal(t, http.StatusNotFound, appErr.HTTPCode)
}

func TestReadTaskContentRequiresActiveWindow(t *testing.T) {
	store, audit, svc := complianceFixtures()
	ctx := context.Background()

	content, err := svc.ReadTaskContent(ctx, adminCaller(), "s1")
	require.NoError(t, err)
	require.Equal(t, "s1", content.TaskID)
	require.Len(t, content.Messages, 1)
	require.Equal(t, "private question", content.Messages[0].Content)
	require.Len(t, audit.entries, 1)
	require.Equal(t, types.AuditActionComplianceContentRead, audit.entries[0].Action)

	// Review Focus 2: an expired (absent) window refuses the read, 403.
	store.window = nil
	_, err = svc.ReadTaskContent(ctx, adminCaller(), "s1")
	require.Error(t, err)
	appErr, _ := apperrors.IsAppError(err)
	require.Equal(t, http.StatusForbidden, appErr.HTTPCode)
}

func TestComplianceAuditFailClosed(t *testing.T) {
	// Review Focus 3: when the audit trail write fails, every compliance
	// operation refuses instead of silently proceeding without a record.
	store := &stubComplianceStore{
		facts: &types.TaskMetadataFacts{TaskID: "s1", OwnerID: "u1"},
		window: &types.TaskComplianceAccess{ID: 7, TenantID: 1, TaskID: "s1", AdminID: "u9", ExpiresAt: time.Now().Add(time.Hour)},
	}
	audit := &recordingAudit{err: context.DeadlineExceeded}
	svc := NewTaskComplianceService(store, audit)
	ctx := context.Background()

	_, err := svc.SetTaskPolicy(ctx, adminCaller(), 30, true)
	require.Error(t, err, "policy write fails closed without its audit row")

	_, err = svc.RequestContentAccess(ctx, adminCaller(), "s1", "audit", time.Hour)
	require.Error(t, err, "no window opens without its audit row")
	require.Empty(t, store.opened, "fail closed means the store was never called")

	_, err = svc.ReadTaskContent(ctx, adminCaller(), "s1")
	require.Error(t, err, "content stays sealed without its audit row")

	// A nil audit service is a mis-assembly: fail closed with 503.
	svcNoAudit := NewTaskComplianceService(store, nil)
	_, err = svcNoAudit.RequestContentAccess(ctx, adminCaller(), "s1", "audit", time.Hour)
	require.Error(t, err)
	appErr, _ := apperrors.IsAppError(err)
	require.Equal(t, http.StatusServiceUnavailable, appErr.HTTPCode)
}

func TestTaskMetadataIsAdminOnlyAndCarriesNoContent(t *testing.T) {
	_, _, svc := complianceFixtures()
	ctx := context.Background()

	_, err := svc.TaskMetadata(ctx, types.Caller{TenantID: 1, UserID: "u1", Role: types.TenantRoleViewer}, "s1")
	require.Error(t, err, "the metadata lane is still admin-only")

	view, err := svc.TaskMetadata(ctx, adminCaller(), "s1")
	require.NoError(t, err)
	require.Equal(t, "task-s1", view.Metadata.Title)
	require.Equal(t, int64(1), view.Metadata.RunCount)

	// Structural (compile-time): the metadata view type has no content field —
	// this line fails to compile if anyone adds one without a spec change.
	var noContent interface{ metadataOnly() } = view
	_ = noContent
}
```

（`metadataOnly()` 是 `types.TaskMetadataView` 的标记方法，见 Step 3——编译期卫兵：给元数据视图加内容字段必须是显式的类型变更。文件头 import 块含 `"net/http"`，`appErr.HTTPCode` 断言需要它。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/application/service/ -run 'TestSetTaskPolicyRequiresAdmin|TestRequestContentAccessValidates|TestReadTaskContentRequires|TestComplianceAuditFailClosed|TestTaskMetadataIsAdminOnly' -v`
Expected: FAIL —— `undefined: NewTaskComplianceService`（编译错误）。

- [ ] **Step 3: 最小实现**

`internal/types/audit_log.go` 在既有 FAQ 常量块（`audit_log.go:191-193`）之后追加：

```go
// T13 (#43) compliance & retention actions. compliance.access_requested /
// compliance.content_read are the "reasoned, time-limited, audited" trail;
// task_policy.updated records who changed the retention/hold switches;
// task.delete_denied is the legal-hold refusal trail; task.purged is the
// permanent-deletion authorization trail.
const (
	AuditActionTaskPolicyUpdated         AuditAction = "task_policy.updated"
	AuditActionComplianceAccessRequested AuditAction = "compliance.access_requested"
	AuditActionComplianceContentRead     AuditAction = "compliance.content_read"
	AuditActionTaskDeleteDenied          AuditAction = "task.delete_denied"
	AuditActionTaskPurged                AuditAction = "task.purged"
)
```

`internal/types/task_compliance.go` 追加视图类型：

```go
// TaskMetadataView is the administrator's default, metadata-only view of one
// task, joined with the tenant policy. The marker method is a compile-time
// guard: adding a content field to this view must be a visible, deliberate
// change (the compliance flow's whole point is that metadata needs no
// reason and content does).
type TaskMetadataView struct {
	Metadata TaskMetadataFacts `json:"metadata"`
	Policy   *TenantTaskPolicy `json:"policy"`
}

// metadataOnly marks the view as content-free (compile-time guard only).
func (TaskMetadataView) metadataOnly() {}

// TaskContentView is the windowed private-content projection.
type TaskContentView struct {
	TaskID   string               `json:"task_id"`
	Window   TaskComplianceAccess `json:"window"`
	Messages []TaskMessageFact    `json:"messages"`
}
```

（`view.Title` 改为 `view.Metadata.Title`、`view.RunCount` 改为 `view.Metadata.RunCount`——同步修正测试断言。）

创建 `internal/application/service/task_compliance.go`：

```go
package service

// Task compliance & retention (T13, #43).
//
// Compliance access is an INDEPENDENT flow (CONTEXT.md 合规访问): an
// administrator sees task METADATA by default; private content requires a
// reasoned, time-limited window that is fully audited BEFORE anything opens
// or reads. The service deliberately holds NO TaskGrantStorePort — a
// compliance window can never join the task collaboration list (AC1 is
// enforced at the type level).

import (
	"context"
	"errors"
	"strings"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// MaxComplianceAccessTTL bounds one compliance window's lifetime (Spec story
// 54 "time-limited"; the concrete ceiling is a plan-level decision).
const MaxComplianceAccessTTL = 7 * 24 * time.Hour

// TaskComplianceStorePort is the persistence seam for the T13 lanes
// (implemented by repository.TaskComplianceStore).
type TaskComplianceStorePort interface {
	GetTaskPolicy(ctx context.Context, tenantID uint64) (*types.TenantTaskPolicy, error)
	UpsertTaskPolicy(ctx context.Context, policy types.TenantTaskPolicy) (types.TenantTaskPolicy, error)
	TaskMetadataFacts(ctx context.Context, tenantID uint64, taskID string) (*types.TaskMetadataFacts, error)
	OpenComplianceAccess(ctx context.Context, access types.TaskComplianceAccess) (types.TaskComplianceAccess, error)
	ActiveComplianceAccess(ctx context.Context, tenantID uint64, taskID, adminID string, now time.Time) (*types.TaskComplianceAccess, error)
	ListTaskMessages(ctx context.Context, taskID string, limit int) ([]types.TaskMessageFact, error)
	PurgeTask(ctx context.Context, tenantID uint64, taskID string) error
}

// TaskComplianceService carries the compliance surface and the deletion
// gates. audit may be nil only in mis-assembled deployments — every audited
// operation then fails closed with 503 instead of proceeding unrecorded.
type TaskComplianceService struct {
	store TaskComplianceStorePort
	audit interfaces.AuditLogService
	now   func() time.Time
}

// NewTaskComplianceService constructs the production service.
func NewTaskComplianceService(store TaskComplianceStorePort, audit interfaces.AuditLogService) *TaskComplianceService {
	return NewTaskComplianceServiceWithClock(store, audit, time.Now)
}

// NewTaskComplianceServiceWithClock is the test/e2e constructor with an
// injectable clock (retention-horizon time travel).
func NewTaskComplianceServiceWithClock(store TaskComplianceStorePort, audit interfaces.AuditLogService, now func() time.Time) *TaskComplianceService {
	return &TaskComplianceService{store: store, audit: audit, now: now}
}

// requireAdmin enforces the compliance lane's role floor: TenantRole admin
// or above (admin and owner both pass; contributors and viewers do not).
func (s *TaskComplianceService) requireAdmin(caller types.Caller) error {
	if caller.TenantID == 0 || strings.TrimSpace(caller.UserID) == "" {
		return apperrors.NewForbiddenError("authenticated tenant identity is required")
	}
	if caller.Role.Level() < types.TenantRoleAdmin.Level() {
		return apperrors.NewForbiddenError("compliance access requires the tenant admin role")
	}
	return nil
}

// audited writes one audit entry or fails closed.
func (s *TaskComplianceService) audited(ctx context.Context, entry *types.AuditLog) error {
	if s.audit == nil {
		return apperrors.NewServiceUnavailableError("compliance audit trail is required")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = s.now().UTC()
	}
	return s.audit.Log(ctx, entry)
}

// GetTaskPolicy reads the tenant's policy (admin-only).
func (s *TaskComplianceService) GetTaskPolicy(ctx context.Context, caller types.Caller) (*types.TenantTaskPolicy, error) {
	if err := s.requireAdmin(caller); err != nil {
		return nil, err
	}
	return s.store.GetTaskPolicy(ctx, caller.TenantID)
}

// SetTaskPolicy rewrites the tenant's retention policy (admin-only,
// negative retention refused, audited before the write).
func (s *TaskComplianceService) SetTaskPolicy(ctx context.Context, caller types.Caller, retentionDays int, legalHold bool) (*types.TenantTaskPolicy, error) {
	if err := s.requireAdmin(caller); err != nil {
		return nil, err
	}
	if retentionDays < 0 {
		return nil, apperrors.NewBadRequestError("retention_days must be >= 0")
	}
	policyDetails, err := json.Marshal(map[string]any{"retention_days": retentionDays, "legal_hold": legalHold})
	if err != nil {
		return nil, err
	}
	if err := s.audited(ctx, &types.AuditLog{
		TenantID: caller.TenantID, ActorUserID: caller.UserID, ActorRole: string(caller.Role),
		Action:   types.AuditActionTaskPolicyUpdated,
		TargetType: "tenant", TargetID: "task-policy",
		Outcome: types.AuditOutcomeSuccess,
		Details: types.JSON(policyDetails),
	}); err != nil {
		return nil, err
	}
	return s.store.UpsertTaskPolicy(ctx, types.TenantTaskPolicy{
		TenantID: caller.TenantID, RetentionDays: retentionDays, LegalHold: legalHold, UpdatedBy: caller.UserID,
	})
}

// TaskMetadata is the administrator's default view: metadata + policy, no
// content (the view type carries no content field at all).
func (s *TaskComplianceService) TaskMetadata(ctx context.Context, caller types.Caller, taskID string) (types.TaskMetadataView, error) {
	if err := s.requireAdmin(caller); err != nil {
		return types.TaskMetadataView{}, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return types.TaskMetadataView{}, apperrors.NewBadRequestError("task id is required")
	}
	facts, err := s.store.TaskMetadataFacts(ctx, caller.TenantID, taskID)
	if err != nil {
		return types.TaskMetadataView{}, normalizeComplianceMiss(err)
	}
	policy, err := s.store.GetTaskPolicy(ctx, caller.TenantID)
	if err != nil {
		return types.TaskMetadataView{}, err
	}
	return types.TaskMetadataView{Metadata: *facts, Policy: policy}, nil
}

// RequestContentAccess opens one time-limited window after the audit row is
// durably written (reason + horizon in the entry). An audit failure refuses
// the window — fail closed.
func (s *TaskComplianceService) RequestContentAccess(ctx context.Context, caller types.Caller, taskID, reason string, ttl time.Duration) (types.TaskComplianceAccess, error) {
	if err := s.requireAdmin(caller); err != nil {
		return types.TaskComplianceAccess{}, err
	}
	taskID = strings.TrimSpace(taskID)
	reason = strings.TrimSpace(reason)
	if taskID == "" {
		return types.TaskComplianceAccess{}, apperrors.NewBadRequestError("task id is required")
	}
	if reason == "" {
		return types.TaskComplianceAccess{}, apperrors.NewBadRequestError("an access reason is required")
	}
	if ttl <= 0 || ttl > MaxComplianceAccessTTL {
		return types.TaskComplianceAccess{}, apperrors.NewBadRequestError("ttl must be between 1 hour and 168 hours")
	}
	facts, err := s.store.TaskMetadataFacts(ctx, caller.TenantID, taskID)
	if err != nil {
		return types.TaskComplianceAccess{}, normalizeComplianceMiss(err)
	}
	now := s.now().UTC()
	access := types.TaskComplianceAccess{
		TenantID: caller.TenantID, TaskID: facts.TaskID, AdminID: caller.UserID,
		Reason: reason, ExpiresAt: now.Add(ttl), CreatedAt: now,
	}
	details, _ := json.Marshal(map[string]any{"reason": reason, "expires_at": access.ExpiresAt})
	if err := s.audited(ctx, &types.AuditLog{
		TenantID: caller.TenantID, ActorUserID: caller.UserID, ActorRole: string(caller.Role),
		Action: types.AuditActionComplianceAccessRequested,
		TargetType: "task", TargetID: facts.TaskID, TargetUserID: facts.OwnerID,
		Outcome: types.AuditOutcomeSuccess, Details: types.JSON(details),
	}); err != nil {
		return types.TaskComplianceAccess{}, err
	}
	return s.store.OpenComplianceAccess(ctx, access)
}

// ReadTaskContent returns the private content projection only under the
// caller's own unexpired window, and only after its read is audited.
func (s *TaskComplianceService) ReadTaskContent(ctx context.Context, caller types.Caller, taskID string) (types.TaskContentView, error) {
	if err := s.requireAdmin(caller); err != nil {
		return types.TaskContentView{}, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return types.TaskContentView{}, apperrors.NewBadRequestError("task id is required")
	}
	facts, err := s.store.TaskMetadataFacts(ctx, caller.TenantID, taskID)
	if err != nil {
		return types.TaskContentView{}, normalizeComplianceMiss(err)
	}
	window, err := s.store.ActiveComplianceAccess(ctx, caller.TenantID, facts.TaskID, caller.UserID, s.now().UTC())
	if err != nil {
		return types.TaskContentView{}, err
	}
	if window == nil {
		return types.TaskContentView{}, apperrors.NewForbiddenError("an active compliance access window is required")
	}
	readDetails, err := json.Marshal(map[string]any{"window_id": window.ID})
	if err != nil {
		return types.TaskContentView{}, err
	}
	if err := s.audited(ctx, &types.AuditLog{
		TenantID: caller.TenantID, ActorUserID: caller.UserID, ActorRole: string(caller.Role),
		Action: types.AuditActionComplianceContentRead,
		TargetType: "task", TargetID: facts.TaskID, TargetUserID: facts.OwnerID,
		Outcome: types.AuditOutcomeSuccess,
		Details: types.JSON(readDetails),
	}); err != nil {
		return types.TaskContentView{}, err
	}
	messages, err := s.store.ListTaskMessages(ctx, facts.TaskID, 200)
	if err != nil {
		return types.TaskContentView{}, err
	}
	return types.TaskContentView{TaskID: facts.TaskID, Window: *window, Messages: messages}, nil
}

// normalizeComplianceMiss maps the store's uniform miss to a uniform 404 and
// passes everything else through untouched (a 5xx must never masquerade as
// a miss — B3-F82 convention).
func normalizeComplianceMiss(err error) error {
	if errors.Is(err, types.ErrTaskComplianceNotFound) {
		return apperrors.NewNotFoundError("task not found")
	}
	return err
}
```

（sentinel `ErrTaskComplianceNotFound` 定义在 `internal/types/task_compliance.go`（Task 1 已建），repository 与 service 共同引用，无 service→repository 反向依赖。service 文件 import 块：）

```go
import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)
```

`normalizeComplianceMiss` 判 `types.ErrTaskComplianceNotFound`。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/application/service/ -run 'TestSetTaskPolicyRequiresAdmin|TestRequestContentAccessValidates|TestReadTaskContentRequires|TestComplianceAuditFailClosed|TestTaskMetadataIsAdminOnly' -v && go test ./internal/application/repository/ -run 'TestTaskCompliance|TestTaskPolicyStore|TestOpenAndActive|TestTaskMetadataFacts|TestListTaskMessages' -v`
Expected: PASS（service 5 测试 + repository 回归全绿）。

- [ ] **Step 5: Commit**

```bash
git add internal/application/service/task_compliance.go internal/application/service/task_compliance_test.go internal/types/task_compliance.go internal/types/audit_log.go internal/application/repository/task_compliance_store.go internal/application/repository/task_compliance_store_test.go
git commit -m "feat(compliance): reasoned+time-limited+audited access flow, fail-closed audit (t13 #43 task 3)"
```

---

### Task 4: HTTP handler、路由注册与容器接线（五方法面；purge 端点由 Task 6 追加）

**Files:**
- Create: `internal/handler/session/workbench_task_compliance.go`
- Modify: `internal/router/routes_workbench.go:241` 之后追加注册函数（尾部追加）
- Modify: `internal/router/router.go:69` 旁加 RouterParams 字段、`router.go:376` 旁加注册调用
- Modify: `internal/container/workbench.go:149` 之后追加三个 provider（尾部追加）
- Modify: `internal/container/container.go:286`（`must(container.Provide(NewWorkbenchTaskGrantsHandler))` 行后）追加三行 Provide
- Test: `internal/handler/session/workbench_task_compliance_test.go`、`internal/container/task_compliance_wiring_test.go`

**Interfaces:**
- Consumes: Task 3 的 `TaskComplianceService` 五方法（GetTaskPolicy/SetTaskPolicy/TaskMetadata/RequestContentAccess/ReadTaskContent——PurgeTask 尚不存在，由 Task 6 追加后再扩接口）；`taskGrantCaller` 同形身份检查（`workbench_task_grants.go:55`）；`g.Admin()`/`g.apiKeyGroup` 惯例；dig Provide 登记先例（`container.go:275/286`）。
- Produces: 五端点 handler 与路由（Produces 列表第 5/6 条的 purge 段除外）；`TaskComplianceManager` 五方法接口（Task 6 扩为六方法）；容器 Provide 三行（`*repository.TaskComplianceStore`/`*service.TaskComplianceService`/`*session.WorkbenchTaskComplianceHandler`——Task 5 的 Invoke 依赖 service provider 已登记）；`TaskComplianceWiring` 源级断言测试。

- [ ] **Step 1: 写失败测试**

创建 `internal/handler/session/workbench_task_compliance_test.go`：

```go
package session

// T13 (#43) Task 4: the compliance HTTP surface. Identity always comes from
// the authenticated context (taskGrantCaller shape); every service error
// maps onto the workbench envelope (400/403/404/409/500).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeComplianceManager struct {
	policy    *types.TenantTaskPolicy
	policyErr error
	facts     *types.TaskMetadataFacts
	setCalls  int
	access    types.TaskComplianceAccess
	accessErr error
	content   types.TaskContentView
	contentErr error
	lastAccess struct {
		taskID string
		reason string
		ttl    time.Duration
	}
	lastContentTask string
}

func (f *fakeComplianceManager) GetTaskPolicy(context.Context, types.Caller) (*types.TenantTaskPolicy, error) {
	return f.policy, f.policyErr
}

func (f *fakeComplianceManager) SetTaskPolicy(_ context.Context, _ types.Caller, retentionDays int, legalHold bool) (*types.TenantTaskPolicy, error) {
	f.setCalls++
	if f.policyErr != nil {
		return nil, f.policyErr
	}
	return &types.TenantTaskPolicy{TenantID: 1, RetentionDays: retentionDays, LegalHold: legalHold}, nil
}

func (f *fakeComplianceManager) TaskMetadata(_ context.Context, _ types.Caller, taskID string) (types.TaskMetadataView, error) {
	if f.policyErr != nil {
		return types.TaskMetadataView{}, f.policyErr
	}
	return types.TaskMetadataView{Metadata: *f.facts}, nil
}

func (f *fakeComplianceManager) RequestContentAccess(_ context.Context, _ types.Caller, taskID, reason string, ttl time.Duration) (types.TaskComplianceAccess, error) {
	f.lastAccess.taskID, f.lastAccess.reason, f.lastAccess.ttl = taskID, reason, ttl
	return f.access, f.accessErr
}

func (f *fakeComplianceManager) ReadTaskContent(_ context.Context, _ types.Caller, taskID string) (types.TaskContentView, error) {
	f.lastContentTask = taskID
	return f.content, f.contentErr
}

func complianceRequest(t *testing.T, method, path, body string, role types.TenantRole) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u9")
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
	return req.WithContext(ctx), httptest.NewRecorder()
}

func newComplianceTestEngine(t *testing.T, fake *fakeComplianceManager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewWorkbenchTaskComplianceHandler(fake)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.GET("/workbench/compliance/task-policy", h.GetTaskPolicy)
	v1.PUT("/workbench/compliance/task-policy", h.SetTaskPolicy)
	v1.GET("/workbench/compliance/tasks/:task_id", h.TaskMetadata)
	v1.POST("/workbench/compliance/tasks/:task_id/access", h.RequestContentAccess)
	v1.GET("/workbench/compliance/tasks/:task_id/content", h.ReadTaskContent)
	return r
}

func TestCompliancePolicyEndpoints(t *testing.T) {
	fake := &fakeComplianceManager{}
	engine := newComplianceTestEngine(t, fake)

	req, w := complianceRequest(t, http.MethodPut, "/api/v1/workbench/compliance/task-policy",
		`{"retention_days":30,"legal_hold":true}`, types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, fake.setCalls)

	// A negative retention is refused before the service (defense in depth:
	// the service repeats the check).
	req, w = complianceRequest(t, http.MethodPut, "/api/v1/workbench/compliance/task-policy",
		`{"retention_days":-5,"legal_hold":false}`, types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)

	req, w = complianceRequest(t, http.MethodGet, "/api/v1/workbench/compliance/task-policy", "", types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// A viewer never reaches the service (route-level role is Admin+ in
	// production; the handler repeats the predicate for unguarded mounts).
	req, w = complianceRequest(t, http.MethodGet, "/api/v1/workbench/compliance/task-policy", "", types.TenantRoleViewer)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "handler repeats the admin predicate: %s", w.Body.String())
}

func TestComplianceAccessEndpoints(t *testing.T) {
	fake := &fakeComplianceManager{
		facts: &types.TaskMetadataFacts{TaskID: "s1", Title: "task-s1", OwnerID: "u1"},
		access: types.TaskComplianceAccess{ID: 7, TenantID: 1, TaskID: "s1", AdminID: "u9", Reason: "audit"},
		content: types.TaskContentView{TaskID: "s1", Messages: []types.TaskMessageFact{{ID: "m1", Role: "user", Content: "secret"}}},
	}
	engine := newComplianceTestEngine(t, fake)

	req, w := complianceRequest(t, http.MethodPost, "/api/v1/workbench/compliance/tasks/s1/access",
		`{"reason":"audit trail","ttl_hours":72}`, types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Equal(t, "s1", fake.lastAccess.taskID)
	require.Equal(t, "audit trail", fake.lastAccess.reason)
	require.Equal(t, 72*time.Hour, fake.lastAccess.ttl)

	req, w = complianceRequest(t, http.MethodGet, "/api/v1/workbench/compliance/tasks/s1/content", "", types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "secret")
	require.Equal(t, "s1", fake.lastContentTask)

	// Malformed bodies are 400, never a panic.
	req, w = complianceRequest(t, http.MethodPost, "/api/v1/workbench/compliance/tasks/s1/access",
		`{"reason":"x","ttl_hours":0}`, types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestComplianceHandlerMapsErrorCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"bad request", apperrors.NewBadRequestError("x"), http.StatusBadRequest},
		{"forbidden", apperrors.NewForbiddenError("x"), http.StatusForbidden},
		{"not found", apperrors.NewNotFoundError("x"), http.StatusNotFound},
		{"legal hold", apperrors.NewConflictError("x"), http.StatusConflict},
		{"internal", apperrors.NewInternalServerError("x"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeComplianceManager{policyErr: tc.err}
			engine := newComplianceTestEngine(t, fake)
			req, w := complianceRequest(t, http.MethodGet, "/api/v1/workbench/compliance/task-policy", "", types.TenantRoleAdmin)
			engine.ServeHTTP(w, req)
			require.Equal(t, tc.want, w.Code, "%s: %s", tc.name, w.Body.String())
		})
	}
}
```

再创建 `internal/container/task_compliance_wiring_test.go`——**装配登记源级断言**。理由：`RouterParams.WorkbenchTaskComplianceHandler` 是 `optional:"true"`，容器漏登记时 handler 为 nil、`RegisterWorkbenchTaskComplianceRoutes` 以 `h == nil` 静默返回——生产路由缺失而 handler 单测（直接构造 handler）完全测不出。源级断言把这层装配钉进 CI：

```go
package container

// T13 (#43) Task 4: wiring-guard. The compliance handler is an OPTIONAL
// RouterParams field — a missing dig Provide leaves it nil and the router
// silently mounts nothing, which no handler-level test can observe (the e2e
// constructs the handler directly). These source-level assertions pin the
// Provide/Invoke registrations into CI (same defense style as an
// architecture guard, scoped to this plan's wiring).

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(rel)
	require.NoError(t, err)
	return string(data)
}

func TestTaskComplianceWiringRegistered(t *testing.T) {
	containerSrc := readRepoFile(t, "container.go")
	for _, want := range []string{
		"must(container.Provide(NewTaskComplianceStore))",
		"must(container.Provide(NewTaskComplianceService))",
		"must(container.Provide(NewWorkbenchTaskComplianceHandler))",
	} {
		require.True(t, strings.Contains(containerSrc, want),
			"container.go must register %q — without it the optional handler stays nil and the compliance routes silently disappear", want)
	}
	workbenchSrc := readRepoFile(t, "workbench.go")
	for _, want := range []string{
		"func NewTaskComplianceStore(",
		"func NewTaskComplianceService(",
		"func NewWorkbenchTaskComplianceHandler(",
	} {
		require.True(t, strings.Contains(workbenchSrc, want),
			"internal/container/workbench.go must define provider %q", want)
	}
}
```

（Task 5 将向本测试追加 `wireTaskDeletionGuard` 的 Invoke 断言。相对路径 `container.go`/`workbench.go` 以测试工作目录=`internal/container` 为准，与 `go test ./internal/container/` 一致。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/handler/session/ -run 'TestCompliancePolicyEndpoints|TestComplianceAccessEndpoints|TestComplianceHandlerMapsErrorCodes' -v && go test ./internal/container/ -run TestTaskComplianceWiring -v`
Expected: FAIL —— handler 测试报 `undefined: NewWorkbenchTaskComplianceHandler`（编译错误）；wiring 测试报 Provide 行缺失（`container.go must register ...`）。

- [ ] **Step 3: 最小实现**

创建 `internal/handler/session/workbench_task_compliance.go`：

```go
package session

// T13 (#43): the compliance HTTP surface. Admin+ lane: metadata by default,
// reasoned+time-limited+audited windows for private content, and the tenant
// policy switches. Identity always arrives from the authenticated context
// (taskGrantCaller shape). The permanent-deletion endpoint joins in Task 6.

import (
	"context"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// TaskComplianceManager is the compliance service port (service.TaskComplianceService).
// Task 6 extends it with PurgeTask once the service side exists.
type TaskComplianceManager interface {
	GetTaskPolicy(ctx context.Context, caller types.Caller) (*types.TenantTaskPolicy, error)
	SetTaskPolicy(ctx context.Context, caller types.Caller, retentionDays int, legalHold bool) (*types.TenantTaskPolicy, error)
	TaskMetadata(ctx context.Context, caller types.Caller, taskID string) (types.TaskMetadataView, error)
	RequestContentAccess(ctx context.Context, caller types.Caller, taskID, reason string, ttl time.Duration) (types.TaskComplianceAccess, error)
	ReadTaskContent(ctx context.Context, caller types.Caller, taskID string) (types.TaskContentView, error)
}

// WorkbenchTaskComplianceHandler serves the T13 compliance lanes.
type WorkbenchTaskComplianceHandler struct {
	compliance TaskComplianceManager
}

// NewWorkbenchTaskComplianceHandler constructs the handler.
func NewWorkbenchTaskComplianceHandler(compliance TaskComplianceManager) *WorkbenchTaskComplianceHandler {
	return &WorkbenchTaskComplianceHandler{compliance: compliance}
}

// complianceWriteError maps service AppErrors onto the workbench envelope.
func complianceWriteError(c *gin.Context, err error) {
	if appErr, ok := apperrors.IsAppError(err); ok {
		status := http.StatusInternalServerError
		switch appErr.Code {
		case apperrors.ErrBadRequest:
			status = http.StatusBadRequest
		case apperrors.ErrForbidden:
			status = http.StatusForbidden
		case apperrors.ErrNotFound:
			status = http.StatusNotFound
		case apperrors.ErrConflict:
			status = http.StatusConflict
		case apperrors.ErrServiceUnavailable:
			status = http.StatusServiceUnavailable
		}
		c.AbortWithStatusJSON(status, gin.H{"success": false, "error": appErr.Message})
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "compliance operation failed"})
}

func (h *WorkbenchTaskComplianceHandler) refuseUnassembled(c *gin.Context) bool {
	if h == nil || h.compliance == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return true
	}
	return false
}

// refuseIdentity derives the caller exactly like taskGrantCaller
// (workbench_task_grants.go:55 shape) — tenant and user come only from the
// request context — and repeats the admin predicate so an unguarded mount
// still fails closed (production routes carry g.Admin()). A missing
// identity is 401; an identifiable non-admin is 403.
func (h *WorkbenchTaskComplianceHandler) refuseIdentity(c *gin.Context) (types.Caller, bool) {
	caller := types.CallerFromContext(c.Request.Context()).Normalize()
	if caller.TenantID == 0 || strings.TrimSpace(caller.UserID) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "identity required"})
		return types.Caller{}, false
	}
	if caller.Role.Level() < types.TenantRoleAdmin.Level() {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "compliance access requires the tenant admin role"})
		return types.Caller{}, false
	}
	return caller, true
}

// GetTaskPolicy GET /workbench/compliance/task-policy
func (h *WorkbenchTaskComplianceHandler) GetTaskPolicy(c *gin.Context) {
	if h.refuseUnassembled(c) {
		return
	}
	caller, ok := h.refuseIdentity(c)
	if !ok {
		return
	}
	policy, err := h.compliance.GetTaskPolicy(c.Request.Context(), caller)
	if err != nil {
		complianceWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"policy": policy}})
}

// SetTaskPolicy PUT /workbench/compliance/task-policy
func (h *WorkbenchTaskComplianceHandler) SetTaskPolicy(c *gin.Context) {
	if h.refuseUnassembled(c) {
		return
	}
	caller, ok := h.refuseIdentity(c)
	if !ok {
		return
	}
	var input struct {
		RetentionDays int  `json:"retention_days"`
		LegalHold     bool `json:"legal_hold"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "retention_days and legal_hold are required"})
		return
	}
	if input.RetentionDays < 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "retention_days must be >= 0"})
		return
	}
	policy, err := h.compliance.SetTaskPolicy(c.Request.Context(), caller, input.RetentionDays, input.LegalHold)
	if err != nil {
		complianceWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"policy": policy}})
}

// TaskMetadata GET /workbench/compliance/tasks/:task_id — the default,
// metadata-only view.
func (h *WorkbenchTaskComplianceHandler) TaskMetadata(c *gin.Context) {
	if h.refuseUnassembled(c) {
		return
	}
	caller, ok := h.refuseIdentity(c)
	if !ok {
		return
	}
	view, err := h.compliance.TaskMetadata(c.Request.Context(), caller, c.Param("task_id"))
	if err != nil {
		complianceWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"task": view}})
}

// RequestContentAccess POST /workbench/compliance/tasks/:task_id/access
// body: {"reason": string, "ttl_hours": int (1..168)}
func (h *WorkbenchTaskComplianceHandler) RequestContentAccess(c *gin.Context) {
	if h.refuseUnassembled(c) {
		return
	}
	caller, ok := h.refuseIdentity(c)
	if !ok {
		return
	}
	var input struct {
		Reason   string `json:"reason"`
		TTLHours int    `json:"ttl_hours"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "reason and ttl_hours are required"})
		return
	}
	if strings.TrimSpace(input.Reason) == "" || input.TTLHours < 1 || input.TTLHours > 168 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "reason is required and ttl_hours must be within 1..168"})
		return
	}
	access, err := h.compliance.RequestContentAccess(c.Request.Context(), caller, c.Param("task_id"), input.Reason, time.Duration(input.TTLHours)*time.Hour)
	if err != nil {
		complianceWriteError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"access": access}})
}

// ReadTaskContent GET /workbench/compliance/tasks/:task_id/content
func (h *WorkbenchTaskComplianceHandler) ReadTaskContent(c *gin.Context) {
	if h.refuseUnassembled(c) {
		return
	}
	caller, ok := h.refuseIdentity(c)
	if !ok {
		return
	}
	content, err := h.compliance.ReadTaskContent(c.Request.Context(), caller, c.Param("task_id"))
	if err != nil {
		complianceWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"content": content}})
}
```

（Task 6 在此文件追加 `PurgeTask` 方法与路由，本任务五方法面到此为止。）

`internal/router/routes_workbench.go` 在 `RegisterWorkbenchLegacyTaskRoutes`（`routes_workbench.go:243-252`）之后追加：

```go
// RegisterWorkbenchTaskComplianceRoutes exposes the T13 compliance lanes:
// tenant task policy, metadata-by-default, and reasoned+time-limited+audited
// content windows. The lane is Admin+ at the route; the handler repeats the
// predicate so an unguarded mount still fails closed. The permanent-deletion
// endpoint joins these groups in Task 6 (it reuses the :task_id wildcard the
// archive/grants trees already bind).
func RegisterWorkbenchTaskComplianceRoutes(r *gin.RouterGroup, h *session.WorkbenchTaskComplianceHandler, g *rbacGuards) {
	if h == nil || g == nil {
		return
	}
	compliance := g.apiKeyGroup(r.Group("/workbench/compliance", g.Admin()), apiKeyChat(apiKeyFullAccess()))
	compliance.GET("/task-policy", h.GetTaskPolicy)
	compliance.PUT("/task-policy", h.SetTaskPolicy)
	compliance.GET("/tasks/:task_id", h.TaskMetadata)
	compliance.POST("/tasks/:task_id/access", h.RequestContentAccess)
	compliance.GET("/tasks/:task_id/content", h.ReadTaskContent)
	// The permanent-deletion route joins in Task 6, appended right here:
	//   purge := g.apiKeyGroup(r.Group("/workbench/tasks", g.Admin()), apiKeyChat(apiKeyFullAccess()))
	//   purge.DELETE("/:task_id", h.PurgeTask)
}
```

（`g.apiKeyGroup(group, apiKeyChat(apiKeyFullAccess()))` 的调用形状与 `routes_workbench.go:150-151` 既有惯例逐字同款。Task 6 把上面两行注释替换为真实代码——DELETE tasks 树已绑定 `:task_id` 通配符（/archive、/grants/:grantee_id 同树），purge 复用该名。若 Task 4 时 purge 路由已被并行计划占用同形通配符名，以 gin 同树通配符同名规则为准，`task_id` 命名不变。

`internal/router/router.go` 修改两处：
1. `RouterParams`（`router.go:69` `WorkbenchTaskGrantsHandler` 行旁）追加：

```go
	WorkbenchTaskComplianceHandler *session.WorkbenchTaskComplianceHandler `optional:"true"`
```

2. 注册段（`router.go:376` `RegisterWorkbenchTaskGrantRoutes(v1, params.WorkbenchTaskGrantsHandler, rbacGuards)` 行后）追加：

```go
		RegisterWorkbenchTaskComplianceRoutes(v1, params.WorkbenchTaskComplianceHandler, rbacGuards)
```

`internal/container/workbench.go` 尾部（`NewWorkbenchLegacyListHandler` 之后）追加**三个 provider**（store→service→handler 链条全部确定化，无任何可选替代）：

```go
// NewTaskComplianceStore wires the T13 compliance store onto the shared DB
// handle (dig provider; the concrete *gorm.DB keeps dig's type graph simple).
func NewTaskComplianceStore(db *gorm.DB) *repository.TaskComplianceStore {
	return repository.NewTaskComplianceStore(db)
}

// NewTaskComplianceService wires the compliance service onto the store and
// the real audit trail. The audit service is REQUIRED — the compliance flow
// fails closed without it (Task 3).
func NewTaskComplianceService(
	store *repository.TaskComplianceStore,
	audit interfaces.AuditLogService,
) *service.TaskComplianceService {
	return service.NewTaskComplianceService(store, audit)
}

// NewWorkbenchTaskComplianceHandler wires the T13 compliance lanes to the
// service built above. Tenant/role always come from the authenticated
// context.
func NewWorkbenchTaskComplianceHandler(compliance *service.TaskComplianceService) *session.WorkbenchTaskComplianceHandler {
	return session.NewWorkbenchTaskComplianceHandler(compliance)
}
```

`internal/container/container.go` 在 `must(container.Provide(NewWorkbenchTaskGrantsHandler))`（`container.go:286`）行后追加三行 Provide 登记——**缺任何一行，容器构建即失败或 optional handler 静默为 nil**（wiring 测试把它钉进 CI）：

```go
		must(container.Provide(NewTaskComplianceStore))
		must(container.Provide(NewTaskComplianceService))
		must(container.Provide(NewWorkbenchTaskComplianceHandler))
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/handler/session/ -run 'TestCompliancePolicyEndpoints|TestComplianceAccessEndpoints|TestComplianceHandlerMapsErrorCodes' -v && go test ./internal/container/ -run TestTaskComplianceWiring -v && go build ./...`
Expected: handler 测试 PASS + wiring 断言 PASS + 构建成功。

- [ ] **Step 5: Commit**

```bash
git add internal/handler/session/workbench_task_compliance.go internal/handler/session/workbench_task_compliance_test.go internal/router/routes_workbench.go internal/router/router.go internal/container/workbench.go internal/container/container.go internal/container/task_compliance_wiring_test.go
git commit -m "feat(compliance): admin HTTP surface + routes + dig providers (t13 #43 task 4)"
```

---

### Task 5: Legal-hold 删除闸门（AllowsTaskDeletion + session.Handler 挂接）

**Files:**
- Modify: `internal/application/service/task_compliance.go`（追加 `AllowsTaskDeletion`）
- Modify: `internal/handler/session/handler.go`（`craftTombstoner` 字段旁加 `taskDeletionGuard` 字段与 setter；`DeleteSession`/`BatchDeleteSessions`/`ClearSessionMessages` 三处前置检查）
- Modify: `internal/container/container.go:1048` 旁追加 1 行 Invoke；`internal/container/workbench.go` 尾部追加 `wireTaskDeletionGuard` 函数（provider 三件套已在 Task 4 登记）
- Test: `internal/application/service/task_compliance_test.go`（追加）、`internal/handler/session/task_deletion_guard_test.go`（新建）、`internal/container/task_compliance_wiring_test.go`（追加 Invoke 断言）

**Interfaces:**
- Consumes: Task 1 的 `GetTaskPolicy`；O03 setter 模式（`handler.go:107-133`）；`AuditActionTaskDeleteDenied`（Task 3）。
- Produces: `session.TaskDeletionGuard` 接口与 `SetTaskDeletionGuard`；`TaskComplianceService.AllowsTaskDeletion(ctx, tenantID, actorUserID, sessionID) error`（Task 7 e2e 消费）。

- [ ] **Step 1: 写失败测试**

service 侧追加到 `internal/application/service/task_compliance_test.go`：

```go
func TestAllowsTaskDeletionUnderLegalHold(t *testing.T) {
	store, audit, svc := complianceFixtures()
	ctx := context.Background()

	// No policy → ungated (today's behavior preserved).
	require.NoError(t, svc.AllowsTaskDeletion(ctx, 1, "u1", "s1"))

	// Policy without hold → allowed.
	store.policy = &types.TenantTaskPolicy{TenantID: 1}
	require.NoError(t, svc.AllowsTaskDeletion(ctx, 1, "u1", "s1"))

	// Legal hold → refused, and the refusal itself is audited.
	store.policy.LegalHold = true
	err := svc.AllowsTaskDeletion(ctx, 1, "u1", "s1")
	require.ErrorIs(t, err, ErrTaskLegalHold)
	require.Len(t, audit.entries, 1)
	require.Equal(t, types.AuditActionTaskDeleteDenied, audit.entries[0].Action)
	require.Equal(t, types.AuditOutcomeDenied, audit.entries[0].Outcome)
	require.Equal(t, "s1", audit.entries[0].TargetID)

	// A broken audit trail must not silently un-gate the hold: the refusal
	// stands even when its audit write fails (deletion is already denied).
	store.policy.LegalHold = true
	audit.err = context.DeadlineExceeded
	err = svc.AllowsTaskDeletion(ctx, 1, "u1", "s2")
	require.ErrorIs(t, err, ErrTaskLegalHold)
}
```

handler 侧新建 `internal/handler/session/task_deletion_guard_test.go`：

```go
package session

// T13 (#43) Task 5: the legal-hold gate covers EVERY internal deletion
// entrance — single delete, batch (ids and delete_all) and message clear.
// The gate is nil-safe (ungated deployments keep today's flow), a refusal
// is 409, and an infrastructure error fails closed with 500.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type recordingGuard struct {
	calls []struct {
		tenant    uint64
		actor     string
		sessionID string
	}
	err error
}

func (g *recordingGuard) AllowsTaskDeletion(_ context.Context, tenant uint64, actor, sessionID string) error {
	g.calls = append(g.calls, struct {
		tenant    uint64
		actor     string
		sessionID string
	}{tenant, actor, sessionID})
	return g.err
}

func newGuardEnv(t *testing.T, guard TaskDeletionGuard) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openCraftHTTPDB(t)
	require.NoError(t, db.Exec(
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s-g', 1, 'guard', 'u1', 'trpc')").Error)
	h := &Handler{
		sessionService:    deletionSessions{},
		agentRunService:   service.NewAgentRunService(repository.NewAgentRunStore(db)),
		taskDeletionGuard: guard,
	}
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	engine.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	engine.DELETE("/api/v1/sessions/:id", h.DeleteSession)
	engine.DELETE("/api/v1/sessions/batch", h.BatchDeleteSessions)
	engine.DELETE("/api/v1/sessions/:id/messages", h.ClearSessionMessages)
	return engine
}

func TestSessionDeleteBlockedByLegalHold(t *testing.T) {
	guard := &recordingGuard{err: apperrors.NewConflictError("task deletion is blocked by the tenant legal hold")}
	engine := newGuardEnv(t, guard)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/s-g", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "legal hold refuses the single delete: %s", w.Body.String())
	require.Len(t, guard.calls, 1, "the gate ran exactly once")
	require.Equal(t, "s-g", guard.calls[0].sessionID)
}

func TestBatchAndClearBlockedByLegalHold(t *testing.T) {
	guard := &recordingGuard{err: apperrors.NewConflictError("task deletion is blocked by the tenant legal hold")}
	engine := newGuardEnv(t, guard)

	// Review Focus 5: batch (ids branch) cannot bypass the hold.
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/batch", httptestBody(t, `{"ids":["s-g"]}`))
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "batch delete is gated: %s", w.Body.String())

	// delete_all=true branch is gated too (tenant-level check, one call).
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/batch", httptestBody(t, `{"delete_all":true}`))
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "delete_all is gated: %s", w.Body.String())

	// Clearing messages destroys evidence — gated as well.
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/s-g/messages", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusConflict, w.Code, "message clear is gated: %s", w.Body.String())
}

func TestDeletionGuardNilKeepsFlowAndInfraErrorFailsClosed(t *testing.T) {
	// nil guard → today's ungated flow (200, zero gate calls).
	engine := newGuardEnv(t, nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/s-g", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// An infrastructure error fails CLOSED (500), never falls through to
	// deleting under a broken policy check.
	engine = newGuardEnv(t, &recordingGuard{err: context.DeadlineExceeded})
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/s-g", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code, "a broken gate must not un-gate deletion: %s", w.Body.String())
}

func httptestBody(t *testing.T, body string) *strings.Reader {
	t.Helper()
	return strings.NewReader(body)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/application/service/ -run 'TestAllowsTaskDeletionUnderLegalHold' -v && go test ./internal/handler/session/ -run 'TestSessionDeleteBlockedByLegalHold|TestBatchAndClearBlockedByLegalHold|TestDeletionGuardNilKeepsFlow' -v`
Expected: FAIL —— `ErrTaskLegalHold` / `taskDeletionGuard` 未定义（编译错误）。

- [ ] **Step 3: 最小实现**

`internal/application/service/task_compliance.go` 追加：

```go
// ErrTaskLegalHold refuses internal deletion lanes under the tenant legal
// hold (users cannot bypass organizational obligations — Spec story 55).
var ErrTaskLegalHold = apperrors.NewConflictError("task deletion is blocked by the tenant legal hold")

// AllowsTaskDeletion is the T13 deletion gate (session.TaskDeletionGuard).
// No policy or hold off → nil (today's flow). Hold on → ErrTaskLegalHold
// with an audit row; the refusal stands even when its audit write fails
// (deletion is already denied — the audit is the trail, not the authority).
func (s *TaskComplianceService) AllowsTaskDeletion(ctx context.Context, tenantID uint64, actorUserID, sessionID string) error {
	policy, err := s.store.GetTaskPolicy(ctx, tenantID)
	if err != nil {
		return err
	}
	if policy == nil || !policy.LegalHold {
		return nil
	}
	_ = s.audit.Log(ctx, &types.AuditLog{
		TenantID: tenantID, ActorUserID: actorUserID,
		Action:       types.AuditActionTaskDeleteDenied,
		TargetType:   "task", TargetID: sessionID,
		Outcome: types.AuditOutcomeDenied,
	})
	return ErrTaskLegalHold
}
```

（审计此处用 `s.audit.Log` 直调并忽略错误——nil audit 时跳过留痕但拒绝不变；与 RequestContentAccess 的 fail-closed 不同：这里删除**已经**被拒，审计失败不能反过来放行。计划已注明。）

`internal/handler/session/handler.go` 三处修改：

1. `CraftSessionTombstoner` 接口定义之后（`handler.go:112` 后）追加接口与字段。字段加在 `craftTombstoner`（`handler.go:69`）旁：

```go
	// taskDeletionGuard refuses the internal task deletion lanes when the
	// tenant policy (legal hold) forbids them (T13). Nil (policy lane not
	// assembled) keeps the deletion flow unchanged.
	taskDeletionGuard TaskDeletionGuard
```

接口与 setter（放在 `SetCraftTombstoner` 之后，`handler.go:116` 后）：

```go
// TaskDeletionGuard refuses internal task deletion lanes when the tenant
// policy (legal hold) forbids them (T13, #43). Implemented by
// service.TaskComplianceService.AllowsTaskDeletion.
type TaskDeletionGuard interface {
	AllowsTaskDeletion(ctx context.Context, tenantID uint64, actorUserID, sessionID string) error
}

// SetTaskDeletionGuard installs the T13 legal-hold gate (see internal/container).
func (h *Handler) SetTaskDeletionGuard(guard TaskDeletionGuard) { h.taskDeletionGuard = guard }

// guardTaskDeletion applies the T13 policy gate before any destructive
// delete. A refusal aborts with the AppError (409 under legal hold); an
// infrastructure error aborts 500 — fail closed, deletion never proceeds
// past a broken policy check.
func (h *Handler) guardTaskDeletion(c *gin.Context, ctx context.Context, sessionID string) bool {
	if h.taskDeletionGuard == nil {
		return true
	}
	tenant, ok := types.TenantIDFromContext(ctx)
	if !ok || tenant == 0 {
		return true
	}
	actor, _ := types.UserIDFromContext(ctx)
	if err := h.taskDeletionGuard.AllowsTaskDeletion(ctx, tenant, actor, sessionID); err != nil {
		c.Error(err)
		return false
	}
	return true
}
```

（`c.Error(err)` 经 `middleware.ErrorHandler` 把 AppError 映射为其 HTTPCode（409）；非 AppError（如 context.DeadlineExceeded）映射 500——`deletionSessions` 环境用 `middleware.ErrorHandler()`，与既有 tombstone 测试同形。）

2. `DeleteSession`（`handler.go:578` `h.tombstoneCraftSession(ctx, id)` 之前）插入：

```go
	// T13: the tenant policy gate precedes every destructive delete.
	if !h.guardTaskDeletion(c, ctx, id) {
		return
	}
```

3. `ClearSessionMessages`（`handler.go:632` `h.fenceSessionRuns` 之前、`GetOwnedSession` 检查之后）插入：

```go
	// T13: the tenant policy gate precedes every destructive delete.
	if !h.guardTaskDeletion(c, ctx, id) {
		return
	}
```

4. `BatchDeleteSessions` 的 ids 分支（`handler.go:735` 循环内 `h.tombstoneCraftSession(ctx, id)` 之前）插入与第 2 点完全相同的 4 行（`if !h.guardTaskDeletion(c, ctx, id) { return }`）；`delete_all` 分支插在 `if req.DeleteAll {`（`handler.go:683`）之后、`h.sessionService.GetSessionsByTenant(ctx)`（`handler.go:684`）**之前**——闸门先于任何会话列举执行，既是策略语义（拒绝时不做无用功），也保证测试环境的最小 sessionService stub 不会在闸门拒绝前被调用：

```go
		// T13: the whole-tenant wipe is one tenant-level gate call, before
		// any session is listed.
		if !h.guardTaskDeletion(c, ctx, "batch:delete_all") {
			return
		}
```

`internal/container/workbench.go` 尾部追加 wiring 函数：

```go
// wireTaskDeletionGuard installs the T13 legal-hold gate at the session
// deletion entrances (O03 wireCraftSessionTombstone shape).
func wireTaskDeletionGuard(handler *session.Handler, compliance *service.TaskComplianceService) {
	if handler == nil || compliance == nil {
		return
	}
	handler.SetTaskDeletionGuard(compliance)
}
```

`internal/container/container.go` 在 `container.go:1048 must(container.Invoke(wireCraftSessionTombstone))` 行后追加：

```go
	must(container.Invoke(wireTaskDeletionGuard))
```

（`service.TaskComplianceService` 的 dig provider `NewTaskComplianceService` 已在 Task 4 登记进 container.go——本任务的 Invoke 直接可解析；若漏了 Task 4 的 Provide，容器构建在此 panic，`TestTaskComplianceWiringRegistered` 会在 CI 更早拦截。）

`internal/container/task_compliance_wiring_test.go` 的 `TestTaskComplianceWiringRegistered` 追加一段（Invoke 断言，紧随 containerSrc 断言之后）：

```go
	require.True(t, strings.Contains(containerSrc, "must(container.Invoke(wireTaskDeletionGuard))"),
		"container.go must Invoke wireTaskDeletionGuard — without it the legal-hold gate is never installed at the deletion entrances")
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/application/service/ -run 'TestAllowsTaskDeletionUnderLegalHold' -v && go test ./internal/handler/session/ -run 'TestSessionDeleteBlockedByLegalHold|TestBatchAndClearBlockedByLegalHold|TestDeletionGuardNilKeepsFlow|TestDeleteSessionTombstonesCraftResources' -v && go build ./...`
Expected: PASS（含既有 tombstone 测试回归——闸门插桩不破坏 O03 行为）。

- [ ] **Step 5: Commit**

```bash
git add internal/application/service/task_compliance.go internal/application/service/task_compliance_test.go internal/handler/session/handler.go internal/handler/session/task_deletion_guard_test.go internal/container/workbench.go internal/container/container.go
git commit -m "feat(compliance): legal-hold deletion gate on all three delete entrances (t13 #43 task 5)"
```

---

### Task 6: 永久删除 PurgeTask（检查链 + 事务删除 + handler/路由接线）

**Files:**
- Modify: `internal/application/service/task_compliance.go`（追加 sentinel ×2 与 `PurgeTask`）
- Modify: `internal/types/task_compliance.go`（追加 `TaskPurgeReceipt`）
- Modify: `internal/application/repository/task_compliance_store.go`（追加 `PurgeTask` 事务）
- Modify: `internal/handler/session/workbench_task_compliance.go`（`TaskComplianceManager` 扩为六方法 + handler 追加 `PurgeTask` 方法）
- Modify: `internal/router/routes_workbench.go`（`RegisterWorkbenchTaskComplianceRoutes` 内追加 purge 两行，替换 Task 4 留下的注释占位）
- Modify: `internal/handler/session/workbench_task_compliance_test.go`（fake 补 purge 字段/方法、engine 挂 purge 路由、错误映射补 503 用例）
- Test: `internal/application/repository/task_compliance_store_test.go`（追加）、`internal/application/service/task_compliance_test.go`（追加）、`internal/handler/session/workbench_task_compliance_test.go`（扩展）

**Interfaces:**
- Consumes: Task 1-3 的 store/service；Task 4 的 handler/路由框架（本任务把 `TaskComplianceManager` 扩为六方法并追加 purge 端点——此时 `*service.TaskComplianceService` 已有 `PurgeTask`，接口扩展后编译成立）。
- Produces: `service.ErrTaskNotSoftDeleted`/`service.ErrTaskRetentionActive`；`store.PurgeTask`；`types.TaskPurgeReceipt`；`DELETE /api/v1/workbench/tasks/:task_id`（purge，Admin+）（Task 7 e2e 消费）。

- [ ] **Step 1: 写失败测试**

repository 侧追加到 `task_compliance_store_test.go`：

```go
func TestPurgeTaskDeletesInternalRowsOnlyKeepsAudit(t *testing.T) {
	db := openTaskComplianceDB(t)
	seedComplianceTask(t, db, 1, "s1")
	store := repository.NewTaskComplianceStore(db)
	ctx := context.Background()

	// Historical audit rows + a compliance window + a grant on the task.
	require.NoError(t, db.Exec(
		"INSERT INTO audit_logs (tenant_id, actor_user_id, action) VALUES (1,'u1','session.created'),(1,'u1','kb.indexed')").Error)
	_, err := store.OpenComplianceAccess(ctx, types.TaskComplianceAccess{
		TenantID: 1, TaskID: "s1", AdminID: "u9", Reason: "audit", ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		"INSERT INTO task_grants (tenant_id, task_id, grantee_id, role, granted_by) VALUES (1,'s1','u2','viewer','u1')").Error)
	var auditsBefore int64
	require.NoError(t, db.Table("audit_logs").Where("tenant_id = ?", uint64(1)).Count(&auditsBefore).Error)
	require.Equal(t, int64(2), auditsBefore)

	// Purge is idempotent-miss on an unknown task.
	require.ErrorIs(t, store.PurgeTask(ctx, 1, "s-missing"), types.ErrTaskComplianceNotFound)
	// Cross-tenant probe: same uniform miss.
	require.ErrorIs(t, store.PurgeTask(ctx, 2, "s1"), types.ErrTaskComplianceNotFound)

	require.NoError(t, store.PurgeTask(ctx, 1, "s1"))

	// Every INTERNAL row of the task is gone. Where clauses are fully
	// explicit per table — no string-built SQL anywhere (values bind as
	// parameters; the column/table names never come from input).
	require.Zero(t, complianceRowCount(t, db, "sessions", "tenant_id = ? AND id = ?", uint64(1), "s1"))
	require.Zero(t, complianceRowCount(t, db, "messages", "session_id = ?", "s1"))
	require.Zero(t, complianceRowCount(t, db, "agent_runs", "tenant_id = ? AND session_id = ?", uint64(1), "s1"))
	require.Zero(t, complianceRowCount(t, db, "task_grants", "tenant_id = ? AND task_id = ?", uint64(1), "s1"))
	require.Zero(t, complianceRowCount(t, db, "task_compliance_access", "tenant_id = ? AND task_id = ?", uint64(1), "s1"))

	// AC2 / Review Focus 4: audit rows survive every purge untouched, and
	// the new trail (task.purged) is APPENDED, never destructive.
	var auditsAfter int64
	require.NoError(t, db.Table("audit_logs").Where("tenant_id = ?", uint64(1)).Count(&auditsAfter).Error)
	require.GreaterOrEqual(t, auditsAfter, auditsBefore, "audit_logs is append-only across purges")
}

// complianceRowCount is the test-only count helper: table and where come
// from call sites above (fixed literals), args bind as parameters.
func complianceRowCount(t *testing.T, db *gorm.DB, table, where string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table(table).Where(where, args...).Count(&n).Error)
	return n
}
```

service 侧追加到 `task_compliance_test.go`。同时给 Task 3 的 `stubComplianceStore` 补一个 `purgeFn func() error` 字段并把其 `PurgeTask` 方法体改为：

```go
func (s *stubComplianceStore) PurgeTask(context.Context, uint64, string) error {
	if s.purgeFn != nil {
		return s.purgeFn()
	}
	return nil
}
```

追加的测试：

```go
func TestPurgeTaskPolicyChain(t *testing.T) {
	deletedAt := time.Now().UTC().Add(-10 * 24 * time.Hour) // soft-deleted 10 days ago
	now := func() time.Time { return time.Now().UTC() }

	newStore := func() *stubComplianceStore {
		return &stubComplianceStore{
			facts: &types.TaskMetadataFacts{TaskID: "s1", OwnerID: "u1", DeletedAt: &deletedAt},
		}
	}
	audit := &recordingAudit{}

	// Legal hold refuses first, even before the soft-delete check.
	store := newStore()
	store.policy = &types.TenantTaskPolicy{TenantID: 1, LegalHold: true}
	_, err := NewTaskComplianceServiceWithClock(store, audit, now).PurgeTask(context.Background(), adminCaller(), "s1")
	require.ErrorIs(t, err, ErrTaskLegalHold)

	// A live (not soft-deleted) task refuses: purge never hard-deletes what
	// the user still sees.
	store = newStore()
	store.facts.DeletedAt = nil
	_, err = NewTaskComplianceServiceWithClock(store, audit, now).PurgeTask(context.Background(), adminCaller(), "s1")
	require.ErrorIs(t, err, ErrTaskNotSoftDeleted)

	// Inside the retention window (30 days, deleted 10 days ago) refuses.
	store = newStore()
	store.policy = &types.TenantTaskPolicy{TenantID: 1, RetentionDays: 30}
	_, err = NewTaskComplianceServiceWithClock(store, audit, now).PurgeTask(context.Background(), adminCaller(), "s1")
	require.ErrorIs(t, err, ErrTaskRetentionActive)

	// Past the horizon (retention 7 days, deleted 10 days ago) proceeds,
	// with the audit row written BEFORE the destructive transaction.
	store = newStore()
	store.policy = &types.TenantTaskPolicy{TenantID: 1, RetentionDays: 7}
	deleted := false
	store.purgeFn = func() error { deleted = true; return nil }
	receipt, err := NewTaskComplianceServiceWithClock(store, audit, now).PurgeTask(context.Background(), adminCaller(), "s1")
	require.NoError(t, err)
	require.Equal(t, "s1", receipt.TaskID)
	require.True(t, deleted)
	require.NotEmpty(t, audit.entries, "purge leaves an audit row")
	require.Equal(t, types.AuditActionTaskPurged, audit.entries[len(audit.entries)-1].Action)
}
```

handler 侧追加到 `workbench_task_compliance_test.go`（四处）：fake 结构体补字段、fake 补方法、engine 挂 purge 路由、新端点测试——

```go
// fakeComplianceManager 结构体追加字段（lastContentTask 之后）：
	purgeReceipt    types.TaskPurgeReceipt
	purgeErr        error
	lastPurgeTask   string

// fake 追加方法（ReadTaskContent 之后）：
func (f *fakeComplianceManager) PurgeTask(_ context.Context, _ types.Caller, taskID string) (types.TaskPurgeReceipt, error) {
	f.lastPurgeTask = taskID
	return f.purgeReceipt, f.purgeErr
}

// newComplianceTestEngine 的 v1 组末尾追加一行（ReadTaskContent 路由之后）：
	v1.DELETE("/workbench/tasks/:task_id", h.PurgeTask)

// 追加端点测试：
func TestCompliancePurgeEndpoint(t *testing.T) {
	fake := &fakeComplianceManager{}
	engine := newComplianceTestEngine(t, fake)

	req, w := complianceRequest(t, http.MethodDelete, "/api/v1/workbench/tasks/s1", "", types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "s1", fake.lastPurgeTask)

	// Service refusals map through the same envelope: legal hold → 409.
	fake.purgeErr = apperrors.NewConflictError("task deletion is blocked by the tenant legal hold")
	req, w = complianceRequest(t, http.MethodDelete, "/api/v1/workbench/tasks/s1", "", types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())

	// Audit missing → 503 (fail closed surfaces as service unavailable).
	fake.purgeErr = apperrors.NewServiceUnavailableError("compliance audit trail is required")
	req, w = complianceRequest(t, http.MethodDelete, "/api/v1/workbench/tasks/s1", "", types.TenantRoleAdmin)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
}
```

（`fake` 的 `purgeReceipt types.TaskPurgeReceipt` 字段引用 `types.TaskPurgeReceipt` 与 fake 方法引用 `h.PurgeTask`——本任务尚未实现它们，编译失败即 RED 的组成部分。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/application/repository/ -run 'TestPurgeTaskDeletesInternalRowsOnlyKeepsAudit' -v && go test ./internal/application/service/ -run 'TestPurgeTaskPolicyChain' -v && go test ./internal/handler/session/ -run 'TestCompliancePurgeEndpoint' -v`
Expected: FAIL —— `store.PurgeTask` / `ErrTaskNotSoftDeleted` / `types.TaskPurgeReceipt` / handler `PurgeTask` 未定义（编译错误）。

- [ ] **Step 3: 最小实现**

`internal/types/task_compliance.go` 追加：

```go
// TaskPurgeReceipt reports one completed permanent deletion.
type TaskPurgeReceipt struct {
	TaskID   string    `json:"task_id"`
	PurgedAt time.Time `json:"purged_at"`
}
```

`internal/application/repository/task_compliance_store.go` 追加：

```go
// PurgeTask permanently deletes ONE task's internal rows inside one
// transaction. The statement set is exactly these five tables, child rows
// first: messages (content), agent_runs (durable runs; their decision/
// interaction/observation children carry ON DELETE CASCADE), task_grants
// (#42 collaboration metadata), task_compliance_access (the windows die
// with the task — the AUDIT rows in audit_logs are a different,
// append-only store and are never touched here), and finally the session
// row itself (its craft workspace/delegation rows cascade with it; the
// external resources they reference were already tombstoned at soft-delete
// time and belong to the sweep lanes, not to this statement). No external
// system is contacted — AC2 is structural.
func (s *TaskComplianceStore) PurgeTask(ctx context.Context, tenantID uint64, taskID string) error {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(taskID) == "" {
		return errors.New("task compliance store is not assembled")
	}
	taskID = strings.TrimSpace(taskID)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM messages WHERE session_id = ?", taskID).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM agent_runs WHERE tenant_id = ? AND session_id = ?", tenantID, taskID).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM task_grants WHERE tenant_id = ? AND task_id = ?", tenantID, taskID).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM task_compliance_access WHERE tenant_id = ? AND task_id = ?", tenantID, taskID).Error; err != nil {
			return err
		}
		res := tx.Exec("DELETE FROM sessions WHERE tenant_id = ? AND id = ?", tenantID, taskID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return types.ErrTaskComplianceNotFound
		}
		return nil
	})
}
```

`internal/application/service/task_compliance.go` 追加：

```go
// ErrTaskNotSoftDeleted refuses purging a task the user still sees.
var ErrTaskNotSoftDeleted = apperrors.NewConflictError("task must be soft-deleted before permanent deletion")

// ErrTaskRetentionActive refuses purging inside the tenant retention window.
var ErrTaskRetentionActive = apperrors.NewConflictError("task is still inside the tenant retention window")

// PurgeTask is the permanent-deletion lane (admin-only). The check chain is
// fixed: task exists (uniform 404) → legal hold off → already soft-deleted
// → past the retention horizon. The authorization audit row is written
// BEFORE the destructive transaction (fail closed); if the transaction then
// fails the row remains as the authorization event, never as proof of
// deletion (the receipt is the proof).
func (s *TaskComplianceService) PurgeTask(ctx context.Context, caller types.Caller, taskID string) (types.TaskPurgeReceipt, error) {
	if err := s.requireAdmin(caller); err != nil {
		return types.TaskPurgeReceipt{}, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return types.TaskPurgeReceipt{}, apperrors.NewBadRequestError("task id is required")
	}
	facts, err := s.store.TaskMetadataFacts(ctx, caller.TenantID, taskID)
	if err != nil {
		return types.TaskPurgeReceipt{}, normalizeComplianceMiss(err)
	}
	policy, err := s.store.GetTaskPolicy(ctx, caller.TenantID)
	if err != nil {
		return types.TaskPurgeReceipt{}, err
	}
	if policy != nil && policy.LegalHold {
		return types.TaskPurgeReceipt{}, ErrTaskLegalHold
	}
	if facts.DeletedAt == nil {
		return types.TaskPurgeReceipt{}, ErrTaskNotSoftDeleted
	}
	now := s.now().UTC()
	if policy != nil && policy.RetentionDays > 0 && now.Sub(*facts.DeletedAt) < time.Duration(policy.RetentionDays)*24*time.Hour {
		return types.TaskPurgeReceipt{}, ErrTaskRetentionActive
	}
	if err := s.audited(ctx, &types.AuditLog{
		TenantID: caller.TenantID, ActorUserID: caller.UserID, ActorRole: string(caller.Role),
		Action: types.AuditActionTaskPurged,
		TargetType: "task", TargetID: facts.TaskID, TargetUserID: facts.OwnerID,
		Outcome: types.AuditOutcomeSuccess,
	}); err != nil {
		return types.TaskPurgeReceipt{}, err
	}
	if err := s.store.PurgeTask(ctx, caller.TenantID, facts.TaskID); err != nil {
		return types.TaskPurgeReceipt{}, normalizeComplianceMiss(err)
	}
	return types.TaskPurgeReceipt{TaskID: facts.TaskID, PurgedAt: now}, nil
}
```

**HTTP 接线（本任务第三部分）**——`*service.TaskComplianceService` 现已具备 `PurgeTask`，把 Task 4 留下的接口/端点缺口补齐：

1. `internal/handler/session/workbench_task_compliance.go`：`TaskComplianceManager` 接口追加第六方法（放在 `ReadTaskContent` 之后），并把文件头注释的 "The permanent-deletion endpoint joins in Task 6." 改为已交付表述：

```go
	PurgeTask(ctx context.Context, caller types.Caller, taskID string) (types.TaskPurgeReceipt, error)
```

2. 同文件追加 handler 方法（`ReadTaskContent` 之后）：

```go
// PurgeTask DELETE /workbench/tasks/:task_id — permanent deletion behind the
// fixed check chain (exists → no legal hold → soft-deleted → past retention).
func (h *WorkbenchTaskComplianceHandler) PurgeTask(c *gin.Context) {
	if h.refuseUnassembled(c) {
		return
	}
	caller, ok := h.refuseIdentity(c)
	if !ok {
		return
	}
	receipt, err := h.compliance.PurgeTask(c.Request.Context(), caller, c.Param("task_id"))
	if err != nil {
		complianceWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"purged": receipt}})
}
```

3. `internal/router/routes_workbench.go`：把 Task 4 在 `RegisterWorkbenchTaskComplianceRoutes` 里留下的两行注释占位替换为真实代码：

```go
	purge := g.apiKeyGroup(r.Group("/workbench/tasks", g.Admin()), apiKeyChat(apiKeyFullAccess()))
	purge.DELETE("/:task_id", h.PurgeTask)
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/application/repository/ -run 'TestPurgeTaskDeletesInternalRowsOnlyKeepsAudit' -v && go test ./internal/application/service/ -run 'TestPurgeTaskPolicyChain' -v && go test ./internal/handler/session/ -run 'TestCompliancePurgeEndpoint|TestCompliancePolicyEndpoints|TestComplianceAccessEndpoints|TestComplianceHandlerMapsErrorCodes' -v && go test ./internal/container/ -run TestTaskComplianceWiring -v && go build ./...`
Expected: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/types/task_compliance.go internal/application/repository/task_compliance_store.go internal/application/repository/task_compliance_store_test.go internal/application/service/task_compliance.go internal/application/service/task_compliance_test.go internal/handler/session/workbench_task_compliance.go internal/handler/session/workbench_task_compliance_test.go internal/router/routes_workbench.go
git commit -m "feat(compliance): policy-driven permanent deletion end-to-end (t13 #43 task 6)"
```

---

### Task 7: 端到端集成证据（AC1 / AC2 / AC3）

**Files:**
- Test: `internal/handler/session/task_compliance_e2e_test.go`（新建，唯一文件）

**Interfaces:**
- Consumes: Task 1-6 全部产出；#42 的 `NewWorkbenchTaskGrantsHandler`/`NewWorkbenchReadHandler(...).WithGrantedRuns(runs)`；`openCraftHTTPDB`（`craft_test.go:115`）；`recordingTombstoner`/`deletionSessions`（`session_delete_tombstone_test.go`）。
- Produces: AC1/AC2/AC3 的最终证据（#71 证据矩阵消费）。

- [ ] **Step 1: 写测试（本任务测试从零即为 RED——依赖 Task 1-6 已合并的符号；若符号缺失即编译失败=RED）**

创建 `internal/handler/session/task_compliance_e2e_test.go`：

```go
package session

// T13 (#43) end-to-end evidence — AC3: every assertion below runs over a
// REAL fully-migrated sqlite database (openCraftHTTPDB) with REAL stores,
// the REAL TaskComplianceService, the REAL compliance/grants/read handlers,
// the REAL legal-hold gate inside the REAL DeleteSession handler, and the
// REAL audit trail (repository.NewAuditLogRepository + service.NewAuditLogService).
// No service mock, no hand-written projection (AC3: 底层单测、静态检查或 mock
// 不冒充真实集成证据).
//
// AC1 = TestComplianceEndToEndAC1IndependentFlow.
// AC2 = TestComplianceEndToEndAC2DeletionNeverCascadesOutward.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appservice "github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// sqlBackedSessions backs the REAL DeleteSession handler with the minimum
// session-service surface: ownership read + the production-shaped soft
// delete (UPDATE sessions SET deleted_at). Everything else in the real
// service (message cleanup goroutine, suggestion/Redis cleanup, sandbox
// teardown) is out of the deletion contract under test.
type sqlBackedSessions struct {
	db *gorm.DB
	interfaces.SessionService
}

func (s sqlBackedSessions) GetOwnedSession(ctx context.Context, id string) (*types.Session, error) {
	tenant, _ := types.TenantIDFromContext(ctx)
	user, _ := types.UserIDFromContext(ctx)
	var sess types.Session
	err := s.db.Unscoped().Where("tenant_id = ? AND id = ? AND user_id = ?", tenant, id, user).Take(&sess).Error
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s sqlBackedSessions) DeleteSession(ctx context.Context, id string) error {
	tenant, _ := types.TenantIDFromContext(ctx)
	return s.db.Model(&types.Session{}).
		Where("tenant_id = ? AND id = ?", tenant, id).
		Update("deleted_at", time.Now().UTC()).Error
}

type complianceE2EEnv struct {
	db         *gorm.DB
	engine     *gin.Engine
	compliance *appservice.TaskComplianceService
	clock      *time.Time
}

func newComplianceE2EEnv(t *testing.T) *complianceE2EEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openCraftHTTPDB(t)
	seedComplianceE2E(t, db)

	store := repository.NewTaskComplianceStore(db)
	audit := appservice.NewAuditLogService(repository.NewAuditLogRepository(db))
	env := &complianceE2EEnv{db: db}
	now := time.Now().UTC()
	env.clock = &now
	env.compliance = appservice.NewTaskComplianceServiceWithClock(store, audit, func() time.Time { return *env.clock })

	runs := repository.NewAgentRunStore(db)
	readHandler := NewWorkbenchReadHandler(runs, repository.NewAgentRunSnapshotRepository(db)).WithGrantedRuns(runs)
	grantsHandler := NewWorkbenchTaskGrantsHandler(appservice.NewTaskGrantService(
		repository.NewTaskGrantStore(db),
		repository.NewSessionRepository(db),
		repository.NewTenantMemberRepository(db),
	))
	complianceHandler := NewWorkbenchTaskComplianceHandler(env.compliance)
	// The REAL #34 archive lane — the e2e asserts legal hold never blocks it.
	taskStateHandler := NewWorkbenchTaskStateHandler(repository.NewWorkbenchTaskStateStore(db))
	sessHandler := &Handler{
		sessionService:    sqlBackedSessions{db: db},
		agentRunService:   appservice.NewAgentRunService(runs),
		craftTombstoner:   &recordingTombstoner{},
		taskDeletionGuard: env.compliance,
	}

	r := gin.New()
	r.Use(middleware.ErrorHandler())
	v1 := r.Group("/api/v1")
	v1.DELETE("/sessions/:id", sessHandler.DeleteSession)
	v1.GET("/workbench/executions/:run_id", readHandler.GetWorkbenchExecution)
	v1.GET("/workbench/tasks/:task_id/grants", grantsHandler.List)
	v1.POST("/workbench/tasks/:task_id/archive", taskStateHandler.Archive)
	v1.GET("/workbench/compliance/task-policy", complianceHandler.GetTaskPolicy)
	v1.PUT("/workbench/compliance/task-policy", complianceHandler.SetTaskPolicy)
	v1.GET("/workbench/compliance/tasks/:task_id", complianceHandler.TaskMetadata)
	v1.POST("/workbench/compliance/tasks/:task_id/access", complianceHandler.RequestContentAccess)
	v1.GET("/workbench/compliance/tasks/:task_id/content", complianceHandler.ReadTaskContent)
	v1.DELETE("/workbench/tasks/:task_id", complianceHandler.PurgeTask)
	env.engine = r
	return env
}

func seedComplianceE2E(t *testing.T, db *gorm.DB) {
	t.Helper()
	// u1 owns s1 (contributor); u9 is the tenant admin; u2 is a viewer.
	require.NoError(t, db.Exec(
		"INSERT INTO users (id, username, email, password_hash, tenant_id) VALUES ('u9','u9','u9@example.test','x',1)").Error)
	for _, m := range []struct {
		user string
		role types.TenantRole
	}{
		{"u1", types.TenantRoleContributor},
		{"u2", types.TenantRoleViewer},
		{"u9", types.TenantRoleAdmin},
	} {
		require.NoError(t, db.Create(&types.TenantMember{
			UserID: m.user, TenantID: 1, Role: m.role, Status: types.TenantMemberStatusActive, JoinedAt: time.Now().UTC(),
		}).Error)
	}
	require.NoError(t, db.Exec(
		"INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES ('s1', 1, 'task-s1', 'u1', 'trpc')").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO messages (id, request_id, session_id, role, content) VALUES ('m1','req-1','s1','user','private question'), ('m2','req-1','s1','assistant','private answer')").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, engine_type, status, snapshot) VALUES (1,'r1','s1','u1','req-1','m2','h1','trpc','succeeded','{}')").Error)
	require.NoError(t, db.Exec(
		"INSERT INTO craft_workspaces (id, tenant_id, session_id, owner_id) VALUES ('cw1', 1, 's1', 'u1')").Error)
	// Historical audit rows — the baseline that must never shrink.
	require.NoError(t, db.Exec(
		"INSERT INTO audit_logs (tenant_id, actor_user_id, action) VALUES (1,'u1','session.created'),(1,'u1','kb.indexed')").Error)
}

func (e *complianceE2EEnv) do(t *testing.T, method, path, body, userID string, role types.TenantRole) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func (e *complianceE2EEnv) count(t *testing.T, table, where string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, e.db.Table(table).Where(where, args...).Count(&n).Error)
	return n
}

// AC1: 合规访问不把管理员加入 Task 协作列表。The compliance flow is an
// INDEPENDENT lane: metadata is default-visible to the admin, private
// content requires a reasoned+time-limited window with a full audit trail,
// and neither the window nor the metadata view ever creates a task_grants
// row — the run read lane and the grants list stay untouched.
func TestComplianceEndToEndAC1IndependentFlow(t *testing.T) {
	env := newComplianceE2EEnv(t)

	// Metadata by default: the admin reads the metadata view, no window needed.
	w := env.do(t, http.MethodGet, "/api/v1/workbench/compliance/tasks/s1", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "task-s1")
	require.NotContains(t, w.Body.String(), "private question", "the default view carries no content")

	// Review Focus 1: the admin cannot reach private content via the run
	// read lane — Admin+ tenant role is NOT run-level authorization.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusNotFound, w.Code, "run lane: owner-or-grant only, admin role gains nothing: %s", w.Body.String())

	// Content without a window is refused (403).
	w = env.do(t, http.MethodGet, "/api/v1/workbench/compliance/tasks/s1/content", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// Reasoned + time-limited window opens (201) with the audit trail.
	w = env.do(t, http.MethodPost, "/api/v1/workbench/compliance/tasks/s1/access",
		`{"reason":"security incident review","ttl_hours":72}`, "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// The window unlocks ONLY the compliance content lane.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/compliance/tasks/s1/content", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "private question")

	// AC1 core: still no grant row for the admin — and the run lane STILL
	// refuses the admin (the window grants no collaboration identity).
	require.Zero(t, env.count(t, "task_grants", "tenant_id = ? AND grantee_id = ?", uint64(1), "u9"),
		"AC1: a compliance window must never write a task_grants row")
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusNotFound, w.Code, "the window grants no run-lane access: %s", w.Body.String())

	// The owner's grants list stays clean of the administrator.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/tasks/s1/grants", "", "u1", types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), "u9", "the collaboration list never contains the admin (AC1)")

	// The full trail exists: requested + content_read.
	require.Equal(t, int64(2), env.count(t, "audit_logs",
		"tenant_id = ? AND action IN (?, ?)", uint64(1),
		types.AuditActionComplianceAccessRequested, types.AuditActionComplianceContentRead))

	// Review Focus 2: after the window expires, content seals again.
	future := time.Now().UTC().Add(73 * time.Hour)
	env.clock = &future
	w = env.do(t, http.MethodGet, "/api/v1/workbench/compliance/tasks/s1/content", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusForbidden, w.Code, "an expired window authorizes nothing: %s", w.Body.String())

	// Non-admin is refused on every compliance endpoint (403).
	w = env.do(t, http.MethodGet, "/api/v1/workbench/compliance/tasks/s1", "", "u1", types.TenantRoleContributor)
	require.Equal(t, http.StatusForbidden, w.Code)
	w = env.do(t, http.MethodPost, "/api/v1/workbench/compliance/tasks/s1/access",
		`{"reason":"x","ttl_hours":1}`, "u2", types.TenantRoleViewer)
	require.Equal(t, http.StatusForbidden, w.Code)
}

// AC2: 删除内部 Task 不隐式删除外部文档、代码或审计。Soft delete keeps the
// craft workspace row (tombstone lanes own external teardown) and every
// audit row; legal hold refuses deletion outright with its own audit trail;
// purge (past retention) removes internal rows only — audit rows only grow.
func TestComplianceEndToEndAC2DeletionNeverCascadesOutward(t *testing.T) {
	env := newComplianceE2EEnv(t)
	auditsBefore := env.count(t, "audit_logs", "tenant_id = ?", uint64(1))

	// Legal hold ON: the owner's delete is refused (409) and audited.
	w := env.do(t, http.MethodPut, "/api/v1/workbench/compliance/task-policy",
		`{"retention_days":30,"legal_hold":true}`, "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = env.do(t, http.MethodDelete, "/api/v1/sessions/s1", "", "u1", types.TenantRoleContributor)
	require.Equal(t, http.StatusConflict, w.Code, "legal hold refuses internal deletion: %s", w.Body.String())
	require.Equal(t, int64(1), env.count(t, "audit_logs",
		"tenant_id = ? AND action = ?", uint64(1), types.AuditActionTaskDeleteDenied))
	require.Zero(t, env.count(t, "sessions", "tenant_id = ? AND id = ? AND deleted_at IS NOT NULL", uint64(1), "s1"),
		"the refused delete must not soft-delete anything")

	// Archiving stays available under hold (non-destructive organization —
	// the REAL #34 archive handler succeeds while the delete lane is gated).
	w = env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/archive", "", "u1", types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, "archive is a non-destructive action, hold never blocks it: %s", w.Body.String())
	require.Equal(t, int64(1), env.count(t, "sessions", "tenant_id = ? AND id = ? AND archived_at IS NOT NULL", uint64(1), "s1"),
		"the archive really landed")

	// Hold OFF + retention 30d: the delete now succeeds (soft), and NOTHING
	// external or auditable disappears.
	w = env.do(t, http.MethodPut, "/api/v1/workbench/compliance/task-policy",
		`{"retention_days":30,"legal_hold":false}`, "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusOK, w.Code)
	w = env.do(t, http.MethodDelete, "/api/v1/sessions/s1", "", "u1", types.TenantRoleContributor)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, int64(1), env.count(t, "sessions", "tenant_id = ? AND id = ? AND deleted_at IS NOT NULL", uint64(1), "s1"),
		"the delete soft-deleted the session")
	require.Equal(t, int64(1), env.count(t, "craft_workspaces", "tenant_id = ? AND session_id = ?", uint64(1), "s1"),
		"AC2: internal soft delete keeps the craft workspace row (external teardown belongs to the tombstone/sweep lanes)")
	require.Equal(t, int64(2), env.count(t, "messages", "session_id = ?", "s1"),
		"soft delete keeps message rows (restorable semantics)")
	auditsAfterDelete := env.count(t, "audit_logs", "tenant_id = ?", uint64(1))
	require.GreaterOrEqual(t, auditsAfterDelete, auditsBefore, "AC2: audit rows never shrink")

	// Purge inside the retention window (10 days in, 30 required): refused.
	inWindow := time.Now().UTC().Add(10 * 24 * time.Hour)
	env.clock = &inWindow
	w = env.do(t, http.MethodDelete, "/api/v1/workbench/tasks/s1", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusConflict, w.Code, "retention window refuses purge: %s", w.Body.String())

	// Past the horizon (40 days in): purge succeeds, internal rows vanish,
	// the audit trail only GREW (task.purged appended), nothing else called.
	pastHorizon := time.Now().UTC().Add(40 * 24 * time.Hour)
	env.clock = &pastHorizon
	w = env.do(t, http.MethodDelete, "/api/v1/workbench/tasks/s1", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Zero(t, env.count(t, "sessions", "tenant_id = ? AND id = ?", uint64(1), "s1"))
	require.Zero(t, env.count(t, "messages", "session_id = ?", "s1"))
	require.Zero(t, env.count(t, "agent_runs", "tenant_id = ? AND session_id = ?", uint64(1), "s1"))
	require.Zero(t, env.count(t, "task_compliance_access", "tenant_id = ? AND task_id = ?", uint64(1), "s1"))
	require.Equal(t, int64(1), env.count(t, "audit_logs", "tenant_id = ? AND action = ?", uint64(1), types.AuditActionTaskPurged),
		"the purge authorization is audited")
	finalAudits := env.count(t, "audit_logs", "tenant_id = ?", uint64(1))
	require.GreaterOrEqual(t, finalAudits, auditsAfterDelete, "AC2: purge never deletes audit rows")

	// Cross-tenant uniform miss on the compliance lane.
	w = env.do(t, http.MethodDelete, "/api/v1/workbench/tasks/s1", "", "u9", types.TenantRoleAdmin)
	require.Equal(t, http.StatusNotFound, w.Code, "a purged task is a uniform 404: %s", w.Body.String())
}
```

执行细节：
1. `openCraftHTTPDB` 已种 tenant 1 与 users u1/u2（`craft_test.go:139-142`），seed 只补 u9/members/s1/messages/run/craft/audit。
2. `NewWorkbenchReadHandler(runs, snapshots)` 第三参 ingestor 是可选 variadic（`workbench_read.go:66` 实测），两参调用合法，`.WithGrantedRuns(runs)` 返回同类型。

- [ ] **Step 2: 运行测试确认失败（在 Task 0-6 已实现的前提下，本测试应直接绿；若本任务先于前置执行，编译失败即 RED）**

Run: `go test ./internal/handler/session/ -run 'TestComplianceEndToEnd' -v`
Expected: 在 Task 1-6 符号齐备时 PASS。这是集成证据任务：其 RED 形态是「任何一处端到端语义缺口 → 断言失败」，而非单元级编译失败。执行者若看到断言失败，按失败断言定位到对应 Task 的实现缺口修复后重跑（不允许改断言迁就实现）。

- [ ] **Step 3: 运行全组测试确认无回归**

Run: `go test ./internal/handler/session/ -run 'TestCompliance|TestSessionDeleteBlockedByLegalHold|TestBatchAndClearBlockedByLegalHold|TestDeletionGuardNilKeepsFlow|TestDeleteSessionTombstonesCraftResources' -v && go test ./internal/application/repository/ -run 'TestTaskCompliance|TestTaskPolicyStore|TestOpenAndActive|TestTaskMetadataFacts|TestListTaskMessages|TestPurgeTask' -v && go test ./internal/application/service/ -run 'TestSetTaskPolicy|TestRequestContentAccess|TestReadTaskContent|TestComplianceAuditFailClosed|TestTaskMetadataIsAdmin|TestAllowsTaskDeletion|TestPurgeTaskPolicyChain' -v && go build ./...`
Expected: 全部 PASS + 构建成功。

- [ ] **Step 4: Commit**

```bash
git add internal/handler/session/task_compliance_e2e_test.go
git commit -m "test(compliance): end-to-end AC1/AC2 evidence over real migrations + real handlers (t13 #43 task 7)"
```

---

## 计划级验证命令（testCommand）

在 worktree 根执行（覆盖本计划全部定向测试 + 编译；**前提：Task 0 的迁移去重已应用**，否则全量迁移装载即失败）：

```bash
go build ./... && go test ./internal/application/repository/ -run 'TestTaskCompliance|TestTaskPolicyStore|TestOpenAndActive|TestTaskMetadataFacts|TestListTaskMessages|TestPurgeTask' -v && go test ./internal/application/service/ -run 'TestSetTaskPolicyRequiresAdmin|TestRequestContentAccessValidates|TestReadTaskContentRequires|TestComplianceAuditFailClosed|TestTaskMetadataIsAdminOnly|TestAllowsTaskDeletionUnderLegalHold|TestPurgeTaskPolicyChain' -v && go test ./internal/handler/session/ -run 'TestCompliancePolicyEndpoints|TestComplianceAccessEndpoints|TestComplianceHandlerMapsErrorCodes|TestCompliancePurgeEndpoint|TestSessionDeleteBlockedByLegalHold|TestBatchAndClearBlockedByLegalHold|TestDeletionGuardNilKeepsFlow|TestDeleteSessionTombstonesCraftResources|TestComplianceEndToEnd' -v && go test ./internal/container/ -run TestTaskComplianceWiring -v
```

（不跑全量 `go test ./...`——避免无关 flaky 套件；回归面由 `go build ./...` 编译全仓 + 四个受影响包的定向测试覆盖。）

## 自我审查记录（writing-plans 四项检查 + 独立审查修复记录）

0. **独立审查八项发现逐一修复**：F1（迁移同号双文件）→ 新增 Task 0（重命名 `agent_adoption_variants` 四文件去重，作者已在临时 worktree 实测：重命名后 `TestDeleteSessionTombstonesCraftResources`、`TestTaskGrant`、`go test ./internal/database/`、`go build ./...` 全 PASS），本计划迁移改用 versioned 000193 / sqlite 000114，Global Constraints 删除「同号先例」错误声明；F2（ActorRole 类型不符）→ 断言改 `string(types.TenantRoleAdmin)`；F3（delete_all 分支 nil 接口 panic）→ 闸门插入点前移到 `GetSessionsByTenant`（handler.go:684）之前；F4（Task4↔Task6 时序矛盾）→ Task 4 接口收窄为五方法/五端点，PurgeTask 的接口扩展+handler 方法+路由移入 Task 6（此时 service 已有该方法）；F5（dig Provide 缺登记）→ Task 4 写死三行 Provide + 三个 provider（删除「二选一」），新增 `task_compliance_wiring_test.go` 源级断言把 optional-handler 静默缺失钉进 CI，Task 5 追加 Invoke 断言；F6（归档钉住表述）→ 差异记录第 4 条改为「Task 7 e2e 钉住」；F7（SQL 拼接字面冲突）→ purge 测试改逐表显式 where（`complianceRowCount`）；F8（AC3 边界明示）→ 差异记录第 8 条。
1. **Spec 覆盖**：Issue 三条 AC → AC1=Task 3（类型级无 grant 路径）+Task 4（独立端点）+Task 7（e2e 断言 task_grants 零行+grants 列表无 admin+run 读面 404）；AC2=Task 5（legal hold 三入口闸门）+Task 6（purge 语句集不含 audit/零外部调用）+Task 7（软删后 craft/audit 行数断言、purge 后 audit 只增）；AC3=Task 7（真实迁移+真实装配+httptest 全链，零 mock 冒充；sqlBackedSessions 边界见差异记录第 8 条）。What to build 三句：元数据默认（Task 3 TaskMetadata+metadataOnly 编译卫兵）、理由+期限+审计（Task 3 RequestContentAccess+fail-closed）、归档/法律保留/永久删除遵守 policy（Task 5 hold 闸门+Task 6 检查链；归档不受 hold 由 Task 7 e2e 的真实 #34 archive handler 断言钉住）。CONTEXT.md 两条术语逐字进 Global Constraints。用户故事 54/55/56 分别由 Task 3/Task 5+6/Task 4+5+6+7 覆盖。无遗漏。
2. **占位符扫描**：全文无 TBD/TODO/「适当处理」类占位；所有代码步骤给出完整代码块（Task 4 五方法接口、refuseIdentity、sentinel 位置、types.JSON 构造、import 块、Task 6 的 fake 扩展四处均为完整最终代码，无「见上文修正」式引用）。
3. **类型一致性**：`TaskComplianceStorePort` 七方法与 `repository.TaskComplianceStore` 七方法逐字对齐（GetTaskPolicy/UpsertTaskPolicy/TaskMetadataFacts/OpenComplianceAccess/ActiveComplianceAccess/ListTaskMessages/PurgeTask）；`TaskComplianceManager` 在 Task 4 为五方法（与 Task 3 后的 `*service.TaskComplianceService` 匹配）、Task 6 扩为六方法（此时 PurgeTask 已存在，编译成立）；Task 5 追加的 `AllowsTaskDeletion` 不入 handler 接口，仅作 `session.TaskDeletionGuard` 消费；`types.TaskMetadataView{Metadata, Policy}` 与测试断言 `view.Metadata.Title` 一致；`TaskPurgeReceipt{TaskID, PurgedAt}` 在 Task 4 fake/Task 6 实现/Task 7 断言中一致；sentinel 统一 `types.ErrTaskComplianceNotFound`（Task 1 定义于 types，store/service/测试三方同源）；`MaxComplianceAccessTTL` 在 Task 3 定义、Task 4 handler 的 1..168 小时上限换算（7*24=168）与之一致；迁移编号全文统一 000193/000114（Task 0 去重后 000192/000113 归 agent_adoption_variants）。
4. **Review Focus**：五条均标注归属测试且测试代码已写入对应任务（1→Task 7 两段 404；2→Task 2 `TestOpenAndActiveComplianceAccessWindow` 的 stale 断言+Task 3 403+Task 7 时钟推进 73h；3→Task 3 fail-closed 三连拒；4→Task 6 audit 计数只增+Task 7 finalAudits 断言；5→Task 5 batch/delete_all/clear 三分支 409）。
