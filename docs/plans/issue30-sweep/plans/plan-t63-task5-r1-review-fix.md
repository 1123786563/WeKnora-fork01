# T63 Task5 R2 — settle existing pending request when early gate denies

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Narrow follow-up after R1 commit `b4ac07f318d7111ba2f427460e3be81f7ddbf41d`.

**Finding:** T63-T5-R1-F1 MEDIUM: `Start` checks retirement before reading existing request. If a prior process persisted a `pending` request and reservation but crashed before `AgentRunStore.Admit`, a later retry after retirement is rejected at the early gate and leaves the durable intent pending/reservation held forever. Current test explicitly expects pending, so it misses cleanup.

## Global constraints
- Continue in existing T63 isolated worktree at R1 checkpoint; only admission code/tests and any minimal request repo method required for safe state CAS.
- Preserve same-hash dispatching/admitted replay, mismatched-hash conflict, final transaction gate, queue_next 409, retirement serialization.
- No PostgreSQL runtime claim beyond inferred row locking; local commit authorized.

## Task 1 — recover pending retry denial
**Role:** backend_implementer.
1. RED: replace/extend `TestAdmissionReplaysCommittedRunAfterRetirementButGatesPendingRetry` to seed a pending request with an unstarted reservation, retire its Agent, retry same request, expect ErrAgentUseDenied, request transitions to rejected, reservation released once, and no Run/session slot; retries remain stable and do not double-release. Include pending record with no reservation if legal.
2. GREEN: reconcile existing same-hash request before gate. For dispatching/admitted return original Run. For pending, apply the early gate; on denial CAS pending→rejected using stored run/reservation identifiers, release only if this transition wins, then return denial. Reuse final-boundary cleanup helper where possible; never release a reservation another retry owns.
3. Run focused request/race/replay tests, workbench/session/command mapping regressions, build and diff-check. Commit and report exact evidence.

**Review focus:** Every denied pending retry reaches a terminal rejected intent and releases its unstarted reservation exactly once; admitted replay unaffected; no forbidden Run is created.
