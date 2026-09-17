import { createAuthApi, createJsonTransport, createProductAuthSession, type BearerCredential } from '@weknora/api-client';
import { createBootstrapPort, type MembershipSummary } from './bootstrap';
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
  loading: boolean;
  scope: ProductScope;
  login: (email: string, password: string) => Promise<void>;
  replaceCredential: (credential: BearerCredential) => Promise<void>;
  logout: () => Promise<void>;
  /** 冷启动/换取后的身份引导：凭据有效但 scope 未定时取 user/memberships（G04）。 */
  bootstrapScope: () => Promise<void>;
  bootstrapError: string | null;
  /** 最近一次身份引导取得的空间成员关系（冷启动/SSO 共用；MX-011 选择屏消费）。 */
  bootstrapMemberships: MembershipSummary[];
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
  // 身份引导端口：真实 /auth/me（经同一 fetch 通道，凭据由 transport 头注入由调用方适配）
  const bootstrap = React.useMemo(() => {
    if (!host) return null;
    return createBootstrapPort(createAuthApi((async (input: { method: string; path: string; body?: unknown; headers?: Record<string, string>; signal?: AbortSignal }) => {
      const credential = await adapter?.read();
      const token = credential && credential.kind === 'bearer' ? credential.accessToken : '';
      const response = await fetch(`${host.origin}${input.path}`, {
        method: input.method,
        headers: { 'content-type': 'application/json', ...(token ? { authorization: `Bearer ${token}` } : {}), ...(input.headers ?? {}) },
        ...(input.body === undefined ? {} : { body: JSON.stringify(input.body) }),
        ...(input.signal === undefined ? {} : { signal: input.signal }),
      });
      return (await response.json()) as unknown;
    }) as never));
  }, [adapter, host]);
  const [bootstrapError, setBootstrapError] = React.useState<string | null>(null);
  const [bootstrapMemberships, setBootstrapMemberships] = React.useState<MembershipSummary[]>([]);
  const bootstrapErrorRef = React.useRef<string | null>(null);

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

  const bootstrapScopeNow = React.useCallback(async () => {
    if (!host || !bootstrap) return;
    try {
      const outcome = await bootstrap.run(scope.identity().tenantId);
      bootstrapErrorRef.current = null;
      setBootstrapError(null);
      setBootstrapMemberships(outcome.memberships);
      scope.switchTo({ origin: host.origin, userId: outcome.userId, tenantId: outcome.tenantId ?? '' });
    } catch (error) {
      // 保留凭据；scope 维持未定（登录门可达），错误暴露给 UI 层
      const message = error instanceof Error ? error.message : 'identity bootstrap failed';
      bootstrapErrorRef.current = message;
      setBootstrapError(message);
    }
  }, [bootstrap, host, scope]);

  React.useEffect(() => {
    let active = true;
    setLoading(Boolean(adapter));
    setCredential(null);
    setBootstrapError(null);
    if (adapter) adapter.read().then(async (value) => {
      if (!active) return;
      if (value.kind !== 'bearer') return;
      setCredential(value);
      // 冷启动不只恢复 Token：scope 未定（缺 userId/tenantId）即以真实 /auth/me 补齐
      const identity = scope.identity();
      if (!identity.userId && !identity.tenantId) await bootstrapScopeNow();
    }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
    // bootstrapScopeNow 经 ref 稳定；scope 读数在冷启动时不订阅变化
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [adapter, host]);

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
    // SSO 换取成功同样必须补齐身份/成员关系，再进入产品 scope；失败上抛由回跳页决策
    await bootstrapScopeNow();
    if (bootstrapErrorRef.current) throw new Error(bootstrapErrorRef.current);
  }, [authSession, bootstrapScopeNow, host]);

  return <ProductAuthContext.Provider value={{ credential, loading, scope, login, logout, replaceCredential, bootstrapScope: bootstrapScopeNow, bootstrapError, bootstrapMemberships }}>{children}</ProductAuthContext.Provider>;
}

export function useProductAuth(): ProductAuthContextValue {
  const value = React.useContext(ProductAuthContext);
  if (!value) throw new Error('PRODUCT_AUTH_PROVIDER_REQUIRED');
  return value;
}
