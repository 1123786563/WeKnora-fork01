import { AppState } from 'react-native';
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { createRefreshCoordinator, createWeKnoraClient, type Credential, type WeKnoraClient } from '@weknora/api-client';
import { resolveMobileApiBaseUrl } from './platform/transport.ts';
import { createSecureCredentialAdapter } from './platform/credentials.ts';
import { createMobileTransport } from './platform/transport.ts';

interface MobileRuntimeValue {
  client: WeKnoraClient;
  baseURL: string;
  credential: Credential;
  hydrating: boolean;
  login(email: string, password: string): Promise<void>;
  logout(): Promise<void>;
}

const RuntimeContext = createContext<MobileRuntimeValue | null>(null);

export function MobileRuntimeProvider({ children }: { children: ReactNode }) {
  const adapter = useMemo(() => createSecureCredentialAdapter(), []);
  const [credential, setCredential] = useState<Credential>({ kind: 'anonymous' });
  const credentialRef = useRef<Credential>({ kind: 'anonymous' });
  const [hydrating, setHydrating] = useState(true);
  const [tenantId] = useState<string | null>(null);
  const baseURL = resolveMobileApiBaseUrl(process.env.EXPO_PUBLIC_API_BASE_URL || '');
  const updateCredential = useCallback((next: Credential) => {
    credentialRef.current = next;
    setCredential(next);
  }, []);
  const refreshClient = useMemo(() => createWeKnoraClient({
    baseURL,
    transport: createMobileTransport({ credential: () => ({ kind: 'anonymous' }) }),
  }), [baseURL]);
  const refreshCoordinator = useMemo(() => createRefreshCoordinator({
    credentials: adapter,
    refresh: async (refreshToken) => {
      const refreshed = await refreshClient.auth.refresh(refreshToken);
      return { success: true, ...refreshed };
    },
  }), [adapter, refreshClient]);
  const refreshSession = useCallback(async () => {
    try {
      const next = await refreshCoordinator.refresh();
      updateCredential(next);
      return next;
    } catch (cause) {
      updateCredential({ kind: 'anonymous' });
      throw cause;
    }
  }, [refreshCoordinator, updateCredential]);
  const transport = useMemo(() => createMobileTransport({
    credential: () => credentialRef.current,
    refresh: refreshSession,
    tenantId: () => tenantId,
    locale: () => undefined,
  }), [refreshSession, tenantId]);
  const client = useMemo(() => createWeKnoraClient({ baseURL, transport }), [baseURL, transport]);

  useEffect(() => { let active = true; void adapter.read().then((next) => { if (active) updateCredential(next); }).finally(() => { if (active) setHydrating(false); }); return () => { active = false; }; }, [adapter, updateCredential]);
  useEffect(() => {
    const subscription = AppState.addEventListener('change', (state) => {
      if (state !== 'active' || credentialRef.current.kind !== 'bearer' || !credentialRef.current.refreshToken) return;
      void refreshSession().catch(() => undefined);
    });
    return () => subscription.remove();
  }, [refreshSession]);

  async function login(email: string, password: string) {
    const session = await client.auth.login({ email, password });
    const next: Credential = { kind: 'bearer', accessToken: session.token, refreshToken: session.refreshToken };
    await adapter.write(next);
    updateCredential(next);
  }
  async function logout() { await refreshCoordinator.logout(); updateCredential({ kind: 'anonymous' }); }

  return <RuntimeContext.Provider value={{ client, baseURL, credential, hydrating, login, logout }}>{children}</RuntimeContext.Provider>;
}

export function useMobileRuntime(): MobileRuntimeValue {
  const value = useContext(RuntimeContext);
  if (!value) throw new Error('useMobileRuntime must be used inside MobileRuntimeProvider');
  return value;
}
