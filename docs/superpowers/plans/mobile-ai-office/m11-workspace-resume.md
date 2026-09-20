# M11 Workspace Resume Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Track checkboxes as work proceeds.

**Goal:** Resume an idle Task workspace only when its configured sandbox provider proves persistent-volume and snapshot recovery behavior through a real provider gate.

**Architecture:** Add a provider capability probe and durable resume state beside the M09 binding, using existing `RemoteSandboxClient` capabilities and `SessionSandboxBinding`. A failed, unsupported, or ambiguous provider operation leaves the workspace unavailable/blocked; no UI may promise resumed files until the integration gate records real evidence.

**Tech Stack:** Go sandbox adapters/service/Gin, Expo TypeScript/Jest, provider integration tests.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; design `docs/superpowers/specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- M09 is required; file CRUD and automatic retries remain out of scope.
- Only a provider that verifies persistent volume + snapshot/restore may advertise resumable; fake tests cannot satisfy the release gate.
- Unknown provider side effects remain `waiting_user`/blocked and are never automatically recreated.
- Controller owns the final migration number and common DI/router integration; this track declares its schema contract and owns its feature files.

## Review Focus

- A provider that reports `SupportsReconnect` but no persistent restore capability must remain non-resumable.
- Connection timeout after a resume request must not create a replacement sandbox.
- Resume must revalidate tenant/owner binding and generation before exposing a workspace.
- A completed provider probe must be tied to provider/template/runtime digest, not a global boolean.
- Mobile must render blocked state without offering a retry that duplicates an unknown provider operation.

---

### Task 1: Provider-proven workspace hibernation/resume vertical slice

**Files:**
- Create: `internal/application/service/mobileworkspace/resume.go`
- Create: `internal/application/service/mobileworkspace/resume_test.go`
- Create: `internal/application/service/mobileworkspace/resume_provider_integration_test.go`
- Create: `migrations/versioned/<controller-assigned>_mobile_workspace_resume.up.sql` and `.down.sql`; matching `migrations/sqlite/<controller-assigned>_mobile_workspace_resume.up.sql` and `.down.sql`.
- Create: `internal/handler/session/mobile_workspace_resume.go`
- Create: `internal/handler/session/mobile_workspace_resume_test.go`
- Create: `apps/mobile-next/src/features/workspace-resume/WorkspaceResumeClient.ts`
- Create: `apps/mobile-next/tests/features/workspace-resume/WorkspaceResumeClient.test.ts`
- Create: `apps/mobile-next/src/features/workspace-resume/WorkspaceResumeState.ts`
- Create: `apps/mobile-next/tests/features/workspace-resume/WorkspaceResumeState.test.ts`

**Interfaces:**
- Consumes: M09 `Binding`, `sandbox.RemoteSandboxClient`, `RemoteSandboxCapabilities`, `RemoteConnectRequest`, and `SessionSandboxBindingStore` lifecycle lock.
- Durable contract: `mobile_workspace_resume(tenant_id,session_id primary key,workspace_id,generation,runtime_digest,provider,template_id,state,request_id unique,revision,last_probe_at,last_error,updated_at)`; `WHERE revision=?` CAS transition; unknown restore persists `waiting_user` then reconciles only the persisted provider binding, never Create.
- Produces: **proposed** `mobileworkspace.ResumeState = "active"|"hibernated"|"restoring"|"waiting_user"|"unsupported"`; `(*ResumeService).Resume(ctx context.Context, scope craft.Scope, requestID string, expectedRevision int64) (ResumeView,error)`, `restoreExisting(ctx, binding sandbox.SessionSandboxBinding) (sandbox.RemoteSandboxHandle,error)`, and `ReconcileUnknown(ctx,scope craft.Scope,requestID string) (ResumeView,error)`; **proposed** `POST /sessions/:session_id/mobile-workspace/resume {request_id,expected_revision}`; **proposed** `WorkspaceResumeClient.resume(input)`.

- [ ] **Step 1: Write the RED tests, including an explicit integration gate.**

```go
func TestResumeUnknownConnectDoesNotProvisionReplacement(t *testing.T) {
  svc, remote := resumeHarness(t, sandbox.RemoteErrorKindUnknown)
  got, err := svc.Resume(context.Background(), ownerScope("s1"), "resume-1", 3)
  require.ErrorIs(t, err, craft.ErrConflict)
  require.Equal(t, mobileworkspace.ResumeWaitingUser, got.State)
  require.Zero(t, remote.CreateCalls())
}
func TestProviderResumeIntegrationPersistsMarkerAcrossPause(t *testing.T) {
  require.NotEmpty(t, os.Getenv("WEKNORA_SANDBOX_PROVIDER"))
  require.NotEmpty(t, os.Getenv("WEKNORA_SANDBOX_TEMPLATE"))
  env := newE2BCompatibleIntegrationHarness(t, os.Getenv("WEKNORA_SANDBOX_PROVIDER"), os.Getenv("WEKNORA_SANDBOX_TEMPLATE"))
  binding := env.CreateBoundSession(t, sandbox.RemoteCreateRequest{VolumeMounts: env.PersistentMounts(), Timeout: sandbox.RemoteTimeoutPolicy{Action:sandbox.RemoteOnTimeoutPause, AutoResume:true}})
  require.NoError(t, env.Files.WriteSessionWorkspaceFile(context.Background(), "s1", "marker.txt", []byte("persisted")))
  require.NoError(t, env.PauseOrSnapshot(context.Background(), binding.SandboxID))
  _, err := env.Client.Connect(context.Background(), sandbox.RemoteConnectRequest{SandboxID:binding.SandboxID, TrafficAccessToken:binding.TrafficAccessToken}); require.NoError(t, err)
  got, err := env.Files.ReadSessionFile(context.Background(), "s1", "marker.txt"); require.NoError(t, err); require.Equal(t, []byte("persisted"), got)
}
```

- [ ] **Step 2: Run RED and classify the provider gate.**

Run: `go test ./internal/application/service/mobileworkspace ./internal/handler/session -run 'TestResumeUnknownConnectDoesNotProvisionReplacement' -count=1`
Run: `WEKNORA_SANDBOX_PROVIDER=e2b WEKNORA_SANDBOX_TEMPLATE=<real-template-id> go test -tags=integration ./internal/application/service/mobileworkspace -run TestProviderResumeIntegrationPersistsMarkerAcrossPause -count=1`
Expected: unit test fails until implementation; integration test is an explicit BLOCKED gate without real provider credentials/template, never a pass-by-skip claim.

- [ ] **Step 3: Write mobile RED tests.**

```ts
it("keeps unknown resume blocked and does not issue a second command", async () => {
  const state = new WorkspaceResumeState(clientReturning("waiting_user"));
  await state.resume("s1", 3, scope());
  await state.resume("s1", 3, scope());
  expect(client.calls).toHaveLength(1);
  expect(state.view).toMatchObject({ state: "waiting_user", canRetry: false });
});
```

- [ ] **Step 4: Implement capability-gated state transitions.**

```go
func (s *ResumeService) Resume(ctx context.Context, scope craft.Scope, requestID string, expected int64) (ResumeView, error) {
  var view ResumeView
  err := s.lock.WithLifecycleLock(ctx, bindingKey(scope), func(ctx context.Context) error {
    if !s.probes.Verified(scope, s.runtimeDigest) { return ErrResumeUnsupported }
    binding, err := s.bindings.Get(ctx, bindingKey(scope)); if err != nil { return err }
    handle, err := s.restoreExisting(ctx, binding)
    if sandbox.IsUnknown(err) { view, err = s.persistWaitingUser(ctx, scope, requestID, expected); return err }
    if err != nil { return err }; view, err = s.publishRestored(ctx, scope, handle, expected); return err
  })
  return view, err
}
```

```ts
if (view.state === "waiting_user" || view.state === "unsupported") return { ...view, canRetry: false };
return { ...view, canRetry: view.state === "hibernated" };
```

- [ ] **Step 5: Run GREEN and review gates.**

Run: `go test ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(Resume|MobileWorkspaceResume)' -count=1`
Run: `cd apps/mobile-next && npm test -- --runInBand tests/features/workspace-resume && npm run typecheck && npm run check:isolation`
Expected: unsupported/unknown transitions remain blocked; client has no automatic retry.

- [ ] **Step 6: Complete the real-provider demo gate before claiming support.** Run the tagged integration test against the configured provider/template, preserve volume marker evidence and provider/template/runtime digest. Until it passes, mark resume support blocked and keep the user-facing capability disabled.

## Execution Handoff

Implement in a fresh context. Declare desired binding-resume state storage to the controller; do not choose migration numbers, change shared container files, or convert a skipped provider test into acceptance evidence.
