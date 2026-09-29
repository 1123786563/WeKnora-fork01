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
T1 evidence/status note ──> T2 plan traceability correction ──> T3 verification-scope clarification ──> T4 correct Task 1 base range and plan DAG/self-check ──> T5 correct source attribution ──> T6 correct residual failure-handling attribution ──> T7 correct partial OCR findings and preserve its report ──> T8 correct Task 7 residuals and record TTY correlation
```

These eight documentation Tasks are in scope. Task 7 addresses the five findings in the partial OCR record; Task 8 addresses its remaining R7 link/scope residuals and adds the requested read-only TTY/session correlation. No application implementation Task is ready under the current #86/#87 gates.

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
- #86 status must state: committed historical artifacts are reachable from current integration ancestry and match the artifact hashes; their referenced live source is `dd089...`; same-method SHA-256 values for `lago.go` and `lago_credits_order_integration_test.go` match at `dd089...` and `8329b85...`, while the revisions differ in 81 files overall. The README records no exact-current-checkpoint live replay or stable-identity verification, so #86 remains acceptance-unverified.
- #87 Task 0 remains blocked; R6 decisions are already recorded and must not be reopened by the audit.
- #88/#89 remain blocked and no implementation Task is promoted to ready/verified.

**Implementation steps:**

- [ ] Write the audit note using the exact evidence and paths listed above; do not copy credentials or unverifiable claims.
- [ ] Append one dated Ledger entry containing the note path, base/branch/checkpoint, validation commands, and bounded conclusion.
- [ ] Run `git diff --check` in this isolated worktree; expect exit code 0.
- [ ] Verify that `git status --short` contains only the two owned documentation paths.
- [ ] Commit only those two paths with message `docs(issue-72): record parallel evidence refresh`.

**Verification:** At Task 1 base `27745734eae97f2f83b8b2ad504d753ed3c8da97` and checkpoint `d2121488f52491add5e3b240b40bf7e30aa2b838`, `git diff --check` passes; relative links in the audit note point to existing paths; the Ledger diff adds a pointer and status only; `git diff --name-only 27745734eae97f2f83b8b2ad504d753ed3c8da97..d2121488f52491add5e3b240b40bf7e30aa2b838` lists exactly the audit note and Ledger. This two-file-only claim is limited to that Task 1 base-to-checkpoint range; it does not include later plan edits or describe Task 3's verification scope.

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

### Task 3: Clarify plan verification scope

**Dependencies:** Tasks 1 and 2 are complete; review identified that Task 1's verification evidence must be scoped to its own checkpoint so later plan edits are not included in that claim.

**Owner role:** `mechanical_worker` for the narrow documentation wording clarification.

**Validator role:** `reviewer` for checkpoint and verification-scope consistency.

**Owned files:**

- Modify only this plan at `docs/plans/issue-72-parallel-refresh-2026-09-29-plan.md`.

**Produces:** Task 1's report supplies base `27745734eae97f2f83b8b2ad504d753ed3c8da97`; Git commit metadata establishes checkpoint `d2121488f52491add5e3b240b40bf7e30aa2b838` as a commit with that parent. Task 1's verification is local to that checkpoint and does not describe later plan edits. Task 3 verifies its own plan edit and makes no claim that only two files changed.

**Implementation steps:**

- [ ] Clarify the scope of Task 1's verification sentence using its recorded base and checkpoint.
- [ ] Run `git diff --check`; expect exit code 0.
- [ ] Verify only this plan changed and commit with message `docs(issue-72): clarify audit plan verification scope`.

**Verification:** The Task 1 verification statement names its base and checkpoint; `git diff --check` passes.

**Failure handling:** If the Task 1 report's base or Git metadata's checkpoint/parent differs from the recorded values, stop and resolve that evidence discrepancy before editing the claim.

### Task 4: Correct Task 1 range and review-task traceability

**Dependencies:** Tasks 1–3 are complete; review found that Task 1's recorded base must match its SDD report, and that the plan DAG and self-check must include Task 3 before this correction.

**Owner role:** `mechanical_worker` for the bounded plan-only corrections.

**Validator role:** `reviewer` for checkpoint-range and task-graph consistency.

**Owned files:**

- Modify only this plan at `docs/plans/issue-72-parallel-refresh-2026-09-29-plan.md`.

**Consumes:** Task 1 report records base `27745734eae97f2f83b8b2ad504d753ed3c8da97`; Git commit metadata establishes checkpoint `d2121488f52491add5e3b240b40bf7e30aa2b838` as a commit whose parent is that base.

**Produces:** Task 1's two-file-only range is explicitly `27745734..d2121488`; Task 3's verification is distinguished from that range claim; the top-level DAG and self-check account for Tasks 1–4.

**Implementation steps:**

- [ ] Correct Task 1's verification base to the report's base and spell out the exact base-to-checkpoint two-file range.
- [ ] Add Task 3 to the top-level DAG and clarify its self-check coverage alongside Task 2.
- [ ] Append this Task 4 record and update the self-check count and explanation.
- [ ] Run `git diff --check`; expect exit code 0.
- [ ] Verify only this plan changed and commit with message `docs(issue-72): correct audit plan task ranges`.

**Verification:** The Task 1 report base and Git-established checkpoint match the stated `27745734..d2121488` range; the DAG orders T1 → T2 → T3 → T4; the self-check names Task 2, Task 3, and Task 4's respective corrections; `git diff --check` passes and only this plan is changed.

**Failure handling:** If the Task 1 report's base or Git metadata's checkpoint/parent cannot be confirmed, stop and resolve the evidence discrepancy before claiming a verified range.

### Task 5: Correct checkpoint evidence source attribution

**Dependencies:** Tasks 1–4 are complete; review found that the Task 1 report provides the base while Git commit metadata provides the checkpoint and parent relationship.

**Owner role:** `mechanical_worker` for the bounded plan-only attribution correction.

**Validator role:** `reviewer` for source attribution and range consistency.

**Owned files:**

- Modify only this plan at `docs/plans/issue-72-parallel-refresh-2026-09-29-plan.md`.

**Consumes:** Task 1 report records base `27745734eae97f2f83b8b2ad504d753ed3c8da97`; `git show -s --format='%H%n%P' d2121488f52491add5e3b240b40bf7e30aa2b838` establishes the checkpoint commit and its parent.

**Produces:** Task 3/Task 4 wording attributes the base to the Task 1 report and the checkpoint/parent relation to Git metadata; Task 1's exact two-file range remains `27745734..d2121488`; the DAG and self-check account for Tasks 1–5.

**Implementation steps:**

- [ ] Correct Task 3 and Task 4 source attribution without changing Task 1's exact two-file range.
- [ ] Append this Task 5 record and update the self-check count and explanation.
- [ ] Run `git diff --check`; expect exit code 0.
- [ ] Verify only this plan changed and commit with message `docs(issue-72): clarify checkpoint evidence source`.

**Verification:** The Task 1 base is attributed to its report; the checkpoint and parent are attributed to Git metadata; the exact `27745734..d2121488` range is unchanged; the DAG orders T1 → T2 → T3 → T4 → T5; `git diff --check` passes and only this plan is changed.

**Failure handling:** If the Task 1 report base or Git checkpoint-parent relationship differs from the recorded values, stop and resolve the evidence discrepancy before editing the attribution.

### Task 6: Correct residual failure-handling source attribution

**Dependencies:** Tasks 1–5 are complete; review found residual Task 3 and Task 4 failure-handling clauses that incorrectly attribute checkpoint evidence to the SDD report.

**Owner role:** `mechanical_worker` for the bounded plan-only consistency correction.

**Validator role:** `reviewer` for source attribution and plan traceability.

**Owned files:**

- Modify only this plan at `docs/plans/issue-72-parallel-refresh-2026-09-29-plan.md`.

**Consumes:** Task 1 report records base `27745734eae97f2f83b8b2ad504d753ed3c8da97`; `git show -s --format='%H%n%P' d2121488f52491add5e3b240b40bf7e30aa2b838` establishes checkpoint `d2121488f52491add5e3b240b40bf7e30aa2b838` and its parent.

**Produces:** Task 3 and Task 4 failure handling consistently identify the Task 1 report as the base source and Git metadata as the checkpoint/parent source; the DAG and self-check account for Tasks 1–6.

**Implementation steps:**

- [ ] Correct the residual Task 3 and Task 4 failure-handling source attribution.
- [ ] Append this Task 6 record and update the self-check count and explanation.
- [ ] Run `git diff --check`; expect exit code 0.
- [ ] Verify only this plan changed and commit with message `docs(issue-72): align verification failure source`.

**Verification:** Task 3 and Task 4 failure handling attribute base evidence to the Task 1 report and checkpoint/parent evidence to Git metadata; the DAG orders T1 → T2 → T3 → T4 → T5 → T6; `git diff --check` passes and only this plan is changed.

**Failure handling:** If the Task 1 report base or Git checkpoint-parent relationship differs from the recorded values, stop and resolve the evidence discrepancy before editing the attribution.

## Plan self-check

- Spec coverage: this is a status/evidence refresh, not an implementation or acceptance promotion; each audited Issue and the R-6 rulings are explicitly mapped.
- Step clarity: each step writes one or verifies one bounded artifact; Task 2 corrects the Task 1 plan's source-hash conclusion, Task 3 scopes its verification to its own plan edit, Task 4 corrects the Task 1 base range and makes the task DAG/self-check traceable, Task 5 attributes report and Git evidence to their respective sources, Task 6 corrects residual failure-handling attribution, Task 7 addresses the five partial OCR findings and records their incomplete coverage, and Task 8 closes the R7 link/scope residuals and records bounded TTY/session correlation without acting on processes.
- Type/interface consistency: not applicable to documentation-only changes.
- Review Focus: every risk is tied to path, hash, process, or dependency checks above.
- Proportion: eight documentation tasks; Tasks 2–6 retain their prior review-driven corrections, Task 7 covers the five findings in the partial OCR record, and Task 8 handles only residual evidence traceability and read-only process/session correlation. No production tests are relevant.

### Task 7: Correct partial OCR documentation findings

**Dependencies:** Tasks 1–6 are complete; the partial OCR report identified five factual/traceability corrections across the audit note and Ledger and requires a durable record.

**Owner role:** `mechanical_worker` for bounded Markdown corrections.

**Validator role:** `reviewer` for exact finding coverage and evidence consistency.

**Owned files:**

- Modify only this plan, the audit note, and the execution Ledger.
- Create `docs/plans/issue-72-parallel-refresh-2026-09-29-ocr-partial.md` as the durable partial-review record.

**Consumes:** `/tmp/issue72-parallel-refresh-ocr-final.txt`; verified refs and ancestry for #86 evidence commits; integration-worktree R7 result/README; approved ruling R-6 and R16 revision hashes; Task 1 checkpoint and captured bounded inventory.

**Produces:** The five partial OCR findings are corrected in the appropriate records; the durable OCR record preserves sessions, exact review range, coverage failure, findings, and non-pass status.

**Implementation steps:**

- [ ] Anchor both #86 evidence commits as ancestors of candidate source checkpoint `85fd67f7...` on `codex/issue-72-r8-runbook` and integration HEAD `8329b85d...` on `codex/issue-72-lago`.
- [ ] Identify R7 result and README paths, scope zero API calls to the R7 rerun after health `000`, distinguish authenticated R2/R6 probes, and preserve offline-only Fix4, hash mismatch, and unverified Task 0 status.
- [ ] Append the R-6 correction with ruling pointer, both R16 commit/hash pairs, and decision-documentation-only limit; preserve the earlier historical text.
- [ ] Add Task 1 checkpoint and the three timestamped inventory observations plus unmapped TTY limitation to the Ledger.
- [ ] Create the durable partial OCR record with exact sessions/range/coverage and all five findings; do not claim OCR pass.
- [ ] Run `git diff --check`; verify only the four owned Markdown paths changed; commit only those paths with `docs(issue-72): correct partial OCR documentation findings`.

**Verification:** Evidence refs, paths and SHA-256 values match the cited records; all five finding summaries appear in the durable partial report; the plan DAG and self-check include Task 7 and state seven documentation tasks; `git diff --check` passes and exactly the four owned Markdown paths are changed. No production tests apply.

**Failure handling:** If a ref/path/hash differs from the verified values, stop and preserve the discrepancy in the partial record rather than asserting the requested fact. The OCR report remains partial unless a separate complete review is run.

### Task 8: Correct Task 7 R7 evidence residuals and record TTY correlation

**Dependencies:** Tasks 1–7 are recorded; the follow-up identified broken R7 links, overbroad zero-call wording, and unresolved root-cwd TTY shells. The requested process/session evidence is read-only and supplied for this bounded correlation.

**Owner role:** `mechanical_worker` for the three-file documentation-only correction.

**Validator role:** `reviewer` for path resolution, evidence scope, and uncertainty wording.

**Owned files:** Modify only this plan, `docs/plans/issue-72-parallel-refresh-2026-09-29.md`, and `docs/plans/issue-72-execution-ledger.md`. Preserve the shared dirty `lago-int` worktree and all earlier Task 1–7 history.

**Consumes:** Existing Task 7 R7 evidence references; sibling integration worktree files `../lago-int/docs/migrations/lago/t15-admission-pricing-r7-evidence/probe-results-r7-wallet-consumption.json` and `README.md`; read-only `ps`, `lsof`, rollout `session_meta`, and user goal/attachment evidence captured 2026-09-29 22:36 CST.

**Produces:** Both R7 links resolve from `docs/plans/`; JSON is described as the initial blocked-env record (`2026-09-28T14:50+08`, `./deploy/lago/lago.sh up`), while the later explicit-isolation rerun and zero-call result are attributed to the exact README. Earlier R2/R6 authenticated calls remain distinct. The audit note and Ledger include one-to-one TTY/PID/session correlation, root cwd/`main`, issue-level goal mapping, duplicate live #72 shells, no lane release, and explicit limits: no child/task or dedicated worktree mapping and no shell action.

**Implementation steps:**

- [ ] Correct both R7 evidence Markdown targets to `../../../lago-int/...` and verify both files exist.
- [ ] Clarify the initial JSON versus later README evidence, and narrow the Ledger zero-call statement to the later explicit-isolation R7 rerun after health HTTP `000`.
- [ ] Add the dated read-only TTY/session correlation section to the audit note and Ledger, preserving the stated mapping limits and no-action boundary.
- [ ] Append Task 8 to this plan DAG and self-check; retain all Tasks 1–7 and their history.
- [ ] Run `git diff --check`; verify only the plan, audit note, and Ledger changed; commit only those paths with `docs(issue-72): clarify TTY mapping and R7 evidence scope`.

**Verification:** `ps`/`lsof` output confirms the four live `codex` PIDs, TTYs, and repo-root cwd; rollout metadata and user goal/attachment evidence support the one-to-one session and issue-level mappings. Both sibling R7 files exist and the corrected Markdown links resolve. The R7 JSON contains the initial start/setup context; README carries the later isolation/HTTP-000/zero-call conclusion. `git diff --check` passes and the only changed paths are this plan, the audit note, and the execution Ledger.

**Failure handling:** If any process is absent or mapping evidence is inconsistent, report only verified fields and leave the unresolved mapping explicit; do not act on or signal any shell, infer child/task ownership, or modify `lago-int`.
