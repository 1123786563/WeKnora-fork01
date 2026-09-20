# M07 Encrypted Cache, Offline Draft and Revocation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Steps use checkbox syntax for tracking.

**Goal:** store permitted read projections and unsent drafts encrypted by trusted cloud scope, while clearing them on logout, scope change and online revocation.

**Architecture:** Extend the existing platform store and scope lifecycle. A per-scope encryption key is held through the platform secure-secret boundary; encrypted records include scope/version metadata. Draft recovery is local-only: reconnection presents an explicit send/discard choice and never creates a Run or approval automatically.

**Tech Stack:** Expo SecureStore, Expo SQLite/FileSystem as selected by existing store seam, Expo Crypto, TypeScript, Jest.

**Spec:** `../../specs/2026-09-20-mobile-ai-office-spec.md`; `../../specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- Cache key is backend, account and tenant; include scope generation/version to discard stale state.
- Read cache and drafts are encrypted, scope isolated and cleared on logout.
- Online revocation clears local cache; offline cannot know revocation immediately.
- User may disable offline cache.
- Reconnect never sends a draft or approves a decision automatically.

## Review Focus

- Identical document IDs in two tenants produce separate ciphertext records.
- Corrupt ciphertext fails closed and is removed rather than rendered as stale content.
- Logout clears records and key material before login UI is displayed.
- A revocation response clears cached Task output and staged drafts.
- Reconnect preserves draft text until an explicit user send/discard choice.

---

### Task 1: Implement encrypted scope-record storage

**Files:**
- Create: `apps/mobile-next/src/cloud-workspace/cache/EncryptedScopeStore.ts`
- Create: `apps/mobile-next/src/cloud-workspace/cache/types.ts`
- Test: `apps/mobile-next/tests/cloud-workspace/cache/encrypted-store.test.ts`
- Modify: `apps/mobile-next/src/platform/store.ts`

**Interfaces:**
- Consumes: `CloudWorkspaceScope`, platform secure-secret adapter and storage adapter.
- Produces: `CacheRecord = { key: string; scope: CloudWorkspaceScope; scopeVersion: number; ciphertext: string }`; `EncryptedScopeStore.write(scope, key, value): Promise<void>`; `read<T>(scope, key): Promise<T | null>`; `clear(scope): Promise<void>`; `clearAll(): Promise<void>`.
- New function: `scopeCacheKey(scope: CloudWorkspaceScope): string` replaces no existing scope key without a controller migration.

- [ ] **Step 1: Write the failing cross-scope encryption fixture test.**

```ts
it("never decrypts tenant t1 cache with tenant t2 scope and clears corrupt ciphertext", async () => {
  await store.write(t1, "task:s1", { title: "机密预算" });
  expect(await store.read(t2, "task:s1")).toBeNull();
  raw.corrupt(scopeCacheKey(t1), "task:s1");
  expect(await store.read(t1, "task:s1")).toBeNull();
  expect(raw.has(scopeCacheKey(t1), "task:s1")).toBe(false);
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/cloud-workspace/cache/encrypted-store.test.ts`

Expected: FAIL because encrypted scoped storage is absent.

- [ ] **Step 3: Implement authenticated encryption through the platform seam.**

```ts
async write(scope: CloudWorkspaceScope, key: string, value: unknown) {
  const encrypted = await this.crypto.seal(await this.keyFor(scope), JSON.stringify(value));
  await this.records.set(scopeCacheKey(scope), key, encrypted);
}
```

Store only ciphertext in ordinary storage. On decrypt/authentication failure delete the record and return `null`; never return a partly decoded value.

- [ ] **Step 4: Run GREEN.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/cloud-workspace/cache/encrypted-store.test.ts tests/domain/scope.test.ts && npm --prefix apps/mobile-next run typecheck`

Expected: PASS for isolation, corruption and scope keys.

### Task 2: Add offline-draft consent and revocation clearing

**Files:**
- Create: `apps/mobile-next/src/cloud-workspace/cache/OfflineDraftController.ts`
- Create: `apps/mobile-next/src/features/offline/OfflineDraftSheet.tsx`
- Test: `apps/mobile-next/tests/features/offline/drafts.test.tsx`
- Handoff only (do not modify): `apps/mobile-next/src/features/auth/AuthController.ts` — fixed integration owner composes M02 trusted-scope publication and this track's cache cleanup callback.

**Interfaces:**
- Consumes: `EncryptedScopeStore`, `CloudWorkspaceClient`, auth lifecycle and M05 `RunRepository` only after user sends.
- Produces: `saveDraft(taskId: string, text: string): Promise<void>`; `onReconnect(): Promise<{ drafts: readonly OfflineDraft[] }>`; `sendDraft(id: string): Promise<void>`; `discardDraft(id: string): Promise<void>`; `handleRevoked(): Promise<void>`.
- New type: `OfflineDraft = { id: string; taskId: string; text: string; createdAt: string }`.
- Auth-lifecycle handoff: `onScopeCacheInvalidated(scope: CloudWorkspaceScope | null): Promise<void>` clears the previous encrypted scope during switch/logout, and is invoked after M02 publishes provisional `null` but before credentials are cleared. This Track supplies the callback constructor; only fixed integration wires it into `AuthDeps`/`AuthController`.

- [ ] **Step 1: Write the failing explicit-consent/revocation test.**

```tsx
it("does not send after reconnect and clears drafts after online revocation", async () => {
  await controller.saveDraft("s1", "继续整理"); await controller.onReconnect();
  expect(runRepository.start).not.toHaveBeenCalled();
  fireEvent.press(screen.getByRole("button", { name: "发送草稿" }));
  expect(runRepository.start).toHaveBeenCalledTimes(1);
  await controller.handleRevoked();
  expect(await store.read(scope, "draft:s1")).toBeNull();
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/features/offline/drafts.test.tsx`

Expected: FAIL until the consent controller exists.

- [ ] **Step 3: Implement consent-first reconnect flow.**

```ts
async onReconnect() { this.state = { kind: "choose-drafts", drafts: await this.list() }; }
async sendDraft(id: string) { const draft = await this.require(id); await this.run.start(this.toRunRequest(draft)); await this.remove(id); }
```

Only an authenticated server `403`/revocation contract invokes `handleRevoked`; connection failure does not masquerade as revocation.

- [ ] **Step 4: Run GREEN and lifecycle regression.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/features/offline/drafts.test.tsx tests/features/auth.test.ts && npm --prefix apps/mobile-next run check:isolation`

Expected: PASS for no-auto-send, explicit send/discard, logout and revocation clearing.

- [ ] **Step 5: Review/demo/evidence gate.**

Demo enabled/disabled cache, offline draft, reconnect choice, logout and revocation fixtures. Encryption tests do not replace device keystore, real revocation, iOS or Android evidence.
