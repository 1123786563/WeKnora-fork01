# #140 后端 OCR round-4 独立验证

- Revision: `76df0cee0bf3ae23c14411c345b151ad518077ee` (`docs(plan): #140 主控终裁…`)
- Workspace: `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`
- Result: **DONE_WITH_CONCERNS** — 所核后端高/中 finding 均有当前代码支持；至少两项是可达的契约缺口。
- Scope: #140 career/workbench/API server 后端；不核小程序 N8/N9。只读代码，唯一新增文件为本报告。

## Findings

| Item | Result | Evidence and impact |
|---|---|---|
| `career_export.go:643` search-rule boundary promises runs and discovery todos travel in the complete export, then are deleted | **VALID / acceptance gap** | `CareerExportArchive` (`career_export.go:216-231`) and `buildCareerExportArchive` (`:523-550`) carry `SearchRules` and `Reminders`, but have no run or discovery-todo fields and do not query `career_search_rule_runs` / `career_search_discovery_todos`. `careerPurgeTables` (`:715-725`) deletes both tables. Boundary description at `:643` therefore promises recoverability the export does not provide. Existing `TestExportCareerArchiveCarriesPreparationsSearchRulesAndReminders` (`career_export_test.go:598-646`) seeds and asserts a rule and reminder, but not runs/todos; it does not catch this gap. |
| N6, `handler.go:171`: raw internal error text in 500 response | **VALID / security-medium** | `writeError` initializes unknown errors as 500/internal (`handler.go:87`) and unconditionally serializes `e.Error()` at `:171`. Thus unclassified infrastructure errors (the report's example is storage backend error from export download) can disclose backend/bucket/endpoint detail to the caller. The same handler has explicit public messages for some known errors, but no generic sanitization. No targeted assertion for an unknown error's response body was found in `handler_test.go`. |
| N10, `ErrUploadClaimLost` escaping from cleanup as 500/internal | **VALID / medium contract gap** | `ClearSourceResource` returns `ErrUploadClaimLost` when its guarded update affects no row (`profile_intake.go:126-137`). `finishClaimFailure` returns that error directly when cleanup fails (`handler.go:1813-1826`); unlike its earlier `FinishSourceClaim` branch (`:1807-1811`), it does not reload latest source / mark superseded. `writeError` has no `ErrUploadClaimLost` case (`:87-170`), so it falls through to 500/internal. Existing claim-lost branches cover other upload stages (`handler.go:1673-1678, 1697-1698, 1732-1734, 1756-1760`), but no cleanup-stage regression test was found. |
| N11, `reminder.go:262-269`: concurrent SetReminder for same source and distinct request IDs | **VALID / medium contract gap** | `attemptReminderWrite` checks for an existing source reminder before it locks the profile (`reminder.go:286-317`), then inserts under unique `(tenant,user,source_kind,source_id)` (`reminder.go:346-369`; model index `:143-157`). Concurrent transactions can both observe no reminder; the loser can fail the unique insert. `SetReminder` classifies this as ambiguous and replays only its own request ID (`:225-258`); a different request ID cannot find the winner's receipt, after which the original transaction error is returned (`:259-269`). `isReceiptRaceError` recognizes uniqueness (`office.go:836-842`) but does not make this source-level race resolve to the winner receipt or a typed outcome. Existing reminder tests cover sequential dedupe and exact replay (`reminder_test.go:93-117,325-345`), not this distinct-request concurrent race. |

No other career/workbench/API-server high or medium finding surfaced in the provided round-4 analyses within this validation's backend scope. The high-risk report's remaining high findings are web/miniprogram/client findings; the resume-2 report's unrelated medium findings are out of scope.

## Verification evidence

- Exact command: `git rev-parse HEAD` → `76df0cee0bf3ae23c14411c345b151ad518077ee`.
- Exact command: `go test -count=1 ./internal/modules/career` → PASS, `ok github.com/Tencent/WeKnora/internal/modules/career 25.396s` (run at the revision above).
- Static evidence commands: `git show HEAD:internal/modules/career/career_export.go`, `git show HEAD:internal/modules/career/handler.go`, `git show HEAD:internal/modules/career/profile_intake.go`, `git show HEAD:internal/modules/career/reminder.go`, `git show HEAD:internal/modules/career/office.go`, and corresponding `git show HEAD:..._test.go` / `rg` inspections. These establish current source and test coverage but are not concurrency integration tests.
- Authentication/scope spot check: `Handler.scope` obtains user and tenant identity from request context, verifies owner-only membership, then installs scoped context (`handler.go:48-72`); reminder and upload DB operations use tenant/user predicates. No cross-scope exposure was identified in these findings.
- Migration/cancellation: none of the four findings changes schema or cancellation behavior; no migration/cancellation check was needed for this scoped validation.

## Gaps and risks

- The provided test command passes, but it does not disprove the identified gaps: export assertions omit runs/todos; no test covers raw unknown-error sanitization, cleanup-stage claim loss, or the concurrent distinct-request reminder collision.
- N11 is specifically a concurrent database behavior; the code path is clear, but this validation did not start PostgreSQL or run a race-specific integration test. Confidence is based on transaction ordering and the declared unique key.
- This is validation only. No source, test, requirement, or remote issue was modified.
