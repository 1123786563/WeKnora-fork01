import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createAttachmentEntryActions,
  createSessionUploadPort,
  createSessionUploads,
  defaultUploadLimits,
  validateUpload,
  type AttachmentCandidate,
  type UploadFileRequest,
  type UploadScope,
  type UploadTransport,
} from './upload.ts';

test('oversized attachment is rejected before network access', () => {
  const f = { sessionID: 's', uri: 'file:///tmp/a.pdf', name: 'a.pdf', mime: 'application/pdf', size: 11 };
  assert.throws(() => validateUpload(f, 10), /UPLOAD_TOO_LARGE/);
  assert.doesNotThrow(() => validateUpload({ ...f, size: 10 }, 10));
});

test('validateUpload rejects malformed inputs', () => {
  const f = { sessionID: 's', uri: 'file:///tmp/a.pdf', name: 'a.pdf', mime: 'application/pdf', size: 5 };
  assert.throws(() => validateUpload({ ...f, sessionID: '' }, 10), /INVALID_UPLOAD/);
  assert.throws(() => validateUpload({ ...f, name: '' }, 10), /INVALID_UPLOAD/);
  assert.throws(() => validateUpload({ ...f, uri: '' }, 10), /INVALID_UPLOAD/);
  assert.throws(() => validateUpload({ ...f, size: -1 }, 10), /INVALID_UPLOAD/);
  assert.throws(() => validateUpload({ ...f, size: 1.5 }, 10), /INVALID_UPLOAD/);
  assert.throws(() => validateUpload(f, 0), /INVALID_UPLOAD/);
});

// ---------------------------------------------------------------------------
// Fake transport implementing the local file port (sendMultipartFile). It
// stands in for the platform adapter that turns content:// and file:// URIs
// into multipart bytes.
// ---------------------------------------------------------------------------

interface CapturedCall { request: UploadFileRequest }

function fakeTransport(options: {
  status?: number;
  body?: unknown;
  delay?: (request: UploadFileRequest) => Promise<void>;
  /** Runs synchronously right before the response settles. */
  afterResponse?: () => void;
} = {}) {
  const calls: CapturedCall[] = [];
  const transport: UploadTransport = {
    async sendMultipartFile(request) {
      calls.push({ request });
      if (options.delay) await options.delay(request);
      // Mirror real transports: an aborted signal rejects even if the gate
      // released afterwards.
      if (request.signal?.aborted) {
        const error = new Error('The upload was aborted');
        error.name = 'AbortError';
        throw error;
      }
      options.afterResponse?.();
      return { status: options.status ?? 202, headers: {}, body: options.body ?? { success: true, data: { id: 'doc-1' } } };
    },
  };
  return { transport, calls };
}

function input(overrides: Partial<Parameters<typeof validateUpload>[0]> = {}) {
  return {
    sessionID: 'sess-1',
    uri: 'content://media/external/file/42',
    name: 'report.pdf',
    mime: 'application/pdf',
    size: 4,
    ...overrides,
  };
}

test('upload port posts the original file through the local file port as multipart', async () => {
  const { transport, calls } = fakeTransport();
  const port = createSessionUploadPort({ baseURL: 'https://api.example', transport, limits: defaultUploadLimits });
  const result = await port.send(input(), new AbortController().signal);
  assert.equal(result.attachmentID, 'doc-1');
  assert.equal(calls.length, 1);
  const request = calls[0].request;
  assert.equal(request.method, 'POST');
  assert.equal(request.url, 'https://api.example/api/v1/sessions/sess-1/attachments');
  assert.equal(request.file.uri, 'content://media/external/file/42');
  assert.equal(request.file.name, 'report.pdf');
  assert.equal(request.file.type, 'application/pdf');
  // The device-local path must never be sent to the backend as a field: only
  // the multipart file part carries content, and fields carry no URI.
  assert.equal(Object.values(request.fields).includes('content://media/external/file/42'), false);
  assert.equal(JSON.stringify(request).includes('file:///data/user'), false);
});

test('upload port handles file:// URIs the same way as content:// URIs', async () => {
  const { transport, calls } = fakeTransport();
  const port = createSessionUploadPort({ baseURL: 'https://api.example', transport, limits: defaultUploadLimits });
  const result = await port.send(input({ uri: 'file:///var/mobile/Media/a.jpg', name: 'a.jpg', mime: 'image/jpeg' }), new AbortController().signal);
  assert.equal(result.attachmentID, 'doc-1');
  assert.equal(calls[0].request.file.uri, 'file:///var/mobile/Media/a.jpg');
});

test('upload port validates size before any network access', async () => {
  const { transport, calls } = fakeTransport();
  const port = createSessionUploadPort({ baseURL: 'https://api.example', transport, limits: { maxBytes: 3 } });
  await assert.rejects(() => port.send(input({ size: 4 }), new AbortController().signal), /UPLOAD_TOO_LARGE/);
  assert.equal(calls.length, 0);
});

test('upload port applies deployment allowed types as a UX-layer check only', async () => {
  const { transport, calls } = fakeTransport();
  const port = createSessionUploadPort({
    baseURL: 'https://api.example',
    transport,
    limits: { maxBytes: 10, allowedMimes: ['application/pdf', 'image/jpeg'] },
  });
  await assert.rejects(() => port.send(input({ mime: 'video/mp4' }), new AbortController().signal), /UPLOAD_TYPE_NOT_ALLOWED/);
  assert.equal(calls.length, 0);
  await port.send(input(), new AbortController().signal);
  assert.equal(calls.length, 1);
});

test('upload port never substitutes a compressed preview for the original file', async () => {
  const { transport, calls } = fakeTransport();
  const port = createSessionUploadPort({ baseURL: 'https://api.example', transport, limits: defaultUploadLimits });
  await port.send(input({ previewUri: 'file:///cache/thumb-1.webp', previewMime: 'image/webp' }), new AbortController().signal);
  assert.equal(calls[0].request.file.uri, 'content://media/external/file/42');
  assert.equal(calls[0].request.file.type, 'application/pdf');
});

test('upload port maps HTTP and envelope failures to stable error codes', async () => {
  const failing = fakeTransport({ status: 413, body: { success: false } });
  const portA = createSessionUploadPort({ baseURL: 'https://api.example', transport: failing.transport, limits: defaultUploadLimits });
  await assert.rejects(() => portA.send(input(), new AbortController().signal), /UPLOAD_HTTP_413/);

  const malformed = fakeTransport({ status: 202, body: { success: true } });
  const portB = createSessionUploadPort({ baseURL: 'https://api.example', transport: malformed.transport, limits: defaultUploadLimits });
  await assert.rejects(() => portB.send(input(), new AbortController().signal), /UPLOAD_RESPONSE_INVALID/);
});

test('upload port fails closed without a multipart file transport', async () => {
  const port = createSessionUploadPort({ baseURL: 'https://api.example', transport: {}, limits: defaultUploadLimits });
  await assert.rejects(() => port.send(input(), new AbortController().signal), /UPLOAD_TRANSPORT_UNSUPPORTED/);
});

test('upload port honours an aborted signal without touching the network', async () => {
  const { transport, calls } = fakeTransport();
  const port = createSessionUploadPort({ baseURL: 'https://api.example', transport, limits: defaultUploadLimits });
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(() => port.send(input(), controller.signal), /CANCELLED/);
  assert.equal(calls.length, 0);
});

// ---------------------------------------------------------------------------
// Session upload manager: cancel, failure retention, scope invalidation.
// ---------------------------------------------------------------------------

function scopeStub(): UploadScope & { switchAway(): void } {
  let generation = 1;
  let current = 1;
  const signals = new Map<number, AbortController>();
  return {
    capture() {
      const controller = new AbortController();
      signals.set(generation, controller);
      return { generation, signal: controller.signal };
    },
    accept(value: number) { return value === current; },
    switchAway() { signals.get(current)?.abort(); generation += 1; current = generation; },
  };
}

function pendingTransport() {
  let release!: () => void;
  const gate = new Promise<void>((resolve) => { release = resolve; });
  const { transport, calls } = fakeTransport({ delay: async () => gate });
  return { transport, calls, release };
}

test('manager tracks an upload from uploading to uploaded', async () => {
  const { transport } = fakeTransport();
  const port = createSessionUploadPort({ baseURL: 'https://api.example', transport, limits: defaultUploadLimits });
  const uploads = createSessionUploads({ port, scope: scopeStub() });
  const done = uploads.attach(input());
  assert.equal(uploads.records().length, 1);
  assert.equal(uploads.records()[0].status, 'uploading');
  await done;
  assert.equal(uploads.records()[0].status, 'uploaded');
  assert.equal(uploads.records()[0].attachmentID, 'doc-1');
  assert.deepEqual(uploads.ready(), [{ attachmentID: 'doc-1', name: 'report.pdf' }]);
});

test('cancelling an in-flight upload aborts it and clears the record', async () => {
  const pending = pendingTransport();
  const port = createSessionUploadPort({ baseURL: 'https://api.example', transport: pending.transport, limits: defaultUploadLimits });
  const uploads = createSessionUploads({ port, scope: scopeStub() });
  const attempt = uploads.attach(input());
  const id = uploads.records()[0].id;
  uploads.cancel(id);
  pending.release();
  await assert.rejects(() => attempt, /CANCELLED/);
  assert.equal(uploads.records().length, 0, 'unfinished upload record must be cleaned up');
  assert.deepEqual(uploads.ready(), []);
});

test('a failed upload keeps its record so the draft is not lost', async () => {
  const { transport } = fakeTransport({ status: 500, body: { success: false } });
  const port = createSessionUploadPort({ baseURL: 'https://api.example', transport, limits: defaultUploadLimits });
  const uploads = createSessionUploads({ port, scope: scopeStub() });
  await assert.rejects(() => uploads.attach(input()), /UPLOAD_HTTP_500/);
  const records = uploads.records();
  assert.equal(records.length, 1);
  assert.equal(records[0].status, 'failed');
  assert.equal(records[0].input.name, 'report.pdf');
  assert.deepEqual(uploads.ready(), []);
  uploads.remove(records[0].id);
  assert.equal(uploads.records().length, 0);
});

test('a late result after a space switch is discarded and un-referenced server-side', async () => {
  const scope = scopeStub();
  // The upload completes server-side and the response is being delivered
  // when the space switches: the attachment exists, so the late result must
  // be discarded and the attachment deleted rather than orphaned.
  const { transport } = fakeTransport({ afterResponse: () => scope.switchAway() });
  const port = createSessionUploadPort({ baseURL: 'https://api.example', transport, limits: defaultUploadLimits });
  const removed: string[] = [];
  const uploads = createSessionUploads({
    port,
    scope,
    removeAttachment: async (attachmentID) => { removed.push(attachmentID); },
  });
  const done = uploads.attach(input());
  await done;
  assert.deepEqual(removed, ['doc-1'], 'late attachment must be deleted so it is not orphaned');
  assert.equal(uploads.records().length, 0, 'late result must not surface in the new space');
  assert.deepEqual(uploads.ready(), []);
});

test('manager notifies subscribers on state changes', async () => {
  const { transport } = fakeTransport();
  const port = createSessionUploadPort({ baseURL: 'https://api.example', transport, limits: defaultUploadLimits });
  const uploads = createSessionUploads({ port, scope: scopeStub() });
  let changes = 0;
  uploads.subscribe(() => { changes += 1; });
  await uploads.attach(input());
  assert.ok(changes >= 2, `expected at least started+finished notifications, got ${changes}`);
});

// ---------------------------------------------------------------------------
// Native entry seams: picker, camera and system share.
// ---------------------------------------------------------------------------

function sourceStub(candidates: {
  documents?: AttachmentCandidate[] | null;
  photo?: AttachmentCandidate | null;
  shared?: AttachmentCandidate | null;
}) {
  const calls: string[] = [];
  return {
    calls,
    source: {
      async pickDocuments() { calls.push('pick'); return candidates.documents ?? null; },
      async capturePhoto() { calls.push('photo'); return candidates.photo ?? null; },
      async consumeSharedFile() { calls.push('shared'); return candidates.shared ?? null; },
    },
  };
}

function uploadsStub() {
  const attached: AttachmentCandidate[] = [];
  return {
    attached,
    uploads: {
      attach: async (file: AttachmentCandidate) => { attached.push(file); },
      records: () => [] as never[],
      ready: () => [] as never[],
      cancel: () => {},
      remove: () => {},
      subscribe: () => () => {},
    } as never,
  };
}

test('entry actions attach picked documents to the session', async () => {
  const { source } = sourceStub({ documents: [input() as AttachmentCandidate] });
  const { attached, uploads } = uploadsStub();
  const actions = createAttachmentEntryActions({ source, uploads, sessionID: 'sess-1' });
  await actions.chooseFromLibrary();
  assert.equal(attached.length, 1);
  assert.equal(attached[0].uri, 'content://media/external/file/42');
});

test('entry actions do nothing when the user cancels the picker or camera', async () => {
  const { source, calls } = sourceStub({});
  const { attached, uploads } = uploadsStub();
  const actions = createAttachmentEntryActions({ source, uploads, sessionID: 'sess-1' });
  await actions.chooseFromLibrary();
  await actions.takePhoto();
  await actions.acceptSharedFile();
  assert.equal(attached.length, 0);
  assert.deepEqual(calls, ['pick', 'photo', 'shared']);
});

test('share entry attaches a shared file with its session scope', async () => {
  const shared: AttachmentCandidate = { uri: 'content://shared/inbox/9', name: 'note.txt', mime: 'text/plain', size: 3 };
  const { source } = sourceStub({ shared });
  const { attached, uploads } = uploadsStub();
  const actions = createAttachmentEntryActions({ source, uploads, sessionID: 'sess-7' });
  await actions.acceptSharedFile();
  assert.equal(attached.length, 1);
  assert.equal((attached[0] as { sessionID?: string }).sessionID, 'sess-7');
});
