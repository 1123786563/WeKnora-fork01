# T14 ctnetlink query-path review fix3 report

Date: 2026-09-29  
Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`  
Base recorded in brief: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`  
Scope: test-only update to `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`.

## Changes

- The fake query success datagram now contains a 20-byte `NLMSG_DONE` with a four-byte signed zero status.
- The query-path test compares the entire proof object, including source, original tuple, reply tuple, and state.
- The fake query failure response contains an `NLMSG_ERROR` followed by `NLMSG_DONE`, allowing the production query loop to stop and the parser to reject the error; the test asserts the `RuntimeError` and that the socket closed.
- No production source was edited.

## Hashes

- Test file before: `e4dbd65ad5f7b7ec34c995583740a0958bff3100ce2ea8695059cb39fd54fbb5` (from fix3 brief; independently confirmed before editing).
- Test file after: `2576671650710e3fbf86e93206473534354573f7b35c5df93238b67614fb8281`.
- `helper.py` after: `b89f1fd93c7368f4e4d9aecfec0dba1e875b72459506fabda261b54eaaff4958` (matches expected unchanged hash).

## Verification

Commands run from `deploy/craft/render-boundary/policy-helper`:

- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_require_tracked_webdriver_flow_uses_linux_netfilter_protocol -v` — exit 0; 1 test passed.
- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v` — exit 0; 13 tests passed.
- `python3 -m unittest tests.test_barrier_adapter -v` — exit 0; 18 tests passed.
- `python3 -m py_compile tests/test_controller_integration.py` — exit 0.

Whitespace/diff checks from the worktree root:

- `git diff --check` — exit 0; no output.
- `git diff --no-index --check /dev/null deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` — exit 1 because the untracked file differs from `/dev/null`; no whitespace diagnostics were emitted.

An initial targeted run exposed that an error-only datagram keeps the query reader waiting for a completion until its message limit. The fixture was adjusted to carry the rejected `NLMSG_ERROR` and a completion in the same datagram; all listed final selectors then passed. No Docker, discovery, staging, or commit was run.
