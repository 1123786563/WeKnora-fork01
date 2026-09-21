# SDD ledger — plan: docs/superpowers/plans/2026-09-20-t01-mobile-runtime-login.md (Task 4 follow-up)

Branch: `codex/t01-oidc-cold-start`
Worktree: `.worktrees/t01-oidc-cold-start`
Parent ticket: #31 / T01
Parent task: Task 4 — Persist OIDC state and PKCE across process restart

## Scope

Wire the native OIDC callback into Mobile Runtime for both warm browser return
and cold-start deep-link delivery. Preserve persisted single-use state/verifier
semantics, and make duplicate delivery unable to consume pending state twice or
revoke a session that the first delivery already authorized.

## Pre-flight and root cause

- Parent Runtime already persists and serializes pending OIDC consumption.
- `apps/mobile/src/composition.ts` completes only from the live
  `openAuthSessionAsync` promise.
- `apps/mobile/src/app/auth-return.tsx` returns `null` and never forwards the
  deep link, so a process restart loses the only completion caller.
- Locked Expo Router 55.0.18 probe maps `weknora://oidc?...` to route path
  `oidc?...`; it does not map to `auth-return?...`.
- Locked Expo Linking 55.0.17 exposes `useLinkingURL()`, which returns the raw
  cold-start URL and later URL changes without reconstructing decoded params.

Ruling: keep the backend-approved `weknora://oidc` redirect and add the matching
`oidc` file route as an alias of the callback component — the backend rejects a
different frontend redirect, and Expo app/SDK config is out of scope — cost if
wrong: a platform-specific router behavior could still require installed-device
evidence, which remains an existing acceptance caveat.

Ruling: a replay delivered after successful authorization is rejected as a
no-op that preserves the current authorized snapshot and lease — invalidating
the newly authorized session would turn duplicate OS delivery into logout-like
behavior — cost if wrong: callers expecting a visible replay error will instead
retain the verified session.

## TDD evidence

RED 1 — route/cold-start behavior and authorized-session preservation:

```text
pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx \
  packages/mobile-core/src/runtime/mobile-runtime.test.ts
# tests 31 / pass 27 / fail 4
TypeError: callback.deliverOidcReturn is not a function
ERR_MODULE_NOT_FOUND: apps/mobile/src/app/oidc.tsx
expected authorized snapshot, received upgrade-required/authentication-required
```

RED 2 — duplicate delivery still touched the pending store twice:

```text
pnpm exec tsx --test --test-name-pattern 'without duplicate delivery' \
  packages/mobile-core/src/runtime/mobile-runtime.test.ts
# tests 1 / pass 0 / fail 1
actual pending-store calls: ['save', 'consume', 'consume']
expected: ['save', 'consume']
```

GREEN — focused regression:

```text
pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx \
  packages/mobile-core/src/runtime/mobile-runtime.test.ts
# tests 31 / pass 31 / fail 0
```

GREEN — task gates:

```text
pnpm --filter @weknora/mobile test
# tests 11 / pass 11 / fail 0

pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts \
  packages/domain/src/mobile/auth-return.test.ts
# tests 27 / pass 27 / fail 0

pnpm --filter @weknora/mobile typecheck
# exit 0

pnpm --filter @weknora/mobile-core exec tsc --noEmit --strict --skipLibCheck \
  --target ES2022 --lib ES2022,DOM --types node --module NodeNext \
  --moduleResolution NodeNext --allowImportingTsExtensions src/index.ts \
  src/runtime/mobile-runtime.test.ts
# exit 0

pnpm typecheck:shared
# exit 0

git diff --check
# exit 0
```
