# RunView create-intent: independent scoped review

Date: 2026-09-23. Read-only review of the RunView create-intent plan/report and seven-file checkpoint in the integration worktree, HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Sources: approved Craft Spec #107/#120/#124, RunView binding/retention decisions, prior RunView reviews, current typed contract, repository store and both migration dialects. No production/test edits, staging, OCR or PostgreSQL execution.

## Exact checkpoint and verdict

| File | Full-content SHA-256 |
| --- | --- |
| `migrations/versioned/000193_craft_run_view_create_intent.up.sql` | `3977f35b38025a516a8e7292331f3acb1cc1638a1264831395949068ba09dce8` |
| `migrations/versioned/000193_craft_run_view_create_intent.down.sql` | `7b39b4f6173ec180117119cfede55ff6284db235d48b5ce352238bd0918c4d52` |
| `migrations/sqlite/000114_craft_run_view_create_intent.up.sql` | `1e83410666e652c458cd68eec697025f8d18c6e0e9edb976d7bae201eb8f504b` |
| `migrations/sqlite/000114_craft_run_view_create_intent.down.sql` | `7b39b4f6173ec180117119cfede55ff6284db235d48b5ce352238bd0918c4d52` |
| `internal/modules/craft/run_view.go` | `d90df7d3f46d4ca9b1d7a096a3a79de7805a388930c5ff33eb9f76e86ea921a5` |
| `internal/modules/craft/run_view_test.go` | `2122e662700b5fe64105bf3bdcbe8e987a5873bcf64fad23bd6ff7350104d6ec` |
| `internal/application/repository/craft_run_view.go` | `e54f8829a68248726014386d46d016c3a69ceb1f18950d74f736fcd2366963dd` |
| `internal/application/repository/craft_run_view_test.go` | `3a1520612858795e304924f795beb9652e2c36dbc10066483bbe0f1b83f46b89` |

The report's complete-file hashes match the live files. Four new SQL files were staged as intent-to-add (`A`); all four Go files are untracked in this shared integration tree. These status facts do not imply a commit.

- **Scoped Spec compliance: PASS.** A one-shot, committed, generation-scoped intent must precede a non-idempotent OpenCode CreateSession call. The store grants `maySend=true` only to the transaction winner; retry/restart sees a durable attempted marker and cannot send again. Bound runtime identity requires that marker and remains a complete immutable triple.
- **Scoped code quality: PASS.** The SQL CAS includes tenant, Run, Owner, Task/Session, generation, allocating state, NULL intent, and the admitted Run's matching identity. The implementation does not expose permission when transaction/commit fails. Focused SQLite race, restart, scope, binding and legacy-migration tests passed. No new scoped finding was identified.
- **Full RunView/T01/T05: NOT VERIFIED.** This is a storage prerequisite only. A crash after intent commit but before POST can conservatively strand the Run; lost provider response still needs authoritative scoped inventory/reconciliation. Per-Run OS/mount isolation, central runtime dispatch and Publisher behavior remain separate gates.

## Evidence and limits

The typed `RunViewStore.BeginSessionCreate` and nullable `SessionCreateIntentAt` are explicit (`internal/modules/craft/run_view.go:38-60`); `ValidateRunView` forbids bound rows without a nonzero marker and rejects a partial runtime (`:83-111`). `Allocate` keeps a stable generation and inserts only for the matching admitted Run (`internal/application/repository/craft_run_view.go:106-146`). `BeginSessionCreate` performs one conditional update before loading and authorizing the row in the same transaction, then returns `maySend` only after `Transaction` returns nil (`:175-236`). Concurrent updates serialize on the row/SQLite writer and one affected row wins; a failed transaction returns `maySend=false`. Wrong tenant, owner, session or stale generation cannot consume the NULL marker (`:190-228`). `BindRuntime` requires a non-NULL marker in its CAS and accepts only exact bound replay (`:239-293`).

PG 000193 and SQLite 000114 each add a nullable timestamp and backfill every preexisting row from non-NULL `updated_at`; this makes legacy allocating rows unknown rather than automatically retryable while preserving old bound rows under the new validator. Fresh allocation leaves the column NULL. Both dialects remove only the added column on rollback, and the previous 000192/000113 migrations are unchanged. The SQLite migration test applies 113→114, checks legacy allocating and bound rows, rolls back and reapplies (`craft_run_view_test.go:63-126`). This is a conservative compatibility choice; a pre-upgrade allocating row may need manual/provider reconciliation.

I independently ran `go test ./internal/modules/craft ./internal/application/repository -run 'RunView' -count=1` (both packages passed). A verbose restart test showed SQLite PASS and PostgreSQL SKIP with `TRPC_TEST_POSTGRES_DSN unset`; no PG17 transaction or migration behavior is claimed. The report also records focused repository tests without the temporary overlay after concurrent T19 source settled and a separate unrelated full Craft-package failure. The SQL is structurally parallel, but PG behavior remains an external verification limit. The four SQL files must remain included in final delivery despite the repository's migration ignore rule.
