# 2026-09-17 Round R436 — Code-parity TDD round: approval countdown, settings save pipeline, doc preview gates, org pending-upgrade

Round type: code-level TDD round (5 parallel agents — A1 chat approval countdown, A2 legacy settings save
pipeline, A3 document preview/merge review, A4 org hasPendingUpgrade, A5 verifier) plus an orchestrator
verification pass that fixed the verifier-flagged A3 concerns against live backend data, plus a paired-browser
evidence pass on the touched surfaces.

## A1 — Tool approval countdown (chat)

Vue contract (ToolApprovalCard.vue): deadline = `(requestedAt||0)*1000 + (timeoutSeconds||600)*1000`; 1s interval;
`<60s` uses i18n `countdownShort` (`{seconds}s`), otherwise zero-padded `m:ss`; `<=30s` critical red, `<=120s`
warning orange; expiry neither auto-rejects nor disables the buttons (timer parks at 0); resolved approvals do not
render the timer. Implemented via three pure functions (`approvalCountdownSeconds`/`approvalTimerClass`/
`formatApprovalCountdown`) + component interval, five-locale `approvalCountdownShort`, and a web-layer bridge
(`approval-state.ts` + `ChatRoutePage.tsx`) that extracts `requested_at`/`timeout_seconds` from the raw SSE events
(confirmed present in `internal/event/event_data.go`) and merges them onto pending approvals by pendingId.
Accepted delta: timer renders at the end of the R435 actions row instead of restructuring the card inline.
Verifier: PASS (SSE chain, thresholds, locales, flake-free timer tests).

## A2 — Legacy knowledge-settings save pipeline

The legacy `/knowledgeBase/:id/settings` surface (R435 gave it live inline sections) had no persistence. Now:
save via `PUT /api/v1/initialization/config/:kbId` with the Vue `KBModelConfigRequest` payload shape (full KB
round-trip, pending parser rules substituted, `vector_store_id` excluded per Vue's immutable binding); save-in-
progress disabled + aria-busy + duplicate-click guard; success feedback + failure keeps the form editable with
the server error surfaced; button rendered only for owner/admin (`knowledgeSettingsCanEdit`); zero new i18n keys.
Verifier: PASS. Live-browser check this round: clicking 保存配置 with unchanged config round-trips the real
backend and shows 配置保存成功, button re-enables
(`screenshots/r436-20260917/react-kb-settings-save-success.png`); the datasource section mounts the live
DataSourcesPage inline.

## A3 — Document preview/merge review + orchestrator follow-up

Review verdicts: three-state switching, error/retry, processing timeline already aligned (R435/R370). Two gaps
fixed by TDD: GAP-1 merged-content assembly now ports Vue `mergeChunks`/`appendChunkContent` (MIN_OVERLAP=12,
`start_at` ordering) with pagination in BOTH 全文 and 分块 views (previously >25-chunk documents were stuck on
page 1 of 全文); GAP-2 `canPreviewDocument` port of Vue `canPreview` (audio excluded, default 全文 + embedded
player).

Orchestrator follow-up (verifier CONCERNS, confirmed against live data): the gate initially keyed on
`source !== 'file'`, but the real backend carries `type: 'manual'|'file'|...` and `source: ""` for file uploads —
the shipped fixture (`source: 'file'`) does not exist in real payloads. Re-keyed both `canPreviewDocument` and
`documentDetailTitle` (title extension stripping never worked on real data) onto `type === 'file'`, matched
Vue's `resolveFilePreviewExt` priority (explicit `file_type` wins over the filename suffix), and added the
`type?: string` field to the shared `KnowledgeDocument` contract (present in `internal/types/knowledge.go`).
Regression fixtures now use the real payload shape (`type: 'file'`, `source: ''`).
Live-browser confirmation on the fixture KB's real document (`type: "manual"`, `parse_status: "failed"`):
React shows no preview tab, defaults to 全文, keeps the `.md` title suffix — identical to the Vue drawer opened
on the same document (`screenshots/r436-20260917/react|vue-doc-detail-manual-type-gate.png`).
Deferred: page-turn keeps stale content + small loading in Vue (React swaps the whole pane); manual docs still
parse-status-gated in React (Vue doesn't check).

## A4 — Organization hasPendingUpgrade

`has_pending_upgrade` confirmed in the backend (`internal/types/organization.go:428`, detail endpoint) and Vue
`OrganizationSettingsModal.vue` semantics: pending disables the upgrade entry/submit and switches
title/aria to 审核 (`organization.upgrade.pending`), popup always shows the current-role bar, successful submit
sets pending locally. Implemented: api-client `Organization.has_pending_upgrade` + boolean normalization;
`OrganizationsPage` fetches the detail endpoint on modal open (stale-guard), disables/resubmits-guard the
submit, current-role tag bar; no new i18n keys. Verifier: PASS.
Deferred (carrier delta, recorded): Vue gates the popup trigger button itself and patches the list store after
submit; React keeps the R435 inline form with submit-level gating.

## Gates

- Final: `pnpm test:web` 1359/1359, `pnpm test:shared` 574/574, `pnpm typecheck:web` clean, `pnpm build:web` ✓
  (run after the orchestrator's A3 follow-up).
- A5 baseline at round start: 1340/1340 + 570/570, all green.

## Browser evidence (this round)

- `screenshots/r436-20260917/react-doc-detail-manual-type-gate.png` + `vue-doc-detail-manual-type-gate.png`:
  paired doc-detail on KB `22d38cb7` document `a35e5ca0` — no preview tab / default 全文 / title keeps `.md` on
  BOTH sides (manual type gate).
- `screenshots/r436-20260917/react-kb-settings-save-success.png`: React settings save pipeline with 配置保存成功
  after a real PUT round-trip; Vue editor-modal pair not captured this round (its entry point was not located
  within the session's timebox — the KB list card exposes no direct edit button without hover interaction;
  recorded as the first item for the next browser pass).
- Chat approval countdown and org pending-upgrade are code+test verified; live approval flow requires a real
  tool-call round and is left for the next interactive round. No Vue, mobile, or Go code modified.
