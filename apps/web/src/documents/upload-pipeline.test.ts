import assert from 'node:assert/strict';
import test from 'node:test';

import { formatBytes, runUploadPipeline, toUploadEntries, uploadSummary, type UploadEntry } from './upload-pipeline.ts';

function entry(name: string): File {
  return new File(['x'], name);
}

function entryWithSize(name: string, size: number): UploadEntry {
  return { file: new File(['x'], name), name, size };
}

test('multi-file pipeline uploads once per file with tag_ids', async () => {
  const entries = toUploadEntries([entry('a.pdf'), entry('b.docx'), entry('c.txt')]);
  const uploaded: string[] = [];
  const seenTags: (string[] | undefined)[] = [];
  const snapshots: number[] = [];
  const final = await runUploadPipeline({
    entries,
    tagIds: ['tag-1', 'tag-2'],
    upload: async (item, tagIds) => {
      uploaded.push(item.name);
      seenTags.push(tagIds);
      snapshots.push(snapshots.length);
    },
  });
  assert.deepEqual(uploaded, ['a.pdf', 'b.docx', 'c.txt']);
  for (const tags of seenTags) assert.deepEqual(tags, ['tag-1', 'tag-2']);
  assert.ok(final.every((state) => state.status === 'done'));
});

test('per-file error surfaces and does not stop the remaining uploads', async () => {
  const entries = toUploadEntries([entry('ok.txt'), entry('bad.txt'), entry('later.txt')]);
  const uploaded: string[] = [];
  const final = await runUploadPipeline({
    entries,
    upload: async (item) => {
      uploaded.push(item.name);
      if (item.name === 'bad.txt') throw new Error('413 too large');
    },
  });
  assert.deepEqual(uploaded, ['ok.txt', 'bad.txt', 'later.txt']);
  assert.equal(final[1].status, 'error');
  assert.equal(final[1].message, '413 too large');
  assert.equal(final[2].status, 'done');
});

test('cancel (pre-aborted signal) clears without uploading', async () => {
  const controller = new AbortController();
  controller.abort();
  let calls = 0;
  const final = await runUploadPipeline({
    entries: toUploadEntries([entry('a.pdf'), entry('b.pdf')]),
    signal: controller.signal,
    upload: async () => { calls += 1; },
  });
  assert.equal(calls, 0);
  assert.ok(final.every((state) => state.status === 'pending'));
});

test('abort mid-batch stops at the next file boundary', async () => {
  const controller = new AbortController();
  const uploaded: string[] = [];
  const final = await runUploadPipeline({
    entries: toUploadEntries([entry('a.pdf'), entry('b.pdf'), entry('c.pdf')]),
    signal: controller.signal,
    upload: async (item) => {
      uploaded.push(item.name);
      if (item.name === 'a.pdf') controller.abort();
    },
  });
  assert.deepEqual(uploaded, ['a.pdf']);
  assert.equal(final[0].status, 'done');
  assert.ok(final.slice(1).every((state) => state.status === 'pending'));
});

test('upload summary lists file names plus sizes for the confirm dialog', () => {
  const entries: UploadEntry[] = [entryWithSize('a.pdf', 1024), entryWithSize('b.txt', 512)];
  const summary = uploadSummary(entries);
  assert.equal(summary.count, 2);
  assert.equal(summary.totalLabel, '1.5 KB');
  assert.equal(formatBytes(512), '512 B');
});
