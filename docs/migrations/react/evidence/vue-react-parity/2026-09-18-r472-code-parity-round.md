# 2026-09-18 Round R472 — skipped domain state, agent image upload, questions live smoke; DEV STACK WIPED mid-round (4 parallel agents)

Round type: TDD round with 4 parallel agents (A1 skipped domain + i18n keys, A2 agent image upload + steer
ruling, A3 questions live smoke + inline anchoring, A4 verifier). Verdict: A1/A2 PASS (byte-exact verified);
A3 partial through no fault (see environment event); gates: test:web 1808/1808 (+7), test:shared 795/795 (+4),
typecheck 0, build ✓, integrity PASS.

## A1 — skipped stage + three i18n keys (PASS)

Vue contract (knowledge-processing-timeline.vue): spans carry started_at/finished_at (Vue reads ONLY those);
a disabled multimodal closes as status 'skipped' (「已跳过」), and done||skipped both count as traversed.
React's domain model gained the skipped state + the timestamp aliases; the aggregate rule matches Vue
(all-skipped → skipped); the timeline renders skipped via knowledgeBase.timeline.skipped ×5 locale byte-exact
(red test reproduced the exact R470 divergence before the flip). The three common keys
(add/saveSuccess/deleteSuccess) landed byte-exact ×5 with the R471 degraded consumers switched back (button
「添加」, toasts 「保存成功」/「删除成功」). domain 9/9; i18n 15/15; documents 238/238. Deferred (optional):
ja/ko/ru's other four timeline states inherit en-US per the supplemental policy.

## A2 — agent image upload + steer ruling (PASS)

Vue contract: the image button appears only when the selected agent's config.image_upload_enabled is true
(model multimodality irrelevant; quick-answer hides), opens a hidden input (4 MIME, multiple), previews, ≤5/10MB,
and rides the SAME temporary-attachment pipeline as the paperclip (attachment_ids). React's pipeline was
already a superset (whitelist covers images, upload carries agentId) — the gap was pure button form: added
the Vue-identical button (icon/badge/active, before the paperclip) gating on the selected agent inside the
composer (host/embed untouched), routing selections into the existing onAttachmentSelect; uploadImage ×5
locale as a views-local label (Vue's paste/drag suffix dropped to avoid over-claiming). +6 tests; chat
174+121. Steer same-frame form RULED NO-CHANGE: the R471 behavior truth-table already landed; a full rework
would touch load-bearing paths across 3 test files for purely visual gain — the real gap (queued-steer chips
promote/remove) is queued as its own item.

## A3 — questions live smoke + inline anchoring (partial; environment event)

- The questions toggle IS visible for creator+admin (the R471 "toggle hidden" mystery did not reproduce; the
  likely root cause of the original observation: kb/settings or auth/me request failure silently degrading
  canMutateDocument to false). CRUD smoke: add (PUT 200, count 0→1) → inline edit (PUT 200) → delete reached
  inline confirm but DELETE 500'd — root-caused to the stack being DOWN mid-flow, not a UI/endpoint bug; the
  delete flow (confirm gate → request → failure keeps confirm state) was fully evidenced.
- Datasource inline error state anchored: raw inline error text + kept action buttons, no retry (vs the R471
  shell patterns) — a per-section pattern note for the error-UX ledger.
- **ENVIRONMENT EVENT**: a parallel process repeatedly reset the dev stack during the window (containers
  killed → backend dead → DB volume rebuilt → tenant 10001's data wiped twice including freshly created
  accounts; Docker daemon EOF once). A3 restored the containers once before it was reset again. Consequence:
  the fixture library (wiki KB, source-doc fixture, shared-fixture account, parity fixtures) is GONE — future
  browser rounds must rebuild fixtures; the DB reset also explains the smoke's DELETE 500.

## Gates (final)

test:web 1808/1808 (+7), test:shared 795/795 (+4), typecheck 0, build ✓, integrity PASS — no flakes (no
browser concurrency during the gate). No Vue, mobile, or Go code modified. Per-agent reports:
.omc/state/r472/report-A{1,2,3}.md + report-A4-review.md.
