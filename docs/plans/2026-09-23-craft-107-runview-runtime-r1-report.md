# RunView Runtime R1 — Coordinator Checkpoint

**Status:** Coordinator contract and fake-backed recovery tests implemented. Runtime resolution remains fail-closed pending a real provider and live pinned OpenCode evidence; this is not production provisioning, dispatch, or OS-isolation proof.

**Assigned role:** `backend_implementer`. Runtime model and reasoning metadata were not exposed; none is asserted.

**Worktree / HEAD:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`, HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. No commit or staging was performed. Shared unrelated changes were preserved.

## Implementation checkpoint

The new coordinator in `internal/container/craft_runview_runtime.go`:

- Loads/allocates by admitted `RunViewKey`, validates persisted scope and uses the persisted generation.
- Derives stable runtime/container IDs and a private in-container root from a SHA-256 digest of the generation. No caller/model path contributes to the directory.
- Requires provider evidence for exact generation/runtime/container/directory identity, a dedicated container, and a canonical directory. Requires complete authoritative session inventory scoped to that exact directory and OpenCode project ID.
- Uses the durable one-shot `BeginSessionCreate` CAS before the first external create. Only its `maySend` winner can call `CreateSession`; later calls are observe-only. A lost response with zero inventory, incomplete inventory, multiple sessions, wrong project/directory, or conflicting bound identity returns `ErrCraftRunViewRuntimeUnresolved` and cannot produce a runnable handle.
- Binds only one session whose observed project and `location.directory` match the inspected container. A bound record is reloaded and compared with provider evidence before returning the internal handle. There is no prompt-dispatch method or production assembly wiring in this task.

The session metadata contract intentionally does **not** invent generation/runtime/container fields on OpenCode `Session.Info`: the pinned schema has no generation marker. The contract requires a provider to establish generation affinity through the inspected dedicated per-generation container, then preserve the session's observed project ID and directory. If the provider cannot establish those facts, it must not set inventory authoritative.

## Pinned OpenCode evidence and limits

See [`2026-09-23-craft-107-opencode-inventory-research.md`](2026-09-23-craft-107-opencode-inventory-research.md). At the pinned OpenCode 1.18.4 source commit, `GET /api/session` returns cursor-paginated `Session.Info` items and supports directory/project filtering; `GET /api/session/:sessionID` retrieves one session. `Session.Info` includes `id`, `projectID`, `location.directory`, and timestamps, but no RunView generation identifier. `POST /api/session` has an optional ID but no idempotency-key field. Therefore an uncertain create with zero visible results remains unresolved; this coordinator never retries it. Multiple same-scope sessions also remain ambiguous.

The checked-in `docker/craft/opencode.lock.json` documents legacy `POST /session`, not the v2 inventory/GET routes. Source research does not show that the deployed binary enables compatible routes, that pagination behavior works live, or that a list immediately reflects a just-created session. The current Go OpenCode client has no list/get-session API and this task did not add a production provider. There is no live container/browser/provider proof here. Until the pinned runtime is tested and an authoritative provider exists, the coordinator contract cannot be wired as evidence of a runnable runtime.

## TDD and verification

RED was observed before implementation: `go test ./internal/container -run CraftRunViewRuntime -count=1` failed to compile because the new coordinator types were undefined. The focused tests now cover normal bind, lost-response recovery after restart without resend, unknown create with zero inventory without retry, non-authoritative inventory, foreign project/directory, multiple sessions, wrong/unverified container identity, foreign store owner, invalid roots, bound-session revalidation, and concurrent resolution with at most one create.

Commands and results:

```text
go test internal/container/craft_runview_runtime.go internal/container/craft_runview_runtime_test.go -run CraftRunViewRuntime -count=1
ok   command-line-arguments  0.305s

go test -race internal/container/craft_runview_runtime.go internal/container/craft_runview_runtime_test.go -run CraftRunViewRuntime -count=1
ok   command-line-arguments  1.548s

go test ./internal/container -run CraftRunViewRuntime -count=1
ok   github.com/Tencent/WeKnora/internal/container  1.380s
```

The package run emitted only the linker warning `ignoring duplicate libraries: '-lc++'`. `git diff --check` and `gofmt` were also run successfully on the two owned Go files. These tests exercise coordinator behavior with fakes; they do not establish provider correctness, external OpenCode route availability, container confinement, browser behavior, or T01 process/OS isolation.

## Owned file hashes

SHA-256 at checkpoint:

```text
internal/container/craft_runview_runtime.go
c4d1196bb8e943f5ca5d9f7b8874d6df2b7df81caf1ff3d8f6e8ddbe7d524a34
internal/container/craft_runview_runtime_test.go
50df0fc18c074a8122dc66ee377e5928876204ac81c983c90c752986a9c9a8de
```

No external commits or live provider/browser runs were made by this task. Owned files are released for independent review; the remaining blocker is a tested provider against the pinned OpenCode binary that can prove complete directory/project inventory and dedicated generation-container affinity. A zero result after an uncertain `POST /api/session` remains permanently observe-only absent an explicit future idempotency/proof seam.
