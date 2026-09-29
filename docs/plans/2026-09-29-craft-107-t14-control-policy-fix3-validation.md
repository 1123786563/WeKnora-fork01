# T14 Control Policy Fix 3 — Independent Validation

Date: 2026-09-29 (Asia/Shanghai)
Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
Task revision (HEAD): `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
Scope: validate the frozen Fix3 report/source identity and its focused non-Docker checks. No source or test files were modified. No Docker/runtime checks were run.

## Frozen identity

The Fix3 report says only `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` changed.

Commands from the worktree root:

- `sha256sum deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
  - matched report final SHA-256: `0f9f69699b5da7ef6bab7437497c666fb0988bb5d2dda082811f887cc7d8b68f`
- `sha256sum .superpowers/sdd/2026-09-29-craft-107-t14-control-policy-fix3-plan/fix3.patch`
  - matched report SHA-256: `2f6d1d3f5d6c2afd1efac795201ed2a1e892d6345568bf8d75bf231deaf4358d`
- `git rev-parse HEAD`
  - `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`

## Checks

1. From `deploy/craft/render-boundary/policy-helper`, ran:
   `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests tests.test_controller_integration.PolicyControllerBoundedEvidenceTests tests.test_controller_integration.PolicyControllerStopFailureTests tests.test_controller_integration.PolicyHelperInvocationUnitTests`
   Result: **17 passed, 1 Docker-gated test skipped** (`Ran 17 tests ... OK (skipped=1)`).
2. From `deploy/craft/render-boundary/policy-helper`, ran:
   `python3 -m py_compile controller.py helper.py barrier_adapter.py tests/test_controller_integration.py`
   Result: exit 0.
3. From worktree root, ran:
   `git diff --check -- deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
   Result: exit 0. The source file is untracked here, so this command alone does not inspect its content. Ran the additional check:
   `git diff --no-index --check /dev/null deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
   Result: no whitespace diagnostics; Git's expected “different from /dev/null” status was normalized by the wrapper (`test $? -eq 1`), and the command completed with exit 0.

## Acceptance status and limitation

The report/source hashes and focused non-Docker checks are independently confirmed. This does **not** establish live T14 acceptance. No Docker disposable integration or immutable probe was run; live socket reuse, fresh same-listener denial/counter behavior, conntrack behavior, and the full T14 attempt matrix remain **unverified**.
