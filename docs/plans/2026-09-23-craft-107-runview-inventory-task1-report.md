# T01 RunView Provider Task 1 — OpenCode Inventory Report

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- Task: Task 1 from `docs/plans/2026-09-23-craft-107-runview-provider-plan.md`
- Starting/current HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)
- Agent role: `backend_implementer` (assigned); runtime model and reasoning effort are not exposed in this context.
- Source manifest: `docs/plans/2026-09-23-craft-107-runview-inventory-task1-checkpoint.json`
- Source manifest SHA-256: `32b917894f3f396dd6210cbaae3404926820ab117ac3ab029c83e98752866767`
- Full-content patch: `docs/plans/2026-09-23-craft-107-runview-inventory-task1-checkpoint.patch`
- Patch SHA-256: `21764fd4bb0dbef64f230f9dabdcfeb1ae5807a67d5df659a86c6d6367ac3cae`

Only these two owned source files were added:

```text
internal/modules/agentruntime/agent/opencode/inventory.go
internal/modules/agentruntime/agent/opencode/inventory_test.go
```

`internal/modules/agentruntime/agent/opencode/client.go` was a shared pre-existing modification and was not edited by this task. Its test-time SHA-256 is recorded in the manifest because inventory reuses its `Client`, `WithDirectory`, bounded JSON request, and same-origin/directory-preserving redirect policy.

## Implementation

- Added a typed OpenCode 1.18.4 `Session.Info` projection and `Inventory` bound to one server-selected canonical directory and expected project. A conflicting pre-bound directory is rejected.
- `ListSessions` requests `GET /api/session` with the supported project filter and bounded page size, while the existing client's `x-opencode-directory` location binding scopes the request. It follows opaque next cursors and returns results only after every page succeeds.
- Inventory fails closed on missing/null data, oversized pages, a full page with no continuation cursor (ambiguous truncation), empty page with a next cursor, looping cursors, excessive page/row/cursor limits, duplicate or malformed session IDs, and any session whose directory/project differs from the exact scope.
- `GetSession` requests `GET /api/session/:sessionID` with the bound directory and validates returned ID, project, and directory. It does not send an undocumented project query on the GET-by-ID route.
- The ID validator follows the pinned 1.18.4 fixture shape: `ses_`, 12 lowercase hex timestamp/counter characters, then 14 base62 characters.
- No changes were made to `client.go`, central wiring, or other shared files.

## RED → GREEN and checks

RED before implementation:

```text
go test ./internal/modules/agentruntime/agent/opencode -run '^TestInventory' -count=1
FAIL: undefined Inventory, NewInventory, SessionInfo, SessionLocation, and SessionTime
```

After implementation and final test refinements:

```text
go test ./internal/modules/agentruntime/agent/opencode -run '^TestInventory' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode 1.254s

go test ./internal/modules/agentruntime/agent/opencode -count=1
PASS: ok github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode 24.840s

git diff --check
PASS (exit 0)
```

The focused tests cover multiple-page success with exact directory/project binding, looping cursor, ambiguous full-page truncation, duplicate/malformed IDs, foreign and missing metadata, exact GET-by-ID, mismatched GET metadata, malformed scope/lookup rejected before HTTP, list and GET route 404, cross-origin redirect denial, and same-origin redirect with directory binding preserved. The full package suite includes the pre-existing protocol/client tests.

## Protocol compatibility gate and limits

The research at `docs/plans/2026-09-23-craft-107-opencode-inventory-research.md` verifies against pinned OpenCode source commit `49c69c5ed3ccf706b61b3febb43c8aaff7f8325e` that inventory is `GET /api/session` and exact metadata is `GET /api/session/:sessionID`. The checked-in runtime lock describes legacy `POST /session` for creation and does not establish that the actual deployed binary exposes the v2 inventory routes. This implementation intentionally has no fallback to a partial/legacy response: a missing route or 404 fails closed.

No live image/binary route compatibility proof was available in this task. Therefore this is protocol-client implementation and unit evidence only; it does **not** claim live route compatibility, production provider readiness, RunView isolation, or Ticket verification. The production enablement gate still needs an image digest and a live pinned-image test proving the list/GET routes and legacy-create/v2-inventory behavior together.
