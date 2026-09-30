# T01 Guarded Input Route Mount Report

## Result

Mounted the T01 input-round and input-decision routes through the container-owned `CraftFeatureRoutes` registry before router construction. The routes inherit the production `/api/v1/sessions` Viewer and API-key chat policy and retain the constrained Craft path registry. Missing service/registry and duplicate registration fail closed.

The existing two-round HTTP fixture now explicitly posts `continue` for its accepted but unrecognized `.txt` input before starting a Run. Production decision enforcement remains unchanged.

This checkpoint does not claim per-Run isolation, admission recovery, or full T01 verification.

## Changed files and hashes

Exact full-content change patch: [checkpoint patch](2026-09-23-craft-107-t01-route-mount-checkpoint.patch), SHA-256 `d38969bc05382a793dae9e88753f4780925d63280f9e727fbed440cbea5b38d4`.

Manifest with before/after tracked hashes, source hashes, task role, and exact HEAD: [checkpoint manifest](2026-09-23-craft-107-t01-route-mount-checkpoint.json).

Owned source/test files:

- `internal/container/craft_input_wiring.go` — `66389e0a7bef930642f0a2831eb2b7d3dd5addc9850ae2e19f03dca337695dba`
- `internal/container/craft_input_wiring_test.go` — `8cd752eb3fc35223f09b772937c18cee0be85dc53e6d8eb85712ea4c732143e4`
- `internal/container/container.go` — `979d27e9ed22e1ece32b127070f7ba640f3bc16767eeb5fac4ad0649f2b9fa35`
- `internal/router/craft_input_routes_test.go` — `d22da26a43c10b1914bd065582024e842da9f723c8c56f9bffde0da8a298af88`
- `internal/handler/session/craft_test.go` — `bee5cedf86fa42bd1c5b5c753fa8ccb62d7c527765b576045a7420dad6ce0788`

## TDD and verification evidence

RED:

- `go test ./internal/container -run '^TestWireCraftInputFeature' -count=1` failed to compile because `wireCraftInputFeature` did not exist.
- Initial router journey failed because the synthetic test router omitted global auth/error handling; after modeling those production middleware boundaries, it passed.
- Before the fixture update, `TestCraftHTTPOwnerCreatesUploadsRunsTwoRounds` failed with HTTP 409: `input "doc-ready.txt" requires a continue decision for this Run`.

GREEN checks, all exit 0:

- `go test ./internal/container -run '^TestWireCraftInputFeature' -count=1`
- `go test ./internal/router -run '^TestCraftInputRoutesInheritProductionSessionAuthAndAPIKeyPolicy$' -count=1`
- `go test ./internal/container -run '^TestCraftAccessFeature|^TestWireCraftInputFeature' -count=1`
- `go test ./internal/router -run '^TestCraftInputRoutesInheritProductionSessionAuthAndAPIKeyPolicy$|^TestCraftFeatureProductionRouterPolicyAndEscape$' -count=1`
- `go test ./internal/handler/session -run '^TestCraftInputRoundHTTP$|^TestCraftHTTPOwnerCreatesUploadsRunsTwoRounds$' -count=1`
- `go test ./internal/application/service -run '^TestCraftT01InputRoundRollsBackRowsAndPreservesExistingBlob$' -count=1`
- `git diff --check`

The route journey invokes production `RegisterSessionRoutes` and `RegisterCraftSessionRoutes`, verifies the exact mounted POST paths and API-key policies, and covers owner upload/decision, anonymous rejection, scoped API key without chat, scoped API key with chat, and cross-tenant rejection from the service seam. The legacy handler journey uses the real Craft session service/database with only its documented test ports faked.

## Scope and checkpoint

- HEAD before/after: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit).
- Shared worktree contained unrelated T01/T05/T14 and integration edits before this task. They were preserved and are excluded from this task patch.
- Runtime model and reasoning effort were not exposed in this task context; no model/effort claim is made.
