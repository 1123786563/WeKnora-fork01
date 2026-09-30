# T14 Control Policy Fix 3 — Task 1 Report

Date: 2026-09-29 (Asia/Shanghai)
Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
Finding source: `docs/plans/2026-09-29-craft-107-t14-control-policy-fix2-review.md`
Status: **DONE_WITH_CONCERNS — marker wait callbacks now honor a shared deadline; live T14 acceptance remains blocked/unverified**

## Change

Only `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` changed. Both marker `docker exec` and renderer `docker inspect` callback invocations use `run_before_deadline`, which computes remaining time from a shared monotonic deadline and forwards it as subprocess `timeout`. It raises when a callback finishes at/after the deadline. The bounded polling loop checks its deadline after liveness and marker callbacks and before accepting the expected bytes; it preserves renderer-exit handling and exact `b"reuse-ok"` comparison.

Plan/brief: `docs/plans/2026-09-29-craft-107-t14-control-policy-fix3-plan.md` and `.superpowers/sdd/2026-09-29-craft-107-t14-control-policy-fix3-plan/task-1-brief.md`.

## TDD and verification

RED after adding the regression but before `run_before_deadline` existed:

`cd deploy/craft/render-boundary/policy-helper && python3 -m unittest tests.test_controller_integration.PolicyControllerBoundedEvidenceTests.test_docker_marker_callbacks_receive_remaining_budget_and_reject_late_result`

Result: failed as expected with `NameError: name 'run_before_deadline' is not defined`.

GREEN focused tests:

`python3 -m unittest tests.test_controller_integration.PolicyControllerBoundedEvidenceTests.test_docker_marker_callbacks_receive_remaining_budget_and_reject_late_result tests.test_controller_integration.PolicyControllerBoundedEvidenceTests.test_reuse_marker_wait_is_bounded_detects_exit_and_requires_exact_bytes`

Result: **2 passed**. The stub runner verified that `.25` seconds remaining is forwarded to the subprocess and that a result returned after the deadline is rejected. A second fake-clock test confirms the polling loop does not accept the expected marker returned after its deadline.

Full focused non-Docker suite:

`python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests tests.test_controller_integration.PolicyControllerBoundedEvidenceTests tests.test_controller_integration.PolicyControllerStopFailureTests tests.test_controller_integration.PolicyHelperInvocationUnitTests`

Run from `deploy/craft/render-boundary/policy-helper`. Result: **17 passed, 1 Docker-gated test skipped**.

Static checks:

- `python3 -m py_compile controller.py helper.py barrier_adapter.py tests/test_controller_integration.py` from the policy-helper directory — passed.
- `git diff --check -- deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` from worktree root — passed.

No Docker command or immutable probe was run. T14 live socket reuse, fresh same-listener denial/counter, conntrack behavior, and the attempt matrix remain unverified due to the previously recorded Docker Buildx timeout.

## Frozen evidence

Baseline hash from Fix2 reviewed snapshot: `bc9affe43234b5da09654e3df9f9b7a5867be4662ea4d663736f31988fd36ab0` for `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`.

Final hash: `0f9f69699b5da7ef6bab7437497c666fb0988bb5d2dda082811f887cc7d8b68f` for the same file. Exact Fix3 unified diff is `.superpowers/sdd/2026-09-29-craft-107-t14-control-policy-fix3-plan/fix3.patch`, SHA-256 `2f6d1d3f5d6c2afd1efac795201ed2a1e892d6345568bf8d75bf231deaf4358d`; preimage reconstructed from the frozen Fix1 review package plus exact Fix2 patch.

Unrelated worktree changes were preserved. No files were staged or committed.
