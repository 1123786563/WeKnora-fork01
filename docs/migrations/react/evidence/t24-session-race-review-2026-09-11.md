# T24 会话竞态复核 — 2026-09-11

## Independent review findings

The review of the previous mobile session patch reproduced four source-level
risks:

1. a refresh credential write could finish after `invalidate({ clear: false })`;
2. an invalidated refresh promise could be reused by the next session;
3. login/OIDC adoption or workspace switching could write state after a newer
   session had started; and
4. a session without an active tenant could retain the previous selection.

The Android native runtime attempt was reviewed separately. Its system ANR and
missing authenticated flow remain a runtime evidence gap, not a source-level
race finding.

## Follow-up boundary

- `packages/api-client/src/auth/refresh-coordinator.ts` now detaches invalidated
  `inFlight` work, serializes credential persistence, reconciles an already
  started stale write to the previous credential when no newer write wins, and
  exposes the same guarded writer to session adoption and workspace switching.
- `apps/mobile/src/runtime.tsx` captures an epoch for adoption and switching,
  checks it after every asynchronous boundary, blocks workspace reconciliation
  during session transitions, guards initial SecureStore hydration and server
  address changes, uses the guarded credential writer, and clears the stored
  tenant when the adopted session has no tenant.
- `apps/mobile/src/platform/workspace.ts` serializes workspace-selection writes
  with newest-wins ordering; stale queued writes are skipped.
- `apps/mobile/src/runtime.tsx` now refuses refresh initiation while any session
  transition is active, and the mobile transport omits Bearer and tenant
  headers during that transition. Changing the server address clears the
  persisted bearer credential and workspace selection before the new origin is
  activated; the tenant ref is cleared synchronously so an old tenant header
  cannot escape through a stale render.

## Verification

Focused command:

```text
pnpm exec tsx --test packages/api-client/src/auth.test.ts apps/mobile/src/platform/workspace.test.ts apps/mobile/src/platform/transport.test.ts
exit 0 — 23/23
```

The regression coverage includes an already-started credential write,
replacement of an invalidated in-flight refresh, and ordering of a delayed
workspace selection write. The broader checks also passed:

```text
pnpm test:shared       exit 0 — 162/162
pnpm test:mobile       exit 0 — 41/41
pnpm typecheck:shared  exit 0
pnpm typecheck:mobile  exit 0
git diff --check       exit 0
```

This evidence closes the identified source-level race follow-up only. It does
not provide Android authenticated business-flow evidence, provider callback
evidence, or a full independent release review.
