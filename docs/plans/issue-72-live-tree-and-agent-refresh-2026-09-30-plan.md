# Issue #72 Live Tree and Agent State Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Preserve an independently reproducible 2026-09-30 refresh of the native Issue #72 tree, its current statuses and root retry-safety comment, plus current process/worktree ownership evidence.

**Architecture:** Add a timestamped read-only snapshot that points to the canonical Issue inventory and dependency DAG rather than duplicating requirement text. Append the observation, verification hashes, unresolved owner mapping, and non-promotion ruling to the Issue #72 execution Ledger. Do not edit application code, sibling worktrees, Issues, or services.

**Tech Stack:** Authenticated GitHub REST API, Git worktree/status metadata, process/cwd metadata, Markdown.

**Spec:** Approved migration spec `docs/specs/2026-09-20-lago-billing-migration-design.md`, Issue #72 native tree snapshot `/tmp/issue72-live-tree-refresh-20260930-0121.md`, sanitized process/worktree snapshot `/tmp/issue72-live-session-map-20260930-0121.md`, committed DAG `docs/plans/issue-72-dag.md`, inventory `docs/plans/issue-72-issues-inventory.md`, and prior readiness report `docs/plans/issue-72-parallel-lane-readiness-2026-09-30.md`.

## Global Constraints

- The native containment graph is one root (#72) with 33 direct child Issues (#73–#105), and no discovered grandchildren; containment does not imply execution dependency.
- Current remote status is a timestamped observation; do not infer implementation acceptance or close/reopen any Issue.
- The root comment's retry-safety guidance is attributed context: stable `operationId`, outcomes `CONFIRMED / ABSENT / UNKNOWN`, and authoritative reconciliation before retry after UNKNOWN; do not create a new business ruling.
- Do not map a process to a task/worktree without direct evidence; shell cwd, TTY, clean Git status, or a registered worktree alone does not establish ownership or release.
- Preserve current R-6 and Usage Charge purchase exclusions; #82/#86/#87 acceptance gates remain as recorded in the approved sources.
- No production code, remote writes, services, database operations, changes to sibling worktrees, or process control.

## Review Focus

- Pagination/completeness: root and every discovered child must be checked through the native sub-issues endpoint; report API failures or missing nodes explicitly.
- Evidence provenance: distinguish native containment, body/timeline references, confirmed user decisions, and inferred dependency edges.
- Current status versus acceptance: GitHub OPEN/CLOSED values must remain separate from local implementation/review evidence.
- Process ownership: report only sanitized PID/TTY/cwd and registered worktree facts; explicitly preserve unknown task-to-worktree mappings.
- Snapshot integrity: link the native tree and process reports, and record SHA-256 hashes so later recovery can verify exact evidence.

---

### Task 1: Archive the live tree and process/worktree refresh

**Files:**
- Create: `docs/plans/issue-72-live-tree-and-agent-refresh-2026-09-30-0121.md`
- Modify: `docs/plans/issue-72-execution-ledger.md`
- Modify: this plan's task checkbox.
- Local SDD evidence: `.superpowers/sdd/issue-72-live-tree-and-agent-refresh-2026-09-30-plan/task-1-report.md`

**Interfaces:**
- Consumes: `/tmp/issue72-live-tree-refresh-20260930-0121.md` (complete 34-node audit); `/tmp/issue72-live-session-map-20260930-0121.md` (sanitized process/worktree audit); current remote root/comment/sub-issue responses; current Git worktree/status snapshot.
- Produces: durable timestamped state report linked to canonical inventory/DAG and a Ledger checkpoint that preserves unknown mappings and readiness gates.

- [x] Archive all 33 child IDs, titles, URLs and remote states with parent #72; record zero descendants, complete pagination, 34 unique nodes and 33 containment edges.
- [x] Record root comment `5785844972` as retry-safety context with its URL and timestamp; state that it does not alter the approved Spec, DAG or ruling register.
- [x] Record remote repository identity, root OPEN state, spot-checked child states, API read time and whether comments/timeline changed the execution boundary.
- [x] Record the sanitized live shell/Paseo snapshot and exact #72-related worktree HEAD/branch/status. Keep TTY/task/worktree attribution unknown unless direct evidence exists; preserve dirty/owner-controlled worktrees.
- [x] Link the committed canonical inventory, DAG and previous readiness report; identify current production/live-acceptance blockers without promoting any lane.
- [x] Append the archive path, hashes, review state, exact HEAD range, no-tests rationale and OCR coverage limitation to the execution Ledger.
- [x] Record both `/tmp` input report SHA-256 values in the synthesized snapshot and verify they match the source files; resolve Markdown links, run `git diff --check`, and confirm only the two assigned Git deliverables plus plan checkbox change.
- [x] Commit the snapshot and Ledger as one documentation-only checkpoint.

**Verification:** A read-only reviewer confirms tree counts and states match the captured API report; the root comment is faithfully summarized and linked; process mappings remain appropriately unknown; worktree status is not treated as release; links/hash/diff checks pass; no Issue or implementation status is promoted.

**Failure handling:** If live API or worktree state changes during the task, preserve the supplied observation and add a separate timestamped observation only when re-reading is safely read-only. Do not inspect or change dirty `lago-int`, root checkout, #84 OCR repair, R8 runbook, or any non-owned worktree contents.
