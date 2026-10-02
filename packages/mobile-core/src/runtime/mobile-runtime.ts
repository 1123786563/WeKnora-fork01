import { clientGate, validateAuthReturn } from '@weknora/domain/mobile';
import type { MobileRuntimePorts, StoredCredential } from './ports.ts';
import type { ScopedVault, VaultRevokeReason } from '../vault/scoped-vault.ts';
import { RuntimeScopeLease } from './scope-lease.ts';
import { createResourceShelf } from '../shelf/resource-shelf.ts';
import type { ResourceShelfHandle } from '../shelf/types.ts';
import type { Deployment, DeploymentInput, MobileRuntime, RuntimeAuthorizedRequest, RuntimeReason, RuntimeSnapshot, RuntimeSurface, ScopeLease } from './types.ts';

function normalizeDeployment(input: DeploymentInput): Deployment {
  if (!input || typeof input.origin !== 'string' || input.origin.trim() === '') throw new Error('deployment origin is required');
  let parsed: URL;
  try {
    parsed = new URL(input.origin);
  } catch {
    throw new Error('deployment origin must be an absolute URL');
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.search || parsed.hash || parsed.pathname !== '/') {
    throw new Error('deployment must be an HTTPS origin');
  }
  const origin = parsed.origin;
  return { origin, label: typeof input.label === 'string' && input.label.trim() !== '' ? input.label.trim() : origin };
}

function userId(value: unknown): string | undefined {
  const id = typeof value === 'object' && value !== null ? (value as { id?: unknown }).id : undefined;
  return typeof id === 'string' && id.trim() !== '' ? id : undefined;
}

function tenantId(value: unknown): string | undefined {
  const id = typeof value === 'object' && value !== null ? (value as { id?: unknown }).id : undefined;
  if (typeof id === 'string' && id.trim() !== '') return id.trim(); // R1-F33：字符串 id 归一（与 membershipTenantId 对齐）
  return typeof id === 'number' && Number.isSafeInteger(id) && id > 0 ? String(id) : undefined;
}

function membershipTenantId(value: unknown): string | undefined {
  const id = typeof value === 'object' && value !== null ? (value as { tenant_id?: unknown }).tenant_id : undefined;
  if (typeof id === 'string' && id.trim() !== '') return id.trim();
  return typeof id === 'number' && Number.isSafeInteger(id) && id > 0 ? String(id) : undefined;
}

function membershipTenantName(value: unknown): string | undefined {
  const name = typeof value === 'object' && value !== null ? (value as { tenant_name?: unknown }).tenant_name : undefined;
  return typeof name === 'string' && name.trim() !== '' ? name.trim() : undefined;
}

function tenantOptions(memberships: unknown, activeTenantId: string): { tenants: Array<{ id: string; name?: string }> } {
  if (!Array.isArray(memberships)) return { tenants: [{ id: activeTenantId }] };
  const tenants: Array<{ id: string; name?: string }> = [];
  for (const membership of memberships) {
    const id = membershipTenantId(membership);
    if (!id || tenants.some((option) => option.id === id)) continue;
    const name = membershipTenantName(membership);
    tenants.push(name ? { id, name } : { id });
  }
  if (!tenants.some((option) => option.id === activeTenantId)) tenants.unshift({ id: activeTenantId });
  return { tenants };
}

function base64Url(bytes: Uint8Array): string {
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function randomBytes(ports: MobileRuntimePorts, size: number): Uint8Array {
  const injected = ports.randomBytes?.(size);
  if (injected) {
    if (injected.length !== size) throw new Error('OIDC_RANDOM');
    return injected;
  }
  const bytes = new Uint8Array(size);
  if (!globalThis.crypto?.getRandomValues) throw new Error('OIDC_RANDOM');
  return globalThis.crypto.getRandomValues(bytes);
}

async function codeChallenge(verifier: string): Promise<string> {
  if (!globalThis.crypto?.subtle) throw new Error('OIDC_CRYPTO');
  const bytes = new TextEncoder().encode(verifier);
  return base64Url(new Uint8Array(await globalThis.crypto.subtle.digest('SHA-256', bytes)));
}

function serverOidcCallback(deployment: Deployment): string {
  return `${deployment.origin}/api/v1/auth/oidc/callback`;
}

function unauthorizedStatus(error: unknown): boolean {
  return typeof error === 'object' && error !== null &&
    (error as { name?: unknown }).name === 'ApiError' &&
    (error as { status?: unknown }).status === 401;
}

export function createMobileRuntime(ports: MobileRuntimePorts): MobileRuntime {
  let epoch = 0;
  let activeDeployment: Deployment | undefined;
  let lease: ScopeLease | undefined;
  let revocableLease: RuntimeScopeLease | undefined;
  let activeShelf: ResourceShelfHandle | undefined;
  let activeCredential: StoredCredential | undefined;
  let oidcCompletion: Promise<RuntimeSnapshot> | undefined;
  const claimedOidcCallbacks = new Set<string>();
  let deploymentMutation: Promise<void> = Promise.resolve();
  let credentialMutation: Promise<void> = Promise.resolve();
  let pendingOidcMutation: Promise<void> = Promise.resolve();
  const refreshFlights = new Map<string, { requestEpoch: number; promise: Promise<StoredCredential | undefined> }>();
  // 在途授权流（R1-F32）：scope 撤销时全部 abort——SSE 挂在服务器侧不出 chunk 就不会回到
  // guardedChunk 的 epoch 检查，必须由 runtime 主动断开传输层。
  const activeStreams = new Set<AbortController>();
  let state: RuntimeSnapshot = { surface: 'deployment-login', reason: 'authentication-required' };
  const listeners = new Set<(snapshot: RuntimeSnapshot) => void>();

  const publish = (next: RuntimeSnapshot): RuntimeSnapshot => {
    state = next;
    for (const listener of listeners) listener(state);
    return state;
  };
  let vaultTail: Promise<void> = Promise.resolve();
  const queueVaultRevoke = (reason: VaultRevokeReason): void => {
    const expired = revocableLease;
    const vault = ports.scopedVault;
    if (!expired || !vault) return;
    const attempt = (): Promise<void> => vault.revoke(expired.asScopeLease(), reason);
    const next = vaultTail.then(attempt, attempt);
    vaultTail = next.then(() => {}, () => {});
  };
  const revoke = (vaultReason: VaultRevokeReason): void => {
    for (const controller of activeStreams) controller.abort();
    activeStreams.clear();
    queueVaultRevoke(vaultReason);
    revocableLease?.revoke();
    revocableLease = undefined;
    lease = undefined;
    activeShelf?.close(vaultReason);
    activeShelf = undefined;
    activeCredential = undefined;
  };
  const begin = (deployment: Deployment, vaultReason: VaultRevokeReason = 'deployment-change'): number => {
    epoch += 1;
    revoke(vaultReason);
    activeDeployment = deployment;
    return epoch;
  };
  const reserve = (vaultReason: VaultRevokeReason = 'deployment-change'): number => {
    epoch += 1;
    revoke(vaultReason);
    activeDeployment = undefined;
    return epoch;
  };
  const current = (requestEpoch: number, deployment: Deployment): boolean => requestEpoch === epoch && activeDeployment?.origin === deployment.origin;
  const mutate = <T>(tail: Promise<void>, setTail: (next: Promise<void>) => void, mutation: () => Promise<T>): Promise<T> => {
    const next = tail.then(mutation, mutation);
    setTail(next.then(() => {}, () => {}));
    return next;
  };
  const mutateDeployment = <T>(mutation: () => Promise<T>): Promise<T> =>
    mutate(deploymentMutation, (next) => { deploymentMutation = next; }, mutation);
  const mutateCredential = <T>(mutation: () => Promise<T>): Promise<T> =>
    mutate(credentialMutation, (next) => { credentialMutation = next; }, mutation);
  const mutatePendingOidc = <T>(mutation: () => Promise<T>): Promise<T> =>
    mutate(pendingOidcMutation, (next) => { pendingOidcMutation = next; }, mutation);
  const persistCredential = async (requestEpoch: number, deployment: Deployment, credential: StoredCredential): Promise<boolean> =>
    mutateCredential(async () => {
      if (!current(requestEpoch, deployment)) return false;
      await ports.credentialStore.write(deployment.origin, credential);
      return current(requestEpoch, deployment);
    });
  const safe = (requestEpoch: number, deployment: Deployment, reason: RuntimeReason): RuntimeSnapshot =>
    current(requestEpoch, deployment) ? publish({ surface: 'upgrade-required', deployment, reason }) : state;

  const refreshedCredential = (requestEpoch: number, deployment: Deployment, credential: StoredCredential): Promise<StoredCredential | undefined> => {
    const flight = refreshFlights.get(deployment.origin);
    if (flight?.requestEpoch === requestEpoch) return flight.promise;
    let promise!: Promise<StoredCredential | undefined>;
    promise = (async (): Promise<StoredCredential | undefined> => {
      try {
        const rotated = await ports.remoteFor(deployment.origin).refresh(credential.refreshToken);
        if (typeof rotated.access_token !== 'string' || rotated.access_token.trim() === '' || typeof rotated.refresh_token !== 'string' || rotated.refresh_token.trim() === '') return undefined;
        const next = { token: rotated.access_token, refreshToken: rotated.refresh_token };
        return await persistCredential(requestEpoch, deployment, next) ? next : undefined;
      } catch {
        return undefined;
      } finally {
        if (refreshFlights.get(deployment.origin)?.promise === promise) refreshFlights.delete(deployment.origin);
      }
    })();
    refreshFlights.set(deployment.origin, { requestEpoch, promise });
    return promise;
  };
  /** Shared authorized-send core: reads the active credential, guards scope at every step, and replays exactly once through a single-flight refresh on a pre-send 401. */
  const sendWithCredential = async <R>(requestEpoch: number, deployment: Deployment, send: (token: string) => Promise<R>, retryUnauthorized = true): Promise<R> => {
    const credential = await ports.credentialStore.read(deployment.origin);
    if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
    if (!credential) throw new Error('RUNTIME_UNAUTHORIZED');
    try {
      const response = await send(credential.token);
      if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
      return response;
    } catch (error) {
      if (!unauthorizedStatus(error)) throw error;
      if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
      if (!retryUnauthorized) throw error;
      const refreshed = await refreshedCredential(requestEpoch, deployment, credential);
      if (!refreshed) throw new Error('RUNTIME_UNAUTHORIZED');
      if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
      const retried = await send(refreshed.token);
      if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
      return retried;
    }
  };
  const accessTokenFor = async (origin: string, options?: { refresh?: boolean }): Promise<string> => {
    const deployment = activeDeployment;
    if (!deployment || deployment.origin !== origin || (state.surface !== 'authorized' && state.surface !== 'read-only') || !activeCredential) throw new Error('SHELF_SCOPE');
    if (!options?.refresh) return activeCredential.token;
    const requestEpoch = epoch;
    const refreshed = await refreshedCredential(requestEpoch, deployment, activeCredential);
    if (!refreshed) throw new Error('SHELF_AUTH');
    // R1-F31：refresh 跨越了 scope 变化（切部署/切租户/登出）时，旧部署凭据不得写回活动态——
    // 新 scope 的 shelf 请求必须继续拿到新部署自己的凭据，而不是被迟到的轮换结果污染。
    if (!current(requestEpoch, deployment)) throw new Error('SHELF_SCOPE');
    activeCredential = refreshed;
    return refreshed.token;
  };

  const authenticate = async (requestEpoch: number, deployment: Deployment, credential: StoredCredential): Promise<RuntimeSnapshot> => {
    try {
      const remote = ports.remoteFor(deployment.origin);
      let verifiedCredential = credential;
      let me;
      try {
        me = await remote.me(credential.token);
      } catch {
        const refreshed = await refreshedCredential(requestEpoch, deployment, credential);
        if (!refreshed) return safe(requestEpoch, deployment, 'authentication-required');
        if (!current(requestEpoch, deployment)) return state;
        verifiedCredential = refreshed;
        me = await remote.me(refreshed.token);
      }
      if (!current(requestEpoch, deployment)) return state;
      const authenticatedUserId = userId(me.user);
      const activeTenantId = tenantId(me.tenant);
      if (!authenticatedUserId || !activeTenantId) return safe(requestEpoch, deployment, 'tenant-required');
      const capabilities = await remote.deploymentCapabilities(verifiedCredential.token);
      if (!current(requestEpoch, deployment)) return state;
      const gate = clientGate(ports.clientVersion, capabilities);
      if (gate.mode !== 'full' && gate.mode !== 'server_upgrade_required') {
        return safe(requestEpoch, deployment, gate.mode === 'unknown_schema' ? 'unknown-capability' : 'protocol-mismatch');
      }
      const surface: RuntimeSurface = gate.mode === 'full' ? 'authorized' : 'read-only';
      await mutateDeployment(async () => {
        // deploymentStore.write 是活动实例记忆（授权主流程），失败语义保持；
        // registry.upsert 是 presentation 辅助数据，失败单独包含，不得把已验证的
        // 授权拖入外层 catch 而呈现 authentication-required（B2-F32）。
        await ports.deploymentStore?.write(deployment);
        try { await ports.deploymentRegistry?.upsert(deployment); } catch { /* presentation 辅助：写失败不阻塞授权 */ }
      });
      if (!current(requestEpoch, deployment)) return state;
      revocableLease = new RuntimeScopeLease({ deploymentOrigin: deployment.origin, userId: authenticatedUserId, tenantId: activeTenantId });
      lease = revocableLease.asScopeLease();
      activeCredential = verifiedCredential;
      activeShelf = ports.resourceShelf
        ? createResourceShelf({ remote: ports.resourceShelf.remoteFor(deployment.origin), accessTokenFor }).open({ lease })
        : undefined;
      return publish({
        surface,
        deployment,
        identity: { userId: authenticatedUserId, activeTenantId, ...tenantOptions(me.memberships, activeTenantId) },
        ...(surface === 'read-only' ? { reason: 'protocol-mismatch' as const } : {}),
      });
    } catch {
      return safe(requestEpoch, deployment, 'authentication-required');
    }
  };
  const signOut = async (): Promise<void> => {
    const deployment = activeDeployment;
    reserve('sign-out');
    publish({ surface: 'deployment-login', reason: 'authentication-required' });
    await Promise.all([
      deployment ? mutateCredential(async () => { await ports.credentialStore.clear(deployment.origin); }) : Promise.resolve(),
      ports.pendingOidcStore ? mutatePendingOidc(async () => { await ports.pendingOidcStore!.clearPending(); }) : Promise.resolve(),
      mutateDeployment(async () => { await ports.deploymentStore?.clear(); }),
    ]);
    await vaultTail;
  };

  return {
    snapshot: () => state,
    subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener); },
    async boot(input?: DeploymentInput): Promise<RuntimeSnapshot> {
      const requestEpoch = reserve();
      publish({ surface: 'deployment-login', reason: 'authentication-required' });
      try {
        const storedDeployment = input ?? await ports.deploymentStore?.read();
        if (requestEpoch !== epoch) return state;
        if (!storedDeployment) return publish({ surface: 'deployment-login', reason: 'authentication-required' });
        const deployment = normalizeDeployment(storedDeployment);
        activeDeployment = deployment;
        const credential = await ports.credentialStore.read(deployment.origin);
        if (!current(requestEpoch, deployment)) return state;
        if (!credential) return publish({ surface: 'deployment-login', deployment, reason: 'authentication-required' });
        return await authenticate(requestEpoch, deployment, credential);
      } catch {
        return requestEpoch === epoch ? publish({ surface: 'deployment-login', reason: 'authentication-required' }) : state;
      }
    },
    async signIn(input): Promise<RuntimeSnapshot> {
      try {
        const deployment = normalizeDeployment(input.deployment);
        const requestEpoch = begin(deployment);
        try {
          const credential = await ports.remoteFor(deployment.origin).passwordLogin({ email: input.email, password: input.password });
          if (!current(requestEpoch, deployment)) return state;
          if (!await persistCredential(requestEpoch, deployment, credential)) return state;
          return await authenticate(requestEpoch, deployment, credential);
        } catch {
          return safe(requestEpoch, deployment, 'authentication-required');
        }
      } finally {
        await vaultTail;
      }
    },
    async wxSignIn(input): Promise<RuntimeSnapshot> {
      try {
        const deployment = normalizeDeployment(input.deployment);
        const requestEpoch = begin(deployment);
        try {
          const credential = await ports.remoteFor(deployment.origin).wechatLogin({ code: input.code });
          if (!current(requestEpoch, deployment)) return state;
          if (!await persistCredential(requestEpoch, deployment, credential)) return state;
          return await authenticate(requestEpoch, deployment, credential);
        } catch {
          return safe(requestEpoch, deployment, 'authentication-required');
        }
      } finally {
        await vaultTail;
      }
    },
    async beginOidc(input): Promise<void> {
      if (!ports.pendingOidcStore || !ports.oidcBrowser) throw new Error('OIDC_UNAVAILABLE');
      const deployment = normalizeDeployment(input.deployment);
      const redirectUri = input.redirectUri.trim();
      if (!redirectUri) throw new Error('OIDC_REDIRECT');
      let registeredRedirect: URL;
      try { registeredRedirect = new URL(redirectUri); } catch { throw new Error('OIDC_REDIRECT'); }
      if (!registeredRedirect.protocol || !registeredRedirect.host) throw new Error('OIDC_REDIRECT');
      const requestEpoch = begin(deployment);
      const verifier = base64Url(randomBytes(ports, 32));
      const challenge = await codeChallenge(verifier);
      if (!current(requestEpoch, deployment)) return;
      try {
        const remote = ports.remoteFor(deployment.origin);
        const authorization = await remote.oidcUrl(serverOidcCallback(deployment), redirectUri, challenge);
        if (!current(requestEpoch, deployment)) return;
        if (!authorization.state.trim() || !authorization.authorizationUrl.trim()) throw new Error('OIDC_AUTHORIZATION');
        const persisted = await mutatePendingOidc(async () => {
          if (!current(requestEpoch, deployment)) return false;
          await ports.pendingOidcStore!.savePending({ deploymentOrigin: deployment.origin, state: authorization.state, codeVerifier: verifier, redirectUri });
          return current(requestEpoch, deployment);
        });
        if (!persisted) return;
        await ports.oidcBrowser.open(authorization.authorizationUrl);
      } catch {
        safe(requestEpoch, deployment, 'authentication-required');
      }
    },
    async completeOidc(callbackUrl: string): Promise<RuntimeSnapshot> {
      if (oidcCompletion) return oidcCompletion;
      if (claimedOidcCallbacks.has(callbackUrl)) return state;
      const completion = (async (): Promise<RuntimeSnapshot> => {
      if (!ports.pendingOidcStore) return state;
      const pending = await mutatePendingOidc(async () => await ports.pendingOidcStore!.consumePending());
      if (!pending) {
        if ((state.surface === 'authorized' || state.surface === 'read-only') && lease) return state;
        if (!activeDeployment) return state;
        const requestEpoch = begin(activeDeployment);
        return safe(requestEpoch, activeDeployment, 'authentication-required');
      }
      claimedOidcCallbacks.add(callbackUrl);
      let deployment: Deployment;
      try { deployment = normalizeDeployment({ origin: pending.deploymentOrigin }); } catch { return state; }
      const requestEpoch = begin(deployment);
      try {
        const callback = validateAuthReturn(pending.state, callbackUrl, pending.redirectUri);
        const code = callback.searchParams.get('code')?.trim();
        if (!code) throw new Error('AUTH_RETURN');
        const credential = await ports.remoteFor(deployment.origin).oidcNativeExchange({
          code, state: pending.state, redirectUri: pending.redirectUri, codeVerifier: pending.codeVerifier,
        });
        if (!current(requestEpoch, deployment)) return state;
        if (!await persistCredential(requestEpoch, deployment, credential)) return state;
        return await authenticate(requestEpoch, deployment, credential);
      } catch {
        return safe(requestEpoch, deployment, 'authentication-required');
      }
      })();
      oidcCompletion = completion;
      try {
        return await completion;
      } finally {
        if (oidcCompletion === completion) oidcCompletion = undefined;
      }
    },
    async authorizedRequest(input: RuntimeAuthorizedRequest): Promise<unknown> {
      const deployment = activeDeployment;
      const transport = deployment && state.surface === 'authorized' ? ports.authorizedTransport?.(deployment.origin) : undefined;
      if (!deployment || !transport) throw new Error('RUNTIME_UNAUTHORIZED');
      const readOnly = ['GET', 'HEAD', 'OPTIONS'].includes(input.method.toUpperCase());
      return await sendWithCredential(epoch, deployment, (token) => transport(input, token), readOnly);
    },
    async authorizedEventStream(input: RuntimeAuthorizedRequest, onChunk: (chunk: string) => void): Promise<void> {
      const deployment = activeDeployment;
      if (deployment === undefined || state.surface !== 'authorized') throw new Error('RUNTIME_UNAUTHORIZED');
      // 已授权但平台/该实例无流通道（authorizedStream 工厂返回 undefined，fail closed）：
      // 与未授权分流（B2-F34）——REST 详情仍可用，错误语义不得误导为「请先登录」。
      // code 化（R1-F22）：与 TASK_STREAM_CURSOR_EXPIRED 同形态的结构化契约，消费方不读 message 文本。
      const transport = ports.authorizedStream?.(deployment.origin);
      if (!transport) throw Object.assign(new Error('RUNTIME_STREAM_UNAVAILABLE'), { code: 'RUNTIME_STREAM_UNAVAILABLE' as const });
      const requestEpoch = epoch;
      const guardedChunk = (chunk: string): void => {
        if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
        onChunk(chunk);
      };
      // 无 signal 的调用也要纳入 revoke 中止（R1-F32）：controller 桥接调用方 signal 与在途流。
      const controller = new AbortController();
      activeStreams.add(controller);
      controller.signal.addEventListener('abort', () => activeStreams.delete(controller), { once: true });
      input.signal?.addEventListener('abort', () => controller.abort(input.signal!.reason), { once: true });
      try {
        await sendWithCredential(requestEpoch, deployment, (token) => transport({ ...input, signal: controller.signal }, token, guardedChunk));
      } finally {
        activeStreams.delete(controller);
      }
    },
    scopeLease: () => lease,
    resourceShelf: () => activeShelf,
    signOut,
    async activateTenant(tenantId: string): Promise<RuntimeSnapshot> {
      try {
        if (typeof tenantId !== 'string' || tenantId.trim() === '') return state;
        const deployment = activeDeployment;
        if (!deployment || state.surface !== 'authorized') return state;
        const requestEpoch = begin(deployment, 'tenant-switch');
        try {
          const credential = await ports.credentialStore.read(deployment.origin);
          if (!current(requestEpoch, deployment)) return state;
          if (!credential) return safe(requestEpoch, deployment, 'authentication-required');
          const switched = await ports.remoteFor(deployment.origin).switchTenant({ tenantId: tenantId.trim(), refreshToken: credential.refreshToken, accessToken: credential.token });
          if (!current(requestEpoch, deployment)) return state;
          if (!await persistCredential(requestEpoch, deployment, switched.credential)) return state;
          return await authenticate(requestEpoch, deployment, switched.credential);
        } catch {
          return safe(requestEpoch, deployment, 'authentication-required');
        }
      } finally {
        await vaultTail;
      }
    },
    async listDeployments(): Promise<Deployment[]> {
      // 失败包含（B2-F42）：registry 读取失败按 fail-closed 语义返回空数组，与端口缺失一致，从不 reject。
      if (!ports.deploymentRegistry) return [];
      try { return await ports.deploymentRegistry.list(); } catch { return []; }
    },
    async switchDeployment(origin: string): Promise<RuntimeSnapshot> {
      try {
        if (typeof origin !== 'string' || origin.trim() === '') return state;
        if (!ports.deploymentRegistry) return state;
        let target: Deployment | undefined;
        try { target = normalizeDeployment({ origin }); } catch { target = undefined; }
        if (!target) return state;
        // 同 origin 短路（B2-F44，收窄 R1-F46）：只覆盖已授权/只读面；upgrade-required 等失败面
        // 必须允许重试完整认证——服务端恢复后重选同实例不能被静默短路在失败状态。
        if ((state.surface === 'authorized' || state.surface === 'read-only') && state.deployment?.origin === target.origin) return state;
        // 失败包含（B2-F17）：SecureStore 读取失败按未登记处理，保持当前面，不得 reject。
        let entries: Deployment[] | undefined;
        try { entries = await ports.deploymentRegistry.list(); } catch { entries = undefined; }
        const record = entries?.find((entry) => entry.origin === target!.origin);
        if (!record) return state;
        const deployment = normalizeDeployment({ origin: record.origin, label: record.label });
        const requestEpoch = begin(deployment);
        try {
          const credential = await ports.credentialStore.read(deployment.origin);
          if (!current(requestEpoch, deployment)) return state;
          if (!credential) return publish({ surface: 'deployment-login', deployment, reason: 'authentication-required' });
          return await authenticate(requestEpoch, deployment, credential);
        } catch {
          return safe(requestEpoch, deployment, 'authentication-required');
        }
      } finally {
        await vaultTail;
      }
    },
    async forgetDeployment(origin: string): Promise<void> {
      let target: Deployment | undefined;
      try { if (typeof origin === 'string' && origin.trim() !== '') target = normalizeDeployment({ origin }); } catch { target = undefined; }
      if (!target) return;
      const deployment = target;
      // R1-F47：凭据清理与登记移除各自包含——SecureStore 清理失败不得吞掉 registry.remove
      // （残留登记会让实例继续出现在可切换列表）。整体仍从不 reject（B2-F43 约定）。
      try {
        if (activeDeployment?.origin === deployment.origin) {
          await signOut();
        } else {
          try {
            await mutateCredential(async () => { await ports.credentialStore.clear(deployment.origin); });
          } catch {
            /* 清理失败：继续移除登记 */
          }
        }
        try {
          await mutateDeployment(async () => { await ports.deploymentRegistry?.remove(deployment.origin); });
        } catch {
          /* 登记移除失败：整体 resolve，不 reject */
        }
      } finally {
        await vaultTail;
      }
    },
    dispose(): void {
      epoch += 1;
      revoke('dispose');
      activeDeployment = undefined;
      listeners.clear();
    },
  };
}
