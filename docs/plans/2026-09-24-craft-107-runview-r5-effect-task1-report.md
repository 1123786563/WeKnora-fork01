# R5 Effect Task 1 — durable network and probe claims

**Checkpoint:** `craft107-r5-effect-task1`
**Plan:** [R5 Effect Task3b physical provider plan](2026-09-24-craft-107-runview-r5-effect-task3b-plan.md)
**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
**Base HEAD:** `a5e9195acd6500c085c85d60c852148e7bbbbf34`
**Commit:** none.

## Scope and interface decision

Implemented Task1 only: durable effect kinds `docker_network_create` and `docker_probe`, request-spec digest pinning, repository tests, and paired SQLite/PostgreSQL schema migrations. Migration slots were vacant immediately before changes: SQLite `000126` (previous tip `000125`) and PostgreSQL `000205` (previous tip `000204`). Both migration directories are ignored by the repository's global ignore rule and need force-tracking in the final workspace review.

The existing `BeginEffect` contract does not receive provider request data. Kind-only authorization would therefore not pin which deterministic network/probe request the one-shot token permits. Added the separate `RunViewEffectRequestAuthority.BeginEffectWithDigest` seam, leaving `RunViewEffectAuthority` and existing kind call sites unchanged. The caller supplies a lowercase 64-character SHA-256 of its complete canonical, domain-separated request. Task2/3 must canonicalize all physical request inputs at their boundary (network: name, driver, internal, labels, scope/options; probe: target container, operation and expected runtime identity). The repository stores and compares the digest; it does not canonicalize provider data. The two new kinds reject legacy `BeginEffect`, invalid digest, or changed digest on replay. Existing effect kinds retain legacy behavior.

The SQLite rebuild copies all old intents with empty request digests and recreates the unresolved index and Run/RunView foreign keys. PostgreSQL adds a nonnull defaulted column and tightens the existing kind constraint while adding digest checks. Both down migrations refuse while either new kind has durable rows, avoiding loss or relabeling of authorization history.

## RED → GREEN evidence

### RED

`go test ./internal/application/repository -run '^TestCraftRunViewEffect(RequestAuthority|RequestAuthorityRequires)' -count=1` initially failed to compile because `BeginEffectWithDigest` and the persisted request-digest field did not exist. This was the expected missing-interface failure.

### GREEN

Final focused verification:

- `go test ./internal/application/repository -run '^TestCraftRunViewEffect' -count=1` — PASS (91.733s). This covers all effect repository and transition/fence behavior. This non-race run preceded the final test-only SQLite structure assertion and logger adjustment; the final snapshot was then covered by the race and dialect-specific migration runs below. A concurrent T19 source edit caused transient compile failures during an interim attempt and was corrected by its owner before this passing run.
- `go test -race ./internal/application/repository -run '^TestCraftRunViewEffect' -count=1` — PASS (33.326s) on the final source and tests.
- `TRPC_TEST_POSTGRES_DSN='<isolated local PG17 DSN; credential omitted>' go test ./internal/application/repository -run '^TestCraftRunViewEffectRequestDigestMigrationPreservesRowsUpDownUp$/postgres$' -count=1 -v` — PASS. The test creates a unique isolated schema, applies baseline effect migration 200, advances only migration state to 204, then verifies migration 205. It checks old-schema rejection, data preservation, new-kind request digest constraints, refusal to downgrade with a new-kind row, and down/up preservation. It deliberately does not execute the vector-dependent base chain.
- `go test ./internal/application/repository -run '^TestCraftRunViewEffectRequestDigestMigrationPreservesRowsUpDownUp$/sqlite$' -count=1 -v` — PASS. Verifies old-schema rejection, existing row preservation, request digest column, unresolved index, both composite foreign keys (5 column mappings), and up/down/up.
- `go test ./internal/modules/craft -run '^TestRunView' -count=1` — package compiled; no tests matched the selector.
- `gofmt -d internal/modules/craft/run_view_effect.go internal/application/repository/craft_run_view_effect.go internal/application/repository/craft_run_view_effect_test.go` — no output.

A transient SQLite assertion initially expected four foreign-key column mappings; the observed schema correctly had five (three for RunView and two for Run), and the test now asserts five. An interim PG migration test hit PostgreSQL's cached-plan result-shape guard because GORM selected `*` around DDL; the migration test now uses a stable explicit projection and passes cleanly. The owner fixed two temporary T19 compile mismatches in shared service code; the final effect selector/race commands passed after those fixes. No Docker/provider operation was run; Task1 does not own the physical adapter and the E0/T14 Docker gate remains separate.

## Remaining integration boundary

Task2/3 must construct the canonical digest from the complete deterministic network/probe request and call `BeginEffectWithDigest`. This Task intentionally stores and compares the supplied digest without trying to derive Docker request serialization from a generic Craft Task. The PG run validates schema/migration constraints only; full PostgreSQL repository claim behavior was not run because it would require the unavailable vector extension full chain.

## Exact uncommitted checkpoint

- Preimage archive: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task1-preimage.tar.gz`
- Preimage hashes: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task1-pre.sha256`
- Task-local patch: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task1-task-local.patch`
- Postimage archive: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task1-postimage.tar.gz`
- Postimage hashes: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task1-post.sha256`
- Machine-readable checkpoint: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task1-checkpoint.json`
