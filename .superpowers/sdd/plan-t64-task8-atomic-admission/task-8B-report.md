# Task 8B implementation report — durable claims and revocation reconciliation

## Scope and source state

- Plan: `docs/plans/issue30-sweep/plans/plan-t64-task8-atomic-admission.md`, Checkpoint 8B.
- Worktree: `/Users/wuyongjun/.codex/worktrees/t64-task8-8b/WeKnora-fork01`, branch `codex/issue30-t64-task8-8b`.
- BASE: `43100052bbb419ad1dd801a9a76b7570d1a3b28f`.
- Source HEAD at verification (implementation commit): `43c77e21d8ce514ff922233313cacfbe650cbb61`. This commit is based directly on the BASE above.
- Brief SHA256: `f97609e52a6614cf4903c8dc4c2c502424e20fc042409caa29a28b65a96cc726`.
- Rechecked paired migration heads before authoring migrations: SQLite `000125_agent_security_revocations.up.sql`; versioned/PostgreSQL `000204_agent_security_revocations.up.sql`. Added `000126_agent_chat_turn_claims` and `000205_agent_chat_turn_claims`.

## Implementation summary

- Added durable source-tenant-scoped AgentChatTurnClaim schema and repository lifecycle operations: ordered distinct tenant guards, idempotency/hash conflict, server-generated assistant placeholder in the admission transaction, bounded expiry cleanup, source/claim/generation/owner/state/lease fences, owner/session checks, and writes bound to the persisted session/message identity.
- Added immutable published Agent Version resolver and its service/interface seam.
- Added exact Variant/Version/Release Run sidecar projection and legacy backfill migrations, all-four-null-or-valid-populated checks, old-writer Marketplace insert guard, immutable sidecar and published Variant identity triggers, stale/duplicate/timestamp preflight checks, and populated-history down refusal.
- Added atomic revocation+audit+active-claim cancellation and placeholder terminalization; dependency target matching includes exact dependency type, ID, version, and digest. `allow` records complete/zero cancellation and does not cancel in-flight state.
- Added exact-pin Run reconciliation with tenant guard, SQLite writer reservation, locked pending-obligation recheck, cumulative count/terminal event/session-slot updates, retry, and concurrent-replica no-op behavior.
- Added startup and periodic bounded reconciler (default 10 seconds, configurable via `AGENT_SECURITY_RECONCILE_INTERVAL`) with ResourceCleaner shutdown, plus Handler claim-store and AgentVersion service injection setters.
- Added read-only SQLite/PostgreSQL preflight SQL and a sanitized unmatched-Run disposition template. No real tenant/run identifiers or signed rollout reports were added.

## TDD and targeted RED/GREEN evidence

- RED: `go test ./internal/application/repository -run '^TestAgentChatTurnClaimAdmitUsesOrderedTenantGuards$' -count=1` failed to compile because `withTenantSecurityGuards` was not present. GREEN after helper/test: the same test passed with ascending/de-duplicated guard call order.
- RED: initial claim-admission test failed with `no such table: agent_chat_turn_claims` before the paired migration. GREEN after migrations: claim tests passed.
- RED: admission test exposed that `types.Message.BeforeCreate` replaced the placeholder ID and claim pointed to a missing message. Fixed by persisting the placeholder first and binding the actual generated ID in the claim transaction; targeted claim tests then passed.
- RED: SQLite empty down/up migration failed because a trigger still referenced a dropped sidecar column. Fixed the down-trigger order; empty roundtrip and populated-history refusal passed.
- RED: partial unique index initially treated empty `local_agent_id` values as duplicates. Changed the partial predicate to exclude both NULL and empty IDs; adoption lifecycle test passed.
- GREEN: expiry test used fixed old time (`2000-01-01 00:00:00`) to deterministically verify placeholder terminalization; test passed.
- GREEN focused repository coverage: ordered lock guards, tenant-scope/idempotent admission, changed-hash conflict, exact Release/Version cancellation, terminal transition single winner, expiry/owner cancellation fences, claim cancellation on revocation, audit/placeholder rollback, allow behavior, exact-pin cancellation, reconciliation retry/concurrency, and published identity immutability.

## Verification log (serial package execution)

Commands were run serially; no Go package test processes overlapped. Test output below is the captured result summary; broad package output includes expected GORM `record not found` logs and was truncated by the tool when voluminous.

| Command | Exit | Result |
|---|---:|---|
| `go test ./internal/database -run '^(TestSQLiteMigrationsCreateVersionedSchema|TestTask8ClaimMigrationEmptyDownUpAndPopulatedDownRefusal|TestTask8RunPinsAndPublishedVariantIdentityAreImmutable)$' -count=1` | 0 | `ok github.com/Tencent/WeKnora/internal/database 5.774s` |
| `go test ./internal/application/repository -run '^(TestAgentChatTurnClaim.*|TestRunCancellationReconciliation.*|TestCancelRunsBySecurityPins.*|TestAgentSecurityStoreAppendReleaseAndAuditRollsBackTogether)$' -count=1` | 0 | `ok github.com/Tencent/WeKnora/internal/application/repository 16.381s` |
| `go test ./internal/application/service -run '^(TestResolvePublishedAgentVersionBindsTenantAndAgent|TestAgentSecurity.*)$' -count=1` | 0 | `ok github.com/Tencent/WeKnora/internal/application/service 22.855s` |
| `go test ./internal/database -run '^TestTask8PostgresMigrationDeclaresTransactionalSecurityGuards$' -count=1` | 0 | `ok github.com/Tencent/WeKnora/internal/database 1.636s`; checks the committed PostgreSQL migration source for explicit transaction/lock, backfill preflights, constraint/index/trigger names, and down refusal. Static inspection only; not a PostgreSQL execution test. |
| `go test ./internal/database -count=1` (after adding PostgreSQL source-contract test) | 0 | `ok github.com/Tencent/WeKnora/internal/database 16.418s` |
| `go test ./internal/container -run '^(TestAgentSecurityReconcilerRunsAtStartupAndStopsWithCleaner|TestAgentSecurityWiringRegistered)$' -count=1` | 0 | `ok github.com/Tencent/WeKnora/internal/container 3.773s`; linker emitted macOS warning `ignoring duplicate libraries: '-lc++'`. |
| `go test ./internal/application/repository -count=1` | 1 | Package completed in `442.568s`; legacy/task/Workbench fixtures that create Marketplace `agent_runs` without security sidecars failed with `agent_run_security_pin_incomplete`. The migration's four-field invariant is active as required; Run admission is owned by 8C and explicitly excluded from 8B changes. Captured broad output was truncated; representative failure was `TestTaskCollaborationEndToEndAC2SharedDoesNotWidenVisibility`, plus existing task-research/Workbench repository fixtures. |
| `go test ./internal/application/service -count=1` | 1 | Package completed in `341.858s`; Run execution/service fixture insertions without security pins failed with `agent_run_security_pin_incomplete` (for example `TestExecuteDurableRunPersistsBudgetExhaustionForNotification`, `TestCancelReleasesSessionSlotOnRealStore`, and Run graph tests). This is the same 8C admission/fixture integration dependency. |
| `go test ./internal/container -count=1` | 1 | Failed one unrelated legacy fixture `TestWireCraftInteractionRegistrarRegistersPendingInteractions` when it inserted an unpinned Marketplace Run (`agent_run_security_pin_incomplete`); reconciler/wiring focused tests pass. Other output includes expected test logs and the same macOS linker warning. |
| `go test ./internal/handler/session -count=1` | 1 | Existing Run/Craft/Workbench integration fixtures create Runs without exact pins and fail with `agent_run_security_pin_incomplete` (examples: `TestAgentRunEventsEndpointReplayAndCursor`, Craft interaction/HTTP flows, legacy Workbench migration, and `TestWorkbenchStartHTTPIntegrationAndIdentityIsolation`). This is the same 8C admission/fixture integration dependency; no session QA behavior was changed in 8B. |
| `go build ./...` | 0 | Succeeded; macOS linker emitted duplicate `-lc++` warnings for `cmd/server` and `cmd/desktop`. |
| `git diff --check` | 0 | No whitespace errors. |

## Migration and PostgreSQL evidence

- SQLite migrations executed through the project migration runner in the migration package. Tests verified schema head/uniqueness, empty down/up, claim-history down refusal, complete sidecar constraint, invalid source rejection, immutable pin updates/clears, old-writer Marketplace insert rejection, published Variant identity immutability, and unchanged snapshot bytes.
- SQLite head after migration: `000126`; versioned/PostgreSQL target head: `000205`.
- The four new migration files are ignored by `.gitignore` and require explicit force-add. Only those exact four files will be force-added.
- `command -v psql` and `command -v pg_isready` returned no executable. No PostgreSQL server/endpoint was exercised. PostgreSQL DDL received static source inspection and the static contract test above; that is not equivalent to PostgreSQL runtime validation.
- The preflight queries are reusable artifacts only. No production database query, writer quiescence, signed disposition, rollout approval, or production signoff occurred.

## Owned paths and commit

The intended diff is limited to the corrected Task 8B Owned paths from the Brief: paired migrations (4); claim type/repository/tests; security guard, adoption, Run projection/reconciliation, and security repository/test paths; security service/tests/interface/persistence type; SQLite/PostgreSQL migration-contract tests; reconciler/container/Handler injection paths; and the three preflight/template files. This report is also owned by the Brief. No Task 8A/Task7 implementation or Task 8C AgentRun admission source was changed.

- Owned-path check: `git diff --name-only` plus `git status --short --untracked-files=all` was reviewed against the complete Brief file map; the only ignored untracked files were the four new paired migrations listed above.
- Owned-path staged check passed: all 30 committed paths are in the corrected Brief owned-path map; the four ignored migrations were force-added by exact path only. `git diff --cached --check` passed before commit.
- Implementation commit (all implementation files and this report): `43c77e21d8ce514ff922233313cacfbe650cbb61` (`feat: add durable agent chat turn claims`). No push or remote operation was performed.

## Remaining risks / integration notes

1. Full owning package suites are not green because existing Run consumers/fixtures do not provide the exact pin sidecars required by the new migration guard. 8C must inject exact identity pins for new Marketplace Run rows, and downstream fixture suites must be updated accordingly before full integration verification.
2. PostgreSQL behavior is statically inspected only. Run the versioned migrations and relevant backfill/down/trigger tests against the supported PostgreSQL version before rollout.
3. The deployment-time signed unmatched-Run disposition remains an operator responsibility and is intentionally not represented as complete by these code artifacts.

## Review-fix round 1 — F1–F4

- Fix-plan Brief: `.worktrees/issue30-sweep/.worktrees/issue30-b6-coordination/.superpowers/sdd/plan-t64-task8b-review-fix-r1/task-1-brief.md`.
- Independent review range: `43100052bbb419ad1dd801a9a76b7570d1a3b28f..cbdb08e627b3466cc419815e3108fe18bb0b3bfc`.
- R1 source base: `cbdb08e627b3466cc419815e3108fe18bb0b3bfc`.

### Corrections

- **F1:** app-side Variant identity updates now lock/re-read and refuse when `published_at` is set, including retired state; SQLite and PostgreSQL triggers use `OLD.published_at IS NOT NULL`. Added retired mutation tests for repository behavior and SQLite direct SQL.
- **F2:** claim cancellation, owner cancellation, and expiry cleanup now require one tenant/session/owner/request-bound assistant placeholder update after a successful claim state transition. Any missing/mismatched row returns an error and rolls back the enclosing transaction. Added missing/mismatched placeholder rollback tests for all three paths.
- **F3:** claim entity and both migration dialects now use composite primary key `(id, source_tenant_id)`, named unique indexes for request replay, `(session_tenant_id, assistant_message_id)`, `(session_tenant_id, user_message_id)`, and the source/state/release lookup index. GORM index metadata and SQLite/PostgreSQL migration source contract are asserted.
- **F4:** revocation views expose `run_cancellation_state`. The service returns the committed revocation ID with `pending` when immediate reconciliation errors, logs the reconciliation error, and retains the existing durable worker retry. Successful immediate reconciliation returns `complete`; `allow` also persists/returns `complete`.

### R1 RED evidence

| Command | Exit | Captured failure |
|---|---:|---|
| `go test ./internal/application/repository -run '^(TestAgentAdoptionRepositoryLifecycle|TestAgentChatTurnClaimMissingPlaceholder.*|TestAgentChatTurnClaimRevocationMissingPlaceholderRollsBackTogether)$' -count=1` | 1 | Before fixes: retired repository identity mutation, owner cancellation with missing placeholder, and revocation with missing placeholder each unexpectedly returned nil. |
| `go test ./internal/application/repository -run '^(TestAgentChatTurnClaimMissingPlaceholder.*|TestAgentChatTurnClaimMissingExpiredPlaceholderRollsBackAdmission|TestAgentChatTurnClaimRevocationMissingPlaceholderRollsBackTogether|TestAgentAdoptionRepositoryLifecycle)$' -count=1` | 1 | Before fixes: same three failures plus expiry cleanup/new admission unexpectedly returned nil with the expired placeholder missing. |
| `go test ./internal/database -run '^(TestTask8ClaimMigrationEmptyDownUpAndPopulatedDownRefusal|TestTask8RunPinsAndPublishedVariantIdentityAreImmutable|TestTask8PostgresMigrationDeclaresTransactionalSecurityGuards)$' -count=1` | 1 | Before fixes: composite PK assertion failed (`[id source_tenant_id]` absent); retired direct SQL identity mutation unexpectedly succeeded; PostgreSQL source contract lacked composite key/required indexes. |
| `go test ./internal/application/service -run '^(TestRevokeReleaseReturnsCommittedPendingResultWhenImmediateReconcileFails|TestRevokeDependencyReturnsCommittedPendingResultWhenImmediateReconcileFails)$' -count=1` against the original service implementation at R1 base | 1 | Both tests failed with `reconcile temporarily unavailable` returned to the caller, despite the revocation having committed. |

### R1 GREEN and verification evidence

| Command | Exit | Output/result |
|---|---:|---|
| `go test ./internal/application/repository -run '^(TestAgentChatTurnClaimEntityUsesTenantScopedKeysAndRevocationIndex|TestAgentAdoptionRepositoryLifecycle|TestAgentChatTurnClaimMissingPlaceholder.*|TestAgentChatTurnClaimMissingExpiredPlaceholderRollsBackAdmission|TestAgentChatTurnClaimRevocationMissingPlaceholderRollsBackTogether)$' -count=1` | 0 | `ok github.com/Tencent/WeKnora/internal/application/repository 6.272s` |
| `go test ./internal/application/repository -run '^(TestAgentChatTurnClaim|TestRunCancellationReconciliation|TestCancelRunsBySecurityPins|TestAgentAdoption.*Variant|TestAgentSecurityStoreAppendReleaseAndAuditRollsBackTogether)' -count=1` | 0 | `ok github.com/Tencent/WeKnora/internal/application/repository 16.406s` |
| `go test ./internal/application/service -run '^(TestRevokeReleaseReturnsCommittedPendingResultWhenImmediateReconcileFails|TestRevokeDependencyReturnsCommittedPendingResultWhenImmediateReconcileFails)$' -count=1` | 0 | `ok github.com/Tencent/WeKnora/internal/application/service 3.635s`; both pending responses preserve ID and durable retry changes state to complete. |
| `go test ./internal/application/service -run '^(TestResolvePublishedAgentVersion|TestAgentSecurity.*)$' -count=1` | 0 | `ok github.com/Tencent/WeKnora/internal/application/service 18.737s` |
| `go test ./internal/database -run '^(TestSQLiteMigrationsCreateVersionedSchema|TestTask8ClaimMigrationEmptyDownUpAndPopulatedDownRefusal|TestTask8RunPinsAndPublishedVariantIdentityAreImmutable|TestTask8PostgresMigrationDeclaresTransactionalSecurityGuards)$' -count=1` | 0 | `ok github.com/Tencent/WeKnora/internal/database 4.305s`; SQLite assertions passed and PostgreSQL SQL source contract passed. |
| `go build ./...` | 0 | Passed; macOS linker emitted `ignoring duplicate libraries: '-lc++'` for `cmd/desktop` and `cmd/server`. |
| `git diff --check` and `git diff --cached --check` | 0 | No whitespace errors before R1 staging. |

R1 focused tests were run serially. No changes were made outside the 14 R1-owned paths in the Task Brief. The pre-existing PostgreSQL execution limitation still applies (`psql`/`pg_isready` unavailable; no server exercised). The full-suite 8C+ unpinned-Run fixture failures recorded above remain integration follow-up and were not changed in this checkpoint.

- R1 source HEAD/commit: pending final local commit.
