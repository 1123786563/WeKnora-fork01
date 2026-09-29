# T14 candidate-layer derivation Fix 4 — independent validation

## Result

**DONE.** The supplied Fix4 checkpoint passes the focused fake-Docker regression suite and requested static checks. The helper and test exactly match the hashes in the Task 1 report. No source or test files were modified by this validation.

## Revision and scope

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- Revision: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
- Plan: `docs/superpowers/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-fix4.md`
- Task report checked: `docs/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-fix4-report.md`
- Helper SHA-256: `c8e6c5037dac9f281826438dbca8ae075ad1cb7162c93abd6cd8ecb78e943c21` (matches expected)
- Test SHA-256: `659766409957036be74fe5e6eb481b736abef3c8cc94a15c83878c6282b4a06a` (matches expected)

## Checks run

All commands ran from the worktree above.

| Command | Result |
| --- | --- |
| `python3 -m unittest deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py -v` | PASS, 15 tests in 64.984 seconds |
| `sh -n deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh` | PASS |
| `PYTHONPYCACHEPREFIX=/tmp/t14-fix4-validation-pycache python3 -m py_compile deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py` | PASS |
| `git diff --check --no-index /dev/null deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh` | PASS, exit 1 denotes file content diff; no whitespace diagnostics |
| `git diff --check --no-index /dev/null deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py` | PASS, exit 1 denotes file content diff; no whitespace diagnostics |
| Explicit trailing whitespace scan of both scoped files | PASS, none found |
| `sha256sum` for helper and test | PASS, hashes remain the expected values above |

The focused unittest harness is the requested fake-Docker suite. No Docker daemon, network, or browser command was invoked.

## Acceptance and risks

The root-resolution preflight and all 14 existing lifecycle/contract checks passed, for 15 total tests. No acceptance gap was found in the assigned Fix4 scope. The planned RED baseline is not part of this validation; as the Task 1 report explains, the checkpoint already contains the repair and regression test.

The worktree contained many pre-existing unrelated changes and untracked artifacts. They were not modified. The only file written by this validation is this report.
