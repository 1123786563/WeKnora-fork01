import { clientGate } from '@weknora/domain/mobile';
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
  };

  return {
    snapshot: () => state,
    subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener); },
    async boot(input?: DeploymentInput): Promise<RuntimeSnapshot> {
      if (!input) return publish({ surface: 'deployment-login', reason: 'authentication-required' });
      const deployment = normalizeDeployment(input);
      const requestEpoch = begin(deployment);
      try {
        const credential = await ports.credentialStore.read(deployment.origin);
        if (!current(requestEpoch, deployment)) return state;
        if (!credential) return publish({ surface: 'deployment-login', deployment, reason: 'authentication-required' });
        return await authenticate(requestEpoch, deployment, credential);
      } catch {
        return safe(requestEpoch, deployment, 'authentication-required');
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
    async beginOidc(): Promise<void> {
      throw new Error('OIDC_NOT_IMPLEMENTED');
    },
    async completeOidc(): Promise<RuntimeSnapshot> {
      return state;
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
