# T23 mobile configuration management follow-up — 2026-09-12

This increment adds native configuration writes for Agent, Model, and MCP
records. Skills remain explicitly native read-only because the mobile sandbox
installation flow is not implemented.

## Implemented boundary

- `ConfigurationScreen` now supports create/edit/remove for Agent, Model, and
  MCP records through the existing typed configuration SDK.
- Safe JSON configuration is validated as an object and recursively rejects
  secret-shaped keys. API keys, app secrets, and MCP tokens use the existing
  dedicated credential endpoints and are never loaded into edit fields.
- MCP transport, URL, enabled state, model type/source, and agent metadata are
  native fields. Failed writes do not optimistically remove or rewrite list
  rows; the list reloads only after a successful server response.
- The capability matrix now splits writable Agent/Model/MCP configuration
  from read-only Skills instead of describing the whole screen as read-only.

## Verification

| Command | Result |
|---|---|
| `pnpm exec tsx --test apps/mobile/src/features/management/capabilities.test.ts apps/mobile/src/features/management/configuration-form.test.ts` | 6/6 passed after the intentional capability RED run |
| `pnpm test:mobile` | 85/85 passed |
| `pnpm --filter @weknora/mobile typecheck` | exit 0 |
| `pnpm exec expo export --platform ios --output-dir /tmp/weknora-react-mobile-config-wt-ios` | exit 0; 1,169 modules bundled |
| `pnpm exec expo export --platform android --output-dir /tmp/weknora-react-mobile-config-wt-android` | exit 0; 1,192 modules bundled |
| `node scripts/check-react-boundaries.mjs` | exit 0 |
| `git diff --check` | exit 0 |

## Evidence boundary

This proves native source wiring, safe credential boundaries, form validation,
type safety, and iOS/Android JS bundleability. It does not prove live model or
MCP credentials, provider connectivity, role/tenant 403/409 permutations, or
native device interaction. Skill catalog installation and T23/T24 remain
review-level until those runtime gates are covered.
