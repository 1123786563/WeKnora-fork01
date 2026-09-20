import { clientGate, validateAuthReturn } from '@weknora/domain/mobile';
import type { MobileRuntimePorts, StoredCredential } from './ports.ts';
import type { Deployment, DeploymentInput, MobileRuntime, RuntimeReason, RuntimeSnapshot, ScopeLease } from './types.ts';

const scopeLeaseBrand = Symbol('ScopeLease');

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

class RuntimeScopeLease {
  readonly [scopeLeaseBrand] = undefined;
  #active = true;
  revoke(): void { this.#active = false; }
}

export function createMobileRuntime(ports: MobileRuntimePorts): MobileRuntime {
  let epoch = 0;
  let activeDeployment: Deployment | undefined;
  let lease: ScopeLease | undefined;
  let revocableLease: RuntimeScopeLease | undefined;
  let oidcCompletion: Promise<RuntimeSnapshot> | undefined;
  let state: RuntimeSnapshot = { surface: 'deployment-login', reason: 'authentication-required' };
  const listeners = new Set<(snapshot: RuntimeSnapshot) => void>();

  const publish = (next: RuntimeSnapshot): RuntimeSnapshot => {
    state = next;
    for (const listener of listeners) listener(state);
    return state;
  };
  const revoke = (): void => { revocableLease?.revoke(); revocableLease = undefined; lease = undefined; };
  const begin = (deployment: Deployment): number => {
    epoch += 1;
    revoke();
    activeDeployment = deployment;
    return epoch;
  };
  const current = (requestEpoch: number, deployment: Deployment): boolean => requestEpoch === epoch && activeDeployment?.origin === deployment.origin;
  const safe = (requestEpoch: number, deployment: Deployment, reason: RuntimeReason): RuntimeSnapshot =>
    current(requestEpoch, deployment) ? publish({ surface: 'upgrade-required', deployment, reason }) : state;

  const authenticate = async (requestEpoch: number, deployment: Deployment, credential: StoredCredential): Promise<RuntimeSnapshot> => {
    try {
      const remote = ports.remoteFor(deployment.origin);
      const me = await remote.me(credential.token);
      if (!current(requestEpoch, deployment)) return state;
      const authenticatedUserId = userId(me.user);
      const activeTenantId = tenantId(me.tenant);
      if (!authenticatedUserId || !activeTenantId) return safe(requestEpoch, deployment, 'tenant-required');
      const capabilities = await remote.deploymentCapabilities(credential.token);
      if (!current(requestEpoch, deployment)) return state;
      const gate = clientGate(ports.clientVersion, capabilities);
      if (gate.mode !== 'full') return safe(requestEpoch, deployment, gate.mode === 'unknown_schema' ? 'unknown-capability' : 'protocol-mismatch');
      await ports.deploymentStore?.write(deployment);
      if (!current(requestEpoch, deployment)) return state;
      revocableLease = new RuntimeScopeLease();
      lease = revocableLease as unknown as ScopeLease;
      return publish({ surface: 'authorized', deployment, identity: { userId: authenticatedUserId, activeTenantId } });
    } catch {
      return safe(requestEpoch, deployment, 'authentication-required');
    }
  };
  const signOut = async (): Promise<void> => {
    const deployment = activeDeployment;
    epoch += 1;
    revoke();
    activeDeployment = undefined;
    publish({ surface: 'deployment-login', reason: 'authentication-required' });
    if (deployment) await ports.credentialStore.clear(deployment.origin);
    await ports.deploymentStore?.clear();
  };

  return {
    snapshot: () => state,
    subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener); },
    async boot(input?: DeploymentInput): Promise<RuntimeSnapshot> {
      try {
        const storedDeployment = input ?? await ports.deploymentStore?.read();
        if (!storedDeployment) return publish({ surface: 'deployment-login', reason: 'authentication-required' });
        const deployment = normalizeDeployment(storedDeployment);
        const requestEpoch = begin(deployment);
        const credential = await ports.credentialStore.read(deployment.origin);
        if (!current(requestEpoch, deployment)) return state;
        if (!credential) return publish({ surface: 'deployment-login', deployment, reason: 'authentication-required' });
        return await authenticate(requestEpoch, deployment, credential);
      } catch {
        return publish({ surface: 'deployment-login', reason: 'authentication-required' });
      }
    },
    async signIn(input): Promise<RuntimeSnapshot> {
      const deployment = normalizeDeployment(input.deployment);
      const requestEpoch = begin(deployment);
      try {
        const credential = await ports.remoteFor(deployment.origin).passwordLogin({ email: input.email, password: input.password });
        if (!current(requestEpoch, deployment)) return state;
        await ports.credentialStore.write(deployment.origin, credential);
        if (!current(requestEpoch, deployment)) return state;
        return await authenticate(requestEpoch, deployment, credential);
      } catch {
        return safe(requestEpoch, deployment, 'authentication-required');
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
        await ports.pendingOidcStore.savePending({ deploymentOrigin: deployment.origin, state: authorization.state, codeVerifier: verifier, redirectUri });
        if (!current(requestEpoch, deployment)) return;
        await ports.oidcBrowser.open(authorization.authorizationUrl);
      } catch {
        safe(requestEpoch, deployment, 'authentication-required');
      }
    },
    async completeOidc(callbackUrl: string): Promise<RuntimeSnapshot> {
      if (oidcCompletion) return oidcCompletion;
      const completion = (async (): Promise<RuntimeSnapshot> => {
      if (!ports.pendingOidcStore) return state;
      const pending = await ports.pendingOidcStore.consumePending();
      if (!pending) {
        if (!activeDeployment) return state;
        const requestEpoch = begin(activeDeployment);
        return safe(requestEpoch, activeDeployment, 'authentication-required');
      }
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
        await ports.credentialStore.write(deployment.origin, credential);
        if (!current(requestEpoch, deployment)) return state;
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
    scopeLease: () => lease,
    signOut,
    dispose(): void {
      epoch += 1;
      revoke();
      activeDeployment = undefined;
      listeners.clear();
    },
  };
}
