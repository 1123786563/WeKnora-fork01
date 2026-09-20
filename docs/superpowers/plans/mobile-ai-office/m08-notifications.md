# M08 Minimal Notification and Revalidated Deeplink Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Steps use checkbox syntax for tracking.

**Goal:** receive only minimal Run outcome notifications and deep-link to a revalidated task snapshot without leaking sensitive content on the lock screen.

**Architecture:** Extend `DeepLinkController` with an app-owned notification adapter. Notification payload contains event category and opaque Run/Task references only; opening it resolves the current route through `CloudWorkspaceClient` and M05 snapshot query. Permission failure clears local projection and opens a forbidden state.

**Tech Stack:** Expo notifications adapter selected at implementation, Expo Linking, TypeScript, Jest.

**Spec:** `../../specs/2026-09-20-mobile-ai-office-spec.md`; `../../specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- Lock-screen content may say only completed, failed, or awaiting approval; no task title or body.
- Notification open rechecks current authorization before displaying Task/Run data.
- Payload routes only to current scope; stale scope/generation is discarded.
- Notifications do not grant access, submit commands, or resolve approvals.
- Fixture validation and actual iOS/Android push/lock-screen acceptance are separate.

## Review Focus

- A notification title/body serializer cannot include a task title even when supplied one.
- A revoked notification target opens forbidden and clears stale cache.
- An old tenant notification cannot navigate the new tenant session.
- A malformed opaque payload opens a safe inbox/error state without network mutation.
- Opening a pending-approval notification displays it but makes no decision.

---

### Task 1: Define minimal notification envelope and presentation

**Files:**
- Create: `apps/mobile-next/src/features/notifications/RunNotification.ts`
- Create: `apps/mobile-next/src/platform/notifications.ts`
- Test: `apps/mobile-next/tests/features/notifications/presentation.test.ts`

**Interfaces:**
- Consumes: `CloudWorkspaceScope` and server-supplied event reference.
- Produces: `RunNotification = { kind: "completed" | "failed" | "waiting_approval"; sessionId: string; runId: string; scope: Pick<CloudWorkspaceScope, "backend" | "accountId" | "tenantId"> }`; `toLockScreen(notification: RunNotification): { title: string; body: string; data: Record<string, string> }`; `schedule(notification: RunNotification): Promise<void>`.
- New invariant: `data` has opaque IDs/scope identifiers only; no content, title, prompt, artifact or approval summary.

- [ ] **Step 1: Write the failing lock-screen privacy test.**

```ts
it("emits a minimal awaiting-approval notification without supplied task text", () => {
  const output = toLockScreen({ ...notice, kind: "waiting_approval", taskTitle: "收购方案正文" } as unknown as RunNotification);
  expect(output.title).toBe("需要处理审批");
  expect(output.body).toBe("");
  expect(JSON.stringify(output)).not.toContain("收购方案正文");
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/features/notifications/presentation.test.ts`

Expected: FAIL because envelope and serializer are absent.

- [ ] **Step 3: Implement whitelist-only presentation.**

```ts
const copy = { completed: "任务已完成", failed: "任务失败", waiting_approval: "需要处理审批" } as const;
export const toLockScreen = (n: RunNotification) => ({ title: copy[n.kind], body: "", data: { runId: n.runId, sessionId: n.sessionId, tenantId: n.scope.tenantId } });
```

Reject unrecognized kinds. Do not accept arbitrary server text as notification display text.

- [ ] **Step 4: Run GREEN.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/features/notifications/presentation.test.ts && npm --prefix apps/mobile-next run typecheck`

Expected: PASS for all three categories and privacy assertion.

### Task 2: Revalidate notification deeplink before routing

**Files:**
- Modify: `apps/mobile-next/src/features/notifications/deeplink/DeepLinkController.ts`
- Create: `apps/mobile-next/src/features/notifications/NotificationOpenController.ts`
- Test: `apps/mobile-next/tests/features/notifications/open.test.ts`
- Modify (controller integration only): root notification listener and router registration.

**Interfaces:**
- Consumes: `RunNotification`, `CloudWorkspaceClient`, M05 `RunRepository.snapshot`, existing `DeepLinkController`.
- Produces: `NotificationOpenController.open(payload: unknown): Promise<{ route: string; allowed: boolean }>`; `parseRunNotification(payload: unknown): RunNotification | null`.
- New route contract: `tasks/[sessionId]?runId=[runId]` is entered only after `snapshot(runId)` returns matching current scope/session.

- [ ] **Step 1: Write the failing revocation/open test.**

```ts
it("clears stale cache and routes to forbidden when snapshot authorization is revoked", async () => {
  repo.snapshot.mockRejectedValue(Object.assign(new Error("forbidden"), { status: 403 }));
  await expect(controller.open(payload)).resolves.toEqual({ route: "/forbidden", allowed: false });
  expect(cache.clear).toHaveBeenCalledWith(scope);
  expect(router.push).toHaveBeenCalledWith("/forbidden");
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/features/notifications/open.test.ts`

Expected: FAIL until open controller validates payload and snapshot.

- [ ] **Step 3: Implement parse, scope check, snapshot revalidation and safe route.**

```ts
const n = parseRunNotification(payload); if (!n || !sameNotificationScope(n.scope, client.scope)) return { route: "/inbox", allowed: false };
try { const snap = await runs.snapshot(n.runId); return snap.sessionId === n.sessionId ? goTask(n) : forbidden(); }
catch (e) { if (statusOf(e) === 403) await cache.clear(client.scope); return forbidden(); }
```

Never resolve an approval while opening; M06 owns explicit decisions.

- [ ] **Step 4: Run GREEN and existing deeplink regression.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/features/notifications/open.test.ts tests/features/deeplink.test.ts && npm --prefix apps/mobile-next run check:isolation`

Expected: PASS for malformed, wrong-scope, authorized and revoked payloads.

- [ ] **Step 5: Review/demo/evidence gate.**

Demo fixture foreground/background opens and privacy serializer. Require real iOS and Android notification permission, lock screen, cold start, live cloud authorization and real sandbox Run evidence before acceptance.

