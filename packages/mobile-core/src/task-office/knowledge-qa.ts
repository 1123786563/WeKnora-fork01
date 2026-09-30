/**
 * T15（Issue #45）知识问答域合同：答案证据信封（版本 + 时间 + 三类区分）与
 * Knowledge QA 后端端口。wire（snake_case）→ 本 DTO（camelCase）的映射由
 * api-client 的 remote 完成；mobile-core 不依赖传输层（module-seams §3）。
 */

/** 三类结论/引用类别（CONTEXT.md「证据引用」；与 contracts EvidenceKindWire 一致）。 */
export type EvidenceKindValue = 'fact' | 'rule_derived' | 'model_inferred';
export type AnswerEvidenceStateValue = 'cited' | 'no_evidence' | 'revoked';
export type EvidenceReasoningStateValue = 'not_requested' | 'incomplete';

export interface EvidenceCitation {
  citationId: string;
  knowledgeId: string;
  knowledgeBaseId: string;
  title?: string;
  revision: number;
  startAt?: number;
  endAt?: number;
  quote?: string;
  kind: 'fact';
  retrievedAt: string;
}

export interface EvidenceConclusion {
  kind: EvidenceKindValue;
  modelId?: string;
  ruleIds?: string[];
  citationIds?: string[];
}

export interface EvidenceReasoning {
  requested: boolean;
  mode?: string;
  state: EvidenceReasoningStateValue;
  reason?: string;
  retryable: boolean;
}

export interface AnswerEvidenceView {
  state: AnswerEvidenceStateValue;
  semanticGraphUsed: boolean;
  retrievedAt: string;
  citations: EvidenceCitation[];
  conclusions: EvidenceConclusion[];
  reasoning: EvidenceReasoning;
}

export interface KnowledgeQATurnBody {
  answer: string;
  isFallback: boolean;
  evidence: AnswerEvidenceView;
}

/** 一次完整问答回合：会话身份 + 答案 + 证据。sessionId = taskId（ADR-0004）。 */
export interface KnowledgeQATurn extends KnowledgeQATurnBody {
  sessionId: string;
}

export interface KnowledgeQAAskInput {
  sessionId: string;
  question: string;
  knowledgeBaseIds?: string[];
  signal?: AbortSignal;
}

export interface KnowledgeQABackendPort {
  ask(input: KnowledgeQAAskInput): Promise<KnowledgeQATurnBody>;
}

/** 展示文案：三类必须可区分地呈现，不得把模型推断显示成来源事实。 */
export const EVIDENCE_KIND_LABEL: Record<EvidenceKindValue, string> = {
  fact: '原文事实',
  rule_derived: '规则推导',
  model_inferred: '模型推断',
};

/** 只有「显式请求过推理且未完成」的回合提供重试入口（ADR-0002）。 */
export function evidenceRetryable(evidence: AnswerEvidenceView): boolean {
  return evidence.reasoning.requested && evidence.reasoning.state === 'incomplete' && evidence.reasoning.retryable;
}

/** Scriptable scenario Adapter（module-seams §12）。 */
export interface ScenarioKnowledgeQABackendHandlers {
  ask?: (input: KnowledgeQAAskInput) => Promise<KnowledgeQATurnBody>;
}

export interface ScenarioKnowledgeQABackend extends KnowledgeQABackendPort {
  calls: Array<{ kind: 'ask'; input: KnowledgeQAAskInput }>;
}

export function createScenarioKnowledgeQABackend(handlers: ScenarioKnowledgeQABackendHandlers = {}): ScenarioKnowledgeQABackend {
  const calls: ScenarioKnowledgeQABackend['calls'] = [];
  return {
    calls,
    async ask(input) {
      const normalized: KnowledgeQAAskInput = {
        sessionId: input.sessionId,
        question: input.question,
        ...(input.knowledgeBaseIds === undefined ? {} : { knowledgeBaseIds: [...input.knowledgeBaseIds] }),
        ...(input.signal === undefined ? {} : { signal: input.signal }),
      };
      calls.push({ kind: 'ask', input: normalized });
      if (handlers.ask === undefined) {
        return { answer: '', isFallback: false, evidence: { state: 'no_evidence', semanticGraphUsed: false, retrievedAt: '', citations: [], conclusions: [], reasoning: { requested: false, state: 'not_requested', retryable: false } } };
      }
      return handlers.ask(normalized);
    },
  };
}
