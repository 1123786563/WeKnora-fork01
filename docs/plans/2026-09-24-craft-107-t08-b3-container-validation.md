# T08 B3 container wiring validation

**Status: DONE_WITH_CONCERNS**  
**Validated revision:** `a5e9195acd6500c085c85d60c852148e7bbbbf34`  
**Workspace:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
**Scope:** T08 B3 production preview constructor binding and focused current-access/revocation behavior. This is not full T08/T14 acceptance.

## Acceptance result

- **Persistent TaskPreview checker injection: PASS.** `internal/container/container.go:2547-2558` takes the registered `*service.CraftAccessService` and assigns that same instance to `CraftPreviewConfig.AccessChecker`. Container registration at line 517 uses this constructor; the persistent access service is made available in the container at lines 489-492.
- **Production preview remains gated: PASS.** The production config sets app/preview origins and `AccessChecker` only. It does not set `BrowserNavigationProtected` or a `NetworkChecker`, so those fields retain their fail-closed zero values. `TestCraftPreviewConstructorUsesCurrentPersistentTaskAccessAndStaysDisabled` passes and confirms production issuance remains unsupported.
- **Current grants/revocation and no bytes after denial: PASS for tested paths.** The container test exercises persistent owner/Viewer allow and Admin/cross-tenant/revoked/stale-membership deny behavior. The handler persistent-access test confirms a previously issued capability cannot serve bytes after grant revocation. Service and preview journey tests pass.
- **Full T08/T14 acceptance: NOT VERIFIED.** Live browser navigation protection, selected-bound-sandbox no-egress, and joined production-route/assembly proof remain outside this B3 validation and are explicitly pending in the B3 plan/review.

## Commands and results

All commands ran at the validated revision above; raw output is saved in `2026-09-24-craft-107-t08-b3-preview-wiring-task1-checkpoint/{container,service,handler}-validation.log`.

| Command | Result |
|---|---|
| `go test ./internal/container -run '^TestCraftPreviewConstructorUsesCurrentPersistentTaskAccessAndStaysDisabled$' -count=1` | PASS (`internal/container`, 1.755s); linker emitted a duplicate `-lc++` warning only |
| `go test ./internal/application/service -run '^TestCraftT08Journey$' -count=1` | PASS (`internal/application/service`, 1.012s) |
| `go test ./internal/handler/session -run '^TestCraftPreview(CapabilityStopsServingAfterPersistentGrantRevocation|T14Journey)$' -count=1` | PASS (`internal/handler/session`, 0.920s) |
| `git diff --check -- internal/container/container.go internal/container/craft_preview_access_wiring_test.go internal/handler/session/craft_preview_persistent_access_test.go` | PASS (no output) |

The three B3 owned-file hashes match the checkpoint post-image: `container.go` `08ac3d8de1e141cdf61af22ea1af67de7cb0ce6d3b2a783f26a6edeeb156d67a`; container wiring test `54278ea0cf5e8ce4221b02cf1fed8a72019803fbc182d86d1e370c290b52726c`; persistent preview handler test `77fa21edd8d80d951716b30e3876b733e5462d20adf48f4b16b38f9d52e29e3f`. Task-local patch SHA-256 remains `6ec5a06367e38fa864720ba8bba7601aa3fa6842cfd46ec246a600d015ea8b69`. HEAD did not change during validation.

## Gaps and risks

- No B3 scoped acceptance gap remains for constructor injection or the focused revocation path.
- Broader T08/T14 evidence remains pending: live browser boundary, selected sandbox no-egress, joined authenticated production route, and ticket revocation before redemption/membership removal between stages.
- `internal/container/craft_runtime.go` and its concurrent R3 changes compiled successfully in this integrated checkpoint; no compile failure or precise source blocker remains. Their behavior was not independently validated by these focused tests.
- Only this report and raw test logs were written; no business source or test source was modified.
