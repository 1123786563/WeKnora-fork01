# R031 — sandbox inventory session titles (api-client + panel wiring)

Worktree: `.worktrees/react-multiclient` @ `codex/react-multiclient` (no commits by this agent).
Date: 2026-09-13. Agent: R031 sandbox settings closeout slice.

## Registered open item addressed

> "inventory session titles need a sessions api-client method" (R031 row,
> `docs/migrations/react/vue-react-parity-matrix.md:60`)

Status: **done** — typed `sessions.get` added to `@weknora/api-client`, panel wired
to render resolved titles with the Vue loading/missing semantics. 2 new panel tests.

## Vue authority (evidence lines)

`frontend/src/views/settings/SandboxSettings.vue`:

| Lines | Fact |
|---|---|
| 219 | `import { getSession } from '@/api/chat/index'` |
| 140-206 | inventory drawer markup; session list rows at 169-183 |
| 171-178 | row: `<span class="inventory-row__title" :title="id">{{ sessionTitle(id) }}</span>` — raw id on the tooltip, resolved title as the label; meta label `inventorySessionKind` |
| 312 | `inventorySessions = inventory.value?.session_ids \|\| []` |
| 315-321 | `sessionTitle(id)`: `sessionTitles[id]` if truthy, else `t('settings.sandbox.inventoryUntitledSession')` |
| 323-339 | `loadSessionTitles(ids)`: dedupe + drop blanks (324), one `getSession(id)` per id in parallel, `next[id] = String(res?.data?.title \|\| '').trim()` (333), `catch → ''` (334-336) |
| 511-528 | `openInventory`: resets `sessionTitles = {}` (517), fetches inventory, then `await loadSessionTitles(...)` (521) — the `t-loading` spinner (154) stays up until titles land, so raw ids never flash |
| 555-566 | `showRefusal` (delete conflict): renders the drawer immediately (`inventoryLoading = false` at 563) and fire-and-forgets `void loadSessionTitles(...)` (565) — rows show the untitled fallback until lookups land |
| 341-344 | `openSession(id)`: row click navigates to `/platform/chat/:id` (**not ported** — see Remaining) |

`frontend/src/api/chat/index.ts:72-74` — `getSession(session_id)` → `GET /api/v1/sessions/${session_id}`.

## API facts

- Backend route exists: `internal/router/routes_chat.go:55` — `sessions.GET("/:id", handler.GetSession)` under `/api/v1/sessions`.
- Response envelope `{ success: true, data: <session> }`; `packages/contracts/src/index.ts:436` `parseChatSessionResponse` validates it (`title` is a required string, may be empty — `requiredString` at 217-220 only checks typeof).
- Shared i18n already carries the fallback copy: `packages/i18n/src/settings.ts` → `settings.sandbox.inventoryUntitledSession` = `未命名会话` / `Untitled session` (verified via `formatMessage('zh-CN'|'en-US', ...)`).

## TDD — before / after

### api-client (`packages/api-client/src/chat/sessions.ts`)

RED (before implementation):

```
✖ fetches one typed session by id ... TypeError: api.get is not a function
✖ refuses to fetch a session without an id ... TypeError: api.get is not a function
(tests 7, pass 5, fail 2)
```

GREEN (after): `tests 7, pass 7, fail 0`.

New method (inside `createChatSessionsApi`, reusing existing `sessionPath` + `parseChatSessionResponse`):

```ts
async get(sessionId: string, signal?: AbortSignal): Promise<ChatSession>
// GET /api/v1/sessions/:sessionId  (empty/blank id → Error('sessionId must not be empty'), path-encoded)
```

New tests in `packages/api-client/src/chat/sessions.test.ts`:
1. `fetches one typed session by id for consumers that need a single title (GET /api/v1/sessions/:id)` — method/path encoding, envelope parsing, signal passthrough.
2. `refuses to fetch a session without an id`.

### Panel (`apps/web/src/settings/SandboxSettingsPanel.tsx`)

RED (before wiring): `tests 26, pass 24, fail 2` — inventory rendered raw ids
(`session-a 对话`, duplicate + blank rows included), no titles, card shown before titles resolved.

GREEN (after): `tests 26, pass 26, fail 0` (~0.7–0.9s). The pre-existing 24 tests stayed green throughout.

Panel changes:

- Exported `uniqueSessionIds(ids)` — SandboxSettings.vue:323-330 semantics (trim, drop blanks, dedupe).
- Exported `sessionTitleText(titles, id, untitledLabel)` — SandboxSettings.vue:317-321 semantics.
- `sessionTitles` state; `loadSessionTitles` (Vue 323-339): parallel `client.sessions.get` per unique id, `title.trim()`, `catch → ''`.
- `inspect` (Vue openInventory 511-528): resets titles, awaits inventory **then** titles before `setInventory` — the list cannot flash raw ids.
- `removeRecord` conflict branch (Vue showRefusal 555-566): `setInventory` immediately + `void loadSessionTitles(...)`.
- Inventory rows: `<strong title={id}>{sessionTitleText(...)}</strong>` — raw id on the tooltip, title/untitled fallback as the label (Vue 171-178).

New tests in `SandboxSettingsPanel.test.tsx`:
1. `inventory resolves session ids into titles with Vue fallbacks for failed lookups (SandboxSettings.vue:171-178, 323-339)` — trimmed title, failed lookup → `未命名会话`, `title` attr = raw id, dedupe + blank-drop (`requested === ['session-a','session-b']`).
2. `inventory stays hidden while session titles resolve, then renders them (SandboxSettings.vue:515-527)` — gate promise holds `sessions.get`; card absent until release; then title + kind label render.
3. Harness: `makeClient` gained a `sessions.get` mock that mirrors the real envelope contract.

## Verification (all green)

| Command | Result |
|---|---|
| `pnpm run test:shared` | exit 0 — `tests 392, pass 392, fail 0` |
| `cd apps/web && npx tsx --test src/settings/SandboxSettingsPanel.test.tsx` | `tests 26, pass 26, fail 0` |
| `pnpm run typecheck:shared` | exit 0 (includes `chat/sessions.ts`) |
| `cd apps/web && npx tsc --noEmit -p tsconfig.json` | 2 errors, both pre-existing in `src/platform/shell-sessions-header.test.tsx` (another agent's untracked WIP; none in R031 files) |

## pick-row percent fan-out — verified, handed off (not in my writable scope)

Vue semantics (authoritative, `frontend/src/views/settings/SkillSettings.vue`):

- Busy pick rows render, in both the add drawer (243-247) and the install drawer (294-298):
  `<div v-if="row.busy" class="sandbox-pick__progress"><t-progress theme="circle" :percentage="sandboxPickPercent(row) ?? 0" :size="18" :stroke-width="2" :label="false" /><span v-if="sandboxPickPercent(row) != null">{{ sandboxPickPercent(row) }}%</span></div>`
- `sandboxPickPercent` (579-583): `if (!row.busy \|\| !row.install) return null; return installEventPercent(row.cfg.id, row.install.skill_id)`.
- `installEventPercent` = `useConfigSkillInstallProgress.percentOf` (useConfigSkillInstallProgress.ts:34-36) → `liveInstallPercent` (skillInstallProgress.ts:22-30): **null** when no event / non-finite percent / `done && percent <= 0` ("phantom done" — a stream without Redis closes with done=true percent=0 and must keep showing 安装中, not 0%); otherwise round+clamp to 0-100 (15-18).
- Fan-out mechanics: `follow`/`sync` (useConfigSkillInstallProgress.ts:56-119) hold one SSE connection per `progressKey(configId, skillId)` = `"configId:skillId"`; finished keys are `forget()`-ed before reconnect so a retry shows fresh progress, not the stale 100%.

React gap (`apps/web/src/settings/SkillSettingsPanel.tsx`, **R033-owned file**): `SandboxPickList` status rows (603-612) show status text + a "view install progress" button (611) but no progress ring / percent text.

Handoff for the owning agent (exact pieces):
1. Add a `liveInstallPercent` equivalent (phantom-done → `null`) to `packages/domain/src/sandbox/skill-install.ts` — `clampInstallPercent` (81-84) exists but `installProgressPercent` (91-100) has *different* semantics (status-based 5/100/0 fallbacks, never null) and must not be reused for pick rows.
2. Transport for the fan-out already exists: `packages/api-client/src/sandbox/skill-install.ts` `skillInstallEventsPath` (60-62) + SSE consumption, and `parseSkillInstallProgressFrame` (domain, 56-74). Needs a per-`configId:skillId` keyed sync/follow registry in the panel mirroring Vue's `sync(targets)`.
3. In `SandboxPickList` busy rows: 18px progress ring fed `percent ?? 0`, plus `{percent}%` text rendered **only when percent is non-null**; keep the existing view-progress button (611).

## Remaining (R031)

- `openSession` navigation (Vue 341-344: row click → `/platform/chat/:id`, closes drawer) — **resolved by the follow-up slice below** (section "openSession navigation").
- Inventory chrome: Vue uses a `SettingDrawer`; React renders an inline Card — registered as a separate surface decision, not addressed here.

## Process notes for future agents

- Node v26.7.0 `node:test` can hang (~50s, then "Promise resolution is still pending but the event loop has already resolved") when an `AssertionError` is thrown at particular await boundaries inside an async test (observed right after `await act(...)` in this file). Writing the test to collect booleans and assert once after the flow fully settles avoids it; `assert.match` after a completed settle phase also works.
- A concurrent `git add -A` sweep (commit `891673ea`, "docs: 删除vue-react-parity迁移的所有旧截图文件") picked up this agent's in-progress files mid-session: the api-client change (+9/+23) and two temporary probe files are inside that commit. The probe files (`apps/web/src/settings/panel-probe*.test.tsx`) are deleted in the working tree and must not ship; the pending deletion needs to be included in the next integration commit. My panel changes (+44/+23) are uncommitted working-tree changes.

## openSession navigation (R031 slice follow-up, same worktree/branch)

Status: **done** — closes the last R031 remaining item (line 112 above).

### Vue authority

`frontend/src/views/settings/SandboxSettings.vue`:

| Lines | Fact |
|---|---|
| 171 | row is a real `<button type="button" class="inventory-row" @click="openSession(id)">` — one per session row; no permission gate at the list |
| 341-344 | `openSession(id)`: `showInventory.value = false`, then  `` router.push(`/platform/chat/${encodeURIComponent(id)}`) `` — in-app SPA navigation on the current page (never `window.open`); no precondition in the component, an inaccessible session is a chat-page concern |
| 176 | affordance: `t-icon name="chevron-right"` at the row end |

React translation (deliberate deltas):

- The host layer owns navigation: `SandboxSettingsPanel` gains optional `onOpenSession?: (sessionId: string) => void`; `SettingsPage` passes  `` window.location.assign(`/platform/chat/${encodeURIComponent(sessionId)}`) `` — the same cross-page convention as `PlatformShell.openShellSession` (`apps/web/src/platform/PlatformShell.tsx:204`) and the settings close button (`apps/web/src/settings/SettingsPage.tsx:195`).
- Vue closes the drawer before pushing (the app survives the SPA push); React skips that step because the full-page assign unloads the settings page anyway, and the panel must stay embed-safe when no handler is given.
- The chevron affordance ships as a plain `aria-hidden` `›` span: the panel CSS file is outside this slice writable scope.

### TDD — before / after (panel)

RED (`cd apps/web && npx tsx --test src/settings/SandboxSettingsPanel.test.tsx`):

```
✖ inventory session rows open the chat session through onOpenSession (SandboxSettings.vue:171, 341-344)
  AssertionError: clicking a row reports its session id exactly once — actual [] vs expected ['session-a']
(tests 28, pass 27, fail 1)
```

The inert-row test passed before implementation by construction (rows were already static); it pins the embed-safety requirement so a later always-button refactor cannot slip through.

GREEN: `tests 28, pass 28, fail 0` (~0.8s). The 26-test baseline stayed green unchanged — without `onOpenSession` the rows keep the previous DOM shape, so existing selectors still match.

Panel changes (`apps/web/src/settings/SandboxSettingsPanel.tsx`):

- `Props.onOpenSession?: (sessionId: string) => void`.
- Inventory rows (Vue 171-178 markup preserved): with the handler each row is `<button type="button" class="wk-sandbox-inventory-row" onClick={() => onOpenSession(id)}>` with an `aria-hidden` chevron; without it the row stays the previous static `<strong>` + meta markup.

Wiring (`apps/web/src/settings/SettingsPage.tsx`): the sandbox panel receives  `` onOpenSession={(sessionId) => { window.location.assign(`/platform/chat/${encodeURIComponent(sessionId)}`); }} ``.

New tests in `SandboxSettingsPanel.test.tsx`:
1. `inventory session rows open the chat session through onOpenSession (SandboxSettings.vue:171, 341-344)` — first-row click calls `onOpenSession` with the session id exactly once; both rows render as click targets.
2. `inventory rows render inert without onOpenSession so embeds stay click-safe (SandboxSettings.vue:171)` — no row button, raw-id tooltip intact, untitled fallback still renders.

### Verification (all green)

| Command | Result |
|---|---|
| `cd apps/web && npx tsx --test src/settings/SandboxSettingsPanel.test.tsx` | `tests 28, pass 28, fail 0` |
| `pnpm run typecheck:web` (repo root) | exit 0, no errors — the shell-sessions-header errors noted above no longer reproduce |

### Remaining (R031)

- Nothing left on openSession. The inventory-chrome surface decision (`SettingDrawer` vs inline Card) stays registered as before.
