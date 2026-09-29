# Task 1 Report — exported DB environment precedence

Date: 2026-09-30
Plan: `docs/plans/issue-72-plan-82-t9-ocr-fix-r5.md`
Finding: R4-F1 (unexported shell values are not visible to child Compose).

## Change

`prepare_t9_env.sh` now inspects each variable's export attribute. An exported value (including empty) is selected; otherwise the value is read from the selected `lab.env`. The existing empty-to-`lago` fallback remains in place. The resolved values still feed both the organization `psql` query and `LAGO_INTEGRATION_DB_USER` / `LAGO_INTEGRATION_DB_NAME` exports.

## Verification

- `bash -n docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh` — passed.
- `git diff --check` — passed.
- Offline deterministic resolver harness — passed for exported nonempty, exported empty, unexported conflicting values, env-file-only values, and absent values; both user and DB variables were asserted in every case.
- Static wiring assertions — passed for the organization query and integration exports using the resolved variables.
- No live services or T9 run were started.

The absent-value case was verified to yield empty from the resolver; the script's existing subsequent fallback sets both labels to `lago`. Independent validation, code review, and final OCR are pending.
