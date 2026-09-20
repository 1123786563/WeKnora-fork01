# T01 Task 5 — Mobile login and safe landing composition report

## Delivered

- Added `apps/mobile/src/composition.ts` as the sole native composition root.
  It builds the Task 2 remote adapter from the normalized Runtime deployment,
  attaches the Task 4 SecureStore and browser adapters, and sends the browser
  callback through `MobileRuntime.completeOidc`.
- Added Runtime presentation surfaces for deployment login, upgrade-required,
  and an authorized Task Office placeholder. Screens receive only display data
  and callbacks; none imports `@weknora/api-client` or `@weknora/contracts`.
- Added a strict self-hosted origin guard: deployment input must normalize to
  an HTTPS origin and cannot contain user-info, path, query, or fragment. The
  optional official-cloud value is read only from
  `EXPO_PUBLIC_WEKNORA_CLOUD_ORIGIN` and is omitted when unset or invalid.
- Replaced the initial-route placeholder with the Runtime-driven application
  root. `RuntimeSurface` selects from `RuntimeSnapshot.surface`; an
  `authorized` snapshot additionally needs a deployment, user identity, and
  active Tenant before the authorized landing is reachable. Every other unsafe
  state renders the upgrade-required surface.
- The authorized landing contains no Task API request. The upgrade-required
  screen exposes no Task Office control.

## TDD evidence

### RED

The required command was run after adding the new smoke expectations and
before production modules existed:

```text
pnpm --filter @weknora/mobile test
# tests 8 / pass 4 / fail 4
Error [ERR_MODULE_NOT_FOUND]: Cannot find module
'.../apps/mobile/src/screens/DeploymentLoginScreen.tsx'
Error [ERR_MODULE_NOT_FOUND]: Cannot find module
'.../apps/mobile/src/composition.ts'
```

The failures were for the newly specified screen/composition seams. The
existing typecheck smoke assertion also failed because those imports did not
yet exist.

### GREEN

```text
pnpm --filter @weknora/mobile test
# tests 7 / pass 7 / fail 0

pnpm --filter @weknora/mobile typecheck
# exit 0

rg -n "@weknora/(api-client|contracts)" apps/mobile/src/screens
# exit 1 (no forbidden Screen imports)

git diff --check
# exit 0
```

The smoke coverage asserts literal safe/unsafe origin cases, routes an
upgrade snapshot to the restricted screen, rejects an authorized snapshot
without an active Tenant, and reaches the authorized screen only when both
identity and Tenant are present.

## Scope

Native composition now uses SecureStore-backed credential, active-deployment,
and pending-OIDC adapters. The credential adapter remains below Runtime and
screens continue to receive presentation-safe snapshots and callbacks only.

## Review fix round 1

The review correctly identified that the original native composition used the
test-only in-memory credential adapter and did not start Runtime restoration.
This round replaces that composition wiring and extends the Runtime seam.

- Added `DeploymentStore` to the Runtime Ports. On an authorized handshake,
  Runtime persists the normalized presentation-safe deployment; `boot()` reads
  it when no explicit input is provided and then performs the normal
  credential → identity → capabilities verification sequence. Sign-out clears
  both the credential and deployment records.
- Added OS-backed credential and deployment adapters. Credential values are
  independently serialized under a key derived from their normalized HTTPS
  origin. The deployment record contains only origin and label.
- `MobileApp` calls Runtime `boot()` through a once-only startup guard. No
  screen receives a credential, storage handle, remote client, or protocol
  input.
- The Node smoke harness now renders the native element tree with controlled
  hooks. It presses the actual sign-in button, proves five invalid origin
  forms do not invoke callbacks, proves a normalized valid origin does, and
  inspects the restricted/authorized screen controls and text.

### Fix-round RED

```text
pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts
# tests 12 / pass 10 / fail 2
AssertionError: deployment-store calls were [] rather than
['read', 'write:https://weknora.example.test']
AssertionError: deployment-store calls were [] rather than
['write:https://weknora.example.test', 'clear']

pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx
# tests 5 / pass 3 / fail 2
TypeError: bootRuntimeOnce is not a function
```

### Fix-round GREEN

```text
pnpm --filter @weknora/mobile test
# tests 9 / pass 9 / fail 0

pnpm --filter @weknora/mobile typecheck
# exit 0

pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts
# tests 12 / pass 12 / fail 0

pnpm --filter @weknora/mobile-core exec tsc --noEmit --strict --skipLibCheck \
  --target ES2022 --lib ES2022,DOM --types node --module NodeNext \
  --moduleResolution NodeNext --allowImportingTsExtensions src/index.ts \
  src/runtime/mobile-runtime.test.ts
# exit 0

rg -n "@weknora/(api-client|contracts)" apps/mobile/src/screens
# exit 1 (no forbidden Screen imports)

git diff --check
# exit 0
```
