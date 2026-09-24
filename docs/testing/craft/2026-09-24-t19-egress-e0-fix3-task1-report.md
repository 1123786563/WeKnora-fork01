# T19 E0 Evidence Fix3 Task 1 report

Date: 2026-09-24 (Asia/Shanghai)  
Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
Plan: `docs/plans/2026-09-24-craft-107-t19-e0-evidence-fix3-plan.md`, Task 1  
Review basis: `docs/plans/2026-09-24-craft-107-t19-e0-fix2-task1-review.md`

## Scope and status

Implemented Fix2 review findings F1, F4 and F5 in the assigned recorder/verifier/test files only. `run_v2.py` and production code were not edited. No Docker run, commit, or E0 verdict change occurred. The measured E0 artifact remains BLOCKED. This checkpoint is ready for independent review; it does not release Fix3 Task 2.

The preimage for all four assigned files is captured in `docs/plans/2026-09-24-craft-107-t19-e0-fix3-task1-preimage/`, with byte counts and SHA256 values in its `index.json`.

## Changes

- The verifier now retains an ordered request list per accepted connection instead of replacing the prior request. It validates generated connection IDs, per-connection request IDs and ordinals, one body record, one terminal response/error for each request, no terminal-after-terminal, one close for each accepted connection, and every request in direct control comparisons. The adapter normal-route check sees all requests, too.
- Recorder request state now tracks request ownership, body observation, completion and terminal state. It rejects unknown/duplicate bodies, duplicate terminals and new keep-alive requests before the previous request reaches a terminal. Closing a connection writes one error terminal for each outstanding request and then records the close.
- Seal now marks the recorder as sealing, releases the recorder lock, stops and joins every TCP listener and waits for handler threads, then checks that no connection/request remains and appends/fsyncs `stream_end`. Shutdown/accept failures poison the recorder. Accepted sockets cannot be silently discarded after the seal.
- The HTTP server uses non-daemon handler threads with `block_on_close`; listener cleanup is idempotent. A recorder accept failure closes the socket and poisons the run. Handler close errors are no longer suppressed by `close_request`.

## RED and verification evidence

RED reproductions against the captured preimage:

- The Fix2 verifier accepts the crafted keep-alive control where one TCP connection carries an unapproved first GET followed by the expected GET (`fix3-keepalive-first-bypass-then-expected-control: forged fixture unexpectedly passed`, verifier verdict PASS). The Fix3 verifier mutation now rejects it.
- The preimage recorder hangs the concurrent accepted-socket-versus-seal regression test; the isolated subprocess hit its 15-second timeout. This reproduces the seal/shutdown lock ordering defect. With the fix, the same regression test completes and proves the accept and close records precede `stream_end`.

Commands and results on the final checkpoint:

- `python3 docs/testing/craft/egress-probe/test_assert_v2.py` — passed. The full same-path synthetic artifact passes; adversarial mutations reject the keep-alive bypass, wrong request ID, wrong ordinal, missing terminal, response-then-error double terminal, plus the existing Fix2 mutations.
- `python3 docs/testing/craft/egress-probe/test_source_owned_v2.py -q` — 14 passed. Includes actual HTTP/1.1 keep-alive GETs on a single connection, wrong body request ID, duplicate terminal rejection, synthesized terminal on close, and a gated concurrent accept/seal proving accept and close precede the final seal.
- `python3 -m py_compile docs/testing/craft/egress-probe/server_v2.py docs/testing/craft/egress-probe/assert_v2.py docs/testing/craft/egress-probe/test_assert_v2.py docs/testing/craft/egress-probe/test_source_owned_v2.py` — passed.
- `git diff --check --` the four assigned files — passed. The files are untracked in this worktree, so whitespace was also checked directly by the checkpoint packaging script.

Synthetic results test the verifier and recorder contract. They are not measured E0 evidence.

## Limits and remaining gates

This task addresses F1/F4/F5 only. Fix2 review findings F2 and F3 remain: the verifier still does not own the full fixed pre/post control schedule or prove actual client stop/helper identity from raw stop/wait/inspect commands; nor does it completely reconstruct argv/environment, config provenance, Docker topology/restart, and cleanup inventory from raw evidence. Those belong to Fix3 Task 2 and remain release blockers. `run_v2.py` still emits the older artifact layout, so no live run should be attempted until the authorized follow-on is reviewed and integrated. OpenCode retry behavior and the previously measured blocked cases also remain unresolved. E0 stays BLOCKED and model egress remains gated.

## Exact checkpoint

Exact pre/post SHA256 values, patch reconstruction evidence, commands, and remaining gates are recorded in `docs/plans/2026-09-24-craft-107-t19-e0-fix3-task1-checkpoint.json`. The patch is `docs/plans/2026-09-24-craft-107-t19-e0-fix3-task1.patch`. Independent review should bind to the checkpoint hashes.
