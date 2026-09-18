# 2026-09-18 Round R466 — merged-view mermaid, chat prefill + history deepThink, save-button loading gate (4 parallel agents)

Round type: TDD round with 4 parallel agents (A1 documents merged mermaid, A2 chat ?q prefill + history
deepThink, A3 save-button loading gate, A4 verifier with live verification). Verdict: **all PASS**; gates:
test:web 1752/1752 (+22 = 5+15+2), test:shared 787/787 (+5), typecheck 0, build ✓, integrity 0 P0 after
staging.

## A1 — merged/chunks mermaid (PASS)

Vue doc-content's merged/chunks views render through processMarkdown and a post-render pipeline
(querySelectorAll('.mermaid') → mermaid.run → bindMermaidClickEvents with idempotent binding) →
openMermaidFullscreen on click. React: new `DocumentMarkdownBody` renders markdown then hydrates
`pre[data-markdown-diagram="mermaid"]` via the shared engine (injectable loader, re-run on page turns, race
guard, empty-scan skip) and binds the R465 fullscreen; the chunks display state upgraded from plain <p> to
markdown per Vue. documents 221/221 (+5). Left: the chunks parent-context popover (React lacks the feature
face — future).

## A2 — ?q= prefill + history deepThink (PASS)

- Prefill per Vue Input-field L1840-1846: fill + focus, NEVER auto-send, one-shot; consumed only on the
  new-chat entry (session routes ignore stray ?q=); send/clear strips the param via replaceState keeping
  sibling params. composer gains a focusSignal pulse.
- History deepThink per Vue handleMsgList L465-483: persisted <think> blocks split
  (`splitHistoryThinking`, mirror of the parser); closed-by-default 已深度思考 collapsible
  (`wk-chat-history-think`); unclosed blocks render the live 思考中 state; the tag never leaks into markdown.
+15 tests; apps chat 143/143, views chat 121/121. Whitelist lesson held (named re-export only).

## A3 — save-button loading gate (PASS)

Vue :451-453 stacks `:loading="saving" :disabled="loading"` — loading disables with unchanged copy and no
spinner (cancel stays enabled per :448, the load-failure path depends on handleClose). React: the save button
gains `disabled={editorOptions.loading}` over the existing saving state; anatomy +2 tests (pending-settings
gate; saving-independent double-submit). 10/10.

## A4 — live verification (PASS, 4 screenshots)

- `?q=测试预填` 6/6 assertions (prefill/focus/no-auto-send/param-preserved-until-consumed/strip-on-clear/
  history-unpolluted).
- Palette empty-state buttons + 420px retrieval drawer + Esc-closes-drawer-first (R465 makeup evidence; an
  initial Esc FAIL was a selector artifact, re-verified via data-testid).
- Merged mermaid: uploaded `r466-verify-mermaid.md` to the parity-smoke fixture KB → 全文 view hydrates
  (figure/svg present, raw fences zero) and click opens fullscreen.

## Gates (final)

`pnpm test:web` 1752/1752, `pnpm test:shared` 787/787, `pnpm typecheck:web` 0, `pnpm build:web` ✓; the
interim integrity P0 was the unstaged prefill-query.ts (staging clears). No Vue, mobile, or Go code modified.
Per-agent reports: .omc/state/r466/report-A{1,2,3}.md + report-A4-review.md (session artifacts).
