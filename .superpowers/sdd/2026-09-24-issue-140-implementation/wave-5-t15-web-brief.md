# Wave 5 — T15 Web 子任务：材料编辑与 V1/V2 比较（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** T15 后端已集成（Wave 4，05122c146，评审通过——1 medium + 3 low 均读路径级不阻塞）；你交付 Web 面并完成 T15 的 Web E2E 验收——T15 verified 的最后缺口。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t15-web/WeKnora-fork01 --detach 05122c146
cd /Users/wuyongjun/.codex/worktrees/issue-140-t15-web/WeKnora-fork01
```
BASE = `05122c146`。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 15 Web 部分（verbatim）

- Step 3：为 Web `material.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；`pnpm typecheck:web && pnpm test:web` 预期 GREEN。（路径按现行布局为 `apps/web/src/career/MaterialPage.tsx` 风格——沿用 OpportunityPage/ApplicationPage/SearchPage 命名先例。）
- 原始证据（Web 部分）：**"Web E2E 修改正文并比较 V1/V2"**。
- 验收（Web 可观察部分）：生成前冻结岗位与档案版本可见；主张链接到确认事实；缺失实习、证书、数字不得补造（审阅风险可见）；**用户确认正文后形成新不可变版本，旧版本可比较**；修改不覆盖旧版本。
- 失败处理：**生成或审阅失败保留草稿与原因，不发布可投递版本**。

## 3. 后端 API 面（集成 HEAD 实核冻结）

`internal/router/routes_career.go:35-41` 七路由：
- `POST /api/v1/career/materials` → `h.EditMaterial`（编辑/起草）
- `POST /api/v1/career/materials/confirm` → `h.ConfirmMaterialBody`（确认正文 → 不可变版本）
- `GET /api/v1/career/materials/receipt` → 收据
- `GET /api/v1/career/materials/:materialId` → 材料
- `GET .../versions`、`GET .../versions/:versionId`、`GET .../versions/:versionId/compare` → 版本列表/读取/比较
请求/响应 JSON 以 `internal/modules/career/material.go` 与 handler 类型为最终依据（先读再写客户端）。既有 api-client 尚无 materials 方法——你新增对应方法，严格解码遵循 `packages/api-client/src/career.ts` 既有模式。

## 4. Files（所有权）

create `apps/web/src/career/MaterialPage.tsx` + 测试 + 本地 css；modify `packages/api-client/src/career.ts` + 聚焦测试；最小入口接线（ApplicationPage 或 Career 导航，不破坏既有页面）。不改后端、不改 career-core 合同（缺口如实报告）。

## 5. 行为要求（RED 测试先行）

- 从申请上下文进入材料：显示冻结的岗位快照/档案版本（pinned evidence）；正文编辑作用于结构化正文。
- 主张/字段呈现事实引用与确认状态；缺失项显示"缺失/待补充 + needs_review"，**界面绝不出现补造值**；审阅风险清单可见。
- 确认正文 → 新不可变版本（版本号递增）；版本列表、V1/V2 **并排比较**（差异可见）；旧版本只读。
- 编辑产生 draft；失败（校验/审阅失败）保留草稿与原因，不发布；未知回执原 requestId 恢复；revision 冲突显示当前 revision；跨租户 403/404 清晰错误态。
- **观测项（本任务附带，不改后端）**：T15 后端评审遗留 1 medium"重复 heading compare 误报"（读路径级、不阻塞，集成台账载明）。在 V1/V2 比较视图中若可复现该误报，**精确记录复现步骤与期望/实际**（供后续修复轮）；不可复现也如实记录"未复现"。

## 6. 验证（全跑附原始输出）

```bash
pnpm typecheck:web
pnpm test:web          # 全量 node26 终验口径；须全绿 0 cancelled（当前基线 2389/2389；跑前 ps 清理孤儿 runner——Wave 3/4 先例）
pnpm build:web
git diff --check
```
**浏览器 E2E（T15 verified 关键验收）**：本地 Lite 服务器（复现提纲见 `docs/plans/issue-140/task-3-live-http-validation.md`；**本轮端口 57810**）。真实浏览器：登录 → 建档/导入岗位/评估/创建申请（或 fixture 直插并声明层级）→ 打开材料编辑 → 修改正文 → 确认发布 V1 → 再修改 → 确认发布 V2 → 版本列表 → **比较 V1/V2 差异** → 旧版本只读回看 → 失败路径（如引用未确认事实被拒）保留草稿。截图/记录入报告。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- Web 使用 WeKnora TDesign 浅色主题与 `#07c05f` 品牌色。
- 材料由同一结构化正文生成；修改后不得覆盖旧投递版；缺失不得由模型补造。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t15-web/` 下写 `task-2-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、全量验证输出、浏览器 E2E 证据（端口、截图清单）、compare 误报观测结论、已知局限。COMMIT：`feat(web): edit materials and compare immutable versions`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T15 verified 由主控裁决。
