# Issue #72 Parallel Lane Readiness Review Fixes Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the three actionable independent-review findings in the 2026-09-30 parallel-lane readiness report and preserve the verifier's authoritative evidence.

**Architecture:** Keep the existing lane state model, add the separately owned dirty #84 OCR repair checkpoint, make both evidence links resolve within the documentation worktree, archive the #75 expiry audit, and record the tag verification and the adjudication of the R8 Task 2 report. Do not alter any issue readiness or integrate sibling worktrees.

**Tech Stack:** Markdown, Git metadata, GitHub REST API read-only response.

**Spec:** [`issue-72-parallel-lane-readiness-2026-09-30-plan.md`](issue-72-parallel-lane-readiness-2026-09-30-plan.md), Task 1 independent review `.superpowers/sdd/issue-72-parallel-lane-readiness-2026-09-30-plan/task-1-review.md`, Task 1 report `.superpowers/sdd/issue-72-parallel-lane-readiness-2026-09-30-plan/task-1-report.md`, and source audit `/tmp/issue72-75-expiry-source-audit-20260930.md`.

## Global Constraints

- Preserve the #84 OCR repair worktree's dirty files and all other owner-controlled Worktrees; this task only records their state.
- Keep #82/#86 acceptance gaps and #87 Task 0/runtime gates open.
- Keep R-6 and R-3 settled exactly as approved.
- Do not rewrite R8 Task 2's status: independently verified integration commits exist in the `lago-int` ancestor history even though the supplemental Task 2 worktree HEAD is not itself an ancestor.
- Never claim a review/pin check that has not been run; preserve the failed `git ls-remote` observation and add the succeeding GitHub API ref response with its timestamp.
- No tests, services, remote writes, integration, or changes to any non-owned worktree.

## Review Focus

- #84 ownership: list the clean narrow #84 branch separately from the dirty OCR-repair checkpoint, with distinct paths, SHAs, review states, and write boundaries.
- Links: resolve both the R16 plan and archived #75 audit links from the report's actual docs path.
- Source pin: report the live API result `refs/tags/v1.53.0 → commit 591ae9005110346f1c6034ec72ea9046625668cf`; do not treat HTTP 404 from querying an annotated tag object as relevant because GitHub reports a direct commit ref.
- R8 adjudication: preserve the current report's integrated claim only with exact implementation/ledger commit ancestry; distinguish it from the supplemental report-only branch tip.
- Readiness: no lane or issue status may be released/promoted by these documentation changes.

---

### Task 1: Correct parallel-lane source and ownership findings

**Files:**
- Create: `docs/plans/issue-72-75-expiry-source-audit-2026-09-30.md` (verbatim copy of the saved audit)
- Modify: `docs/plans/issue-72-parallel-lane-readiness-2026-09-30.md`
- Modify: `docs/plans/issue-72-parallel-lane-readiness-2026-09-30-plan.md`
- Modify: `docs/plans/issue-72-execution-ledger.md`
- Modify: `.superpowers/sdd/issue-72-parallel-lane-readiness-2026-09-30-plan/task-1-report.md` (local SDD evidence, not a Git deliverable)
- Create: `docs/plans/issue-72-parallel-lane-review-fixes-2026-09-30-plan.md` (this plan)

**Interfaces:**
- Consumes: Task 1 reviewer report; #84 lane audit from `/root/issue72_live84_lane`; R8 lane audit `/root/issue72_live_r8_lane`; current ref verification `gh api repos/getlago/lago-api/git/ref/tags/v1.53.0` returned `refs/tags/v1.53.0`, type `commit`, SHA `591ae9005110346f1c6034ec72ea9046625668cf`; exact Git ancestry check of R8 implementation/ledger commits to `lago-int` HEAD `8329b85d4dfff301d03f94406dfc829d87cb5b26`.
- Produces: accurate distinct #84 rows, resolving source links, durable #75 expiry report, and ledgered reviewer findings/rulings. No interface or implementation readiness change.

- [ ] Copy the complete #75 expiry source audit to the new `docs/plans` artifact without altering its content.
- [ ] Add a distinct #84 OCR R1 repair row with the dirty worktree path `/Users/wuyongjun/.paseo/worktrees/144ixsa6/codex-issue-72-84-full-review-fix-r1`, branch `codex/issue-72-84-full-review-fix-r1`, HEAD `2a10570cf87cb49aefd938d473709495441fdff8`, exact untracked paths and pending OCR status; retain the clean `issue-84` narrow-repair worktree as a separate row.
- [ ] Change the R16 plan link to the same-repository tracked `docs/plans/issue-72-plan-87-r16.md`; change the #75 report link to the newly archived audit path. Resolve both links from `docs/plans/issue-72-parallel-lane-readiness-2026-09-30.md` and record the link check.
- [ ] Preserve the R8 Task2 row after confirming `485c18a72`, `09f9094c1`, `dba299fab`, and ledger closure `ea76c90fd` are ancestors of integration `8329b85d4`; record that `32aadf55c` is only a supplemental report branch tip and not the integrated code checkpoint.
- [ ] Add the exact GitHub API `v1.53.0` ref response and timestamp to the plan Task1 report/ledger, superseding its earlier unverified `git ls-remote` attempt without deleting that chronology.
- [ ] Record all three reviewer findings, the no-change ruling for R8 Task2, and the repair checkpoint in the execution Ledger; leave the original readiness Task1 status pending reviewer pass until the controller records approval.
- [ ] Run `git diff --check`; verify only the five owned Git documentation paths changed (the SDD report is a local uncommitted artifact), all Markdown links resolve, issue/worktree states were not promoted, and the archived source file matches `/tmp/issue72-75-expiry-source-audit-20260930.md` byte-for-byte.
- [ ] Commit only the five owned paths with a message beginning `docs(issue-72): fix lane readiness evidence links`.

**Verification:** Independent reviewer confirms the #84 rows are distinct and correctly sourced, both links resolve, the #75 report is exact, GitHub's tag ref result is accurately recorded, the R8 Task 2 conclusion matches exact commit ancestry, and no issue readiness/status is changed. `git diff --check` exits 0.

**Failure handling:** If owner/status evidence changes during the repair, retain the prior timestamped snapshot and add a separate new observation. If any R8 commit ancestry check fails, do not retain the integration claim without further evidence. Do not modify or clean the #84 worktree or `lago-int`.
