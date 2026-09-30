# Wave 10 — T22-Web 报告：导出与完整删除 Web 面（Issue #162）

- 实现者：frontend_implementer（独立 worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t22-web/WeKnora-fork01`）
- BASE `f6b14cf95` → HEAD `ea6da5677`（2 提交：`d9b1a6d17 feat(web): export and completely delete career data`、`ea6da5677 docs(sdd): T22-Web execution report with E2E evidence`）；未 push；留在 worktree 等独立评审与集成。
- 完整报告（含全部证据索引）：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t22-web/task-2-report.md`；截图/导出包/验证日志在同目录 `evidence/`。

## 交付摘要

- `packages/api-client/src/career.ts`：T22 合同 + 五方法（exportCareer / careerExportReceipt / careerDeletionBoundary / deleteCareer / careerDeletionReceipt），严格解码含"真话状态"不变式（`deleted ⟺ 全部步骤 done`；`partial ⟺ ≥1 失败`；非 deleted 无 completedAt）。聚焦测试 4 个（+31/31 全文件）。
- `apps/web/src/career/ExportDeletionPage.tsx`（+测试 7 个 + css）：导出发起→六段包含性呈现（档案/事实历史/岗位快照/申请事件/材料版本/投递记录）→认证下载；删除先拉边界清单（空间内 12 节 vs 外部平台不可撤回 + 保留范围）→强制确认→发起→步骤/保留呈现；部分失败保留原请求编号可恢复重试、绝不显示"已完全删除"；删除完成自动重放删除前导出回执验证旧授权 404 并回调 `onCareerDeleted` 清客户端缓存。
- `CareerPage.tsx` 最小侵入挂载（+1 行 render）；`CareerPage.test.tsx` 仅补 CSS stub 两行（同既有页面测试模式）。路由未改（报告说明：选择挂载 `/platform/career` 而非新路由）。

## 验证结果（真实输出）

| 命令 | 结果 |
| --- | --- |
| `pnpm typecheck:web` | exit 0 |
| `pnpm test:web` | `tests 2438 / pass 2437 / fail 1 / cancelled 0`；唯一 fail 为 **已知** agent-editor 并发 flaky（T19/T20 台账转交项，提交 86040459f/a98837877 原文"agent-editor 并发 flaky 单独复跑 49/49"）；按先例单独复跑 → `tests 49 / pass 49 / fail 0 / cancelled 0`（本会话 3 次验证均过）。因果核查：该测试运行时模块图不含本任务改动文件；A/B 各 3 次样本不足归因（带改动 1 挂/基线 0 挂），已如实记录。并行会话（issue106 worktree）跨包测试在同时段占用机器。 |
| `pnpm build:web` | exit 0（`✓ built in 29.77s`） |
| `git diff --check`（工作树与 f6b14cf95..HEAD 范围） | 均 exit 0 |

基线 2442 + 新增 7 = 2449；全量计数 2438 = 2449 − 11（被杀 agent-editor 子进程未计入的 11 个测试；38/49 已完成）。RED 证据：api-client 4 失败（`exportCareer is not a function` 等）+ 页面模块未存在（ERR_MODULE_NOT_FOUND）。

## 浏览器 E2E（端口 57816，全新隔离临时 DB）

Lite 服务器 127.0.0.1:57816（一次性 SQLite DB + memory stream + 一次性账号）；Web dev 57818 代理。真实浏览器登录 → API 链路构造档案/岗位/评估/申请（含 Task）/事件/材料+导出发布/投递（声明层级：全部真实后端 API）→ UI 导出（六段清单）→ 真实下载 `career-export-*.json`（结构核对六段齐全、taskId 在内、**客户端独立重算 sha256 与服务端 digest 逐位一致**）→ 边界清单呈现（12 节/外部 2 项不可撤回/保留 4 行）→ 确认后删除 → "已完全删除"+4 步骤+保留披露 → **旧授权验证**：页面自动重放导出回执 404；同会话 API 探测旧导出回执/旧岗位/旧材料/旧材料下载授权全部 404；`/career/open` 200 空 facts；页面缓存清理生效；旧岗位 URL UI 失效态。截图 5 张 + 导出包 JSON 存 `evidence/`。部分失败场景：真实服务器无故障注入，声明不可构造（组件+解码层测试覆盖）。环境已全部清理（进程停止、临时目录删除、主仓库无残留）。

## 自查与遗留

- 简报 Web 可观察验收逐条满足（见 worktree 报告 §6）。
- 遗留：①部分失败 E2E 不可构造（无注入通道）；②digest 客户端校验依赖 JSON 键序，页面不声称实时校验（下载文件保留摘要供离线核验）；③OpportunityPage 404 文案为存量行为；④agent-editor 并发 flake 为既有转交 concern，建议主控延续 T19/T20 台账处理口径。
- 无迁移（T22 后端迁移已在 Wave 9 集成）；未改后端与 career-core 合同。
