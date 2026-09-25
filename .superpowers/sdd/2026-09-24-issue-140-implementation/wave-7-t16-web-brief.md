# Wave 7 — T16 Web 子任务：材料发布与授权下载 E2E（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** T16 后端已集成（Wave 6，067aa1d0e，评审通过——2 low 不阻塞）；你交付 Web 面并完成 T16 的 Web 下载 E2E 验收——T16 verified 的最后缺口。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t16-web/WeKnora-fork01 --detach 067aa1d0e
cd /Users/wuyongjun/.codex/worktrees/issue-140-t16-web/WeKnora-fork01
```
BASE = `067aa1d0e`。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 16 Web 部分（verbatim）

- Step 3：为 Web `rendering.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；`pnpm typecheck:web && pnpm test:web` 预期 GREEN。（路径按现行布局为 `apps/web/src/career/ExportPage.tsx` 或并入 MaterialPage——实现者按最小侵入选择并报告。）
- 原始证据（Web 部分）：**"Workbench 授权测试和 Web 下载 E2E"**。
- 验收（Web 可观察部分）：两种文件绑定同一正文摘要与材料版本；**两种验证均通过后版本才可标记可用于投递**（staged/failed/submittable 状态可见）；旧版本保持可下载；**删除或撤销后旧授权立即失效**。
- 失败处理：单一格式失败保留 staged 状态和错误，不只发布成功的一半为可投递。

## 3. 后端 API 面（集成 HEAD 实核冻结）与主控裁决

`internal/router/routes_career.go:42-46` 五路由：
- `POST /api/v1/career/materials/:materialId/exports` → 发布（双格式渲染+验证，staged→submittable/failed）
- `GET /api/v1/career/materials/:materialId/exports` → 导出列表（状态/摘要/版本绑定）
- `POST /api/v1/career/materials/:materialId/exports/:exportId/signed-url` → 签发下载授权
- `GET /api/v1/career/materials/:materialId/exports/:exportId/download` → 下载兑付
- `DELETE /api/v1/career/materials/:materialId/exports/:exportId` → 撤销
请求/响应 JSON 以 `internal/modules/career/rendering.go` 与 handler 类型为最终依据（先读再写客户端）。
**主控裁决（Wave 7，记台账）**：导出下载为**认证兑付**（signed-url 签发后下载仍带认证），与 T04 Task 产物的 credential-free 兑付不同——该差异是 T16 评审披露项，裁定成立不改（理由：材料属 Career 域更高敏感级；T06 小程序下载本就用带 Authorization 的 downloadFile，认证兑付全端兼容）。Web 客户端下载须带认证（fetch + Authorization → blob）。

## 4. Files（所有权）

`apps/web/src/career/`（MaterialPage 扩展导出区或新 ExportPage + 测试 + css）；modify `packages/api-client/src/career.ts` + 聚焦测试（五方法严格解码）。不改后端、不改 career-core 合同（缺口如实报告）。

## 5. 行为要求（RED 测试先行）

- 材料版本视图内发起发布：过程状态可见（staged→双验→submittable 或 failed+错误）；**双格式都成功才显示"可用于投递"**；单格式失败显示 staged/failed 与错误，绝不显示半发布为可投递。
- 导出列表：每项显示双格式文件、同一 digest、绑定 material version、状态。
- 下载：签发授权 → 认证下载 PDF 与 DOCX；（E2E 中）对下载字节做 SHA-256 与响应 digest 比对。
- 撤销：DELETE 后旧授权立即失效（再次下载被拒）；旧版本（未撤销）仍可下载。
- revision 冲突/跨租户 403/404/未知回执恢复——house 呈现。

## 6. 验证（全跑附原始输出）

```bash
pnpm typecheck:web
pnpm test:web          # 全量 node26 终验口径；须全绿 0 cancelled（当前基线 2412/2412；跑前 ps 清理孤儿 runner——Wave 3-6 先例）
pnpm build:web
git diff --check
```
**浏览器 E2E（T16 verified 关键验收）**：本地 Lite 服务器（复现提纲见 `docs/plans/issue-140/task-3-live-http-validation.md`；**本轮端口 57813**）。真实浏览器：登录 → 建档/岗位/申请/材料链路或 fixture（声明层级）→ 确认材料版本 → 发布 → 状态流转（staged→submittable）→ 下载 PDF 与 DOCX → **SHA-256 与响应 digest 比对一致** → 撤销 → 旧授权立即失效 → 旧版本仍可下载 → 失败路径（如构造单格式失败 fixture，如实声明）保留 staged。截图/记录入报告。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- Web 使用 WeKnora TDesign 浅色主题与 `#07c05f` 品牌色。
- 材料由同一结构化正文生成，真实核验后发布不可变版本。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t16-web/` 下写 `task-2-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、全量验证输出、浏览器 E2E 证据（端口、截图清单、digest 比对记录）、已知局限（后端 2 low 转述）。COMMIT：`feat(web): publish and download verified material exports`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T16 verified 由主控裁决。
