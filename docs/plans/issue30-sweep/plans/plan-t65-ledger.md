# T65 Implementation Ledger

- Plan: [plan-t65.md](plan-t65.md)
- Issue source: [issue-65.md](../issues/issue-65.md)
- Root Issue: #30; see [Issue DAG](../dag.md) and [B6 Ledger](../B6-execution-ledger.md).
- Coordination branch: `codex/issue30-b6-coordination`
- Implementation base: `93706830b78205de0c7d433097e89f33d9726513`
Recorded: 2026-09-28 UTC.

## Task DAG

T1 Evaluation store → T4 catalog HTTP acceptance
T2 Publisher Custody → T4 catalog HTTP acceptance
T3 privacy Metrics → T4 catalog HTTP acceptance

External prerequisite re-audit: #60 remains evidenced in the earlier B6 ledger. The 93706830b base contains #63/#64 Task6 integration evidence, but the full #63 and #64 Issues are not yet verified: B6 readiness scan reports #63 Task2 race-fix still in review/fix and #64 Task2 blocked on that interface. Therefore #65 remains issue-level blocked and none of its tasks may be marked `verified` or unlock Task4 until both predecessor Issues are complete and their reviewed work is integrated. See the 2026-09-28 gate correction below.

## Task state

| Task | Status | Worktree / branch | Base | Commit / checkpoint | Review | Validation |
|---|---|---|---|---|---|---|
| T1 Release Evaluation persistence | blocked by incomplete #63/#64 predecessors (implementation gate passed; do not treat as verified) | /Users/wuyongjun/.codex/worktrees/issue30-t65-eval/WeKnora-fork01, detached at dispatch | 93706830b78205de0c7d433097e89f33d9726513 | source `bbc2e89c9`; coordination integration reverted pending gate; evidence retained | PASS after R1; initial findings resolved | PASS at bbc2e89c; raw output retained |
| T2 Publisher Custody and atomic source/adoption gate | blocked by incomplete #63/#64 predecessors (implementation gate passed; do not treat as verified) | /Users/wuyongjun/.codex/worktrees/issue30-t65-custody/WeKnora-fork01, detached at dispatch | 93706830b78205de0c7d433097e89f33d9726513 | source `e7c42c51b`; coordination integration reverted pending gate; evidence retained | PASS after R1; T2-R1-1 ruled N/A | PASS at e7c42c51b; raw output retained |
| T3 privacy Adoption/Upgrade Metrics | blocked by incomplete #63/#64 predecessors (R1 implementation/review done; full-suite validator concern) | /Users/wuyongjun/.codex/worktrees/issue30-t65-metrics/WeKnora-fork01, detached at dispatch | 30a711f4e27c4ad1208dbfcdc03971ad62b30ce8 | source `8ddee8089`, retained pending gate release | PASS after R1; no new findings | Focused tests/build/diff PASS; independent full package run timed out at ~10m; serial diagnostic rerun requested |
| T4 catalog, Evaluation endpoint and end-to-end HTTP proof | blocked by full #63/#64 predecessors and T1–T3 verification/integration | not created | pending | pending | pending | pending |

## Ownership and scheduling

- T1 owned paths and SDD Brief: .superpowers/sdd/plan-t65/task-1-brief.md in T1 worktree. Report: .superpowers/sdd/plan-t65/task-1-report.md.
- T2 owned paths and SDD Brief: .superpowers/sdd/plan-t65/task-2-brief.md in T2 worktree. Report: .superpowers/sdd/plan-t65/task-2-report.md.
- T1, T2 and T3 run in separate worktrees and have no shared owned files. T1 owns the Evaluation table migrations; T2 and T3 add no schema. T3 reads existing introduction, Adoption and Upgrade Proposal rows only and writes only new metrics files. T4 serializes catalog interfaces, handler, route, container and HTTP fixture changes.
- Every implemented task requires an independent Spec/Quality reviewer and a role-matched behavior validator. A task is integrated only after both pass and source blobs are checked against the reviewed checkpoint.
- Local commits are authorized. Push, remote merge, release, deploy, and GitHub Issue mutation are not authorized.

## Known Spec gap / blocker

Approved Spec §12 requires adopter-level error-category metrics. Research in evidence/t65-preflight/ found no immutable Run→local Version→Variant→Adoption→Release attribution, while raw Run/Task data and error text are content-bearing. The plan deliberately returns not_collected, not fabricated zero or inferred categories. T1–T4 may complete their bounded delivery, but #65 and #30 remain incomplete until a separately designed, reviewed provenance/event source exists or an approved Spec decision changes scope. This known limitation is not a verification pass.

## Task2 review and rulings

- Task2 source commit `5688cfffd70edb0d268c0d85445a4efa5bb4dac0` passed focused independent backend validation. Independent reviewer returned Spec and Quality FAIL: Medium T2-R1-2 (duplicated eligibility query) and Low T2-R1-3 (existing Adoption/provenance preservation assertions incomplete); both are accepted and routed to `plans/plan-t65-task2-review-fix-r1.md`.
- **Ruling T2-R1-1:** the reviewer’s possible public Listing unlist race is not reachable through a supported writer in the current code. The only state transition API is tenant-scoped `AgentMarketplaceRepository.TransitionListingState`; public marketplace repository writes its Listing current Release pointer during approval, not its state. The only public state mutation found is a test-only direct DB seed in `service/public_marketplace_test.go`. Supported source Listing unlisting already acquires the publisher tenant guard, and deterministic validator tests passed. No new public unlisting API is added. Cost if wrong: any overlooked or future public Listing state writer that bypasses the publisher guard could race Introduction; the repair plan records the required guard for a future writer.
- Scoped Task2 repair owns only `internal/application/repository/public_marketplace.go` and `public_marketplace_test.go`. It shares no paths with active T1/T3 streams.

## Task1 review and repair

- Task1 implementation includes commits `ee6b0634c` and enum-conformance correction `0dba37e17`. Independent backend validation passed focused repository and service validation tests, SQLite migration/down-up tests, and diff check at exact HEAD `0dba37e17940acc4c02c4e2f5eede48c7b1bfde1`.
- Independent Spec/Quality review returned Changes Required: Medium T1-R1-1 (invalid-result test's nonexistent Release FK masks result validation), Medium T1-R1-2 (missing second-Release pointer-advance pinning scenario), Low T1-R1-3 (fresh migration test omits the new table’s column assertions). All findings are accepted in `plans/plan-t65-task1-review-fix-r1.md`; repair owns only repository tests and SQLite schema tests.
- Reviewer raised an ⚠️ question whether immutable semantics require DB-level UPDATE/DELETE triggers. **Ruling T1-R1-4:** preserve immutability at the application contract, matching the existing immutable Public Release pattern: Evaluation repositories expose insert/list only, with no update/delete surface. The approved Spec requires immutable Evaluation meaning but does not prescribe privileged-SQL triggers. Cost if wrong: an out-of-band privileged database writer could mutate/delete stored evidence; preventing that would require dialect-specific trigger migrations beyond the current repository pattern.

## Task3 review and repair

- Task3 source commit `0b0057f78dffa4b2db62f0f4cc9de99f27b010a5` independently validated: repository/service Marketplace Metrics tests, `git diff --check`, and `go build ./...` passed at the exact HEAD. Duplicate `-lc++` linker warnings were emitted for cmd/server and cmd/desktop; no failures.
- Independent Spec/Quality review found Spec compliance PASS but Code Quality needs correction: Medium T3-R1-1 (four queries can observe different database snapshots), Low T3-R1-2 (boundary proposals lack matching introductions, so the date predicate is not actually exercised), and Low T3-R1-3 (no same-tenant Adoption/Proposal duplicates and privacy serialization uses a stub). All findings are accepted in `plans/plan-t65-task3-review-fix-r1.md`; fix owns only the three Task3 repository/service files.

## Integration record

| Integrated commit | Change | Validation/review evidence |
|---|---|---|
| 6f8f091d4 | T64 Task6 initial guarded service gate | T64 R1/R2 reports and B6 ledger |
| ed6968dd8 | T64 Task6 R1 atomic repository guard | R1 reports and B6 ledger |
| cde067062 | T64 Task6 R2 direct revocation append guard | evidence/t64-task6-r2/ |
| 93706830b | T65 plan, preflight evidence, and #64 integration base | plan-t65.md, evidence folders; focused T64 integration test passed |

## Next actions

1. Finish already-running T3 R1 validation at its exact source HEAD and archive its review/validation record; do not integrate or dispatch further T65 work before the Issue-level predecessors complete.
2. Complete and integrate the remaining #63 work, then unblock #64 using the verified #63 interfaces; complete and integrate #64.
3. Re-audit #60/#63/#64 evidence and current migration/source base. Only then integrate the reviewed T65 T1–T3 source commits and mark those task nodes verified.
4. Refresh the Task4 brief from the final interfaces and dispatch it only after T1–T3 are verified/integrated. Run real SQLite migrations through the production router fixture.
5. Keep #65 partial/blocked for the error-category portion even if the planned T1–T4 implementation passes.

## 2026-09-28 issue-level gate correction

The B6 readiness scan found that #65 is still blocked by the full #63/#64 Issues: #63 Task2 race-fix and its independent review remain outstanding, and #64 Task2 depends on the stable #63 Adoption/lifecycle interface. Only specific #63/#64 Task6 slices were integrated at the T65 dispatch base; this does not satisfy the Issue dependencies. T65 T1/T2 had already passed their own task review/validation and were cherry-picked into the private coordination branch before this full-Issue gate was re-audited. Their source worktrees and evidence are preserved; their production commits are being removed from the coordination branch pending gate release. T3 R1 review is PASS; focused checks/build/diff passed, while the independent full-package run hit its 10-minute timeout, so a serialized diagnostic rerun is requested. No production changes are being reverted from source worktrees.

This is a scheduling correction, not a scope change. It keeps the T65 implementation available for later integration while restoring the DAG gate. Cost if this ruling is wrong: unnecessary waiting for full Issues instead of their dependency-providing slices; the Issue DAG and B6 ledger currently require full #63/#64 completion, so the stricter gate is retained.
