import assert from 'node:assert/strict';
import test from 'node:test';
import {
  UPLOAD_CLEANUP_DELAY_MS,
  UPLOAD_REFRESH_DELAY_MS,
  applyUploadTaskEvent,
  clampUploadProgress,
  createUploadTaskCleanups,
  dispatchUploadEvent,
  subscribeToUploadEvents,
  summarizeUploadTasks,
  type UploadEvent,
  type UploadTaskState,
} from './upload-progress.ts';

function task(uploadId: string, kbId: string, progress: number, status: UploadTaskState['status'] = 'uploading'): UploadTaskState {
  return { uploadId, kbId, progress, status };
}

test('upload events create, advance and settle a task like the Vue handlers', () => {
  let tasks: UploadTaskState[] = [];
  tasks = applyUploadTaskEvent(tasks, { type: 'start', uploadId: 'u1', kbId: 7, fileName: 'a.pdf' });
  assert.deepEqual(tasks, [{ uploadId: 'u1', kbId: '7', fileName: 'a.pdf', progress: 0, status: 'uploading' }]);
  tasks = applyUploadTaskEvent(tasks, { type: 'progress', uploadId: 'u1', progress: 41.6 });
  assert.equal(tasks[0].progress, 42);
  tasks = applyUploadTaskEvent(tasks, { type: 'complete', uploadId: 'u1' });
  assert.equal(tasks[0].status, 'success');
  assert.equal(tasks[0].progress, 100);
});

test('progress is clamped to 0-100 and rounded', () => {
  assert.equal(clampUploadProgress(-5), 0);
  assert.equal(clampUploadProgress(240), 100);
  assert.equal(clampUploadProgress(41.6), 42);
});

test('progress before start still registers when the event carries a kbId', () => {
  let tasks: UploadTaskState[] = [];
  tasks = applyUploadTaskEvent(tasks, { type: 'progress', uploadId: 'late', kbId: 'kb-9', progress: 10 });
  assert.equal(tasks.length, 1);
  assert.equal(tasks[0].kbId, 'kb-9');
  // Without a kbId the Vue handler drops the event entirely.
  tasks = applyUploadTaskEvent(tasks, { type: 'progress', uploadId: 'orphan', progress: 10 });
  assert.equal(tasks.length, 1);
});

test('start replaces a duplicate uploadId instead of duplicating it', () => {
  let tasks = applyUploadTaskEvent([], { type: 'start', uploadId: 'u1', kbId: 'kb' });
  tasks = applyUploadTaskEvent(tasks, { type: 'start', uploadId: 'u1', kbId: 'kb', progress: 30 });
  assert.equal(tasks.length, 1);
  assert.equal(tasks[0].progress, 30);
});

test('failed uploads settle as errors and keep their message', () => {
  let tasks: UploadTaskState[] = applyUploadTaskEvent([], { type: 'start', uploadId: 'u1', kbId: 'kb' });
  tasks = applyUploadTaskEvent(tasks, { type: 'complete', uploadId: 'u1', status: 'error', error: 'duplicate_file' });
  assert.equal(tasks[0].status, 'error');
  assert.equal(tasks[0].error, 'duplicate_file');
  assert.equal(tasks[0].progress, 100);
});

test('uploaded events never mutate the task list', () => {
  const tasks = applyUploadTaskEvent([task('u1', 'kb', 50)], { type: 'uploaded', kbId: 'kb' });
  assert.equal(tasks.length, 1);
});

test('summaries aggregate per KB with averaged progress and sorted names', () => {
  const summaries = summarizeUploadTasks([
    task('u1', 'kb-b', 100, 'success'),
    task('u2', 'kb-a', 40),
    task('u3', 'kb-a', 60, 'error'),
  ], (kbId) => (kbId === 'kb-a' ? 'Alpha KB' : 'Beta KB'));
  assert.deepEqual(summaries, [
    { kbId: 'kb-a', kbName: 'Alpha KB', total: 2, completed: 1, progress: 50, hasError: true },
    { kbId: 'kb-b', kbName: 'Beta KB', total: 1, completed: 1, progress: 100, hasError: false },
  ]);
});

test('unknown KB names fall back to the caller resolver', () => {
  const summaries = summarizeUploadTasks([task('u1', 'kb-x', 10)], () => 'fallback');
  assert.equal(summaries[0].kbName, 'fallback');
});

test('the event bus fans out to every subscriber and supports unsubscribe', () => {
  const seen: UploadEvent[] = [];
  const seenElsewhere: UploadEvent[] = [];
  const unsubscribe = subscribeToUploadEvents((event) => seen.push(event));
  const unsubscribeElsewhere = subscribeToUploadEvents((event) => seenElsewhere.push(event));
  dispatchUploadEvent({ type: 'uploaded', kbId: 'kb-1' });
  unsubscribeElsewhere();
  dispatchUploadEvent({ type: 'start', uploadId: 'u1', kbId: 'kb-1' });
  assert.equal(seen.length, 2);
  assert.equal(seenElsewhere.length, 1);
  unsubscribe();
});

test('completed tasks are removed after the Vue cleanup delay', () => {
  const removed: string[] = [];
  const scheduled: Array<{ handler: () => void; delayMs: number }> = [];
  const cleanups = createUploadTaskCleanups((uploadId) => removed.push(uploadId), (handler, delayMs) => {
    const entry: { handler: () => void; delayMs: number; fired?: boolean } = { handler: () => {}, delayMs };
    entry.handler = () => { const index = scheduled.indexOf(entry); if (index >= 0) scheduled.splice(index, 1); handler(); };
    scheduled.push(entry);
    return { cancel: () => { const index = scheduled.indexOf(entry); if (index >= 0) scheduled.splice(index, 1); } };
  });
  cleanups.schedule('u1');
  cleanups.schedule('u1'); // rescheduling replaces, never doubles
  assert.equal(scheduled.length, 1);
  assert.equal(scheduled[0].delayMs, UPLOAD_CLEANUP_DELAY_MS);
  assert.equal(UPLOAD_CLEANUP_DELAY_MS, 10_000);
  assert.equal(UPLOAD_REFRESH_DELAY_MS, 800);
  scheduled[0].handler();
  assert.deepEqual(removed, ['u1']);
  cleanups.schedule('u2');
  cleanups.clear();
  assert.equal(scheduled.length, 0);
});
