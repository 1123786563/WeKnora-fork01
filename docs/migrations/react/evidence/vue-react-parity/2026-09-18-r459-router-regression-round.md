# 2026-09-18 Round R459 — Post-refactor navigation regression, join invite_code fix, wiki source-doc end-to-end (4 parallel agents)

Round type: verification + TDD round with 4 parallel agents (A1 browser navigation regression, A2 router URL
contract audit/fix, A3 wiki source_refs fixture + click test, A4 verifier) — the TanStack Router refactor
(external, 07d07a75) had never received a browser-level navigation regression. Verdict: **all PASS**; final
gates test:web 1665/1665 (+3), test:shared 749/749, typecheck 0, build ✓.

## A1 — Browser navigation regression (PASS, 14 screenshots)

Six guarded routes render with zero console errors (no `Failed to construct 'URL'` residue from the refactor
window); sidebar navigation aria-current and URLs track correctly; KB `?tab=` writes/clears/refresh-restores;
「全部设置」lands at `/platform/settings?section=general` (the R450 URL-normalization ruling UNCHANGED by the
refactor); back/forward history chains correct with stable history.length; Vue sidebar round-trip matches.
One observation CLOSED by A4 as aligned-not-regression: unauthenticated redirects go to bare `/login` without
`next` — Vue itself does exactly that (frontend/src/router/index.ts:368 bare next('/login'); Login.vue:597
fixed landing). Two low-confidence one-off incidents (a Page crashed, one forward landing) did not reproduce on
atomic rerun. Pre-existing contract notes restated: React KB path /knowledgeBase/:id vs Vue
/platform/knowledge-bases/:id (Vue No-match on the React form); Wiki tab is a full-screen view.

## A2 — Router URL contract audit (PASS, 2 CONCERNS)

28 legacy URL classes audited against routes.tsx pure functions + pre-router bootstrap + Vue's router/index.ts.
One REAL drift found and fixed TDD: `/join?code=c` for authenticated users hard-replaced with the query
stripped (`split('?')[0]`) → invite_code lost, violating the REDIRECT profile's exact-query-preservation and
Vue's `redirect: {query: {invite_code}}`; the fix threads the path through an `internalTarget` allowlist and
re-appends the query (matching runGuardDecision's existing pattern). Zero-drift findings documented with
forensics (e.g. /creatChat and /platform/chat were never actually redirected pre-refactor). Test-coverage gap
closed: the assembly-layer hard-replace path had zero coverage (a stubbed window.location silently swallowed
exceptions) — 3 new tests: 7 legacy platform redirects (q→cmdk, tab→section folding with unrelated params
preserved), join dual-direction handoff, kb view query matching. CONCERNS (non-blocking): AuthPages.tsx keeps
3 `?next` consumption sites (a parity superset, no open-redirect risk — cleanup queued); the concurrent
external "de-next login" work in the same files was verified Vue-aligned and adopted as the new contract.

## A3 — Wiki source_refs root-caused + first end-to-end click (PASS)

Backend forensics: source_refs format `"knowledgeID"`/`"knowledgeID|title"` (wiki_page.go L239/L823), populated
by the wiki ingest pipeline (wiki_ingest_batch.go L1846/L1968) — the fixture KB's pages were API-created and
never ingested, hence R458's all-null. Fixture created and retained: page slug `r459-source-doc-fixture`
(source_refs → real document 2dc2b763-… 「R459 来源文档溯源手册」, manual+publish via API). React end-to-end
PASSES: footer 来源文档 row renders with `a[data-source-id]`, click navigates to
/knowledgeBase/7cea6ec0-…/documents/2dc2b763-… and the document page renders — the R457 wiring's first live
verification. Vue comparison degraded to source-level (shared browser contention): WikiBrowser.vue L603-611 +
KnowledgeBase.vue L1431 open a card-details drawer vs React's route navigation — the established architecture
difference, restated.

## Gates (final)

`pnpm test:web` 1665/1665 (+3), `pnpm test:shared` 749/749, `pnpm typecheck:web` 0, `pnpm build:web` ✓ (A4's
final run; two intermediate-state failures during the window were fixed by A2 before close). No Vue, mobile,
or Go code modified by this round. Per-agent reports: .omc/state/r459/report-A{1,2,3}.md + report-A4-review.md
(session artifacts).
