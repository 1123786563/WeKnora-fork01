import { clientGate } from '@weknora/domain/mobile';
import type { MobileRuntimePorts, StoredCredential } from './ports.ts';
import { scopeLeaseBrand, type MobileRuntime, type RuntimeIdentity, type RuntimeSnapshot, type RuntimeTenant, type ScopeLease } from './types.ts';

function normalizeDeployment(value: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error('deployment is required');
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    throw new Error('deployment must be an absolute URL');
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.search || parsed.hash || parsed.pathname !== '/') {
    throw new Error('deployment must be an HTTPS origin');
  }
  return parsed.origin;
}

function identity(value: unknown): RuntimeIdentity | undefined {
  if (typeof value !== 'object' || value === null || typeof (value as { id?: unknown }).id !== 'string' || (value as { id: string }).id.trim() === '') return undefined;
  return { id: (value as { id: string }).id };
}

function tenant(value: unknown): RuntimeTenant | undefined {
  if (typeof value !== 'object' || value === null) return undefined;
  const id = (value as { id?: unknown }).id;
  if (typeof id === 'string' && id.trim() !== '') return { id };
  if (typeof id === 'number' && Number.isSafeInteger(id) && id > 0) return { id: String(id) };
  return undefined;
}

class RuntimeScopeLease implements ScopeLease {
  readonly [scopeLeaseBrand] = undefined;
  #active = true;
  isActive(): boolean { return this.#active; }
  revoke(): void { this.#active = false; }
}

export function createMobileRuntime(ports: MobileRuntimePorts): MobileRuntime {
  let epoch = 0;
  let activeDeployment: string | undefined;
  let lease: RuntimeScopeLease | undefined;
  let state: RuntimeSnapshot = { surface: 'signed-out' };
  const listeners = new Set<(snapshot: RuntimeSnapshot) => void>();

  const publish = (next: RuntimeSnapshot): RuntimeSnapshot => {
    state = next;
    for (const listener of listeners) listener(state);
    return state;
  };
  const revoke = (): void => { lease?.revoke(); lease = undefined; };
  const begin = (deployment: string, surface: RuntimeSnapshot['surface']): number => {
    epoch += 1;
    revoke();
    activeDeployment = deployment;
    publish({ surface, deployment });
    return epoch;
  };
  const current = (requestEpoch: number, deployment: string): boolean => requestEpoch === epoch && activeDeployment === deployment;
  const safe = (requestEpoch: number, deployment: string, gate?: RuntimeSnapshot['gate']): RuntimeSnapshot =>
    current(requestEpoch, deployment) ? publish({ surface: 'upgrade-required', deployment, ...(gate ? { gate } : {}) }) : state;

  const authenticate = async (requestEpoch: number, deployment: string, credential: StoredCredential): Promise<RuntimeSnapshot> => {
    try {
      const remote = ports.remoteFor(deployment);
      const me = await remote.me(credential.accessToken);
      if (!current(requestEpoch, deployment)) return state;
      const currentIdentity = identity(me.user);
      const currentTenant = tenant(me.tenant);
      if (!currentIdentity || !currentTenant) return safe(requestEpoch, deployment);
      const capabilities = await remote.deploymentCapabilities(credential.accessToken);
      if (!current(requestEpoch, deployment)) return state;
      const gate = clientGate(ports.clientVersion, capabilities);
      if (gate.mode !== 'full') return safe(requestEpoch, deployment, gate);
      lease = new RuntimeScopeLease();
      return publish({ surface: 'full', deployment, identity: currentIdentity, tenant: currentTenant, gate, scopeLease: lease });
    } catch {
      return safe(requestEpoch, deployment);
    }
  };
  const signOut = async (): Promise<void> => {
    const deployment = activeDeployment;
    epoch += 1;
    revoke();
    activeDeployment = undefined;
    publish({ surface: 'signed-out' });
    if (deployment) await ports.credentialStore.clear(deployment);
  };

  return {
    async boot(deploymentHint?: string): Promise<RuntimeSnapshot> {
      if (!deploymentHint) return state;
      const deployment = normalizeDeployment(deploymentHint);
      const requestEpoch = begin(deployment, 'restoring');
      try {
        const credential = await ports.credentialStore.read(deployment);
        if (!current(requestEpoch, deployment)) return state;
        if (!credential) return publish({ surface: 'signed-out', deployment });
        return await authenticate(requestEpoch, deployment, credential);
      } catch {
        return safe(requestEpoch, deployment);
      }
    },
    async signIn(deploymentInput, input): Promise<RuntimeSnapshot> {
      const deployment = normalizeDeployment(deploymentInput);
      const requestEpoch = begin(deployment, 'restoring');
      try {
        const credential = await ports.remoteFor(deployment).passwordLogin(input);
        if (!current(requestEpoch, deployment)) return state;
        await ports.credentialStore.write(deployment, credential);
        if (!current(requestEpoch, deployment)) return state;
        return await authenticate(requestEpoch, deployment, credential);
      } catch {
        return safe(requestEpoch, deployment);
      }
    },
    snapshot: () => state,
    subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener); },
    signOut,
    async dispose(): Promise<void> {
      await signOut();
      listeners.clear();
    },
  };
}
