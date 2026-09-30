# Issue #72 R-6 Approved Billing Rulings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record the user's approved Issue #87 dimension ownership and R-3 Charges decisions consistently across the approved billing Spec, ADR, domain glossary, and user ruling log.

**Architecture:** Documentation-only amendment. The Spec is the behavioral authority, ADR-0012 captures the trade-off, `CONTEXT.md` defines the domain term, and the user ruling log records provenance and error cost. No runtime implementation or integration-worktree file is changed.

**Tech Stack:** Markdown; `git diff --check` for whitespace validation.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`; `docs/adr/0012-lago-as-commercial-billing-authority.md`; `CONTEXT.md`; `docs/plans/issue-72-user-rulings.md`.

## Global Constraints

- Lago remains the commercial billing authority.
- Every published Plan Version is immutable.
- Base and paid subscriptions retain separate prices.
- A price is evaluated only through its owning subscription and Plan Version.
- R-3's approved single-line payment path remains in force; do not add a Usage Charge purchase path.
- Record user decisions as requirements, not as evidence of implemented behavior.

## Review Focus

- Base/paid overlap or duplicate declarations must not silently select a winner; only the affected dimension is blocked until configuration is unambiguous.
- Published Plan Version metric-to-dimension mapping must be the explicit ownership basis; do not infer from recency, subscription type, or blended prices.
- Pending, canceled, stale, unreadable, inactive, or otherwise unusable authority must preserve existing fail-closed behavior.
- Charge-bearing paid Plans remain outside the approved purchase path until an R-3-compatible contract specifies amount, currency, Usage-line exclusions, and payment-time invoice comparison.
- Documentation must not imply that #87 implementation gates (#86 integration and authenticated Lago contract evidence) have passed.

---

### Task 1: Record the approved #87 billing rulings

**Files:**
- Modify: `docs/specs/2026-09-20-lago-billing-migration-design.md`
- Modify: `docs/adr/0012-lago-as-commercial-billing-authority.md`
- Modify: `CONTEXT.md`
- Modify: `docs/plans/issue-72-user-rulings.md`

**Interfaces:**
- Consumes: User's two answers in the Issue #72 execution conversation on 2026-09-29.
- Produces: One consistent binding rule across the four documents: published Plan Version explicit Billable Metric→dimension mapping selects the unique active subscription owner; ambiguity blocks new calls for the affected dimension; `PurchaseService.ensureNoCharges` remains; charge-bearing paid plans are excluded until the named R-3-compatible rules are approved.

- [ ] Append a dated 2026-09-29 Spec amendment that defines the unique owner source, dimension-scoped fail-closed behavior, and the no-Charges purchase constraint. State explicitly that this decision closes the business-rule ambiguity but does not satisfy the independent #86 integration or authenticated Lago contract gates.
- [ ] Add the selected fail-closed behavior and its trade-off to ADR-0012, including that unaffected uniquely owned dimensions may continue.
- [ ] Add a concise glossary entry for “计费维度（Billable Dimension）” explaining it is the named unit assigned an explicit billable metric and subscription/Plan Version pricing authority.
- [ ] Append R-6 to the ruling log with both decisions, source/date, operational scope, retained `ensureNoCharges` behavior, the future R-3-compatible evidence required to revisit Charges, and error costs.
- [ ] Review all four changes side by side to confirm terms, scope, and dates align and that no prose claims implementation or external evidence exists.
- [ ] Run `git diff --check`; inspect the complete diff and confirm `paseo.json` remains untracked and untouched.
- [ ] Commit only the four owned documents and this plan as `docs: record issue 87 billing rulings`.

**Acceptance:** The four authoritative records express the same approved behavior and provenance; the diff is whitespace-clean; unrelated files are not staged; no runtime code or external state changes.

**Failure handling:** If a source document contradicts an approved decision outside the touched amendment area, record it in the SDD ledger and stop before broadening the change. Resolve simple wording drift within the owned sections.
