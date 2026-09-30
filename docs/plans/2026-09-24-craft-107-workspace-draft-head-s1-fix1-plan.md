# S1 Task1 fix1: scoped consistent Read and SQL identity

> **For Codex:** Execute the three independent review findings with RED→GREEN→REFACTOR, exact uncommitted checkpoint, independent re-review. No commit.

**Sources:** `2026-09-24-craft-107-workspace-draft-head-s1-plan.md`, Task1 report/checkpoint and `2026-09-24-craft-107-workspace-draft-head-s1-task1-review.md`; approved Spec #107. S2 remains unassigned until PASS. Capture live HEAD and full owned-file preimage/diff/staged/unstaged first.

## Global Constraints

`Read` must authorize tenant, Workspace, Session and owner from a consistent snapshot with the returned revision/files; it must never return refs after owner transfer or mix a changed head with older files. Legacy existing Workspace without head has an independent unresolved error, never `ErrNotFound`. SQL must bind tenant Workspace and source Run where schema supports it, without changing unrelated table semantics or deleting identity on parent cascade. Go transactional checks remain required. Four ignored migration SQL files must be present in exact checkpoint; PostgreSQL proof must be attempted with available disposable PG if possible.

## Review Focus

Concurrent owner transfer/read and head change/read; error sentinel classification; PG/SQLite FK parity and retention; migration up/down/up; exact task-local patch and no S2 `Advance` implementation.

## Task 1 — close High and two Medium findings

**Depends on:** S1 Task1 failed independent review. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/modules/craft/draft_head.go/_test.go`, `internal/application/repository/craft_draft_head.go/_test.go`, four PG000195/SQLite000116 draft-head SQL files, and `craft_workspace.go/_test.go` only if needed for deterministic owner-race test. Other files read-only. **Consumes:** current Workspace and head rows; **Produces:** safe S1 Task1 Read contract.

1. RED tests: concurrent owner transfer during `Read` cannot leak selected object refs or mix revision/files; selected read after head advance is internally consistent; `ErrDraftHeadUnresolved` does not match `ErrNotFound`; SQL insert of wrong-tenant head or foreign/missing source Run rejected in both dialects. Use controlled test hooks/transactions, avoid timing-only races.
2. Implement scoped consistent read via transaction isolation/locking or one scoped joined query that binds owner/tenant/session at the data read; validate returned revision/digest/files together. Make unresolved independent. Add composite tenant FKs and supporting unique keys only after checking migrations and deletion semantics. If a FK is incompatible, document exact schema reason and add equivalent transactional integrity enforcement plus tests; no silent omission.
3. Run focused Craft/repository tests, SQLite migration up/down/up, disposable PG DDL/runtime if available, `gofmt`, `git diff --check`. Save exact pre/post contents/hashes, task-local patch including ignored SQL, report. Independent reviewer rechecks all three findings.

**Acceptance:** no authorization race, no unresolved-as-not-found fallback, and tenant/source identity enforced at stored-row boundary. **Failure handling:** if consistent read or referential constraints cannot be established, keep S1 and R4 fail closed and report precise blocker.
