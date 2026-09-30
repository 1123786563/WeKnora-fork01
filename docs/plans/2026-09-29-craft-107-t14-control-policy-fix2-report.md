# T14 Control Policy Fix 2 — Task 1 Report

Date: 2026-09-29 (Asia/Shanghai)
Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
Source review: `docs/plans/2026-09-29-craft-107-t14-control-policy-fix1-review.md`
Status: **DONE_WITH_CONCERNS — both source-level findings repaired and non-Docker checks pass; live T14 acceptance remains blocked/unverified**

## Scope and changes

Only these source files changed:

- `deploy/craft/render-boundary/policy-helper/controller.py`
  - Extracted `same_listener_canary_code` and inserted a newline before the generated compound `if`. Its generated IPv4 and IPv6 programs are both compiled by regression tests.
  - Added `wait_for_reuse_marker`: bounded monotonic deadline, checks renderer liveness on each poll, accepts only the exact expected bytes, and distinguishes renderer exit, deadline without marker, and persistent wrong contents.
- `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
  - Added compile checks for generated IPv4 and IPv6 code.
  - Added success-after-delay, renderer-exit, wrong-content, and timeout checks for marker polling.
  - Disposable test now polls marker and renderer state for at most 2 seconds, then requires exact `b"reuse-ok"`.

Plan and brief: `docs/plans/2026-09-29-craft-107-t14-control-policy-fix2-plan.md` and `.superpowers/sdd/2026-09-29-craft-107-t14-control-policy-fix2-plan/task-1-brief.md`.

## TDD and verification evidence

RED: after adding the tests but before production changes:

`cd deploy/craft/render-boundary/policy-helper && python3 -m unittest tests.test_controller_integration.PolicyControllerBoundedEvidenceTests.test_generated_same_listener_canary_compiles_for_ipv4_and_ipv6 tests.test_controller_integration.PolicyControllerBoundedEvidenceTests.test_reuse_marker_wait_is_bounded_detects_exit_and_requires_exact_bytes`

Result: **failed as expected** with missing `same_listener_canary_code` and `wait_for_reuse_marker` attributes (three subtest/error reports); this confirms tests exercised the absent behavior.

GREEN focused non-Docker suite:

`python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests tests.test_controller_integration.PolicyControllerBoundedEvidenceTests tests.test_controller_integration.PolicyControllerStopFailureTests tests.test_controller_integration.PolicyHelperInvocationUnitTests`

Run from `deploy/craft/render-boundary/policy-helper`. Result: **16 tests passed, 1 Docker-gated test skipped**.

Static checks:

- `python3 -m py_compile controller.py helper.py barrier_adapter.py tests/test_controller_integration.py` from the policy-helper directory — passed.
- `git diff --check -- deploy/craft/render-boundary/policy-helper/controller.py deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` from worktree root — passed.

No Docker command, disposable integration attempt, or immutable probe was run in this task. Fix1's recorded Docker Buildx timeout remains the runtime blocker; the source corrections do not prove live socket reuse, policy denial, counters, conntrack, or the T14 attempt matrix.

## Frozen source evidence

Task baseline hashes (Fix1 reviewed live source):

```text
eb2540a84873bb4a6a98764a4fb6459e6f9b887f46ffd04fabc3ac6e04487add  deploy/craft/render-boundary/policy-helper/controller.py
d3a0b7deb5f9e2a76b4c429ea73ea55ef8ccb9097cc6acd99a387522444a6d77  deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py
```

Final hashes:

```text
9e6840357d22e4f2da1d4f79d52a01f5953991766fe9a30b9433973902f0a439  deploy/craft/render-boundary/policy-helper/controller.py
bc9affe43234b5da09654e3df9f9b7a5867be4662ea4d663736f31988fd36ab0  deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py
```

`fix2.patch` is an exact unified diff from the Fix1 review package's `after/` source bodies to this frozen source; SHA-256 `61a07909877134a3edc2720db1fcf1def02e2427d6297c819b1707c459e5b388`. See `.superpowers/sdd/2026-09-29-craft-107-t14-control-policy-fix2-plan/source-hashes.sha256` and `fix2.patch`. Unrelated existing dirty T14 files were preserved. Nothing was staged or committed.

## Remaining risk

The independent reviewer and validator must assess the frozen source. T14 live acceptance remains **unverified** until the disposable proof can run successfully; no immutable probe may be run before that proof.
