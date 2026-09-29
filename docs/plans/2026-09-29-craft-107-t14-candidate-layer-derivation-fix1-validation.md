# T14 Candidate Layer Derivation Fix 1 Validation

**Status:** DONE_WITH_CONCERNS

**Workspace:** `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`  
**Revision:** `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c` (HEAD unchanged during validation)

## Frozen inputs

Start and end SHA-256 matched for all three assigned files:

- `deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh`: `efa0207e4fb6ed585e586bf39f1fd478fb91890904cc5aa3d7e14a9ce0f592b5`
- `deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py`: `ee7bb91957d6d53046a95fec22fd75eda7987b5c7d154ffd6acf298730a895e1`
- `docs/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-report.md`: `1af83aba336cc2901c9031f962b7416e235b56653d469156742751f4b75b9a00`

## Checks

Commands run in the workspace above:

- `python3 -m unittest discover -s deploy/craft/render-boundary -p 'test_derive_volume_free_diagnostics_candidate.py' -v` — PASS, all 10 tests, 45.370s.
- `python3 -m py_compile deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py` — PASS.
- `sh -n deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh` — PASS.
- `git diff --check -- deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py docs/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-report.md` — PASS for tracked diff; these assigned files are untracked at this revision, so the command alone does not cover them.
- Scoped untracked-file check using `git diff --no-index --check /dev/null <file>` for each of the three assigned files — PASS, no whitespace diagnostics.
- Scoped Python trailing-whitespace scan over the same three files — PASS.

The fake-Docker tests invoke only their temporary fake executable. No real Docker, browser, or network was used.

## Acceptance evidence

- Success test asserts candidate retention only after both temporary containers are absent.
- Create-timeout test simulates daemon-side named container creation followed by client exit 124; it asserts nonzero helper exit, `create_outcome=reconciled_timeout`, and empty container inventory.
- Commit-timeout test simulates creation of an untagged dangling candidate followed by client exit 124; it asserts successful reconciliation, retained candidate, and empty container inventory.
- Cleanup-failure test makes verification-container removal fail; it asserts nonzero exit, candidate removal, and no retained-candidate output.
- Static tests cover pinned IDs/hashes, no container start, one-file diff constraint, candidate config/platform/file metadata, cleanup primitives, bounded cleanup window, and no caller-provided IDs.

## Acceptance gap and risk

The commit-timeout fixture starts without any pre-existing dangling images. It therefore does not behaviorally prove that a pre-existing image with matching metadata is excluded from reconciliation. The helper snapshots pre-commit dangling IDs and explicitly skips those IDs, then inspects the selected candidate and verifies embedded file hashes in the later candidate-copy loop; this supports the intended behavior by inspection, but the acceptance case lacks an adversarial fixture. No source or test files were changed during this validation.
