# Task 8 serial schedule amendment — independent ruling

Scope: read-only comparison of the uncommitted plan amendment and SDD progress ledger with the R6 approved plan review and the active user instruction to dispatch SDD implementation serially. No implementation, requirements, remote issues, or tests were changed or run.

## Verdict

- **Spec compliance: PASS for this schedule amendment.** The plan retains the actual dependency graph: 8A → 8B; 8B → 8C and 8D; 8C → 8E; all three downstream checkpoints → Task9. The Mermaid DAG (plan lines 232–247) does not invent an 8D → 8E product dependency. The R6 approved admission, cancellation, migration, and rollback requirements are untouched by this diff.
- **Dispatch quality: PASS with one minor ledger correction advised.** Plan lines 43–47 and 247 explicitly make controller implementation dispatch 8C → 8D → 8E, with review, backend validation, integration, and affected package checks before the next checkpoint. Ownership remains disjoint in the file map. Review/validation concurrency is limited to frozen commits without shared mutable resources; shared package checks are serialized.

## Finding

| Severity | Evidence / affected text | Impact | Smallest correction |
|---|---|---|---|
| Low — documentation consistency | `.superpowers/sdd/plan-t64-task8-atomic-admission/progress.md:51` says 8E waits for 8B and verified/integrated 8C, but omits the new controller schedule gate that 8D must finish first. The same ledger's ruling at line 39 and plan lines 47/247 require serial 8C → 8D → 8E. | A controller reading only the status table could dispatch 8E while 8D implementation is active, contrary to the user's serial dispatch instruction. The true product DAG is unaffected. | Add “and completion of 8D's review, validation and integration under the serial controller schedule” to the 8E status row; keep the actual DAG unchanged. |

The plan's 8D file-map dependency cell correctly distinguishes its product prerequisites (8A/8B) from its controller dispatch position after 8C. The 8E cell correctly lists only its product prerequisites (8B/8C); the prose supplies the additional schedule constraint. No other contradictory parallel implementation instruction was found in the two reviewed files.

## R1 resolution

**Low finding resolved.** The amended 8E file-map row now says the controller dispatches it after 8D under the serial SDD constraint, and the progress status row requires both 8C and 8D to pass review, validation, and integration before 8E. These lines preserve 8B/8C as 8E's direct interface prerequisites while making the scheduling gate explicit. No tests were run.

## R1 ownership amendment

**Contract and ownership verdict: PASS with one Low documentation gap.** The 8B file-map additions match its stated contracts: `withTenantSecurityGuards` extends `agent_security_guard.go` after 8A's reviewed single-tenant predicate is integrated (shared interfaces and 8B RED → GREEN), while `ResolvePublishedAgentVersion` must be added to `interfaces.AgentSecurityService` in `internal/types/interfaces/agent_security.go` so 8E can consume the immutable snapshot/Release resolver. Existing `AgentVersionSnapshot` is already an interface-layer alias. 8A owns the initial guard implementation; 8B's later extension is sequential. 8C–8E own neither new path, so there is no concurrent file ownership conflict.

**Low — affected plan `### Files` under Checkpoint 8B:** the detailed list still omits both `internal/application/repository/agent_security_guard.go` with its tests and `internal/types/interfaces/agent_security.go`, although the top ownership map includes them. A worker relying on the detailed list could miss authorized edits or have an incomplete review package. Smallest correction: add explicit Modify bullets for those two paths and guard tests to 8B's `### Files` list. This is a plan consistency issue; it does not change the approved behavior or dependency graph. No tests were run.

**R1 closure:** The Checkpoint 8B `### Files` list now explicitly includes `agent_security_guard.go` and its test for the ordered dual-tenant guard, plus `internal/types/interfaces/agent_security.go` for `ResolvePublishedAgentVersion`. Both match the ownership map and stated 8B contracts. The Low consistency finding is resolved. No tests were run.
