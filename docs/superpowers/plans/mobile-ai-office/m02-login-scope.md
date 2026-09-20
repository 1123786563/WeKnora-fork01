# M02 Trusted Login and Workspace Scope Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Steps use checkbox syntax for tracking.

**Goal:** make authenticated account and workspace scope the trusted prerequisite for every mobile cloud-workspace request.

**Architecture:** Extend the existing `AuthController`, `ScopeCoordinator`, credential store and `WeKnoraApi` rather than replacing them. A new app-owned `CloudWorkspaceClient` receives an immutable scope carrying backend, account, tenant and generation; all feature constructors receive that client, never raw tokens or a workspace ID as authority.

**Tech Stack:** Expo SecureStore, TypeScript, existing `WeKnoraApi`, Jest.

**Spec:** `../../specs/2026-09-20-mobile-ai-office-spec.md`; `../../specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- WeKnora authenticates, authorizes and owns Task, Run, resource and workspace truth.
- Scope is `{ backend, accountId, tenantId, generation }`; a workspace identifier never substitutes tenant or owner authorization.
- Switching tenant or logging out invalidates old streams and cached data before a new scope is visible.
- Credentials must not enter URLs, logs, chat content, artifacts or ordinary persisted state.
- Existing `apps/mobile` is independent; implement only `apps/mobile-next`.

## Review Focus

- A delayed old-tenant response must be ignored after the generation changes.
- A failed server tenant switch must retain the last trusted scope rather than showing a requested scope.
- A tokenless process restart must render login and make no workspace query.
- A forged workspace ID must still be presented with the authenticated tenant scope.
- Logout must clear credentials and every scoped read before navigation exposes login.

---

### Task 1: Define a narrow trusted client factory

**Files:**
- Create: `apps/mobile-next/src/cloud-workspace/CloudWorkspaceClient.ts`
- Test: `apps/mobile-next/tests/cloud-workspace/client-scope.test.ts`
- Modify: `apps/mobile-next/src/domain/scope.ts`

**Interfaces:**
- Consumes: `WeKnoraApi`, `ScopeKey` and `ScopeCoordinator`.
- Produces: `CloudWorkspaceScope = { backend: string; accountId: string; tenantId: string; generation: number }`; `CloudWorkspaceClient(scope: CloudWorkspaceScope, api: WeKnoraApi)`; `assertCurrent(generation: number): void`.
- New function: `toCloudWorkspaceScope(scope: ScopeKey, generation: number): CloudWorkspaceScope`.
- Handoff: M03/M05 must not edit this file. They provide `TaskPort`/`RunPort` constructors; the fixed controller/integrator is the sole writer that composes those ports into `CloudWorkspaceClient`. The shared SDK/backend contract remains authoritative; do not add a second prototype client.

- [ ] **Step 1: Write the failing scope-generation test.**

```ts
it("rejects a delayed old-generation result after a trusted tenant switch", async () => {
  const client = new CloudWorkspaceClient({ backend: "https://api", accountId: "u1", tenantId: "t1", generation: 4 }, api);
  expect(() => client.assertCurrent(5)).toThrow("stale cloud workspace scope");
  expect(client.scope).toEqual({ backend: "https://api", accountId: "u1", tenantId: "t1", generation: 4 });
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/cloud-workspace/client-scope.test.ts`

Expected: FAIL because the narrow client is absent.

- [ ] **Step 3: Implement the opaque scope boundary.**

```ts
export class CloudWorkspaceClient {
  constructor(readonly scope: CloudWorkspaceScope, private readonly api: WeKnoraApi) {}
  assertCurrent(generation: number) { if (generation !== this.scope.generation) throw new Error("stale cloud workspace scope"); }
}
```

Do not add an app-side authorization decision or Task state machine.

- [ ] **Step 4: Run GREEN.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/cloud-workspace/client-scope.test.ts && npm --prefix apps/mobile-next run typecheck`

Expected: PASS with a type-safe scope constructor.

### Task 2: Bind login, tenant selection and logout to client lifetime

**Files:**
- Handoff only (do not modify): `apps/mobile-next/src/features/auth/AuthController.ts` — fixed integration owner composes M02 scope publication with M07 cache-clearing callbacks.
- Create: `apps/mobile-next/src/cloud-workspace/CloudWorkspaceProvider.tsx`
- Test: `apps/mobile-next/tests/cloud-workspace/provider-auth.test.tsx`

**Interfaces:**
- Consumes: `AuthController` stages, `ScopeCoordinator`, `CloudWorkspaceClient` from Task 1.
- Produces: `CloudWorkspaceProvider(props: { children: React.ReactNode }): React.ReactElement`; `useCloudWorkspaceClient(): CloudWorkspaceClient | null`.
- New callback handoff: `onTrustedScope(scope: CloudWorkspaceScope | null): void` passed from auth composition, not raw request headers. Fixed integration owner adds it to `AuthDeps` and calls it for provisional invalidation, rollback/success publication and logout.

- [ ] **Step 1: Write the failing authentication lifecycle test.**

```tsx
it("drops t1 client before publishing t2 after a successful server switch", async () => {
  const scopes: Array<string | null> = [];
  const unsubscribe = subscribeCloudWorkspace((c) => scopes.push(c?.scope.tenantId ?? null));
  await auth.switchSpace("t2");
  expect(scopes).toContain(null);
  expect(scopes.at(-1)).toBe("t2");
  unsubscribe();
});
```

The fixture defines `subscribeCloudWorkspace` as a `CloudWorkspaceProvider` test harness: it receives the provider `onTrustedScope` callback, records every emitted `CloudWorkspaceClient | null`, and returns an unsubscribe function. Reuse `tests/setup.ts` only for React Native/Jest setup; it does not supply this application fixture.

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/cloud-workspace/provider-auth.test.tsx`

Expected: FAIL until auth composition publishes client lifetime events.

- [ ] **Step 3: Hand off provisional invalidation and exact rollback to fixed integration.**

```ts
const previous = this.d.scope.scope!;
const next = this.d.scope.switchTenant(tenantId);
onTrustedScope(null);
try { await api.switchTenant(tenantId, { signal: this.d.scope.signal ?? undefined }); }
catch (error) { this.d.scope.enter(previous); onTrustedScope(toCloudWorkspaceScope(previous, this.d.scope.generation!.value)); throw error; }
this.d.scope.reEnableWrites();
onTrustedScope(toCloudWorkspaceScope(next.scope, next.value));
```

The fixed integration owner changes `AuthController.switchSpace`, not a non-existent `switchTenant`, and captures `previous` before `ScopeCoordinator.switchTenant`. Keep the existing forbidden path to membership refresh/pick-space; use the rollback branch for retryable failure so the prior trusted scope is restored with a new generation. On logout publish `null`, invoke M07 cache cleanup, then clear credentials.

- [ ] **Step 4: Run GREEN and existing auth regression.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/cloud-workspace/provider-auth.test.tsx tests/features/auth.test.ts && npm --prefix apps/mobile-next run check:isolation`

Expected: PASS; no existing auth scope isolation regression.

- [ ] **Step 5: Review/demo/evidence gate.**

Demo login, tenant switch and logout with an intercepted stale response in the mobile app. Record contract/Jest evidence separately from a real WeKnora API login and iOS/Android demo. M25/M26 aggregate final live mobile acceptance; M08 owns notifications only.
