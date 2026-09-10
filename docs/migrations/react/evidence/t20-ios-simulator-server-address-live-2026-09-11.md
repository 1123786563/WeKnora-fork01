# T20 mobile server-address evidence — 2026-09-11

## Environment

- iPhone 17 Pro simulator, iOS 26.5, native host `com.weknora.mobile`.
- Worktree Expo Metro bundle and the isolated Lite backend at
  `http://127.0.0.1:18082`.

## Evidence

The native `/server` route displayed the persisted server value
`http://127.0.0.1:18082` after opening the route through the app deep link.
Changing the field to `file:///tmp/weknora` and selecting `Save server` kept
the route open and rendered `Server address must use HTTP(S)`. Restoring the
HTTP address and selecting `Save server` returned to the native login route.

The value is stored through the mobile SecureStore adapter rather than a
second endpoint constant. The adapter normalizes a trailing slash, rejects
non-HTTP(S) schemes, and deletes the override for an empty value; the runtime
falls back to the build-time `EXPO_PUBLIC_API_BASE_URL` only when no override
is stored.

This is a native iOS simulator proof of server-address validation and
persistence. It does not prove Android runtime, multi-server production
deployment, or full login/OIDC/refresh recovery acceptance.
