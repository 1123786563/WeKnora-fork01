import type { ExecutionEvent } from '@weknora/contracts';

/**
 * W12 lifecycle recovery controller.
 *
 * One recovery pass is strictly ordered: query the durable status first,
 * refresh the visible history second, and only open the event stream when the
 * run is still active. A terminal run therefore never opens a stream, and an
 * active run resumes exactly where W09 last committed.
 */
export interface RecoveryPorts {
  /** W06 request/run reconciliation mapped to a terminal verdict. */
  status(): Promise<{ terminal: boolean }>;
  /** Re-reads the durable snapshot so the history survives a relaunch. */
  refreshHistory(): Promise<void>;
  /** Opens the SSE subscription from the last committed cursor. */
  subscribe(): Promise<void>;
}

/** status -> refreshHistory -> subscribe only while the run is non-terminal. */
export async function recoverRun(ports: RecoveryPorts): Promise<void> {
  const state = await ports.status();
  await ports.refreshHistory();
  if (!state.terminal) await ports.subscribe();
}

/**
 * Recognizes the 404 family across the client stack: fetch/stream adapters
 * raise `execution stream HTTP 404`, transport ports attach `status`, and the
 * server surface leaks gorm's `record not found`. A 404 must surface as a
 * retryable failure; it never authorizes creating a replacement session (this
 * controller structurally has no start/create port at all).
 */
export function isRecoveryNotFound(error: unknown): boolean {
  if (!error) return false;
  if (typeof error === 'object' && 'status' in error) {
    const status = (error as { status?: unknown }).status;
    if (status === 404) return true;
  }
  const message = error instanceof Error ? error.message : String(error);
  return /HTTP 404\b/.test(message) || /NOT_FOUND/i.test(message) || /record not found/i.test(message);
}

export type RecoveryState = 'idle' | 'recovering' | 'failed';

/**
 * Maps a W06 execution status onto the terminal verdict. Everything else —
 * including `unknown` — stays non-terminal so the run remains visible and
 * the subscription path stays eligible.
 */
const TERMINAL_EXECUTION_STATUSES = new Set(['succeeded', 'failed', 'canceled']);

export function isTerminalExecutionStatus(status: string): boolean {
  return TERMINAL_EXECUTION_STATUSES.has(status.trim().toLowerCase());
}

/** The scope seam already owned by the product session (W07). */
export interface RecoveryScope {
  capture(): { generation: number; signal: AbortSignal };
  accept(generation: number): boolean;
  subscribe?(listener: () => void): () => void;
}

export interface ExecutionRecoveryInput {
  ports: RecoveryPorts;
  /** When provided, an identity transition aborts the in-flight pass. */
  scope?: RecoveryScope;
  onStatus?: (state: RecoveryState, error?: unknown) => void;
}

/**
 * The handle a conversation screen drives. `appStateChange('background')`
 * only closes the subscription; this controller has no cancel command and can
 * never terminate a server-side run.
 */
export interface ExecutionRecovery {
  /**
   * Single-flighted: concurrent callers await the same in-flight pass. The
   * returned promise never rejects; failures surface through `getState` and
   * `onStatus` so an unawaited AppState trigger stays unhandled-rejection
   * free and the retry affordance is state driven.
   */
  recover(): Promise<void>;
  appStateChange(next: string): void;
  getState(): { state: RecoveryState; error?: unknown };
  subscribe(listener: () => void): () => void;
  dispose(): void;
}

function recoveryAborted(): Error {
  return Object.assign(new Error('RECOVERY_ABORTED'), { name: 'AbortError' });
}

function isAbort(error: unknown): boolean {
  return error instanceof Error && (error.name === 'AbortError' || /RECOVERY_ABORTED|aborted/i.test(error.message));
}

/** Creates the production recovery controller. */
export function createExecutionRecovery(input: ExecutionRecoveryInput): ExecutionRecovery {
  const listeners = new Set<() => void>();
  const activeAborts = new Set<() => void>();
  let state: RecoveryState = 'idle';
  let failure: unknown;
  let inFlight: Promise<void> | undefined;
  let disposed = false;
  let unsubscribeScope: (() => void) | undefined;

  const publish = (next: RecoveryState, error?: unknown): void => {
    state = next;
    failure = next === 'failed' ? error : undefined;
    input.onStatus?.(state, failure);
    listeners.forEach((listener) => listener());
  };

  // Each step races the ports against scope/background aborts so a hung
  // request can never outlive the lifecycle that started it.
  const step = async <T,>(operation: () => Promise<T>, signals: readonly AbortSignal[]): Promise<T> => (
    new Promise<T>((resolve, reject) => {
      const abort = () => reject(recoveryAborted());
      const cleanups = signals.map((signal) => {
        if (signal.aborted) {
          abort();
          return () => undefined;
        }
        signal.addEventListener('abort', abort, { once: true });
        return () => signal.removeEventListener('abort', abort);
      });
      operation().then((value) => {
        cleanups.forEach((cleanup) => cleanup());
        resolve(value);
      }, (error) => {
        cleanups.forEach((cleanup) => cleanup());
        reject(error);
      });
    })
  );

  const run = async (): Promise<void> => {
    const captured = input.scope?.capture();
    const background = new AbortController();
    const signals = [background.signal, ...(captured ? [captured.signal] : [])];
    const stillCurrent = () => captured === undefined || input.scope!.accept(captured.generation);
    const close = () => { background.abort(); };
    activeAborts.add(close);
    try {
      publish('recovering');
      const verdict = await step(input.ports.status, signals);
      if (!stillCurrent()) { publish('idle'); return; }
      await step(input.ports.refreshHistory, signals);
      if (!stillCurrent()) { publish('idle'); return; }
      if (!verdict.terminal) await step(input.ports.subscribe, signals);
      if (!stillCurrent()) { publish('idle'); return; }
      publish('idle');
    } catch (error) {
      // A background transition or scope switch stopped this pass on purpose.
      // The run itself stays untouched server-side: background closes the
      // subscription only and there is no cancel command anywhere here.
      if (isAbort(error) || !stillCurrent()) {
        publish('idle');
        return;
      }
      publish('failed', error);
    } finally {
      activeAborts.delete(close);
    }
  };

  if (input.scope) {
    unsubscribeScope = input.scope.subscribe?.(() => {
      // Identity transition: cancel the current pass; late responses are
      // fenced by the generation check inside the pass as well.
      activeAborts.forEach((close) => close());
    });
  }

  const controller: ExecutionRecovery = {
    recover(): Promise<void> {
      if (disposed) return Promise.resolve();
      if (inFlight) return inFlight;
      let settle!: () => void;
      const current = new Promise<void>((done) => { settle = done; });
      inFlight = current;
      // Single-flight is scoped to the in-flight pass only: the slot is
      // released BEFORE settling so the next foreground transition or
      // retry-button press observably starts a fresh pass (review C-1; a
      // `.finally` cleanup would run after awaiting continuations resume).
      const finish = () => { if (inFlight === current) inFlight = undefined; settle(); };
      run().then(finish, finish);
      return current;
    },
    appStateChange(next: string): void {
      if (disposed) return;
      if (next === 'active') {
        void controller.recover();
        return;
      }
      if (next === 'background') {
        // Only closes the subscription; there is deliberately no cancel path.
        activeAborts.forEach((close) => close());
      }
    },
    getState() {
      return { state, ...(state === 'failed' ? { error: failure } : {}) };
    },
    subscribe(listener: () => void): () => void {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    dispose(): void {
      disposed = true;
      activeAborts.forEach((close) => close());
      unsubscribeScope?.();
      listeners.clear();
    },
  };
  return controller;
}

/**
 * Wraps a W09 event store plus the W06 stream call so a resumed subscription
 * starts from the highest committed sequence instead of replaying from zero.
 */
export function subscribeFromLastCommittedCursor(input: {
  read: (runID: string) => Promise<readonly ExecutionEvent[]>;
  stream: (runID: string, lastEventID: string | undefined, onEvent?: (event: ExecutionEvent) => void, signal?: AbortSignal) => Promise<void>;
}): (runID: string, onEvent?: (event: ExecutionEvent) => void, signal?: AbortSignal) => Promise<void> {
  return async (runID: string, onEvent?: (event: ExecutionEvent) => void, signal?: AbortSignal): Promise<void> => {
    const committed = await input.read(runID);
    // Events may be committed out of order; the cursor is the durable maximum.
    const cursor = committed.reduce((max, event) => Math.max(max, event.seq), 0);
    await input.stream(runID, cursor > 0 ? String(cursor) : undefined, onEvent, signal);
  };
}
