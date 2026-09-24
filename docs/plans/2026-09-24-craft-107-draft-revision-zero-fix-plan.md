# Craft #107 T01 R4 revision-zero provenance Fix Plan

> **For Codex:** SDD RED → GREEN → REFACTOR, exact uncommitted checkpoint, disposable PostgreSQL and SQLite migration/behavior proof, independent Spec/quality review. R4 runtime Task1 remains gated.

**Goal:** Close the Medium finding in `2026-09-24-craft-107-draft-revision-read-seam-review.md`: after D1/D2 advances, `ReadRevision(...,0)` currently invents an empty predecessor from the mutable current head without a persisted immutable origin. Also strengthen historical corruption/authorization tests.

**Sources:** Approved Craft Spec current Workspace continuation; S1 head/revision migration and review; draft `ReadRevision` prerequisite plan/review.

## Global constraints

- Integration Worktree, no commit/push. Existing S1 head/revision rows and D1/D2 must retain their meaning. The origin marker is server-created for each Workspace only, immutable thereafter, tenant/workspace scoped and independent of current head advancement.
- Use the next free paired migration IDs after current tips (verify first; likely PG000198 and SQLite000119). `migrations/` is Git-ignored: save full bytes/hashes, and use `git add -N -f` only for diff visibility. Do not silently rewrite earlier migration files.
- Backfill preexisting workspaces/head rows only when provenance can be established from S1 invariant/migration history; mark/deny ambiguous cases instead of fabricating origin.

## Task 1 — Persist and require empty origin

**Role:** backend_implementer. **Owned files:** paired new PG/SQLite up/down migrations, `internal/application/repository/craft_draft_head.go`, focused draft-revision repository tests; narrow `internal/application/repository/craft_workspace.go` Workspace-creation transaction insertion; `internal/modules/craft/draft_head.go` only if the port shape must change. The Workspace creation file was added after the implementer identified the atomic insertion point; no concurrent owner has it. No runtime/container.go/T19/H2 files. **Consumes:** initial Workspace/head creation and S1 revision history. **Produces:** immutable revision-zero origin marker and `ReadRevision(0)` that requires it.

1. RED SQLite/PG17: D1 then D2, revision-zero remains empty only with origin marker; delete/corrupt marker and assert fail closed. Existing workspaces migrated through up/down/up retain a valid marker when their S1 empty lineage is known. Missing historical positive revision/file or changed D1 source after D2 must fail, as must owner transfer at the authorization snapshot. Generic Workspace creation must atomically create the marker with initial head; no window where admitted Run can observe head without origin.
2. GREEN: introduce one immutable origin record or equally strong persisted invariant keyed by tenant/workspace. Create it in the same transaction that creates the initial empty head, backfill controlled legacy S1 heads based on known invariant, and protect against mutation/deletion except authorized Workspace teardown. `ReadRevision(0)` must join the marker with Workspace authorization and current head in one statement; require exact tenant/workspace and immutable zero identity. Keep positive revision read by requested number and validate historical manifest/files.
3. Run paired migration up/down/up, focused SQLite/PG17 tests, race, gofmt/diff check. Record disposable DB versions/cleanup, task-local pre/post hashes including ignored SQL bytes. Independent review checks zero provenance, backfill safety, historical tampering and no same-key replay regression.

**Acceptance:** both engines read a genuinely persisted D0 after D2 and reject missing/corrupt lineage; positive D1 remains immutable; independent review PASS releases R4 runtime Task1. **Failure:** if safe backfill cannot be proven, leave legacy ambiguous workspaces unseedable and report migration policy.

## Review focus

Origin row creation/backfill in same authorization transaction, immutability despite head updates, scope consistency, no false empty fallback, and exact cross-engine behavior.
