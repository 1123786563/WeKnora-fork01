# T01 Native Runtime and Deployment Login Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the first installable iOS/Android WeKnora app shell that authenticates against one official or HTTPS self-hosted Deployment and only enters an authorized surface after a fail-closed protocol capability check.

**Architecture:** `packages/mobile-core` owns Mobile Runtime, the opaque Scope Lease, authentication sequencing, capability gating and recovery state. The API client, Expo secure storage, browser/linking and lifecycle are injected Ports/Adapters. `apps/mobile` composes the Runtime and renders only its snapshots; Screens do not call wire clients directly.

**Tech Stack:** TypeScript workspace, Expo SDK 55.0.0 with React Native 0.83 and React 19.2.0 as listed in the [official Expo SDK 55 reference](https://docs.expo.dev/versions/v55.0.0/), existing `@weknora/api-client` auth endpoints, existing `@weknora/domain/mobile` compatibility and OIDC-return policies, `expo-secure-store`, Expo AuthSession/Linking, Node test runner through `tsx`, existing mobile GitHub Actions workflow.

**Spec:** `docs/specs/2026-09-20-mobile-ai-office-design.md`; supporting seam contract: `docs/specs/2026-09-20-mobile-module-seams.md`; Ticket: GitHub issue #31.

## Global Constraints

- WeKnora remains the only identity and Tenant authorization authority.
- Use iOS and Android native app targets; do not import DOM packages `packages/ui` or `packages/views`.
- `packages/mobile-core` owns Module Interfaces, orchestration and Ports; `apps/mobile` is a composition root and presentation/native Adapter.
- Screens must not import `packages/contracts` or `packages/api-client`, and must not own tokens, cursors, request IDs, protocol decisions or scope generations.
- `GET /api/v1/system/capabilities` is authenticated; login/OIDC exchange and `/auth/me` precede capability negotiation.
- Unknown capability schema or protocol outside the supported window fails closed and renders only the login/upgrade-safe surface.
- OIDC state and PKCE verifier survive app process restart in protected storage and are consumed once.
- Deployment origin is HTTPS, normalized before credential use; bearer credentials never cross origin changes.
- Official cloud URL is supplied through explicit build configuration; no guessed production origin is embedded.
- Native test bundles and mocked HTTP tests do not count as installed-device acceptance evidence.

## Review Focus

- A malformed or unknown capability payload must never create a Scope Lease that allows an authorized surface; pin this in Task 3's Runtime interface tests.
- A Deployment origin change after login must not reuse the previous Deployment's credential; pin this in Task 2's remote adapter tests.
- An interrupted OIDC callback or app restart must neither lose the PKCE verifier nor accept a replayed state; pin this in Task 4's persistence tests.
- A stale auth/capability response after sign-out or Deployment change must not resurrect the previous scope; pin this in Task 3's concurrency tests.
- A valid credential with no active Tenant must not be shown a Task surface; pin this in Task 5's shell navigation tests.

---

## File Structure

| Path | Responsibility |
|---|---|
| `packages/mobile-core/package.json`, `src/index.ts` | Public mobile-core package surface and package boundary. |
| `packages/mobile-core/src/runtime/types.ts` | Runtime views, identity/deployment types, safe-surface enum and opaque lease contract. |
| `packages/mobile-core/src/runtime/ports.ts` | Identity, credential, capability, OIDC, and lifecycle Ports. |
| `packages/mobile-core/src/runtime/mobile-runtime.ts` | State machine and orchestration for restore, sign-in, OIDC callback, capability gate, scope revoke and sign-out. |
| `packages/mobile-core/src/runtime/in-memory-adapters.ts` | Deterministic adapters used by Interface tests. |
| `packages/mobile-core/src/runtime/mobile-runtime.test.ts` | Runtime behavior tests at its public Interface. |
| `packages/api-client/src/mobile/runtime.ts` | Narrow remote adapter over the existing auth API and authenticated capabilities request. |
| `packages/api-client/src/mobile/runtime.test.ts` | Request path, origin, envelope and error parsing tests. |
| `packages/api-client/package.json` | Explicit `./mobile/runtime` package export. |
| `apps/mobile/package.json`, `app.json`, `tsconfig.json`, `babel.config.js` | Expo app configuration and workspace scripts. |
| `apps/mobile/src/composition.ts` | Build-time deployment configuration and concrete Adapter composition. |
| `apps/mobile/src/adapters/secure-store.ts` | Expo SecureStore implementation for credential and pending OIDC state Ports. |
| `apps/mobile/src/adapters/oidc-browser.ts` | Expo browser and deep-link Adapter. |
| `apps/mobile/src/screens/DeploymentLoginScreen.tsx` | HTTPS Deployment selection and password/OIDC entry. |
| `apps/mobile/src/screens/UpgradeRequiredScreen.tsx` | Safe incompatibility explanation with no business controls. |
| `apps/mobile/src/screens/AuthorizedLandingScreen.tsx` | Minimal authenticated landing surface for later Task Office work. |
| `apps/mobile/src/app/_layout.tsx`, `index.tsx`, `auth-return.tsx` | Native navigation and callback routes. |
| `apps/mobile/src/app-smoke.test.tsx` | Composition and screen routing smoke tests using Runtime snapshots. |
| `package.json`, `pnpm-workspace.yaml`, `pnpm-lock.yaml` | Root mobile scripts and workspace membership. |
| `.github/workflows/mobile.yml` | Existing mobile CI path is made runnable by the new app. |

Use Expo SDK `~55.0.0`, React Native `0.83.x`, React `19.2.0`, and Node.js `24` in CI. Install platform packages such as SecureStore/AuthSession using `npx expo install` so their exact compatible versions are recorded in `apps/mobile/package.json` and `pnpm-lock.yaml`. The Expo SDK reference maps SDK 55 to React Native 0.83 and React 19.2.0 and sets a Node minimum of 20.19.x; the existing mobile workflow's Node 24 satisfies that floor.

## Interface Contract

`MobileRuntime` is the only business entry point used by the app shell:

```ts
export type RuntimeSurface = 'deployment-login' | 'upgrade-required' | 'authorized';
export interface RuntimeSnapshot {
  surface: RuntimeSurface;
  deployment?: { origin: string; label: string };
  identity?: { userId: string; activeTenantId?: string };
  reason?: 'protocol-mismatch' | 'unknown-capability' | 'tenant-required' | 'authentication-required';
}
export interface ScopeLease { readonly __scopeLeaseBrand: unique symbol }
export interface MobileRuntime {
  snapshot(): RuntimeSnapshot;
  subscribe(listener: (snapshot: RuntimeSnapshot) => void): () => void;
  boot(deployment?: DeploymentInput): Promise<RuntimeSnapshot>;
  signIn(input: { deployment: DeploymentInput; email: string; password: string }): Promise<RuntimeSnapshot>;
  beginOidc(input: { deployment: DeploymentInput; redirectUri: string }): Promise<void>;
  completeOidc(callbackUrl: string): Promise<RuntimeSnapshot>;
  scopeLease(): ScopeLease | undefined;
  signOut(): Promise<void>;
  dispose(): void;
}
```

The lease is an opaque runtime-owned token. Implementations keep its revocation bit and scope generation private; consumers can only pass the lease to a Port, never construct or inspect it. The remote adapter exposes `passwordLogin`, `me`, `oidcConfig`, `oidcUrl`, `oidcExchange`, `refresh`, and `deploymentCapabilities` using the existing `createAuthApi` and existing transport. `deploymentCapabilities` sends the bearer token to `GET /api/v1/system/capabilities` and parses the `{ success, data: { protocol_minimum, protocol_maximum } }` envelope through `clientGate` from `@weknora/domain/mobile`.

## Implementation Tasks

### Task 1: Add the native app workspace shell

**Files:**
- Create: `apps/mobile/package.json`, `apps/mobile/app.json`, `apps/mobile/tsconfig.json`, `apps/mobile/babel.config.js`, `apps/mobile/src/app/_layout.tsx`, `apps/mobile/src/app/index.tsx`
- Modify: `pnpm-workspace.yaml`, root `package.json`, `pnpm-lock.yaml`
- Test: `apps/mobile/src/app-smoke.test.tsx`

**Interfaces:**
- Consumes: none; this establishes the Expo workspace.
- Produces: package `@weknora/mobile` with scripts `test` and `typecheck`; a runnable Expo Router app entry.

- [ ] **Step 1: Write the failing workspace smoke test** asserting the app module exports an application root and that `pnpm --filter @weknora/mobile typecheck` resolves the package.
- [ ] **Step 2: Run it to verify it fails** with the package/filter missing because `apps/mobile` does not exist in this checkout.
- [ ] **Step 3: Create the Expo shell and wire the workspace** with Expo SDK `~55.0.0`, React Native `0.83.x` and React `19.2.0`; use `npx expo install expo-secure-store expo-auth-session expo-web-browser expo-linking` for native modules. Set `test` to `tsx --test src/**/*.test.ts*` and `typecheck` to `tsc --noEmit`.
- [ ] **Step 4: Run `pnpm install --lockfile-only` and the smoke test**; expect the app workspace to resolve and the smoke test to pass.
- [ ] **Step 5: Run `pnpm --filter @weknora/mobile exec expo export --platform ios --output-dir /tmp/weknora-mobile-ios` and the Android equivalent**; expect both static exports to succeed.
- [ ] **Step 6: Commit** the scaffold, workspace manifests and test as `feat(mobile): add native app workspace shell`.

### Task 2: Add a typed WeKnora Runtime remote adapter

**Files:**
- Create: `packages/api-client/src/mobile/runtime.ts`, `packages/api-client/src/mobile/runtime.test.ts`
- Modify: `packages/api-client/package.json`
- Test: `packages/api-client/src/mobile/runtime.test.ts`

**Interfaces:**
- Consumes: existing `createAuthApi(request)` from `packages/api-client/src/auth/endpoints.ts`; `ClientRequest` and `HttpTransport` from existing api-client ports.
- Produces: `createMobileRuntimeRemote(request)` with typed `passwordLogin`, `me`, `oidcConfig`, `oidcUrl`, `oidcExchange`, `refresh`, and `deploymentCapabilities` methods.

- [ ] **Step 1: Write failing adapter tests** for the authenticated capability path/envelope and for rejecting an invalid or non-HTTPS origin before a request is sent.
- [ ] **Step 2: Run `pnpm exec tsx --test packages/api-client/src/mobile/runtime.test.ts`**; expect missing export/function failures.
- [ ] **Step 3: Implement `createMobileRuntimeRemote`** by delegating auth calls to `createAuthApi` and issuing `GET /api/v1/system/capabilities` with the same client transport and bearer Authorization header. Return raw capability payload only to Mobile Runtime; do not make the adapter decide the safe surface.
- [ ] **Step 4: Run the adapter tests and existing auth suite** with `pnpm exec tsx --test packages/api-client/src/mobile/runtime.test.ts packages/api-client/src/auth/endpoints.test.ts`; expect all assertions to pass and invalid origins to produce zero captured requests.
- [ ] **Step 5: Add the `./mobile/runtime` export** and run `pnpm typecheck:shared`; expect no unresolved package paths.
- [ ] **Step 6: Commit** as `feat(api-client): expose mobile runtime auth and capability adapter`.

### Task 3: Implement Mobile Runtime Interface and fail-closed scope gate

**Files:**
- Create: `packages/mobile-core/package.json`, `packages/mobile-core/src/index.ts`, `packages/mobile-core/src/runtime/types.ts`, `packages/mobile-core/src/runtime/ports.ts`, `packages/mobile-core/src/runtime/mobile-runtime.ts`, `packages/mobile-core/src/runtime/in-memory-adapters.ts`, `packages/mobile-core/src/runtime/mobile-runtime.test.ts`
- Modify: `pnpm-workspace.yaml`, `pnpm-lock.yaml`
- Test: `packages/mobile-core/src/runtime/mobile-runtime.test.ts`

**Interfaces:**
- Consumes: `clientGate` and `ClientGateVerdict` from `@weknora/domain/mobile`; Runtime remote Port created in Task 2.
- Produces: `createMobileRuntime(ports): MobileRuntime` with the public contract above; only this Module may mint/revoke `ScopeLease`.

- [ ] **Step 1: Write Interface tests** for boot restore ordering, successful login→me→capabilities ordering, authorized surface only for mode `full`, unknown/malformed capability staying on `upgrade-required`, missing active Tenant staying safe, deployment changes invalidating the prior lease, and late responses ignored after sign-out.
- [ ] **Step 2: Run `pnpm exec tsx --test packages/mobile-core/src/runtime/mobile-runtime.test.ts`**; expect import/type failures because the package does not exist.
- [ ] **Step 3: Implement the smallest Runtime state machine** with injected Identity, Capability, CredentialStore, OIDC state and lifecycle Ports. `signIn` must save credentials scoped to normalized Deployment, call `/auth/me`, then capabilities, and mint a lease only after the gate passes and Tenant identity is valid. Use a monotonically increasing private epoch to reject late results; no epoch escapes the Interface.
- [ ] **Step 4: Run the Interface tests**; expect safe-state, ordering, lease revocation and late-response assertions to pass.
- [ ] **Step 5: Run the relevant existing policy suites** with `pnpm exec tsx --test packages/domain/src/mobile/compatibility.test.ts packages/domain/src/mobile/auth-return.test.ts`; expect them still green.
- [ ] **Step 6: Commit** as `feat(mobile-core): add fail-closed Mobile Runtime`.

### Task 4: Persist OIDC state and PKCE across process restart

**Files:**
- Create: `apps/mobile/src/adapters/secure-store.ts`, `apps/mobile/src/adapters/oidc-browser.ts`, `apps/mobile/src/app/auth-return.tsx`, `apps/mobile/src/adapters/oidc-adapters.test.ts`
- Modify: `packages/mobile-core/src/runtime/ports.ts`, `packages/mobile-core/src/runtime/mobile-runtime.ts`, `packages/mobile-core/src/runtime/mobile-runtime.test.ts`
- Test: `apps/mobile/src/adapters/oidc-adapters.test.ts`, `packages/mobile-core/src/runtime/mobile-runtime.test.ts`

**Interfaces:**
- Consumes: `PendingOidcStore` and `OidcBrowserPort` from Runtime Ports; existing OIDC start/exchange helpers in api-client and `consumeAuthReturn` policy in `@weknora/domain/mobile`.
- Produces: native secure store adapter with `savePending`, `loadPending`, and one-time `consumePending`; browser adapter opens the deployment authorization URL and returns only the registered callback URL.

- [ ] **Step 1: Write failing tests** for persisted state/verifier restore, wrong redirect rejection, one-time state consumption and callback replay rejection.
- [ ] **Step 2: Run the adapter and Runtime tests**; expect absent native adapter exports or verifier not restored after constructing a fresh Runtime.
- [ ] **Step 3: Implement secure pending-auth persistence and browser callback**. Generate state and PKCE verifier/challenge with platform cryptographic randomness; store state, verifier, redirect URI and deployment origin in SecureStore before opening the browser. Callback validation uses the exact registered scheme/path and existing URL/state policy; exchange consumes the saved verifier once.
- [ ] **Step 4: Run tests under a two-Runtime restart scenario**; expect Runtime A's pending OIDC record to be consumed successfully by Runtime B, then rejected on replay.
- [ ] **Step 5: Commit** as `feat(mobile): persist OIDC callback state securely`.

### Task 5: Compose login, upgrade and authorized landing screens

**Files:**
- Create: `apps/mobile/src/composition.ts`, `apps/mobile/src/screens/DeploymentLoginScreen.tsx`, `apps/mobile/src/screens/UpgradeRequiredScreen.tsx`, `apps/mobile/src/screens/AuthorizedLandingScreen.tsx`
- Modify: `apps/mobile/src/app/_layout.tsx`, `apps/mobile/src/app/index.tsx`
- Test: `apps/mobile/src/app-smoke.test.tsx`

**Interfaces:**
- Consumes: `MobileRuntime.snapshot/subscribe/signIn/beginOidc/completeOidc/signOut` and `RuntimeSnapshot`.
- Produces: Root navigation switches exclusively on `RuntimeSnapshot.surface`; screens receive callbacks and display data, never a transport/client object.

- [ ] **Step 1: Write failing render tests** proving `upgrade-required` cannot render authorized controls, `authorized` requires an identity and active Tenant, and `deployment-login` can switch only among validated HTTPS origins.
- [ ] **Step 2: Run `pnpm --filter @weknora/mobile test`**; expect failures for missing screens/composition.
- [ ] **Step 3: Implement the three screens and composition root**. Official cloud is present only when its explicit build variable is set; self-hosted origins must be HTTPS with no embedded user-info. OIDC callbacks route through Runtime. The authorized landing is a placeholder for the Task Office Module and contains no Task API call.
- [ ] **Step 4: Run app smoke tests and typecheck**; expect no Screen imports from `@weknora/api-client` or `@weknora/contracts` and all route assertions green.
- [ ] **Step 5: Commit** as `feat(mobile): add deployment login and safe landing surfaces`.

### Task 6: Exercise Runtime through a real HTTP authentication flow

**Files:**
- Create: `packages/api-client/src/mobile/runtime.integration.test.ts`, `apps/mobile/src/runtime-integration-smoke.ts`
- Modify: `.github/workflows/mobile.yml` only if a required CI command is missing.
- Test: `packages/api-client/src/mobile/runtime.integration.test.ts`

**Interfaces:**
- Consumes: concrete WeKnora remote adapter and Mobile Runtime; real `httptest` Go auth/capability endpoints are already covered by `internal/handler/auth_oidc_mobile_test.go`, `auth_mobile_exchange_test.go`, and `internal/router/deployment_capabilities_protocol_test.go`.
- Produces: opt-in integration command that accepts a test Deployment URL and short-lived test credentials from environment and proves real login→identity→capability→authorized Runtime surface; credentials are never logged or committed.

- [ ] **Step 1: Write the failing integration assertion** for a real configured Deployment producing a stable user/Tenant identity and an authorized Runtime surface.
- [ ] **Step 2: Run it without environment credentials**; expect an explicit skip message, not mock success. Run it with the local integration fixture and expect initial failure until transport wiring is complete.
- [ ] **Step 3: Implement the opt-in runner** using the production transport and adapter. It must redact token fields and include deployment origin, client protocol, capability mode and outcome in a machine-readable evidence record.
- [ ] **Step 4: Run the integration test against the local WeKnora server fixture** and the focused Go tests; expect `authorized` only with successful `/auth/me` and compatible capability window.
- [ ] **Step 5: Document the environment-only command and evidence fields** beside the test, then commit as `test(mobile): verify login and capability flow over HTTP`.

### Task 7: Run native build gates and record the real-deployment check

**Files:**
- Modify: root `package.json`, `.github/workflows/mobile.yml` if the current path assumptions need correction.
- Test: Expo iOS/Android exports, `pnpm test:mobile`, `pnpm typecheck:mobile`, native real-deployment smoke evidence.

**Interfaces:**
- Consumes: Tasks 1–6 app and Runtime surfaces.
- Produces: green CI checks for app tests/typecheck/platform exports plus an operator-readable checklist for installed-device login against a real WeKnora Deployment. Device acceptance remains independently recorded; a JS export is not substituted for it.

- [ ] **Step 1: Run `pnpm test:mobile` and confirm it fails before wiring** if the root script still points to missing package scripts.
- [ ] **Step 2: Add or correct root scripts and mobile workflow commands** to run app tests, typecheck and both Expo exports with the committed lockfile.
- [ ] **Step 3: Run `pnpm test:mobile`, `pnpm typecheck:mobile`, both `expo export` commands, and `node scripts/check-react-boundaries.mjs`**; expect all pass.
- [ ] **Step 4: Install the iOS and Android development builds, point each at an authorized staging Deployment, and record successful password or OIDC login and protocol-gate behavior without secrets**; record the evidence path and any unavailable platform check explicitly.
- [ ] **Step 5: Commit** as `test(mobile): gate native runtime login builds`.

## Self-Review

1. **Spec coverage:** Deployment choice/auth/OIDC/capability gate/scope ownership are Tasks 1–5; real HTTP evidence is Task 6; native build and real-deployment smoke evidence is Task 7. Task lists, durable Task work, vault encryption, device registration and notifications remain with dependent Tickets.
2. **Placeholder scan:** No `TBD`, TODO, generic “add validation” or unspecified test steps are present. SDK 55's React Native and React versions are taken from the official Expo reference and will be locked with the native package dependencies.
3. **Type consistency:** Task 2 produces the remote Port consumed by Task 3; Task 3 defines the exact Runtime Interface consumed by Tasks 4–7. Scope Lease is minted only after validated identity and compatible capabilities.
4. **Review Focus:** malformed capability, cross-origin credential reuse, OIDC replay/restart, late response and missing Tenant each have named owning tests in Tasks 2–5.

## Handoff

This plan covers the approved Ticket #31 only. After plan approval, execute it with `superpowers:subagent-driven-development`; each Task gets a fresh implementer and task review, and the whole ticket gets a final review. Real installed-device evidence requiring deployment credentials/device access must be collected from the authorized staging environment during execution.
