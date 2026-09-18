# 2026-09-18 Round R463 — App form structure aligned, mermaid viewer toolbar, four-surface sweep (4 parallel agents)

Round type: TDD round with 4 parallel agents (A1 App.tsx form structure, A2 four-surface browser sweep, A3
mermaid viewer toolbar, A4 verifier). Verdict: **A1/A3 PASS**; A2's sweep queued the next round's head (a P1).
Final gates on the merged state: test:web 1689/1689 (+15 all this round), test:shared 765/765 (+9), typecheck
0, build ✓, integrity 0 P0 after staging.

## A1 — KB editor form structure (PASS)

Vue's editor modal has ZERO native `<form>` elements — footer buttons are @click (handleSubmit :451), name
input is maxlength-only, no Enter semantics anywhere. React converged: the wrapping form became a div with the
same class, the save button type=button onClick invoking the identical R437/R440/R441 pipeline (double-submit
guard, empty-name block, section-jump validation, base-update-then-config-PUT order byte-identical), name
`required` removed (JS guard governs, per Vue). +4 anatomy tests including an all-sections DOM walk asserting
form count 0 — the R462 hydration-nesting source is structurally gone. 33/33 App-related tests. Left: Vue's
save-button `:disabled="loading"` and the storage-change confirm branch (both pre-existing gaps, queued).

## A3 — Mermaid viewer toolbar (PASS)

Vue mermaidViewer.ts contract fully mirrored in a new shared engine `packages/views/src/chat/mermaid-viewer.ts`
(labels injected — views stays i18n-free): five controls zoomIn/zoomOut/reset/download/close, STEP 0.2 clamp
[0.2,10] (wheel zoom anchored to the cursor with preventDefault, buttons same rule), drag pan, reset clears
scale+pan, PNG download (white-backed canvas at render size, `mermaid-diagram.png`, Blob URL + revoke, 1.5s
downloading flash; SVG-Blob fallback under jsdom — both paths network-free). Wired into the embed face's
fullscreen overlay (+5 keys ×5 locale byte-exact in embed messages only; packages/i18n untouched). 11 new
tests; embed 68/68, views chat 100/100, i18n 73/73, build ✓. Left: wiki/documents faces unwired (domains
closed this round; the toolbar attach is ready).

## A2 — Four-surface sweep (27 screenshots; queue populated)

- **P1**: the React global command palette lacks knowledge content search entirely — typing issues zero
  knowledge-search requests, renders no chunk cards, no KB scope filter — while `command-palette-search.ts`
  wiring EXISTS unused (a comment marks live search "intentionally out of scope (S03)"). Vue's palette runs
  POST /api/v1/knowledge-search with grouped results and scope placeholder switching. Next round's head.
- P2×5: the FAQ route lacks Vue's document-type gate (empty state + 14 rapid 400s on a document KB);
  React-only reasoning collapse block; missing image-placeholder copy; session-header model naming mismatch;
  (+1 P2 folded into the FAQ gate item).
- P3×4: agents favorites-count badge text; streaming 404 copy generality; two minor notes.
- Agents main view otherwise verbatim-identical (grouping, cards, scope segments, URL sync).

## Gates (final)

`pnpm test:web` 1689/1689, `pnpm test:shared` 765/765, `pnpm typecheck:web` 0, `pnpm build:web` ✓; the interim
integrity P0 was the unstaged mermaid-viewer.ts (commit clears). The external process advanced A11 phases
mid-round (backend-only diffs verified empty on apps/packages). No Vue, mobile, or Go code modified. Per-agent
reports: .omc/state/r463/report-A{1,2,3}.md + report-A4-review.md (session artifacts).
