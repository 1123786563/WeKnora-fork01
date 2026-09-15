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
  | { type: 'progress'; uploadId: string; kbId?: string | number; progress: number }
  | { type: 'complete'; uploadId: string; kbId?: string | number; fileName?: string; progress?: number; status?: UploadTaskStatus; error?: string };

export function findUploadTargetPage<T extends { id: string }>(items: readonly T[], id: string, pageSize = 12): number | null {
  const index = items.findIndex((item) => String(item.id) === id);
  return index < 0 ? null : Math.floor(index / pageSize) + 1;
}

export function clampUploadProgress(value: number): number {
  return Math.min(100, Math.max(0, Math.round(value)));
}

export function upsertUploadTask(tasks: readonly UploadTaskState[], task: UploadTaskState): UploadTaskState[] {
  return [...tasks.filter((item) => item.uploadId !== task.uploadId), { ...task, progress: clampUploadProgress(task.progress) }];
}

export function patchUploadTask(tasks: readonly UploadTaskState[], uploadId: string, patch: Partial<UploadTaskState>): UploadTaskState[] {
  return tasks.map((task) => task.uploadId === uploadId ? { ...task, ...patch, progress: patch.progress === undefined ? task.progress : clampUploadProgress(patch.progress) } : task);
}

/**
 * Applies the Vue upload-mask event contract. Existing tasks are keyed by
 * uploadId, so progress/complete events do not need to repeat kbId.
 */
export function applyUploadTaskEvent(tasks: readonly UploadTaskState[], event: UploadEvent): UploadTaskState[] {
  if (!event.uploadId) return [...tasks];

  if (event.type === 'start') {
    if (!event.kbId) return [...tasks];
    return upsertUploadTask(tasks, {
      uploadId: event.uploadId,
      kbId: String(event.kbId),
      fileName: event.fileName,
      progress: typeof event.progress === 'number' ? event.progress : 0,
      status: 'uploading',
    });
  }

  const existing = tasks.some((task) => task.uploadId === event.uploadId);
  if (existing) {
    return patchUploadTask(tasks, event.uploadId, event.type === 'progress'
      ? { progress: event.progress }
      : {
          status: event.status ?? 'success',
          progress: typeof event.progress === 'number' ? event.progress : 100,
          error: event.error,
        });
  }

  if (event.kbId === undefined || event.kbId === null || event.kbId === '') return [...tasks];
  return upsertUploadTask(tasks, {
    uploadId: event.uploadId,
    kbId: String(event.kbId),
    fileName: event.type === 'complete' ? event.fileName : undefined,
    progress: typeof event.progress === 'number' ? event.progress : 0,
    status: event.type === 'complete' ? event.status ?? 'success' : 'uploading',
    error: event.type === 'complete' ? event.error : undefined,
  });
}

export function summarizeUploadTasks(tasks: readonly UploadTaskState[], getName: (kbId: string) => string): UploadSummary[] {
  const grouped = new Map<string, UploadTaskState[]>();
  for (const task of tasks) grouped.set(task.kbId, [...(grouped.get(task.kbId) ?? []), task]);
  return [...grouped.entries()].map(([kbId, items]) => ({
    kbId,
    kbName: getName(kbId),
    total: items.length,
    completed: items.filter((item) => item.status !== 'uploading').length,
    progress: clampUploadProgress(items.reduce((sum, item) => sum + item.progress, 0) / items.length),
    hasError: items.some((item) => item.status === 'error'),
  })).sort((left, right) => left.kbName.localeCompare(right.kbName));
}
