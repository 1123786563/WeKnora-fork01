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
| 2 | Transactional end precondition, tenant/id/expected-state Adoption guard shared with CreateVariant, listing/release CAS, and retired-agent query have explicit repository/service concurrency tests and exact signatures | Verified |

## Task status
- Task 1: verified and integrated; versioned/PostgreSQL runtime remains unverified and must remain a stated risk.
- Task 2: verified and integrated. Initial implementation commit `6346ca0ee8e194e87080f858f5ba2f27660a0752`, tenant identity repair `81abe4136d654f8e272ab8531dcb9bbe08624f42`, race serialization repair `4fed833ab28c3846ede6c8a71260e3fcddbd0603`, and concurrent-upgrade conflict repair `1d957470b2032cc876924d469f90ef8ca0fc1668`; integrated HEAD through docs checkpoint `c02c603ac`. Round 1 fixed CreateVariant/EndAdoption interleaving using the shared expected-state row UPDATE guard; independent review found one medium regression in concurrent upgrade acceptance. Round 2 maps that sentinel to `ErrAgentUpgradeStateConflict`; gated test proves an end committed after upgrade prechecks returns conflict and inserts no Variant. Round 2 review package `.superpowers/sdd/plan-t63/task-2-fix-round2-review.patch`, SHA-256 `2a5400cd43df81250bcc9478bfcd07c64bf2fade4cb1b42345f0eb72c41c63dc`; Spec PASS / Quality PASS. Evidence: repository lifecycle tests and focused service suite passed on integration; implementer also reports 5 repeated SQLite interleavings and service `-race` PASS. PostgreSQL runtime is unavailable; lock semantics were verified from guarded UPDATE behavior by reviewer. BASE `db234c5` reproduction of `go test ./internal/application/repository/ -run 'TestDeliveryCollaboration' -count=1` fails the three existing `TestDeliveryCollaboration*` cases at fixture prepare with 400 `code_delivery_unsupported_provider`; this is the same provider fixture gap repaired by reviewed #55 Task1, not a T63 change. Full repository package test still has no fully green result on this branch until that sibling fixture repair is integrated.
- Task 3: ready; all Task 2 interfaces/code are reviewed and integrated at current T63 head.
- Task 4: blocked on T3.
- Task 5: blocked on T2/T3.
- Task 6: blocked on T4/T5.

## Task 2 review and integration checkpoint

- Integration branch: `codex/issue30-b6-t63-cont`; BASE `197794496`, integrated Task 2 HEAD before this documentation update: `c02c603ac`.
- Independent round-1 reviewer confirmed the shared tenant/id/state guarded UPDATE closes the CreateVariant/EndAdoption race under PostgreSQL READ COMMITTED and SQLite; found the upgrade acceptance error mapping regression, fixed in round 2.
- Task 2 integration verification: `go test ./internal/application/repository/ -run 'Test(EndAdoptionRequiresAllVariantsRetiredAndIsTransactional|CreateVariantAndEndAdoptionSerializeOnAdoptionRow|TransitionListingStateIsCAS|DeprecateReleaseIsCASAndPointsAtSuccessor|RetiredVariantAgentExists|AgentMarketplaceLifecycleMigrationColumns)$' -count=1` PASS; `go test ./internal/application/service/ -run 'TestAcceptUpgradeProposalMapsConcurrentAdoptionEndToConflict|TestAgentUpgradeService' -count=1` PASS; `git diff --check` PASS.
- The delivery fixture failures are confirmed on BASE `db234c5` and are outside Task 2 owned files; see task status above.
- External Issue dependency refresh (authenticated GitHub API, 2026-09-29): #59 and #61 remain open with `ready-for-agent`, comments 0, updated `2026-09-20T13:44:21Z` and `2026-09-20T13:44:24Z` respectively. These Issues remain outside #30's declared descendant set only insofar as they are prerequisites to #63; no external code scope is added here.

## Task 3 repair and integration — 2026-09-29

- Task 3 implementation integrated at `254bd83ab`; original review found high/medium race findings and they were addressed in three SDD repair rounds.
- R1 source `0fac377a7`: transactional listing/release guards, shared locks and successor validation. R1 independent review found ordinary AdoptListing guards/writes autocommitted separately and proposal accepted CAS lacked the full downstream lifecycle checks.
- R2 source `d3a35794e`: ordinary AdoptListing now wraps guard+write in one transaction with gated Adopt-vs-Unlist coverage; accepted proposal transition checks listing/release in its transaction; allowed separate draft/accepted CAS behavior retained. Review then found adoption active/from-release state also needed revalidation, storage errors must propagate rather than map to conflict, and existing CAS fixture needed parent rows.
- R3 source `0f91403ab`: accepted proposal CAS now locks/guards tenant listing → target release → active adoption, validates same listing/current from-release before CAS, maps only known lifecycle conflicts and preserves storage errors. Added gated EndAdoption/pointer-advance cases, storage-failure injection, and fixed repository fixture. Round-3 independent review: Spec PASS / Quality PASS, no actionable findings. Patch SHA-256 `6ad05a121d840ccfe8428204cfab3ff065fadc719e598184fbbd3649bab35d87`.
- Implementer evidence on isolated SQLite: repository and service F1/F2 focused suites PASS; targeted `-race` suites for Adopt/Unlist, reciprocal deprecation, proposal/Unlist and other gates PASS; gofmt and source diff-check PASS. PostgreSQL runtime/row-lock behavior remains unverified.
- Task 3 implementation is integrated. Overall Issue #63 remains blocked from Task 4/5 dispatch by declared external dependencies #59 and #61, last refreshed 2026-09-29 as open/ready-for-agent; no code from those Issues was added. Revalidate their status and interfaces before releasing dependent work. `paseo.json` remains unrelated untracked baseline.
