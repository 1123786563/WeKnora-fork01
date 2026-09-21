# 2026-09-18 Round R475 — Upload no-POST closed (double root cause), mock LLM unlocks completed, inject preview wired (4 parallel agents)

Round type: TDD round with 4 parallel agents (A1 upload-dialog root cause, A2 mock LLM + SSRF whitelist,
A3 inject preview + retry delivery, A4 verifier with live verification). Verdict: **all delivered**; final
gates under Node v26: test:web 1853/1853, test:shared 799/799, typecheck 0, build ✓.

## A1 — Upload confirm no-POST (PASS; R474's P2 closed with a DOUBLE root cause)

1. Seeding semantics: the fixture KBs store `chunking_config={chunk_size:0}` (0 = unset); React's
   `num(chunk_size,512)` only falls back on non-finite, so the dialog opened with chunkSize 0 — Vue's
   `initFromKbInfo` uses `chunk_size || 512` (0 falls back). Fixed with `vueNumOr` (\|\| semantics).
2. A Vue-absent hard guard: React's confirmUpload rejected chunkSize<100||>4000 with only a small inline
   error line (no POST, no console error — exactly the observed symptom); Vue validates only
   multimodal/ASR (100-4000 is just the input's UI range). The guard is removed; the UI min/max stays.
First jsdom interaction test for the flow (drag→dialog→confirm→upload asserted, incl. the 0-regression case);
live-verified POST /knowledge/file 200 on the wiki-fixture KB. The 38 test:web failures A1 saw were the
Node-v22 toolchain artifact (below), stash-verified unrelated.

## A2 — Mock LLM reachable + SSRF whitelist (DELIVERED; completed-state verification UNLOCKED)

The SSRF implementation (internal/utils/security.go) has a legitimate whitelist: `SSRF_WHITELIST_EXTRA`
(env; the DB system_settings replaces the primary list, only EXTRA merges — local dev must use EXTRA).
192.168.3.30 IS the machine's LAN IP (last round's mock bound only 127.0.0.1). Fixes (zero business code):
the mock rebinds 0.0.0.0:18090 (now also answering the wiki-extraction prompt shape), a gitignored
.env.local carries the EXTRA whitelist, the dev backend restarted with the applied log line, and the model
endpoint needed no change (DB already pointed at the LAN IP; the builtin API path masks cross-tenant reads).
E2E: r475-a2-ssrf-mock-verified.md → parse completed, summary completed, 2 chunks — the long-blocked
completed-state pipeline is finally live. Left: the mock is session-scoped (manual restart), old finalizing
docs not batch-retried.

## A3 — Inject preview + retry delivery (PASS; completed an external partial)

The external 751b3d71 had delivered ONLY the pure-function layer (steer-preview.ts, the delivery field) plus
failing red tests — no ChatRoutePage wiring, 4 typecheck breaks. Completed: retry now uses the persisted
delivery (the R474 degradation fixed, plus a hidden param misalignment where onSteer's third argument landed
in retrySteerId); the inject optimistic preview per Vue's reconcileSteerMessageId (preview before POST, mark
failed on catch, SSE-first dedup vs server-id rebase, per-path discards; promote keeps the item as
delivery='inject'). steer 38/38, chat 214+124, both typechecks clean. Left: differentiated toasts
(steerAlreadyInjected etc.) still unified as operationFailed. (The external process swept this work into
ef62cbe7 mid-round — content verified in-tree.)

## A4 — Verdicts + the round's meta-finding (PASS)

**Node-version qualification**: the R474-declared 1833/1833 baseline measured RED this round (38+3
createPortal/createRoot failures) — a double-node experiment proved them a Node v22.22.3 toolchain issue
(registerHooks + tsx CJS named-export loss; minimal repro done); the SAME code is green under v26.7.0
(1853/1853 + 799/799). Future rounds must pin node v26 (or record the runner version with every gate).
Live-verified: the A1 fix independently reproduced and closed; the mock-LLM document completed with a green
timeline (postprocess.summary 30ms — the R474 long-failure now green), 5/5 stages, skipped '—' intact, and
the questions panel visible/regenerable. New observation queued: the upload config's AI 问题生成 3 个 did not
auto-populate (manual regenerate needed) — next-round check.

## Gates (final, Node v26)

test:web 1853/1853, test:shared 799/799, typecheck 0, build ✓. No Vue, mobile, or Go code modified. Per-agent
reports: .omc/state/r475/report-A{1,2,3}.md + report-A4-review.md.
