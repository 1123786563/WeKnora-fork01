# R130 sandbox settings shared controls

- Scope: `apps/web/src/settings/SandboxSettingsPanel.tsx`
- Change: migrated sandbox backend/configuration, provider credentials, Docker settings and endpoint controls to shared `Input`, `Select`, and `Checkbox` components while preserving Vue-shaped fields, validation, disabled retarget behavior, and payload callbacks.
- Validation: `pnpm typecheck:web` passed; Web regression passed 895/895.
- Boundary: protected sandbox CRUD/terminal runtime and same-session Vue visual comparison remain open.
