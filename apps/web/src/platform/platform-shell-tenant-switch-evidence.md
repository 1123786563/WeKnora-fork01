# Platform shell parity evidence

Date: 2026-09-15

Reference: `frontend/src/components/UserMenu.vue` (`switchToTenant` / `closeAll`)

Change: `PlatformShell.switchTenant` now closes both the account menu and the
tenant submenu before handling a tenant selection. Clicking the active tenant
remains a no-op while still closing the menu, matching Vue; switching to
another tenant closes the menu before awaiting the caller's navigation/session
work.

Focused evidence:

- `node --import tsx --test src/platform/platform-shell-guide-reopen.test.tsx` — 6/6 passed.
- `node --import tsx --test src/platform/*.test.ts src/platform/*.test.tsx` — 141/141 passed.
- `pnpm exec tsc -p tsconfig.json --noEmit` — exit 0.
- `git diff --check` — passed.

Acceptance boundary: jsdom interaction and static/type evidence only; no live
backend, browser screenshot, Wails, iOS, or Android acceptance is claimed.
