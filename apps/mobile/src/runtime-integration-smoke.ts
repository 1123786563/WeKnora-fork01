import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime } from '@weknora/mobile-core';

const CLIENT_PROTOCOL = CLIENT_PROTOCOL_VERSION;

export type MobileRuntimeIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; reason: string };

export interface MobileRuntimeIntegrationEvidence {
  deploymentOrigin: string;
  clientProtocol: number;
  capabilityMode: 'compatible' | 'incompatible' | 'unknown';
  identity: 'present' | 'absent';
  outcome: 'authorized' | 'not-authorized';
  commandTimestamp: string;
}

function capabilityEvidenceMode(capabilities: unknown): MobileRuntimeIntegrationEvidence['capabilityMode'] {
  if (typeof capabilities !== 'object' || capabilities === null || Array.isArray(capabilities)) return 'unknown';
  const { protocol_minimum: minimum, protocol_maximum: maximum } = capabilities as Record<string, unknown>;
  if (typeof minimum !== 'number' || typeof maximum !== 'number' ||
      !Number.isSafeInteger(minimum) || !Number.isSafeInteger(maximum) || minimum < 1 || maximum < minimum) return 'unknown';
  return minimum <= CLIENT_PROTOCOL && CLIENT_PROTOCOL <= maximum ? 'compatible' : 'incompatible';
}

/** Reads only opt-in test variables. No fallback makes an absent environment authorized. */
export function mobileRuntimeIntegrationConfig(env: Record<string, string | undefined>): MobileRuntimeIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    const missing = [
      deploymentOrigin ? undefined : 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL',
      email ? undefined : 'WEKNORA_MOBILE_TEST_EMAIL',
      password ? undefined : 'WEKNORA_MOBILE_TEST_PASSWORD',
    ].filter((name): name is string => name !== undefined);
    return { enabled: false, reason: `missing ${missing.join(', ')}` };
  }

  let parsed: URL;
  try {
    parsed = new URL(deploymentOrigin);
  } catch {
    return { enabled: false, reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

/**
 * Executes the exact production JSON transport, concrete remote adapter, and
 * Mobile Runtime. The returned evidence intentionally contains no credential
 * fields, bearer tokens, request bodies, or raw server responses.
 */
export async function runMobileRuntimeIntegration(config: Extract<MobileRuntimeIntegrationConfig, { enabled: true }>): Promise<MobileRuntimeIntegrationEvidence> {
  let capabilityMode: MobileRuntimeIntegrationEvidence['capabilityMode'] = 'unknown';
  const fetcher: FetchLike = (input, init) => fetch(input, init as RequestInit);
  const runtime = createMobileRuntime({
    credentialStore: createInMemoryCredentialStore(),
    clientVersion: CLIENT_PROTOCOL,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      const remote = createMobileRuntimeRemote({ origin, request: client.request });
      return {
        ...remote,
        async deploymentCapabilities(accessToken: string) {
          const capabilities = await remote.deploymentCapabilities(accessToken);
          capabilityMode = capabilityEvidenceMode(capabilities);
          return capabilities;
        },
      };
    },
  });

  const snapshot = await runtime.signIn({
    deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
    email: config.email,
    password: config.password,
  });
  const identityPresent = Boolean(snapshot.identity?.userId && snapshot.identity.activeTenantId);
  return {
    deploymentOrigin: config.deploymentOrigin,
    clientProtocol: CLIENT_PROTOCOL,
    capabilityMode,
    identity: identityPresent ? 'present' : 'absent',
    outcome: snapshot.surface === 'authorized' && identityPresent ? 'authorized' : 'not-authorized',
    commandTimestamp: new Date().toISOString(),
  };
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitMobileRuntimeIntegrationEvidence(evidence: MobileRuntimeIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
