# 2026-09-18 Round R474 — Dev-stack root causes overturned & fixed, N1 race + ⌘Enter, pending contract + legacy guard (4 parallel agents)

Round type: environment-forensics + TDD round with 4 parallel agents (A1 dev-stack P0, A2 N1 + shortcut,
A3 pending contract + legacy guard, A4 verifier with live verification). Verdict: **all delivered**; gates:
test:web 1833/1833 (+7), test:shared 799/799 (+4), typecheck 0, build ✓, integrity PASS.

## A1 — Dev-stack P0: assumed root causes OVERTURNED (DELIVERED, zero business code)

- "Missing vectorstore container" was FALSE: RETRIEVE_DRIVER=postgres means pgvector (present, healthy);
  wiki-only KBs skip BatchIndex entirely (NeedsEmbeddingModel=false) and chunks land unconditionally. R472's
  VECTORSTORE_WRITE_FAILED was actually the embedding call to api.openai.com being SSRF-blocked, whose failure
  branch also deleted the written chunks.
- The docreader break's real culprit: a LEFTOVER main-repo backend process (PID 15483, :8082, main-repo CWD)
  shared the same Postgres+redis queue and read the MAIN repo's .local-data — files uploaded to the worktree
  were unreadable to it. Fixed: killed the stale process; added a pure-additive socat proxy container
  (WeKnora-docreader-grpc-proxy, 127.0.0.1:50051→docreader:50051 — the original container publishes no host
  port, breaking pdf/docx remote parsing); synced the main repo's .local-data files into the worktree.
- E2E verified on tenant 10002: md → chunks 1, docreader done; pdf → REAL remote chain (container logged
  PDFParser→1 page), chunks 1 with extracted text. Documents rest at finalizing (the only model's direct-IP
  endpoint is SSRF-blocked/unreachable — known limitation; needs a reachable mock LLM + SSRF_WHITELIST
  restart). Test docs (r474-a1-e2e.md/.pdf) left in Parity KB Demo for reuse.

## A2 — N1 race fixed + ⌘Enter promote shortcut (PASS)

- N1: Vue's handleSteerMsg degrades an idle-new_run enqueue to `await sendMsg(value)`; React only dropped.
  Fixed with an applySteerEnqueueResult boolean driving the plain-send fallback on both the main and
  409-retry paths (red test asserted the second stream carried the original text).
- ⌘Enter/Alt+Enter per Vue's chatSubmitShortcut + injectCurrentInput: current input FIRST (non-empty draft →
  inject), empty draft → promote the first promotable queued steer; chip tooltip gains a `· ⌘ Enter` suffix
  (pure concatenation, no new keys); onSteer extended with an inject delivery through to the api-client.
  Plus the promoting disabled state (data-steer-promoting) matching Vue's item.promoting. +tests; chat
  197+124. Deferred (disclosed): the retry path's delivery degrades to after; no inject optimistic preview.

## A3 — Pending-span contract + legacy guard (PASS, live-verified)

Vue contract verified at the source: span status reads node.status with NO timestamp inference and
current_stage NEVER participates; a no-span stage is a pure pending placeholder; waterfall pending rows show
duration '—' (em-dash) with NO status text; only running gets inProgress copy + live duration. Three React
root fixes (spanStatus no longer swallows explicit pending into running; stateFor drops the
current_stage→running inference; waterfall pending '—'/no copy) plus the LOW-COST drawer chrome landed (LIVE
badge, 当前阶段 n/5 per Vue's currentStageIndex; TRACE_HEAD_COPY ×5 byte-exact). Heavy controls (attempt
tabs, live-tick duration) queued; 停止解析 already has a React equivalent. Legacy guard: Vue hides
edit/delete on legacy- ids AND guards the delete function (warning, no request) — React had the button guard;
the function-layer guard was the gap (fixed; existing 5-locale key). documents 240/240, domain 28/28.
A4 live-verified: chunks land, the drawer shows LIVE + 5/5 + skipped '—'; questions C→U→D smoke (the D 400 is
the backend embedding limitation, Vue hits it identically). NEW P2 queued: the upload confirm dialog's
确认上传并解析 click issues no POST (no console error) — needs Vue-side comparison.

## Gates (final)

test:web 1833/1833, test:shared 799/799, typecheck 0, build ✓, integrity PASS — no flakes across both runs.
No Vue, mobile, or Go code modified. Per-agent reports: .omc/state/r474/report-A{1,2,3}.md +
report-A4-review.md.
