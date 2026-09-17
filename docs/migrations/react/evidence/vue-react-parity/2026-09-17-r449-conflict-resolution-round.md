# 2026-09-17 Round R449 — External stash conflicts resolved, platform menu finish, dead-key cleanup, embed mermaid chrome

Round type: 3 parallel agents + dispatched verifier (A4). Verdict: **all PASS; the full gate suite recovered to
green for the first time since the external stash-pop** — test:web 1602/1602, packages/i18n 65/65,
typecheck:web 0 errors, build:web ✓.

## A1 — External stash conflicts resolved (PASS)

The three test files the external process stash-popped with TS1185 markers are resolved with per-conflict git
forensics (`git show HEAD:` vs `git show stash@{0}:` both sides):
- `integrations/route.test.ts`: the stashed "new" test was a VERBATIM duplicate of HEAD L19 (appearing twice on
  the stash side) — deduplicated; the resolved file matches HEAD exactly (zero diff).
- `knowledge-settings/GraphSettings.test.ts`: union — HEAD's 2 tests + stash's 3 tests all preserved (shared
  closing brace restored); every assertion verified against source.
- `settings/McpToolsDirectory.test.tsx`: union with implementation-based trimming — full-history forensics
  (`git log --all`) proved `policyLoading`/`policyLoadFailed`/「正在读取工具策略」 were NEVER part of any
  committed McpToolsDirectory.tsx (the landed implementation b6328fb6 uses `busy || busyTools.has(...) ||
  policyError`); the busyTools per-tool blocking + global blocking intent and the Chinese copy assertions were
  preserved with regexes adapted to the landed implementation; 3 assertions targeting never-existing behavior
  were dropped.
Three files 28/28; typecheck TS1185 blockage cleared; `pnpm test:web` recovered (1595 at A1's run, before the
other agents' additions).

## A2 — Platform menu: 系统管理 entry + 空间设置 rename (PASS)

- Vue UserMenu.vue:110-113 contract: the 系统管理 entry (`settings.navGroups.systemAdministration`, 5 locales
  already present) sits after 全部设置 before 帮助与文档, gated on `authStore.isSystemAdmin`
  (`user.is_system_admin === true`, platform-level, independent of space role), navigating
  `/platform/settings?section=system-global` (the React SettingsPage already serves that section). React
  PlatformShell's applyAuthMe now derives isSystemAdmin with the same semantics; the divider from Vue :114
  added.
- The menu entry renamed 空间信息 → 空间设置 (`settings.workspaceSettings`, byte-exact ×5 locale, SAMPLE_KEYS
  guard synced) — both navigate identically (`?section=tenant`), pure naming divergence.
platform-shell-user-menu 14/14; platform directory 190 tests (two cross-run order flakes in unrelated
invitations/contextual-guide cases, isolated runs all green — pre-existing concurrency jitter, documented).
Deferred: SettingsPage.tsx:410 tenant section title still reads 空间信息 (settings domain decision); React
shell has no isLiteMode gating (pre-existing).

## A3 — Dead-key cleanup + embed mermaid chrome (PASS)

- `knowledgeBase.documents.viewerReadonly` deleted from knowledgeSurfacesSupplemental (en-US/zh-CN removed;
  ja/ko/ru inherit via spread) after a repo-wide zero-consumer search; documentsPageKeys guard synced.
  i18n 65/65.
- Embed mermaid chrome: the assumption "the Vue embed face is minimal" was REVERSED — EmbedBotMessage.vue
  reuses the main-chat chrome (badge mermaid.diagram + expand button → openMermaidFullscreen). The React embed
  face now wraps hydrated diagrams with the badge/expand header (idempotent, button disabled without SVG) and
  a fullscreen overlay dialog (close/mask/Esc). Copy ×5 locale byte-exact from Vue. 5 new tests red → 16/16;
  embed suites 66/66; typecheck clean (4 pre-existing TS2345 in the domain also fixed); build ✓ (30.79s).
Deferred: fullscreen viewer zoom/reset/download toolbar (main chat face lacks it too — shared-engine task);
viewer focus trap (Vue original also lacks it).

## Gates (final)

`pnpm test:web` 1602/1602, packages/i18n 65/65, `pnpm typecheck:web` 0 errors, `pnpm build:web` ✓ — full
recovery from the R448 external-conflict red. No Vue, mobile, or Go code modified by this round. Per-agent
reports: .omc/state/r449/report-A{1,2,3}.md + report-A4-review.md (session artifacts).
