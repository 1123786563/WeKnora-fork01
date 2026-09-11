# T15 React configuration live evidence — 2026-09-12

## Scope

This run covers the React configuration route against an isolated Lite
backend. It verifies authenticated rendering of the Agent/Model/MCP/Skill
surfaces and one server-confirmed Agent visibility mutation. It does not
claim a real provider debug call, MCP test/OAuth flow, Skill installation or
file content, because the isolated backend had no configured model/MCP and
its Lite schema does not include the Skill catalog tables.

## Environment

- React Web dev server: `http://127.0.0.1:5175/`
- Backend: isolated Lite binary at `http://127.0.0.1:8080`
- Database/files: `/tmp/weknora-react-t15-live-20260912/`
- Browser: Chrome tab `948799916`
- Source: `codex/react-multiclient` at `fbdc25b`
- Account: temporary local test account; no production credentials or data

## Observed evidence

1. The React sign-in and registration flow completed against the isolated
   backend. The browser then loaded `/platform/configuration` with the
   authenticated tenant scope.
2. The page rendered four explicit configuration areas. The Agent list
   contained the server-provided built-in agents. Model and MCP lists were
   empty, so `Run debug` was disabled and no provider call was fabricated.
3. Clicking `Hide selected agent` for the server-provided built-in Agent
   displayed `Agent hidden for this workspace.`. The backend log records a
   `POST /api/v1/shared-agents/disabled` request with the selected server ID
   and `disabled=true`; the UI did not report success before the request
   resolved.
4. The Skill catalog request returned an error in the browser. The backend
   log identifies the cause as `no such table: tenant_skill_catalog` for
   `GET /api/v1/skills/catalog`. The page showed the error and kept the
   catalog empty; it did not turn the failed response into a successful
   installation state.

## Fresh command checks

| Command | Result |
|---|---|
| `curl -fsS http://127.0.0.1:8080/health` | HTTP 200 (`{"status":"ok"}`) |
| `pnpm exec tsx --test packages/api-client/src/configuration.test.ts apps/web/src/configuration/surface.test.ts apps/web/src/configuration/editor.test.ts apps/web/src/configuration/management.test.ts` | 34/34 passed |
| `pnpm test:shared` | 176/176 passed |
| `pnpm test:web` | 80/80 passed |
| `pnpm typecheck:shared` | exit 0 |
| `pnpm typecheck:web` | exit 0 |
| `pnpm build:web` | exit 0; 124 modules |
| `node scripts/check-react-boundaries.mjs` | exit 0 |
| `git diff --check` | exit 0 |

## Boundary

This is isolated backend plus browser evidence for the listed slices. T15
remains `review`: live configuration writes, real model/MCP calls, OAuth
callback, Skill catalog install/stop/file content, permission-negative
coverage, and production/provider acceptance remain open. The Lite schema
gap is a backend capability/deployment issue and was not changed in this
React task.
