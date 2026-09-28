# T64 Task 8 independent plan review — round 4

**Scope:** `plan-t64-task8-atomic-admission.md`, parent `plan-t64.md` Task 8/9, proposed ADR-0015, approved Agent Marketplace spec §§10–11, Issue #64 snapshot, and relevant current repository code. Read-only review of requirements and source; no implementation or tests run.

## Verdict

- **Spec compliance: conditional PASS for the forward design.** The proposed tenant-guarded admissions, exact Release/dependency pins, durable AgentQA claims, in-flight `cancel`/`allow` behavior, and append-only history align with the approved spec. The migration downgrade gap below prevents approving the full lifecycle as written.
- **Plan/code quality: FAIL for dispatch.** The Task 9 sidecar read assertion is inconsistent with GORM column mapping, and deployment/downgrade gates need explicit sequencing. Resolve the findings before issuing the 8A Brief.

## Findings

### R4-1 — High — Live downgrade can discard pending cancellation and claim history

**Evidence:** Task 8 defines paired up/down migrations adding `run_cancellation_state`, `agent_runs.security_*`, and `agent_chat_turn_claims` (`plan-t64-task8-atomic-admission.md:167–172`), and tests a down/up round trip (`:194`, `:199`). It does not define when a production down migration is permitted. A `cancel` revocation may be committed with `run_cancellation_state='pending'` and await the worker (`:198`); dropping that column removes the retry obligation. Dropping the claim table or Run pins also removes security and historical admission facts that the approved spec requires retaining (`docs/specs/2026-09-20-agent-marketplace-domain-model.md:153–156`; `CONTEXT.md:161,185`).

**Impact:** Rolling back the schema while live claims, pinned Runs, or pending cancellation obligations exist can let affected work continue and destroy evidence. An empty-database down/up test does not establish safe live rollback.

**Smallest correction:** State that operational rollback uses code rollback while retaining the forward schema. Make the SQL down migration refuse live rollback unless there are no pending obligations, active claims, or retained Run pins requiring it, and document a deliberate archival/restore procedure if a destructive downgrade is ever required. Add a populated-state downgrade refusal test.

### R4-2 — Medium — Signed preflight can become stale before migration

**Evidence:** The plan requires a signed per-tenant review report before startup migration (`plan-t64-task8-atomic-admission.md:62`, `:167`). It explicitly says the SQL migration can recheck mapped-row facts but cannot attest human classification of unmatched active Runs. No writer-quiescence or final report-to-database snapshot boundary is specified between signoff and the startup migration.

**Impact:** An unmatched active Run created after signoff can be absent from the signed report. The migration cannot determine whether a row without a Variant is native or has lost Marketplace lineage, so it cannot enforce the review gate for that new row.

**Smallest correction:** Quiesce old Run writers before the final per-tenant preflight, sign the final report against that quiescent database state, then run the migration before reopening writes. Record this order and a check that the reviewed active-Run set has not changed. Continue to make unresolved rows block rollout.

### R4-3 — Medium — Task 9 sidecar assertion scans into mismatched field names

**Evidence:** The amended fixture inserts exact `security_*` columns (`plan-t64.md:1747–1750`), but its `db.Raw(...).Scan(&livePin)` selects `security_agent_id`, `security_local_agent_version_id`, `security_release_id`, and `security_pin_source` into a struct with fields `AgentID`, `VersionID`, `ReleaseID`, and `Source` (`:1751–1756`). GORM's default names for those fields are `agent_id`, `version_id`, `release_id`, and `source`; the selected column names do not map to them.

**Impact:** The intended exact-pin HTTP evidence fails at its fixture assertion, or reads zero values instead of the persisted sidecars.

**Smallest correction:** Add explicit `gorm:"column:security_*"` tags or SQL aliases matching the struct fields, then assert the values as planned.

## Resolved prior findings and scheduling

The direct Task 9 Run fixture now supplies exact sidecar pins (`plan-t64.md:1747–1750`). The migration now backfills sidecars without rewriting JSON snapshots, adds a post-backfill immutability trigger, and guards old unpinned Marketplace inserts (`plan-t64-task8-atomic-admission.md:62,167,171–172`). The signed report is acknowledged as an operational gate rather than falsely attributed to SQL enforcement. Reconciliation is tenant-scoped and follows tenant guard → ledger row recheck → atomic Run/event/session/count/state update, with cumulative counts, two-replica tests, and startup/periodic cleanup ownership (`:176,181,198,201`). The 8A→8B→8C/8D→8E dependencies and disjoint file map remain coherent (`:40–46`).
