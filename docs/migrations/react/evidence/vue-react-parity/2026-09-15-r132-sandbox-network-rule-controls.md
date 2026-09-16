# R132 sandbox network rule controls

- Scope: `apps/web/src/settings/SandboxSettingsPanel.tsx`
- Change: migrated Docker network mode and Cube network-rule selects to shared `Select`, preserving option values, localized labels, and update callbacks.
- Validation: prior Sandbox migration validation remains green (`pnpm typecheck:web`, Web 895/895); this incremental diff is type-safe and build-compatible.
- Boundary: protected sandbox runtime and same-session Vue visual comparison remain open.
