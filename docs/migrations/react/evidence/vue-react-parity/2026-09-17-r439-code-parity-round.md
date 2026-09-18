# 2026-09-17 Round R439 — Settings editors phase 2, administration/appconnector/platform-settings sweeps (TDD, 5 parallel agents)

Round type: code-level TDD round (5 parallel agents — A1 settings editors phase 2, A2 administration sweep,
A3 integrations/appconnector sweep, A4 platform-settings sweep, A5 verifier) plus an orchestrator closure that
wired the verifier-flagged A3 dead code in-round.

## A1 — Knowledge settings editors phase 2 (PASS)

Three editor sections migrated against Vue KnowledgeBaseEditorModal.vue, all landing in the R437
`PUT /initialization/config/:kbId` pipeline:
- **models**: LLM (KnowledgeQA) + embedding selectors with the live `/api/v1/models` catalogue wired as a fourth
  independently-degraded path of `loadKnowledgeSettingsOptions`; non-active models excluded per Vue
  `modelDefaults`; saves `llmModelId`/`embeddingModelId`.
- **chunking**: strategy select (auto/heading/heuristic/legacy + explanation panel + legacy disabled),
  size/overlap/parent/child sliders with Vue ranges/steps, `overlapTooHigh` warning, separators multi-select,
  parent-child toggle coupling, collapsible token-limit/languages panel; saves `documentSplitting`.
- **advanced**: question-generation toggle with count (1–10 clamp) + instructions (4000) subform; saves the
  existing `questionGeneration` payload field.
`buildKnowledgeSettingsConfigPayload` gained an optional overrides 4th argument covering only explicitly edited
fields — calls without overrides produce the exact R437 round-trip payload (existing payload tests unchanged).
TDD red (missing module/wrong model selection/dead model not excluded) → green: 7 pure + 4 DOM tests; directory
regression 50/50; 16 reused `knowledgeEditor.*` keys ×5 locale, zero new copy.
Phase-3 backlog: multimodal/asr/faq sections, wiki synthetic model row (indexing_strategy), autoTag +
tableMetadataInstructions, KBChunkingDebug panel.

## A2 — Administration sweep (PASS)

Against Vue frontend/src/views/settings/TenantMembers.vue: the four system-admin panels are gated identically
(`systemAdminPanelKeys`). Two real gaps fixed by TDD (red 3/3 → green 3/3; summary/i18n regression 16/16):
1. Member search — Vue's 320ms debounce + server-side `q` filter + page reset + search-aware empty state were
   missing entirely; implemented via the existing `members.list({q,page,pageSize})` API (contract unchanged),
   with `mobileAdministration.noMembersForQuery` + `searchPlaceholder` mirrored per locale from Vue.
2. Invite default role `viewer` → Vue contract `contributor`.
Deferred: management pagination UI (full port lives in the externally-occupied settings/TenantMembersPanel.tsx),
popconfirm vs window.confirm shape, two-step invite confirmation.

## A3 — Integrations/appconnector sweep (PASS after wiring closure)

Vue authority clarified: the Apps four-view sources exist in the CURRENT checkout (frontend/src/views/apps/),
not only at historical commit 9b0c11c4 as the artifacts note claimed. Sweep found three real gaps, fixed by TDD
(red 1 → green 10/10; appconnector regression 18/18): ① ConnectionsPage revoke CAS conflict branch (409 /
VERSION_CONFLICT → "连接授权版本已变化，请刷新后重试" warning + list reload; React previously fell into a plain
error); ② connection state badges (bare English `{row.state}` → Vue zh vocabulary + theme tone); ③ AppsPage
published badge, risk vocabulary, installation state mapping.
Orchestrator closure of the verifier CONCERNS: `catalogRiskTone` was implemented + tested but never consumed —
the catalog risk label rendered as plain text. Wired into the AppsPage risk badge (`<Status tone={...}>`), so
the read/write/send/delete tone mapping is now visible. Locale note: new copy uses existing Vue `apps.*` keys
(the appconnector module keeps its established hardcoded-zh pattern; no new key layer introduced).
Deferred: schema-digest truncation shape, table-vs-list baseline, React's extra authorize busy-guard kept
(safety-side deviation, documented).

## A4 — Platform settings sweep (PASS)

Vue Settings.vue's seven navGroups / 23 sections map 1:1 to React's NAV_GROUP_DEFS + registry including
role/capability gates and platform-admin disabled semantics; save semantics (immediate, no explicit button),
validation and invalid-value rollback match; language/theme/font apply instantly via the same mechanisms.
One real gap fixed by TDD (red assertion on /成功/ → green 10/10): Vue shows a success toast after each accepted
preference change (`language.languageSaved` / `common.success`) — React panels had no feedback; added the
existing `Status tone="success"` pattern with shared i18n keys (5 locale present, zero new copy).
Deferred (outside file domain, recorded): G2 — Vue persists preferences under per-user
`WeKnora_${userId}_*` namespacing while React uses flat keys (`weknora-theme` etc.), a cross-user crosstalk risk;
fix belongs in packages/domain/src/settings/local-preferences.ts + apps/web/src/theme.ts.

## Gates (final, after the A3 wiring)

`pnpm test:web` 1423/1423, `pnpm test:shared` 574/574, `pnpm typecheck:web` clean, `pnpm build:web` ✓
(A5 baseline at round start: 1398/1398 + 574/574; +25 web tests = this round's additions).
No Vue, mobile, or Go code modified. Per-agent reports: .omc/state/r439/report-A{1..5}.md (session artifacts).
