// T15（Issue #45）：knowledge-chat SSE evidence 帧的信封契约。键集与
// internal/types/evidence.go 的 JSON tag 逐字对应；解析器整体拒绝（不部分渲染），
// 与服务端 ValidateAnswerEvidence 同一组不变量的客户端侧守门。

export type EvidenceKindWire = 'fact' | 'rule_derived' | 'model_inferred';
export type AnswerEvidenceStateWire = 'cited' | 'no_evidence' | 'revoked';
export type EvidenceReasoningStateWire = 'not_requested' | 'incomplete';

export interface EvidenceCitationWire {
  citation_id: string;
  knowledge_id: string;
  knowledge_base_id: string;
  title?: string;
  revision: number;
  start_at?: number;
  end_at?: number;
  quote?: string;
  kind: 'fact';
  retrieved_at: string;
}

export interface EvidenceConclusionWire {
  kind: EvidenceKindWire;
  model_id?: string;
  rule_ids?: string[];
  citation_ids?: string[];
}

export interface EvidenceReasoningWire {
  requested: boolean;
  mode?: string;
  state: EvidenceReasoningStateWire;
  reason?: string;
  retryable: boolean;
}

export interface AnswerEvidenceWire {
  state: AnswerEvidenceStateWire;
  semantic_graph_used: boolean;
  retrieved_at: string;
  citations: EvidenceCitationWire[];
  conclusions: EvidenceConclusionWire[];
  reasoning: EvidenceReasoningWire;
}

const rfc3339 = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/;

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function stringField(row: Record<string, unknown>, key: string, path: string, optional = false): string | undefined {
  const value = row[key];
  if (value === undefined) {
    if (optional) return undefined;
    throw new Error(`${path}.${key} is required`);
  }
  if (typeof value !== 'string' || value === '') throw new Error(`${path}.${key} must be a non-empty string`);
  return value;
}

function intField(row: Record<string, unknown>, key: string, path: string): number {
  const value = row[key];
  if (typeof value !== 'number' || !Number.isSafeInteger(value)) throw new Error(`${path}.${key} must be an integer`);
  return value;
}

function parseCitation(value: unknown, path: string): EvidenceCitationWire {
  if (!isObject(value)) throw new Error(`${path} must be an object`);
  if (value.kind !== 'fact') throw new Error(`${path}.kind must be "fact"`);
  const revision = intField(value, 'revision', path);
  if (revision < 0) throw new Error(`${path}.revision must not be negative`);
  const retrievedAt = stringField(value, 'retrieved_at', path)!;
  if (!rfc3339.test(retrievedAt)) throw new Error(`${path}.retrieved_at must be RFC3339`);
  const citation: EvidenceCitationWire = {
    citation_id: stringField(value, 'citation_id', path)!,
    knowledge_id: stringField(value, 'knowledge_id', path)!,
    knowledge_base_id: stringField(value, 'knowledge_base_id', path)!,
    revision,
    kind: 'fact',
    retrieved_at: retrievedAt,
  };
  const title = stringField(value, 'title', path, true);
  if (title !== undefined) citation.title = title;
  const quote = stringField(value, 'quote', path, true);
  if (quote !== undefined) citation.quote = quote;
  if (value.start_at !== undefined) citation.start_at = intField(value, 'start_at', path);
  if (value.end_at !== undefined) citation.end_at = intField(value, 'end_at', path);
  return citation;
}

function parseConclusion(value: unknown, path: string, citationIds: Set<string>): EvidenceConclusionWire {
  if (!isObject(value)) throw new Error(`${path} must be an object`);
  const kind = value.kind;
  if (kind !== 'fact' && kind !== 'rule_derived' && kind !== 'model_inferred') {
    throw new Error(`${path}.kind must be fact | rule_derived | model_inferred`);
  }
  const conclusion: EvidenceConclusionWire = { kind };
  const modelId = stringField(value, 'model_id', path, true);
  const ruleIds = value.rule_ids;
  if (kind === 'model_inferred') {
    if (modelId === undefined) throw new Error(`${path} model_inferred requires model_id`);
    conclusion.model_id = modelId;
  }
  if (kind === 'rule_derived') {
    if (!Array.isArray(ruleIds) || ruleIds.length === 0 || !ruleIds.every((id) => typeof id === 'string' && id !== '')) {
      throw new Error(`${path} rule_derived requires non-empty rule_ids`);
    }
    conclusion.rule_ids = ruleIds as string[];
  }
  if (value.citation_ids !== undefined) {
    if (!Array.isArray(value.citation_ids) || !value.citation_ids.every((id) => typeof id === 'string')) {
      throw new Error(`${path}.citation_ids must be an array of strings`);
    }
    for (const id of value.citation_ids as string[]) {
      if (!citationIds.has(id)) throw new Error(`${path} references unknown citation ${id}`);
    }
    conclusion.citation_ids = value.citation_ids as string[];
  }
  return conclusion;
}

export function parseAnswerEvidence(value: unknown): AnswerEvidenceWire {
  if (!isObject(value)) throw new Error('answer evidence must be an object');
  const state = value.state;
  if (state !== 'cited' && state !== 'no_evidence' && state !== 'revoked') {
    throw new Error('answer evidence state must be cited | no_evidence | revoked');
  }
  if (typeof value.semantic_graph_used !== 'boolean') throw new Error('answer evidence semantic_graph_used must be a boolean');
  const retrievedAt = stringField(value, 'retrieved_at', 'answer evidence')!;
  if (!rfc3339.test(retrievedAt)) throw new Error('answer evidence retrieved_at must be RFC3339');
  if (!Array.isArray(value.citations)) throw new Error('answer evidence citations must be an array');
  if (!Array.isArray(value.conclusions)) throw new Error('answer evidence conclusions must be an array');
  const citations = value.citations.map((citation, index) => parseCitation(citation, `citations[${index}]`));
  const citationIds = new Set(citations.map((citation) => citation.citation_id));
  if (citationIds.size !== citations.length) throw new Error('answer evidence citation_id is duplicated');
  const conclusions = value.conclusions.map((conclusion, index) => parseConclusion(conclusion, `conclusions[${index}]`, citationIds));
  if (state === 'cited' && citations.length === 0) throw new Error('cited answer evidence requires at least one citation');
  if (state === 'cited' && conclusions.length === 0) throw new Error('cited answer evidence requires at least one conclusion');
  if (state !== 'cited' && (citations.length > 0 || conclusions.length > 0)) {
    throw new Error(`${state} answer evidence must not carry citations or conclusions`);
  }
  const reasoningValue = value.reasoning;
  if (!isObject(reasoningValue)) throw new Error('answer evidence reasoning is required');
  const reasoningState = reasoningValue.state;
  if (reasoningState !== 'not_requested' && reasoningState !== 'incomplete') {
    throw new Error('answer evidence reasoning.state must be not_requested | incomplete');
  }
  const reasoning: EvidenceReasoningWire = {
    requested: reasoningValue.requested === true,
    state: reasoningState,
    retryable: reasoningValue.retryable === true,
  };
  const mode = stringField(reasoningValue, 'mode', 'answer evidence reasoning', true);
  if (mode !== undefined) reasoning.mode = mode;
  const reason = stringField(reasoningValue, 'reason', 'answer evidence reasoning', true);
  if (reason !== undefined) reasoning.reason = reason;
  return { state, semantic_graph_used: value.semantic_graph_used, retrieved_at: retrievedAt, citations, conclusions, reasoning };
}
