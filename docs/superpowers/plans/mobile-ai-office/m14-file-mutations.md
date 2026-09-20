# M14 File Mutations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Track checkboxes as work proceeds.

**Goal:** Permit an owner to manually write, delete, or explicitly import workspace files only after quota and Run/PTY admission succeed.

**Architecture:** Build on M10’s normalized reader, M12 reservations, and M13’s writer lease. Each mutation has an idempotency key and expected revision; bytes are staged, reserved, written, and committed atomically enough to preserve a recoverable reservation. Import reads an authorized source Task then writes a new target copy; it never creates a shared writable directory.

**Tech Stack:** Go/Gin/GORM/sandbox filesystem, Expo document picker/file-system, TypeScript/Jest.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; design `docs/superpowers/specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- Dependencies are M10, M12, and M13; no mutation begins until all three contracts are available.
- Owner-only write checks, storage reservation, and stopped/PTY writer admission run server-side for every command.
- Import is an explicit copy with source-read and target-write authorization; Task workspaces remain isolated.
- Controller handles shared routes, migration numbering, root layout, and index exports in serial integration.

## Review Focus

- A write that exceeds quota performs zero sandbox writes and leaves no held reservation.
- A running or blocked-unknown Run prevents write, delete, and import—even if the mobile UI is stale.
- Replayed request ID with changed bytes/path conflicts rather than silently overwriting.
- Import from another tenant or an unreadable source Task discloses neither filename nor bytes.
- Delete releases usage only after the remote delete result is confirmed; unknown result remains recoverable/blocked.

---

### Task 1: Guarded file write/delete/import vertical slice

**Files:**
- Create: `internal/application/service/mobileworkspace/mutations.go`
- Create: `internal/application/service/mobileworkspace/mutations_test.go`
- Create: `internal/handler/session/mobile_workspace_mutations.go`
- Create: `internal/handler/session/mobile_workspace_mutations_test.go`
- Create: `apps/mobile-next/src/features/file-mutations/WorkspaceMutationClient.ts`
- Create: `apps/mobile-next/tests/features/file-mutations/WorkspaceMutationClient.test.ts`
- Create: `apps/mobile-next/src/features/file-mutations/WorkspaceFileMutations.ts`
- Create: `apps/mobile-next/tests/features/file-mutations/WorkspaceFileMutations.test.ts`

**Interfaces:**
- Consumes: M10 `Reader.Read`, M12 `Reserve/Commit/Release`, M13 `TerminalAdmission.BeforeManualWrite(ctx,scope,requestID,expectedRevision) error` (it does not open or lease a PTY), `sandbox.SessionFileStore.WriteSessionWorkspaceFile(ctx,sessionID,filePath string,content []byte) error`, and `craft.Scope`. **Proposed** `RemoveSessionWorkspaceFile` must be added to `SessionFileStore` and provider implementations before DeleteFile is exposed; no generic `files.Write` exists.
- Produces: **proposed** `WriteFile(ctx,scope,path string,body []byte,requestID string,expectedRevision int64) (FileEntry,error)`, `DeleteFile(...) error`, `ImportFile(ctx, target craft.Scope, sourceSessionID, sourcePath, targetPath, requestID string, expectedRevision int64) (FileEntry,error)`; **proposed** `POST/DELETE /sessions/:session_id/mobile-workspace/files` and `POST .../files/import`.

- [ ] **Step 1: Write server RED tests.**

```go
func TestWriteOverQuotaDoesNotTouchSandbox(t *testing.T) {
  svc, remote := mutationHarness(t, quotaAtLimit(), stoppedWriter())
  _, err := svc.WriteFile(context.Background(), ownerScope("target"), "notes.txt", []byte("x"), "req-1", 2)
  require.ErrorIs(t, err, mobileworkspace.ErrQuotaExceeded); require.Empty(t, remote.writes)
}
func TestImportRequiresSourceReadAndTargetWrite(t *testing.T) {
  _, err := mutationHarness(t, quotaAvailable(), stoppedWriter()).ImportFile(ctx, ownerScope("target"), "source", "a.txt", "copied.txt", "req-2", 2)
  require.ErrorIs(t, err, craft.ErrForbidden)
}
```

- [ ] **Step 2: Run RED.**

Run: `go test ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(WriteOverQuotaDoesNotTouchSandbox|ImportRequiresSourceReadAndTargetWrite)' -count=1`
Expected: FAIL because mutation commands are absent.

- [ ] **Step 3: Write mobile RED tests.**

```ts
it("does not submit a picked document while server writer state is blocked", async () => {
  const view = new WorkspaceFileMutations(clientReturning({ writer_state: "blocked_unknown" }));
  await view.importPickedFile(fakePicked("a.txt", "a"));
  expect(view.client.mutationCalls).toBe(0); expect(view.error).toContain("停止状态未知");
});
```

- [ ] **Step 4: Implement ordered admission and compensation.**

```go
func (s *Mutations) WriteFile(ctx context.Context, scope craft.Scope, path string, body []byte, id string, rev int64) (FileEntry,error) {
  if err := s.admission.BeforeManualWrite(ctx, scope, id, rev); err != nil { return FileEntry{}, err }
  r, err := s.quota.Reserve(ctx, scope, int64(len(body)), id); if err != nil { return FileEntry{}, err }
  if err := s.files.WriteSessionWorkspaceFile(ctx, scope.SessionID, path, body); err != nil { _ = s.quota.Release(ctx,r.ID); return FileEntry{},err }
  if err := s.quota.Commit(ctx,r.ID); err != nil { return FileEntry{},err }; return s.reader.Stat(ctx,scope,path)
}
```

```ts
await client.write({ sessionId, path, bytes, requestId: newRequestId(), expectedRevision });
// refresh file list from M10 after confirmed command; never append optimistic bytes as authoritative.
```

- [ ] **Step 5: Run GREEN gates.**

Run: `go test ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(Write|Delete|Import|MobileWorkspaceMutation)' -count=1`
Run: `cd apps/mobile-next && npm test -- --runInBand tests/features/file-mutations && npm run typecheck && npm run check:isolation`
Expected: quota, stop, replay, source/target authorization, and explicit-copy semantics pass.

- [ ] **Step 6: Review and demo gate.** Demonstrate owner write/delete/import with a stopped Run, then repeat with quota exhausted and an active/unknown Run. Confirm no cross-Task shared write mount exists; record endpoint/contract evidence only.

## Execution Handoff

Use one fresh context and the listed files. Do not add sharing policy or a second sandbox runtime; M15/M16 define reader sharing separately.
