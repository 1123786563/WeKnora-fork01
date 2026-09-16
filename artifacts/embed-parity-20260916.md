# Embed parity review — 2026-09-16

## Scope and checkout facts

- Requested scope: `apps/embed/**` and `frontend/embed-main.ts`; mobile code is excluded.
- The requested paths are not present in the current `main` checkout. The current
  independent Vue entry is `frontend/src/embed-main.ts`; `frontend/embed-main.ts`
  also does not exist.
- `apps/embed/**` exists only in the historical commit `4ee56e08` (`feat: add
  isolated embed and integrations surfaces`), which is not an ancestor of the
  current `HEAD` (`b65696b4`). I reviewed that commit as the candidate React
  implementation, but did not copy it into the working tree because doing so
  would be an unapproved cross-branch import rather than a scoped repair.
- Existing dirty files were preserved unchanged:
  `apps/web/src/knowledge-settings/GraphSettings.test.ts`,
  `apps/web/src/settings/McpToolsDirectory.test.tsx`,
  `docs/superpowers/plans/mobile-workbench-progress.md`,
  `artifacts/browser-evidence-20260915/current/`, and
  `docs/superpowers/plans/2026-09-15-protected-route-playwright-evidence.md`.

## Findings

### F-01 — high: React bridge can broadcast bootstrap/ready to `*`

The historical React implementation uses `postToHost()` at
`apps/embed/src/EmbedApp.tsx:81-84`. Its guard intentionally blocks sensitive
messages before pinning, but `createEmbedBridgeGuard().targetOrigin()` returns
`*` until the first accepted host message (`packages/views/src/embed/bridge.ts`).
The initial `bootstrap_request` at `EmbedApp.tsx:147-150` therefore uses `*`.
This conflicts with the documented contract that the iframe-to-host bridge must
use a precise origin and creates a weaker handshake boundary. The current Vue
authority has the same explicit fallback at `frontend/src/api/embed/index.ts:479-495`.

Required repair when the React files are restored: make bootstrap/ready delivery
wait for a verifiable parent origin (referrer or an explicit, origin-pinned
handshake), or use a separately defined non-sensitive handshake protocol whose
security properties are documented and tested. Do not silently accept `*` as a
parity implementation.

### F-02 — high: React candidate has no file-proxy consumption path

The backend registers the channel-scoped proxy at
`internal/router/routes_agent.go:232-255` as
`GET /api/v1/embed/:channel_id/files`; anonymous embed responses are deliberately
forced to protected resource handles, as covered by
`internal/handler/embed_resource_urls_test.go:53-83`.

The historical React API client exposes `chunk()` and chat/session methods, but no
embed file-proxy URL resolver or file fetch method. `EmbedApp.tsx:202-210` only
copies references into message state, and `EmbedApp.tsx:241-250` renders message
content as plain text. There is no equivalent of the Vue protected-file access
plane used by `frontend/src/views/embed/EmbedPage.vue:82-94`, so image/chart/file
references cannot be rendered through the channel-scoped proxy. This is a
functional and authorization parity gap, not merely a visual difference.

Required repair: add a React-owned adapter for `resource://` handles and render
all embed message/reference attachments through
`/api/v1/embed/:channel_id/files?file_path=...` with the short-lived embed token
and session headers as required by the backend contract. Add tests for handle
rewriting, encoded channel/path values, and refusal to emit public or
credential-free URLs.

### F-03 — medium: React error state loses actionable recovery semantics

The historical candidate has one `error` status (`EmbedApp.tsx:69-74`) and renders
the raw exception text at `EmbedApp.tsx:128-133` and `EmbedApp.tsx:248-249`.
It does not distinguish missing channel/token, exchange refusal, disabled channel,
stale session, file failure, or chat-stream failure, and provides no retry action.
The current Vue authority has distinct loading/awaiting-token/error branches at
`frontend/src/views/embed/EmbedPage.vue:1-52` and maps known backend errors in
`frontend/src/composables/useEmbedBridge.ts:175-189`.

Required repair: preserve the state matrix (`loading`, `awaiting token`, ready,
session recovery, send/streaming, stopped, and error), map known failures to
stable localized messages, and expose a retry/new-session action without
discarding a valid session unnecessarily.

### F-04 — medium: candidate route/token contract is only partially proven

The historical candidate does have a sound independent path parser and storage
tests in `apps/embed/src/bootstrap.test.ts`, plus exchange/header/path tests in
`packages/api-client/src/embed/index.test.ts`. It also uses an anonymous transport
with `credentials: 'omit'` (`EmbedApp.tsx:50-52`) and distinguishes `ems_` session
tokens from publish tokens (`EmbedApp.tsx:46-48`, `96-104`).

However, no current checkout contains the app entry, no React build can be run
from this `HEAD`, and the tests are not present in the working tree. Therefore
these are historical static signals only, not current acceptance evidence.
The candidate also accepts query-string tokens through `extractEmbedToken()`;
the preferred secure path is the hash/postMessage flow so publish tokens do not
enter URL/referrer logs.

## Positive evidence from current authority/backend

- The Vue entry is genuinely independent: `frontend/src/embed-main.ts:14-22`
  owns only `/embed/:channelId`, and mounts separately at `#embed-app` in
  `frontend/src/embed-main.ts:25-34`.
- Secure exchange is correctly separated from session-token use in the Vue
  bridge: it exchanges only non-`ems_` tokens and fails closed in production when
  exchange returns no session token (`frontend/src/composables/useEmbedBridge.ts:91-121`).
- The backend exposes exchange, config, session, both chat modes, message
  history, stop, and file-proxy routes under the channel-scoped EmbedAuth group
  (`internal/router/routes_agent.go:232-255`).
- Backend tests cover exchange success/unavailability/session-token rejection and
  origin/token extraction (`internal/handler/embed_channel_exchange_test.go`,
  `internal/middleware/embed_auth_test.go`).

## Verification evidence

Run from the current checkout on 2026-09-16:

```text
go test ./internal/handler ./internal/middleware
ok github.com/Tencent/WeKnora/internal/handler 1.287s
ok github.com/Tencent/WeKnora/internal/middleware 0.570s

pnpm --dir frontend test
1..821
# tests 824
# suites 0
# pass 824
# fail 0
```

No React embed test was added: the implementation and its test fixtures are
absent from the current checkout, and adding the historical app would exceed a
review-only, current-branch repair. No mobile files were read or changed.

## Decision / next gate

Status: `blocked-env` for React embed implementation acceptance, not a claim that
the historical candidate is production-ready. Restore or explicitly select the
React embed commit/files in a follow-up task, then run a red-green repair for
F-01/F-02 and an independent browser check covering direct iframe, widget,
postMessage origin rejection, secure token exchange, disabled/expired-session
errors, and protected image/file rendering.
