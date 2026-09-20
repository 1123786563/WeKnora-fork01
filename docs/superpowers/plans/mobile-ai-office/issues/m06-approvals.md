# M06: Approval compare-and-set

## Parent

Parent #3

## What to build

Present WeKnora approval interactions and let only the Task Owner resolve the exact pending operation through a server-enforced compare-and-set decision.

## Acceptance criteria

- [ ] Approval screens show the run, operation, parameter hash, credential version and expected version supplied by WeKnora.
- [ ] Conflicts replace stale approval data and do not retry a decision automatically.
- [ ] Changed approval fields or credential version require a new server approval.
- [ ] Read-only users cannot resolve approvals.
- [ ] Unknown decision outcomes remain available for reconciliation.

## Blocked by

- M05: Create, continue and recover Run.

## Further Notes

Implementation plan: [M06 Approval compare-and-set](../m06-approvals.md). Not published.


## Plan reference

[独立实施计划](../m06-approvals.md)（发布时替换为固定提交链接）。
