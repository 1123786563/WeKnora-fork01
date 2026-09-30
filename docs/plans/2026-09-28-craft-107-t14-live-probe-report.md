# Craft #107 T14 live renderer probe attempt

## Scope

This is a diagnostic continuation of the exact local T14 renderer image work recorded in `docs/plans/2026-09-28-craft-107-t14-exact-local-export-fix3-reviewfix-report.md`. It does not change the renderer source or assert T14 acceptance.

## Exact execution

- Date: 2026-09-28 (Asia/Shanghai).
- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`.
- Renderer candidate: `sha256:a2dc460f05d310ad0de3f82658a83676d71db6628832ad0d9958075d3893b590`, inspected as `linux/arm64`, `RepoTags=[]`.
- Policy helper: `craft-t14-policy-helper:2026-09-24`, inspected locally as image ID `sha256:46bf8a9b656436d5cf2b6bac15f846f384d812a47012acf88da1141a67becb9a`.
- Command: `CRAFT_RENDER_BOUNDARY_IMAGE=sha256:a2dc460f05d310ad0de3f82658a83676d71db6628832ad0d9958075d3893b590 ./deploy/craft/render-boundary/run-probe.sh /tmp/craft107-t14-live.aN19RZ`.
- Exit: nonzero; no `evidence.json` or `host-evidence.json` was produced.

## Observed outcome

The script passed immutable image identity checks and started the renderer with the configured constrained container policy. It reported Python `/opt/venv/bin/python3`, Chromium 153.0.8010.52, ChromeDriver 153.0.8010.52 and Selenium 4.35.0. Selenium returned `SessionNotCreatedException: Chrome instance exited`; therefore the browser matrix did not run and no browser evidence was accepted.

`barrier-failure.json` records `status=incomplete`, no completed policy windows and the host receipt read failure after the renderer exited. `cleanup.json` records `status=verified-clean`, empty cleanup errors and `renderer_absent_confirmed=true`. The incomplete result is not an acceptance pass.

Evidence directory: `/tmp/craft107-t14-live.aN19RZ` (machine-local diagnostic output, not a durable repository artifact).

## Disposition

The immutable candidate selection and cleanup boundary were exercised. T14 live acceptance remains unverified. Next work is to diagnose the Chromium startup failure within the existing T14 worktree, preserve the same runtime constraints, rerun the full probe and independently review the resulting complete evidence before changing T14 status.
