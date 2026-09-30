# Issue #72 Task 17 Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct the three evidence-traceability findings from the independent review of Task 17's live Issue refresh.

**Architecture:** Make a bounded documentation-only repair to the live-tree report and recovery DAG, then record the correction and review lineage in the execution Ledger. The fixes add source-backed #74 governance context, correct temporal attribution of `lago-int` state, and preserve the exact reproducible command templates from the saved traversal report.

**Tech Stack:** Markdown documentation and Git diff checks.

**Spec:** Task 17 brief and review in `.superpowers/sdd/issue-72-parallel-refresh-2026-09-29-plan/`; live traversal source `/tmp/issue72-live-issue-tree-audit.md`; #82 frontier source `/tmp/issue72-82-frontier-audit-20260930.md`; #86 checkpoint source `/tmp/issue72-86-checkpoint-validator.md`; repository Issue #72 DAG and inventory.

## Global Constraints

- Do not change Issue, PR, remote branch, product code, service, or shared resource state.
- Keep #74's remote `OPEN` status distinct from its local `done` acceptance assessment.
- Attribute every `lago-int` state observation to the source audit and its timestamp/checkpoint; do not imply #82 observed a dirty integration worktree if its report says clean.
- Keep #82/#86/#87 acceptance gates unchanged and do not promote downstream status.
- Do not modify or stage files outside the Task 1 owned paths.

## Review Focus

- #74 remote OPEN versus local accepted evidence: retain both states with their separate sources and avoid implying the GitHub Issue was closed.
- Temporal `lago-int` ownership observations: distinguish #82's clean-at-checkpoint report from #86's dirty-at-audit-start observation.
- Reproducibility: preserve the exact command templates in the authenticated traversal audit, including `--paginate` and endpoint paths.
- Scope: only the three evidence documents named by Task 1 change; no acceptance or implementation state is promoted.

---

### Task 1: Correct Task 17 review findings

**Files:**
- Modify: `docs/plans/issue-72-live-tree-refresh-2026-09-30.md`
- Modify: `docs/plans/issue-72-dag.md` (recovery overlay worktree/resource boundary)
- Modify: `docs/plans/issue-72-execution-ledger.md`
- Modify: `docs/plans/issue-72-parallel-refresh-2026-09-29-plan.md` (mark Task 17 review/fix verified only after reviewer approval)
- Create: `docs/plans/issue-72-task17-review-fixes-2026-09-30-plan.md` (this repair plan)

**Interfaces:**
- Consumes: Task 17 reviewer report `.superpowers/sdd/issue-72-parallel-refresh-2026-09-29-plan/task-17-review.md`; traversal audit `/tmp/issue72-live-issue-tree-audit.md`; #82 and #86 audit reports above; existing inventory evidence for #74.
- Produces: corrected source-backed live Issue note, temporal worktree attribution, exact source command templates, and Ledger entry that links the review/fix checkpoint.

- [x] Add a sentence in the live-tree report explaining #74 governance lag: remote Issue #74 was OPEN at the later spot check, while the local inventory retains `done` based on T02 evidence `run_id 3dc51207`, 21/21 assertions, and promoted gating/activation/retries records; remote closure is a separate governance action. Cite the local inventory and make clear that GitHub was not changed.
- [x] Replace the erroneous worktree attribution in DAG §8 with two distinct observations: the #82 audit describes the fixture branch as clean/unintegrated and its integration checkpoint as clean at `8329b85d...`; the #86 audit separately reports that `lago-int` was already dirty at its audit start. Retain the boundary that this refresh did not inspect or modify owner-controlled files.
- [x] Replace ellipsized traversal command descriptions in the report with the exact command templates from `/tmp/issue72-live-issue-tree-audit.md`, including the root fetch, paginated root comments/timeline/sub_issues, child Issue fetch, paginated child sub_issues and timelines.
- [x] Add a Task 17 review-fix entry to the execution Ledger, recording the reviewer findings, fix checkpoint and unchanged gates; update the Task 17 status in the main execution plan only after independent review passes.
- [x] Run `git diff --check`; verify the changed files are limited to the three corrected existing evidence docs plus this repair plan; leave the main Task 17 plan unchanged pending independent review, and confirm the #82/#86/#87 statuses and graph edges did not change.
- [x] Commit the authorized documentation repair with a message beginning `docs(issue-72): correct task 17 evidence attribution`.

**Verification:** Reviewer confirms all three findings are resolved against the named source reports, no contradictory #74 status is implied, the exact command templates match the traversal audit, and the diff is limited to the specified documentation. `git diff --check` exits 0.

**Failure handling:** If any cited source conflicts with the current wording, preserve the disagreement and record the limitation in the Ledger; do not infer a status transition or inspect/modify the dirty `lago-int` worktree.
