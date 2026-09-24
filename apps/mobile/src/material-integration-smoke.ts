import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileMaterialRemote } from '@weknora/api-client/mobile/materials';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskMaterial, createTaskOffice } from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';
import { createFetchBlobAdapter } from './adapters/material-adapters.ts';

export type MaterialIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface MaterialIntegrationEvidence {
  deploymentOrigin: string;
  listed: 'listed' | 'no-tasks' | 'no-materials' | 'failed';
  materialCount?: number;
  kindCounts?: { artifact: number; diff: number; 'test-report': number };
  grant?: 'minted' | 'failed';
  grantTtlSeconds?: number;   // 断言 ≤ 900（AC1「签名 URL 不长期缓存」的端到端面）
  downloaded?: 'fetched' | 'failed' | 'skipped';
  downloadedBytes?: number;
  terminal?: 'paged' | 'empty' | 'unavailable' | 'failed';
  citations?: number;
  failure?: string;
  commandTimestamp: string;
}

/** 与 T04/T05 相同的 opt-in 语义（自包含，不跨计划 import 凭据逻辑）。 */
export function materialIntegrationConfig(env: Record<string, string | undefined>): MaterialIntegrationConfig {
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
 * 真实 JSON transport + 授权通道 + 具体 Remote Adapter + Task Material 编排 + 免凭据
 * 签名下载。账号无任务/无材料时如实记 'no-tasks'/'no-materials'；任何步骤异常 →
 * listed:'failed' + failure 摘要（无凭据字段），从不 reject；所有路径经同一 finally 清理。
 */
export async function runMaterialIntegration(config: Extract<MaterialIntegrationConfig, { enabled: true }>): Promise<MaterialIntegrationEvidence> {
  const evidence: MaterialIntegrationEvidence = { deploymentOrigin: config.deploymentOrigin, listed: 'failed', commandTimestamp: new Date().toISOString() };
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

    const office = createTaskOffice({
      backend: createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
      lease: () => runtime.scopeLease(),
    });
    const page = await office.tasks({});
    if (page.items.length === 0) { evidence.listed = 'no-tasks'; return evidence; }
    const runId = page.items[0]!.runId;

    const material = createTaskMaterial({
      remote: createMobileMaterialRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
      blob: createFetchBlobAdapter(),
    });
    const lease = runtime.scopeLease();
    if (lease === undefined) return evidence;
    const handle = material.open({ lease });
    try {
      const index = await handle.index({ runId });
      evidence.terminal = index.terminal.available ? 'paged' : 'unavailable';
      if (index.materials.length === 0) { evidence.listed = 'no-materials'; evidence.materialCount = 0; return evidence; }
      evidence.listed = 'listed';
      evidence.materialCount = index.materials.length;
      evidence.kindCounts = {
        artifact: index.materials.filter((entry) => entry.kind === 'artifact').length,
        diff: index.materials.filter((entry) => entry.kind === 'diff').length,
        'test-report': index.materials.filter((entry) => entry.kind === 'test-report').length,
      };
      const target = index.materials[0]!;

      const grant = await handle.act({ kind: 'download', runId, materialId: target.materialId });
      if (grant.kind === 'grant') {
        evidence.grant = 'minted';
        const expiresAt = Date.parse(grant.expiresAt);
        const ttlSeconds = Number.isFinite(expiresAt) ? Math.round((expiresAt - Date.now()) / 1000) : -1;
        evidence.grantTtlSeconds = ttlSeconds;
        if (ttlSeconds < 0 || ttlSeconds > 900) {
          evidence.failure = `grant ttl ${ttlSeconds}s exceeds the 900s cap`;
        } else {
          // 真实免凭据下载：签名链接本身不带登录态，走全局 fetch。
          const response = await fetch(grant.url);
          if (response.ok) {
            evidence.downloaded = 'fetched';
            evidence.downloadedBytes = (await response.arrayBuffer()).byteLength;
          } else {
            evidence.downloaded = 'failed';
          }
        }
      } else {
        evidence.grant = 'failed';
      }

      const citationsView = await handle.open({ kind: 'evidence', runId });
      if (citationsView.kind === 'evidence') evidence.citations = citationsView.citations.length;

      if (evidence.terminal === 'paged') {
        const terminalView = await handle.open({ kind: 'terminal', runId });
        if (terminalView.kind === 'terminal' && terminalView.lines.length === 0) evidence.terminal = 'empty';
      }
    } finally {
      try { handle.close('integration-complete'); } catch { /* 清理不得再抛 */ }
    }
  } catch (error) {
    evidence.listed = 'failed';
    evidence.failure = error instanceof Error ? error.message : String(error);
  } finally {
    runtime.dispose();
  }
  return evidence;
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitMaterialIntegrationEvidence(evidence: MaterialIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
