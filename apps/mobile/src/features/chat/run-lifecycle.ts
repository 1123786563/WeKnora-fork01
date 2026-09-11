export type RunLifecycleStatus =
  | 'idle'
  | 'running'
  | 'background-interrupted'
  | 'stopped'
  | 'failed'
  | 'completed';

export interface RunLifecycle {
  status: RunLifecycleStatus;
  assistantMessageId?: string;
}

export interface RunLifecycleStore {
  getItemAsync(key: string): Promise<string | null>;
  setItemAsync(key: string, value: string): Promise<void>;
}

export type RunLifecycleEvent =
  | { type: 'start'; assistantMessageId?: string }
  | { type: 'assistant-message'; id: string }
  | { type: 'background' }
  | { type: 'user-stop' }
  | { type: 'failure' }
  | { type: 'complete' };

const RUN_LIFECYCLE_STATUSES: readonly RunLifecycleStatus[] = [
  'idle',
  'running',
  'background-interrupted',
  'stopped',
  'failed',
  'completed',
];

export function initialRunLifecycle(): RunLifecycle {
  return { status: 'idle' };
}

export function transitionRunLifecycle(state: RunLifecycle, event: RunLifecycleEvent): RunLifecycle {
  if ((state.status === 'stopped' || state.status === 'failed' || state.status === 'completed')
    && event.type !== 'start' && event.type !== 'assistant-message') {
    return state;
  }
  switch (event.type) {
    case 'start':
      return { status: 'running', ...(event.assistantMessageId ? { assistantMessageId: event.assistantMessageId } : {}) };
    case 'assistant-message':
      return { ...state, assistantMessageId: event.id };
    case 'background':
      return state.status === 'running' ? { ...state, status: 'background-interrupted' } : state;
    case 'user-stop':
      return { ...state, status: 'stopped' };
    case 'failure':
      return { ...state, status: 'failed' };
    case 'complete':
      return { ...state, status: 'completed' };
  }
}

export function canAutoResumeRun(state: RunLifecycle): boolean {
  return state.status === 'idle' || state.status === 'background-interrupted';
}

export function serializeRunLifecycle(state: RunLifecycle): string {
  return JSON.stringify(state);
}

export function deserializeRunLifecycle(value: string | null | undefined): RunLifecycle {
  if (!value) return initialRunLifecycle();
  try {
    const parsed: unknown = JSON.parse(value);
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) return initialRunLifecycle();
    const record = parsed as Record<string, unknown>;
    if (!RUN_LIFECYCLE_STATUSES.includes(record.status as RunLifecycleStatus)) return initialRunLifecycle();
    return {
      status: record.status as RunLifecycleStatus,
      ...(typeof record.assistantMessageId === 'string' && record.assistantMessageId.trim() !== ''
        ? { assistantMessageId: record.assistantMessageId }
        : {}),
    };
  } catch {
    return initialRunLifecycle();
  }
}

export function createRunLifecyclePersistence(store: RunLifecycleStore) {
  const keyFor = (sessionId: string) => `weknora.mobile.chat.run-lifecycle.${sessionId}`;
  return {
    read(sessionId: string): Promise<RunLifecycle> {
      return store.getItemAsync(keyFor(sessionId)).then(deserializeRunLifecycle);
    },
    write(sessionId: string, state: RunLifecycle): Promise<void> {
      return store.setItemAsync(keyFor(sessionId), serializeRunLifecycle(state));
    },
  };
}
