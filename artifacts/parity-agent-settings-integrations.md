# Settings / Integrations parity slice

Date: 2026-09-15

## Scope selected

This slice covers the integration history entry and its canonical Settings
section. It was selected because the React app had no implementation for this
surface, while the existing `apps/web/src/settings/*` and knowledge-base files
were already dirty from other agents.

## Vue inventory

| Vue entry | Authority | React result |
| --- | --- | --- |
| `/platform/settings` | `frontend/src/views/settings/Settings.vue` | Canonical integration sections are parsed by `src/integrations/route.ts` |
| `/platform/integrations?tab=<tab>` | `frontend/src/router/index.ts` + `settingsRoute.ts` | Preserved as a legacy alias and mapped to the same tab |
| `integration-im` | `IntegrationSettingsSection.vue` / `IMChannelPanel` | Navigation shell and contract-preserving status |
| `integration-embed` | `AgentEmbedChannelPanel` | Navigation shell and optional `agentId` preservation |
| `integration-api` | `ApiIntegrationSettings.vue` | Navigation shell; no API-key mutation invented |
| `integration-cli` | `CliIntegrationLanding.vue` | Navigation shell and CLI landing copy |
| `integration-chrome` | `ChromeExtensionLanding.vue` | Navigation shell |
| `integration-claw` | `ClawSkillLanding.vue` | Navigation shell |

Vue requires authenticated access and admin-level visibility for integration
settings. The React page keeps a `canView` gate and does not claim backend
authorization; the server remains authoritative for all future mutations.

## React route inventory before this slice

`apps/web/src/main.tsx` mounted `/craft*` through `CraftRoutes` and rendered the
knowledge-base page for every other path. There was no React route for
`/platform/settings`, `/platform/integrations`, or an integration tab.

## Implemented

- `src/integrations/route.ts`: canonical tab list, legacy URL parsing,
  `agentId` preservation, and canonical URL construction.
- `src/integrations/IntegrationsPage.tsx`: project UI `Button`, `Card`, and
  `Status` wrappers; tab navigation; permission-denied state; explicit status
  that backend integration configuration is not fabricated by this slice.
- `src/main.tsx`: mounts the integration page for both canonical and legacy
  URLs while leaving Craft and the existing knowledge-base entry untouched.
- `src/integrations/route.test.ts`: route alias, filter, unrelated-route, and
  canonical-path coverage.

## Verification

| Command | Result |
| --- | --- |
| `node --import tsx --test apps/web/src/integrations/route.test.ts` | PASS: 3 tests, 0 failures |
| `pnpm --dir apps/web test -- src/integrations/route.test.ts` | The package script expands `src/**/*.test.ts`; integration tests pass, but unrelated dirty-agent tests fail because `auth/auth-state.ts` and `knowledge-bases/editor.ts` are absent and `legacy-session` expectations differ. |
| `pnpm --dir apps/web build` | BLOCKED by unrelated knowledge-bases TypeScript errors in `src/knowledge-bases/editor.test.ts`, `states.test.ts`, and `states.ts`. |

## Explicitly not covered

- Full Settings drawer and all non-integration sections (`general`, models,
  storage, sandbox, skills, MCP, system, members, profile, and tenant).
- `/platform/agents` and `frontend/src/views/agent/AgentList.vue`.
- Knowledge-base datasource settings and editor dialog
  (`DataSourceSettings.vue`, `DataSourceEditorDialog.vue`).
- `/platform/apps`, `/platform/apps/connections`, authorization, and action
  approval pages; these are app-connector history surfaces, not part of this
  integration landing slice.
- Real IM/embed/API provider configuration, API-key CRUD, capability loading,
  browser visual parity, Wails, mobile, and production-provider acceptance.
- Auth/login/onboarding and `/knowledgeBase` compatibility.

The branch is intentionally left uncommitted for independent review. No
unrelated dirty files were reverted or edited.
