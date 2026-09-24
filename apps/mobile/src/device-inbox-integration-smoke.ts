import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileDeviceRemote } from '@weknora/api-client/mobile/devices';
import { createMobileInboxRemote } from '@weknora/api-client/mobile/inbox';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import {
  createDeviceRegistry, createInMemoryCredentialStore, createMobileRuntime, createNotificationInbox,
  DeviceError,
} from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type DeviceInboxIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; deviceToken: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface DeviceInboxIntegrationEvidence {
  deploymentOrigin: string;
  deviceRegister: 'registered' | 'failed';
  deviceTokenTakeover: 'rotated' | 'failed';
  deviceRevoke: 'revoked' | 'failed';
  deviceRevokeAgain: 'not-found' | 'unexpected';
  inboxRead: 'read' | 'failed';
  inboxUnreadCount?: number;
  markReadIdempotent: 'ok' | 'failed';
  deepLinksObserved: number;
  commandTimestamp: string;
}

/** 与 T04/T05 冒烟相同的 opt-in 语义（自包含，不跨计划 import 配置解析）。 */
export function deviceInboxIntegrationConfig(env: Record<string, string | undefined>): DeviceInboxIntegrationConfig {
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
  // 占位 token 不是可用凭据（无 APNs/FCM 真实效力）；可用真实 token 经变量覆盖。
  const deviceToken = env.WEKNORA_MOBILE_TEST_DEVICE_TOKEN?.trim() || 'integration-placeholder-token';
  return { enabled: true, deploymentOrigin: parsed.origin, email, password, deviceToken };
}

/**
 * 真实 JSON transport + 具体 Remote Adapter + Mobile Runtime + 服务端两步注册/接管/撤销/
 * inbox 读/markRead（#41 AC3）。证据契约不含任何凭据字段；任何步骤异常只记枚举失败，从不 reject。
 */
export async function runDeviceInboxIntegration(config: Extract<DeviceInboxIntegrationConfig, { enabled: true }>): Promise<DeviceInboxIntegrationEvidence> {
  const evidence: DeviceInboxIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    deviceRegister: 'failed',
    deviceTokenTakeover: 'failed',
    deviceRevoke: 'failed',
    deviceRevokeAgain: 'unexpected',
    inboxRead: 'failed',
    markReadIdempotent: 'failed',
    deepLinksObserved: 0,
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
    // 授权通道（对照 composition.ts:62-65 既有写法）：devices/inbox remote 全走
    // runtime.authorizedRequest——无此 port 时 authorizedRequest 直接抛
    // 'RUNTIME_UNAUTHORIZED'（mobile-runtime.ts:383-388，name='Error' 非 ApiError），
    // 证据会全部退化为 failed，因此本冒烟必须显式装配。
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
    },
  });
  const throughRuntime = (input: { method: string; path: string; headers?: Record<string, string>; body?: unknown }): Promise<unknown> =>
    runtime.authorizedRequest(input);
  const deviceId = 'integration-smoke-device';
  const registry = createDeviceRegistry({
    remote: createMobileDeviceRemote({ origin: config.deploymentOrigin, request: throughRuntime }),
    lease: () => runtime.scopeLease(),
  });
  const inbox = createNotificationInbox({
    remote: createMobileInboxRemote({ origin: config.deploymentOrigin, request: throughRuntime }),
    lease: () => runtime.scopeLease(),
  });

  const snapshot = await runtime.signIn({
    deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
    email: config.email,
    password: config.password,
  });
  if (snapshot.surface !== 'authorized') return evidence; // 未授权：证据保持 failed 枚举，如实记录

  try {
    await registry.register({ deviceId, token: `${config.deviceToken}#1`, platform: 'ios' });
    evidence.deviceRegister = 'registered';
  } catch {
    return evidence;
  }
  try {
    await registry.register({ deviceId, token: `${config.deviceToken}#2`, platform: 'ios' }); // token 接管
    evidence.deviceTokenTakeover = 'rotated';
  } catch {
    return evidence;
  }
  try {
    await registry.revoke({ deviceId });
    evidence.deviceRevoke = 'revoked';
  } catch {
    return evidence;
  }
  try {
    await registry.revoke({ deviceId }); // 撤销后再撤销：必须 404 → DEVICE_NOT_FOUND
    evidence.deviceRevokeAgain = 'unexpected';
  } catch (error) {
    evidence.deviceRevokeAgain = error instanceof DeviceError && error.code === 'DEVICE_NOT_FOUND' ? 'not-found' : 'unexpected';
  }
  try {
    const view = await inbox.page();
    evidence.inboxRead = 'read';
    evidence.inboxUnreadCount = view.unreadCount;
    evidence.deepLinksObserved = view.items.filter((item) => item.deepLink !== undefined && item.deepLink !== '').length;
  } catch {
    return evidence;
  }
  try {
    // 服务端对不存在 id 幂等 success 且不执行任何业务操作（workbench_inbox.go:123-130）
    await inbox.markRead('integration-smoke-nonexistent');
    evidence.markReadIdempotent = 'ok';
  } catch {
    /* 保持 failed：如实记录该部署的语义差异 */
  }
  return evidence;
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitDeviceInboxIntegrationEvidence(evidence: DeviceInboxIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
