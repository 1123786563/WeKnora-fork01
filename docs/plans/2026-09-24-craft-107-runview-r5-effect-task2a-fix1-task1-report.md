# Craft #107 R5 Effect Task2a Fix1 Task1 Report

## Result

`BeginEffect` now requires the exact RunView generation to have an allocation intent with matching tenant/Run/owner/session/actor/admission digest, a real stored writer fence, `state=finished`, `outcome=succeeded`, and receipt equal to generation. This check runs in the same locked Run transaction before replaying or inserting any provider-kind intent, and is shared by admitted allocation replay. Consequently, a legacy key-only RunView cannot produce a provider send permission, and missing, pending, failed, or foreign allocation intent identity fails closed.

`FinishEffect(succeeded)` now rejects a blank/whitespace receipt before mutating the pending claim, leaving it unresolved and replayable only as the same token with `maySend=false`. Effect-specific receipt associations remain a Task3 responsibility; no physical provider calls or production enablement were added.

## TDD and verification

- RED focused run demonstrated the finding: legacy allocation and malformed allocation intent cases returned `maySend=true`; successful finish with an empty receipt returned nil. A first pending-state fixture mutation violated the SQLite state/outcome check constraint; the fixture was corrected to a valid pending tuple before GREEN.
- Final SQLite and isolated PostgreSQL focused run: `TRPC_TEST_POSTGRES_DSN='postgres://postgres:craft107_local_test_only@127.0.0.1:32771/craft107_r5?sslmode=disable' go test ./internal/application/repository -run 'TestCraftRunView(Effect|EffectAuthority|EffectIntent)' -count=1` — PASS, 15.820s.
- Final race selector on the exact postimage with the same isolated PG DSN — PASS, 14.757s.
- `gofmt`, `git diff --check`, and explicit trailing-whitespace scan on both owned files — PASS.

## Exact checkpoint

- Preimage: `2026-09-24-craft-107-runview-r5-effect-task2a-postimage.tar.gz`, verified against the preimage hashes recorded in the checkpoint; preimage HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`.
- Postimage archive/manifest: `2026-09-24-craft-107-runview-r5-effect-task2a-fix1-task1-postimage.tar.gz` and `...-post.sha256`.
- Task-local patch/checkpoint: `2026-09-24-craft-107-runview-r5-effect-task2a-fix1-task1-task-local.patch` and `...-checkpoint.json`; patch SHA-256 `7ef217a2d1dca63211f1cfd3ca38aec739ce2da9e6140cebff6baea7f1a0d04a`. The patch contains only the two owned source/test files.
- Exact file hashes and command evidence are in the checkpoint JSON. No commit was created.

## Remaining gates

Task2b unresolved-intent transition integration and Task3 physical provider routing remain separate required gates. This change introduces neither provider calls nor enablement; the feature remains default-off.
