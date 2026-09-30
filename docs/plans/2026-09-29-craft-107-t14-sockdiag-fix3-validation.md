# T14 SOCK_DIAG same-port timeout evidence Fix 3 validation

## Verdict

**DONE_WITH_CONCERNS** — all assigned static and unit checks pass at the pinned source revision. No acceptance gap was found within the requested no-Docker scope. Live runtime behavior was not tested and is outside this validation brief.

## Scope and revision

- Brief/plan/report: `2026-09-29-craft-107-t14-sockdiag-fix3-{brief,plan,report}.md`.
- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`.
- HEAD: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c` (unchanged during validation).
- Validated source: `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`.
- Pre-source SHA-256: `a7f47443efbc9132b369b1a5e80848f7351a36d4878df425eb4ef22e28bfca3d`.
- Post-source SHA-256: `9c95492af5f2f72c78c1f95f742a82aed5d1e03c3269561be97007586a3c0bf8`, matching the required value.

## Evidence

All commands ran against the same working source hash above. Working source equals the saved post snapshot byte-for-byte (`cmp` exit 0); the saved pre snapshot matches the stated pre-source hash.

| Command | Result |
|---|---|
| `python3 -m unittest test_controller_integration.PolicyTargetCounterUnitTests.test_same_port_probe_validator_requires_bind_denial_and_counter_isolation test_controller_integration.PolicyTargetCounterUnitTests.test_same_port_probe_accepts_only_bounded_errno_or_timeout_denial test_controller_integration.PolicyTargetCounterUnitTests.test_failed_subprocess_and_invalid_json_receipts_are_durable` (from `deploy/craft/render-boundary/policy-helper/tests`) | Exit 0; 3 passed |
| `python3 -m unittest test_controller_integration.PolicyTargetCounterUnitTests` (same directory) | Exit 0; 12 passed |
| `python3 -m py_compile test_controller_integration.py` (same directory) | Exit 0 |
| `git diff --check` (worktree root) | Exit 0 |
| `git apply --reverse --check .superpowers/sdd/2026-09-23-craft-107-implementation/t14-fix3/task.patch` (worktree root) | Exit 0; patch corresponds to the current post image |
| `cmp -s .superpowers/sdd/2026-09-23-craft-107-implementation/t14-fix3/after-test_controller_integration.py deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` (worktree root) | Exit 0; byte-identical |

Checkpoint files verified:

- Before: `.superpowers/sdd/2026-09-23-craft-107-implementation/t14-fix3/before/test_controller_integration.py`
- After: `.superpowers/sdd/2026-09-23-craft-107-implementation/t14-fix3/after-test_controller_integration.py`
- Patch: `.superpowers/sdd/2026-09-23-craft-107-implementation/t14-fix3/task.patch`, SHA-256 `db338477fa87fd25088524261bfff900426177977f878052623e5e9fd33deeb7`

## Acceptance review

- Pure same-port validator preserves positive errno denial and accepts an errno-free timeout only with recognized timeout type, configured 500 ms duration, bounded elapsed time, successful bind, attempted unsuccessful connect, exact endpoints, positive loopback-drop delta, and zero deltas for both ACK exception counters. Focused tests passed.
- Adverse validator cases and subprocess/invalid-JSON durable failed receipt cases are covered by the focused tests and passed.
- All 12 `PolicyTargetCounterUnitTests`, syntax compilation, and whitespace validation passed.
- No acceptance gaps identified for the assigned checks. No Docker/discovery or live behavior was exercised or claimed.

## Risks and limits

The validator intentionally accepts elapsed time through 1000 ms around the configured 500 ms timeout; this validation confirms the unit contract, not runtime timing on the target container platform. The worktree also contains unrelated pre-existing dirty and untracked files; they were not modified. No production API/auth/data/migration behavior is in scope for this test-only task.
