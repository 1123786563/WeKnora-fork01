import { createJsonTransport, createProductAuthSession, type BearerCredential } from '@weknora/api-client';
import * as SecureStore from 'expo-secure-store';
import * as React from 'react';
import { useMobileHost } from '@/weknora/platform/host';
import { createProductScope, type ProductScope } from '@/weknora/platform/product-session';
import { createCredentials, productCredentialKey } from './credentials';

const store = {
  get: (key: string) => SecureStore.getItemAsync(key),
  set: (key: string, value: string) => SecureStore.setItemAsync(key, value),
  remove: (key: string) => SecureStore.deleteItemAsync(key),
};

type ProductAuthContextValue = {
  credential: BearerCredential | null;
  loading: boolean;
  scope: ProductScope;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
};
const ProductAuthContext = React.createContext<ProductAuthContextValue | null>(null);

export function ProductAuthProvider({ children }: React.PropsWithChildren) {
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
    return () => {
      unsubscribe();
      // A host change retires the old credential store as well as its refresh.
      void authSession.refreshCoordinator.invalidate();
    };
  }, [authSession, redrawForScope, scope]);

  React.useEffect(() => {
    let active = true;
    setLoading(Boolean(adapter));
    setCredential(null);
    if (adapter) adapter.read().then((value) => {
      if (active) setCredential(value.kind === 'bearer' ? value : null);
    }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [adapter]);

  const login = React.useCallback(async (email: string, password: string) => {
    if (!host || !adapter || !authSession) throw new Error('SERVER_REQUIRED');
    const result = await authSession.login(email, password);
    await adapter.write(result.credential);
    scope.switchTo({ origin: host.origin, userId: result.userId, tenantId: result.tenantId });
    setCredential(result.credential);
  }, [adapter, authSession, host, scope]);
  const logout = React.useCallback(async () => {
    scope.logout();
    if (adapter) await adapter.clear();
    setCredential(null);
  }, [adapter, scope]);

  return <ProductAuthContext.Provider value={{ credential, loading, scope, login, logout }}>{children}</ProductAuthContext.Provider>;
}

export function useProductAuth(): ProductAuthContextValue {
  const value = React.useContext(ProductAuthContext);
  if (!value) throw new Error('PRODUCT_AUTH_PROVIDER_REQUIRED');
  return value;
}
