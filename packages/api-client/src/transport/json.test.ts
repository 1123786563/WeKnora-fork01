import assert from 'node:assert/strict';
import test from 'node:test';

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
  sent = false;

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
    this.sent = true;
    this.body = body;
  }

  abort(): void {
    this.onabort?.();
  }

  getAllResponseHeaders(): string {
    return 'content-type: application/json\r\nx-request-id: req-1\r\n';
  }

  respond(status: number, body: unknown): void {
    this.status = status;
    this.responseText = typeof body === 'string' ? body : JSON.stringify(body);
    this.onload?.();
  }
}

const { createJsonTransport } = await import('./json.ts');
const realCreateObjectURL = URL.createObjectURL;
const realRevokeObjectURL = URL.revokeObjectURL;

function stubObjectUrls(revoked: string[]): void {
  let counter = 0;
  (URL as unknown as { createObjectURL: (value: Blob) => string }).createObjectURL = () => 'blob:stub-' + (counter += 1);
  (URL as unknown as { revokeObjectURL: (uri: string) => void }).revokeObjectURL = (uri: string) => {
    revoked.push(uri);
  };
}

function restoreObjectUrls(): void {
  URL.createObjectURL = realCreateObjectURL;
  URL.revokeObjectURL = realRevokeObjectURL;
}

test('sendMultipartFile reports byte-granular XHR upload progress', async () => {
  StubXHR.instances = [];
  const globalScope = globalThis as unknown as { XMLHttpRequest?: unknown };
  globalScope.XMLHttpRequest = StubXHR;
  const revoked: string[] = [];
  stubObjectUrls(revoked);
  const fetchedUrls: string[] = [];
  try {
    const transport = createJsonTransport((async (url) => {
      fetchedUrls.push(String(url));
      return {
        status: 200,
        headers: new Headers({ 'content-type': 'application/zip' }),
        json: async () => ({}),
        text: async () => '',
        blob: async () => new Blob([new Uint8Array(1024)], { type: 'application/zip' }),
      };
    }) satisfies Parameters<typeof createJsonTransport>[0]);

    const progress: Array<{ loaded: number; total: number }> = [];
    const pending = transport.sendMultipartFile!({
      method: 'POST',
      url: 'https://api.test/api/v1/skills/catalog',
      headers: { accept: 'application/json', authorization: 'Bearer token-1' },
      file: { uri: 'blob:skill-zip', name: 'skill.zip', type: 'application/zip', size: 1024 },
      fields: {},
      onProgress: (event) => progress.push({ loaded: event.loaded, total: event.total }),
    });

    // The transport reads the blob: source through an async fetch first.
    await new Promise<void>((resolve) => setTimeout(resolve, 0));
    const xhr = StubXHR.instances.at(-1)!;
    assert.equal(xhr.sent, true);
    assert.equal(xhr.method, 'POST');
    assert.equal(xhr.url, 'https://api.test/api/v1/skills/catalog');
    // The browser must set the multipart boundary, so content-type stays unset.
    xhr.upload.onprogress?.({ loaded: 256, total: 1048, lengthComputable: true });
    xhr.upload.onprogress?.({ loaded: 1048, total: 1048, lengthComputable: true });
    xhr.respond(200, { success: true, data: { id: 'cat-1' } });

    const result = await pending;
    assert.deepEqual(progress, [{ loaded: 256, total: 1048 }, { loaded: 1048, total: 1048 }]);
    assert.equal(result.status, 200);
    assert.deepEqual(result.body, { success: true, data: { id: 'cat-1' } });
    assert.equal(result.headers['x-request-id'], 'req-1');
    assert.equal(xhr.headers.authorization, 'Bearer token-1');
    assert.equal(xhr.headers.accept, 'application/json');
    assert.equal(xhr.headers['content-type'], undefined);
    assert.ok(xhr.body instanceof FormData);
    assert.deepEqual(fetchedUrls, ['blob:skill-zip']);
    assert.deepEqual(revoked, ['blob:skill-zip']);
  } finally {
    delete globalScope.XMLHttpRequest;
    restoreObjectUrls();
  }
});

test('sendMultipartFile keeps the fetch path when no onProgress is provided', async () => {
  StubXHR.instances = [];
  const globalScope = globalThis as unknown as { XMLHttpRequest?: unknown };
  globalScope.XMLHttpRequest = StubXHR;
  const revoked: string[] = [];
  stubObjectUrls(revoked);
  try {
    const uploads: Array<{ body: unknown; headers: Record<string, string> }> = [];
    const transport = createJsonTransport((async (url, init) => {
      if (String(url) === 'blob:doc-1') {
        return {
          status: 200,
          headers: new Headers(),
          json: async () => ({}),
          text: async () => '',
          blob: async () => new Blob(['pdf-bytes'], { type: 'application/pdf' }),
        };
      }
      uploads.push({ body: init?.body, headers: init?.headers ?? {} });
      return {
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ success: true, data: { id: 'doc-1' } }),
        text: async () => '',
      };
    }) satisfies Parameters<typeof createJsonTransport>[0]);

    const result = await transport.sendMultipartFile!({
      method: 'POST',
      url: 'https://api.test/api/v1/knowledge-bases/kb-1/knowledge/file',
      headers: { accept: 'application/json', authorization: 'Bearer token-2' },
      file: { uri: 'blob:doc-1', name: 'spec.pdf', type: 'application/pdf', size: 9 },
      fields: { tag_ids: 'tag-1' },
    });

    assert.equal(result.status, 200);
    assert.deepEqual(result.body, { success: true, data: { id: 'doc-1' } });
    assert.equal(uploads.length, 1);
    const form = uploads[0]!.body as FormData;
    assert.ok(form instanceof FormData);
    assert.equal(form.get('tag_ids'), 'tag-1');
    assert.ok(form.get('file') instanceof Blob);
    assert.equal(uploads[0]!.headers.authorization, 'Bearer token-2');
    assert.equal(StubXHR.instances.length, 0);
    assert.deepEqual(revoked, ['blob:doc-1']);
  } finally {
    delete globalScope.XMLHttpRequest;
    restoreObjectUrls();
  }
});

test('sendMultipartFile falls back to fetch when the platform has no XMLHttpRequest', async () => {
  StubXHR.instances = [];
  const globalScope = globalThis as unknown as { XMLHttpRequest?: unknown };
  delete globalScope.XMLHttpRequest;
  const revoked: string[] = [];
  stubObjectUrls(revoked);
  try {
    let sawProgress = false;
    const transport = createJsonTransport((async (url) => {
      if (String(url) === 'blob:doc-2') {
        return {
          status: 200,
          headers: new Headers(),
          json: async () => ({}),
          text: async () => '',
          blob: async () => new Blob(['zip'], { type: 'application/zip' }),
        };
      }
      return {
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ success: true }),
        text: async () => '',
      };
    }) satisfies Parameters<typeof createJsonTransport>[0]);

    const result = await transport.sendMultipartFile!({
      method: 'POST',
      url: 'https://api.test/api/v1/skills/catalog',
      headers: {},
      file: { uri: 'blob:doc-2', name: 'skill.zip', type: 'application/zip', size: 3 },
      fields: {},
      onProgress: () => { sawProgress = true; },
    });

    assert.equal(result.status, 200);
    assert.equal(sawProgress, false, 'fetch fallback cannot observe byte progress');
    assert.equal(StubXHR.instances.length, 0);
  } finally {
    restoreObjectUrls();
  }
});

test('sendMultipartFile rejects with the upload source status when the file cannot be read', async () => {
  StubXHR.instances = [];
  const revoked: string[] = [];
  stubObjectUrls(revoked);
  try {
    const transport = createJsonTransport((async () => ({
      status: 404,
      headers: new Headers(),
      json: async () => ({}),
      text: async () => '',
    })) satisfies Parameters<typeof createJsonTransport>[0]);

    await assert.rejects(
      transport.sendMultipartFile!({
        method: 'POST',
        url: 'https://api.test/api/v1/skills/catalog',
        headers: {},
        file: { uri: 'blob:missing', name: 'skill.zip', type: 'application/zip' },
        fields: {},
      }),
      /unable to read upload source: 404/,
    );
    assert.equal(StubXHR.instances.length, 0);
  } finally {
    restoreObjectUrls();
  }
});

test('sendMultipartFile maps an aborted upload to an AbortError rejection', async () => {
  StubXHR.instances = [];
  const controller = new AbortController();
  controller.abort();
  const globalScope = globalThis as unknown as { XMLHttpRequest?: unknown };
  globalScope.XMLHttpRequest = StubXHR;
  try {
    const transport = createJsonTransport((async () => ({
      status: 200,
      headers: new Headers(),
      json: async () => ({}),
      text: async () => '',
      blob: async () => new Blob(['zip'], { type: 'application/zip' }),
    })) satisfies Parameters<typeof createJsonTransport>[0]);

    await assert.rejects(
      transport.sendMultipartFile!({
        method: 'POST',
        url: 'https://api.test/api/v1/skills/catalog',
        headers: {},
        file: { uri: 'blob:doc-3', name: 'skill.zip', type: 'application/zip' },
        fields: {},
        signal: controller.signal,
        onProgress: () => {},
      }),
      (error: unknown) => (error as Error).name === 'AbortError',
    );
    assert.equal(StubXHR.instances.length, 0, 'an aborted request never reaches the wire');
  } finally {
    delete globalScope.XMLHttpRequest;
  }
});
