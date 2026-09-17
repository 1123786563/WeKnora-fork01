# 2026-09-18 Round R465 — Documents mermaid viewer, palette empty-state + capability gate, live-thinking contract (4 parallel agents + orchestrator)

Round type: TDD round with 4 parallel agents (A1 mermaid viewer wiring with Vue-contract audit, A2 palette
empty-state buttons + agentsEnabled gate, A3 live-thinking + stream-404 copy, A4 verifier) plus an
orchestrator fix for A3's blocking subpath-registration miss. Verdict: A1/A2 PASS; A3 PASS after the
orchestrator's 3-line whitelist registration; final gates: test:web 1730/1730 (load-flaky reruns settled),
test:shared 782/782, typecheck 0, build ✓, integrity 0 P0 after staging.

## A1 — Mermaid viewer: wiki closed as no-op, documents wired (PASS)

- Wiki face: Vue's WikiBrowser renderMarkdown chain has ZERO mermaid hydration/viewer (re-verified post-R463)
  and React's wiki/markdown.ts is a line-for-line port of the same chain — contract already equal; closed
  under "Vue 没有的不加" with zero changes.
- Documents preview face: Vue doc-content → document-preview has the full chain (renderMermaidToSvg →
  `.preview-mermaid` click → openMermaidFullscreen with the five-control toolbar). React's MermaidPreview had
  hydration only — wired this round: `openDocumentMermaidFullscreen` reuses the R463 shared engine with the
  Vue overlay/draggable details, click opens (no-op without SVG, mirroring Vue's guard), labels per-locale
  byte-exact from Vue's mermaid.* keys (no shared-i18n additions). +3 tests; documents 216/216, wiki 59/59.
  Left: the merged/full-text view's inline mermaid hydration gap (out of the preview.ts pipeline — queued).

## A2 — Palette empty-state + capability gate (PASS)

- Empty-state buttons per Vue (:126-138): 问 AI = record recent + close + startChat(query); 调整检索参数 =
  420px right drawer reusing the existing ConfigSettingsPanel('retrieval') (no new editor); the input-row ⚙
  opens the same drawer; Esc closes the drawer first while the palette stays open.
- agentsEnabled: GET /api/v1/system/capabilities with fail-open semantics (only supported:false hides;
  organizations additionally requires non-lite + admin) — gating both the agents search fan-out (skipped
  entirely when disabled) and the quick actions (the previously-dead `visibleCommands` finally wired).
  capabilities() was already on the client (zero api-client changes). +3 i18n keys ×5 locale; 16 new tests;
  palette suites 80/80.
- Left: the askAi prefill consumer — the palette navigates /platform/creatChat?q=<query>; the chat side
  consuming `q` into the composer completes the contract (queued for the chat domain).

## A3 — Live-thinking contract + stream-404 copy (PASS after orchestrator fix)

- Live thinking: Vue's main face DOES have a streaming thinking indicator — the deepThink component (pulsing
  dot + 思考中... + live content, auto-collapsing to 已深度思考), driven by accumulated `<think>` tags in the
  answer, NOT by SSE thinking events (those only feed the agent timeline). React converged: new
  `splitLiveThinking` mirrors the parsing; LiveResponse renders the tag-driven LiveThinking block; the
  React-only SSE-thinking collapse removed.
- Stream 404: Vue surfaces `HTTP <status>` (onopen throws without reading the body) — React's
  `streamFailureMessage` unifies send/continue failures to 流式连接失败: HTTP <status> (five-locale
  streamFailed copy byte-exact), server-side SSE application errors still pass through, retry entry kept.
  chat domain 244/244.
- Blocking defect (caught by A4): the `@weknora/views/chat/live-thinking` subpath was missing from the three
  whitelists (package.json exports / tsconfig paths / vite alias) — the sole cause of the interim typecheck
  and build failures. Fixed by the orchestrator (one line ×3); typecheck 0.

## Gates (final)

`pnpm test:web` 1730/1730, `pnpm test:shared` 782/782, `pnpm typecheck:web` 0, `pnpm build:web` ✓ (the
interim 4+2 failures were load/timing flakes under a 50-60 load machine — isolated reruns and the post-fix
full runs green). No Vue, mobile, or Go code modified. Per-agent reports:
.omc/state/r465/report-A{1,2,3}.md + report-A4-review.md (session artifacts).
