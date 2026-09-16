# R033 — skill sandbox pick-row progress fan-out

Date: 2026-09-14

## Vue baseline

`frontend/src/views/settings/SkillSettings.vue` renders busy sandbox-pick rows with a compact circular progress indicator and a percentage. `sandboxPickPercent` reads the per-skill `install-events` registry; the row remains actionable through “查看安装进度”. Rows that are not busy do not render the progress affordance.

## React implementation

`apps/web/src/settings/SkillSettingsPanel.tsx` now subscribes to `client.sandbox.skills.followInstallEvents(configId, skillId, ...)` for every busy pick row in both the add wizard and install drawer. Subscriptions are keyed by sandbox config, cleaned up with `AbortController`, and reuse the Vue-compatible `installProgressPercent` fallback (installing starts at 0; removing at 5; terminal ready/failed at 100). Busy rows render the existing 18px `ProgressRing`, percent text when an event is available, and the existing manage action. `apps/web/src/settings/skill-settings.css` supplies the row-level sizing and spacing.

## Verification

- Focused browser/SSR component suite: `node --import tsx --test apps/web/src/settings/SkillSettingsPanel.test.tsx` — 31/31 passed.
- Full Web suite: `pnpm run test:web` — 681/681 passed.
- Static diff check: `git diff --check` — passed.
- Typecheck: `pnpm run typecheck:web` remains blocked by unrelated parallel dirty changes in `apps/web/src/main.tsx` (`persistBrowserCredential`, `createBrowserCredentialAdapter`) and `apps/web/src/theme.ts` (`Document` vs `ThemeDocumentLike`).
- Browser authenticated Vue/React screenshot comparison and Wails/native verification remain open; no credentials were available for a same-account Vue capture.

Status: implementation verified by focused and full Web tests; runtime browser/native parity evidence remains open.
