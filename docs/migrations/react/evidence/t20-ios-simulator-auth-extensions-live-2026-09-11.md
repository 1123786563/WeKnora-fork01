# T20 mobile auth and workspace extensions — 2026-09-11

## Environment

- iPhone 17 Pro simulator, iOS 26.5, native host `com.weknora.mobile`.
- Worktree Expo Metro bundle and the isolated Lite backend at
  `http://127.0.0.1:18082`.
- Existing authenticated owner session from the T20/T21 simulator evidence;
  no production credentials were added.

## Workspace membership

Opening the native `workspace` route called the real `GET /api/v1/auth/me`
contract and rendered `reactmobile's Workspace · owner · Current`. The mobile
runtime stores the selected positive-safe-integer tenant id in SecureStore,
adds it as `X-Tenant-ID` for non-Embed requests, and exposes a server-backed
`switchTenant` seam; malformed membership entries are discarded by the strict
pure parser rather than converted into selectable spaces.

## Registration, invitation and OIDC surfaces

The native auth stack now exposes account registration, invitation lookup and
invite registration through the existing typed auth client. The OIDC start
path obtains the server authorization URL and state, stores the state in
SecureStore, opens the provider URL, and consumes only the signed callback
payload from the `weknora://oidc` deep link. The callback parser has focused
coverage for success, provider errors, malformed payloads, and optional state
comparison.

On the isolated Lite server, selecting `Continue with SSO` rendered the real
server response `Single sign-on is not enabled on this server`; the mobile UI
did not treat the unavailable provider as a successful login.

This is native iOS simulator and source/contract evidence. A successful OIDC
provider callback, registration/invite permission matrix, Android native
runtime, and refresh/logout/AppState/network recovery remain open.
