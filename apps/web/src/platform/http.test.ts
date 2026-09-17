import assert from 'node:assert/strict';
import test from 'node:test';
import { ApiError, type FetchLike } from '@weknora/api-client';
import { createBrowserTransport, observeUploadProgress, uploadProgressListener } from './http.ts';

test('injects scoped auth headers without changing the shared transport', async () => {
  let seen: { url: string; headers: Record<string, string> } | undefined;
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'access-1' },
    tenantId: 'tenant-7',
    locale: 'zh-CN',
    requestId: () => 'request-1',
    fetcher: (async (url, init) => {
      seen = { url, headers: init?.headers ?? {} };
      return { status: 200, headers: new Headers({ 'content-type': 'application/json' }), json: async () => ({ success: true, data: [] }), text: async () => '' };
    }) satisfies FetchLike,
  });

  await transport.send({ method: 'GET', url: 'https://api.test/api/v1/knowledge-bases', headers: { accept: 'application/json' } });
  assert.equal(seen?.headers.authorization, 'Bearer access-1');
  assert.equal(seen?.headers['x-tenant-id'], 'tenant-7');
  assert.equal(seen?.headers['accept-language'], 'zh-CN');
  assert.equal(seen?.headers['x-request-id'], 'request-1');
});

test('reads the current tenant for every request after a scope switch', async () => {
  let tenantId: string | null = 'tenant-a';
  const seen: Array<string | undefined> = [];
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'access-1' },
    tenantId: () => tenantId,
    fetcher: (async (_url, init) => {
      seen.push(init?.headers?.['x-tenant-id']);
      return { status: 204, headers: new Headers(), json: async () => undefined, text: async () => '' };
    }) satisfies FetchLike,
  });

  await transport.send({ method: 'GET', url: 'https://api.test/api/v1/knowledge-bases', headers: {} });
  tenantId = 'tenant-b';
  await transport.send({ method: 'GET', url: 'https://api.test/api/v1/knowledge-bases', headers: {} });

  assert.deepEqual(seen, ['tenant-a', 'tenant-b']);
});

test('isolates embed headers from bearer tenant context', async () => {
  const seen: Record<string, string>[] = [];
  for (const credential of [{ kind: 'embed', token: 'visitor-token', sessionSig: 'sig-1', visitorId: 'visitor-1' } as const, { kind: 'anonymous' } as const]) {
    const transport = createBrowserTransport({ credential, tenantId: 'must-not-leak', fetcher: (async (_url, init) => {
      seen.push(init?.headers ?? {});
      return { status: 204, headers: new Headers(), json: async () => undefined, text: async () => '' };
    }) satisfies FetchLike });
    await transport.send({ method: 'GET', url: 'https://api.test/files/x', headers: {} });
  }
  assert.equal(seen[0]?.authorization, 'Embed visitor-token');
  assert.equal(seen[0]?.['x-tenant-id'], undefined);
  assert.equal(seen[0]?.['x-embed-session'], 'sig-1');
  assert.equal(seen[0]?.['x-embed-visitor'], 'visitor-1');
  assert.equal(seen[1]?.authorization, undefined);
});

// Vue request.ts:137-139 rejects response-less failures with the localized
// error.networkError copy; the raw "Failed to fetch" previously leaked to pages.
test('surfaces network failures as the localized network-error ApiError', async () => {
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'access-1' },
    fetcher: (async () => { throw new TypeError('Failed to fetch'); }) satisfies FetchLike,
  });

  await assert.rejects(
    transport.send({ method: 'GET', url: 'https://api.test/api/v1/organizations', headers: {} }),
    (error: unknown) => {
      assert.ok(error instanceof ApiError);
      assert.equal((error as ApiError).code, 'NETWORK_ERROR');
      // No localStorage in the test runtime -> zh-CN default, byte-exact Vue copy.
      assert.equal((error as ApiError).message, '网络错误，请检查您的网络连接');
      return true;
    },
  );
});

test('network-error copy follows the stored locale, with Vue fallback-locale values', async () => {
  const stored: Record<string, string> = { locale: 'en-US' };
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: { getItem: (key: string) => stored[key] ?? null },
  });
  try {
    const transport = createBrowserTransport({
      credential: { kind: 'bearer', accessToken: 'access-1' },
      fetcher: (async () => { throw new TypeError('Failed to fetch'); }) satisfies FetchLike,
    });
    // Vue lacks error.networkError in en-US and falls back to zh-CN; the port
    // reproduces the rendered result instead of inventing an English copy.
    await assert.rejects(
      transport.send({ method: 'GET', url: 'https://api.test/api/v1/organizations', headers: {} }),
      (error: unknown) => error instanceof ApiError && error.message === '网络错误，请检查您的网络连接',
    );
  } finally {
    delete (globalThis as { localStorage?: unknown }).localStorage;
  }
});

test('non-network transport errors are not rewritten', async () => {
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'access-1' },
    fetcher: (async () => { throw new Error('boom'); }) satisfies FetchLike,
  });
  await assert.rejects(
    transport.send({ method: 'GET', url: 'https://api.test/api/v1/organizations', headers: {} }),
    (error: unknown) => !(error instanceof ApiError) && (error as Error).message === 'boom',
  );
});

test('forwards streaming requests with the same scoped headers', async () => {
  let seen: Record<string, string> | undefined;
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'stream-access' }, tenantId: 'tenant-stream',
    fetcher: (async (_url, init) => {
      seen = init?.headers;
      return { status: 200, headers: new Headers(), json: async () => ({}), text: async () => 'data: {}\n\n' };
    }) satisfies FetchLike,
  });
  const result = await transport.sendStream!({ method: 'POST', url: 'https://api.test/chat', headers: { accept: 'text/event-stream' }, body: { query: 'x' } });
  const chunks: string[] = [];
  for await (const chunk of result.chunks) chunks.push(chunk);
  assert.equal(seen?.authorization, 'Bearer stream-access');
  assert.equal(seen?.['x-tenant-id'], 'tenant-stream');
  assert.deepEqual(chunks, ['data: {}\n\n']);
});

test('forwards authenticated binary requests and retries them after bearer refresh', async () => {
  let accessToken = 'expired-binary-access';
  let refreshCalls = 0;
  const requests: string[] = [];
  const transport = createBrowserTransport({
    credential: () => ({ kind: 'bearer', accessToken, refreshToken: 'refresh-binary' }),
    tenantId: 'tenant-binary',
    fetcher: (async (_url, init) => {
      const authorization = init?.headers?.authorization ?? '';
      requests.push(authorization);
      if (authorization === 'Bearer expired-binary-access') {
        return {
          status: 401,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: async () => ({ success: false }),
          text: async () => '',
        };
      }
      return {
        status: 200,
        headers: new Headers({ 'content-type': 'application/pdf', 'content-disposition': 'inline' }),
        json: async () => ({ success: false }),
        text: async () => 'unexpected text read',
        blob: async () => new Blob(['private pdf'], { type: 'application/pdf' }),
      };
    }) satisfies FetchLike,
    refresh: async () => {
      refreshCalls += 1;
      accessToken = 'fresh-binary-access';
    },
  });

  const result = await transport.sendBinary!({
    method: 'GET',
    url: 'https://api.test/api/v1/knowledge/doc-1/preview',
    headers: {},
  });

  assert.equal(result.status, 200);
  assert.equal(result.headers['content-disposition'], 'inline');
  assert.equal(await (result.body as Blob).text(), 'private pdf');
  assert.equal(refreshCalls, 1);
  assert.deepEqual(requests, ['Bearer expired-binary-access', 'Bearer fresh-binary-access']);
});

test('refreshes once and retries concurrent bearer requests after 401', async () => {
  let accessToken = 'expired-access';
  let refreshCalls = 0;
  let releaseRefresh!: () => void;
  const refreshGate = new Promise<void>((resolve) => { releaseRefresh = resolve; });
  const requests: string[] = [];
  const transport = createBrowserTransport({
    credential: () => ({ kind: 'bearer', accessToken, refreshToken: 'refresh-1' }),
    fetcher: (async (_url, init) => {
      const authorization = init?.headers?.authorization ?? '';
      requests.push(authorization);
      if (authorization === 'Bearer expired-access') {
        return { status: 401, headers: new Headers({ 'content-type': 'application/json' }), json: async () => ({ success: false }), text: async () => '' };
      }
      return { status: 200, headers: new Headers({ 'content-type': 'application/json' }), json: async () => ({ success: true, data: [] }), text: async () => '' };
    }) satisfies FetchLike,
    shouldRefresh: (request) => request.url.endsWith('/knowledge-bases'),
    refresh: async () => {
      refreshCalls += 1;
      await refreshGate;
      accessToken = 'fresh-access';
    },
  });

  const first = transport.send({ method: 'GET', url: 'https://api.test/api/v1/knowledge-bases', headers: {} });
  const second = transport.send({ method: 'GET', url: 'https://api.test/api/v1/knowledge-bases', headers: {} });
  await new Promise<void>((resolve) => setTimeout(resolve, 0));
  assert.equal(refreshCalls, 1, 'concurrent 401 responses share one refresh operation');
  releaseRefresh();

  assert.deepEqual((await Promise.all([first, second])).map((result) => result.status), [200, 200]);
  assert.deepEqual(requests, ['Bearer expired-access', 'Bearer expired-access', 'Bearer fresh-access', 'Bearer fresh-access']);
});

test('does not replay a JSON write after 401', async () => {
  let refreshCalls = 0;
  let requests = 0;
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'expired-write', refreshToken: 'refresh-write' },
    fetcher: (async () => {
      requests += 1;
      return { status: 401, headers: new Headers(), json: async () => ({ success: false }), text: async () => '' };
    }) satisfies FetchLike,
    refresh: async () => { refreshCalls += 1; },
  });

  const result = await transport.send({ method: 'POST', url: 'https://api.test/api/v1/knowledge-bases', headers: {}, body: { name: 'must-not-duplicate' } });

  assert.equal(result.status, 401);
  assert.equal(requests, 1);
  assert.equal(refreshCalls, 0);
});

test('does not replay a streaming POST after 401', async () => {
  let refreshCalls = 0;
  let requests = 0;
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'expired-stream', refreshToken: 'refresh-stream' },
    fetcher: (async () => {
      requests += 1;
      return { status: 401, headers: new Headers(), json: async () => ({ success: false }), text: async () => '', body: null };
    }) satisfies FetchLike,
    refresh: async () => { refreshCalls += 1; },
  });

  const result = await transport.sendStream!({ method: 'POST', url: 'https://api.test/api/v1/knowledge-chat/session-1', headers: {}, body: { query: 'must-not-duplicate' } });

  assert.equal(result.status, 401);
  assert.equal(requests, 1);
  assert.equal(refreshCalls, 0);
});

test('does not refresh 403 responses or retry an unauthorized response twice', async () => {
  let refreshCalls = 0;
  let requests = 0;
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'access-1', refreshToken: 'refresh-1' },
    fetcher: (async () => {
      requests += 1;
      return { status: requests === 1 ? 403 : 401, headers: new Headers(), json: async () => ({}), text: async () => '' };
    }) satisfies FetchLike,
    shouldRefresh: () => true,
    refresh: async () => { refreshCalls += 1; },
  });

  assert.equal((await transport.send({ method: 'GET', url: 'https://api.test/forbidden', headers: {} })).status, 403);
  assert.equal((await transport.send({ method: 'GET', url: 'https://api.test/unauthorized', headers: {} })).status, 401);
  assert.equal(refreshCalls, 1);
  assert.equal(requests, 3);
});

test('keeps refresh endpoint failures from recursively refreshing', async () => {
  let refreshCalls = 0;
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'access-1', refreshToken: 'refresh-1' },
    fetcher: (async () => ({ status: 401, headers: new Headers(), json: async () => ({}), text: async () => '' })) satisfies FetchLike,
    shouldRefresh: (request) => !request.url.endsWith('/auth/refresh'),
    refresh: async () => { refreshCalls += 1; },
  });

  assert.equal((await transport.send({ method: 'POST', url: 'https://api.test/api/v1/auth/refresh', headers: {} })).status, 401);
  assert.equal(refreshCalls, 0);
});

test('does not refresh Embed stream responses', async () => {
  let refreshCalls = 0;
  const transport = createBrowserTransport({
    credential: { kind: 'embed', token: 'embed-token' },
    fetcher: (async () => ({ status: 401, headers: new Headers(), json: async () => ({}), text: async () => '', body: null })) satisfies FetchLike,
    shouldRefresh: () => true,
    refresh: async () => { refreshCalls += 1; },
  });

  const result = await transport.sendStream!({ method: 'GET', url: 'https://api.test/embed/chat', headers: {} });
  assert.equal(result.status, 401);
  assert.equal(refreshCalls, 0);
});

// --- Multipart upload progress (browser XHR path) ---------------------------------

type XhrProgressEvent = { loaded: number; total: number; lengthComputable: boolean };

class StubXHR {
  static instances: StubXHR[] = [];
  status = 0;
  responseText = '';
  method = '';
  url = '';
  body: FormData | string | null | undefined;
  headers: Record<string, string> = {};
  upload = { onprogress: null as ((event: XhrProgressEvent) => void) | null };
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onabort: (() => void) | null = null;

  constructor() {
    StubXHR.instances.push(this);
  }

  open(method: string, url: string): void {
    this.method = method;
    this.url = url;
  }

  setRequestHeader(name: string, value: string): void {
    this.headers[name] = value;
  }

  send(body?: FormData | string | null): void {
    this.body = body;
  }

  abort(): void {
    this.onabort?.();
  }

  getAllResponseHeaders(): string {
    return 'content-type: application/json\r\nx-request-id: upload-req-1\r\n';
  }

  respond(status: number, body: unknown): void {
    this.status = status;
    this.responseText = JSON.stringify(body);
    this.onload?.();
  }
}

function withXhrStub(run: () => Promise<void>): Promise<void> {
  const globalScope = globalThis as unknown as { XMLHttpRequest?: unknown };
  globalScope.XMLHttpRequest = StubXHR;
  StubXHR.instances = [];
  return run().finally(() => {
    delete globalScope.XMLHttpRequest;
  });
}

test('routes observed uploads through XHR with scoped auth and byte progress', async () => {
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'upload-access' },
    tenantId: 'tenant-upload',
    locale: 'zh-CN',
    requestId: () => 'upload-request-id',
    fetcher: (async (url) => {
      assert.equal(String(url), 'blob:doc-1', 'only the blob source is fetched');
      return {
        status: 200,
        headers: new Headers(),
        json: async () => ({}),
        text: async () => '',
        blob: async () => new Blob(['pdf-bytes'], { type: 'application/pdf' }),
      };
    }) satisfies FetchLike,
  });

  await withXhrStub(async () => {
    const progress: number[] = [];
    const stop = observeUploadProgress('blob:doc-1', (event) => {
      progress.push(event.total ? Math.round((event.loaded * 100) / event.total) : 0);
    });
    const pending = transport.sendMultipartFile!({
      method: 'POST',
      url: 'https://api.test/api/v1/knowledge-bases/kb-1/knowledge/file',
      headers: { accept: 'application/json' },
      file: { uri: 'blob:doc-1', name: 'spec.pdf', type: 'application/pdf', size: 9 },
      fields: { tag_ids: 'tag-1' },
    });
    await new Promise<void>((resolve) => setTimeout(resolve, 0));
    const xhr = StubXHR.instances.at(-1)!;
    xhr.upload.onprogress?.({ loaded: 3, total: 12, lengthComputable: true });
    xhr.respond(200, { success: true, data: { id: 'doc-1' } });
    const result = await pending;
    stop();

    assert.equal(result.status, 200);
    assert.deepEqual(result.body, { success: true, data: { id: 'doc-1' } });
    assert.deepEqual(progress, [25], 'byte-level XHR progress reaches the observer');
    assert.equal(xhr.method, 'POST');
    assert.equal(xhr.url, 'https://api.test/api/v1/knowledge-bases/kb-1/knowledge/file');
    assert.equal(xhr.headers.authorization, 'Bearer upload-access');
    assert.equal(xhr.headers['x-tenant-id'], 'tenant-upload');
    assert.equal(xhr.headers['accept-language'], 'zh-CN');
    assert.equal(xhr.headers['x-request-id'], 'upload-request-id');
    assert.ok(xhr.body instanceof FormData);
    assert.equal((xhr.body as FormData).get('tag_ids'), 'tag-1');
    assert.ok((xhr.body as FormData).get('file') instanceof Blob);
    // The observer is consumed by the upload it described.
    assert.equal(uploadProgressListener('blob:doc-1'), undefined);
  });
});

test('keeps the fetch multipart path when no progress observer is registered', async () => {
  const uploads: Array<{ body: unknown; headers: Record<string, string> }> = [];
  const transport = createBrowserTransport({
    credential: { kind: 'bearer', accessToken: 'fetch-upload-access' },
    fetcher: (async (url, init) => {
      if (String(url) === 'blob:doc-2') {
        return {
          status: 200,
          headers: new Headers(),
          json: async () => ({}),
          text: async () => '',
          blob: async () => new Blob(['zip'], { type: 'application/zip' }),
        };
      }
      uploads.push({ body: init?.body, headers: init?.headers ?? {} });
      return {
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ success: true, data: { id: 'doc-2' } }),
        text: async () => '',
      };
    }) satisfies FetchLike,
  });

  await withXhrStub(async () => {
    const result = await transport.sendMultipartFile!({
      method: 'POST',
      url: 'https://api.test/api/v1/skills/catalog',
      headers: { accept: 'application/json' },
      file: { uri: 'blob:doc-2', name: 'skill.zip', type: 'application/zip', size: 3 },
      fields: {},
    });

    assert.equal(result.status, 200);
    assert.deepEqual(result.body, { success: true, data: { id: 'doc-2' } });
    assert.equal(uploads.length, 1);
    assert.ok(uploads[0]!.body instanceof FormData);
    assert.equal(uploads[0]!.headers.authorization, 'Bearer fetch-upload-access');
    assert.equal(StubXHR.instances.length, 0, 'unobserved uploads stay on fetch');
  });
});

test('stop() releases a progress observer before the upload runs', async () => {
  const stop = observeUploadProgress('blob:doc-3', () => {});
  stop();
  assert.equal(uploadProgressListener('blob:doc-3'), undefined);
});

test('accept-language defaults to the app locale convention, never the browser language', async () => {
  // Upstream request.ts sends getCurrentLanguage() (i18n locale || localStorage
  // 'locale' || zh-CN) — the browser language is never sniffed. The transport
  // must default the same way so server-localized payloads (builtin agent
  // names) follow the UI language; main.tsx used to pass locale:
  // navigator.language, which forced en-US inside a Chinese deployment
  // (2026-09-18 round 7).
  const seen: Array<string | undefined> = [];
  const originalWindow = (globalThis as { window?: unknown }).window;
  const fetcher = (async (_url: unknown, init: { headers?: Record<string, string> } | undefined) => {
    seen.push(init?.headers?.['accept-language']);
    return { status: 200, headers: new Headers({ 'content-type': 'application/json' }), json: async () => ({ success: true, data: [] }), text: async () => '' };
  }) as unknown as FetchLike;

  try {
    // No options.locale and no stored preference: zh-CN deployment default.
    (globalThis as { window?: unknown }).window = { localStorage: { getItem: () => null } };
    const transport = createBrowserTransport({ fetcher });
    await transport.send({ method: 'GET', url: 'https://api.test/api/v1/agents', headers: { accept: 'application/json' } });
    assert.equal(seen.at(-1), 'zh-CN');

    // Stored language switch wins.
    (globalThis as { window?: unknown }).window = { localStorage: { getItem: (key: string) => (key === 'locale' ? 'ja-JP' : null) } };
    await transport.send({ method: 'GET', url: 'https://api.test/api/v1/agents', headers: { accept: 'application/json' } });
    assert.equal(seen.at(-1), 'ja-JP');

    // A reactive provider (function) re-resolves per request, matching the
    // upstream interceptor that reads the live i18n locale.
    let current = 'zh-CN';
    const reactive = createBrowserTransport({ locale: () => current, fetcher });
    await reactive.send({ method: 'GET', url: 'https://api.test/api/v1/agents', headers: { accept: 'application/json' } });
    assert.equal(seen.at(-1), 'zh-CN');
    current = 'ko-KR';
    await reactive.send({ method: 'GET', url: 'https://api.test/api/v1/agents', headers: { accept: 'application/json' } });
    assert.equal(seen.at(-1), 'ko-KR');
  } finally {
    (globalThis as { window?: unknown }).window = originalWindow;
  }
});
