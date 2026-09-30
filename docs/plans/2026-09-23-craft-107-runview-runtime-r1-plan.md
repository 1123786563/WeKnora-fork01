# RunView Runtime R1 — Recoverable Allocation Plan

> **For Codex:** Execute with SDD RED → GREEN → REFACTOR, exact checkpoint, independent Review. No commit.

**Goal:** Implement the server-owned RunView runtime allocation coordinator in a new container module so retries use the persisted generation and never create a second OpenCode session after an uncertain CreateSession result. This is R1 of `2026-09-23-craft-107-runview-runtime-adapter-plan.md`, not deployment or T01 isolation acceptance.

**Sources:** Approved Spec #107/#120/#124, RunView binding contract/store and reviews, per-Run isolation design in T01 Worktree, pinned OpenCode v1.18.4 directory protocol review. Integration Worktree HEAD `a5e9195...` plus active T08/T05/T19 edits.

**Global Constraints:** backend_implementer owns only new `internal/container/craft_runview_runtime.go` and focused `craft_runview_runtime_test.go`, plus report. Do not edit `container.go`, `craft_runtime.go`, RunView store/schema, OpenCode client/executor, T05 Publisher or other shared files without controller amendment. Other agents share the integration Worktree; preserve their changes. No commit/subagents. Use narrow interfaces for real runtime provider, but do not create a production fake that could be wired as isolation evidence.

**Review Focus:** key only from admitted Run; `Allocate` stable generation; deterministic generation-scoped runtime/container identity and private root; same-generation retries only observe or resume; ambiguous create produces unresolved state and no second CreateSession; `BindRuntime` only after session-directory affinity is verified by provider; foreign scope/generation fail closed; no model prompt while allocating. Existing `RunViewStore` has no partial runtime ID, so if provider cannot deterministically recover by generation, fail closed and report additive schema/interface need rather than inventing a new container.

## Task 1 — coordinator and recoverable provider contract

1. RED: fake provider with create success/lost response, restart, zero/one/multiple matching sessions, wrong directory, foreign generation and concurrent callers. Assert one generation/container and at most one session creation per Run; ambiguous outcome cannot create another or return a runnable handle. Verify client directory comes only from canonical server root, not request/model input.
2. GREEN: add a typed `RunViewRuntimeProvider` for generation-scoped `InspectOrCreateContainer`, `FindSessions`, and `CreateSession` with identity/affinity evidence; coordinator calls `Store.Allocate`, validates canonical dedicated root, resolves deterministic provider identity, and binds exact runtime triple via CAS only when uniqueness/affinity are proved. Provider uncertainty leaves RunView `allocating` and returns a typed unresolved error; it must not retry a create blindly. A bound row is reloaded and verified against provider identity before returning a handle.
3. REFACTOR: focused tests, `go test ./internal/container -run CraftRunViewRuntime -count=1` if shared container package compiles; otherwise explicit-file tests plus report of concurrent RED. Diff check, full-content hashes/status/checkpoint. No production assembly yet, no OS isolation claim.
