# 2026-09-18 Round R478 — Toast-window re-anchor, pending mapping, question-skip hint, endgame map (salvaged round)

Round type: TDD + audit round with 4 parallel agents (A1 toast window re-anchor + pending mapping, A2
question-generation skip hint, A3 coverage-map refresh + endgame plan; A4 not dispatched — the dispatch was
interrupted but all three agents HAD executed; their staged deliverables were swept into the external
commits a605d530 + 066eb742). Post-salvage verification: **six gates ALL PASS via `pnpm gates` (v26)** on
the merged state.

## Environment note (this round's prelude)

The user's OTHER project (onyx-foss uvicorn) now holds IPv4 :8080; the WeKnora dev backend binds IPv6
[::1]:8080 — the vite proxy (localhost→::1 in Node) reaches the right backend, verified end-to-end by a
successful parity login through :5181. The onyx service was NOT touched. The mock LLM daemon was restarted
(scripts/dev-mock-llm.py direct) after the parallel stack churn; migration warning "version 157 down-file
missing" recorded (the parallel stack's newer schema vs the dev migration dir — non-fatal, login/KB verified
working).

## A1 — Toast-window re-anchor + pending mapping (delivered; in a605d530)

The 900-char source window retired: `resolveSteerSubmitFailure(copy, cause)` exported (Vue's
server-message-first fallback), behavioral assertions (Error message priority; non-Error/undefined/null/empty
→ steerFailed; the two keys proven distinct) + a single-line wiring anchor replace the window. The pending
attachment state now maps to the FIRST key (Vue's attachment state machine: pending counts as not-yet-
uploaded) per the pending-mapping contract. chat files staged early per discipline — swept in.

## A2 — Question-generation skip hint (delivered; in a605d530; SUPERSET, not parity)

Verified Vue has NO mitigation (the dialog's question section is a bare switch+count+instructions with zero
indexing-strategy linkage — both ends silently skip). React's first-adopter hint: `kbSkipsQuestionGeneration`
mirrors the backend gate exactly (only when vector AND keyword are explicitly off; absent strategy = default
both-on = no false positive), seeded into UploadConfirmUIState at all three call sites, rendering a
role=note hint under the question fieldset — the switch stays enabled (inform, not block); +1 key ×5 locale.
Documents 205/205 green (the DetailPage 38 createPortal failures are the known Node-env issue, module graph
disjoint). Backend behavior unchanged. Queued: the same hint for the Vue side, migration of the key into
packages/i18n, and the product-level root fix (backend reporting the skip reason).

## A3 — Coverage map refresh + ENDGAME plan (delivered)

Baseline rebuilt from in-tree anchors (the r468 detail table was purged; ±2-row tolerance declared). New
map: **A 56 (62.9%) / B 17 (19.1%) / C 16 (18.0%) / D 0**. Endgame list (16 C rows, prioritized):
① settings error/failure states 9-row batch (R028-R042) — one method (symmetric interception + the R471
three-mode comparison), 1-2 rounds; ② N021 tool-results — needs a tool_calls fixture (code ready via A13);
③ N015 role-denied — an adjudication + half-round; then the remaining rows. Quantified: ≤1% means C must
reach ≤1 row — 15-16 rows to close, with a per-row plan in the report.

## Gates (post-salvage, merged state)

`pnpm gates`: test:shared PASS, typecheck:shared PASS, test:web PASS, typecheck:web PASS, check:integrity
PASS, build:web PASS (exit 0). No Vue, mobile, or Go code modified by this round's agents. Per-agent
reports: .omc/state/r478/report-A{1,2,3}.md.
