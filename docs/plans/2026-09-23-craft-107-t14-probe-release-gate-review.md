# T14 probe release gate independent review

Date: 2026-09-23. Read-only scoped review of `run-probe.sh` and `test_probe_release_gate.py` in the T14 Worktree, HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`. Sources: approved #107/#129, the release-gate plan/report, current `validate_browser_evidence` result shape, and the prior probe-correlation review. Concurrent changes to `probe.py`, `test_probe.py` and the preview fixture are outside this two-file review. No code edits, staging, OCR or live Docker/Chromium run.

## Exact checkpoint and verdict

Full-content SHA-256 matches the implementation report: `deploy/craft/render-boundary/run-probe.sh` `8641ab50a111b93f9b3f919b9e83fbd436f5dbe0980207b33265d9d1cc6710b1`; `deploy/craft/render-boundary/test_probe_release_gate.py` `d395271fe56882ba85ea6921ecc13bbfe12bd434223ada478a4b3f816d5b2614`. The shell file is modified and the test is untracked; neither is committed.

- **Scoped Spec compliance: PASS.** Immediately after browser evidence validation, the runner requires list-valued `unobserved_browser_attempt_ids` and `browser_observation_limits`, then exits nonzero when either is nonempty (`run-probe.sh:73-85`). This occurs before screenshot and evidence success artifacts are written (`:86-93`) and before success paths are printed (`:104-108`). Bash `set -euo pipefail` propagates Python's nonzero exit, while container output, build log and inspection files remain available for diagnosis.
- **Scoped code quality: PASS.** The focused harness executes the actual embedded Python block, verifies both incomplete-result shapes fail without writing new evidence/screenshot files, and verifies a complete synthetic result writes them (`test_probe_release_gate.py:19-114`). It is a meaningful guard of the release decision, while the real validator remains the source of attempt correlation and denial evidence. No scoped blocking finding.
- **Full T14 browser/broker gate: NOT VERIFIED.** The current validator treats `Document` mechanism classes as ambiguous, and the fixture/validator denial contract remains under separate correction. Therefore the live runner should remain nonzero until complete, corroborated Chromium evidence exists. This test does not prove network isolation or authenticated render broker behavior.

## Verification and operating limit

I independently ran `python3 -m unittest test_probe_release_gate.py` (2 passed), `bash -n run-probe.sh`, Python compile, and `git diff --check` on the two files; all passed. The implementation report records the RED test against the old runner; I did not recreate that historical state. The harness mocks `validate_browser_evidence` and does not execute Docker or the outer shell. It proves the gate's handling of validator output, not that the validator has seen real Chromium requests.

The output directory can already contain evidence/screenshot files from an earlier run; this change prevents **new** success artifact writes on failure but does not remove old files. Consumers must use this invocation's exit status and run-specific logs, or select a fresh output directory, when interpreting an artifact. This is an operational limit of the existing output layout rather than a failure of the scoped exit-code gate.
