# R283 — User menu management shortcuts

- Scope: `apps/web/src/platform/PlatformShell.tsx`, aligned with `frontend/src/components/UserMenu.vue` management entries.
- Change: added localized links for members, model management, and skills settings to the authenticated admin/owner user menu. Existing viewer gating remains closed through the resolved tenant role.
- Verification: focused `platform-shell-guide-reopen.test.tsx` 4/4; `pnpm test:web` 909/909; `pnpm run typecheck:web`; `git diff --check`.
- Boundary: this is static/jsdom and authenticated React-shell evidence. Tenant switching, protected backend CRUD, same-session Vue pixel comparison, responsive, Wails/native acceptance remain open.
