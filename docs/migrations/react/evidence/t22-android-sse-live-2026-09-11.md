# T22 Android SSE chat evidence — 2026-09-11

## Environment

- App: `com.weknora.mobile` release APK rebuilt from the current React
  worktree.
- AVD: `test36-small`, Android API 36, serial `emulator-5554`.
- Android transport: `adb reverse tcp:18084 tcp:18084`.
- Backend: isolated FTS5-enabled Lite server on `127.0.0.1:18084`, using a
  temporary SQLite database and a temporary owner account. No production data
  was used.
- Model transport: local OpenAI-compatible SSE stub on `127.0.0.1:19000`,
  configured as the temporary Lite remote model `probe-remote-chat`.
- APK: `apps/mobile/android/app/build/outputs/apk/release/app-release.apk`
- APK SHA-256: `6aad5e918fe0bd1c646afe6f0956680ef38547d20b863e3f08abbacbe04218c5`.

## Build and native live result

The release artifact was rebuilt with the explicit Android SDK path using
`./gradlew :app:assembleRelease --console=plain`; Gradle reported `BUILD
SUCCESSFUL` with 603 actionable tasks. The APK installed with `adb install -r`
returning `Success`, and `com.weknora.mobile/.MainActivity` remained the
focused activity on the emulator.

The installed native sequence was:

1. Open the authenticated `weknora://chat` route using the persisted temporary
   SecureStore session.
2. Select the server-backed `Probe KB` knowledge base.
3. Enter `hello` and tap the native `Send` control. The mobile request carries
   the selected `knowledge_base_ids`, empty `attachment_ids`, and
   `channel: "mobile"`.
4. Wait for the real Lite SSE stream to complete. The UI rendered the user
   message `hello` and one assistant message `Hello from Android`; the final
   accessibility dump contained exactly one occurrence of each, zero `Stop`
   nodes, and zero error-alert nodes.

The native UI dump was captured at `/tmp/weknora-android-chat-postfix.xml` and
the screenshot at `/tmp/weknora-android-chat-postfix.png` during this probe.

## Transport evidence

A separate authenticated request to the same isolated Lite knowledge-chat
route with the selected KB emitted, in order, `agent_query`,
`query_understand`, `knowledge_search`, two incremental `answer` events
(`Hello`, ` from Android`), a completed `answer`, and `complete` with
`final_content: "Hello from Android"`. The search result was an explicit
empty result (`candidate_count: 0`) and the answer was marked as the local
stub fallback; this proves framing and completion handling without claiming
provider quality or production model behavior.

## Boundary

This closes the successful Android native token/SSE path against an isolated
FTS5 Lite backend and the no-duplicate completed-answer rendering observed in
the same run. It does not prove production-provider behavior, iOS native SSE,
server continuation after interruption, remote stop after assistant-id
assignment, approval/OAuth, attachment processing, steering, network recovery,
or the remaining T22/T24 release gates. T22 and T24 therefore remain
`review`; T25 remains gated.
