import { createJsonTransport, createProductAuthSession, type BearerCredential } from '@weknora/api-client';
import * as SecureStore from 'expo-secure-store';
import * as React from 'react';
import { useMobileHost } from '@/weknora/platform/host';
import { createCredentials, productCredentialKey } from './credentials';

const store = {
  get: (key: string) => SecureStore.getItemAsync(key),
  set: (key: string, value: string) => SecureStore.setItemAsync(key, value),
  remove: (key: string) => SecureStore.deleteItemAsync(key),
};

type ProductAuthContextValue = {
  credential: BearerCredential | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
};
const ProductAuthContext = React.createContext<ProductAuthContextValue | null>(null);

export function ProductAuthProvider({ children }: React.PropsWithChildren) {
  const host = useMobileHost();
  const [credential, setCredential] = React.useState<BearerCredential | null>(null);
  const [loading, setLoading] = React.useState(Boolean(host));
  const adapter = React.useMemo(() => host ? createCredentials(store, productCredentialKey(host.origin)) : null, [host]);

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
    if (!host || !adapter) throw new Error('SERVER_REQUIRED');
    const session = createProductAuthSession({ baseURL: host.origin, transport: createJsonTransport(fetch), credentials: adapter });
    const result = await session.login(email, password);
    await adapter.write(result.credential);
    setCredential(result.credential);
  }, [adapter, host]);
  const logout = React.useCallback(async () => {
    if (adapter) await adapter.clear();
    setCredential(null);
  }, [adapter]);

  return <ProductAuthContext.Provider value={{ credential, loading, login, logout }}>{children}</ProductAuthContext.Provider>;
}

export function useProductAuth(): ProductAuthContextValue {
  const value = React.useContext(ProductAuthContext);
  if (!value) throw new Error('PRODUCT_AUTH_PROVIDER_REQUIRED');
  return value;
}
