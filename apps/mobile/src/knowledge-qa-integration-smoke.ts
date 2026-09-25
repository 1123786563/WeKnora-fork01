import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileKnowledgeQARemote } from '@weknora/api-client/mobile/knowledge-qa';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice } from '@weknora/mobile-core';
import { streamAuthorizedSse } from './adapters/sse-stream.ts';

/**
 * T15（Issue #45）AC3 移动面集成证据：真实部署 + 真实授权通道 + 真实
 * knowledge-chat SSE + Task Office 编排。opt-in（WEKNORA_MOBILE_TEST_*），无凭据
 * 即 skip——不得伪造通过。撤权/无证据两场景的服务端权威证据在 Go 侧
 *（knowledge_evidence_test.go，真实 sqlite + 真实 share 行翻转）；本冒烟验证
 * 移动面闭环：提问 → evidence 帧在场 → 引用携带版本与时间 → 探针任务可归档。
 */

export type KnowledgeQAIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; question: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface KnowledgeQAIntegrationEvidence {
  deploymentOrigin: string;
  asked: 'answered' | 'failed';
  evidenceState: 'cited' | 'no_evidence' | 'revoked' | 'absent';
  citations: number;
  revisions: string[];
  retrievedAtParsed: boolean;
  semanticGraphUsed: boolean | 'absent';
  reasoningState: string | 'absent';
  archived: 'archived' | 'archive-failed' | 'skipped';
  commandTimestamp: string;
}

/** 与 T01 mobileRuntimeIntegrationConfig 相同的 opt-in 语义（自包含，不跨计划 import）。 */
export function knowledgeQAIntegrationConfig(env: Record<string, string | undefined>): KnowledgeQAIntegrationConfig {
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
  const question = env.WEKNORA_MOBILE_TEST_KNOWLEDGE_QUESTION?.trim() || '用一句话介绍本部署知识库中最相关的文档。';
  return { enabled: true, deploymentOrigin: parsed.origin, email, password, question };
}

export async function runKnowledgeQAIntegration(config: Extract<KnowledgeQAIntegrationConfig, { enabled: true }>): Promise<KnowledgeQAIntegrationEvidence> {
  const evidence: KnowledgeQAIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    asked: 'failed',
    evidenceState: 'absent',
    citations: 0,
    revisions: [],
    retrievedAtParsed: false,
    semanticGraphUsed: 'absent',
    reasoningState: 'absent',
    archived: 'skipped',
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
      // SseFetchLike 要求真实 Response（ok/body reader）；宿主全局 fetch 即真实通道
      //（与 task-detail 集成冒烟同型）。无此通道 authorizedEventStream 会抛
      // RUNTIME_STREAM_UNAVAILABLE，knowledge-chat SSE 全链路即不可验证。
      return (input, accessToken, onChunk) => streamAuthorizedSse(origin, input, accessToken, onChunk, fetch);
    },
  });
  const snapshot = await runtime.signIn({
    deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
    email: config.email,
    password: config.password,
  });
  if (snapshot.surface !== 'authorized') return evidence;

  const office = createTaskOffice({
    backend: createTaskOfficeRemote({
      origin: config.deploymentOrigin,
      request: (input) => runtime.authorizedRequest(input),
    }),
    knowledgeQA: createMobileKnowledgeQARemote({
      origin: config.deploymentOrigin,
      request: (input) => runtime.authorizedRequest(input),
      stream: (input, onChunk) => runtime.authorizedEventStream(input, onChunk),
    }),
    lease: () => runtime.scopeLease(),
  });

  try {
    const turn = await office.askKnowledge({ question: config.question });
    evidence.asked = 'answered';
    evidence.evidenceState = turn.evidence.state;
    evidence.citations = turn.evidence.citations.length;
    evidence.revisions = turn.evidence.citations.map((citation) => `v${citation.revision}`);
    evidence.retrievedAtParsed = turn.evidence.citations.every((citation) => !Number.isNaN(Date.parse(citation.retrievedAt)));
    evidence.semanticGraphUsed = turn.evidence.semanticGraphUsed;
    evidence.reasoningState = turn.evidence.reasoning.state;
    try {
      await office.archive(turn.sessionId);
      evidence.archived = 'archived';
    } catch {
      evidence.archived = 'archive-failed';
    }
  } catch {
    evidence.asked = 'failed'; // 如实记录：不伪造 evidenceState
  }
  return evidence;
}
