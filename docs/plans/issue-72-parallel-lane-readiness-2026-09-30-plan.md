# Issue #72 Parallel Lane Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve an authoritative live snapshot of Issue #72 lane ownership/readiness and the new official Lago v1.53.0 Task 0 source evidence without interfering with ongoing work.

**Architecture:** Synthesize current read-only process, Worktree, Issue API, and lane audit evidence into one dated report. Update the Issue DAG recovery overlay and execution Ledger only to record verified facts and remaining gates; do not claim that non-integrated branches or static upstream research satisfy runtime acceptance.

**Tech Stack:** Markdown, Git state inspection, authenticated GitHub API snapshots, and pinned upstream Lago source references.

**Spec:** [Issue #72 execution DAG](issue-72-dag.md), [Issue inventory](issue-72-issues-inventory.md), [approved user rulings](issue-72-user-rulings.md), [R16 plan in owner-controlled integration worktree](../../.worktrees-issue72/lago-int/docs/plans/issue-72-plan-87-r16.md), and read-only evidence reports listed in Task 1.

## Global Constraints

- Preserve active/owner-controlled Worktrees, dirty files, branches, shared Docker/PostgreSQL services, and all running processes.
- Treat only verified integrated work as part of the current integration baseline; do not merge, cherry-pick, stage, or commit from another lane.
- Keep #82/#86 acceptance gaps and #87 Task 0 / integration gates open until their exact evidence requirements pass.
- Record the fresh GitHub checks as a partial status spot check; they do not replace the prior complete recursive Issue traversal.
- Keep user rulings R-6 and R-3 settled: explicit published-version metric-to-dimension ownership with dimension-scoped fail-closed ambiguity; continue excluding Usage Charges from paid-plan purchase.
- Do not expose credentials, perform remote writes, start/stop services, or alter GitHub state.

## Review Focus

- **Lane ownership:** Distinguish active writer/draft branches, reviewed-but-unintegrated branches, dirty owner-controlled checkpoints, and stale/read-only evidence using path, SHA, status, commit ancestry, and agent/process evidence.
- **Parallel readiness:** State whether independent implementation Tasks are ready under the DAG; label shared-file and service conflicts explicitly.
- **Issue status:** Separate fresh remote OPEN state from local acceptance state and preserve the limited scope/timestamp of API checks.
- **Lago Task 0 evidence:** Integrate official v1.53.0 source findings with pinned links and formulas; separate source facts from unproven runtime timing/configuration.
- **Acceptance gates:** Do not infer #82 AC4, #86 exact-current-hash runtime acceptance, or #87 readiness from static research, helper tests, branch cleanliness, or a plan review.

---

### Task 1: Record the current parallel frontier and Lago Task 0 source evidence

**Files:**
- Create: `docs/plans/issue-72-parallel-lane-readiness-2026-09-30.md`
- Modify: `docs/plans/issue-72-dag.md` (recovery overlay only)
- Modify: `docs/plans/issue-72-execution-ledger.md`
- Modify: this plan's checkboxes

**Interfaces:**
- Consumes: live Issue API spot checks at 2026-09-30 00:42 CST; current `git worktree list` and per-worktree `git status`; process/Paseo snapshot at 00:34–00:42 CST; read-only audits `/tmp/issue72-82-frontier-audit-20260930.md`, `/tmp/issue72-86-checkpoint-validator.md`, `/tmp/issue72-recovery-gate-audit.md`, and worker lane reports for #84/#85/#86/#87/R8; official-source research `/tmp/issue72-lago153-task0-contract-research-20260930.md`, `/tmp/issue72-lago153-debit-source-20260930.md`, `/tmp/issue72-lago153-package-rounding-20260930.md`, `/tmp/issue72-lago153-api-errors-20260930.md`, and `/tmp/issue72-75-expiry-source-audit-20260930.md`.
- Produces: a durable current lane/ownership table, ready/blocked assessment, R16 Task 0 source-fact matrix and remaining runtime probe checklist; DAG/Ledger updates that do not promote Issue acceptance or integrate any other branch.

- [x] Record the fresh GitHub status spot check for #72, #74, and #82–#87 (all OPEN at read time), and state that the complete native tree traversal remains the 2026-09-29 audit of 33 direct children and no grandchildren.
- [x] Record each relevant Worktree's current path, branch, HEAD, clean/dirty status, owner task, review/validation state, and write-overlap boundary: #82 T9 fixture, #84 OCR R1, #85, #86, #87 `lago-int`, R8 Task 2/Task 8, the shared root checkout, and this documentation worktree. Distinguish audited fact from an unverified process-to-worktree inference.
- [x] Record the four still-live Codex shells and known session/task correlation, current root cwd, unrelated live #140 OCR, idle unrelated Paseo agent, and already-running shared service containers; state that no process, lane, or service was changed or released.
- [x] Summarize official Lago v1.53.0 source facts: package charge formula and cents path; Wallet field equations; asynchronous refresh chain, draft-fee participation and lazy/negative balance behavior; traceable-transaction refusal codes and HTTP 422 mapping; cite pinned primary-source URLs/lines.
- [x] List unresolved R16 Task 0 evidence: actual parent charge configuration/free_units and pricing-unit paths; event store/queue and observed timing; endpoint/payload correlation for refusal; expiry-boundary consumption proof; and exact runtime verification on an isolated pinned Lago stack. Keep Task 0 and #87 implementation not ready.
- [x] Update DAG §8 and the execution Ledger with the recovery snapshot and explicit next-safe action(s), preserving R-6/R-3 and all #82/#86/#87 gates.
- [x] Run `git diff --check`; verify DAG node/edge counts did not change, only the specified four Markdown paths changed, and no sibling/root/`lago-int` worktree changed.
- [x] Commit only the allowed documentation files with a message beginning `docs(issue-72): record parallel lane readiness`.

**Verification:** All reported hashes and status values match the current Git/API evidence or are explicitly labelled as a timestamped audit observation; pinned Lago URLs resolve to tag `v1.53.0` / commit `591ae9005110346f1c6034ec72ea9046625668cf`; no unsupported runtime claim or readiness promotion is present; `git diff --check` passes; only the Task 1 owned documentation paths are included.

**Failure handling:** If sources disagree, record both source and time without reconciling by guess. If a lane becomes dirty/active during synthesis, preserve it and do not inspect beyond the assigned read-only audit. If upstream source cannot substantiate a behavior, classify it as unknown and leave the runtime gate open.
