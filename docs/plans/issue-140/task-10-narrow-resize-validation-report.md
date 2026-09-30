# T10 narrow sidebar resize independent validation

- **Assigned source revision:** `2d391912b9015866654ec25e727848061b9b4923`
- **Observed HEAD at validation:** `94d83b2a09fe5a05cfcd45a0287ca50ad29dc2e4` (docs-only descendant; source/test tree matches assigned revision)
- **Working tree before report:** clean
- **Verdict:** `DONE_WITH_CONCERNS`

## Commands and evidence

- `git rev-parse HEAD` — `94d83b2a09fe5a05cfcd45a0287ca50ad29dc2e4`.
- `git diff --stat 2d391912b9015866654ec25e727848061b9b4923..HEAD` — only `task-10-narrow-resize-report.md` documentation changed after the assigned code revision.
- `git show --no-ext-diff --format=fuller 2d391912b9015866654ec25e727848061b9b4923 -- apps/web/src/platform/PlatformShell.tsx apps/web/src/platform/platform-u.css apps/web/src/platform/platform-shell-sidebar-state.test.tsx` — inspected the assigned source/test changes.
- `pnpm exec tsx --test apps/web/src/platform/platform-shell-sidebar-state.test.tsx` — PASS, 8 tests, 0 failures. Run at docs-only descendant `94d83b2a0`, whose source/test tree includes the assigned revision unchanged. Covers 1024→390→1024 for stored `false` and `true`, narrow initial collapse, accessible expand/collapse labels, narrow overlay class, preference preservation, re-entry closing the overlay, outlet mounted, and CSS source assertions for min-width reset and fixed overlay.
- Reused exact-SHA implementation evidence in `task-10-narrow-resize-report.md`: `pnpm --filter @weknora/web test` PASS (2,356 tests); `pnpm typecheck:web` PASS; `pnpm build:web` PASS; `git diff --check` PASS. Build emitted existing CSS import-order, `calc()` spacing, and large-chunk warnings.

## Acceptance assessment

- Desktop preference `false`: PASS; resizing to 390 collapses without storage mutation, returning to 1024 restores expanded state.
- Desktop preference `true`: PASS; it remains collapsed through narrow mode and is restored on return to 1024.
- Narrow toggle and overlay: PASS; narrow expansion uses the overlay class, retains an accessible collapse button, closes back to the rail, and leaves desktop preference untouched.
- CSS width floor: PASS by source assertion for `.wk-shell-1 { min-width: 0; }` at <=640px. Overlay source assertion confirms fixed positioning at <=640px.
- Live rendered layout: NOT VERIFIED here. The implementation report explicitly assigns 390px measured width/screenshot and visual stacking to the controller's browser acceptance.

## Remaining concern

JSDOM validates state transitions and CSS source, but does not prove actual viewport scroll width, rendered overlay stacking, or interaction with browser-specific layout. Controller live browser check remains necessary.
