# Wave 6 — T17 Web 子任务：申请进展时间线（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** T17 后端已集成（Wave 5，4b09af298，第 1 轮评审通过——3 low 不阻塞）；你交付 Web 面并完成 T17 的 Web E2E 验收——T17 verified 的最后缺口。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t17-web/WeKnora-fork01 --detach 4b09af298
cd /Users/wuyongjun/.codex/worktrees/issue-140-t17-web/WeKnora-fork01
```
BASE = `4b09af298`。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 17 Web 部分（verbatim）

- Step 3：为 Web `progress.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；`pnpm typecheck:web && pnpm test:web` 预期 GREEN。（路径按现行布局为 `apps/web/src/career/ProgressPage.tsx` 风格——沿用 OpportunityPage/ApplicationPage/MaterialPage 命名先例。）
- 原始证据（Web 部分）：**"Web E2E 录入面试后筛选并查看历史"**。
- 验收（Web 可观察部分）：事件追加保存；**纠错追加更正事件而非覆盖原记录**（历史中原文+更正都可见）；当前阶段由已确认事件投影，**重新打开结果一致**；同一请求重放不出现第二个事件；事件来源和确认者可追溯；跨申请记录不串联。
- 失败处理：**写入结果未知先查回执，不用新 ID 盲目重试**。

## 3. 后端 API 面（集成 HEAD 实核冻结）

`internal/router/routes_career.go:42-45`：
- `POST /api/v1/career/applications/:applicationId/progress` → `h.AppendProgress`（追加事件）
- `POST /api/v1/career/applications/:applicationId/progress/correct` → `h.CorrectProgress`（纠错：追加更正事件）
- `GET /api/v1/career/applications/:applicationId/progress` → `h.ApplicationProgress`（事件历史 + 当前阶段投影）
- `GET /api/v1/career/progress/receipt?requestId=...` → `h.ProgressReceiptHandler`
请求/响应 JSON 以 `internal/modules/career/progress.go` 类型为最终依据（先读再写客户端；阶段投影为 7 阶段封闭词汇、更正就地替换语义——报告冻结的枚举不得发明新值）。既有 api-client 尚无 progress 方法——你新增对应方法，严格解码遵循 `packages/api-client/src/career.ts` 既有模式。

## 4. Files（所有权）

create `apps/web/src/career/ProgressPage.tsx` + 测试 + 本地 css；modify `packages/api-client/src/career.ts` + 聚焦测试；最小入口接线（ApplicationPage 详情进入该申请的进展时间线，不破坏既有页面）。不改后端、不改 career-core 合同（缺口如实报告）。

## 5. 行为要求（RED 测试先行）

- 从申请详情进入进展时间线：事件按序展示（时间、来源、确认者、内容）；当前阶段投影醒目呈现。
- 录入事件（如"面试"）→ 阶段投影更新；**筛选/刷新/重新打开后阶段一致**（确定性投影的 UI 验证）。
- 纠错：对既有事件发起更正 → 历史显示原事件 + 更正事件（原文不被覆盖、两者可见），投影采用更正语义。
- 未知回执：原 requestId 查回执恢复（不自动换新 ID）；revision 冲突显示当前 revision；跨租户 403/404 清晰错误态；跨申请不串联（换申请只看到自己的事件）。
- 同请求重放 UI 不产生第二个事件的呈现（replay 返回原收据）。

## 6. 验证（全跑附原始输出）

```bash
pnpm typecheck:web
pnpm test:web          # 全量 node26 终验口径；须全绿 0 cancelled（当前基线 2401/2401；跑前 ps 清理孤儿 runner——Wave 3/4/5 先例）
pnpm build:web
git diff --check
```
**浏览器 E2E（T17 verified 关键验收）**：本地 Lite 服务器（复现提纲见 `docs/plans/issue-140/task-3-live-http-validation.md`；**本轮端口 57812**）。真实浏览器：登录 → 申请详情（fixture 构造或既有链路，声明层级）→ 录入"面试"事件 → 筛选/刷新查看阶段与历史一致 → 对早期事件纠错 → 原文+更正并见 → 投影更新 → 跨租户拒绝 → 未知回执恢复路径。截图/记录入报告。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- Web 使用 WeKnora TDesign 浅色主题与 `#07c05f` 品牌色。
- 不自动投递、不发送邮件、不从点击下载推断进展。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t17-web/` 下写 `task-2-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、全量验证输出、浏览器 E2E 证据（端口、截图清单）、已知局限（后端 3 low 转述）。COMMIT：`feat(web): track application progress timeline`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T17 verified 由主控裁决。
