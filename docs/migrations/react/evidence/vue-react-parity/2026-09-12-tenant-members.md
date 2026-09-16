# Tenant members parity evidence

Date: 2026-09-12

## Scope

The React Settings `members` entry now uses the existing tenant member, invitation, and audit APIs. It covers explicit loading/error/empty states, viewer read-only behavior, admin/owner search and pagination, invitation with the Vue contributor default, share-link generation/copy fallback, role-permission summary, current-user self-edit/remove protection, invitation and pending-invitation revoke, role changes, member removal with confirmation, and lazy audit-log loading. No duplicate identity API was introduced.

This remains `implementing`. The Vue pending-invitation table and revoke/copy actions, permission popover, audit drawer, exact localized copy and visual states, authenticated browser, real-backend, Wails, iOS and Android evidence remain open.

## Evidence

| Layer | Command / result | Classification |
|---|---|---|
| Component SSR | `pnpm exec tsx --test apps/web/src/settings/TenantMembersPanel.test.tsx` — 2 passed, 0 failed | focused component evidence |
| Web typecheck | `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit` — passed | static integration evidence |
| Browser / real backend / native | not completed; protected route requires authenticated SSO and provider/device environments | missing evidence |

## Remaining work

Do not move R038 to `accepted` until the Vue child surfaces and protected runtime/visual/native evidence are closed.
