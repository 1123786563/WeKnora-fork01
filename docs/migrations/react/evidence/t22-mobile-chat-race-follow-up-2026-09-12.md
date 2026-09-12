# T22 mobile chat race follow-up — 2026-09-12

## Scope

This increment closes two source-level race defects in the native chat
implementation and one desktop renderer build defect found during the
cross-client regression:

- Stream callbacks now carry a session/run token. Events from an older run or
  a different session are ignored, including the first message immediately
  after `ensureSession()` creates a session.
- Run-lifecycle writes are serialized per session, so asynchronous
  SecureStore completion cannot restore an older state after a newer stop,
  failure, or completion transition.
- The Desktop Vite configuration now resolves the shared chat session-state
  module instead of allowing the broad `@weknora/domain` alias to consume the
  deep import.

## TDD and verification

- RED: the new stream-token test failed because `isCurrentChatRun` was absent;
  the persistence test observed two concurrent writes for one session.
- GREEN: focused parity tests passed 7/7 and lifecycle tests passed 8/8.
- `pnpm test:mobile`: 77/77, exit 0.
- `pnpm typecheck:mobile`: exit 0.
- `pnpm test:shared`: 203/203, exit 0.
- `pnpm test:web`: 107/107, exit 0.
- `pnpm test:embed`: 3/3, exit 0.
- `pnpm test:desktop`: 2/2, exit 0.
- Shared, Web, Mobile, Embed, and Desktop typechecks exited 0.
- Web, Embed, and Desktop production builds exited 0 after the alias fix;
  the Web/Desktop bundles retain the existing >500 kB warning.
- `node scripts/check-react-boundaries.mjs`: exit 0.
- `git diff --check`: exit 0.
- Expo iOS export exited 0 (`/tmp/weknora-mobile-ios.zHwQbi`).
- Expo Android export exited 0 (`/tmp/weknora-mobile-android.bujSZX`).

## Evidence boundary

This is source, Node test, bundle, and Expo export evidence. No physical iOS
or Android device interaction, provider-backed SSE, native stop/resume,
production deployment, or Wails installed-app cross-OS acceptance is claimed.
T22 and T24 therefore remain `review`.
