# Craft Workspace Draft Head S1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan authorizes only S1; do not dispatch S2/R4 or enable RunView.

**Goal:** Persist a scoped, immutable, revisioned current-Workspace draft head that can be safely read after restart and advanced only for an authorized, confirmed quiescent source Run.

**Architecture:** `craft.DraftHeadStore` is the small module interface. A SQL adapter creates an explicit empty head atomically with new Workspace creation, stores each revision's immutable manifest, and atomically changes the current revision by CAS. The caller captures and verifies RunView output before calling `Advance`; SQL independently checks owner/Workspace/Run identity and durable terminal state. An absent head for an old Workspace is unresolved, never empty.

**Tech Stack:** Go, GORM, PostgreSQL/SQLite migrations, `craft` domain validation, repository tests.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md` lines 57–66; corrected `docs/plans/2026-09-24-craft-107-runview-base-version-seam-plan.md`; `CONTEXT.md`; ADR-0004/0008/0009. T01 #120 supplies the Workspace/Run admission context; T15 #130 is downstream for default-preview promotion, not this task's dependency.

## Global Constraints

- The current Workspace draft is the continuation source; the last successful Version is the default-preview source. Do not substitute `VersionStore.List()[0]` or client `BaseVersionID`.
- Store only immutable object refs and canonical relative `output/` paths, digests and byte counts; no host path, Run directory, HOME, OpenCode state, inputs or knowledge.
- Do not claim quiescence from a `failed` status alone. Caller needs a verified stopped/idle runtime capture; the repository must also verify durable source Run identity and terminal confirmation. Unknown/active/pending states reject.
- Integration worktree has concurrent edits. Record live HEAD/status and exact owned-file staged, unstaged and untracked baseline; do not commit. Preserve all unrelated edits.
- S2 admission freezing and R4 materialization are separate tasks. Do not add runtime fallbacks or production enablement here.

## File ownership and interfaces

**Create:** `internal/modules/craft/draft_head.go` (types, validation, interface); `internal/application/repository/craft_draft_head.go` (SQL adapter); `internal/application/repository/craft_draft_head_test.go` (behavior and migration tests); `migrations/versioned/000195_craft_workspace_draft_head.up.sql` and `.down.sql`; `migrations/sqlite/000116_craft_workspace_draft_head.up.sql` and `.down.sql`.

**Modify:** `internal/application/repository/craft_workspace.go` only at `PutWorkspace(expected==0)` to create the initial empty head in its existing transaction. Add focused existing/new `craft_workspace_test.go` cases only where needed. Current latest migration files observed: PG `000194_craft_run_actor`, SQLite `000115_craft_run_actor`; recheck before writing and stop on number collision.

**Public seam:** `craft.DraftHead{WorkspaceID string; Revision int64; State DraftHeadState; SourceRunID string; ManifestDigest string; Files []craft.File}` and `craft.DraftHeadStore` with `Read(ctx context.Context, scope craft.Scope, workspaceID string) (craft.DraftHead,error)` and `Advance(ctx context.Context, scope craft.Scope, workspaceID string, expectedRevision int64, sourceRunID string, files []craft.File) (craft.DraftHead,error)`. `State` is `empty` or `selected`; `Read` returns a distinct unresolved error when an existing Workspace has no head. Do not expose a mutable `Files` slice. `Advance` never accepts `empty` or an arbitrary historical revision.

**Schema:** `craft_workspace_draft_heads` has `workspace_id` PK/FK to `craft_workspaces(id)`, tenant ID, current revision, state, source Run ID nullable for empty, manifest digest nullable for empty, timestamps, and checks enforcing empty/selected combinations. `craft_workspace_draft_revisions` has `(workspace_id,revision)` PK, source Run ID, manifest digest, created timestamp and FK to head; `craft_workspace_draft_files` has `(workspace_id,revision,path)` PK, object ref, SHA-256, bytes, MIME, FK to revision. Use composite tenant/Run referential checks where supported, and **always** recheck tenant/owner/session/source Run in the same transaction in Go; no FK alone is authorization. Selected revision rows and files are insert-only. Current head update is `WHERE workspace_id=? AND revision=?`; affected rows must be exactly one. Revisions begin at 0 for explicit empty, first selected is 1. If the existing SQL dialect/constraints prohibit a composite FK without a supporting unique key, use the established FK shape and transactional identity checks rather than adding broad indexes to unrelated tables.

**Capture contract:** `Advance` requires a complete sealed manifest supplied by a future RunView collector. Validate `craft.ManifestDigest`, canonical paths, uniqueness, SHA-256, nonnegative sizes, total limits and nonempty object refs. The repository cannot prove live filesystem quiescence, object bytes, or read-only capture. It rejects source Runs whose DB status is not a confirmed terminal state, whose tenant/owner/session differ, or whose active slot/unfinished delegation still suggests an unresolved writer. The caller must hold the Workspace lifecycle/writer fence through capture and `Advance`; S3 owns that integration and must not call `Advance` after losing the fence. Do not create an interface that suggests DB checks alone prove quiescence.

## Review Focus

1. Concurrent `Advance` from the same revision: exactly one wins and no losing manifest becomes current.
2. Failed Run with verified quiescent capture can advance draft while default-preview Version remains untouched; unknown/active/stop-requested cannot.
3. Cross-tenant, wrong owner/session, foreign source Run and a reused Run ID cannot change the head.
4. Duplicate/unsafe paths, missing refs, bad digest/size and partial inserts roll back all rows.
5. Legacy Workspace missing a head produces unresolved, whereas newly created Workspace produces explicit empty after restart.

## Task 1 — schema, domain contract and empty creation

**Depends on:** reviewed T01 Workspace creation contract; migration allocator still free at 000195/000116. **Owner:** `backend_implementer`; **validator:** `backend_validator`. **Consumes:** existing `craft.Workspace`, `craft.File`, `craft.ManifestDigest`, `CraftStore.PutWorkspace`. **Produces:** `DraftHeadStore.Read` and explicit empty revision 0 on new Workspace.

- [ ] Record task-local baseline for owned files; inspect the two migration sequences and existing Workspace tests.
- [ ] RED: add domain validation tests for empty versus selected invariants, canonical digest/paths and defensive copy; add repository tests that new `PutWorkspace(expected=0)` yields `Read(...).State=empty, Revision=0` after database reopen, concurrent creator creates one head, and legacy Workspace row without head returns unresolved. Add migration up/down/up test for both SQLite DDL and PG syntax/real PG when available.
- [ ] Run `go test ./internal/modules/craft ./internal/application/repository -run 'DraftHead|Workspace' -count=1`; record expected failures, not just exit code.
- [ ] Implement domain types/interface/errors and paired SQL migrations. Modify only `PutWorkspace(expected==0)` so Workspace row and empty head insert commit or roll back together. `Read` must first authorize Workspace by tenant/session/owner; absence of a head after Workspace exists is unresolved, not `empty`.
- [ ] Re-run focused tests, `gofmt -d` on Go files, `git diff --check`; record exact outputs. Save checkpoint/hash/patch and independent Spec plus quality review before Task 2.

**Failure handling:** migration collision or inability to atomically initialize empty head blocks Task 1; do not initialize it lazily on `Read`.

## Task 2 — immutable manifest revision and CAS

**Depends on:** Task 1 reviewed checkpoint integrated. **Owner:** `backend_implementer`; **validator:** `backend_validator`. **Owned files:** `craft_draft_head.go`, focused repository tests, migration files only for a review finding; domain validation only if Task 1 review requires it. **Consumes:** explicit head revision and sealed `[]craft.File`. **Produces:** `Advance` returning exact revision and immutable manifest.

- [ ] RED: tests for first selected revision, two sequential revisions, same-revision concurrent CAS, losing transaction leaves no visible revision/files, restart reads same manifest, selected rows cannot be changed by replay, unauthorized Workspace and foreign Run, active/waiting/unknown/stop-requested Run, and terminal Run with pending/unknown delegation. Test duplicate/traversal/credential paths, malformed hash, negative/over-limit sizes and empty refs. Confirm the stored digest equals `craft.ManifestDigest(files)` and tampered stored digest fails `Read`. A failed terminal Run test must use an explicit verified quiescent capture fixture supplied by the trusted caller, not status alone. Assert no `craft_versions` row or preview default moves.
- [ ] Run `go test ./internal/application/repository -run 'DraftHead' -count=1` and capture RED evidence.
- [ ] Implement `Advance` as one DB transaction: authorize Workspace; verify current head/revision and source Run's tenant/owner/session/confirmed terminal state and no unresolved writer; validate/canonicalize complete manifest; insert immutable revision and files; CAS current head; roll back on any mismatch. To avoid races, use the established DB locking/CAS pattern, and ensure PostgreSQL and SQLite both return exactly one winner. Return a defensive copy loaded from committed rows.
- [ ] Run focused tests and applicable race test, `go test ./internal/application/repository -run 'DraftHead' -race -count=1`, `gofmt -d`, `git diff --check`; save checkpoint/hash/patch and independent Spec plus quality review.

**Failure handling:** if status/lease/delegation data cannot prove a confirmed quiescent source, reject `Advance` and document the missing capture authority for S3. Never accept unknown as failed, infer a seed from Version order, or create a head from a partial capture.

## Handoff

S1 is verified only after paired migrations, focused tests, independent review and exact checkpoint integration. Report interface, schema names, migration numbers, command results, and any unresolved quiescence proof. S2 may use `Read` only after S1 integration; S3 must supply the verified RunView capture/fence before calling `Advance`. Keep production RunView disabled until S2/R4 and real Run A→B acceptance pass.
