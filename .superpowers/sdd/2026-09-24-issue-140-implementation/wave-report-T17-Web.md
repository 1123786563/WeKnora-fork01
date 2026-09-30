# Wave 6 — T17-Web 报告：申请进展时间线（Issue #157）

- 实现者：frontend_implementer（T17-Web）；BASE `4b09af298` → HEAD `d6c0b1ec2`（`feat(web): track application progress timeline`，独立 worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t17-web/WeKnora-fork01` 本地提交，未 push）
- 详细报告（RED/GREEN 全文、逐相位证据、自查与局限）：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t17-web/task-2-report.md`
- E2E 归档：本目录 `t17-web-shots/`（截图 8 张 + evidence.log + server-startup.log + e2e-run.mjs）

## 1. 交付物（文件所有权 = 简报声明范围）

- create：`apps/web/src/career/ProgressPage.tsx` + `ProgressPage.test.tsx`（10 测试）+ `progress.css`
- modify：`packages/api-client/src/career.ts`（冻结 progress 类型+严格解码+四方法 appendProgress/correctProgress/applicationProgress/progressReceipt，客户端不声明 source）+ `career.test.ts`（2 聚焦测试）+ `apps/web/src/career/ApplicationPage.tsx`（回执块「查看申请进展时间线」按需入口，key 隔离跨申请）+ `ApplicationPage.test.tsx`（1 接线测试）

## 2. TDD 与验证（真实输出）

- RED：api-client `TypeError: api.appendProgress is not a function`（18/20）、ProgressPage `ERR_MODULE_NOT_FOUND`、ApplicationPage 接线 15/16；GREEN：api-client 20/20、ProgressPage 10/10、ApplicationPage 16/16。
- `pnpm typecheck:web` 通过；`pnpm test:web` **2412/2412，0 fail 0 cancelled**（基线 2401 + 11 新 web 测试）；`pnpm build:web` `✓ built in 24.52s`；`git diff --check` exit 0。
- 中途缺陷修复（先测后修）：网络失败曾落入确定错误分支 → `ReceiptMismatchError` 区分 + 第 10 个组件测试。

## 3. 浏览器 E2E（后端端口 57812；T17 verified 关键验收）

Lite SQLite 一次性库 + vite dev 57814 代理 + 真实 Chromium headless；申请详情走真实链路（注册→档案确认→JD→评估→创建申请→Task ready）。**9/9 相位 OK**：入口打开（准备中/0 事件）→ 录入已投递（来源/确认者可见）→ 录入面试（投影 面试/2 事件）→ **刷新+reload 重开一致（面试/2）**→ 纠错最新事件（原文「已更正」标记 + 更正事件并见，投影 测评或笔试）→ **route 中断 POST 制造未知 → 原 requestId 查回执 404 提示 → 原编号重试 200 → 恰好 4 事件无重复**→ 跨租户 B 打开同 URL（opportunities/progress 均 403，无泄露）→ HTTP 探针（A 重放同 ID 200 同 eventId 事件数不变；同 ID 不同内容 409 idempotency_conflict；回执 200 同 eventId；B 403）。

语义澄清（读码 `progress.go:517-549` + 实测）：更正在目标事件的时间线位置生效——纠错非最新事件时，最新事件仍决定阶段（与后端冻结语义一致，UI 呈现不改写）。

## 4. 遗留与局限（详见 task-2-report §5）

- occurredAt 不提供录入（后端默认当前时刻）；纠错入口对已更正原事件仍开放（后发更正胜出，符合后端多更正语义）。
- 后端 Wave 5 遗留 3 low 转述不阻塞；本轮未发现后端合同缺口。
- 一次与本任务无关文件（agent-editor.test.tsx）的环境性 test:web 停滞：单跑 49/49、清理后整库重跑全绿（记录于 task-2-report §3 注）。
- 服务器/端口已停；临时目录 `/tmp/weknora-t17-browser.*` 保留至集成评审。

## 5. 结论

T17 Web 面交付并完成 Web E2E 验收全链路（录入→投影一致→纠错并见→跨租户拒绝→未知回执恢复）。T17 verified 由主控裁决。
