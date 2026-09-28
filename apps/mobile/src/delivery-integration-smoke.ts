import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createMobileCodeDeliveryRemote } from '@weknora/api-client/mobile/code-delivery';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createDeliveryRecovery, DeliveryRecoveryError, createInMemoryCredentialStore, createMobileRuntime, createTaskOffice } from '@weknora/mobile-core';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';

export type DeliveryIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; recover: boolean }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface DeliveryIntegrationEvidence {
  deploymentOrigin: string;
  login: 'ok' | 'failed';
  executionListed: 'listed' | 'no-tasks' | 'failed';
  deliveryRead: 'read' | 'absent' | 'failed';
  deliveryState?: string;
  recovery: 'skipped' | 'not-needed' | 'recovered' | 'failed';
  recoveryState?: string;
  recoveryRunId?: string;
  recoveryDeliveryId?: string;
  failureRunId?: string;
  commitSha?: string;
  prUrl?: string;
  approver?: string;
  remoteLogin?: string;
  failure?: string;
  commandTimestamp: string;
}

interface RecoveryCandidate {
  id: string;
  state: string;
  commitSha?: string;
  prUrl?: string;
  approver?: string;
  remoteLogin?: string;
}

export interface DeliveryRecoveryEvidenceResult<T extends RecoveryCandidate> {
  recovery: 'not-needed' | 'recovered' | 'failed';
  recoveryState?: string;
  recoveryRunId?: string;
  recoveryDeliveryId?: string;
  failureRunId?: string;
  firstDelivery?: { runId: string; delivery: T };
  evidenceDelivery?: { runId: string; delivery: T };
  failure?: string;
}

/** Scan delivery rows in task order and attempt at most one ambiguous recovery write. */
export async function runDeliveryRecoveryEvidence<T extends RecoveryCandidate>(ports: {
  runIds: string[];
  readDelivery(runId: string): Promise<T | null>;
  recover(input: { runId: string; deliveryId: string }): Promise<{ state: string }>;
}): Promise<DeliveryRecoveryEvidenceResult<T>> {
  const result: DeliveryRecoveryEvidenceResult<T> = { recovery: 'not-needed' };
  let lastReadState: string | undefined;
  let attemptedState: string | undefined;

  for (const runId of ports.runIds) {
    let delivery: T | null;
    try {
      delivery = await ports.readDelivery(runId);
    } catch (error) {
      result.recovery = 'failed';
      result.failure = error instanceof Error ? error.message : String(error);
      result.failureRunId = runId;
      result.recoveryRunId = undefined;
      result.recoveryDeliveryId = undefined;
      result.recoveryState = undefined;
      return result;
    }
    if (delivery === null) continue;

    result.firstDelivery ??= { runId, delivery };
    result.evidenceDelivery ??= { runId, delivery };
    lastReadState = delivery.state;
    if (delivery.state !== 'pushed' && delivery.state !== 'unknown') continue;

    result.recoveryRunId = runId;
    result.recoveryDeliveryId = delivery.id;
    result.evidenceDelivery = { runId, delivery };
    attemptedState = delivery.state;
    try {
      const recovered = await ports.recover({ runId, deliveryId: delivery.id });
      result.recovery = 'recovered';
      result.recoveryState = recovered.state;
      return result;
    } catch (error) {
      if (error instanceof DeliveryRecoveryError && (error.code === 'DELIVERY_STATE_CONFLICT' || error.code === 'DELIVERY_INVALID_INPUT')) {
        continue;
      }
      result.recovery = 'failed';
      result.recoveryState = delivery.state;
      result.failure = error instanceof Error ? error.message : String(error);
      return result;
    }
  }

  result.recoveryState = attemptedState ?? lastReadState;
  return result;
}

/** Apply the selected delivery and recovery outcome to the integration report row. */
export function applyDeliveryRecoveryEvidence<T extends RecoveryCandidate>(
  evidence: DeliveryIntegrationEvidence,
  result: DeliveryRecoveryEvidenceResult<T>,
): DeliveryIntegrationEvidence {
  const next = { ...evidence };
  const receipt = result.evidenceDelivery ?? result.firstDelivery;
  if (receipt !== undefined) {
    next.deliveryRead = 'read';
    next.deliveryState = receipt.delivery.state;
    next.commitSha = receipt.delivery.commitSha;
    next.prUrl = receipt.delivery.prUrl;
    next.approver = receipt.delivery.approver;
    next.remoteLogin = receipt.delivery.remoteLogin;
  } else {
    next.deliveryRead = result.failure ? 'failed' : 'absent';
  }
  next.recovery = result.recovery;
  next.recoveryState = result.recoveryState;
  next.recoveryRunId = result.recoveryRunId;
  next.recoveryDeliveryId = result.recoveryDeliveryId;
  next.failureRunId = result.failureRunId;
  if (result.failure) next.failure = next.failure ? `${next.failure}; ${result.failure}` : result.failure;
  return next;
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
  return {
    enabled: true,
    deploymentOrigin,
    email,
    password,
    recover: env.WEKNORA_MOBILE_TEST_DELIVERY_RECOVER === '1',
  };
}

/** 真实 transport + Runtime 授权通道 + 交付读面。默认只读；opt-in recovery 可通过
 * 授权测试部署 resolve/dispatch。真实交付链证据由 Go 侧 blocked-env 测试承载。 */
export async function runDeliveryIntegration(config: Extract<DeliveryIntegrationConfig, { enabled: true }>): Promise<DeliveryIntegrationEvidence> {
  const evidence: DeliveryIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin, login: 'failed', executionListed: 'failed', deliveryRead: 'failed',
    recovery: 'skipped',
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
    if (config.recover) {
      const recovery = createDeliveryRecovery({ remote, lease: () => runtime.scopeLease() });
      const recoveryEvidence = await runDeliveryRecoveryEvidence({
        runIds: page.items.map((item) => item.runId),
        readDelivery: (runId) => remote.delivery(runId),
        recover: (input) => recovery.recover(input),
      });
      return applyDeliveryRecoveryEvidence(evidence, recoveryEvidence);
    }

    for (const item of page.items) {
      const record = await remote.delivery(item.runId);
      if (record !== null) {
        evidence.deliveryRead = 'read';
        evidence.deliveryState = record.state;
        evidence.commitSha = record.commitSha;
        evidence.prUrl = record.prUrl;
        evidence.approver = record.approver;
        evidence.remoteLogin = record.remoteLogin;
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
