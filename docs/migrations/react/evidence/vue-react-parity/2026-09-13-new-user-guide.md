# Welcome tour (NewUserGuide) — Vue → React parity

Date: 2026-09-13 · Branch: `codex/react-multiclient` · Scope: cross-page chrome parity slice
Status: DONE (code + tests + live screenshot verification).

## Vue baseline (authoritative, read-only)

| Concern | Source |
| --- | --- |
| Trigger wrapper (event listener, 700 ms double-checked auto-open, `onFinish` writes key, step-change closes guide-opened settings) | `frontend/src/components/NewUserGuide.vue:16-53` (7 steps), `:64-67` (finish → `localStorage.setItem(GLOBAL_USER_GUIDE_KEY,'1')`), `:69-73` (close settings when leaving `models`), `:84-93` (mount listener + delayed auto-open with re-check) |
| Spotlight renderer (hole/backdrop/card/dots/buttons/keyboard) | `frontend/src/components/SpotlightGuide.vue` — constants `:57-62`, `computeHighlightHole` `:118-145`, backdrop pieces `:164-185`, `cardStyle` `:210-251`, `queryTarget` `:253-262`, `goTo` `:309-326`, finish/dismiss `:341-349`, styles `:400-599` |
| Storage key + reopen event | `frontend/src/config/contextualGuides.ts:3-8` (`weknora:new-user-guide-done:v1`, `weknora:open-new-user-guide`), `:149-151` (`isGlobalUserGuideDone`) |
| Mount point (all `/platform` pages, no auth/role gate beyond the shell) | `frontend/src/views/platform/index.vue:18` |
| Copy (5 locales, `newUserGuide` block) | `frontend/src/i18n/locales/zh-CN.ts:7024-7061`, `en-US.ts:185-222`, `ja-JP.ts:181-218`, `ko-KR.ts:7018-7055`, `ru-RU.ts:7018-7055` |

Behavior contract verified from source: the guide shows whenever the key is absent (any value != '1'), 700 ms after platform mount, re-checked inside the timer callback; skip (跳过引导), close X, Esc, and 完成 all write the key '1'; dots are progress-only (not clickable); Esc dismisses, arrow keys navigate; welcome and done steps are centered cards, the other five anchor to `data-guide` attributes (the agents step is `optional` and auto-skips when its target never appears; targets are retried 12x120 ms before falling back to a centered card).

## React implementation (new files under `packages/views/src/guides/`)

- `new-user-guide.ts` — storage key/event literals, `isNewUserGuideDone` / `shouldAutoOpenNewUserGuide` / `markNewUserGuideDone` / `openNewUserGuide` (structural storage type, mirrors `contextualGuides.ts`).
- `steps.ts` — the 7-step catalog (same keys, selectors, placements, `optional` flags) + the full 5-locale copy under the **same** `newUserGuide.*` key names, byte-identical to the Vue locale files; `guideMessage` replicates vue-i18n named interpolation for `stepOf`.
- `geometry.ts` — pure port of `computeHighlightHole`, `computeCardStyle`, `computeBackdropPieces` with the exact Vue constants (`CARD_WIDTH 340`, `GAP 16`, `EDGE 16`, `PAD 8`, hole radius 8, backdrop `rgba(15,18,22,0.58)`).
- `NewUserGuide.tsx` — the component: mount-time event listener + 700 ms double-checked auto-open; the step driver runs the step action, waits `beforeDelayMs` (280), locates (12x120 ms retries, optional auto-skip), focuses the card; Esc/←/→ handling; skip and finish both persist; step-change closes guide-opened settings when leaving `models`.
- `guides.css` — byte-approximate port of the SpotlightGuide scoped styles (class prefix `.guide*` → `.wk-guide*`, TDesign vars consumed with the Vue app's own fallbacks, e.g. `var(--td-brand-color, #07c05f)`).

### Mount (minimal, `apps/web/src/platform/PlatformShell.tsx`)

- `<NewUserGuide locale={locale} actions={guideActions} />` rendered after `GlobalCommandPalette` (Vue mounts it in `platform/index.vue`, i.e. on every platform page — PlatformShell covers the same set).
- Added `data-guide` anchors matching the Vue selectors: `nav-creatChat`, `nav-knowledge-bases`, `nav-agents` on the nav items, `user-menu` on the user button.
- `guideActions`: `expandSidebar` → reveal sidebar (non-persistent); `openModelsSettings` → `history.pushState('/platform/settings?section=models')` (SPA nav keeps the shell mounted, mirroring Vue's settings-modal-over-the-page); `closeGuideSettings` → `history.back()` only when the guide initiated the navigation.
- One extra import: `../../../packages/views/src/guides/guides.css` (the views package must stay css-import-free for `typecheck:shared`).

### Exports

`packages/views/src/index.ts` gained the guides exports (component, geometry helpers, storage helpers, catalog/messages) following the existing sibling convention.

## Persistence

- Key: **`weknora:new-user-guide-done:v1`** (exact Vue literal).
- Value: `'1'`, written on finish AND on skip/dismiss; the guide auto-opens only while the key is absent or != '1'; the `weknora:open-new-user-guide` window event re-opens regardless of the key (parity with the Vue user-menu help button entry point).

## Steps (order and content identical to Vue)

1. `welcome` — centered card — 欢迎使用 WeKnora
2. `knowledge` → `[data-guide="nav-knowledge-bases"]` (right)
3. `agents` → `[data-guide="nav-agents"]` (right, optional)
4. `chat` → `[data-guide="nav-creatChat"]` (right)
5. `settings` → `[data-guide="user-menu"]` (right)
6. `models` → `[data-guide="settings-add-model"], [data-guide="settings-models"]` (left) + opens settings
7. `done` — centered card — 一切就绪

## TDD evidence

- RED: `packages/views/src/guides/new-user-guide.test.ts` written first → failed (module missing). GREEN after implementing `new-user-guide.ts` + `steps.ts`.
- RED: the geometry test initially asserted stricter-than-Vue non-overlap for a degenerate fullscreen hole; corrected to assert the exact Vue fallback position (the Vue algorithm itself can overlap there).
- RED: the jsdom test (e) asserted the step label after 完成 had already unmounted the card; assertion order fixed (capture before click).
- Final counts:
  - `pnpm exec tsx --test packages/views/src/guides/*.test.tsx packages/views/src/guides/*.test.ts` → **14 pass / 0 fail** (8 storage/semantics/catalog/copy + 6 geometry).
  - `apps/web` jsdom component tests (`new-user-guide.test.tsx`, harness copied from `ModelSettingsPanel.test.tsx`) → **6 pass / 0 fail**: (a) key='1' → no modal; (b) key absent → auto-open step 1 with 欢迎使用 WeKnora, `1 / 7`, 7 dots; (c) 下一步 → 创建你的知识库 `2 / 7`; (d) 跳过引导 → key '1' + overlay unmounted; (e) full walk → 完成 → key '1' + unmounted; (f) reopen event reopens a finished tour.
  - Full `apps/web/src/platform` test dir after my shell edit → **75 pass / 0 fail**.
  - `pnpm run typecheck:shared`: no errors from my files (one error in `packages/views/src/chat/message-list.tsx` introduced mid-session by concurrent work — absent from my baseline snapshot, file not mine).
  - `pnpm --filter @weknora/web exec tsc --noEmit`: no errors from my files (my baseline had 39 errors, all in the concurrent agent's `src/documents/`; the only current extra error is their in-progress `src/agents/list.ts`).

Note: the component tests live under `apps/web/src/platform/` because only that package resolves `react-dom` (documented convention from `packages/views/src/chat/tool-approval.test.tsx`); the component and all pure logic live in `packages/views/src/guides/`.

## Deliberate deltas (visuals/behavior preserved, mechanics adapted)

1. No `<Teleport to="body">` — the overlay renders inline as a `position:fixed` `z-index:5000` layer (`react-dom` is not a dependency of `packages/views`). Same stacking result inside the shell.
2. Class names `.guide*` → `.wk-guide*` to avoid global-CSS collisions; declarations copied from the Vue scoped styles.
3. `t-icon close` → inline SVG X; `t-button` small/primary/outline → `.wk-guide__btn` approximations with TDesign small-button metrics; brand via `var(--td-brand-color, #07c05f)` (the Vue app's own fallback convention, cf. `frontend/src/components/Input-field.vue:2986`).
4. `<Transition name="guide-fade">` → CSS entry animation `wk-guide-fade` (same 0.2 s ease fade).
5. Vue's `before` callbacks (`uiStore.expandSidebar` / `openSettings('models')`) become serializable `action` ids in the catalog, mapped onto shell-provided callbacks (settings opens via SPA pushState instead of a modal).
6. `autoOpenDelayMs` (Vue hardcodes 700) and `locateRetryDelayMs` (Vue hardcodes 120) extracted as props with identical defaults — for tests only.
7. `scrollIntoView` feature-guarded (jsdom compatibility; identical behavior in browsers).
8. Guide copy is served from `steps.ts` under the same `newUserGuide.*` key names because `packages/i18n` has no such block (see below); `packages/views` deliberately does not depend on `@weknora/i18n` (precedent: `packages/views/src/integrations/messages.ts`).
9. SpotlightGuide's `interact` / `interactHint` machinery is not ported — the welcome tour never uses it (no `newUserGuide.interactHint` key exists in any Vue locale).

## Needed i18n keys (NOT added to packages/i18n — reported instead)

`packages/i18n` has no `newUserGuide.*` keys. When they are generated there, `steps.ts` can swap to `formatMessage` with zero call-site changes. Missing keys, for each of `zh-CN`, `en-US`, `ja-JP`, `ko-KR`, `ru-RU`:

- `newUserGuide.stepOf`, `newUserGuide.skip`, `newUserGuide.prev`, `newUserGuide.next`, `newUserGuide.done`, `newUserGuide.reopen`
- `newUserGuide.steps.{welcome,knowledge,agents,chat,settings,models,done}.title` and `.desc`

## Live check (PASSED)

`.parity-tools/guide-slice-shot.cjs` (login as parity-test@local.dev, clear `weknora:new-user-guide-done:v1`, reload, wait past the 700 ms auto-open) against the vite dev server on :5181:

- step 1 rendered: title `欢迎使用 WeKnora`, centered card, dark overlay, dots `1 / 7`, 跳过引导 + 下一步 — `screenshots/guide-slice/react-guide-step1.png`
- after 下一步: title `创建你的知识库`, `2 / 7`, green spotlight ring + 4-piece backdrop around the shell 知识库 nav item (`data-guide="nav-knowledge-bases"`), card placed to the right — `screenshots/guide-slice/react-guide-step2.png`
- after 跳过引导: `localStorage[weknora:new-user-guide-done:v1] === 1` (string) and the overlay unmounted — `screenshots/guide-slice/react-guide-after-skip.png`

(Two earlier attempts hit a transient full-screen vite HMR overlay from a concurrent agent broken import, `SkillSettingsPanel.tsx` -> `@weknora/domain/sandbox/skill-install`; unrelated to this slice and resolved by the time of the successful run.)

## Remaining gaps

- Contextual guides unported (Vue `frontend/src/config/contextualGuides.ts:46-135` + `ContextualGuide.vue`, `KbCreateContextualGuide.vue`, tenant-models contextual guide, etc.): keys `weknora:contextual-guide-kb-list:v2`, `-kb-create:v3`, `-kb-detail:v1`, `-chat:v1`, `-tenant-models:v1`, `-agent-list:v1`, `-agent-create:v1`.
- The React shell user menu has no help/reopen button yet (Vue `UserMenu.vue:45-46,503`); the reopen event contract is already live (`openNewUserGuide()` exported, component listens), so the button is a one-liner when the menu grows one.
- `models` step spotlight target: the React settings UI does not yet render `data-guide="settings-add-model"`/`"settings-models"`; the guide falls back to the centered card exactly like Vue does when a non-optional target is missing. Adding the attribute in the settings panels lights the spotlight up automatically.
