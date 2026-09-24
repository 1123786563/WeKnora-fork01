import test from 'node:test';
import assert from 'node:assert/strict';
import type { ClientRequest } from '../client.ts';
import { createMobileLegacyTaskRemote } from './legacy-tasks.ts';

test('legacy remote maps the list wire into module rows and requires the legacy kind marker', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return {
        success: true,
        data: {
          items: [
            { task_id: 'lg-1', title: '旧聊天：周报素材', attention: 'none', updated_at: '2026-09-20T08:00:00Z', kind: 'legacy' },
            { task_id: 'lg-2', attention: 'none', archived_at: '2026-09-22T12:00:00Z', updated_at: '2026-09-20T09:00:00Z', kind: 'legacy' },
          ],
          next_cursor: 'cursor-2',
        },
      };
    },
  });
  const page = await remote.list({ search: '周报', archived: true, limit: 15 });
  assert.equal(requests[0]!.method, 'GET');
  assert.equal(requests[0]!.path, '/api/v1/workbench/legacy-tasks?q=%E5%91%A8%E6%8A%A5&archived=true&limit=15');
  assert.deepEqual(page.items, [
    { taskId: 'lg-1', title: '旧聊天：周报素材', attention: 'none', updatedAt: '2026-09-20T08:00:00Z' },
    { taskId: 'lg-2', title: '', attention: 'none', archivedAt: '2026-09-22T12:00:00Z', updatedAt: '2026-09-20T09:00:00Z' },
  ]);
  assert.equal(page.nextCursor, 'cursor-2');
});

test('legacy remote rejects rows without the legacy kind marker or with fabricated attention', async () => {
  const remote = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: true, data: { items: [{ task_id: 'r-1', updated_at: '2026-09-20T08:00:00Z', run_status: 'running' }] } }),
  });
  await assert.rejects(remote.list({}), /kind must be "legacy"/);

  const fabricated = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: true, data: { items: [{ task_id: 'lg-1', updated_at: '2026-09-20T08:00:00Z', kind: 'legacy', attention: 'required' }] } }),
  });
  await assert.rejects(fabricated.list({}), /attention must be "none"/);
});

test('legacy remote maps message history from the messages-load wire', async () => {
  const requests: ClientRequest[] = [];
  const remote = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async (input) => {
      requests.push(input);
      return {
        success: true,
        data: [
          { id: 'm1', session_id: 'lg-1', role: 'user', content: '第一问' },
          { id: 'm2', session_id: 'lg-1', role: 'assistant', content: '第一答', created_at: '2026-09-20T08:00:01Z' },
        ],
      };
    },
  });
  const messages = await remote.history('lg-1');
  assert.equal(requests[0]!.path, '/api/v1/messages/lg-1/load?limit=20');
  assert.deepEqual(messages, [
    { messageId: 'm1', role: 'user', content: '第一问' },
    { messageId: 'm2', role: 'assistant', content: '第一答', createdAt: '2026-09-20T08:00:01Z' },
  ]);
});

test('legacy followUp posts the ordinary chat body through the stream channel and resolves on completion', async () => {
  const streams: ClientRequest[] = [];
  const remote = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async () => { throw new Error('follow-up must not use the JSON channel'); },
    stream: async (input, onChunk) => {
      streams.push(input);
      onChunk('data: {"response_type":"answer","content":"答"}\n\n');
      onChunk('data: {"response_type":"complete"}\n\n');
    },
  });
  await remote.followUp({ taskId: 'lg-1', question: '继续这个话题' });
  assert.equal(streams.length, 1);
  assert.equal(streams[0]!.method, 'POST');
  assert.equal(streams[0]!.path, '/api/v1/knowledge-chat/lg-1');
  assert.equal((streams[0]!.headers ?? {}).accept, 'text/event-stream');
  assert.deepEqual((streams[0]!.body as Record<string, unknown>).query, '继续这个话题');
});

test('legacy followUp fails closed without a stream transport and rejects error frames', async () => {
  const closed = createMobileLegacyTaskRemote({ origin: 'https://weknora.example.test', request: async () => undefined });
  await assert.rejects(closed.followUp({ taskId: 'lg-1', question: 'x' }), /authorized stream transport/);

  const failing = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async () => undefined,
    stream: async (_input, onChunk) => { onChunk('data: {"response_type":"error","content":"boom"}\n\n'); },
  });
  await assert.rejects(failing.followUp({ taskId: 'lg-1', question: 'x' }), /LEGACY_FOLLOW_UP_FAILED/);

  const malformed = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async () => undefined,
    stream: async (_input, onChunk) => { onChunk('data: not-json\n\n'); },
  });
  await assert.rejects(malformed.followUp({ taskId: 'lg-1', question: 'x' }), /LEGACY_FOLLOW_UP_MALFORMED_FRAME/);
});

test('legacy remote validates the deployment origin like the task office remote', async () => {
  assert.throws(() => createMobileLegacyTaskRemote({ origin: 'https://weknora.example.test/path', request: async () => undefined }), /must not include a path/);
  assert.throws(() => createMobileLegacyTaskRemote({ origin: 'http://weknora.example.test', request: async () => undefined }), /HTTPS/);
});

test('legacy followUp fails closed with LEGACY_FOLLOW_UP_TRUNCATED when the stream ends without a terminal frame', async () => {
  const truncated = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async () => undefined,
    stream: async (_input, onChunk) => {
      onChunk('data: {"response_type":"answer","content":"部分回答"}\n\n');
      onChunk('data: {"response_type":"references"}\n\n');
    },
  });
  await assert.rejects(truncated.followUp({ taskId: 'lg-9', question: 'x' }), /LEGACY_FOLLOW_UP_TRUNCATED: .*task lg-9.*2 frame\(s\)/);
});

test('legacy followUp succeeds when a stop terminal frame arrives', async () => {
  const streams: ClientRequest[] = [];
  const remote = createMobileLegacyTaskRemote({
    origin: 'https://weknora.example.test',
    request: async () => undefined,
    stream: async (input, onChunk) => {
      streams.push(input);
      onChunk('data: {"response_type":"answer","content":"答"}\n\n');
      onChunk('data: {"response_type":"stop"}\n\n');
    },
  });
  await remote.followUp({ taskId: 'lg-1', question: 'x' });
  assert.equal(streams.length, 1);
});
