# 2026-09-17 Round R448 — Shared-drawer banner removal, user-menu 全部设置, live D2 rework (rejected:null), isolated-context verification

Round type: TDD round with 4 parallel agents — A1 isolated-context live verification of the R447 fixes,
A2 user-menu 全部设置 entry, A3 the R445-blocked read-only banner removal (documents domain freed this round),
A4 verifier. A4 verdict: A2/A3 PASS; A1's live evidence surfaced that R447's D2 fix was INCOMPLETE, and the
orchestrator reworked it in-round against the live captured payload.

## A1 — Isolated-context live verification (PASS as evidence; one fix invalidated)

Atomic-script capture discipline (post-R447 rule) worked: 11-second sampling showed zero external navigation
(no R447-style pollution this time).
- **D2 chunking preview: R447's fix did NOT resolve the live failure.** The captured real response
  (`POST /api/v1/chunker/preview`, HTTP 200, top-level `["data","success"]`) has `rejected: null` when nothing
  was rejected — R447's envelope unwrap WAS in effect, but the parser still demanded `Array.isArray(rejected)`.
  Orchestrator rework: `rejected` normalizes to `[]` when absent/null, with a contract test replaying the exact
  live payload (legacy tier, nested profile, single chunk). Live-request probe via the real login/preview API
  confirmed the shape before the fix.
- D7 logout: VERIFIED fixed — 退出 → POST /auth/logout → /login in 74ms, no console errors, the 4s fallback
  never fired. The R447-inferred reject/hang on-site did not reproduce.
- New product-level finding: logout revokes the account's sessions on ALL devices (the Vue tab 401'd to /login
  immediately after the React logout) while login does not kick the other end — needs a backend/product
  confirmation, recorded.
- Shell menu state (post-A2): 帮助与文档 translated, 全部设置 present (A2's change, live via HMR); React-only
  新手引导 entry and 空间信息 vs 空间设置 naming recorded as diffs.

## A2 — User menu 全部设置 entry (PASS)

Vue UserMenu.vue:96-100 contract: key `general.allSettings` (5 locales byte-exact), positioned after the skills
shortcut + divider before 帮助文档, unconditional across roles, no shortcut, navigating `/platform/settings`
WITHOUT a section query (distinct from the ⌘1-9 section shortcuts). Added to PlatformShell with tests (3 new);
the guide-reopen href-sequence contract updated. platform 185/185; i18n 65/65; each locale exactly +1 key.
Recorded: Vue's 系统管理 entry (isSystemAdmin-gated) is still unmigrated — separate-round candidate.

## A3 — Read-only banner removal (PASS, R445 blocker cleared)

The external occupation of `KnowledgeDocumentsPage.tsx` ended this round; A3 removed the React-only
「查看权限：编辑操作已隐藏。」banner (5 lines) — Vue's shared-KB document page renders no banner (read-only is
expressed purely by hiding edit entries via canEdit; the 20+ other canContribute uses are untouched so upload
hiding still works). TDD: new SSR test asserting the banner text never renders red → green; documents 207/207.
Deferred: `knowledgeBase.documents.viewerReadonly` is now a dead i18n key (outside A3's file domain).

## External breakage (attributed, not ours to resolve)

The external process's stash-pop left TS1185 conflict markers in THREE test files it owns:
`apps/web/src/integrations/route.test.ts` (2 conflicts), `apps/web/src/knowledge-settings/GraphSettings.test.ts`
(1), `apps/web/src/settings/McpToolsDirectory.test.tsx` (1). The conflicts are non-mechanical (two different
test sets per region, one truncated) — resolving them uninvited risks discarding the external work-in-progress
intent, so they were left in place and every gate failure they cause is attributed. A4 verified zero failures
attributable to A2/A3.

## Verification under external breakage

- `pnpm test:shared` 603/603 (full suite unaffected; includes the new rejected:null contract test).
- Orchestrator scoped run across every domain touched this round: knowledge-settings (13 files) + platform
  (6 files) + documents page-chrome = **144/144 green**.
- `pnpm typecheck:web` filtered for the three external conflict files reports ZERO further errors (all
  remaining diagnostics are TS1185 in those files).
- test:web full-suite and build:web remain red SOLELY due to the three external files; they recover the moment
  the external process resolves its stash.

## Per-agent reports

.omc/state/r448/report-A{1,2,3}.md + report-A4-review.md (session artifacts).
