import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileMaterialRemote } from '@weknora/api-client/mobile/materials';
import { createMobileResearchRemote } from '@weknora/api-client/mobile/research';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice } from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type ResearchIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface ResearchIntegrationEvidence {
  deploymentOrigin: string;
  listed: 'delegated' | 'no-tasks' | 'no-materials' | 'failed';
  delegationCreated?: boolean;
  delegationCount?: number;
  annotated?: 'recorded' | 'skipped-no-materials' | 'failed';
  annotationCount?: number;
  annotatedVersion?: string;
  revision: 'not-dispatched';
  failure?: string;
  commandTimestamp: string;
}

/** 与 T16 material 证据相同的 opt-in 语义（自包含，不跨计划 import 凭据逻辑）。 */
export function researchIntegrationConfig(env: Record<string, string | undefined>): ResearchIntegrationConfig {
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
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

/**
 * 真实 JSON transport + 授权通道 + 具体 Remote Adapter + research 编排。
 * 副作用声明（如实，不伪装）：探测到的首个任务上留下 1 条只读研究委派与
 * 1 条版本钉定批注（append-only，无删除端点是设计事实）；修订请求不派发。
 * 委派源 'probe-kb' 多半被租户范围围栏以 400 拒绝——这本身是 AC1 的真实证据：
 * 记 delegationCreated:false + failure 前缀 delegation-rejected-by-scope-fence。
 * 任何步骤异常 → listed:'failed' + failure 摘要（无凭据字段），从不 reject。
 */
export async function runResearchIntegration(config: Extract<ResearchIntegrationConfig, { enabled: true }>): Promise<ResearchIntegrationEvidence> {
  const evidence: ResearchIntegrationEvidence = { deploymentOrigin: config.deploymentOrigin, listed: 'failed', revision: 'not-dispatched', commandTimestamp: new Date().toISOString() };
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
  try {
    const snapshot = await runtime.signIn({ deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' }, email: config.email, password: config.password });
    if (snapshot.surface !== 'authorized' || !snapshot.deployment) { evidence.failure = 'sign-in did not reach the authorized surface'; return evidence; }
    const origin = config.deploymentOrigin;
    const office = createTaskOffice({
      backend: createTaskOfficeRemote({ origin, request: (input) => runtime.authorizedRequest(input) }),
      lease: () => runtime.scopeLease(),
    });
    const page = await office.tasks({});
    if (page.items.length === 0) { evidence.listed = 'no-tasks'; return evidence; }
    const runId = page.items[0]!.runId;

    const research = createMobileResearchRemote({ origin, request: (input) => runtime.authorizedRequest(input) });
    const material = createMobileMaterialRemote({ origin, request: (input) => runtime.authorizedRequest(input) });

    let delegated = true;
    try {
      await research.delegate({ runId, objective: 'integration probe: verify read-only research delegation', sources: ['probe-kb'] });
    } catch (failure) {
      delegated = false;
      evidence.failure = `delegation-rejected-by-scope-fence:${failure instanceof Error ? failure.message.slice(0, 120) : 'unknown'}`;
    }
    evidence.delegationCreated = delegated;
    const delegations = await research.list(runId);
    evidence.listed = 'delegated';
    evidence.delegationCount = delegations.delegations.length;
    if (!delegated) return evidence;

    const index = await material.list(runId);
    const first = index.artifacts[0];
    if (first === undefined) { evidence.annotated = 'skipped-no-materials'; return evidence; }
    await research.annotate({ runId, materialId: first.id, baseVersion: first.version, body: 'integration probe annotation (version-pinned)' });
    const annotations = await research.annotations(runId);
    evidence.annotated = 'recorded';
    evidence.annotationCount = annotations.annotations.length;
    evidence.annotatedVersion = first.version;
    return evidence;
  } catch (error) {
    evidence.listed = 'failed';
    evidence.failure = error instanceof Error ? error.message.slice(0, 300) : String(error).slice(0, 300);
    return evidence;
  } finally {
    runtime.dispose();
  }
}

/** 证据契约输出（JSONL）：无凭据字段；revision 恒 not-dispatched。 */
export function emitResearchIntegrationEvidence(evidence: ResearchIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify({ kind: 'research-integration', ...evidence }));
}
