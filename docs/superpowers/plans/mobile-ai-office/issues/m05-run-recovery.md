# M05: Create, continue and recover Run

## Parent

Parent #3

## What to build

Allow a user to create or continue a WeKnora Run within one persistent Task, observe authoritative snapshots and streaming updates, and recover a disrupted request or stream without duplicate execution.

## Acceptance criteria

- [ ] Task session identity is retained across multiple Runs.
- [ ] Create commands use idempotency and unknown outcomes reconcile through lookup before retry.
- [ ] Stream gaps and expired cursors recover from an authoritative snapshot.
- [ ] Scope changes close old streams and prevent stale updates.
- [ ] Unknown side effects remain blocked for user reconciliation.
- [ ] Composer and timeline-detail presentation adapters retain documented Paseo source provenance while consuming only WeKnora projections.

## Blocked by

- M03: Task list, detail and workspace navigation.
- M04: Agent directory, knowledge and attachment selection.

## Further Notes

Implementation plan: [M05 Create, continue and recover Run](../m05-run-recovery.md). Not published.

## Plan reference

[独立实施计划](../m05-run-recovery.md)（发布时替换为固定提交链接）。
