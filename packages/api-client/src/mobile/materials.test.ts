import test from 'node:test';
import assert from 'node:assert/strict';
import type { ClientRequest } from '../client.ts';
import { createMobileMaterialRemote } from './materials.ts';

const listData = {
  items: [
    { index: 0, id: 'msg-1:0', name: 'report.md', mime: 'text/markdown', version: 'aaaaaaaaaaaaaaaa', digest: 'aaaaaaaaaaaabbbbbbbbbbccccccccccccdddddddddddd', size: 12, source_run: 'run-1', created_at: '2026-09-24T00:00:00Z' },
    { index: 1, id: 'msg-1:1', name: 'changes.diff', mime: 'text/x-diff', version: 'msg-1:1', size: 40, source_run: 'run-1' },
  ],
  terminal: { available: true },
};

test('material remote maps the artifact list wire and degrades without the terminal flag', async () => {
  const remote = createMobileMaterialRemote({ origin: 'https://weknora.example.test', request: async () => ({ success: true, data: listData }) });
  const list = await remote.list('run-1');
  assert.equal(list.runId, 'run-1');
  assert.equal(list.terminalAvailable, true);
  assert.deepEqual(list.artifacts.map((row) => row.version), ['aaaaaaaaaaaaaaaa', 'msg-1:1']);
  assert.equal(list.artifacts[0]!.digest, 'aaaaaaaaaaaabbbbbbbbbbccccccccccccdddddddddddd');
  assert.equal('digest' in list.artifacts[1]!, false, '无 digest 的行不下发该字段');

  // 旧服务端：terminal 标志缺失 → false（能力降级，不猜成 true）。
  const legacy = createMobileMaterialRemote({ origin: 'https://weknora.example.test', request: async () => ({ success: true, data: { items: [] } }) });
  assert.equal((await legacy.list('run-1')).terminalAvailable, false);
});

test('material remote posts signed-url with the numeric index and maps the grant', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileMaterialRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return {
        success: true,
        data: {
          url: 'https://weknora.example.test/api/v1/workbench/artifacts/download?index=1',
          expires_at: '2026-09-24T01:00:00Z',
          artifact: listData.items[1],
        },
      };
    },
  });
  const grant = await remote.signedUrl({ runId: 'run-1', index: 1 });
  assert.equal(requests[0]!.method, 'POST');
  assert.equal(requests[0]!.path, '/api/v1/workbench/executions/run-1/artifacts/1/signed-url');
  assert.equal(grant.expiresAt, '2026-09-24T01:00:00Z');
  assert.equal(grant.artifact.id, 'msg-1:1');
  assert.equal(grant.artifact.version, 'msg-1:1');
});

test('terminalLog reports undefined when the cursor did not advance (end of log)', async () => {
  const remote = createMobileMaterialRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({
      success: true,
      data: { lines: [{ seq: 5, occurred_at: '2026-09-24T00:00:01Z', stream: 'stdout', text: 'ls' }], next_cursor: 5 },
    }),
  });
  // after=5 末页：游标未推进 → undefined（mobile-core 以 undefined 判末页，B3-F52）。
  const page = await remote.terminalLog({ runId: 'run-1', after: 5, limit: 200 });
  assert.equal(page.nextCursor, undefined, 'B3-F52：末页（游标未推进）必须转译为 undefined');
  const page2 = await remote.terminalLog({ runId: 'run-1', after: 0, limit: 200 }); // after=0 → next=5 推进
  assert.equal(page2.nextCursor, 5);
});

test('material remote pages the terminal log with after/limit query facets', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileMaterialRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return { success: true, data: { lines: [{ seq: 3, occurred_at: '2026-09-24T00:00:03Z', stream: 'stdout', text: '$ ls\n' }], next_cursor: 3 } };
    },
  });
  const page = await remote.terminalLog({ runId: 'run-1', after: 0, limit: 200 });
  assert.equal(requests[0]!.method, 'GET');
  assert.equal(requests[0]!.path, '/api/v1/workbench/executions/run-1/terminal-log?after=0&limit=200');
  assert.equal(page.lines[0]!.stream, 'stdout');
  assert.equal(page.nextCursor, 3);
});

test('material remote projects evidence events from the run snapshot', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileMaterialRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return {
        success: true,
        data: {
          execution: { schema_version: 1, run_id: 'run-1', session_id: 'task-1', revision: 1, driver: 'platform', run_status: 'succeeded', execution_status: 'succeeded', settlement_status: 'settled', seq: 2, capabilities: {} },
          watermark: 5,
          incomplete: false,
          confirmed_watermark: 5,
          events: [
            { schema_version: 1, run_id: 'run-1', attempt_id: 'a1', seq: 5, type: 'tool.started', occurred_at: '2026-09-24T00:00:05Z', payload: { tool: 'shell' } },
          ],
        },
      };
    },
  });
  const events = await remote.events('run-1');
  assert.equal(requests[0]!.path, '/api/v1/workbench/executions/run-1/snapshot');
  assert.deepEqual(events, [{ seq: 5, type: 'tool.started', occurredAt: '2026-09-24T00:00:05Z', payload: { tool: 'shell' } }]);
});

test('material remote requires a credential-free absolute https origin', async () => {
  const request = async () => ({ success: true, data: { items: [] } });
  assert.throws(() => createMobileMaterialRemote({ origin: 'http://weknora.example.test', request }), /HTTPS/);
  assert.throws(() => createMobileMaterialRemote({ origin: 'https://user:pass@weknora.example.test', request }), /user info/);
  assert.throws(() => createMobileMaterialRemote({ origin: 'not-a-url', request }), /absolute URL/);
});
