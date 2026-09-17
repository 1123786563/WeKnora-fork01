/**
 * W29 — hold-to-talk dictation with transcript confirmation.
 *
 * The controller enforces the product contract for voice input:
 * - A finished transcription only fills the draft (never auto-submits);
 *   the user's explicit confirm consumes their latest edited value.
 * - Confirm rides the same single-flight send controller as typed input
 *   (W10 SendController), so a voice send can never double-submit.
 * - Every failure path (mic denied, interrupted recording, transcription
 *   timeout/failure, empty audio, over-long capture, scope switch)
 *   preserves the draft untouched so the text input stays as the fallback.
 * - A cancelled hold (drag-out, backgrounding, incoming call, space switch)
 *   stops the recorder and deletes the temporary audio file.
 *
 * The `DictationPort` is the native audio seam. Its `transcribe` operation
 * is the W30 consumption point: the server-side transcription endpoint
 * (product auth + budget, never a client-held long-lived model key) is
 * W30's domain; this controller only awaits its string result.
 */

/** Controller lifecycle: idle -> recording -> transcribing -> ready; error returns to idle on the next begin. */
export type DictationState = 'idle' | 'recording' | 'transcribing' | 'ready' | 'error';

export type DictationErrorCode =
  | 'MIC_PERMISSION_DENIED'
  | 'RECORD_INTERRUPTED'
  | 'TRANSCRIBE_TIMEOUT'
  | 'TRANSCRIBE_FAILED'
  | 'EMPTY_AUDIO'
  | 'AUDIO_TOO_LONG'
  | 'SCOPE_CHANGED'
  | 'TRANSCRIPT_REQUIRED';

export class DictationError extends Error {
  constructor(readonly code: DictationErrorCode, message?: string) {
    super(message ?? code);
    this.name = 'DictationError';
  }
}

export function isDictationError(error: unknown): error is DictationError {
  return error instanceof DictationError;
}

export function dictationErrorCode(error: unknown): DictationErrorCode | null {
  return isDictationError(error) ? error.code : null;
}

/**
 * Native audio seam. `start` requests microphone permission and begins
 * capturing; `stop` finalizes the capture and reports the temporary file;
 * `cancel` stops and deletes the temporary file; `transcribe` hands the
 * capture to the transcription service (W30 consumption point — the
 * product-authenticated, budget-metered server endpoint).
 */
export interface DictationPort {
  start(): Promise<void>;
  stop(): Promise<{ uri: string; durationMs: number }>;
  cancel(): Promise<void>;
  /** The optional signal (M-3) aborts the underlying upload on timeout/cancel. */
  transcribe(uri: string, signal?: AbortSignal): Promise<string>;
}

/** Structural subset of ProductScope (W07): space switches invalidate an in-flight dictation. */
export interface DictationScope {
  capture(): { generation: number; signal: AbortSignal };
  accept(generation: number): boolean;
}

export interface DictationLimits {
  /** Maximum capture length before an automatic finish. Default 60s; capability config may tighten it. */
  maxDurationMs: number;
  /** Wall clock budget for one transcription call. */
  transcribeTimeoutMs: number;
}

export const defaultDictationLimits: DictationLimits = {
  maxDurationMs: 60_000,
  transcribeTimeoutMs: 30_000,
};

/** Timer slack allowed between the JS cap firing and the native stop reporting duration. */
const CAP_TOLERANCE_MS = 2_000;

export interface DictationFailure {
  code: DictationErrorCode;
  message?: string;
}

export interface DictationControllerOptions {
  /** Native audio port; without it the controller runs the portless confirmation-only form. */
  port?: DictationPort;
  limits?: Partial<DictationLimits>;
  /** Invalidation seam: a rejected generation cancels the flow without touching the draft. */
  scope?: DictationScope;
  onStateChange?(state: DictationState, failure: DictationFailure | null): void;
}

export interface DictationController {
  /** Press-in: requests mic permission and starts capturing. Errors land in the error state. */
  begin(): Promise<void>;
  /** Press-out: stops capturing, transcribes, and fills the draft only. Never submits. */
  finish(): Promise<void>;
  /** Aborts the flow: stops recording, deletes the temporary audio, leaves the draft untouched. */
  cancel(): Promise<void>;
  /** Sends the user's latest edited draft through the single-flight send controller. */
  confirm(): Promise<void>;
  state(): DictationState;
  failure(): DictationFailure | null;
  /** True while the error state came from a denied microphone (UI falls back to text input). */
  isPermissionDenied(): boolean;
}

function withTimeout<T>(value: Promise<T>, ms: number, code: DictationErrorCode, abort?: AbortController): Promise<T> {
  if (!(ms > 0)) return value;
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => {
      // M-3: a timed-out transcription also aborts the underlying upload so
      // the request cannot linger past its deadline.
      abort?.abort();
      reject(new DictationError(code));
    }, ms);
    value.then(
      (result) => { clearTimeout(timer); resolve(result); },
      (error) => { clearTimeout(timer); reject(error); },
    );
  });
}

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/**
 * Builds the dictation controller. The portless form (no `options.port`)
 * is the minimal confirmation semantics; production always passes the
 * native audio port, in which case `finish` stops the capture and calls
 * `port.transcribe(uri)` directly.
 */
export function createDictationController(
  transcribe: () => Promise<string>,
  setDraft: (text: string) => void,
  getDraft: () => string,
  send: (text: string) => Promise<void>,
  options: DictationControllerOptions = {},
): DictationController {
  const limits: DictationLimits = { ...defaultDictationLimits, ...options.limits };
  const port = options.port;
  let phase: DictationState = 'idle';
  let failure: DictationFailure | null = null;
  /** Flow generation: bumped by every state-leaving op so stale async results are dropped. */
  let flow = 0;
  let recordingActive = false;
  let capped = false;
  let capTimer: ReturnType<typeof setTimeout> | null = null;
  let capturedGeneration: number | null = null;
  let transcribeAbort: AbortController | null = null;

  const notify = () => options.onStateChange?.(phase, failure);
  const setState = (next: DictationState) => { phase = next; notify(); };
  const fail = (code: DictationErrorCode, message?: string) => {
    failure = { code, ...(message ? { message } : {}) };
    setState('error');
  };
  const clearCapTimer = () => {
    if (capTimer !== null) { clearTimeout(capTimer); capTimer = null; }
  };
  /**
   * Best-effort temporary-audio deletion on abandoned paths. Only invoked
   * while this flow is still current (a superseding begin/cancel owns the
   * recorder and the port's own stale-file cleanup covers its predecessor).
   */
  const cleanupTemp = () => {
    if (!port) return;
    void port.cancel().catch(() => undefined);
  };

  const begin = async (): Promise<void> => {
    if (phase === 'recording' || phase === 'transcribing') return;
    failure = null;
    const run = ++flow;
    capturedGeneration = options.scope ? options.scope.capture().generation : null;
    if (port) {
      try {
        await port.start();
      } catch (error) {
        if (run !== flow) return;
        if (dictationErrorCode(error) === 'MIC_PERMISSION_DENIED') fail('MIC_PERMISSION_DENIED', messageOf(error));
        else fail('RECORD_INTERRUPTED', messageOf(error));
        return;
      }
      if (run !== flow) return; // superseded while permission/start resolved
      recordingActive = true;
    }
    capped = false;
    clearCapTimer();
    capTimer = setTimeout(() => {
      // The duration cap auto-finishes the hold: the captured audio is still
      // transcribed (WeChat-style), so hitting the cap is not an error.
      if (flow !== run) return;
      capped = true;
      void finish();
    }, limits.maxDurationMs);
    setState('recording');
  };

  const finish = async (): Promise<void> => {
    if (phase !== 'recording') {
      // Portless form: a bare finish (no begin) still runs the transcription
      // seam into the draft. With a native port a stray release is ignored —
      // only an active hold can finish.
      if (port) return;
    } else {
      clearCapTimer();
    }
    const run = ++flow;
    let transcribeCall: (signal?: AbortSignal) => Promise<string>;
    // M-3: one abort channel per transcription — timeout and cancel both
    // abort the underlying upload instead of merely dropping its result.
    transcribeAbort = new AbortController();
    if (port) {
      if (phase !== 'recording' || !recordingActive) return;
      recordingActive = false;
      let recording: { uri: string; durationMs: number };
      try {
        recording = await port.stop();
      } catch (error) {
        if (run !== flow) return;
        fail('RECORD_INTERRUPTED', messageOf(error));
        return;
      }
      if (run !== flow) return; // superseded while stopping; new owner cleans up
      if (!recording.uri || recording.durationMs <= 0) {
        fail('EMPTY_AUDIO');
        cleanupTemp();
        return;
      }
      if (!capped && recording.durationMs > limits.maxDurationMs + CAP_TOLERANCE_MS) {
        fail('AUDIO_TOO_LONG');
        cleanupTemp();
        return;
      }
      const uri = recording.uri;
      transcribeCall = (signal?: AbortSignal) => port.transcribe(uri, signal);
    } else {
      transcribeCall = () => transcribe();
    }
    failure = null;
    setState('transcribing');
    try {
      const text = await withTimeout(transcribeCall(transcribeAbort.signal), limits.transcribeTimeoutMs, 'TRANSCRIBE_TIMEOUT', transcribeAbort);
      if (run !== flow) return;
      if (capturedGeneration !== null && options.scope && !options.scope.accept(capturedGeneration)) {
        fail('SCOPE_CHANGED');
        cleanupTemp();
        return;
      }
      // Transcription completion only fills the draft; submission waits for
      // the user's explicit confirm of their (possibly edited) value.
      setDraft(text);
      setState('ready');
      cleanupTemp();
    } catch (error) {
      if (run !== flow) return;
      fail(dictationErrorCode(error) === 'TRANSCRIBE_TIMEOUT' ? 'TRANSCRIBE_TIMEOUT' : 'TRANSCRIBE_FAILED', messageOf(error));
      cleanupTemp();
    }
  };

  const cancel = async (): Promise<void> => {
    if (phase === 'idle') return;
    flow += 1; // supersede any in-flight stop/transcribe
    clearCapTimer();
    recordingActive = false;
    transcribeAbort?.abort();
    transcribeAbort = null;
    if (port) await port.cancel().catch(() => undefined);
    failure = null;
    setState('idle');
  };

  const confirm = async (): Promise<void> => {
    const text = getDraft();
    if (text.trim() === '') throw new DictationError('TRANSCRIPT_REQUIRED');
    // Single-flight is owned by the injected send (the W10 SendController):
    // a busy sender throws SEND_IN_PROGRESS and this draft is preserved.
    await send(text);
    setDraft('');
    if (phase === 'ready') setState('idle');
  };

  return {
    begin,
    finish,
    cancel,
    confirm,
    state: () => phase,
    failure: () => failure,
    isPermissionDenied: () => failure?.code === 'MIC_PERMISSION_DENIED',
  };
}
