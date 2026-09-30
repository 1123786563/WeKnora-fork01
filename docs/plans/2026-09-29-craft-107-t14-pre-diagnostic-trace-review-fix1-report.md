# Task 1 Report — T14 Pre-Diagnostic Trace Review Fix 1

## Outcome

Implemented finding `T14-TRACE-R1` by adding fixed stage timing boundaries around the performance-log drain and control-server shutdown before diagnostic receipt publication. The existing stage timing schema and call order are preserved. No Docker, browser, network, live service, issue mutation, git stage, or commit action was performed.

## Changed paths

- `deploy/craft/render-boundary/probe.py`
- `deploy/craft/render-boundary/test_probe.py`
- `deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh`
- `deploy/craft/render-boundary/test_build_volume_free_diagnostics_candidate.py`
- `docs/plans/2026-09-29-craft-107-t14-pre-diagnostic-trace-review-fix1-report.md`

The worktree already contained other T14 changes and untracked files before this task. The probe's supplied pre-task SHA matches the frozen source in the Brief. For the other modified paths, pre-task hashes below are reconstructed by reversing only this task's exact text substitutions.

## Hashes

| Path | Pre-task SHA-256 | Post-task SHA-256 |
|---|---|---|
| `deploy/craft/render-boundary/probe.py` | `4b580323f086622ca6e4954f7c9ad347dc8a9ed11bcdb27566ec492a7ce8668f` | `1f0d7a0a1fd871356061a9bb5f5250965898ebc1f765e38b4bbba7f090cee81e` |
| `deploy/craft/render-boundary/test_probe.py` | `c170fc2ee57c4556e68554a37afa63beb290a58ad6a6d4b88794fa46c9828077` | `96ac91ddc4cfbbee2ce6f64655cd522a9ca6951ee0032cf142b6a10fab67670b` |
| `deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh` | `8b504ec814310c212b64b342a44b4aa0f75003e710cceb9c341e91bb3f80e6c2` | `5f49639ac191757aff871bfac1eca2c84561784f1b5487179844b30c57e55385` |
| `deploy/craft/render-boundary/test_build_volume_free_diagnostics_candidate.py` | `915638b0f60590ecfc509e92e8d2b658771f5300a316621307c074eb1328a0cf` | `ed822100b38c5638ad8ccc0a25bcb6f11578c602e3db44db86566535af76f1e2` |

Builder and topology-test probe pins equal the final probe hash. The candidate `SOURCE_ID` remains `sha256:7e3c24469815aaed751e89d2d9fb6ac3899b0bfa57a925befd333639aa622c27`; all preview hashes were left unchanged.

## TDD and verification evidence

- RED: `python3 -m unittest test_probe.ProbeBarrierTests.test_pre_diagnostic_stage_trace_uses_fixed_start_and_completion_records -v` from `deploy/craft/render-boundary` — failed as expected because required labels were missing from `_STAGE_TIMING_LABELS`.
- A first attempt to run that test from repository root failed at import (`ModuleNotFoundError: No module named 'probe'`); rerunning from its test directory produced the intended RED failure.
- GREEN focused test: `python3 -m unittest test_probe.ProbeBarrierTests.test_pre_diagnostic_stage_trace_uses_fixed_start_and_completion_records -v` from `deploy/craft/render-boundary` — PASS.
- Full renderer suite: `python3 -m unittest test_probe -v` from `deploy/craft/render-boundary` — PASS, 148 tests.
- Candidate-builder suite: Brief's dotted command cannot import the hyphenated `render-boundary` module path (`ModuleNotFoundError: No module named 'deploy'`). Equivalent discovery command `python3 -m unittest discover -s deploy/craft/render-boundary -p 'test_build_volume_free_diagnostics_candidate.py' -v` — PASS, 4 tests.
- `python3 -m py_compile deploy/craft/render-boundary/probe.py deploy/craft/render-boundary/test_probe.py` — PASS.
- `sh -n deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh` — PASS.
- `git diff --check` — PASS.
- Explicit trailing-space/tab scan of the four code paths — PASS. The report was written after this scan.

## Task-only delta

- Added `renderer_diagnostic_performance_log_drain` and `renderer_control_server_shutdown` to the fixed-label allowlist.
- Extended the existing pre-diagnostic source-order test to require both labels between WebSocket control and diagnostic receipt publication.
- Wrapped only `browser.get_log("performance")` plus `_append_raw_performance_entries` in the performance-drain timing context.
- Wrapped only `control_server.shutdown()`, `server_close()`, and `control_server = None` in the shutdown timing context.
- Updated the renderer probe SHA pin in the candidate builder and topology test after source freeze.

No receipt payload, event schema, barrier, timeout, policy/network behavior, browser configuration, call order, preview hash, or source identity was changed. T14 acceptance is not claimed; independent validation and review remain with the parent workflow.
