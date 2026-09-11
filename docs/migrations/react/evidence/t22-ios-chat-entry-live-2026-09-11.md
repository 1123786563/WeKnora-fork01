# T22 iOS native chat entry evidence — 2026-09-11

## Environment

- App: `com.weknora.mobile` iOS Release build from the current React
  worktree.
- Simulator: iPhone 17 Pro, iOS 26.5, UDID
  `5EECD8BB-4B4A-473C-85C3-7841329FDF3C`.
- Build command: `npx expo run:ios --device 'iPhone 17 Pro' --no-bundler
  --configuration Release`.
- Build result: `Build Succeeded`, 0 errors; the app was installed and opened
  on the simulator.

## Native entry result

The authenticated knowledge-base route rendered a native `Chat` button in
the header. Tapping that button changed the native route to `chat/index` and
rendered the native chat screen with its knowledge-base selector, session
list, composer, and `Send` control.

The temporary Lite server was restarted during this probe, so the previously
persisted test bearer was invalidated by the process-local JWT secret. The
chat screen consequently showed an observable HTTP 401 load error; no iOS
SSE/token-delivery acceptance is claimed from this run.

## Boundary

This proves that the iOS release app has a reachable native entry into the
chat route without relying on a broken scheme deep link. It does not prove
iOS authenticated SSE, provider behavior, approval/OAuth, attachments,
steering, or recovery. Those remain open T22/T24 gates.

## Follow-up: authenticated native SSE — 2026-09-11 23:56 CST

- Restarted the same isolated FTS5 Lite server on `127.0.0.1:18084` and
  authenticated the temporary owner account
  `chatfts5_1789138760@example.test` through the native iOS login form.
- Started a local OpenAI-compatible SSE mock on `127.0.0.1:19000`; the
  configured tenant model `probe-remote-chat` points to
  `http://127.0.0.1:19000/v1`.
- On the iPhone 17 Pro iOS 26.5 simulator, tapped `Chat`, selected the real
  `Probe KB`, typed `hello from ios`, and tapped `Send`. The native screen
  rendered one user message and one completed assistant message, `Hello from
  iOS`; `Send` returned and no error alert or duplicate assistant appeared.
- Server-side SQLite evidence for the resulting session
  `d1f41b92-b616-43d0-896e-d69ce7c89f99` contains exactly the completed user
  and assistant rows. The authenticated `GET /api/v1/sessions` response also
  returned the session with the selected KB in `last_request_state`.
- Screenshot captured with `xcrun simctl io booted screenshot`:
  `/tmp/weknora-ios-chat-latest.png` (SHA-256
  `9e6cf4455037a98a0cc81c0ce9af6ea0949cba7ce4569bfcd844a74a4c94ec88`).

### Evidence boundary

This is real native iOS → isolated Lite → local OpenAI-compatible SSE mock
evidence, not production-provider or physical-device acceptance. It does not
close approval/OAuth, attachments, artifact download/share, steering,
background interruption/continuation, remote stop-after-assistant-id, or
production-provider gates.

## Follow-up: native Stop / interrupted stream — 2026-09-11 23:58 CST

- Replaced the mock with a delayed OpenAI-compatible stream, sent `Stop after
  token` from the same native composer, and observed the native `Stop` action
  while the request was in progress.
- After the tap, the composer returned to `Send`. The session retained the
  user message and the server-backed assistant row was incomplete
  (`is_completed = 0`); the UI displayed `Resuming…` for that incomplete row.
- SQLite evidence for session
  `d1f41b92-b616-43d0-896e-d69ce7c89f99` recorded the user message and the
  incomplete assistant row. Screenshot:
  `/tmp/weknora-ios-chat-stop-latest.png` (SHA-256
  `6b6e07c836c2197e1c1c8f7590bc4838673e8044744423968ab3f78a6cfa59dc`).

This is bounded native cancellation evidence. Because the delayed fixture
closed before a server assistant-id was persisted, it does not claim the
remote stop-after-assistant-id contract or successful background continuation.
