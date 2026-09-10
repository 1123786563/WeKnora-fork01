import assert from 'node:assert/strict';
import test from 'node:test';

import { createWeKnoraClient } from './client.ts';
import { ApiError } from './errors.ts';
import { createJsonTransport } from './transport/json.ts';

const jsonResponse = (status: number, body: unknown, contentType = 'application/json') => ({
  status,
  headers: new Headers({ 'content-type': contentType }),
  json: async () => body,
  text: async () => typeof body === 'string' ? body : JSON.stringify(body),
});

test('knowledgeBases.list uses a base URL subpath and parses its DTO', async () => {
  const requests: Request[] = [];
  const transport = createJsonTransport(async (input, init) => {
    requests.push(new Request(input, init));
    return jsonResponse(200, { success: true, data: [{ id: 'kb-1', name: 'Docs', tenant_id: 't-1' }] });
  });
  const client = createWeKnoraClient({ baseURL: 'https://api.example.test/weknora/', transport });

  const result = await client.knowledgeBases.list({ creator: 'mine' });

  assert.equal(result[0]?.id, 'kb-1');
  assert.equal(requests[0]?.url, 'https://api.example.test/weknora/api/v1/knowledge-bases?creator=mine');
});

test('returns an empty body for 204', async () => {
  const transport = createJsonTransport(async () => jsonResponse(204, '', 'text/plain'));
  const client = createWeKnoraClient({ baseURL: 'https://api.example.test', transport });
  assert.equal(await client.request({ method: 'DELETE', path: '/api/v1/knowledge-bases/kb-1' }), undefined);
});

test('normalizes JSON, non-JSON and 413 errors', async () => {
  const cases = [
    [500, { code: 'server_error', message: 'broken' }, 'application/json', 'server_error'],
    [500, 'upstream broke', 'text/plain', 'HTTP_500'],
    [413, { message: 'too large' }, 'application/json', 'PAYLOAD_TOO_LARGE'],
  ] as const;

  for (const [status, body, contentType, code] of cases) {
    const client = createWeKnoraClient({
      baseURL: 'https://api.example.test',
      transport: createJsonTransport(async () => jsonResponse(status, body, contentType)),
    });
    await assert.rejects(
      client.request({ method: 'GET', path: '/api/v1/knowledge-bases' }),
      (error: unknown) => error instanceof ApiError && error.status === status && error.code === code,
    );
  }
});

test('normalizes the backend success-false error envelope', async () => {
  const client = createWeKnoraClient({
    baseURL: 'https://api.example.test',
    transport: createJsonTransport(async () => jsonResponse(403, {
      success: false,
      error: { code: 'TENANT_FORBIDDEN', message: 'No access', details: { tenant: 'b' } },
    })),
  });

  await assert.rejects(
    client.request({ method: 'GET', path: '/api/v1/knowledge-bases' }),
    (error: unknown) => error instanceof ApiError
      && error.status === 403
      && error.code === 'TENANT_FORBIDDEN'
      && error.message === 'No access'
      && (error.details as { tenant?: string })?.tenant === 'b',
  );
});

test('preserves cancellation and converts timeout to a typed error', async () => {
  const aborted = new DOMException('Aborted', 'AbortError');
  const client = createWeKnoraClient({
    baseURL: 'https://api.example.test',
    transport: { send: async () => { throw aborted; } },
  });
  await assert.rejects(client.request({ method: 'GET', path: '/api/v1/knowledge-bases' }), (error: unknown) =>
    error instanceof ApiError && error.code === 'CANCELLED',
  );

  const timeoutClient = createWeKnoraClient({
    baseURL: 'https://api.example.test',
    timeoutMs: 1,
    transport: { send: async ({ signal }) => await new Promise((_resolve, reject) => {
      signal?.addEventListener('abort', () => reject(new DOMException('Timed out', 'TimeoutError')), { once: true });
    }) },
  });
  await assert.rejects(timeoutClient.request({ method: 'GET', path: '/api/v1/knowledge-bases' }), (error: unknown) =>
    error instanceof ApiError && error.code === 'TIMEOUT',
  );
});

test('honors an already-aborted caller signal before transport starts', async () => {
  const controller = new AbortController();
  controller.abort();
  let sawAbortedSignal = false;
  const client = createWeKnoraClient({
    baseURL: 'https://api.example.test',
    transport: { send: async ({ signal }) => {
      sawAbortedSignal = signal?.aborted === true;
      return { status: 200, headers: {}, body: { success: true, data: [] } };
    } },
  });

  await assert.rejects(
    client.request({ method: 'GET', path: '/api/v1/knowledge-bases', signal: controller.signal }),
    (error: unknown) => error instanceof ApiError && error.code === 'CANCELLED',
  );
  assert.equal(sawAbortedSignal, true);
});
