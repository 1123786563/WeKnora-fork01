import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileResourceRemote } from '@weknora/api-client/mobile/resources';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createInMemoryDeploymentRegistry, createMobileRuntime } from '@weknora/mobile-core';
import type { ResourceShelfHandle } from '@weknora/mobile-core';

const CLIENT_PROTOCOL = CLIENT_PROTOCOL_VERSION;

export type MobileRuntimeIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; switchTenantId?: string; alt?: { deploymentOrigin: string; email: string; password: string } }
  | { enabled: false; disposition: 'skip'; reason: string }
  | { enabled: false; disposition: 'invalid'; reason: string };

export interface MobileRuntimeIntegrationEvidence {
  deploymentOrigin: string;
  clientProtocol: number;
  capabilityMode: 'compatible' | 'incompatible' | 'unknown';
  identity: 'present' | 'absent';
  outcome: 'authorized' | 'not-authorized';
  tenantSwitch: 'skipped' | 'switched' | 'switch-failed';
  deploymentSwitch: 'skipped' | 'switched' | 'switch-failed';
  registeredDeployments: number;
  resourceShelf: 'not-authorized' | 'browsed' | 'browse-failed';
  resourceCounts?: { agents: number; knowledge: number; connections: number };
  commandTimestamp: string;
}

function capabilityEvidenceMode(capabilities: unknown): MobileRuntimeIntegrationEvidence['capabilityMode'] {
  if (typeof capabilities !== 'object' || capabilities === null || Array.isArray(capabilities)) return 'unknown';
  const { protocol_minimum: minimum, protocol_maximum: maximum } = capabilities as Record<string, unknown>;
  if (typeof minimum !== 'number' || typeof maximum !== 'number' ||
      !Number.isSafeInteger(minimum) || !Number.isSafeInteger(maximum) || minimum < 1 || maximum < minimum) return 'unknown';
  return minimum <= CLIENT_PROTOCOL && CLIENT_PROTOCOL <= maximum ? 'compatible' : 'incompatible';
}

/** 部署 URL 主机防线：仅允许公网主机，拒绝 localhost、环回、私网、链路本地与保留地址（含 IPv6 形式）。 */
function disallowedIpv4Octets(octets: number[], variable: string): string | undefined {
  const a = octets[0]!;
  const b = octets[1]!;
  if (a === 127 || a === 0 || a >= 240) return `${variable} must not target loopback or reserved addresses`;
  if (a === 10) return `${variable} must not target private addresses`;
  if (a === 172 && b >= 16 && b <= 31) return `${variable} must not target private addresses`;
  if (a === 192 && b === 168) return `${variable} must not target private addresses`;
  if (a === 169 && b === 254) return `${variable} must not target link-local addresses`;
  return undefined;
}

/** 展开去掉方括号后的 IPv6 字面量为 16 字节；非合法 IPv6 返回 undefined（zone id 视为非法）。 */
function parseIpv6Literal(host: string): number[] | undefined {
  if (host.includes('%')) return undefined;
  const halves = host.split('::');
  if (halves.length > 2) return undefined;
  const expandGroups = (half: string): string[] | undefined => {
    if (half === '') return [];
    const groups = half.split(':');
    const dotted = groups[groups.length - 1]!.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/);
    if (dotted) {
      const octets = [Number(dotted[1]), Number(dotted[2]), Number(dotted[3]), Number(dotted[4])];
      if (octets.some((value) => value > 255)) return undefined;
      groups.splice(groups.length - 1, 1, `${(octets[0]! << 8) | octets[1]!}`, `${(octets[2]! << 8) | octets[3]!}`);
    }
    if (!groups.every((group) => /^[0-9a-f]{1,4}$/.test(group))) return undefined;
    return groups;
  };
  const head = expandGroups(halves[0]!);
  const tail = expandGroups(halves.length === 2 ? halves[1]! : '');
  if (!head || !tail) return undefined;
  if (halves.length === 1 && head.length !== 8) return undefined;
  if (halves.length === 2 && head.length + tail.length > 7) return undefined;
  const groups = halves.length === 1
    ? head
    : [...head, ...Array.from({ length: 8 - head.length - tail.length }, () => '0'), ...tail];
  const bytes: number[] = [];
  for (const group of groups) {
    const word = Number.parseInt(group, 16);
    bytes.push((word >> 8) & 0xff, word & 0xff);
  }
  return bytes;
}

function disallowedIpv6Bytes(bytes: number[], variable: string): string | undefined {
  const words = Array.from({ length: 8 }, (_, i) => (bytes[i * 2]! << 8) | bytes[i * 2 + 1]!);
  const wordsZero = (from: number, to: number) => words.slice(from, to).every((word) => word === 0);
  // ::ffff:0:0/96 IPv4-mapped：内嵌 IPv4 部分复用 IPv4 规则。
  if (wordsZero(0, 6) && words[6] === 0xffff) return disallowedIpv4Octets(bytes.slice(12), variable);
  if (wordsZero(0, 7)) {
    if (words[7] === 0) return `${variable} must not target the unspecified address`;
    if (words[7] === 1) return `${variable} must not target a loopback or wildcard address`;
    // ::/96 IPv4-compatible：内嵌 IPv4 部分复用 IPv4 规则。
    return disallowedIpv4Octets(bytes.slice(12), variable);
  }
  // fe80::/10 链路本地（含全部 zone-scoped 地址）。
  if ((words[0]! & 0xffc0) === 0xfe80) return `${variable} must not target link-local addresses`;
  // fc00::/7 ULA 私有。
  if ((words[0]! & 0xfe00) === 0xfc00) return `${variable} must not target private addresses`;
  // ff00::/8 组播。
  if ((words[0]! & 0xff00) === 0xff00) return `${variable} must not target multicast or reserved addresses`;
  // 2001::/32 Teredo 与 2001:db8::/32 文档保留段。
  if (words[0] === 0x2001 && (words[1] === 0x0db8 || words[1] === 0)) return `${variable} must not target reserved addresses`;
  // 2002::/16 6to4：内嵌 IPv4（第 2、3 字节组）复用 IPv4 规则。
  if (words[0] === 0x2002) return disallowedIpv4Octets(bytes.slice(2, 6), variable);
  // 100::/64 discard-only。
  if (words[0] === 0x0100 && wordsZero(1, 4)) return `${variable} must not target reserved addresses`;
  // 除 2000::/3 全球单播外的其余地址段均为保留/未分配。
  if ((words[0]! & 0xe000) !== 0x2000) return `${variable} must not target reserved addresses`;
  return undefined;
}

function disallowedDeploymentHost(hostname: string, variable: string): string | undefined {
  const host = hostname.toLowerCase().replace(/^\[|\]$/g, '');
  if (host === 'localhost' || host.endsWith('.localhost')) return `${variable} must not target localhost`;
  const match = host.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/);
  if (match) return disallowedIpv4Octets([Number(match[1]), Number(match[2]), Number(match[3]), Number(match[4])], variable);
  if (host.includes(':')) {
    const bytes = parseIpv6Literal(host);
    if (!bytes) return `${variable} must not target an invalid IPv6 host`;
    return disallowedIpv6Bytes(bytes, variable);
  }
  return undefined;
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
    return { enabled: false, disposition: 'skip', reason: `missing ${missing.join(', ')}` };
  }

  let parsed: URL;
  try {
    parsed = new URL(deploymentOrigin);
  } catch {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };

  const altOrigin = env.WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL?.trim();
  const altEmail = env.WEKNORA_MOBILE_TEST_ALT_EMAIL?.trim();
  const altPassword = env.WEKNORA_MOBILE_TEST_ALT_PASSWORD;
  const altPresent = [altOrigin, altEmail, altPassword].filter((value) => value !== undefined && value !== '').length;
  if (altPresent !== 0 && altPresent !== 3) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL, WEKNORA_MOBILE_TEST_ALT_EMAIL and WEKNORA_MOBILE_TEST_ALT_PASSWORD must be provided together' };
  }
  let alt: { deploymentOrigin: string; email: string; password: string } | undefined;
  if (altPresent === 3) {
    let parsedAlt: URL;
    try {
      parsedAlt = new URL(altOrigin!);
    } catch {
      return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL is not an absolute URL' };
    }
    if (parsedAlt.protocol !== 'https:' || parsedAlt.username || parsedAlt.password || parsedAlt.pathname !== '/' || parsedAlt.search || parsedAlt.hash) {
      return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
    }
    const altHostRejection = disallowedDeploymentHost(parsedAlt.hostname, 'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL');
    if (altHostRejection) return { enabled: false, disposition: 'invalid', reason: altHostRejection };
    if (parsedAlt.origin === parsed.origin) {
      return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_ALT_DEPLOYMENT_URL must differ from WEKNORA_MOBILE_TEST_DEPLOYMENT_URL' };
    }
    alt = { deploymentOrigin: parsedAlt.origin, email: altEmail!, password: altPassword! };
  }

  const switchTenantId = env.WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID?.trim();
  if (switchTenantId !== undefined && switchTenantId !== '' && !/^\d+$/.test(switchTenantId)) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID must be a positive integer tenant id' };
  }
  if (/^0+$/.test(switchTenantId ?? '')) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_SWITCH_TENANT_ID must be a positive integer tenant id' };
  }
  return { enabled: true, deploymentOrigin: parsed.origin, email, password, ...(switchTenantId ? { switchTenantId } : {}), ...(alt ? { alt } : {}) };
}

/** Browses the real tenant resources through the shelf interface; evidence carries counts only. */
export async function collectResourceShelfEvidence(
  runtime: { resourceShelf(): ResourceShelfHandle | undefined },
): Promise<Pick<MobileRuntimeIntegrationEvidence, 'resourceShelf' | 'resourceCounts'>> {
  const handle = runtime.resourceShelf();
  if (!handle) return { resourceShelf: 'not-authorized' };
  try {
    const page = await handle.browse();
    return {
      resourceShelf: 'browsed',
      resourceCounts: { agents: page.agents.length, knowledge: page.knowledge.length, connections: page.connections.length },
    };
  } catch {
    return { resourceShelf: 'browse-failed' };
  }
}

/**
 * Executes the exact production JSON transport, concrete remote adapter, and
 * Mobile Runtime. The returned evidence intentionally contains no credential
 * fields, bearer tokens, request bodies, or raw server responses.
 */
export async function runMobileRuntimeIntegration(config: Extract<MobileRuntimeIntegrationConfig, { enabled: true }>): Promise<MobileRuntimeIntegrationEvidence> {
  let capabilityMode: MobileRuntimeIntegrationEvidence['capabilityMode'] = 'unknown';
  let passwordLogins = 0;
  const fetcher: FetchLike = (input, init) => fetch(input, init as RequestInit);
  const runtime = createMobileRuntime({
    credentialStore: createInMemoryCredentialStore(),
    deploymentRegistry: createInMemoryDeploymentRegistry(),
    clientVersion: CLIENT_PROTOCOL,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      const remote = createMobileRuntimeRemote({ origin, request: client.request });
      return {
        ...remote,
        async passwordLogin(input: { email: string; password: string }) {
          passwordLogins += 1;
          return remote.passwordLogin(input);
        },
        async deploymentCapabilities(accessToken: string) {
          const capabilities = await remote.deploymentCapabilities(accessToken);
          capabilityMode = capabilityEvidenceMode(capabilities);
          return capabilities;
        },
      };
    },
    resourceShelf: {
      remoteFor(origin) {
        const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
        return createMobileResourceRemote({ origin, request: client.request });
      },
    },
  });

  const snapshot = await runtime.signIn({
    deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
    email: config.email,
    password: config.password,
  });
  const identityPresent = Boolean(snapshot.identity?.userId && snapshot.identity.activeTenantId);

  let tenantSwitch: MobileRuntimeIntegrationEvidence['tenantSwitch'] = 'skipped';
  if (config.switchTenantId && snapshot.surface === 'authorized') {
    const leaseBefore = runtime.scopeLease();
    const switched = await runtime.activateTenant(config.switchTenantId);
    const switchIdentityPresent = Boolean(switched.identity?.userId && switched.identity.activeTenantId && switched.identity.activeTenantId !== snapshot.identity?.activeTenantId);
    tenantSwitch = switched.surface === 'authorized' && switchIdentityPresent && runtime.scopeLease() !== undefined && runtime.scopeLease() !== leaseBefore
      ? 'switched'
      : 'switch-failed';
  }

  let deploymentSwitch: MobileRuntimeIntegrationEvidence['deploymentSwitch'] = 'skipped';
  if (config.alt && snapshot.surface === 'authorized') {
    const altSignIn = await runtime.signIn({
      deployment: { origin: config.alt.deploymentOrigin, label: 'Alt deployment' },
      email: config.alt.email,
      password: config.alt.password,
    });
    const loginsAfterAlt = passwordLogins;
    const leaseBeforeSwitch = runtime.scopeLease();
    const switchedBack = await runtime.switchDeployment(config.deploymentOrigin);
    deploymentSwitch = altSignIn.surface === 'authorized' &&
      switchedBack.surface === 'authorized' &&
      switchedBack.deployment?.origin === config.deploymentOrigin &&
      runtime.scopeLease() !== undefined &&
      runtime.scopeLease() !== leaseBeforeSwitch &&
      passwordLogins === loginsAfterAlt
      ? 'switched'
      : 'switch-failed';
  }
  const registeredDeployments = (await runtime.listDeployments()).length;

  const shelfEvidence = await collectResourceShelfEvidence(runtime);

  return {
    deploymentOrigin: config.deploymentOrigin,
    clientProtocol: CLIENT_PROTOCOL,
    capabilityMode,
    identity: identityPresent ? 'present' : 'absent',
    outcome: snapshot.surface === 'authorized' && identityPresent ? 'authorized' : 'not-authorized',
    tenantSwitch,
    deploymentSwitch,
    registeredDeployments,
    ...shelfEvidence,
    commandTimestamp: new Date().toISOString(),
  };
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitMobileRuntimeIntegrationEvidence(evidence: MobileRuntimeIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
