# T01 Workspace Draft Seed S2 PostgreSQL Validation

## Scope and result

Validated only S2 Task 2 Fix1 acceptance evidence at integration checkpoint `a5e9195acd6500c085c85d60c852148e7bbbbf34` plus its uncommitted Fix1 files. No source, test, requirement, or remote issue was changed. This report is the only file created by this validation.

**Status: DONE_WITH_CONCERNS.** PostgreSQL 17 verifies registered Craft marker enforcement, non-Craft compatibility, admission rollback, scheduler-driven admission/head contention, S1 migration up/down/up, and the S1 same-revision CAS test. The central same-key replay-after-head-advance test fails on PostgreSQL with `agent runtime conflict` at `agent_run_craft_seed_test.go:155`; SQLite passes the same subtest. Therefore the Fix1 replay acceptance criterion is not verified on PostgreSQL.

## Checkpoint identity

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD/base: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (matches `2026-09-24-craft-107-workspace-draft-seed-s2-task2-fix1-base-head.txt`)
- The three live owned files match the Fix1 checkpoint `post_sha256` values:
  - `internal/application/repository/agent_run.go`: `7476cabff6f9c1a8939489b5e301145f2f65e494225cc5361c66eb8b5e384001`
  - `internal/application/repository/agent_run_craft_seed_test.go`: `feca684053c195e10ccbb73fb9bfe99c0a696f758a8478e8b5c8624b47906f70`
  - `internal/application/service/agent_run_graph_test.go`: `1e2d9a14af812ebefb7ae38a0ebf99fe5f0867157c919f0befbcb5258fad5f27`
- Fix1 checkpoint JSON SHA-256: `6cc5e61b5745363dc88ab21b9340b8f5928829514025a6fc76447fcee3dcf1c2`
- Fix1 incremental patch SHA-256: `33e92c927b3edf957357857a4452b45a94174318f14d45152e1ec82b170f3d03`
- Repository status contained extensive pre-existing shared-worktree changes. No source/test file was edited by this validation.

## PostgreSQL environment and cleanup

- Image: `paradedb/paradedb:v0.22.2-pg17`
- Server: PostgreSQL `17.9 (Debian 17.9-1)` on `aarch64-unknown-linux-gnu`
- Disposable container ID: `8dcc5865293373a940bd2cf9b74086c610444ec036782fcb46cc788c770577e2`
- Disposable database: `craft_validator`; login role: `craft_validator`; test subtests create/drop their own random schemas. The role was made `SUPERUSER` inside this disposable instance because the full migration chain updates the image's `pg_search` extension. The initial least-privilege run failed during migration with `must be owner of extension pg_search`; after the disposable-role elevation, the suite reached application assertions.
- `docker rm -f craft107-s2-pg17-validator` — exit 0. Follow-up `docker ps -a --filter name=craft107-s2-pg17-validator --format '{{.ID}} {{.Status}} {{.Names}}'` — empty output; container removed.

## Commands and outcomes

1. First attempt (before disposable role elevation):

   ```sh
   TRPC_TEST_POSTGRES_DSN='postgres://craft_validator:craft_validator_pw@localhost:55440/craft_validator?sslmode=disable' go test ./internal/application/repository ./internal/application/service -run 'CraftSeed|DraftHeadAdmission|CraftDraftHeadAdvanceSameRevisionCASLeavesOneManifest|TestExecuteDurableCraftRunRejectsLegacySnapshotWithoutWorkspaceSeed|TestExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery|Craft.*StartRun' -count=1 -v
   ```

   **Exit 1.** PostgreSQL migration setup failed because the login role did not own `pg_search`. SQLite subtests and service package passed; this first run is environment setup evidence only.

2. Same focused command after granting `SUPERUSER` to the disposable test role:

   ```sh
   TRPC_TEST_POSTGRES_DSN='postgres://craft_validator:craft_validator_pw@localhost:55440/craft_validator?sslmode=disable' go test ./internal/application/repository ./internal/application/service -run 'CraftSeed|DraftHeadAdmission|CraftDraftHeadAdvanceSameRevisionCASLeavesOneManifest|TestExecuteDurableCraftRunRejectsLegacySnapshotWithoutWorkspaceSeed|TestExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery|Craft.*StartRun' -count=1 -v
   ```

   **Exit 1 overall.** Repository package failed only at `TestAgentRunCraftSeedAdmissionFreezesAndReplaysOriginalHead/postgres`, line 155, where replaying the same request after advancing the head returned `agent runtime conflict`; its SQLite subtest passed. In the same run, PostgreSQL passed marker cases missing/null/malformed/incomplete, non-Craft generic admission, owner/source mismatch cases, rollback of writer slot and input claim, corrupt/foreign head rejection, and `TestAgentRunCraftSeedAdmissionSerializesAgainstDraftAdvance/postgres`. PostgreSQL passed `TestCraftDraftHeadAdvanceSameRevisionCASLeavesOneManifest/postgres`. Service package exited 0; selected `CraftSessionStartRun` actor, scope, submit, replay, and workspace validation tests passed. Legacy missing-seed worker failure-before-resolution and actor recovery tests passed.

3. Isolated S1 migration proof on PostgreSQL 17:

   ```sh
   TRPC_TEST_POSTGRES_DSN='postgres://craft_validator:craft_validator_pw@localhost:55440/craft_validator?sslmode=disable' go test ./internal/application/repository -run '^TestCraftDraftHeadMigrationUpDownUp$' -count=1 -v
   ```

   **Exit 0.** `TestCraftDraftHeadMigrationUpDownUp/postgres` passed.

## Acceptance gaps and risks

- **Replay parity is an unresolved acceptance gap.** The PostgreSQL failure is at the replay assertion after revision D2 becomes current; the test expects the original admitted run and frozen D1 seed. This is observed as a test failure, not attributed to a specific root cause here. S2 replay acceptance is not green until resolved and rerun.
- **Both transaction orderings are not deterministically forced.** `TestAgentRunCraftSeedAdmissionSerializesAgainstDraftAdvance` passed in SQLite and PostgreSQL but synchronizes only on a start channel and accepts scheduler order. It does not independently prove admission-before-Advance and Advance-before-admission. The existing code/test seam offers no controlled pause point; no source or test hook was added under this validation assignment.
- **S1 PostgreSQL CAS evidence is limited.** The existing CAS test passed on PostgreSQL but submits identical source Run/manifests from both contenders. It proves one winner and one persisted revision, not that a losing distinct manifest can never become current. This limitation is also disclosed in the S1 Task 2 review/report.
- The validation role used elevated privileges only inside the disposable container to satisfy extension migration setup. The container and its database/role were removed; no external database was touched.
