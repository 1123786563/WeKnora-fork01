# Shell sessions header slice — visible 我的对话 title + 新对话 ⌘1 hint (2026-09-13)

Branch: `codex/react-multiclient` (worktree `.worktrees/react-multiclient`). No commits made.

Follow-up to `2026-09-13-shell-session-list.md`: the grouped session list already
lives in the React platform shell (`nav.plat-shell__sessions`), but the
screenshot text-fingerprint comparison against the Vue shell showed the session
area header elements were missing: no visible 我的对话 label, and no shortcut
hint on the 新对话 entry (Vue fingerprint shows 新建对话⌘1).

## Vue anatomy (read before implementing)

- `frontend/src/components/menu.vue:4-31` — logo row: logo box + `logo_actions`
  (search icon button with `aria-label=t('menu.search')` and a `t-tooltip`
  whose `cmdk-tip` content is `menu.search` + `⌘K` (lines 10-21), then the
  collapse toggle).
- `frontend/src/components/menu.vue:58` comment 上半部分：新对话吸顶 +
  `menu.vue:79-101` — the first `menu_top` item is creatChat (`stores/menu.ts:24-31`,
  `titleKey: 'menu.newChat'`), sticky via `menu_box--sticky` because it has
  `children` (the sessions) — i.e. the 新对话 entry sits directly above the
  session area and stays pinned there.
- `frontend/src/components/menu.vue:104-112` — `.submenu` (session area) opens
  with a `session-list-scope-header` row containing the inline
  `SessionSourceFilter`; its trigger label is the current source, whose web
  value is `t('menu.myChats')` (`menu.vue:332` via `buildSessionSourceOptions`,
  and `menu.vue:752` as the web bucket label). Styled light/secondary,
  top-right (`menu.vue:1583-1606`, `SessionSourceFilter.vue:186-201`).
- `frontend/src/components/menu.vue:303-304` — `cmdModKeyLabel = isMacLike ? '⌘' : 'Ctrl'`
  keys off `navigator.platform`.
- ⌘1: Vue renders ⌘1-9 kbd chips on command-palette rows
  (`GlobalCommandPalette.vue:146`, `ResultItem.vue:33`) and binds ⌘1-9 to jump
  to the Nth flat item while the palette is open (`GlobalCommandPalette.vue:512-514`).
  The first quick action is 新建对话 (`commandPalette.quick.newChat`,
  zh-CN '新建对话') → `/platform/creatChat` — this is the fingerprint's
  "新建对话⌘1" (the palette quick action), distinct from the sidebar nav label
  `menu.newChat` = 新对话 (`frontend/src/i18n/locales/zh-CN.ts:6964`;
  `menu.myChats` = 我的对话 at :6983, `menu.search` = 搜索 at :6979).

## Diff table (Vue → React before this slice)

| # | Vue (menu.vue) | React shell before | Action |
|---|---|---|---|
| 1 | Session area carries a visible 我的对话 label (`menu.myChats` as the source trigger text, menu.vue:109-112/332/752) | `nav.plat-shell__sessions aria-label="我的对话"` only — no visible text | **Fixed**: visible `h2.plat-shell__sessions-title` 我的对话 (`formatMessage(locale,'menu.myChats')`) as the first child of the sessions nav |
| 2 | 新对话 entry = sticky first item of the session area (menu.newChat, 新对话) | `plat-shell__nav` first item, same key/copy/href, position directly above the sessions nav | **Kept** (already parity) |
| 3 | ⌘1 shortcut badge (新建对话⌘1 fingerprint; GlobalCommandPalette ⌘1-9 → first quick action → /platform/creatChat) | no shortcut hint, no ⌘1 binding | **Fixed**: `kbd.plat-shell__shortcut` (⌘1 / Ctrl+1, platform-aware per menu.vue:303-304) on the 新对话 entry + app-wide ⌘1/Ctrl+1 keydown → `/platform/creatChat` |
| 4 | logo row search icon with `cmdk-tip` 搜索 ⌘K tooltip (menu.vue:10-21) | shell logo row has only the collapse toggle; ⌘K palette exists (keyboard + deep link) but no visible search button | **Not in this slice** — recorded under 遗留 (logo-row slice, not the sessions-header slice) |

SessionSidebarList / the chat page's fallback in-page sidebar are untouched:
the fallback sidebar is a React-only affordance (Vue's chat page has no
sidebar), and the shell still suppresses it via `SessionSidebarShellContext`
(test (e) of shell-session-list.test.tsx stays green).

## Changes

### apps/web/src/platform/PlatformShell.tsx

- `platformModKeyLabel(platform)` export — Vue menu.vue:303-304 logic
  (⌘ on `/Mac|iPod|iPhone|iPad/`, else `Ctrl+`).
- `shellNavigation.assign(path)` export — navigation seam around
  `window.location.assign` (the app's cross-page convention) so tests can
  observe navigations (jsdom's `location.assign` is non-writable).
- `NavItem.shortcut` + `buildNavItems(..., newChatShortcut)`; the 新对话 item
  renders `<kbd className="plat-shell__shortcut">{shortcut}</kbd>` next to the
  label while the rail is expanded (collapsed keeps the title-attr tooltip,
  like Vue's collapsed items).
- New global keydown effect: ⌘1 / Ctrl+1 (no shift/alt, non-editable target)
  → `shellNavigation.assign('/platform/creatChat')` — the same destination the
  hint's Vue counterpart (palette quick action 新建对话⌘1) resolves to.
- Sessions nav: `<h2 className="plat-shell__sessions-title">{labels.myChats}</h2>`
  before `<SessionSidebarList>` (visible 我的对话).

### apps/web/src/platform/shell.css

- `.plat-shell__shortcut` — right-aligned kbd chip (Vue ResultItem kbd look:
  hairline border, muted text, 11px).
- `.plat-shell__sessions-title` — small secondary-tone section title
  (`#8b97a8`, 12px/600), matching the light tone of Vue's session-area label.

## i18n (reuse-first; packages/i18n NOT edited)

- `menu.myChats` (visible title; previously only the aria-label), `menu.newChat`
  (nav entry copy — byte-exact Vue zh-CN 新对话), `menu.search` left untouched
  (logo-row search button out of slice). No new keys needed.

## TDD (red → green)

New: `apps/web/src/platform/shell-sessions-header.test.tsx` (jsdom + act, same
harness as shell-session-list.test.tsx; jsdom `navigator.language='zh-CN'`,
`platform='MacIntel'` to pin the ⌘ variant).

- RED first: **0/5** — title missing (`actual: null`), kbd missing,
  `platformModKeyLabel is not a function`, no ⌘1 navigation.
- GREEN: **5/5**
  - (a) sessions region shows a visible 我的对话 title (aria-label kept)
  - (b) 新对话 entry carries the ⌘1 kbd hint; collapsed rail hides hint +
    sessions area entirely (first mount torn down explicitly — a leaked root
    keeps its window keydown listener and double-fires later tests)
  - (c) `platformModKeyLabel`: MacIntel/Macintosh/iPad → ⌘; Win32/Linux/'' → Ctrl+
  - (d) ⌘1 and Ctrl+1 navigate to `/platform/creatChat`
  - (e) bare `1`, ⌘X, ⌘1 from an editable target never navigate

## Verification

- `npx tsx --test "src/platform/"*.test.tsx "src/platform/"*.test.ts` (cwd
  apps/web): **102/102 pass** (97 baseline + 5 new; includes
  platform-shell-guide-reopen.test.tsx, still green).
- `packages/views/src/chat/session-sidebar*.test.tsx`: no such files exist
  (session-sidebar.tsx untouched this slice); whole views chat suite
  `npx tsx --test packages/views/src/chat/*.test.tsx packages/views/src/chat/*.test.ts`
  → **55/55 pass**.
- `pnpm run typecheck:web`: **0 errors** (no new errors).

## 遗留 (remaining gaps for the coordinator)

1. **Logo-row search entry (Vue menu.vue:10-21):** the Vue logo row carries a
   search icon button (`menu.search` + ⌘K `cmdk-tip` tooltip) opening the
   command palette; the React shell logo row has only the collapse toggle.
   The palette itself is fully ported (⌘K + `?cmdk=` deep link), so this is a
   visible-button gap in the logo-row slice — needs a coordinator call since
   the logo row may belong to another slice.
2. The ⌘1 binding is app-wide in React; Vue binds ⌘1-9 only while the palette
   is open (GlobalCommandPalette.vue:512-514) where the ⌘1 badge actually
   lives. The sidebar hint makes the app-wide binding the honest affordance;
   if the coordinator prefers strict palette-scoped behavior, drop the
   keydown effect and keep (or drop) the kbd hint.
3. Vue's session-area label is a hover-revealed source *dropdown* trigger
   (SessionSourceFilter, opacity 0 → 1 on hover/emphasis) when IM/embed/API
   channel buckets exist; React renders a plain always-visible title and still
   has no channel buckets (pre-existing gap #1 of 2026-09-13-shell-session-list.md).
4. `h2.plat-shell__sessions-title` introduces the first heading in the shell
   sidebar; group headers are `h3` (unchanged), so the outline stays ordered.
