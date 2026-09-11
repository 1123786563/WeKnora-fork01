import { AppState, Linking } from 'react-native';
import * as SecureStore from 'expo-secure-store';
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { createRefreshCoordinator, createWeKnoraClient, type AuthSession, type Credential, type WeKnoraClient } from '@weknora/api-client';
import { resolveMobileApiBaseUrl } from './platform/transport.ts';
import { createSecureCredentialAdapter } from './platform/credentials.ts';
import { createServerAddressAdapter } from './platform/server.ts';
import { createMobileTransport } from './platform/transport.ts';
import { createSingleFlight, createWorkspaceSelectionAdapter, parseMobileWorkspaces, shouldHydrateWorkspaceMemberships, toWorkspaceId, type MobileWorkspace } from './platform/workspace.ts';
import { parseMobileOIDCCallback } from './platform/oidc.ts';

const OIDC_STATE_KEY = 'weknora.mobile.oidc-state';

interface MobileRuntimeValue {
  client: WeKnoraClient;
  baseURL: string;
  credential: Credential;
  hydrating: boolean;
  oidcError: string;
  tenantId: string | null;
  workspaces: MobileWorkspace[];
  setServerAddress(value: string): Promise<void>;
  refreshWorkspaces(): Promise<void>;
  switchWorkspace(id: number): Promise<void>;
  register(username: string, email: string, password: string): Promise<void>;
  registerByInvite(token: string, username: string, email: string, password: string): Promise<void>;
  startOIDC(): Promise<void>;
  login(email: string, password: string): Promise<void>;
  logout(): Promise<void>;
}

const RuntimeContext = createContext<MobileRuntimeValue | null>(null);

export function MobileRuntimeProvider({ children }: { children: ReactNode }) {
  const adapter = useMemo(() => createSecureCredentialAdapter(), []);
  const serverAdapter = useMemo(() => createServerAddressAdapter(SecureStore), []);
  const workspaceAdapter = useMemo(() => createWorkspaceSelectionAdapter(SecureStore), []);
  const [credential, setCredential] = useState<Credential>({ kind: 'anonymous' });
  const credentialRef = useRef<Credential>({ kind: 'anonymous' });
  const [hydrating, setHydrating] = useState(true);
  const [baseURL, setBaseURL] = useState(() => resolveMobileApiBaseUrl(process.env.EXPO_PUBLIC_API_BASE_URL || ''));
  const [tenantId, setTenantId] = useState<string | null>(null);
  const [workspaces, setWorkspaces] = useState<MobileWorkspace[]>([]);
  const [oidcError, setOidcError] = useState('');
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

  const adoptSession = useCallback(async (session: AuthSession) => {
    const next: Credential = { kind: 'bearer', accessToken: session.token, refreshToken: session.refreshToken };
    await adapter.write(next);
    updateCredential(next);
    setWorkspaces(parseMobileWorkspaces(session.memberships));
    const activeTenantId = toWorkspaceId(session.tenant?.id);
    if (activeTenantId !== null) {
      setTenantId(String(activeTenantId));
      await workspaceAdapter.write(activeTenantId);
    }
  }, [adapter, updateCredential, workspaceAdapter]);

  useEffect(() => {
    let active = true;
    void Promise.all([adapter.read(), serverAdapter.read(), workspaceAdapter.read()]).then(([nextCredential, savedBaseURL, savedTenantId]) => {
      if (!active) return;
      updateCredential(nextCredential);
      if (savedBaseURL) setBaseURL(savedBaseURL);
      if (savedTenantId !== null) setTenantId(String(savedTenantId));
    }).finally(() => { if (active) setHydrating(false); });
    return () => { active = false; };
  }, [adapter, serverAdapter, updateCredential, workspaceAdapter]);
  useEffect(() => {
    const subscription = AppState.addEventListener('change', (state) => {
      if (state !== 'active' || credentialRef.current.kind !== 'bearer' || !credentialRef.current.refreshToken) return;
      void refreshSession().catch(() => undefined);
    });
    return () => subscription.remove();
  }, [refreshSession]);

  useEffect(() => {
    if (hydrating) return;
    let active = true;
    async function consume(raw: string | null) {
      if (!raw) return;
      const callback = parseMobileOIDCCallback(raw);
      if (!callback || !active) return;
      const expectedState = await SecureStore.getItemAsync(OIDC_STATE_KEY);
      await SecureStore.deleteItemAsync(OIDC_STATE_KEY);
      if (callback.state && expectedState && callback.state !== expectedState) {
        setOidcError('The OIDC callback state did not match this device.');
        return;
      }
      if (callback.kind === 'error') {
        setOidcError(callback.message);
        return;
      }
      try { await adoptSession(callback.session); setOidcError(''); }
      catch (cause) { setOidcError(cause instanceof Error ? cause.message : 'Unable to finish OIDC sign in'); }
    }
    void Linking.getInitialURL().then((url) => consume(url));
    const subscription = Linking.addEventListener('url', ({ url }) => { void consume(url); });
    return () => { active = false; subscription.remove(); };
  }, [adoptSession, hydrating]);

  async function login(email: string, password: string) {
    await adoptSession(await client.auth.login({ email, password }));
  }
  async function register(username: string, email: string, password: string) {
    await client.auth.register({ username, email, password });
  }
  async function registerByInvite(token: string, username: string, email: string, password: string) {
    await adoptSession(await client.auth.registerByInvite({ token, username, email, password }));
  }
  async function startOIDC() {
    setOidcError('');
    try {
      const config = await client.auth.oidcConfig();
      if (!config.enabled) throw new Error('Single sign-on is not enabled on this server');
      const { authorizationUrl, state } = await client.auth.oidcUrl('weknora://oidc');
      await SecureStore.setItemAsync(OIDC_STATE_KEY, state);
      await Linking.openURL(authorizationUrl);
    } catch (cause) {
      setOidcError(cause instanceof Error ? cause.message : 'Unable to start single sign-on');
      throw cause;
    }
  }
  async function setServerAddress(value: string) {
    const next = await serverAdapter.write(value);
    setBaseURL(next || resolveMobileApiBaseUrl(process.env.EXPO_PUBLIC_API_BASE_URL || ''));
  }
  const refreshWorkspaces = useMemo(() => createSingleFlight(async () => {
    const identity = await client.auth.me();
    setWorkspaces(parseMobileWorkspaces(identity.memberships));
    const activeTenantId = toWorkspaceId(identity.tenant?.id);
    setTenantId(activeTenantId === null ? null : String(activeTenantId));
    await workspaceAdapter.write(activeTenantId);
  }), [client, workspaceAdapter]);

  useEffect(() => {
    if (!shouldHydrateWorkspaceMemberships({ hydrating, credentialKind: credential.kind, workspaceCount: workspaces.length })) return;
    void refreshWorkspaces().catch(() => undefined);
  }, [credential.kind, hydrating, refreshWorkspaces, workspaces.length]);

  async function switchWorkspace(id: number) {
    const workspaceId = toWorkspaceId(id);
    if (workspaceId === null) throw new Error('workspace id must be a positive safe integer');
    const refreshToken = credentialRef.current.kind === 'bearer' ? credentialRef.current.refreshToken : undefined;
    const session = await client.auth.switchTenant(workspaceId, refreshToken);
    const next: Credential = { kind: 'bearer', accessToken: session.token, refreshToken: session.refreshToken };
    await adapter.write(next);
    updateCredential(next);
    setTenantId(String(workspaceId));
    await workspaceAdapter.write(workspaceId);
    if (session.memberships) setWorkspaces(parseMobileWorkspaces(session.memberships));
  }
  async function logout() { await refreshCoordinator.logout(); await workspaceAdapter.write(null); setWorkspaces([]); setTenantId(null); updateCredential({ kind: 'anonymous' }); }

  return <RuntimeContext.Provider value={{ client, baseURL, credential, hydrating, oidcError, tenantId, workspaces, setServerAddress, refreshWorkspaces, switchWorkspace, register, registerByInvite, startOIDC, login, logout }}>{children}</RuntimeContext.Provider>;
}

export function useMobileRuntime(): MobileRuntimeValue {
  const value = useContext(RuntimeContext);
  if (!value) throw new Error('useMobileRuntime must be used inside MobileRuntimeProvider');
  return value;
}
