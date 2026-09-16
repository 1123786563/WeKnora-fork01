# React/Vue parity evidence — platform API key copy

- Scope: `apps/web/src/settings/PlatformApiKeysPanel.tsx`
- Change: localized API-key creation, validation, token reveal, empty state, revoke action, table headings and date fallback for all five supported locales.
- Behavior preserved: existing create/revoke API calls, capability selection, token handling and role boundaries.
- Validation: `pnpm run test:web` (891/891), `pnpm run typecheck:web`, `git diff --check`.
- Limitation: authenticated runtime evidence for create/revoke and non-Chinese locales remains pending.
