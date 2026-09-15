# R359 — Knowledge-base connector credential fields live parity

Date: 2026-09-15

Vue `DataSourceEditorDialog.vue` defines connector-specific fields. For Feishu
the credentials step contains App ID, App Secret, and optional Base URL rather
than a generic JSON editor; RSS uses feed URLs in connector settings.

React `DataSourcesPage.tsx` now defines Vue-derived field maps for Feishu, Lark,
Feishu Drive, Lark Drive, Notion, Yuque, Tencent IMA, GitLab, and RSS. Field
edits serialize back through the existing `credentialsText`/`settingsText`
key-value boundary and `buildDataSourceInput` remains unchanged.

Authenticated React AX after selecting Feishu shows App ID, password App Secret,
optional Base URL, connector settings, and the existing schedule/strategy
controls. The generic credential textarea is no longer rendered for Feishu.

Verification: authenticated local browser AX comparison; no save or provider
credential submission; Web regression 963/963; `typecheck:web`; `git diff --check`.
Prerequisite guide, live connection test, resource/strategy steps, mutation
outcomes, Wails/native, provider sandbox/live, and production acceptance remain
unestablished.
