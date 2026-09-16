# R027 settings wrapper slice — 2026-09-13 (items A–F)

Branch `codex/react-multiclient`. Vue baselines: `frontend/src/views/settings/*.vue`, `frontend/src/components/ModelEditorDialog.vue` (authoritative, read-only). Live evidence pairs in `screenshots/settings-wrapper-slice/` (`vue-settings-*.png` vs `react-settings-*.png`, captured against :5180 / :5181, login parity-test@local.dev, tenant 10000).

## Red/green

TDD: tests were extended/added first, then implemented.

- Red (before implementation): `surface.test.ts` + `model-settings.test.ts` failed to load (missing `settingsSectionHeading` / `modelFieldErrorKey` / `subsectionToFilter`); `ModelSettingsPanel.test.tsx` 15/21 failing (new header/card/combobox/blur/ESC/subsection tests + menu-ized actions); `SettingsPage.test.tsx` 6/7 failing.
- Green (final): **70/70 passing** across `surface.test.ts` (19), `model-settings.test.ts` (18), `ModelSettingsPanel.test.tsx` (21), `ModelDebugPanel.test.tsx` (4), `SettingsPage.test.tsx` (8).
- `tsc -p apps/web/tsconfig.json --noEmit`: 0 errors under `src/settings` (the remaining `TenantMembersPanel.tsx` error predates this slice and belongs to another agent's in-flight edit).

## A — SettingsPage wrapper

| Item | Vue evidence | Change |
| --- | --- | --- |
| A1 no per-section 刷新 | Settings.vue drawer has no section-level refresh; every section component owns its own header | removed the `wks-reload` button from the wrapper heading |
| A2 loading copy | Vue uses shared loading copy; never an API domain | `Loading from {section.apiDomain}…` → localized `common.loading` (加载中…) |
| A3 general mounts | GeneralSettings.vue lines 3-6 (h2 常规设置 + 配置语言、外观等基础选项), rows lines 8-136 | `GeneralPreferencesPanel` rebuilt and mounted for `section=general`; language select (language.* keys), theme light/dark/system (`local-preferences`), UI/code font selects with live preview boxes (`font.sansPreview` / `font.monoPreview`), font-size segmented 小/正常/大 (`font.size.*`); localStorage: `locale`, `weknora-theme`/`weknora-font-size` via `readLocalPreferences`, `font_sans`/`font_mono` (see gaps) |
| A4 no raw payload dump | Vue never renders raw payloads | removed `<dl className="wk-settings-values">` and the read-note fallback |
| A5 localized tenant/userprofile | TenantInfo.vue lines 3-186 (setting rows, inline name/description edit with 保存/取消, Enter/Esc semantics lines 56-66/92-98), UserProfile.vue lines 24-167 | new `TenantUserProfileSections.tsx`: 空间 ID/名称/描述(inline edit)/业务/状态/创建时间/存储配额 rows + UserProfile 用户 ID/用户名/邮箱/注册时间/修改密码 rows; shared keys `tenant.details.*`, `tenant.storage.*`, `tenant.api.*`, `userProfile.changePassword.*`, `auth.password*`; `TenantDeleteZone.tsx` localized with `tenant.deleteDangerZone.*` |

Wrapper headings (title + description) now come from `settingsSectionHeading(locale, key)` in `surface.ts` (localized key map); sections that render their own Vue header (general, models) skip the wrapper heading to avoid doubles.

## B — ModelSettingsPanel

| Item | Vue evidence | Change |
| --- | --- | --- |
| B1 header | ModelSettings.vue lines 3-21: h2 模型配置 + subtitle, single green ▶ 模型测试 text trigger (`modelSettings.actions.debugModel`, PlayCircleIcon) | heading rebuilt; 刷新 and header 添加模型 removed; `.wk-model-test-trigger` green text button with play SVG; debug drawer unchanged (`ModelDebugPanel.tsx`) |
| B2 cards | ModelSettings.vue lines 53-127 (badge lines 1052-1090, title/lock lines 1101-1147, subtitle 1149-1176, actions 1178-1207), add tile lines 128-139 + 983-1026 | Vue card markup: `.model-card__badge` (per-type colors), `.model-card__title`, `.model-card__lock` (🔒, ✎ for system admin per line 71), `.model-card__subtitle` (vendor · dimension/context · vision, `vendorLabel` lines 447-453), hover/affix actions: ellipsis menu (编辑/复制 per `getModelOptions` lines 738-770) + delete icon button; dashed `.wk-model-card--add` tile opens the existing editor |
| B3 tabs | ModelSettings.vue lines 38-46 (全部/对话/Embedding/ReRank/视觉/语音 with counts), 924-952 (tab styling) | counts kept, green active underline via CSS |
| B4a Ollama combobox | ModelEditorDialog.vue lines 109-133 (filterable select, downloaded options with size, download option line 124) | replaced `datalist` with `.wk-ollama-combobox` (role=combobox, aria-expanded) + `.wk-ollama-listbox` suggestions with sizes, ArrowUp/Down/Enter/Escape keyboard nav, `下载: <keyword>` option when unknown |
| B4b blur validation | ModelEditorDialog.vue rules lines 907-946 (name required/empty/max-100; base URL required/empty/invalid; trigger blur) | `modelFieldErrorKey` in `model-settings.ts` + per-field `.wk-field-error` under name and base URL; messages are the exact `model.editor.validation.*` copy; errors clear when fixed |
| B4c ESC draft | ModelEditorDialog.vue visible watcher lines 1063-1104 (two consecutive add-opens keep the draft when closed via ESC/overlay; edit open always rehydrates), handleCancel lines 1715-1719 (cancel resets) | document-level Escape closes the editor preserving the add draft; reopen 添加模型 restores it; 取消/保存 clear it |
| B4d subsection deep link | ModelSettings.vue lines 329-337 watch `uiStore.settingsInitialSubSection` (set programmatically via `openSettings(section, subSection)` — e.g. AgentEditorModal.vue line 3925); Vue has no URL query for it | React equivalent: `/platform/settings?section=models&subsection=chat|embedding|rerank|vllm|asr` preselects the type tab (`subsectionToFilter` + `initialSubSection` prop) |
| B4e post-save | ModelSettings.vue handleModelSave lines 657-671 (close dialog, reload list) | unchanged behavior: close editor → reload list, localized toasts 模型已添加/已更新 |

Note: the matrix' "设为默认" card action does not exist in the Vue baseline (`ModelSettings.vue` offers 编辑/复制 menu + 删除 only); the React card follows the baseline and the matrix entry is recorded as inaccurate.

## C — EnvVarSettingsPanel

Vue EnvVarSettings.vue lines 3-15: h2 沙箱密钥 (`envVarSettings.title`) + description. The panel's English "Personal environment variables" h3 was removed; the wrapper heading now renders 沙箱密钥 + 给技能和沙箱用的个人密钥，不是 WeKnora 的系统或部署配置。 via `settingsSectionHeading`.

## D — SystemInfo (added by coordinator)

SystemInfo.vue lines 21-197 (rows), 233-248 (`formatUptime`). New `SystemInfoPanel.tsx` + `systemInfoRows`/`formatUptimeText` in `surface.ts`: 应用版本 (+edition tag Standard/Lite + commit), UI 版本, 构建时间, Go 版本, 服务启动时间, 运行时长 (humanized, live from `started_at`), 数据库版本 (+迁移失败 tag), 关键词索引引擎, 向量存储引擎, 图数据库引擎 (values fall back to 未知 like Vue). Verified live: version 0.8.0 (3de3a999) Standard, uptime humanized.

## E — Memory (added by coordinator)

1. Wrapper subtitles localized from the shared registry for every section that has a key (memory/memoryWorkspaceSettings.description = the exact Vue copy; mymemory, chathistory, ollama, weknoracloud, vectorstore, parser, sandbox, skills, retrieval added; storage/members/websearch/mcp have no shared description key — see gaps).
2. Duplicate inner 长期记忆 heading removed from `MemoryWorkspacePanel` (wrapper heading renders the same pair). 我的记忆 keeps its 记忆列表 card headings (different text, like Vue).
3. Memory/config rows realigned via `settings-wrapper.css` (label left / control right / hint under label); switches styled as Vue t-switch for chat history and memory toggles.
4. English server note in `PersonalMemorySettingsPanel` replaced with `memorySettings.description` (Vue MemorySettings copy).

## F — Chathistory (added by coordinator)

ChathistorySettings.vue lines 9-55. `ConfigSettingsPanel` chathistory branch rebuilt: 启用消息索引 row (label left, right-aligned switch, hint 开启后，新的对话消息将自动索引到知识库，支持向量搜索), Embedding 模型 row aligned, English "server owns…" note removed, 保存 localized (`common.save`), and the 索引统计 section added: stats grid (已索引消息) or the centered empty box 消息索引未配置 + 启用并选择 Embedding 模型后，对话消息将自动向量化索引. Wrapper heading 消息管理 + 配置聊天历史知识库，将对对话消息自动向量化索引，实现语义搜索 (item F1) via the surface heading map. Inner duplicate 消息管理 h3 removed (F2).

## Files changed (all under the granted slice)

- Modified: `apps/web/src/settings/{SettingsPage,ModelSettingsPanel,EnvVarSettingsPanel,GeneralPreferencesPanel,TenantDeleteZone,PersonalMemoryPanel,ConfigSettingsPanel,surface,model-settings}.tsx/.ts` + their tests (`surface.test.ts`, `model-settings.test.ts`, `ModelSettingsPanel.test.tsx`)
- Added: `apps/web/src/settings/{SettingsPage.test.tsx,SystemInfoPanel.tsx,TenantUserProfileSections.tsx,settings-wrapper.css}`
- Note: `PersonalMemoryPanel.tsx` / `ConfigSettingsPanel.tsx` were on the original read-only list; the coordinator's items E/F explicitly extend the slice to them (minimal, behavior-only edits, listed above).

## Needed i18n keys (missing from packages/i18n — please port)

1. `font.sans.{system,pingfang,georgia,yahei,times,noto-cjk,dejavu-serif,sans-serif}` and `font.mono.{system,menlo,monaco,consolas,cascadia,dejavu-mono,liberation-mono,monospace}` — the panel ships byte-identical zh-CN fallbacks from the Vue locale (frontend/src/i18n/locales/zh-CN.ts font block) until ported.
2. Optional: `settings.webSearchConfig.description`, `tenantMember.description`, storage description, mcp description — those sections keep English registry fallbacks until these exist.
3. Optional: a `system.graphDatabaseNotEnabled`-style key is not needed; Vue derives 未启用 from the backend value — replicated (value or 未知).

## Remaining gaps

1. UI 版本 row shows `unknown` — React vite.config has no `__FRONTEND_VERSION__`/`__FRONTEND_COMMIT__` defines (frontend/vite.config.ts line 72 does); needs a build constant to match Vue.
2. `GeneralPreferencesPanel` persists fonts under plain `font_sans`/`font_mono` keys; Vue namespaces per user (`WeKnora_${userId}_font_sans` via preferenceStorage). Existing React local-preferences already used global keys (`weknora-theme`), so semantics follow the current React convention; per-user namespacing would need a `packages/domain` change (read-only for this slice).
3. Font/locale/theme changes persist + broadcast (`weknora:locale-changed` / `weknora:theme-changed` events) but the shell does not live-swap translations yet (a page reload applies them), unlike Vue's reactive i18n.
4. Tenant danger zone renders above the info rows (Vue places it as an aside after the list); content and copy match.
5. The dev-server model list shows the seeded builtin model only; hover actions/menu/edit flows are covered by jsdom tests but not in the stills.
6. `ModelSettings.vue` card "设为默认" from the matrix does not exist in the Vue source (see B note).
