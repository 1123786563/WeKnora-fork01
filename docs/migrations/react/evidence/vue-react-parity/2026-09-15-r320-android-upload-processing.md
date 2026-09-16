# R320 Android native upload and processing detail (2026-09-15)

From the authenticated Android document surface, the native file picker was opened, the existing small text fixture was selected, and the app returned with a new document row. UIAutomator then showed the uploaded file in `processing` state and exposed its detail screen with localized 返回, 文件详情, 预览, 下载并分享, document ID, type and size controls.

This proves the native upload-entry/file-picker handoff and processing/detail state. The isolated backend currently does not advance this fixture beyond `processing`, so completed preview and post-processing mutation evidence remain open. The temporary document should be removed through the authenticated cleanup path after the run.
