import test from 'node:test';
import assert from 'node:assert/strict';
import { parseAnswerEvidence, type AnswerEvidenceWire } from '../src/mobile/knowledge-evidence.ts';

const citedEnvelope = {
  state: 'cited',
  semantic_graph_used: false,
  retrieved_at: '2026-09-24T08:00:00Z',
  citations: [
    { citation_id: 'chunk-1', knowledge_id: 'doc-1', knowledge_base_id: 'kb-own', title: '手册', revision: 3, start_at: 10, end_at: 40, quote: '命中', kind: 'fact', retrieved_at: '2026-09-24T08:00:00Z' },
    { citation_id: 'chunk-2', knowledge_id: 'doc-2', knowledge_base_id: 'kb-shared', revision: 5, kind: 'fact', retrieved_at: '2026-09-24T08:00:00Z' },
  ],
  conclusions: [
    { kind: 'model_inferred', model_id: 'chat-model-1', citation_ids: ['chunk-1', 'chunk-2'] },
  ],
  reasoning: { state: 'not_requested' },
};

test('parseAnswerEvidence accepts a multi-source cited envelope and keeps wire fields verbatim', () => {
  const parsed = parseAnswerEvidence(citedEnvelope);
  assert.equal(parsed.state, 'cited');
  assert.equal(parsed.citations.length, 2);
  assert.equal(parsed.citations[0]!.revision, 3);
  assert.equal(parsed.citations[0]!.kind, 'fact');
  assert.equal(parsed.citations[1]!.knowledge_base_id, 'kb-shared');
  assert.deepEqual(parsed.conclusions[0]!.citation_ids, ['chunk-1', 'chunk-2']);
  assert.equal(parsed.semantic_graph_used, false);
});

test('parseAnswerEvidence accepts explicit no_evidence and reasoning-incomplete envelopes', () => {
  const noEvidence = parseAnswerEvidence({ state: 'no_evidence', semantic_graph_used: false, retrieved_at: '2026-09-24T08:00:00Z', citations: [], conclusions: [], reasoning: { state: 'not_requested' } });
  assert.equal(noEvidence.state, 'no_evidence');
  assert.equal(noEvidence.citations.length, 0);
  const incomplete = parseAnswerEvidence({ state: 'no_evidence', semantic_graph_used: false, retrieved_at: '2026-09-24T08:00:00Z', citations: [], conclusions: [], reasoning: { requested: true, mode: 'rules', state: 'incomplete', reason: 'semantic_reasoning_unavailable', retryable: true } });
  assert.equal(incomplete.reasoning.state, 'incomplete');
  assert.equal(incomplete.reasoning.retryable, true);
});

test('parseAnswerEvidence rejects malformed and inconsistent envelopes wholesale', () => {
  const cases: unknown[] = [
    null,
    'cited',
    {},
    { ...citedEnvelope, state: 'maybe' },
    { ...citedEnvelope, retrieved_at: '2026-09-24 08:00:00' },
    { ...citedEnvelope, citations: [] },
    { ...citedEnvelope, conclusions: [] },
    { ...citedEnvelope, citations: [{ ...citedEnvelope.citations[0], kind: 'model_inferred' }] },
    { ...citedEnvelope, citations: [{ ...citedEnvelope.citations[0], revision: -1 }] },
    { ...citedEnvelope, citations: [{ ...citedEnvelope.citations[0], retrieved_at: 'yesterday' }] },
    { ...citedEnvelope, conclusions: [{ kind: 'rule_derived', citation_ids: ['chunk-1'] }] },
    { ...citedEnvelope, conclusions: [{ kind: 'model_inferred', citation_ids: ['missing'] }] },
    { ...citedEnvelope, conclusions: [{ kind: 'oracle', model_id: 'x' }] },
    { state: 'no_evidence', semantic_graph_used: false, retrieved_at: '2026-09-24T08:00:00Z', citations: citedEnvelope.citations, conclusions: [], reasoning: { state: 'not_requested' } },
  ];
  for (const value of cases) {
    assert.throws(() => parseAnswerEvidence(value), undefined, `must reject ${JSON.stringify(value).slice(0, 60)}`);
  }
});

test('parseAnswerEvidence type-level sanity: parsed cited envelope matches AnswerEvidenceWire', () => {
  const parsed: AnswerEvidenceWire = parseAnswerEvidence(citedEnvelope);
  assert.ok(Array.isArray(parsed.citations));
  assert.ok(Array.isArray(parsed.conclusions));
});

test('parseAnswerEvidence rejects non-boolean reasoning.requested/retryable instead of coercing to false', () => {
  // 回归（修复轮审查 finding）：requested/retryable 存在但非 boolean 时必须整体拒绝，
  // 不得静默归 false；缺省（undefined）仍兼容——Go 生产者 EvidenceReasoning 的
  // Requested/Retryable 无 omitempty，真实 wire 恒带这两个 boolean 键。
  const base = { state: 'no_evidence', semantic_graph_used: false, retrieved_at: '2026-09-24T08:00:00Z', citations: [], conclusions: [] };
  assert.throws(() => parseAnswerEvidence({ ...base, reasoning: { state: 'not_requested', requested: 'yes', retryable: false } }), undefined, 'must reject string requested');
  assert.throws(() => parseAnswerEvidence({ ...base, reasoning: { state: 'not_requested', requested: false, retryable: 1 } }), undefined, 'must reject numeric retryable');
  assert.throws(() => parseAnswerEvidence({ ...base, reasoning: { state: 'not_requested', requested: null } }), undefined, 'must reject null requested');
  const explicitFalse = parseAnswerEvidence({ ...base, reasoning: { state: 'not_requested', requested: false, retryable: false } });
  assert.equal(explicitFalse.reasoning.requested, false);
  assert.equal(explicitFalse.reasoning.retryable, false);
});
