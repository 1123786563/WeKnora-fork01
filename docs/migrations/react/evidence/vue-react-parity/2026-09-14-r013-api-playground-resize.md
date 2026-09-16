# R013 API Playground drawer resize — 2026-09-14

## Vue baseline

`frontend/src/components/settings/SettingDrawer.vue` uses a right-side body-attached drawer with a visible vertical separator. Width starts at the configured 640px, clamps to 560–960px and the viewport, and persists on mouseup under `setting-drawer:width:api-playground`.

## React implementation

`apps/web/src/integrations/ApiPlaygroundDrawer.tsx` now exposes the same separator handle, drag direction, body cursor/user-select lock, viewport/max/min clamp, resize listener, and localStorage key. The drawer remains a body Portal and keeps its existing overlay and keyboard behavior.

## Verification

- Pure clamp assertions: 560px minimum, 960px maximum, viewport cap.
- DOM drag sequence: 640px → 940px with persisted `setting-drawer:width:api-playground=940`.
- API Playground tests: 10/10.
- Web TypeScript check: pass.
- Browser computed-style and Vue same-condition screenshot comparison: pending.
