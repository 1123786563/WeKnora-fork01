# Wave 12 — T20 Web 子任务：站内待办收件箱（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** T20 后端已集成（Wave 11，14b81d24c，评审通过——1 medium 并发缺口可自愈+2 low）；你交付 Web 面并完成 T20 的 Web E2E 验收——T20 verified 的最后缺口。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t20-web/WeKnora-fork01 --detach 14b81d24c
cd /Users/wuyongjun/.codex/worktrees/issue-140-t20-web/WeKnora-fork01
```
BASE = `14b81d24c`。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 20 Web 部分（verbatim）

- Step 3：为 Web `reminder.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；`pnpm typecheck:web && pnpm test:web` 预期 GREEN。（路径按现行布局为 `apps/web/src/career/InboxPage.tsx`——沿用命名先例。）
- 原始证据（Web 部分）：**"Web E2E 从待办进入权威申请"**。
- 验收（Web 可观察部分）：同一机会和事件只一条待办；站内记录权威；**通知正文无公司、岗位、面试细节**（隐私文案呈现）；取消订阅后不再发送、已存在待办仍可读取。
- 失败处理：通知送达失败不丢站内事实，也不将推送视为状态更新。

## 3. 后端 API 面（集成 HEAD 实核冻结）

`internal/router/routes_career.go:60-62`：
- `POST /api/v1/career/reminders` → `h.SetReminderHandler`（set_reminder：订阅/退订/提醒设置）
- `GET /api/v1/career/reminders` → `h.ListRemindersHandler`（待办列表+订阅状态）
- `GET /api/v1/career/reminders/receipt?requestId=...` → `h.ReminderReceiptHandler`
JSON 以 `internal/modules/career/reminder.go` 类型为最终依据（隐私文案冻结模板/待办去重语义以后端冻结为准）。既有 api-client 尚无 reminders 方法——你新增，严格解码遵循既有模式。

## 4. Files（所有权）

`apps/web/src/caree/`（InboxPage + 测试 + css；CareerPage/导航最小入口）；modify `packages/api-client/src/career.ts` + 聚焦测试（三方法）。不改后端、不改 career-core 合同（缺口如实报告）。

## 5. 行为要求（RED 测试先行）

- 收件箱：待办列表（隐私安全文案：**不出现公司/岗位/面试细节**——显示后端冻结的泛化正文）；每条待办可**进入权威申请/事实页**（E2E 关键路径：待办→点击→权威申请详情）。
- 同一机会/事件只一条（重复触发不出现两条的 UI 呈现）；站内权威语义（推送仅提醒——UI 呈现"推送只是提醒，以站内为准"）。
- 订阅管理：取消订阅后推送停止（状态可见）；**已存在待办仍可读取**（退订后列表仍在）。
- 失败/权限/未知回执恢复态；revision 冲突；跨租户错误态。

## 6. 验证（全跑附原始输出）

```bash
pnpm typecheck:web
pnpm test:web          # 全量 node26 终验口径；须全绿 0 cancelled（当前基线 2459/2459；跑前 ps 清理孤儿 runner——Wave 3-11 先例）
pnpm build:web
git diff --check
```
**浏览器 E2E（T20 verified 关键验收）**：本地 Lite 服务器（复现提纲见 `docs/plans/issue-140/task-3-live-http-validation.md`；**本轮端口 57821**）。真实浏览器：登录 → 构造事件/发现待办（API 链路，声明层级）→ 收件箱显示（隐私正文核对）→ **从待办进入权威申请** → 重复触发不二条 → 取消订阅 → 待办仍可读 → 推送失败场景（fixture/seam，声明）站内事实不丢 → 跨租户拒绝。截图/记录入报告。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- Web 使用 WeKnora TDesign 浅色主题与 `#07c05f` 品牌色；通知正文无公司、岗位、面试细节。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t20-web/` 下写 `task-2-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、全量验证输出、浏览器 E2E 证据（端口、截图清单、隐私正文核对）、已知局限（后端 1 medium+2 low 转述）。COMMIT：`feat(web): read privacy-safe inbox todos`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T20 verified 由主控裁决。
