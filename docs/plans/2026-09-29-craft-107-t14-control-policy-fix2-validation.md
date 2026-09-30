# T14 Control Policy Fix 2 — Independent Validation

Date: 2026-09-29 (Asia/Shanghai)
Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
Task revision (HEAD): `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
Scope: validate only the frozen Fix2 sources named by `.superpowers/sdd/2026-09-29-craft-107-t14-control-policy-fix2-plan/source-hashes.sha256`. No source or test files were modified. No Docker/live checks were run.

## Frozen source identity

Command: `sha256sum controller.py tests/test_controller_integration.py` (from `deploy/craft/render-boundary/policy-helper`).

Result matched the manifest exactly:

```text
9e6840357d22e4f2da1d4f79d52a01f5953991766fe9a30b9433973902f0a439  deploy/craft/render-boundary/policy-helper/controller.py
bc9affe43234b5da09654e3df9f9b7a5867be4662ea4d663736f31988fd36ab0  deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py
```

## Checks

1. From `deploy/craft/render-boundary/policy-helper`, ran:
   `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests tests.test_controller_integration.PolicyControllerBoundedEvidenceTests tests.test_controller_integration.PolicyControllerStopFailureTests tests.test_controller_integration.PolicyHelperInvocationUnitTests`
   Result: **16 tests passed, 1 Docker-gated test skipped** (`Ran 16 tests ... OK (skipped=1)`).
2. From `deploy/craft/render-boundary/policy-helper`, ran:
   `python3 -m py_compile controller.py helper.py barrier_adapter.py tests/test_controller_integration.py`
   Result: **exit 0**.
3. From worktree root, ran `git diff --check -- deploy/craft/render-boundary/policy-helper/controller.py deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`. Result: exit 0, but these paths are untracked in this worktree, so this command does not inspect their content. To check the untracked files, also ran `git diff --no-index --check /dev/null <path>` for each file. Both emitted no whitespace diagnostics; git returns 1 because each file differs from `/dev/null` (expected for this no-index comparison).
4. Revision command `git rev-parse HEAD`: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`.

## Acceptance status and limitation

Fix2's frozen hashes and stated non-Docker checks are independently confirmed. This is **not** live T14 acceptance: the disposable Docker integration and immutable probe were not run. Accordingly, live socket reuse, same-listener denial, counter behavior, conntrack behavior, and the full T14 attempt matrix remain unverified, consistent with the Fix2 report's Buildx/runtime limitation.
