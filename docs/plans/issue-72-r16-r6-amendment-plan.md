# Issue #87 R16 Plan Amendment for R-6 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update the active #87 R16 implementation plan so its Task 0A and downstream dispatch rules reflect the user's approved R-6 dimension ownership and Charges decisions.

**Architecture:** Keep R16 as the active implementation plan and preserve its historical review/task structure. Amend only stale approval gates: the approved explicit published-version metric-to-dimension owner rule is now fixed, and the approved R-3 scope keeps charge-bearing paid purchases excluded. #86 acceptance/integration, isolated authenticated Lago contract evidence, and migration-head rechecks remain dispatch gates.

**Tech Stack:** Markdown; `git diff --check` and targeted stale-gate search.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md` 2026-09-29 amendment; `docs/adr/0012-lago-as-commercial-billing-authority.md` 2026-09-29 ruling; `docs/plans/issue-72-user-rulings.md` R-6; `docs/plans/issue-72-plan-87-r16.md`.

## Global Constraints

- Published Plan Version explicit Billable Metric→dimension mapping is the ownership basis.
- Only a unique active subscription with applicable published mapping and usable authority can own a new paid call for a dimension.
- Base/paid overlap or duplicate declarations fail closed for the affected dimension; other uniquely owned dimensions may continue.
- Pending, canceled, inactive, stale, unreadable, or otherwise unusable authority remains fail-closed.
- Keep `PurchaseService.ensureNoCharges`; no Usage-Charge-bearing paid-plan purchase path is in scope.
- Do not claim #87 implementation readiness until #86 is verified/integrated and the isolated authenticated Lago v1.53 contract gate passes.

## Review Focus

- No R16 prose may say the owner/overlap ruling is pending after R-6.
- No R16 prose may require an R-3 amount/currency/Usage-line purchase oracle to remove `ensureNoCharges`; the approved choice is to retain the guard and exclude that purchase path.
- Tests and task interfaces must distinguish dimension-scoped ambiguity from tenant-wide denial.
- Unrelated #86, Task 0, migration-head, PostgreSQL, and static review gates remain intact.
- The plan must not imply that user approval or documentation updates prove runtime behavior.

---

### Task 1: Amend R16 gates and ownership interfaces from R-6

**Files:**
- Modify: `docs/plans/issue-72-plan-87-r16.md`

**Interfaces:**
- Consumes: Approved R-6 in `docs/plans/issue-72-user-rulings.md`, the 2026-09-29 approved Spec amendment, and ADR-0012 revision.
- Produces: An R16 plan where Task 0A is closed by R-6; Task 4 has explicit mapping/overlap behavior and unconditional `ensureNoCharges` retention; Task 6 has fixed owner and charge-exclusion fixtures; independent #86/Lago/migration gates remain.

- [ ] Update the Spec provenance in the R16 header and its first Global Constraint to cite the 2026-09-29 R-6 amendment, state the exact owner mapping/overlap rule, and state that paid Usage-Charge plans remain outside the purchase path.
- [ ] Replace Task 0A's pending owner and R-3 approval checklist with a dated “resolved by R-6” record that cites the four authoritative documents and makes clear that this resolves product ambiguity only.
- [ ] Remove Task 0A as an owner-choice dependency from Task 4. Require the resolver tests to cover unique Base owner, unique paid owner, Base/paid overlap, duplicate declarations, and dimension-scoped blocking from the explicit published mapping. Keep stale/inactive/lifecycle fail-closed cases.
- [ ] Make `PurchaseService.ensureNoCharges` retention unconditional throughout R16. State that charge-bearing paid-plan purchases and their acceptance are excluded; do not design or test a new payment line. Keep the existing valid no-Charges single-line purchase path constraints.
- [ ] Update Task 6, the interface table, coverage self-review, and dispatch blockers to use the fixed R-6 rule. Retain #86 acceptance/integration, authenticated isolated Lago v1.53 evidence, schema stream head rechecks, and PostgreSQL runtime requirements as gates.
- [ ] Search the resulting plan for stale wording that says R-6 ownership/Charges decisions remain pending; inspect every hit and preserve references to historical R-3 decisions only when they describe the currently approved no-Charges path.
- [ ] Run `git diff --check`; confirm no other file changed; commit only `docs/plans/issue-72-plan-87-r16.md` and this amendment plan.

**Acceptance:** Task 0A records the two settled user rulings; Task 4's exact owner/overlap behavior is implementable without new product choices; charge-bearing paid purchases remain excluded; all remaining independent technical gates are explicit and unchanged.

**Failure handling:** If R16 depends on a broader behavior that conflicts with R-6 or another approved source, document the exact conflicting line in the SDD ledger and stop before broadening requirements.
