import type { Credential } from '@weknora/api-client';
import type { ClientRequest } from '@weknora/api-client';
import type { MobileRuntime, RuntimeSnapshot } from '@weknora/mobile-core';
import type { SessionView } from '../core/auth.ts';
import { anonymous } from '../core/auth.ts';
import { ScopeGuard, scopeKey, type ScopeIdentity } from '../core/scope.ts';
import { readStoredCredential } from '../platform/credential-store.ts';
import type { ValueStore } from '../core/intent.ts';

export interface TaroSessionDeps {
  runtime: MobileRuntime;
  origin: string;
  /** wx.login code 注入缝：生产侧由 services/runtime 提供，测试侧注入桩。 */
  wxCode?: () => Promise<string>;
  storage: ValueStore;
}

export interface TaroSessionFacade {
  bootstrap(): Promise<void>;
  login(email: string, password: string): Promise<void>;
  /** 微信静默登录：code 的获取（wx.login）由 services/runtime 的薄适配完成，本层保持无 Taro 可测。 */
  wxLogin(): Promise<void>;
  switchTenant(id: number): Promise<void>;
  logout(): Promise<void>;
  snapshot(): SessionView;
  subscribe(listener: () => void): () => void;
  credential(): Credential;
  readonly scope: ScopeGuard;
  abortSubscriptions(): void;
}

interface MeEnrichment { userId: string; userName: string; memberships: unknown[] }

/**
 * SessionView 门面：MobileRuntime 是唯一会话编排器（登录/刷新/切租户/撤销），本门面只做
 * 「RuntimeSnapshot → 呈现视图」的投影与 UI 侧迟到拒绝（ScopeGuard 桥）。userName/成员
 * 角色不在 RuntimeSnapshot 里（presentation-safe），由授权通道 GET /auth/me 富集一次。
 */
export function createTaroSessionFacade(deps: TaroSessionDeps): TaroSessionFacade {
  const { runtime, origin, storage } = deps;
  const listeners = new Set<() => void>();
  const guard = new ScopeGuard({ origin, userId: null, tenantId: null });
  let enrichment: MeEnrichment | undefined;
  let enrichFlight: Promise<void> = Promise.resolve();
  let transition: 'idle' | 'loading' | 'switching' = 'idle';
  let view: SessionView = anonymous();

  const identityOf = (snapshot: RuntimeSnapshot): ScopeIdentity => ({
    origin,
    userId: snapshot.identity?.userId ?? null,
    tenantId: snapshot.identity?.activeTenantId ?? null,
  });

  const publish = (): void => { for (const fn of listeners) fn(); };

  const fallbackMemberships = (snapshot: RuntimeSnapshot): unknown[] =>
    (snapshot.identity?.tenants ?? []).map(tenant => ({ tenant_id: tenant.id, tenant_name: tenant.name ?? `工作空间 ${tenant.id}`, role: '成员' }));

  const project = (snapshot: RuntimeSnapshot): SessionView => {
    if (snapshot.surface === 'authorized' && snapshot.identity) {
      const identity = snapshot.identity;
      const tenantName = identity.tenants?.find(tenant => tenant.id === identity.activeTenantId)?.name ?? '';
      return {
        phase: 'ready',
        userId: identity.userId,
        userName: enrichment?.userId === identity.userId ? enrichment.userName : '',
        tenantId: identity.activeTenantId ?? null,
        tenantName,
        memberships: enrichment?.userId === identity.userId ? enrichment.memberships : fallbackMemberships(snapshot),
      };
    }
    if (snapshot.surface === 'read-only') return { ...anonymous(), phase: 'error', error: '服务端当前为只读模式，请稍后重试或到 Web 工作台操作' };
    if (snapshot.surface === 'upgrade-required') return { ...anonymous(), phase: 'error', error: '服务端协议版本不兼容，请更新小程序' };
    return anonymous();
  };

  /** 身份富集：authorized 后经授权通道读一次 /auth/me；迟到（登出/换身份）即丢弃。 */
  const enrichIdentity = (userId: string): void => {
    if (enrichment?.userId === userId) return;
    enrichFlight = enrichFlight.then(async () => {
      try {
        const raw = await runtime.authorizedRequest({ method: 'GET', path: '/api/v1/auth/me' });
        const snapshot = runtime.snapshot();
        if (snapshot.surface !== 'authorized' || snapshot.identity?.userId !== userId) return;
        const record = (typeof raw === 'object' && raw !== null ? raw : {}) as Record<string, unknown>;
        const data = (typeof record.data === 'object' && record.data !== null ? record.data : {}) as Record<string, unknown>;
        const user = (typeof data.user === 'object' && data.user !== null ? data.user : {}) as Record<string, unknown>;
        enrichment = {
          userId,
          userName: String(user.username ?? user.name ?? user.email ?? '用户'),
          memberships: Array.isArray(data.memberships) ? data.memberships : [],
        };
        applySnapshot(runtime.snapshot());
      } catch { /* 展示名缺失不致命：视图以空名渲染，绝不伪造 */ }
    });
  };

  let lastSnapshot: RuntimeSnapshot = runtime.snapshot();

  const applySnapshot = (snapshot: RuntimeSnapshot): void => {
    const next = identityOf(snapshot);
    if (scopeKey(next) !== scopeKey(identityOf(lastSnapshot))) guard.switchTo(next);
    lastSnapshot = snapshot;
    view = transition === 'idle' ? project(snapshot) : { ...project(snapshot), phase: transition };
    if (snapshot.surface === 'authorized' && snapshot.identity) enrichIdentity(snapshot.identity.userId);
    publish();
  };

  runtime.subscribe(applySnapshot);

  const facade: TaroSessionFacade = {
    bootstrap: async () => {
      const hadCredential = readStoredCredential(storage, origin) !== undefined;
      transition = 'loading'; view = { ...project(lastSnapshot), phase: 'loading' }; publish();
      await runtime.boot({ origin });
      transition = 'idle';
      applySnapshot(runtime.snapshot());
      // 存有凭据但 boot 后仍是匿名且凭据未被清除 => 网络不可达，给可重试错误而非静默登出。
      if (hadCredential && view.phase === 'anonymous' && readStoredCredential(storage, origin) !== undefined) {
        view = { ...view, phase: 'error', error: '无法验证登录状态，请检查网络后重试' };
        publish();
      }
    },
    login: async (email, password) => {
      if (!email.trim() || !password) throw new Error('请输入邮箱和密码');
      transition = 'loading'; guard.invalidate(); view = { ...view, phase: 'loading' }; publish();
      try {
        await runtime.signIn({ deployment: { origin }, email: email.trim(), password });
      } finally {
        transition = 'idle';
        applySnapshot(runtime.snapshot());
      }
    },
    wxLogin: async () => {
      transition = 'loading'; guard.invalidate(); view = { ...view, phase: 'loading' }; publish();
      try {
        const code = deps.wxCode ? await deps.wxCode() : '';
        if (!code) throw new Error('WECHAT_CODE_UNAVAILABLE');
        await runtime.wxSignIn({ deployment: { origin }, code });
      } finally {
        transition = 'idle';
        applySnapshot(runtime.snapshot());
      }
    },
    switchTenant: async (id) => {
      if (view.phase !== 'ready' || !Number.isSafeInteger(id) || id <= 0) throw new Error('工作空间不可切换');
      transition = 'switching'; guard.invalidate(); view = { ...view, phase: 'switching' }; publish();
      try {
        await runtime.activateTenant(String(id));
      } finally {
        transition = 'idle';
        applySnapshot(runtime.snapshot());
      }
    },
    logout: async () => { await runtime.signOut(); },
    snapshot: () => view,
    subscribe: (listener) => { listeners.add(listener); return () => listeners.delete(listener); },
    credential: () => {
      const stored = readStoredCredential(storage, origin);
      return stored ? { kind: 'bearer', accessToken: stored.token, refreshToken: stored.refreshToken } : { kind: 'anonymous' };
    },
    scope: guard,
    abortSubscriptions: () => guard.abortAll(),
  };
  return facade;
}
