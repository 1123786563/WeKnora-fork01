# T14 SOCK_DIAG same-port timeout evidence Fix 3 report

## Scope

Implemented only the assigned test change in `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`. No production code, Docker, discovery, staging, or commit was used. The pre-change source was copied from the pinned Fix 2 checkpoint before editing.

## Change

- The same-port validator accepts absent `connect_errno` only for a recorded `TimeoutError`/`timeout`, an explicit 500 ms timeout, and elapsed time in `(0, 1000]` ms. The existing requirements remain: successful bind, attempted but unsuccessful connect, exact client and driver endpoints, positive loopback-drop delta, and exactly zero deltas for both ACK exception counters.
- Positive integer errno denial remains supported.
- The probe retains the exception class and message. Probe subprocess errors, nonzero status, or invalid JSON write a durable `passed: false` receipt with command, status when available, stdout/stderr, monotonic bounds, and error before failing the test.
- Pure tests cover bounded timeout acceptance, positive errno acceptance, missing drop delta, successful connect, unsupported timeout shape, endpoint mismatch, nonzero ACK delta, and durable failure receipts.

## TDD and verification evidence

RED command:

```text
python3 -m unittest test_controller_integration.PolicyTargetCounterUnitTests.test_same_port_probe_accepts_only_bounded_errno_or_timeout_denial test_controller_integration.PolicyTargetCounterUnitTests.test_failed_subprocess_and_invalid_json_receipts_are_durable
```

Result: failed as expected at the newly added timeout acceptance assertion (`same_port_probe_passed(...)` returned false); the failure receipt helper test passed.

GREEN targeted command:

```text
python3 -m unittest test_controller_integration.PolicyTargetCounterUnitTests.test_same_port_probe_validator_requires_bind_denial_and_counter_isolation test_controller_integration.PolicyTargetCounterUnitTests.test_same_port_probe_accepts_only_bounded_errno_or_timeout_denial test_controller_integration.PolicyTargetCounterUnitTests.test_failed_subprocess_and_invalid_json_receipts_are_durable
```

Result: 3 tests passed.

Required no-Docker checks:

```text
python3 -m unittest test_controller_integration.PolicyTargetCounterUnitTests
```

Result: 12 tests passed.

```text
python3 -m py_compile test_controller_integration.py
```

Result: exit 0.

```text
git diff --check
```

Result: exit 0.

The two unittest commands ran from `deploy/craft/render-boundary/policy-helper/tests`. `git diff --check` ran from the worktree root. No live or Docker acceptance is claimed.

## Checkpoint

- Worktree HEAD: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c` (unchanged).
- Before source SHA-256: `a7f47443efbc9132b369b1a5e80848f7351a36d4878df425eb4ef22e28bfca3d`.
- Post source SHA-256: `9c95492af5f2f72c78c1f95f742a82aed5d1e03c3269561be97007586a3c0bf8`.
- Exact before copy, post copy, patch, and hash index are under `.superpowers/sdd/2026-09-23-craft-107-implementation/t14-fix3/`.
- Report SHA-256 is recorded in that checkpoint's `hashes.txt` after the report is finalized.

## Risks and limits

The 1000 ms upper bound allows normal scheduling overhead around the configured 500 ms socket timeout while remaining bounded. Actual OrbStack behavior was not exercised in this assigned no-Docker task.
