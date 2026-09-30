# Craft #107 T01 R5 Effect Authority Task2a Task1 Report

## Result

Implemented the typed `RunViewEffectAuthority` port and repository-backed authority store. `AllocateAdmitted` locks and validates the current admitted Craft Run, original Task/Fence digest, authenticated actor, active session writer slot, and Task Workspace binding before atomically creating one RunView and its completed allocation intent. `BeginEffect` repeats the authority validation and inserts one unique generation/kind intent with a random server token; exact replays return that same token with `maySend=false`. `FinishEffect` records a known outcome or durable `unknown`; changed outcome/receipt replay conflicts, and unknown does not grant a retry. An un-fenced legacy key-only allocation is not adopted by the new API.

Added SQLite 000121 and PostgreSQL 000200 effect-intent tables with unique Run/generation/kind and claim-token constraints, durable fence/digest/actor/scope identity, state constraints, exact RunView-generation foreign key, and unresolved-state index. No physical provider calls or transition wiring were added. The old `RunViewStore.Allocate(ctx,key)` remains for compatibility but is not authorization for any new physical call; Task3 must route all physical mutation paths through `AllocateAdmitted`/`BeginEffect`, and production must remain default-off until that integration is complete.

## TDD and verification evidence

- RED before the port/store implementation: `go test ./internal/application/repository -run '^TestCraftRunViewEffect' -count=1` failed compilation on the intentionally new, missing `RunViewEffect` contract/store symbols. This demonstrated the fenced API did not exist at the preimage.
- The first runtime probe after initial implementation failed because allocation persisted `succeeded` as a state, while the schema permits `pending|finished|unknown`. The implementation was corrected to `state=finished, outcome=succeeded` for the atomic database allocation.
- A first full-chain PostgreSQL migration-test attempt stopped before the R5 migration because this PG17 image lacks the base-chain `vector` extension. The final PostgreSQL effect behavior test sets and verifies `app.skip_embedding=true`; the PostgreSQL 000200 migration up/down/up runs in a separate unique schema with only its two prerequisite tables, so neither path needs the extension or touches `public`.
- Final GREEN on the exact postimage: `TRPC_TEST_POSTGRES_DSN=postgres://postgres:craft107_local_test_only@127.0.0.1:32771/craft107_r5?sslmode=disable go test ./internal/application/repository -run 'TestCraftRunView(Effect|EffectAuthority|EffectIntent)' -count=1` — PASS (`27.489s`). This runs the SQLite behavior and migration tests, a PostgreSQL admitted-Run allocation/claim/unknown-replay test, and PostgreSQL migration 200 absent/up/down/up.
- `gofmt -w internal/application/repository/craft_run_view_effect.go internal/application/repository/craft_run_view_effect_test.go` — PASS.
- `git diff --check` and trailing-whitespace scan of owned files — PASS.
- Added a lease-recovery regression: after the original allocation intent finishes, a newly claimed epoch reuses the same generation and does not create a second allocation intent. Provider effect replay continues to require the exact fence.

## Exact checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- Preimage HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit created.
- Preimage archive and SHA manifest: `2026-09-24-craft-107-runview-r5-effect-task2a-preimage.tar.gz`, `2026-09-24-craft-107-runview-r5-effect-task2a-pre.sha256`.
- Postimage archive: `2026-09-24-craft-107-runview-r5-effect-task2a-postimage.tar.gz`; postimage SHA manifest: `2026-09-24-craft-107-runview-r5-effect-task2a-post.sha256`.
- Task-local delta: `2026-09-24-craft-107-runview-r5-effect-task2a-task-local.patch` (SHA-256: `0ebf28d7b0d59a38403f6fcc290d9f12765186a30ba10c26b9a4425076859076`). It explicitly includes ignored migration SQL bytes.
- Exact postimage SHA-256 values:

| File | SHA-256 |
| --- | --- |
| `internal/application/repository/agent_run.go` | `aec1b2be82e5570e38aed2010f854b430c479f4f59717540308c163aebe0848a` |
| `internal/application/repository/craft_run_view.go` | `e54f8829a68248726014386d46d016c3a69ceb1f18950d74f736fcd2366963dd` |
| `internal/application/repository/craft_run_view_test.go` | `3a1520612858795e304924f795beb9652e2c36dbc10066483bbe0f1b83f46b89` |
| `internal/modules/craft/run_view_effect.go` | `99ce8924e492cba509a0a06e9545658a952f5dbd5e3db91b8f6ffef26fa159a0` |
| `internal/application/repository/craft_run_view_effect.go` | `a94a8289a5b9bd45d70e0920772432f00e6136c30285b7ac8fce69c58bec496d` |
| `internal/application/repository/craft_run_view_effect_test.go` | `fd38b46b61f2c6f89cb7540461ea3d1c1ed03f69445a259f5b7a3339fa5cb21f` |
| `migrations/sqlite/000121_craft_run_view_effect_intent.up.sql` | `6c17dbec058eb5910a46dcd0e345bad64f35761809bfb618b961e64702588398` |
| `migrations/sqlite/000121_craft_run_view_effect_intent.down.sql` | `e03db5c89b066a61c5b1c687854f2ed565618f69210b025d182d2e22cbb68bf5` |
| `migrations/versioned/000200_craft_run_view_effect_intent.up.sql` | `f6cb5de9e04a1ef138c6b04c205d71047019842ff5548eecd98a9bb4c382167e` |
| `migrations/versioned/000200_craft_run_view_effect_intent.down.sql` | `e03db5c89b066a61c5b1c687854f2ed565618f69210b025d182d2e22cbb68bf5` |

## Remaining integration gate

- Task2b must make every cancellation, wait/terminal transition, lease reclaim, and writer-slot transfer check `hasUnresolvedCraftRunViewEffects` under the same Run transition lock. Until then, this store does not make a claimed provider operation safe to dispatch.
- Task3 must use this typed authority for allocation and each Docker/OpenCode mutation; the existing key-only allocator is intentionally not upgraded into an effect claim.
- No Docker/OpenCode calls, production enablement, or route changes were made.
