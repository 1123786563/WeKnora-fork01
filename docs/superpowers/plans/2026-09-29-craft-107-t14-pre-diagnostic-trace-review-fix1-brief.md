# Task Brief — T14 Pre-Diagnostic Trace Review Fix 1

## Objective

Resolve independent review finding `T14-TRACE-R1` by placing fixed timing boundaries around the synchronous performance-log drain and control-server shutdown before the diagnostic receipt.

## Facts and pointers

- Approved Spec: `docs/specs/2026-09-23-craft-web-artifact-spec.md`.
- Domain constraints: `CONTEXT.md`, `docs/adr/0004-task-is-session.md`.
- Fix plan: `docs/superpowers/plans/2026-09-29-craft-107-t14-pre-diagnostic-trace-review-fix1.md`.
- Prior checkpoint report: `docs/plans/2026-09-29-craft-107-t14-pre-diagnostic-boundary-trace-report.md`.
- Finding report: `docs/plans/2026-09-29-craft-107-t14-pre-diagnostic-boundary-trace-review.md`.
- Prior validation: `docs/plans/2026-09-29-craft-107-t14-pre-diagnostic-boundary-trace-validation.md`.
- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`, HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`; no commits or staging authorized.

## Interfaces consumed / produced

- Consume existing `_stage_timing`, `_STAGE_TIMING_LABELS`, task-1 probe pin, and source-order test.
- Add `renderer_diagnostic_performance_log_drain` around only `browser.get_log("performance")` and `_append_raw_performance_entries`.
- Add `renderer_control_server_shutdown` around only `control_server.shutdown()`, `control_server.server_close()`, and `control_server = None`.
- Extend the source-order test so both occur after `renderer_websocket_control` and before `renderer_diagnostic_receipt_publish`.
- Preserve exact stage event schemas and flush behavior.

## Ownership and constraints

Write only `deploy/craft/render-boundary/probe.py`, `deploy/craft/render-boundary/test_probe.py`, `deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh`, `deploy/craft/render-boundary/test_build_volume_free_diagnostics_candidate.py`, and `docs/plans/2026-09-29-craft-107-t14-pre-diagnostic-trace-review-fix1-report.md`.

Preserve all other existing T14 files and edits. Do not change barriers, call order, timeouts, policy/network behavior, receipt payloads, browser configuration, candidate source identity, or preview hashes. No Docker, browser, network, commit, stage, issue mutation, or subagents. Parent runs Docker only after independent validation and Review.

## Required verification

1. RED: add the two required labels/order assertion and run the focused test before implementation; capture expected failure.
2. GREEN: implement only the two wrappers; update candidate builder and topology-test probe hashes after source freeze.
3. Run the targeted stage test, full `test_probe` suite, candidate-builder suite (`python3 -m unittest deploy.craft.render-boundary.test_build_volume_free_diagnostics_candidate -v`), Python compile, `sh -n`, `git diff --check`, and a trailing-space/tab scan of all five owned paths.
4. Report pre/post hashes, exact commands/results, task-only diff, and no-live-action confirmation.

## Completion

Report exact modified paths and hashes, RED/GREEN outcomes, any limitations, and update `.superpowers/sdd/2026-09-29-craft-107-t14-pre-diagnostic-trace-review-fix1/progress.md`. Do not claim T14 acceptance.
