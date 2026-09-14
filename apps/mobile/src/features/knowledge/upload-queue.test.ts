import assert from 'node:assert/strict';
import test from 'node:test';
import type { NativeFileSource } from '@weknora/api-client';
import { uploadKnowledgeFiles } from './upload-queue.ts';
import type { UploadEvent } from './upload-progress.ts';

const files: NativeFileSource[] = [
  { uri: 'file:///one.pdf', name: 'one.pdf', type: 'application/pdf' },
  { uri: 'file:///two.md', name: 'two.md', type: 'text/markdown' },
];

test('uploads a picker batch FIFO and emits one lifecycle pair per file', async () => {
  const calls: string[] = [];
  const events: UploadEvent[] = [];
  const result = await uploadKnowledgeFiles(files, 'kb-1', {
    signal: new AbortController().signal,
    createUploadId: (_file, index) => `u${index}`,
    upload: async (file) => { calls.push(file.name); },
    dispatch: (event) => events.push(event),
  });

  assert.deepEqual(calls, ['one.pdf', 'two.md']);
  assert.equal(result.succeeded, 2);
  assert.deepEqual(result.failures, []);
  assert.deepEqual(events.map((event) => [event.type, 'uploadId' in event ? event.uploadId : undefined]), [
    ['start', 'u0'], ['complete', 'u0'], ['start', 'u1'], ['complete', 'u1'],
  ]);
});

test('keeps processing later files when one upload fails', async () => {
  const events: UploadEvent[] = [];
  const result = await uploadKnowledgeFiles(files, 'kb-1', {
    signal: new AbortController().signal,
    createUploadId: (_file, index) => `u${index}`,
    upload: async (file) => { if (file.name === 'one.pdf') throw new Error('duplicate'); },
    dispatch: (event) => events.push(event),
  });

  assert.equal(result.succeeded, 1);
  assert.deepEqual(result.failures.map(({ file, error }) => [file.name, error]), [['one.pdf', 'duplicate']]);
  const failed = events.find((event) => event.type === 'complete' && event.uploadId === 'u0');
  const passed = events.find((event) => event.type === 'complete' && event.uploadId === 'u1');
  assert.equal(failed?.type === 'complete' ? failed.status : undefined, 'error');
  assert.equal(passed?.type === 'complete' ? passed.status : undefined, 'success');
});

test('stops before the next file when the active upload is cancelled', async () => {
  const controller = new AbortController();
  const calls: string[] = [];
  const result = await uploadKnowledgeFiles(files, 'kb-1', {
    signal: controller.signal,
    createUploadId: (_file, index) => `u${index}`,
    upload: async (file) => { calls.push(file.name); controller.abort(); throw new Error('cancelled'); },
    dispatch: () => undefined,
  });

  assert.deepEqual(calls, ['one.pdf']);
  assert.equal(result.succeeded, 0);
  assert.equal(result.aborted, true);
  assert.equal(result.failures[0]?.error, 'cancelled');
});
