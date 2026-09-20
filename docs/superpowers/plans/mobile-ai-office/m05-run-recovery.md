# M05 Create, Continue and Recover Run Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Steps use checkbox syntax for tracking.

**Goal:** start or continue a WeKnora Run from an existing Task, project its authoritative snapshots and SSE events, and recover safely after interruption.

**Architecture:** Build on `WeKnoraApi.startExecution`, `lookupRequest`, `snapshot`, `getExecution`, `RunStreamClient`, and `SubmissionService`. A task is `sessionId`; each command supplies an idempotency key and expected version. Snapshot is authoritative during initial load, cursor expiration and stream gaps; the mobile projection never decides Run transitions.

**Tech Stack:** TypeScript, existing workbench wire decoders/SSE parser, Jest fixtures.

**Spec:** `../../specs/2026-09-20-mobile-ai-office-spec.md`; `../../specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- Task session ID is the sole Task authority; one Task may have many Runs.
- Queries include snapshot, scope, sequence and cursor; SSE deduplicates scope/sequence/cursor.
- Commands carry a request id and expected version; conflict returns current snapshot.
- Cursor expiration/gap performs full snapshot recovery, not guessed events.
- Unknown side effect status remains `waiting_user` until reconciliation; never auto-retry.

## Review Focus

- A POST timeout followed by lookup must show the accepted Run exactly once.
- Duplicate/out-of-order SSE frames cannot regress the current snapshot.
- Cursor-expired recovery replaces stale projection and resumes only after its watermark.
- A different scope/generation must close the stream before it can paint data.
- Continuing a task makes a new Run while retaining the same session ID.

---

### Task 1: Specify the cloud Run contract and snapshot projection

**Files:**
- Create: `apps/mobile-next/src/cloud-workspace/runs/RunRepository.ts`
- Create: `apps/mobile-next/src/cloud-workspace/runs/types.ts`
- Test: `apps/mobile-next/tests/cloud-workspace/runs/repository.test.ts`
- Handoff only (do not modify): `apps/mobile-next/src/contracts/workbench.ts` — fixed integration owner applies this track's `RunSnapshotWire` decoder/type additions after resolving M06's approval additions.

**Interfaces:**
- Consumes: `CloudWorkspaceClient`, `ResourceSelection`, existing `StartExecutionInput`, `SnapshotWire`, `RunStreamClient`.
- Produces: `RunSnapshot = { runId: string; sessionId: string; status: string; expectedVersion: number; sequence: number; cursor: string | null; scope: CloudWorkspaceScope }`; `RunRequest = { requestId: string; sessionId: string; agentId: string; knowledgeBaseIds: readonly string[]; attachmentIds: readonly string[]; text: string; expectedVersion: number }`; `start(input: RunRequest): Promise<RunSnapshot>`; `recover(requestId: string): Promise<RunSnapshot | null>`.
- Produces for fixed integration: `RunPort = { start(input: RunRequest): Promise<RunSnapshot>; recover(requestId: string): Promise<RunSnapshot | null>; snapshot(runId: string, cursor?: string): Promise<RunSnapshot> }`; the fixed controller/integrator alone composes it into `CloudWorkspaceClient`. Current `SnapshotWire` is insufficient until expanded.
- Type-change handoff: supply `RunSnapshotWire = { run_id: string; session_id: string; status: string; revision: number; scope: CloudWorkspaceScope; sequence: number; cursor: string | null }` and `decodeRunSnapshotWire(value: unknown): RunSnapshotWire` to the fixed integration owner; this Track never edits the shared contract file.

- [ ] **Step 1: Write the failing accepted-command/lookup fixture test.**

```ts
it("recovers a timed-out idempotent create through request lookup without a second POST", async () => {
  transport.timeoutOnce("POST", "/workbench/executions");
  transport.replyLookup("rq-1", { run_id: "r1", session_id: "s1", status: "queued", revision: 3, scope, sequence: 9, cursor: "c9" });
  await expect(repo.start(request("rq-1"))).rejects.toThrow("request outcome unknown");
  await expect(repo.recover("rq-1")).resolves.toMatchObject({ runId: "r1", sessionId: "s1", expectedVersion: 3 });
  expect(transport.postCount).toBe(1);
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/cloud-workspace/runs/repository.test.ts`

Expected: FAIL because `RunRepository` and extended snapshot contract do not exist.

- [ ] **Step 3: Implement explicit request and snapshot decoding.**

```ts
async recover(requestId: string) {
  const found = await this.client.lookupRunRequest(requestId);
  return found ? this.decodeSnapshot(found) : null;
}
```

Use the existing API only where fields are factual; add endpoint/wire changes through the backend-contract track, never invent missing access or cursor data locally.

- [ ] **Step 4: Run GREEN.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/cloud-workspace/runs/repository.test.ts tests/api/http.test.ts && npm --prefix apps/mobile-next run typecheck`

Expected: PASS and no second POST after a timeout.

### Task 2: Stream, gap recovery and continue UI

**Files:**
- Create: `apps/mobile-next/src/features/runs/RunController.ts`
- Create: `apps/mobile-next/src/features/runs/RunScreen.tsx`
- Test: `apps/mobile-next/tests/features/runs/recovery.test.tsx`
- Modify: `apps/mobile-next/src/features/executions/sse/RunStreamClient.ts`

**Interfaces:**
- Consumes: `RunRepository`, `RunSnapshot`, existing `RunEventAssembler` and `RunStreamClient`.
- Produces: `RunController.open(runId: string): Promise<void>`; `RunController.continue(input: RunRequest): Promise<RunSnapshot>`; `RunController.dispose(): void`; `RunScreen(props: { controller: RunController }): React.ReactElement`.
- New callback: `onCursorExpired(): Promise<void>` performs snapshot replacement before reconnect.

- [ ] **Step 1: Write the failing cursor-gap behavior test.**

```tsx
it("replaces a gapped stream with snapshot watermark before rendering later events", async () => {
  stream.emit({ seq: 1, type: "log", payload: {} }); stream.emit({ seq: 5, type: "log", payload: {} });
  await controller.flush();
  expect(repo.snapshotCalls).toEqual(["r1"]);
  expect(screen.queryByText("seq:5")).toBeNull();
  stream.emit({ seq: 6, type: "log", payload: {} });
  expect(await screen.findByText("seq:6")).toBeTruthy();
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/features/runs/recovery.test.tsx`

Expected: FAIL until controller recovery wiring exists.

- [ ] **Step 3: Implement stream ownership and recovery.**

```ts
private async recoverFromGap() {
  const snapshot = await this.runs.snapshot(this.runId);
  this.state = { snapshot, items: [] };
  this.stream.reconnect({ cursor: snapshot.cursor, startSeq: snapshot.sequence });
}
```

Close/discard on scope-generation change. Render server `waiting_user` verbatim and offer only reconciliation paths, never automatic retry.

- [ ] **Step 4: Run GREEN.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/features/runs/recovery.test.tsx tests/sse/parser.test.ts tests/sse/client.test.ts && npm --prefix apps/mobile-next run check:isolation`

Expected: PASS for fixture SSE, gap recovery and scope disposal.

- [ ] **Step 5: Review/demo/evidence gate.**

Demo create, app close/reopen, request lookup, cursor-expired snapshot recovery, continue and `waiting_user`. Record fixtures separately; real SSE, persistent cloud Run and Android/iOS recovery require live evidence.

### Task 3: Adapt Paseo composer diff and timeline-detail presentation

**Files:**
- Create: `apps/mobile-next/src/paseo-adapters/runs/PaseoWorkspaceDiffPill.tsx`
- Create: `apps/mobile-next/src/paseo-adapters/runs/PaseoToolCallOverview.tsx`
- Create: `apps/mobile-next/src/paseo-adapters/runs/provenance.ts`
- Test: `apps/mobile-next/tests/paseo-adapters/runs/presentation.test.tsx`

**Interfaces:**
- Consumes: Paseo `packages/app/src/composer/diff-stat-pill.tsx` and `packages/app/src/tool-calls/detail-level/overview/view.tsx` at `d636abd7a4ce302e7ccb9eb6074f637c6dd4d83b`; M05 projected events.
- Produces: `PaseoWorkspaceDiffPill(props: { additions: number; deletions: number; onOpenGit(): void }): React.ReactElement`; `PaseoToolCallOverview(props: { summary: { editedFileCount: number; commandCount: number; readFileCount: number; otherToolCount: number }; loading: boolean; children: React.ReactNode }): React.ReactElement`; `PASEO_RUN_PRESENTATION_SOURCES: readonly PaseoSource[]` with both exact paths, SHA, Apache-2.0 and local adaptation.

- [ ] **Step 1: Write the failing adapter behavior test.**

```tsx
it("retains Paseo diff-pill press and compact tool-detail expand behavior using projected WeKnora data", () => {
  const open = jest.fn(); render(<PaseoWorkspaceDiffPill additions={3} deletions={1} onOpenGit={open} />);
  fireEvent.press(screen.getByTestId("paseo-workspace-diff-pill")); expect(open).toHaveBeenCalledOnce();
  render(<PaseoToolCallOverview summary={{ editedFileCount: 1, commandCount: 2, readFileCount: 0, otherToolCount: 0 }} loading={false}><Text>git diff</Text></PaseoToolCallOverview>);
  fireEvent.press(screen.getByTestId("paseo-tool-call-overview")); expect(screen.getByText("git diff")).toBeTruthy();
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/paseo-adapters/runs/presentation.test.tsx`

Expected: FAIL because the attributed adapters are absent.

- [ ] **Step 3: Copy only the two presentational seams and replace host dependencies.**

For the diff pill retain Paseo's `Pressable`, hover state, addition/deletion rendering and open callback; remove `useVisibleWorkspaceDiffStat` so M05 props are the only data source. For tool overview retain its compact expand/detail flow and bounded `ScrollView`; replace `useTranslation`, `ExpandableBadge`, Unistyles and Paseo sheets with app-owned text/token/sheet primitives. Do not copy `OverviewToolCallGroup`, `run.id`, session stores, daemon hooks, or tool execution behavior.

- [ ] **Step 4: Mount from M05 RunScreen and run GREEN.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/paseo-adapters/runs/presentation.test.tsx tests/features/runs/recovery.test.tsx && npm --prefix apps/mobile-next run typecheck`

Expected: PASS; `RunScreen` supplies decoded projection counts and detail children only, preserving WeKnora as event authority.
