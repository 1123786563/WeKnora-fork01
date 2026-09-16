import assert from 'node:assert/strict';
import test from 'node:test';
import type { NativeFileSource } from '@weknora/api-client';
import { ApiError } from '@weknora/api-client';
import { knowledgeUploadErrorLabel, uploadKnowledgeFiles } from './upload-queue.ts';
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
    locale: 'en-US',
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

test('forwards byte progress into the Vue-parity upload event bus', async () => {
  const events: UploadEvent[] = [];
  await uploadKnowledgeFiles([files[0]], 'kb-1', {
    signal: new AbortController().signal,
    locale: 'en-US',
    createUploadId: () => 'u0',
    upload: async (_file, _signal, onProgress) => {
      onProgress({ loaded: 25, total: 100 });
      onProgress({ loaded: 75, total: 100 });
    },
    dispatch: (event) => events.push(event),
  });
  assert.deepEqual(events.map((event) => event.type), ['start', 'progress', 'progress', 'complete']);
  assert.deepEqual(events.filter((event): event is Extract<UploadEvent, { type: 'progress' }> => event.type === 'progress').map((event) => event.progress), [25, 75]);
});

test('keeps processing later files when one upload fails', async () => {
  const events: UploadEvent[] = [];
  const result = await uploadKnowledgeFiles(files, 'kb-1', {
    signal: new AbortController().signal,
    locale: 'en-US',
    createUploadId: (_file, index) => `u${index}`,
    upload: async (file) => { if (file.name === 'one.pdf') throw new Error('duplicate'); },
    dispatch: (event) => events.push(event),
  });

  assert.equal(result.succeeded, 1);
  assert.deepEqual(result.failures.map(({ file, error }) => [file.name, error]), [['one.pdf', 'File already exists']]);
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
    locale: 'en-US',
    createUploadId: (_file, index) => `u${index}`,
    upload: async (file) => { calls.push(file.name); controller.abort(); throw new Error('cancelled'); },
    dispatch: () => undefined,
  });

  assert.deepEqual(calls, ['one.pdf']);
  assert.equal(result.succeeded, 0);
  assert.equal(result.aborted, true);
  assert.equal(result.failures[0]?.error, 'File upload failed!');
});

test('localizes stable duplicate and generic upload errors for every mobile locale', () => {
  for (const locale of ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const) {
    assert.notEqual(knowledgeUploadErrorLabel(locale, new Error('duplicate_file')), 'duplicate_file');
    assert.notEqual(knowledgeUploadErrorLabel(locale, new Error('network timeout')), 'network timeout');
  }
  assert.equal(knowledgeUploadErrorLabel('zh-CN', new Error('duplicate_file')), '文件已存在');
  assert.equal(knowledgeUploadErrorLabel('en-US', new Error('duplicate_file')), 'File already exists');
});

test('reads duplicate codes from structured ApiError fields and wrapped response bodies', () => {
  assert.equal(knowledgeUploadErrorLabel('zh-CN', new ApiError({ code: 'duplicate_file', message: 'request rejected' })), '文件已存在');
  assert.equal(knowledgeUploadErrorLabel('en-US', { status: 'duplicate', body: { error: { code: 'duplicate_file' } } }), 'File already exists');
});
