import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { INTERACTION_ACTIONS, createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, type TaskOffice } from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type AttentionInboxIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; decideEnabled: boolean }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface AttentionInboxIntegrationEvidence {
  deploymentOrigin: string;
  inbox: 'browsed' | 'browse-failed';
  pendingCount: number;
  decide: 'skipped' | 'no-pending' | 'recorded' | 'delivery-unknown' | 'superseded' | 'gone' | 'failed';
  /** 异常路径失败摘要（仅 error message，证据契约无凭据字段）。 */
  failure?: string;
  commandTimestamp: string;
}

/** opt-in 语义与 T04/T05 相同（自包含，不跨计划 import）；决定动作额外要求
 * WEKNORA_MOBILE_TEST_DECIDE_INTERACTION=1——不在真实账号上擅自决定。 */
export function attentionInboxIntegrationConfig(env: Record<string, string | undefined>): AttentionInboxIntegrationConfig {
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
  return { enabled: true, deploymentOrigin: parsed.origin, email, password, decideEnabled: env.WEKNORA_MOBILE_TEST_DECIDE_INTERACTION === '1' };
}

/** 真实 JSON transport + 授权通道 + 具体 Remote Adapter（含 interactions 端口）+ Task Office 编排。
 *  total 契约：任何步骤异常 → inbox:'browse-failed' + failure 摘要，从不 reject；所有路径经同一 finally 清理。 */
export async function runAttentionInboxIntegration(config: Extract<AttentionInboxIntegrationConfig, { enabled: true }>): Promise<AttentionInboxIntegrationEvidence> {
  const evidence: AttentionInboxIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    inbox: 'browse-failed',
    pendingCount: 0,
    decide: 'skipped',
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
  try {
    const snapshot = await runtime.signIn({ deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' }, email: config.email, password: config.password });
    if (snapshot.surface !== 'authorized' || !snapshot.deployment) return evidence;
    // 同一 remote 同时作为 backend/detail/interactions 三个端口（与 composition 相同装配）。
    const remote = createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) });
    const office: TaskOffice = createTaskOffice({ backend: remote, detail: remote, interactions: remote, lease: () => runtime.scopeLease() });

    const view = await office.inbox();
    evidence.inbox = 'browsed';
    evidence.pendingCount = view.items.length;

    if (!config.decideEnabled) return evidence;
    if (view.items.length === 0) {
      evidence.decide = 'no-pending';
      return evidence;
    }
    const target = view.items[0]!;
    const action = INTERACTION_ACTIONS[target.kind][0]!;
    const receipt = await office.decide({ item: target, action });
    evidence.decide = receipt.status;
    return evidence;
  } catch (error) {
    evidence.failure = error instanceof Error ? error.message : String(error);
    if (evidence.decide !== 'skipped') evidence.decide = 'failed';
    return evidence;
  } finally {
    runtime.dispose(); // dispose(): void（packages/mobile-core/src/runtime/types.ts:72）
  }
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitAttentionInboxIntegrationEvidence(evidence: AttentionInboxIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
