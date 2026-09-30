# T01 Guarded Input Route Mount Plan

> **For Codex:** Execute with Superpowers SDD, RED → GREEN → REFACTOR, exact checkpoint, independent Review and backend validation. Do not commit.

**Goal:** Mount T01/#120's input-round and decision routes in the actual authenticated Craft session router before router construction. This is vertical support for T01; admission recovery, per-Run sandbox and product decision UI remain separate gates.

**Sources:** Approved Spec #107/#120; T01 handler `internal/handler/session/craft_input_round.go`; T00 constrained per-container feature registry; reviewed T08 central-A `craft_access_wiring.go` pattern and production route tests. Integration Worktree has interim T01 source checkpoint `eecd663d...`; wait for T01 fix3 scoped review/integration before running final vertical validation.

**Global Constraints:** backend_implementer owns only a new `internal/container/craft_input_wiring.go` and its tests, the narrow provider invocation line in `internal/container/container.go`, and a focused authenticated route test in `internal/router` or `internal/handler/session`. No edits to T01 service/handler, T08 access service, shared router source, migrations, frontend, commits or subagents. Other agents share integration Worktree; preserve their changes. If route setup requires unlisted file ownership, ask controller for a narrow amendment before editing.

**Review Focus:** handler receives the actual assembled `*CraftSessionService`, registers before one container/router assembly freezes feature routes, inherits JWT/API-key session guards and Craft namespace path constraints, fails closed on duplicate/missing provider, and does not leak across a second container assembly. An anonymous or cross-scope caller cannot reach decision/upload logic. Route and method table matches T01 handler exactly.

## Task 1 — production registration and HTTP journey

**Depends on:** T00 reviewed route registry and T01 handler checkpoint; T01 fix3 may update service but not handler signature. **Produces:** guarded live route mount.

1. RED: Add an integration test that builds the actual container/router and asserts `POST /sessions/:session_id/craft/input-rounds` and `/inputs/decision` are mounted only under authenticated API-key policy; exercise valid owner round/decision and anonymous/cross-tenant denial. It should fail before registration.
2. GREEN: Add `wireCraftInputFeature`/`registerCraftInputFeature` following per-instance access pattern, using `session.RegisterCraftInputRoutes(group, session.NewCraftInputHandler(svc))`. Invoke before router creation in `BuildContainer`, propagate registration errors.
3. REFACTOR: Run focused container/router/handler T01 tests, `git diff --check`, record actual HEAD/staged/unstaged/untracked and complete changed-file checkpoint. Do not claim T01 verified without R3 and RunView isolation.

**Failure handling:** If existing production handler route conflicts with this table, report exact route/method collision; do not bypass feature registry or loosen its validator.
