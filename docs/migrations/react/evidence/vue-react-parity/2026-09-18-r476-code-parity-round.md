# 2026-09-18 Round R476 — gates harness + persistent mock, differentiated steer toasts, question-generation attributed to backend gating (4 parallel agents)

Round type: TDD + infrastructure round with 4 parallel agents (A1 node pin + gates harness + mock
persistence, A2 differentiated steer toasts, A3 question-generation auto-populate investigation, A4 verifier).
Verdict: **all PASS**; final gates under Node v26: test:web 1860/1860, test:shared 800/800, typecheck 0,
build ✓ (the interim integrity P0 was the unstaged steer-toast.ts — staging clears).

## A1 — Node pin, gates harness, persistent mock (PASS)

- `engines: node>=26` + `.nvmrc` (26.7.0; NOTE the `.*` gitignore rule blocks it — a `!.nvmrc` exception
  needs a future round) + `scripts/run-gates.mjs` wired as `pnpm gates`: serial node-check → shared →
  typecheck:shared → web → typecheck:web → integrity, auto re-exec'ing a v22 shell onto v26 via a
  node-symlinks-only shim directory (fronting the whole dir drops pnpm onto v11.1.2 — a live trap found and
  fixed). Verified: v26 full test:web 1858/1858; the gates end-to-end smoke passes. The R475
  node-version qualification is now institutionalized.
- Mock persistence: `scripts/dev-mock-llm.py` (the R475 mock, verified credential-free) + a nohup
  supervisor (`dev:mock-llm`/:stop/:status; pidfile/log under /tmp; adopts legacy instances, refuses foreign
  port owners). Live-tested: 127.0.0.1 AND the LAN 192.168.3.30 path both 200; the backend's whitelisted
  calls hit it mid-round.
- INCIDENT (recovered, zero loss): 21:39-41 an external parallel session reset+clean'd the tree, deleting
  A1/A2's uncommitted deliverables; both agents re-created them from context (the mock daemon survived on
  its held inode). Lesson promoted: stage deliverables early.

## A2 — Differentiated steer toasts (PASS)

Vue's eight MessagePlugin sites mapped to React: enqueue/retry failure → steerFailed (a swallowed
retrySteer rejection fixed along the way); promote failure → steerPromoteFailed; remove failure →
steerRemoveFailed; already_injected (three answer sites) → an INFO toast with Vue's gone/already/refused
distinction on remove. Server-message-first fallback per Vue (`e?.message || t(key)`); +4 keys ×5 locale
byte-exact (views-local table); only the steer chain's operationFailed exits replaced. chat 219+125.
Deferred: follow-up-timeout key (no React path — no dead keys), attachment-warning dual keys, failure-toast
vs inline-alert carrier.

## A3 — Question auto-populate attributed to BACKEND gating (PASS; no frontend gap)

Three-way contract verified identical (Vue processConfig → multipart process_config; React seeds/overrides/
payload; backend parses and persists). Decisive API-level reproduction: a completed doc carries
process_overrides.question_generation_config={enabled,true,3} yet all chunks' generated_questions are empty,
and the backend log shows only summary+wiki enqueued — `knowledge_post_process.go:207-210` gates question
spawn on `kb.NeedsEmbeddingModel()` (vector/keyword on); the wiki-only fixture KB has both off → silently
skipped. **Vue walks the same path** — not a parity gap. +2 characterization tests pin the payload contract.
Queued as a product issue: the dialog option has no linkage to the KB's index strategy (silently no-op on
wiki-only KBs). The environment still lacks a reachable embedding model to verify the positive path.

## Gates (final, Node v26)

test:web 1860/1860 (+7 = A2 5+1, A3 2 — numbers close exactly), test:shared 800/800, typecheck 0, build ✓
5.31s. No Vue, mobile, or Go code modified. Per-agent reports: .omc/state/r476/report-A{1,2,3}.md +
report-A4-review.md.
