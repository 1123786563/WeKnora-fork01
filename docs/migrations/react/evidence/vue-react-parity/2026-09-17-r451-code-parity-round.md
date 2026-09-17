# 2026-09-17 Round R451 — Datasource credential removal landed, resource-step editors, edition probe (3 parallel agents)

Round type: TDD round with 3 parallel agents (A1 datasource closeout, A2 edition probe, A3 verifier).
Verdict: **A2 PASS; A1 initially FAIL on one endpoint (caught by the dispatched verifier, reworked by the
orchestrator in-round — the verifier catching a production-404 before merge is the process working as
designed)**. Final gates: test:web 1626/1626 (+13), test:shared 606/606, typecheck clean, build ✓.

## A1 — Datasource closeout (FAIL→PASS after orchestrator rework)

- `removeCredentials(id)` landed on the typed client (DELETE, no more feature-detect at the call site) and the
  Remove three-state UI activates it with the Vue removeFailed fallback.
- Resource-step editors ported: rss custom headers as key-value rows (`serializeAuthHeaders`, treated as
  credential draft — included in the connection test and putCredentials gates, cleared on cancel-replace);
  Drive `folder_token` row (load button, share hint, inline required error, persisted as `resource_ids=[token]`
  with root loading and reopen prefill). GitLab projects multi-select recorded as deferred (structured settings
  channel needed — the key=value settingsText protocol cannot carry it; i18n keys already in place).
- **Verifier-caught defect (reworked)**: the removeCredentials path was `/credentials` but the backend route is
  `/:id/credentials/:field` with only `credentials` existing (internal/router/routes_infra.go; Vue dels the
  doubled `/credentials/credentials` at frontend/src/api/datasource/index.ts:174) — production would 404. Fixed
  to the doubled path with the contract test replaying it; the mock that encoded the wrong shape was corrected.
data-sources 43/43; api-client datasource tests updated.

## A2 — Edition probe (PASS)

React already had the data source: `client.settings.system.info()` (GET /api/v1/system/info) with
`SystemInfo.edition` — zero api-client changes. PlatformShell now probes once on mount (defensive, skipped
without the settings namespace) and merges with Vue's semantics: localStorage init first, server
`edition === 'lite'` promotes AND persists the key (one-way upgrade), non-lite never downgrades, probe failure
silent. 4 behavioral tests red→green; lite-mode file 9/9; platform 199/199.
Environment note recorded: a failing jsdom PlatformShell test can hang node:test until SIGKILL (40-70s) —
green runs exit cleanly; future agents should suspect the environment first when a failure "hangs".

## Gates (final)

`pnpm test:web` 1626/1626 (+13), `pnpm test:shared` 606/606 (+2), `pnpm typecheck:web` clean, `pnpm build:web` ✓.
No Vue, mobile, or Go code modified. Per-agent reports: .omc/state/r451/report-A{1,2}.md +
report-A3-review.md (session artifacts).
