# T14 ctnetlink protocol fallback Task 1 Report

## Scope and baseline

Implemented only Task 1 from `2026-09-29-craft-107-t14-ctnetlink-review-fix2-plan.md` in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`. Owned source files: `deploy/craft/render-boundary/policy-helper/helper.py` and `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`; this report is the requested task artifact. Existing unrelated worktree state was preserved. No staging or commit was performed.

Starting SHA-256 values matched the brief:

- `helper.py`: `e9a36006796055cd11fc26ae37f2d53ea08f7a07e7a2cd56ea29b82649ee1b96`
- `tests/test_controller_integration.py`: `5f2a5907e24b572a0bcb369c92ce8d453caf4a97cfc39db5657a389c0a1e2d41`

## Changes

- Added `_NETLINK_NETFILTER = getattr(socket, "NETLINK_NETFILTER", 12)` beside netlink constants and used it for the ctnetlink socket protocol argument.
- Added a deterministic fake-socket test exercising `require_tracked_webdriver_flow()`. It checks constructor arguments `(AF_NETLINK=16, SOCK_DGRAM, protocol=12)`, destination `(0, 0)`, CT_GET message type and dump request flags, IPv4 nfgen family, nonzero sequence, exact tuple proof, and socket closure.

## TDD and verification

RED:

- Command from `deploy/craft/render-boundary/policy-helper`: `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_require_tracked_webdriver_flow_uses_linux_netfilter_protocol -v`
- Result before production edit: `AttributeError: module 'socket' has no attribute 'NETLINK_NETFILTER'`.
- This host's Python also lacks `AF_NETLINK`, unlike the Alpine runtime described in the brief. To isolate the assigned protocol-constant failure, the fake-socket test supplies the Linux UAPI `AF_NETLINK` value 16 while leaving `NETLINK_NETFILTER` absent. The initial unadjusted test attempt consequently failed first on missing `AF_NETLINK`; that run was not counted as the required RED proof.

GREEN and required checks:

- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_require_tracked_webdriver_flow_uses_linux_netfilter_protocol -v` — 1 passed.
- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v` — 13 passed.
- `python3 -m unittest tests.test_barrier_adapter -v` — 18 passed.
- `python3 -m py_compile helper.py controller.py barrier_adapter.py` — passed (exit 0, no output).
- `git diff --check -- deploy/craft/render-boundary/policy-helper/helper.py deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix2-report.md` — passed (exit 0, no output).
- Both source files are untracked in this worktree, so the tracked-only diff check does not inspect them. `git diff --no-index --check /dev/null <file>` was also run on each: both returned exit 1 for the expected file difference, with no whitespace diagnostics.

No Docker, daemon/image mutation, or unittest discovery was run.

## Final hashes and remaining validation

- `helper.py`: `b89f1fd93c7368f4e4d9aecfec0dba1e875b72459506fabda261b54eaaff4958`
- `tests/test_controller_integration.py`: `e4dbd65ad5f7b7ec34c995583740a0958bff3100ce2ea8695059cb39fd54fbb5`

The fake socket verifies the request path without a kernel netfilter socket. The disposable renderer exact-flow integration remains for parent coordination after independent review and validation. The fix2 brief referenced `2026-09-29-craft-107-t14-ctnetlink-review-fix1-review.md`, but that file was not present in this worktree; the available fix1 plan/report and original ctnetlink review were read.
