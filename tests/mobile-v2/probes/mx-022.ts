// MX-022 probe · 知识撤权观察器
// frozen 场景：cached-knowledge-title × server-403。
// 真实 resource-presentation 领域模型：曾有缓存标题，服务端 403 后——
// 敏感字段零可见（撤权投影）、「用此知识提问」禁用（引用构造返回 null）。
import {
  knowledgeRefForPrompt,
  revokedKnowledgeProjection,
  toKnowledgeResource,
} from '../../../packages/domain/src/mobile/resource-presentation.ts';

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  visibleSensitiveFields: string[];
  canAskWithKnowledge: boolean;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'cached-knowledge-title' || input.fault !== 'server-403') {
    throw new Error(`unsupported fixture/fault: ${input.fixture}/${input.fault}`);
  }
  // 客户端曾缓存的知识标题（撤权前真实投影）
  const cached = toKnowledgeResource({ id: 'kb-1', title: '内部财务知识库', scan_status: 'indexed', document_count: 12, updated_at: '2026-09-18T07:00:00Z' });
  if (cached.title !== '内部财务知识库' || cached.scanStatus !== 'indexed') {
    throw new Error('precondition: cached projection intact before revocation');
  }

  // 服务端 403：撤权投影替代缓存回填——敏感字段不可见
  const revoked = revokedKnowledgeProjection();
  // 引用构造在撤权下必须返回 null（提问按钮禁用的领域依据）
  const ref = knowledgeRefForPrompt(cached, { revoked: true });
  if (ref !== null) throw new Error('knowledge ref must be null after revocation');
  const allowed = knowledgeRefForPrompt(cached, { revoked: false });
  if (allowed?.kind !== 'knowledge_ref' || allowed.knowledgeId !== 'kb-1') {
    throw new Error('knowledge ref must carry id-only reference when allowed');
  }

  // visibleSensitiveFields：撤权投影携带的字段集合（空=敏感零可见）
  const visibleSensitiveFields = Object.keys(revoked).filter((key) => {
    const value = (revoked as Record<string, unknown>)[key];
    return Array.isArray(value) ? (value as unknown[]).length > 0 : value !== false && value !== undefined && value !== '';
  });
  return {
    visibleSensitiveFields,
    canAskWithKnowledge: ref !== null,
  };
}
