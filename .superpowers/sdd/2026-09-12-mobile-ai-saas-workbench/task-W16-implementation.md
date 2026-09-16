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
