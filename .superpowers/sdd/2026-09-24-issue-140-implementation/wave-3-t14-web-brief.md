# Wave 3 — T14 Web 子任务：从岗位卡打开独立申请（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** T14 后端（子任务1+2）已集成（Wave 2，d88cd513a），四条 applications 路由可用；你交付 Web 面并完成 T14 的 Web E2E 验收，这是 T14 verified 的最后缺口。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t14-web/WeKnora-fork01 --detach d88cd513a
cd /Users/wuyongjun/.codex/worktrees/issue-140-t14-web/WeKnora-fork01
```
BASE = `d88cd513a`。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 14 Web 部分（verbatim）

- Step 3：为 Web `application.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。
- 原始证据（Web 部分）：**"Web E2E 从岗位卡打开独立申请"**。
- 失败处理：**"Task 创建失败保留可恢复申请，不将未关联状态显示为就绪"**。
- 验收（Web 可观察部分）：申请固定所用岗位快照、档案版本与资格评估；硬条件不符可显式继续，但**警示常驻**且不计合格申请指标；跨模块 Task 建立未知时保留 **linking 状态**并用原请求 ID 对账；同岗不同批次可分别申请，重复请求不创建第二个 Task。

## 3. 后端 API 面（集成 HEAD 实核冻结）

`internal/router/routes_career.go:28-31`：
- `POST /api/v1/career/applications` → body 为 `CreateApplicationInput`：`{requestId, opportunityId, snapshotId, evaluationId, batchIdentity, continueDespiteHardFailure, expectedRevision}`（json tag 见 `internal/modules/career/application.go`，以其为最终依据）
- `GET /api/v1/career/applications/receipt?requestId=...`
- `GET /api/v1/career/applications/:applicationId`
- `POST /api/v1/career/applications/link/reconcile` body `{"requestId":"..."}`
响应 `ApplicationReceipt`：`{applicationId, requestId, linkState, taskId?, runId?, qualified, warning?, pinnedEvidence}`，`pinnedEvidence={opportunityId, snapshotId, evaluationId, profileRevision, evaluationStatus, batchIdentity}`；`linkState ∈ linking | ready | link_failed`（后端冻结，前端不得发明新值）。既有 api-client（`packages/api-client/src/career.ts`）尚无 applications 方法——你新增 `createApplication`/`applicationReceipt`/`application`/`reconcileApplicationLink`，严格解码（strict decode）遵循该文件既有模式。

## 4. Files（所有权）

create `apps/web/src/career/ApplicationPage.tsx` + 测试 + 本地 css（可复用 opportunity.css 模式）；modify `apps/web/src/career/OpportunityPage.tsx`（岗位卡增加"创建申请"入口，不改其既有行为）与对应测试；modify `packages/api-client/src/career.ts` + 聚焦测试。不改后端、不改 career-core 合同（若发现合同缺口如实报告，不擅改）。

## 5. Step 0（单独 commit，先做）：偿还 T09-Web 评审遗留 F2——api-client 空 rawText 证据解码被拒

T09-Web 实现者如实报告：T09 后端按计划为**失败观察**落"空文本快照"（SHA-256(空字节)、needs_review），而 api-client 的 opportunity evidence 严格解码器拒绝空 `rawText`，导致证据页打不开。裁决（调度员 Wave 3）：后端行为符合 T09 计划（失败观察落空文本快照是设计使然），修解码器——允许 `rawText` 为空字符串，其余字段严格性不变；先写失败测试（空 rawText 证据页可解码）再修。单独 commit `fix(api-client): decode empty rawText evidence pages`。

## 6. 行为要求（RED 测试先行）

- 岗位卡 → 创建申请：展示并固定 所用岗位快照/档案版本/资格评估（pinnedEvidence 可见）；revision 冲突显示当前 revision 并可恢复。
- 硬条件不符（evaluationStatus=ineligible）：默认阻断 + 常驻警示；勾选"显式继续"后可提交，警示保持可见，`qualified=false` 可见。
- linkState 呈现：`linking` 显示"关联中/可恢复"（不显示为就绪），提供用原 requestId 的 reconcile 动作；`ready` 显示 TaskID；`link_failed` 显示失败与重试。结果未知（网络/超时）→ 先 `applicationReceipt(requestId)` 恢复，不自动换新 ID。
- 同岗不同批次分别申请；重复请求不产生第二个 Task（replay 返回原收据的 UI 呈现）。
- 跨 User/Tenant 403/404 → 清晰错误态。

## 7. 验证（全跑附原始输出）

```bash
pnpm typecheck:web
pnpm test:web          # 全量，须全绿 0 cancelled（当前基线 2363/2363）
pnpm build:web
git diff --check
```
**浏览器 E2E（T14 verified 的关键验收）**：本地 Lite 服务器（复现提纲见 `docs/plans/issue-140/task-3-live-http-validation.md` 与 `task-6-live-validation.md`；**本轮端口 57806**）。真实浏览器：登录 → 建档/导入岗位/评估（或用 fixture 直插 opportunity+evaluation 行——如实声明层级）→ 岗位卡点击"创建申请" → 硬条件警示与显式继续 → linking→ready 迁移（可借助 fixture 控制 linker 结果）→ 刷新后申请仍可读 → 跨租户拒绝。截图/记录入报告。

## 8. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- Web 使用 WeKnora TDesign 浅色主题与 `#07c05f` 品牌色。
- 所有写入使用 request ID 与 expected revision；同一 request ID 内容变化拒绝；空间切换后旧响应失效（切空间 abort 并清缓存，遵循 Desk 既有语义）。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 9. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t14-web/` 下写 `task-3-report.md`（mkdir -p）：BASE/HEAD、Step 0 与主任务 RED/GREEN 证据、全量验证输出、浏览器 E2E 证据（端口、截图清单、观测记录）、已知局限。COMMIT：Step 0 与主任务分开（主任务 `feat(web): open per-application tasks from opportunities`）。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T14 verified 由主控在本子任务 reviewed+integrated 后裁决。
