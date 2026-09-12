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
- `validateMcpDraft` now rejects missing names, missing/invalid HTTP(S) URLs, unsupported stdio editing, and missing step-2 usage instructions before a mutation. The validator is pure and has direct regression coverage.
- The existing editor is now mounted in the Vue-aligned `wks-overlay`/`wks-modal` container with a bounded 720px drawer, sticky heading, scrollable body, and a mobile viewport rule. This is a structural/layout repair; pixel comparison is still outstanding.
- The remaining visible editor labels and placeholders in this slice use the existing Vue-derived keys for import, basic/connection/auth/advanced sections, custom headers, OAuth scopes, usage instructions, navigation, and save/cancel actions. `packages/i18n/test/mcpMessages.test.ts` verifies an identical key set across all supported locales.

## Evidence

| Layer | Result | Command / artifact |
|---|---|---|
| Focused component/static markup and validation | PASS, 5/5 | `pnpm --filter @weknora/web exec tsx --test src/settings/McpSettingsPanel.test.tsx` |
| MCP locale key set | PASS, 1/1 | `pnpm exec tsx --test packages/i18n/test/mcpMessages.test.ts` |
| Shared locale contract | PASS through Web consumer | same test asserts zh-CN and en-US action copy |
| Diff hygiene | PASS | `git diff --check` |
| Full Web typecheck/build | BLOCKED outside this slice | existing dirty `apps/web/src/auth/onboarding.ts` imports a missing `@weknora/domain/auth/onboarding`; `WorkspaceOnboardingPage.tsx` also has an implicit-any error. The MCP test fixture error found during this slice was fixed; the remaining output contains only the onboarding errors. |
| Browser / real backend / Wails / iOS / Android | NOT COLLECTED | required before R024 or child rows can be `accepted` |

The isolated services were reachable during this continuation (`Vue :5180`, `React :5181`, backend `:8080`), but browser evidence could not be collected: the browser-use runtime lacks the macOS arm64 Node 24 `classic-level` native build, and the desktop browser control surface reported that its browser request-header policy could not be loaded. No login, mutation, or screenshot claim is made from this attempt.

## Remaining R024 gaps

R024 remains `implementing`: the React form is still an inline editor rather than a Vue `SettingDrawer`, several editor/detail strings remain hardcoded, and exact visual, validation, stdio, browser, real-backend, Wails, iOS, and Android evidence is not present. This report does not promote any matrix row to `accepted`.
