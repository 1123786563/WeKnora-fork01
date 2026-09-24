import type { TaskBackendListInput, TaskBackendLookup, TaskBackendOverview, TaskBackendPage, TaskBackendPort, TaskBackendStartAck, TaskBackendStartInput } from './task-office.ts';

/** Scriptable scenario Adapter（module-seams §12：remote-owned 依赖的 in-memory 场景）。 */
export interface ScenarioTaskBackendHandlers {
  overview?: () => Promise<TaskBackendOverview>;
  list?: (input: TaskBackendListInput) => Promise<TaskBackendPage>;
  archive?: (taskId: string) => Promise<void>;
  restore?: (taskId: string) => Promise<void>;
  createSession?: (input: { title: string }) => Promise<{ sessionId: string }>;
  start?: (input: TaskBackendStartInput) => Promise<TaskBackendStartAck>;
  lookup?: (requestId: string) => Promise<TaskBackendLookup>;
}

export interface ScenarioTaskBackend extends TaskBackendPort {
  calls: Array<
    | { kind: 'overview' }
    | { kind: 'list'; input: TaskBackendListInput }
    | { kind: 'archive' | 'restore'; taskId: string }
    | { kind: 'createSession'; title: string }
    | { kind: 'start'; input: TaskBackendStartInput }
    | { kind: 'lookup'; requestId: string }
  >;
}

export function emptyOverview(asOf = '2026-09-23T00:00:00Z'): TaskBackendOverview {
  return { needsMe: [], running: [], recentlyCompleted: [], unreadNotifications: 0, asOf };
}

export function createScenarioTaskBackend(handlers: ScenarioTaskBackendHandlers = {}): ScenarioTaskBackend {
  const calls: ScenarioTaskBackend['calls'] = [];
  let sessionSeq = 0;
  let runSeq = 0;
  const admitted = new Map<string, TaskBackendStartAck>();
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
    async createSession(input) {
      calls.push({ kind: 'createSession', title: input.title });
      return handlers.createSession ? handlers.createSession(input) : { sessionId: `session-scenario-${sessionSeq += 1}` };
    },
    async start(input) {
      calls.push({ kind: 'start', input });
      const ack = handlers.start ? await handlers.start(input) : { run_id: `run-scenario-${runSeq += 1}`, request_id: input.request_id, status: 'queued' };
      admitted.set(input.request_id, ack);
      return ack;
    },
    async lookup(requestId) {
      calls.push({ kind: 'lookup', requestId });
      if (handlers.lookup) return handlers.lookup(requestId);
      const ack = admitted.get(requestId);
      return ack ? { state: 'admitted', run_id: ack.run_id } : { state: 'unknown' };
    },
  };
  return scenario;
}
