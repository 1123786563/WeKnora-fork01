# T63 Task3 Review Fix R2 — public conflict mapping and guard-first introduction

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Narrow follow-up to `plan-t63-task3-review-fix.md`, Task1 checkpoint `151ee5eddd61df4b377f39bd6754f70defc03493`.

**Goal:** Make ended-adoption conflicts consistently surface as public 409 and ensure `IntroduceRelease` acquires the tenant/listing guard before introduction reads or writes.

**Findings:** T63-T3-R1-F1 MEDIUM: public AdoptPublicListing propagates repository ErrAgentAdoptionTransition raw, yielding HTTP 500 instead of conflict. F2 MEDIUM: IntroduceRelease reads/inserts introduction before tenant/listing scope guard, allowing EndAdoption interleaving/SQLite snapshot failure rather than deterministic transition result. F3 LOW: no ended winner unique-conflict-loser test.

## Global Constraints
- Continue the existing isolated T63 worktree on Task1 repair chain; coordinate with currently pending downstream plan.
- Own only the smallest relevant public handler/service/repository tests needed to address these findings; do not modify unrelated Task3 Task2 release lifecycle files.
- Preserve transaction rollback of introduced row and existing active-adoption behavior. Local commit authorized; no live services or external changes.

## Task 1 — guard-first public introduction and public conflict mapping
**Role:** backend_implementer. Independent reviewer + backend validator afterward.
1. RED: public handler test expects 409 for ended-adoption Adopt; IntroduceRelease schedule test must demonstrate guard runs before introduction read/write and ended state returns mapped transition without leaked row. Add ended winner unique-loser coverage if the existing deterministic helper can exercise it without unrelated refactor.
2. GREEN: map `ErrAgentAdoptionTransition` to established public conflict error in handler/service boundary. Reorder IntroduceRelease transaction so tenant/listing scoped writer guard is the first relevant DB operation before introduction lookup/insert, and use common ended-state error path.
3. Run focused handler/repository/service tests and diff-check; inspect atomic rollback and guard order. Commit only owned files, report exact evidence/limits.

**Review focus:** HTTP status is 409; guard precedes reads/writes in transaction; ended outcome deterministic; rollback leaves no introduction; active introduction works; unique-loser is covered or explicitly shown infeasible.
