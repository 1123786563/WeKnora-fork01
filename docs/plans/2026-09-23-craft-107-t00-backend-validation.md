# T00 / #119 Backend Validation

Status: **DONE_WITH_CONCERNS**

## Scope and revision

Validated only T00 / #119 against the brief and captured acceptance criteria. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`. HEAD/BASE: `4bcad69baf033a1310b4dce1372c8153e66adc81`; implementation remains uncommitted. Checkpoint: `.superpowers/sdd/2026-09-23-craft-107-implementation/t00-checkpoint-01`; recorded checkpoint digest from `checkpoint.sha256`: `f3f0eae50510c6094ab04c33b7aad701e7cf7862e30ebf708d8ba69984ed5d63`. All 14 paths in `manifest.json` exist and match their recorded SHA-256 values (manifest check exit 0).

## Focused checks run

| Command | Exit | Evidence |
| --- | ---: | --- |
| `go test ./internal/handler/session/... -run '^TestCraftFeatureRoutesInheritGuardsAndFreezeInNameOrder$' -count=1` | 0 | `ok github.com/Tencent/WeKnora/internal/handler/session 2.781s`; exercises parent middleware inheritance, alphabetical mount order, freeze-on-registration, and mounting the frozen registry into a second router. |
| `go test ./internal/modules/craft/... -run '^TestWebContractFacts$|^TestWebConsentAndBudgetContracts$' -count=1` | 0 | `ok github.com/Tencent/WeKnora/internal/modules/craft 3.519s`; validates typed fact values, invalid enumerants/relationships, and required identifiers/bounds. |
| `git diff --check` | 0 | No whitespace errors. |

Reused same-checkpoint evidence in `docs/plans/2026-09-23-craft-107-t00-report.md` for `go test ./internal/modules/craft/... -count=1`, `go test ./internal/application/service/... -run Craft -count=1`, `go test ./internal/handler/session/... -run Craft -count=1`, `pnpm test:shared`, `pnpm typecheck:web`, `go test ./internal/container/... -run '^$' -count=1`, and the web workbench slot tests. The report records all as exit 0; this validation run did not rerun those suites.

## Acceptance assessment

- Separate accepted/understood input facts: **met**; DTO is optional/additive and legacy omission is covered by the report’s handler test evidence.
- Independent build, entry, preview reachability, and page-load evidence with passed/failed/not_run: **met**; typed validation passed.
- Stop request, confirmed cancellation, and unknown: **met**; typed validation passed.
- Writer acquisition acquired/conflict/unknown: **met**; typed validation passed.
- Typed budget, restricted contribution, share decision, export manifest, and export decision: **met**; typed validation passed.
- Stable backend and workbench extension slots with compatibility: **met based on checkpoint report/test evidence**; backend route guard/order/re-mount test passed; existing Craft suite and workbench-slot test evidence is reused from the same checkpoint report.
- Ownership matrix for central T00/T20 files: **present** at `docs/plans/2026-09-23-craft-107-ownership.md` and included in checkpoint manifest.

No T00 acceptance gap was found in the inspected contract and registration seams. API authorization proven here is inheritance of the caller-provided authenticated session group; Task-level authorization remains owned by consuming services and is outside T00’s implementation scope.

## Concern / evidence limit

The implementation report quotes a sorted file-hash digest `b734861eefc61d2c13853cf125deb9060cc7ce3193bdb330465264caeedf1f9e`. Recomputing SHA-256 over the current 14 `shasum -a 256` lines (sorted by path, as described) produced `9456e86c4d60609fb7f9762885c9c876f7e694824c844b07501608e7db9aba9d`, so that report-level digest does not reproduce. The checkpoint manifest’s individual file hashes do match the current files, and the checkpoint directory records digest `f3f0eae50510c6094ab04c33b7aad701e7cf7862e30ebf708d8ba69984ed5d63`; treat the quoted `b734…` aggregate as stale or calculated with an undocumented method. Full-suite results are reused from the implementer report rather than raw logs independently reproduced here.

## Fix round 1 — focused backend validation

Reviewed `docs/plans/2026-09-23-craft-107-t00-review.md` (both Important findings: missing per-file export origins in the consent-bound manifest; route extensions could escape the Craft Task namespace and bypass intended route policy) and the revised fix evidence in `docs/plans/2026-09-23-craft-107-t00-report.md`.

Revision remains HEAD/BASE `4bcad69baf033a1310b4dce1372c8153e66adc81`. Checkpoint `t00-checkpoint-02` records digest `dcb8d8adc44113df8ec54a3831f9782747165bb0cebbe23b6e1ffc3a28457114`. All 15 manifest entries matched both current worktree contents and checkpoint snapshot copies (0 missing, 0 mismatches; verification command exit 0). The reconstructed path-sorted digest is `9d3a4e1d2f1197674b349ecdfb7a83bd6ef39af91ace14c752dbd82893e33ef0`.

Focused checks:

| Exact command | Exit | Result |
| --- | ---: | --- |
| `go test ./internal/modules/craft/... -run 'WebExportConsent|WebConsent' -count=1` | 0 | `ok github.com/Tencent/WeKnora/internal/modules/craft 1.366s`; export-origin consent contract tests pass. |
| `go test ./internal/router/... -run CraftFeatureProductionRouterPolicyAndEscape -count=1` | 0 | `ok github.com/Tencent/WeKnora/internal/router 1.110s`; production router policy and escape test passes. |

Acceptance gaps: none found for the two reviewed findings. Export manifests now carry typed origins into the consent-bound view, while original-source authorization remains separate. The constrained route slot's production policy/escape test passes, supplementing the focused session route test from the prior validation round. Same-checkpoint full-suite evidence is recorded in the revised implementer report and was reused as instructed; this round ran only the two focused Go commands above. The previous aggregate digest concern is clarified in the revised report: `b734…` used non-sorted argument order and is not a checkpoint identity; checkpoint-02's manifest and recorded digest now verify.
