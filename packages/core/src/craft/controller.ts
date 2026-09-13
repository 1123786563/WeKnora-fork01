// W04 craft workbench controller + C03 snapshot reconnection.
//
// Owns the workbench's live projection of one craft session: it loads the
// authoritative workspace snapshot first and only then subscribes to the
// run-event stream after the snapshot's last_seq, merges events by run+seq
// through the pure domain state machine, and keeps user intent (the submit
// idempotency key) strictly separate from transport health: stream failures
// reconnect the subscription, they NEVER re-submit the prompt. Main-run
// terminal state comes only from the Run status projection — child events
// never finish the main message.
//
// C03 additions:
//   * every incoming seq goes through replayAction: duplicates drop, the
//     exact next seq applies, and REAL GAPS (plus expired cursors, ended or
//     failed streams) reload the AUTHORITATIVE snapshot — gaps are never
//     bridged by guessing;
//   * reconnects back off 1/2/4/8/15s + jitter (capped at 15s, injectable
//     for tests); dispose and scope switches terminate pending timers and
//     in-flight requests at once;
//   * snapshot replacement is ATOMIC per generation: event-won projections
//     survive only when they are ahead of the snapshot watermark (same
//     generation AND same run); anything older is replaced wholesale;
//   * every async continuation carries the sync epoch captured at its start,
//     so a superseded cycle (refresh, dispose, scope switch) can never write
//     state — scope switches never surface as fake STREAM_FAILED;
//   * onSync announces 'syncing' -> 'synced' (or 'failed') so the UI can show
//     a syncing state that CLEARS on recovery; onError stays for permanent
//     (non-retried) failures only.
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
import { reconnectDelayMs, replayAction } from '@weknora/domain/craft/reconnect';
import type { ScopeController, ScopeHandle } from '@weknora/domain/scope';

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

/** Explicit reload entry — never skipped. */
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

/** Sync lifecycle the UI projects (C03): syncing shows during reconnects. */
export type CraftSyncPhase = 'synced' | 'syncing' | 'failed';

/** Injectable reconnect backoff (tests inject zero delays and count attempts). */
export interface CraftBackoffOptions {
  /** Pure schedule for consecutive reconnect attempt n (0-based). */
  delayMs?(attempt: number): number;
  /** Sleep primitive; the default stores its timer so dispose cancels it. */
  sleep?(ms: number): Promise<void>;
}

export interface CraftControllerOptions {
  api: CraftApi;
  scope: ScopeController;
  events: CraftEventTransport;
  /** Kept from W04: fires only on permanent (non-retried) failures. */
  onError?(error: CraftSyncError): void;
  /** C03: every sync phase transition, with the triggering error if any. */
  onSync?(phase: CraftSyncPhase, error?: CraftSyncError): void;
  /** Injectable idempotency-key source; one key per user send. */
  newRequestId?(): string;
  /** C03: reconnect backoff injection (defaults to the domain schedule). */
  backoff?: CraftBackoffOptions;
}

export interface CraftWorkbenchController {
  /** Fetches the snapshot, then subscribes after last_seq. Refresh-safe. */
  load(sessionId: string): Promise<void>;
  /** One user send: generates the request id once, retries reuse it. */
  submit(prompt: string): Promise<void>;
  /**
   * C03: reload the authoritative snapshot and resubscribe after its
   * watermark — the manual/offline-online recovery entry. Reuses the same
   * session and run; never creates a run.
   */
  reconnect(): Promise<void>;
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
  const backoffDelayMs = options.backoff?.delayMs ?? reconnectDelayMs;
  const backoffSleep = options.backoff?.sleep;
  const listeners = new Set<(state: CraftState | null) => void>();

  let currentState: CraftState | null = null;
  let syncError: CraftSyncError | null = null;
  let syncPhase: CraftSyncPhase = 'synced';
  let subscription: ActiveSubscription | null = null;
  let sessionId: string | null = null;
  let pendingSubmit: { prompt: string; requestId: string } | null = null;
  let disposed = false;
  /** Invalidates every in-flight async continuation when bumped. */
  let syncEpoch = 0;
  /** Consecutive reconnect attempts without a healthy frame (backoff index). */
  let reconnectAttempt = 0;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  const disposeAbort = new AbortController();

  function notify(next: CraftState | null): void {
    currentState = next;
    for (const listener of listeners) listener(next);
  }

  function setSyncPhase(phase: CraftSyncPhase, error?: CraftSyncError): void {
    if (phase === 'failed' && error !== undefined) syncError = error;
    if (phase === syncPhase && phase !== 'syncing') return;
    syncPhase = phase;
    options.onSync?.(phase, error);
    if (phase === 'failed' && error !== undefined) options.onError?.(error);
  }

  function defaultSleep(ms: number): Promise<void> {
    return new Promise((resolve) => {
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        resolve();
      }, ms);
    });
  }

  function sleep(ms: number): Promise<void> {
    return backoffSleep !== undefined ? backoffSleep(ms) : defaultSleep(ms);
  }

  /** Kills a pending reconnect timer and invalidates every running cycle. */
  function cancelPendingReconnect(): void {
    if (reconnectTimer !== null) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    syncEpoch += 1;
  }

  function destroySubscription(): void {
    if (subscription === null) return;
    subscription.detachScopeAbort();
    subscription.abort.abort();
    subscription = null;
  }

  function resetForScopeChange(): void {
    cancelPendingReconnect();
    destroySubscription();
    pendingSubmit = null;
    notify(null);
    setSyncPhase('synced');
  }

  /** Permanent failure: reloading cannot fix it (contract violations). */
  function fail(error: CraftSyncError): void {
    destroySubscription();
    syncError = error;
    setSyncPhase('failed', error);
  }

  /** Abort signal covering BOTH the request scope and controller disposal. */
  function requestSignal(handle: ScopeHandle): AbortSignal {
    if (typeof AbortSignal.any === 'function') return AbortSignal.any([handle.signal, disposeAbort.signal]);
    return handle.signal;
  }

  /**
   * Atomic snapshot application (C03 Step 4): event-won projections survive
   * ONLY when they are ahead of the snapshot watermark within the same
   * generation AND the same run; anything else is a wholesale replacement.
   */
  function applySnapshot(view: CraftWorkspaceView, generation: number): void {
    const previous = currentState;
    const runId = view.active_run_id;
    const mainStatus = view.active_run !== null ? view.active_run.status : runId === null ? 'idle' : 'queued';
    const ahead =
      previous !== null && previous.generation === generation && previous.runId === runId && previous.seq > view.last_seq;
    notify({
      generation,
      runId,
      seq: ahead ? previous.seq : view.last_seq,
      mainStatus,
      delegationStatus: ahead ? previous.delegationStatus : 'idle',
      versionId: ahead ? previous.versionId : view.current_version !== null ? view.current_version.id : null,
    });
  }

  async function refreshSnapshot(id: string, generation: number, epoch: number): Promise<CraftWorkspaceView> {
    const handle = scope.current();
    if (handle.scope.generation !== generation) throw new CraftSyncError('STREAM_FAILED', 'scope changed during refresh');
    const view = await api.get(id, requestSignal(handle));
    if (disposed || epoch !== syncEpoch || !scope.isCurrent(generation)) {
      throw new CraftSyncError('STREAM_FAILED', 'scope changed during refresh');
    }
    return view;
  }

  /**
   * One reconnect cycle: authoritative snapshot reload, then resubscribe
   * after the fresh watermark while the run is still live. Scheduled with
   * backoff for every stream failure, gap and expired cursor; the epoch
   * guards keep superseded cycles from ever writing.
   */
  async function runReconnectCycle(id: string, generation: number, epoch: number): Promise<void> {
    let snapshot: CraftWorkspaceView;
    try {
      snapshot = await refreshSnapshot(id, generation, epoch);
    } catch {
      if (disposed || epoch !== syncEpoch) return;
      if (!scope.isCurrent(generation)) {
        resetForScopeChange();
        return;
      }
      // The snapshot endpoint is unreachable too (e.g. still offline): keep
      // retrying under the same growing backoff — never re-send the prompt.
      scheduleReconnect(
        new CraftSyncError('STREAM_FAILED', 'workspace snapshot unreachable while reconnecting'),
        id,
        generation,
      );
      return;
    }
    if (disposed || epoch !== syncEpoch) return;
    if (!scope.isCurrent(generation)) {
      resetForScopeChange();
      return;
    }
    applySnapshot(snapshot, generation);
    const state = currentState;
    setSyncPhase('synced');
    if (state === null || state.runId === null || isTerminal(state.mainStatus)) return;
    startSubscription(id, state.runId, state.seq, generation, epoch);
  }

  function scheduleReconnect(reason: CraftSyncError, id: string, generation: number): void {
    destroySubscription();
    syncError = reason;
    setSyncPhase('syncing', reason);
    const epoch = ++syncEpoch;
    const attempt = reconnectAttempt;
    reconnectAttempt += 1;
    void sleep(backoffDelayMs(attempt)).then(
      () => {
        if (disposed || epoch !== syncEpoch) return;
        if (!scope.isCurrent(generation)) {
          resetForScopeChange();
          return;
        }
        void runReconnectCycle(id, generation, epoch);
      },
      () => {
        // The sleep primitive never rejects by contract; nothing to do.
      },
    );
  }

  function startSubscription(id: string, runId: string, after: number, generation: number, epoch: number): void {
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
        // Transport failure (fetch dropped, HTTP error, offline): only the
        // subscription stops — no cancel call, no prompt re-send.
      }
      if (disposed || abort.signal.aborted) return;
      if (epoch !== syncEpoch) return; // superseded by load/dispose/scope switch
      if (!scope.isCurrent(generation)) {
        resetForScopeChange();
        return;
      }
      // The stream ended or failed: the snapshot decides whether the run is
      // still live — reconnect through an authoritative reload, with backoff.
      scheduleReconnect(
        new CraftSyncError('STREAM_FAILED', 'run event stream ended; reconnecting to the authoritative snapshot'),
        id,
        generation,
      );
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
    if (subscription !== active) return; // frame from a superseded subscription
    if (!scope.isCurrent(active.generation)) {
      resetForScopeChange();
      return;
    }
    const state = currentState;
    if (state === null || state.runId !== active.runId) return;
    try {
      if (frame.event === 'keepalive') {
        reconnectAttempt = 0; // a live stream restarts the backoff schedule
        return;
      }
      if (frame.event === 'error') {
        const value = parseFrameData(frame);
        const code = typeof value === 'object' && value !== null && !Array.isArray(value)
          ? (value as Record<string, unknown>)['code']
          : undefined;
        if (code === 'cursor_expired') {
          // The retained history no longer covers our cursor: reload the
          // authoritative snapshot and resume from its fresh watermark.
          scheduleReconnect(
            new CraftSyncError('CURSOR_EXPIRED', 'run event cursor expired; reloading the authoritative snapshot'),
            sessionId ?? '',
            active.generation,
          );
        }
        // Other stream errors end the stream server-side; pump reconnects.
        return;
      }
      if (frame.event === 'run') {
        reconnectAttempt = 0;
        applyRunProjection(parseCraftRunView(parseFrameData(frame)), active);
        return;
      }
      if (frame.event === undefined || /^[0-9]+$/.test(frame.event)) {
        const wire = parseCraftRunEvent(parseFrameData(frame));
        // The FULL main-run sequence counts (craft and non-craft events
        // alike — foreign payloads advanced the cursor above); the pure
        // domain rule decides what this seq means against our cursor.
        const action = replayAction(state.seq, wire.seq);
        if (action === 'drop') {
          reconnectAttempt = 0; // duplicate replay from a reconnect
          return;
        }
        if (action === 'reload') {
          scheduleReconnect(
            new CraftSyncError('SEQ_GAP', 'run event gap detected; reloading the authoritative snapshot', {
              expectedSeq: state.seq + 1,
              receivedSeq: wire.seq,
            }),
            sessionId ?? '',
            active.generation,
          );
          return;
        }
        reconnectAttempt = 0;
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
      // malformed frame is a permanent failure (a reload cannot fix it).
      fail(new CraftSyncError('STREAM_FAILED', 'malformed run event rejected by the contract parsers'));
      if (error instanceof Error) console.error('[craft] contract violation:', error.message);
    }
  }

  return {
    async load(id: string): Promise<void> {
      if (disposed) throw new Error('controller is disposed');
      sessionId = id;
      const handle = scope.current();
      cancelPendingReconnect(); // a refresh invalidates any pending reconnect
      const epoch = syncEpoch;
      destroySubscription();
      const view = await api.get(id, requestSignal(handle));
      if (disposed || epoch !== syncEpoch) return;
      if (!scope.isCurrent(handle.scope.generation)) {
        // A concurrent scope switch already reset the workbench.
        return;
      }
      syncError = null;
      // Refresh path: the snapshot's active_run_id reuses the SAME session
      // and run — load never creates a run.
      applySnapshot(view, handle.scope.generation);
      const state = currentState;
      setSyncPhase('synced');
      if (state !== null && state.runId !== null && !isTerminal(state.mainStatus)) {
        startSubscription(id, state.runId, state.seq, state.generation, epoch);
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
        const run = await api.submit(sessionId, { request_id: requestId, prompt }, requestSignal(handle));
        if (disposed || !scope.isCurrent(generation)) {
          // The server holds the admission under this key; the next load in
          // the new scope recovers it. Never re-submit from here.
          return;
        }
        pendingSubmit = null;
        syncError = null;
        cancelPendingReconnect();
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
          startSubscription(sessionId, run.run_id, run.seq, generation, syncEpoch);
        }
      } catch (error) {
        if (!scope.isCurrent(generation)) pendingSubmit = null;
        throw error;
      }
    },

    async reconnect(): Promise<void> {
      if (disposed) throw new Error('controller is disposed');
      if (sessionId === null) throw new Error('load the craft workspace before reconnecting');
      const handle = scope.current();
      const generation = handle.scope.generation;
      cancelPendingReconnect();
      const epoch = syncEpoch;
      destroySubscription();
      reconnectAttempt = 0; // an explicit recovery attempt restarts the schedule
      setSyncPhase('syncing');
      const view = await refreshSnapshot(sessionId, generation, epoch);
      if (disposed || epoch !== syncEpoch) return;
      if (!scope.isCurrent(generation)) {
        resetForScopeChange();
        return;
      }
      applySnapshot(view, generation);
      const state = currentState;
      setSyncPhase('synced');
      if (state !== null && state.runId !== null && !isTerminal(state.mainStatus)) {
        startSubscription(sessionId, state.runId, state.seq, generation, epoch);
      }
    },

    dispose(): void {
      disposed = true;
      cancelPendingReconnect(); // terminates the pending reconnect timer
      destroySubscription();
      disposeAbort.abort(); // terminates in-flight snapshot/submit requests
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
