# Task8 interface audit R4 — 2026-09-29

Scope: read-only final check of Task8 plan, ADR-0015, and current migration runner. No production code or tests were changed or run.

## Result

No remaining architecture blocker in the assigned scope.

- **Rollback compatibility:** The plan and ADR forbid a pre-Task8 binary while adopted AgentQA or Workbench ingress is reachable. They require a security-compatible binary with both admission gates, or gateway rejection of both ingress routes before older code starts. Populated down migrations refuse to drop claim, revocation, or pinned Run history; production rollback retains the forward schema. This preserves the approved security boundary.
- **PostgreSQL transaction:** The plan explicitly requires `BEGIN;`/`COMMIT;` around each versioned migration because the configured golang-migrate PostgreSQL path does not promise an implicit file transaction. `ACCESS EXCLUSIVE` locks on `agent_runs` and `agent_adoption_variants` precede the final mapping check and remain held through backfill and guard creation. Quiesced old writers and the release operator's frozen-set hash check cover the period before locks are taken. The migration Brief must preserve this exact order and verify it against the actual driver.
- **SQLite transaction:** `internal/database/migration.go` uses the normal per-file transactional driver for migrations other than its explicit NoTxWrap exceptions at 55 and 114. The proposed 126 file remains on that path; writer quiescence and one committed migration close the backfill/guard gap.
- **Run identity and evidence:** The plan requires all four `security_*` fields NULL or all populated with a restricted source, then forbids updates after the one-time backfill. Exact cancellation compares tenant, Agent, Version, and Release; non-Marketplace Runs with NULL pins remain outside the target. Signed unmatched-Run decisions and raw IDs stay in controlled deployment evidence, while Git receives only templates and opaque references.
- **Ownership:** 8B owns migrations, Run cancellation/reconciliation, claim repository, Task7 Container follow-on wiring, and the handler dependency seam. 8C owns Run admission; 8D owns AgentQA behavior; 8E owns Workbench composition. Task9 waits for their integration and owns only its end-to-end tests.

This is approval readiness for the design and interface contract. The actual SQL, migration behavior, live-data signoff, and rollback procedure require implementation and deployment verification.
