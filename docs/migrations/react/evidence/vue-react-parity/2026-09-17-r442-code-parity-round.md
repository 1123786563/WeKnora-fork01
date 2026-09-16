# 2026-09-17 Round R442 — Chunking UI finish, wiki reader closeout, embed chat parity, platform shell sweep (TDD, 5 parallel agents)

Round type: TDD round with 5 parallel agents (A1 knowledge-settings finish, A2 wiki reader closeout, A3 embed
chat parity, A4 platform-shell sweep, A5 verifier) plus an orchestrator closure of the verifier's A3 CONCERNS.

## A1 — Knowledge settings finish (PASS)

- Separator control rebuilt as Vue's tag chips (t-select multiple creatable filterable semantics): selected
  values render as removable chips (presets show i18n labels, custom values show raw), Enter adds, dropdown
  filters presets, Backspace pops the last chip, Esc clears the draft without submitting.
- 「测试分块效果」preview entry ported: text trigger beside the strategy select opens a 720px right drawer
  (sample textarea 64K, four presets auto-loaded verbatim from Vue chunkingSamples.ts, existing
  `previewChunking` API, tier normalization recursive→legacy, six-tile profile, stats row, expandable chunk
  cards).
- models validation joins `validateForm`: embedding required when `vectorEnabled||keywordEnabled`
  (`indexing.embeddingRequired`), LLM always required (`messages.summaryRequired`), failing blocks the request
  and jumps to models.
- Ledger corrections recorded (「Vue 没有的不加」enforced): the "200K badge" does NOT exist anywhere in the Vue
  repo (only upload copy mentions 200K — grep-verified), and the Embedding-locked warning binds to
  `ragEnabled && hasFiles`, not keyword-only. Both corrected entries removed/blocked accordingly.
- Blocked honestly: `isIndexingLocked` needs a files-count signal the KB payload lacks (contracts and the Go
  handler verified; Vue derives it from a separate `GET /:id/knowledge?page_size=1` total — implementation plan
  recorded, needs a standalone round because it adds a request to 12 existing count assertions).
Directory regression 72/72; i18n guards 9/9 (zero new keys).

## A2 — Wiki reader closeout (PASS)

- Image preview dialog ported (Vue picture-preview: mask + centered image + zoom controls + close, Esc, mask
  click closes) wired through the content click handler.
- Index view moved into the reader pane and rendered through the R441 markdown pipeline (Vue
  `renderedIndexMarkdown`), with `stripLegacyIndexDirectory`/`appendWikiIndexDirectoryLines`/
  `assembleWikiIndexMarkdown` ported verbatim.
- Reader footer 「Linked from」/「Source documents」 implemented (Vue footer L591-612; backend wiki_page.go
  carries both fields). Partial block recorded honestly: source-title hydration needs a knowledge-details
  endpoint missing from api-client and a host-level `onOpenSourceDoc` (exposed as an optional prop; titles fall
  back to Vue's own `id|title` / truncated id). Scoped 38/38 across the three suites.

## A3 — Embed chat parity (PASS after orchestrator closure)

Verified against the Vue embed pipeline before coding: history restore loads the latest 20 messages
(useEmbedChatSession getmsgList), answer references come from the chat SSE event fields with the same three-level
fallback, and suggested questions come from `GET /embed/:channelId/suggested-questions` shown only when the
channel flag is on and the visitor has not spoken. All three implemented (`chat-data.ts` new module, references
block mirrors docInfo's collapsible shape with web links / expandable chunks, suggestion cards reuse the submit
path). 8 new tests red → green.
Orchestrator closure of the verifier CONCERNS: `referenceHeadline` counted raw chunk entries while Vue docInfo
counts document GROUPS keyed `knowledge_id || knowledge_title || id` — same-document chunks inflated the count.
Fixed with a grouping Set + behavioral test (5 chunks across 3 documents → "Referenced 3 document(s)"; mixed
zh branch too). Scoped embed suites 21/21.
Deferred: history scroll pagination (before_time), per-message follow-up suggestions, markdown citation pills.

## A4 — Platform shell sweep (PASS)

Five contract areas audited: sidebar collapse (260/60px, 250ms ease, activation, collapsed search/session
hiding), tenant switcher (visibility, switch closes menus, current-tenant no-op, 当前+role tag), user menu
(entries, admin gating, guide reopen event replay), global command palette (⌘K/Ctrl+K + bare /, Esc, arrows,
recent per user+tenant, `?cmdk=`), and empty/loading/no-permission states — all matching. Two gaps fixed by
TDD (red 2 assertions → green, platform 170/170): the collapse persistence key now matches Vue's
`sidebar_collapsed` (React wrote `weknora_sidebar_collapsed`; legacy-key list updated) and the expanded logo row
renders the literal `Lite` sup under `weknora_lite_mode` (Vue leaves it untranslated — no locale keys needed).
Deferred: organizations pending-approval badge, memberships 2s throttle refresh, last-active persistence on
tenant switch (App.tsx/main.tsx domain), collapsed drag handle.

## Gates (final)

Re-run on the final code (including the referenceHeadline closure): `pnpm test:web` 1499/1499 (+26 this round),
`pnpm test:shared` 596/596, `pnpm typecheck:web` clean, `pnpm build:web` ✓. A5's pre-closure run: 1498/1498
with zero regressions vs baseline 1473. No Vue, mobile, or Go code modified.
Per-agent reports: .omc/state/r442/report-A{1..4}.md + report-A5-review.md (session artifacts).
