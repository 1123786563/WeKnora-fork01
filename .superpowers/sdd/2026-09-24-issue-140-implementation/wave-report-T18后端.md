# Wave 8 — T18 后端任务报告：本人投递确认与实际材料绑定（Issue #159，implement）

- **BASE**：`24708145c`（集成 HEAD，wave 7 收官）
- **HEAD**：`dfd411bb4`（本地提交，未 push）
- **Worktree**：`/Users/wuyongjun/.codex/worktrees/issue-140-t18-submission/WeKnora-fork01`（独立、detach）
- **提交**：`feat(career): record user-confirmed submissions with bound versions`（13 files, +1318/−21）
- **证据文件**：`red-evidence.txt`、`green-evidence.txt`（本目录）

## 1. RED → GREEN 证据

### RED（先写测试、运行并记录）

1. **office 层 9+1 个命名测试**写入 `internal/modules/career/submission_test.go` 后运行
   `go test ./internal/modules/career/... -count=1` → **build failed**：
   `undefined: RecordSubmissionInput`、`o.RecordSubmission undefined (type *Office has no field or method RecordSubmission)`、
   `undefined: SubmissionChannelEmail`、`undefined: SubmissionKindRecorded` 等（API 尚不存在，测试先行）。
2. **迁移测试** `TestSubmissionMigrationUpAndDown` 写入 `internal/database/career_migration_test.go` 后运行 →
   `--- FAIL: ... unable to find file ".../migrations/versioned/000204_career_submissions.up.sql"`（迁移尚未创建）。
3. **house 语义补充** `TestRecordSubmissionUnknownOutcomeRecoversViaReceipt` 写入后运行 → **build failed**
   （`afterSubmissionPersist` hook 尚不存在）。

以上均存档于 `red-evidence.txt`。

### GREEN（最小实现 + 重构后全量验证，2026-09-25 21:33）

| 命令（照简报 §4 原样执行） | 输出 |
| --- | --- |
| `gofmt -w internal/modules/career internal/router`（另对本任务触碰的 `tools/architectureguard`、`internal/database` 一并 gofmt） | 完成（submission.go 被格式化） |
| `go test ./internal/modules/career/... -count=1` | `ok github.com/Tencent/WeKnora/internal/modules/career 11.906s` |
| `go test ./internal/database/... -count=1` | `ok github.com/Tencent/WeKnora/internal/database 24.550s` |
| `go test ./internal/router/... -count=1` | `ok github.com/Tencent/WeKnora/internal/router 3.122s` |
| `go test ./tools/architectureguard/... -count=1` | `ok github.com/Tencent/WeKnora/tools/architectureguard 0.561s` |
| `git diff --check` | 无输出（clean） |

10 个命名 RED 测试全部存在且通过（office 9 个 + `TestSubmissionMigrationUpAndDown`），
另有补充：`TestRecordSubmissionUnknownOutcomeRecoversViaReceipt`（house 未知回执恢复）、
`TestCareerSubmissionHTTPContract`（handler 聚焦）、`TestCareerSubmissionRoutesAreRegistered`（route 聚焦）。

**重构轮**：`FindSubmissionReceipt` 从复用 replay helper 改为直查（消除 `Committed()` 导出方法与双重错误路径）；
hook 化的 `afterSubmissionPersist` 与 progress 先例同构。重构后全量验证重跑通过。

## 2. 冻结的业务语义（结构由本报告冻结）

1. **纯记录、零外部动作**：`RecordSubmission` 只持久化"用户声明的投递事实"。服务端代码无任何发信/点击/填表/
   邮箱读取路径；也无任何"从下载推断投递"的代码。测试 `TestRecordSubmissionNeverPerformsOrInfersExternalAction`
   在代码层证明：兑换 3 次下载授权后 `career_submissions` 仍为空；安装"一调用即置位"的 transport 与零调用计数
   linker 后记录成功且两者零触碰；导出存储文件数不变；无 progress 事件被自动追加。
2. **渠道封闭枚举**：`email` / `web` / `other`，由用户选择；非法渠道 `ErrInvalidRequest`。
3. **时间**：`occurredAt` 为用户声明时间（可缺省=记录时刻），与渠道一并冻结进回执与行。
4. **版本二选一（显式）**：
   - 绑定已验证版：必须给 `materialId` + `exportId`，且该 export 在记录时刻为 `submittable` 且未 revoked
     （staged/failed/revoked → `ErrExportNotSubmittable`；不存在 → `ErrExportNotFound`）。绑定四元组
     `(materialId, exportId, version, contentDigest)` 冻结写入行。
   - 显式未知：`versionUnknown: true`（不允许同时携带版本引用），记录 `versionConfirmed=false` 且引用列全空，
     绝不虚构版本。
5. **重复确认不二建（语义冻结：typed conflict）**：同一 application 最多一条 `career_submissions`
   （UNIQUE(tenant,user,application) + 事务内先查）。同 request ID 精确 replay 返回原回执；同 request ID 变更内容
   → `ErrIdempotencyConflict`；**不同 request ID 再确认 → `ErrSubmissionAlreadyConfirmed`
   （HTTP 409 `submission_already_confirmed`）**。选择 typed conflict 而非"返回既有记录"的理由：换 request ID
   返回既有记录会让新 request ID 永远无回执可 replay（后续 receipt 查询 404），并把两个请求的幂等台账混写；
   typed conflict 迫使客户端经 `GET applications/:id/submissions` 显式对账。
6. **版本引用不可改写且精确解析**：行内绑定列只在插入时写入；后续新版本/新导出/撤销导出（revoke 改的是 export
   状态，不动 submission 行）后 `SubmissionBoundVersion` 仍返回记录时的精确四元组，行 byte 级不变
   （`TestRecordSubmissionImmutableVersionReferenceResolvesExactly`）。`MaterialVersion(material, version)`
   由 T15/T16 的不可变版本表保证精确解析。
7. **house 语义**：request ID replay/conflict、`expectedRevision` 对 profile head 的 CAS（与 applications/
   materials/exports 先例一致；冲突返回 `RevisionConflictError{CurrentRevision}`，HTTP 409 带 `currentRevision`；
   记录不推进 profile revision）、scope 全部从认证上下文派生（他租户/同租户他人分别得 404/403，无存在性泄漏）、
   未知回执恢复（事务后取消 → `OutcomeUnknownError`，同 request ID 重试安全且恰一条记录）。

## 3. 与 progress 事件的投影取舍（冻结）

**决定：record_submission 不自动投影为 progress 事件。** 理由：

1. progress 事件有独立幂等台账（request ID 作用域）与按 application 的 revision CAS；自动追加需要合成第二个
   request ID 或在一个事务里耦合两套 CAS/恢复语义，破坏两侧 replay 语义。
2. Spec 中 progress 事件是"用户录入的事实 + 更正语义"；submission 是带版本绑定的独立事实表。若自动投影，
   progress 的更正（correct）会间接"改写"投递事实的呈现，违背本任务"引用不被改写"的验收。
3. 记录会推进 progress revision，令"仅确认投递"的客户端遭遇意外的 revision 冲突。
4. 展示层的阶段投影可由 Web 在读侧合并（submissions + progress 两个只读视图），不需要写入侧耦合。

测试侧以 `TestRecordSubmissionNeverPerformsOrInfersExternalAction` 断言记录后 progress 事件数为 0 固化该决定。

## 4. 接口冻结（HTTP + JSON，供 Web/小程序消费）

### 路由（+3，architectureguard 609/678 → 612/681）

| 方法 | 路径 | 处理器 |
| --- | --- | --- |
| POST | `/api/v1/career/applications/:applicationId/submissions` | `Handler.RecordSubmission` |
| GET | `/api/v1/career/applications/:applicationId/submissions` | `Handler.ApplicationSubmissions`（响应 `{"submissions":[...]}`） |
| GET | `/api/v1/career/submissions/receipt?requestId=...` | `Handler.SubmissionReceiptHandler` |

### 请求体（POST，≤16KB，`DisallowUnknownFields`，body applicationId 与路径不符 → 400）

```json
{
  "requestId": "string ≤128 必填",
  "applicationId": "可省（取路径参数）；若给必须与路径一致",
  "channel": "email|web|other",
  "occurredAt": "RFC3339 可选，缺省=记录时刻",
  "materialId": "与 exportId 成对，二选一于 versionUnknown",
  "exportId": "同上",
  "versionUnknown": true,
  "note": "≤4096 可选",
  "expectedRevision": 0
}
```

### 回执（SubmissionReceipt，POST 200 / receipt 200）

```json
{
  "kind": "submission_recorded",
  "requestId": "...", "applicationId": "...", "submissionId": "uuid",
  "channel": "email", "occurredAt": "2026-09-22T08:30:00Z",
  "versionConfirmed": true,
  "boundVersion": { "materialId": "...", "exportId": "...", "version": 1, "contentDigest": "sha256hex..." },
  "note": "", "confirmer": "<认证用户>", "revision": 3, "createdAt": "..."
}
```
（未知版本时 `versionConfirmed:false` 且无 `boundVersion` 字段。）

### 错误码（writeError 映射）

| 场景 | HTTP | code |
| --- | --- | --- |
| 渠道/参数/未知字段非法 | 400 | `invalid_request` |
| application 不在本 scope | 404 | `not_found` |
| export 不存在 / 绑定引用不可见 | 404 | `not_found` |
| export 非 submittable/已 revoked | 409 | `export_not_submittable` |
| 重复确认（新 request ID） | 409 | `submission_already_confirmed` |
| 同 request ID 内容变更 | 409 | `idempotency_conflict` |
| expectedRevision 过期 | 409 | `revision_conflict`（附 `currentRevision`） |
| 未知结局 | 504 | `outcome_unknown`（附 `requestId`） |

### Office seam（供后续后端任务消费）

`RecordSubmission(ctx, RecordSubmissionInput) (SubmissionReceipt, error)`、
`FindSubmissionReceipt(ctx, requestID)`、`ApplicationSubmissions(ctx, applicationID)`、
`SubmissionBoundVersion(ctx, submissionID) (SubmissionVersionBinding, error)`（未确认版本 → `ErrSubmissionNotFound`）。

## 5. 迁移

- `migrations/versioned/000204_career_submissions.up/.down.sql`（PG 风格）
- `migrations/sqlite/000125_career_submissions.up/.down.sql`
- 表 `career_submissions`：`UNIQUE(tenant_id,user_id,application_id)` + `UNIQUE(tenant_id,user_id,request_id)` +
  索引 `idx_career_submission_scope`；down 仅 drop 本表。
- `TestSubmissionMigrationUpAndDown` 验证：head=125、列齐全、两条唯一约束生效、down→124 保留全部既有 career 表、
  再 up 恢复空表。既有迁移测试的 head 断言 124→125 一并更新（16 处，均为"升到 head"的断言；down 目标断言不变）。
- `NewOffice` 模型清单加入 `submissionRecord`；`validateSQLiteCareerSchema` 增加 career_submissions 列与两条
  唯一约束校验（部分启动保护测试随之覆盖新表）。

## 6. 文件清单（所有权核对，全部在简报声明范围内）

| 文件 | 动作 |
| --- | --- |
| `internal/modules/career/submission.go` | 新建（领域模型 + Office seam，~490 行） |
| `internal/modules/career/submission_test.go` | 新建（10 个 office 层测试，其中 9 个为命名 RED + unknown 恢复） |
| `internal/modules/career/office.go` | 修改（模型清单、SQLite schema 校验、`afterSubmissionPersist` hook） |
| `internal/modules/career/handler.go` | 修改（writeError 两映射 + 3 个 handler + 16KB 限额） |
| `internal/modules/career/handler_test.go` | 修改（聚焦 HTTP contract 测试） |
| `internal/router/routes_career.go` | 修改（+3 路由） |
| `internal/router/routes_career_test.go` | 修改（聚焦 route 注册测试） |
| `migrations/versioned/000204_career_submissions.{up,down}.sql` | 新建（分配编号 000204） |
| `migrations/sqlite/000125_career_submissions.{up,down}.sql` | 新建（分配编号 000125） |
| `internal/database/career_migration_test.go` | 扩展（新迁移测试 + head 124→125） |
| `tools/architectureguard/discovery_test.go` | 修改（609→612、678→681 + 历史注释） |

未触碰主仓库与其他 worktree；未 push；未改动 `.worktrees/issue30-sweep`。

## 7. 自查与已知局限

- **零外部动作（代码层确认）**：`submission.go` 全文无 `sourceTransport`、`linker`、`exportStorage`、任何
  net/http 或邮件调用；唯二的"外部世界"接触是参数化 SQL 读写 `career_submissions`。
- **未跑 pnpm install**：本任务验证命令全部为 Go 工具链（简报 §4），worktree 无 node_modules 也不影响；
  Web 子任务接入时再按需安装。
- **已知局限（如实声明）**：
  1. 一个 application 终身只允许一条投递记录（UNIQUE 硬约束）。若产品后续需要"撤回后重投"产生第二条绑定，
     需新迁移放宽数据模型（如引入 voided 状态 + 部分唯一索引）；当前语义下重投以 progress `resubmitted`
     事件表达，版本绑定仍指向首次确认的版本。
  2. `SubmissionBoundVersion` 对"未知版本"记录返回 `ErrSubmissionNotFound`——下游准备消费未确认投递时应先读
     回执的 `versionConfirmed`。
  3. 版ed 迁移（000204，PostgreSQL 风格）按既有先例编写，但本环境无 PG 实例，仅通过 sqlite 迁移与
     `NewOffice` schema 校验实证；PG 侧靠语法同构与既有 000200-000203 先例保证（与 T16/T17 同一验证口径）。
  4. `bindProgressApplication` 复用自 progress handler（参数名通用，行为是"路径与 body 的 applicationId
     一致性校验"），未重命名以免无谓扰动 progress 面。
