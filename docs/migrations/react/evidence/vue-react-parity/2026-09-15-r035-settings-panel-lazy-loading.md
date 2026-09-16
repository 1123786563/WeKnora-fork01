# React settings panel lazy-loading evidence

- Scope: `apps/web/src/settings/SettingsPage.tsx`
- Change: settings panels are loaded through `React.lazy` and rendered under the existing section loading boundary; system global settings remains eager because the current SettingsPage synchronous render contract has a dedicated test.
- Build evidence: `pnpm run build:web` succeeds; SettingsPage route chunk decreased from about 562KB to about 49KB gzip 15KB.
- Regression evidence: `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Limitation: route chunk still has a separate bundle-size warning only for unrelated visualization/dependency chunks.
