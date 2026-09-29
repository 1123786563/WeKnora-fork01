# T14 Candidate Layer Derivation Fix 2 — Independent Validation

## Result

**PASS for the Fix 2 helper and its stated fake-Docker behavioral/static acceptance.** No source or test files were edited. The helper was not run against Docker, as required by the brief; actual candidate construction and live T14 acceptance remain unverified.

## Scope and frozen revision

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- Revision: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
- Task brief: `.superpowers/sdd/2026-09-29-craft-107-t14-candidate-layer-derivation-fix2/task-1-brief.md`
- Plan: `docs/superpowers/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-fix2.md`
- Progress ledger: `.superpowers/sdd/2026-09-29-craft-107-t14-candidate-layer-derivation-fix2/progress.md`

Required SHA-256 values matched before and after validation:

| File | SHA-256 |
| --- | --- |
| `deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh` | `8070571364b8d14906f3f226c6cf46f94568c39ddc4bbdba7a89edb76d48b833` |
| `deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py` | `7a7bd83bc253daafb1a57728beadb3f13e7862584ef353004860be1757b51764` |
| `docs/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-report.md` | `3af713b09fd390d90241cd7f0cb5b10e7ccad1f74d11dbb49042cc1ddd2a2d34` |

## Checks run

Commands were run from the worktree above.

| Command | Result |
| --- | --- |
| `python3 -m unittest discover -s deploy/craft/render-boundary -p 'test_derive_volume_free_diagnostics_candidate.py' -v` | PASS, 11 tests in 220.642 seconds (5 fake-Docker behavioral cases and 6 contract tests) |
| `sh -n deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh` | PASS |
| `PYTHONPYCACHEPREFIX=/tmp/t14-fix2-validation-pycache python3 -m py_compile deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py` | PASS |
| `git diff --check -- deploy/craft/render-boundary/derive-volume-free-diagnostics-candidate.sh deploy/craft/render-boundary/test_derive_volume_free_diagnostics_candidate.py docs/plans/2026-09-29-craft-107-t14-candidate-layer-derivation-report.md` | PASS, no diagnostics |
| Explicit trailing-whitespace scan across helper, test, and implementation report | PASS, none found |
| `git rev-parse HEAD` | `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c` |

The focused behavioral suite uses a stateful fake Docker executable; it does not start or contact a Docker daemon. It passed real Python subprocess timeout paths for create and commit. The observed cases cover exact-name create timeout reconciliation/removal, preserving all pre-existing and concurrent lookalike dangling images after ambiguous commit timeout, reference-aware verification-container cleanup before candidate removal, temp-directory cleanup failure causing owned-candidate removal, and successful retention after clean verification and cleanup.

## Acceptance review

- **Recovery reserve on mutating calls:** helper reserves time inside the total deadline; recovery commands are bounded by both remaining overall time and reserve. Behavioral create timeout test passes.
- **Commit ownership:** only a full ID returned directly by commit establishes candidate ownership. Timeout inventory is diagnostic and no inventory image is adopted or removed. The adversarial fixture seeds pre-existing and multiple newly created lookalikes and verifies they remain untouched.
- **Docker image reference cleanup:** fake image removal rejects referenced images; the cleanup-failure case proves the exact verification container is force-removed before owned candidate removal.
- **Temporary directory retention gate:** injected local removal failure produces a failing exit and owned candidate removal attempt, with no retained-candidate success marker.
- **Other pinned helper invariants:** static contract suite passes for fixed provenance/hash pins, no container start, exact `/opt/probe.py` diff, candidate config/layer/platform/tag/volume checks, and file metadata verification.
- **Backend API/auth/data migration checks:** not applicable to this local Docker image derivation helper.

## Gaps and risks

This validation does not establish behavior against a real Docker daemon. Docker CLI/daemon timeout behavior, actual container/image reference semantics, tar metadata from real `docker cp`, candidate layer ancestry/config, anonymous-volume cleanup, and helper end-to-end candidate derivation still require the separately authorized runtime acceptance. The helper was not invoked, and no browser, network, issue, staging, commit, or other external mutation command was run. This report therefore validates Fix 2's fake-Docker and static acceptance only; it does not claim live T14 acceptance.
