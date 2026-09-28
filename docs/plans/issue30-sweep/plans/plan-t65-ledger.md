# T65 Implementation Ledger

Plan: [plan-t65.md](plan-t65.md)  
Issue source: [issue-65.md](../issues/issue-65.md)  
Root Issue: #30; see [Issue DAG](../dag.md) and [B6 Ledger](../B6-execution-ledger.md).  
Coordination branch: codex/issue30-b6-coordination  
Implementation base: 93706830b78205de0c7d433097e89f33d9726513  
Recorded: 2026-09-28 UTC.

## Task DAG

T1 Evaluation store → T3 Adoption/Upgrade metrics → T4 catalog HTTP acceptance  
T1 → T4  
T2 Publisher Custody → T4

External verified prerequisites: #60, #63, #64 integrated at base 93706830b. #60/#63 evidence predates this plan in B6 Ledger. #64 Task6 R2 reports are archived under evidence/t64-task6-r2/.

## Task state

| Task | Status | Worktree / branch | Base | Commit / checkpoint | Review | Validation |
|---|---|---|---|---|---|---|
| T1 Release Evaluation persistence | running | /Users/wuyongjun/.codex/worktrees/issue30-t65-eval/WeKnora-fork01, detached at dispatch | 93706830b78205de0c7d433097e89f33d9726513 | pending | pending | pending |
| T2 Publisher Custody and atomic source/adoption gate | running | /Users/wuyongjun/.codex/worktrees/issue30-t65-custody/WeKnora-fork01, detached at dispatch | 93706830b78205de0c7d433097e89f33d9726513 | pending | pending | pending |
| T3 privacy Adoption/Upgrade Metrics | pending | not created | depends on reviewed/integrated T1 contract | pending | pending | pending |
| T4 catalog, Evaluation endpoint and end-to-end HTTP proof | pending | not created | depends on verified/integrated T1–T3 | pending | pending | pending |

## Ownership and scheduling

- T1 owned paths and SDD Brief: .superpowers/sdd/plan-t65/task-1-brief.md in T1 worktree. Report: .superpowers/sdd/plan-t65/task-1-report.md.
- T2 owned paths and SDD Brief: .superpowers/sdd/plan-t65/task-2-brief.md in T2 worktree. Report: .superpowers/sdd/plan-t65/task-2-report.md.
- T1 and T2 run in separate worktrees and have no shared owned files. T1 owns the Evaluation table migrations; T2 adds no schema. T3 waits for T1 interface review/integration and writes only new metrics files. T4 serializes catalog interfaces, handler, route, container and HTTP fixture changes.
- Every implemented task requires an independent Spec/Quality reviewer and a role-matched behavior validator. A task is integrated only after both pass and source blobs are checked against the reviewed checkpoint.
- Local commits are authorized. Push, remote merge, release, deploy, and GitHub Issue mutation are not authorized.

## Known Spec gap / blocker

Approved Spec §12 requires adopter-level error-category metrics. Research in evidence/t65-preflight/ found no immutable Run→local Version→Variant→Adoption→Release attribution, while raw Run/Task data and error text are content-bearing. The plan deliberately returns not_collected, not fabricated zero or inferred categories. T1–T4 may complete their bounded delivery, but #65 and #30 remain incomplete until a separately designed, reviewed provenance/event source exists or an approved Spec decision changes scope. This known limitation is not a verification pass.

## Integration record

| Integrated commit | Change | Validation/review evidence |
|---|---|---|
| 6f8f091d4 | T64 Task6 initial guarded service gate | T64 R1/R2 reports and B6 ledger |
| ed6968dd8 | T64 Task6 R1 atomic repository guard | R1 reports and B6 ledger |
| cde067062 | T64 Task6 R2 direct revocation append guard | evidence/t64-task6-r2/ |
| 93706830b | T65 plan, preflight evidence, and #64 integration base | plan-t65.md, evidence folders; focused T64 integration test passed |

## Next actions

1. Complete T1 and T2 in parallel; review and validate each against its own base/head diff.
2. Integrate only verified source commits into a fresh coordination checkpoint; update both ledgers and plan task status.
3. Dispatch T3 after T1's exact reviewed interface is integrated; it may overlap T2 validation only after another changed-file/fixture conflict scan.
4. Dispatch T4 only after T1–T3 are verified/integrated. Run real SQLite migrations through the production router fixture.
5. Keep #65 partial/blocked for the error-category portion even if the planned T1–T4 implementation passes.
