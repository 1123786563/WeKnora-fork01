# T14 Web terminal and route/runtime evidence (2026-09-12)

## Scope

This follow-up covers the React Web sandbox terminal panel, its ticket-backed
WebSocket controller, route dispatch corrections, and candidate static runtime
assets. It does not claim a real provider-backed shell session.

## Implementation evidence

- `5431f30` adds the shared-view terminal panel and the Web controller. The
  controller issues a short-lived sandbox ticket, derives the `ws:`/`wss:` URL
  while preserving an API deployment sub-path, sends PTY input as binary data,
  handles binary output and JSON `ready`/`error`/`exited` frames, caps retained
  output at 1 MB, validates resize bounds, and rejects stale socket events
  after close or session changes. No bearer token is put in the WebSocket URL.
- The same commit makes `/platform/knowledge-bases/:id` and its
  `/creatChat` deep link explicit, maps `/platform/agents` to the typed
  configuration surface, redirects the root and legacy knowledge-search path,
  preserves `/platform/system/*` admin routing, and renders an explicit 404
  instead of silently falling back to the knowledge-base list.
- React Web and Embed entrypoints now load `/config.js` and a bundled favicon.
  The candidate `config.js` is a development default; the Docker entrypoint
  still replaces it with the validated runtime size/locale values.

## Verification

- `pnpm --filter @weknora/web test`: 94/94
- `pnpm typecheck:web`: passed
- `pnpm typecheck:shared`: passed
- `pnpm build:web`: passed (127 modules; bundle-size warning only)
- `pnpm build:embed`: passed (68 modules)
- `node scripts/check-react-boundaries.mjs`: passed
- `git diff --check`: passed

## Evidence boundary

The terminal tests use a deterministic fake socket and prove protocol and
generation behavior only. A real shell input/output, resize, paused/no-sandbox
provider response, cross-tenant live switch, and browser E2E remain open. T14
therefore remains `review`.
