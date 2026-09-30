# T14 preview → integration seam map (read-only)

Date: 2026-09-24. Compared T14 worktree `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01` (HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`) with integration worktree `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01` (HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`). No source, test, remote issue, staging, or commit was changed; this report is the sole new file.

## Verified state

T14’s reviewed preview service changes are present in T14’s committed range (`ef3a0a867` and `e2f335dc6`). The integration tree has an older preview seam and does not contain those service-level hardening changes:

| Seam | T14 state | Integration state | Meaning |
|---|---|---|---|
| `internal/application/service/craft_preview.go` | Adds `TaskAccessChecker`, `NetworkChecker`, live Docker `network_mode=none` inspector/checker, `BrowserNavigationProtected`, `AcceptsPreviewHost`, and issue/open/read checks. | Has only origin/TTL config; `Enabled()` checks nonempty origin; no access/network/browser gates. | T14 implementation is absent from integration and is the primary transfer candidate. |
| `internal/handler/session/craft_preview.go` | File route gates with `AcceptsPreviewHost(c.Request.Host)`. | File route gates only `Enabled()`. | Must transfer with the service API; otherwise the isolated host check is lost. |
| `internal/handler/session/craft_preview_test.go` | T14 journey fixtures inject access, no-egress checker, browser gate and preview host; includes T14 journey coverage. | Older fixtures do not inject these dependencies. | Transfer tests together with the service/handler seam, then adapt to integration fixtures. |
| `internal/handler/session/craft_preview_policy_test.go` | Byte-identical to integration. | Byte-identical. | No transfer needed. |
| `internal/container/container.go` | T14 branch has older, conflicting assembly edits that remove T08 feature wiring and does not provide the final central preview constructor. | Integration has current T08 assembly and `newCraftPreviewService` with only origins. | Do not copy T14 `container.go`; add the narrow dependency wiring on the integration version after B3/T08 access is stable. |
| `internal/router/routes_chat.go`, `routes_craft_features_test.go` | T14 versions remove/alter T08 route-feature wiring. | Integration versions contain current T08 route work and are dirty. | Do not copy T14 router files. Preserve integration route assembly. |

T14 render-boundary files (`deploy/craft/render-boundary/*`) are dirty/untracked in T14 and are already represented by separate T14 review artifacts in integration; they are not part of the B3 server guard transfer. The T14 render/browser reviews consistently state that live browser/broker and same-bound-container evidence is **not verified**, and that `BrowserNavigationProtected` must remain false until such attestation exists (`docs/plans/2026-09-23-craft-107-t14-review.md`, `...-fix1-review.md`, `...-render-boundary-review.md`, `...-request-correlation-review.md`).

## B3 stable interface

The B3 guard should consume the narrow existing port `craft.TaskAccessChecker`, called with the current `craft.Scope` and `craft.TaskPreview`. T14’s service calls it at ticket issue, capability open/redemption, and every file lookup/read. The intended contract is fail-closed when the checker is nil or denies; a previously issued capability does not outlive current TaskPreview authority. The network seam is the narrow `CraftPreviewNetworkChecker` (`CheckPreviewNoEgress(ctx, scope) error`), with the Docker implementation resolving the bound sandbox rather than accepting a caller-selected container ID. `CraftPreviewService.Issue`, `Open`, and `Resolve` remain the service API; the handler continues to expose issue on the authenticated session route and file reads on the isolated preview host.

Evidence source: `docs/plans/2026-09-24-craft-107-t08-remaining-gap-map.md` B3 section (issue, redemption, every read and revoke tests), T14 review lines describing `TaskAccessChecker`/`NetworkChecker`, and the T14 service symbols listed above. B3 must not replace the current isolated-origin/no-main-site-cookie shape with a main-site authenticated file route.

## Safe transfer / checkpoint sequence

1. Freeze and record the integration baseline (`HEAD`, status, and hashes) for the four owned seam files before any copy. Preserve all current dirty integration edits, especially `container.go`, `routes_chat.go`, and route tests.
2. From T14, create a path-scoped checkpoint/patch for the reviewed service/handler/module files only. Do not cherry-pick `ef3a0a867` or `e2f335dc6` wholesale: both commits contain unrelated render-boundary/UI work, and the T14 `container.go`/router versions conflict with current T08 integration wiring.
3. Apply the service and handler changes onto the integration versions with a three-way/manual review. Include the T14 service tests and network test only after checking imports and existing integration fixture seams. Leave render-boundary proof files and all T08-owned router/container edits untouched.
4. Recompute full-file hashes and save a checkpoint manifest plus task-local patch in `docs/plans/` before B3 implementation. Run focused preview/service/handler tests and `git diff --check`; the checkpoint must identify every added/deleted file.
5. B3 then owns the guard tests and the minimal integration wiring in the existing `newCraftPreviewService` constructor: inject the real T08 `TaskAccessChecker`; inject the selected-bound-sandbox network checker only when its binding/inspector is available; retain `BrowserNavigationProtected=false` until the separately reviewed browser boundary is actually attested. Do not enable the static bool as a substitute for that evidence.
6. After B3, validate issue → redeem/open → second read after revoke/member removal through the real route/assembly. Re-run the focused preview tests and an independent review against the checkpoint; only then map the result into the B5 joined journey.

## Ownership and conflicts

* T14-owned transfer candidates: `internal/application/service/craft_preview.go`, `internal/handler/session/craft_preview.go`, their focused tests, and the narrow craft preview contract additions required by compilation.
* B3/T08 integration-owned files: `internal/container/container.go` (constructor dependency injection), `internal/router/routes_chat.go`, `internal/router/routes_craft_features_test.go`, and B3 guard/route integration tests. These are currently dirty in integration and must not be overwritten.
* Shared boundary: the service config and `TaskAccessChecker` port. Resolve against the integration `craft`/T08 definitions first; any signature mismatch is a blocking interface issue requiring a new checkpoint, not a silent rename.

Conclusion: T14’s reviewed security hardening is absent from the integration service/handler, while integration already contains conflicting and more recent T08 assembly edits. Transfer only the narrowly reviewed preview service/handler delta with a hashable checkpoint; let B3 add the real access dependency and revocation tests in the integration-owned constructor/route seam, keeping preview closed until live browser/network attestation exists.
