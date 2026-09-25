import test from 'node:test';
import assert from 'node:assert/strict';
import type { ClientRequest } from '../client.ts';
import { createMobileKnowledgeQARemote } from './knowledge-qa.ts';

const origin = 'https://weknora.example.test';

function sseFrame(payload: unknown): string {
	return `data: ${JSON.stringify(payload)}\n\n`;
}

function harness(frames: string[]) {
	const calls: ClientRequest[] = [];
	const remote = createMobileKnowledgeQARemote({
		origin,
		request: async () => {
			throw new Error('ask must use the stream transport only');
		},
		stream: async (input, onChunk) => {
			calls.push(input);
			for (const frame of frames) {
				onChunk(frame);
			}
		},
	});
	return { remote, calls };
}

const evidenceFrame = {
	response_type: 'evidence',
	data: {
		evidence: {
			state: 'cited',
			semantic_graph_used: false,
			retrieved_at: '2026-09-24T08:00:00Z',
			citations: [
				{ citation_id: 'chunk-1', knowledge_id: 'doc-1', knowledge_base_id: 'kb-own', title: '手册', revision: 3, kind: 'fact', retrieved_at: '2026-09-24T08:00:00Z' },
				{ citation_id: 'chunk-2', knowledge_id: 'doc-2', knowledge_base_id: 'kb-shared', revision: 5, kind: 'fact', retrieved_at: '2026-09-24T08:00:00Z' },
			],
			conclusions: [{ kind: 'model_inferred', model_id: 'chat-model-1', citation_ids: ['chunk-1', 'chunk-2'] }],
			reasoning: { state: 'not_requested' },
		},
	},
};

test('ask posts to knowledge-chat SSE with the selected knowledge and returns the evidence turn', async () => {
	const { remote, calls } = harness([
		sseFrame(evidenceFrame),
		sseFrame({ response_type: 'answer', content: '依赖', done: false }),
		sseFrame({ response_type: 'answer', content: '关系如下', done: true }),
		sseFrame({ response_type: 'complete' }),
	]);
	const turn = await remote.ask({ sessionId: 'sess 1', question: '依赖关系', knowledgeBaseIds: ['kb-own'] });
	assert.equal(turn.answer, '依赖关系如下');
	assert.equal(turn.isFallback, false);
	assert.equal(turn.evidence.state, 'cited');
	assert.equal(turn.evidence.citations.length, 2);
	assert.equal(turn.evidence.citations[0]!.citationId, 'chunk-1');
	assert.equal(turn.evidence.citations[0]!.revision, 3);
	assert.equal(turn.evidence.conclusions[0]!.kind, 'model_inferred');
	assert.equal(calls.length, 1);
	assert.equal(calls[0]!.method, 'POST');
	assert.equal(calls[0]!.path, '/api/v1/knowledge-chat/sess%201');
	assert.deepEqual(calls[0]!.body, { query: '依赖关系', knowledge_base_ids: ['kb-own'] });
	assert.equal((calls[0]!.headers as Record<string, string>)['accept'], 'text/event-stream');
});

test('ask records fallback answers and no-evidence envelopes', async () => {
	const { remote } = harness([
		sseFrame({ response_type: 'evidence', data: { evidence: { state: 'no_evidence', semantic_graph_used: false, retrieved_at: '2026-09-24T08:00:00Z', citations: [], conclusions: [], reasoning: { state: 'not_requested' } } } }),
		sseFrame({ response_type: 'answer', content: '抱歉，无法回答。', done: true, data: { is_fallback: true } }),
		sseFrame({ response_type: 'complete' }),
	]);
	const turn = await remote.ask({ sessionId: 's', question: 'q' });
	assert.equal(turn.isFallback, true);
	assert.equal(turn.evidence.state, 'no_evidence');
	assert.equal(turn.evidence.citations.length, 0);
});

test('ask fails closed on error frames, malformed frames, missing evidence and truncated streams', async () => {
	await assert.rejects(
		harness([sseFrame({ response_type: 'error' })]).remote.ask({ sessionId: 's', question: 'q' }),
		/KNOWLEDGE_QA_FAILED/,
	);
	await assert.rejects(
		harness(['data: {not json\n\n', sseFrame({ response_type: 'complete' })]).remote.ask({ sessionId: 's', question: 'q' }),
		/KNOWLEDGE_QA_MALFORMED_FRAME/,
	);
	await assert.rejects(
		harness([sseFrame({ response_type: 'answer', content: '答', done: true }), sseFrame({ response_type: 'complete' })]).remote.ask({ sessionId: 's', question: 'q' }),
		/KNOWLEDGE_QA_MISSING_EVIDENCE/,
		'旧服务端（无 evidence 帧）不得被当作有证据回答',
	);
	await assert.rejects(
		harness([sseFrame(evidenceFrame), sseFrame({ response_type: 'answer', content: '答', done: true })]).remote.ask({ sessionId: 's', question: 'q' }),
		/KNOWLEDGE_QA_TRUNCATED/,
		'无终止帧＝结果未知，不得静默判成功',
	);
	await assert.rejects(
		harness([sseFrame({ response_type: 'evidence', data: { evidence: { state: 'cited', semantic_graph_used: false, retrieved_at: '2026-09-24T08:00:00Z', citations: [], conclusions: [], reasoning: { state: 'not_requested' } } } }), sseFrame({ response_type: 'complete' })]).remote.ask({ sessionId: 's', question: 'q' }),
		/cited answer evidence requires/,
		'不自洽信封整体拒绝',
	);
});

test('ask rejects evidence frames arriving after the terminal frame (out-of-order stream, fail closed)', async () => {
	await assert.rejects(
		harness([
			sseFrame({ response_type: 'complete' }),
			sseFrame(evidenceFrame),
		]).remote.ask({ sessionId: 's', question: 'q' }),
		/KNOWLEDGE_QA_EVIDENCE_AFTER_TERMINAL/,
		'晚于终止帧到达的 evidence＝流序错乱，不得采信',
	);
});

test('constructor validates the deployment origin and ask validates its input', async () => {
	assert.throws(() => createMobileKnowledgeQARemote({ origin: 'http://insecure.example.test', request: async () => undefined, stream: async () => undefined }));
	const { remote } = harness([sseFrame({ response_type: 'complete' })]);
	await assert.rejects(remote.ask({ sessionId: ' ', question: 'q' }), /sessionId/);
	await assert.rejects(remote.ask({ sessionId: 's', question: '' }), /question/);
});
