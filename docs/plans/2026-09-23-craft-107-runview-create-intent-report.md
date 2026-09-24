# RunView Durable Create-Intent — Checkpoint

**Status:** DONE_WITH_CONCERNS — the durable one-shot pre-CreateSession permission is implemented and focused SQLite/Go tests pass. This closes only the R1 storage prerequisite. It does not prove scoped OpenCode session inventory/recovery, production dispatch, or OS-level Run isolation. No commit was made.

**Worktree / base:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`, HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. The integration worktree is shared and contains unrelated in-flight T01/T05/T08/T19 changes; this task preserved them. The assigned role was `backend_implementer`; model/effort metadata is not exposed by the runtime.

## Change

- Added `RunView.SessionCreateIntentAt` and `RunViewStore.BeginSessionCreate(ctx, key, generation)`. The repository performs one scoped SQL `UPDATE ... WHERE session_create_intent_at IS NULL AND state='allocating'` inside a transaction. `maySend=true` is returned only to the sole row-update winner and only after transaction commit succeeds. Any later attempt returns `maySend=false`; if commit fails, the caller receives `maySend=false` even if the database may have committed, which conservatively strands the view for reconciliation.
- `BindRuntime` now requires the durable intent. The bound view validator rejects missing/zero intent timestamps. Allocating rows may have a marker while an outcome remains unresolved, but runtime identity stays all-or-nothing.
- Added SQLite migration 000114 and PostgreSQL migration 000193. Both add a nullable timestamp. Existing rows are backfilled from `updated_at` so pre-migration allocating rows are treated as unknown and cannot receive a new CreateSession permission; pre-migration bound rows continue to validate. Fresh rows remain `NULL` until the winner commits the intent.
- Repository race/restart/scope tests use independent stores/connections. The one-shot restart case deliberately simulates an unknown external outcome by leaving the intent row allocating and reopening the DB; it proves no second permission is granted.

**Conservative upgrade behavior:** every legacy row is backfilled as potentially attempted because the old schema cannot establish whether CreateSession was sent. A pre-upgrade allocating row with no session may therefore require explicit reconciliation; this is safer than silently reissuing a non-idempotent request.

## RED → GREEN and verification

RED was observed before implementation:

```text
go test ./internal/modules/craft ./internal/application/repository -run 'RunView.*(Intent|Binding)' -count=1
FAIL: SessionCreateIntentAt undefined; BeginSessionCreate undefined
```

After implementation:

```text
go test ./internal/modules/craft -run RunView -count=1
ok   github.com/Tencent/WeKnora/internal/modules/craft  1.118s

go test -overlay=/tmp/craft-runview-create-intent-overlay/overlay.json ./internal/application/repository -run CraftRunView -count=1
ok   github.com/Tencent/WeKnora/internal/application/repository  14.871s

git diff --check -- internal/modules/craft/run_view.go internal/modules/craft/run_view_test.go internal/application/repository/craft_run_view.go internal/application/repository/craft_run_view_test.go
PASS (no output)
```

The temporary overlay excluded only unrelated, currently incomplete `agent_run_craft_charge_test.go` from test-package compilation. At test time that concurrent T19 file had an undefined `journal` variable; it was not modified by this task. T19's missing `errors` import was repaired concurrently after the first compile failure. The RunView repository tests themselves ran unchanged under the overlay. A normal repository package test was not green in the shared in-flight tree because of that separate T19 compile error.

After the concurrent T19 edit settled, the focused package test was rerun without any overlay:

```text
go test ./internal/application/repository -run CraftRunView -count=1
ok   github.com/Tencent/WeKnora/internal/application/repository  9.072s
```

This is the final repository verification for the checkpoint. The earlier overlay run is retained as the intermediate evidence while the shared package was temporarily uncompilable.

SQLite migration DDL was exercised against SQLite 3.54.0 using an in-memory old-schema table with both allocating and bound rows. Output:

```text
UP_BACKFILL_PASS
DOWN_PASS
REAPPLY_PASS
```

The repo migration test additionally migrates from 113 to 114 with legacy allocating/bound rows, loads both through the store, verifies the legacy allocating row cannot win `BeginSessionCreate`, and checks down/up reapplication.

`go test ./internal/modules/craft -count=1` was also attempted and remains red in unrelated `TestExcerptOfBoundsAtRuneBoundary` (`knowledge_test.go:83`, expected overshoot marker missing; got `"知知"`). The focused RunView contract test passes. No PostgreSQL test ran: `TRPC_TEST_POSTGRES_DSN` was unset, so PG 000193 portability is encoded in the paired migration but not exercised against PG17 here.

## Ignored migration contents and full-content hashes

Migration paths are ignored by `.gitignore:96` (`migrations/`). The controller must force-add them after review. Exact contents at this checkpoint:

`migrations/sqlite/000114_craft_run_view_create_intent.up.sql` — SHA-256 `1e83410666e652c458cd68eec697025f8d18c6e0e9edb976d7bae201eb8f504b`:

```sql
-- A non-null intent means a non-idempotent session create may already have
-- been sent. Existing rows predate this fence, so treat them conservatively
-- as attempted instead of granting an unsafe retry permission.
ALTER TABLE craft_run_views
    ADD COLUMN session_create_intent_at DATETIME NULL;

UPDATE craft_run_views
SET session_create_intent_at = updated_at
WHERE session_create_intent_at IS NULL;
```

`migrations/sqlite/000114_craft_run_view_create_intent.down.sql` — SHA-256 `7b39b4f6173ec180117119cfede55ff6284db235d48b5ce352238bd0918c4d52`:

```sql
ALTER TABLE craft_run_views DROP COLUMN session_create_intent_at;
```

`migrations/versioned/000193_craft_run_view_create_intent.up.sql` — SHA-256 `3977f35b38025a516a8e7292331f3acb1cc1638a1264831395949068ba09dce8`:

```sql
-- A non-null intent means a non-idempotent session create may already have
-- been sent. Existing rows predate this fence, so treat them conservatively
-- as attempted instead of granting an unsafe retry permission.
ALTER TABLE craft_run_views
    ADD COLUMN session_create_intent_at TIMESTAMPTZ NULL;

UPDATE craft_run_views
SET session_create_intent_at = updated_at
WHERE session_create_intent_at IS NULL;
```

`migrations/versioned/000193_craft_run_view_create_intent.down.sql` — SHA-256 `7b39b4f6173ec180117119cfede55ff6284db235d48b5ce352238bd0918c4d52`:

```sql
ALTER TABLE craft_run_views DROP COLUMN session_create_intent_at;
```

## Checkpoint and limits

Tracked/untracked ownership paths changed for this task:

```text
internal/modules/craft/run_view.go
SHA-256 d90df7d3f46d4ca9b1d7a096a3a79de7805a388930c5ff33eb9f76e86ea921a5
internal/modules/craft/run_view_test.go
SHA-256 2122e662700b5fe64105bf3bdcbe8e987a5873bcf64fad23bd6ff7350104d6ec
internal/application/repository/craft_run_view.go
SHA-256 e54f8829a68248726014386d46d016c3a69ceb1f18950d74f736fcd2366963dd
internal/application/repository/craft_run_view_test.go
SHA-256 3a1520612858795e304924f795beb9652e2c36dbc10066483bbe0f1b83f46b89
```

This marker prevents a second blind POST but does not identify or recover a created OpenCode session. An unknown attempt with zero or ambiguous inventory must remain unresolved. R1 remains blocked on an authoritative generation/container-scoped inventory and verified session-directory affinity; T01 isolation remains pending until actual container/mount/network and end-to-end evidence exists. No `container.go`, `craft_runtime.go`, OpenCode, Publisher, T08 or T19 implementation file was edited by this task.
