# T08 membership migration journey fixture correction

> **For Codex:** Correct only the deterministic duplicate fixture insert, run the focused migration journey and obtain independent test-only review.

**Source:** `2026-09-24-craft-107-t08-migration-journey-triage.md`; B2 broad selector failed before assertions because `openCraftSessionDB` already seeds u1/u2 and this test inserts them again under SQLite's unique `(user_id, tenant_id)` index. No commits; record current HEAD and before hash.

## Global Constraints

No production source/migration change. Keep membership grant/revoke/incarnation assertions and the documented SQLite index-parity limitation. Do not drop the index earlier to hide duplicate setup. Preserve all other tests in the shared file and concurrent edits.

## Task 1 — reuse seeded active memberships

**Depends on:** deterministic isolated triage. **Owner:** mechanical_worker. **Validator:** backend_validator. **Owned file:** `internal/application/service/craft_access_test.go` only. **Consumes:** `openCraftSessionDB` seeded u1/u2. **Produces:** migration journey reaches intended grant/revoke/rejoin assertions.

1. RED: record isolated `TestCraftAccessMigrationJourney` failing UNIQUE at setup.
2. Remove only the redundant u1/u2 insert; explicitly assert both seeded memberships are active if useful. Leave later soft-delete/rejoin and audit assertions intact.
3. Run exact isolated test and affected Craft access tests; save task-local patch, pre/post hash/report, `git diff --check`.

**Acceptance:** journey reaches and passes membership-incarnation behavior without weakening production uniqueness. **Failure handling:** report any next real assertion failure; do not alter schema/production code in this task.
