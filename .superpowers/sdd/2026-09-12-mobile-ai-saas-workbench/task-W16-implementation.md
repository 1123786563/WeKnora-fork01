# W16 implementation report — secure deep links and pending notification card

Status: implemented; awaiting independent review.

Base: `6ba63c4b` (`docs(sdd): accept W15 and dispatch W16`)

## Scope

- Added a strict `weknora://execution?tenant=<id>&run=<id>` parser. It rejects
  extra or duplicate parameters, alternate authorities, credentials, fragments,
  actions, tokens, server overrides, control characters, and unsafe IDs.
- Added `NotificationRouter`, which accepts cold-start, URL, and Expo push
  response events through one deduplicated in-memory intent path. Intents expire
  after ten minutes and are never persisted with credentials or command data.
- Before navigation, the router requires the authenticated product session,
  optionally delegates tenant selection to a trusted member resolver, then
  queries the authenticated W03 execution endpoint and validates returned owner
  and tenant fields. It only navigates to the execution screen; it never
  approves or cancels an interaction.
- Added a pending notification card with retry/dismiss controls and wrapped the
  retained Happy home view in a product-owned `WorkbenchScreen`.

## Validation

| Check | Result |
| --- | --- |
| `pnpm exec tsx --test apps/mobile/sources/weknora/notifications/deep-link.test.ts` | blocked-env: workspace has no `tsx` executable (`ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL`) |
| `pnpm exec vitest run ...` | blocked-env: workspace has no `vitest` executable |
| `../../node_modules/.bin/tsc -p apps/mobile/tsconfig.json --noEmit` | blocked-env: `expo/tsconfig.base` is not installed; TypeScript also reports the existing baseUrl deprecation |
| `git diff --check` | passed before commit |

Native iOS/Android notification delivery and cold-start interaction were not
claimed because the required native build/device and push credentials are not
available in this worktree.

## Round 1 constrained fixes

Applied review findings from `task-W16-final-review.md`:

- Successful ownership verification now routes to `/(app)`, whose product
  `WorkbenchScreen` owns the notification card and retained Happy renderer;
  it no longer routes to `/session/[id]` and cannot fall through to the legacy
  Happy `SessionView` route.
- Startup reads `Notifications.getLastNotificationResponseAsync()` and sends
  its payload through the same parser, deduplication, expiry, and auth path as
  foreground responses.
- A 2xx execution response is rejected unless non-empty owner and tenant
  fields exist and match the authenticated product identity and deep-link
  tenant.
- Added Router behavior coverage for login recovery, terminated-app response,
  duplicate/expired intents, ownership rejection, and the absence of POST
  approve/cancel traffic.

Focused validation after the fix: `git diff --check` passed. TypeScript/Vitest
and native checks remain `blocked-env` for the dependency/device reasons above.
