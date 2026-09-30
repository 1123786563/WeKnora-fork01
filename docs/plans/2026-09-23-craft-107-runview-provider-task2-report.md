# T01 RunView Provider Task 2 — Deterministic container provider report

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- Task: Task 2 from `docs/plans/2026-09-23-craft-107-runview-provider-plan.md`
- HEAD before and after: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)
- Role: `backend_implementer` (assigned). Runtime model and reasoning effort are not exposed in this context.
- New owned source files:
  - `internal/container/craft_runview_container_provider.go` — SHA-256 `df104e82c6890dcd5082fd497f64cd6eebc4ffef9d9c5a10124a301d5115d156`
  - `internal/container/craft_runview_container_provider_test.go` — SHA-256 `8615aa037ea77de8480132e5ea87f24c016e0ec3703215c1c4c6942947a98771`
- Full-content patch: `docs/plans/2026-09-23-craft-107-runview-provider-task2-checkpoint.patch`
- Patch SHA-256: `0f9354a8b7e5746fbe96d6903542d120cd8928ce6dd30b57b3fe6bc96708bb6d`
- Checkpoint manifest: `docs/plans/2026-09-23-craft-107-runview-provider-task2-checkpoint.json`

No other source files were changed by this task. The worktree contains many unrelated shared changes and untracked files; they were preserved. Both owned source files remain untracked (`??`).

## Implementation

- Added an injectable `CraftRunViewContainerEngine` seam for private-network ensure, exact-name inspection, create, start, and runtime probing. There is no concrete production engine adapter and no `container.go`/`craft_runtime.go` wiring in this checkpoint.
- Provider construction rejects missing or malformed image, OpenCode binary, and runtime-config pins; requires the configured image reference to end in the exact `@sha256:<digest>`; and requires pinned OpenCode version `1.18.4`.
- The provider accepts only generation-derived runtime/container/directory values. It ensures an opaque host root and deterministic per-generation input, knowledge, output, and private OpenCode HOME paths. It checks real directories, exact modes, and symlink resolution before each use.
- First create requires definitive absence plus a private, fsynced one-shot create marker. If create outcome is uncertain, the marker prevents blind replacement. Recovery inspects and restarts the same deterministic container only; it validates the marker, engine ID, labels, image digest, user, command/environment, read-only root, dropped capabilities, resource limits, mounts, and runtime probe before returning identity flags.
- Container evidence includes the complete inspected network attachment set; exactly one internal/private generation network is required. Published ports, host namespaces/devices, an extra shared network, unexpected environment entries (including credentials), host/home/socket mounts, writable input/knowledge mounts, and shared HOME mounts fail closed.
- Session inventory uses the reviewed typed OpenCode inventory bound to the exact in-container directory and configured project. It marks the result authoritative only after full inventory returns and each session ID/project/directory matches. CreateSession requires exact directory and a verified current binding, sends one legacy client create call, and treats errors or malformed IDs as unresolved.
- Nothing is assembled or enabled by this task. The input/knowledge directories are isolated mount targets, but this provider does not stage selected input or accepted knowledge contents; upstream staging/publication and executor integration remain separate gates.

## RED → GREEN and checks

RED before implementation was observed with:

```text
go test ./internal/container -run '^TestCraftRunViewContainerProvider' -count=1
FAIL: undefined provider/config, fake engine, and session API contract symbols
```

After implementation:

```text
gofmt -w internal/container/craft_runview_container_provider.go internal/container/craft_runview_container_provider_test.go
PASS (no formatting changes reported by `gofmt -d`)

go test ./internal/container -run '^TestCraftRunViewContainerProvider' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/container 2.930s

go test -race ./internal/container -run '^TestCraftRunViewContainerProvider' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/container 8.438s

git diff --check
PASS (exit 0; no output)

git diff --no-index --check /dev/null internal/container/craft_runview_container_provider.go
git diff --no-index --check /dev/null internal/container/craft_runview_container_provider_test.go
PASS: both produced no whitespace diagnostics (exit 1 denotes the expected untracked-file diff)
```

The focused tests cover deterministic create and inspected identity, secure request/mount/environment constraints, same-container restart with persistent private HOME, foreign same-name/mismatched generation and image/runtime labels, mount/user/network/command/probe mismatch, extra shared network, missing-after-create and uncertain-create recovery, changed generation, inspect uncertainty, concurrent single create, complete scoped inventory versus incomplete inventory, exact CreateSession directory, and one-call failure behavior. A fake engine is unit-contract evidence only.

## Local binary and production limits

The pinned Darwin binary exists at `/Users/wuyongjun/.opencode/bin/opencode`; its reported version is `1.18.4` and its SHA-256 is `9449af91f517eacc2b0742fa93ae0da64fa6e5db7b714e30c62edea2a8de3f98`, matching `docker/craft/opencode.lock.json`. This can support a bounded **host-only** localhost smoke test for legacy `POST /session` followed by v2 list/GET inventory. That evidence would not establish the target Linux image's binary/digest, container engine inspection, network isolation, mount behavior, restart behavior, or deployment compatibility. The global `/opt/homebrew/bin/opencode` is a different `1.18.7` binary and is not the pinned candidate.

The checked-in lock still has `container_digest: null`; no target Linux image or live container-engine proof was supplied. Therefore this report does not claim live route compatibility, per-Run isolation verification, production readiness, production wiring, or Ticket completion. Keep production default-off. The independent route gate must still demonstrate that the pinned Linux image supports legacy create and v2 complete inventory together; the container-engine adapter must expose complete network/mount facts and the deployment must prove the approved model-gateway-only topology. R1 is a separate gate and is not claimed verified here.
