# Independent Review — T14 ctnetlink query-path test fix3

Reviewed 2026-09-29 in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01` at HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`. Scope: the test-only fix3 brief and plan, prior fix2 review findings, approved Craft web artifact spec, ADR 0004, `CONTEXT.md`, and the current fake-socket test and production query/parser seam. No OCR, Docker, discovery, source edit, or issue change was performed.

## Verdict

- **Scoped Spec compliance: PASS.** The three assigned review gaps are closed by the current test. The success fake returns a 20-byte `NLMSG_DONE` with a signed zero status; the assertion compares the entire source/original tuple/reply tuple/state proof; the `NLMSG_ERROR` response raises `RuntimeError` and the failed socket is asserted closed.
- **Scoped code quality: PASS.** The response is keyed to the request sequence, the success and failure paths exercise the production socket query rather than calling the parser directly, and the targeted selector passes. No new finding in this test-only scope.
- **T14 integration acceptance: OPEN.** This fake-socket test does not prove the real renderer-namespace connection survives policy installation, that a fresh same-listener connection is denied with a positive exact drop-counter delta, or that disposable resources are cleaned up. The parent-coordinated live exact-flow check remains required before T14 completion.

## Evidence

1. `tests/test_controller_integration.py:179-181` concatenates a CT record with `struct.pack("=IHHIIi", 20, 3, 0, sequence, 0, 0)`. The `i` field is a signed four-byte zero status. Production `parse_ctnetlink_dump()` accepts that shape and rejects nonzero status (`helper.py:265-270`).
2. `tests/test_controller_integration.py:202-209` compares the entire proof dictionary: `source`, all four fields of each original and reply tuple, and `ESTABLISHED` state. The success socket closure is asserted at line 210.
3. `tests/test_controller_integration.py:175-178,212-218` supplies `NLMSG_ERROR` followed by completion for the same sequence, asserts `RuntimeError` containing `NLMSG_ERROR`, and asserts the failed socket closed. Production rejects `NLMSG_ERROR` (`helper.py:263-264`) and closes in `finally` (`helper.py:335`).
4. Independently run from `deploy/craft/render-boundary/policy-helper`: `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_require_tracked_webdriver_flow_uses_linux_netfilter_protocol -v` — exit 0, one test passed. This review did not repeat the broader validator suite.

## Hashes and limits

- Test file SHA-256: `2576671650710e3fbf86e93206473534354573f7b35c5df93238b67614fb8281`. The fix3 brief records preimage `e4dbd65ad5f7b7ec34c995583740a0958bff3100ce2ea8695059cb39fd54fbb5`; this review verified the current hash, not the unavailable preimage contents.
- Production `helper.py` SHA-256: `b89f1fd93c7368f4e4d9aecfec0dba1e875b72459506fabda261b54eaaff4958`, identical to the expected unchanged hash in the brief.
- Both source files are untracked in this worktree, so `git diff` alone cannot establish the task delta. The verdict is based on current contents, recorded preimage/final hashes, and the targeted behavioral run.
