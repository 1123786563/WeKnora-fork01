# React/Vue parity evidence — system global settings copy

- Scope: `apps/web/src/settings/SystemGlobalSettingsPanel.tsx`
- Change: localized high-risk confirmation, group tabs, empty state, restart/secret metadata, enable/reset labels and save/reset status fallbacks for zh-CN, en-US, ja-JP, ko-KR and ru-RU.
- Validation: `pnpm run test:web` (891/891), `pnpm run typecheck:web`, `git diff --check`.
- Limitation: runtime authenticated evidence for every locale and mutation path remains outstanding.
