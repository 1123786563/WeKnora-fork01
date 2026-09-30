# T14 Pre-Diagnostic Trace Review Fix 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete T14 pre-diagnostic boundary instrumentation by distinguishing performance-log drain and control-server shutdown before the diagnostic receipt.

**Architecture:** Extend the existing fixed-label `_stage_timing` diagnostics around the two synchronous operations identified in the first independent Review. Preserve call order, exception behavior, receipts, protocol, timeout values, policy behavior, and the exact candidate-source pin. Update the focused test and rebuild the source pin only after the final source hash is known.

**Tech Stack:** Python 3, `unittest`, shell candidate builder.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md`; implementation seam and constraints are inherited from `docs/superpowers/plans/2026-09-29-craft-107-t14-pre-diagnostic-boundary-trace.md`. Finding source: `docs/plans/2026-09-29-craft-107-t14-pre-diagnostic-boundary-trace-review.md`.

## Global Constraints

- Diagnostic evidence must remain bounded, fixed-schema, and fail closed; it cannot itself grant egress/policy credit.
- Keep browser attempts and control receipts serialized and bound to the current run nonce.
- Preserve the existing 30-second barrier limits and all operation order.
- Do not add dependencies or network access.
- Do not build or invoke a browser/container in the implementation task.

## Review Focus

- Performance-log retrieval blocks: a start record must exist before the synchronous call and completion after it returns or raises.
- Control-server shutdown blocks: shutdown and close must have a separately identifiable fixed timing boundary, including exceptions.
- Ordering and schema: both new labels must appear in the allowlist and the source-order test, with unchanged event fields.
- Pin integrity: the builder and topology test must match the final `probe.py` SHA-256 exactly.
- No semantic drift: protocol, receipt payload, timeout constants, and browser/policy behavior stay byte-for-byte unchanged outside marker wrapping and pin/test updates.

## Task 1: Instrument the two omitted synchronous boundaries

**Depends on:** `docs/superpowers/plans/2026-09-29-craft-107-t14-pre-diagnostic-boundary-trace.md` Task 1 checkpoint and its independent finding `T14-TRACE-R1`.

**Owner role:** `backend_implementer`; **validator:** `backend_validator`; **reviewer:** read-only reviewer dispatched by parent.

**Files owned:**

- Modify: `deploy/craft/render-boundary/probe.py`
- Modify: `deploy/craft/render-boundary/test_probe.py`
- Modify: `deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh`
- Modify: `deploy/craft/render-boundary/test_build_volume_free_diagnostics_candidate.py`
- Create: `docs/plans/2026-09-29-craft-107-t14-pre-diagnostic-trace-review-fix1-report.md`

**Consumes:** frozen first trace checkpoint (`probe.py` SHA `4b580323f086622ca6e4954f7c9ad347dc8a9ed11bcdb27566ec492a7ce8668f`), independent validation, Review finding `T14-TRACE-R1`.

**Produces:** two fixed labels, ordered regression assertions, exact builder pin, task report with hashes and commands.

1. Add a RED test requiring fixed labels `renderer_diagnostic_performance_log_drain` and `renderer_control_server_shutdown`, and assert their order between `renderer_websocket_control` and `renderer_diagnostic_receipt_publish`.
2. Run the targeted test and record the expected missing-label failure.
3. Wrap only `browser.get_log("performance")` plus raw-entry append in the performance-log label; wrap only `control_server.shutdown()`, `server_close()`, and the assignment to `None` in the shutdown label. Preserve statements and their order.
4. Run targeted tests, the full renderer suite, candidate-builder tests, `py_compile`, `sh -n`, and whitespace checks including untracked files.
5. Update only the renderer probe SHA in builder and topology test after the final probe source is frozen. Confirm the preview/source pins are unchanged.
6. Write exact before/after hashes, task-only diff, RED/GREEN output, commands, and the assertion that no Docker/browser/live action occurred.

**Verification:** targeted trace test; `python3 -m unittest test_probe -v`; `python3 -m unittest deploy.craft.render-boundary.test_build_volume_free_diagnostics_candidate -v`; `python3 -m py_compile deploy/craft/render-boundary/probe.py deploy/craft/render-boundary/test_probe.py`; `sh -n deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh`; `git diff --check` and explicit trailing-whitespace scan of every owned file.

**Acceptance:** both new boundaries emit existing exact start/completion schemas; order test passes; all required checks pass; final builder/test pins equal final probe hash; no non-marker behavior, protocol, timeout or receipt changes.

**Failure handling:** preserve the first checkpoint; report exact failing check. Do not expand scope or alter runtime timeout/receipt behavior. Parent dispatches a fix plan only for a verified finding.
