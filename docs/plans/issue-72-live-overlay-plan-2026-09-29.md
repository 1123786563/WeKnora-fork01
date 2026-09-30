# Issue #72 Live Execution Overlay Plan — 2026-09-29

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist a timestamped, evidence-linked status overlay that reconciles the latest #72 live workspace/resource inventory with the existing Issue DAG and execution Ledger.

**Architecture:** Create one append-only overlay note. It complements the existing historical DAG/Ledger without rewriting their accumulated history. The note separates directly observed facts from scheduling conclusions and identifies the evidence/hash used for each.

**Tech Stack:** Markdown; shell hash/status snapshots; no runtime calls.

**Spec:** `docs/plans/issue-72-dag.md`; `docs/plans/issue-72-execution-ledger.md`; `docs/plans/issue-72-issues-inventory.md`; the approved Lago billing Spec and ADR-0012; current #87 R-6 and R16 amendment records.

## Global Constraints

- The authenticated Issue #72 tree remains 33 direct children (#73–#105), with #30 excluded.
- User-approved R-6 is binding: published-version explicit metric→dimension mapping, unique active owner, dimension-scoped fail closed on overlap/duplicates, and no Usage-Charge paid purchase path.
- Existing integration worktree changes are user work; do not overwrite, reset, clean, or stash them.
- No production/database/Docker task starts while the required PostgreSQL and authenticated isolated Lago gates are unavailable.
- Local records distinguish code integration from issue acceptance and from runtime evidence.

## Review Focus

- #87 is not marked ready merely because R-6 is resolved; #86 acceptance and Lago Task 0 remain separate gates.
- A clean dedicated worktree or old commit does not prove a lane is released; use current process/session state.
- A live shared PostgreSQL container is not an isolated R7/R8 test database.
- Task2 source hashes and the exact #72 integration commits must match before calling R8 Task2 integrated.
- Do not record credentials, email addresses, or unrelated private terminal output in the repository note.

---

### Task 1: Record the live ownership and dispatch overlay

**Files:**
- Create: `docs/plans/issue-72-live-execution-overlay-2026-09-29.md`

**Interfaces:**
- Consumes: Current root execution Ledger/DAG snapshot at observed integration HEAD `8329b85d4dfff301d03f94406dfc829d87cb5b26`; the verified 2026-09-29 Paseo terminal/workspace inventory; R6 commit/review records; R16 amendment/review; current root and Task2 branch source hashes.
- Produces: A timestamped overlay that names the active #72 gates, confirms or rejects candidate parallel lanes using current evidence, and links exact reports/commits without claiming completion beyond that evidence.

- [x] Record the integration branch, HEAD, pre-existing staged/unstaged/untracked scope, and the R-6/R16 local commits/reviews; say the R6/R16 content was transferred as a path-scoped patch because the existing staged index prevented cherry-pick, and do not say it is committed on the integration branch.
- [x] Summarize live terminal mapping without identifiers or secrets: the four Codex terminals are #30, this #72 task, #140, and Craft #107; the active Claude terminal is Vue/React parity. State that no extra #72 implementation writer was found in these sessions.
- [x] Record the resource evidence: shared WeKnora PostgreSQL/Redis/docreader are running, `SAAS_TEST_PG_DSN` is unset, no isolated PostgreSQL toolchain/DSN is available, and Craft #107 has not explicitly released the shared resource. Do not assert a Craft process is currently using PostgreSQL.
- [x] Verify and record R8 Task2 integration commits (`485c18a72`, `09f9094c1`, `dba299fab`) and matching SHA-256 values for fulfillment service/test, handler/test, route guard, and both migration files against its independently reviewed branch. State that this is Task2 code verification, not whole #84 acceptance.
- [x] Record that #86 source/repair commits are in the integration history but #86 acceptance remains blocked on its isolated live evidence; #87 remains blocked on #86 acceptance and authenticated isolated pinned Lago v1.53 Task0; #88/#89 remain predecessor-blocked. Mark the two R-6 business choices settled and preserve `ensureNoCharges`.
- [x] State the safe next frontier: finish review/integration of these docs and continue only isolated read-only audits until the resource/acceptance gates change; do not dispatch another implementation or touch the shared database based on shell count or stale worktree status.
- [x] Run `git diff --check`; verify the note separates observed facts from inference and includes no credentials or unrelated user data; commit only this note and the plan.

**Acceptance:** The overlay is date-stamped, links to the authoritative evidence, reconciles the newly approved rulings, marks current blockers accurately, and leaves pre-existing integration changes untouched.

**Failure handling:** If an item cannot be verified from the supplied snapshots, mark it unknown and preserve the gate; do not infer that a process ended or a lane was released.
