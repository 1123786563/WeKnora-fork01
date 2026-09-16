import { AppState, Linking } from 'react-native';
import NetInfo from '@react-native-community/netinfo';
import * as SecureStore from 'expo-secure-store';
import * as Crypto from 'expo-crypto';
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { AuthError, createRefreshCoordinator, createWeKnoraClient, type AuthSession, type Credential, type WeKnoraClient } from '@weknora/api-client';
import { resolveMobileApiBaseUrl } from './platform/transport.ts';
import { createSecureCredentialAdapter } from './platform/credentials.ts';
import { createServerAddressAdapter } from './platform/server.ts';
import { createMobileTransport } from './platform/transport.ts';
import { createNetworkRecovery } from './platform/network.ts';
import { createLatestAsyncWriter, createSessionEpoch, createSingleFlight, createWorkspaceSelectionAdapter, parseMobileWorkspaces, resetMobileSessionState, shouldHydrateWorkspaceMemberships, shouldRefreshMobileSession, toWorkspaceId, type MobileWorkspace } from './platform/workspace.ts';
import { createMobileOIDCPKCE, matchesMobileOIDCState, MOBILE_OIDC_REDIRECT, parseMobileOIDCCallback } from './platform/oidc.ts';
import { isLocale, type Locale } from '@weknora/i18n';

const OIDC_STATE_KEY = 'weknora.mobile.oidc-state';
const OIDC_VERIFIER_KEY = 'weknora.mobile.oidc-verifier';

interface MobileRuntimeValue {
  client: WeKnoraClient;
  baseURL: string;
  credential: Credential;
  hydrating: boolean;
  oidcError: string;
  locale: Locale;
  setLocale(value: Locale): Promise<void>;
  userId: string | null;
  tenantId: string | null;
  workspaces: MobileWorkspace[];
  canCreateTenant: boolean;
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
  const sessionEpoch = useMemo(() => createSessionEpoch(), []);
  const [credential, setCredential] = useState<Credential>({ kind: 'anonymous' });
  const credentialRef = useRef<Credential>({ kind: 'anonymous' });
  const [hydrating, setHydrating] = useState(true);
  const [baseURL, setBaseURL] = useState(() => resolveMobileApiBaseUrl(process.env.EXPO_PUBLIC_API_BASE_URL || ''));
  const [tenantId, setTenantId] = useState<string | null>(null);
  const tenantIdRef = useRef<string | null>(null);
  const [userId, setUserId] = useState<string | null>(null);
  const [workspaces, setWorkspaces] = useState<MobileWorkspace[]>([]);
  const [canCreateTenant, setCanCreateTenant] = useState(false);
  const [oidcError, setOidcError] = useState('');
  const [locale, setCurrentLocale] = useState<Locale>('zh-CN');
  const setLocale = useCallback(async (value: Locale) => {
    setCurrentLocale(value);
    await SecureStore.setItemAsync('locale', value);
  }, []);
  const sessionTransitions = useRef(0);
  const appActiveRef = useRef(AppState.currentState === 'active');
  const updateCredential = useCallback((next: Credential) => {
    credentialRef.current = next;
    setCredential(next);
  }, []);
  const updateTenantId = useCallback((next: string | null) => {
    tenantIdRef.current = next;
    setTenantId(next);
  }, []);
  const updateUserId = useCallback((next: string | null) => {
    setUserId(next);
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
  const workspaceWriter = useMemo(() => createLatestAsyncWriter((value: number | null) => workspaceAdapter.write(value)), [workspaceAdapter]);
  const refreshSession = useCallback(async () => {
    const currentCredential = credentialRef.current;
    if (!shouldRefreshMobileSession({
      transitionCount: sessionTransitions.current,
      credentialKind: currentCredential.kind,
      hasRefreshToken: currentCredential.kind === 'bearer' && Boolean(currentCredential.refreshToken),
    })) {
      throw new AuthError('AUTH_INVALIDATED', 'The credential cannot be refreshed during a session transition');
    }
    const startedAt = sessionEpoch.current();
    try {
      const next = await refreshCoordinator.refresh();
      if (!sessionEpoch.isCurrent(startedAt)) throw new AuthError('AUTH_INVALIDATED', 'The credential was invalidated during refresh');
      updateCredential(next);
      return next;
    } catch (cause) {
      if (sessionEpoch.isCurrent(startedAt)) {
        // A refresh failure invalidates the whole authenticated scope. Clear
        // the in-memory scope immediately so pins/list screens cannot hydrate
        // the old user or tenant while the persisted selection is removed.
        const failedSession = sessionEpoch.invalidate();
        if (sessionEpoch.isCurrent(failedSession)) {
          resetMobileSessionState({ updateCredential, updateUserId, updateTenantId, setWorkspaces, setCanCreateTenant });
          await workspaceWriter.write(null).catch(() => undefined);
        }
      }
      throw cause;
    }
  }, [refreshCoordinator, sessionEpoch, updateCredential, updateTenantId, updateUserId, workspaceWriter]);
  const transport = useMemo(() => createMobileTransport({
    credential: () => credentialRef.current,
    refresh: refreshSession,
    tenantId: () => tenantIdRef.current,
    isTransitioning: () => sessionTransitions.current > 0,
    locale: () => locale,
  }), [locale, refreshSession]);
  const client = useMemo(() => createWeKnoraClient({ baseURL, transport }), [baseURL, transport]);

  const adoptSession = useCallback(async (session: AuthSession) => {
    sessionTransitions.current += 1;
    const startedAt = sessionEpoch.invalidate();
    try {
      await refreshCoordinator.invalidate({ clear: false });
      if (!sessionEpoch.isCurrent(startedAt)) throw new AuthError('AUTH_INVALIDATED', 'The session was superseded before it could be adopted');
      const next: Credential = { kind: 'bearer', accessToken: session.token, refreshToken: session.refreshToken };
      await refreshCoordinator.write(next);
      if (!sessionEpoch.isCurrent(startedAt)) throw new AuthError('AUTH_INVALIDATED', 'The session was superseded while it was being adopted');
      const activeTenantId = toWorkspaceId(session.tenant?.id);
      await workspaceWriter.write(activeTenantId);
      if (!sessionEpoch.isCurrent(startedAt)) throw new AuthError('AUTH_INVALIDATED', 'The session was superseded while its workspace was being stored');
      setCanCreateTenant(false);
      updateCredential(next);
      const sessionUserId = typeof session.user?.id === 'string' && session.user.id.trim() ? session.user.id : null;
      updateUserId(sessionUserId);
      setWorkspaces(parseMobileWorkspaces(session.memberships));
      updateTenantId(activeTenantId === null ? null : String(activeTenantId));
    } finally {
      sessionTransitions.current -= 1;
    }
  }, [refreshCoordinator, sessionEpoch, updateCredential, updateTenantId, updateUserId, workspaceWriter]);

  useEffect(() => {
    let active = true;
    const startedAt = sessionEpoch.current();
    void Promise.all([adapter.read(), serverAdapter.read(), workspaceAdapter.read(), SecureStore.getItemAsync('locale')]).then(([nextCredential, savedBaseURL, savedTenantId, savedLocale]) => {
      if (!active || !sessionEpoch.isCurrent(startedAt) || sessionTransitions.current > 0) return;
      updateCredential(nextCredential);
      if (savedBaseURL) setBaseURL(savedBaseURL);
      if (savedTenantId !== null) updateTenantId(String(savedTenantId));
      if (savedLocale && isLocale(savedLocale)) setCurrentLocale(savedLocale);
    }).finally(() => { if (active) setHydrating(false); });
    return () => { active = false; };
  }, [adapter, serverAdapter, sessionEpoch, updateCredential, updateTenantId, workspaceAdapter]);
  useEffect(() => {
    const subscription = AppState.addEventListener('change', (state) => {
      appActiveRef.current = state === 'active';
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
      const codeVerifier = await SecureStore.getItemAsync(OIDC_VERIFIER_KEY);
      await SecureStore.deleteItemAsync(OIDC_STATE_KEY);
      await SecureStore.deleteItemAsync(OIDC_VERIFIER_KEY);
      if (!matchesMobileOIDCState(callback, expectedState)) {
        setOidcError('The OIDC callback state did not match this device.');
        return;
      }
      if (callback.kind === 'error') {
        setOidcError(callback.message);
        return;
      }
      try {
        if (callback.kind !== 'code') return;
        if (!codeVerifier) throw new Error('The OIDC PKCE verifier is missing on this device.');
        await adoptSession(await client.auth.oidcExchange(callback.code, callback.state, codeVerifier));
        setOidcError('');
      }
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
      const redirectURI = `${baseURL.replace(/\/+$/, '')}/api/v1/auth/oidc/callback`;
      const pkce = await createMobileOIDCPKCE({
        getRandomBytesAsync: Crypto.getRandomBytesAsync,
        digestStringAsync: (algorithm, data, options) => Crypto.digestStringAsync(
          algorithm as Crypto.CryptoDigestAlgorithm,
          data,
          { encoding: options.encoding as Crypto.CryptoEncoding },
        ),
      });
      const { authorizationUrl, state } = await client.auth.oidcUrl(redirectURI, MOBILE_OIDC_REDIRECT, pkce.challenge);
      await SecureStore.setItemAsync(OIDC_STATE_KEY, state);
      await SecureStore.setItemAsync(OIDC_VERIFIER_KEY, pkce.verifier);
      await Linking.openURL(authorizationUrl);
    } catch (cause) {
      setOidcError(cause instanceof Error ? cause.message : 'Unable to start single sign-on');
      throw cause;
    }
  }
  async function setServerAddress(value: string) {
    if (value.trim() && !resolveMobileApiBaseUrl(value)) throw new Error('Server address must use HTTP(S)');
    sessionTransitions.current += 1;
    const startedAt = sessionEpoch.invalidate();
    try {
      updateCredential({ kind: 'anonymous' });
      updateUserId(null);
      updateTenantId(null);
      setWorkspaces([]);
      setCanCreateTenant(false);
      await refreshCoordinator.invalidate();
      await workspaceWriter.write(null);
      const next = await serverAdapter.write(value);
      if (!sessionEpoch.isCurrent(startedAt)) return;
      setBaseURL(next || resolveMobileApiBaseUrl(process.env.EXPO_PUBLIC_API_BASE_URL || ''));
    } finally {
      sessionTransitions.current -= 1;
    }
  }
  const refreshWorkspaces = useMemo(() => createSingleFlight(async () => {
    if (sessionTransitions.current > 0) return;
    const startedAt = sessionEpoch.current();
    const identity = await client.auth.me();
    if (!sessionEpoch.isCurrent(startedAt) || credentialRef.current.kind !== 'bearer' || sessionTransitions.current > 0) return;
    const activeTenantId = toWorkspaceId(identity.tenant?.id);
    const nextUserId = identity.user.id;
    const nextTenantId = activeTenantId === null ? null : String(activeTenantId);
    const nextWorkspaces = parseMobileWorkspaces(identity.memberships);
    const nextCanCreateTenant = identity.capabilities?.can_create_tenant === true;
    await workspaceWriter.write(activeTenantId);
    if (!sessionEpoch.isCurrent(startedAt) || credentialRef.current.kind !== 'bearer') return;
    // Commit all identity-derived state only after the storage write and the
    // final epoch check. A superseded auth/me response must not leak even a
    // transient user/tenant/workspace combination into the UI.
    updateUserId(nextUserId);
    setCanCreateTenant(nextCanCreateTenant);
    setWorkspaces(nextWorkspaces);
    updateTenantId(nextTenantId);
  }), [client, sessionEpoch, updateTenantId, updateUserId, workspaceWriter]);

  useEffect(() => createNetworkRecovery({
    subscribe: (listener) => NetInfo.addEventListener(listener),
    onReconnect: () => {
      if (!appActiveRef.current || credentialRef.current.kind !== 'bearer' || !credentialRef.current.refreshToken) return;
      void refreshSession().catch(() => undefined);
    },
  }), [refreshSession]);

  useEffect(() => {
    if (!shouldHydrateWorkspaceMemberships({ hydrating, credentialKind: credential.kind, workspaceCount: workspaces.length })
      && !(hydrating === false && credential.kind === 'bearer' && userId === null)) return;
    void refreshWorkspaces().catch(() => undefined);
  }, [credential.kind, hydrating, refreshWorkspaces, userId, workspaces.length]);

  async function switchWorkspace(id: number) {
    const workspaceId = toWorkspaceId(id);
    if (workspaceId === null) throw new Error('workspace id must be a positive safe integer');
    sessionTransitions.current += 1;
    const startedAt = sessionEpoch.invalidate();
    try {
      await refreshCoordinator.invalidate({ clear: false });
      if (!sessionEpoch.isCurrent(startedAt)) throw new AuthError('AUTH_INVALIDATED', 'The workspace switch was superseded');
      const refreshToken = credentialRef.current.kind === 'bearer' ? credentialRef.current.refreshToken : undefined;
      const session = await client.auth.switchTenant(workspaceId, refreshToken);
      if (!sessionEpoch.isCurrent(startedAt)) throw new AuthError('AUTH_INVALIDATED', 'The workspace switch was superseded');
      const next: Credential = { kind: 'bearer', accessToken: session.token, refreshToken: session.refreshToken };
      await refreshCoordinator.write(next);
      if (!sessionEpoch.isCurrent(startedAt)) throw new AuthError('AUTH_INVALIDATED', 'The workspace switch was superseded while storing credentials');
      await workspaceWriter.write(workspaceId);
      if (!sessionEpoch.isCurrent(startedAt)) throw new AuthError('AUTH_INVALIDATED', 'The workspace switch was superseded while storing its selection');
      updateCredential(next);
      updateTenantId(String(workspaceId));
      if (session.memberships) setWorkspaces(parseMobileWorkspaces(session.memberships));
    } finally {
      sessionTransitions.current -= 1;
    }
  }
  async function logout() {
    sessionTransitions.current += 1;
    const startedAt = sessionEpoch.invalidate();
    try {
      await refreshCoordinator.logout();
      await workspaceWriter.write(null);
      if (!sessionEpoch.isCurrent(startedAt)) return;
      setWorkspaces([]);
      setCanCreateTenant(false);
      updateTenantId(null);
      updateUserId(null);
      updateCredential({ kind: 'anonymous' });
    } finally {
      sessionTransitions.current -= 1;
    }
  }

  return <RuntimeContext.Provider value={{ client, baseURL, credential, hydrating, oidcError, locale, setLocale, userId, tenantId, workspaces, canCreateTenant, setServerAddress, refreshWorkspaces, switchWorkspace, register, registerByInvite, startOIDC, login, logout }}>{children}</RuntimeContext.Provider>;
}

export function useMobileRuntime(): MobileRuntimeValue {
  const value = useContext(RuntimeContext);
  if (!value) throw new Error('useMobileRuntime must be used inside MobileRuntimeProvider');
  return value;
}
