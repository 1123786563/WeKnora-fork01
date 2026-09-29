# Craft #107 OCR R2 Promotion Report

## Scope and inputs

- Assigned base: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`.
- Findings: F06 receipt scan starvation, F26 per-Run promotion context/global scan, F35 non-atomic revision fence.
- Read the OCR finding blocks, approved web artifact spec and scope design, relevant T15/T20 ledger entries, and promotion/repository source. The assigned R2 plan was initially absent from this checkout; Task G was later read from the integration checkout without modifying it.
- The completed budget-auth work owns SQLite migration 000136 and PostgreSQL migration 000215. This isolated promotion work adds the next versions, 000137 and 000216.
- No commits or pushes were made.

## Implementation

- **F06 durable progress:** Added a singleton keyset cursor, persisted in its own table. A bounded page claim advances the cursor and reserves receipt cooldowns in one transaction. The cursor row serializes competing global scan claims across processes; SQLite first performs a no-op write to acquire its writer slot, and PostgreSQL uses a row lock.
- **F06 bounded retries/fairness:** A receipt attempt row records `retry_after` and `completed`. Claims reserve a five-minute cooldown before probe/publish work, so process failure cannot hot-loop a receipt. Successful and terminally ineligible receipts are marked completed. Transient failures become due again after cooldown. A global page reserves one quarter for fresh receipts on every tick (at least one slot); due retries use the other at most three quarters and sort by earliest `retry_after`, so a due backlog cannot stop cursor progress and selected failures rotate behind later due receipts until cooldown expires.
- **F06 claim serialization:** Global and per-Run claim transactions both take the singleton cursor row lock before selecting/reserving attempts. The targeted path leaves the cursor unchanged but cannot concurrently reserve a receipt already claimed by the global path.
- **F26 per-Run trigger:** `RecoverRun` selects only receipts for `(tenant_id, run_id)` and uses an independent 20-second context. This path does not advance the global cursor.
- **F35 atomic revision fence:** `PromoteWebVersion` passes its validated draft head to `DraftFencedVersionStore`. The production SQL store locks and rechecks current revision, selected state, source Run and manifest digest, verifies the immutable revision row, then publishes the version and optional evidence in the same transaction. Mismatch returns `craft.ErrConflict` without publishing.
- The capture receipt itself remains immutable; promotion progress is held in dedicated tables.

## Verification

Executed from the promotion worktree:

```text
go test ./internal/application/repository -run '^TestCraftRunCapturePromotionMigration$' -count=1
PASS: SQLite migration Up/Down/Up; PostgreSQL subtest skipped because TRPC_TEST_POSTGRES_DSN is unset.

go test ./internal/container -run 'TestCraftT20(PostTerminalPromotion|PromotionRecoveryPagesPastOlderReceipts|PromotionFailureCooldownIsBoundedAndRetryable|PerRunPromotion|PromotionRejectsHeadAdvancedAfterValidationSQLite)' -count=1
PASS.

go test ./internal/container -run 'TestCraftT20(FreshPromotionQuotaSurvivesOverdueBacklog|TargetedAndGlobalPromotionClaimsSerializeSQLite|PromotionRecoveryPagesPastOlderReceipts|PromotionFailureCooldownIsBoundedAndRetryable)' -count=5
PASS: repeated fresh-quota, page-restart, cooldown, and cross-path claim race regressions.

go test ./internal/application/service -run '^TestCraftT15' -count=1
PASS.

go test ./internal/container -run '^TestCraftT20PromotionRejectsHeadAdvancedAfterValidationSQLite$' -count=5
PASS: five deterministic SQLite head-advance interleavings.

git diff --check
PASS.
```

The restart regression creates more than one page of receipts, claims the first page, creates a new promoter instance, and verifies it reaches the later receipt using database state. The overdue-backlog regression creates 40 due failed receipts plus a fresh receipt and verifies the fresh quota returns it within the bounded batch. The cooldown regression verifies an incomplete candidate is excluded before `retry_after` and claimable after its timestamp expires. The cross-path race starts global and targeted claimers together and verifies exactly one reserves the receipt. The F35 regression holds promotion after revision 1 validation, advances the real SQLite head to revision 2, resumes publication, and asserts conflict plus an empty published-version list.

PostgreSQL runtime and migration behavior remain unverified because `TRPC_TEST_POSTGRES_DSN` is not configured. The gated PostgreSQL migration test is present and will run when that DSN is available.

## Finding disposition

- **F06: resolved.** Global progress and retry schedule survive promoter restart. Fresh work uses bounded keyset pages; due failures receive a bounded retry page after cooldown.
- **F26: resolved.** Immediate promotion is tenant/Run targeted and independently bounded.
- **F35: resolved for the production SQL version store.** Custom stores without the fenced capability fail closed. SQLite check/publish race coverage passes; PostgreSQL runtime remains unverified.

## Checkpoint

HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; all task changes are uncommitted. No integration files were changed.

Final SHA-256 values for changed implementation, tests, and migrations (the report itself is the checksum ledger). Migration and new repository files are intent-to-add in this worktree so they appear in the review diff:

```text
db3844f1f1c47cea8b51513280de26b7850ebb690359f4992057fb0ee08a765f  internal/application/repository/craft_version.go
9214ed0501d95b4338012fdb3179f327ea28cddd477144a79c3f55b4a98dd68a  internal/application/repository/craft_version_test.go
32dadb12cfdd20c7387d18b4db074f0f9c8d1982d94a76ca328a0520f968c8c5  internal/application/repository/craft_run_capture_promotion.go
b0fd12d5582372f5bb9c64246416d3eba01994dd8d53a17fff92adb01e1b5391  internal/application/repository/craft_run_capture_promotion_test.go
7a71fb8ce4ffe5840a1a8811f40bd86a7c11ac9f5e45284fcd57357a575cfda5  internal/application/service/craft_artifacts.go
d3a7ab52464f499aaad7d755b83e12c984dd02cfb0e5de482b0f841c0325dd62  internal/application/service/craft_artifacts_test.go
e38e0b4650610775090bf5c2ae4780c249c45cd218aeffaaf48539bd6b447b87  internal/container/craft_run_capture_promotion.go
324807c2789df27eb1f64b649bc7df92c54abce491badb656fbe3c2c2ec7b0b2  internal/container/craft_run_capture_promotion_test.go
4fc0d759a1ff8c8ff7f87dde7c2f1885ed84fb5e17c6a8b395c5f3dc5b962d8b  internal/container/craft_run_capture_wiring.go
a5af324517b25f26f0ed3b034fa2e81a756aee81025c28ef9b06271d99c50ebe  internal/modules/craft/version.go
31100397a17b673170bfa47610c29a042c805dcb51f1534a6e9631a901c80354  migrations/sqlite/000137_craft_run_capture_promotion_scan.up.sql
12ba927db171f995110843bc19910b4f909c6fa3147edbe7e32cdcca8d403402  migrations/sqlite/000137_craft_run_capture_promotion_scan.down.sql
3aab92516ee5047093aafdf24069f978152aff82911fca7d4022dcb6e08ba291  migrations/versioned/000216_craft_run_capture_promotion_scan.up.sql
12ba927db171f995110843bc19910b4f909c6fa3147edbe7e32cdcca8d403402  migrations/versioned/000216_craft_run_capture_promotion_scan.down.sql
```

## OCR R2 repair task

Repair ownership is limited to the promotion repository/migration files and this report:

- `internal/application/repository/craft_run_capture_promotion.go` — starting SHA-256 `32dadb12cfdd20c7387d18b4db074f0f9c8d1982d94a76ca328a0520f968c8c5`
- `internal/application/repository/craft_run_capture_promotion_test.go` — starting SHA-256 `b0fd12d5582372f5bb9c64246416d3eba01994dd8d53a17fff92adb01e1b5391`
- `migrations/sqlite/000137_craft_run_capture_promotion_scan.up.sql` — starting SHA-256 `31100397a17b673170bfa47610c29a042c805dcb51f1534a6e9631a901c80354`
- `migrations/versioned/000216_craft_run_capture_promotion_scan.up.sql` — starting SHA-256 `3aab92516ee5047093aafdf24069f978152aff82911fca7d4022dcb6e08ba291`
- This report — starting SHA-256 `1732969b7ec12da05b4330d472d4119c81eb1d52a44e9a792499597d2972132a`

Repair steps: (1) add a migration index matching the fresh keyset order in both dialects, with plan evidence that SQLite can satisfy bounded keyset reads from that index; (2) add RED coverage for both the bounded plan and real PostgreSQL migration/competing claims; (3) implement PostgreSQL DSN-gated isolated schema test, with explicit skip if `TRPC_TEST_POSTGRES_DSN` is unset; (4) run focused tests and refresh hashes. Preserve existing fairness and cooldown behavior.

### Repair implementation and evidence

- Added a partial index on `(updated_at, tenant_id, workspace_id, run_id)` for eligible sealed/advanced receipts to both migration streams. The down migrations remove it explicitly. Existing `(state, updated_at)` recovery index remains available to recovery scans.
- Added a SQLite `EXPLAIN QUERY PLAN` acceptance check with the fresh keyset predicate, `LIMIT 8`, and the partial index selected. It asserts indexed search and no temporary B-tree for sorting. The pre-fix RED run failed with the prior plan: `SEARCH c USING INDEX idx_craft_run_captures_recovery (state=?) ... USE TEMP B-TREE FOR ORDER BY`.
- Replaced the misleading PostgreSQL pseudo-subtest (which had called the SQLite fixture) with a `TRPC_TEST_POSTGRES_DSN`-gated test. When configured, it creates an isolated schema, applies the actual 000216 PostgreSQL migration, checks the index, then races global and targeted claimers on separate DB connections and asserts only one claim. When unset, the explicit skip says PostgreSQL migration/claim serialization is NOT VERIFIED.
- PostgreSQL execution remains unverified here because `TRPC_TEST_POSTGRES_DSN` is unset; the fixture is ready for an environment with the repository DSN configured.

Commands and results:

```text
go test ./internal/application/repository -run 'TestCraftRunCapturePromotionFreshScanUsesBoundedIndex|TestCraftRunCapturePromotionPostgresMigrationAndClaimSerialization' -count=1
RED: FAIL before migration index was added; planner selected idx_craft_run_captures_recovery and reported USE TEMP B-TREE FOR ORDER BY.

go test -v ./internal/application/repository -run 'TestCraftRunCapturePromotion(Migration|FreshScanUsesBoundedIndex|PostgresMigrationAndClaimSerialization)$' -count=1
PASS: SQLite migration Up/Down/Up and bounded keyset plan. PostgreSQL test explicitly SKIPPED because TRPC_TEST_POSTGRES_DSN is unset.

go test ./internal/container -run 'TestCraftT20(FreshPromotionQuotaSurvivesOverdueBacklog|TargetedAndGlobalPromotionClaimsSerializeSQLite|PromotionRecoveryPagesPastOlderReceipts|PromotionFailureCooldownIsBoundedAndRetryable)' -count=1
PASS: existing fairness, cursor progress, cooldown, and SQLite cross-path serialization regressions.

git diff --check
PASS.
```

Repair SHA-256 values:

```text
32dadb12cfdd20c7387d18b4db074f0f9c8d1982d94a76ca328a0520f968c8c5  internal/application/repository/craft_run_capture_promotion.go
6c347c29be18bf3c3247127e4bcf16d36f61e53b3a2557d50b66c3980c598666  internal/application/repository/craft_run_capture_promotion_test.go
e7f047de6ccc4d5cb2a89aa41955f1622bf93a7913af85c70f643e7b43c8d49c  migrations/sqlite/000137_craft_run_capture_promotion_scan.up.sql
2108919e75451a4bdd3b4a1d231c7f928da16f41beeec27f5a3ab29fefed59b4  migrations/sqlite/000137_craft_run_capture_promotion_scan.down.sql
3634bd3985ec8dc13a004500debe6d1b71aa4cebc0bea8af9fb9667d8886b7d4  migrations/versioned/000216_craft_run_capture_promotion_scan.up.sql
2108919e75451a4bdd3b4a1d231c7f928da16f41beeec27f5a3ab29fefed59b4  migrations/versioned/000216_craft_run_capture_promotion_scan.down.sql
```

### OCR fix round 2/5

The independent review reproduced a remaining F06 issue: the previous plan test forced the index in test SQL, while SQLite's actual parameterized query still selected `idx_craft_run_captures_recovery` and sorted through a temp B-tree. The production query now uses fixed terminal-state predicates matching the partial index and adds SQLite-only `INDEXED BY idx_craft_run_captures_promotion_scan`; PostgreSQL query text does not receive SQLite syntax. The shared production fresh-query builder drives both execution and plan verification.

The plan test seeds 96 representative sealed/advanced receipts, generates the exact SQL and bound arguments through the production GORM query builder, then explains the generated SQL with both a reset and persisted keyset cursor. Both plans must use the promotion scan index without `TEMP B-TREE`. The PostgreSQL fixture now adds schema selection to URL DSNs as a URL parameter and to keyword DSNs as a `search_path` option; a focused test covers both formats without printing DSN values.

Round 2 commands and results:

```text
go test ./internal/application/repository -run '^TestCraftRunCapturePromotionFreshScanUsesBoundedIndex$' -count=1
RED before production fix: FAIL; recovery index chosen and TEMP B-TREE FOR ORDER BY reported for the unforced production-shaped query.

go test ./internal/application/repository -run 'TestCraftRunCapturePromotion(Migration|FreshScanUsesBoundedIndex|PostgresMigrationAndClaimSerialization)$|TestPostgresDSNWithSchemaSupportsURLAndKeywordFormats' -count=1
PASS: SQLite migration and both generated production query plans; URL/keyword DSN selection tests pass. PostgreSQL migration/claim fixture skips because TRPC_TEST_POSTGRES_DSN is unset.

go test ./internal/container -run 'TestCraftT20(FreshPromotionQuotaSurvivesOverdueBacklog|TargetedAndGlobalPromotionClaimsSerializeSQLite|PromotionRecoveryPagesPastOlderReceipts|PromotionFailureCooldownIsBoundedAndRetryable|PromotionRejectsHeadAdvancedAfterValidationSQLite)' -count=1
PASS: fairness, cursor progress, cooldown, cross-path claim serialization, and F35 SQLite revision fence.

git diff --check
PASS.
```

Round 2 latest SHA-256 values:

```text
6b779e2c43e86e1082def644b3be5274c72e227f9ccbe0d5a26b5c6ae7f8840d  internal/application/repository/craft_run_capture_promotion.go
628f8ca13dd3376d803d08e8f59ea706b1248cca328341607746cb2b0bc474c9  internal/application/repository/craft_run_capture_promotion_test.go
```

### OCR fix round 3/5

The fresh scan now selects an ordered raw capture page from the matching partial index with `LIMIT` before checking attempt presence. The raw page is bounded by the remaining batch capacity (maximum 32); attempted rows are checked only by bounded key predicates for identities in that page. The durable keyset cursor advances to the last raw scanned row even when every row already has an attempt and the call returns no candidates. A shorter raw page marks tail exhaustion and resets the cursor. The singleton transaction lock remains common to global and targeted claim paths.

The repository regression seeds 96 terminal captures and future attempt rows, verifies an empty claim still advances the cursor through an eight-row page, inserts newer eligible work, and verifies bounded repeated calls reach it. Query-plan coverage uses SQL generated by the production raw-page builder with representative receipt rows and both reset/persisted cursor states. The existing overdue-backlog acceptance now checks eventual fresh-work progress across bounded ticks, since raw keyset paging intentionally does not search through all historical attempts in one call. URL and keyword PostgreSQL DSN support is retained.

Round 3 commands and results:

```text
go test ./internal/application/repository -run 'TestCraftRunCapturePromotion(Migration|FreshScanUsesBoundedIndex|CursorAdvancesAcrossAttemptedHistory|PostgresMigrationAndClaimSerialization)$|TestPostgresDSNWithSchemaSupportsURLAndKeywordFormats' -count=1
PASS. PostgreSQL migration/claim serialization explicitly skips because TRPC_TEST_POSTGRES_DSN is unset.

go test ./internal/container -run 'TestCraftT20(FreshPromotionQuotaSurvivesOverdueBacklog|TargetedAndGlobalPromotionClaimsSerializeSQLite|PromotionRecoveryPagesPastOlderReceipts|PromotionFailureCooldownIsBoundedAndRetryable|PromotionRejectsHeadAdvancedAfterValidationSQLite)' -count=1
PASS: eventual fresh-work progress, global/targeted claim serialization, cursor recovery, cooldown, and F35 revision fence.

git diff --check
PASS.
```

Round 3 latest SHA-256 values:

```text
81605b79a4c4dfc3271c5e06c4d69563f80853f59cb48fafe2e5b25d808e89b3  internal/application/repository/craft_run_capture_promotion.go
e901de39afb9a377c746e3469ca45b34b36cc2ee628b09f3323ccd375a7eeb07  internal/application/repository/craft_run_capture_promotion_test.go
597a191fa7b311dcfab460f89e2153e64d7496b0f585914e05fb6552ed490679  internal/container/craft_run_capture_promotion_test.go
```

### OCR fix round 4/5 — owned-file snapshot and plan

Starting hashes: repository implementation `81605b79a4c4dfc3271c5e06c4d69563f80853f59cb48fafe2e5b25d808e89b3`; repository test `e901de39afb9a377c746e3469ca45b34b36cc2ee628b09f3323ccd375a7eeb07`; container promotion test `597a191fa7b311dcfab460f89e2153e64d7496b0f585914e05fb6552ed490679`; SQLite migration up/down `e7f047de6ccc4d5cb2a89aa41955f1622bf93a7913af85c70f643e7b43c8d49c` / `2108919e75451a4bdd3b4a1d231c7f928da16f41beeec27f5a3ab29fefed59b4`; PostgreSQL migration up/down `3634bd3985ec8dc13a004500debe6d1b71aa4cebc0bea8af9fb9667d8886b7d4` / `2108919e75451a4bdd3b4a1d231c7f928da16f41beeec27f5a3ab29fefed59b4`.

Repair plan: order due retries by `(retry_after, tenant_id, workspace_id, run_id)` to match a composite due index in both dialect migrations; verify the actual generated query under a large equal-time backlog has no temp sort; add a tail exhaustion/reset and wrap test proving older newly eligible work is then claimed; rerun focused promotion, migration and F35 checks.

### OCR fix round 4/5 — implementation and evidence

- Due retries now order by their stable attempt identity after `retry_after`; migrations add `(completed, retry_after, tenant_id, workspace_id, run_id)` in SQLite and PostgreSQL. Equal-deadline retries therefore use one index order through `LIMIT` rather than sorting capture timestamps and identity columns afterward.
- Added a due query plan test with 96 captures and equal-time attempts, explaining SQL from the shared production query builder. The RED plan before the index/order change was `SEARCH a USING INDEX idx_craft_capture_promotion_retry (completed=? AND retry_after<?)`, capture PK lookup, then `USE TEMP B-TREE FOR LAST 4 TERMS OF ORDER BY`.
- Added a tail/wrap behavior test that advances through three full raw pages, verifies an empty page clears the cursor, deletes the oldest attempt, and confirms that receipt is claimable after wrap. It caught GORM's `UpdatedAt` auto-population during reset; reset now uses `UpdateColumns` so all nullable cursor fields remain cleared.
- Existing overdue-backlog coverage passes with repeated bounded fresh scans, preserving cursor progression and fresh work fairness.

Round 4 commands and results:

```text
go test ./internal/application/repository -run 'TestCraftRunCapturePromotion(Migration|FreshScanUsesBoundedIndex|CursorAdvancesAcrossAttemptedHistory|DuePageUsesBoundedOrderIndex|CursorWrapsAfterRawTailExhaustion|PostgresMigrationAndClaimSerialization)$|TestPostgresDSNWithSchemaSupportsURLAndKeywordFormats' -count=1
PASS: migration Up/Down/Up, fresh/due indexed query plans, raw cursor progress and wrap. PostgreSQL migration/claim test explicitly skips because TRPC_TEST_POSTGRES_DSN is unset.

go test ./internal/container -run 'TestCraftT20(FreshPromotionQuotaSurvivesOverdueBacklog|TargetedAndGlobalPromotionClaimsSerializeSQLite|PromotionRecoveryPagesPastOlderReceipts|PromotionFailureCooldownIsBoundedAndRetryable|PromotionRejectsHeadAdvancedAfterValidationSQLite)' -count=1
PASS: fairness, SQLite global/targeted serialization, cursor recovery, cooldown and F35 revision fence.

git diff --check
PASS.
```

Round 4 latest SHA-256 values:

```text
dacde1df21e422b82f87a16852bbb4bd6f7e9494d5a52775f6c6bd05787cb366  internal/application/repository/craft_run_capture_promotion.go
41cea6f850838f8b113394d842fcfe9265229f5c603dd17ad41620a542392d99  internal/application/repository/craft_run_capture_promotion_test.go
597a191fa7b311dcfab460f89e2153e64d7496b0f585914e05fb6552ed490679  internal/container/craft_run_capture_promotion_test.go
578dd4288487510e3a7e443e6e86530131d2972c76e2912aad07b6beef87cbe2  migrations/sqlite/000137_craft_run_capture_promotion_scan.up.sql
377e2b24274fe9f610a839fb4f3ee48b21353d2a842812d4a19754b0d4c1db1f  migrations/sqlite/000137_craft_run_capture_promotion_scan.down.sql
e533bf759d282597ede3c4de90b65cbb11fb1bacea90648b36058c9b6bfc0861  migrations/versioned/000216_craft_run_capture_promotion_scan.up.sql
377e2b24274fe9f610a839fb4f3ee48b21353d2a842812d4a19754b0d4c1db1f  migrations/versioned/000216_craft_run_capture_promotion_scan.down.sql
```
