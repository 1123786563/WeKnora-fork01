# 2026-09-18 Round R470 — N009 spans fix (live-verified), stop-button contract, settings error-state anchoring (4 parallel agents)

Round type: TDD round with 4 parallel agents (A1 N009 spans fix, A2 composer stop button, A3 settings
error-state anchoring, A4 verifier with live verification). Verdict: A1 PASS, A2 PASS-with-reservations,
A3 PASS; final gates: test:web 1778/1778 (+7, zero flakes), test:shared 789/789, typecheck 0, build ✓,
integrity PASS.

## A1 — N009 spans consumption fix (PASS, live-verified)

Root cause: `GET /knowledge/:id/spans` wraps in `{success, data:{parse_status, current_stage, trace,
last_error}}`; Vue reads `res.data` while the React client's `spans()` returned the raw body — three consumers
read `trace === undefined`, so the timeline collected nothing (all stages 等待中) and the poller judged
terminal on first tick. Fixed via `resolveKnowledgeSpansView` (envelope unwrap, pass-through if already
unwrapped) wired at all three consumers (detail poller, detail Trace drawer, list Trace drawer), plus
`knowledgeSpansLastError` type-narrowing; fixtures are the R469-captured real shape (130400ms total,
multimodal skipped, postprocess.summary failed×4). Live-verified by A4 with a fresh tenant account + KB +
real upload/parse: the panel renders real stage states and the waterfall — the "all waiting" symptom is gone
(`a4-n009-react-trace-drawer.png`, a three-way pair against R469's both-end baseline). documents 230/230 (+4).
Deferred (packages/** closed): multimodal skipped needs a domain-model skipped state + started_at/finished_at
aliases + a timeline.skipped key; postprocess failure aggregation onto the stage pill vs Vue's independent
「失败 4」 counter.

## A2 — Stop button contract (PASS with reservations; live gap queued highest)

Vue contract: stop shows when `isReplying` — true from sendMsg() through the whole SSE life (not just during
events); click = local abort + server stop endpoint + toast. React's stop branch already existed; the real
gaps were `streaming` only reflecting `phase === 'streaming'` (the send→first-event window showed a disabled
send) and the stop guard blocking that window. Fixed: `streaming || sending` models isReplying; the
`stopStream` guard widened to allow pre-stream aborts; +3 tests (one red reproducing N019). Attachment-button
R469 note CORRECTED: Vue has an unconditional attachment button — the real gap is the reverse (Vue's
agent-scoped image upload missing in React), recorded for adjudication. chat 158+121; full 1778/1778.
RESERVATION: A4's three live attempts did NOT reproduce the stop button (web main path keeps canSteer true and
enters the steer branch; pre-stream textarea not disabled contradicts the sending-timing assumption) — two
candidate causes unresolved; the code matches the extracted Vue contract, but the N019 live symptom persists.
Highest-priority follow-up for R471.

## A3 — Settings error-state anchoring (PASS; UX-mode diffs queued)

Symmetric route-interception (500s on identical endpoints both ends) anchored the settings error states
(12 screenshots): React renders raw backend English with NO retry everywhere (and several sections drop even
the panel title); Vue uses three aligned patterns (toast+kept UI for models; toast+central empty+retry for
skills/mcp; banner+retry for members/storage). Row upgrades: R027/R024/R033/R035 C→B (error evidence
anchored, presentation NOT aligned); R038 stays A with a P2; R018 blocked on a system-admin account. Also
noted: on insufficient permission React renders an explanation card where Vue silently doesn't open the
section. The error-UX alignment (localized copy + retry affordances per Vue's per-section pattern) is queued
as an R471 work item.

## Gates (final)

test:web 1778/1778 (+7 = A1×4 + A2×3, zero flakes under the serial-chain schedule), test:shared 789/789,
typecheck 0, build ✓ 5.95s, integrity PASS. No Vue, mobile, or Go code modified. Per-agent reports:
.omc/state/r470/report-A{1,2,3}.md + report-A4-review.md.
