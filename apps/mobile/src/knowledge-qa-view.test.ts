import test from 'node:test';
import assert from 'node:assert/strict';
import type { KnowledgeResource } from '@weknora/domain/mobile';
import type { KnowledgeQATurn, TaskOffice } from '@weknora/mobile-core';
import {
  EVIDENCE_NO_EVIDENCE_COPY, EVIDENCE_REASONING_INCOMPLETE_COPY, EVIDENCE_REVOKED_COPY,
  createKnowledgeQAController, evidenceCitationLine, formatEvidenceTimestamp,
} from './knowledge-qa-view.ts';

function turn(overrides: Partial<KnowledgeQATurn['evidence']> = {}): KnowledgeQATurn {
  return {
    sessionId: 'sess-1',
    answer: 'A 间接依赖 C',
    isFallback: false,
    evidence: {
      state: 'cited',
      semanticGraphUsed: false,
      retrievedAt: '2026-09-24T08:00:00Z',
      citations: [
        { citationId: 'chunk-1', knowledgeId: 'doc-1', knowledgeBaseId: 'kb-own', title: '手册', revision: 3, kind: 'fact', retrievedAt: '2026-09-24T08:00:00Z' },
        { citationId: 'chunk-2', knowledgeId: 'doc-2', knowledgeBaseId: 'kb-shared', title: '指南', revision: 5, kind: 'fact', retrievedAt: '2026-09-24T08:00:01Z' },
      ],
      conclusions: [{ kind: 'model_inferred', modelId: 'chat-model-1', citationIds: ['chunk-1', 'chunk-2'] }],
      reasoning: { requested: false, state: 'not_requested', retryable: false },
      ...overrides,
    },
  };
}

function knowledge(): KnowledgeResource[] {
  return [
    { id: 'kb-own', title: '手册', scanStatus: 'indexed', documentCount: 3, updatedAt: '2026-09-01T00:00:00Z' },
    { id: 'kb-shared', title: '指南', scanStatus: 'indexed', documentCount: 1, updatedAt: '2026-09-01T00:00:00Z' },
  ];
}

function controllerWith(scripted: KnowledgeQATurn[] = [turn()], asked: Array<{ question: string; knowledgeBaseIds?: string[] }> = []) {
  let call = 0;
  const office = {
    askKnowledge: async (input: { question: string; knowledgeBaseIds?: string[] }) => {
      asked.push({ question: input.question, ...(input.knowledgeBaseIds === undefined ? {} : { knowledgeBaseIds: [...input.knowledgeBaseIds] }) });
      const next = scripted[Math.min(call, scripted.length - 1)]!;
      call += 1;
      return next;
    },
  } as unknown as Pick<TaskOffice, 'askKnowledge'>;
  return createKnowledgeQAController({ office, knowledge: async () => knowledge() });
}

test('ask passes the selected knowledge scope and renders the evidence turn', async () => {
  const asked: Array<{ question: string; knowledgeBaseIds?: string[] }> = [];
  const controller = controllerWith([turn()], asked);
  await controller.whenInitialized();
  controller.update({ question: '  依赖关系是什么  ' });
  controller.toggleKnowledge('kb-own');
  controller.toggleKnowledge('kb-shared');
  controller.toggleKnowledge('kb-shared'); // 再点一次取消选择
  const result = await controller.ask();
  assert.deepEqual(asked, [{ question: '依赖关系是什么', knowledgeBaseIds: ['kb-own'] }]);
  assert.equal(result!.sessionId, 'sess-1');
  const state = controller.state();
  assert.equal(state.phase, 'answered');
  assert.equal(state.turn!.evidence.citations.length, 2);
  assert.equal(state.question, '', '成功后清空输入，防重复提交');
  assert.deepEqual(state.selectedKnowledgeIds, ['kb-own']);
});

test('citation lines show source, version, readable time and kind distinctly', () => {
  const line = evidenceCitationLine(turn().evidence.citations[0]!);
  assert.ok(line.includes('手册'));
  assert.ok(line.includes('v3'));
  assert.ok(line.includes('2026-09-24 08:00:00 UTC'), '检索时间以无歧义的可读 UTC 形态展示');
  assert.ok(!line.includes('T08:00:00Z'), '不再直接暴露原始 RFC3339 串');
  assert.ok(line.includes('原文事实'));
});

test('evidence timestamps stay audit-exact and fall back to the raw string on unparsable input', () => {
  assert.equal(formatEvidenceTimestamp('2026-09-24T08:00:00Z'), '2026-09-24 08:00:00 UTC');
  assert.equal(formatEvidenceTimestamp('2026-09-24T08:00:01.500Z'), '2026-09-24 08:00:01 UTC', '亚秒精度丢弃不得进位篡改审计时刻');
  assert.equal(formatEvidenceTimestamp('2026-09-24T08:00:00+08:00'), '2026-09-24 00:00:00 UTC', '带时区偏移的输入归一化为 UTC，避免歧义');
  assert.equal(formatEvidenceTimestamp('not-a-timestamp'), 'not-a-timestamp', '不可解析原样保留，不丢审计信息');
  assert.equal(formatEvidenceTimestamp('2026-09-24 08:00:00'), '2026-09-24 08:00:00', '非 RFC3339 样式不强行转换');
});

test('no-evidence, revoked and reasoning-incomplete states get honest copy and retry only when retryable', async () => {
  const noEvidence = controllerWith([turn({ state: 'no_evidence', citations: [], conclusions: [] })]);
  await noEvidence.whenInitialized();
  await noEvidence.ask();
  assert.equal(noEvidence.state().statusLine, EVIDENCE_NO_EVIDENCE_COPY);

  const revoked = controllerWith([turn({ state: 'revoked', citations: [], conclusions: [] })]);
  await revoked.whenInitialized();
  await revoked.ask();
  assert.equal(revoked.state().statusLine, EVIDENCE_REVOKED_COPY);

  const incomplete = controllerWith([turn({
    state: 'no_evidence', citations: [], conclusions: [],
    reasoning: { requested: true, mode: 'rules', state: 'incomplete', reason: 'semantic_reasoning_unavailable', retryable: true },
  })]);
  await incomplete.whenInitialized();
  await incomplete.ask();
  assert.equal(incomplete.state().statusLine, EVIDENCE_REASONING_INCOMPLETE_COPY);
  assert.equal(incomplete.state().retryAvailable, true);
  const asked2: Array<{ question: string }> = [];
  const retried = controllerWith([
    turn({ state: 'no_evidence', citations: [], conclusions: [], reasoning: { requested: true, mode: 'rules', state: 'incomplete', reason: 'x', retryable: true } }),
    turn(),
  ], asked2 as Array<{ question: string }>);
  await retried.whenInitialized();
  await retried.ask();
  await retried.retry();
  assert.equal(retried.state().phase, 'answered');
  assert.equal(asked2.length, 2, 'retry re-asks the same question');
});

test('retry is unavailable for non-retryable turns; ask failure keeps the question and surfaces the error', async () => {
  const cited = controllerWith([turn()]);
  await cited.whenInitialized();
  await cited.ask();
  assert.equal(cited.state().retryAvailable, false);
  const failing = createKnowledgeQAController({
    office: {
      askKnowledge: async () => {
        throw new Error('KNOWLEDGE_QA_MISSING_EVIDENCE');
      },
    } as unknown as Pick<TaskOffice, 'askKnowledge'>,
  });
  await failing.whenInitialized();
  failing.update({ question: '保留我' });
  await failing.ask();
  const state = failing.state();
  assert.equal(state.phase, 'failed');
  assert.equal(state.question, '保留我', '失败不清空输入（可修改后重提）');
  assert.ok(state.error!.includes('KNOWLEDGE_QA_MISSING_EVIDENCE'));
});
