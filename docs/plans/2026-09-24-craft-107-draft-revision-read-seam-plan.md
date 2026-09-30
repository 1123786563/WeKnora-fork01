# Craft #107 T01 R4 prerequisite: immutable draft revision read

> **For Codex:** SDD RED → GREEN → REFACTOR, exact uncommitted checkpoint and independent Spec/quality review. R4 Task1 output seeding waits this seam.

**Goal:** Read the exact immutable Workspace draft revision frozen in an admitted Run after the mutable current head advances. The current `DraftHeadStore.Read` only joins files for the *current* head and cannot safely serve an older seed.

**Sources:** Approved Craft Spec current Workspace continuation; S1 head/revision schema and reviews; S2 typed seed and PG replay review; `2026-09-24-craft-107-runview-r4-version-plan.md` Task1. The R4 worker found this interface gap before runtime edits.

## Global constraints

- Integration Worktree, no commit/push. Preserve existing S1 `Read`/`Advance` behavior, owner authorization and immutable revision/file constraints. No promoted Version or previous Run directory fallback.
- A revision 0 seed is explicitly empty and must match the persisted empty head lineage; selected revisions are positive and must return the exact stored manifest/files even when a later head is current. Missing, wrong tenant/session/owner, corrupt file set or digest fails closed.

## Task 1 — Add frozen revision lookup

**Role:** backend_implementer. **Owned files:** `internal/modules/craft/draft_head.go`, `internal/application/repository/craft_draft_head.go`, focused new repository tests. **Consumes:** `(scope, workspaceID, revision)` from a server-owned admitted seed. **Produces:** `ReadRevision(ctx, scope, workspaceID, revision) (craft.DraftHead,error)` with immutable identity and files.

1. RED SQLite and disposable PostgreSQL: advance D1 then D2, read revision 1 and prove D1 source/digest/files are unchanged; read revision 2 gives D2; revision 0 explicit empty; wrong owner/tenant/session, missing revision, changed persisted file/digest, duplicate/invalid path, corrupt metadata fail closed. A concurrent owner transfer cannot authorize a separate file read using a newer owner snapshot.
2. GREEN: extend the domain port narrowly. Use one authorized joined statement (or transaction snapshot with lock) over Workspace, immutable revision and files rather than joining only current head; validate `DraftHead` state, exact manifest digest, canonical files and identity. Do not trust the current head revision as a substitute for requested revision. Preserve zero-state provenance without inventing files.
3. Run focused SQLite/PG17 normal/race tests, S1 head selectors, gofmt/diff-check; save task-local patch, hashes, DB version/setup/cleanup and independent review. If revision-0 provenance cannot be established from existing rows, report the precise schema gap before coding around it.

**Acceptance:** R4 can resolve a frozen D1 after D2 advances and compare it with the admitted typed seed. Independent review PASS releases R4 Task1. **Failure handling:** keep output seeding and production runtime disabled.

## Review focus

Authorization and revision/file reads share a consistent DB snapshot; immutable revision identity vs mutable head; manifest recomputation; no fallback to current head or last Version.
