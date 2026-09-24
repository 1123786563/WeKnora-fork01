import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createScenarioTaskBackend } from './in-memory-task-backend.ts';
import { createScenarioKnowledgeQABackend } from './knowledge-qa.ts';
import { createTaskOffice, TaskOfficeError } from './task-office.ts';

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: 'tenant-1' });
  return { revocable, lease: revocable.asScopeLease() };
}

function officeWithKnowledgeQA(leaseRef: { lease?: ScopeLease }, handlers: Parameters<typeof createScenarioKnowledgeQABackend>[0] = {}, created: string[] = []) {
  const backend = createScenarioTaskBackend({
    createSession: async (input) => {
      created.push(input.title);
      return { sessionId: `new-${created.length}` };
    },
  });
  const knowledgeQA = createScenarioKnowledgeQABackend(handlers);
  const office = createTaskOffice({ backend, knowledgeQA, lease: () => leaseRef.lease });
  return { backend, knowledgeQA, office };
}

test('askKnowledge creates a session for a quick question and returns the evidence turn', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const created: string[] = [];
  const { knowledgeQA, office } = officeWithKnowledgeQA(leaseRef, {
    ask: async (input) => ({ answer: '回答', isFallback: false, evidence: citedOfficeEvidence() }),
  }, created);

  const turn = await office.askKnowledge({ question: '  依赖关系是什么？  ', knowledgeBaseIds: ['kb-own'] });

  assert.equal(turn.sessionId, 'new-1');
  assert.equal(turn.answer, '回答');
  assert.equal(turn.evidence.citations.length, 2);
  assert.deepEqual(created, ['依赖关系是什么？'.slice(0, 60)]);
  assert.deepEqual(knowledgeQA.calls, [{ kind: 'ask', input: { sessionId: 'new-1', question: '依赖关系是什么？', knowledgeBaseIds: ['kb-own'] } }]);
});

test('askKnowledge reuses a provided sessionId and never creates a second session', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const created: string[] = [];
  const { knowledgeQA, office } = officeWithKnowledgeQA(leaseRef, {
    ask: async () => ({ answer: '追问回答', isFallback: false, evidence: citedOfficeEvidence() }),
  }, created);

  const turn = await office.askKnowledge({ question: '再展开讲讲', sessionId: 'sess-existing' });

  assert.equal(turn.sessionId, 'sess-existing');
  assert.deepEqual(created, [], '同一 Task 的追问绝不新建 session（ADR-0004）');
  assert.equal(knowledgeQA.calls[0]!.input.sessionId, 'sess-existing');
});

test('askKnowledge fails closed without a knowledgeQA port and rejects invalid input', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const backend = createScenarioTaskBackend({});
  const office = createTaskOffice({ backend, lease: () => leaseRef.lease });
  await assert.rejects(
    office.askKnowledge({ question: '问' }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_KNOWLEDGE_QA_UNAVAILABLE',
  );
  // 空白/超长问题在端口检查之前就被 INVALID_INPUT 拒绝（office 无端口也先命中输入校验）。
  await assert.rejects(
    office.askKnowledge({ question: '   ' }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT',
  );
  await assert.rejects(
    office.askKnowledge({ question: 'x'.repeat(8001) }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT',
  );
});

test('a late ask response after the scope lease was revoked is rejected, never resolves with foreign-scope data', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  let releaseAsk: ((value: { answer: string; isFallback: boolean; evidence: ReturnType<typeof citedOfficeEvidence> }) => void) | undefined;
  const askGate = new Promise<{ answer: string; isFallback: boolean; evidence: ReturnType<typeof citedOfficeEvidence> }>((resolve) => { releaseAsk = resolve; });
  const { office } = officeWithKnowledgeQA(leaseRef, { ask: () => askGate });

  const pending = office.askKnowledge({ question: '问' });
  revocable.revoke();
  releaseAsk!({ answer: 'foreign', isFallback: false, evidence: citedOfficeEvidence() });
  await assert.rejects(pending, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
});

test('askKnowledge invalidates in-flight list reads like followUp does', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  // list 必须挂起：若它先返回，settle 已在旧 epoch 下完成，就测不到作废语义。
  let resolveList: ((page: { items: []; nextCursor?: undefined }) => void) | undefined;
  const listGate = new Promise<{ items: []; nextCursor?: undefined }>((resolve) => { resolveList = resolve; });
  const backend = createScenarioTaskBackend({ list: () => listGate });
  const knowledgeQA = createScenarioKnowledgeQABackend({ ask: async () => ({ answer: '答', isFallback: false, evidence: citedOfficeEvidence() }) });
  const office = createTaskOffice({ backend, knowledgeQA, lease: () => leaseRef.lease });

  const older = office.tasks({});
  await office.askKnowledge({ question: '问' });
  resolveList!({ items: [] });
  await assert.rejects(older, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SUPERSEDED');
});

function citedOfficeEvidence() {
  return {
    state: 'cited' as const,
    semanticGraphUsed: false,
    retrievedAt: '2026-09-24T08:00:00Z',
    citations: [
      { citationId: 'chunk-1', knowledgeId: 'doc-1', knowledgeBaseId: 'kb-own', title: '手册', revision: 3, kind: 'fact' as const, retrievedAt: '2026-09-24T08:00:00Z' },
      { citationId: 'chunk-2', knowledgeId: 'doc-2', knowledgeBaseId: 'kb-shared', revision: 5, kind: 'fact' as const, retrievedAt: '2026-09-24T08:00:00Z' },
    ],
    conclusions: [{ kind: 'model_inferred' as const, modelId: 'chat-model-1', citationIds: ['chunk-1', 'chunk-2'] }],
    reasoning: { requested: false, state: 'not_requested' as const, retryable: false },
  };
}
