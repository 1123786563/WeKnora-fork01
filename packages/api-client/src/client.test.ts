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

const binaryResponse = (status: number, body: Blob | string, contentType: string) => ({
  status,
  headers: new Headers({
    'content-type': contentType,
    'content-disposition': 'attachment; filename="guide.md"',
    'x-request-id': 'request-binary-1',
  }),
  json: async () => ({ error: 'unexpected json read' }),
  text: async () => typeof body === 'string' ? body : await body.text(),
  blob: async () => typeof body === 'string' ? new Blob([body], { type: contentType }) : body,
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

test('knowledgeBases.search posts to /api/v1/knowledge-search with the query and KB scope, parsing chunk hits', async () => {
  const requests: Request[] = [];
  const transport = createJsonTransport(async (input, init) => {
    requests.push(new Request(input, init));
    return jsonResponse(200, {
      success: true,
      data: [{
        id: 'chunk-1',
        content: 'full chunk text',
        matched_content: 'full <em>chunk</em> text',
        knowledge_id: 'doc-1',
        knowledge_base_id: 'kb-1',
        knowledge_title: 'Doc title',
        knowledge_filename: 'doc.pdf',
        chunk_index: 3,
        score: 0.87,
        match_type: 'vector',
      }],
    });
  });
  const client = createWeKnoraClient({ baseURL: 'https://api.example.test', transport });

  const result = await client.knowledgeBases.search({ query: 'hello', knowledgeBaseIds: ['kb-1'] });

  assert.equal(requests[0]?.url, 'https://api.example.test/api/v1/knowledge-search');
  assert.equal(requests[0]?.method, 'POST');
  assert.deepEqual(await requests[0]?.json(), { query: 'hello', knowledge_base_ids: ['kb-1'] });
  assert.deepEqual(result, [{
    id: 'chunk-1',
    content: 'full chunk text',
    matchedContent: 'full <em>chunk</em> text',
    knowledgeId: 'doc-1',
    knowledgeBaseId: 'kb-1',
    knowledgeTitle: 'Doc title',
    knowledgeFilename: 'doc.pdf',
    chunkIndex: 3,
    score: 0.87,
    matchType: 'vector',
  }]);
});

test('knowledgeBases.search refuses to call the backend with no KB/knowledge scope', async () => {
  const client = createWeKnoraClient({
    baseURL: 'https://api.example.test',
    transport: { send: async () => { throw new Error('must not be called without a scope'); } },
  });

  await assert.rejects(
    client.knowledgeBases.search({ query: 'hello', knowledgeBaseIds: [] }),
    /at least one knowledgeBaseId or knowledgeId/,
  );
});

test('requestBinary preserves protected bytes and response content headers', async () => {
  const requests: Request[] = [];
  const transport = createJsonTransport(async (input, init) => {
    requests.push(new Request(input, init));
    return binaryResponse(200, '# private guide\n', 'text/markdown; charset=utf-8');
  });
  const client = createWeKnoraClient({ baseURL: 'https://api.example.test', transport });

  const result = await client.requestBinary({ method: 'GET', path: '/api/v1/knowledge/doc-1/preview' });

  assert.equal(result.body, '# private guide\n');
  assert.equal(result.contentType, 'text/markdown; charset=utf-8');
  assert.equal(result.headers['content-disposition'], 'attachment; filename="guide.md"');
  assert.equal(requests[0]?.headers.get('accept'), '*/*');
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

test('preserves numeric backend error codes for structured lifecycle errors', async () => {
  const client = createWeKnoraClient({
    baseURL: 'https://api.example.test',
    transport: createJsonTransport(async () => jsonResponse(400, {
      success: false,
      error: { code: 2300, message: 'model is in use', details: { marker: true } },
    })),
  });

  await assert.rejects(
    client.request({ method: 'DELETE', path: '/api/v1/models/model-1' }),
    (error: unknown) => error instanceof ApiError
      && error.code === '2300'
      && (error.details as { marker?: boolean })?.marker === true,
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

test('routes native file uploads through the cancellable transport seam', async () => {
  const calls: unknown[] = [];
  const transport = {
    send: async () => { throw new Error('generic fetch must not handle native uploads'); },
    sendMultipartFile: async (request: unknown) => {
      calls.push(request);
      return { status: 200, headers: {}, body: { success: true, data: { id: 'doc-native' } } };
    },
  };
  const client = createWeKnoraClient({ baseURL: 'https://api.example.test', transport });

  const result = await client.knowledge.documents.upload('kb-1', {
    file: { uri: 'content://picker/notes.txt', name: 'notes.txt', type: 'text/plain' },
    metadata: { source: 'mobile' },
  });

  assert.equal(result.id, 'doc-native');
  const call = calls[0] as Record<string, unknown>;
  assert.ok(call.signal instanceof AbortSignal);
  assert.deepEqual({ ...call, signal: undefined }, {
    method: 'POST',
    url: 'https://api.example.test/api/v1/knowledge-bases/kb-1/knowledge/file',
    headers: { accept: 'application/json', 'accept-language': 'zh-CN' },
    file: { uri: 'content://picker/notes.txt', name: 'notes.txt', type: 'text/plain' },
    fields: { metadata: JSON.stringify({ source: 'mobile' }) },
    signal: undefined,
  });
});

test('preserves multipart fields when a multipart request has no native file', async () => {
  let received: unknown;
  const transport = createJsonTransport(async (_input, init) => {
    received = init?.body;
    return jsonResponse(200, { success: true, data: { ok: true } });
  });
  const client = createWeKnoraClient({ baseURL: 'https://api.example.test', transport });

  await client.request({ method: 'POST', path: '/api/v1/models/model-1/debug', multipartFields: { input: 'hello', options: '{}' } });

  assert.ok(received instanceof FormData);
  assert.deepEqual(Object.fromEntries((received as FormData).entries()), { input: 'hello', options: '{}' });
});

test('classifies abort-like errors when DOMException is unavailable', async () => {
  const globals = globalThis as typeof globalThis & { DOMException?: typeof DOMException };
  const originalDOMException = globals.DOMException;
  try {
    delete globals.DOMException;
    const aborted = new Error('network cancelled');
    aborted.name = 'AbortError';
    const client = createWeKnoraClient({
      baseURL: 'https://api.example.test',
      transport: { send: async () => { throw aborted; } },
    });

    await assert.rejects(
      client.request({ method: 'GET', path: '/api/v1/knowledge-bases' }),
      (error: unknown) => error instanceof ApiError && error.code === 'CANCELLED',
    );
  } finally {
    if (originalDOMException) globals.DOMException = originalDOMException;
    else delete globals.DOMException;
  }
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

test('exposes approval and steer actions under the chat namespace', async () => {
  const client = createWeKnoraClient({
    baseURL: 'https://api.example.test',
    transport: {
      send: async ({ url }) => ({
        status: 200,
        headers: {},
        body: url.endsWith('/steer')
          ? { success: true, items: [] }
          : { success: true },
      }),
    },
  });

  assert.deepEqual(await client.chat.approvals.cancelOAuth('pending-1'), { success: true });
  assert.deepEqual(await client.chat.steer.list('session-1'), { success: true, items: [] });
});

test('exposes the complete settings API under the client', async () => {
  const client = createWeKnoraClient({
    baseURL: 'https://api.example.test',
    transport: {
      send: async () => ({ status: 200, headers: {}, body: { code: 0, data: { version: 'test' } } }),
    },
  });

  assert.equal((await client.settings.system.info()).version, 'test');
});

test('exposes the embed API with an isolated credential profile', async () => {
  const client = createWeKnoraClient({
    baseURL: 'https://api.example.test',
    transport: {
      send: async ({ headers }) => ({
        status: 200,
        headers: {},
        body: headers.Authorization === 'Embed ems-1'
          ? { success: true, data: { channel_id: 'c-1', agent_id: 'a-1' } }
          : { success: false },
      }),
    },
  });

  assert.equal((await client.embed.public.config('c-1', 'ems-1')).agent_id, 'a-1');
});

test('every request carries Accept-Language from the stored locale, defaulting to zh-CN', async () => {
  // Upstream frontend/src/utils/request.ts injects Accept-Language:
  // getCurrentLanguage() on every API call (i18n locale || localStorage
  // 'locale' || zh-CN), so server-side localized payloads (builtin agent
  // names, error copy) follow the UI language instead of the browser's
  // default header. The api client must do the same.
  const originalWindow = (globalThis as { window?: unknown }).window;
  const captured: Array<Record<string, string>> = [];
  const captureTransport = {
    send: async ({ headers }: { headers: Record<string, string> }) => {
      captured.push(headers);
      return { status: 200, headers: {}, body: { success: true, data: [] } };
    },
  };

  try {
    // No stored preference: deployment default zh-CN (never the browser
    // language — mirrors the app locale convention).
    (globalThis as { window?: unknown }).window = { localStorage: { getItem: () => null } };
    const client = createWeKnoraClient({ baseURL: 'https://api.example.test', transport: captureTransport });
    await client.configuration.agents.list();
    assert.equal(captured.at(-1)?.['accept-language'], 'zh-CN');

    // Stored language switch wins.
    (globalThis as { window?: unknown }).window = { localStorage: { getItem: (key: string) => (key === 'locale' ? 'ja-JP' : null) } };
    await client.configuration.agents.list();
    assert.equal(captured.at(-1)?.['accept-language'], 'ja-JP');
  } finally {
    (globalThis as { window?: unknown }).window = originalWindow;
  }
});
