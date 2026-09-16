# Shell session-list slice — session list moves into the platform shell (2026-09-13)

Branch: `codex/react-multiclient` (worktree `.worktrees/react-multiclient`). No commits made.

## Gap (from 2026-09-13-chat-visual-form.md)

In Vue the session list lives IN the platform sidebar
(`frontend/src/components/menu.vue`, mounted once by
`frontend/src/views/platform/index.vue`) and is therefore visible on **every**
protected page — kb-list, agents, organizations, settings and chat (screenshots
show 昨天-grouped titles under the nav on each page). The React shell
(`apps/web/src/platform/PlatformShell.tsx`) only had the 新对话/知识库/智能体/共享空间
nav + user area; sessions lived in a separate in-page chat sidebar that only
rendered on chat routes.

## Vue references (read before implementing)

- `frontend/src/components/menu.vue` (2001 lines, read completely): logo row,
  nav items (knowledge-bases/agents/organizations/creatChat), then
  `.submenu` = session list: skeleton rows while booting, `menu.noSessions`
  empty state, `groupSessionsByDate` groups (pinned/today/yesterday/
  last7Days/last30Days/lastYear/earlier — pinned first), rows with
  `session-chat-row--active` driven by `chat/<route.params.chatid>`,
  hover ⋯ menu options built by `buildSessionMenuOptions` (pin/unpin,
  menu.renameSession, menu.clearMessages, menu.batchManage,
  upload.deleteRecord), delete/clear via inline menu confirms
  (`chatHeader.deleteConfirmTitle/Body`, `chatHeader.clearConfirmTitle/Body`),
  rename via inline title edit; list hidden when the sidebar is collapsed.
- `frontend/src/components/sessionMutations.ts`: API mutations
  (updateSession/pinSession/unpinSession/clearSessionMessages/delSession) +
  `SESSION_MUTATION_EVENT`; deleting the open session routes to
  `/platform/creatChat`.
- `frontend/src/components/sessionGrouping.ts`: date bucket classification;
  the group-mode storage key `weknora:session-group-mode` exists but menu.vue
  groups by date unconditionally.
- `frontend/src/components/SessionSidebarRow.vue`: row anatomy (pin icon,
  full title, api-owner tag, running spinner, ⋯ menu with inline confirms).
- `frontend/src/components/UserMenu.vue`: bottom user area (the React shell
  already had the user button/dropdown; unchanged this slice).
- `frontend/src/views/chat/index.vue`: **no sidebar of its own**.
- Filters in the Vue sidebar: only the source filter
  (`SessionSourceFilter`, shown when IM/embed/API channel folders exist);
  no keyword search, no group-mode dropdown.

## Changes

### packages/views/src/chat/session-sidebar.tsx (minimal, reuse-oriented)

- Extracted the grouped list body into an exported `SessionSidebarList`
  (time-group headers via existing `sessionGroupLabel`, full titles, green
  active row via `aria-current`, hover ⋯ menu). New optional `onClear`
  (清空消息) and `emptyLabel` (menu.noSessions) / `untitledLabel`
  (menu.newSession = 新会话, matching Vue mapSessionRow) props; existing
  props/markup unchanged, so SessionSidebar consumers are unaffected.
- Added `SessionSidebarShellContext`: when the platform shell provides the
  list, a SessionSidebar mounted under the shell renders **nothing**, so
  chat routes no longer double-render a sidebar.
- Exported both plus types from `packages/views/src/index.ts`.

### apps/web/src/platform/PlatformShell.tsx (owner)

- Loads sessions with the same API the chat page uses:
  `client.sessions.list({ page: 1, pageSize: 30, source: 'web' })`.
- Groups with `sessionGroups(sessions, new Date(), 'date')` (Vue menu.vue
  groups by date unconditionally → 已置顶/今天/昨天/近7天/近30天/更早).
- Route-driven selection: `chatSessionIdFromPath(pathname)` (the patched
  history keeps pathname fresh) — the current /platform/chat/:chatid path IS
  the selection.
- Renders `nav.plat-shell__sessions` with SessionSidebarList under the nav on
  **all** protected pages; hidden when the sidebar is collapsed (Vue parity).
  Empty state = menu.noSessions (暂无对话).
- Row actions wired to the same session API actions the chat page uses:
  pin/unpin (`client.sessions.pin/unpin`), rename (window.prompt +
  `client.sessions.update`), clear (confirm + `client.sessions.clear`),
  delete (confirm + `client.sessions.remove`, navigating to
  /platform/creatChat when the deleted row is the open session — Vue
  menu.vue parity). Confirms reuse the Vue copy (see keys below).
- Row selection navigates by route: on global chat routes the shell dispatches
  `SHELL_SESSION_ROUTE_EVENT` (weknora:session-route-change) and the mounted
  ChatRoutePage performs the in-place switch (it owns stream teardown and
  pushes the new /platform/chat/:id route, which the shell mirrors into the
  active highlight); on every other page the shell performs a full
  location.assign navigation (the app's route-transition convention, also how
  the 新对话 nav item behaves).
- Wraps the outlet children in `SessionSidebarShellContext.Provider`.

### apps/web/src/chat/ChatRoutePage.tsx + session-route.ts

- `SHELL_SESSION_ROUTE_EVENT` constant lives in the leaf
  `apps/web/src/chat/session-route.ts` (shared by shell + chat page; keeps
  PlatformShell out of the ChatRoutePage bundle used by
  chat-route-page-send.test.ts).
- ChatRoutePage listens for the event and switches sessions in place via its
  existing selectSession (route-driven). No sidebar code of its own.

### apps/web/src/chat/chat.css → apps/web/src/platform/shell.css (style relocation)

- The whole session-list style block (wk-chat-sidebar, wk-chat-new-chat,
  wk-chat-session-group/item/title/menu/pagination) moved from chat.css to
  shell.css (the list now lives in the shell; the same classes still style
  the fallback in-page sidebar). Added .plat-shell__sessions container +
  wk-chat-sidebar-empty (menu.noSessions row).
- main.wk-chat-page defaults to a single column and widens to 260px/1fr only
  via :has(.wk-chat-sidebar) — the chat page renders full-width under the
  shell (Vue chat/index.vue has no sidebar).

## Tests (TDD, red → green)

- New: `apps/web/src/platform/shell-session-list.test.tsx` (jsdom + act,
  same harness as new-user-guide.test.tsx). RED first: 6 failed / 0 passed.
  GREEN: **6/6**
  - (a) grouped list renders (已置顶/今天/昨天/更早 headers, full titles,
    untitled row → 新会话)
  - (b) active row follows the route (/platform/chat/session-2 → exactly one
    is-active row with aria-current="page")
  - (c) selecting a session on a chat route dispatches
    weknora:session-route-change with the target id
  - (d) ⋯ menu wires 置顶/取消置顶/清空消息/删除会话 to
    client.sessions.pin/unpin/clear/remove (confirms stubbed true)
  - (e) ChatPage under the shell renders **no** aside.wk-chat-sidebar while
    the shell list renders exactly once; deleting a non-active row only
    refreshes the shell list, route untouched
  - (f) empty list renders 暂无对话
- Focused suites (final):
  - pnpm --filter @weknora/web exec tsx --test 'src/platform/*.test.ts'
    'src/platform/*.test.tsx' 'src/chat/*.test.ts' → **132/132 pass**
    (includes the 75 previously-green platform tests and other agents' new ones)
  - pnpm exec tsx --test packages/views/src/chat/*.test.tsx
    packages/views/src/chat/*.test.ts → **48/48 pass**
- `pnpm run typecheck:shared` → clean.
- `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit` → no errors
  from this slice's files. Remaining errors are parallel agents' WIP
  (src/faq/FAQPage.test.tsx ×7, src/settings/ConfigSettingsPanel.tsx ×13,
  appeared mid-session).

## Live verify (1440×900, zh-CN, parity-test@local.dev / tenant 10000, backend :8080)

React :5181 (dev server restarted with VITE_API_BASE_URL=http://127.0.0.1:8080
after the previous process died; tracked as background job bash-67), Vue :5180
baseline. Screenshots: `screenshots/shell-sessions-slice/`

- 01-react-kb-list-shell-sessions.png — kb-list with the grouped session list
  in the platform sidebar (13 rows, 昨天 header) — matches
  05-vue-kb-list-baseline.png structure.
- 02-react-chat-shell-sessions.png — chat route: same shell list, active row
  修复后首发截图 green; **no** in-page sidebar; chat area full width.
- 03-react-chat-session-switch.png — in-place switch to another session; URL
  and active row follow.
- 04-react-row-actions-menu.png — row ⋯ menu: 置顶 / 重命名会话 / 清空消息 /
  删除会话 (danger red), matching Vue menu.vue options.
- 07-react-pin-roundtrip.png — live API round-trip: pin via row menu →
  已置顶 group appears (★ row), unpin restores (verified both ways).
- 05/06-vue-*.png — Vue baselines for comparison.

Browser console pageerrors during the run: none.

## i18n keys (reuse-first; packages/i18n NOT edited)

Used via formatMessage: menu.myChats (list aria-label), menu.noSessions,
menu.newSession (untitled rows), menu.renameSession (rename prompt), plus the
existing nav keys. Group headers and row-menu labels reuse the views-package
chat copy (chat-copy.ts / sessionGroupLabel) exactly like the chat sidebar, so
the ⋯ menu reads 置顶/重命名会话/清空消息/删除会话.

**Missing keys to add to packages/i18n later (inlined for now):**

- chatHeader.clearConfirmBody = 确认清空当前对话的全部消息？对话本身会保留，此操作无法恢复。 (inlined CLEAR_SESSION_CONFIRM)
- chatHeader.deleteConfirmBody = 确认删除当前对话？删除后将无法恢复。 (inlined DELETE_SESSION_CONFIRM)
- menu.deleteSession = 删除会话 (Vue uses upload.deleteRecord 删除记录; chat-copy has 删除会话)
- time.* group labels (已置顶/今天/昨天/近7天/近30天/更早) and the sidebar
  row-menu labels (置顶/取消置顶/重命名会话/清空消息) still live in
  packages/views chat-copy.ts; a chat/menu sidebar domain in @weknora/i18n
  would let the shell drop chat-copy imports.

## Remaining gaps

1. Shell list loads the first page only (30 rows, web source). Vue pages
   deeper buckets on scroll and shows IM/embed/API channel folders
   (SessionSourceFilter) when channels exist.
2. Shell mutations update the shell list optimistically; a simultaneously
   mounted chat page does not hear about rename/pin/clear until reload
   (delete-of-active navigates to creatChat like Vue). A shared
   session-mutation event (Vue SESSION_MUTATION_EVENT analog) would close this.
3. Rename = window.prompt, delete/clear = window.confirm (chat-page
   convention) instead of Vue's inline row-menu title edit / confirm panel.
4. Vue's 批量管理 (batch manage) entry and the running-session spinner
   (menu.sessionInProgress) are not ported to the shell list.
5. Chat→chat switching is route-driven via a custom event; Vue keeps the
   sidebar mounted app-wide, so its scroll position survives navigation while
   the React shell list remounts per page load.
