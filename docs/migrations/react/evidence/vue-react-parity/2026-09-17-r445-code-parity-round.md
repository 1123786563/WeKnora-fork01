# 2026-09-17 Round R445 — Shared-drawer diffs, embed rich rendering, storage editability, embedding visibility, auth copy

Round type: TDD round under degraded infrastructure — the 5- and 6-agent parallel dispatches were interrupted
three times before execution, so the round ran as a 2-agent dispatch (A1 shared drawer, A2 embed rich rendering)
plus orchestrator-executed A3 (knowledge-settings signals) and an orchestrator review pass (A5v). The relaxed
minimum (2 agents) was honored; verification combined the dispatched reviewer with full orchestrator-run gates.

## Infrastructure incident (recorded)

- Three consecutive parallel-dispatch attempts (5→5→5 agents) were interrupted before resume; the third
  dispatch's A3 agent had partially executed (it wrote the complete red test file
  `KnowledgeSettingsPage.r445.test.tsx`, 5 cases, with deeper Vue fidelity than the orchestrator's own draft —
  including the storage handleChange emitting id+provider and the wiki-only optional description), and the
  second dispatch's A4 had partially executed the auth slice (below). Both partials were kept and completed.
- Commit-integrity incident found and fixed: R442 and R444 had each missed one file —
  `git ls-files` showed `chunkingSamples.ts` (referenced by the committed KnowledgeSettingsPage.tsx) and
  `embed/markdown.ts` (referenced by EmbedEntryPage.tsx + embed-markdown.test.ts) as untracked, so a fresh
  checkout of those commits would not build. Fixed by `ca059689` (+637 lines). Follow-up: untracked new files
  referenced by committed code must be checked per round (`git status` review before every commit now includes
  a referenced-file sanity check).

## A1 — Shared-drawer diffs (PASS, 3 of 4 fixed; 1 blocked by file domain)

1. Shared card's extra 设置 button removed for non-owned shared cards (`isSharedCard` convergence; Vue
   KnowledgeBaseList.vue:304-315 renders only the info-circle detail trigger).
2. Document count badge uses Vue's `knowledge_count || '-'` on shared cards (:336); own cards keep `|| 0`.
3. Drawer close button re-audit: Vue's header is an × icon with `aria-label=general.close` (:713-716) —
   React's closeLabel moved from `common.close` to `general.close` (5 locales present). R438's note stands
   corrected in the details: × icon matches Vue; only the aria key needed the change.
4. BLOCKED: the React-only read-only banner lives in `KnowledgeDocumentsPage.tsx:3362-3366` — the externally
   occupied file. Exact deletion point recorded for the moment that domain frees up.
Also fixed an inherited test-runner hazard: an interrupted-session assertion handed a jsdom Element to
node:test's deep diff, OOM-killing the runner (~45s silent kill); converted to boolean `assert.ok(!el, …)`.
knowledge-bases 113/113.

## A2 — Embed rich rendering (PASS)

- KaTeX: `katex@^0.16.45` + `marked-katex-extension@^5.1.8` pinned to the Vue frontend's literal versions;
  wired on a dedicated `new Marked()` instance (a global `marked.use` would leak into the wiki face sharing the
  global marked) with Vue's `{throwOnError:false, nonStandard:true}`; katex.min.css imported; DOMPurify math
  passthrough verified against Vue's config. `$…$`/`$$…$$` render; code spans unaffected.
- Citation icons restored: `ziliao.svg`/`websearch-globe.svg` copied into embed assets, the R444 display:none
  removed, mask-image rules mirrored from chat-citations.less.
- Mermaid: the task assumption was REVERSED by verification — the Vue embed face DOES hydrate mermaid
  (`createMermaidCodeRenderer` behind is_completed). React-side mermaid hydration is a real gap recorded with a
  reuse path (views chat mermaid engine, mermaid 11.15.0 already pinned) for a follow-up round; images were
  already aligned (scheme-gated) and locked with a test.
embed suites 50/50 (+6); build emits the katex chunk.

## A3 — Settings signals (orchestrator-executed, red tests inherited from the interrupted partial)

- Storage instance select now binds Vue KBStorageSettings semantics: `disabled` only while the KB has files
  (the R443 probe signal), editable otherwise, with handleChange emitting BOTH the backend id and its provider
  into the draft; both persist through the config PUT (`storageBackendId`/`storageProvider` override fields).
  The committed backend stays selectable even when absent from the live backends list.
- Embedding row visibility per KBModelConfig `v-if="ragEnabled !== false || wikiEnabled"`: a pure-LLM draft
  removes the row (LLM row stays), a wiki-only draft keeps it WITHOUT the required star and with the Vue
  `embeddingWikiOptionalDesc` copy replacing the RAG description. R444's "all-off keeps the selector editable"
  test was rewritten to the Vue-accurate row-removal contract (the R444 note reflected the pre-v-if reading).
r445 tests 5/5 green; directory 74/74; typecheck clean.

## A4 (interrupted-session continuation, committed as its own slice `2f0fc5ef`)

Login language switch confirms with a `language.languageSaved` success toast rendered in the new locale (Vue
Login.vue:522-528) and OIDC-entry failures use `auth.oidcLoginFailed` instead of the generic retry copy (Vue
Login.vue:636/647). auth suites 22/22.

## Gates (final)

`pnpm test:web` 1560/1560, `pnpm test:shared` 600/600, `pnpm typecheck:web` clean, `pnpm build:web` ✓ (both the
dispatched reviewer's and the orchestrator's runs agree; +15 vs R444's 1545). No Vue, mobile, or Go code
modified. Per-agent reports: .omc/state/r445/report-A{1,2}.md + report-A5-review.md (session artifacts).
