import test from 'node:test';
import assert from 'node:assert/strict';
import { createRecordingSharePort, createScenarioMaterialRemote, createScriptedBlobFetch } from './in-memory-material-remote.ts';

const artifact = { id: 'msg-1:0', index: 0, name: 'report.md', mime: 'text/markdown', version: 'aaaaaaaaaaaaaaaa', size: 12, sourceRun: 'run-1' };

test('the scenario remote records calls and serves scripted answers', async () => {
  const remote = createScenarioMaterialRemote({
    list: () => ({ runId: 'run-1', artifacts: [artifact], terminalAvailable: true }),
    signedUrl: ({ index }) => ({ url: `https://weknora.example.test/api/v1/workbench/artifacts/download?index=${index}`, expiresAt: '2026-09-24T01:00:00Z', artifact }),
    terminalLog: ({ after }) => ({ lines: [{ seq: after + 1, occurredAt: '2026-09-24T00:00:01Z', stream: 'stdout', text: '$ ls\n' }], nextCursor: after + 1 }),
    events: () => [{ seq: 5, type: 'tool.started', occurredAt: '2026-09-24T00:00:05Z', payload: { tool: 'shell' } }],
  });
  const list = await remote.list('run-1');
  assert.equal(list.terminalAvailable, true);
  const grant = await remote.signedUrl({ runId: 'run-1', index: 0 });
  assert.match(grant.url, /index=0$/);
  const page = await remote.terminalLog({ runId: 'run-1', after: 0, limit: 200 });
  assert.equal(page.lines.length, 1);
  assert.equal((await remote.events('run-1'))[0]!.type, 'tool.started');
  assert.deepEqual(remote.listCalls, ['run-1']);
  assert.deepEqual(remote.grantCalls, [{ runId: 'run-1', index: 0 }]);
  assert.deepEqual(remote.terminalCalls, [{ runId: 'run-1', after: 0, limit: 200 }]);
  assert.deepEqual(remote.eventCalls, ['run-1']);
});

test('unscripted calls fail loudly instead of answering empty', async () => {
  const remote = createScenarioMaterialRemote({});
  await assert.rejects(() => remote.list('run-1'), /not scripted/);
});

test('the scripted blob fetch and recording share port observe usage', async () => {
  const blob = createScriptedBlobFetch({ 'https://weknora.example.test/blob': { bytes: new Uint8Array([104, 105]), mime: 'text/plain' } });
  const fetched = await blob.fetch('https://weknora.example.test/blob');
  assert.equal(new TextDecoder().decode(fetched.bytes), 'hi');
  await assert.rejects(() => blob.fetch('https://weknora.example.test/missing'), /not scripted/);
  assert.deepEqual(blob.calls, ['https://weknora.example.test/blob', 'https://weknora.example.test/missing']);

  const share = createRecordingSharePort();
  await share.share({ url: 'https://weknora.example.test/blob', name: 'report.md' });
  assert.deepEqual(share.shared, [{ url: 'https://weknora.example.test/blob', name: 'report.md' }]);
});
