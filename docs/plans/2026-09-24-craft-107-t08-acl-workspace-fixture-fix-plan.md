# T08 ACL test fixture correction after draft-head FK

> **For Codex:** Correct only two stale test fixture Workspace ID rewrites with RED→GREEN evidence and exact uncommitted checkpoint/review. No production change or commit.

**Sources:** T08 denial audit Task1 independent review, broad selector two failures in `TestCraftSessionTaskAccessGatesContentWritesAndDownloadBytes` and `TestCraftSessionTaskAccessRejectsStaleMembershipAndAdminWithoutGrant` at `publishCraftVersion`: `craft not found: workspace ws-of-...`. Both tests issue unchecked `UPDATE craft_workspaces SET id='ws-of-'+session_id`; new reviewed S1 schema has FK from draft head to Workspace ID. Need preserve original access assertions and not weaken FK.

## Global Constraints

Own only `internal/application/service/craft_session_acl_test.go` and report/checkpoint. Do not alter migration, repository, production access or version source, audit test, or other fixtures. Existing persisted Workspace ID is authoritative; do not force rewrite. If helper requires an ID, use returned Workspace.ID from `createCraftSession` rather than fabricate one. Keep stale-membership and all read/write/download assertions intact.

## Review Focus

Exact baseline failing selector, minimal fixture correction, no loss of ACL coverage, no hidden SQL errors, focused tests and diff.

## Task 1

**Depends on:** broad selector reproducible failure after S1 Task1. **Owner:** mechanical_worker. **Validator:** reviewer. **Owned file:** `craft_session_acl_test.go` only.

1. RED: run the two named tests, capture `publishCraftVersion` failure; inspect `createCraftSession`/`publishCraftVersion` identity assumptions and assert unchecked Workspace ID UPDATE is the cause.
2. Remove or replace only the two stale ID rewrites with correct persisted ID use. Ensure SQL errors are checked for any remaining setup mutation. No reduction in test assertions.
3. Run two named tests and `go test ./internal/application/service -run 'Craft.*Access|SessionShare' -count=1` once concurrent files compile, `git diff --check`, exact pre/post hash and task-local patch/report. Independent review.

**Acceptance:** original ACL journey tests pass on FK-preserving schema with all assertions. **Failure handling:** if version helper itself assumes fabricated ID, report exact seam and stop rather than weakening FK.
