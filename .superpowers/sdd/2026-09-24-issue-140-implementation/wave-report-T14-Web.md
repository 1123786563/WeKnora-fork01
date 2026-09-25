# Wave 3 — T14-Web 实现报告：从岗位卡打开独立申请（含 Step 0 偿还 T09-Web F2）

- BASE `d88cd513a` → HEAD `4f3b3beda`（worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t14-web/WeKnora-fork01`，本地提交未 push）
- 提交：`f11184413` `fix(api-client): decode empty rawText evidence pages`（Step 0 单独提交）；`4f3b3beda` `feat(web): open per-application tasks from opportunities`
- 文件（均在简报所有权内，7 files / +928 −6）：
  - create `apps/web/src/career/ApplicationPage.tsx`、`ApplicationPage.test.tsx`（14 用例）、`application.css`
  - modify `apps/web/src/career/OpportunityPage.tsx`（岗位卡申请入口；EvaluationAction 增可选 `onReceipts`，既有行为不变）、`OpportunityPage.test.tsx`（37 旧用例 + 1 新）
  - modify `packages/api-client/src/career.ts`（Step 0 空 rawText 解码 + createApplication/applicationReceipt/application/reconcileApplicationLink 严格解码）、`career.test.ts`

## Step 0（T09-Web F2：空 rawText 证据页打不开）

- RED：api-client 测试断言空 rawText 证据页可解码 → `node --import tsx --test packages/api-client/src/career.test.ts`：`# tests 9 / pass 8 / fail 1`（career-core 严格解码拒绝，与 T09-Web 报告 L45 发现一致）。
- 修复：api-client 本地 `decodeOpportunityEvidencePage` 允许 `rawText === ''`（空白非空串仍拒、rawSha256/extracted/source/acquiredAt/status 严格性不变）；career-core 合同未动（其 9/9 用例含拒绝空 rawText 用例继续通过）。
- GREEN：`# tests 9 / pass 9 / fail 0`。单独 commit。

## 主任务 RED → GREEN 摘要（详细证据见 worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t14-web/task-3-report.md`）

- api-client 四方法：RED `# tests 12 / pass 9 / fail 3` → GREEN `# tests 12 / pass 12 / fail 0 / cancelled 0`。冻结枚举（linking|ready|link_failed、eligible|ineligible|unknown）与 pin 六字段严格解码，发明值拒绝。
- ApplicationPage（14 用例）：固定证据与 pinnedEvidence 可见、ready 显示 TaskID/RunID、硬条件警示常驻+默认阻断+显式继续后 qualified=false、linking 不显示就绪且用原 requestId 对账→ready、结果未知先 applicationReceipt(原 ID) 再原样重放（deepEqual、不二建）、revision_conflict 显示当前修订并新 ID 重提、link_failed 呈现+原 ID 重试对账、application_conflict 明确拒绝、403 清空、URL `application=` 参数刷新恢复、同岗不同批次、空间切换清除、reload 后存储回据回填固定评估。
- OpportunityPage 岗位卡入口：RED `# tests 38 / pass 37 / fail 1` → GREEN `# tests 38 / pass 38 / fail 0 / cancelled 0`（含既有“snapshot A→B 清除/加载期零按钮”断言不回归）。

## 验证命令与真实输出

- `pnpm typecheck:web`：0 错误（`grep -cE "error TS"`=0）。
- `pnpm build:web`：`✓ built in 1m 24s`（仅既有 chunk 体积告警）。
- `git diff --check`（d88cd513a..HEAD）：无输出。
- `pnpm test:web`（全量，0 cancelled 要求）：**已跑完（4h10m）：`tests 2367 / pass 2366 / fail 0 / cancelled 1`（EXIT=1）**。唯一 cancelled 是无关既有文件 `src/agents/agent-editor.test.tsx` 文件级取消（4.17h 后被 runner 取消；该文件在本分支与 BASE 逐字节一致，当日多会话同样卡死；单跑 ~300s 报 `'Promise resolution is still pending…'`——环境级问题，非本任务引入）。本任务全部新增/修改测试在全量运行中通过、0 fail；简报“全绿 0 cancelled”因该无关文件未满足。完整输出 /tmp/t14-testweb-full.log（2357 ✔ / 0 ✖）。

## 浏览器 E2E（端口 57806，T14 verified 关键验收）

Lite SQLite 后端 `127.0.0.1:57806`（一次性目录 `/tmp/weknora-t14-browser.uKGB4q`）+ vite `57807` 代理 + 仓库 Playwright 真实 Chromium。12/12 相位通过；12 张截图与逐步日志在 `t14-web-shots/`（本目录）。链路：注册/登录 → 建档确认 毕业时间=2026 → 粘贴 JD（`仅限2027届`，冻结硬规则要求整段单从句）→ 评估不符合 → 岗位卡“创建求职申请”→ 警示常驻+默认阻断 → 勾选显式继续 → 创建（浏览器级 fixture 将首个响应改写为 linking 展示“Task 关联中”）→ 用原请求编号对账 → 真实 ready（Task+Run ID）→ qualified=false + 警示常驻 + pinned 六字段 → 刷新后 URL `application=` 恢复 → 第二批次成功 → 同批次再建被 application_conflict 拒 → 账户 B 跨租户打开同一 URL 仅见“当前空间不可访问”，无泄露。回执显示后端规范化批次（`2026 秋招 a 批`）逐字呈现。

## 自查与遗留

- 简报行为要求逐条覆盖（见上）；`pinnedEvidence.batchIdentity` 与提交值以后端规范化为准，前端回执校验只比对 requestId+三 ID。
- 已知局限：E2E linking 展示为浏览器级 fixture（服务端 linking 留存需 linker 失败注入，Lite 未诱导）；reconcile 及其后全部为真实服务端行为；“重复请求不二建”由组件测试覆盖（deepEqual 重放返回原回执），浏览器链路未重复触发；全量 test:web 在本机多会话高负载下运行缓慢（诚实记录，见补记）。
