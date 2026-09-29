# Issue #72 Live Terminal Map and OCR Result Archive Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve the directly verified Paseo terminal-to-process/task mapping and final documentation OCR coverage result in the Issue #72 recovery record.

**Architecture:** Archive the sanitized read-only terminal map as a source-linked snapshot and append the exact-range OCR result and terminal evidence to the existing execution ledger. Keep all issue readiness and lane-release states unchanged.

**Tech Stack:** Paseo terminal capture, process metadata, Markdown, SHA-256.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`, `docs/plans/issue-72-execution-ledger.md`, `/tmp/issue72-live-terminal-map-20260930-0213.md`, `docs/plans/issue-72-r16-task0-parent-config-audit-2026-09-30-final-ocr.md`.

## Global Constraints

- R-6 remains explicit published Billable Metric→dimension ownership; ambiguous ownership fails closed only for the affected dimension.
- R-3 continues excluding Lago Usage Charges from paid plan purchases.
- #87 remains gated by #86 acceptance and Lago Task 0 runtime contract evidence.
- A terminal task map does not establish lane release; clean worktree/process waiting status is not release evidence.
- Preserve the exact distinction between verified process↔terminal association and task identity derived from captured terminal output.
- Documentation-only work must not start, stop, send input to, archive, or otherwise change terminals, agents, worktrees, Issues, services, or containers.
- A zero-selected OCR result is incomplete coverage, never a pass.

## Review Focus

- Association accuracy: distinguish direct `PASEO_TERMINAL_ID` PID mapping from task classification based on terminal captures.
- Worktree evidence: distinguish process cwd from explicitly shown edited file paths in isolated worktrees.
- Gate integrity: do not use these observations to promote #72/#86/#87 readiness or declare a production lane released.
- OCR coverage: record the exact range/session/output and state that zero selected Markdown files is not a pass.
- Privacy: include only process/terminal IDs, task names, worktree paths, status and hashes; do not include environment secrets or unrelated terminal content.

---

### Task 1: Archive terminal mapping and final OCR coverage

**Files:**
- Create: `docs/plans/issue-72-live-terminal-map-2026-09-30-0213.md`
- Create: `docs/plans/issue-72-r16-task0-parent-config-audit-2026-09-30-final-ocr.md`
- Modify: `docs/plans/issue-72-execution-ledger.md`
- Modify: this plan's Task 1 checkbox and checkpoint record.
- Modify: `.superpowers/sdd/issue-72-r16-task0-parent-config-audit-2026-09-30-plan/progress.md` (local SDD recovery evidence)
- Local SDD report: `.superpowers/sdd/issue-72-live-terminal-map-2026-09-30-plan/task-1-report.md`

**Interfaces:**
- Consumes: `/tmp/issue72-live-terminal-map-20260930-0213.md` (SHA-256 verified by the controller), final OCR file `docs/plans/issue-72-r16-task0-parent-config-audit-2026-09-30-final-ocr.md`, and current SDD review `.superpowers/sdd/issue-72-r16-task0-parent-config-audit-2026-09-30-plan/task-1-fix1-rereview.md`.
- Produces: a durable, sanitized shell map and a Ledger checkpoint recording terminal IDs, PIDs/TTYs, captured task evidence, exact OCR range/session, no-pass outcome, and unchanged gates.

- [x] Preserve the exact capture timestamp, PIDs, TTYs and directly observed `PASEO_TERMINAL_ID` values.
- [x] Record all four captured task identities and only the worktree paths explicitly supported by terminal output.
- [x] State that #72 activity is OCR/recovery documentation activity; no production lane was released or changed.
- [x] Record final OCR BASE `59ad12e0f6b445186894a4a9cd8ee8422a39dddf`, HEAD `2ef29a67a06c42e8a55a844879670dc7bf309821`, session `a01f3ae3-1274-4dc8-b103-7a7487a7d93c`, output path, zero files reviewed, and not-a-pass status.
- [x] Preserve the exact OCR output artifact; state its `Review skipped: no items were selected.` result without calling it a pass.
- [x] Verify source SHA, snapshot hash, links, `git diff --check`, and only the snapshot, OCR output, Ledger and this plan change.
- [x] Commit the documentation checkpoint locally and record the exact commit range covering the snapshot, OCR output, Ledger and this plan.

**Verification:** Independent reviewer confirms mapping method, task classifications, worktree scope, gate status and OCR result against the report and terminal evidence already captured; no live terminal access is required.

**Failure handling:** If the `/tmp` source is missing or its SHA differs, do not recreate terminal history from memory. Keep the new record pending and report the evidence gap.
