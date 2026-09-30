# T14 Pre-Diagnostic Trace Review Fix 1 — Independent Review

## Scope and evidence

Read-only source review of the frozen Fix1 checkpoint against the approved Craft web-artifact Spec, `CONTEXT.md`, ADR-0004, the Fix1 plan and brief, the predecessor task report and review, and finding `T14-TRACE-R1`. Only the four assigned renderer source/test/builder paths were reviewed. No OCR, Docker, browser, or live run was performed. The implementer's 148 renderer and 4 builder passing tests are reported evidence, not independently rerun results.

| Path | SHA-256 observed and matched to Fix1 report |
| --- | --- |
| `deploy/craft/render-boundary/probe.py` | `1f0d7a0a1fd871356061a9bb5f5250965898ebc1f765e38b4bbba7f090cee81e` |
| `deploy/craft/render-boundary/test_probe.py` | `96ac91ddc4cfbbee2ce6f64655cd522a9ca6951ee0032cf142b6a10fab67670b` |
| `deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh` | `5f49639ac191757aff871bfac1eca2c84561784f1b5487179844b30c57e55385` |
| `deploy/craft/render-boundary/test_build_volume_free_diagnostics_candidate.py` | `ed822100b38c5638ad8ccc0a25bcb6f11578c602e3db44db86566535af76f1e2` |

The task report's before/after hashes and narrow delta identify the Fix1 changes in this shared, uncommitted worktree. `git diff --check` on the four paths returned success. This review did not treat the worktree's unrelated changes as part of Fix1.

## Finding disposition

**`T14-TRACE-R1` — Medium — resolved for the assigned Fix1 objective.** In `probe.run`, the `renderer_diagnostic_performance_log_drain` context starts before `browser.get_log("performance")` and includes `_append_raw_performance_entries` (`probe.py:3305-3309`). The distinct `renderer_control_server_shutdown` context encloses `shutdown`, `server_close`, and setting the server reference to `None` (`probe.py:3311-3314`). Both occur after the WebSocket control call and before diagnostic receipt publication (`probe.py:3292-3334`). The previous unmarked synchronous call group is therefore distinguishable in a stalled trace.

The two new labels are fixed members of `_STAGE_TIMING_LABELS` (`probe.py:129-130`). The unchanged `_stage_timing` manager emits and flushes the two-field start record before its body, then emits and flushes the four-field completion record in `finally` (`probe.py:134-161`). A body exception still propagates after completion emission; no new catch, fallback, timeout, receipt field, or barrier argument appears in the Fix1 delta. Source order of the original operations is preserved. The source-order test includes both labels in the required interval (`test_probe.py:2097-2110`). The builder and topology-test probe pins equal the observed final probe SHA (`build-volume-free-diagnostics-candidate.sh:7`, `test_build_volume_free_diagnostics_candidate.py:15`); the source image ID and preview pins remain as reported in the predecessor checkpoint.

## Residual quality finding

**`T14-TRACE-02` — Low — inherited, still open.** The extended trace test checks textual marker order and exercises `_stage_timing` with a synthetic operation (`test_probe.py:2108-2146`). It does not drive `probe.run` through controlled performance-log and shutdown seams, so it would not detect a later misplaced or empty wrapper, nor demonstrate completion and original-exception propagation at either real call site. This is a behavioral regression-coverage risk, not evidence that the current wrappers are misplaced. **Smallest correction:** add a local controlled `probe.run` test that records start → actual `get_log`/shutdown operation → completion, including one failing operation; retain the existing source-order/schema test.

## Verdict

**Spec compliance: pass for the narrow Fix1 objective.** `T14-TRACE-R1` is resolved on the reviewed hashes, and the source shows no deviation from the approved Spec's isolated, no-egress Craft boundary or ADR-0004's Task/Run identity. This is not full T14 or live Craft acceptance.

**Code quality: pass with the low residual test gap above.** The change is confined to marker wrapping, ordered assertions, and the exact probe pins. No critical, high, or medium finding remains in this Fix1 scope. Live runtime observability remains for the parent workflow to establish.
