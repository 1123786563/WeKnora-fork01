# Wave 4 — T11 Web 子任务：一次性找岗页面与恢复历史（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。** T11 后端已集成（Wave 3，6a21e0df5，评审通过门全绿）；你交付 Web 面并完成 T11 的 Web E2E 验收——这是 T11 verified 的最后缺口。

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t11-web/WeKnora-fork01 --detach 6a21e0df5
cd /Users/wuyongjun/.codex/worktrees/issue-140-t11-web/WeKnora-fork01
```
BASE = `6a21e0df5`。本地 commit 允许；绝不 push/merge；严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 主计划 Task 11 Web 部分（verbatim）

- Step 3：为 Web `search_once.tsx` 写用户可观察行为测试，显示来源、权限、失败和恢复状态；运行 `pnpm typecheck:web && pnpm test:web`，预期 GREEN。（注：路径按现行布局为 `apps/web/src/caree/SearchPage.tsx` 风格——沿用 OpportunityPage/ApplicationPage 命名先例，主计划 search_once.tsx 为旧稿路径。）
- 原始证据（Web 部分）：**"Web E2E 从指令到岗位详情再到恢复历史结果"**。
- 失败处理：**"外部来源不可用时说明范围与失败，不以演示数据替代"**。
- 验收（Web 可观察部分）：找岗默认一次性，指令不自动创建持续规则；首批来源明确实际可用方式与城市覆盖，**不宣称全国完整**；结果列出**检查时间、资格状态、原始链接及不确定性**；执行失败、未知回执和额度拒绝均可恢复，不复制搜索。

## 3. 后端 API 面（集成 HEAD 实核冻结）

`internal/router/routes_career.go:32-34`：
- `POST /api/v1/career/searches` → body `SearchOnceInput {requestId, query, expectedRevision}`（`internal/modules/career/search_once.go:55`）
- `GET /api/v1/career/searches/receipt?requestId=...` → `SearchReceipt {kind, requestId, searchId, status, query, coverage, scopeNotes[], failureCode?}`（:92-99）
- `GET /api/v1/career/searches/:searchId` → 结果：`coverage.sources[] = {sourceId, label, accessMethods[], cities[], available, failureCode?}`（:64-69）；结果行 `{resultId, sourceId, link, checkedAt, qualification, uncertainty}`（:81-86）
枚举以后端冻结为准（`internal/modules/career/search_once.go` 为最终依据），前端严格解码、不得发明新值。既有 api-client（`packages/api-client/src/career.ts`）尚无 search 方法——你新增 `searchOnce`/`searchReceipt`/`search`，严格解码遵循该文件既有模式（参照 importUrl/applications 先例）。

## 4. Files（所有权）

create `apps/web/src/career/SearchPage.tsx` + 测试 + 本地 css；modify `packages/api-client/src/career.ts` + 聚焦测试；如需入口，在既有 Career 导航/页面加"找岗"入口（最小改动，不破坏既有页面）。不改后端、不改 career-core 合同（缺口如实报告）。

## 5. 行为要求（RED 测试先行）

- 输入指令 → 一次性搜索：结果列表每行显示 检查时间、资格状态（needs_review 等）、原始链接（可点但不自动抓取）、不确定性标注。
- 覆盖清单如实展示：当前生产无已核验来源 → 显示"暂无已核验来源/范围说明 + 失败原因"，**无"全国"类表述**，不用演示数据填充。
- 失败/未知/额度拒绝：typed 状态可恢复——未知回执先 `searchReceipt(requestId)` 恢复，不自动换新 ID；不复制搜索（重复请求返回同一 search 的 UI 呈现）。
- revision 冲突显示当前 revision；跨 User/Tenant 403/404 清晰错误态；空间切换 abort 并清缓存（Desk 语义）。
- 明确不提供"持续找岗"开关（那是 T13；页面不得出现规则类控件）。

## 6. 验证（全跑附原始输出）

```bash
pnpm typecheck:web
pnpm test:web          # 全量，node26（nvm）为终验口径；须全绿 0 cancelled（当前基线 2378/2378）
pnpm build:web
git diff --check
```
注意：全量 test:web 之前若卡死，先 `ps` 检查并清理陈旧孤儿 runner 子进程（Wave 3 集成裁定先例：环境级遗留，kill 后正常）；在报告记录。
**浏览器 E2E（T11 verified 的关键验收）**：本地 Lite 服务器（复现提纲见 `docs/plans/issue-140/task-3-live-http-validation.md`、`task-6-live-validation.md`；**本轮端口 57808**）。真实浏览器：登录 → 输入找岗指令 → 覆盖/范围说明呈现 → 结果列表（资格状态/链接/不确定性）→ 点结果进入岗位详情（OpportunityPage 既有页或证据页）→ 恢复历史结果（重进/刷新后历史 search 仍可读）→ 失败路径（无来源/超时 mock 或 fixture）如实呈现。截图/记录入报告。

## 7. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree；local commits authorized, no push/merge/deploy/GitHub action；禁止派发子 agent。
- Web 使用 WeKnora TDesign 浅色主题与 `#07c05f` 品牌色。
- 搜索规则由用户显式启停并经预算准入（本页面零规则控件）。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 8. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t11-web/` 下写 `task-2-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、全量验证输出、浏览器 E2E 证据（端口、截图清单）、已知局限（T11 评审 4 条 low 不在本任务范围，如实转述）。COMMIT：`feat(web): run one-shot searches with recovery`。最终消息报告 commit SHA 与全部验证结果。留在 worktree 等独立评审与集成；T11 verified 由主控裁决。
