/**
 * W31 — realtime voice controls, grant lifecycle and voice utterance policy.
 *
 * Three commands are deliberately SEPARATED:
 * - `interrupt()` only stops the assistant audio playback (barge-in). It
 *   never calls a cancel API: silencing the speaker is not a product
 *   decision about the run.
 * - `endVoice()` only closes the provider voice connection (media leg +
 *   W30 settle). The run keeps executing server-side.
 * - `cancelTask()` is the one and only path that cancels the run, and it is
 *   an explicit product command (the W05 command surface), never a side
 *   effect of audio controls.
 *
 * The session controller owns the W30 grant lifecycle:
 * - Connections are created per grant (`POST /mobile/voice/sessions`
 *   admission) and connect with exactly that short-lived token.
 * - An expired grant renews through FRESH admission (release + re-admit +
 *   reconnect). There is no timer and no retry loop: every renewal is an
 *   explicit call, a failed renewal surfaces `failed`, and renewals are
 *   capped per begun session — no infinite reconnect.
 * - A system disconnect (incoming call, backgrounding) closes the provider
 *   and settles the session, runs one W12 recoverRun-shaped query (the app
 *   is still foregrounded) and NEVER reopens the paid session on its own.
 *
 * Approval policy: high-risk operations triggered by voice must appear as
 * the W05 structured approval card. Spoken answers — ambiguous or not —
 * never approve it; `createVoiceUtteranceHandler` structurally has no
 * approve port, so only the card's buttons can.
 */
import { createJsonTransport, type BearerCredential, type FetchLike, type ProductAuthSession } from '@weknora/api-client';

/** W30 wire contract: short-lived grant, ExpiresAt is RFC3339. */
export interface RealtimeVoiceGrant {
  token: string;
  /** RFC3339 timestamp (W30 `expires_at`); the only clock the client trusts. */
  expiresAt: string;
}

/** Result of one W30 admission (POST /mobile/voice/sessions). */
export interface VoiceSessionAdmission {
  id: string;
  grant: RealtimeVoiceGrant;
  /** W30 replay marker: an existing open session answers without a fresh token. */
  tokenIssued: boolean;
}

/**
 * Realtime provider seam. `connect` consumes exactly one W30 grant; `close`
 * ends the media leg; `mute` gates the microphone. `stopAudio` is the
 * barge-in seam owned by the audio playout stack (optional: ports without a
 * live playout surface have nothing to stop).
 */
export interface RealtimeVoicePort {
  connect(grant: RealtimeVoiceGrant): Promise<void>;
  close(): Promise<void>;
  mute(value: boolean): void;
  /** Optional: stops the current assistant audio playout (barge-in). */
  stopAudio?(): void;
}

export interface VoiceControlsPorts {
  /** Stops the assistant audio playback only. No API call. */
  stopAudio(): void;
  /** Closes the provider voice connection (media leg + settle). No cancel. */
  closeVoice(): Promise<void>;
  /** The explicit product cancel command (W05 command surface). */
  cancelRun(): Promise<void>;
}

export interface VoiceControls {
  /** Barge-in: stop the playback, keep the run and the connection alive. */
  interrupt(): void;
  /** End the voice connection. The run itself is untouched. */
  endVoice(): Promise<void>;
  /** Explicit product cancel of the run (separate user command). */
  cancelTask(): Promise<void>;
}

/**
 * Builds the three separated voice controls. The ports are injected so the
 * conversation screen maps them onto the realtime session (stopAudio /
 * closeVoice) and the view-model command boundary (cancelRun).
 */
export function createVoiceControls(ports: VoiceControlsPorts): VoiceControls {
  return {
    interrupt(): void {
      ports.stopAudio();
    },
    endVoice(): Promise<void> {
      return ports.closeVoice();
    },
    cancelTask(): Promise<void> {
      return ports.cancelRun();
    },
  };
}

export type RealtimeVoiceErrorCode =
  | 'ADMISSION_HTTP'
  | 'ADMISSION_RESPONSE_INVALID'
  | 'ADMISSION_NO_TOKEN'
  | 'RELEASE_HTTP'
  | 'RENEWAL_EXHAUSTED'
  | 'PROVIDER_UNAVAILABLE';

export class RealtimeVoiceError extends Error {
  constructor(readonly code: RealtimeVoiceErrorCode, message?: string) {
    super(message ?? code);
    this.name = 'RealtimeVoiceError';
  }
}

export function isRealtimeVoiceError(error: unknown): error is RealtimeVoiceError {
  return error instanceof RealtimeVoiceError;
}

// ---------------------------------------------------------------------------
// Session controller: W30 grant lifecycle with bounded, explicit renewals.
// ---------------------------------------------------------------------------

export type RealtimeVoiceState =
  | 'idle'
  | 'connecting'
  | 'connected'
  | 'renewing'
  | 'interrupted'
  | 'ended'
  | 'failed';

export interface RealtimeVoiceSessionInput {
  port: RealtimeVoicePort;
  /** W30 admission (POST /mobile/voice/sessions): one fresh grant per call. */
  admit(): Promise<VoiceSessionAdmission>;
  /** W30 stop/settle (DELETE /mobile/voice/sessions/:id); idempotent. */
  release(id: string): Promise<void>;
  /**
   * W12 recoverRun-shaped query, invoked at most once per system disconnect
   * while the app is still foregrounded (incoming call). A background
   * transition deliberately does NOT run it here: the W12 controller owns
   * the foreground recovery pass.
   */
  recover?(): Promise<void>;
  /** Injectable clock for grant expiry math. Defaults to Date.now. */
  now?(): number;
  /** Clock-skew tolerance when judging grant expiry. Default 5s. */
  expirySkewMs?: number;
  /** Successful renewals allowed per begun session (default 3). */
  maxRenewals?: number;
  onStateChange?(state: RealtimeVoiceState): void;
}

export interface RealtimeVoiceSession {
  /** Explicit connect: fresh admission (releasing anything held) + connect. */
  begin(): Promise<void>;
  /**
   * Renewal only when the grant is expired. Returns true when a renewal ran.
   * Single-flight per call, never loops, capped by `maxRenewals`.
   */
  renewIfExpired(): Promise<boolean>;
  /** End the voice connection (provider close + settle). Never cancels. */
  end(): Promise<void>;
  /** Barge-in: stops the audio playout only. */
  interruptPlayback(): void;
  /** Microphone gate only. */
  setMuted(value: boolean): void;
  /**
   * System lifecycle interruption (background / incoming call): close the
   * provider, settle the session, query the run once — and never re-admit.
   */
  interruptedBySystem(reason: 'background' | 'connection-lost'): Promise<void>;
  state(): RealtimeVoiceState;
  grant(): RealtimeVoiceGrant | null;
  sessionID(): string | null;
  renewals(): number;
  /** The last failure's typed code/message; null outside the failed state. */
  failure(): { code: string; message: string } | null;
  subscribe(listener: () => void): () => void;
  dispose(): void;
}

/** Default renewal cap: re-authorization is bounded within one user action. */
const DEFAULT_MAX_RENEWALS = 3;
/** Default clock-skew tolerance around the RFC3339 expiry. */
const DEFAULT_EXPIRY_SKEW_MS = 5_000;

/**
 * Creates the realtime voice session controller. Every admission is an
 * explicit user-visible action; nothing in here runs on a timer, so a dead
 * grant or a failed admission can never produce a reconnect loop.
 */
export function createRealtimeVoiceSession(input: RealtimeVoiceSessionInput): RealtimeVoiceSession {
  const now = input.now ?? (() => Date.now());
  const skew = input.expirySkewMs ?? DEFAULT_EXPIRY_SKEW_MS;
  const maxRenewals = input.maxRenewals ?? DEFAULT_MAX_RENEWALS;
  const listeners = new Set<() => void>();
  let state: RealtimeVoiceState = 'idle';
  let grant: RealtimeVoiceGrant | null = null;
  let sessionID: string | null = null;
  let renewals = 0;
  let disposed = false;
  let failure: { code: string; message: string } | null = null;

  const publish = (next: RealtimeVoiceState) => {
    state = next;
    if (next !== 'failed') failure = null;
    input.onStateChange?.(state);
    listeners.forEach((listener) => listener());
  };
  // W12 recover() precedent: lifecycle entry points never reject — failures
  // surface through the state so an unawaited button press cannot produce an
  // unhandled rejection.
  const fail = (error: unknown) => {
    grant = null;
    sessionID = null;
    failure = isRealtimeVoiceError(error)
      ? { code: error.code, message: error.message }
      : { code: 'ADMISSION_HTTP', message: error instanceof Error ? error.message : String(error) };
    publish('failed');
  };
  const expired = (value: RealtimeVoiceGrant): boolean => {
    const at = Date.parse(value.expiresAt);
    return !Number.isFinite(at) || now() >= at - skew;
  };
  /** Best-effort teardown of whatever session we currently hold. */
  const dropCurrent = async (): Promise<void> => {
    const heldID = sessionID;
    grant = null;
    sessionID = null;
    if (heldID !== null) {
      await input.port.close().catch(() => undefined);
      await input.release(heldID).catch(() => undefined);
    }
  };
  /**
   * One admission attempt. A replay answer without a fresh token means a
   * stale row blocks the product session: release it and retry admission
   * exactly once more — still no token is a hard failure, never a loop.
   */
  const admitFresh = async (): Promise<VoiceSessionAdmission> => {
    let admitted = await input.admit();
    if (admitted.tokenIssued && admitted.grant.token !== '') return admitted;
    await input.release(admitted.id).catch(() => undefined);
    admitted = await input.admit();
    if (!admitted.tokenIssued || admitted.grant.token === '') {
      throw new RealtimeVoiceError('ADMISSION_NO_TOKEN', 'ADMISSION_NO_TOKEN');
    }
    return admitted;
  };

  const begin = async (): Promise<void> => {
    if (disposed) return;
    publish('connecting');
    try {
      await dropCurrent();
      const admitted = await admitFresh();
      if (disposed) return;
      grant = admitted.grant;
      sessionID = admitted.id;
      renewals = 0;
      await input.port.connect(admitted.grant);
      publish('connected');
    } catch (error) {
      // F-1 (review round 1): the admission already took a durable W30
      // budget hold and an open row, so a failed connect must settle the
      // granted session — never dangle it until the deadline sweeper.
      await dropCurrent();
      fail(error);
    }
  };

  const renewIfExpired = async (): Promise<boolean> => {
    if (disposed) return false;
    if (state !== 'connected' || !grant) return false;
    if (!expired(grant)) return false;
    if (renewals >= maxRenewals) {
      // Bounded by design: the user must begin a new session explicitly.
      failure = { code: 'RENEWAL_EXHAUSTED', message: 'RENEWAL_EXHAUSTED' };
      publish('failed');
      return false;
    }
    publish('renewing');
    try {
      await dropCurrent();
      const admitted = await admitFresh();
      if (disposed) return false;
      grant = admitted.grant;
      sessionID = admitted.id;
      await input.port.connect(admitted.grant);
      renewals += 1;
      publish('connected');
      return true;
    } catch (error) {
      // F-1: same settle-on-failure contract as begin — the fresh admission
      // from this renewal must not leak its hold either.
      await dropCurrent();
      fail(error);
      return false;
    }
  };

  const end = async (): Promise<void> => {
    if (state === 'ended' || state === 'idle') return;
    await dropCurrent();
    if (!disposed) publish('ended');
  };

  const interruptedBySystem = async (reason: 'background' | 'connection-lost'): Promise<void> => {
    if (disposed) return;
    if (state !== 'connected' && state !== 'connecting' && state !== 'renewing') return;
    await dropCurrent();
    publish('interrupted');
    // The recovery query belongs to the foreground lifecycle (W12): an
    // incoming call keeps the app foregrounded so the query runs here,
    // exactly once; a background transition leaves it to the W12 controller.
    if (reason === 'connection-lost') await input.recover?.().catch(() => undefined);
  };

  return {
    begin,
    renewIfExpired,
    end,
    interruptPlayback(): void {
      input.port.stopAudio?.();
    },
    setMuted(value: boolean): void {
      input.port.mute(value);
    },
    interruptedBySystem,
    state: () => state,
    grant: () => grant,
    sessionID: () => sessionID,
    renewals: () => renewals,
    failure: () => failure,
    subscribe(listener: () => void): () => void {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    dispose(): void {
      if (disposed) return;
      disposed = true;
      void dropCurrent();
      listeners.clear();
    },
  };
}

// ---------------------------------------------------------------------------
// Voice utterances: classified conservatively; approvals stay on the card.
// ---------------------------------------------------------------------------

export type VoiceUtteranceIntent =
  | { kind: 'interrupt-playback' }
  | { kind: 'end-voice' }
  | { kind: 'cancel-task' }
  | { kind: 'approval-answer'; explicit: boolean }
  | { kind: 'unknown' };

/** Wording that names the task/run explicitly — the only cancel phrases. */
const CANCEL_VERB_PATTERN = /取消|停止|中止|终止|\b(cancell?\w*|stop\w*|abort\w*|kill\w*)\b/;
const TASK_NOUN_PATTERN = /任务|执行|作业|\b(task|run|agent|job)\w*\b/;
const END_VOICE_MARKERS = ['结束语音', '挂断', '结束通话', 'end voice', 'end the call', 'hang up'];
const INTERRUPT_MARKERS = ['停止播放', '别说了', '停下', '闭嘴', '安静', 'stop playback', 'stop speaking', 'stop'];
/** Strong, unambiguous approval words (still never auto-approve). */
const EXPLICIT_APPROVAL_MARKERS = ['批准', '同意', '确认', 'approve', 'approved', 'confirm', 'yes'];
/** Weak acknowledgements — ambiguous by definition. */
const AMBIGUOUS_APPROVAL_MARKERS = ['嗯', '好', '好的', '可以', '行', 'ok', 'okay', '好吧', '随便'];

function containsAny(normalized: string, markers: readonly string[]): boolean {
  return markers.some((marker) => normalized.includes(marker));
}

/**
 * Classifies one user utterance from the realtime channel. Conservative by
 * design: a cancel must pair a cancel verb with an explicit task noun, so
 * interleaved wording still lands while vague talk never cancels; approval
 * acknowledgements are recognized for card surfacing only — the classifier
 * never emits an approve action.
 */
export function classifyVoiceUtterance(text: string): VoiceUtteranceIntent {
  const normalized = text.trim().toLowerCase();
  if (normalized === '') return { kind: 'unknown' };
  // Cancel first: phrases that name the task outrank playback-stop wording.
  if (CANCEL_VERB_PATTERN.test(normalized) && TASK_NOUN_PATTERN.test(normalized)) return { kind: 'cancel-task' };
  if (containsAny(normalized, END_VOICE_MARKERS)) return { kind: 'end-voice' };
  if (containsAny(normalized, INTERRUPT_MARKERS)) return { kind: 'interrupt-playback' };
  if (containsAny(normalized, EXPLICIT_APPROVAL_MARKERS)) return { kind: 'approval-answer', explicit: true };
  if (containsAny(normalized, AMBIGUOUS_APPROVAL_MARKERS)) return { kind: 'approval-answer', explicit: false };
  return { kind: 'unknown' };
}

export interface VoiceUtteranceOutcome {
  intent: VoiceUtteranceIntent;
  /** True when the W05 structured approval card must be surfaced. */
  approvalSurfaced: boolean;
  /** User-facing hint (why nothing executed, what to press instead). */
  notice?: string;
}

export interface VoiceUtteranceHandlerInput {
  controls: VoiceControls;
  /** Whether a W05 pending interaction is currently awaiting a decision. */
  hasPendingApproval(): boolean;
  /**
   * Surfacing the card NEVER approves it — this callback may scroll/highlight
   * the pending interaction; there is deliberately no approve port here.
   */
  surfaceApprovalCard?(): void;
}

/**
 * Builds the utterance dispatcher the realtime provider's transcripts ride.
 * Audio-stop / end-voice / cancel go to the separated controls; approval
 * answers only surface the W05 card — the decision itself stays on the
 * card's 批准/拒绝 buttons.
 */
export function createVoiceUtteranceHandler(input: VoiceUtteranceHandlerInput): (text: string) => VoiceUtteranceOutcome {
  return (text: string): VoiceUtteranceOutcome => {
    const intent = classifyVoiceUtterance(text);
    switch (intent.kind) {
      case 'cancel-task':
        // The one explicit product cancel path.
        void input.controls.cancelTask().catch(() => undefined);
        return { intent, approvalSurfaced: false };
      case 'end-voice':
        void input.controls.endVoice().catch(() => undefined);
        return { intent, approvalSurfaced: false };
      case 'interrupt-playback':
        input.controls.interrupt();
        return { intent, approvalSurfaced: false };
      case 'approval-answer': {
        if (!input.hasPendingApproval()) {
          return { intent, approvalSurfaced: false, notice: '当前没有等待确认的操作' };
        }
        // Ambiguous or explicit: the spoken answer never decides a high-risk
        // operation. The structured approval card is the only decision
        // surface; this handler has no approve port at all.
        input.surfaceApprovalCard?.();
        return { intent, approvalSurfaced: true, notice: '高风险操作请在审批卡上手动确认' };
      }
      default:
        return { intent, approvalSurfaced: false, notice: '未识别的语音指令' };
    }
  };
}

// ---------------------------------------------------------------------------
// Product admission adapter: the W30 three endpoints behind the authSession.
// ---------------------------------------------------------------------------

interface VoiceSessionEnvelope {
  success?: unknown;
  data?: {
    id?: unknown;
    token?: unknown;
    expires_at?: unknown;
    max_seconds?: unknown;
    token_issued?: unknown;
  };
}

export interface ProductVoiceSessionApiInput {
  origin: string;
  credential: BearerCredential;
  /** Carried for the authSession-wrapped transport (W25 precedent). */
  authSession: ProductAuthSession;
  /** Injectable fetch for harness tests; defaults to the global fetch. */
  fetcher?: FetchLike;
  /** Requested billing cap for one voice session (1..600; default 300). */
  maxSeconds?: number;
}

export interface ProductVoiceSessionApi {
  admit(sessionID: string, runID?: string): Promise<VoiceSessionAdmission>;
  release(id: string): Promise<void>;
}

/**
 * Maps the W30 wire onto the grant contract: `expires_at` (RFC3339) becomes
 * `grant.expiresAt`, the one-time `token` becomes `grant.token`, and a
 * replay answer without a token reports `tokenIssued: false` instead of
 * inventing one. Transport policy follows the W25/W30 product-transcriber
 * precedent: the product bearer rides the request, the provider key never
 * leaves the server.
 */
export function createProductVoiceSessionApi(input: ProductVoiceSessionApiInput): ProductVoiceSessionApi {
  const transport = createJsonTransport(input.fetcher ?? fetch);
  const base = input.origin.replace(/\/+$/, '');
  const maxSeconds = input.maxSeconds ?? 300;
  const headers = { authorization: `Bearer ${input.credential.accessToken}`, 'content-type': 'application/json', accept: 'application/json' };

  const admit = async (sessionID: string, runID?: string): Promise<VoiceSessionAdmission> => {
    const result = await transport.send({
      method: 'POST',
      url: `${base}/api/v1/mobile/voice/sessions`,
      headers,
      body: { session_id: sessionID, ...(runID ? { run_id: runID } : {}), max_seconds: maxSeconds },
    });
    if (result.status < 200 || result.status >= 300) {
      // Errors never carry the token; only the HTTP status is surfaced.
      throw new RealtimeVoiceError('ADMISSION_HTTP', `ADMISSION_HTTP_${result.status}`);
    }
    const root = result.body as VoiceSessionEnvelope | undefined;
    const data = root?.data;
    if (!root || typeof root !== 'object' || root.success !== true || !data || typeof data !== 'object' || typeof data.id !== 'string' || data.id === '') {
      throw new RealtimeVoiceError('ADMISSION_RESPONSE_INVALID');
    }
    const expiresAt = data.expires_at;
    if (typeof expiresAt !== 'string' || !Number.isFinite(Date.parse(expiresAt))) {
      throw new RealtimeVoiceError('ADMISSION_RESPONSE_INVALID');
    }
    const token = data.token;
    const tokenIssued = typeof token === 'string' && token !== '' && data.token_issued !== false;
    return { id: data.id, grant: { token: tokenIssued ? token : '', expiresAt }, tokenIssued };
  };

  const release = async (id: string): Promise<void> => {
    const result = await transport.send({
      method: 'DELETE',
      url: `${base}/api/v1/mobile/voice/sessions/${encodeURIComponent(id)}`,
      headers,
    });
    // Idempotent by contract: a 404 for an already-gone session is a
    // successful release for our purposes.
    if (result.status >= 200 && result.status < 300) return;
    if (result.status === 404) return;
    throw new RealtimeVoiceError('RELEASE_HTTP', `RELEASE_HTTP_${result.status}`);
  };

  return { admit, release };
}

/**
 * Fail-closed realtime provider port: no native realtime media module is
 * linked in this app yet (real-supplier integration is blocked-env), so the
 * production assembly point mounts this seam until the provider adapter
 * lands. `connect` fails with the typed PROVIDER_UNAVAILABLE after the W30
 * admission succeeded, and the session controller's failure path releases
 * the granted session (settle, no usage) — the same typed-failure pattern
 * W29 shipped for the pre-W30 transcription seam. The media surface ops
 * (`mute`/`stopAudio`) are inert on an unconnected session.
 */
export function createUnavailableRealtimeVoicePort(): RealtimeVoicePort {
  return {
    async connect() {
      throw new RealtimeVoiceError('PROVIDER_UNAVAILABLE', 'PROVIDER_UNAVAILABLE');
    },
    async close() { /* nothing was ever connected */ },
    mute() { /* no media leg to gate */ },
  };
}
