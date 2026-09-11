# T21 Android upload, detail and share evidence — 2026-09-11

## Environment

- App: `com.weknora.mobile` release APK from the React worktree.
- AVD: `test36-small`, Android API 36, serial `emulator-5554`.
- Backend: isolated Lite on `127.0.0.1:18082`, reachable through
  `adb reverse tcp:18082 tcp:18082`.
- Knowledge base: temporary owner-owned `Android native upload probe` in the
  isolated database. No production data was used.

## Live sequence

1. The authenticated Android release opened the real knowledge-base list and
   entered the server-backed `Files` route.
2. A 65-byte `.txt` fixture was pushed to the emulator's `Download` folder.
   Tapping `Upload` opened the Android system `DocumentsUI`; selecting
   `weknora-native-upload.txt` returned to the app and issued the real
   multipart upload.
3. The native list rendered `1 files` and the uploaded row with
   `weknora-native-upload.txt, processing`. The backend kept the item in
   `processing` during the bounded wait; this is recorded as backend queue
   state, not as a completed parse/search claim.
4. Opening the row rendered the server-backed `File details` screen with the
   document id, type `txt`, size `65`, and `Download and share` action.
5. Tapping `Download and share` opened Android's real Sharesheet. Its
   accessibility tree showed `Sharing 1 file` and the selected filename
   `weknora-native-upload.txt`.

## Boundary

This proves Android native file selection, multipart upload, server-backed
processing-state rendering, document detail navigation, authenticated file
download handoff, and native share invocation. It does not prove processing
completion, search/indexing, large-file behavior, cancellation, 403/413
handling, or iOS file-picker/share behavior. The Lite parser remained in
`processing`, so no stronger document-content or retrieval conclusion is
made.
