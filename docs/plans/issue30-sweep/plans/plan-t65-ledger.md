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

External prerequisite re-audit: #60 and #63 are verified at task/evidence level and integrated at base `93706830b`. The T63 Task2 reviewed patch `512ff27cb` and integrated patch `156f070e0` have identical complete binary-diff hashes. #64 is not complete: its plan contains nine acceptance-bearing tasks, while coordination HEAD contains verified/integrated Tasks 1–6 only; HTTP/container wiring (T7), runtime admission gates (T8), and highest-interface end-to-end evidence (T9) are absent. Therefore #65 remains issue-level blocked and none of its tasks may be marked `verified` or unlock Task4 until #64 T7–T9 pass independent review/validation and are integrated. See the 2026-09-28 gate correction below.

## Task state

| Task | Status | Worktree / branch | Base | Commit / checkpoint | Review | Validation |
|---|---|---|---|---|---|---|
| T1 Release Evaluation persistence | blocked by incomplete #64 predecessor (implementation gate passed; do not treat as Issue-verified) | /Users/wuyongjun/.codex/worktrees/issue30-t65-eval/WeKnora-fork01, detached at dispatch | 93706830b78205de0c7d433097e89f33d9726513 | source `bbc2e89c9`; coordination integration reverted pending gate; evidence retained | PASS after R1; initial findings resolved | PASS at bbc2e89c; raw output retained |
| T2 Publisher Custody and atomic source/adoption gate | blocked by incomplete #64 predecessor (implementation gate passed; do not treat as Issue-verified) | /Users/wuyongjun/.codex/worktrees/issue30-t65-custody/WeKnora-fork01, detached at dispatch | 93706830b78205de0c7d433097e89f33d9726513 | source `e7c42c51b`; coordination integration reverted pending gate; evidence retained | PASS after R1; T2-R1-1 ruled N/A | PASS at e7c42c51b; raw output retained |
| T3 privacy Adoption/Upgrade Metrics | blocked by incomplete #64 predecessor (task code gate passed; do not treat as Issue-verified) | /Users/wuyongjun/.codex/worktrees/issue30-t65-metrics/WeKnora-fork01, detached at dispatch | 30a711f4e27c4ad1208dbfcdc03971ad62b30ce8 | source `8ddee8089`, retained pending gate release | PASS after R1; no new findings | Focused tests/build/diff PASS; repository/service full packages PASS serially at exact HEAD (429.414s / 234.662s); one earlier concurrent attempt hit 10m timeout, likely resource contention but original stack was not retained |
| T4 catalog, Evaluation endpoint and end-to-end HTTP proof | blocked by incomplete #64 predecessor and T1–T3 verification/integration | not created | pending | pending | pending | pending |

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
2. Historical at the time of this plan snapshot: complete #63 before #64. Later evidence records #63 Tasks 1–6 verified/integrated; #64 remains partial until Tasks 7–9 pass.
3. Re-audit #60/#63/#64 evidence and current migration/source base. Only then integrate the reviewed T65 T1–T3 source commits and mark those task nodes verified.
4. Refresh the Task4 brief from the final interfaces and dispatch it only after T1–T3 are verified/integrated. Run real SQLite migrations through the production router fixture.
5. Keep #65 partial/blocked for the error-category portion even if the planned T1–T4 implementation passes.

## 2026-09-28 issue-level gate correction

The initial readiness scan's Task2 blockers were stale. Re-audit shows the T63 Task2 repaired source patch `512ff27cb` and integrated commit `156f070e0` have identical binary diff hashes; B6 records T63 Tasks 1–6 reviewed/validated/integrated. T64 Task2 and Tasks 1–6 are also integrated with recorded review/validation evidence. However, plan-t64.md has nine tasks: Task7 HTTP/container, Task8 runtime admission gates, and Task9 highest-interface end-to-end evidence are absent from coordination HEAD and remain pending. That makes #64, and therefore #65, still blocked. T65 T1/T2 had already passed their own task review/validation and were cherry-picked into the private coordination branch before this full-Issue gate was re-audited. Their source worktrees and evidence are preserved; their production commits were reverted from the coordination branch pending #64 T7–T9. T3 R1 review and validation pass: focused tests/build/diff pass, and repository/service package suites pass serially at exact HEAD. An earlier concurrent two-package run hit its 10-minute timeout; resource contention is the likely explanation, but the old stack was not retained. No production changes were reverted from source worktrees.

This is a scheduling correction, not a scope change. The #63 Task2 provenance gap is closed by exact patch-hash comparison; the earlier scan's #63/#64 Task2 blockers are superseded. T64 Tasks 7–9 remain required by the approved nine-task plan and have no integrated source/tests yet. T65 implementation stays available for later integration while that gate is restored. Cost if this ruling is wrong: unnecessary waiting for T64's highest-interface tasks; the approved plan maps them directly to Issue AC3, so the gate remains.

## 2026-10-02 终局段（subagent 执行轮，branch codex/issue30-t63-closure @ b65b58428）

- **前置**：#63/#64 已在本分支收口（T33 终局段 + #64 8D/8E/Task9 终审 APPROVED），T35 门解除。
- **T1–T3：verified/integrated（恢复）**。三链源提交自对象库恢复（d70fcd073..961faf590，8 提交 + 2 Minor 清理），独立审查（opus）判定与原 review 结论等价（T3 与源终态零差异；T1 仅夹具内联；T2 custody 要素全在场），冲突解决未削弱新基线（8E pin/T33 锁序）语义；迁移实落 versioned 000271 / sqlite 000190（成对无重号）。AB-BA 理论天花板经审查判定支持写路径不可达（注释已点名，agent_marketplace_lifecycle.go:117-123）。
- **T4：verified**。全新实现（b65b58428），独立审查 8/8 checkbox ✅ / Quality Approved（0 Critical/Important；3 Minor 登记：守卫基线归属注记 713→714、marker 循环漏 detail 首快照、DTO 解码失败 500 无 HTTP 直测）。真实 router fixture + 严格 DTO + 隐私红线零违反（not_collected/桶/阈值/无下钻/Reason 缺席全量断言）。
- **issue 级判词：#65 在 error-category=not_collected 边界内 closure-ready**。按既定裁定（2026-09-28），Spec §12 的 error-category 指标在独立 provenance/事件源设计落地前保持 not_collected，#65/#30 不得宣称「全部完成」——此为范围裁定而非实现缺口。AC1 隐私 ✅、AC2 custody 边界 ✅、AC3 真实最高稳定 HTTP ✅。
- **基线腐化披露**：`go test ./...` 于恢复前 HEAD 即有 87 个失败（career/material 等存量），本分支 82（差分自洽：顺带修复 5、新增 0）——全仓绿不可作为本票门槛，登记为跨票 follow-up。
- follow-up（非阻塞）：三 Minor；基线 87 失败腐化清偿；PG 版本化迁移 unrun（无 DSN）；#65 关票与分支合入 owner 裁决。
