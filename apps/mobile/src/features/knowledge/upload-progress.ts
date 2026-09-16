// Upload progress surface for the mobile knowledge screens — a port of the
// Vue KnowledgeBaseList.vue upload-progress panel (lines 37-69 template,
// 879-881/1200-1210 state, 1576-1745 event handlers).
//
// Vue communicates between the KB detail page (uploader) and the KB list page
// (mask) through four window events: knowledgeFileUploadStart,
// knowledgeFileUploadProgress, knowledgeFileUploadComplete and
// knowledgeFileUploaded. React Native has no window; this module's module-level
// event bus is the equivalent channel, and the pure reducer/summarizer below
// mirror the Vue handler logic exactly (including the 10s completed-task
// cleanup and the 800ms debounced list refresh).

export type UploadTaskStatus = 'uploading' | 'success' | 'error';

export interface UploadTaskState {
  uploadId: string;
  kbId: string;
  fileName?: string;
  progress: number;
  status: UploadTaskStatus;
  error?: string;
}

export interface UploadSummary {
  kbId: string;
  kbName: string;
  total: number;
  completed: number;
  progress: number;
  hasError: boolean;
}

export type UploadEvent =
  | { type: 'start'; uploadId: string; kbId: string | number; fileName?: string; progress?: number }
  | { type: 'progress'; uploadId: string; kbId?: string | number; fileName?: string; progress: number }
  | { type: 'complete'; uploadId: string; kbId?: string | number; fileName?: string; progress?: number; status?: UploadTaskStatus; error?: string }
  | { type: 'uploaded'; kbId: string | number };

/** Vue UPLOAD_CLEANUP_DELAY (KnowledgeBaseList.vue:882). */
export const UPLOAD_CLEANUP_DELAY_MS = 10_000;
/** Vue upload-refresh debounce (handleUploadFinishedEvent). */
export const UPLOAD_REFRESH_DELAY_MS = 800;

export function clampUploadProgress(value: number): number {
  return Math.min(100, Math.max(0, Math.round(value)));
}

function ensureUploadTaskEntry(tasks: UploadTaskState[], detail: { uploadId: string; kbId?: string | number; fileName?: string; progress?: number; status?: UploadTaskStatus; error?: string }): UploadTaskState[] {
  if (!detail.uploadId) return tasks;
  if (tasks.some((task) => task.uploadId === detail.uploadId)) return tasks;
  // Vue ensureUploadTaskEntry returns null when a progress/complete event
  // arrives before start without a kbId to key the new row.
  if (detail.kbId === undefined || detail.kbId === null || detail.kbId === '') return tasks;
  return [...tasks, {
    uploadId: detail.uploadId,
    kbId: String(detail.kbId),
    fileName: detail.fileName,
    progress: typeof detail.progress === 'number' ? clampUploadProgress(detail.progress) : 0,
    status: detail.status ?? 'uploading',
    error: detail.error,
  }];
}

/** Pure port of handleUploadStartEvent / handleUploadProgressEvent /
 * handleUploadCompleteEvent. `uploaded` only triggers the debounced list
 * refresh in the subscriber and never mutates the task list. */
export function applyUploadTaskEvent(tasks: UploadTaskState[], event: UploadEvent): UploadTaskState[] {
  if (event.type === 'start') {
    if (!event.uploadId || !event.kbId) return tasks;
    return [
      ...tasks.filter((task) => task.uploadId !== event.uploadId),
      { uploadId: event.uploadId, kbId: String(event.kbId), fileName: event.fileName, progress: typeof event.progress === 'number' ? clampUploadProgress(event.progress) : 0, status: 'uploading' },
    ];
  }
  if (event.type === 'progress') {
    if (!event.uploadId || typeof event.progress !== 'number') return tasks;
    const next = ensureUploadTaskEntry(tasks, event);
    return next.map((task) => task.uploadId === event.uploadId ? { ...task, progress: clampUploadProgress(event.progress) } : task);
  }
  if (event.type === 'complete') {
    if (!event.uploadId) return tasks;
    const progress = typeof event.progress === 'number' ? clampUploadProgress(event.progress) : 100;
    const next = ensureUploadTaskEntry(tasks, { ...event, progress });
    return next.map((task) => task.uploadId === event.uploadId ? { ...task, status: event.status ?? 'success', progress, error: event.error } : task);
  }
  return tasks;
}

/** Port of the uploadSummaries computed: group per KB, completed = settled
 * tasks, progress = clamped average, sorted by KB display name. */
export function summarizeUploadTasks(tasks: UploadTaskState[], resolveKbName: (kbId: string) => string): UploadSummary[] {
  if (!tasks.length) return [];
  const grouped = new Map<string, UploadTaskState[]>();
  for (const task of tasks) {
    const bucket = grouped.get(task.kbId);
    if (bucket) bucket.push(task);
    else grouped.set(task.kbId, [task]);
  }
  return [...grouped.entries()].map(([kbId, kbTasks]) => {
    const total = kbTasks.length;
    const completed = kbTasks.filter((task) => task.status !== 'uploading').length;
    const progressSum = kbTasks.reduce((sum, task) => sum + (task.progress ?? 0), 0);
    return {
      kbId,
      kbName: resolveKbName(kbId),
      total,
      completed,
      progress: total === 0 ? 0 : clampUploadProgress(progressSum / total),
      hasError: kbTasks.some((task) => task.status === 'error'),
    };
  }).sort((a, b) => a.kbName.localeCompare(b.kbName));
}

type UploadEventListener = (event: UploadEvent) => void;

const uploadEventListeners = new Set<UploadEventListener>();

/** Mobile equivalent of the four window events the Vue pages use. */
export function dispatchUploadEvent(event: UploadEvent): void {
  for (const listener of [...uploadEventListeners]) listener(event);
}

export function subscribeToUploadEvents(listener: UploadEventListener): () => void {
  uploadEventListeners.add(listener);
  return () => { uploadEventListeners.delete(listener); };
}

/** Vue removeUploadTask + scheduleUploadTaskCleanup, with the timer factory
 * injected so tests can run without real waits. */
export function createUploadTaskCleanups(remove: (uploadId: string) => void, schedule: (handler: () => void, delayMs: number) => { cancel(): void } = (handler, delayMs) => {
  const timer = setTimeout(handler, delayMs);
  return { cancel: () => clearTimeout(timer) };
}): { schedule(uploadId: string): void; cancel(uploadId: string): void; clear(): void } {
  const timers = new Map<string, { cancel(): void }>();
  return {
    schedule(uploadId: string) {
      timers.get(uploadId)?.cancel();
      timers.set(uploadId, schedule(() => { timers.delete(uploadId); remove(uploadId); }, UPLOAD_CLEANUP_DELAY_MS));
    },
    cancel(uploadId: string) {
      timers.get(uploadId)?.cancel();
      timers.delete(uploadId);
    },
    clear() {
      for (const timer of timers.values()) timer.cancel();
      timers.clear();
    },
  };
}
