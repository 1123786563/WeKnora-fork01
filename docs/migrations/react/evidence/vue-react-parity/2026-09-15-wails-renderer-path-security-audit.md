# Wails renderer path/security audit — 2026-09-15

## Scope

Reviewed only `apps/desktop/src` and its desktop renderer tests against the
Vue Web/Wails boundary: API/deep-link path handling, external navigation,
credential storage, window defaults, and renderer bootstrap behavior.

## Changes

- Desktop credential storage now treats an available Wails credential bridge as
  authoritative. It no longer mirrors credentials into WebView `localStorage`
  and no longer revives a stale browser token when the bridge returns empty.
- Added a regression test covering both no-localStorage mirroring and stale
  credential rejection.

## Evidence

- RED: the new regression test failed because `write()` wrote into
  `localStorage` despite the bridge being present.
- GREEN: `pnpm --filter @weknora/desktop-renderer test --
  --test-name-pattern='desktop credential'` — 6/6 passed.
- `pnpm --filter @weknora/desktop-renderer typecheck` — passed.
- Existing static tests continue to cover HTTP(S)-only external navigation,
  legacy deep-link normalization, fixed viewport defaults, Wails bridge
  detection, and desktop API-root validation.

## Acceptance boundary

This is source and renderer-test evidence only. No claim is made here for
authenticated Vue/React parity, a real Wails window click-through, external
browser launch, native window close/reopen lifecycle, or real backend
credential persistence. Those require a launched Wails window and remain
unverified in this record.
