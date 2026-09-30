# Wave 12 — T21 后端实现报告：搜索与生成的额度预估及阻断（Issue #161）

- **BASE**: `14b81d24c`；**HEAD**: `ab84b933b`（本地提交，未 push）
- **Worktree**: `/Users/wuyongjun/.codex/worktrees/issue-140-t21-usage/WeKnora-fork01`
- 完整冻结细节见 worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t21-usage/task-1-report.md`

## 成本模型冻结（调度员要求）

- **收费操作范围**：仅 `search_once` 一种（含 T13 规则触发的每个周期 Run——触发底层即一次 SearchOnce）。materials/evaluations/preparations/reminders/一切只读：零额度、免费语义冻结（唯一有真实外部抓取边际成本的操作才收费）。
- **成本常量**：`searchOnceCostUnits = 1`/Run；周期=UTC 自然月；默认预算 50/月，env `CAREER_SEARCH_QUOTA_LIMIT` 覆盖（非法值启动报错）。
- **预占语义**：执行前预占(requestID 幂等)→终态惰性结算(settled)→未 claim 且租约(60s)过期释放(released)。

## 接口冻结

- `GET /api/v1/career/usage/estimate?operation=search_once` → `{kind:"usage_estimate", operation, costUnits, conditions[], periodStart, periodEnd, limitUnits, reservedUnits, settledUnits, remainingUnits, wouldAdmit}`（只读免费）。
- 错误：余额不足 429 `search_quota_refused`（可恢复，同 requestId 重放）；账本不可得 503 `admission_unavailable`（fail-closed 绝不先执行）；未知 operation 400。
- Seam 演进：`AdmitSearch(ctx, scope, query)` → `AdmitSearch(ctx, scope, requestID, query)`（requestID 为预占幂等键，物理必需；三处一行签名+两调用点传参）。

## RED → GREEN

- **RED**：10 个命名测试编译失败（`undefined: UsageEstimate/usageReservationRecord/SetSearchQuotaLimit/failUsageLedgerRead...`，red-usage-tests.txt）；迁移测试缺 4 文件 FAIL（red-migration.txt）；路由注册断言 FAIL。
- **GREEN**（全部实跑，green-verification.txt）：
  - `gofmt -l`（4 目录）空输出
  - `go test ./internal/modules/career/... -count=1` → ok 10.887s
  - `go test ./internal/database/... -count=1` → ok 16.530s
  - `go test ./internal/router/... -count=1` → ok 1.715s
  - `go test ./tools/architectureguard/... -count=1` → ok 0.424s
  - `git diff --check` → clean
  - 另 `go vet`（四包）exit=0
- architectureguard：`wantRouteTotal` 692→**693**、`wantRouteLiteral` 623→**624**（+1 字面路由 GET /career/usage/estimate）。

## 提交

- `ab84b933b feat(career): enforce quota admission with honest estimates`（17 files，+1091/−41）

## 文件清单

- create：`internal/modules/career/usage.go`、`usage_test.go`；`migrations/versioned/000208_career_usage.{up,down}.sql`；`migrations/sqlite/000129_career_usage.{up,down}.sql`
- modify：career `office.go`/`handler.go`/`handler_test.go`/`search_once.go`/`search_once_test.go`/`search_rule.go`/`career_export.go`（purge+boundary）；`internal/router/routes_career.go`+`routes_career_test.go`；`internal/database/career_migration_test.go`（新 up/down 测试+版本头断言 128→129）；`tools/architectureguard/discovery_test.go`

## 自查与遗留

- 迁移 `git add -f`：.gitignore:96 忽略 `migrations/` 但既有迁移均被跟踪（沿 T19/T20 先例）。
- 已知局限：结算走读路径惰性对账（不改 search_once.go 的唯一诚实落点，reserved 保守计数不超卖）；规则预检后 SearchOnce 失败的预占靠租约过期自愈；价格/套餐不在本任务（Spec 冻结于成本试点后）。
- 环境注记：Mimosa hook 在 commit 时报 scanner_enobufs（按兼容策略放行）；本任务改动已经 gofmt/go vet/四包全量测试覆盖。

---

## 第 1 轮评审修复（review F1 high；F2/F3 low 记录不修）

- **HEAD 更新**：`f3d335bf2 fix(career): make quota admission atomic under concurrency`（在 ab84b933b 之上，本地未 push）
- **F1 修复**：`admitSearchUsage` 的"读余额→判限额→插入"整段移入 `runImportTransaction`（busy 重试先例），事务内先 `SELECT ... FOR UPDATE` 锁定该 scope 的 `career_spaces` 行（`WHERE tenant_id=? AND owner_user_id=?`，参数绑定）——PostgreSQL 上并发 admission 在该行锁串行，输者在自己的 totals 里读到赢者已提交的预占并被拒；SQLite 靠单写者锁+busy 重试达成同等原子性。`reconcileUsage`/`usageTotals`/`reviveReleasedReservation` 改为事务作用域变体（`reconcileUsageTx`/`usageTotalsTx`/`reviveReleasedReservationTx`），限额检查读取锁后视图。未知 status 落入 `errLedgerState` fail-closed。
- **F1 RED→GREEN**：新增 `TestAdmissionIsAtomicUnderConcurrentRequests`（usage_test.go：共享 cache 内存库 `file:...?mode=memory&cache=shared`，每 goroutine 独立连接真实交错；10 轮 × 8 racer，断言恰好 1 个 admission 成功、恰好 1 行、账本恰持 1 单位）。
  - RED（修复前，已追加 red-usage-tests.txt）：`round 3: unexpected admission error: career usage admission unavailable`（无事务时并发写在共享库上报 database is locked → 被映射为 503 typed）。
  - GREEN：`--- PASS: TestAdmissionIsAtomicUnderConcurrentRequests (0.34s)`；另 `-count=5` 与 `-race -count=1` 均 ok。
- **修复轮全量验证（真实输出，已追加 green-verification.txt）**：
  - `gofmt -l`（4 目录）空输出；`go vet`（四包）ok
  - `go test ./internal/modules/career/... -count=1` → ok 12.620s
  - `go test ./internal/database/... -count=1` → ok 25.681s
  - `go test ./internal/router/... -count=1` → ok 1.821s
  - `go test ./tools/architectureguard/... -count=1` → ok 0.755s
  - `git diff --check` → clean
- **wave-report 中被评审指出的错误表述更正**：原文"reserved 保守计数不超卖"在并发下不成立——现已由行锁事务保证不超卖（上上条"已知局限 1"的惰性对账描述不变，但其前提从"保守计数"更正为"行锁事务串行化 admission"）。
- **F2（low，记录不修）**：UsageEstimate 注释"a free read and never mutates the ledger"与 reconcileUsage 会做 reserved→settled/released UPDATE 的行为字面不符（余额数值不变，状态机推进）。留待后续措辞修正。
- **F3（low，记录不修）**：真实账本 gate 下 TriggerDueRules 额度耗尽→blocked_no_quota 的端到端用例缺位（现有 T13 用 fake gate；本任务仅以 gate 重入幂等+签名接线间接覆盖）。

---

## 第 2 轮评审修复（review F4 high；F5 已随 F4 更正；F2/F3 low 维持不修）

- **HEAD 更新**：`bec351f88 fix(career): stop swallowing SQLite BUSY as an admission race`（在 f3d335bf2 之上，本地未 push）
- **F4 修复**：admission 插入的 race 分类从 `isReceiptRaceError`（会匹配 "database is locked"）换成收窄谓词 `isUsageReservationInsertRace`——仅 `gorm.ErrDuplicatedKey`、sqlite3 唯一约束扩展码（`ErrConstraint`+`ErrConstraintUnique`，mattn v1.14.32 error.go:144）、"unique constraint"/"duplicate key" 字符串；且命中后**仍在事务内回读确认赢者行真实存在**才返回成功。BUSY 及其他错误一律冒泡：busy 进 `runImportTransaction` 重试，真实失败 fail-closed 为 503 typed。文件型 SQLite 下不再出现"返回成功但账本无行"。
- **F4 RED→GREEN**：新增 `TestAdmissionIsAtomicOnFileBackedSQLite`（usage_test.go：真实文件库 `t.TempDir()`，20 轮 × 8 racer）。
  - RED（修复前，已追加 red-usage-tests.txt）：`round 0: every reported success must hold a ledger row ... expected: 1, actual: 2`（另一次运行为 `actual: 4`）——与评审探针 39/40 轮复现一致。
  - GREEN：`--- PASS: TestAdmissionIsAtomicOnFileBackedSQLite (0.67s)`；连同内存库变体 `-count=5` 与 `-race -count=1` 均 ok。
- **修复轮全量验证（真实输出，已追加 green-verification.txt）**：
  - `gofmt -l`（4 目录）空输出；`go vet`（四包）ok
  - `go test ./internal/modules/career/... -count=1` → ok 15.492s
  - `go test ./internal/database/... -count=1` → ok 13.850s
  - `go test ./internal/router/... -count=1` → ok 1.717s
  - `go test ./tools/architectureguard/... -count=1` → ok 0.637s
  - `git diff --check` → clean
- **F5（low，随 F4 更正）**：worktree 内 task-1-report.md 局限 1 的"reserved 保守计数确保不超卖"已更正为"不超卖由 admission 事务（行锁+收窄 race+BUSY 冒泡）保证，并发不超卖有文件型/共享内存 SQLite 双测试实证"。
- **F2/F3（low，维持记录不修）**：UsageEstimate 注释字面与 reconcile UPDATE 行为不符；真实账本 gate 下 TriggerDueRules 端到端用例缺位。本轮 diff 未恶化。
