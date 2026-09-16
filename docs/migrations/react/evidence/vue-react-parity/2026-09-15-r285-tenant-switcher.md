# R285 — Tenant switcher implementation

- Scope: React `PlatformShell` and `main.tsx`, using the existing `createWebScopeRuntime` contract.
- Behavior: authenticated memberships render as an accessible `listbox`; the active tenant is marked with `aria-selected`; selecting another tenant calls `auth.switchTenant`, persists the returned bearer credential, commits the scoped tenant, and reloads the platform home.
- Verification: tenant-switcher jsdom interaction test passes; `pnpm run typecheck:web` passes; latest `pnpm test:web` passes with 909 tests; `git diff --check` passes.
- Boundary: the test uses a controlled auth client. Live provider callback, protected backend data refresh, paired Vue visual comparison, responsive, Wails/native evidence remain open.
