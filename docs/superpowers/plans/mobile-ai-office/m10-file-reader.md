# M10 File Reader Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Track checkboxes as work proceeds.

**Goal:** Let an authorized Task reader list, read, and download bounded workspace files without receiving any write capability.

**Architecture:** Extend the M09 binding boundary with a read-only file service backed by the existing `sandbox.SessionFileStore`/Craft workspace. All reads resolve the owner workspace after session ACL evaluation; mobile uses signed/download URLs or bytes returned by the existing API client and never handles a sandbox credential.

**Tech Stack:** Go/Gin, existing sandbox filesystem port, Expo file-system/sharing, TypeScript/Jest.

**Spec:** `docs/superpowers/specs/2026-09-20-mobile-ai-office-spec.md`; design `docs/superpowers/specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- M09 is required; this slice does not add manual mutation, import, reservation, or shared-resource grants.
- Canonical file paths are server-normalized; reject traversal, symlinks, directories-as-files, and oversized reads.
- Read ACL later expands in M15/M16; until then the endpoint is owner-only and must not impersonate a reader.
- Controller integrates common router/container exports serially; this track owns only its unique mobile-next feature and server service/handler files.

## Review Focus

- `../` and encoded traversal must not escape the workspace root.
- Listing a symlink may not disclose its target; download only regular files.
- A file larger than the bounded download limit must return a clear range/too-large result rather than allocating unbounded memory.
- Download failures must not leave a partial visible mobile file advertised as complete.
- A stale workspace generation must require a refreshed binding before file reads.

---

### Task 1: Read-only workspace files vertical slice

**Files:**
- Create: `internal/application/service/mobileworkspace/reader.go`
- Create: `internal/application/service/mobileworkspace/reader_test.go`
- Create: `internal/handler/session/mobile_workspace_files.go`
- Create: `internal/handler/session/mobile_workspace_files_test.go`
- Create: `apps/mobile-next/src/features/file-reader/CloudWorkspaceFilesClient.ts`
- Create: `apps/mobile-next/tests/features/file-reader/CloudWorkspaceFilesClient.test.ts`
- Create: `apps/mobile-next/src/features/file-reader/downloadWorkspaceFile.ts`
- Create: `apps/mobile-next/tests/features/file-reader/downloadWorkspaceFile.test.ts`

**Interfaces:**
- Consumes: M09 `BindingService.GetOrBind`, existing `sandbox.SessionFileStore.ListSessionFiles(ctx context.Context, sessionID, dir string) ([]sandbox.RemoteDirEntry,error)`, `StatSessionFile`, and `ReadSessionFile(ctx context.Context, sessionID,path string) ([]byte,error)`.
- Produces: **proposed** `mobileworkspace.FileEntry{Path string, Bytes int64, MIME string, SHA256 string}`; `(*Reader).List(ctx context.Context, scope craft.Scope) ([]FileEntry,error)`; `(*Reader).Read(ctx context.Context, scope craft.Scope, path string) (FileContent,error)`; **proposed** `GET /sessions/:session_id/mobile-workspace/files` and `/files/*path`; **proposed** `CloudWorkspaceFilesClient.list(sessionId, scope)` and `.download(sessionId,path,scope)`.

- [ ] **Step 1: Write server RED tests.**

```go
func TestReaderRejectsTraversalBeforeSandboxRead(t *testing.T) {
  reader, source := readerHarness(t)
  _, err := reader.Read(context.Background(), ownerScope("s1"), "../secrets.env")
  require.ErrorIs(t, err, craft.ErrInvalidInput)
  require.Empty(t, source.readPaths)
}
func TestMobileWorkspaceFileDownloadRefusesOtherTenant(t *testing.T) {
  res := getFile(t, handlerWithWorkspace("tenant-a", "owner"), tenant("tenant-b"), "s1", "report.txt")
  require.Equal(t, http.StatusNotFound, res.Code)
}
```

- [ ] **Step 2: Run RED.**

Run: `go test ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(ReaderRejectsTraversalBeforeSandboxRead|MobileWorkspaceFileDownloadRefusesOtherTenant)' -count=1`
Expected: FAIL because the reader API is absent.

- [ ] **Step 3: Write mobile RED tests.**

```ts
it("writes the complete verified download before sharing", async () => {
  const fs = fakeFiles({ writeFailsAfter: 4 });
  await expect(downloadWorkspaceFile(client, fs, "s1", "report.pdf", scope())).rejects.toThrow("download incomplete");
  expect(fs.shared).toEqual([]);
});
```

- [ ] **Step 4: Implement bounded read behavior.**

```go
func (r *Reader) Read(ctx context.Context, scope craft.Scope, requested string) (FileContent, error) {
  path, err := workspaceRelativePath(requested); if err != nil { return FileContent{}, err }
  if err := r.authorizeRead(ctx, scope); err != nil { return FileContent{}, err }
  return r.readRegularBounded(ctx, scope.SessionID, path, r.maxBytes)
}
```

```ts
export async function downloadWorkspaceFile(client: CloudWorkspaceFilesClient, fs: FileStore, sessionId: string, path: string, scope: CloudWorkspaceScope) {
  const file = await client.download(sessionId, path, scope);
  await fs.writeAtomic(file.suggestedName, file.bytes, file.sha256);
  return fs.share(file.suggestedName);
}
```

- [ ] **Step 5: Run GREEN gates.**

Run: `go test ./internal/application/service/mobileworkspace ./internal/handler/session -run 'Test(Reader|MobileWorkspaceFile)' -count=1`
Run: `cd apps/mobile-next && npm test -- --runInBand tests/features/file-reader && npm run typecheck && npm run check:isolation`
Expected: safe list/read/download, tenant isolation, traversal denial, and no write endpoint pass.

- [ ] **Step 6: Review and demo gate.** Use a seeded owner workspace containing one text file, one symlink, and one oversized file: the regular file lists/downloads, the other two do not disclose contents. This is mocked/contract evidence, not device storage evidence.

## Execution Handoff

One fresh context owns the listed files. M14 consumes these normalized read contracts; do not pre-create its mutation code or shared ACL policy.
