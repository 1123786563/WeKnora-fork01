# R133 system-global shared switch contract

- Scope: `apps/web/src/settings/SystemGlobalSettingsPanel.tsx` and its regression test.
- Change: boolean system settings now use the shared accessible `Switch` control; the test contract asserts the semantic `role="switch"` instead of coupling to a native checkbox implementation.
- Validation: `pnpm typecheck:web` passed; Web regression passed 895/895.
- Boundary: protected system-setting save/reset runtime and same-session Vue visual comparison remain open.
