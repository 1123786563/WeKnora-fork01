# Wave 14 — T12 Web 子任务：岗位差异展示与历史可访问（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** T12 后端已集成（Wave 13，5f32c77b8，第 4 轮复审通过）；你交付 Web 面并完成 T12 的 Web E2E 验收——T12 verified 的最后缺口，也是 **#140 最后一个 Web 子任务**。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t12-web/WeKnora-fork01 --detach 5f32c77b8
cd /Users/wuyongjun/.codex/worktrees/issue-140-t12-web/WeKnora-fork01
```
BASE = `5f32c77b8`。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 12 Web 部分（verbatim）

- Step 3：为 Web `reconciliation.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；`pnpm typecheck:web && pnpm test:web` 预期 GREEN。（路径按现行布局并入 OpportunityPage 或新差异面板——最小侵入选择并报告。）
- 原始证据（Web 部分）：**"Web 展示变化前后差异且历史可访问"**。
- 验收（Web 可观察部分）：仅有充分证据才合并（合并/并列状态呈现）；不确定重复**并列**展示；**所有原始链接和检查时间保留可见**；**过期、下架和要求变化显式标注**；**旧申请仍展示旧快照**；已接入来源与覆盖城市可查。
- 失败处理：来源检查失败保留最后成功观察并**显示陈旧时间**。

## 3. 后端 API 面（集成 HEAD 实核冻结）

`internal/router/routes_career.go:26-28`：
- `GET /api/v1/career/opportunities/:opportunityId/reconciliations` → `h.OpportunityReconciliationsHandler`（该岗位的合并/并列/状态标注历史）
- `POST /api/v1/career/opportunities/reconcile` → `h.ReconcileOpportunities`（发起对账；含 conflictingBatches 显式披露字段）
- `GET /api/v1/career/reconciliations/receipt?requestId=...` → `h.ReconciliationReceipt`
请求/响应 JSON 以 `internal/modules/career/reconciliation.go` 类型为最终依据（先读再写客户端）。既有 api-client 尚无 reconciliation 方法——你新增，严格解码遵循既有模式。

## 4. Files（所有权）

`apps/web/src/caree/`（OpportunityPage 差异/对账区或独立面板 + 测试 + css）；modify `packages/api-client/src/career.ts` + 聚焦测试（三方法）。不改后端、不改 career-core 合同（缺口如实报告）。

## 5. 行为要求（RED 测试先行）

- 岗位详情呈现：合并状态（充分证据合并）/ **不确定重复并列**（疑似重复双条可见）；每条原始链接与检查时间完整可查。
- **变化前后差异**（E2E 关键）：JD 更新/要求变化/过期/下架的**前后对比视图**（旧快照 vs 新观察差异可读）；**历史可访问**（旧快照永久可打开）。
- 旧申请仍展示旧快照（申请详情指向 pinned 旧版——与 T14 语义一致的 UI 呈现）。
- 覆盖说明：已接入来源与实际覆盖城市可查（空清单如实呈现）。
- 来源检查失败：最后成功观察保留 + **陈旧时间标注**（stale 可见）。
- 未知回执恢复/revision 冲突/跨租户错误态；conflictingBatches 披露呈现。

## 6. 验证（全跑附原始输出）

```bash
pnpm typecheck:web
pnpm test:web          # 全量 node26 终验口径；须全绿 0 cancelled（当前基线 2478/2478；跑前 ps 清理孤儿 runner；若环境 CPU 争用致超时，先记录 ps 取证再重试——Wave 13 先例）
pnpm build:web
git diff --check
```
**浏览器 E2E（T12 verified 关键验收）**：本地 Lite 服务器（复现提纲见 `docs/plans/issue-140/task-3-live-http-validation.md`；**本轮端口 57826**）。真实浏览器：登录 → 构造双来源同岗（API/fixture，声明层级）→ 对账 → 并列/合并状态 → **JD 更新→变化前后差异展示** → 历史快照可访问 → 旧申请展示旧快照 → 覆盖清单 → stale 场景。截图/记录入报告。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- Web 使用 WeKnora TDesign 浅色主题与 `#07c05f` 品牌色；不确定重复不得静默合并。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t12-web/` 下写 `task-2-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、全量验证输出、浏览器 E2E 证据、已知局限。COMMIT：`feat(web): show job change diffs with accessible history`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T12 verified 由主控裁决。
