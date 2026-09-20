# T01 Task 3 — Mobile Runtime report

## Delivered

- Added `@weknora/mobile-core` as the workspace home for the Mobile Runtime interface, ports, implementation, in-memory credential adapter, and interface-level tests.
- `createMobileRuntime` owns deployment normalization, identity and active-Tenant validation, capability handshake, presentation-safe snapshots, opaque/revocable `ScopeLease` creation, credential scope, sign-out, and late-result rejection.
- Credentials, tokens, protocol details, and the private monotonic epoch do not appear in `RuntimeSnapshot`. Only a full `clientGate` verdict after a valid `/auth/me` identity and Tenant mints a lease and enters `full`; all unknown, malformed, incompatible, missing-Tenant, and remote-failure paths stay `upgrade-required`.
- `remoteFor(normalizedDeployment)` is the Runtime composition seam. The Runtime accepts only a pure HTTPS origin (no user-info, query, fragment, or path), normalizes it to `URL.origin`, and uses that same value as the credential-store key and remote-factory input.

## Required red-green evidence

The test file was added before Runtime production code. The required initial command failed as expected because `packages/mobile-core/src/runtime/mobile-runtime.ts` did not exist:

```text
ERR_MODULE_NOT_FOUND: Cannot find module .../packages/mobile-core/src/runtime/mobile-runtime.ts
```

After the implementation, the focused interface suite reported 7 passing tests and no failures.

## Verification

| Command | Result |
| --- | --- |
| `pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts` | 7 passed, 0 failed |
| `pnpm exec tsx --test packages/domain/src/mobile/compatibility.test.ts packages/domain/src/mobile/auth-return.test.ts` | 14 passed, 0 failed |
| `pnpm --filter @weknora/mobile-core exec tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --types node --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions src/index.ts src/runtime/mobile-runtime.test.ts` | exit 0 |
| `git diff --check` | exit 0 |

## Self-review

- Verified boot reads its deployment-scoped credential before `/auth/me`, and only then fetches capabilities.
- Verified sign-in persists the new credential before `/auth/me` and capability requests.
- Verified a changed deployment revokes the old lease and uses a freshly logged-in credential on a freshly constructed remote bound to the second normalized origin.
- Verified sign-out advances the internal epoch before clearing credentials, so late asynchronous results are ignored.

## Ambiguity resolved

Task 2 validates its supplied `origin`, but it deliberately reuses a caller-supplied request function, so it cannot prove that function's request base URL equals the origin; it also accepts a path-bearing HTTPS URL. Task 3 treats a Deployment as an HTTPS **origin** only and makes the Runtime pass that exact normalized origin to the remote factory. Composition must construct the Task 2 adapter and its request transport inside `remoteFor` from that argument. This fails closed and prevents a prior deployment's credential from being selected for another deployment's transport without widening Task 2 files.

## Scope note

The documented `activateTenant` flow needs a dedicated authenticated tenant-switch Port, which Task 2 does not expose and Task 3's brief does not authorize. This task therefore validates that a missing active Tenant cannot enter the authorized Runtime surface; it does not invent a parallel tenant-switch endpoint or identity authority.
