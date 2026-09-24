# T10 evaluation UI review fixes

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a user re-evaluate a saved fixed JD after editing the profile and show a hard qualification warning in the current conversation result.

**Architecture:** Give a stable, owner-scoped Opportunity evidence or Evaluation detail page an explicit “Evaluate with current profile” action using the page's fixed `(opportunityId,snapshotId)` and a fresh request ID. Keep old evaluation links; a new result gets a distinct immutable ID. The transient import result card renders receipt status immediately, with `ineligible` prominent before the next-action link. A 403/scope change clears local result and redirects consistently. No backend/contract changes.

**Sources:** #150, approved Job Search Spec §6.1, T10 implementation plan Task 4, independent UI review of `6e6a903bd` high re-evaluation gap and medium missing conversation warning.

## Global Constraints

- Work in isolated T10 UI worktree; own only `apps/web/src/career/OpportunityPage.tsx`, its tests, local CSS, and affected route tests if needed. Preserve others' edits. No Go, migration, shared contract/client, or architectureguard edits.
- Local task commit authorized; no push, merge, deploy. Reuse reviewed typed API and immutable snapshot IDs. Each action uses a fresh idempotency request ID; retry of uncertain write retains that ID and uses receipt lookup.
- Hard `ineligible` is shown before soft match/next action. `unknown` is never presented as eligible. JD remains inert text.

## Review Focus

- Import/evaluate success card immediately shows `不符合`, `符合` or `待确认` from receipt status, with warning first for `ineligible` and a stable evidence/result link.
- After profile edit, page reload or navigation, user can open saved Opportunity evidence or old Evaluation detail and evaluate the same fixed snapshot with the current confirmed profile; new evaluation ID differs, old URL still opens its historical profile revision and conclusion.
- Pending, failure, unknown receipt, 403/logout and repeated click states are explicit; no duplicate evaluation intent for one click. Narrow viewport keeps warning, evidence and action visible.

---

### Task 1: Stable re-evaluation and immediate verdict

**Depends:** T10 UI initial code `6e6a903bd`. **Owner/validator:** frontend_implementer / frontend_validator. **Files:** OpportunityPage component/tests, local CSS and narrow route tests as needed. **Consumes:** fixed `OpportunityEvidence`, `EvaluationReceipt`, `Evaluation` APIs. **Produces:** stable current-profile evaluation action and visible tri-state result card.

- [ ] RED: Add test for navigation away to profile, profile edit, return to fixed evidence/old detail route, fresh evaluation of the same snapshot, distinct result link, old result link unchanged. Add tests asserting conversation card shows ineligible warning immediately, unknown distinct, repeat click guarded and uncertain receipt recovery.
- [ ] GREEN: Implement stable action on evidence/detail page using immutable IDs and fresh request ID. Reuse or extract the existing evaluate/reconcile flow without widening ownership. Render receipt status on transient card before result link; preserve hard-first detail layout.
- [ ] VERIFY: Focused UI/route tests, `pnpm typecheck:web`, `pnpm test:web`, `pnpm build:web`, `git diff --check`. Independent Spec/quality review and frontend validation before integration; controller performs live authenticated browser and narrow layout acceptance on integrated code.

## Shared-file and interface preflight

Only one frontend implementer writes UI files. Go and typed client are already reviewed and integrated; this task consumes those exact interfaces. Reviewers and validators are read-only. No concurrent Web build writes in the same worktree.
