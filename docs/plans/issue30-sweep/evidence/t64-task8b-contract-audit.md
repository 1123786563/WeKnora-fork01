# T64 Task 8B independent contract audit — review findings confirmed

- Audited HEAD: `cbdb08e627b3466cc419815e3108fe18bb0b3bfc`.
- Inputs: Task 8B implementation plan/checkpoint, ADR-0015, Marketplace spec §§10–11, original Task 8B report and independent review.
- Role: read-only architecture audit. No source edits or tests.

All four independent-review findings are valid:

1. **F1 — HIGH, retired Variant identity mutable.** SQLite `000126` and versioned PostgreSQL `000205` triggers only guard `OLD.state='published'`; repository update guard has the same limitation. Retired rows with non-null `published_at` still provide exact cancellation attribution. Enforce immutability for every previously published Variant (`published_at IS NOT NULL`) in triggers and repository behavior.
2. **F2 — HIGH, zero-row placeholder transition commits.** Release cancellation in `agent_security.go` and owner/expiry cancellation in `agent_chat_turn_claim.go` check SQL errors but not exact message-row cardinality. A missing or mismatched placeholder can leave the claim cancelled while the message is unchanged. Require exactly one tenant/session-bound placeholder transition so the enclosing transaction rolls back on mismatch.
3. **F3 — MEDIUM, claim DDL does not match storage contract.** The approved plan specifies primary key `(id, source_tenant_id)`, unique `(session_tenant_id, assistant_message_id)`, unique `(session_tenant_id, user_message_id)`, and index `(source_tenant_id, state, release_id)`. Both dialects and GORM tags omit/alter these guarantees. Align entity and both schemas; nullable user-message IDs remain compatible with a unique index.
4. **F4 — MEDIUM, committed revocation is returned as failure.** Service transactions commit revocation, audit, claim cancellation, and durable pending Run reconciliation before the immediate reconciler runs. A subsequent reconciliation error is returned as a command error, and the handler withholds the committed revocation ID. Return the committed identity with explicit pending status and report immediate reconciliation failure separately; preserve durable worker retries and never imply Run cancellation is complete while pending.

## Task boundary and current status

- The corrected Task 8B Brief owns all affected repository, service, type/interface, migration, migration-test, and report paths. No Task8C+ source ownership is needed.
- Task 8B original focused validation passed at the audited HEAD; broad repository/service/container/session suites still fail on Marketplace Run fixtures lacking security pins. This is documented integration debt for 8C/8E and remains an integration verification gate.
- Task 7 R1 review/validation reports are complete at `9114a843bf25a14ed1edeac16719c6fad223fe08`. The coordination tree's `container.go` and `agent_security_wiring_test.go` are already byte-identical to the reviewed Task7 tree for the touched paths (`git diff 9114a843..HEAD -- <paths>` was empty); cherry-picking those commits was therefore empty and was skipped. No Task7 change was lost.
- Task 9 has no active implementation worktree. Its tests own separate test files but require the reviewed Task7 stack and verified/integrated Task8 runtime gates; it remains dependency-blocked.

## Ruling

Proceed with one serial Task 8B review-fix implementation round for F1–F4 in the original 8B worktree. The architecture audit verified every finding, and the fix plan preserves the Task8B file boundary. Do not dispatch Task8C until the amended 8B checkpoint passes independent re-review and backend validation and is integrated. Cost if wrong: a mistaken storage/service contract correction could alter retry or lifecycle behavior; mitigate with failing regressions and scoped independent re-review.
