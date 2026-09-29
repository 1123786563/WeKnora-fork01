# T14 Candidate Layer Derivation Fix3 — Independent Validation

## Result

**DONE_WITH_CONCERNS — frozen source checkpoint passed the assigned local acceptance checks.** No source or test file was modified. This validates the fake-Docker behavior and static checks only; no live Docker derivation was run.

## Scope and revision

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- Revision: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
- Brief: `.superpowers/sdd/2026-09-29-craft-107-t14-candidate-layer-derivation-fix3/task-1-brief.md`
- Plan: `docs/superpowers/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-fix3.md`
- Prior review: `docs/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-fix2-review.md`

Required hashes matched both before and after validation:

| File | SHA-256 |
| --- | --- |
| `deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh` | `95b1446d524c63ec90765f708f2da010c88ce6a0571b4405ae18557aa09977b1` |
| `deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py` | `49d7153163f88a2ce8698e7685a5c7f0e69e0c13391fdcd23d5a1de22e0d27a5` |

## Checks

Commands were run from the worktree above.

| Command | Result |
| --- | --- |
| `python3 -m unittest deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py -v` | PASS, 14 tests in 103.276 seconds |
| `bash -n deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh` | PASS |
| `PYTHONPYCACHEPREFIX=/tmp/t14-fix3-validation-pycache python3 -m py_compile deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py` | PASS |
| `git diff --check -- deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py` | PASS; checked tracked diff scope only |
| Explicit trailing-whitespace scan over both owned files | PASS, no trailing whitespace |
| `git rev-parse HEAD` | `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c` |

The fake-Docker behavioral tests cover both zero-exit truncated-ID create positions: derivation (`test_zero_exit_truncated_derivation_id_reconciles_and_removes_named_container`) and verification (`test_zero_exit_truncated_verification_id_reconciles_and_removes_named_container`). The derivation cleanup-failure case (`test_zero_exit_truncated_derivation_id_cleanup_failure_reports_exact_unresolved_identity`) asserts nonzero failure, reports `unknown_outcome=create_untrusted_id` and the exact generated name, leaves the unresolved container visible in fake state, and emits no `candidate_retained_id=` success marker. Separate cleanup failure tests also assert candidate removal and failure.

## Acceptance and limits

The assigned Fix2 medium finding is covered at both create positions, and cleanup failure is fail-closed with exact unresolved identity. No Docker daemon, browser, network, Issue, staging, or commit operation was used. Actual Docker CLI/daemon behavior, anonymous-volume removal, image-reference semantics, candidate layer/config, and the end-to-end T14 acceptance remain unverified here.

`git diff --check` does not inspect untracked files in this shared worktree; the explicit whitespace scan covered both assigned untracked/modified source files. The worktree contains unrelated in-progress changes, which were not examined or altered.
