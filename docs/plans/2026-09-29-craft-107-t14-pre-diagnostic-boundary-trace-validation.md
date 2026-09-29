# T14 Pre-Diagnostic Boundary Trace — Independent Validation

## Result

**PASS for the frozen Task 1 checkpoint.** No source, test, or implementation report was changed. Validation report only.

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- Revision: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
- Scope: fixed, flushed pre-diagnostic stage start/completion trace and exact candidate-builder pin. This does not assert live T14 acceptance.

## Hash verification

All four reported post-change SHA-256 values match the worktree exactly:

| File | Reported and observed SHA-256 | Match |
|---|---|---|
| `deploy/craft/render-boundary/probe.py` | `4b580323f086622ca6e4954f7c9ad347dc8a9ed11bcdb27566ec492a7ce8668f` | yes |
| `deploy/craft/render-boundary/test_probe.py` | `c170fc2ee57c4556e68554a37afa63beb290a58ad6a6d4b88794fa46c9828077` | yes |
| `deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh` | `8b504ec814310c212b64b342a44b4aa0f75003e710cceb9c341e91bb3f80e6c2` | yes |
| `deploy/craft/render-boundary/test_build_volume_free_diagnostics_candidate.py` | `915638b0f60590ecfc509e92e8d2b658771f5300a316621307c074eb1328a0cf` | yes |

Command: `sha256sum deploy/craft/render-boundary/probe.py deploy/craft/render-boundary/test_probe.py deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh deploy/craft/render-boundary/test_build_volume_free_diagnostics_candidate.py` (worktree root). All output matched the task report.

## Acceptance evidence

- The allowlist includes all twelve required labels. Their call sites appear in the expected source order around the named startup, browser, control, and receipt operations.
- The existing timing manager emits `T14_STAGE_START` with only `label` and `started_monotonic_ns`, uses `time.monotonic_ns()`, and flushes stderr before yielding to the operation. Its `finally` emits the established completion schema (`label`, start, end, duration), also flushed. No dynamic operation data is added to these records.
- The focused trace test asserts the exact schemas, integer timestamps, allowlisted labels, order, and start-before-operation behavior. Startup failure coverage also checks the bounded fixed marker sequence and cleanup.
- The builder's probe pin and topology test pin both match the observed probe SHA. The builder keeps the specified immutable source image ID and the five other preview hashes present in the task report unchanged.
- Inspection of the instrumentation shows wrapper contexts around existing operations; no barrier method, protocol field, browser/policy configuration, timeout, or receipt sequencing change was found in the assigned checkpoint.

## Commands and results

All commands were run with the explicit worktree as `workdir` (the focused renderer tests use its `deploy/craft/render-boundary` subdirectory).

1. From `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01/deploy/craft/render-boundary`:
   `python3 -m unittest test_probe.ProbeBarrierTests.test_pre_diagnostic_stage_trace_uses_fixed_start_and_completion_records -v` — **PASS**, 1 test.
2. Same directory:
   `python3 -m unittest test_probe.ChromeDriverVersionTests.test_run_startup_failure_emits_bounded_diagnostics_and_cleans_up -v` — **PASS**, 1 test.
3. From worktree root:
   `python3 -m unittest deploy.craft.render-boundary.test_build_volume_free_diagnostics_candidate.DiagnosticsCandidateBuildTopologyTest.test_helper_pins_source_and_overlays_only_reviewed_runtime_files -v` — **PASS**, 1 test.
4. From worktree root:
   `python3 -m py_compile deploy/craft/render-boundary/probe.py deploy/craft/render-boundary/test_probe.py` — **PASS**.
5. From worktree root:
   `sh -n deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh` — **PASS**.
6. From worktree root:
   `git diff --check -- deploy/craft/render-boundary/probe.py deploy/craft/render-boundary/test_probe.py deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh deploy/craft/render-boundary/test_build_volume_free_diagnostics_candidate.py` — **PASS**.

An initial invocation of the renderer test from the worktree root failed import (`ModuleNotFoundError: test_probe`); rerunning from the prescribed renderer directory passed. This was a command working-directory issue, not a code failure.

## Gaps and risks

No live browser/container run was performed, as required by the brief. The report also records the full renderer unit suite as 148 passing tests; this independent validation reran only the directly relevant trace, startup-failure, candidate pin, and syntax checks. Therefore this result validates the frozen instrumentation checkpoint and pin, not runtime observability or overall T14 acceptance. The worktree contains unrelated existing modifications/untracked files; the four assigned code/test files match the report hashes, and no changes were made by this validation beyond this report.
