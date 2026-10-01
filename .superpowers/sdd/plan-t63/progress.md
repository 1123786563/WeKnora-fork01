# SDD ledger — plan: docs/plans/issue30-sweep/plans/plan-t63.md

## Recovery facts
- Root: Issue #30; slice Issue #63/T33 Task 2 repository lifecycle primitives.
- BASE: `197794496` (Task 1 reviewed completion metadata integrated on T63 branch).
- Worktree: `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t63-task2`; branch `codex/issue30-b6-t63-task2`.
- Task 1 migration/entity columns are integrated in BASE; reviewer approved Task 1 code and report fixes. Task 2 is ready after T1.
- Owned files from Task Brief: `internal/application/repository/agent_marketplace_lifecycle.go`; interfaces in `agent_adoption.go`, `agent_marketplace.go`; tests in `agent_marketplace_lifecycle_test.go`.
- Brief: `.superpowers/sdd/plan-t63/task-2-brief.md`; report target `.superpowers/sdd/plan-t63/task-2-report.md`.
- Commit strategy: task-local commit authorized; no push/shared merge/deploy/Issue mutation.
- Facts read: `CONTEXT.md`, approved specs `docs/specs/2026-09-20-mobile-ai-office-design.md`, `docs/specs/2026-09-20-mobile-module-seams.md`, T63 Issue snapshot, current T63 plan, Task 1 lifecycle entity/migrations and repository interfaces.

## Preflight conflict scan
| Pair/interface | Check | Ruling |
|---|---|---|
| T1 → T2 | T1 creates lifecycle state columns/entities; T2 repository transaction/CAS methods consume those columns | Task 1 code reviewed/integrated at BASE; ready |
| T2 → T3 | Service methods consume `EndAdoption`, `TransitionListingState`, `DeprecateRelease`, `RetiredVariantAgentExists` and new sentinels | Preserve exact Task 2 signatures; T3 remains blocked until review/integration |
| T2 file ownership | Repository interfaces, one new implementation file, lifecycle repository test; no conflict with Task 5/6 current T63 task files | Safe isolated backend task |
| #63 Task 2 vs #64 | #64 overlaps adoption/router/container lifecycle code | Do not start #64 until #63 integration/review advances the shared seams |

| Task | Self-consistency check | Status |
|---|---|---|
| 2 | Specified transactional end precondition/CAS, listing CAS, release deprecation CAS, tenant scoping and retired-agent query each have explicit repository tests and exact signatures | Ready |

## Task status
- Task 1: verified and integrated; versioned/PostgreSQL runtime remains unverified and must remain a stated risk.
- Task 2: implementation and tenant-identity guard commits remain integrated. Repair round 1 is committed at `4fed833ab28c3846ede6c8a71260e3fcddbd0603` from BASE `3b2429119d4b11a6a5b4d4f15238dd65c0be3118`; plan/report: `.superpowers/sdd/plan-t63/task-2-fix-round1-plan.md`, `.superpowers/sdd/plan-t63/task-2-fix-round1-report.md`. Review Package `.superpowers/sdd/plan-t63/task-2-fix-round1-review.patch` SHA-256 `09f96650133c775d459e9b4209ddb8dd220a7e96a38c33434c3fb7de22871690`. Targeted SQLite/service and race checks pass; repair round 1 independently reviewed PASS. Repair round 2 code commit `1d957470b2032cc876924d469f90ef8ca0fc1668` (BASE `7dd4074323c658d51c3012a87533d260fc1066cb`) is independently reviewed PASS. Round 2 plan/report: `.superpowers/sdd/plan-t63/task-2-fix-round2-plan.md`, `.superpowers/sdd/plan-t63/task-2-fix-round2-report.md`; exact patch SHA-256 `2a5400cd43df81250bcc9478bfcd07c64bf2fade4cb1b42345f0eb72c41c63dc`. Full repository package test was stopped after ~6m20s after two delivery collaboration HTTP test failures; see Task 2 report.
- Task 3: blocked on T2 review/integration.
- Task 4: blocked on T3.
- Task 5: blocked on T2/T3.
- Task 6: blocked on T4/T5.
