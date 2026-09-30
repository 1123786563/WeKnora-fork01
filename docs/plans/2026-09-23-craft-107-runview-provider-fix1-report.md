# T01 RunView provider Task 2 — review fix 1 report

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- Plan: `docs/plans/2026-09-23-craft-107-runview-provider-fix1-plan.md`
- Review: `docs/plans/2026-09-23-craft-107-runview-provider-task2-review.md`
- HEAD before and after: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)
- Role: `backend_implementer` (assigned); model and reasoning effort are not exposed in this context.
- Pre-fix source hashes from the reviewed Task2 manifest: provider `df104e82c6890dcd5082fd497f64cd6eebc4ffef9d9c5a10124a301d5115d156`; test `8615aa037ea77de8480132e5ea87f24c016e0ec3703215c1c4c6942947a98771`.
- Current source hashes:
  - `internal/container/craft_runview_container_provider.go`: `2872bb3360bd4f85c52185ae8ea24419089a4b8f4369fedac7ff88fa1d98ae6d`
  - `internal/container/craft_runview_container_provider_test.go`: `edc72c3dda199405148a198885cbfc6cad091559c2b086f69b5d4e15d12497ae`
- Full-content checkpoint: `docs/plans/2026-09-23-craft-107-runview-provider-fix1-checkpoint.patch` (SHA-256 `6cdc87240105c86e3f401011d41dd788d98850155c8c029e8fd1484ba4c27e3e`)
- Checkpoint manifest: `docs/plans/2026-09-23-craft-107-runview-provider-fix1-checkpoint.json`

Only the two plan-owned Go source files were edited. They remain untracked at the unchanged integration HEAD. Other worktree changes were preserved.

## Findings addressed

1. **Canonical sandbox path policy:** `prepareSandboxRoot` now resolves the longest existing path prefix without creating missing segments, appends the remaining path, and checks the canonical candidate against forbidden host roots before `MkdirAll` or `Chmod`. It checks the resulting `EvalSymlinks` path again and rejects a path that changed through a symlink during creation. The new test points an allowed-looking symlink at a disposable directory under the current user's forbidden home root, expects constructor rejection, and verifies the target mode was not changed.
2. **Meaningful inspected-state mismatch tests:** image, labels, mounts, user, network, command and runtime-probe mismatches now begin by creating a valid generation through the provider. Each test confirms the create marker exists, mutates the engine's inspected state, then calls inspect again and asserts the specific inspection/probe failure stage without a second create. The tests therefore reach the intended post-marker validation path.
3. **Exact GET-by-ID session recheck:** the private session API seam now exposes `GetSession`; the production adapter delegates to Task1 `Inventory.GetSession`. After complete list validation, `FindSessions` exact-GETs a single candidate and independently requires returned ID, project ID and directory to equal the list candidate and the inspected container. GET failure or any disagreement returns an empty non-authoritative/incomplete inventory. Zero or multiple list rows do not trigger GET.

The host-only protocol smoke in `docs/plans/2026-09-23-craft-107-t01-host-protocol-smoke.md` confirms the pinned Darwin binary supports `/api/session` GET-by-ID. It also demonstrates why returned metadata must be checked: the host server returned a record despite a deliberately mismatched directory query. The provider checks the returned metadata itself and does not treat query parameters as proof.

## RED → GREEN and checks

RED was observed before provider implementation changes:

```text
go test ./internal/container -run 'TestCraftRunViewContainerProvider(RejectsSandboxRootSymlinkIntoForbiddenHostPath|RejectsForeignAndMisconfiguredContainers|FindSessionsRequiresCompleteScopedInventory|RequiresExactGetForUniqueListedSession)$' -count=1
FAIL as expected: the symlink constructor returned nil error; unique inventory made 0 exact GETs; GET failure and changed ID/project/directory were incorrectly accepted. The marker-backed mismatch cases exercised the intended path and passed their rejection assertions.
```

After implementation on the final source/test state:

```text
gofmt -d internal/container/craft_runview_container_provider.go internal/container/craft_runview_container_provider_test.go
PASS: no output

go test ./internal/container -run '^TestCraftRunViewContainerProvider' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/container 2.387s

go test -race ./internal/container -run '^TestCraftRunViewContainerProvider' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/container 3.540s

git diff --check
PASS: exit 0, no output

git diff --no-index --check /dev/null internal/container/craft_runview_container_provider.go
git diff --no-index --check /dev/null internal/container/craft_runview_container_provider_test.go
PASS: neither produced whitespace diagnostics (exit 1 denotes the expected untracked-file diff)
```

The linker emitted its existing warning `ignoring duplicate libraries: '-lc++'`; tests passed.

## Limits

This fix does not add a concrete engine adapter, production assembly, input/knowledge staging, Linux image digest, or live container-engine evidence. The lock still has `container_digest: null`; production remains default-off. The Darwin smoke is supplemental host-protocol evidence only and does not establish Linux image compatibility, mount/network isolation, restart behavior, or Run A→B exclusion. Independent review is required before considering the Task2 checkpoint accepted.
