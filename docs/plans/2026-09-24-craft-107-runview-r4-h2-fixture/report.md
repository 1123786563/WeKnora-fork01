# T01 R4 H2 fixture compatibility — task report

## Scope

Only `internal/container/craft_knowledge_runview_h2_test.go` is changed. The originally assigned `craft_runview_live_engine_h2_test.go` was left unchanged after checking the R4 Fix1 report: the two failing runtime-boundary tests are in this knowledge H2 file. Parent confirmed this corrected ownership.

## RED evidence

- `go test ./internal/container -run '^TestLocalCraftRuntimeRejectsKnowledgeMutationAfterResolver$' -count=1` failed at `changed_published_bytes_after_resolver` because the fixture returned `craft forbidden: incomplete durable Run writer fence` instead of reaching the final verifier's `craft.ErrNotFound` assertion.
- `go test ./internal/container -run '^TestLocalCraftRuntimeAcceptsIdenticalKnowledgeRetryAndRestart$' -count=1` failed because the fixture returned `craft forbidden: incomplete durable Run writer fence`.
- A combined rerun also showed the restart test's incomplete-fence failure.

## Change

The H2 fixture now adds a positive epoch to its SQLite `agent_runs` row and matching `task.Fence`/`Run` owner and epoch. It adds a typed `service.CraftWorkspaceSeedSnapshot` in explicit empty revision-zero state for the fixture workspace. Runtime parsing and production checks remain unchanged; original knowledge assertions are intact.

## GREEN / remaining verification

GREEN `go test ./internal/container -run '^TestLocalCraftRuntimeRejectsKnowledgeMutationAfterResolver$' -count=1` — PASS; `go test ./internal/container -run '^TestLocalCraftRuntimeAcceptsIdenticalKnowledgeRetryAndRestart$' -count=1` — PASS. Original final-verifier, accepted retry/restart, no-search, no-republication and byte-stability assertions remain intact.

Full `go test ./internal/container -count=1` — FAIL with two unrelated assembly cases: `TestCraftAccessFeatureRegistriesAreAssemblyOwned` (missing `craft.TaskAccessChecker`) and `TestWireCraftInteractionRegistrarRegistersPendingInteractions` (`agent runtime conflict`). No H2 test failed. This matches the existing R4 Fix2 full-package failure classification. The Docker-gated live-engine test was not run; this patch does not alter it.

`gofmt -d internal/container/craft_knowledge_runview_h2_test.go` and `git diff --check --no-index` against the saved preimage were clean.

## Checkpoint

- HEAD: see `base.txt`; no commit created.
- Preimage SHA-256: `bf57256860ddacf6f736ce99dd7df0e861a61c82794ebf3c845756f4a41e8b9b`
- Postimage SHA-256: `923b7009bd15712e9e65dcf455997d1a79e3c833128a5ed73f8391c3ec852219`
- Task-local patch SHA-256: `2d1463deeafaba5c0eff1655080a547e47178c84780ff182048fedf8b77ca433`
