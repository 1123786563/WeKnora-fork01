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
- Task 2: implementation checkpoint `6346ca0ee8e194e87080f858f5ba2f27660a0752` and tenant-identity guard fix `81abe4136d654f8e272ab8531dcb9bbe08624f42` committed; targeted lifecycle tests pass. Review Package `.superpowers/sdd/plan-t63/review-package-task-2-81abe4136.patch` SHA-256 `357b2d74e3beaf66ecab87197882638061d2019745a5503262864f13f85e9c0f`. Independent review pending. Full repository package test was stopped after ~6m20s after two delivery collaboration HTTP test failures; see Task 2 report.
- Task 3: blocked on T2 review/integration.
- Task 4: blocked on T3.
- Task 5: blocked on T2/T3.
- Task 6: blocked on T4/T5.
