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

## Scope and concern

`createInMemoryCredentialStore` remains the available credential-store adapter
at the approved Task 3 seam. Task 4 provides native secure storage only for
short-lived pending OIDC state, which this composition uses. Persisting login
credentials across app restart would require a separately approved native
CredentialStore adapter rather than exposing tokens to a screen.
