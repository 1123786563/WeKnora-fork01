# Wave 8 — T13 Web 子任务：持续找岗规则页面（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** T13 后端已集成（Wave 7，24708145c，评审通过——4 low 不阻断）；你交付 Web 面并完成 T13 的 Web E2E 验收——T13 verified 的最后缺口。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t13-web/WeKnora-fork01 --detach 24708145c
cd /Users/wuyongjun/.codex/worktrees/issue-140-t13-web/WeKnora-fork01
```
BASE = `24708145c`。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 13 Web 部分（verbatim）

- Step 3：为 Web `search_rule.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；`pnpm typecheck:web && pnpm test:web` 预期 GREEN。（路径按现行布局为 `apps/web/src/career/RulePage.tsx` 或并入 SearchPage——实现者按最小侵入选择并报告。）
- 原始证据（Web 部分）：**"Web E2E 修改规则后显示下次运行计划"**。
- 验收（Web 可观察部分）：未开启不后台运行（UI 无"暗中运行"暗示）；暂停后下一次触发被取消或不再入队（状态可见）；**启用前展示条件、频率与预计消耗**；同一新岗位只产生一个发现待办；重复触发和结果未知经同一请求身份对账。
- 失败处理：**预算不足或来源不完整形成可见状态，不静默跳过。**

## 3. 后端 API 面（集成 HEAD 实核冻结）

`internal/router/routes_career.go:35-37`：
- `POST /api/v1/career/rules` → `h.SetRule`（创建/更新规则：条件、频率、启停）
- `GET /api/v1/career/rules/receipt?requestId=...` → `h.RuleReceipt`
- `GET /api/v1/career/rules/:ruleId` → `h.GetRule`（规则状态 + 下次运行计划 + 执行历史/可见状态如 blocked_no_quota、no_vetted_sources）
请求/响应 JSON 以 `internal/modules/career/search_rule.go` 类型为最终依据（先读再写客户端；预计消耗的估算口径以后端冻结为准，前端如实展示不重算）。既有 api-client 尚无 rules 方法——你新增，严格解码遵循 `packages/api-client/src/career.ts` 既有模式。

## 4. Files（所有权）

`apps/web/src/career/`（RulePage 或 SearchPage 扩展规则区 + 测试 + css）；modify `packages/api-client/src/career.ts` + 聚焦测试（三方法）。不改后端、不改 career-core 合同（缺口如实报告）。

## 5. 行为要求（RED 测试先行）

- 规则创建/编辑：条件、频率、启停显式控件；**启用前展示条件、频率与预计消耗**（后端估算值如实显示，标注口径）。
- 修改规则后显示**下次运行计划**（E2E 关键验收）；暂停显示"下一次触发被取消/顺延"状态；关闭显示未运行。
- 发现待办：同一新岗位仅一条（列表呈现）；执行历史含可见状态（预算不足/来源不完整**不静默跳过**——状态行展示）。
- 未知回执原 requestId 对账恢复；revision 冲突显示当前 revision；跨租户错误态；空间切换 abort 清缓存。
- 明确语义呈现：规则默认不开启；无暗中后台运行的 UI 暗示。

## 6. 验证（全跑附原始输出）

```bash
pnpm typecheck:web
pnpm test:web          # 全量 node26 终验口径；须全绿 0 cancelled（当前基线 2420/2420；跑前 ps 清理孤儿 runner——Wave 3-7 先例）
pnpm build:web
git diff --check
```
**浏览器 E2E（T13 verified 关键验收）**：本地 Lite 服务器（复现提纲见 `docs/plans/issue-140/task-3-live-http-validation.md`；**本轮端口 57814**）。真实浏览器：登录 → 创建规则（默认关闭）→ 启用（条件/频率/预计消耗可见）→ **修改规则 → 下次运行计划更新显示** → 暂停 → 取消/顺延状态 → 执行历史可见状态（no_vetted_sources 如实呈现）→ 重复/未知回执恢复路径。截图/记录入报告。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- Web 使用 WeKnora TDesign 浅色主题与 `#07c05f` 品牌色。
- 搜索规则由用户显式启停并经预算准入；额度耗尽仍可读取既有档案和申请。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t13-web/` 下写 `task-2-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、全量验证输出、浏览器 E2E 证据（端口、截图清单）、已知局限（后端 4 low 转述）。COMMIT：`feat(web): manage recurring search rules with run plans`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T13 verified 由主控裁决。
