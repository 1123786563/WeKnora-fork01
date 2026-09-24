import { createChatSessionsApi, createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileLegacyTaskRemote } from '@weknora/api-client/mobile/legacy-tasks';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, type TaskOffice } from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';
import { streamAuthorizedSse } from './adapters/sse-stream.ts';

export type LegacyTasksIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface LegacyTasksIntegrationEvidence {
  deploymentOrigin: string;
  legacyList: 'loaded' | 'failed';
  legacyCount: number;
  probeCreated: boolean;
  followUp: 'submitted' | 'unavailable' | 'failed';
  history: 'loaded' | 'unavailable' | 'failed';
  cleanup: 'removed' | 'left' | 'failed';
  commandTimestamp: string;
}

/** 与 T01/T04 opt-in 语义一致（自包含，不跨计划 import；主机防线复用 B2-F15 同一函数）。 */
export function legacyTasksIntegrationConfig(env: Record<string, string | undefined>): LegacyTasksIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    return { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' };
  }
  let parsed: URL;
  try { parsed = new URL(deploymentOrigin); } catch {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  // 主机防线（B2-F15）：拒绝 localhost/环回/私网/链路本地/保留地址，与 runtime-integration-smoke 同一语义。
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

/**
 * 真实生产 JSON transport + 具体 Remote Adapter + Runtime 授权 REST/SSE 通道 +
 * Task Office 编排。探针会话（新建即天然 legacy：0 run）承担完整端到端：
 * legacy 投影可读 → 普通追问（既有聊天 SSE wire）→ 历史可读 → 清理删除。
 * 每一步如实记录，不伪造通过。
 */
export async function runLegacyTasksIntegration(config: Extract<LegacyTasksIntegrationConfig, { enabled: true }>): Promise<LegacyTasksIntegrationEvidence> {
  const evidence: LegacyTasksIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    legacyList: 'failed',
    legacyCount: 0,
    probeCreated: false,
    followUp: 'unavailable',
    history: 'unavailable',
    cleanup: 'left',
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
    authorizedStream(origin) {
      return (input, accessToken, onChunk) => streamAuthorizedSse(origin, input, accessToken, onChunk, (url, init) => fetch(url, init as RequestInit));
    },
  });
  const snapshot = await runtime.signIn({
    deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
    email: config.email,
    password: config.password,
  });
  if (snapshot.surface !== 'authorized' || !snapshot.deployment) return evidence;

  const office: TaskOffice = createTaskOffice({
    backend: createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
    legacy: createMobileLegacyTaskRemote({
      origin: config.deploymentOrigin,
      request: (input) => runtime.authorizedRequest(input),
      stream: (input, onChunk) => runtime.authorizedEventStream(input, onChunk),
    }),
    lease: () => runtime.scopeLease(),
  });
  const sessions = createChatSessionsApi((input) => runtime.authorizedRequest(input));

  // 1) 探针会话：新建空会话即天然 legacy（0 run），标题唯一以便精确搜索。
  const probeTitle = `legacy-smoke-${Date.now()}`;
  let probeId = '';
  try {
    const probe = await sessions.create({ title: probeTitle });
    probeId = probe.id;
    evidence.probeCreated = probeId !== '';
  } catch {
    return evidence;
  }

  // 2) legacy 投影可读（真实 wire + 真实解析）。
  try {
    const page = await office.legacyTasks({ search: probeTitle });
    evidence.legacyCount = page.items.length;
    evidence.legacyList = page.items.some((item) => item.taskId === probeId && item.kind === 'legacy') ? 'loaded' : 'failed';
  } catch {
    evidence.legacyList = 'failed';
  }

  // 3) 普通追问（既有聊天 SSE wire；结果如实记录，不伪造）。
  if (evidence.legacyList === 'loaded') {
    try {
      await office.followUp({ taskId: probeId, question: 'integration probe: reply with the single word ok' });
      evidence.followUp = 'submitted';
    } catch {
      evidence.followUp = 'failed';
    }
    if (evidence.followUp === 'submitted') {
      try {
        const messages = await office.legacyHistory(probeId);
        evidence.history = messages.length > 0 ? 'loaded' : 'failed';
      } catch {
        evidence.history = 'failed';
      }
    }
  }

  // 4) 清理：软删除探针会话并复核 legacy 投影不再包含它。
  try {
    await sessions.remove(probeId);
    const after = await office.legacyTasks({ search: probeTitle });
    evidence.cleanup = after.items.some((item) => item.taskId === probeId) ? 'failed' : 'removed';
  } catch {
    evidence.cleanup = 'failed';
  }
  return evidence;
}
