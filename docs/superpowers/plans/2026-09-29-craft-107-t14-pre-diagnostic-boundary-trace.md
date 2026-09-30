# T14 Pre-Diagnostic Boundary Trace Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Task uses an uncommitted checkpoint; no commit.

**Goal:** Identify the first synchronous T14 renderer startup or diagnostic-control boundary that prevents publication of `001-diagnostic_complete-receipt.json`, without changing runtime behavior or barrier policy.

**Architecture:** Extend the existing allowlisted `T14_STAGE_START` / `T14_STAGE_TIMING` mechanism to cover renderer entry, startup, initial UX, loopback diagnostic controls, and diagnostic receipt publication. Update the candidate builder's pinned `probe.py` digest to the resulting independently reviewed source hash. After implementation review and validation, the parent builds one immutable candidate and runs one bounded diagnostic; stage labels contain no dynamic page, URL, request, or exception data.

**Tech Stack:** Python 3, Selenium/Chromium, shell candidate builder, Docker immutable local image workflow.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md` (approved Spec #107; T14/#129 no-egress preview evidence). Related implementation pointers: `docs/plans/2026-09-23-craft-107-dag.md`, `docs/plans/2026-09-29-craft-107-t14-current-source-candidate-live-attempt.md`, `docs/plans/2026-09-29-craft-107-t14-browser-matrix-image-blocker.md`, T14 Worktree's stage-start plan/review and candidate pin review; `CONTEXT.md`; ADR-0004.

## Global Constraints

- Preserve renderer `--network none`, read-only root, UID 10001, dropped capabilities, resource ceilings, no-egress policy, and cleanup behavior exactly.
- Preserve 30-second host/renderer receipt caps and 120-second maximum; do not add retries or weaken receipt validation.
- Emit only fixed allowlisted stage names and monotonic integer timestamps to stderr; never include URL, DOM, command, status, response, or exception data.
- Candidate input must remain the exact source image ID `sha256:7e3c24469815aaed751e89d2d9fb6ac3899b0bfa57a925befd333639aa622c27` and only the six already allowlisted probe/preview files.
- No commits, staging, issue mutation, push, publication, or shared-worktree edits.
- Implementation and no-Docker validation use T14 Worktree `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`; parent owns Docker slot after task Review and validation.

## Review Focus

- A synchronous ChromeDriver/Selenium/CDP call stalls before the receipt: preceding stage-start is visible and names the bounded call group.
- An earlier preparation or import fails before browser creation: a fixed earlier marker distinguishes that boundary without logging exception details.
- The local preview navigation or UX capture hangs: startup markers identify navigation/initial UX separately from later policy controls.
- HTTP diagnostic control completes but WebSocket diagnostic control hangs: separate markers preserve that distinction.
- Candidate pin drift or added files: preflight rejects before Docker, and post-import embedded hashes still bind the exact reviewed six-file set.

---

### Task 1: Instrument and pin the pre-diagnostic renderer path

**Depends on:** Current T14 checkpoint SHA `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`; current `probe.py` source SHA `6537b05abdcdd917a95b5ae65248b66e87ed717aa61ccab2788f805a6376157b`; candidate builder pin review `docs/plans/2026-09-28-craft-107-t14-stage-start-pin-review.md`.

**Role:** `backend_implementer`; independent validator `backend_validator`; independent task reviewer assigned by parent.

**Files:**

- Modify: `deploy/craft/render-boundary/probe.py`
- Test: `deploy/craft/render-boundary/test_probe.py`
- Modify: `deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh`
- Test: `deploy/craft/render-boundary/test_build_volume_free_diagnostics_candidate.py`
- Report: `docs/plans/2026-09-29-craft-107-t14-pre-diagnostic-boundary-trace-report.md`

**Interfaces:**

- Consumes existing `_STAGE_TIMING_LABELS`, `_emit_stage_start`, `_emit_stage_timing`, `_stage_timing`, and host `barrier_adapter` protocol unchanged.
- Produces new fixed stage events using the existing two schemas. Labels must distinguish Selenium import, identity wait, preview listener setup, WebDriver session creation, initial CDP bootstrap, control server startup, preview navigation/ready wait, UX state capture, viewport interaction/capture, HTTP diagnostic control, WebSocket diagnostic control, and diagnostic-complete receipt publication.
- Candidate builder's `overlay_sha256("probe.py")` and topology test assertion consume the final reviewed `probe.py` SHA; the five preview digests and source image ID stay unchanged.

**Owned state and conflict preflight:** The implementer owns only the four code/test files and its report in T14 Worktree. No other active implementer owns these paths. The integration Worktree OCR run is read-only over another checkout. No shared test DB, browser profile, container name, port, or Docker mutation is used during implementation/tests. Parent reserves the disposable Docker/browser run after independent Review and validation.

- [ ] RED: add tests asserting every required fixed pre-diagnostic label is allowlisted, start records contain exactly label + integer monotonic timestamp, completion retains its fixed schema, and a staged fake operation emits start before the operation; add candidate-builder pin assertion tied to final digest without invoking Docker.
- [ ] Run focused tests and capture expected failures before implementation:
  - `python3 -m unittest test_probe.ProbeBarrierTests.test_pre_diagnostic_stage_trace_uses_fixed_start_and_completion_records -v`
  - `python3 -m unittest deploy.craft.render-boundary.test_build_volume_free_diagnostics_candidate.DiagnosticsCandidateBuildTopologyTest.test_helper_pins_source_and_overlays_only_reviewed_runtime_files -v`
  Expected: missing labels/call-site instrumentation and stale pin fail.
- [ ] GREEN: add `_stage_timing` contexts around the exact pre-diagnostic boundaries named above, beginning before Selenium imports and ending after diagnostic receipt publication. Keep each marker fixed and flushed. Do not move browser operations across policy barriers, catch/ignore errors, alter timeouts, or change receipt sequencing.
- [ ] Update the builder/test probe SHA pin only after source is complete; retain all other five overlay hashes and exact immutable source-image check.
- [ ] Run focused renderer tests, focused candidate-builder tests, `python3 -m py_compile deploy/craft/render-boundary/probe.py deploy/craft/render-boundary/test_probe.py`, `sh -n deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh`, and untracked-aware whitespace checks on all four files.
- [ ] Save exact before/after SHA-256, task patch, actual command/output/exit status in the task report. No image build, Docker call, browser execution, or commit by implementer.

**Acceptance / failure handling:** All tests and static checks pass; code review and backend validation approve the exact hashes; independent review confirms no behavioral/timeout/policy change; candidate builder's source/overlay preflight passes for the final reviewed hash. Any need to change timeouts, barrier semantics, policy or public acceptance stops this narrow task and returns to architecture review. After task gates pass, parent performs one bounded immutable candidate build and diagnostic run. The run must identify a stage boundary or reproduce a separately evidenced environment failure; missing receipt remains failure and no browser cell is credited.
