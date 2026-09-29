# T14 Pre-Diagnostic Trace Review Fix 1 — Independent Validation

## Result

**DONE_WITH_CONCERNS** for the assigned Fix1 brief. The two synchronous pre-diagnostic intervals are visibly timed with the existing fixed-schema context manager; labels, source ordering, probe pins, and required local checks pass. This validates only Fix1 and does not establish overall T14 or live acceptance.

## Revision and scope

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- HEAD: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c` (unchanged during validation)
- Fix1 report: `docs/plans/2026-09-29-craft-107-t14-pre-diagnostic-trace-review-fix1-report.md`
- Plan and brief: `docs/superpowers/plans/2026-09-29-craft-107-t14-pre-diagnostic-trace-review-fix1.md`; `.superpowers/sdd/2026-09-29-craft-107-t14-pre-diagnostic-trace-review-fix1/task-1-brief.md`
- Reviewed predecessor finding: `docs/plans/2026-09-29-craft-107-t14-pre-diagnostic-boundary-trace-review.md`, specifically T14-TRACE-01. Also consulted predecessor report, approved Craft web-artifact Spec, `CONTEXT.md`, and ADR-0004.
- The worktree contains unrelated pre-existing T14 edits/untracked files. This validation changed no source or tests; this report is the only file created by this validator.

## Frozen file hashes

All current SHA-256 values match the Fix1 report's post-task hashes:

| Path | SHA-256 | Result |
|---|---|---|
| `deploy/craft/render-boundary/probe.py` | `1f0d7a0a1fd871356061a9bb5f5250965898ebc1f765e38b4bbba7f090cee81e` | Match |
| `deploy/craft/render-boundary/test_probe.py` | `96ac91ddc4cfbbee2ce6f64655cd522a9ca6951ee0032cf142b6a10fab67670b` | Match |
| `deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh` | `5f49639ac191757aff871bfac1eca2c84561784f1b5487179844b30c57e55385` | Match |
| `deploy/craft/render-boundary/test_build_volume_free_diagnostics_candidate.py` | `ed822100b38c5638ad8ccc0a25bcb6f11578c602e3db44db86566535af76f1e2` | Match |

Builder and topology test both pin the current `probe.py` digest. `EXPECTED_SOURCE_ID` remains `sha256:7e3c24469815aaed751e89d2d9fb6ac3899b0bfa57a925befd333639aa622c27`; the five preview digest entries match those recorded in the Fix1 report.

## Acceptance evidence

Source inspection confirms the labels are allowlisted and ordered as:

`renderer_websocket_control` → `renderer_diagnostic_performance_log_drain` → `renderer_control_server_shutdown` → `renderer_diagnostic_receipt_publish`.

The performance-drain wrapper contains only the synchronous `browser.get_log("performance")` call and `_append_raw_performance_entries`. The shutdown wrapper contains `control_server.shutdown()`, `control_server.server_close()`, and `control_server = None`, in that order. Both use `_stage_timing`; its existing start record remains `{label, started_monotonic_ns}` and is flushed before the operation, and the completion record remains `{label, started_monotonic_ns, ended_monotonic_ns, duration_ns}` and is emitted in `finally`. Receipt call arguments, barrier timeout constant, browser control scripts, policy/network settings, and existing exception paths show no change in the assigned delta.

Commands were run from `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01/deploy/craft/render-boundary` unless noted:

| Command | Result |
|---|---|
| `python3 -m unittest test_probe.ProbeBarrierTests.test_pre_diagnostic_stage_trace_uses_fixed_start_and_completion_records -v` | PASS, 1 test |
| `python3 -m unittest test_probe -v` | PASS, 148 tests |
| `python3 -m unittest discover -s . -p 'test_build_volume_free_diagnostics_candidate.py' -v` | PASS, 4 tests |
| `python3 -m py_compile probe.py test_probe.py` | PASS |
| `sh -n build-volume-free-diagnostics-candidate.sh` | PASS |
| `git diff --check -- probe.py test_probe.py build-volume-free-diagnostics-candidate.sh test_build_volume_free_diagnostics_candidate.py` | PASS |
| Python trailing-space/tab scan over the four assigned code/test/builder paths | PASS |

The brief's dotted unittest module form was not used because the package directory `render-boundary` is not a valid Python dotted module component; discovery selected the exact named candidate-builder test file.

## Gaps and risks

- This check is limited to the assigned Fix1 delta. It does not run Docker, a browser, a live service, or T14's denied-egress acceptance and makes no claim about those outcomes.
- The predecessor review's separate low-severity T14-TRACE-02 observation remains outside this Fix1 brief: the regression test asserts label allowlisting/order and exercises schema via a synthetic context-manager body; it does not drive `probe.run` with controlled WebDriver/server seams to bind emitted markers to the real operations or test their exception paths. Manual inspection verifies the wrappers in this frozen source, but future movement/refactoring of the body has weaker regression protection.
- Authentication/authorization and database migration/data-consistency checks do not apply to this local renderer trace instrumentation.
