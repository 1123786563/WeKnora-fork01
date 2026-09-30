import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, type TaskOffice } from '@weknora/mobile-core';
import { streamAuthorizedSse } from './adapters/sse-stream.ts';

export type TaskDetailIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface TaskDetailIntegrationEvidence {
  deploymentOrigin: string;
  opened: 'hydrated' | 'no-tasks' | 'failed';
  connection?: 'syncing' | 'live' | 'interrupted' | 'drained';
  lifecycle?: string;
  runStatus?: string;
  attention?: string;
  timelineEntries?: number;
  cursor?: number;
  resync: 'resynced' | 'failed' | 'skipped';
  commandTimestamp: string;
}

/** 与 T04 taskOfficeIntegrationConfig 相同的 opt-in 语义（自包含，不跨计划 import）。 */
export function taskDetailIntegrationConfig(env: Record<string, string | undefined>): TaskDetailIntegrationConfig {
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
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

const settle = async (rounds = 10): Promise<void> => {
  for (let index = 0; index < rounds; index += 1) await new Promise<void>((resolve) => setTimeout(resolve, 25));
};

/** 真实 JSON transport + 授权 SSE 通道 + 具体 Remote Adapter + Task Office 详情编排。账号无任务时如实记 'no-tasks'。 */
export async function runTaskDetailIntegration(config: Extract<TaskDetailIntegrationConfig, { enabled: true }>): Promise<TaskDetailIntegrationEvidence> {
  const evidence: TaskDetailIntegrationEvidence = { deploymentOrigin: config.deploymentOrigin, opened: 'failed', resync: 'skipped', commandTimestamp: new Date().toISOString() };
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
      // SseFetchLike 要求真实 Response（ok/body reader）；宿主全局 fetch 即真实通道，
      // FetchLike 的窄返回类型（FetchResponseLike）无法满足 SSE 解码所需的 Response 形态。
      return (input, accessToken, onChunk) => streamAuthorizedSse(origin, input, accessToken, onChunk, fetch);
    },
  });
  const snapshot = await runtime.signIn({ deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' }, email: config.email, password: config.password });
  if (snapshot.surface !== 'authorized' || !snapshot.deployment) return evidence;

  // 同一 remote 同时作为 backend 与 detail：漏传 detail 会让 open() fail closed，
  // 具备真实环境时 AC3 用例将以 opened:'failed' 如实暴露（本处由 Task 9 的
  // app-smoke 源级断言 `detail:\s*remote` 同类防护）。
  const remote = createTaskOfficeRemote({
    origin: config.deploymentOrigin,
    request: (input) => runtime.authorizedRequest(input),
    stream: (input, onChunk) => runtime.authorizedEventStream(input, onChunk),
  });
  const office: TaskOffice = createTaskOffice({ backend: remote, detail: remote, lease: () => runtime.scopeLease() });
  const page = await office.tasks({});
  if (page.items.length === 0) { evidence.opened = 'no-tasks'; return evidence; }
  const target = page.items[0]!;
  const handle = office.open({ taskId: target.taskId, runId: target.runId });
  const view = await handle.hydrate();
  await settle();
  evidence.opened = 'hydrated';
  const settled = handle.view() ?? view;
  evidence.connection = settled.connection;
  evidence.lifecycle = settled.lifecycle;
  evidence.runStatus = settled.runStatus;
  evidence.attention = settled.attention;
  evidence.timelineEntries = settled.timeline.length;
  evidence.cursor = settled.cursor;
  try {
    const resynced = await handle.resync();
    evidence.resync = resynced.connection === 'interrupted' ? 'failed' : 'resynced';
  } catch {
    evidence.resync = 'failed';
  }
  handle.close('integration-complete');
  runtime.dispose();
  return evidence;
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitTaskDetailIntegrationEvidence(evidence: TaskDetailIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
