# Task23 Report — Submission Material Identity

**BASE:** `1f53403a08fabe57b633b28c14fa0b6ccecb3515` (Task4 source plus corrected Task23 plan)

**Scope:** `ApplicationPage.tsx` now passes its existing `careerMaterialId` to `ProgressPage`; `ProgressPage.tsx` forwards that exact optional ID to the existing `SubmissionPage`. No API lookup, guessed ID, or SubmissionPage behavior change was added. Tests cover exact known export submission and missing material behavior (only explicit unknown is available, with the existing unavailable hint).

## Verification

- RED: `pnpm --filter @weknora/web exec node --import tsx --test src/career/ProgressPage.test.tsx` — failed as expected because the known export option was absent from the ProgressPage entry.
- GREEN: same ProgressPage command — 14/14 passed.
- `pnpm --filter @weknora/web exec node --import tsx --test src/career/SubmissionPage.test.tsx` — 19/19 passed.
- `pnpm typecheck:web` — passed.
- `git diff --check` — passed.

The new integration assertion verifies export `export-known`, material `material-current`, version 7 is offered and the submitted payload uses that exact binding. The absent-material case verifies no export lookup occurs, no fabricated version appears, the explicit unknown option remains enabled, and the unavailable guidance is visible.

**HEAD:** `9f8ce1729571992d5b37ce4ebc8982d0e518dd85`.

## Task25 follow-up — parent integration coverage

Added an ApplicationPage-level test that starts with the existing `?material=` pointer, creates the application, lets the mounted MaterialPage restore the actual material and publish its ID through `onMaterialId`, then opens progress confirmation and records the chosen export. The assertion verifies the exact loaded material ID, export ID, and version binding. No production code changes were needed.

- `pnpm --filter @weknora/web exec node --import tsx --test src/career/ApplicationPage.test.tsx` — 18/18 passed.
- `pnpm --filter @weknora/web exec node --import tsx --test src/career/ProgressPage.test.tsx` — 14/14 passed.
- `pnpm --filter @weknora/web exec node --import tsx --test src/career/SubmissionPage.test.tsx` — 19/19 passed.
- `pnpm typecheck:web` — passed.
- `git diff --check` — passed.

Task25 commit: `47f31ff3698fae9f5adda21d9930bd4321f8989f`.
