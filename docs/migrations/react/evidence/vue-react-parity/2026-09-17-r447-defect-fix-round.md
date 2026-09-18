# 2026-09-17 Round R447 — Defect-fix round: chunking preview envelope, shell copy/logout, D1 redirect root-caused as external (TDD, 4 parallel agents)

Round type: defect-fix round for the R446 browser-sweep findings. 4 parallel agents (A1 knowledge-settings
defects, A2 platform-shell defects, A3 D1 root-cause investigation, A4 verifier). A4 verdict: **all PASS, zero
rework**, final gates web 1583/1583 / shared 602/602, typecheck and build clean, zero new failures.

## A1 — Chunking preview, strategy placeholder, activity card (PASS)

- D2 root cause: the backend (`internal/handler/chunker_debug.go:256`) always wraps the preview response in a
  `{success, data}` envelope, while the api-client `parseChunkingPreview` looked for `selected_tier` at the top
  level → 「Invalid chunking preview response」. Request payload and endpoint were already correct (field-by-field
  match with Vue and the Go handler). Fixed by unwrapping the envelope before parsing (minimal, type-preserving
  change in `packages/api-client/src/knowledge/settings.ts`; the old mock encoded a shape the backend never
  sends — corrected with the real contract documented).
- D3: with no strategy set (`strategy:''`) React rendered a bare empty option where Vue's wk-select shows the
  placeholder 「选择分块策略（不填则按长度切分）」 — the placeholder key already exists in 5 locales; rendered it.
- D5: the summary overview card keyed off a KB-payload `activity` field the settings API never returns, so the
  empty card always showed alongside the independently-fetched (populated) table; the activity section's empty
  overview card no longer renders (the pure `summarizeKnowledgeSettings` function is untouched).
r447 tests 4/4 red→green; knowledge-settings 90/90; api-client settings 8/8.
Deferred: a verification round should re-click the preview in the real environment.

## A2 — Shell copy, logout, naming (PASS, one non-blocking CONCERN)

- D6: `general.helpAndDocs` did not exist in packages/i18n at all — added ×5 locale with copy taken verbatim
  from the Vue locale files (帮助与文档 / Help & Documentation / ヘルプとドキュメント / 도움말 및 문서 / Справка и
  документация).
- D7: jsdom reproduction proved the click binding itself works; the real gap is the logout chain — the shell
  called `void onLogout()` bare, so a rejected or hung chain (e.g. a throwing credential clear) stranded the
  user in the shell. `runShellLogout` now guarantees /login (no interference on success; hard-navigate on
  reject or 4s non-settle). CONCERN recorded: the on-site failure mode (reject vs hang) is inferred, not
  observed — R448 browser re-check queued.
- Naming: the org settings tab is 「加入申请」 in Vue (OrganizationSettingsModal.vue:505-516,1006-1011) with the
  inner list titled 「待审核申请」+ pending badge — React used the inner title in both places; fixed, description
  row added, inner header + badge added.
platform 182/182 (+6), organizations 37/37, i18n guards green.

## A3 — D1 root cause: external browser automation, NOT a React defect (PASS)

Exhaustive exclusion evidence: the settings page module contains zero setTimeout/setInterval/navigations
(grep counts), every `location.assign/replace` in apps/web is same-origin relative (the only absolute URLs are
the OIDC authorization links), the vite proxy only covers /api and /files, no service worker / BroadcastChannel
/ postMessage navigation exists, no 5180 anywhere in env or config, and no recent commit added navigation code.
Root cause: the evidence and the external automation shared ONE real Chrome tab — the external driver navigates
to `http://localhost:5180/platform/knowledge-bases` on an ~8s action cycle and Vue's `useListUrlState.ts:84`
appends `?scope=all` (React never constructs `scope=all`). D1 CLOSED as external behavior; no code can fix it.
Mitigations: future browser agents must use isolated contexts and freeze parallel driving during evidence
capture; a `navigation-origin-guard.test.ts` sentinel now statically scans all of src for hardcoded absolute-URL
navigations (planted cross-origin samples — including the exact `:5180/...?scope=all` literal — are caught;
current relative code passes).
A4 note: two more absolute-URL sites exist (LoginPage OIDC authorizationUrl, ConfigurationEditor) — both
legitimate, neither a 5180 vector.

## Gates (final)

`pnpm test:web` 1583/1583, `pnpm test:shared` 602/602, `pnpm typecheck:web` clean, `pnpm build:web` ✓ (A4 final
run; baseline all green at HEAD 16ecf817 — the external process advanced 3 commits mid-round, attributed
there). No Vue, mobile, or Go code modified by this round. Per-agent reports:
.omc/state/r447/report-A{1,2,3}.md + report-A4-review.md (session artifacts).
