# Wave 1 — T09 后端子任务：Career Source Adapter And Durable URL Evidence（implement）

你是 backend_implementer。本简报自包含：只读本简报 + 下述计划/规范文件，不依赖任何父会话。**禁止派发子 agent。** 本任务只做后端子任务（T09 专用计划的 Task 1）；Web 子任务（Task 2）等你的合同 reviewed 并集成后另行派发，**不要写任何 Web 文件**。

## 1. Worktree 与 BASE

- 在集成工作区执行（只读）：
  ```bash
  git -C /Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01 worktree add /Users/wuyongjun/.codex/worktrees/issue-140-t09-source-import/WeKnora-fork01 --detach 21df162a24d73d9ec68d012458577050c18fd59f
  cd /Users/wuyongjun/.codex/worktrees/issue-140-t09-source-import/WeKnora-fork01
  ```
- BASE = `21df162a24d73d9ec68d012458577050c18fd59f`（当前集成 HEAD，工作区干净，实测 `git status --short` 为空）。detached HEAD 有意为之；本地 commit 允许，绝不 push/merge、绝不改集成分支。

## 2. 主计划 Task 9 文本（verbatim 提取）

### Task 9: T09/#149 链接导入与不完整来源回退

**Depends:** #146。**Owner/validator:** backend_implementer + frontend_implementer / backend_validator + frontend_validator。**Files:** `internal/modules/career/service/source_import.go`、`internal/modules/career/service/source_import_test.go`、`apps/web/src/career/source_import.tsx`。**Consumes:** 已集成的前置 Task 合同与认证 scope。**Produces:** `CareerRemote.act({kind: "import_url", payload}, requestId, expectedRevision): Promise<CareerReceipt>`。**Parallel:** 仅与同 DAG 波次且文件、数据库、端口和构建目录隔离的 Task 同时执行；否则串行。

**验收：** 只接入经核验允许读取的来源，不绕过登录或反爬限制；保留原链接、取得时间、完整性及失败原因；摘要不足以推断届别、学历等硬条件；用户补充 JD 后产生新的固定快照，原始来源仍可追溯

- **Step 1:** 先在 `source_import_test.go` 写服务端公共 seam 的失败测试，逐条覆盖本 Task 验收与 request ID、revision、Tenant 隔离；运行 `go test ./internal/modules/career/...`，预期 RED。
- **Step 2:** 在 `source_import.go` 实现最小持久业务行为及封闭 intent；输入只消费已确认事实和不可变快照，外部副作用用收据对账，不能把未知结果当成功。
- **Step 3:** （Web，本轮不做）
- **Step 4:** 运行 `go test ./internal/modules/career/...`；保存 API fixture、数据库迁移证据；后端验证，独立 reviewer 给出 Spec 与质量结论后提交。

**验证命令（本轮范围）：** `go test ./internal/modules/career/...`。**原始证据：** 固定来源响应契约覆盖完整、登录阻断、摘要、不存在和超时。**失败处理：** 来源未知时不标为已核验；重试不覆写历史快照。

注意：主计划写的文件路径 `internal/modules/career/service/source_import.go` 与现行 Career 模块布局（T08 后实际文件在 `internal/modules/career/` 包根，如 `opportunity.go`、`office.go`、`handler.go`）不同。以 T09 专用计划的文件清单为准（见下），即 `internal/modules/career/source_import.go`。这是既定裁决，不要自行发明第三种路径。

## 3. 权威执行计划（全文遵守）

`/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01/docs/plans/2026-09-24-issue-140-t09-source-import.md`（提交 2a0f7c631 定义）。其 Task 1 要点（与其冲突时以计划文件为准）：

**Files**：create `internal/modules/career/source_import.go`、`source_import_test.go`；modify `internal/modules/career/opportunity.go`、`office.go`、`handler.go`、focused handler tests；modify `internal/router/routes_career.go` and route tests；create migration `000198_source_import_observations`（versioned + SQLite 两树）；extend `internal/database/career_migration_test.go`； Career 自有的窄 transport adapter（优先放 `source_import.go` 内，消费接口而非 `web_fetch.NewFetcher`）。

**冻结接口**：
```go
type SourcePolicy interface {
    Verify(rawURL string) (ApprovedSource, error)
    VerifyRedirect(current, next string) error
}
type SourceTransport interface {
    Fetch(ctx context.Context, source ApprovedSource, rawURL string) (SourceFetchResult, error)
}
type ImportURLInput struct {
    RequestID string `json:"requestId"`
    URL       string `json:"url"`
}
type ImportURLResult struct {
    OpportunityID string    `json:"opportunityId"`
    ObservationID string    `json:"observationID"` // 以计划文件字段表为准：observationId
    SnapshotID    string    `json:"snapshotId"`
    SourceStatus  string    `json:"sourceStatus"`
    Completeness  string    `json:"completeness"`
    FailureCode   string    `json:"failureCode,omitempty"`
    SubmittedURL  string    `json:"submittedUrl"`
    AcquiredAt    time.Time `json:"acquiredAt"`
    NeedsUserJD   bool      `json:"needsUserJD"`
}
```
冻结枚举：`sourceStatus = complete | partial | login_required | blocked | not_found | timed_out | fetch_failed | policy_unverified`；`completeness = complete | incomplete | unknown`；failure codes 恰为 `login_required, access_blocked, not_found, timeout, source_unverified, unsupported_content, empty_content, response_too_large, network_error, redirect_disallowed`。

**HTTP**：`POST /api/v1/career/opportunities/import-url`；`GET /api/v1/career/opportunities/:opportunityId/observations`；`POST /api/v1/career/opportunities/import` 仅接受可选 `opportunityId` 与 `priorObservationId`（owner-scoped URL observation append）。

**九个命名 RED 测试**（名字逐字，见计划 Step 1）：
```
TestImportURLCompleteCreatesImmutableOpportunityEvidence
TestImportURLPolicyUnverifiedDoesNotFetchAndRequestsJD
TestImportURLLoginSummaryMissingAndTimeoutClassifications
TestImportURLRejectsUnapprovedOrPrivateRedirect
TestImportURLDoesNotInferHardFieldsFromPartialText
TestImportURLReplayConflictAndConcurrentSingleObservation
TestManualJDAfterURLCreatesNewSnapshotAndPreservesTrace
TestImportURLScopeAndErrorSanitization
TestSourceObservationMigrationUpAndDown
```

**实现要点**：additively 扩展 `career_opportunity_observations`（`source_status`、`completeness`、`failure_code`、`submitted_url`、`final_url`、`adapter_id`、`adapter_version`、`observed_http_status`），保留既有列/行有效，down 只删新增列/索引；失败观察也落 snapshot 行（空文本、空字节 SHA-256、`needs_review`、显式 unavailable）；更新 `NewOffice` 模型校验与 SQLite shape 检查；复用/提取 T08 的私有持久化 helper 保持不可变 append 语义；**网络 I/O 绝不在 DB 事务内**；fetch 前 claim scoped request，fetch 后 reconcile，原子插入 observation+snapshot+terminal receipt；生产 policy 为空实现（一律 `policy_unverified`），测试 policy/transport 注入；仅 adapter 特定正向完整性检查才判 complete；transport 失败映射为有界 failure code；不泄漏原始上游错误/私有 IP/redirect 细节/凭据。

**Step 4 VERIFY（全跑并附原始输出）**：
```bash
gofmt -w internal/modules/career internal/router
go test ./internal/modules/career/... -count=1
go test ./internal/database/... -count=1
go test ./internal/router/... -count=1
go test ./tools/architectureguard/... -count=1
git diff --check
git add internal/modules/career internal/router migrations/versioned/000198_source_import_observations.* migrations/sqlite/000119_source_import_observations.* internal/database tools/architectureguard
git commit -m 'feat(career): record URL source evidence'
```

## 4. 迁移编号分配（本轮唯一分配，勿改）

- 你使用 **versioned `000198_source_import_observations` / SQLite `000119_source_import_observations`**。
- 实核依据（调度员 2026-09-25 `ls migrations/versioned | sort | tail` 与 `ls migrations/sqlite | sort | tail`）：集成 HEAD 21df162a2 实际最大为 versioned `000195_career_evaluations` / sqlite `000116_career_evaluations`。`000196/000117` 已被 T14 子任务1（checkpoint c67cec29d，未集成）占用，`000197/000118` 已被 T14 子任务2 brief 预留。你的基线里没有 196/117、197/118——**这是预期现象，不要改用这两个编号**；集成时它们会按序排在你的 198/119 之前。
- 注意：主台账曾记录"已用到 197/118"，那是对预留的记载；以上述目录实核为准。

## 5. CareerRemote 冻结合同（消费语义，不重定义）

`packages/career-core/src/contracts.ts` 由 T03 拥有；封闭 intent 集含 `import_url`。本任务的服务端行为必须与下述合同语义一致（Go 服务端 scope 从认证上下文生成，绝不信任客户端 `CareerScope` 字段；同一 request ID 内容变化拒绝；所有写入使用 request ID 与 expected revision）：
```ts
type CareerReceipt = { requestId: string; revision: number; resultRef: CareerRef; status: "applied" | "pending" };
interface CareerRemote {
  open(ref: CareerRef, signal?: AbortSignal): Promise<unknown>;
  list(kind: CareerRef["kind"], cursor?: string, signal?: AbortSignal): Promise<{ items: unknown[]; nextCursor?: string }>;
  act(intent: CareerIntent, requestId: string, expectedRevision: number): Promise<CareerReceipt>;
  changes(cursor?: string, signal?: AbortSignal): Promise<{ events: unknown[]; nextCursor?: string }>;
  receipt(requestId: string): Promise<CareerReceipt | null>;
}
```

## 6. 全局约束（verbatim，违反即 Spec FAIL）

T09 计划 Global Constraints：
- Work only in the assigned isolated Worktree and only on files owned by the active Task.
- Production source allowlist starts empty. No recruiting domain may be marked vetted without an authorized source-review record; unapproved URLs produce durable `policy_unverified` evidence and request user JD.
- Never use cookies, login sessions, Chromium fallback, CAPTCHA solving, anti-crawler bypass, or client-declared source trust.
- Preserve the exact submitted URL, attempt/acquisition time, source status, completeness, and bounded failure code. Do not leak raw upstream errors, private IP/redirect details, or credentials.
- Partial, login, blocked, missing, timed-out, and unverified observations never infer graduation, degree, or other hard conditions; all extracted hard fields remain `unknown`/`needs_review`.
- User JD append creates a new fixed snapshot and manual observation; it never rewrites or replaces the original URL observation.
- Network I/O must occur outside a database transaction. Durable request claims/receipts reconcile concurrent and unknown outcomes with the original request ID.
- Add only forward migrations numbered after the integrated T14 migrations; expected next files are versioned `000198_source_import_observations` and SQLite `000119_source_import_observations`.
- Local commits are authorized; no push/merge/deploy/GitHub action.

主计划 Global Constraints（相关项）：Career Office 仅接受服务端认证所得 User/Tenant 作用域；所有写入使用 request ID 与 expected revision，同一 request ID 内容变化拒绝；不绕过 CAPTCHA/登录；所有实现者只修改任务所有权内的文件。

另（现场规则）：严禁改动主仓库 `/Users/wuyongjun/trea/WeKnora-fork01`（尤其 `.worktrees/issue30-sweep`）；数据库查询一律参数绑定；报告只写事实与实测输出；环境不满足时如实 blocked，绝不伪造。

## 7. Review Focus（独立评审按此查你）

- An unapproved host must be recorded without any network attempt and must return `policy_unverified` with `needsUserJD=true`.
- A redirect from an approved host to an unapproved or private host must be rejected at the redirect, not retroactively trusted.
- A summary page containing degree words must still leave all hard fields unknown.
- URL failure followed by manual paste must leave both observations open and produce distinct snapshot IDs.
- Same request ID with changed URL or linkage conflicts; concurrent identical requests append only one observation.

## 8. 路由清单义务

新增 2 条路由（`import-url`、`observations` 列表）+ 扩展既有 `import` 后，必须同步更新 `tools/architectureguard` 的 Career 清单与路由计数到精确新值并跑 `go test ./tools/architectureguard/...`（T08/T10 已有同类先例，参考其集成方式）。未更新清单会阻塞集成。

## 9. 报告与完成

在 worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t09-source-import/` 下写 `task-1-report.md`（mkdir -p）：BASE/HEAD SHA、RED 证据（先跑失败输出）、GREEN 全量 VERIFY 原始输出、设计取舍、已知局限（如无 vetted 真实来源是设计使然，T33 另行把关）。最终消息报告：新 HEAD SHA、全部验证命令结果、迁移编号使用。留在 worktree 等独立评审，不集成、不宣布 T09 完成。
