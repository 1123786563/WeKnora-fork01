# Issue #72 Parallel Evidence Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist the 2026-09-29 refresh of Issue #72 scope, live agent/resource inventory, #86 evidence provenance, and #87–#89 readiness in a reviewable record without changing product code or the shared integration worktree.

**Architecture:** Add one dated audit note and one append-only execution Ledger entry in this isolated worktree. Separate GitHub containment from dependencies, current-source evidence from historical live evidence, and active sessions from idle or unrelated workspaces. Keep all changes documentation-only and leave the dirty `lago-int` worktree untouched.

**Tech Stack:** Markdown, Git ancestry/tree/blob inspection, authenticated read-only GitHub CLI, process and Paseo inventory already captured during this refresh.

**Spec:** `docs/plans/issue-72-dag.md`, `docs/plans/issue-72-execution-ledger.md`, `docs/plans/issue-72-user-rulings.md`, and approved billing spec `docs/specs/2026-09-20-lago-billing-migration-design.md`; root objective is Issue #72 in `/Users/wuyongjun/.codex/attachments/f2475d5c-b99d-461a-8a7c-2475ec8b0c21/pasted-text-1.txt`.

## Global Constraints

- Preserve the #72 descendant scope; do not add #30 as a child Issue.
- No source code, service configuration, GitHub state, production credentials, or external systems may be changed.
- Do not edit the shared dirty integration worktree at `.worktrees-issue72/lago-int` or any other task's worktree.
- #86 historical live results do not prove current exact-integration-code acceptance unless source hashes and replay bind to the current checkpoint.
- #87 implementation remains gated on verified #86 integration and authenticated isolated pinned Lago v1.53 Task 0 contract evidence.
- #88 and #89 remain blocked until #87 is verified and integrated; no new implementation readiness may be inferred from partial groundwork.
- User ruling R-6 selects the unique active subscription by the latest Published Plan Version's explicit Billable Metric→dimension mapping and fails closed on dimension ambiguity; paid purchase plans continue excluding Usage Charges.
- Do not expose secrets or raw authentication material in audit records.

## Review Focus

- Native Issue containment and child counts must match authenticated API evidence; validate all 33 direct children and exclude #30.
- #86 historical evidence must be tied to exact commits and clearly distinguished from current integration source hashes.
- Running OCR/process claims must bind to a live process observed at the recorded capture time; do not claim completion based on intent or stale sessions.
- Downstream readiness must preserve the explicit #87 and #88 dependency gates.
- Record the dirty integration worktree as owner state; do not stage or commit its files.

## Task DAG

```text
T1 evidence/status note ──> T1 Ledger pointer and exact checkpoint ──> T2 plan traceability correction
```

Only these documentation Tasks are in scope. No application implementation Task is ready under the current #86/#87 gates.

### Task 1: Record Issue #72 parallel evidence refresh

**Dependencies:** None; evidence collection is complete and read-only.

**Owner role:** `mechanical_worker` for the narrow documentation-only synthesis.

**Validator role:** `reviewer` for independent factual, scope, and traceability review.

**Owned files:**

- Create `docs/plans/issue-72-parallel-refresh-2026-09-29.md`.
- Modify `docs/plans/issue-72-execution-ledger.md` by appending a short dated pointer to the audit note.
- Do not modify any other file.

**Consumes:**

- Root Issue #72 authenticated native-subissue audit and current issue graph in `docs/plans/issue-72-dag.md`.
- #86 artifact-origin findings: evidence package commits `c7b696b6a` and `65647cc55`, descendant checkpoint `85fd67f7f8be9d39d7f9f30b9a439524866fcffe`, current integration HEAD `8329b85d4dfff301d03f94406dfc829d87cb5b26`, historical evidence source `dd089662becc43edb5a44dfb6f2a29f5b343ae37`.
- #87 Task 0 read-only audit of the R7 blocked-env result, missing authenticated runtime calls, offline-only Fix4, and script/result hash mismatch.
- #88 and #89 independent readiness audits; both remain predecessor-blocked.
- Current Paseo registry/workspace and process inventory; separate the active unrelated Issue #140 OCR process from the #72 task processes and preserve the shared integration worktree's dirty ownership.

**Produces:**

- One audit note with timestamp, sources, exact relevant SHAs/hashes, evidence classifications, active/idle task inventory, and safe next frontier.
- One Ledger entry linking the note and recording that no #72 implementation agent was active in the inspected agent registry, OCR for Issue #140 was live, no #86/#87 probe/test/Docker command was live, and shared DB/Docker resource ownership remains unresolved.
- #86 status must state: committed historical artifacts are reachable from current integration ancestry and match the artifact hashes; their referenced live source is `dd089...`; current integration code hashes differ, so fresh exact-checkpoint live acceptance is not established.
- #87 Task 0 remains blocked; R6 decisions are already recorded and must not be reopened by the audit.
- #88/#89 remain blocked and no implementation Task is promoted to ready/verified.

**Implementation steps:**

- [ ] Write the audit note using the exact evidence and paths listed above; do not copy credentials or unverifiable claims.
- [ ] Append one dated Ledger entry containing the note path, base/branch/checkpoint, validation commands, and bounded conclusion.
- [ ] Run `git diff --check` in this isolated worktree; expect exit code 0.
- [ ] Verify that `git status --short` contains only the two owned documentation paths.
- [ ] Commit only those two paths with message `docs(issue-72): record parallel evidence refresh`.

**Verification:** `git diff --check` passes; relative links in the audit note point to existing paths; the Ledger diff adds a pointer and status only; `git diff --name-only <BASE>..HEAD` lists exactly the audit note and Ledger.

**Acceptance mapping:** #72 issue-tree scope → authenticated GitHub audit evidence; #86 → ancestry, artifact hashes, and current-source hash comparison; #87 → pinned-v1.53 evidence classification; #88/#89 → issue acceptance and explicit predecessor gates; safe work frontier → current registry, process, and worktree state.

**Failure handling:** If any cited path, hash, or status cannot be independently confirmed from the collected report and current worktree state, omit the claim or label it partial/missing; do not edit another Worktree to resolve a discrepancy.

### Task 2: Correct the plan's source-hash conclusion

**Dependencies:** Task 1 note and Ledger pointer reviewed; the scoped re-review identified an incorrect comparison between SHA-256 file digests and Git blob SHA-1 IDs in this plan's Task 1 deliverable description.

**Owner role:** `mechanical_worker` for the single-sentence documentation correction.

**Validator role:** `reviewer` for independent hash-method and plan-to-deliverable consistency review.

**Owned files:**

- Modify only this plan at `docs/plans/issue-72-parallel-refresh-2026-09-29-plan.md`.
- Do not alter Task 1's reviewed audit note or execution Ledger.

**Consumes:** The Task 1 audit note's verified comparison: `git show <revision>:<path> | shasum -a 256` yields `f1fe9f...` for `lago.go` and `4553d1...` for `lago_credits_order_integration_test.go` at both `dd089662...` and `8329b85...`; the revisions differ in 81 files overall; the README records no fresh live replay at current integration HEAD or stable-identity verification.

**Produces:** The Task 1 description in this plan states that the two representative files match, the revisions differ in 81 files overall, and the absent exact-checkpoint live replay leaves #86 acceptance unverified. It makes no file-drift claim from mismatched hash algorithms.

**Implementation steps:**

- [ ] Replace the false assertion in Task 1's #86 `Produces` bullet with the precise same-method result and replay limitation above.
- [ ] Recompute both historical/current file SHA-256 pairs with `git show <revision>:<path> | shasum -a 256`; expect identical values for each path.
- [ ] Run `git diff --check`; expect exit code 0.
- [ ] Verify the diff changes only this plan and commit with message `docs(issue-72): correct evidence refresh plan wording`.

**Verification:** Both SHA-256 pairs match, `git diff --check` passes, and `git diff --name-only <BASE>..HEAD` lists only this plan file.

**Failure handling:** If either pair no longer matches the recorded value, stop the documentation correction and inspect the exact commit/path; do not infer acceptance from source similarity or ancestry alone.

## Plan self-check

- Spec coverage: this is a status/evidence refresh, not an implementation or acceptance promotion; each audited Issue and the R-6 rulings are explicitly mapped.
- Step clarity: each step writes one or verifies one bounded artifact; Task 2 is a review-driven correction to the Task 1 plan wording.
- Type/interface consistency: not applicable to documentation-only changes.
- Review Focus: every risk is tied to path, hash, process, or dependency checks above.
- Proportion: one documentation task; no production tests are relevant.
