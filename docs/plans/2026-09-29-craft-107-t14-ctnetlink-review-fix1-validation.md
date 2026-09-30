# T14 ctnetlink Review Fix 1 — Independent Validation

**Status:** DONE_WITH_CONCERNS for the assigned parser/adapter checks. The exact disposable renderer flow and live socket query path remain unverified.

## Source identity

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- HEAD: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
- Revision state: validation was run against uncommitted source files; both owned source files were untracked.
- `deploy/craft/render-boundary/policy-helper/helper.py` SHA-256: `e9a36006796055cd11fc26ae37f2d53ea08f7a07e7a2cd56ea29b82649ee1b96` — matches the implementation report.
- `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` SHA-256: `5f2a5907e24b572a0bcb369c92ce8d453caf4a97cfc39db5657a389c0a1e2d41` — matches the implementation report.

## Checks

Commands were run from `deploy/craft/render-boundary/policy-helper` unless stated otherwise:

- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v` — PASS, 12 tests.
- `python3 -m unittest tests.test_barrier_adapter -v` — PASS, 18 tests.
- `python3 -m py_compile helper.py controller.py barrier_adapter.py` — PASS, exit 0, no output.
- From the worktree root: `git diff --check -- deploy/craft/render-boundary/policy-helper/helper.py deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix1-report.md` — PASS for tracked diff (no diagnostics). Since both source files are untracked, also ran `git diff --no-index --check /dev/null deploy/craft/render-boundary/policy-helper/helper.py` and the equivalent command for `tests/test_controller_integration.py`; both emitted no diagnostics and returned expected exit 1 for file differences.

An initial hash command used worktree-root-relative paths while running from the helper directory and failed with `FileNotFoundError`; it was corrected and both hashes were then verified as listed above. An initial diff-check attempt used an incorrect working-directory traversal; it was corrected and the commands above are the final successful checks.

## Acceptance evidence and gaps

The targeted parser selector exercises acceptance of the kernel's zero-status `NLMSG_DONE` payload and rejection of zero-status `NLMSG_ERROR`; all 12 parser/counter unit tests passed. The barrier adapter selector passed all 18 tests. Python compilation succeeded.

Live exact-flow acceptance remains unverified: reuse of the existing connection, fresh same-listener denial, counter delta, cleanup, and real socket query-path behavior were not run. This validation makes no live-policy, renderer, or end-to-end acceptance claim. No unittest discovery, Docker command, daemon operation, or image mutation was performed.
