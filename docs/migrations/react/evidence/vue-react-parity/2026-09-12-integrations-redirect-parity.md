# R013 legacy integrations redirect: RED evidence

Status: implementing. Snapshot HEAD: `0914921c7f02d7a92a2626fbac60573480606278`. This narrow check does not establish page acceptance.

## Authoritative behavior and current gap

Vue `frontend/src/router/index.ts` legacy `integrations` route redirects to `/platform/settings`, consumes a string `tab`, defaults absent/non-string `section` to `integrations`, calls `normalizeSettingsSection`, removes `tab`, and preserves remaining query values. `frontend/src/config/settingsRoute.ts` normalizes `integrations` to `integration-im` by default; invalid legacy tabs also resolve to `im`. Bare valid integration names become `integration-<name>`. Other sections remain unchanged. The Vue redirect route has authentication/init metadata and no whole-route integrations capability requirement.

React `apps/web/src/routes.tsx` currently returns no redirect for that path and allows authenticated tenant sessions to render the old standalone integration page. Its whole-route integrations capability gate differs from Vue.

A route-only fix would regress functionality: `apps/web/src/settings/SettingsPage.tsx` resolves sections through `settingsSectionMeta`, whose shared registry lacks all `integration-*` sections. The destination falls back to `general`; it does not render `IntegrationsRoutePage`. Product modification is held for coordinated destination wiring.

The existing `packages/views/src/integrations/registry.ts` function `integrationKeyFromQuery` is not a drop-in Vue normalizer: absent and invalid values fall back to `embed`, and it trims strings. Vue defaults legacy redirects to `im` and preserves unknown section values. Reuse the existing integration registry for valid names, with a single shared settings-query normalization contract reflecting Vue semantics. Avoid separate normalizers for desktop, web, and mobile.

## Reproduction

Run from the target worktree:

```sh
node --import tsx --test apps/web/src/routes.test.ts
pnpm typecheck:web
```

Observed on 2026-09-12: route tests 8 total, 6 pass, 2 fail. New redirect assertion received `undefined` instead of `/platform/settings?section=integration-im`; guard assertion received `allow` instead of settings redirect. Existing login/no-tenant assertions in the new guard test passed before the failing authenticated assertion. Web typecheck passed. These are unit/static checks; they are not browser or backend acceptance.

Added cases cover default, old tab, integrations alias, bare section precedence, canonical section, invalid tab, ordinary/unknown section preservation, repeated unrelated query values, agentId retention, tab removal, login, missing workspace, and disabled legacy capability. The suite currently remains intentionally RED while destination wiring is unresolved.

## Coordinated completion steps

1. Add canonical integration sections and Vue-equivalent query normalization to existing shared capabilities. Verify roles and per-tab deployment capabilities against Vue Settings and real handlers.
2. Wire the existing integration panels into the settings destination and preserve the current settings container/navigation. Coordinate ownership of `SettingsPage.tsx`, its `surface.ts` metadata, shared registry, and integration wrapper. Completion criterion: canonical URLs display the requested real panel and retain role/capability enforcement.
3. Implement legacy redirect and guarded navigation using the shared normalization. Completion criterion: all route tests pass and a real browser follows the URL to the correct panel after authentication/tenant checks.
4. Collect same-account, same-tenant Vue/React browser evidence for the cases below, then run independent review. Native and full visual checks remain separate requirements.

## Browser cases for the main agent

| Input | Vue destination / expected check |
| --- | --- |
| `/platform/integrations` | `/platform/settings?section=integration-im`; IM content inside settings |
| `/platform/integrations?tab=embed&agentId=<test-agent>` | `section=integration-embed`, agentId retained, tab removed; embed panel |
| `/platform/integrations?section=integrations&tab=api` | `section=integration-api`; owner allowed, lower role follows Vue denial/fallback behavior |
| `/platform/integrations?section=claw&tab=embed` | `section=integration-claw`; section takes precedence |
| `/platform/integrations?tab=unknown` | `section=integration-im` |
| `/platform/integrations?section=general&tab=api` | general section, tab removed |
| Legacy URL while logged out | login gate followed by requested destination after successful authentication |
| Legacy URL with authenticated no-tenant session | workspace onboarding gate |
| Legacy URL with integrations capability disabled | settings destination; validate per-section Vue capability behavior |

No real browser interaction, screenshots, backend E2E, Wails, iOS, or Android evidence was collected by this narrow task. No local commit was created. No production implementation file or Vue baseline was modified.
