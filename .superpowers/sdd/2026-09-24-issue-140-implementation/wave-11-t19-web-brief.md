# Wave 11 — T19 Web 子任务：面试准备与来源修订（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** T19 后端已集成（Wave 10，ff5b2e843，评审通过+F1 已由集成员偿还）；你交付 Web 面并完成 T19 的 Web E2E 验收——T19 verified 的最后缺口。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t19-web/WeKnora-fork01 --detach ff5b2e843
cd /Users/wuyongjun/.codex/worktrees/issue-140-t19-web/WeKnora-fork01
```
BASE = `ff5b2e843`。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 19 Web 部分（verbatim）

- Step 3：为 Web `preparation.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；`pnpm typecheck:web && pnpm test:web` 预期 GREEN。（路径按现行布局为 `apps/web/src/career/PreparationPage.tsx`——沿用命名先例。）
- 原始证据（Web 部分）：**"Web E2E 查看来源并修订草稿"**。
- 验收（Web 可观察部分）：求职信与回答只引用已确认事实和岗位快照；**面试准备优先固定实际投递版本，版本未知必须提示**；**生成结果可审阅、修订且有来源**；不自动发送或替用户承诺事实。
- 失败处理：模型失败保留请求和可恢复状态，**不展示空白成功产物**。

## 3. 后端 API 面（集成 HEAD 实核冻结）

`internal/router/routes_career.go:57-59`：
- `POST /api/v1/career/applications/:applicationId/preparations` → `h.GeneratePreparationHandler`（生成/更新准备草稿；版本锚定：优先实际投递版本，未知返回 typed 提示态）
- `GET /api/v1/career/applications/:applicationId/preparations` → `h.ApplicationPreparations`（列表）
- `GET /api/v1/career/preparations/receipt?requestId=...` → `h.PreparationReceiptHandler`
草稿物化为 materials 域 edit_material 正文（可审阅/修订/带来源链）。JSON 以 `internal/modules/career/preparation.go` + materials 域类型为最终依据（先读再写客户端）。既有 api-client 尚无 preparations 方法——你新增，严格解码遵循既有模式；修订流复用既有 materials 编辑方法。

## 4. Files（所有权）

`apps/web/src/career/`（PreparationPage + 测试 + css；ApplicationPage/ProgressPage 最小入口）；modify `packages/api-client/src/career.ts` + 聚焦测试。不改后端、不改 career-core 合同（缺口如实报告）。

## 5. 行为要求（RED 测试先行）

- 从申请进入准备：生成（或列出既有草稿）；**版本锚定可见**——"基于实际投递版本 V2"；投递未知 → **明确提示态**（引导先记录投递或显式选择未知口径，绝不静默用最新版）。
- 草稿审阅：正文 + **来源链可见**（每条主张/内容可追溯到确认事实/岗位快照/投递版本引用）。
- 修订：编辑草稿→保存（不覆盖既有草稿语义按后端冻结呈现）；**E2E 关键路径：查看来源并修订草稿**。
- 生成失败（seam 注入/网络）：保留请求与可恢复状态（重试同 requestId），**无空白成功产物**；revision 冲突/跨租户错误态；零自动发送控件/暗示。

## 6. 验证（全跑附原始输出）

```bash
pnpm typecheck:web
pnpm test:web          # 全量 node26 终验口径；须全绿 0 cancelled（当前基线 2449/2449；跑前 ps 清理孤儿 runner——Wave 3-10 先例）
pnpm build:web
git diff --check
```
**浏览器 E2E（T19 verified 关键验收）**：本地 Lite 服务器（复现提纲见 `docs/plans/issue-140/task-3-live-http-validation.md`；**本轮端口 57818**）。真实浏览器：登录 → 申请+投递（记录版本 V2）→ 生成准备（锚定 V2 可见）→ **查看来源链** → **修订草稿保存** → 未知投递场景提示态 → 失败保留可恢复 → 跨租户拒绝。截图/记录入报告。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- Web 使用 WeKnora TDesign 浅色主题与 `#07c05f` 品牌色；缺失不得补造；不自动发送。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t19-web/` 下写 `task-2-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、全量验证输出、浏览器 E2E 证据（端口、截图清单）、已知局限。COMMIT：`feat(web): review sourced interview preparations`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T19 verified 由主控裁决。
