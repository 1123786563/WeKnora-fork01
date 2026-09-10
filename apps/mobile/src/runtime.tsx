import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { createWeKnoraClient, type Credential, type WeKnoraClient } from '@weknora/api-client';
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
  const [hydrating, setHydrating] = useState(true);
  const [tenantId] = useState<string | null>(null);
  const baseURL = resolveMobileApiBaseUrl(process.env.EXPO_PUBLIC_API_BASE_URL || '');
  const client = useMemo(() => createWeKnoraClient({ baseURL, transport: createMobileTransport({ credential: () => credential, tenantId: () => tenantId, locale: () => undefined }) }), [baseURL, credential, tenantId]);

  useEffect(() => { let active = true; void adapter.read().then((next) => { if (active) setCredential(next); }).finally(() => { if (active) setHydrating(false); }); return () => { active = false; }; }, [adapter]);

  async function login(email: string, password: string) {
    const session = await client.auth.login({ email, password });
    const next: Credential = { kind: 'bearer', accessToken: session.token, refreshToken: session.refreshToken };
    await adapter.write(next);
    setCredential(next);
  }
  async function logout() { await adapter.clear(); setCredential({ kind: 'anonymous' }); }

  return <RuntimeContext.Provider value={{ client, baseURL, credential, hydrating, login, logout }}>{children}</RuntimeContext.Provider>;
}

export function useMobileRuntime(): MobileRuntimeValue {
  const value = useContext(RuntimeContext);
  if (!value) throw new Error('useMobileRuntime must be used inside MobileRuntimeProvider');
  return value;
}
