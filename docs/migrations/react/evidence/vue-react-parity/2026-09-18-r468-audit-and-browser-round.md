# 2026-09-18 Round R468 — Parent-context blocked (api-client gap), coverage map, five-surface browser pairing (4 parallel agents)

Round type: audit + browser round with 4 parallel agents (A1 chunks parent-context, A2 matrix coverage
audit, A3 five-surface browser pairing, A4 verifier). No production code changed this round (A1 BLOCKED on an
api-client gap; A2/A3 read-only). Verdict: A1's block upheld by the verifier, A2 spot-checked 3/3, A3's
evidence landed; final gates: test:web 1762/1762 (two interim failures triple-proven flaky), test:shared
787/787, typecheck 0, build ✓, integrity PASS.

## A1 — chunks parent-context popover (BLOCKED, contract documented)

Vue contract (doc-content.vue L1479-1521/L1886-1908): chunks with parent_chunk_id show a git-branch icon
(viewParentContext); click opens a bottom-right 520px popover lazily loading
`GET /api/v1/chunks/by-id/{parent_chunk_id}` (cached per id, failure toasts parentContextLoadFailed and
closes), rendering only the parent text as markdown; mutually exclusive with question/history popovers.
BLOCK: the trigger data transmits fine (parent_chunk_id passes through), but the popover body needs the by-id
endpoint — absent from the api-client (which also exposes no generic request), and packages/** was outside
A1's domain. The backend route EXISTS (routes_knowledge.go:37, GetChunkByIDOnly); i18n keys already
generated ×5 locale. Unblocking contract for the next round: `documents.getChunkById(chunkId)` → GET
/api/v1/chunks/by-id/{id} with envelope-validated KnowledgeChunk; also evaluate the generated-questions
popover (same endpoint family, also missing).

## A2 — Coverage map (the ≤1% gap map)

89 matrix rows graded A(browser-paired)/B(code-tested)/C(thin)/D(none): **50 A (56.2%) / 10 B / 29 C / 0 D**.
documents/wiki/KB are essentially all A; chat is the thinnest high-frequency domain (7 C rows);
settings holds 17 C rows (error/failure states concentrated). Cross-cutting findings: early screenshot assets
(accept-20260914-round5, state-coverage-20260913|14) referenced by the matrix no longer exist in the tree
(earliest surviving dirs are R428+); the ledger lacks R434/R454 headings (both have screenshot dirs);
evidence paths resolve under docs/migrations/react/. Top10 next-verification list produced (N019 composer →
N021/N022 tool/approval → N023 → N009 → settings error sweep → N017 → redirects → N024 WS → re-capture →
N030/N032); completing 1-7 lifts A coverage to ~77.5%.

## A3 — Five-surface browser pairing (19 screenshots; new P1 found)

Aligned (high fidelity): KB list (cards/badges/filters; the ⋯ menu re-verified first time since R437 —
identical 置顶/创建副本/设置/删除), KB documents tab (columns/badges/upload/filter set), agents page digest
verbatim, settings five-group nav, console clean both ends. New diffs:
- **P1**: Vue's new chat has a 4-step wizard (model 1/4 + knowledge scope @KB selection) that React lacks;
  Vue's 近7天 session groups are non-clickable headers where React's navigate directly.
- P2: KB detail keeps React's global session sidebar vs Vue's KB-back nav; new-chat suggestion area shapes
  differ; chunking field sets differ both directions (React: strategy chips/token limit/table metadata;
  Vue: separator chips/long copy/advanced/quick values).
- P3: composer model chip subtitle, a 🦞 emoji nav glyph Vue-only, menu trigger semantics.
Process notes: a stale React token caused a skeleton hang (probe screenshot kept), recovered by re-login;
A3's report landed past the verifier's grace window — folded in here (verifier had noted the screenshots
arriving).

## Gates (final)

`pnpm test:web` 1762/1762 (interim 2 failures isolated-rerun green + full rerun green — flaky, not
regressions), `pnpm test:shared` 787/787, `pnpm typecheck:web` 0, `pnpm build:web` ✓, integrity PASS. No Vue,
mobile, or Go code modified. Per-agent reports: .omc/state/r468/report-A{1,2,3}.md + report-A4-review.md.
