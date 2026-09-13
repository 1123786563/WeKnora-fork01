# 2026-09-13 round-3 live dual-end batch sweep + theme-default fix

## Environment
- Worktree .worktrees/react-multiclient, branch codex/react-multiclient.
- Backend :8080 via make dev-app (docker dev infra up). Vue :5180 (vite dev).
- React :5181 (vite dev) requires VITE_API_BASE_URL=http://localhost:8080 —
  apps/web/vite.config.ts has NO dev proxy; without the env var every
  same-origin /api call 404s. Recorded as a startup fact for future sweeps.
- Account parity-test@local.dev / tenant 10000 (verified via POST
  /api/v1/auth/login and an interactive browser login on both ends).

## Sweep
- .parity-tools/accept-batch.cjs now seeds guide-done keys on both ends
  before captures (weknora:new-user-guide-done:v1 plus the seven
  weknora:contextual-guide-* keys; same key scheme both impls) so
  first-visit overlays no longer dominate the diff.
- 20 routes x 2 ends captured to screenshots/accept-20260913-round3/
  (vue-*/react-*.png + text-fingerprint .json), 40/40 ok.

## Pixel ranking (diff>16 noise floor, 1440x900)
- Non-settings routes: kb-list 6.6, kb-documents 5.9, chat-session 4.2,
  agents 4.1, creatChat 4.0, organizations 3.8, kb-faq 3.4 (%).
- settings-* and integrations cluster at 37.9-41.6%.
- Heatmap attribution (/tmp/diffheat-*): settings deltas are dominated by
  the blurred backdrop amplifying underlying-page noise; both overlays are
  value-identical (rgba(0,0,0,.5) + blur(4px); Vue Settings.vue:652-659,
  React styles.css:560-570). Actionable in-drawer diffs registered for a
  settings visual slice: (a) drawer heading vertical rhythm ~10px;
  (b) font-size segmented control selected state (Vue solid green bg +
  white text vs React tinted bg + green text); (c) native select vs
  TDesign select chrome (already a known item).

## Theme default fix (live-discovered, commit 8493557e)
- settings-general showed 主题模式 Vue=浅色 vs React=跟随系统.
- Root cause: packages/domain local-preferences defaulted missing/invalid
  stored theme to system; Vue useTheme.ts:14-15 resolves to light.
- TDD: domain expectations flipped first (2 failed as system!=light), then
  the domain default + panel catch fallback fixed. domain 4/4, settings
  panels 149/149; post-fix live screenshots show 浅色 on both ends.

## Limits
- Screenshots are one logged-in admin-capable account, zh-CN, 1440x900,
  light theme, one data fixture set; they do not by themselves move any
  matrix row to accepted.
- Dynamic regions (streaming chat, toasts) excluded by route choice.
