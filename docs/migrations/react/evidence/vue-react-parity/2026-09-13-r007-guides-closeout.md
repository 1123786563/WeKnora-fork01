# 2026-09-13 — R007 guides closeout (user-menu reopen entry + newUserGuide i18n byte-pinning + views layering)

**Branch:** `codex/react-multiclient` · **Scope:** guides/i18n closeout slice (no commits — left in the working tree for the coordinator). Concurrent agents' files (mobile, api-client, domain/settings, web/settings) untouched.

## 1. User-menu reopen entry (Vue parity)

### Vue baseline (verified from source, not guessed)

| Concern | Source |
| --- | --- |
| Entry UI | `frontend/src/components/UserMenu.vue:45-50` — `t-tooltip` + `.dropdown-guide-btn` (help-circle icon) inside the dropdown user header, `aria-label`/tooltip `$t('newUserGuide.reopen')`, `@click.stop="reopenGuide"` |
| Behavior | `UserMenu.vue:501-504` — `reopenGuide()`: close menu, then `openNewUserGuide()` |
| Trigger primitive | `frontend/src/config/contextualGuides.ts:6-8` — `openNewUserGuide()` **only dispatches** `CustomEvent('weknora:open-new-user-guide')`; it does not touch storage |
| Replay vs done-key | `NewUserGuide.vue:84-93` — the event listener opens unconditionally; only the 700 ms **auto-open** path checks `weknora:new-user-guide-done:v1`. So a replay happens with the key still `'1'`; the key is rewritten only on finish/skip. The React port (`NewUserGuide.tsx` `handleOpenEvent`) already mirrored this. |

### Copy (zh-CN `newUserGuide.reopen`)

The task hint guessed "重新查看引导"; the authoritative locale files say otherwise. Per locale (`frontend/src/i18n/locales/*.ts`, newUserGuide block):

| locale | `newUserGuide.reopen` |
| --- | --- |
| zh-CN | `新手引导` |
| en-US | `Product tour` |
| ja-JP | `プロダクトツアー` |
| ko-KR | `사용 가이드` |
| ru-RU | `Обучение` |

### React implementation (`apps/web/src/platform/PlatformShell.tsx`)

- `plat-shell__dropdown` gains a first `role="menuitem"` button (`data-testid="plat-shell-guide-reopen"`) with an inline help-circle SVG + label `formatMessage(locale, 'newUserGuide.reopen')` and the same value as `aria-label` (Vue's entry is tooltip+aria-label only; the React flat menu has no user header, so a labelled first menu item is the equivalent surface).
- Click → `setMenuOpen(false); openNewUserGuide()` — imported from `@weknora/views` (already exported). Replay overrides the done-key gate via the existing event listener; the stored key is not rewritten (Vue parity), verified by test (b).

### TDD

- **RED:** new `apps/web/src/platform/platform-shell-guide-reopen.test.tsx` (harness copied from `shell-session-list.test.tsx`; done-key preset to `'1'`) → **0 pass / 3 fail** (reopen entry missing in all three).
- **GREEN** after the shell edit → **3 pass / 0 fail**:
  - (a) menu item exists, `role=menuitem`, `aria-label`/text = `新手引导` (= `formatMessage('zh-CN','newUserGuide.reopen')`), icon present;
  - (b) click closes the menu, tour replays from `欢迎使用 WeKnora` `1 / 7` despite done-key `'1'`, stored key still `'1'`;
  - (c) all five locales: label equals `formatMessage(locale, 'newUserGuide.reopen')` (shared-bundle resolution per mount via `navigator.language`).

## 2. i18n backfill verification + byte-level pinning

The `newUserGuide` shared block already existed (commit `2f8a6c1e`, import/re-export in `packages/i18n/src/index.ts:883-884`, merge chain line 911). This slice converted the weak `>= 20`-keys test into a **chat.test.ts-style byte-level pin** against the Vue locale files, and found + fixed one real drift.

- Key path: `newUserGuide.{stepOf,skip,prev,next,done,reopen}` + `newUserGuide.steps.{welcome,knowledge,agents,chat,settings,models,done}.{title,desc}` = **20 keys × 5 locales = 100 values**.
- `packages/i18n/test/newUserGuide.test.ts` rewritten: exact 20-key set per locale; every value byte-compared to `frontend/src/i18n/locales/{zh-CN,en-US,ja-JP,ko-KR,ru-RU}.ts`; merged-bundle locale-drift check; `formatMessage` resolution incl. `reopen` ×5 and `stepOf` interpolation.
- **RED:** byte-comparison caught `newUserGuide.steps.agents.desc` (ru-RU) drift — generated `…в переиспользуемых агентах…` vs Vue `…в переиспользуемых агентов…`. Fixed in **both** the shared bundle (`generated/newUserGuide.ts`) and the views local table (`guides/steps.ts`). Full sweep confirmed no other drift among the 100 values.
- **GREEN:** 4 pass / 0 fail.

## 3. Views-side layering (formatMessage shadows the local table)

`packages/views` cannot depend on the `@weknora/i18n` package name, so — per the established `integrations/messages.ts` relative-import precedent — `guides/steps.ts` now imports `formatMessage` from `../../../i18n/src/index.ts` and `guideMessage` resolves the **shared bundle first** (`integrationsT` pattern: shared wins unless it returns the bare key; local byte-exact table stays as fallback; `formatGuidePattern` still handles interpolation for the fallback path).

- `guideMessage` is the only copy resolver for `NewUserGuide.tsx`, so the shared values now win automatically with zero call-site changes (rendered output unchanged — tables are byte-identical).
- **TDD:** `packages/views/src/guides/guide-message-layering.test.ts` written first → **RED 1/4** (shadow test: mutating `messages['zh-CN']['newUserGuide.skip']` was not visible through `guideMessage`) → **GREEN 4/4** after the layering: shared-first shadowing, local-fallback on missing shared key, `guideMessage ≡ formatMessage` for all 20 keys × 5 locales, and tables-stay-byte-identical pin.
- `contextual-guide-messages.ts` (`contextualGuide.*`, a different key namespace) has **no shared counterpart registered** in `packages/i18n` yet — header note added registering the backfill as follow-up; per the slice brief this is a non-blocking leftover.

## Files changed (this slice)

| File | Change |
| --- | --- |
| `apps/web/src/platform/PlatformShell.tsx` | + reopen menu item (icon, label/aria from shared key, `openNewUserGuide` dispatch), label in `labels` |
| `apps/web/src/platform/platform-shell-guide-reopen.test.tsx` | **new** — 3 jsdom tests (existence, replay-overrides-done-key, 5-locale label) |
| `packages/i18n/src/generated/newUserGuide.ts` | ru-RU `agents.desc` drift fix (агентах → агентов); header now states the true Vue-locale source + key math |
| `packages/i18n/test/newUserGuide.test.ts` | rewritten as byte-exact pin (4 tests) |
| `packages/views/src/guides/steps.ts` | `guideMessage` shared-first layering (relative `formatMessage` import); same ru-RU drift fix; header updated |
| `packages/views/src/guides/guide-message-layering.test.ts` | **new** — 4 layering tests |
| `packages/views/src/guides/contextual-guide-messages.ts` | comment-only R007 leftover note |
| `docs/migrations/react/evidence/vue-react-parity/2026-09-13-r007-guides-closeout.md` | this file |

Net-new i18n keys: **0** (the block pre-existed from commit `2f8a6c1e`); this slice fixed **1 drifted value** (×1 locale) and added byte-level pinning of all **100 values**.

## Verification (all green)

| Command | Result |
| --- | --- |
| `pnpm run test:shared` | **387 pass / 0 fail** |
| `cd apps/web && npx tsx --test src/platform/contextual-guide.test.tsx` | **10 pass / 0 fail** |
| `cd apps/web && npx tsx --test src/platform/*.test.tsx src/platform/*.test.ts` (12 files, incl. new + pre-existing new-user-guide/contextual-guide/shell suites) | **97 pass / 0 fail** |
| `npx tsx --test packages/views/src/guides/*.test.tsx packages/views/src/guides/*.test.ts` | **34 pass / 0 fail** (30 pre-existing + 4 new) |
| `npx tsx --test packages/i18n/test/newUserGuide.test.ts` | **4 pass / 0 fail** |
| `pnpm run typecheck:shared` | exit 0 |
| `cd apps/web && npx tsc -p tsconfig.json --noEmit` | 0 errors (platform files clean) |

TDD red/green sequence: reopen entry **3 fail → 3 pass**; i18n byte pin **1 fail (real drift) → 4 pass**; layering **1 fail → 4 pass**.

## Leftovers / notes for the coordinator

1. **contextualGuide.* shared keys not backfilled** — `packages/i18n` has no `contextualGuide.*` block; `packages/views/src/guides/contextual-guide-messages.ts` keeps the local table (header note added). Backfill + layering is mechanical once registered (same pipeline as this slice).
2. **Entry placement delta** — Vue renders the reopen button as an icon inside the dropdown's user-header row; the React dropdown has no user header, so the entry is the first labelled menu item. Same key, same action, same replay semantics; visual structure differs deliberately.
3. **Pre-existing weak test replaced, not extended** — the old `newUserGuide.test.ts` (`>= 20` keys + 3 spot values) is fully superseded by the byte-exact suite; nothing else referenced it.
4. `pnpm run test:shared` already includes concurrent agents' in-flight changes (api-client chat sessions, domain/local-preferences) — all green as of this run.
5. No commits made, per slice instructions; working tree left for coordinator integration.
