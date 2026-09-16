# General settings font parity — 2026-09-15

## Scope

Selected panel: `settings:GeneralSettings` / React `GeneralPreferencesPanel`.

Vue authority:

- `frontend/src/views/settings/GeneralSettings.vue`
- `frontend/src/composables/useFont.ts`

React files in this slice:

- `apps/web/src/settings/GeneralPreferencesPanel.tsx`
- `apps/web/src/settings/GeneralPreferencesPanel.test.tsx`

## Repairs

1. Apply persisted sans, mono, and font-size preferences when the React panel mounts, matching Vue `useFont` initialization instead of waiting for a new user selection.
2. Use the existing `@weknora/ui` semantic `bg-surface` and `border-line-neutral` utilities for both font previews, matching Vue's theme container and component-stroke tokens.

Business/API/permission/i18n behavior was not changed. Vue source was not modified.

## Evidence boundary

- RED: the two new assertions failed before the implementation (`font` root variables remained at fallback and preview semantic classes were absent).
- GREEN: `npx tsx --test src/settings/GeneralPreferencesPanel.test.tsx` — 4/4 passed.
- Static/component evidence only. No claim is made here for live browser pixel diff, Wails, iOS, Android, backend, or production acceptance.
