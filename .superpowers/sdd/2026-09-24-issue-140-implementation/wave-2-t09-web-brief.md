# Wave 2 — T09 Web 子任务：Web Source Status And Paste Fallback（implement）

你是 frontend_implementer。本简报自包含：只读本简报 + 引用文件，不依赖任何父会话。**禁止派发子 agent。**

## 1. Worktree 与 BASE

```bash
git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t09-web/WeKnora-fork01 --detach 7cbad8941
cd /Users/wuyongjun/.codex/worktrees/issue-140-t09-web/WeKnora-fork01
```
BASE = `7cbad8941`（当前集成 HEAD）。本地 commit 允许；绝不 push/merge、绝不改集成分支、严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`。

## 2. 任务（T09 计划 Task 2，verbatim）

> **Depends:** reviewed and integrated Task 1 exact API commit.（已满足：T09 后端 483bcfe38 已集成为 `aad1262b9`，集成门全绿。）
> **Files:** modify `apps/web/src/career/OpportunityPage.tsx`、its tests and local CSS；`packages/api-client/src/career.ts` and focused test；route tests only if a new detail subroute is required.
> **Interfaces:** consume `importUrl`, `opportunityObservations`, and extended `importOpportunity` exactly as serialized by Task 1. Preserve request ID through unknown recovery and use a new ID only for the subsequent manual paste.
> - **RED:** add browser-level tests for complete URL, `policy_unverified`, login/summary/timeout, unknown POST recovery, 403/scope clear, and failure → paste → new snapshot → original trace. Assert incomplete text cannot show hard-condition conclusions.
> - **GREEN:** render typed status, submitted link, attempt/acquisition time, bounded reason, and "需用户补充 JD". Paste sends prior observation IDs and opens the new snapshot; history links remain inert and owner-scoped.
> - **VERIFY:** focused tests, `pnpm typecheck:web`, `pnpm test:web`, `pnpm build:web`, `git diff --check`; then local authenticated browser fixture with controlled source responses.
> - **COMMIT:** `git add apps/web/src/career packages/api-client && git commit -m 'feat(web): guide career URL import fallback'`.

主计划 Task 9 对应 Web 验收：**Web 演示链接失败后粘贴 JD 成功**；显示来源、权限、失败和恢复状态；摘要不足以推断届别、学历等硬条件（UI 不得为不完整来源显示任何硬条件结论）。

## 3. 后端 API 面（集成 HEAD 实核，以此为准）

- `POST /api/v1/career/opportunities/import-url`（`internal/router/routes_career.go:21` → `h.ImportURL`，`internal/modules/career/handler.go:261`）：body 上限 16KB；请求 `ImportURLInput {requestId, url}`；响应按 `ImportURLResult` 序列化（opportunityId/observationId/snapshotId/sourceStatus/completeness/failureCode/submittedUrl/acquiredAt/needsUserJD）。以 `internal/modules/career/source_import.go` 中类型定义与 json tag 为最终依据。
- `GET /api/v1/career/opportunities/:opportunityId/observations`（routes_career.go:24 → `h.OpportunityObservations`）：owner-scoped 不可变观察历史列表。
- `POST /api/v1/career/opportunities/import`（T08 既有，本轮已扩展）：仅接受可选 `opportunityId` 与 `priorObservationId`，用于 owner-scoped URL 观察 append；产生**新快照**，不改写原观察。
- 冻结枚举（后端逐字冻结，前端不得发明新值）：`sourceStatus = complete | partial | login_required | blocked | not_found | timed_out | fetch_failed | policy_unverified`；`completeness = complete | incomplete | unknown`；`failureCode ∈ {login_required, access_blocked, not_found, timeout, source_unverified, unsupported_content, empty_content, response_too_large, network_error, redirect_disallowed}`。
- 生产环境 source allowlist 为空：任何 URL 都返回 `policy_unverified` + `needsUserJD=true`——这是设计使然（真实来源核验属 T33 发布门槛），不是缺陷。

## 4. Desk/合同语义（冻结，违反即 Spec FAIL）

`packages/career-core/src/contracts.ts`（T03 拥有，你只消费不修改——若发现合同缺口如实报告，不擅自改）：`act` 结果未知时先 `receipt(requestId)` 恢复；切空间 abort 并清除缓存；同一 request ID 贯穿 unknown 恢复，**只有**后续手工粘贴才用新 request ID。`packages/api-client/src/career.ts` 现有 `importOpportunity`（T08）——你新增 `importUrl`/`opportunityObservations` 并按需扩展 `importOpportunity` 入参，序列化逐字对齐后端。

## 5. 验证命令与预期

```bash
pnpm --filter @weknora/web test -- src/career   # 或仓库等价的聚焦命令；先 RED 后 GREEN
pnpm typecheck:web
pnpm test:web          # 全量，须全绿 0 cancelled（当前基线全绿）
pnpm build:web
git diff --check
```
浏览器 fixture 验证：本地 Lite 服务器（模式见 `docs/plans/issue-140/task-3-live-http-validation.md` 与 `task-6-live-validation.md` 的复现提纲；**本轮端口分配 57803**，避开已用的 57690/62628/54512/57801），受控 source 响应（可用本地反向代理或 hosts 级 fixture，但绝不为演示把某 host 写入生产 allowlist——生产 policy 保持空）。真实浏览器登录后走：输入 URL → 显示 policy_unverified/需补 JD → 粘贴 JD → 新快照 → 原观察仍可追溯。截图/记录存 worktree 报告。

## 6. 全局约束（verbatim 摘录）

- Work only in the assigned isolated Worktree and only on files owned by the active Task.
- Never use cookies, login sessions, Chromium fallback, CAPTCHA solving, anti-crawler bypass, or client-declared source trust（这是后端约束，前端同样不得伪造来源状态或把 URL 当 Agent 指令）。
- Web 使用 WeKnora TDesign 浅色主题与 `#07c05f` 品牌色；不解析助手 prose。
- 本地 commits authorized; no push/merge/deploy/GitHub action；禁止派发子 agent。
- 报告只写事实与实测输出；环境不满足如实 blocked 附证据，绝不伪造。

## 7. 报告与完成

worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t09-web/` 下写 `task-2-report.md`（mkdir -p）：BASE/HEAD、RED/GREEN 证据、全量测试与构建输出、浏览器 fixture 证据（端口、截图清单、观测记录）、已知局限。最终消息报告 HEAD SHA 与全部验证结果。留在 worktree 等独立评审与集成；T09 是否 verified 由主控在后端遗留 medium（冻结分类断言，已另行登记跟踪）补齐后裁决。
