# Wave 10 — T22 Web 子任务：导出与完整删除 E2E（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** T22 后端已集成（Wave 9，f6b14cf95，第 2 轮复审通过）；你交付 Web 面并完成 T22 的 Web E2E 验收——T22 verified 的最后缺口。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t22-web/WeKnora-fork01 --detach f6b14cf95
cd /Users/wuyongjun/.codex/worktrees/issue-140-t22-web/WeKnora-fork01
```
BASE = `f6b14cf95`。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 22 Web 部分（verbatim）

- Step 3：为 Web `career_export.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；`pnpm typecheck:web && pnpm test:web` 预期 GREEN。（路径按现行布局为 `apps/web/src/career/ExportDeletionPage.tsx` 或并入 CareerPage——最小侵入选择并报告。）
- 原始证据（Web 部分）：**"Web E2E 导出后发起删除并检查不可读取"**。
- 验收（Web 可观察部分）：导出包含档案、原岗位快照、申请事件和材料版本（导出包结构可见/可下载）；**删除前说明空间内与外部平台资料的边界**；删除使旧 Task、Artifact 授权和客户端缓存不可再访问；保留策略显示范围与状态。
- 失败处理：**局部删除失败保留可恢复状态与审计，不声称已完全删除**。

## 3. 后端 API 面（集成 HEAD 实核冻结）

`internal/router/routes_career.go:57-61` 五路由：
- `POST /api/v1/career/exports` → `h.ExportCareerHandler`（发起导出）
- `GET /api/v1/career/exports/receipt?requestId=...` → `h.ExportCareerReceiptHandler`（导出收据/状态/下载）
- `GET /api/v1/career/deletions/boundary` → `h.CareerDeletionBoundaryHandler`（**删除前边界清单**：空间内 vs 外部平台资料）
- `POST /api/v1/career/deletions` → `h.DeleteCareerHandler`（发起删除）
- `GET /api/v1/career/deletions/receipt?requestId=...` → `h.CareerDeletionReceiptHandler`（删除状态：进行/部分失败+审计/完成、保留范围披露）
请求/响应 JSON 以 `internal/modules/career/career_export.go` 类型为最终依据（先读再写客户端）。既有 api-client 尚无方法——你新增，严格解码遵循 `packages/api-client/src/career.ts` 既有模式。

## 4. Files（所有权）

`apps/web/src/career/`（导出/删除页 + 测试 + css）；modify `packages/api-client/src/career.ts` + 聚焦测试（五方法）。不改后端、不改 career-core 合同（缺口如实报告）。

## 5. 行为要求（RED 测试先行）

- 导出：发起 → 状态（准备/完成）→ 下载导出包（认证）；导出包含性呈现（档案/快照/事件/材料版本清单）。
- 删除：**先拉取并呈现边界清单**（空间内数据可删；外部平台资料不可撤回——用户确认后才能发起）；删除过程状态（进行/部分失败+审计+**可恢复重试**/完成）；**保留范围与状态显式披露**（若有）。
- 删除完成后旧授权/旧数据不可再访问（页面侧验证：既有入口返回 404/失效态；客户端缓存清理）。
- 未知回执原 requestId 对账；revision 冲突；跨租户错误态；**绝不显示"已完全删除"当状态是部分失败**。

## 6. 验证（全跑附原始输出）

```bash
pnpm typecheck:web
pnpm test:web          # 全量 node26 终验口径；须全绿 0 cancelled（当前基线 2442/2442；跑前 ps 清理孤儿 runner——Wave 3-9 先例）
pnpm build:web
git diff --check
```
**浏览器 E2E（T22 verified 关键验收，使用隔离临时数据库）**：本地 Lite 服务器（复现提纲见 `docs/plans/issue-140/task-3-live-http-validation.md`；**本轮端口 57816**；全新临时 DB）。真实浏览器：登录 → 构造档案/岗位/申请/材料数据（API 链路或 fixture，声明层级）→ 导出 → 下载导出包（结构核对）→ 拉取边界清单呈现 → 确认发起删除 → 完成态 → **旧授权/旧入口不可读取验证** → 部分失败场景（fixture 构造或如实声明不可构造）。截图/记录入报告；结束清理临时目录。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- 删除验证只用隔离临时数据库；Web 使用 TDesign 浅色主题与 `#07c05f`。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t22-web/` 下写 `task-2-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、全量验证输出、浏览器 E2E 证据（端口、截图清单、导出结构核对、不可读取验证）、已知局限。COMMIT：`feat(web): export and completely delete career data`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T22 verified 由主控裁决。
