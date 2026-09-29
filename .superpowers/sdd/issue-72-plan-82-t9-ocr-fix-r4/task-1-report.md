# Task 1 Report — Match Compose database label precedence

Date: 2026-09-30
Plan: `docs/plans/issue-72-plan-82-t9-ocr-fix-r4.md`

## Change

Updated `docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh` to check whether `POSTGRES_USER` and `POSTGRES_DB` are present in the shell environment. A set value wins over `lab.env`; a set-empty value is preserved until the existing nonempty fallback selects `lago`. An unset shell variable uses `env_value` from the selected env file, then falls back to `lago` if absent/empty. Organization discovery continues to use these resolved values, which are also exported for webhook polling.

## Verification

- `bash -n docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh` — PASS.
- `git diff --check` — PASS.
- Python source assertions for both shell presence guards, organization query arguments, and exported DB labels — PASS.
- Deterministic shell resolver checks: nonempty shell override wins; set-empty shell override selects `lago`; unset shell variables use env-file-only values; absent env-file keys select `lago` — PASS.
- No Docker/Compose services or live T9 were started.

## Status

Implementation and assigned checks complete. Independent validation, task review, and final OCR are pending. Live T9/AC3/AC4 remain unverified.
