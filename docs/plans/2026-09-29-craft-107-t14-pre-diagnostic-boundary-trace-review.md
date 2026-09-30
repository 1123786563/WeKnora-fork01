# T14 Pre-Diagnostic Boundary Trace — Independent Review

## Scope and evidence

Read-only review of the frozen Task 1 checkpoint against the approved Craft web-artifact Spec, `CONTEXT.md`, ADR-0004, the assigned brief, the implementation plan, and the implementer's report. Reviewed only the four assigned source/test paths. No OCR, Docker, browser, network, or live acceptance run was performed. The implementer's reported test results are cited as reported evidence, not independently rerun results.

| Path | SHA-256 reviewed |
| --- | --- |
| `deploy/craft/render-boundary/probe.py` | `4b580323f086622ca6e4954f7c9ad347dc8a9ed11bcdb27566ec492a7ce8668f` |
| `deploy/craft/render-boundary/test_probe.py` | `c170fc2ee57c4556e68554a37afa63beb290a58ad6a6d4b88794fa46c9828077` |
| `deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh` | `8b504ec814310c212b64b342a44b4aa0f75003e710cceb9c341e91bb3f80e6c2` |
| `deploy/craft/render-boundary/test_build_volume_free_diagnostics_candidate.py` | `915638b0f60590ecfc509e92e8d2b658771f5300a316621307c074eb1328a0cf` |

The new call sites use the existing `_stage_timing` context manager. Its start record is flushed before entering the body and contains exactly `label` and integer `started_monotonic_ns`; its `finally` completion contains the existing four fixed fields. The twelve names are allowlisted. The task patch shows no changed browser command, barrier call arguments, timeout value, policy setting, or exception handler. The builder's probe pin equals the reviewed source hash. Its exact source image ID and all five preview pins remain unchanged; the preview file hashes also match those pins.

## Findings

### T14-TRACE-01 — Medium — Diagnostic path contains an unmarked blocking interval

**Evidence / affected symbol:** In `probe.run`, `renderer_websocket_control` completes at `probe.py:3301`. The next marker, `renderer_diagnostic_receipt_publish`, begins only at line 3327. Between them, `browser.get_log("performance")` (line 3305), `control_server.shutdown()` (line 3308), `server_close()` (line 3309), and evidence processing execute without a stage start. `get_log` is a synchronous WebDriver call; `shutdown` can wait for the server loop. If either stalls, the trace ends with a completed WebSocket stage and never names the blocking call group or reaches the receipt marker.

**Impact:** The plan's stated goal is to identify the first synchronous startup or diagnostic-control boundary that prevents the diagnostic-complete receipt. This interval preserves a material ambiguity in precisely that failure window, so a bounded rerun could still fail without locating the blocking boundary.

**Smallest defensible correction:** Add fixed, flushed stage contexts for the performance-log read and control-server close (or one clearly named post-control collection/close group), before the receipt marker. Preserve call order, existing exception handling, and timeouts. Test the new labels at their real call sites.

### T14-TRACE-02 — Low — New trace test does not bind markers to real pre-diagnostic operations

**Evidence / affected test:** `test_probe.py:2107-2129` checks source-text positions and exercises `_stage_timing` around a synthetic `events.append("operation")`. It does not execute `probe.run` with controlled browser/barrier seams to assert that each new start precedes its corresponding operation, or that a failing operation emits completion while retaining the original exception. The startup-failure test at lines 2659-2677 covers only the first four stages and the outer failure path.

**Impact:** The tests can remain green if a later wrapper is moved before or after its intended browser call, or if its body becomes empty. The core diagnostic ordering promise then depends on manual inspection alone. An earlier post-release test uses a real seam and ordered operation events, showing a suitable local pattern.

**Smallest defensible correction:** Add a controlled `probe.run` test for representative later boundaries, especially HTTP, WebSocket, post-control collection, and receipt publication. Assert start → actual operation → completion and an exception path. Keep it local and avoid a live browser or Docker dependency.

## Verdict

**Spec compliance: conditional fail for this Task 1 objective.** The patch preserves the approved no-egress and receipt behavior as far as source inspection shows, and it meets the fixed schema, flush, label, and pin requirements. Finding T14-TRACE-01 leaves the planned diagnostic attribution objective incomplete. This review does not establish full Craft Spec or T14 live acceptance.

**Code quality: conditional fail.** The source change is narrow and the hash pin is correct, but the unmarked synchronous interval is a substantive diagnostic gap. The missing real-call-site regression test in T14-TRACE-02 adds a lower-severity maintenance risk. The implementer reports 148 renderer and 4 candidate-helper tests passing, plus compile and shell syntax checks; those results do not cover the identified gap.
