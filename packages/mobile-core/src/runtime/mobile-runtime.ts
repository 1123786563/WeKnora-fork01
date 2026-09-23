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
  if (typeof id === 'string' && id.trim() !== '') return id;
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
  const sendWithCredential = async <R>(requestEpoch: number, deployment: Deployment, send: (token: string) => Promise<R>): Promise<R> => {
    const credential = await ports.credentialStore.read(deployment.origin);
    if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
    if (!credential) throw new Error('RUNTIME_UNAUTHORIZED');
    try {
      const response = await send(credential.token);
      if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
      return response;
    } catch (error) {
      if (!unauthorizedStatus(error)) throw error;
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
    const refreshed = await refreshedCredential(epoch, deployment, activeCredential);
    if (!refreshed) throw new Error('SHELF_AUTH');
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
        await ports.deploymentStore?.write(deployment);
        await ports.deploymentRegistry?.upsert(deployment);
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
      return await sendWithCredential(epoch, deployment, (token) => transport(input, token));
    },
    async authorizedEventStream(input: RuntimeAuthorizedRequest, onChunk: (chunk: string) => void): Promise<void> {
      const deployment = activeDeployment;
      const transport = deployment && state.surface === 'authorized' ? ports.authorizedStream?.(deployment.origin) : undefined;
      if (!deployment || !transport) throw new Error('RUNTIME_UNAUTHORIZED');
      const requestEpoch = epoch;
      const guardedChunk = (chunk: string): void => {
        if (!current(requestEpoch, deployment)) throw new Error('RUNTIME_SCOPE_CHANGED');
        onChunk(chunk);
      };
      await sendWithCredential(requestEpoch, deployment, (token) => transport(input, token, guardedChunk));
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
          const switched = await ports.remoteFor(deployment.origin).switchTenant({ tenantId: tenantId.trim(), refreshToken: credential.refreshToken });
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
      return ports.deploymentRegistry ? await ports.deploymentRegistry.list() : [];
    },
    async switchDeployment(origin: string): Promise<RuntimeSnapshot> {
      try {
        if (typeof origin !== 'string' || origin.trim() === '') return state;
        if (!ports.deploymentRegistry) return state;
        let target: Deployment | undefined;
        try { target = normalizeDeployment({ origin }); } catch { target = undefined; }
        if (!target) return state;
        const record = (await ports.deploymentRegistry.list()).find((entry) => entry.origin === target!.origin);
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
      try {
        if (activeDeployment?.origin === deployment.origin) {
          await signOut();
        } else {
          await mutateCredential(async () => { await ports.credentialStore.clear(deployment.origin); });
        }
        await mutateDeployment(async () => { await ports.deploymentRegistry?.remove(deployment.origin); });
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
