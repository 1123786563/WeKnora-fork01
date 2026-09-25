# Wave 12 — T28：微信小程序申请时间线与按需准备（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** 依赖均就绪：#156=T19 verified（面试准备合同）、#166=T26 verified（小程序申请/材料/投递基础设施）。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t28-miniprogram/WeKnora-fork01 --detach 14b81d24c
cd /Users/wuyongjun/.codex/worktrees/issue-140-t28-miniprogram/WeKnora-fork01
```
BASE = `14b81d24c`（含 T19/T26/T32 已集成的小程序 Career 能力）。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 28 文本（verbatim）

### Task 28: T28/#169 微信小程序（Taro 4 + TDesign Miniprogram）申请时间线与按需准备
**Depends:** #156、#166。**Files:** `apps/miniprogram/src/career/progress-preparation.tsx`、`progress-preparation.test.tsx`、`adapters/career-platform.ts`。
**验收：** 可见小程序控件使用 TDesign Miniprogram 或记录明确的原生能力例外；实际开发工具与真机验证；**事件列表与其他端共享权威顺序**；**跨端更正后旧事件仍可追溯**；**准备内容引用确定投递版，未知时提示**；**切换账号不显示前一用户缓存**
**原始证据：** Career Desk 事件与 scope 测试；真实环境录入、修改并查看面试准备。**失败处理：** **网络断开只保留本地草稿，不静默改申请状态。**
- Step 1-4 框架与验证命令同小程序先例（test/typecheck/build:weapp 三命令 RED→GREEN；真实 DevTools 验证；未通过真实设备门槛则保留 blocked）。

## 3. 可消费的服务端合同（集成 HEAD 冻结）

- **进展时间线**（T17）：`POST/GET /applications/:applicationId/progress`（append/correct/history+7 阶段确定性投影）+ `GET /progress/receipt`（routes_career.go:46-49 一带，以实际为准）；纠错=追加更正事件原事件保留。
- **面试准备**（T19）：`POST/GET /applications/:applicationId/preparations` + `GET /preparations/receipt`（routes_career.go:57-59）；版本锚定实际投递版，未知返回 typed 提示态（HTTP 409 preparation_version_unknown）；草稿物化 materials 域可修订。
JSON 以 `internal/modules/career/{progress,preparation}.go` 为最终依据。小程序侧复用 T26 career service/adapter 模式扩展（`adapters/career-platform.ts` 为共享文件——本任务为唯一所有者）。

## 4. 工程事实（沿用）

TDesign 构建机制/事件绑定/DevTools 验证模式（T06/T24 修复链）；typecheck 13 基线豁免；T24/T26/T32 教训为验收前置（可区分截图/无死代码提示/404 与未知分支恢复态）。

## 5. 行为要求（RED 测试先行）

- **时间线**：事件按服务端权威顺序呈现（时间/来源/确认者/内容+7 阶段投影）；**跨端更正**：在 Web（或 API）对小程序已见事件发起更正后，小程序刷新显示**原文+更正并见**（旧事件可追溯）。
- **按需准备**：从时间线（如面试事件）进入准备；准备内容**引用确定投递版**（锚定可见），未确认投递 → **提示态**（与 Web 同语义，绝不静默用最新版）；录入、**修改**并查看准备草稿（真实环境关键路径）。
- **账号切换**：切换账号（登出→另一账号登录）**不显示前一用户缓存**（storage 按 scope 隔离/清理，测试断言）。
- **断网**：断网只保留本地草稿（可再编辑），**不静默改申请状态**（重联网不自动提交——显式同步）。
- 冲突回执/未知对账/空间切换三 seam 可观察测试。

## 6. 验证（全跑附原始输出）

```bash
pnpm --filter @weknora/miniprogram test       # 132 基线+新增全绿
pnpm --filter @weknora/miniprogram typecheck  # 13 基线外零新增
pnpm --filter @weknora/miniprogram build:weapp
git diff --check
```
**真实 DevTools 验证（本轮端口 57822，隔离临时 DB）**：官方构建加载→登录→申请时间线→录入事件→（经 API 模拟跨端）更正→原文+更正并见→进入准备→录入/修改草稿→未知投递提示态→账号切换缓存隔离→断网草稿保留→跨租户拒绝。截图逐张可区分；真机无设备单列 blocked。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- 不长期缓存 URL/凭据；账号切换缓存隔离；凭据 disposable 结束清理。
- 测试端口（57822）与构建目录隔离。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t28-miniprogram/` 下写 `task-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、三命令输出、DevTools 各步证据（可区分截图+跨端更正+账号切换验证）、原生例外清单、已知局限（真机 blocked）。COMMIT 按逻辑分批。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T28 verified 由主控裁决。
