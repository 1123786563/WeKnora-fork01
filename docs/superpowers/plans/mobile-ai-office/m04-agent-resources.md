# M04 Agent Directory, Knowledge and Attachment Selection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Steps use checkbox syntax for tracking.

**Goal:** select an available WeKnora Agent, permitted knowledge resources and verified attachments for a new or continuing Task.

**Architecture:** Reuse existing `WeKnoraApi.agents`, `knowledgeBases`, session attachment endpoints, `agentSelection`, and `AttachmentUploader`; wrap them in a scoped selection model. The model only stages an intent for M05 and cannot itself submit, approve, or persist authority.

**Tech Stack:** Expo DocumentPicker, existing attachment uploader, TypeScript, Jest.

**Spec:** `../../specs/2026-09-20-mobile-ai-office-spec.md`; `../../specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- Cloud reads only content explicitly selected and authorized for the Task.
- Use WeKnora Agent and knowledge APIs; never adopt Happy/Paseo provider or sync protocols.
- Attachments may be submitted only after their server processing state is ready.
- Resource results and staged selections are bound to the current cloud workspace scope.
- Credentials and connection secrets never enter a chat, attachment metadata or artifact.

## Review Focus

- An attachment still processing is visibly blocked from Run creation.
- Switching workspace clears prior agent, knowledge and attachment selections.
- A server-revoked knowledge base is removed rather than silently preserved in the request.
- Empty agent directory has a usable explicit state and cannot select a fake default.
- A filename containing long Chinese text remains identifiable with its readiness state.

---

### Task 1: Create a scoped resource-selection model

**Files:**
- Create: `apps/mobile-next/src/cloud-workspace/resources/ResourceSelection.ts`
- Test: `apps/mobile-next/tests/cloud-workspace/resources/selection.test.ts`
- Modify: `apps/mobile-next/src/features/workbench/agentSelection.ts`

**Interfaces:**
- Consumes: `CloudWorkspaceClient`, `AgentWire`, `KnowledgeBaseWire`, `AttachmentUploader`.
- Produces: `class ResourceSelection { readonly scope: CloudWorkspaceScope; agentId: string | null; knowledgeBaseIds: readonly string[]; setAgent(agentId: string): void; toggleKnowledge(id: string): void; replaceAttachments(items: readonly AttachmentItem[]): void; readyAttachmentIds(): readonly string[] }`; `createResourceSelection(scope: CloudWorkspaceScope): ResourceSelection`.
- New types: `ResourceCatalog = { agents: readonly string[]; knowledge: readonly string[]; readyAttachments: readonly string[] }`; `AttachmentItem` is imported from `@/features/workbench/attachments/AttachmentUploader`; `validateSelection(selection: ResourceSelection, catalog: ResourceCatalog): { ok: true } | { ok: false; reason: "agent unavailable" | "knowledge unavailable" }`.

- [ ] **Step 1: Write the failing fixture behavior test.**

```ts
it("rejects a revoked knowledge base and a processing attachment", () => {
  const selection = createResourceSelection(scope);
  selection.setAgent("a1"); selection.toggleKnowledge("kb-revoked");
  expect(validateSelection(selection, { agents: ["a1"], knowledge: [], readyAttachments: ["doc-ready"] })).toEqual({ ok: false, reason: "knowledge unavailable" });
  selection.replaceAttachments([{ id: "doc-wait", state: "processing" }]);
  expect(selection.readyAttachmentIds()).toEqual([]);
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/cloud-workspace/resources/selection.test.ts`

Expected: FAIL because selection validation is absent.

- [ ] **Step 3: Implement scope-local staging.**

```ts
export function validateSelection(s: ResourceSelection, c: ResourceCatalog) {
  if (!s.agentId || !c.agents.includes(s.agentId)) return { ok: false as const, reason: "agent unavailable" };
  if (s.knowledgeBaseIds.some((id) => !c.knowledge.includes(id))) return { ok: false as const, reason: "knowledge unavailable" };
  return { ok: true as const };
}
```

Selection must be dropped on a new `CloudWorkspaceScope.generation`; it is not a durable Task property until M05 server acceptance.

- [ ] **Step 4: Run GREEN.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/cloud-workspace/resources/selection.test.ts tests/features/attachments.test.ts && npm --prefix apps/mobile-next run typecheck`

Expected: PASS with existing upload readiness behavior intact.

### Task 2: Render agent/resource picker and safe attachment status

**Files:**
- Create: `apps/mobile-next/src/features/resources/ResourcePickerScreen.tsx`
- Create: `apps/mobile-next/src/features/resources/AttachmentStatusList.tsx`
- Test: `apps/mobile-next/tests/features/resources/picker.test.tsx`

**Interfaces:**
- Consumes: `ResourceSelection`, `validateSelection`, `AttachmentUploader`.
- Produces: `ResourcePickerScreen(props: { selection: ResourceSelection; onReady(selection: ResourceSelection): void }): React.ReactElement`; `AttachmentStatusList(props: { items: readonly AttachmentItem[] }): React.ReactElement`.
- Produces for M05: a validated `ResourceSelection` only; it never calls `startExecution`.

- [ ] **Step 1: Write the failing user-visible selection test.**

```tsx
it("does not offer submit while an uploaded file is processing", () => {
  render(<ResourcePickerScreen selection={processingSelection} onReady={jest.fn()} />);
  expect(screen.getByText("正在处理，完成前不能发送")).toBeTruthy();
  expect(screen.getByRole("button", { name: "开始任务" }).props.accessibilityState.disabled).toBe(true);
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/features/resources/picker.test.tsx`

Expected: FAIL until picker exists.

- [ ] **Step 3: Implement controlled selections and status copy.**

```tsx
<ActionButton disabled={!validation.ok} onPress={() => validation.ok && onReady(selection)}>
  开始任务
</ActionButton>
```

Use existing i18n and uploader state. Do not invent client-side permission success; refreshed server catalog decides availability.

- [ ] **Step 4: Run GREEN and evidence gate.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/features/resources/picker.test.tsx tests/features/attachments.test.ts && npm --prefix apps/mobile-next run check:isolation`

Expected: PASS. Demonstrate empty, revoked, ready and processing fixture states; record a real attachment/cloud authorization check as separate, later evidence.

### Task 3: Adapt Paseo's explicit composer import interaction

**Files:**
- Create: `apps/mobile-next/src/paseo-adapters/resources/PaseoAttachmentImportPill.tsx`
- Create: `apps/mobile-next/src/paseo-adapters/resources/provenance.ts`
- Test: `apps/mobile-next/tests/paseo-adapters/resources/import-pill.test.tsx`

**Interfaces:**
- Consumes: Paseo `packages/app/src/composer/draft/import-pill.tsx` at `d636abd7a4ce302e7ccb9eb6074f637c6dd4d83b`; M04 picker `onPick(): void` and `disabled: boolean`.
- Produces: `PaseoAttachmentImportPill(props: { label: string; onPick(): void; disabled: boolean }): React.ReactElement`; `PASEO_ATTACHMENT_IMPORT_SOURCE = { upstreamPath: "packages/app/src/composer/draft/import-pill.tsx"; upstreamSha: "d636abd7a4ce302e7ccb9eb6074f637c6dd4d83b"; license: "Apache-2.0"; localChanges: string }`.

- [ ] **Step 1: Write the failing behavior/provenance test.**

```tsx
it("keeps Paseo import-pill activation semantics and does not activate while disabled", () => {
  const onPick = jest.fn(); render(<PaseoAttachmentImportPill label="添加附件" disabled onPick={onPick} />);
  fireEvent.press(screen.getByTestId("paseo-attachment-import-pill"));
  expect(onPick).not.toHaveBeenCalled();
  expect(PASEO_ATTACHMENT_IMPORT_SOURCE.upstreamPath).toBe("packages/app/src/composer/draft/import-pill.tsx");
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/paseo-adapters/resources/import-pill.test.tsx`

Expected: FAIL because this attributed adapter is absent.

- [ ] **Step 3: Copy the presentation interaction, replacing only host hooks.**

Copy its `Pressable`/hover/disabled/one-line-label interaction into the target. Replace `useTranslation` with required `label`, `lucide-react-native` with existing `@/components/Icon`, and `react-native-unistyles`/`composerPillStyles` with existing mobile theme tokens. Do not import Paseo agent/session hooks, daemon state, or a provider protocol.

- [ ] **Step 4: Run GREEN and wire through the M04 picker.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/paseo-adapters/resources/import-pill.test.tsx tests/features/resources/picker.test.tsx && npm --prefix apps/mobile-next run typecheck`

Expected: PASS; M04 supplies `onPick` to invoke its existing attachment selection flow and still blocks processing files.
