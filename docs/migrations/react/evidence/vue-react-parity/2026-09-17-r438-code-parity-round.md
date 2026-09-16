# 2026-09-17 Round R438 — Shared-KB drawer, settings IA phase 1, datasource card anatomy, agents sweep (TDD, 5 parallel agents)

Round type: code-level TDD round (5 parallel agents — A1 shared-KB detail drawer, A2 settings grouped-IA phase 1,
A3 datasource card anatomy, A4 agents page sweep, A5 verifier) plus an orchestrator closure of the single gate
regression A3 introduced.

## A1 — Shared knowledge-base detail drawer (PASS)

Vue KnowledgeBaseList.vue:305-315 (info-circle 查看详情 entry on non-owned shared cards) and :709-776 (right
drawer) ported: `SharedKnowledgeBaseDrawer.tsx` (+ pure `agentKbStrategyKey`/`formatSharedAt`/`permissionTone`)
with the project Sheet/Button/Badge; App.tsx mounts it (+42 lines, A1's exclusive file this round): the
`openSharedDetail` handler re-resolves the original share row by `share_id` (restoring `source_from_agent` lost
in the merge flattening), the trigger renders only when `isMine === false` with stopPropagation, and 「进入知识库」
navigates `knowledgeBaseDetailPath`. TDD: 6 jsdom behavioral tests red (0/6) → green (6/6); kb-list/regression
37/37; typecheck clean.
Recorded deltas: close button uses `common.close` (React's `general.close` means "close settings"); Esc-close is
a deliberate superset (Vue drawer has no Esc; Sheet focus trap provides it).

## A2 — Knowledge settings grouped IA, phase 1 (PASS)

The legacy `/knowledgeBase/:id/settings` surface (R437 evidence: flat 7 sections vs Vue's 5-group modal) now
mirrors Vue KnowledgeBaseEditorModal.vue navGroups (L637-669): **basic**（基础: basic/models/vectorStore, FAQ 型
追加 faq）、**processing**（索引与解析: parser/chunking/multimodal/asr/graph/advanced）、**data**（存储与数据:
storage/datasource + 数据源数 badge）、**integration**（发布集成: share）、**management**（管理与审计: activity,
owner/admin gated）— pickItems order, empty-group filtering, 14 t-icons as inline SVG, default section `basic`.
Implemented sections are housed in their groups; Vue-present-but-unported sections show the existing
`settings.notYetPorted` placeholder (no invented editors). `.wkbs-*` shell matches the Vue modal computed
styles (1000×750 frame, 208px sidebar, #f3f3f3 hover, #07c05f active, 12px radius, 24/32px padding). TDD: 10
nav-contract tests 8-fail red → 10/10 green; directory regression 39/39; 19 i18n keys pre-existing ×5 locale,
zero new copy.
Phase-2 backlog: basic/models, chunking/multimodal/asr/advanced, faq editors; A5 note — the `!isLiteMode` gate
on Vue's share item is not yet modeled.

## A3 — Datasource list-card anatomy (PASS after gate fix)

cronHumanize.ts ported to pure functions (`humanizeCron` 5 cron presets → i18n natural language with raw fallback,
`relativeTime` six buckets, `syncResultPills` non-zero metrics with +N/~N/-N prefixes); the naive rows became the
Vue card grid (auto-fill minmax(320px,1fr), 10px radius, interactive hover), 36px connector icon badge with
letter fallback, ellipsed title, status dot (active green / paused warning / error red), detail row (humanized
schedule · relative-time with full-time title · colored sync pills, 11px tabular), error box, dashed add-card for
canManage. New `dataSource.scheduleHuman.*` keys ×5 locale byte-identical to the Vue baseline (incl. en `Hourly`).
Gate regression + closure: the new keys tripped the hardcoded key-count guard (packages/i18n/test/
dataSource.test.ts:87 expected 164, actual 169) — the only failing test in A5's final run; the guard was updated
to 169. Deferred: jsdom render-level tests for the page (new assertions remain source-regex pins per that file's
existing convention; behavior is covered by card.test.ts), connector image assets (letter fallback used), Vue's
ellipsis-dropdown action grouping.

## A4 — Agents page sweep (PASS)

Against frontend/src/views/agent/AgentList.vue: list grid/card fields/sort segments/pin/menus/delete-confirm/
deep-links/empty skeletons all match; three real gaps fixed by TDD — ① space view「我共享的」cards open the editor
on click in Vue (handleSpaceAgentCardClick) with no three-dot menu; React wrongly opened a read-only drawer and
showed an admin disable menu → `opensEditorOnCardClick` + empty cardActions for sharedByMe; ② space view lacked
Vue's `spaceAgentsLoading` centered spinner (empty state flashed during load) → loading spinner suppresses the
empty state; ③ space cards rendered an org source pill Vue doesn't have on that tab → excluded for sharedByMe
rows. Agents domain 88/88. Deferred: DB-backed favorites (api-client lacks the endpoint; React uses localStorage),
`section=im/embed/integrations` settings deep-link redirects.

## Gates (final, after the guard fix)

`pnpm test:web` 1398/1398, `pnpm test:shared` 574/574, `pnpm typecheck:web` clean, `pnpm build:web` ✓
(A5 baseline at round start: 1370/1370 + 574/574; +28 web tests = this round's additions).
No Vue, mobile, or Go code modified. Per-agent reports: .omc/state/r438/report-A{1..5}.md (session artifacts).
