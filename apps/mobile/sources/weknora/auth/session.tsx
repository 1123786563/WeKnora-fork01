import { createJsonTransport, createProductAuthSession, type BearerCredential, type ProductAuthSession } from '@weknora/api-client';
import * as SecureStore from 'expo-secure-store';
import * as React from 'react';
import { useMobileHost } from '@/weknora/platform/host';
import { closeProductClient, createProductScope, type ProductClientTeardown, type ProductScope } from '@/weknora/platform/product-session';
import { stopRealtimeSession } from '@/realtime/RealtimeSession';
import { apiSocket } from '@/sync/apiSocket';
import { createCredentials, productCredentialKey } from './credentials';

const store = {
  get: (key: string) => SecureStore.getItemAsync(key),
  set: (key: string, value: string) => SecureStore.setItemAsync(key, value),
  remove: (key: string) => SecureStore.deleteItemAsync(key),
};

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
    }).catch(() => { if (active) setCredential(null); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [adapter, authSession, host?.origin, scope]);

  const login = React.useCallback(async (email: string, password: string) => {
    if (!host || !adapter || !authSession) throw new Error('SERVER_REQUIRED');
    const result = await authSession.login(email, password);
    await authSession.refreshCoordinator.replace(result.credential);
    scope.switchTo({ origin: host.origin, userId: result.userId, tenantId: result.tenantId });
    setCredential(result.credential);
  }, [adapter, authSession, host, scope]);
  const logout = React.useCallback(async () => {
    scope.logout();
    if (authSession) await authSession.refreshCoordinator.invalidate();
    else if (adapter) await adapter.clear();
    setCredential(null);
  }, [adapter, authSession, scope]);

  const replaceCredential = React.useCallback(async (next: BearerCredential) => {
    if (!host || !authSession) throw new Error('SERVER_REQUIRED');
    await authSession.refreshCoordinator.replace(next);
    setCredential(next);
  }, [authSession, host]);

  return <ProductAuthContext.Provider value={{ credential, authSession, loading, scope, login, logout, replaceCredential }}>{children}</ProductAuthContext.Provider>;
}

export function useProductAuth(): ProductAuthContextValue {
  const value = React.useContext(ProductAuthContext);
  if (!value) throw new Error('PRODUCT_AUTH_PROVIDER_REQUIRED');
  return value;
}
