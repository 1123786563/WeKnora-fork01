import * as React from 'react';
// @ts-expect-error react-test-renderer has no declarations in this workspace.
import { act, create } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => {
  const secure = new Map<string, string>();
  const session = {
    me: vi.fn(async () => ({ userId: 'user-a', tenantId: 'tenant-a' })),
    login: vi.fn(async () => ({ credential: { kind: 'bearer', accessToken: 'new-token' }, userId: 'user-a', tenantId: 'tenant-a' })),
    refreshCoordinator: { replace: vi.fn(async (): Promise<void> => { }), invalidate: vi.fn(async () => undefined), advanceGeneration: vi.fn() },
  };
  return { secure, session, host: { backend: 'weknora' as const, origin: 'https://api.example.test' }, revoke: vi.fn(), register: vi.fn(async (_input: Record<string, unknown>) => ({ revision: 4 })), issue: vi.fn(async () => ({ registrationIntent: 'signed', scopeGeneration: 8 })), flush: vi.fn(async () => 0), permission: vi.fn(async () => ({ granted: true })), pushToken: vi.fn(async () => 'push-token'), scopeGeneration: 0 };
});

vi.mock('expo-secure-store', () => ({
  getItemAsync: async (key: string) => mocks.secure.get(key) ?? null,
  setItemAsync: async (key: string, value: string) => { mocks.secure.set(key, value); },
  deleteItemAsync: async (key: string) => { mocks.secure.delete(key); },
}));
vi.mock('react-native', () => ({ Platform: { OS: 'ios' } }));
vi.mock('@/weknora/platform/host', () => ({ useMobileHost: () => mocks.host }));
vi.mock('@/weknora/platform/product-session', async () => {
  const ReactModule = await import('react');
  return {
    closeProductClient: vi.fn(async () => undefined),
    createProductScope: () => {
      let identity = { origin: mocks.host.origin, userId: null as string | null, tenantId: null as string | null };
      let generation = 0;
      const listeners = new Set<() => void>();
      return { capture: () => ({ generation, signal: new AbortController().signal }), accept: (captured: number) => captured === generation, identity: () => identity, switchTo(next: typeof identity) { identity = next; generation += 1; listeners.forEach((listener) => listener()); }, logout() { identity = { origin: mocks.host.origin, userId: null, tenantId: null }; generation += 1; listeners.forEach((listener) => listener()); }, subscribe(listener: () => void) { listeners.add(listener); return () => listeners.delete(listener); }, registerLifecycle: () => () => undefined };
    },
    // Keep React imported in this mock so bundlers do not elide the module as
    // a side-effect-free replacement of the real platform seam.
    React: ReactModule,
  };
});
vi.mock('@/sync/pushRegistration', () => ({ getPushPermissionInfo: mocks.permission, getCurrentExpoPushToken: mocks.pushToken }));
vi.mock('@/weknora/notifications/registration', () => ({
  issueRegistrationIntent: mocks.issue,
  registerDevice: mocks.register,
  revokeOnLogout: mocks.revoke,
  flushPendingRevocations: mocks.flush,
}));
vi.mock('@/realtime/RealtimeSession', () => ({ stopRealtimeSession: vi.fn() }));
vi.mock('@/sync/apiSocket', () => ({ apiSocket: { disconnect: vi.fn() } }));
vi.mock('@weknora/api-client', () => ({
  createJsonTransport: vi.fn(),
  createProductAuthSession: vi.fn(() => mocks.session),
}));
vi.mock('./credentials', () => ({
  createCredentials: vi.fn(() => ({
    read: async () => ({ kind: 'bearer', accessToken: 'stored-token' }),
    clear: async () => undefined,
  })),
  productCredentialKey: (origin: string) => origin,
}));

import { ProductAuthProvider, useProductAuth } from './session';

let auth: ReturnType<typeof useProductAuth> | undefined;
function Harness() {
  auth = useProductAuth();
  return null;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.secure.clear();
  mocks.revoke.mockResolvedValue({ scopeGeneration: 12 });
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});
afterEach(() => { auth = undefined; });

describe('ProductAuthProvider device lifecycle', () => {
  it('consumes the server revoke epoch before clearing the product scope', async () => {
    let renderer: ReturnType<typeof create> | undefined;
    await act(async () => { renderer = create(<ProductAuthProvider><Harness /></ProductAuthProvider>); await Promise.resolve(); await Promise.resolve(); });
    await vi.waitFor(() => expect(mocks.register).toHaveBeenCalled());
    expect(auth?.credential?.accessToken).toBe('stored-token');
    await act(async () => { await auth?.logout(); });
    expect(mocks.revoke).toHaveBeenCalledWith(expect.objectContaining({ tenantId: 'tenant-a', ownerId: 'user-a' }));
    const epoch = Array.from(mocks.secure.entries()).find(([key]) => key.includes('weknora:mobile-device-epoch:'));
    expect(epoch?.[1]).toBe('12');
    renderer?.unmount();
  });

  it('keeps the product session usable when notification permission is denied', async () => {
    mocks.permission.mockResolvedValueOnce({ granted: false });
    await act(async () => { create(<ProductAuthProvider><Harness /></ProductAuthProvider>); await Promise.resolve(); await Promise.resolve(); });
    await vi.waitFor(() => expect(auth?.credential?.accessToken).toBe('stored-token'));
    expect(mocks.register).not.toHaveBeenCalled();
  });

  it('does not replay a pending revoke for another owner during login', async () => {
    await act(async () => { create(<ProductAuthProvider><Harness /></ProductAuthProvider>); await Promise.resolve(); await Promise.resolve(); });
    expect(mocks.flush).toHaveBeenCalledWith(expect.anything(), expect.anything(), { tenantId: 'tenant-a', ownerId: 'user-a' });
  });

  it('resolves the replacement identity and drops the previous owner revision', async () => {
    await act(async () => { create(<ProductAuthProvider><Harness /></ProductAuthProvider>); await Promise.resolve(); await Promise.resolve(); });
    await vi.waitFor(() => expect(mocks.register).toHaveBeenCalled());
    mocks.session.me.mockResolvedValueOnce({ userId: 'user-b', tenantId: 'tenant-b' });
    await act(async () => { await auth?.replaceCredential({ kind: 'bearer', accessToken: 'token-b' }); });
    const last = mocks.register.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(last.credential).toEqual({ kind: 'bearer', accessToken: 'token-b' });
    expect(last).not.toHaveProperty('revision');
    expect(mocks.flush).toHaveBeenLastCalledWith(expect.anything(), { kind: 'bearer', accessToken: 'token-b' }, { tenantId: 'tenant-b', ownerId: 'user-b' });
  });

  it('invalidates the old scope before a replacement refresh can await', async () => {
    let resolveOldRegistration!: (value: { revision: number }) => void;
    const oldRegistration = new Promise<{ revision: number }>((resolve) => { resolveOldRegistration = resolve; });
    mocks.register.mockImplementationOnce(() => oldRegistration);
    await act(async () => { create(<ProductAuthProvider><Harness /></ProductAuthProvider>); await Promise.resolve(); await Promise.resolve(); });
    await vi.waitFor(() => expect(mocks.register).toHaveBeenCalledTimes(1));

    let resolveRefresh!: () => void;
    mocks.session.refreshCoordinator.replace.mockImplementationOnce(() => new Promise<void>((resolve) => { resolveRefresh = resolve; }));
    mocks.session.me.mockResolvedValueOnce({ userId: 'user-b', tenantId: 'tenant-b' });
    const replacing = auth?.replaceCredential({ kind: 'bearer', accessToken: 'token-b' });
    await Promise.resolve();
    expect(mocks.session.refreshCoordinator.replace).toHaveBeenCalled();

    // The old request completes while refresh replacement is still paused.
    // It must fail its captured lifecycle fence and never install revision 99.
    resolveOldRegistration({ revision: 99 });
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(mocks.register).toHaveBeenCalledTimes(1);

    resolveRefresh();
    await act(async () => { await replacing; await Promise.resolve(); await Promise.resolve(); });
    await vi.waitFor(() => expect(mocks.register).toHaveBeenCalledTimes(2));
    const replacement = mocks.register.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(replacement.credential).toEqual({ kind: 'bearer', accessToken: 'token-b' });
    expect(replacement).not.toHaveProperty('revision');
  });

  it('does not commit a registration response captured before logout', async () => {
    let resolveRegistration!: (value: { revision: number }) => void;
    const pending = new Promise<{ revision: number }>((resolve) => { resolveRegistration = resolve; });
    mocks.register.mockImplementationOnce(() => pending);
    await act(async () => { create(<ProductAuthProvider><Harness /></ProductAuthProvider>); await Promise.resolve(); await Promise.resolve(); });
    await vi.waitFor(() => expect(mocks.register).toHaveBeenCalled());
    await act(async () => { await auth?.logout(); });
    resolveRegistration({ revision: 99 });
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    await act(async () => { await auth?.login('user-a@example.test', 'secret'); });
    await vi.waitFor(() => expect(mocks.register).toHaveBeenCalledTimes(2));
    const afterLogin = mocks.register.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(afterLogin).not.toHaveProperty('revision');
  });

  it('flushes an offline logout intent after the same account logs in again', async () => {
    await act(async () => { create(<ProductAuthProvider><Harness /></ProductAuthProvider>); await Promise.resolve(); await Promise.resolve(); });
    await vi.waitFor(() => expect(mocks.register).toHaveBeenCalled());
    // Mirror the verified revokeOnLogout seam contract (registration.test.ts
    // "keeps a minimal pending revocation on offline logout"): a failed revoke
    // queues a pending revocation instead of rejecting, so logout completes
    // and the next same-owner login can flush the queued intent.
    mocks.revoke.mockImplementationOnce(async (input: { origin: string; deviceId: string; tenantId: string; ownerId: string; revision?: number; pending: { read(): Promise<unknown[]>; write(rows: unknown[]): Promise<void> } }) => {
      const rows = await input.pending.read();
      await input.pending.write([...rows, { origin: input.origin, deviceId: input.deviceId, tenantId: input.tenantId, ownerId: input.ownerId, ...(input.revision === undefined ? {} : { revision: input.revision }) }]);
      return {};
    });
    await act(async () => { await auth?.logout(); });
    await act(async () => { await auth?.login('user-a@example.test', 'secret'); });
    expect(mocks.flush).toHaveBeenLastCalledWith(expect.anything(), expect.objectContaining({ accessToken: 'new-token' }), { tenantId: 'tenant-a', ownerId: 'user-a' });
  });
});
