# R319 platform navigation item geometry

The paired computed-style audit found a remaining shared-shell mismatch on the expanded platform navigation items. Vue `menu.vue` uses 38px rows, `8px 10px 8px 14px` item padding, 4px radius, and 600-weight 14px labels. React was using 9px vertical padding, 8px radius, extra horizontal margins, and 400/500-weight labels.

React `PlatformShell` now uses the Vue row height, inset, radius, and label weight while preserving the existing Tailwind/shadcn composition, active colors, route links, ARIA current state, collapsed layout, and keyboard behavior.

Verification:

- Vue/React computed-style comparison identified the mismatch on the authenticated `Parity KB Demo` fixture at approximately 1355x720.
- Focused platform shell suites: 30/30 passed.
- Web suite: 911/911 passed, with 0 failed/cancelled/skipped.
- `pnpm typecheck:web` and `git diff --check` passed.

The post-edit authenticated screenshot could not be reacquired after the independent browser session rehydrated to the public login surface; therefore this slice remains review evidence, not final acceptance. Responsive, other-locale, dark-theme, protected role, Wails/native, and full matrix evidence remain open.
