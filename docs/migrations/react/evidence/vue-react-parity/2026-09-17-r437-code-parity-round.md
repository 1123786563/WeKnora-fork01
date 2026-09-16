# 2026-09-17 Round R437 — Code-parity TDD round: doc pagination/preview gates, datasource lifecycle, FAQ batch bar + browser sweep

Round type: code-level TDD round (5 parallel agents — A1 document-detail residuals, A2 datasource page sweep,
A3 FAQ + KB-list sweep, A4 browser-evidence-only, A5 verifier) plus an orchestrator closure that fixed the
verifier-flagged A2 CONCERNS in-round.

## A1 — Document detail: pagination transition + parse-status gating (PASS)

- Pagination transition now mirrors Vue doc-content.vue: during a page turn the section header and pager stay
  mounted, the content area swaps to the small `chunk-page-loading` row (old page hidden via the v-else branch),
  the pager is disabled while transitioning (double-click guard), a failed fetch keeps the previously loaded page
  with a retry, document switches reset the state, and segment numbering uses Vue's `(page-1)*25+i+1` formula.
  Ledger correction recorded: the R436 note said Vue "keeps the old page + small loading" — the actual Vue
  template hides the old page during the transition; React now matches the source, not the note.
- parse-status gating removed where Vue has none: Vue `canPreview` (doc-content.vue:743-749) checks only
  type/file_type/audio; React's `previewStatus` had gated the preview tab content and the embedded audio player
  on `parse_status`. `ready` is now `type === 'file' && inline kind`; the Vue-absent
  `availability`/`downloadOnly` model fields and their dead 5-locale strings were removed.
- Verifier: PASS (state machine, CHUNK_PAGE_SIZE=25, controllable-Promise behavioral tests).

## A2 — Datasource page: lifecycle contract (PASS after orchestrator closure)

Three real gaps fixed by TDD against Vue DataSourceSettings.vue + DataSourceEditorDialog.vue:
1. Create branch now triggers the first sync immediately (success → `createAndSyncSuccess`, failed trigger →
   warning `createButSyncFailed`); edit branch keeps the Vue warning tone `updateSuccessSyncHint` with no auto
   sync (React previously success-toned both and never synced).
2. Per-field required-credential validation with the Vue `${label} ${isRequired}` warning now blocks the submit
   before the connection test (was a single generic saveFailed).
3. Create-type step title localized (`dataSource.step.selectType`), hardcoded Chinese removed.

Orchestrator closure of the verifier CONCERNS: Vue's `credentialsRequired` exemption — an edit of an
already-configured connector without typed replacement credentials skips the required walk entirely — was
missing. Added `credentialsRequiredForValidation` (form.ts, mirroring
`!(isEdit && credentialsConfigured && !replaceMode)`) wired into `save()` with the backend
`credentials.credentials.configured` flag (confirmed in internal/handler/datasource_credentials.go:85-91), plus
a truth-table unit test. The backend flag is absent on some list payloads; missing → `=== true` is false →
validation runs, matching Vue's optional-chain behavior exactly.
Deferred: render-level behavioralization of DataSourcesPage.test.tsx (its older assertions are source-regex
pins — flagged by the verifier; a JSDOM harness with client mocks is a standalone work item); list-card anatomy
(icons/badges/cron humanize/sync-result pills), Vue credential-step Replace/Remove + prereq guides, and React's
Vue-absent per-row "测试连接" button are recorded for adjudication/future rounds.

## A3 — FAQ batch bar + KB list entry audit (PASS)

- FAQ batch bar 「启用」/「禁用」buttons now render conditionally exactly per FAQBatchBar.vue:53-63 (enable only
  when the selection contains disabled entries and vice versa), with matching `is_enabled !== false` counting.
- KB list audit: grid, sorting, space filter, create entry, four empty states, and the card more-menu
  (pin/duplicate/settings/delete with identical gating) all match. The R436 open question is closed from the
  code side: Vue's card menu 「设置」→ `openKBSettings` has a React counterpart (`openKbSettings` → in-place
  edit Dialog), no gap.
- Deferred (recorded, outside this round's file domains): Vue's shared-KB detail drawer for non-owned cards
  (KnowledgeBaseList.vue:710-776; data + 5-locale keys already exist, mount point is App.tsx); FAQ batch-delete
  gating is one tier looser than Vue (canContribute vs canManage — backend RBAC backstops, permission
  derivation lives in the externally-occupied knowledge/permissions.ts).

## A4 — Browser evidence (PASS, zero code changes)

11 paired screenshots in `screenshots/r437-20260917/` (report: .omc/state/r437/report-A4.md). Highlights:
- Vue KB editor modal entry located: KB-list card top-right persistent `.more-wrap` icon → t-popup menu
  (置顶/创建副本/设置/删除) → 「设置」 → `uiStore.openKBSettings(id)` → Teleported `.settings-overlay`
  (1000×750, 5 groups / 13 Chinese nav items, 取消/保存并关闭). The React legacy `/knowledgeBase/<id>/settings`
  surface remains a flat 7-section English page with per-section save — the structural difference (basic-info /
  model / chunking / image / audio / advanced sections; per-file-type parser granularity) is documented with
  paired screenshots as the largest known settings-surface gap for future rounds.
- Org upgrade gating: real route is `/platform/organizations` (`/organizations` 404s); gating renders per
  contract (edit mode + non-admin myRole + tenantAdmin, submit disabled by has_pending_upgrade); the current
  parity account is an org admin so both sides render no upgrade form (DOM-verified `#upgrade-role` absent) —
  matches the Vue contract, recorded honestly.
- Approval countdown not witnessed: the sandbox blocks the test model endpoint (SSRF guard), no approval event
  in ~2 minutes; left for a round with a live approval flow.
- Side observations: transient Vite HMR 500 on DataSourcesPage.tsx during the round (transient, resolved);
  Vue's card more button has no a11y name (upstream note, not a React item).

## Gates

- Final (after the A2 closure): `pnpm test:web` 1370/1370, `pnpm test:shared` 574/574, `pnpm typecheck:web`
  clean, `pnpm build:web` ✓. A5 baseline at round start was 1359/1359 + 574/574, all green; the +11 web tests
  are exactly this round's additions.
- No Vue, mobile, or Go code modified. Per-agent reports: .omc/state/r437/report-A{1..5}.md (session artifacts).
