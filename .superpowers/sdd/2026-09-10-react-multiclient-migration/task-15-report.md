# T15 local implementation report

- Added strict model/MCP credential subresource methods and MCP test/tool parsers in `packages/api-client/src/configuration.ts`; exported public types from `packages/api-client/src/index.ts`.
- Added configuration payload/draft helpers with recursive secret removal and a Web editor for Agent/Model/MCP; Skill remains explicitly read-only.
- Added RED-to-GREEN tests in `packages/api-client/src/configuration.test.ts`, `apps/web/src/configuration/surface.test.ts`, and `apps/web/src/configuration/editor.test.ts`.
- Verification: focused tests 12/12 and editor tests 3/3; `pnpm test:web` 69/69; `pnpm typecheck:web` 0; `pnpm typecheck:shared` 0; `pnpm build:web` 0; `node scripts/check-react-boundaries.mjs` 0; `git diff --check` 0.
- Commit: `77d431a`.
- Missing evidence: live configuration writes, role-negative matrix, real model/MCP call, OAuth callback, Skill install progress/file panel, and browser interaction acceptance.

## Fix round 1 (2026-09-12)

- Hardened main model/Agent/MCP detail parsing to recursively reject secret-shaped keys, including camelCase and nested keys, while allowing safe structural MCP auth configuration.
- Made credential status parsing fail closed when any required field metadata is absent.
- Made model, MCP credential, and MCP OAuth DELETE paths accept only the shared 204 representation (`undefined`) and reject response bodies.
- Accepted successful empty MCP tool lists and empty tool/resource descriptions; added strict nested MCP tool/resource parsing for test results.
- Added MCP `transport_type` validation/default (`sse`), editor selector options (`sse`, `http-streamable`, `stdio`), and local credential-clear state updates.

TDD evidence and verification:

- `pnpm exec tsx --test packages/api-client/src/configuration.test.ts apps/web/src/configuration/surface.test.ts apps/web/src/configuration/editor.test.ts` — exit 1 (RED): 18 tests, 11 passed, 7 failed on the intended missing contracts.
- `pnpm exec tsx --test packages/api-client/src/configuration.test.ts` after adding OAuth DELETE coverage — exit 1 (RED): 12 tests, 11 passed, 1 failed on the existing action-envelope parser.
- `pnpm exec tsx --test packages/api-client/src/configuration.test.ts apps/web/src/configuration/surface.test.ts apps/web/src/configuration/editor.test.ts` — exit 0 (GREEN): 23/23 passed.
- `pnpm exec tsx --test apps/web/src/configuration/editor.test.ts` after adding direct-record camelCase redaction coverage — exit 1 (RED): 6 tests, 5 passed, 1 failed on the existing underscore-only filter.
- `pnpm exec tsx --test apps/web/src/configuration/editor.test.ts && pnpm exec tsx --test packages/api-client/src/configuration.test.ts apps/web/src/configuration/surface.test.ts apps/web/src/configuration/editor.test.ts` — exit 0 (GREEN): 6/6 and 24/24 passed.
- `pnpm test:web` — exit 0: 74/74 passed.
- `pnpm test:shared` — exit 0: 171/171 passed.
- `pnpm typecheck:shared` — exit 0: no diagnostics.
- `pnpm typecheck:web` — exit 0: no diagnostics.
- `pnpm build:web` — exit 0: Vite transformed 122 modules and produced the production bundle.
- `node scripts/check-react-boundaries.mjs` — exit 0: `React boundary checks passed`.
- `git diff --check` — exit 0: no whitespace errors.

No live configuration writes, production model/MCP calls, OAuth callback, permission-negative matrix, browser acceptance, or Skill install/file-panel evidence was claimed.
