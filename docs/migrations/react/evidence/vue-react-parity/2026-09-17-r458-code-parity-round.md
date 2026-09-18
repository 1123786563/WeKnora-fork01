# 2026-09-17 Round R458 — Contextual-guide flake eliminated; external router refactor documented (3 parallel agents)

Round type: TDD round with 3 parallel agents (A1 flake elimination, A2 browser evidence, A3 verifier).
Verdict: **A1 PASS** (4× full-suite green during its window); A2's live evidence documented two things: the
R457 wiki source-doc wiring states and an EXTERNAL router-refactor WIP that now breaks three routing tests.

## A1 — Contextual-guide flake eliminated (PASS)

Root cause: the optional chat-kb step needs a 13-hop setTimeout retry ladder (GUIDE_LOCATE_RETRIES=12) before
auto-skipping, but the test budgeted a fixed 40ms sleep — green in isolation (~62ms), intermittent under
full-suite concurrency. Fix (test-only, +28/−19): a `waitFor(check, label, timeoutMs=5000)` act-stepped
polling helper replaces 10 fixed-sleep+assertion sites; semantic assertions unrelaxed, negative assertions
verified safe and untouched, no fake timers. Evidence: single-file ×3 green; full `pnpm test:web` ×2 green at
completion; CPU-saturated stress (10 busy-loop processes) still 14/14. Sibling guidance tests may reuse the
pattern (noted, not in domain).

## A2 — Browser evidence (read-only; report on time, verifier note resolved)

- Wiki source-doc: the fixture pages carry `source_refs: null` (backend-verified) so the footer row cannot
  render — the R457 wiring remains click-untestable until a fixture page has sources; the footer's
  linkedFrom row renders correctly.
- Live-caught an EXTERNAL mid-refactor regression: an untracked `apps/web/src/router.tsx` (new router WIP by
  the external process) calls `new URL(href)` without a base, white-screening every guarded route
  ("Failed to construct 'URL'") — confirmed by atomic rerun, with router.test.tsx also present.
- Settings zh-CN check and graph node-click were blocked by that white-screen.

## Attribution (final tree state)

`pnpm test:web` on the current tree: 1660 tests, 1657 pass, **3 fail — ALL in the external router WIP's
surface** (authenticated deep links / anonymous redirect / unmatched public paths), and `pnpm typecheck:web`
has 16 errors — ALL in `src/router.tsx` (15) + `src/router.test.tsx` (1), the external WIP files. A1's slice
is green in isolation (14/14) and its flake fix was proven with 4× full-suite green runs before the external
refactor advanced. No Vue, mobile, or Go code modified by this round.

## Gates (final, attributed)

- A1's slice: contextual-guide 14/14 ×(1 scoped + full-suite green ×4 during its window).
- Current tree: test:shared (not re-run this attribution; last green 606→ later runs), test:web
  1657/1660 attributed to external router WIP, typecheck 16 errors attributed to external router WIP.
- The R457 wiki source-doc wiring is committed (7a86a0a2 by the external process — its message confirms it
  picked up the wiring screenshots).

Per-agent reports: .omc/state/r458/report-A{1,2}.md + report-A3-review.md (session artifacts).
