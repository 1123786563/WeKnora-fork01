import test from 'node:test';
import assert from 'node:assert/strict';
import { ApiError } from '@weknora/api-client';
import { streamAuthorizedSse } from './sse-stream.ts';

const encoder = new TextEncoder();
const streamOf = (parts: string[]) => new ReadableStream<Uint8Array>({
  start(controller) { for (const part of parts) controller.enqueue(encoder.encode(part)); controller.close(); },
});

test('streams sse bytes as text chunks and sends the bearer token', async () => {
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  const chunks: string[] = [];
  await streamAuthorizedSse(
    'https://weknora.example.test',
    { method: 'GET', path: '/api/v1/workbench/executions/r1/events?version=2', headers: { 'Last-Event-ID': '5' } },
    'access-1',
    (chunk) => chunks.push(chunk),
    async (url, init) => { calls.push({ url, init: init as RequestInit }); return new Response(streamOf(['id: 6\neve', 'nt: run.started\ndata: {}\n\n'])); },
  );
  assert.equal(calls.length, 1);
  assert.equal(calls[0]!.url, 'https://weknora.example.test/api/v1/workbench/executions/r1/events?version=2');
  const headers = calls[0]!.init!.headers as Record<string, string>;
  assert.equal(headers.authorization, 'Bearer access-1');
  assert.equal(headers['Last-Event-ID'], '5');
  assert.equal(headers.accept, 'text/event-stream');
  assert.equal(chunks.join(''), 'id: 6\nevent: run.started\ndata: {}\n\n');
});

test('a non-200 stream rejects in the ApiError shape (401 refresh / 409 cursor mapping depend on it)', async () => {
  await assert.rejects(
    streamAuthorizedSse('https://weknora.example.test', { method: 'GET', path: '/p' }, 't', () => {}, async () => new Response('gone', { status: 409 })),
    (error: unknown) => (error as { name?: string; status?: number }).name === 'ApiError' && (error as { status?: number }).status === 409,
  );
});

test('a non-2xx response rejects with a real ApiError carrying code', async () => {
  const fetchLike = async () => new Response('nope', { status: 503 });
  await assert.rejects(
    streamAuthorizedSse('https://weknora.example.test', { method: 'GET', path: '/api/x' }, 'token', () => {}, fetchLike),
    (error: unknown) => error instanceof ApiError && error.status === 503 && error.code === 'HTTP_503',
  );
});

test('a throwing onChunk cancels the reader before the error propagates', async () => {
  const cancelled: boolean[] = [];
  const body = new ReadableStream<Uint8Array>({
    start(controller) { controller.enqueue(encoder.encode('data: x\n\n')); /* 保持打开：只有 cancel 能释放 */ },
    cancel() { cancelled.push(true); },
  });
  const fetchLike = async () => new Response(body, { status: 200, headers: { 'content-type': 'text/event-stream' } });
  await assert.rejects(
    streamAuthorizedSse('https://weknora.example.test', { method: 'GET', path: '/api/x' }, 'token', () => { throw new Error('RUNTIME_SCOPE_CHANGED'); }, fetchLike),
    /RUNTIME_SCOPE_CHANGED/,
  );
  assert.deepEqual(cancelled, [true], 'onChunk 同步抛出时 reader 必须 cancel，连接不得保持打开');
});
