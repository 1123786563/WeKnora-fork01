# M06 Approval Compare-and-Set Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Steps use checkbox syntax for tracking.

**Goal:** show and resolve WeKnora approval interactions with the exact compare-and-set snapshot required by the cloud authority.

**Architecture:** Extend existing `WeKnoraApi.interactions` and `decide`, `InteractionWire`, and `DecisionSheet`. The client submits the interaction ID, operation identity, approved-parameter hash, credential version and expected version supplied by the snapshot; server conflict response replaces the displayed interaction. Mobile never decides approval validity.

**Tech Stack:** TypeScript, existing workbench decision API, React Native, Jest.

**Spec:** `../../specs/2026-09-20-mobile-ai-office-spec.md`; `../../specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- Approval binds Run, interaction, operation, parameter hash, credential version and expected version.
- Any approved field or credential-version change requires a new approval.
- Only the Task Owner may resolve; read-only users are denied by server and receive no executable control.
- Each Git remote write is a separate approval; force push and remote branch deletion are unavailable.
- Unknown outcome remains waiting for reconciliation.

## Review Focus

- Two devices approving the same revision leave one resolved and one refreshed conflict view.
- A changed branch, commit or credential version cannot reuse a prior approval.
- A read-only screen omits controls and server 403 remains a forbidden state if invoked.
- Expired approval does not silently turn into rejection or approval.
- Network timeout after decision prompts reconciliation rather than sending a duplicate decision.

---

### Task 1: Extend interaction snapshot and decision contract

**Files:**
- Handoff only (do not modify): `apps/mobile-next/src/contracts/workbench.ts` — fixed integration owner applies this track's approval decoder/type additions alongside M05's Run additions.
- Create: `apps/mobile-next/src/cloud-workspace/approvals/ApprovalRepository.ts`
- Test: `apps/mobile-next/tests/cloud-workspace/approvals/repository.test.ts`

**Interfaces:**
- Consumes: `CloudWorkspaceClient`, `InteractionWire`, `WeKnoraApi.decide`.
- Produces: `ApprovalSnapshot = { interactionId: string; runId: string; operationId: string; argsHash: string; credentialVersion: string; expectedVersion: number; status: "pending" | "resolved" | "expired"; scope: CloudWorkspaceScope }`; `decide(input: { interactionId: string; action: "approve" | "reject"; expectedVersion: number; argsHash: string; credentialVersion: string }): Promise<ApprovalSnapshot>`.
- New decoder: `decodeApprovalSnapshot(value: unknown): ApprovalSnapshot`; backend contract must return the binding fields or the UI shows unavailable, never substitutes blanks.
- Type-change handoff: supply `ApprovalSnapshotWire = { interaction_id: string; run_id: string; operation_id: string; args_hash: string; credential_version: string; revision: number; status: "pending" | "resolved" | "expired"; scope: CloudWorkspaceScope }` and `decodeApprovalSnapshotWire(value: unknown): ApprovalSnapshotWire` to the fixed integration owner; this Track never edits the shared contract file.

- [ ] **Step 1: Write the failing CAS fixture test.**

```ts
it("replaces a stale approval with the server current snapshot", async () => {
  transport.conflict({ interaction_id: "i1", status: "resolved", revision: 4, operation_id: "push", args_hash: "new", credential_version: "cv2" });
  await expect(repo.decide({ interactionId: "i1", action: "approve", expectedVersion: 3, argsHash: "old", credentialVersion: "cv1" })).rejects.toMatchObject({ current: { status: "resolved", expectedVersion: 4 } });
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/cloud-workspace/approvals/repository.test.ts`

Expected: FAIL because the full binding decoder and conflict result are absent.

- [ ] **Step 3: Implement strict decoding and conflict mapping.**

```ts
if (!wire.operation_id || !wire.args_hash || !wire.credential_version || wire.revision == null) {
  throw new Error("approval snapshot incomplete");
}
```

On HTTP conflict, decode its authoritative snapshot and surface it to the controller; do not retry the command.

- [ ] **Step 4: Run GREEN.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/cloud-workspace/approvals/repository.test.ts tests/api/http.test.ts && npm --prefix apps/mobile-next run typecheck`

Expected: PASS for conflict replacement and incomplete snapshot rejection.

### Task 2: Render owner-only decision sheet and reconciliation state

**Files:**
- Create: `apps/mobile-next/src/features/approvals/ApprovalController.ts`
- Modify: `apps/mobile-next/src/components/DecisionSheet.tsx`
- Test: `apps/mobile-next/tests/features/approvals/decision-sheet.test.tsx`

**Interfaces:**
- Consumes: `ApprovalRepository`, `ApprovalSnapshot`, `TaskSummary.access`.
- Produces: `ApprovalController.resolve(action: "approve" | "reject"): Promise<void>`; `DecisionSheet(props: { approval: ApprovalSnapshot; writable: boolean; onResolve(action: "approve" | "reject"): void }): React.ReactElement`.
- New function: `approvalCanResolve(approval: ApprovalSnapshot, access: "owner" | "read_only"): boolean`.

- [ ] **Step 1: Write the failing owner/read-only test.**

```tsx
it("hides resolve actions for read-only and shows current conflict data after CAS failure", async () => {
  render(<DecisionSheet approval={pending} writable={false} onResolve={jest.fn()} />);
  expect(screen.queryByRole("button", { name: "批准" })).toBeNull();
  await controller.resolve("approve");
  expect(screen.getByText("此审批已由其他操作更新")).toBeTruthy();
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/features/approvals/decision-sheet.test.tsx`

Expected: FAIL until controlled decision state is implemented.

- [ ] **Step 3: Implement no-retry decision display.**

```ts
if (!approvalCanResolve(this.approval, this.access)) return;
try { this.approval = await this.repo.decide(input); } catch (error) { this.state = { kind: "reconcile", error }; }
```

Display server-provided repo/branch/commit/PR summary only as approval content; never store a connection credential.

- [ ] **Step 4: Run GREEN and evidence gate.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/features/approvals/decision-sheet.test.tsx tests/features/pages-execution-approval.test.tsx && npm --prefix apps/mobile-next run check:isolation`

Expected: PASS. Demo pending/resolved/expired/conflict/read-only fixtures; require real owner/connection/Git approval evidence separately.
