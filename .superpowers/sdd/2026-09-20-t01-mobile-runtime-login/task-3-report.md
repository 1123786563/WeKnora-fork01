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

## Review fix round 1

### Rulings

- The Runtime Port now uses Task 2's exact authenticated-session credential fields: `{ token, refreshToken }`. It is a structural subset of `MobileRuntimeRemote`, so mobile-core still does not depend on the concrete adapter in production. The interface test imports Task 2 only as a development dependency and runs the real adapter over a captured request transport.
- The approved public contract takes precedence over the initial implementation: snapshots now expose only `deployment-login`, `upgrade-required`, and `authorized`; private restore work is never published. `DeploymentInput`, the approved reason union, object-form `signIn`, `scopeLease()`, OIDC entry points, and synchronous `dispose()` are present.
- `ScopeLease` is opaque and absent from snapshots. The Runtime keeps revocation internal and returns the token only through `scopeLease()` for passing to downstream Ports.
- `PendingOidcStore`, `OidcBrowserPort`, and `AppLifecyclePort` are declared at the Runtime seam. Task 4 remains responsible for state persistence, browser interaction, callback validation, and OIDC behavior; Task 3's OIDC methods deliberately do not fabricate that behavior.

### Fix red-green evidence

After replacing the interface test with the approved contract and an actual Task 2 adapter scenario, the pre-implementation command failed as expected because the test package had no `@weknora/api-client` development dependency:

```text
ERR_MODULE_NOT_FOUND: Cannot find package '@weknora/api-client'
```

After adding the test-only workspace dependency and changing the Runtime Port/implementation, the exact focused command completed with 7 passed and 0 failed:

```text
pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts
```

The Task 2 adapter scenario asserts both `/api/v1/auth/me` and `/api/v1/system/capabilities` receive `Bearer task-2-token`, where `task-2-token` is returned by Task 2's real `passwordLogin` parser.

### Fix verification

| Command | Result |
| --- | --- |
| `pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts` | 7 passed, 0 failed |
| `pnpm exec tsx --test packages/domain/src/mobile/compatibility.test.ts packages/domain/src/mobile/auth-return.test.ts` | 14 passed, 0 failed |
| `pnpm --filter @weknora/mobile-core exec tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --types node --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions src/index.ts src/runtime/mobile-runtime.test.ts` | exit 0 |
| `git diff --check` | exit 0 |
