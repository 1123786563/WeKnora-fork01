# Craft version test helper: use persisted Workspace identity

> **For Codex:** Correct a test-only helper broken by the reviewed draft-head FK, with RED→GREEN, exact checkpoint and independent review. No production/migration change or commit.

**Sources:** `2026-09-24-craft-107-t08-acl-workspace-fixture-report.md` proved removing ACL test rewrites alone does not fix `publishCraftVersion`, which fabricates `ws-of-<session>`; new S1 FK prevents unchecked Workspace PK rewrites. Two ACL tests and `TestCraftSessionVersionsAndDownloadChain` use the helper. Current persisted Workspace ID is authoritative.

## Global Constraints

Own only `internal/application/service/craft_session_test.go`, `craft_session_acl_test.go` and report/checkpoint. No production source, migration, audit/router test edits. Keep every existing access/version/download assertion. Do not disable FK or fabricate Workspace ID. Any setup SQL mutation must check its error.

## Review Focus

Helper reads exact persisted tenant/session/owner Workspace ID; no cross-tenant lookup or silent missing row; three stale unchecked ID rewrites removed; original ACL and version chain tests retain coverage; exact pre/post patch.

## Task 1

**Depends on:** failed test-only fixture correction and S1 FK. **Owner:** mechanical_worker. **Validator:** reviewer. **Owned files:** two named test files.

1. RED: run two ACL tests plus version/download chain and capture `craft not found: workspace ws-of-...`; show helper's fabricated ID and unchecked PK rewrites.
2. Make `publishCraftVersion` load persisted Workspace ID by authenticated tenant/session/owner from fixture DB and require exactly one row; compute Version ID from that returned ID. Remove all three stale PK rewrites. Do not alter production VersionStore or assertions.
3. Run three named tests and broad `Craft.*Access|SessionShare` selector when concurrent code compiles, `git diff --check`, exact pre/post hashes/patch/report; independent review.

**Acceptance:** version/ACL fixture passes with real FK and same assertions. **Failure handling:** if production VersionStore itself rejects a correctly scoped persisted Workspace, report separate seam rather than loosening FK.
