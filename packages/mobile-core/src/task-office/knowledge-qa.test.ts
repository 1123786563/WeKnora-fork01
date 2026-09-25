import test from 'node:test';
import assert from 'node:assert/strict';
import {
  EVIDENCE_KIND_LABEL, createScenarioKnowledgeQABackend, evidenceRetryable,
  type AnswerEvidenceView,
} from './knowledge-qa.ts';

function citedEvidence(): AnswerEvidenceView {
  return {
    state: 'cited',
    semanticGraphUsed: false,
    retrievedAt: '2026-09-24T08:00:00Z',
    citations: [
      { citationId: 'chunk-1', knowledgeId: 'doc-1', knowledgeBaseId: 'kb-own', title: '手册', revision: 3, kind: 'fact', retrievedAt: '2026-09-24T08:00:00Z' },
      { citationId: 'chunk-2', knowledgeId: 'doc-2', knowledgeBaseId: 'kb-shared', revision: 5, kind: 'fact', retrievedAt: '2026-09-24T08:00:00Z' },
    ],
    conclusions: [{ kind: 'model_inferred', modelId: 'chat-model-1', citationIds: ['chunk-1', 'chunk-2'] }],
    reasoning: { requested: false, state: 'not_requested', retryable: false },
  };
}

test('kind labels keep the three classes distinct for display', () => {
  assert.equal(EVIDENCE_KIND_LABEL.fact, '原文事实');
  assert.equal(EVIDENCE_KIND_LABEL.rule_derived, '规则推导');
  assert.equal(EVIDENCE_KIND_LABEL.model_inferred, '模型推断');
});

test('evidenceRetryable is true only for an explicitly requested, incomplete reasoning turn', () => {
  assert.equal(evidenceRetryable(citedEvidence()), false);
  const incomplete: AnswerEvidenceView = {
    state: 'no_evidence', semanticGraphUsed: false, retrievedAt: '2026-09-24T08:00:00Z',
    citations: [], conclusions: [],
    reasoning: { requested: true, mode: 'rules', state: 'incomplete', reason: 'semantic_reasoning_unavailable', retryable: true },
  };
  assert.equal(evidenceRetryable(incomplete), true);
  const unrequested: AnswerEvidenceView = { ...incomplete, reasoning: { requested: false, state: 'not_requested', retryable: false } };
  assert.equal(evidenceRetryable(unrequested), false);
});

test('scenario backend records ask calls and returns scripted turns', async () => {
  const backend = createScenarioKnowledgeQABackend({
    ask: async (input) => ({ answer: '答', isFallback: false, evidence: citedEvidence() }),
  });
  const turn = await backend.ask({ sessionId: 'sess-1', question: '问', knowledgeBaseIds: ['kb-own'] });
  assert.equal(turn.answer, '答');
  assert.equal(turn.evidence.citations.length, 2);
  assert.deepEqual(backend.calls, [{ kind: 'ask', input: { sessionId: 'sess-1', question: '问', knowledgeBaseIds: ['kb-own'] } }]);
  const empty = createScenarioKnowledgeQABackend();
  const none = await empty.ask({ sessionId: 'sess-1', question: '问' });
  assert.equal(none.evidence.citations.length, 0);
});
