# Wave 2 — T09-Web 任务报告（Web Source Status And Paste Fallback）

- 执行者：frontend_implementer（T09 Task 2, Issue #149 implement）
- 日期：2026-09-25（Asia/Shanghai）
- 独立 Worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t09-web/WeKnora-fork01`
- BASE `7cbad8941` → HEAD `3f7323d4b`（本地提交 1 个，未 push；worktree 另含未跟踪的 `.superpowers/sdd/2026-09-24-issue-140-t09-web/` 证据目录：task-2-report.md、4 张浏览器截图、live-results.json）
- 改动（全部在任务所有权内，+420/−9）：`apps/web/src/career/OpportunityPage.tsx`、`OpportunityPage.test.tsx`、`opportunity.css`、`packages/api-client/src/career.ts`、`career.test.ts`
- 评审包：`.superpowers/sdd/2026-09-24-issue-140-implementation/review-7cbad8941..3f7323d4b.diff`（61,772 bytes，位于 T09 worktree；1 commit）

## 交付内容

1. **api-client**：冻结枚举（sourceStatus 8 值 / completeness 3 值 / failureCode 10 值，逐字对齐后端冻结定义）+ 严格解码器（枚举外的值一律 TypeError，前端不可能发明新值）+ `importUrl`（POST `/opportunities/import-url`）+ `opportunityObservations`（GET `/opportunities/:id/observations`）+ `importOpportunity` 可选 `opportunityId`/`priorObservationId` 成对追加入参。
2. **Web UI**：URL 导入子面板（独立 request ID；`outcome_unknown` 用原请求 ID 重试恢复；403/空间切换清除链接与粘贴草稿并撤销全部渲染）；typed status 卡（来源状态/完整度/有界原因/提交链接（惰性文本）/尝试与采集时间/需用户补充 JD/应用内 owner-scope 证据链接）；URL 观察后粘贴自动 append 到同一 opportunity 为新不可变快照并展示观察历史（原 URL 观察可追溯）；不完整来源不渲染任何评估入口或硬条件结论。

## TDD 证据

- RED：api-client 3 新测试失败（`api.importUrl is not a function` ×2 + append body 深比较失败），既有 5 过；Web 7 新浏览器级测试失败（URL 输入不存在），既有 30 过。
- GREEN：api-client 8/8；Web career 聚焦 50/50（含修正一处测试桩缺陷：mock 未按 URL 变化 sourceStatus）。

## 验证结果（真实命令与输出）

| 命令 | 结果 |
| --- | --- |
| 聚焦 `node --import tsx --test src/career.test.ts` | 8/8 pass |
| 聚焦 `node --import tsx --test --test-concurrency=1 src/career/OpportunityPage.test.tsx src/career/CareerPage.test.tsx` | 50/50 pass |
| `pnpm typecheck:web` | EXIT=0（修复 2 轮 decoder 类型收窄） |
| `pnpm build:web` | EXIT=0（`✓ built in 2m 1s`） |
| `git diff --check` | EXIT=0 |
| `pnpm test:web`（全量） | 最终尝试（concurrency=1 + force-exit 偏差已记录）：2327 tests / 2326 pass / **1 fail（预存在 agent-editor 卡死文件，被外部终止；子测试单跑全过）** / 0 cancelled |

## 全量 `pnpm test:web` 环境记录（如实，共 4 次尝试）

- 两次精确 `pnpm test:web` 均在预存在文件上无限挂起：`GeneralPreferencesPanel.test.tsx`（单跑 13/13 pass）与 `agent-editor.test.tsx`（全部子测试通过但子进程在 `node --test` runner 下不退出/死循环 86.8% CPU；并行会话在 node 26.4 下同文件同样卡死）。两文件与本任务改动集零交集（`git diff --name-only 7cbad8941 3f7323d4b`；该测试仅 type-only 导入 api-client）。engines 要求 node ≥26，本机 22.22.3。改动前基线全量 8.25 分钟跑完但含 2 个未定位失败。
- **最终结果**（同测试集 + `--test-concurrency=1 --test-force-exit` 两个偏差，手动 kill 卡死的 agent-editor 子进程后 runner 自然跑完）：**`# tests 2327 / # pass 2326 / # fail 1 / # cancelled 0`**，唯一 fail 为被外部终止的 `agent-editor.test.tsx`（其 20 个子测试单跑全过）。本任务全部新测试与既有 career 测试在该全量运行中通过。
- 建议主控在低争用/node26 环境复跑一次精确 `pnpm test:web` 作为最终门。

## 浏览器 fixture 实线验证

- 端口偏差：分配的 57803 被并行 T02 会话遗留 `tls-proxy`（pid 20938）占用（不可终止他人进程），改用 API `127.0.0.1:57804` / Vite `127.0.0.1:57805`。
- 一次性 SQLite Lite + 随机密钥 + 一次性账号 + Playwright Chromium（无头，仓库自带 `@playwright/test`）。生产 source allowlist 保持为空——实测 URL 导入返回 `policy_unverified`（设计使然）。
- 实测链路全部达成：输入 URL → 来源状态：来源未核验 + 原因 + 需用户补充 JD（0 处硬条件结论字样）→ 粘贴合成 JD（含惰性 `<system>` 指令，证据页按字面显示未执行）→ 新快照挂同一 opportunityId（两个不同 snapshotId）→ 观察历史双条目（URL 未核验 + 手工粘贴），链接全部为应用内 owner-scope 路径。SQLite 落库核验通过。截图 4 张存 worktree 证据目录。

## 报告事项（不擅自修，需主控/T03 裁决）

- **T03 合同缺口**：后端为失败 URL 导入刻意存空正文快照，而 `career-core/contracts.ts` 的 `decodeOpportunityEvidence` 拒绝空 `rawText` → 原 URL 观察的证据页在 Web 端进入「无法读取职位证据」错误态（无结论泄露；可追溯性由观察历史承担）。建议解码器放行空 rawText 或后端证据视图返回占位正文。
- `packages/api-client/src/index.ts` 未导出新类型（不在所有权内）；Web 按仓库既有相对路径模式直接 import career.ts。
- T09 后端遗留 medium（冻结分类断言）已 park 至 final validation 轮，不属本任务。
