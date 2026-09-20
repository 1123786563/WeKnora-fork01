# M03 Task List, Detail and Workspace Navigation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Steps use checkbox syntax for tracking.

**Goal:** project WeKnora sessions as Tasks and navigate their task-scoped conversation, files, Git, terminal and artifacts without creating a client Task authority.

**Architecture:** The existing `/sessions` result is the current source to inspect; this track introduces a cloud-workspace query contract that returns scope, snapshot, cursor and projectable Task rows. Expo Router receives only IDs and rehydrates through the scoped client. Root route registration is controller-owned.

**Tech Stack:** Expo Router, React Native, TypeScript, Jest, existing `SessionWire` and `WeKnoraApi`.

**Spec:** `../../specs/2026-09-20-mobile-ai-office-spec.md`; `../../specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- Task is the persistent WeKnora Session ID; one Task may contain multiple Runs.
- Task pages are scoped to the current tenant and authorization is rechecked by WeKnora on each read.
- Workspace is task-internal, never a space-wide writable file browser.
- Shared readers may view only permitted material and cannot continue, use terminal, or approve.
- Do not promise sandbox sleep/resume until a real provider verifies it.

## Review Focus

- Direct navigation to an inaccessible task must replace visible stale content with forbidden state.
- Browser/navigation back must retain the selected task without duplicating a session.
- A long task title must not hide the workspace selector.
- A completed Task with no Run still presents its conversation and permitted outputs.
- A read-only viewer sees no executable action even if a stale local row says owner.

---

### Task 1: Define Task projection and scoped list/detail query

**Files:**
- Create: `apps/mobile-next/src/cloud-workspace/tasks/TaskRepository.ts`
- Create: `apps/mobile-next/src/cloud-workspace/tasks/types.ts`
- Test: `apps/mobile-next/tests/cloud-workspace/tasks/repository.test.ts`
- Create: `apps/mobile-next/src/cloud-workspace/tasks/createTaskPort.ts`

**Interfaces:**
- Consumes: M02 `CloudWorkspaceScope`, this track's `TaskPort`, existing `WeKnoraApi.sessions(params)` and `SessionWire`.
- Produces: `TaskSummary = { sessionId: string; title: string; updatedAt: string; access: "owner" | "read_only" }`; `TaskSnapshot = { task: TaskSummary; cursor: string | null; sequence: number; scope: CloudWorkspaceScope }`; `listTasks(input: { cursor?: string }): Promise<TaskSnapshot[]>`; `getTask(sessionId: string): Promise<TaskSnapshot>`.
- Produces for fixed integration: `TaskPort = { getTaskSnapshot(sessionId: string): Promise<CloudTaskSnapshotWire>; listTaskSnapshots(input: { cursor?: string }): Promise<readonly CloudTaskSnapshotWire[]> }` and `createTaskPort(input: { api: WeKnoraApi; scope: CloudWorkspaceScope }): TaskPort`. The fixed controller/integrator is sole owner of `CloudWorkspaceClient.ts` and composes this port; do not infer access from navigation.

- [ ] **Step 1: Write the failing external-contract fixture test.**

```ts
it("projects a scoped server task and rejects a mismatched tenant snapshot", async () => {
  transport.reply({ scope: { tenantId: "t1" }, sequence: 7, cursor: "c7", task: { session_id: "s1", access: "read_only", title: "预算复盘" } });
  await expect(repo.getTask("s1")).resolves.toMatchObject({ task: { sessionId: "s1", access: "read_only" } });
  transport.reply({ scope: { tenantId: "t2" }, sequence: 8, cursor: "c8", task: { session_id: "s1" } });
  await expect(repo.getTask("s1")).rejects.toThrow("scope mismatch");
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/cloud-workspace/tasks/repository.test.ts`

Expected: FAIL because Task projection is absent.

- [ ] **Step 3: Implement one query adapter and explicit wire decoder.**

```ts
async getTask(sessionId: string): Promise<TaskSnapshot> {
  const wire = await this.port.getTaskSnapshot(sessionId);
  if (wire.scope.tenantId !== this.scope.tenantId) throw new Error("scope mismatch");
  return decodeTaskSnapshot(wire, this.scope);
}
```

Do not fabricate a status machine from `SessionWire`; expose server snapshot fields verbatim where unknown.

- [ ] **Step 4: Run GREEN.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/cloud-workspace/tasks/repository.test.ts && npm --prefix apps/mobile-next run typecheck`

Expected: PASS for owner/read-only projection and mismatched-scope rejection.

### Task 2: Render list/detail and task-internal workspace navigation

**Files:**
- Create: `apps/mobile-next/src/features/tasks/TaskListScreen.tsx`
- Create: `apps/mobile-next/src/features/tasks/TaskDetailScreen.tsx`
- Create: `apps/mobile-next/src/features/tasks/WorkspaceSections.tsx`
- Test: `apps/mobile-next/tests/features/tasks/navigation.test.tsx`
- Handoff only (do not modify): `apps/mobile-next/app/**` route registration.

**Interfaces:**
- Consumes: `TaskRepository`, `TaskSnapshot`, `access` from Task 1.
- Produces: `TaskListScreen(props: { repository: TaskRepository; open(sessionId: string): void }): React.ReactElement`; `TaskDetailScreen(props: { sessionId: string; repository: TaskRepository }): React.ReactElement`.
- New type: `WorkspaceSection = "conversation" | "files" | "git" | "terminal" | "artifacts"`.

- [ ] **Step 1: Write the failing navigation behavior test.**

```tsx
it("opens task-local Files and hides continue/terminal controls for a read-only task", async () => {
  const view = render(<TaskDetailScreen sessionId="s1" repository={readOnlyRepo} />);
  await screen.findByText("预算复盘");
  fireEvent.press(screen.getByRole("button", { name: "文件" }));
  expect(screen.getByTestId("workspace-files")).toBeTruthy();
  expect(screen.queryByRole("button", { name: "继续执行" })).toBeNull();
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/features/tasks/navigation.test.tsx`

Expected: FAIL until screens and access projection exist.

- [ ] **Step 3: Implement projection-driven pages.**

```tsx
const sections: WorkspaceSection[] = ["conversation", "files", "git", "terminal", "artifacts"];
return <WorkspaceSections sections={sections} writable={task.access === "owner"} />;
```

Only the backend refusal is authoritative; hiding a button is a usability projection, not authorization.

- [ ] **Step 4: Run GREEN and router handoff.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/features/tasks/navigation.test.tsx && npm --prefix apps/mobile-next run typecheck`

Expected: PASS. Give the screen constructors and route parameters to the controller; do not modify root router files.

- [ ] **Step 5: Review/demo/evidence gate.**

Demo list, direct detail, back navigation, owner/read-only and forbidden fetch states. A fixture proves projection only; record live cloud ACL and real-device navigation as pending evidence.
