# R135 tenant and profile shared controls

- Scope: `apps/web/src/settings/TenantUserProfileSections.tsx`
- Change: tenant name/description editing and password fields now use shared `Input`/`Textarea`, preserving localized labels, auto-focus, keyboard shortcuts, validation, disabled state and submit callbacks.
- Validation: `pnpm typecheck:web` passed; Web regression passed 895/895.
- Boundary: protected tenant/profile writes and same-session Vue visual comparison remain open.
