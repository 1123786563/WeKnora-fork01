# M23 Home Real API Aggregation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`.

**Goal:** Show Home from real Agent, Task/Run, approval, and publication data for the current scope.

**Architecture:** M04, M06, and M17 remain sources of truth. A read-only server aggregator or mobile composition consumes their actual models with scope/generation, and mobile links into each existing feature; it creates no synthetic push inbox. Controller serially owns shared assembly.

**Tech Stack:** Go/Gin, existing Workbench overview/read handlers, Expo/Jest.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; Parent #3.

## Global Constraints

- Blocked by M04, M06, M17. Scope backend/accountId/tenantId/generation; Task=sessionId.
- Read only current tenant and authenticated account visibility. Do not fabricate data for unavailable APIs.
- Home is an entry point, not a second Task state machine or a Git-push inbox.
- Controller owns router/container/exports/lock/root layout/migration.

## Review Focus

1. Scope switch discards old aggregate response.
2. Viewer sees only M15/M17-authorized records.
3. Empty/failed source shows explicit unavailable/empty state.
4. Pending approval opens M06 exact interaction.
5. Publication link uses immutable M17 identifier.

### Task M23: scope-safe Home aggregation

**Files:**
- Create: `internal/handler/session/workbench_home.go`, `internal/handler/session/workbench_home_test.go`.
- Create: `apps/mobile-next/src/features/home/home.ts`, `apps/mobile-next/tests/features/home.test.tsx`.
- Controller only: route/DI/export/root/lock changes.

**Interfaces:**
- Consumes: M04 agents/knowledge query, M06 durable approval list, M17 `PublicationView`.
- Produces (proposed): `GET /workbench/home` -> `HomeView{Scope,Generation,Tasks,PendingApprovals,Publications,Agents}` and `loadHome(scope: Scope): Promise<HomeView>`.

- [ ] **Step 1: RED.**

```ts
it("drops a late Home response from an old tenant generation", async () => {
  const { result } = renderHook(() => useHome()); switchScope("tenant-b"); resolveOldTenant();
  expect(result.current.scope.tenantId).toBe("tenant-b"); expect(result.current.tasks).not.toContainEqual(oldTask);
});
```

- [ ] **Step 2: Run RED.** `go test ./internal/handler/session -run 'TestWorkbenchHome' -count=1` and `cd apps/mobile-next && npm test -- --runInBand home.test.tsx`; expect missing aggregation behavior.

- [ ] **Step 3: Implement.** Derive server scope from authentication, aggregate only authorized source views, return source-specific empty/unavailable items, and include snapshot generation. The mobile hook rejects response generation mismatches.

- [ ] **Step 4: Render/link.** Render Tasks, Agents, approvals, and immutable publications from returned IDs only; links navigate to owning M04/M06/M17 screens and no push cards are invented.

- [ ] **Step 5: GREEN/handoff.** Repeat commands for tenant switch/viewer/empty/failed source tests and submit wiring request to controller.

## Review Gate

Reject client-side fake inboxes, cross-tenant cache reuse, or any Home write command.

## Demo and evidence

- [ ] Seed authorized Task, Agent, pending approval, and immutable publication records in one tenant.
- [ ] Capture `HomeView` scope and generation with each returned item identifier.
- [ ] Switch account/tenant before a response resolves and prove generation rejects the late response.
- [ ] Capture reader-visible and owner-visible Home snapshots separately.
- [ ] Force one source unavailable and show its explicit unavailable card with no fabricated content.
- [ ] Follow an approval and publication entry to its owning feature by server identifier.
- [ ] Preserve output as contract evidence; defer real-device proof to M25/M26.

## Controller assembly request

- [ ] Register the read-only Home endpoint with existing Workbench guard and scope derivation.
- [ ] Wire aggregate readers in container after all source interfaces are integrated.
- [ ] Add exports/root navigation only in controller serial pass.
- [ ] Repeat handler and mobile tests after assembly.
