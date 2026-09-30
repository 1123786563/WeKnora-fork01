# Issue #82 T9 OCR Finding Repair Plan — Round 2

**Source plan:** `docs/plans/issue-72-plan-82-t9-webhook-replay-fix-r1.md`.
**Finding source:** `.superpowers/sdd/issue-72-plan-82-t9-fixture-r1/ocr-t9-webhook-replay-shell-range.md` (first OCR round findings, with review_round_failed on the second round).
**Base / current implementation:** `8329b85d4dfff301d03f94406dfc829d87cb5b26` → `60eceeebee9fada534ec7b4d63c5a54207e03fac`.

## Goal

Resolve both low-severity OCR findings in the T9 environment preparation script, then rerun OCR until the changed script has a complete review result.

## Global Constraints

- Only edit `docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh` and the assigned plan/ledger/report files.
- For `docker compose ps -q db`, select one container ID before trimming whitespace so multiple scaled IDs cannot be concatenated.
- Keep the `POSTGRES_USER` and `POSTGRES_DB` fallbacks aligned with the Compose defaults in `deploy/lago/compose.yaml`; document that coupling at the fallback.
- Do not start services or run live T9. Preserve offline/live evidence distinction.
- Commit only the assigned changes; preserve unrelated untracked files.

## Task 1 — Harden environment discovery against OCR findings

**Dependencies:** OCR report at the source above.
**Role:** `mechanical_worker`; validator `backend_validator`; reviewer `reviewer`.
**Owned files:** `docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh`, `docs/plans/issue-72-ledger-82.md`, this plan and its SDD report.
**Consumes:** Existing environment prep code and `deploy/lago/compose.yaml` defaults.
**Produces:** First DB container ID is selected deterministically; fallback values carry an explicit synchronization comment; ledger records finding dispositions and validation.

**Steps:**

1. Add `head -n 1` before whitespace removal in the Compose DB container lookup.
2. Add a nearby comment documenting that `lago` fallbacks must match `deploy/lago/compose.yaml` defaults.
3. Run `bash -n`, `git diff --check`, and a focused shell assertion/source inspection. Do not start Compose services.
4. Commit the owned files and record exact HEAD, file hash, commands, and results.
5. Obtain independent review, then rerun OCR on the complete changed shell file; OCR must finish with full coverage and no unresolved critical/high/medium findings.

**Acceptance:** Both OCR low findings are fixed; shell syntax and diff checks pass; reviewer accepts; final OCR completes without review-round failures. This task does not prove live T9/AC3/AC4.

## Review Focus

- Multiple container IDs cannot be concatenated into an invalid identifier.
- Fallback values remain explicitly tied to the pinned Compose defaults.
- No live service was started; exact validation and OCR coverage are recorded.

## Task 1 execution record

- Status: implemented; awaiting independent review and final OCR.
- Script changes: container lookup selects `head -n 1` before whitespace removal; fallback comment points to `deploy/lago/compose.yaml`.
- Validation: `bash -n docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh` and `git diff --check` passed. No Compose services or live T9 were started.
- Detailed evidence: `.superpowers/sdd/issue-72-plan-82-t9-ocr-fix-r2/task-1-report.md`.
