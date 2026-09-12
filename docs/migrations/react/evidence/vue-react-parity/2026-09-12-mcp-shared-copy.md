# R024 MCP settings — shared-copy regression slice

Date: 2026-09-12  
Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`  
Branch: `codex/react-multiclient`  
HEAD before this slice: `0701346675941d65b347c32d14ea4f4cd9f24fb9`

## Scope

This slice covers the React MCP settings list/card copy and the enabled-state label. The Vue authority is `frontend/src/views/settings/McpSettings.vue` and its existing keys in `frontend/src/i18n/locales/*` (already ported to `packages/i18n/src/settings.ts`).

## Regression and repair

- The React card rendered `enabled: true` as the disabled label. The condition was corrected so true renders `mcpSettings.enabled` and false renders `mcpSettings.disabled`.
- The list heading, description, empty state, add/edit/delete labels, built-in label, usage fallback, state labels, MCP detail headings and connection actions now read the shared i18n contract.
- Missing OAuth/action keys were added once in `packages/i18n/src/mcp.ts` for all current client locales and merged by `packages/i18n/src/index.ts`; no page-local translation table was introduced.

## Evidence

| Layer | Result | Command / artifact |
|---|---|---|
| Focused component/static markup | PASS, 4/4 | `pnpm --filter @weknora/web exec tsx --test src/settings/McpSettingsPanel.test.tsx` |
| Shared locale contract | PASS through Web consumer | same test asserts zh-CN and en-US action copy |
| Diff hygiene | PASS | `git diff --check` |
| Full Web typecheck/build | BLOCKED outside this slice | existing dirty `apps/web/src/auth/onboarding.ts` imports a missing `@weknora/domain/auth/onboarding`; `WorkspaceOnboardingPage.tsx` also has an implicit-any error |
| Browser / real backend / Wails / iOS / Android | NOT COLLECTED | required before R024 or child rows can be `accepted` |

## Remaining R024 gaps

R024 remains `implementing`: the React form is still an inline editor rather than a Vue `SettingDrawer`, several editor/detail strings remain hardcoded, and exact visual, validation, stdio, browser, real-backend, Wails, iOS, and Android evidence is not present. This report does not promote any matrix row to `accepted`.
