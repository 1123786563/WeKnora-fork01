# R319 Android native authenticated KB flow (2026-09-15)

Using the installed Release APK (`com.weknora.mobile`) on the `test36-small` Android 16 emulator:

- The app launched into the authenticated tenant `Parity KB Demo` workspace without a login prompt.
- The KB list exposed the real `Parity KB Demo` card, `parity test data` description and `文档 · 0 项` count.
- Tapping the card opened the document surface. Accessibility/UIAutomator inspection exposed `返回文档列表`, Wiki, 问答库, 图谱, 数据源管理, 上传文件, search, root-folder and the localized empty-document state.
- Screenshots were captured for the authenticated KB list and document detail at 720×1440.

This proves Android install, launch, persisted authentication and KB list/detail rendering against the available fixture. The fixture has zero documents, so upload, completed preview, editor, graph data, chat success and protected mutation flows remain open.
