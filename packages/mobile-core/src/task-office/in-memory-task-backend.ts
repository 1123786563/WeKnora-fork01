import type { TaskBackendListInput, TaskBackendOverview, TaskBackendPage, TaskBackendPort } from './task-office.ts';

/** Scriptable scenario Adapter（module-seams §12：remote-owned 依赖的 in-memory 场景）。 */
export interface ScenarioTaskBackendHandlers {
  overview?: () => Promise<TaskBackendOverview>;
  list?: (input: TaskBackendListInput) => Promise<TaskBackendPage>;
  archive?: (taskId: string) => Promise<void>;
  restore?: (taskId: string) => Promise<void>;
}

export interface ScenarioTaskBackend extends TaskBackendPort {
  calls: Array<{ kind: 'overview' } | { kind: 'list'; input: TaskBackendListInput } | { kind: 'archive' | 'restore'; taskId: string }>;
}

export function emptyOverview(asOf = '2026-09-23T00:00:00Z'): TaskBackendOverview {
  return { needsMe: [], running: [], recentlyCompleted: [], unreadNotifications: 0, asOf };
}

export function createScenarioTaskBackend(handlers: ScenarioTaskBackendHandlers = {}): ScenarioTaskBackend {
  const calls: ScenarioTaskBackend['calls'] = [];
  const scenario: ScenarioTaskBackend = {
    calls,
    async overview() {
      calls.push({ kind: 'overview' });
      return handlers.overview ? handlers.overview() : emptyOverview();
    },
    async list(input) {
      calls.push({ kind: 'list', input });
      return handlers.list ? handlers.list(input) : { items: [] };
    },
    async archive(taskId) {
      calls.push({ kind: 'archive', taskId });
      await handlers.archive?.(taskId);
    },
    async restore(taskId) {
      calls.push({ kind: 'restore', taskId });
      await handlers.restore?.(taskId);
    },
  };
  return scenario;
}
