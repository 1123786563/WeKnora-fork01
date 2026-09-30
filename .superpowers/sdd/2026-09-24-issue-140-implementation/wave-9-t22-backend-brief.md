# Wave 9 — T22 后端子任务：求职数据导出与完整删除（implement）

你是 backend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 本任务只做后端；Web 子任务等你的合同 reviewed+integrated 后另行派发。T22 无专用计划——本简报即权威任务书。**本波唯一的 career 后端任务**（office.go/handler.go/routes_career.go/career_migration_test.go 独占）。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t22-export/WeKnora-fork01 --detach 10e2f30da
cd /Users/wuyongjun/.codex/worktrees/issue-140-t22-export/WeKnora-fork01
```
BASE = `10e2f30da`（当前集成 HEAD）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 22 文本（verbatim）

### Task 22: T22/#162 求职数据导出与完整删除
**Depends:** #158（T16 verified）、#157（T17 verified）。**Produces:** `CareerRemote.act({kind: "export_career", payload}, requestId, expectedRevision)`（封闭 intent 集另含 `delete_career`）。
**验收：** **导出包含档案、原岗位快照、申请事件和材料版本**；**删除前说明空间内与外部平台资料的边界**；**删除使旧 Task、Artifact 授权和客户端缓存不可再访问**；若保留策略要求延迟或例外，**显示范围与状态**
**原始证据：** Career Office/Identity/Workbench 联合合同测试导出完整性和旧链接失效。**失败处理：** **局部删除失败保留可恢复状态与审计，不声称已完全删除。**
（注：主计划 `internal/modules/career/service/` 为旧稿路径，实际在 `internal/modules/career/` 包根——沿用先例。）

## 3. 设计约定（遵循既有先例，结构由你实现并在报告冻结）

**Files**：create `internal/modules/career/career_export.go`、`career_export_test.go`；modify `internal/modules/career/office.go`、`handler.go`、`handler_test.go`（聚焦）；modify `internal/router/routes_career.go` + 聚焦 route tests；create `migrations/versioned/000205_career_exports_deletions.up/.down.sql`、`migrations/sqlite/000126_career_exports_deletions.up/.down.sql`；extend `internal/database/career_migration_test.go`；更新 architectureguard Career 清单。

**迁移编号（本轮唯一分配）**：versioned **000205** / sqlite **000126**（调度员实核空闲：目录现有最大 204/125）。

**Consumes**：Career 全域已集成表（facts/opportunities+snapshots+observations/applications/progress_events/materials+versions/exports/submissions/searches/rules）；T14 Workbench projection（`interfaces.CareerApplicationTaskLinker` + `career_application_tasks` 表）；T16 exports 授权（signed-url/撤销）；Workbench artifact 授权语义（T04 先例）。

**行为要点**：
1. **export_career**：产出**完整导出包**（结构化归档：档案事实、原岗位快照、申请事件（progress）、材料版本、投递记录），owner-scoped；导出为异步/两段式或同步按 house 取舍冻结；导出文件 digest 可校验。request ID 幂等。
2. **delete_career**：
   - **删除前边界说明**：删除请求先返回/呈现"空间内数据 vs 外部平台资料"边界清单（系统只能删本空间数据；外部平台投递/邮件等不可撤回——文案性边界由 API 返回结构化清单，前端呈现）。
   - **完整删除**：Career 域数据 + **Workbench 侧申请 Task 投影**（经窄接口或由本任务新增的删除 seam——**不得直接写 Workbench 表**，扩 `internal/types/interfaces` 加删除端口并在 container 接线，参照 T14 linker 先例）+ **使旧 Artifact/导出授权失效**（撤销既有 signed grants/exports——经对应撤销 API/接口，不越权直写）。
   - **延迟/例外保留策略**：若存在法定/技术保留（如审计行），显示保留范围与状态（保留行清单+原因），不静默保留。
   - **部分失败**：任一子删除失败 → 整体状态 `deleting/partial` + 审计记录 + 可恢复（重试同 request ID 续删）；**绝不声称已完全删除**。
3. **客户端缓存不可再访问**：删除后 Career scope 无效化（space epoch/revision 语义，changes 流呈现 deletion 事件——消费端失效由既有 Desk 语义承接，报告说明链路）。
4. **house 语义**：request ID replay/conflict、expectedRevision、tenant/owner scope、未知恢复。

**命名 RED 测试**：
```go
func TestExportCareerIncludesProfileSnapshotsEventsAndMaterialVersions(t *testing.T)
func TestExportCareerIsIdempotentByRequestId(t *testing.T)
func TestDeleteCareerExplainsInSpaceVersusExternalBoundary(t *testing.T)
func TestDeleteCareerRemovesCareerDataAndWorkbenchProjection(t *testing.T)
func TestDeleteCareerRevokesOldExportAndArtifactGrants(t *testing.T)
func TestDeleteCareerDisclosesRetentionScopeAndStatus(t *testing.T)
func TestDeleteCareerPartialFailureKeepsRecoverableStateAndAudit(t *testing.T)
func TestDeleteCareerInvalidatesClientVisibleScopeOrChanges(t *testing.T)
func TestExportDeletionScopeRejectsOtherTenantAndOwner(t *testing.T)
func TestCareerExportMigrationUpAndDown(t *testing.T)
```

## 4. 验证（全跑附原始输出）

```bash
gofmt -w internal/modules/career internal/types/interfaces internal/container internal/router
go test ./internal/modules/career/... -count=1
go test ./internal/modules/workbench/service/workbench/... -count=1
go test ./internal/database/... -count=1
go test ./internal/router/... -count=1
go test ./tools/architectureguard/... -count=1
git diff --check
```
architectureguard：当前基线 **612/681**（T18 后），你的新路由按精确新值更新。
**隔离数据库强制**：删除测试只允许使用临时/内存 SQLite（task 级 DB），绝不触碰任何持久库（全局约束明载"删除测试使用隔离数据库"）。

## 5. 全局约束（verbatim 摘录，违反即 Spec FAIL）

- 删除测试使用隔离数据库。
- Career 不直接写 Workbench 表——经窄接口（T14 linker 同型先例）。
- 只在 assigned isolated Worktree 内改本任务所有权文件（本任务所有权含 internal/types/interfaces 新删除端口与 container 接线）；local commits authorized, no push/merge/deploy/GitHub action。
- 所有写入使用 request ID 与 expected revision；scope 从认证上下文派生；数据库查询一律参数绑定。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 6. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t22-export/` 下写 `task-1-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、接口冻结说明（导出归档结构+删除边界清单结构）、保留策略取舍、已知局限。COMMIT：`feat(career): export and completely delete job search data`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成。
