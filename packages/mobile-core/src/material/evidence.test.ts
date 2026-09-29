import test from 'node:test';
import assert from 'node:assert/strict';
import { projectCitations } from './evidence.ts';

test('citations project tool and artifact events with their source binding', () => {
  const citations = projectCitations([
    { seq: 5, type: 'tool.started', occurredAt: '2026-09-24T00:00:05Z', payload: { tool: 'knowledge_search', knowledge_base_id: 'kb-7', query: 'handbook' } },
    { seq: 6, type: 'text.delta', occurredAt: '2026-09-24T00:00:06Z', payload: { text: 'partial' } },
    { seq: 7, type: 'artifact.available', occurredAt: '2026-09-24T00:00:07Z', payload: { file_name: 'report.md' } },
  ]);
  assert.deepEqual(citations.map((citation) => citation.seq), [5, 7], '只有 tool./artifact. 事件是可追溯来源；text 流不混入');
  assert.equal(citations[0]!.type, 'tool.started');
  assert.equal(citations[0]!.source, 'kb-7', 'source 取载荷中第一个已知的来源键');
  assert.ok(citations[0]!.detail.includes('knowledge_search'));
  assert.equal(citations[1]!.source, 'report.md', 'file_name 也是来源键');
});

test('citations never invent a source and always carry the raw detail', () => {
  const citations = projectCitations([
    { seq: 2, type: 'tool.completed', occurredAt: '2026-09-24T00:00:02Z', payload: { elapsed_ms: 42 } },
  ]);
  assert.equal(citations.length, 1);
  assert.equal('source' in citations[0]!, false, '没有已知来源键时不编造 source');
  assert.ok(citations[0]!.detail.includes('42'));
});

test('citation details are truncated before entering view state', () => {
  const citations = projectCitations([
    { seq: 1, occurredAt: '2026-09-24T00:00:01Z', type: 'tool.terminal', payload: { blob: 'x'.repeat(5000) } },
  ]);
  assert.ok((citations[0]!.detail ?? '').length <= 2000, 'B3-F35：大载荷截断后进入视图状态');
});
