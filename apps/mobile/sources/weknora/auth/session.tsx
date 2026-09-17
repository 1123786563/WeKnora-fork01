import { createJsonTransport, createProductAuthSession, type BearerCredential, type ProductAuthSession } from '@weknora/api-client';
import * as SecureStore from 'expo-secure-store';
import { Platform } from 'react-native';
import * as React from 'react';
import { useMobileHost } from '@/weknora/platform/host';
import { closeProductClient, createProductScope, type ProductClientTeardown, type ProductScope } from '@/weknora/platform/product-session';
import { stopRealtimeSession } from '@/realtime/RealtimeSession';
import { apiSocket } from '@/sync/apiSocket';
import { createCredentials, productCredentialKey } from './credentials';
import { getCurrentExpoPushToken, getPushPermissionInfo } from '@/sync/pushRegistration';
import { flushPendingRevocations, issueRegistrationIntent, registerDevice, revokeOnLogout, type PendingRevocation, type PendingRevocationStore } from '@/weknora/notifications/registration';

const store = {
  get: (key: string) => SecureStore.getItemAsync(key),
  set: (key: string, value: string) => SecureStore.setItemAsync(key, value),
  remove: (key: string) => SecureStore.deleteItemAsync(key),
};

function pendingStore(origin: string): PendingRevocationStore {
  const key = `weknora:mobile-device-revocations:${encodeURIComponent(new URL(origin).origin)}`;
  return {
    async read() {
      const raw = await SecureStore.getItemAsync(key); if (!raw) return [];
      try { const value = JSON.parse(raw) as unknown; return Array.isArray(value) ? value as PendingRevocation[] : []; } catch { return []; }
    },
    write(rows) { return SecureStore.setItemAsync(key, JSON.stringify(rows)); },
  };
}

async function nativeDeviceId(): Promise<string> {
  const key = 'weknora:mobile-device-id';
  const existing = await SecureStore.getItemAsync(key);
  if (existing) return existing;
  const generated = `mobile-${Date.now()}-${Math.random().toString(36).slice(2)}`;
  await SecureStore.setItemAsync(key, generated);
  return generated;
}

function deviceEpochKey(origin: string, deviceId: string): string {
  return `weknora:mobile-device-epoch:${encodeURIComponent(new URL(origin).origin)}:${encodeURIComponent(deviceId)}`;
}

/** Server epoch is authoritative, while local values protect against a
 * restart that occurs between revoke and SecureStore persistence. */
export function mergeDeviceScopeHighWater(...values: Array<number | undefined>): number {
  return values.reduce<number>((highWater, value) => Number.isSafeInteger(value) && (value as number) >= 0 ? Math.max(highWater, value as number) : highWater, 0);
}

type ProductAuthContextValue = {
  credential: BearerCredential | null;
  authSession: ProductAuthSession | null;
  loading: boolean;
  scope: ProductScope;
  login: (email: string, password: string) => Promise<void>;
  replaceCredential: (credential: BearerCredential) => Promise<void>;
  logout: () => Promise<void>;
};
const ProductAuthContext = React.createContext<ProductAuthContextValue | null>(null);

const defaultTeardown: ProductClientTeardown = {
  stopVoice: stopRealtimeSession,
  disconnectRemote: () => apiSocket.disconnect(),
};

export function ProductAuthProvider({ children, teardown = defaultTeardown }: React.PropsWithChildren<{ teardown?: ProductClientTeardown }>) {
  const host = useMobileHost();
  const [credential, setCredential] = React.useState<BearerCredential | null>(null);
  // scopeGeneration is the server-controlled epoch observed by the last
  // registration/revocation. Keeping it with the device reference prevents a
  // cold-start login from falling back to the process-local scope counter.
  const mobileDevice = React.useRef<{ deviceId: string; revision?: number; scopeGeneration?: number } | null>(null);
  const [loading, setLoading] = React.useState(Boolean(host));
  const [, redrawForScope] = React.useReducer((version: number) => version + 1, 0);
  const adapter = React.useMemo(() => host ? createCredentials(store, productCredentialKey(host.origin)) : null, [host]);
  const scope = React.useMemo(() => createProductScope({
    origin: host?.origin ?? '',
    userId: null,
    tenantId: null,
  }), [host?.origin]);
  const authSession = React.useMemo(() => {
    if (!host || !adapter) return null;
    return createProductAuthSession({ baseURL: host.origin, transport: createJsonTransport(fetch), credentials: adapter });
  }, [adapter, host]);

  React.useEffect(() => {
    if (!authSession) return;
    // Scope transitions invalidate an in-flight refresh generation while
    // retaining the credential for a workspace switch.
    const unsubscribe = scope.subscribe(() => {
      authSession.refreshCoordinator.advanceGeneration();
      redrawForScope();
    });
    const unregisterLifecycle = scope.registerLifecycle(() => {
      void closeProductClient(teardown);
    });
    return () => {
      unsubscribe();
      unregisterLifecycle();
      // A host change retires the old credential store as well as its refresh.
      void authSession.refreshCoordinator.invalidate();
    };
  }, [authSession, redrawForScope, scope, teardown]);

  React.useEffect(() => {
    let active = true;
    setLoading(Boolean(adapter));
    setCredential(null);
    if (adapter) adapter.read().then(async (value) => {
      if (!active || value.kind !== 'bearer') { if (active) setCredential(null); return; }
      // The server is the only authority for the product identity. A bearer
      // token alone must never revive a stale/local tenant selection.
      const identity = await authSession?.me();
      if (!active || !identity) return;
      scope.switchTo({ origin: host?.origin ?? '', userId: identity.userId, tenantId: identity.tenantId });
      setCredential(value);
      // Flush only with the freshly restored in-memory credential. The queue
      // itself contains no bearer and therefore cannot authenticate a request.
      if (identity.tenantId) void flushPendingRevocations(pendingStore(host?.origin ?? ''), value, { tenantId: identity.tenantId, ownerId: identity.userId }).catch(() => undefined);
      const captured = scope.capture();
      void syncMobileDevice(host?.origin ?? '', value, captured, mobileDevice).catch(() => undefined);
    }).catch(() => { if (active) setCredential(null); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [adapter, authSession, host?.origin, scope]);

  async function syncMobileDevice(origin: string, next: BearerCredential, captured: { generation: number; signal: AbortSignal }, ref: React.MutableRefObject<{ deviceId: string; revision?: number; scopeGeneration?: number } | null>) {
    const ensureCurrent = () => {
      if (captured.signal.aborted || !scope.accept(captured.generation)) {
        const error = new Error('Product scope changed');
        error.name = 'AbortError';
        throw error;
      }
    };
    ensureCurrent();
    const permission = await getPushPermissionInfo();
    ensureCurrent();
    if (!permission.granted || (Platform.OS !== 'ios' && Platform.OS !== 'android')) return;
    const token = await getCurrentExpoPushToken(); if (!token) return;
    ensureCurrent();
    const deviceId = await nativeDeviceId();
    ensureCurrent();
    const persisted = await SecureStore.getItemAsync(deviceEpochKey(origin, deviceId));
    ensureCurrent();
    const persistedGeneration = persisted ? Number.parseInt(persisted, 10) : 0;
    const intent = await issueRegistrationIntent({ origin, deviceId, credential: next, signal: captured.signal });
    ensureCurrent();
    const serverGeneration = Math.max(intent.scopeGeneration, ref.current?.scopeGeneration ?? 0, Number.isSafeInteger(persistedGeneration) ? persistedGeneration : 0);
    const result = await registerDevice({ origin, deviceId, platform: Platform.OS, token, scopeGeneration: intent.scopeGeneration, registrationIntent: intent.registrationIntent, ...(ref.current?.revision === undefined ? {} : { revision: ref.current.revision }), credential: next, signal: captured.signal });
    ensureCurrent();
    ref.current = { deviceId, revision: result.revision, scopeGeneration: Math.max(serverGeneration, intent.scopeGeneration) };
    ensureCurrent();
    await SecureStore.setItemAsync(deviceEpochKey(origin, deviceId), String(Math.max(serverGeneration, intent.scopeGeneration)));
  }

  const login = React.useCallback(async (email: string, password: string) => {
    if (!host || !adapter || !authSession) throw new Error('SERVER_REQUIRED');
    const result = await authSession.login(email, password);
    await authSession.refreshCoordinator.replace(result.credential);
    scope.switchTo({ origin: host.origin, userId: result.userId, tenantId: result.tenantId });
    setCredential(result.credential);
    if (result.tenantId) void flushPendingRevocations(pendingStore(host.origin), result.credential, { tenantId: result.tenantId, ownerId: result.userId }).catch(() => undefined);
    const captured = scope.capture();
    void syncMobileDevice(host.origin, result.credential, captured, mobileDevice).catch(() => undefined);
  }, [adapter, authSession, host, scope]);
  const logout = React.useCallback(async () => {
    const current = scope.identity();
    const currentCredential = credential;
    const captured = scope.capture();
    const previousDevice = mobileDevice.current;
    // Retire the captured lifecycle before any network or storage await. A
    // registration already in flight must not commit into a post-logout
    // account while the remote revoke is still pending.
    scope.logout();
    let serverScopeGeneration: number | undefined;
    if (currentCredential && current.origin && previousDevice && current.tenantId && current.userId) {
      const revoked = await revokeOnLogout({ origin: current.origin, deviceId: previousDevice.deviceId, revision: previousDevice.revision, credential: currentCredential, pending: pendingStore(current.origin), tenantId: current.tenantId, ownerId: current.userId });
      serverScopeGeneration = revoked.scopeGeneration;
    }
    // RevokeForTenant advances the durable server epoch by one. scope.logout
    // advances the local generation by the same transition; retain that
    // observed value for a subsequent login, including after auth replacement.
    if (previousDevice) {
      const persisted = await SecureStore.getItemAsync(deviceEpochKey(current.origin, previousDevice.deviceId));
      const persistedGeneration = persisted ? Number.parseInt(persisted, 10) : 0;
      const highWater = mergeDeviceScopeHighWater(captured.generation + 1, previousDevice.scopeGeneration, Number.isSafeInteger(persistedGeneration) ? persistedGeneration : undefined, serverScopeGeneration);
      // An account replacement may have installed a new device reference
      // while revoke was in flight. Never overwrite that reference.
      if (mobileDevice.current === previousDevice) mobileDevice.current = { ...previousDevice, scopeGeneration: highWater, revision: undefined };
      await SecureStore.setItemAsync(deviceEpochKey(current.origin, previousDevice.deviceId), String(highWater));
    }
    if (authSession) await authSession.refreshCoordinator.invalidate();
    else if (adapter) await adapter.clear();
    setCredential(null);
  }, [adapter, authSession, credential, scope]);

  const replaceCredential = React.useCallback(async (next: BearerCredential) => {
    if (!host || !authSession) throw new Error('SERVER_REQUIRED');
    // Retire the old bearer scope before the first await.  Replacing the
    // refresh credential can wait on a provider/network operation; leaving
    // the old generation live during that window lets an in-flight device
    // registration pass its final fence and write the previous account's
    // revision into the replacement account.
    scope.switchTo({ origin: host.origin, userId: null, tenantId: null });
    mobileDevice.current = null;
    await authSession.refreshCoordinator.replace(next);
    const identity = await authSession.me();
    if (!identity) throw new Error('PRODUCT_IDENTITY_REQUIRED');
    // Resolve the new bearer before starting device work. This creates a new
    // lifecycle generation and drops revision/scope state belonging to the
    // previous owner; queued revocations remain owner-bound in SecureStore.
    scope.switchTo({ origin: host.origin, userId: identity.userId, tenantId: identity.tenantId });
    setCredential(next);
    if (identity.tenantId) void flushPendingRevocations(pendingStore(host.origin), next, { tenantId: identity.tenantId, ownerId: identity.userId }).catch(() => undefined);
    const captured = scope.capture();
    void syncMobileDevice(host.origin, next, captured, mobileDevice).catch(() => undefined);
  }, [authSession, host, scope]);

  return <ProductAuthContext.Provider value={{ credential, authSession, loading, scope, login, logout, replaceCredential }}>{children}</ProductAuthContext.Provider>;
}

export function useProductAuth(): ProductAuthContextValue {
  const value = React.useContext(ProductAuthContext);
  if (!value) throw new Error('PRODUCT_AUTH_PROVIDER_REQUIRED');
  return value;
}
