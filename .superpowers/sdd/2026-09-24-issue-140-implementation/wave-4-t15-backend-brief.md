# Wave 4 — T15 后端子任务：可信结构化材料与不可变版本（implement）

你是 backend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 本任务只做后端；Web 子任务（material.tsx）等你的合同 reviewed+integrated 后另行派发。T15 无专用计划——本简报即权威任务书，实现约定遵循已集成的 T08-T14 后端先例。**本波唯一的 career 后端任务**（office.go/handler.go/routes_career.go 每波独占），无并发所有者。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t15-material/WeKnora-fork01 --detach 6a21e0df5
cd /Users/wuyongjun/.codex/worktrees/issue-140-t15-material/WeKnora-fork01
```
BASE = `6a21e0df5`（当前集成 HEAD）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 15 文本（verbatim）

### Task 15: T15/#153 可信结构化材料与不可变版本
**Depends:** #147、#155。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Produces:** `CareerRemote.act({kind: "edit_material", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。
**验收：** 生成前冻结岗位与档案版本，主张链接到确认事实；缺失实习、证书、数字不得由模型补造，审阅指出风险；用户确认正文后形成新不可变版本，旧版本可比较；三端共同编辑的是结构化正文，修改后不得覆盖旧投递版
**原始证据：** Career Office 公共 seam 测试覆盖未确认事实、伪造成果、版本冲突。**失败处理：** 生成或审阅失败保留草稿与原因，不发布可投递版本。
（注：主计划写的 `internal/modules/career/service/` 路径与现行布局不符，实际在 `internal/modules/career/` 包根——沿用先例。Step 1-4 框架：RED 服务端公共 seam 测试覆盖验收+request ID+revision+Tenant 隔离 → 最小实现封闭 intent → 验证。）

前置已就绪：#147=T07、#155=T14 均 verified（Wave 3 集成载明）。

## 3. 设计约定（遵循既有先例，结构由你实现并在报告冻结）

**Files**：create `internal/modules/career/material.go`、`material_test.go`；modify `internal/modules/career/office.go`、`handler.go`、`handler_test.go`（聚焦）；modify `internal/router/routes_career.go` + 聚焦 route tests；create `migrations/versioned/000200_career_materials.up/.down.sql`、`migrations/sqlite/000121_career_materials.up/.down.sql`；extend `internal/database/career_migration_test.go`；更新 architectureguard Career 清单。

**迁移编号（本轮唯一分配）**：versioned **000200** / sqlite **000121**（调度员实核空闲：目录现有最大 199/120）。

**HTTP（沿用三件套约定，路由数由你按需定并在报告冻结 JSON 形态）**：材料创建/编辑（draft）、收据查询、材料读取、版本读取/比较（如 `POST /materials`、`GET /materials/receipt`、`GET /materials/:materialId`、`GET /materials/:materialId/versions`、`GET /materials/:materialId/versions/:versionId`——具体由你设计，保持与 opportunities/evaluations/applications/searches 一致风格）。

**行为要点**：
1. **冻结证据**：材料创建时冻结所用 opportunity snapshot 与 profile revision（pinned evidence 模式参照 career_applications 的 ApplicationEvidencePin 先例）；后续档案/岗位变更不回写已冻结材料。
2. **主张链接确认事实**：正文中的每条主张（claim）带事实引用（指向已确认 career fact / 评估 / 快照 ID）；**未确认事实不得进入正文主张**。
3. **不得补造**：缺失的实习、证书、数字（成绩/年限/人数等）绝不生成或虚构——以显式"缺失/待补充"占位并标注 needs_review；"审阅指出风险"= 材料带 review 风险清单（如"该主张引用未确认事实"），服务端校验主张引用有效性。
4. **不可变版本**：用户确认正文（confirm 语义）后形成新不可变版本（版本号自增，正文/结构/冻结证据整体快照）；旧版本永远可读可比较；**任何修改不得覆盖既有版本**（编辑只产生新 draft/新版本）；为 T18 投递绑定预留版本引用语义但不实现投递（T18 专属）。
5. **失败保留**：生成/校验/审阅失败 → 草稿与失败原因持久保留（不发布），typed error 可恢复。
6. **幂等与并发**：request ID exact replay / 变更冲突 / expectedRevision 冲突返回当前 revision / scope 隔离——全部沿用 house 语义（T03/T09/T14 先例）。
7. 结构化正文：单一结构化正文模型（sections/claims 引用），三端共同编辑的就是它；禁止为不同端产生分叉正文。

**命名 RED 测试**：
```go
func TestEditMaterialFreezesOpportunityAndProfileEvidence(t *testing.T)
func TestMaterialClaimsLinkOnlyConfirmedFacts(t *testing.T)
func TestMaterialDoesNotFabricateMissingExperienceCertificatesNumbers(t *testing.T)
func TestConfirmBodyCreatesImmutableVersionAndOldVersionsComparable(t *testing.T)
func TestMaterialEditNeverOverwritesExistingVersions(t *testing.T)
func TestMaterialFailurePreservesDraftAndReason(t *testing.T)
func TestMaterialExactReplayAndChangedIntentConflict(t *testing.T)
func TestMaterialRevisionConflictReturnsCurrentRevision(t *testing.T)
func TestMaterialScopeRejectsOtherTenantAndOwner(t *testing.T)
func TestMaterialMigrationUpAndDown(t *testing.T)
```

## 4. 验证（全跑附原始输出）

```bash
gofmt -w internal/modules/career internal/router
go test ./internal/modules/career/... -count=1
go test ./internal/database/... -count=1
go test ./internal/router/... -count=1
go test ./tools/architectureguard/... -count=1
git diff --check
```
architectureguard：当前基线 **590/659**（T11 后），你的新路由按精确新值更新清单。

## 5. 全局约束（verbatim 摘录，违反即 Spec FAIL）

- 材料由同一结构化正文生成 PDF/DOCX，真实核验后发布不可变版本（T15 交付结构化正文与不可变版本；PDF/DOCX 属 T16，不实现）。
- 上传简历只生成待确认事实（材料主张只引用已确认事实，待确认事实不进正文）。
- 所有写入使用 request ID 与 expected revision；同一 request ID 内容变化拒绝；scope 从认证上下文派生。
- 数据库查询一律参数绑定；只在 assigned isolated Worktree 内改本任务所有权文件；no push/merge/deploy/GitHub action。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 6. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t15-material/` 下写 `task-1-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、接口冻结说明（HTTP+JSON 形态供 Web/移动/小程序消费）、已知局限。COMMIT：`feat(career): version structured materials immutably`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成。
