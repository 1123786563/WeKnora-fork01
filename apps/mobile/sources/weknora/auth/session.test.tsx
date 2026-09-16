import * as React from 'react';
// @ts-expect-error react-test-renderer has no declarations in this workspace.
import { act, create } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => {
  const secure = new Map<string, string>();
  const session = {
    me: vi.fn(async () => ({ userId: 'user-a', tenantId: 'tenant-a' })),
    login: vi.fn(async () => ({ credential: { kind: 'bearer', accessToken: 'new-token' }, userId: 'user-a', tenantId: 'tenant-a' })),
    refreshCoordinator: { replace: vi.fn(async () => undefined), invalidate: vi.fn(async () => undefined), advanceGeneration: vi.fn() },
  };
  return { secure, session, host: { backend: 'weknora' as const, origin: 'https://api.example.test' }, revoke: vi.fn(), register: vi.fn(async () => ({ revision: 4 })), issue: vi.fn(async () => ({ registrationIntent: 'signed', scopeGeneration: 8 })), flush: vi.fn(async () => 0), permission: vi.fn(async () => ({ granted: true })), pushToken: vi.fn(async () => 'push-token'), scopeGeneration: 0 };
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
      return { capture: () => ({ generation, signal: new AbortController().signal }), identity: () => identity, switchTo(next: typeof identity) { identity = next; generation += 1; listeners.forEach((listener) => listener()); }, logout() { identity = { origin: mocks.host.origin, userId: null, tenantId: null }; generation += 1; listeners.forEach((listener) => listener()); }, subscribe(listener: () => void) { listeners.add(listener); return () => listeners.delete(listener); }, registerLifecycle: () => () => undefined };
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
vi.mock('@/weknora/auth/credentials', () => ({
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

  it('does not replay a pending revoke for another owner during login', async () => {
    await act(async () => { create(<ProductAuthProvider><Harness /></ProductAuthProvider>); await Promise.resolve(); await Promise.resolve(); });
    expect(mocks.flush).toHaveBeenCalledWith(expect.anything(), expect.anything(), { tenantId: 'tenant-a', ownerId: 'user-a' });
  });
});
