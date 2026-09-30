# T08 B3 Task 1 report: production TaskAccessChecker injection

## Result

Implemented the production preview constructor binding to the same persistent `*service.CraftAccessService` provided by the container. `CraftPreviewConfig.AccessChecker` receives that instance. The constructor leaves `BrowserNavigationProtected` false and does not install a production network checker. Added focused tests for persistent T08 access behavior, the production disabled gate, and revocation of a previously issued preview capability.

No commits were made. HEAD stayed `a5e9195acd6500c085c85d60c852148e7bbbbf34`.

## Changed files

- `internal/container/container.go`: `newCraftPreviewService` takes the current `*service.CraftAccessService` provider and passes it as `AccessChecker`.
- `internal/container/craft_preview_access_wiring_test.go`: uses the migrated persistent membership/grant store and asserts owner/Viewer allow `TaskPreview`, ungranted tenant Admin and cross-tenant actor are denied, revoked Viewer and stale membership incarnation are denied, and the production constructor leaves browser protection disabled.
- `internal/handler/session/craft_preview_persistent_access_test.go`: with a real persistent checker and test-only preview gates, issues and redeems a Viewer capability, confirms preview bytes before revoke, then confirms owner revocation yields 404 with zero response bytes.

## TDD and validation evidence

RED: added the container test before the constructor change. `go test ./internal/container -run '^TestCraftPreviewConstructorUsesCurrentPersistentTaskAccessAndStaysDisabled$' -count=1` failed at the expected constructor call because the checker dependency was missing. It also exposed existing shared-worktree compile errors in `internal/container/craft_runtime.go` and `craft_runtime_test.go` (three-argument `stageWorkspaceInputs` call sites against a two-argument method, plus missing `sort`).

GREEN evidence:

- `go test ./internal/application/service -run '^TestCraftT08Journey$' -count=1` — PASS.
- `go test ./internal/handler/session -run '^TestCraftPreview(CapabilityStopsServingAfterPersistentGrantRevocation|T14Journey)$' -count=1` — PASS. This includes the new persistent grant/capability/revoke test and existing preview journey test.
- `git diff --check -- internal/container/container.go internal/container/craft_preview_access_wiring_test.go internal/handler/session/craft_preview_persistent_access_test.go` — PASS.

Container-focused test rerun after the wiring change remains BLOCKED by the same unrelated shared-worktree container package compile errors. The new test reaches type checking with the updated four-argument constructor call; package compilation stops first at `craft_runtime.go:190,399` and stale test call sites in `craft_runtime_test.go`. Those files are outside this task's ownership and were not changed.

## Assumptions and remaining risks

- `*service.CraftAccessService` is the intended DI binding because the container already provides this persistent service and its `craftTaskAccessChecker` projection from that same instance. It satisfies `craft.TaskAccessChecker` at compile time.
- Production preview remains fail-closed: valid origins can make `Enabled()` true, but issuance still returns `craft.ErrUnsupported` while browser navigation protection is false. The test-only service fixture uses a no-op network checker only to prove immediate revoke behavior; no fake gate is wired in production.
- The container wiring test cannot be executed until the concurrent `craft_runtime.go` mismatch is reconciled. No evidence is claimed for the container package as a whole.

## Checkpoint

Exact pre/post file contents, hashes, pre-existing unstaged container diff, task-local patch, and staged-diff record are in `docs/plans/2026-09-24-craft-107-t08-b3-preview-wiring-task1-checkpoint.json` and its sibling checkpoint directory. Task-local patch SHA-256: `6ec5a06367e38fa864720ba8bba7601aa3fa6842cfd46ec246a600d015ea8b69`.
