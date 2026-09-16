export type UploadTaskStatus = 'uploading' | 'success' | 'error' | 'cancelled';

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
  hasCancelled: boolean;
}

export interface UploadActionState {
  cancel: boolean;
  retry: boolean;
  cancelLabelKey: 'common.cancel';
  retryLabelKey: 'common.retry';
}

export type UploadEvent =
  | { type: 'start'; uploadId: string; kbId: string | number; fileName?: string; progress?: number }
  | { type: 'progress'; uploadId: string; kbId?: string | number; progress: number }
  | { type: 'cancel'; uploadId: string; kbId?: string | number }
  | { type: 'retry'; uploadId: string; kbId?: string | number }
  | { type: 'complete'; uploadId: string; kbId?: string | number; fileName?: string; progress?: number; status?: UploadTaskStatus; error?: string };

export function findUploadTargetPage<T extends { id: string }>(items: readonly T[], id: string, pageSize = 12): number | null {
  const index = items.findIndex((item) => String(item.id) === id);
  return index < 0 ? null : Math.floor(index / pageSize) + 1;
}

export function clampUploadProgress(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.min(100, Math.max(0, Math.round(value)));
}

export function upsertUploadTask(tasks: readonly UploadTaskState[], task: UploadTaskState): UploadTaskState[] {
  return [...tasks.filter((item) => item.uploadId !== task.uploadId), { ...task, progress: clampUploadProgress(task.progress) }];
}

export function patchUploadTask(tasks: readonly UploadTaskState[], uploadId: string, patch: Partial<UploadTaskState>): UploadTaskState[] {
  return tasks.map((task) => task.uploadId === uploadId ? { ...task, ...patch, progress: patch.progress === undefined ? task.progress : clampUploadProgress(patch.progress) } : task);
}

function hasUploadKnowledgeBaseId(kbId: string | number | undefined | null): boolean {
  // KnowledgeBaseList.vue guards upload events with `!detail.kbId`.
  // Keep the same contract for numeric IDs as well as empty strings.
  return Boolean(kbId);
}

function terminalStatus(status: UploadTaskStatus | undefined): Exclude<UploadTaskStatus, 'uploading'> {
  return status === 'error' || status === 'cancelled' ? status : 'success';
}

export function cancelUploadTask(tasks: readonly UploadTaskState[], uploadId: string): UploadTaskState[] {
  const task = tasks.find((item) => item.uploadId === uploadId);
  if (!task || task.status !== 'uploading') return [...tasks];
  return patchUploadTask(tasks, uploadId, { status: 'cancelled' });
}

export function retryUploadTask(tasks: readonly UploadTaskState[], uploadId: string): UploadTaskState[] {
  const task = tasks.find((item) => item.uploadId === uploadId);
  if (!task || (task.status !== 'error' && task.status !== 'cancelled')) return [...tasks];
  return tasks.map((item) => {
    if (item.uploadId !== uploadId) return item;
    const next = { ...item, status: 'uploading' as const, progress: 0 };
    delete next.error;
    return next;
  });
}

/** The page supplies these keys to the shared translator; never hard-code UI copy here. */
export function getUploadAction(status: UploadTaskStatus): UploadActionState {
  return {
    cancel: status === 'uploading',
    retry: status === 'error' || status === 'cancelled',
    cancelLabelKey: 'common.cancel',
    retryLabelKey: 'common.retry',
  };
}

/** Native buttons already handle these keys; custom upload affordances can use this guard. */
export function isUploadActionKey(key: string): boolean {
  return key === 'Enter' || key === ' ' || key === 'Space' || key === 'Spacebar';
}

/**
 * Applies the Vue upload-mask event contract. Existing tasks are keyed by
 * uploadId, so progress/complete events do not need to repeat kbId.
 */
export function applyUploadTaskEvent(tasks: readonly UploadTaskState[], event: UploadEvent): UploadTaskState[] {
  if (!event.uploadId) return [...tasks];

  if (event.type === 'start') {
    if (!hasUploadKnowledgeBaseId(event.kbId)) return [...tasks];
    const next: UploadTaskState = {
      uploadId: event.uploadId,
      kbId: String(event.kbId),
      progress: typeof event.progress === 'number' ? event.progress : 0,
      status: 'uploading',
    };
    if (event.fileName !== undefined) next.fileName = event.fileName;
    return upsertUploadTask(tasks, next);
  }

  const existing = tasks.some((task) => task.uploadId === event.uploadId);
  if (existing) {
    if (event.type === 'cancel') return cancelUploadTask(tasks, event.uploadId);
    if (event.type === 'retry') return retryUploadTask(tasks, event.uploadId);
    const current = tasks.find((task) => task.uploadId === event.uploadId)!;
    if (current.status !== 'uploading') return [...tasks];
    if (event.type === 'progress') return patchUploadTask(tasks, event.uploadId, { progress: event.progress });

    const status = terminalStatus(event.status);
    const progress = typeof event.progress === 'number' ? event.progress : 100;
    return tasks.map((task) => {
      if (task.uploadId !== event.uploadId) return task;
      const next = { ...task, status, progress: clampUploadProgress(progress) };
      if (event.error !== undefined) next.error = event.error;
      else if (status !== 'error') delete next.error;
      return next;
    });
  }

  if (!hasUploadKnowledgeBaseId(event.kbId)) return [...tasks];
  if (event.type !== 'progress' && event.type !== 'complete') return [...tasks];
  const next: UploadTaskState = {
    uploadId: event.uploadId,
    kbId: String(event.kbId),
    progress: typeof event.progress === 'number' ? event.progress : 0,
    status: event.type === 'complete' ? event.status ?? 'success' : 'uploading',
  };
  if (event.type === 'complete') {
    next.status = terminalStatus(event.status);
    if (event.fileName !== undefined) next.fileName = event.fileName;
    if (event.error !== undefined) next.error = event.error;
  }
  return upsertUploadTask(tasks, next);
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
    hasCancelled: items.some((item) => item.status === 'cancelled'),
  })).sort((left, right) => left.kbName.localeCompare(right.kbName));
}
