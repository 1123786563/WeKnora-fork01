import type { ScopeLease, TaskListPage } from '@weknora/mobile-core';
import type { ExistingTaskRow } from '../task-office/native-boundary/TaskEntryScreen.tsx';

const leaseIds = new WeakMap<object, number>();
let nextLeaseId = 1;

export function taskOfficeLifecycleKey(origin: string, tenantId: string, userId: string, lease: ScopeLease): string {
  let leaseId = leaseIds.get(lease);
  if (leaseId === undefined) { leaseId = nextLeaseId++; leaseIds.set(lease, leaseId); }
  return `${origin}:${tenantId}:${userId}:${leaseId}`;
}

export interface TaskOfficeListState {
  tasks: ExistingTaskRow[];
  loading: boolean;
  error?: string;
}

/** Owns the route's visible list lifecycle; revocation and superseded reads fail closed. */
export function createTaskOfficeListController(input: {
  openingLease: ScopeLease;
  read(): Promise<TaskListPage>;
  publish(state: TaskOfficeListState): void;
}) {
  let generation = 0;
  let active = false;
  let revoked = false;
  let state: TaskOfficeListState = { tasks: [], loading: true };
  const publish = (next: TaskOfficeListState) => {
    if (!active) return;
    state = next;
    input.publish(state);
  };
  const invalidate = () => {
    revoked = true;
    generation += 1;
    publish({ tasks: [], loading: false });
  };
  let unsubscribe: (() => void) | undefined;

  return {
    state: () => state,
    mount() {
      active = true;
      unsubscribe = input.openingLease.onRevoke?.(invalidate);
      return () => {
        active = false;
        generation += 1;
        unsubscribe?.();
        unsubscribe = undefined;
      };
    },
    async load(): Promise<void> {
      if (!active || revoked) return;
      const request = ++generation;
      publish({ ...state, loading: true, error: undefined });
      try {
        const page = await input.read();
        if (!active || request !== generation) return;
        publish({ tasks: page.items.map((card) => ({ taskId: card.taskId, runId: card.runId, title: card.title, runStatus: card.runStatus, updatedAt: card.updatedAt })), loading: false });
      } catch (failure) {
        if (!active || request !== generation) return;
        publish({ tasks: [], loading: false, error: failure instanceof Error ? failure.message : String(failure) });
      }
    },
    revoke: invalidate,
  };
}
