# Issue #82 T9 OCR Finding Repair Plan — Round 5

**Source plan:** `docs/plans/issue-72-plan-82-t9-ocr-fix-r4.md`.
**Finding source:** `.superpowers/sdd/issue-72-plan-82-t9-ocr-fix-r4/task-1-review.md` (Medium R4-F1).
**Finding:** The sourced script treats shell variables as Compose overrides even if they are not exported. The `docker compose` child cannot see unexported variables, so the resolver may diverge from Compose.

## Goal

Make the resolver honor only exported shell values, matching the environment visible to the `docker compose` child, then fall back to `lab.env` and Compose defaults.

## Global Constraints

- Only edit the T9 environment preparation script and assigned plan/ledger/report files.
- Do not change the script's sourced/executed failure semantics.
- No live services or T9 run.
- Commit only assigned changes; preserve unrelated work.

## Task 1 — Align resolver with child process environment

**Dependencies:** Medium finding R4-F1; implementation HEAD `631b4395a15fff3a45ef465dc07e871f08f2d985`.
**Role:** `mechanical_worker`; validator `backend_validator`; reviewer `reviewer`.
**Owned files:** `docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh`, `docs/plans/issue-72-ledger-82.md`, this plan and its SDD report.
**Consumes:** POSIX child-process environment behavior and Compose's shell-over-env-file interpolation precedence.
**Produces:** Resolver order: exported shell value → env-file value when shell variable is not exported → `lago` default. An exported empty value follows `${VAR:-lago}` and resolves to `lago`; unexported shell values are ignored by the Compose override layer.

**Steps:**

1. Detect the child-visible/exported `POSTGRES_USER` and `POSTGRES_DB` values (including exported-empty versus absent) without treating unexported shell locals as overrides.
2. For absent exports, use the selected `lab.env` values; retain the existing Compose default fallback for absent or empty values.
3. Add deterministic coverage for exported nonempty, exported empty, unexported with conflicting env-file value, unset with env-file value, and both absent.
4. Keep organization discovery and webhook polling wired to the same resolved values.
5. Run `bash -n`, `git diff --check`, and independent review/validation; no live stack.

**Acceptance:** All five precedence cases match the effective child environment and Compose semantics; no previous behavior regresses; exact HEAD/hash and checks are recorded.

**Execution status:** Task 1 implemented and the requested local checks passed. The independently assigned validation/review and final OCR remain separate gates.

## Review Focus

- Only exported values can influence Compose interpolation.
- Exported empty values resolve through `:-lago`.
- Unexported values do not override env-file values.
- The same resolved DB labels are used for organization discovery and test polling.
