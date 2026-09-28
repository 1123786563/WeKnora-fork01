# T65 Implementation Ledger

Plan: [plan-t65.md](plan-t65.md)  
Issue source: [issue-65.md](../issues/issue-65.md)  
Root Issue: #30; see [Issue DAG](../dag.md) and [B6 Ledger](../B6-execution-ledger.md).  
Coordination branch: codex/issue30-b6-coordination  
Implementation base: 93706830b78205de0c7d433097e89f33d9726513  
Recorded: 2026-09-28 UTC.

## Task DAG

T1 Evaluation store → T4 catalog HTTP acceptance
T2 Publisher Custody → T4 catalog HTTP acceptance
T3 privacy Metrics → T4 catalog HTTP acceptance

External verified prerequisites: #60, #63, #64 integrated at base 93706830b. #60/#63 evidence predates this plan in B6 Ledger. #64 Task6 R2 reports are archived under evidence/t64-task6-r2/.

## Task state

| Task | Status | Worktree / branch | Base | Commit / checkpoint | Review | Validation |
|---|---|---|---|---|---|---|
| T1 Release Evaluation persistence | running (integration pending) | /Users/wuyongjun/.codex/worktrees/issue30-t65-eval/WeKnora-fork01, detached at dispatch | 93706830b78205de0c7d433097e89f33d9726513 | bbc2e89c9 | PASS after R1; initial findings resolved | PASS at bbc2e89c9; raw output retained |
| T2 Publisher Custody and atomic source/adoption gate | running (integration pending) | /Users/wuyongjun/.codex/worktrees/issue30-t65-custody/WeKnora-fork01, detached at dispatch | 93706830b78205de0c7d433097e89f33d9726513 | e7c42c51b | PASS after R1; T2-R1-1 ruled N/A | PASS at e7c42c51b; raw output retained |
| T3 privacy Adoption/Upgrade Metrics | fixing | /Users/wuyongjun/.codex/worktrees/issue30-t65-metrics/WeKnora-fork01, detached at dispatch | 30a711f4e27c4ad1208dbfcdc03971ad62b30ce8 | 0b0057f78 (R1 pending) | FAIL R1; fix plan active | focused validator PASS at 0b0057f78 |
| T4 catalog, Evaluation endpoint and end-to-end HTTP proof | pending | not created | depends on verified/integrated T1–T3 | pending | pending | pending |

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

1. Complete T1 and T2 in parallel; review and validate each against its own base/head diff.
2. Integrate only verified source commits into a fresh coordination checkpoint; update both ledgers and plan task status.
3. Dispatch T3 after T1's exact reviewed interface is integrated; it may overlap T2 validation only after another changed-file/fixture conflict scan.
4. Dispatch T4 only after T1–T3 are verified/integrated. Run real SQLite migrations through the production router fixture.
5. Keep #65 partial/blocked for the error-category portion even if the planned T1–T4 implementation passes.
