# 2026-09-18 Round R471 — stop gap closed (live-verified), settings error-UX modes, generated-questions landed (4 parallel agents)

Round type: TDD round with 4 parallel agents (A1 stop-button root cause, A2 settings error-UX, A3
generated-questions popover, A4 verifier with live verification). Verdict: **all PASS**; gates: test:web
1801/1801 (+23 = 10+6+7), test:shared 791/791, typecheck 0, build ✓, integrity PASS.

## A1 — Stop-button live gap CLOSED (PASS; R470's highest-priority item resolved)

Both R470 candidates confirmed, ① primary: Vue's canSteer is `isAgentStreamSession()` (quick-answer sessions
have NO steer — stop is the only mid-reply action) while React passed `canSteer={Boolean(onSteer)}` with the
page always supplying onSteer, so the live main path never entered the stop branch; ② Vue's createSession
clears the draft on BOTH send and steer emits while React's three submit layers had no clear point, keeping
`streaming && (!canSteer || draft)` false. Fixed: an explicit `canSteer` prop gated by
`Boolean(selectedAgentId)` (agent pipeline ⇔ Vue's isAgentStreamSession), SteerComposer hidden for quick-answer,
and `submitDraft` clears the draft immediately (Vue clearvalue). LIVE-VERIFIED first time: stop appears within
84ms (replacing, not disabling, the send button), the draft clears in sync, and clicking stop rolls back in
565ms. +10 tests; chat 168+121.

## A2 — Settings error-UX three modes (PASS)

Per-section matrix aligned to the Vue catch branches: models = localized toast + UI kept (title preserved, no
raw backend text); skills = toast (backend-message-first / loadFailed fallback) + central empty + retry; mcp =
pure-localized toast + empty + retry; members = banner with raw message + retry; storage = shell banner + raw
message + retry, title kept. A new imperative settings-toast bus (3s auto-dismiss, MessagePlugin semantics).
Two SOURCE-vs-EVIDENCE discrepancies documented (Vue ModelSettings actually passes error.message through —
React follows the R470 task spec/localized copy; Vue's skills/mcp empty states have no retry button in source
— React implements it as a sanctioned superset): flagged for an upstream-Vue adjudication, not silently
chosen. +6 tests; settings+platform 482/482; knowledge-settings consumers 108/108.

## A3 — generated-questions landed in full (PASS; R469 contract, 2 corrections)

Vue contract re-read corrected the R469 digest twice: DELETE carries `{question_id}` as a JSON BODY (not a
query param — A4's decisive probe: with-body reaches the embedding sync, without errors "Question ID is
required"), and regenerate has NO confirm dialog (loading + toast; success writes generated_questions_revision
clearing staleness). api-client gains the type + upsert/delete/regenerate (envelope-validated); the documents
panel implements the full CRUD (help-circle entry gated on questions>0 || canEdit, count/stale hints, add
composer, inline edit, inline-confirm delete, regenerate, empty state, legacy id compatibility, mutual
exclusion, doc-switch close, canEditKB||admin gating). +2 api-client +7 documents tests; 36/237 green.
Deferred: three common i18n keys absent in React (saveSuccess/deleteSuccess/add — degraded to
success/confirm), the Vue questionDeleted emit has no React consumer (not added).

## Gates (final)

test:web 1801/1801, test:shared 791/791, typecheck 0, build ✓, integrity PASS (one interim 3-P0 was A2's
untracked files — staged; one test:web first-run failure was CPU contention with the live browser — isolated
rerun green in 6.9s). No Vue, mobile, or Go code modified. Per-agent reports:
.omc/state/r471/report-A{1,2,3}.md + report-A4-review.md.
