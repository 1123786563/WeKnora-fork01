# Wave 6 — T16 后端子任务：同版 PDF/DOCX 生成与验证（implement）

你是 backend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 本任务只做后端；Web 子任务（下载 E2E）等你的合同 reviewed+integrated 后另行派发。T16 无专用计划——本简报即权威任务书。**本波唯一的 career 后端任务**（office.go/handler.go/routes_career.go/career_migration_test.go 独占）。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t16-rendering/WeKnora-fork01 --detach 4b09af298
cd /Users/wuyongjun/.codex/worktrees/issue-140-t16-rendering/WeKnora-fork01
```
BASE = `4b09af298`（当前集成 HEAD）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 16 文本（verbatim）

### Task 16: T16/#158 同版 PDF/DOCX 生成与验证
**Depends:** #142（T04 verified）、#153（T15 verified）。**Produces:** `CareerRemote.act({kind: "publish_material", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。
**验收：** **两种文件绑定同一正文摘要与材料版本**；PDF 校验可提取文本与页面版面，DOCX 校验完整与可编辑性；**两种验证均通过后版本才可标记可用于投递**；旧版本保持可下载，删除或撤销后旧授权立即失效
**原始证据：** 真实渲染器验收检查 PDF 文本与页面、DOCX 内容与编辑；Workbench 授权测试。**失败处理：** **单一格式失败保留 staged 状态和错误，不只发布成功的一半为可投递。**
（注：主计划 `internal/modules/career/service/` 为旧稿路径，实际在 `internal/modules/career/` 包根——沿用先例。）

## 3. 设计约定（遵循既有先例，结构由你实现并在报告冻结）

**Files**：create `internal/modules/career/rendering.go`、`rendering_test.go`；modify `internal/modules/career/office.go`、`handler.go`、`handler_test.go`（聚焦）；modify `internal/router/routes_career.go` + 聚焦 route tests；create `migrations/versioned/000202_career_material_exports.up/.down.sql`、`migrations/sqlite/000123_career_material_exports.up/.down.sql`；extend `internal/database/career_migration_test.go`；更新 architectureguard Career 清单。

**迁移编号（本轮唯一分配）**：versioned **000202** / sqlite **000123**（调度员实核空闲：目录现有最大 201/122）。

**Consumes**：T15 `career_materials` 不可变版本（`internal/modules/career/material.go`，七路由已在 routes_career.go:35-41）；存储经既有 local storage / `local://` object key 模式；下载授权语义对齐 T04（固定版本、签发时查授权/撤销态、撤销后立即失效——参考 `docs/plans/issue-140/task-4-live-http-validation.md`）。

**行为要点**：
1. **publish_material**：对指定 material version 渲染 **PDF 与 DOCX 两种文件，来自同一结构化正文**；两文件记录同一 content digest（正文摘要）与 material version 绑定；落 `career_material_exports`（staged → verified/submittable / failed 状态机）。
2. **真实校验**：渲染后立即独立验证——PDF：可提取文本（用与生成独立的解析路径复核文本内容）与页面版面（页数/版面结构）；DOCX：zip 结构完整（archive/zip 校验）+ document.xml 内容对应正文 + 可编辑性（标准 OOXML 可被编辑器打开的结构要件）。**两种验证都通过才标记 submittable**；任一失败 → staged/failed 状态 + 保留错误，**绝不全量发布成功的一半**。
3. **授权失效**：下载经签发式授权（对齐 T04 语义：短时效、绑定 owner/资源、撤销/删除后旧授权立即失效——如复用/对齐 artifact 版本授权模式或 Career 自有同构实现，报告说明取舍）；旧版本导出永远可再取（若 material version 未删除）。
4. **幂等/scope**：request ID replay/conflict、expectedRevision、tenant/owner 隔离——house 语义。
5. **渲染器策略（受约束的实现者决策，报告必须完整披露）**：优先**纯 Go stdlib**（DOCX=archive/zip+encoding/xml；PDF=最小自写 PDF writer 或既有依赖）；go.mod 实勘无现成 PDF/DOCX 写入库（仅 chromedp 间接依赖）。如需新 Go 依赖：先验证模块代理可得性（`go get` 实测），披露依赖与理由并更新 go.mod/go.sum；**不得引入运行时网络服务或外部渲染 API**；不得调用 chromedp 拉起浏览器作为生产路径（冷重/可重复性风险，若论证必要须主控裁决）。渲染确定性：同正文同版本 → 同 digest（内容稳定）。

**命名 RED 测试**：
```go
func TestPublishMaterialBindsSameDigestAndVersionToPDFAndDOCX(t *testing.T)
func TestPublishMaterialVerifiesPDFTextAndLayoutIndependently(t *testing.T)
func TestPublishMaterialVerifiesDOCXCompletenessAndEditability(t *testing.T)
func TestPublishMaterialMarksSubmittableOnlyAfterBothFormatsVerified(t *testing.T)
func TestPublishMaterialSingleFormatFailureStaysStagedWithError(t *testing.T)
func TestPublishMaterialDownloadGrantFailsImmediatelyAfterRevoke(t *testing.T)
func TestPublishMaterialOldVersionsRemainDownloadable(t *testing.T)
func TestPublishMaterialExactReplayAndChangedIntentConflict(t *testing.T)
func TestPublishMaterialScopeRejectsOtherTenantAndOwner(t *testing.T)
func TestPublishMaterialMigrationUpAndDown(t *testing.T)
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
architectureguard：当前基线 **601/670**（T17 后），你的新路由按精确新值更新。
另：真实渲染器验收证据——生成的 PDF/DOCX 样本（fixture 正文）在测试外用独立工具复核（如 `python3 -c` pdf 文本抽取/unzip -l + xmllint 或等价命令行，记录命令与输出入报告）。

## 5. 全局约束（verbatim 摘录，违反即 Spec FAIL）

- 材料由同一结构化正文生成 PDF/DOCX，真实核验后发布不可变版本。
- 只在 assigned isolated Worktree 内改本任务所有权文件；local commits authorized, no push/merge/deploy/GitHub action。
- 所有写入使用 request ID 与 expected revision；scope 从认证上下文派生；数据库查询一律参数绑定。
- 服务器端不发起外网请求（渲染本地完成；URL 校验类约束不适用于本任务）。
- 报告只写事实与实测输出；环境不满足（如依赖不可得）如实 blocked 附证据，绝不伪造。

## 6. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t16-rendering/` 下写 `task-1-report.md`（mkdir -p）：BASE/HEAD、渲染器策略与披露、RED/GREEN 证据、接口冻结说明、独立复核证据（命令+输出）、已知局限。COMMIT：`feat(career): render and verify same-version pdf docx`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成。
