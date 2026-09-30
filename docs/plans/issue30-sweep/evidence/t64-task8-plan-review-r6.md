# T64 Task 8 plan review — round 6

## Scope and verdict

Read-only review of `plans/plan-t64-task8-atomic-admission.md`, parent `plans/plan-t64.md` Task 9 fixture, and `docs/adr/0015-agent-security-transactional-admission.md` after the round 5 revisions. No implementation or tests were run.

- **Spec compliance: PASS for planning.** Exact Version/Release admission, tenant-scoped revocation, durable cancellation, claim fencing, and retained history remain represented in the plan and ADR. The revised rollback rule preserves both AgentRun and AgentQA security gates (`plan-t64-task8-atomic-admission.md:18–23,59–67,199`; `docs/adr/0015-agent-security-transactional-admission.md:32–38`).
- **Dispatch quality: PASS.** No remaining blocking plan finding. Checkpoint dependencies, owned files, interface handoffs, and integration gates are explicit (`plan-t64-task8-atomic-admission.md:37–47,49–67,170–185`). This verdict approves dispatch of implementation checkpoints; it does not attest that their future SQL, code, or tests pass.

## Prior findings rechecked

| Finding | Status | Evidence |
|---|---|---|
| R5-1, unsafe code-only rollback | Resolved | Production rollback is limited to a build retaining **both** exact AgentRun admission and AgentQA claim admission/fencing. Before an older binary starts, adopted AgentQA and Workbench ingress must be closed at the gateway and rejection verified; otherwise the current build stays in service (`plan-t64-task8-atomic-admission.md:22,172–173,195`; ADR 0015:35). |
| R4-1, destructive downgrade with security history | Resolved | Both down paths must refuse claim history, release/dependency revocation history, pending cancellation obligations, or non-NULL Run pins; populated-state refusal is a test requirement (`plan-t64-task8-atomic-admission.md:172–173,195`). |
| R4-2, stale signed preflight or migration gap | Resolved | Old writers are quiesced before final preflight and remain so until commit. The named data owner signs a per-tenant report bound to database identity, schema version, query and result hashes; the operator checks the frozen set at go/no-go. PostgreSQL obtains `ACCESS EXCLUSIVE` locks before the final mapping recheck and holds them through backfill plus INSERT/UPDATE trigger creation (`plan-t64-task8-atomic-admission.md:63,168,203`). The PostgreSQL up **and down** SQL files are explicitly required to wrap their complete bodies in `BEGIN;`/`COMMIT;`, with the down refusal inside that transaction (`:173`). SQLite uses its transactional migration runner (`:63`). |
| R4-3, Task 9 direct Run fixture scan | Resolved | The fixture inserts all four exact `security_*` sidecars and scans them using explicit matching GORM column tags before asserting their values (`plan-t64.md:1747–1756`). |

## Implementation review gates

The SQL files and production changes are still future work. The checkpoint 8B reviewer should inspect the actual PostgreSQL transaction boundaries and lock order, SQLite transactional behavior, backfill/trigger atomicity, and refusal on populated down migrations. The checkpoint 8C–8E reviewers should verify the exact pin and claim paths and gateway rollback procedure against the stated tests. These are verification gates, not unresolved plan blockers.
