import assert from 'node:assert/strict';
import test from 'node:test';
import { applyUploadTaskEvent, cancelUploadTask, clampUploadProgress, findUploadTargetPage, getUploadAction, isUploadActionKey, patchUploadTask, retryUploadTask, summarizeUploadTasks, upsertUploadTask, type UploadTaskState } from './upload-progress.ts';

const task = (uploadId: string, kbId: string, progress: number, status: UploadTaskState['status'] = 'uploading'): UploadTaskState => ({ uploadId, kbId, progress, status });

test('upload progress clamps and replaces duplicate task events', () => {
  assert.equal(clampUploadProgress(123.4), 100);
  assert.equal(clampUploadProgress(Number.NaN), 0);
  assert.equal(clampUploadProgress(Number.POSITIVE_INFINITY), 0);
  const next = upsertUploadTask(upsertUploadTask([], task('u1', 'kb1', -4)), task('u1', 'kb1', 55.2));
  assert.deepEqual(next, [task('u1', 'kb1', 55)]);
});

test('upload events accept zero-like knowledge-base ids but reject only absent ids', () => {
  const started = applyUploadTaskEvent([], { type: 'start', uploadId: 'u0', kbId: 0 });
  assert.deepEqual(started, [task('u0', '0', 0)]);
  assert.deepEqual(applyUploadTaskEvent([], { type: 'start', uploadId: 'missing', kbId: '' }), []);
  assert.deepEqual(applyUploadTaskEvent([], { type: 'start', uploadId: 'missing', kbId: null as never }), []);
});

test('cancel and retry keep terminal states explicit and only retry failed work', () => {
  const tasks = [task('u1', 'kb1', 42), task('u2', 'kb1', 100, 'success')];
  const cancelled = cancelUploadTask(tasks, 'u1');
  assert.deepEqual(cancelled, [task('u1', 'kb1', 42, 'cancelled'), task('u2', 'kb1', 100, 'success')]);
  assert.deepEqual(retryUploadTask(cancelled, 'u1'), [task('u1', 'kb1', 0), task('u2', 'kb1', 100, 'success')]);
  assert.deepEqual(retryUploadTask(cancelled, 'u2'), cancelled);
});

test('upload actions expose localized keys and native button keyboard activation', () => {
  assert.deepEqual(getUploadAction('uploading'), { cancel: true, retry: false, cancelLabelKey: 'common.cancel', retryLabelKey: 'common.retry' });
  assert.deepEqual(getUploadAction('error'), { cancel: false, retry: true, cancelLabelKey: 'common.cancel', retryLabelKey: 'common.retry' });
  assert.equal(isUploadActionKey('Enter'), true);
  assert.equal(isUploadActionKey(' '), true);
  assert.equal(isUploadActionKey('Spacebar'), true);
  assert.equal(isUploadActionKey('Space'), true);
  assert.equal(isUploadActionKey('Escape'), false);
});

test('terminal upload state ignores stale progress and completion events', () => {
  const completed = applyUploadTaskEvent([task('u1', 'kb1', 100, 'success')], {
    type: 'progress', uploadId: 'u1', progress: 12,
  });
  assert.deepEqual(completed, [task('u1', 'kb1', 100, 'success')]);

  const cancelled = applyUploadTaskEvent([task('u2', 'kb1', 48, 'cancelled')], {
    type: 'complete', uploadId: 'u2', status: 'success', progress: 100,
  });
  assert.deepEqual(cancelled, [task('u2', 'kb1', 48, 'cancelled')]);
});

test('successful completion clears an earlier error and never leaves an uploading terminal state', () => {
  const retrying = applyUploadTaskEvent([{ ...task('u1', 'kb1', 40, 'error'), error: 'failed' }], {
    type: 'retry', uploadId: 'u1',
  });
  const recovered = applyUploadTaskEvent(retrying, {
    type: 'complete', uploadId: 'u1', status: 'success', progress: 100,
  });
  assert.deepEqual(recovered, [task('u1', 'kb1', 100, 'success')]);

  const malformed = applyUploadTaskEvent([task('u2', 'kb1', 20)], {
    type: 'complete', uploadId: 'u2', status: 'uploading', progress: 20,
  });
  assert.deepEqual(malformed, [task('u2', 'kb1', 20, 'success')]);
});

test('highlight target page is calculated for every filtered knowledge-base scope', () => {
  const items = Array.from({ length: 25 }, (_, index) => ({ id: `kb-${index + 1}` }));
  assert.equal(findUploadTargetPage(items, 'kb-13'), 2);
  assert.equal(findUploadTargetPage(items, 'missing'), null);
});

test('upload progress patches only the matching task', () => {
  const next = patchUploadTask([task('u1', 'kb1', 10), task('u2', 'kb1', 20)], 'u2', { status: 'success', progress: 100 });
  assert.deepEqual(next, [task('u1', 'kb1', 10), task('u2', 'kb1', 100, 'success')]);
});

test('progress and completion events update an existing task without repeating kbId', () => {
  const started = [task('u1', 'kb1', 10)];
  const progressing = applyUploadTaskEvent(started, { type: 'progress', uploadId: 'u1', progress: 42.4 });
  assert.deepEqual(progressing, [task('u1', 'kb1', 42)]);

  const completed = applyUploadTaskEvent(progressing, { type: 'complete', uploadId: 'u1', status: 'error', error: 'failed' });
  assert.deepEqual(completed, [{ ...task('u1', 'kb1', 100, 'error'), error: 'failed' }]);
});

test('cancel and retry events reuse the task identity without requiring kbId', () => {
  const started = [task('u1', 'kb1', 35)];
  const cancelled = applyUploadTaskEvent(started, { type: 'cancel', uploadId: 'u1' });
  assert.equal(cancelled[0]?.status, 'cancelled');
  const retried = applyUploadTaskEvent(cancelled, { type: 'retry', uploadId: 'u1' });
  assert.deepEqual(retried, [task('u1', 'kb1', 0)]);
});

test('upload summaries aggregate progress by knowledge base and preserve errors', () => {
  const summaries = summarizeUploadTasks([
    { ...task('u1', 'kb1', 100, 'success'), fileName: 'a.pdf' },
    { ...task('u2', 'kb1', 40, 'error'), fileName: 'b.pdf', error: 'failed' },
    task('u3', 'kb2', 20),
  ], (id) => id === 'kb1' ? 'Alpha' : 'Beta');
  assert.deepEqual(summaries, [
    { kbId: 'kb1', kbName: 'Alpha', total: 2, completed: 2, progress: 70, hasError: true, hasCancelled: false },
    { kbId: 'kb2', kbName: 'Beta', total: 1, completed: 0, progress: 20, hasError: false, hasCancelled: false },
  ]);
});

test('cancelled summaries are terminal but do not claim a successful upload', () => {
  const summaries = summarizeUploadTasks([
    task('u1', 'kb1', 20, 'cancelled'),
    task('u2', 'kb1', 100, 'success'),
  ], () => 'Alpha');
  assert.deepEqual(summaries, [{ kbId: 'kb1', kbName: 'Alpha', total: 2, completed: 2, progress: 60, hasError: false, hasCancelled: true }]);
});
