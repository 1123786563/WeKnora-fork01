import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileDeviceRemote } from '@weknora/api-client/mobile/devices';
import { createMobileInboxRemote } from '@weknora/api-client/mobile/inbox';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createDeviceRegistry, createInMemoryCredentialStore, createMobileRuntime, createNotificationInbox, DeviceError } from '@weknora/mobile-core';
import { isValidWeKnoraAppId } from './app-id.ts';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type BlindPushIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; enterpriseAppId?: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface BlindPushIntegrationEvidence {
  deploymentOrigin: string;
  officialRegister: 'registered' | 'failed';
  enterpriseRegister: 'registered' | 'rejected-by-policy' | 'failed';
  isolation: 'isolated' | 'mixed' | 'unverified';
  foregroundSync: 'synced' | 'failed';
  inboxUnreadCount?: number;
  commandTimestamp: string;
}

export function blindPushIntegrationConfig(env: Record<string, string | undefined>): BlindPushIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    return { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' };
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
  const rawEnterprise = env.WEKNORA_MOBILE_TEST_ENTERPRISE_APP_ID?.trim();
  // 非法企业 id 静默降级为「未声明」：冒烟只做官方通道 + 策略探测。
  const enterpriseAppId = rawEnterprise !== undefined && isValidWeKnoraAppId(rawEnterprise) && rawEnterprise !== 'official' ? rawEnterprise : undefined;
  return { enabled: true, deploymentOrigin: parsed.origin, email, password, ...(enterpriseAppId === undefined ? {} : { enterpriseAppId }) };
}

function wireStatus(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null) return undefined;
  if ((error as { name?: unknown }).name !== 'ApiError') return undefined;
  const status = (error as { status?: unknown }).status;
  return typeof status === 'number' ? status : undefined;
}

/** 真实 transport + Runtime + 两步注册（#67 AC1 客户端观察 + AC2 前台权威同步）。 */
export async function runBlindPushIntegration(config: Extract<BlindPushIntegrationConfig, { enabled: true }>): Promise<BlindPushIntegrationEvidence> {
  const evidence: BlindPushIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    officialRegister: 'failed',
    enterpriseRegister: 'failed',
    isolation: 'unverified',
    foregroundSync: 'failed',
    commandTimestamp: new Date().toISOString(),
  };
  const fetcher: FetchLike = (input, init) => fetch(input, init as RequestInit);
  const runtime = createMobileRuntime({
    credentialStore: createInMemoryCredentialStore(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
    },
  });
  const throughRuntime = (input: { method: string; path: string; headers?: Record<string, string>; body?: unknown }): Promise<unknown> =>
    runtime.authorizedRequest(input);
  const deviceId = 'blind-push-smoke-device';
  const registry = createDeviceRegistry({
    remote: createMobileDeviceRemote({ origin: config.deploymentOrigin, request: throughRuntime }),
    lease: () => runtime.scopeLease(),
  });
  try {
    const official = await registry.register({ deviceId, token: 'integration-placeholder-token', platform: 'ios' });
    evidence.officialRegister = 'registered';
    const officialRevision = official.revision;

    const enterpriseAppId = config.enterpriseAppId ?? 'enterprise:probe';
    try {
      const enterprise = await registry.register({ deviceId, token: 'integration-placeholder-token-enterprise', platform: 'ios', appId: enterpriseAppId });
      evidence.enterpriseRegister = 'registered';
      const rows = await registry.list();
      const mine = rows.filter((row) => row.deviceId === deviceId);
      const apps = new Set(mine.map((row) => row.appId));
      const officialRow = mine.find((row) => row.appId === 'official');
      evidence.isolation = mine.length >= 2 && apps.has('official') && apps.has(enterprise.appId) && officialRow?.revision === officialRevision
        ? 'isolated'
        : 'mixed';
    } catch (error) {
      // 部署未声明该企业 App（服务端允许清单拒绝 400）——隔离的另一半证据。
      const status = error instanceof DeviceError ? wireStatus((error as DeviceError & { cause?: unknown }).cause) : wireStatus(error);
      evidence.enterpriseRegister = status === 400 ? 'rejected-by-policy' : 'failed';
      const rows = await registry.list().catch(() => []);
      evidence.isolation = rows.some((row) => row.deviceId === deviceId && row.appId !== 'official') ? 'mixed' : 'unverified';
    }
  } catch {
    // 任一步失败保留枚举失败值
  }

  // AC2：与推送完全无关的前台权威同步——同一授权 scope 下 inbox page() 必须可用。
  try {
    const snapshot = runtime.snapshot();
    const origin = snapshot.deployment?.origin ?? config.deploymentOrigin;
    const inbox = createNotificationInbox({
      remote: createMobileInboxRemote({ origin, request: throughRuntime }),
      lease: () => runtime.scopeLease(),
    });
    const view = await inbox.page();
    evidence.foregroundSync = 'synced';
    evidence.inboxUnreadCount = view.unreadCount;
  } catch {
    evidence.foregroundSync = 'failed';
  }
  return evidence;
}

export function emitBlindPushIntegrationEvidence(evidence: BlindPushIntegrationEvidence, diagnostic: (record: string) => void): void {
  diagnostic(`BLIND_PUSH_EVIDENCE ${JSON.stringify(evidence)}`);
}
