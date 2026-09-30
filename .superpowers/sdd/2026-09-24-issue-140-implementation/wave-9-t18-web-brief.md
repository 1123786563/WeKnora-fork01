# Wave 9 — T18 Web 子任务：投递确认与版本回看（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** T18 后端已集成（Wave 8，10e2f30da，评审通过——4 low 不阻塞）；你交付 Web 面并完成 T18 的 Web E2E 验收——T18 verified 的最后缺口。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t18-web/WeKnora-fork01 --detach 10e2f30da
cd /Users/wuyongjun/.codex/worktrees/issue-140-t18-web/WeKnora-fork01
```
BASE = `10e2f30da`。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 18 Web 部分（verbatim）

- Step 3：为 Web `submission.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；`pnpm typecheck:web && pnpm test:web` 预期 GREEN。（路径按现行布局并入 ApplicationPage/ProgressPage 或新 SubmissionPanel——实现者按最小侵入选择并报告。）
- 原始证据（Web 部分）：**"Web E2E 记录投递并回看版本及时间线"**。
- 验收（Web 可观察部分）：系统不点击外部提交或自动发信；实际版本可选择已验证版或**未知时明确记录未确认**；投递事件绑定渠道、时间、版本或未知标记；重复确认不生成第二次投递记录。
- 失败处理：**外部提交是否成功由用户确认；产品不得从点击下载推断投递**。

## 3. 后端 API 面（集成 HEAD 实核冻结）

`internal/router/routes_career.go:54-56`：
- `POST /api/v1/career/applications/:applicationId/submissions` → `h.RecordSubmission`（渠道枚举 email/web/other + 声明时间 + submittable export 绑定或显式未知标记）
- `GET /api/v1/career/applications/:applicationId/submissions` → `h.ApplicationSubmissions`
- `GET /api/v1/career/submissions/receipt?requestId=...` → `h.SubmissionReceiptHandler`
请求/响应 JSON 以 `internal/modules/career/submission.go` 类型为最终依据（先读再写客户端；一 application 一记录、重复确认 typed conflict 语义以后端冻结为准）。既有 api-client 尚无 submissions 方法——你新增，严格解码遵循 `packages/api-client/src/career.ts` 既有模式。

## 4. Files（所有权）

`apps/web/src/career/`（ApplicationPage 投递区或独立面板 + 测试 + css）；modify `packages/api-client/src/career.ts` + 聚焦测试（三方法）。不改后端、不改 career-core 合同（缺口如实报告）。

## 5. 行为要求（RED 测试先行）

- 投递确认表单：渠道选择（封闭枚举）、声明时间、版本选择（列出 submittable 版本）或**显式"未知版本"**；文案明确"由本人确认外部投递结果，系统不代投"。
- 记录成功 → 投递记录呈现在申请时间线（与 ProgressPage 时间线并存或衔接，报告冻结）；**回看版本**：点击版本引用可打开对应材料版本（MaterialPage 既有能力）。
- 重复确认 → typed conflict 呈现（"已有投递记录"），不产生第二条；未知回执原 requestId 对账恢复；revision 冲突显示当前 revision；跨租户错误态。
- **界面绝无"从下载推断已投递"的暗示或自动化**（下载按钮与投递确认明确分离）。

## 6. 验证（全跑附原始输出）

```bash
pnpm typecheck:web
pnpm test:web          # 全量 node26 终验口径；须全绿 0 cancelled（当前基线 2430/2430；跑前 ps 清理孤儿 runner——Wave 3-8 先例）
pnpm build:web
git diff --check
```
**浏览器 E2E（T18 verified 关键验收）**：本地 Lite 服务器（复现提纲见 `docs/plans/issue-140/task-3-live-http-validation.md`；**本轮端口 57815**）。真实浏览器：登录 → 申请/材料/发布链路或 fixture（声明层级）→ 记录投递（渠道/时间/版本）→ 时间线回看 → **回看绑定的材料版本** → 未知版本显式 unconfirmed 呈现 → 重复确认被拒 → 未知回执恢复。截图/记录入报告。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- Web 使用 WeKnora TDesign 浅色主题与 `#07c05f` 品牌色。
- 外部提交是否成功由用户确认；产品不得从点击下载推断投递；不自动投递/发信。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t18-web/` 下写 `task-2-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、全量验证输出、浏览器 E2E 证据（端口、截图清单）、已知局限（后端 4 low 转述）。COMMIT：`feat(web): confirm submissions with bound versions`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T18 verified 由主控裁决。
