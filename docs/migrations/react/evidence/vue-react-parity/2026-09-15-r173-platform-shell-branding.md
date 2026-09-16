# R173 platform shell branding and search affordance

- Scope: `apps/web/src/platform/PlatformShell.tsx`.
- Change: sidebar branding now uses the existing WeKnora logo asset and the expanded shell exposes a localized command-palette search affordance beside collapse, preserving navigation and collapse behavior.
- Validation: platform shell/palette focused tests passed 19/19; Web typecheck and diff check passed.
- Boundary: browser visual comparison and native shell screenshots remain open.
