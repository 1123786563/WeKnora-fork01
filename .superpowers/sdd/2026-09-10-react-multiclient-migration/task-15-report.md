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

## Fix round 2 / shared runtime contract expansion (2026-09-12)

- Fixed the compatibility regression found in re-review: legacy MCP records without `transport_type` now hydrate to the server-default `sse`; the regression test is in `apps/web/src/configuration/editor.test.ts`.
- Added strict shared client contracts for the existing model provider list and model debug endpoint. Debug input uses the injected native multipart boundary and returns the server's redacted request preview, observations, raw response, elapsed time, and explicit error state.
- Added strict shared client contracts for the existing Skill catalog and sandbox-installed-skill routes: catalog registration/install/files/content/delete; installed-skill list/detail/files/content/reinstall/stop/update/remove. Accepted `202` envelopes, empty maps, unknown statuses, and missing data remain observable rather than becoming fabricated success.
- No model usage/occupancy route exists in the inspected Go router/handler contract, so no such client method was invented.
- Verification after this slice: `pnpm test:shared` 175/175 (exit 0), `pnpm typecheck:shared` exit 0, focused configuration tests 16/16 (exit 0), and `git diff --check` exit 0. Commits: `63ce00d` and `3b70657`.
- Remaining evidence/implementation gap: Web controls for agent sharing/selection, model debug, Skill installation progress/file panel, live writes, role-negative matrix, real model/MCP calls, OAuth callback, and browser acceptance remain open pending the Web UI slice.

## Web operations slice (2026-09-12)

- Added `ConfigurationOperations.tsx`: Agent source filtering plus selection/share/hide controls; model debug form with server-side call result and redacted rendering; Skill catalog registration/install, asynchronous status polling, stop action, and safe catalog file listing/content panel.
- Added pure Web helpers for sandbox ID normalization, traversal-safe file paths, in-progress polling, install summary, and recursive debug redaction. The catalog is intentionally independent from the usable-sandbox availability flag because the backend returns `skills_available=false` when no sandbox ID is supplied.
- Added guarded Remove actions for Agent/Model/MCP rows. Server errors remain visible and rows reload only after confirmed mutation.
- Verification: `pnpm test:web` 80/80 (exit 0), `pnpm test:shared` 176/176 (exit 0), `pnpm typecheck:web` exit 0, `pnpm typecheck:shared` exit 0, `pnpm build:web` exit 0 (124 Vite modules), `node scripts/check-react-boundaries.mjs` exit 0, and `git diff --check` exit 0. Commits: `2102a70`, `533e86e`.
- Live/browser acceptance remains unclaimed: model debug/provider calls, Skill catalog install/stop progress, file contents, Agent share/hide and deletion still need real backend role/permission and browser evidence.
