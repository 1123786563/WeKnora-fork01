# T14 candidate-layer derivation Fix 4 — Task 1 report

## Result

**PASS for the local root-resolution repair and fake-Docker regression checks.** The supplied checkpoint already had the requested three-parent traversal and exact `repo/deploy/craft/render-boundary` fake fixture with a root-preflight behavioral test. I did not alter either source file. No Docker daemon, browser, network, staging, or commit operation was used; the Python tests use only their isolated fake Docker executable.

## Scope and hashes

Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`

| Owned file | SHA-256 before checks | SHA-256 after checks |
| --- | --- | --- |
| `deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh` | `c8e6c5037dac9f281826438dbca8ae075ad1cb7162c93abd6cd8ecb78e943c21` | `c8e6c5037dac9f281826438dbca8ae075ad1cb7162c93abd6cd8ecb78e943c21` |
| `deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py` | `659766409957036be74fe5e6eb481b736abef3c8cc94a15c83878c6282b4a06a` | `659766409957036be74fe5e6eb481b736abef3c8cc94a15c83878c6282b4a06a` |

The helper resolves `SCRIPT_DIR/../../../`, which maps `repo/deploy/craft/render-boundary` to `repo`. The fake-Docker harness creates that exact layout and invokes the helper with a different current working directory. Its root-preflight test checks the pinned report digest is reached and only then observes fake Docker calls.

## RED baseline

The planned RED run could not be reproduced safely against this checkpoint: the helper already contains the three-parent traversal, and the exact-layout test already exists. The focused regression therefore passed on its first run in this task. Fix3's recorded live attempt at `docs/testing/craft/t14/2026-09-29-candidate-layer-fix3/derivation.log` shows the prior failure: `shasum` searched under `/Users/wuyongjun/.codex/worktrees/craft-107-t14/docs/...` (outside the checkout), then the predecessor report digest check failed with an empty actual digest and exit code 1. This happened during preflight before any Docker mutation.

## Checks

Commands run from the worktree:

| Command | Result |
| --- | --- |
| `python3 -m unittest deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py -v` | PASS, 15 tests in 28.863 seconds, including exact-layout root preflight and all lifecycle behavior cases |
| `sh -n deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh` | PASS |
| `PYTHONPYCACHEPREFIX=/tmp/t14-fix4-validation-pycache python3 -m py_compile deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py` | PASS |
| Explicit trailing-whitespace scan of both owned files | PASS |
| `git diff --check --no-index /dev/null <each owned file>` | PASS; no whitespace errors (nonzero diff-presence status expected for these untracked files) |

No files outside the two listed source/test files and this task report were written by this task. Existing unrelated worktree changes were left untouched.
