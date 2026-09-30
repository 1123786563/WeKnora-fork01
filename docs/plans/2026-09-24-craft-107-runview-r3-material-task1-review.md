# T01 RunView R3 Task 1 — independent scoped review

**Scope:** `craft_runview_material.go`, the Task 1 delta in `craft_runview_container_provider_test.go`, and unchanged provider behavior used by the new method. Reviewed against the approved Craft #107 Spec, `CONTEXT.md`, ADR-0004/0008/0009, the R3/R4 design, Task 1 plan/report/checkpoint, and prior R1/provider reviews. No source, test, or remote issue was modified.

## Verdict

- **Scoped Spec compliance: FAIL pending one medium finding.** The method binds the admitted key to the persisted bound RunView, derives generation paths server-side, rechecks container/network/mount/probe facts and exact session metadata, and rejects stable symlinks and foreign Run handles. It can nevertheless return a newly recreated host child path after the running container retained the old bind-mounted directory.
- **Scoped code quality: FAIL pending the same finding.** The security-sensitive revalidation path calls a layout *preparation* routine that mutates missing directories. The focused tests pass but do not exercise this replacement case. Exact-session GET failures and metadata disagreement also lack direct Task 1 tests; this is a low-severity test gap.
- **Production isolation and end-to-end T01 acceptance: NOT VERIFIED.** The engine is fake-backed here. No claim is made for a Linux mount, pinned production image, assembly, or Run A→B acceptance.

## Findings

### M1 — Medium: bound material directory can be recreated behind an existing bind mount

**Evidence:** `MaterialHandle` calls `currentBinding` and later `prepareGenerationLayout` (`internal/container/craft_runview_material.go:69-92`). `verifyCurrent` also calls `prepareGenerationLayout` before inspecting mount path strings (`internal/container/craft_runview_container_provider.go:430-455`). `prepareGenerationLayout` invokes `secureDirectory` for `inputs`, `knowledge`, `output`, and HOME children; `secureDirectory` creates a missing directory (`:650-697`). `validateInspectedMounts` compares inspected source path strings, not the identity of the mounted directory (`:555-579`).

**Impact:** If a generation child is removed or renamed while its container is running, verification recreates that pathname and accepts the unchanged inspected mount source string. A writer receiving the new host path can stage material into a different directory inode than the container has mounted. Input/knowledge may be absent from the Run despite a successful material handle; output collection may similarly read a tree other than the live container's output. This violates the Task 1 requirement that the returned layout be the currently verified generation material boundary. A stable symlink is rejected, but this case does not require a symlink.

**Smallest defensible correction:** Separate initial layout creation from bound-layout verification. On a bound/running generation, fail closed if any expected child is missing instead of creating it, and pin or verify directory identity across container creation and material use (for example, record device/inode for the bound host directories and require the same identity during reinspection). Add a test that replaces a mounted child with a fresh directory at the same path and requires unresolved before a handle is returned.

### L1 — Low: exact-session GET branch lacks direct material-handle regression tests

**Evidence:** `MaterialHandle` checks GET error and exact ID/project/directory (`internal/container/craft_runview_material.go:73-76`), but the new material tests in `craft_runview_container_provider_test.go:716-850` only mutate stored/runtime session IDs. They do not make `GetSession` fail or return an altered ID, project, or directory after a valid persisted binding. Prior provider inventory tests cover a different `FindSessions` call path.

**Impact:** A future change could bypass this last session identity check while the Task 1 suite remains green.

**Smallest defensible correction:** Add table cases for GET error, missing session, and mismatched ID/project/directory in `TestCraftRunViewMaterialHandleRejectsForgedStaleAndUnsafeBindings`.

## Evidence and limits

- SHA-256 matches the checkpoint: provider `2872bb3360bd4f85c52185ae8ea24419089a4b8f4369fedac7ff88fa1d98ae6d` (unchanged), test `519da4d8c7a3f1eef02ed92cfb40a5c1c41fce0e0f9a669e851f52c9d8c59ebe`, new material file `676902fc87f2267249c0abc909e3019df1c9d5ee86ef2cd44148e7d514e6a8b9`, task-local patch `3781eb3eb055530cc150827f66428b49bff391331e527c8ebb69baf2828e4708`. The patch contains only the Task 1 provider-test delta and new material file; provider source is not in it. HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`.
- Independent `go test ./internal/container -run '^TestCraftRunView(ContainerProvider|MaterialHandle)' -count=1` and the same focused command with `-race -timeout=120s` passed. Both emitted only the linker duplicate `-lc++` warning.
- Independent `go test ./internal/container -count=1` failed at `TestCraftAccessFeatureRegistriesAreAssemblyOwned` (missing `craft.TaskAccessChecker`) and `TestWireCraftInteractionRegistrarRegistersPendingInteractions` (`agent runtime conflict`). Their failing symbols are in assembly tests and outside the Task 1 patch; the focused provider/material suite passed. This attribution does not turn the full package result into a pass.
