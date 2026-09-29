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
T1 evidence/status note ──> T2 plan traceability correction ──> T3 verification-scope clarification ──> T4 correct Task 1 base range and plan DAG/self-check ──> T5 correct source attribution ──> T6 correct residual failure-handling attribution ──> T7 correct partial OCR findings and preserve its report ──> T8 correct Task 7 residuals and record TTY correlation ──> T9 correct TTY issue mapping after independent review ──> T10 qualify sibling evidence durability and PID/session inference ──> T11 clarify evidence path base and mapping provenance ──> T12 restore Task 8 historical wording ──> T13 restore Task 8 output-history wording ──> T14 durable final review/OCR/status checkpoint ──> T15 refresh final review record and process snapshot ──> T16 qualify unavailable evidence provenance ──> T17 refresh live Issue tree and resolved Rulings
```

These seventeen documentation Tasks are in scope. Task 7 addresses the five findings in the partial OCR record; Task 8 addresses its remaining R7 link/scope residuals and adds the requested read-only TTY/session correlation; Task 9 corrects the issue-level goal mapping using independently reviewed rollout-goal attachments; Task 10 qualifies the sibling evidence durability and PID/session inference; Task 11 clarifies only the path base and evidence provenance wording; Task 12 restores Task 8's implementation steps and verification to their historical checkpoint wording; Task 13 restores its historical `Produces` wording and keeps later audit conclusions in Task 10; Task 14 records the durable whole-branch review disposition, incomplete OCR state, and bounded process snapshot; Task 15 addresses the missing-source record and adds the supplied 23:24:09 snapshot while retaining review/OCR boundaries; Task 16 qualifies unavailable reviewer/process artifacts and records bounded verification without inventing provenance; Task 17 refreshes the native Issue tree and aligns the current gate record with approved R-6/R-3 and current #82/#86 evidence without promoting acceptance. No application implementation Task is ready under the current #82/#86/#87 evidence gates.

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
- Step clarity: each step writes one or verifies one bounded artifact; Task 2 corrects the Task 1 plan's source-hash conclusion, Task 3 scopes its verification to its own plan edit, Task 4 corrects the Task 1 base range and makes the task DAG/self-check traceable, Task 5 attributes report and Git evidence to their respective sources, Task 6 corrects residual failure-handling attribution, Task 7 addresses the five partial OCR findings and records their incomplete coverage, Task 8 closes the R7 link/scope residuals and records bounded TTY/session correlation without acting on processes, Task 9 corrects TTY-to-issue mapping from independently reviewed rollout-goal attachments, Task 10 states that R7 sibling files were staged but absent from HEAD and that PID/session pairs are inferred by timestamps, Task 11 clarifies the refresh-worktree-root path base and separates direct session-to-Issue attachment evidence from inferred PID/session pairing, Task 12 restores Task 8's historical implementation steps and verification while recording chronology, Task 13 restores Task 8's historical `Produces` wording while keeping current audit conclusions in Task 10, Task 14 records the final review/OCR/status checkpoint without promoting incomplete OCR to a pass, Task 15 records the attributed earlier review and supplied process snapshot without asserting unavailable source evidence, Task 16 qualifies those provenance limits and verifies the correction diff, and Task 17 records the live native tree, current remote state, approved #87 rulings, and bounded #82/#86/#87 acceptance state.
- Type/interface consistency: not applicable to documentation-only changes.
- Review Focus: every risk is tied to path, hash, process, or dependency checks above.
- Proportion: seventeen documentation tasks; Tasks 2–6 retain their prior review-driven corrections, Task 7 covers the five findings in the partial OCR record, Task 8 handles residual evidence traceability and read-only process/session correlation, Task 9 handles the mapping correction, Task 10 makes the two evidence-strength corrections, Task 11 tightens only path-base and provenance wording, Task 12 restores Task 8's implementation-step and verification history, Task 13 restores Task 8's output-history wording, Task 14 records durable final review/OCR/process status, Task 15 records the attributed review summary and supplied process snapshot, Task 16 documents missing-source limits and repairs the resulting overstatement, and Task 17 reconciles the refreshed API tree and approved Rulings with existing acceptance gates. No production tests are relevant.

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

**Chronology:** The follow-up corrections to Task 8's issue mapping, evidence durability, PID/session inference, and path base are recorded in Tasks 9–11; those later entries supersede this historical checkpoint wording.

**Failure handling:** If any process is absent or mapping evidence is inconsistent, report only verified fields and leave the unresolved mapping explicit; do not act on or signal any shell, infer child/task ownership, or modify `lago-int`.

### Task 9: Correct TTY-to-issue mapping after independent review

**Dependencies:** Tasks 1–8 are recorded; independent review verified the rollout-goal attachments and found the recorded issue-level mapping incorrect.

**Owner role:** `mechanical_worker` for bounded documentation-only corrections.

**Validator role:** `reviewer` for mapping and scope consistency.

**Owned files:** Modify only this plan, `docs/plans/issue-72-parallel-refresh-2026-09-29.md`, and `docs/plans/issue-72-execution-ledger.md`.

**Consumes:** Verified rollout-goal attachments directly map sessions to Issue goals: session `01a0ebb7-72be-7cb2-a48e-bddb059ec1a5` → Issue #30 (attachment `b10249b8...`); session `01a0ebb8-01aa-7201-9632-1cbe61427b36` → Issue #72 (`f247...`); session `01a0ebbb-3fa1-7b30-9be0-85351aeb9463` → Issue #140 (`6c1...`); session `01a0ebbb-eb35-7453-b985-f9c0ceaef7b7` → Craft #107, supported by its rollout goal and T14 answer. Separately, PID/session pairings (TTY001/PID 78779, TTY002/PID 81039, TTY003/PID 84486, TTY004/PID 88833) are inferred by comparing process and session timestamps; attachments do not establish those pairings.

**Produces:** The audit note and Ledger carry direct session-to-issue goal mappings and remove the claim that two #72 shells share the same goal. PID/session pairs remain timestamp-inferred. They retain that all four processes have repository-root cwd and branch `main`, no child/task or dedicated worktree mapping is established, the inferred #72 TTY002 remains live so no lane release is inferred, and no shell was acted on.

**Implementation steps:**

- [ ] Correct the TTY goal mapping in both the audit note and Ledger from the verified attachment evidence; preserve all process/session/cwd/branch facts and mapping limits.
- [ ] Add Task 9 to this plan's DAG and update the task count and self-check.
- [ ] Run `git diff --check`; verify only this plan, the audit note, and the execution Ledger changed; commit only those paths with `docs(issue-72): correct TTY issue mapping`.

**Verification:** Direct session-to-issue goal mappings and separately timestamp-inferred PID/session pairs appear in the audit note and Ledger; no duplicate-#72-goal assertion remains; retained operational facts remain present. `git diff --check` passes and the changed paths are exactly the three owned Markdown files. No tests apply.

**Failure handling:** If an attachment cannot support the mapping stated above, preserve the discrepancy and stop short of assigning that goal; do not act on any shell or infer child/task/worktree ownership.

### Task 10: Qualify provisional R7 evidence and inferred PID/session correlation

**Dependencies:** Tasks 1–9 are recorded; full-branch review `bdfa6c4..c6feff1` found a Medium durability overstatement for sibling staged files and a Low overstatement of PID/session certainty.

**Owner role:** `mechanical_worker` for bounded documentation-only corrections.

**Validator role:** `reviewer` for evidence-strength and wording consistency.

**Owned files:** Modify only this plan, `docs/plans/issue-72-parallel-refresh-2026-09-29.md`, and `docs/plans/issue-72-execution-ledger.md`.

**Consumes:** Read-only sibling status and hashes: JSON `c7c72e9a3eccb4900fb51ac5c6ed19a2a37a36c98b8f0ab2f29a42e51fc6c7d1`; README `5547afbc7fdd10b606d9080156c2d754c79cd61eecfb2d052618bff9e23b27ca`; sibling HEAD `8329b85d4dfff301d03f94406dfc829d87cb5b26`. `session_meta` timestamps and goal attachments support inferred PID/session pairs and direct session/Issue mapping respectively.

**Produces:** R7 evidence is explicitly provisional/staged at capture, excluded from sibling HEAD, and not claimed as durable; Task 0 remains blocked with the source-loss caveat. PID/session pairing is timestamp-inferred because metadata has no PID/TTY; direct session→Issue goal/attachment mappings are preserved. The current audit note and Ledger conclude that PID/session pairs are inferred, sessions map to the correct issue goals, and there is no duplicate #72 claim; Task 8's historical checkpoint wording is retained as history and superseded by Tasks 9–11. The R7 JSON remains the initial blocked result and README remains the later explicit rerun HTTP000/zero-call record. TTY001 maps #30, TTY002 #72, TTY003 #140, TTY004 Craft #107; all cwd root/main, no lane release, and no shell action.

**Implementation steps:**

- [ ] Replace durable-looking sibling R7 links with literal paths and state exact hashes, staged-at-capture status, exclusion from HEAD `8329...`, and the source-loss caveat.
- [ ] Label every PID/session pair inferred; preserve direct session-to-Issue goal mapping and all bounded operational facts.
- [ ] Add this Task 10 to the DAG/self-check while retaining prior task history.
- [ ] Record full-branch review findings: Medium R7 sibling files not committed, Low PID/session inference; reviewer Spec pass / quality fail pending these corrections.
- [ ] Verify sibling hashes and staged state read-only, run `git diff --check`, confirm only these three documents changed, and commit as `docs(issue-72): qualify provisional evidence and TTY correlation`.

**Verification:** The recorded paths `../lago-int/docs/migrations/lago/t15-admission-pricing-r7-evidence/probe-results-r7-wallet-consumption.json` and `../lago-int/docs/migrations/lago/t15-admission-pricing-r7-evidence/README.md` are relative to the refresh worktree root; supplied sibling file SHA-256 values match and Git reports both as staged additions outside HEAD `8329...`; the changed paths are exactly the plan, audit note, and Ledger; `git diff --check` passes. No tests apply.

**Failure handling:** If either sibling file status/hash differs, record the observed discrepancy and do not claim durable source evidence. Do not change R7 semantics, act on shells, or infer task/worktree ownership.

### Task 11: Clarify evidence path base and mapping provenance

**Dependencies:** Tasks 1–10 are recorded; review found that Task 10's path wording should name the base for the recorded sibling-worktree-relative paths, and Task 9 should distinguish direct attachment mapping from inferred process/session correlation.

**Owner role:** `mechanical_worker` for bounded plan-only wording corrections.

**Validator role:** `reviewer` for path-base and provenance precision.

**Owned files:** Modify only this plan at `docs/plans/issue-72-parallel-refresh-2026-09-29-plan.md`.

**Consumes:** Task 10's recorded R7 paths `../lago-int/docs/migrations/lago/t15-admission-pricing-r7-evidence/probe-results-r7-wallet-consumption.json` and `../lago-int/docs/migrations/lago/t15-admission-pricing-r7-evidence/README.md`, interpreted relative to the refresh worktree root; Task 9's direct rollout-goal attachment/session-to-Issue evidence and timestamp-inferred PID/session pairs.

**Produces:** Task 10 verification explicitly states that each `../lago-int/...` path is relative to the refresh worktree root. Task 9 Consumes explicitly separates direct session-to-Issue evidence from inferred PID/session pairing using process/session timestamps. The DAG and self-check account for Tasks 1–11.

**Implementation steps:**

- [ ] State the refresh worktree root as the base for Task 10's `../lago-int/...` R7 paths.
- [ ] Clarify in Task 9 Consumes that rollout attachments directly map sessions to Issue goals, while PID/session pairings are inferred from process/session timestamps.
- [ ] Add this Task 11 record and update the DAG, task count, and self-check.
- [ ] Run `git diff --check` and verify only this plan changed.
- [ ] Commit only this plan with `docs(issue-72): clarify evidence path base`.

**Verification:** Task 10 names the refresh worktree root as the base for both sibling paths; Task 9 distinguishes direct attachment/session-to-Issue evidence from inferred PID/session pairing; the DAG/self-check list eleven documentation tasks; `git diff --check` passes and only this plan is changed.

**Failure handling:** If either recorded path is not relative to the refresh worktree root or the cited evidence supports a different relationship, preserve the discrepancy and stop before strengthening the wording.

### Task 12: Restore Task 8 historical wording

**Dependencies:** Tasks 1–11 are recorded; whole-branch review found Task 8 had retroactively absorbed later corrections.

**Owner role:** `mechanical_worker` for a plan-only historical wording restoration.

**Validator role:** `reviewer` for comparison against the Task 8 checkpoint.

**Owned files:** Modify only this plan.

**Consumes:** Task 8's Implementation steps and Verification in commit `aad6ae84bf32d7404fc3bb88f794b11f38580e8c`; follow-up corrections recorded in Tasks 9–11.

**Produces:** Task 8 Implementation steps and Verification match their historical checkpoint wording; the chronology note points to Tasks 9–11; the DAG and self-check account for Task 12.

**Implementation steps:**

- [ ] Restore only Task 8's Implementation steps and Verification from the specified checkpoint.
- [ ] Add a short chronology note to Task 8 pointing to follow-up corrections in Tasks 9–11.
- [ ] Add Task 12 to the DAG, self-check, and task count.
- [ ] Run `git diff --check` and verify only this plan changed.
- [ ] Commit only this plan with `docs(issue-72): restore task eight chronology`.

**Verification:** Task 8's Implementation steps and Verification match commit `aad6ae84bf32d7404fc3bb88f794b11f38580e8c`; follow-up corrections remain recorded in Tasks 9–11; DAG and self-check state twelve documentation tasks; `git diff --check` passes and only this plan changed.

### Task 13: Restore Task 8 output-history wording

**Dependencies:** Tasks 1–12 are recorded; whole-branch review found Task 8 `Produces` still folded in later Task 9–11 conclusions.

**Owner role:** `mechanical_worker` for a bounded plan-only wording correction.

**Validator role:** `reviewer` for comparison against the Task 8 checkpoint and current Task 10 conclusions.

**Owned files:** Modify only this plan.

**Consumes:** Task 8 `Produces` from checkpoint `aad6ae84bf32d7404fc3bb88f794b11f38580e8c`; Task 10's current audit-note and Ledger conclusions.

**Produces:** Task 8 `Produces` matches the checkpoint's original one-to-one TTY/PID/session correlation, root cwd/main, issue goal mapping, duplicate live #72 shells, and no-lane-release claims, with the chronology note that Tasks 9–11 supersede that wording. Task 10 explicitly describes the current audit note and Ledger conclusions: PID/session pairing is inferred, sessions map to correct issues, and there is no duplicate #72 claim. DAG and self-check account for thirteen tasks.

**Implementation steps:**

- [ ] Restore Task 8 `Produces` exactly from the specified checkpoint, retaining its chronology note for Tasks 9–11.
- [ ] Clarify Task 10 `Produces` as current audit-note and Ledger conclusions, separate from Task 8 history.
- [ ] Add Task 13 to the DAG, self-check, and task count.
- [ ] Run `git diff --check` and verify only this plan changed.
- [ ] Commit only this plan with `docs(issue-72): preserve task eight output chronology`.

**Verification:** Task 8 `Produces` matches the checkpoint text; Task 10 distinguishes current inferred PID/session and correct issue mappings from historical wording; the DAG and self-check state thirteen documentation tasks; `git diff --check` passes and only this plan changed.

### Task 14: Record durable final review, OCR, and process status checkpoint

**Dependencies:** Tasks 1–13 are recorded; independent whole-branch review is complete for its stated range, while OCR remains incomplete.

**Owner role:** `mechanical_worker` for append-only documentation recording.

**Validator role:** `reviewer` for range, status, and chronology consistency.

**Owned files:** Modify only this plan and `docs/plans/issue-72-execution-ledger.md`.

**Consumes:** Independent review exact range `bdfa6c4bec3aa25c04ac7598418ee8cb222c120f..33841cb48e0b8f1cbac143a0d3be56515648c67b`; partial OCR note `docs/plans/issue-72-parallel-refresh-2026-09-29-ocr-partial.md`; process/session snapshot supplied for 2026-09-29 23:19 CST.

**Produces:** The Ledger records whole-branch Spec PASS / quality PASS with no actionable findings, the four reviewed documentation paths and clean links/diff, prior finding resolution including Task 8 chronology restored by Tasks 12–13, explicit incomplete OCR status and its recorded attempt limits, plus the bounded live-process snapshot and mapping limits. Current R-6 and #87 gating remain as previously recorded.

**Implementation steps:**

- [ ] Append the dated review and OCR disposition to the execution Ledger; do not characterize OCR as complete or full-range.
- [ ] Append the supplied process snapshot, preserving timestamp-inferred PID/session pairing, direct issue/attachment mapping, absent task/worktree mapping, and no lane release; do not stop or alter any process or worktree.
- [ ] Add Task 14 to the DAG, self-check, and task count while preserving Tasks 1–13 chronology.
- [ ] Run `git diff --check`; confirm only this plan and the execution Ledger changed; commit only those two paths as `docs(issue-72): record refresh review checkpoint`.

**Verification:** Recorded review range is exact; the four paths are the plan, audit note, execution Ledger, and partial OCR note; OCR remains explicitly incomplete, with sessions and exact earlier range/coverage linked to the partial note; process identities/status and limitations match the supplied 23:19 CST snapshot. The R-6/#87 gates are unmodified. `git diff --check` passes and only the two owned paths change. No tests apply.

**Failure handling:** If the supplied status conflicts with the durable partial OCR note or the requested process snapshot, preserve the discrepancy without upgrading coverage or acting on external processes.


### Task 15: Persist final review provenance and refreshed process snapshot

**Dependencies:** Tasks 1–14 are recorded; follow-up review found Task 14's review-source wording needed a durable report pointer and supplied a later process snapshot.

**Owner role:** `mechanical_worker` for append-only Markdown recording.

**Validator role:** `reviewer` for source, range, status, and uncertainty consistency.

**Owned files:** Create `docs/plans/issue-72-parallel-refresh-2026-09-29-final-review.md`; modify only this plan and `docs/plans/issue-72-execution-ledger.md`.

**Consumes:** Independent review checkpoint BASE `bdfa6c4bec3aa25c04ac7598418ee8cb222c120f` to HEAD `33841cb48e0b8f1cbac143a0d3be56515648c67b`; reviewer report from `issue72_refresh_full_review`; partial OCR record; supplied read-only snapshot captured 2026-09-29 23:24:09 CST.

**Produces:** A durable report and Ledger pointer record the four-file Spec PASS / quality PASS review with no actionable findings, identify prior resolved issues and clarify that this review predates Task 14; preserve OCR as partial/non-pass with no completed current-range coverage; and capture process/session details with direct versus inferred mappings clearly separated. No changes to R-6/#87 gates or `lago-int`; no process action or lane-release assertion.

**Implementation steps:**

- [ ] Create the final-review report with checkpoint/range, four reviewed paths, prior finding resolution, partial OCR sessions/range/coverage and the supplied 23:24:09 process snapshot.
- [ ] Append the report pointer and bounded review/OCR/process status to the execution Ledger; retain the existing Task 14 entry as historical evidence.
- [ ] Add Task 15 to the DAG, self-check, and task count without removing prior chronology.
- [ ] Run `git diff --check`, confirm exactly the three owned Markdown paths changed, and commit only those files as `docs(issue-72): persist final review and process snapshot`.

**Verification:** Report and Ledger agree on exact independent review range/verdict and its pre-Task-14 scope; OCR remains partial, with original/resumed sessions and `bdfa6c4..78e1b593` range cited; process PIDs, TTYs, elapsed times, cwd, direct issue mappings, inferred PID/session pairing, mapping limits, and no-action status match the supplied snapshot. R-6/#87 gates and shared `lago-int` are unchanged. `git diff --check` passes and only the three owned Markdown paths change. No tests apply.

**Failure handling:** If the supplied review report or process/OCR evidence conflicts with the preserved records, state the discrepancy and do not upgrade OCR coverage or infer task/worktree ownership.


### Task 16: Qualify unavailable reviewer and process evidence

**Dependencies:** Task 15 is recorded; review findings require precise attribution for unavailable source artifacts and command templates.

**Owner role:** `mechanical_worker` for the narrow Markdown correction.

**Validator role:** `reviewer` for provenance language and scope.

**Owned files:** Modify only `docs/plans/issue-72-parallel-refresh-2026-09-29-final-review.md`, `docs/plans/issue-72-execution-ledger.md`, and this plan.

**Consumes:** Task 15 records and the finding that raw reviewer/process artifacts were not located.

**Produces:** The report and Ledger label unavailable reviewer/process observations as attributed/supplied and unverified, describe commands as templates, and disclose that the placeholder rollout path cannot establish provenance. The plan records this bounded repair. No source artifacts are fabricated.

**Implementation steps:**

- [x] Qualify any remaining assertions that imply unavailable process or rollout artifacts were independently inspected.
- [x] Preserve historical values while labeling their evidence limitations.
- [x] Add Task 16 and its result to this plan and the Ledger.
- [x] Run `git diff --check` and confirm only the three owned Markdown paths changed.

**Verification:** `git diff --check` passes; exactly the three owned Markdown paths are modified. Reviewer/process raw artifacts remain unavailable and are explicitly disclosed.

**Failure handling:** If original artifacts become available, compare them against the attributed values and amend only with source-backed corrections; do not infer missing data.


### Task 17: Refresh live Issue tree and resolved Rulings

**Dependencies:** Tasks 1–16 are recorded and reviewed. Current root/direct-child API refresh, #82 frontier audit, #86 acceptance audit, and post-Ruling #87 plan review are available.

**Owner role:** `mechanical_worker` for bounded read-only evidence synthesis.

**Validator role:** `reviewer` for source attribution, current status, DAG, and gate consistency.

**Owned files:** Create `docs/plans/issue-72-live-tree-refresh-2026-09-30.md`; modify only this plan, `docs/plans/issue-72-issues-inventory.md`, `docs/plans/issue-72-dag.md`, and `docs/plans/issue-72-execution-ledger.md`.

**Consumes:** Authenticated recursive Issue audit `/tmp/issue72-live-issue-tree-audit.md` (captured 2026-09-29 23:49 CST); root/#74/child-count checks at 2026-09-30 00:00 CST; #82 frontier audit `/tmp/issue72-82-frontier-audit-20260930.md`; #86 checkpoint audit `/tmp/issue72-86-checkpoint-validator.md`; #87 R16 independent plan review at integration HEAD `8329b85d4dfff301d03f94406dfc829d87cb5b26`; current user R-6/R-3 approvals in `docs/plans/issue-72-user-rulings.md`.

**Produces:** A dated Issue-tree audit records #72's 33 unique native direct children, leaf status, state counts, source commands, #74 governance lag, and reference-versus-dependency cycle adjudication; the inventory links to this audit; the DAG/ledger state that R-6 and R-3 are settled, #82/#86 acceptance gaps remain, #87 implementation is gated, and no downstream status is promoted. It records the reviewed-but-unintegrated #82 T9 harness branch and current dirty `lago-int` owner state without integrating or modifying it.

**Implementation steps:**

- [x] Create the dated issue-tree report with API read times, commands, root metadata, 33 children/34 nodes, no grandchildren, current state counts, timeline-reference limits, and the validated existing inventory/DAG paths.
- [x] Add a concise live-refresh pointer to the inventory without rewriting historical per-Issue analysis or implying GitHub state changed.
- [x] Append a current recovery addendum to DAG §8 that supersedes R-6/R-3 pending language and cites the exact remaining #82/#86/#87 gates; explain why timeline references do not add the apparent `73→77→73` dependency cycle.
- [x] Record the live refresh, R16 plan-review result, #82/#86 audit limits, current worktree/resource observations, and next safe task in the execution Ledger.
- [x] Recount DAG nodes/dependencies/self-check and confirm no acceptance or integration was promoted; run `git diff --check` and verify exactly the four owned existing paths plus the one new report changed.
- [x] Commit only these five documentation paths as `docs(issue-72): refresh live issue and gate status`.

**Verification:** Authenticated API evidence gives 33 unique native direct children and every child has no native children; status summary is 7 closed children / 26 open children plus open #72. The dated report distinguishes the 9/29 full traversal from 9/30 spot checks, shows exact source paths and commands, and treats cross-reference-only edges as non-dependencies absent body/API evidence. Issue inventory, DAG, Ledger, Rulings, and R16 plan agree that R-6 is resolved and R-3 continues to exclude Usage Charges. #82 AC4 and dedicated live T9 remain outstanding; #86 fails full AC despite 110 helper tests; #87 stays blocked by #86 and Lago Task0. `lago-int` remains untouched; no remote writes, service starts, or product tests apply. `git diff --check` passes and only the five owned documentation paths change.

**Failure handling:** If current API results differ from the supplied traversal, rerun recursive pagination before recording; if a source report or reviewed hash differs, preserve the disagreement and do not promote status. Do not infer missing credentials, merge the #82 fixture branch, modify `lago-int`, or alter any running process.
