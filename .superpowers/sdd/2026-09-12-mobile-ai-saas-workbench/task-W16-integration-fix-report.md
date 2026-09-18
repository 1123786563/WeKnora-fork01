# W16 integration fix report — make the never-run notification tests actually pass

Status: fixed; commit `96efa90c` on top of FIX_BASE `4480674e`.

Scope: `apps/mobile/sources/weknora/auth/session.test.tsx` (7 failing) and
`apps/mobile/sources/weknora/notifications/NotificationRouter.test.tsx` (3
failing). Both files landed on lanes where Vitest was `blocked-env`, so they
had never actually executed; after integration they failed on first real run.

## Root causes

The mobile Vitest setup has **no `@/` alias resolution** (no vitest.config in
`apps/mobile`; `tsconfig.json` paths are not picked up). `vi.mock('<alias>')`
therefore only matches modules the source imports under the *same string
specifier*. This single fact drove most of the failures.

1. **session.test.tsx 6/7 — credentials mock never applied.** `session.tsx`
   imports credentials via `'./credentials'` while the test mocked
   `'@/weknora/auth/credentials'`. The real module loaded, `read()` returned
   `{kind:'anonymous'}`, and the whole mount chain (me → switchTo →
   setCredential → syncMobileDevice → register/flush) early-returned. All six
   register/flush/credential assertions starved. Fix: mock `'./credentials'`
   (test defect; verified by instrumenting the mount chain — the read result
   was the divergence point).
2. **session.test.tsx 1/7 — offline logout mock violated the seam contract.**
   The test `mockRejectedValueOnce`'d `revokeOnLogout`, but the verified
   contract (`registration.test.ts` "keeps a minimal pending revocation on
   offline logout", `registration.ts` L119-130) is that a failed revoke
   *queues* a pending revocation and returns `{}` — it never rejects. Fix:
   the mock now mirrors the real catch-branch (queue into `input.pending`,
   return `{}`) so logout completes and the same-owner login flushes.
3. **NotificationRouter — unstable `useRouter` mock caused an effect loop.**
   The mock returned a fresh `{replace}` object every render. `openIntent`
   depends on `router`, so every re-render produced new `openIntent`/`acceptRaw`
   identities, re-ran both listener effects, and verify-false → setError →
   re-render → verify… looped forever ("fails closed" timed out at 5s). Real
   `useRouter` returns a stable reference. Fix: module-level stable router
   object in the mock (test defect).
4. **NotificationRouter source — double navigation after authorization.** The
   verify-success path calls `router.replace('/(app)')` and then
   `setIntent({...authorizedAt})`, which re-triggers the intent effect and
   navigates a second time via the `authorizedAt` branch. "deduplicates it"
   asserted `toHaveBeenCalledTimes(1)` and got 2. Fix: the intent effect now
   skips intents that already carry `authorizedAt` (they navigated at
   authorization time); the effect remains the login-recovery path for
   unverified intents (source fix).
5. **NotificationRouter source — startup events replayed after auth change.**
   `getInitialURL`/`getLastNotificationResponseAsync` were re-consumed every
   time the listener effect re-ran (credential change ⇒ new `openIntent`
   identity). Past the 10-minute TTL the `seen` map no longer deduplicates, so
   an *expired* launch intent was resurrected and navigated ("rejects expired
   and repeated intents" saw `replace('/(app)')` twice). Fix: startup events
   are consumed exactly once per mount via a ref guard (source fix; matches
   the W16 report's "same parser, deduplication, expiry path" semantics).
6. **Pre-existing typecheck debt (same blocked-env cause):** 9 errors in
   `session.tsx`/`session.test.tsx` — `reduce` generic inference, untyped
   `register` mock making `mock.calls.at(-1)?.[0]` `undefined`, and a
   `Promise<void>` vs `Promise<undefined>` mismatch. All in-scope files,
   fixed minimally; zero new errors introduced.

## Extra integration gap found and fixed (step 2c category)

`WorkbenchScreen.test.tsx` failed at baseline (verified by stashing my
changes): the W16 merge mounted `PendingNotificationCard` in
`WorkbenchScreen.tsx` via `@/weknora/notifications/NotificationRouter`, but
the W11 suite never mocked that specifier, so the module could not even load.
Added a minimal `PendingNotificationCard: () => null` stub following the
suite's existing `@/…` mock precedent.

## Disposition of the 10 cases

- 7 fixed in tests (mock path ×6 via one-line specifier fix counted per-case;
  offline-contract mock ×1)
- 3 fixed in source (NotificationRouter double-navigation, startup-event
  replay, — counted per failing case: dedup 1, fails-closed 1 [source loop +
  mock stability], expired 1 [source replay + mock stability])
- 0 assertion changes — every original assertion held once the defects were
  fixed
- Plus 1 integration adapter (WorkbenchScreen.test.tsx stub) and 9 pre-existing
  typecheck errors cleared in the same two domain files

## Verification (GREEN)

| Check | Result |
| --- | --- |
| `vitest run` session.test.tsx + NotificationRouter.test.tsx | 11/11 passed (10 previously failing + 1 previously passing) |
| `vitest run` session/[id].test.tsx + ConversationScreen.test.tsx | 27/27 passed |
| `vitest run` WorkbenchScreen.test.tsx | passed (was failing at baseline) |
| combined run of all five files | 44/44 passed |
| `tsc --noEmit` (apps/mobile) | clean (was 9 pre-existing errors) |
| `tsx --test deep-link.test.ts` | passes (node:test file; only fails when a directory is passed to vitest — not a vitest suite, by design) |

## Concerns

- The missing `@/` alias resolution in vitest is a latent trap for every
  future mobile test that mocks an alias-imported module while the source
  uses a relative specifier (or vice versa). A one-line `resolve.alias`
  vitest config would make mock paths spec-independent; left out of this
  commit to keep the fix scoped, flagged for the platform lane.
- `live-progress.test.ts` and `native-progress-port.test.ts` are also
  node:test files; running `vitest run <directory>` reports them as "No test
  suite found". They pass under `tsx --test` (deep-link verified; the other
  two follow the same harness and were out of the 10-case scope).
