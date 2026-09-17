# 2026-09-17 Round R446 — Embed mermaid hydration + paired-browser regression sweep (4 parallel agents)

Round type: TDD round with 4 parallel agents — A1/A2 paired-browser regression sweeps over the six code rounds
since the last full browser pass (R439-R445), A3 implementing the R445 mermaid gap, A4 verifier. A4 verdict:
**A3 PASS with zero rework**; gates on the merged state: test:web 1571/1571 (+11), test:shared 602/602,
typecheck clean, build ✓.

## A3 — Embed mermaid hydration (PASS)

The R445 gap closed: mermaid blocks in embed answers now hydrate after message completion, mirroring Vue
EmbedBotMessage.vue (`createMermaidCodeRenderer('mermaid-embed-botmsg')` semantics): the markdown renderer tags
mermaid fences (`data-markdown-diagram="mermaid"`, DOMPurify ALLOWED_ATTR extended by exactly that attribute),
`hydrateEmbedAnswerAnswerMermaid(root, isCompleted)` delegates to the shared views engine (mermaid 11.15.0 via
packages/views — no direct dependency), the live answer row gains `is_completed: true` on submit success (Vue
turn-closure parity), failures degrade to the plain code block without throwing. 11 controlled-promise tests
(no real mermaid, no flake — triple-run verified); embed suites 61/61; mermaid ships as its own async chunk.
Deferred (views-readonly constrained): badge/fullscreen chrome, per-color theme alignment.

## A1 — Browser regression, settings/wiki/login (24 screenshots)

Verified live: the R444 chunk-overlap fix (React 80 = Vue), separator chips anatomy, the Embedding row
required-star with vector on, wiki markdown + [[link]] navigation + footer 「被链接」, login language toast on
both ends. NEW DEFECTS (R447 candidates):
- **D1 (high)**: the React settings page auto-redirects to the VUE origin
  (`:5180/platform/knowledge-bases?scope=all`) after ~8s idle — reproduced 3× with URL timelines. Root cause
  unknown (no obvious cross-origin navigation in the settings code); needs investigation.
- **D2 (high)**: 「测试分块效果」preview fails with 「预览失败 Invalid chunking preview response」 — the
  drawer's response-shape expectation does not match the backend/API (R442 slice). Stats/cards unobservable
  until fixed.
- D3: chunking strategy dropdown renders blank (React) vs Vue values; D5: activity empty-state card coexists
  with the audit table; D6: untranslated `general.helpAndDocs` in the user menu; D7: 「退出」 click ineffective.
- D4 (correction): Parity KB Demo holds 1 file (not 0) — the storage select being disabled on BOTH ends is the
  correct Vue contract behavior (R445's "editable when no files" logic stands, this fixture simply isn't empty).

## A2 — Browser regression, shared drawer/badge/embed states (15 screenshots)

- Shared drawer R445 regression PASSES on both ends with the R444 fixture: no 设置 button, "-" count, close
  aria, five fields verbatim, read-only entry.
- Organizations: pending-approval badge correctly absent on both ends (no pending requests); the 待审核申请
  empty state exists on both. Recorded diff: the tab is named 待审核申请 on React vs 加入申请 on Vue.
- Embed entry live states captured: no-token → 「缺少嵌入渠道或 Token」, fabricated token → 「无效的嵌入渠道」
  (exchange 401) — matching Vue's copy. Full handshake/chat could not be exercised (no embed channel
  configured in the environment); the mermaid baseline for A3 was locked from the no/invalid-token states.
- Process note: the shared Playwright browser was contended between A1/A2; A2 re-captured atomically after
  detecting polluted first takes.

## Gates (final)

`pnpm test:web` 1571/1571 (+11 = A3), `pnpm test:shared` 602/602, `pnpm typecheck:web` clean, `pnpm build:web` ✓
(A4 final run on merged HEAD 09aba1ba; the external process landed commits mid-round including a fix for a
transient attachments.ts TS error — attributed there, not to this round). No Vue, mobile, or Go code modified
by this round. Per-agent reports: .omc/state/r446/report-A{1,2,3}.md + report-A4-review.md (session artifacts).
