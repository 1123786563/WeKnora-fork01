# Issue #72 Unsupported Workspace Documents R1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct verified stale or unsafe Issue #72 worktree documents that the OCR CLI excluded because their file types are unsupported.

**Architecture:** Preserve approved historical decision/evidence records, but annotate superseded statements and make current acceptance state explicit. Remove secret values from the untracked environment snapshot and add a narrow ignore rule so it cannot enter a later commit.

**Tech Stack:** Markdown, Git ignore rules, redaction-only text transformation.

**Spec:** Approved Issue #72 migration design, ADR-0012, CONTEXT.md, R-5 user ruling in `docs/plans/issue-72-user-rulings.md`, execution Ledger, and independent reports `/tmp/issue72-unsupported-docs-core-review-20260928.md` and `/tmp/issue72-unsupported-docs-plans-review-20260928.md`.

## Global Constraints

- R-5 says Base and paid subscriptions are separately priced and separately identified; do not revert or weaken the ruling.
- Historical run outputs and plans remain recognizable as historical evidence; annotate status instead of deleting legitimate evidence.
- Never copy secret values into the report, plan, or Git output. The runtime env snapshot is not executable evidence and is not needed by tests.
- Issue #86's historical live flow PASS does not prove current exact-hash replay, duplicate response behavior, stable top-up identity, or tenant mapping.
- Preserve all unrelated edits. No live service/network, Issue mutation, push, merge, or deployment.

## Review Focus

- No current plan/DAG states Base and paid subscription transitions reuse the same external subscription identity.
- Historical #86 live evidence is labeled with its original hash/date and clearly separated from current unverified acceptance.
- No credential value remains in the environment snapshot or enters the Git change set.
- Current plan-review and #86 R5 ledger status agrees with execution evidence.
- DAG readiness is an evidence-backed current overlay, not the stale earlier #82-only-ready snapshot.

## Task 1: Align documents and sanitize the local environment snapshot

**Dependencies:** Independent read-only reviews above. **Owner:** mechanical_worker (precise documentation edits). **Validator:** reviewer. **Execution:** serial within the existing Issue #72 integration worktree.

**Files:**

- Modify `docs/specs/2026-09-20-lago-billing-migration-design.md`
- Modify `docs/plans/issue-72-dag.md`
- Modify `docs/plans/issue-72-issues-inventory.md`
- Modify `docs/plans/issue-72-flow-evidence-86/README.md`
- Modify `docs/plans/issue-72-plan-87-r15.md`
- Modify `docs/plans/issue-72-ledger-86.md`
- Modify `.gitignore`
- Sanitize `docs/plans/issue-72-flow-evidence-86/ocr-r2-replay/backend.env.snapshot`

**Consumes:** R-5 ruling and current execution ledger; current DAG audit `/tmp/issue72-current-dag-audit-20260928.md`; unsupported-doc reviews cited in Spec.

**Produces:** current Base/paid identity language; supersession marker for stale status in #72 DAG; honest historical/current #86 README status; current R15 plan review status; persisted R5 offline checkpoint status in issue ledger; sanitized non-executable snapshot with exact `.gitignore` exclusion.

- [ ] **Step 1 — Capture before state.** Record line excerpts (without any credential values), hashes for all owned files, `git status --short`, and verify the env snapshot is untracked and unignored. Do not print or copy any snapshot values.
- [ ] **Step 2 — Requirements language.** Revise the Spec continuity sentence to say paid upgrade/downgrade identity behavior remains unchanged, while Base and paid subscriptions have separate identities and prices under R-5; leave other Base lifecycle identity transitions unresolved. In DAG edges 80→94 and 93→94, remove “same ExternalSubscriptionID” claims and describe the distinct Base/paid identity coordination seam. Update #94 inventory wording from “same Subscription chain” to explicit transition between distinct paid and Base subscriptions.
- [ ] **Step 3 — Readiness status.** Add a dated supersession note near the stale #82-only-ready DAG summary: current evidence overlay in `issue-72-execution-ledger.md` governs and currently no implementation node is verified-ready. Do not erase the historic DAG structure or topology evidence. Update `issue-72-plan-87-r15.md` line 167 to say its plan review is conditionally PASS as recorded by the execution ledger; implementation remains gated on unresolved upstream contracts. Append Task 1–3 R5 offline checkpoint/review status to `issue-72-ledger-86.md`, stating full #86 live acceptance remains unverified.
- [ ] **Step 4 — Historical #86 README status.** Keep the detailed historical test record. Replace the unqualified current conclusion with language identifying the original 2026-09-28 run and recorded integration HEAD as historical evidence, then state exact-hash replay/duplicate contract and stable identity remain unverified at the current integration checkpoint; do not claim full current Issue #86 acceptance. Explicitly correct the credential assurance at README line 14: report that a later audit found the local environment snapshot, that its values have been redacted and its path ignored, and that no credential value is reproduced. Preserve the original run facts separately from the later handling correction.
- [ ] **Step 5 — Secret snapshot sanitation.** Replace every right-hand-side value in `backend.env.snapshot` with the literal `<REDACTED>` and prepend a note that it is a sanitized, non-executable evidence copy. Do not preserve an original value in another file or report. Add the exact snapshot path to `.gitignore` so it remains outside future review/commit scope. A safe scanner must verify all expected assignment names remain and every RHS is exactly `<REDACTED>`; never print values. Verify `git check-ignore` recognizes the path.
- [ ] **Step 6 — Verify and review.** Record `git diff --name-status` plus `git ls-files --others --exclude-standard`; include each owned untracked Markdown file in the before/after hash manifest and human review package. Inspect exact diffs without rendering any prior secret value; review the sanitized snapshot only by variable-name list, placeholder predicate, and hash. Confirm all statements match current ruling/Ledger and no factual historical record was erased. Run `git diff --check` and a whitespace check over owned untracked Markdown using a command that emits no file contents; tests do not apply to documentation-only changes. Record hashes and verification in the task report, then get an independent reviewer to inspect the complete owned-file package including each changed/untracked document.

**Acceptance mapping:** R-5 alignment → Task 1 Step 2; readiness → Step 3; historical acceptance truth → Step 4; secret exclusion → Step 5; validation/evidence → Step 6.

## Plan self-review

- Changes are limited to evidence and planning documents plus exact ignore rule; no product behavior is changed.
- The secret snapshot is redacted in place to avoid moving credentials elsewhere; ignored status prevents accidental staging.
- Historical Issue #86 proof is retained but no longer substitutes for current acceptance evidence.
- The existing full OCR limitation (only code file types supported) remains disclosed; this plan does not claim OCR reviewed unsupported documents.
