# T01 RunView OpenCode Inventory Completeness Fix Report

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- Plan: `docs/plans/2026-09-23-craft-107-runview-inventory-fix-plan.md`
- Review findings: INV-1 and INV-2 from `docs/plans/2026-09-23-craft-107-runview-inventory-review.md`
- Starting/current HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)
- Agent role: `backend_implementer` (assigned); runtime model/reasoning effort are not exposed.
- Manifest: `docs/plans/2026-09-23-craft-107-runview-inventory-fix-checkpoint.json`
- Manifest SHA-256: `fe18ed194b4163ca6683dba022774a1f9979ac7818712b611688c88bc4cecd0a`
- Full-content checkpoint patch: `docs/plans/2026-09-23-craft-107-runview-inventory-fix-checkpoint.patch`
- Patch SHA-256: `cd3a69166731d52db1e1286b67e1ecc996bcaf602006170e7ae0902d88aa061d`

Only `internal/modules/agentruntime/agent/opencode/inventory.go` and `inventory_test.go` are owned source files. Their full-content hashes and byte lengths are in the manifest. `client.go` remains an unmodified shared dependency; its test-time SHA-256 is recorded in the manifest.

## Fixes

- **INV-1:** every successful list page must include a non-null `cursor` envelope. An empty/missing `next` is accepted as final only within that envelope. Tests cover missing and null envelopes on the first and later pages, plus a valid final `cursor:{}` response.
- **INV-2:** inventory wraps a copy of the existing directory-bound client. It first invokes the existing same-origin/directory redirect checks, then compares `project`, `limit`, and `cursor` values between each redirect source and target. Missing, duplicate, malformed, or changed values fail before following the redirect. Exact-preserving same-origin redirects remain supported, including a later page with its cursor intact.
- No shared `client.go` code was changed or weakened.

## RED → GREEN and verification

Before implementation, the new focused cases failed against the prior code:

```text
go test ./internal/modules/agentruntime/agent/opencode -run '^TestInventory(ListSessionsRequiresCursorEnvelopeOnEveryPage|RedirectRejectsChangedProjectLimitOrCursor|LaterPageRedirectCannotDropOrChangeCursor|FollowsSameOriginRedirectWithoutLosingDirectory)$' -count=1
FAIL as expected: missing/null cursor envelopes and same-origin changes to project, limit, or later-page cursor returned successful inventories.
```

After the fix:

```text
go test ./internal/modules/agentruntime/agent/opencode -run '^TestInventory' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode 1.744s

go test ./internal/modules/agentruntime/agent/opencode -count=1
PASS: ok github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode 17.627s

git diff --check
PASS (exit 0)
```

The complete package test includes existing OpenCode client/protocol tests. The new tests verify first/later missing and null cursor envelopes, valid final envelope, first-page project/limit changes and omissions, later-page cursor changes and omissions, exact-preserving same-origin later-page redirect, and retained directory binding.

## Protocol compatibility remains a separate gate

Pinned source research describes inventory as `GET /api/session` and metadata lookup as `GET /api/session/:sessionID`; the checked-in runtime lock still documents legacy `POST /session` for create and does not prove the deployed binary exposes those v2 reads. A 404 remains an error. No live pinned-image test was run, so this fix establishes unit-level fail-closed pagination/redirect behavior only. It does not claim live route compatibility, production provider readiness, per-Run isolation, or overall T01 completion.
