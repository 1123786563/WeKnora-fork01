import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createMobileCodeDeliveryRemote } from '@weknora/api-client/mobile/code-delivery';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createDeliveryRecovery, createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, DeliveryRecoveryError } from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type DeliveryIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; recover: boolean }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface DeliveryIntegrationEvidence {
  deploymentOrigin: string;
  login: 'ok' | 'failed';
  executionListed: 'listed' | 'no-tasks' | 'failed';
  deliveryRead: 'read' | 'absent' | 'failed';
  recovery: 'skipped' | 'not-needed' | 'recovered' | 'failed';
  recoveryState?: string;
  deliveryState?: string;
  commitSha?: string;
  prUrl?: string;
  approver?: string;
  remoteLogin?: string;
  failure?: string;
  commandTimestamp: string;
}

/** 与 T04/T05/T16 相同的 opt-in 语义（自包含，不跨计划 import 凭据逻辑）。 */
export function deliveryIntegrationConfig(env: Record<string, string | undefined>): DeliveryIntegrationConfig {
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
  if (parsed.protocol !== 'https:' || parsed.username !== '' || parsed.password !== '' || parsed.pathname !== '/' || parsed.search !== '' || parsed.hash !== '') {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  if (disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL')) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL host is disallowed (private/loopback/reserved)' };
  }
  return { enabled: true, deploymentOrigin, email, password, recover: env.WEKNORA_MOBILE_TEST_DELIVERY_RECOVER === '1' };
}

/** 将恢复调用结果映射为可审计证据；不重试，也不把未完成错误伪装为成功。 */
export function deliveryRecoveryEvidenceOf(result: { state: string } | undefined, error?: unknown): Pick<DeliveryIntegrationEvidence, 'recovery' | 'recoveryState' | 'failure'> {
  if (error === undefined && result !== undefined) return { recovery: 'recovered', recoveryState: result.state };
  if (error instanceof DeliveryRecoveryError && (error.code === 'DELIVERY_STATE_CONFLICT' || error.code === 'DELIVERY_INVALID_INPUT')) {
    return { recovery: 'not-needed' };
  }
  const detail = error instanceof Error ? error.message : String(error);
  return { recovery: 'failed', failure: `recovery: ${detail}` };
}

/** 真实 transport + Runtime 授权通道 + 交付读面。默认流程只读；显式启用 recovery 时
 * 可通过授权通道发起 dispatch/resolve（真实交付链证据由 Go 侧 blocked-env 测试承载）。装配与 material-integration-smoke.ts
 * 的 runMaterialIntegration 同构。 */
export async function runDeliveryIntegration(config: Extract<DeliveryIntegrationConfig, { enabled: true }>): Promise<DeliveryIntegrationEvidence> {
  const evidence: DeliveryIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin, login: 'failed', executionListed: 'failed', deliveryRead: 'failed', recovery: 'skipped',
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
    evidence.login = 'ok';

    const office = createTaskOffice({
      backend: createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
      lease: () => runtime.scopeLease(),
    });
    const page = await office.tasks({});
    if (page.items.length === 0) { evidence.executionListed = 'no-tasks'; evidence.deliveryRead = 'absent'; return evidence; }
    evidence.executionListed = 'listed';

    const remote = createMobileCodeDeliveryRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) });
    for (const item of page.items) {
      const record = await remote.delivery(item.runId);
      if (record !== null) {
        evidence.deliveryRead = 'read';
        evidence.deliveryState = record.state;
        evidence.commitSha = record.commitSha;
        evidence.prUrl = record.prUrl;
        evidence.approver = record.approver;
        evidence.remoteLogin = record.remoteLogin;
        if (config.recover) {
          if (record.state === 'delivered') {
            evidence.recovery = 'not-needed';
            return evidence;
          }
          let actionInvoked = false;
          const recovery = createDeliveryRecovery({
            remote: {
              ...remote,
              dispatchDelivery(input) { actionInvoked = true; return remote.dispatchDelivery(input); },
              resolveDelivery(input) { actionInvoked = true; return remote.resolveDelivery(input); },
            },
            lease: () => runtime.scopeLease(),
          });
          try {
            const recovered = await recovery.recover({ runId: item.runId, deliveryId: record.id });
            Object.assign(evidence, actionInvoked ? deliveryRecoveryEvidenceOf(recovered) : { recovery: 'not-needed' });
          } catch (error) {
            if (error instanceof DeliveryRecoveryError && (error.code === 'DELIVERY_STATE_CONFLICT' || error.code === 'DELIVERY_INVALID_INPUT')) {
              Object.assign(evidence, deliveryRecoveryEvidenceOf(undefined, error));
            } else {
              const recoveryEvidence = deliveryRecoveryEvidenceOf(undefined, error);
              evidence.recovery = recoveryEvidence.recovery;
              evidence.failure = evidence.failure ? `${evidence.failure}; ${recoveryEvidence.failure}` : recoveryEvidence.failure;
            }
          }
        }
        return evidence;
      }
    }
    evidence.deliveryRead = 'absent';
    return evidence;
  } catch (error) {
    evidence.failure = error instanceof Error ? error.message : String(error);
    return evidence;
  } finally {
    runtime.dispose();
  }
}

/** 证据以 JSON 行吐出（供批次报告引用）。 */
export function emitDeliveryIntegrationEvidence(evidence: DeliveryIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify({ kind: 'delivery-integration', ...evidence }));
}
