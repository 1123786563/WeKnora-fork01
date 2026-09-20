# M13 Terminal Exclusion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Track checkboxes as work proceeds.

**Goal:** Enforce two-way exclusion between a running Task Run and an interactive workspace PTY.

**Architecture:** Extend the existing Craft lifecycle/stop and sandbox terminal services with a server-owned lease state. Starting a Run first reclaims an existing PTY write lease; opening a PTY or permitting manual writes first issues a CAS stop and waits for a confirmed stopped terminal state. A stop with unknown side effect holds the lease blocked, never releases it.

**Tech Stack:** Go Craft lifecycle/sandbox terminal/Gin/WebSocket, Expo/TypeScript/Jest.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; design `docs/superpowers/specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- M09 is required; all Run state remains the existing session/run authority.
- Active Runs expose terminal observation only; no interactive PTY or manual file write may race them.
- Stop must carry request identity and expected revision; unknown side effects become `waiting_user` and retain the block.
- Controller serially mounts shared routes and persistence wiring; do not edit shared router/container/root layout files in this track.

## Review Focus

- A StartRun racing OpenPTY produces one writer at most, regardless of request order.
- A stale expected revision cannot revoke a newer PTY lease.
- Transport disconnect during stop keeps PTY closed and Run write ownership blocked.
- Read-only observer terminal frames never gain input forwarding.
- A revoked/other-tenant caller cannot stop a Run or attach to its terminal by session ID.

---

### Task 1: Run/PTy exclusion vertical slice

**Files:**
- Create: `internal/application/service/mobileworkspace/terminal_exclusion.go`
- Create: `internal/application/service/mobileworkspace/terminal_exclusion_test.go`
- Create: `internal/handler/session/mobile_workspace_terminal.go`
- Create: `internal/handler/session/mobile_workspace_terminal_test.go`
- Create: `apps/mobile-next/src/features/terminal-exclusion/TerminalAccessClient.ts`
- Create: `apps/mobile-next/tests/features/terminal-exclusion/TerminalAccessClient.test.ts`
- Create: `apps/mobile-next/src/features/terminal-exclusion/TerminalAccessState.ts`
- Create: `apps/mobile-next/tests/features/terminal-exclusion/TerminalAccessState.test.ts`
- Modify: `internal/application/service/craft_session.go:689-740`, `internal/handler/session/sandbox_terminal_bridge.go`, and `internal/handler/session/sandbox_terminal_ws.go:143-218,321-350`.
- Test: `internal/application/service/craft_session_test.go`, `internal/handler/session/sandbox_terminal_bridge_test.go`, and `internal/handler/session/sandbox_terminal_ws_test.go`.

**Interfaces:**
- Consumes: M09 `Binding`, `CraftSessionService.StartRun(ctx,scope,req) (agentruntime.Run,error)`, `service.ErrRemoteStopUnknown`, and terminal ticket/WS handlers.
- Produces: **proposed** `TerminalAdmission.BeforeStartRun(ctx,scope,requestID string,expectedRevision int64) error`, `.BeforePTY(ctx,scope,requestID string,expectedRevision int64) error`, and `.BeforeManualWrite(ctx,scope,requestID string,expectedRevision int64) error`. `BeforeManualWrite` stops/confirms a Run and checks the lease but neither opens a PTY nor claims a PTY lease. Existing StartRun calls the first seam; ticket/WS call the second. `TerminalLeaseState = "observer"|"pty"|"stopping"|"blocked_unknown"`.

- [ ] **Step 1: Write concurrency RED tests.**

```go
func TestStartRunReclaimsExistingPTYBeforeRunWriteLease(t *testing.T) {
  svc, leases := terminalHarness(t, leasePTY("s1", 4))
  _, err := svc.StartRun(context.Background(), ownerScope("s1"), "r1", 4)
  require.NoError(t, err); require.Equal(t, []string{"close-pty", "start-run"}, leases.events)
}
func TestUnknownStopDoesNotOpenPTY(t *testing.T) {
  svc := terminalWithStopResult(t, service.ErrRemoteStopUnknown)
  _, err := svc.OpenPTY(context.Background(), ownerScope("s1"), "p1", 4)
  require.ErrorIs(t, err, craft.ErrConflict); require.Equal(t, TerminalBlockedUnknown, svc.State("s1"))
}
```

- [ ] **Step 2: Run RED.**

Run: `go test ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(StartRunReclaimsExistingPTYBeforeRunWriteLease|UnknownStopDoesNotOpenPTY)' -count=1`
Expected: FAIL because no mobile workspace exclusion service is present.

- [ ] **Step 3: Write mobile RED tests.**

```ts
it("renders active Run terminal as observer-only", () => {
  expect(projectTerminal({ run: "active", lease: "observer" })).toMatchObject({ canSendInput:false, canOpenPty:false, canObserve:true });
});
it("does not retry blocked_unknown stop", async () => {
  await state.openPty(); await state.openPty(); expect(client.stopCalls).toBe(1);
});
```

- [ ] **Step 4: Implement the lease state machine.**

```go
func (s *TerminalExclusion) BeforePTY(ctx context.Context, scope craft.Scope, id string, expected int64) error {
  return s.withSessionCAS(ctx, scope, expected, func() error {
    if s.runs.Active(scope) { if err := s.stopConfirmed(ctx, scope, id, expected); err != nil { return err } }
    return nil // ticket handler claims the PTY only after this admission succeeds
  })
}
func (s *TerminalExclusion) BeforeManualWrite(ctx context.Context, scope craft.Scope, id string, expected int64) error {
  return s.withSessionCAS(ctx, scope, expected, func() error { return s.stopConfirmed(ctx, scope, id, expected) })
}
```

```ts
export const projectTerminal = (s: ServerTerminalState) => s.run === "active" ? { canObserve:true, canSendInput:false, canOpenPty:false } : stateActions(s);
```

- [ ] **Step 5: Run GREEN gates.**

Run: `go test ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(StartRun|UnknownStop|MobileWorkspaceTerminal)' -count=1`
Run: `cd apps/mobile-next && npm test -- --runInBand tests/features/terminal-exclusion && npm run typecheck && npm run check:isolation`
Expected: exactly one writer, no release on unknown stop, and observer-only projection pass.

- [ ] **Step 6: Review and demo gate.** Demonstrate PTY → StartRun (PTY closes first), active Run → PTY request (stop-confirm then opens), and unknown stop (no PTY/open or retry). WebSocket/mock evidence does not replace a real sandbox terminal acceptance run.

## Execution Handoff

One fresh context owns only the listed files and its DI declaration. M14 consumes `OpenPTY`/manual-write admission; do not implement mutation paths here.
