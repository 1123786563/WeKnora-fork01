# 2026-09-13 — contextualGuide i18n backfill (shared bundle + byte-level pinning)

**Branch:** `codex/react-multiclient` · **Scope:** contextualGuide i18n backfill slice (no commits — left in the working tree for the coordinator). Concurrent agents' files (`apps/web/src/settings/**`, `packages/api-client/**`, `apps/mobile/**`, `packages/views/src/chat/session-sidebar*`, `apps/web/src/platform/PlatformShell.tsx`) untouched. `guides/steps.ts` guideMessage layering untouched (it consumes the separate `newUserGuide.*` block).

## 1. Key inventory (verified from Vue source, not guessed)

All contextual-guide copy in the Vue client resolves from the locale files — **no component-only inline copy exists**, so this slice has **no `vue-component-source` entries**:

- `SpotlightGuide.vue` chrome renders exactly `<labelsPrefix>.{skip, stepOf, interactHint, prev, next, done}` (`SpotlightGuide.vue:17-42`) plus `<stepI18nPrefix>.<key>.{title,desc}` (`:154-155`).
- Tour prefixes come from `frontend/src/config/contextualGuides.ts:49-130` (kbList, kbCreate, tenantModels, kbDetail, chat, agentList, agentCreate) and the two wrapper components (`KbCreateContextualGuide.vue:2`, `AgentCreateContextualGuide.vue:2`); `TenantModelsGuide.vue:28-29` switches `steps`/`stepsAgent`.
- `contextualGuide.tenantModels.needChatModelFirst` is consumed as a toast in `frontend/src/views/agent/AgentList.vue:1599`.
- `frontend/src/i18n/localeKeyAudit.ts:83-90` lists the same prefixes (cross-check).

### Locale sources (contextualGuide block)

| locale | block start |
| --- | --- |
| `frontend/src/i18n/locales/zh-CN.ts` | line 6689 |
| `frontend/src/i18n/locales/en-US.ts` | line 97 |
| `frontend/src/i18n/locales/ja-JP.ts` | line 97 |
| `frontend/src/i18n/locales/ko-KR.ts` | line 6687 |
| `frontend/src/i18n/locales/ru-RU.ts` | line 6687 |

**89 keys × 5 locales = 445 values**, all present in the locale files (verified by a flatten-and-diff script over the imported locale modules, then re-pinned by the test suite):

| group | keys |
| --- | --- |
| chrome | `stepOf`, `skip`, `prev`, `next`, `done`, `interactHint` (6) |
| `agentCreate.steps.*` | 12 steps × {title,desc} (24) |
| `agentList.steps.create.*` | 2 |
| `chat.steps.{input,kb,send,done}.*` | 4 × 2 (8) |
| `kbCreate.steps.*` | 14 steps × 2 (28) |
| `kbDetail.steps.{intro,upload,done}.*` | 3 × 2 (6) |
| `kbList.steps.create.*` | 2 |
| `tenantModels.needChatModelFirst` | 1 |
| `tenantModels.steps.{intro,addModel,done}.*` | 3 × 2 (6) |
| `tenantModels.stepsAgent.{intro,addModel,done}.*` | 3 × 2 (6) |

vue-i18n syntax survives verbatim in the shared bundle: `{current}`/`{total}` in `stepOf` and the literal escapes `{'@'}` in `chat.steps.kb.desc` / `kbDetail.steps.done.desc` are byte-identical to the locale sources (formatMessage's `{\w+}` interpolation does not touch `{'@'}`; renderContextualGuideMessage keeps resolving it as before).

## 2. Changes

| file | change |
| --- | --- |
| `packages/i18n/src/generated/contextualGuide.ts` | **new** — `contextualGuideMessages`, byte-exact port of the 445 values, key order follows the Vue locale files (structure mirrors `generated/chat.ts` / `generated/newUserGuide.ts`) |
| `packages/i18n/src/index.ts` | import + re-export `contextualGuideMessages`; spread into the merged `messages` bundle (last in the chain, after `chatMessages`) |
| `packages/i18n/test/contextualGuide.test.ts` | **new** — byte-pinning suite (5 tests), modeled on `newUserGuide.test.ts` |
| `packages/views/src/guides/contextual-guide-messages.ts` | **comment-only** — the stale R007 note ("shared keys NOT registered in packages/i18n yet") replaced with the backfill note. Runtime table untouched: byte-compare against Vue showed **zero drift**, so no R007-style dual-side value fix was needed. |

## 3. Drift audit (React local table vs Vue vs shared bundle)

A flatten-and-diff script (importing the five Vue locale modules and `CONTEXTUAL_GUIDE_MESSAGES`) compared, per locale: key sets (Vue-only / React-only) and every value byte-exactly. Result: **89 = 89 keys, 0 key-set differences, 0 value drifts across all 5 locales** — the views local table was already a faithful port. The audit is now permanent: `contextualGuide.test.ts` pins (a) shared table ↔ Vue baseline, (b) merged bundle key sets ↔ 89 per locale, (c) views local table ↔ shared bundle, so any future drift on either copy fails CI. (A suspicious-looking `Doト…` ja-JP string seen in an early transcript turned out to be a display artifact; the real value `プリセットを選ぶと…` matches on both sides.)

## 4. TDD evidence

- **RED:** `packages/i18n/test/contextualGuide.test.ts` written first → `SyntaxError: The requested module '../src/index.ts' does not provide an export named 'contextualGuideMessages'` — 0 pass / 1 fail (file-level). (First RED run also caught a wrong relative path in the test's views import, fixed before generation.)
- **GREEN:** after generating `generated/contextualGuide.ts` + wiring `index.ts` → **5 pass / 0 fail**: (1) exactly the 89-key inventory per locale; (2) every value byte-equal to the Vue locale baseline; (3) merged `messages` bundle carries the block with no locale drift; (4) `formatMessage` resolution incl. `stepOf` interpolation, `done` ×5 locales, and verbatim `{'@'}` preservation; (5) views local table byte-identical to the shared bundle.
- One iteration inside the RED test itself: the expected-key builder initially mixed bare leaves with prefixed paths (assertion diff caught it) — fixed in the test, suite unchanged.

## 5. Final verification

| suite | result |
| --- | --- |
| `pnpm run test:shared` | **392 pass / 0 fail** (387 baseline + 5 new) |
| `pnpm run typecheck:shared` | exit 0, no errors |
| `packages/views/src/guides/*.test.ts` (outside test:shared, insurance) | 28 pass / 0 fail |
| `packages/i18n/test/*.test.ts` standalone | 55 pass / 0 fail (50 baseline + 5 new) |

Other agents' WIP visible in the worktree (`apps/web/src/settings/*`, `packages/api-client/src/chat/*`, locale-sweep screenshots under `docs/.../screenshots/locale-sweep-20260913/`) was left untouched.
