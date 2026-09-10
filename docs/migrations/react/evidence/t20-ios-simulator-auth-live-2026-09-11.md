# T20 iOS Simulator authenticated mobile evidence — 2026-09-11

## Environment

- Source: isolated worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`.
- Native host: `com.weknora.mobile`, iPhone 17 Pro simulator, iOS 26.5.
- Metro: worktree Expo development build over LAN at `192.168.139.3:8082`.
- Backend: isolated Lite binary on `127.0.0.1:18082`, SQLite database and local files under `/tmp/weknora-react-mobile-live.yAlv9W/`; no production database, credentials, Redis, or provider was used.
- Test account: temporary `reactmobile@example.test`; the password was entered only into the simulator and is not stored in the repository.

## Red → green native compatibility finding

The first request from an unavailable/stale development endpoint surfaced
`Property 'DOMException' doesn't exist` in the React Native login error state.
Tracing the shared request boundary showed direct `instanceof DOMException`
checks in `packages/api-client/src/client.ts`, which is unsafe when the native
runtime does not expose that global. A failing regression test was added before
the fix: deleting `globalThis.DOMException` caused the request classifier to
throw a `ReferenceError` instead of returning a typed cancellation error.

The fix adds portable `createAbortError`/`isNamedError` helpers, uses them in
the shared client and Embed client, and replaces direct native stream/file
abort construction. The focused test then passed and the iOS bundle reloaded
without the ReferenceError.

## Live sequence

1. `POST /api/v1/auth/register` created the isolated owner account (`201`).
2. `POST /api/v1/auth/login` returned access and refresh tokens (`200`).
3. The iOS native login form submitted the same account through the mobile
   transport and navigated to the native knowledge-base route.
4. The first authenticated `GET /api/v1/knowledge-bases` returned an empty
   valid list; the simulator rendered `No knowledge bases available.`.
5. A real `POST /api/v1/knowledge-bases` created `iOS Live KB` (`201`) with
   description `Native simulator evidence`.
6. Metro reload caused the native route to fetch the authenticated list again;
   the simulator rendered the actual `iOS Live KB` row and its description.

This proves native login, Bearer credential propagation, authenticated list
fetching, empty-state rendering, and refresh-visible backend data on iOS. It
does not claim Android native runtime, refresh-token rotation, logout,
AppState/network recovery, upload/share, or production deployment acceptance.

## Verification after the fix

- `pnpm test:shared`: 154/154, exit 0.
- `pnpm typecheck:shared`: exit 0.
- `pnpm test:mobile`: 9/9, exit 0.
- `pnpm typecheck:mobile`: exit 0.
- `pnpm --filter @weknora/mobile exec expo export --platform ios`: exit 0;
  Hermes bundle about 3 MB.
- `pnpm --filter @weknora/mobile exec expo export --platform android`: exit 0;
  Hermes bundle about 3.1 MB.
- `node scripts/check-react-boundaries.mjs`: exit 0.
- `git diff --check`: exit 0.

The Lite process still emitted the known pre-existing `tenant_skills` schema
warning from the background reaper; it was not counted as mobile success or
silently suppressed.
