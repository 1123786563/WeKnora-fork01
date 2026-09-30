# Issue #72 R16 Task 0 Parent Charge Configuration Audit Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Preserve durable evidence explaining whether the committed Issue #72 checkout contains actual parent Lago Plan charge configuration required by R16 Task 0.

**Architecture:** Archive the read-only audit without converting synthetic T04 fixture values into parent-plan claims. Link each conclusion to committed source evidence and keep the Task 0/#87 gate open until an isolated pinned Lago readback and quantity-boundary probe exist. Record the prior refresh task's final OCR-record review outcome in its own Ledger entry.

**Tech Stack:** Git committed-file inspection, Markdown, SHA-256.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`, `docs/plans/issue-72-plan-87-r16.md` Task 0, `/tmp/issue72-task0-parent-charge-config-audit-20260930.md`, and prior Task 1 review `.superpowers/sdd/issue-72-live-tree-and-agent-refresh-2026-09-30-plan/final-ocr-record-review.md`.

## Global Constraints

- T04 values `7` fen/unit, `1500` fen/package, `package_size=100`, `free_units=0` belong to synthetic `weknora-t04-*` experiment objects, not the parent plan.
- Parent-looking historical `weknora-pro-v1`, `weknora-pro-v2`, and `weknora-base-v1` readbacks have `charges:[]`; they do not prove current parent charge configuration.
- R16 Task 0 must capture the exact parent publication identity, charge/metric fields and `1/99/100/101` measured behavior on an isolated pinned v1.53.0 Lago stack.
- Preserve approved R-6 explicit metric→dimension mapping and dimension-scoped fail-closed behavior; continue excluding Usage Charges from purchase under the confirmed rule.
- Keep #87 implementation gated by #86 acceptance and Lago Task 0 runtime contract evidence.
- Do not inspect dirty `lago-int` or root files, access credentials/secrets, query or mutate a live Lago service, or start/stop services.

## Review Focus

- Synthetic-versus-parent identity: never present T04 fixture constants as actual parent configuration.
- Evidence strength: historical charge-less readbacks prove only that those snapshots have no charges; they do not prove that no current parent version can carry charges.
- R16 acceptance: separate captured base amount from missing package/free-unit/metric/dimension facts and required runtime probe.
- Gate discipline: no Task 0 completion, #87 readiness, or #86 acceptance promotion from this static audit.
- Ledger consistency: preserve the exact source audit hash, review result, and the exact current implementation/runtime evidence still needed.

---

### Task 1: Archive the parent charge configuration evidence gap

**Files:**
- Create: `docs/plans/issue-72-r16-task0-parent-config-audit-2026-09-30.md`
- Modify: `docs/plans/issue-72-execution-ledger.md`
- Modify: this plan's Task 1 checkbox.
- Local SDD evidence: `.superpowers/sdd/issue-72-r16-task0-parent-config-audit-2026-09-30-plan/task-1-report.md`

**Interfaces:**
- Consumes: `/tmp/issue72-task0-parent-charge-config-audit-20260930.md`; committed T04 fixture and parent-looking readback files; R16 Task 0 requirements and readiness gates.
- Produces: a durable, source-linked evidence audit and a Ledger update that closes no acceptance item; also records the independent review of the prior final OCR skip record.

- [x] Preserve the audit's verdict that no authoritative parent Plan charge config exists in the inspected committed checkout at `86b6e7ac0e921f7e1d4fd328ce29be3aa12dc6ce`.
- [x] Distinguish T04 synthetic values from parent-looking readbacks; record their paths, IDs, and observed `charges:[]` without generalizing to all current Plans.
- [x] Record what R16 Task 0 still needs: exact parent publication/version identity, billable metric-to-dimension mapping, charge model/amount/package/free-unit/pricing fields, plus 1/99/100/101 runtime measurement and sanitized evidence on isolated v1.53.0.
- [x] Keep Task 0 incomplete and #87 not ready; carry forward the #86 acceptance gate and approved R-6/R-3 rulings unchanged.
- [x] Record the prior final OCR result-record Review (range `3a3ec8820150aefb1edf913c0fe775c53722efc2..59ad12e0f6b445186894a4a9cd8ee8422a39dddf`) as Spec PASS / quality PASS, no finding, with report path.
- [x] Verify the archived source SHA-256 matches `/tmp`, all local links resolve, `git diff --check` passes, and only the new report, Ledger, and plan checkbox change.
- [x] Commit the archive and Ledger as one local documentation checkpoint.

**Verification:** Independent reviewer confirms the archive distinguishes fixture from parent evidence, cites the exact committed paths/values, does not overclaim current Lago state, preserves all open gates/rulings, records the source hash and prior review verdict, and changes no Issue/lane status.

**Failure handling:** If the audit source hash changes or current repository contents no longer match the audited commit, state the exact inspected commit and keep the report explicitly historical. Do not query live Lago, inspect owner-controlled dirty contents, or infer a missing parent charge config from a fixture.
