# Craft Budget Extension Intent Backend Report

## Scope and baseline

- Task H: server-owned durable extension intent and quantum contract, closing F13/F14 while preserving the reviewed F16 TaskRead fix.
- Worktree: `/Users/wuyongjun/.codex/worktrees/ocr-budget-auth/WeKnora-fork01`
- Starting/current HEAD: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no commit created.
- The referenced plan `docs/plans/2026-09-28-craft-107-ocr-r2-plan.md` was absent from both the worktree and integration checkout. The parent supplied the frozen `extension_action` interface in task direction. The existing UI source established its current quantum as 10 calls and 10,000,000 microcredits; service persists credits from the deployment-owned `CraftBudgetPolicy.TaskLimit` (currently 10,000,000 microcredits in `defaultCraftBudgetPolicy`).
- Migration heads at task start: `migrations/versioned` 000214, so Postgres/shared next is 000215; `migrations/sqlite` 000135, so SQLite next is 000136.

## Interface contract

Owner or eligible billing admin GET:

```json
{
  "success": true,
  "data": {
    "run_id": "<run>",
    "reason": "exhausted",
    "limit": 1000,
    "used": 1000,
    "extension_action": {
      "key": "budget-extension-<128-bit-random-hex>",
      "extra_calls": 10,
      "extra_credits": 10000000
    }
  },
  "can_extend": true
}
```

The exact `extra_credits` value is the service's configured `TaskLimit` in microcredits; the example reflects current production policy. A TaskRead-only viewer receives the same pause fact with `data.extension_action: null` and `can_extend: false`. The HTTP handler still gates both endpoints with TaskRead, while the service continues to recheck owner/billing-admin authority on mutation.

POST remains `{"key":"<same key>","extra_calls":10,"extra_credits":10000000}`. The service rejects either amount or key when it differs from the persisted pending action (`craft.ErrConflict`). A retry after the commercial extension lands but resume fails reuses that same action. The extension row is marked completed in the same transaction as the Run's `waiting_user` → `recovering` transition. A later pause replaces the completed row with a fresh random action key and current configured quantum.

## Durable representation and migrations

`craft_budget_extension_intents` stores one current action per `(tenant_id, session_id, run_id)` with `intent_key`, `extra_calls`, `extra_credits`, `status`, `created_at`, and `updated_at`. The service locks the Run row before creating/reusing/replacing an action, serializing concurrent GETs with resume and later pause transitions. Concurrent GETs therefore observe one pending key. The completed action is replaced only after a later pause; a failed resume leaves the action pending.

- PostgreSQL/shared versioned migration: `migrations/versioned/000215_craft_budget_extension_intents.up.sql` and `.down.sql`.
- SQLite migration: `migrations/sqlite/000136_craft_budget_extension_intents.up.sql` and `.down.sql`.
- The repository ignores `migrations/` for untracked files; the four migration files are marked intent-to-add with `git add -N -f` so their contents remain unstaged but visible in the worktree diff. No commit was made.

## TDD and verification

- RED: `go test ./internal/application/service -run '^TestCraftBudget(ExtensionIntentPersistsAcrossResumeFailureAndRenews|PauseConcurrentIntentCreationHasSingleWinner)$' -count=1` failed because pause JSON had no `extension_action` (nil action assertion), as expected before implementation.
- GREEN: the same focused service command passed after implementation (`ok .../internal/application/service 4.755s`; the later run adding explicit pending/completed assertions also passed, `4.755s` output recorded in terminal).
- Service regression selection: `go test ./internal/application/service -run 'CraftBudget|CraftT19' -count=1` passed (`ok .../internal/application/service 54.996s`).
- HTTP journeys: `go test ./internal/handler/session -run '^TestCraftT20(BudgetPauseHTTPJourney|Journey)$' -count=1` passed (`ok .../internal/handler/session 1.628s`).
- Full handler package: `go test ./internal/handler/session -count=1` passed (`ok .../internal/handler/session 43.963s`).
- Craft module: `go test ./internal/modules/craft -count=1` passed (`ok .../internal/modules/craft 5.371s`).
- SQLite migration coverage: `go test ./internal/database -run 'TestSQLiteMigrationsCreateVersionedSchema|TestSemanticMigrationSQLiteUpDownUp' -count=1` passed (`ok .../internal/database 8.574s`), covering the new SQLite table and full migration up/down/up chain.
- Static checks: `go vet ./internal/application/service ./internal/handler/session ./internal/modules/craft` passed with no diagnostics; `git diff --check` passed.
- The tagged Postgres integration migration test was not run; it requires `TRPC_TEST_POSTGRES_DSN`.

### Scoped review follow-up

- Added a service reconstruction over the same persisted database after the injected resume failure, before the refetch and retry. This proves the pending key/quantum survive loss of the original service instance.
- Rerun: `go test ./internal/application/service -run '^TestCraftBudget(ExtensionIntentPersistsAcrossResumeFailureAndRenews|PauseConcurrentIntentCreationHasSingleWinner)$' -count=1` passed (`ok .../internal/application/service 6.892s`).
- `git diff --check` passed.

## Changed source and test file hashes

- `internal/application/service/craft_budget.go`: `361f1527e05b32015873a17ce3fc05cd7e3ca6e6ddf8b50c7e6ae2a7fa7bef43`
- `internal/application/service/craft_budget_t19_test.go`: `3a3fd257875dd362fbc82ac831a71f15d359f16685c7eabaec388f0a584559d1`
- `internal/handler/session/craft_budget_pause.go`: `70097443953b0479e3a6165362332464971d28d050c0855d6016a7ad5252c86c`
- `internal/handler/session/craft_budget_pause_test.go`: `528909bc7fe4d57e7a3bebc541529b601f8da7ffb822730964c387cb9b35b282`
- `internal/handler/session/craft_t20_journey_test.go`: `11afc66ee10fe4570f7827eb27a35cc835e73fd2d5116958615e181c79689325`
- `internal/handler/session/craft_test.go` (preserved F16 non-member identity fixture): `de98d6833b39415b0421b39ac16cc9c2ec90a67854f84e3dc30e1c805a52c87d`
- `internal/modules/craft/web_contracts.go`: `87e7ab7fa2ab8d5fb12c535e4e9e986657a56bcde2ab016afe36c47370463b84`
- `internal/database/migration_sqlite_versioned_schema_test.go`: `df6b1d3673a5d69f0796b98de6a938ad409685d2ce6d2dae2d169bcdfa02f64f`
- `migrations/versioned/000215_craft_budget_extension_intents.up.sql`: `d3c27c8a93443e150b688a9870f2eb7c54e1f672670131be9f83d1941bca8ab8`
- `migrations/versioned/000215_craft_budget_extension_intents.down.sql`: `305a02162391771adb31d6b8e9be5cdcaa9d2bfef3e1bfd08b8229f9802901bc`
- `migrations/sqlite/000136_craft_budget_extension_intents.up.sql`: `e2a4698a6f175cee12188758e85198a26d1b0ca96b2df2d86e4d8202678127f7`
- `migrations/sqlite/000136_craft_budget_extension_intents.down.sql`: `305a02162391771adb31d6b8e9be5cdcaa9d2bfef3e1bfd08b8229f9802901bc`

## Self-review and remaining risk

- Mutation authorization in `MayExtendBudget` and `ExtendAndResume` is unchanged. F16 read access stays with TaskRead; the new action is only projected to callers currently authorized to extend.
- No TypeScript, Docker, repository-process, push, or commit changes were made.
- No schema/data backfill is required; existing paused Runs receive their action on the first authorized pause GET.
- PostgreSQL migration execution remains unverified in this environment because the required integration DSN is unavailable.
