import test from 'node:test';
import assert from 'node:assert/strict';
import { createScopedNewTaskDrafts } from './new-task-drafts.ts';

function memoryDrafts() {
  const rows = new Map<string, string>();
  return {
    async get(id: string) { const body = rows.get(id); return body === undefined ? undefined : { body }; },
    async put(input: { id: string; body: string }) { rows.set(input.id, input.body); },
    dump: (id: string) => rows.get(id),
  };
}

test('drafts round-trip through the scoped store under the fixed id', async () => {
  const backing = memoryDrafts();
  const drafts = createScopedNewTaskDrafts(backing);
  await drafts.save({ text: '整理周报', agentId: 'a-1', budgetUpper: 200, attachments: [], knowledgeIds: [] });
  const loaded = await drafts.load();
  assert.deepEqual(loaded, { text: '整理周报', agentId: 'a-1', budgetUpper: 200, attachments: [], knowledgeIds: [] });
  assert.equal(backing.dump('new-task'), JSON.stringify(loaded));
  await drafts.save({ text: '', agentId: null, budgetUpper: 0, attachments: [], knowledgeIds: [] });
  assert.equal((await drafts.load())?.text, '');
});

test('a corrupted or missing draft body reads as undefined, never crashes the New flow', async () => {
  const backing = memoryDrafts();
  await backing.put({ id: 'new-task', body: '{not json' });
  const drafts = createScopedNewTaskDrafts(backing);
  assert.equal(await drafts.load(), undefined);
  assert.equal(await createScopedNewTaskDrafts(memoryDrafts()).load(), undefined);
});

test('a corrupted draft degrades field-by-field instead of crashing the screen (B3-F42)', async () => {
  const backing = memoryDrafts();
  await backing.put({ id: 'new-task', body: JSON.stringify({ text: 42, attachments: 'not-an-array', knowledgeIds: null, budgetUpper: 'x' }) });
  const drafts = createScopedNewTaskDrafts(backing);
  const draft = await drafts.load();
  assert.deepEqual(draft, { text: '', agentId: null, budgetUpper: 0, attachments: [], knowledgeIds: [] }, '脏字段逐项降级，evaluateSubmitReadiness 不再抛 TypeError');
});
