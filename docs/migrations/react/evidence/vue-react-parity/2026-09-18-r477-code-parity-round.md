# 2026-09-18 Round R477 — Early-staging discipline, steer attachment dual-keys, embedding positive path proven (4 parallel agents)

Round type: process-hardening + TDD round with 4 parallel agents (A1 discipline + gates build, A2 steer
attachment warnings, A3 embedding model + positive-path reproduction, A4 verifier). Verdict: **all PASS**;
final gates via `pnpm gates` (six, now including build:web): test:shared 811/811, test:web 1861/1861,
typecheck ×2 clean, integrity ✓, build ✓ — no flakes.

## A1 — Process hardening (PASS)

`!.nvmrc` gitignore exception (the `.*` rule was the blocker; verified via check-ignore exit=1 — noting the
`-v` mode's negation-rule trap) with .nvmrc staged; gates now runs SIX gates ending in build:web (+3 lines,
full live pass in 98s); the early-staging discipline landed both as a new「流程教训（跨轮沉淀）」section in the
ledger AND as practice — all four deliverables staged on completion (the R476 reset+clean incident's direct
countermeasure).

## A2 — Steer attachment dual-keys (PASS)

Vue's two ordered gates (Input-field.vue 1944-1958, isReplying && canSteer): uploading-attachment → toast
steerAttachmentPending; any attachment/image → toast steerHasAttachments. React's zh-only composite key
retired; `resolveSteerAttachmentWarning` (pure, upload-first order) + an onSteerWarning toast channel with an
inline fallback; the host reuses the R476 steer toast bus. +2 keys ×5 byte-exact (keyof-derived type
closure); zero residue of the old key (independently grepped). chat 220+136. One regression fixed in-flight
(a test's 900-char source window stretched to 979 — compressed the gate code back under). Cleanly cut from
the external A13 phase-4a's adjacent chat-copy changes (steer section only). Deferred: the pending-state
mapping nuance; the 900-char window as an implicit contract (re-anchor queued).

## A3 — Embedding model + the positive path PROVEN (PASS; zero business code)

The mock already served /v1/embeddings (8-dim — dimension matched via embedding_parameters, not a fake
1536); extended with a `<main_content>` branch returning three question lines (the backend splits by line).
The R476 blocker's real cause: the vector KB lacked `embedding_model_id` (settable directly at creation).
Registered `r477-mock-embedding` (id 30751f9f, dim 8) and bound it. E2E on a vector KB: document completed,
1 chunk, **exactly 3 generated_questions auto-persisted and vector-indexed** (decisive log: Indexed 3
generated questions … index(entries=3,succeeded=true); 9 whitelisted embedding calls). This POSITIVELY
proves the R476 product issue: the pipeline works when the gate is satisfied — the UX gap (silent skip on
wiki-only KBs) stands as the issue. Test KB deleted; the embedding model kept as a fixture for future
vector-KB rounds.

## A4 — Verdicts + a methodology note (PASS)

All three upheld (check-ignore re-verified; the dual-keys' cut from A13 verified; A3's evidence chain
independent-checked). Gates: six green via `pnpm gates` (+3 tests close exactly). NOTE: running
run-gates.mjs with a bare v26 node does NOT trigger the re-exec and lets child PATHs fall back to v22 —
**gates must always run through `pnpm gates`**. Recommended: stage A3's mock script change (done at close).

## Gates (final)

test:shared 811/811, test:web 1861/1861, typecheck ×2 0, integrity ✓, build:web ✓ (all six via pnpm gates
under v26). No Vue, mobile, or Go code modified. Per-agent reports: .omc/state/r477/report-A{1,2,3}.md +
report-A4-review.md.
