# 2026-09-18 Round R464 — Palette knowledge search (P1), FAQ type gate, chat P2s (4 parallel agents + orchestrator)

Round type: TDD round with 4 parallel agents (A1 P1 palette knowledge search, A2 FAQ type gate + loading
reset, A3 chat P2×3, A4 verifier with live verification) plus an orchestrator fix for the new defect A4's
live pass found. Verdict: **all PASS**; final gates: test:web 1704/1704 (+15), test:shared 776/776 (+6),
typecheck 0, build ✓, integrity 0 P0 after staging.

## A1 — Palette knowledge content search (PASS; the R463 P1 closed)

Vue contract (GlobalCommandPalette.vue + useSearch.ts): 350ms debounce, trim-non-empty trigger; query-state
group order chunks(≤5)→messages(≤4)→kbs→agents→sessions→commands; chunks grouped per knowledgeId into file
cards (vector/keyword badges + score + matched_content highlight); KB scope chip inferred from the route with
placeholder switching, scoped = chunks-only with message search disabled, Backspace/✕ clears and re-searches;
chunk click navigates /platform/knowledge-bases/{kbId}?knowledge_id=; errors degrade to empty results.
React: new `usePaletteLiveSearch` hook (debounce, seq stale-response discard, scope lock, KB-list preload for
the chip name) + GlobalCommandPalette rewritten to the five-group rendering with unified flat keyboard
navigation (↑↓/Enter/⌘1-9 across mixed results+commands) and safe <mark> highlighting — reusing the existing
command-palette-search wiring and the R447-era api-client search verbatim (zero api-client changes).
PlatformShell passes searchClient + initialKbScope. +10 i18n keys ×5 locale (guard synced). RED 7 fail →
GREEN; platform 207/207. Deferred: empty-state askAi/retrieval-settings buttons; same-KB CustomEvent fast
path; agentsEnabled capability gate.

## A4 live verification (3 screenshots)

Debounced single request per keystroke-burst confirmed; "parity" produces the full group order with file
cards (KB tag/keyword badge/score/highlight); chunk click navigates AND opens the target document; scope chip
back-fills its name and the scoped request carries only the current KB with zero message search; a document
KB hitting /faq is replaced to the documents view with zero FAQ requests. **New defect found**: the scope
regex only matched the Vue-form alias path — the user's primary /knowledgeBase/:id path seeded nothing.

## Orchestrator — scope path fix

`kbScopeFromLocation` now matches BOTH forms (`/knowledgeBase/:id…` and `/platform/knowledge-bases/:id`),
exported for a pinning test covering both forms plus negative paths (list, chat). (Note: the Mimosa
command-injection flag on this edit was a false positive — a path regex written with String.match; no shell
surface exists in this frontend module.)

## A2 — FAQ type gate + loading reset (PASS)

Vue: `isFAQ = (type||'')==='faq'` — FAQEntryManager mounts only for FAQ KBs; there is no /faq route at all,
document KBs always land the documents view. The React 400-storm was a double defect: `load()`'s catch never
reset `hasMore`, and the fill-viewport effect had no dependency array — an empty state re-triggered loadMore
on every render (unbounded retry). Fixed: a faqGate (pending/allowed/blocked, settings-failure degrades to
allowed) replaces blocked visits to the documents view; first loads and the raw progress probe gate on
allowed (zero FAQ requests for document KBs); the catch resets hasMore — the retry chain is structurally
severed. +3 tests; faq 92/92; three fixture KB types corrected to the legal union.

## A3 — Chat P2×3 (PASS)

- Reasoning collapse REMOVED (verified subtraction: Vue renders main-face thinking only via inline `<think>`
  deepThink or the agent timeline — message-level `thinking`/agent_steps are never rendered).
- Invalid-image placeholder ported (botmsg.vue:302 contract: invalid URL → t('error.invalidImageLink')
  paragraph; five-locale byte-exact with per-language guards added).
- Model chip: root cause was the missing user-pick input, not a name-mapping gap — resolveChatModelChip now
  applies Vue's priority (explicit pick > agent binding > list-first; unmatched pick → 未配置), with
  ChatRoutePage tracking userModelPick; the loader's synthetic default no longer pollutes the chip.
views chat 111/111, apps chat 121/121. Deferred: live「正在思考」block adjudication; scoped-pick storage-key
sharing; sharedAgentModelLabel concept.

## Gates (final)

`pnpm test:web` 1704/1704, `pnpm test:shared` 776/776, `pnpm typecheck:web` 0, `pnpm build:web` ✓,
integrity 0 P0 after staging. No Vue, mobile, or Go code modified. Per-agent reports:
.omc/state/r464/report-A{1,2,3}.md + report-A4-review.md (session artifacts).
