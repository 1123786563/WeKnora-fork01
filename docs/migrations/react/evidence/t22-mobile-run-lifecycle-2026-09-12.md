# T22 mobile run-lifecycle follow-up (2026-09-12)

- Commit `2d72d0f` adds a pure mobile run lifecycle with explicit running,
  background-interrupted, stopped, failed, and completed states. The state is
  scoped per session and persisted through SecureStore.
- ChatScreen records starts, assistant IDs, background interruption, user
  stop, failure, and completion. AppState recovery is allowed only for an
  idle/background-interrupted run; a user-stopped run cannot be silently
  resumed. Stop retains the assistant ID and calls the remote stop route when
  one is available.
- Verification passed: mobile tests 71/71, mobile typecheck, and
  `git diff --check`. Deterministic tests cover persistence, stale cleanup,
  stop-after-id, and failed/completed negative recovery.
- Follow-up commit `d884340` adds a lifecycle revision guard around async
  hydration and local transitions, so a late SecureStore read cannot replace a
  newer user stop/failure/completion state. Current mobile verification is
  75/75 with typecheck passing.
- Real provider approval/OAuth, device process-reclaim behavior, server
  continuation after interruption, and remote stop-after-id runtime evidence
  remain open; T22 stays `review`.
