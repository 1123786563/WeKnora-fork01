// W04 craft workbench controller.
//
// Owns the workbench's live projection of one craft session: it loads the
// authoritative workspace snapshot first and only then subscribes to the
// run-event stream after the snapshot's last_seq, merges events by run+seq
// through the pure domain state machine, and keeps user intent (the submit
// idempotency key) strictly separate from transport health: stream failures
// reconnect the subscription, they NEVER re-submit the prompt. Main-run
// terminal state comes only from the Run status projection — child events
// never finish the main message. Seq gaps and expired cursors stop the stream
// with an explicit CraftSyncError so the C03 reload flow can take over.
import {
  CRAFT_TERMINAL_RUN_STATUSES,
  parseCraftEventPayload,
  parseCraftRunEvent,
  parseCraftRunView,
  type CraftEventPayloadView,
  type CraftRunView,
  type CraftWorkspaceView,
} from '@weknora/contracts';
import type { CraftApi } from '@weknora/api-client';
import { applyCraftEvent, type CraftState } from '@weknora/domain/craft/state';
import type { ScopeController } from '@weknora/domain/scope';

/** One parsed SSE frame (same shape api-client's createServerSentEventParser emits). */
export interface CraftEventFrame {
  id?: string;
  event?: string;
  data: string;
}

/**
 * Adapter over the EXISTING run-event stream (GET
 * /api/v1/sessions/:session_id/runs/:run_id/events with Last-Event-ID/after):
 * the web layer implements it with the current request scope + SSE parser
 * (R05 pattern); W04 injects it, it does not invent a new network stack.
 * Resolves when the stream ends; rejects on transport failure.
 */
export interface CraftSubscribeInput {
  sessionId: string;
  runId: string;
  /** Resume after this server-assigned RunEvent seq (snapshot last_seq). */
  after: number;
  signal: AbortSignal;
  onEvent(frame: CraftEventFrame): void;
}

export interface CraftEventTransport {
  subscribe(input: CraftSubscribeInput): Promise<void>;
}

export type CraftSyncErrorCode = 'SEQ_GAP' | 'CURSOR_EXPIRED' | 'STREAM_FAILED';

/** Explicit reload entry for the C03 snapshot/reconnect flow — never skipped. */
export class CraftSyncError extends Error {
  readonly code: CraftSyncErrorCode;
  readonly expectedSeq?: number;
  readonly receivedSeq?: number;

  constructor(code: CraftSyncErrorCode, message: string, details: { expectedSeq?: number; receivedSeq?: number } = {}) {
    super(message);
    this.name = 'CraftSyncError';
    this.code = code;
    this.expectedSeq = details.expectedSeq;
    this.receivedSeq = details.receivedSeq;
  }
}

export interface CraftControllerOptions {
  api: CraftApi;
  scope: ScopeController;
  events: CraftEventTransport;
  onError?(error: CraftSyncError): void;
  /** Injectable idempotency-key source; one key per user send. */
  newRequestId?(): string;
}

export interface CraftWorkbenchController {
  /** Fetches the snapshot, then subscribes after last_seq. Refresh-safe. */
  load(sessionId: string): Promise<void>;
  /** One user send: generates the request id once, retries reuse it. */
  submit(prompt: string): Promise<void>;
  dispose(): void;
  state(): CraftState | null;
  lastError(): CraftSyncError | null;
  onChange(listener: (state: CraftState | null) => void): () => void;
}

function isTerminal(status: string): boolean {
  return (CRAFT_TERMINAL_RUN_STATUSES as readonly string[]).includes(status);
}

let requestCounter = 0;
function defaultRequestId(): string {
  const uuid = globalThis.crypto?.randomUUID?.();
  if (uuid) return uuid;
  requestCounter += 1;
  return 'craft-' + Date.now().toString(36) + '-' + requestCounter.toString(36);
}

function publishedVersionId(payload: CraftEventPayloadView): string | undefined {
  if (payload.kind !== 'artifact.published') return undefined;
  const version = payload.data['version_id'];
  return typeof version === 'string' && version.trim() !== '' ? version : undefined;
}

function parseFrameData(frame: CraftEventFrame): unknown {
  return JSON.parse(frame.data);
}

interface ActiveSubscription {
  abort: AbortController;
  runId: string;
  generation: number;
  detachScopeAbort(): void;
}

export function createCraftWorkbenchController(options: CraftControllerOptions): CraftWorkbenchController {
  const { api, scope, events } = options;
  const newRequestId = options.newRequestId ?? defaultRequestId;
  const listeners = new Set<(state: CraftState | null) => void>();

  let currentState: CraftState | null = null;
  let syncError: CraftSyncError | null = null;
  let subscription: ActiveSubscription | null = null;
  let sessionId: string | null = null;
  let pendingSubmit: { prompt: string; requestId: string } | null = null;
  let disposed = false;

  function notify(next: CraftState | null): void {
    currentState = next;
    for (const listener of listeners) listener(next);
  }

  function destroySubscription(): void {
    if (subscription === null) return;
    subscription.detachScopeAbort();
    subscription.abort.abort();
    subscription = null;
  }

  function resetForScopeChange(): void {
    destroySubscription();
    pendingSubmit = null;
    notify(null);
  }

  function fail(error: CraftSyncError): void {
    destroySubscription();
    syncError = error;
    options.onError?.(error);
  }

  function buildStateFromSnapshot(view: CraftWorkspaceView, generation: number): CraftState {
    const previous = currentState;
    const runId = view.active_run_id;
    const mainStatus = view.active_run !== null ? view.active_run.status : runId === null ? 'idle' : 'queued';
    const sameRun = previous !== null && previous.runId === runId;
    return {
      generation,
      runId,
      // Never regress the cursor: applied events can outrun the snapshot.
      seq: sameRun ? Math.max(view.last_seq, previous.seq) : view.last_seq,
      mainStatus,
      delegationStatus: sameRun ? previous.delegationStatus : 'idle',
      versionId: view.current_version !== null ? view.current_version.id : null,
    };
  }

  async function refreshSnapshot(id: string, generation: number): Promise<CraftWorkspaceView> {
    const handle = scope.current();
    if (handle.scope.generation !== generation) throw new CraftSyncError('STREAM_FAILED', 'scope changed during refresh');
    const view = await api.get(id, handle.signal);
    if (disposed || !scope.isCurrent(generation)) throw new CraftSyncError('STREAM_FAILED', 'scope changed during refresh');
    return view;
  }

  function startSubscription(id: string, runId: string, after: number, generation: number): void {
    destroySubscription();
    const abort = new AbortController();
    const handle = scope.current();
    const onScopeAbort = (): void => resetForScopeChange();
    if (handle.scope.generation === generation) {
      if (handle.signal.aborted) {
        resetForScopeChange();
        return;
      }
      handle.signal.addEventListener('abort', onScopeAbort, { once: true });
    }
    const active: ActiveSubscription = {
      abort,
      runId,
      generation,
      detachScopeAbort(): void {
        if (handle.scope.generation === generation) handle.signal.removeEventListener('abort', onScopeAbort);
      },
    };
    subscription = active;

    const pump = async (): Promise<void> => {
      try {
        await events.subscribe({
          sessionId: id,
          runId,
          after,
          signal: abort.signal,
          onEvent: (frame) => onFrame(frame, active),
        });
      } catch {
        // Transport failure: fall through — reconnect the subscription only.
      }
      if (disposed || abort.signal.aborted) return;
      if (!scope.isCurrent(generation)) {
        resetForScopeChange();
        return;
      }
      // The stream ended (server closes it once the run is terminal). The
      // snapshot is the authority for whether the run is still live; C03
      // replaces this immediate refresh with backoff + replay.
      try {
        const view = await refreshSnapshot(id, generation);
        notify(buildStateFromSnapshot(view, generation));
      } catch {
        fail(new CraftSyncError('STREAM_FAILED', 'could not refresh the workspace snapshot after the stream ended'));
        return;
      }
      const state = currentState;
      if (disposed || !scope.isCurrent(generation)) return;
      if (state === null || state.runId === null || isTerminal(state.mainStatus)) return;
      // Another path (refresh load/submit/scope reset) replaced the stream.
      if (subscription !== active) return;
      startSubscription(id, state.runId, state.seq, generation);
    };
    void pump();
  }

  function applyRunProjection(run: CraftRunView, active: ActiveSubscription): void {
    const state = currentState;
    if (state === null || state.runId !== run.run_id) return;
    notify({ ...state, mainStatus: run.status });
    if (isTerminal(run.status)) destroySubscription();
  }

  function onFrame(frame: CraftEventFrame, active: ActiveSubscription): void {
    if (disposed) return;
    if (!scope.isCurrent(active.generation)) {
      resetForScopeChange();
      return;
    }
    const state = currentState;
    if (state === null || state.runId !== active.runId) return;
    try {
      if (frame.event === 'keepalive') return;
      if (frame.event === 'error') {
        const value = parseFrameData(frame);
        const code = typeof value === 'object' && value !== null && !Array.isArray(value)
          ? (value as Record<string, unknown>)['code']
          : undefined;
        if (code === 'cursor_expired') {
          fail(new CraftSyncError('CURSOR_EXPIRED', 'run event cursor expired; a snapshot reload is required'));
        }
        // Other stream errors end the stream server-side; pump reconnects.
        return;
      }
      if (frame.event === 'run') {
        applyRunProjection(parseCraftRunView(parseFrameData(frame)), active);
        return;
      }
      if (frame.event === undefined || /^[0-9]+$/.test(frame.event)) {
        const wire = parseCraftRunEvent(parseFrameData(frame));
        if (wire.seq <= state.seq) return; // duplicate replay (refresh/reconnect)
        if (wire.seq > state.seq + 1) {
          fail(new CraftSyncError('SEQ_GAP', 'run event gap detected; a snapshot reload is required', {
            expectedSeq: state.seq + 1,
            receivedSeq: wire.seq,
          }));
          return;
        }
        const payload = parseCraftEventPayload(wire.payload);
        if (payload === null) {
          // Foreign run events still advance the server cursor — skipping
          // them would fabricate a gap on the next craft event.
          notify({ ...state, seq: wire.seq });
          return;
        }
        notify(applyCraftEvent(state, {
          generation: active.generation,
          runId: active.runId,
          seq: wire.seq,
          kind: payload.kind,
          ...(publishedVersionId(payload) === undefined ? {} : { versionId: publishedVersionId(payload) }),
        }));
      }
      // Unknown event names are future protocol: ignored without touching state.
    } catch (error) {
      // Contract parsers reject unknown statuses/kinds and illegal seqs; a
      // malformed frame stops the stream instead of poisoning the state.
      fail(new CraftSyncError('STREAM_FAILED', 'malformed run event rejected by the contract parsers'));
      if (error instanceof Error) console.error('[craft] contract violation:', error.message);
    }
  }

  return {
    async load(id: string): Promise<void> {
      if (disposed) throw new Error('controller is disposed');
      sessionId = id;
      const handle = scope.current();
      const view = await api.get(id, handle.signal);
      if (disposed) return;
      if (!scope.isCurrent(handle.scope.generation)) {
        // A concurrent scope switch already reset the workbench.
        return;
      }
      syncError = null;
      destroySubscription();
      notify(buildStateFromSnapshot(view, handle.scope.generation));
      const state = currentState;
      if (state !== null && state.runId !== null && !isTerminal(state.mainStatus)) {
        startSubscription(id, state.runId, state.seq, state.generation);
      }
    },

    async submit(prompt: string): Promise<void> {
      if (disposed) throw new Error('controller is disposed');
      if (sessionId === null) throw new Error('load the craft workspace before submitting');
      const handle = scope.current();
      const generation = handle.scope.generation;
      // One request id per user send: a retry of the SAME send reuses the key
      // so the server replays the original admission instead of starting two.
      const requestId = pendingSubmit !== null && pendingSubmit.prompt === prompt
        ? pendingSubmit.requestId
        : newRequestId();
      pendingSubmit = { prompt, requestId };
      try {
        const run = await api.submit(sessionId, { request_id: requestId, prompt }, handle.signal);
        if (disposed || !scope.isCurrent(generation)) {
          // The server holds the admission under this key; the next load in
          // the new scope recovers it. Never re-submit from here.
          return;
        }
        pendingSubmit = null;
        syncError = null;
        destroySubscription();
        notify({
          generation,
          runId: run.run_id,
          seq: run.seq,
          mainStatus: run.status,
          delegationStatus: 'idle',
          versionId: currentState?.versionId ?? null,
        });
        if (!isTerminal(run.status)) {
          startSubscription(sessionId, run.run_id, run.seq, generation);
        }
      } catch (error) {
        if (!scope.isCurrent(generation)) pendingSubmit = null;
        throw error;
      }
    },

    dispose(): void {
      disposed = true;
      destroySubscription();
      pendingSubmit = null;
    },

    state(): CraftState | null {
      return currentState;
    },

    lastError(): CraftSyncError | null {
      return syncError;
    },

    onChange(listener: (state: CraftState | null) => void): () => void {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}
