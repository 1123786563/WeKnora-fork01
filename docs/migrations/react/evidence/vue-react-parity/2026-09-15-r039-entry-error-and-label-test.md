# Web entry copy and system label regression evidence

- Scope: `apps/web/src/main.tsx`, `apps/web/src/settings/SettingsPage.test.tsx`
- Change: localized the isolated Embed entrypoint error for five locales; updated the system settings test to assert the new user-facing localized field label.
- Validation: `pnpm run build:web`, `pnpm run test:web` (891/891), `pnpm run typecheck:web`, `git diff --check`.
