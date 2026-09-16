# 2026-09-16 Round R435 — Code-parity round: settings surface, doc retry contract, org gating, chat approval args (TDD, 5 parallel agents)

Round type: code-level TDD round (no browser pairing; per-agent scoped runs + full gates).
Execution: 5 parallel agents dispatched in one message — A1 knowledge-settings domain, A2 document-detail domain,
A3 metadata/org domain, A4 chat domain, A5 verifier/reviewer. All four implementation agents followed
red→green TDD; A5 established the baseline gates, reviewed every diff for domain/m methodology compliance, and ran
the final gates. Numbering note: R434 was claimed mid-flight by the external concurrent orchestrator (graph
fit-to-view/guide-placement WIP visible in the worktree); this round took R435 to keep attribution clean.

## Baseline and attribution

- Baseline at round start (A5): `pnpm test:shared` 568→570 region, `pnpm test:web` 1333/1333, typecheck clean
  (HEAD moved twice during the round as the external process landed `docs(sdd)` commits).
- External concurrent R434 WIP files (knowledge graph page/tests, `graph.ts`, `KnowledgeDocumentsPage.tsx`,
  `guides.css`, `_page-probe.test.tsx`, r432/r434 screenshot dirs) were untouched by this round; the graph-page
  test failure observed mid-round (1339/1340) was caused by THIS round's A1 change against the graph page's mock
  client and was fixed here (see A1 follow-up), not attributed to the external WIP.

## A1 — Legacy knowledge-settings surface: live datasource/share/activity + option catalogues

- Gap (ledger R347/R348 residue): the aligned modal in `App.tsx` already mounts live inline surfaces, but the
  legacy `apps/web/src/knowledge-settings/KnowledgeSettingsPage.tsx` (routed at `/knowledgeBase/:id/settings` and
  embedded by the KB settings dialogs) rendered datasource/share/activity as plain text rows and showed
  parser/vector/storage as static read-only text with no live option loading — diverging from Vue
  `KnowledgeBaseEditorModal.vue:423-434` inline contract.
- Fix: exported `loadKnowledgeSettingsOptions(client)` loading parserEngines/storageBackends/vectorStores through
  the authenticated settings API with per-endpoint independent degradation (`Promise.allSettled`); datasource
  section now mounts `DataSourcesPage` (live data + canManage gating), share mounts `KnowledgeBaseShareDialog`
  inline, activity mounts `KnowledgeBaseActivityPanel`; parser select lists live available engines (loading
  disabled, shared i18n engine names); vectorStore/storage selects render disabled with immutable/migrate hints
  (Vue edit-state binding is read-only).
- TDD: new `KnowledgeSettingsPage.inline.test.tsx` — red 0/4 → green; scoped suite 21/21.
- Orchestrator follow-up (verifier-found cross-domain regression): mounting the page called the three catalogue
  methods synchronously, so client mocks lacking those methods (e.g. the graph page test's mock) threw a
  synchronous TypeError that `allSettled` could not absorb → `KnowledgeGraphPage.test.tsx` failed 1339/1340.
  Fixed by wrapping each call in a lazy thunk (`Promise.resolve().then(() => client…())`) so missing methods land
  in the existing rejection-degradation path, plus a `loadKnowledgeSettingsOptionsToleratesMissingMethods`
  regression test mirroring the sparse-mock shape.
- Deferred: the legacy surface still has no save pipeline (full save semantics remain in the App.tsx aligned
  modal); host dialogs needing the complete save flow are owned by the R434 graph/documents work.

## A2 — Document detail retry-index feedback contract

- Confirmed first: the R369-era "merged/preview/chunk tabs + retry controls" gap was closed in R370 (tab
  structure, `index_status==='failed' && canEdit` gating, `{ expected_revision }` payload all match Vue
  `frontend/src/components/doc-content.vue`).
- Residual contract gaps fixed (TDD red 3 → green 20/20; documents scoped regression 199/199):
  button copy `common.retry`「重试」→ Vue `retryIndex`「重试索引」; success feedback「索引已同步」; still-failed
  error「索引同步失败」with controls retained; retry moved off the shared `savingId` onto dedicated
  `retryingId`/`retryNotice` state so retry loading no longer disables the enable/disable toggles (Vue isolates
  the loading state); title/aria-label localized; viewer gating asserted.
- Known accepted delta (recorded, not a regression): Vue renders retry as icon+tooltip with global toast; React
  keeps the R370-established text button + inline Status with equivalent semantics.

## A3 — Metadata editing closed; org upgrade-request gating

- Primary task (structured metadata-row editing) verified ALREADY CLOSED in R370: typed four-kind rows, key
  required/duplicate/number validation, 20-row cap, draft preservation all match Vue `doc-content.vue`. No code
  changed (file also belongs to A2's domain — constraint honored, verification evidence recorded only).
- Fallback task executed (TDD red 21/22 → green 22/22): Vue `OrganizationSettingsModal.vue` gates the
  role-upgrade request behind `canRequestUpgrade` (editing state + non-admin space role + tenant admin+) and
  converges role options by current role (viewer→editor/admin; editor→admin). React rendered the form
  unconditionally with all three roles. Fixed via exported `canRequestUpgradeForOrg`/`upgradeRoleOptionsForRole`
  in `OrganizationsPage.tsx`; no new i18n keys needed (all five locales already present).
- Deferred: `hasPendingUpgrade` submit-disabled state needs an api-client field + org detail endpoint call —
  recorded for a future round.

## A4 — Chat tool-approval args editing contract

- Ten-state CHAT-SURFACE audit: nine states verified aligned (stream error + retry entry, run-disabled semantics,
  history restore, empty/new chat, no-permission, sending, complete, failure retry, thinking copy from N020).
- One real gap fixed (TDD red 2/13 → green 22/22 across three files; chat scoped regression apps 107/107 +
  views 83/83), against Vue `ToolApprovalCard.vue`: per-keystroke JSON validation with `approvalArgsStatus`
  (`isJsonValid`/`argsDirty`) disabling approve on invalid args + inline alert; `argsModified`「已修改」status
  hint; reject body now carries Vue's localized `reason` (`userRejected`) through `ChatRoutePage.tsx` →
  api-client; button order swapped to Vue's reject · approve; new 5-locale keys `approvalArgsModified` /
  `approvalRejectedReason` mirroring Vue i18n verbatim.
- Deferred: Vue's approval-card countdown not implemented (single-item future round); history-restore loading
  shape (Vue intentionally shows none) and stream-error carrier shape (React inline failed row + explicit retry,
  informationally equivalent) recorded as accepted deltas.

## Gates (final, after A1 follow-up)

- `pnpm test:shared` 570/570; `pnpm test:web` 1340/1340 (includes the external graph-page suite back to green);
  `pnpm typecheck:web` clean; `pnpm build:web` ✓ 4.07s.
- No Vue (`frontend/`), mobile, or Go code modified. No browser screenshots this round (code-contract round);
  the four items above carry scoped suite + typecheck + full-gate evidence, with live paired-browser capture
  left to the next browser round for the touched surfaces (KB settings dialog, doc detail retry, org settings,
  chat approval).

## Evidence artifacts

- Per-agent reports (session artifacts, not committed): `.omc/state/r435/report-A{1..4}.md`, `report-A5-review.md`.
- Committed with this round: ledger entry + this evidence document + matrix addendum.
