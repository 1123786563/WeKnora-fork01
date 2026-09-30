# T63 Task5 R3 — settle only explicit retirement denial

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Narrow follow-up to Task5 R2 commit `7e3a9534e172d69e24407c60b37c9eb43e61bbbc`.

**Finding:** T63-T5-R2-F1 MEDIUM: early settlement currently treats every agent-use gate error as retirement. A transient repository/database lookup error can mark a valid pending intent rejected and release its reservation; a concurrently admitted Run can then disagree with rejected intent.

## Constraints
- Continue in T63 worktree, own only admission coordinator and its agent-use tests unless a compile-required fake file is needed.
- Preserve final AgentRunStore transactional retirement check, explicit AgentID, committed replay, pending cleanup on confirmed retirement, and queue_next 409.
- No external access; local commit authorized.

## Task 1 — distinguish denial from transient errors
1. RED: seed pending request with reservation. Configure gate to return a generic transient error; assert Start returns that error, request remains pending, reservation is not released. Next retry with gate returning `ErrAgentUseDenied`; assert rejected and reservation released once. Keep existing already-admitted replay and confirmed-retirement tests.
2. GREEN: invoke reject/release settlement only when `errors.Is(gateErr, ErrAgentUseDenied)`; return other gate errors without mutating request or reservation. Preserve classification through wrapping.
3. Run focused gate/race/replay tests, relevant request/session/container tests, build and diff-check. Commit, report exact evidence.

**Review focus:** Only a confirmed retirement denial terminalizes pending; temporary lookup failure remains safely retryable.
