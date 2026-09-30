# Wave 13 — T21 Web 子任务：额度预估与超额阻断（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** T21 后端已集成（Wave 12，38c22abe9，第 3 轮复审通过 F4 high 已修）；你交付 Web 面并完成 T21 的 Web E2E 验收——T21 verified 的最后缺口。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t21-web/WeKnora-fork01 --detach 38c22abe9
cd /Users/wuyongjun/.codex/worktrees/issue-140-t21-web/WeKnora-fork01
```
BASE = `38c22abe9`。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 21 Web 部分（verbatim）

- Step 3：为 Web `usage.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；`pnpm typecheck:web && pnpm test:web` 预期 GREEN。（路径按现行布局为 `apps/web/src/career/UsagePage.tsx` 或并入既有页面——最小侵入选择并报告。）
- 原始证据（Web 部分）：**"Web E2E 模拟超额后仍能打开旧申请"**。
- 验收（Web 可观察部分）：执行前展示将消耗额度与触发条件；超额阻止新的收费 Run 而非删除既有档案或申请；重复请求不重复预占或收费；付费状态不改变岗位排序或资格判定。
- 失败处理：**预估不可得时不得先执行后补报，显示可理解的不可用原因**。

## 3. 后端 API 面（集成 HEAD 实核冻结）

`internal/router/routes_career.go:35`：`GET /api/v1/career/usage/estimate` → `h.UsageEstimate`（预估：将消耗额度+触发条件；不可得时 typed 原因）。admission 语义内嵌于 search_once/规则触发等收费 act 响应（超额 typed 拒绝、余额展示）——JSON 以 `internal/modules/career/usage.go` 类型为最终依据（先读再写客户端）。既有 api-client 尚无 usage 方法——你新增，严格解码遵循既有模式；收费入口（SearchPage/RulePage）补预估展示与超额态。

## 4. Files（所有权）

`apps/web/src/caree/`（UsagePage 或并入选购入口 + 测试 + css）；modify `packages/api-client/src/career.ts` + 聚焦测试；最小侵入修改 SearchPage/RulePage（预估/超额呈现，不破坏既有测试）。不改后端、不改 career-core 合同（缺口如实报告）。

## 5. 行为要求（RED 测试先行）

- 预估展示：收费动作（搜索/规则启用）执行前显示"将消耗额度+触发条件"（来自 estimate 端点）；**预估不可得 → 明确不可用原因，不提供"先执行"路径**。
- 超额态：超额时新收费 Run 被阻（typed 错误呈现"额度不足"）；**既有档案/申请/评估仍可打开**（E2E 关键路径：超额后打开旧申请详情完整可读）。
- 重复请求不重复扣（replay 呈现）；付费状态不影响排序/资格（UI 无任何付费影响展示的断言）。
- 失败/权限/未知恢复态。

## 6. 验证（全跑附原始输出）

```bash
pnpm typecheck:web
pnpm test:web          # 全量 node26 终验口径；须全绿 0 cancelled（当前基线 2466/2466；跑前 ps 清理孤儿 runner）
pnpm build:web
git diff --check
```
**浏览器 E2E（T21 verified 关键验收）**：本地 Lite 服务器（复现提纲见 `docs/plans/issue-140/task-3-live-http-validation.md`；**本轮端口 57824**）。真实浏览器：登录 → 构造数据/申请 → 预估展示 → 消耗至超额（fixture/直接消耗，声明构造）→ 新搜索被阻（超额态）→ **旧申请仍完整可读** → 重复请求不二扣 → 预估不可得场景（seam，声明）。截图/记录入报告。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- 额度耗尽仍可读取既有档案和申请；Web 使用 TDesign 浅色主题与 `#07c05f`。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t21-web/` 下写 `task-2-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、全量验证输出、浏览器 E2E 证据、已知局限（后端 F2/F3 low 转述）。COMMIT：`feat(web): show quota estimates and overage gates`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T21 verified 由主控裁决（解锁 T30 最后前置）。
