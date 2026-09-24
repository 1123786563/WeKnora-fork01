# T10 narrow shell independent validation

- **Assigned revision:** `a44679859dcb8ad0a95768b790881968f08e6696`
- **Observed HEAD:** `a44679859dcb8ad0a95768b790881968f08e6696`
- **Working tree before report:** clean
- **Scope:** `.wk-shell-1` min-width <=640px, narrow initial sidebar state without changing saved desktop preference, explicit toggle, desktop persisted state.
- **Verdict:** `DONE_WITH_CONCERNS`

## Commands and evidence

- `git rev-parse HEAD` — PASS; exact assigned SHA above.
- `git status --short` — PASS; clean before writing this validation report.
- `git show --no-ext-diff --format=fuller --no-renames a44679859dcb8ad0a95768b790881968f08e6696 -- apps/web/src/platform/PlatformShell.tsx apps/web/src/platform/platform-u.css apps/web/src/platform/platform-shell-sidebar-state.test.tsx` — inspected only the assigned changes.
- `pnpm exec tsx --test apps/web/src/platform/platform-shell-sidebar-state.test.tsx` — PASS, 5 tests, 0 failures. Covers default/edition shell behavior, desktop collapse persistence and remount restoration, narrow initial collapse with saved `false`, accessible expand/collapse labels, no preference write on narrow initialization, normal persistence on explicit expansion, outlet remains mounted, and CSS min-width override source assertion.
- Reused exact-SHA evidence in `task-10-narrow-shell-report.md`: `pnpm --filter @weknora/web test` PASS (2,353 tests); `pnpm typecheck:web` PASS; `pnpm build:web` PASS; `git diff --check` PASS. Build emitted existing CSS import-order, `calc()` spacing, and bundle-size warnings.

## Acceptance assessment

- CSS width floor: PASS by source assertion for `@media (max-width: 640px)` and `.wk-shell-1 { min-width: 0; }`.
- Narrow initial state and persisted preference: PASS by focused test: viewport 390, persisted `sidebar_collapsed=false`, starts collapsed and leaves the key unchanged.
- Interaction and accessibility: PASS by focused test for labeled expand/collapse buttons and persisted explicit toggle; main outlet remains mounted.
- Desktop preference: PASS for existing persisted collapsed restoration test. The component samples `innerWidth` at mount; no resize listener was added, and live resize behavior was not independently measured.
- Loading/empty/error/success feature states: not applicable to this shell-only change.
- Rendered responsive/browser layout: NOT VERIFIED here. The implementation report says its live regression measured 390px viewport against 600px document scroll width before this fix, but no post-fix browser measurement or screenshot is included. Controller owns that browser acceptance gate.

## Gaps and risks

JSDOM confirms state and CSS source but cannot prove actual computed layout, horizontal overflow, or behavior while resizing an already-mounted page across the 640px breakpoint. The only outstanding concern for this narrow-shell assignment is post-fix live browser measurement at narrow width and desktop/resize inspection by the controller.
