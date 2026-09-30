/**
 * Voice Room 深模块（Issue #57 / T27）——module-seams §8「Voice Room Module」的实时会话面。
 * 所有权（§8.1）：实时语音 session、microphone permission、音频状态、转写草稿与断线结束语义。
 * 不拥有 Task 权限、审批或时间线：确认后的文字以 steer TaskIntent 形态交还宿主，
 * 由宿主经 TaskHandle.act 提交（spec §8.1 逐字——本模块没有任何提交/决定通道）。
 *
 * 非协商不变量：
 *  - 语音会话经产品 API 授权的短期 grant 承载；provider 令牌在 Adapter 边界即被剥离，
 *    绝不进入模块状态（W30：internal/handler/mobile_voice.go:37-39「token exists exactly once」）。
 *  - AC2：confirmTranscript 的返回类型收窄为 { kind: 'steer'; text: string }——类型系统
 *    本身排除 decision/stop/queue-next；语音通道结构性不具备审批能力。
 *  - AC1：断线（转写失败/会话开启失败/scope 撤销）如实呈现；leave() 明确结束
 *    （服务端 stop/settle 幂等可重放，mobile_voice.go:359-420）；resume() 以新
 *    productSessionId 恢复——旧会话已结束，复用它只会撞上服务端 closed 绑定 409。
 *  - 原始音频默认删除（CONTEXT.md:339）：音频引用只存活于 endTurn 的转写在途窗口；
 *    每个终态恰好一次 audioDisposition.onDiscarded；模块状态只有转写文本，绝无音频字段。
 */

import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import type { DictationAudio, DictationCapturePort } from '../voice/dictation.ts';

/** 服务端单次语音会话窗口上限（internal/handler/mobile_voice.go:46 voiceSessionMaxSeconds）。 */
export const VOICE_ROOM_MAX_SECONDS = 600;
/** 单轮转写音频上限，与服务端上传上限一致（mobile_voice.go:51 voiceMaxAudioBytes）。 */
export const VOICE_ROOM_MAX_AUDIO_BYTES = 16 << 20;

/** 会话授权的模块侧视图：不含 token——media 凭据在 Adapter 边界即剥离。 */
export interface VoiceSessionGrant {
  id: string;
  expiresAt: string;
  maxSeconds: number;
}

export interface VoiceSessionEndReceipt {
  id: string;
  state: string;
  settled: boolean;
  /** 服务端幂等重放（同一会话的第二次 stop）时为 true。 */
  replay?: boolean;
}

/** Realtime Voice Port（module-seams §8.3）的会话生命周期面。今天的真实 Adapter 是 W30
 * REST 会话面（open=POST /api/v1/mobile/voice/sessions、end=DELETE …/sessions/:id）；
 * 未来 WebSocket/WebRTC Adapter 实现同一 Port（spec：「WebSocket or WebRTC is reserved
 * for real-time voice」），本模块零改动。 */
export interface VoiceSessionPort {
  open(input: { productSessionId: string; runId?: string; maxSeconds?: number }): Promise<VoiceSessionGrant>;
  end(sessionId: string): Promise<VoiceSessionEndReceipt>;
}

/** 单轮转写 Port：#56 DictationTranscriptionPort 加可选 voiceSessionId 绑定（用量记到
 * 自己的语音会话，mobile_voice.go:503-520）。api-client 的 createMobileVoiceTranscriptionRemote
 * 结构性满足本接口（apps/mobile 类型证明测试落实）。 */
export interface VoiceTurnTranscriptionPort {
  transcribe(input: { requestId: string; audio: DictationAudio; voiceSessionId?: string }): Promise<{ text: string; audioSeconds?: number }>;
}

/** 原始音频处置观察（AC1「原始音频默认删除」的可审计出口）：每轮被模块收留过的音频到达
 * 终态恰好回调一次。模块默认策略是纯删除；限期留存（空间显式启用 + 参与者提示，CONTEXT.md:339）
 * 属未来功能，届时以 Scoped Vault Adapter 实现的独立 Retention Port 承载，不改本模块。 */
export interface VoiceAudioDisposition {
  onDiscarded(turnId: string, reason: 'transcribed' | 'failed' | 'cancelled' | 'session-ended'): void;
}

export type VoiceRoomPhase = 'idle' | 'connecting' | 'listening' | 'transcribing' | 'ready' | 'interrupted' | 'ended';
export type VoiceRoomNoticeReason = 'session-open-failed' | 'transcription-failed' | 'permission-denied' | 'capture-failed' | 'audio-too-large' | 'scope-revoked';

/** 轮次视图：只有文本与状态，绝无原始音频（AC1 的类型级事实）。 */
export interface VoiceTurnView {
  turnId: string;
  transcript: string;
  state: 'review' | 'confirmed' | 'discarded';
  confirmedAt?: string;
}

export interface VoiceRoomState {
  phase: VoiceRoomPhase;
  taskId: string;
  runId?: string;
  sessionId?: string;
  sessionExpiresAt?: string;
  /** 最近一次服务端结束的结算事实（「明确结束」的可观测面；scope 撤销收敛无此字段）。 */
  lastLeave?: { settled: boolean; state: string; at: string };
  notice?: { reason: VoiceRoomNoticeReason; at: string };
  /** 语音交互记录（CONTEXT.md:339）：已确认文字跨断线/leave 保留；原始音频从不入内。 */
  turns: VoiceTurnView[];
  /** 当前待确认（review）的轮次 id。 */
  pendingTurnId?: string;
}

export type VoiceRoomErrorCode = 'VOICE_ROOM_SCOPE_CHANGED' | 'VOICE_ROOM_INVALID_INPUT';

export class VoiceRoomError extends Error {
  readonly code: VoiceRoomErrorCode;
  constructor(code: VoiceRoomErrorCode, message: string) {
    super(message);
    this.code = code;
  }
}

/** 确认意图的类型收窄：结构同 TaskIntent 的 steer 分支（可赋值），但排除其它分支——
 * 语音通道在类型层面就不产生 stop/queue-next，更不产生 decision。 */
export interface VoiceSteerIntent {
  kind: 'steer';
  text: string;
}

export interface VoiceHandle {
  state(): VoiceRoomState;
  subscribe(listener: (state: VoiceRoomState) => void): () => void;
  /** 开始一轮发言（权限请求 + 捕获；无会话时在捕获成功后开会话）。仅 idle 态可进入。 */
  beginTurn(): Promise<void>;
  /** 结束本轮并派发转写；结果进 review（可编辑）。转写失败按断线语义明确结束。 */
  endTurn(): Promise<void>;
  /** 编辑 review 中的转写文本（module-seams「转写草稿」所有权）。 */
  editTranscript(turnId: string, text: string): void;
  /** AC2：确认一轮转写——返回 steer 意图交宿主 act；空白确认返回 undefined 并保持 review。 */
  confirmTranscript(turnId: string): VoiceSteerIntent | undefined;
  /** 放弃 review 中的轮次（文字留档为 discarded）。 */
  discardTurn(turnId: string): void;
  /** AC1 恢复：丢弃旧代次、收尾旧会话（如仍挂起）、以新 productSessionId 重新授权。 */
  resume(): Promise<void>;
  /** AC1 明确结束：取消捕获、丢弃音频、review 轮次留档为 discarded、服务端 stop/settle。幂等。 */
  leave(reason?: VoiceRoomNoticeReason): Promise<void>;
  dispose(): void;
}

export interface VoiceRoom {
  /** 加入绑定 Task 的语音房（spec §8.2 join）。lease 缺席/已撤销 fail closed。 */
  join(input: { taskId: string; runId?: string }): VoiceHandle;
}

export interface VoiceRoomPorts {
  session: VoiceSessionPort;
  transcribe: VoiceTurnTranscriptionPort;
  /** Audio Device Port（§8.3）：apps/mobile 复用 #56 的原生捕获 Adapter。 */
  capture: DictationCapturePort;
  /** 轮次幂等身份（即转写 request_id；服务端按 request_id 幂等重放）。 */
  newRequestId(): string;
  /** product session id（服务端 unknown-pending 门与幂等授权的产品侧身份）。 */
  newSessionId(): string;
  /** 存活观察的 scope lease（Runtime 铸造；每个异步边界复查）。 */
  lease(): ScopeLease | undefined;
  audioDisposition?: VoiceAudioDisposition;
  /** 会话请求窗口；缺省 VOICE_ROOM_MAX_SECONDS（服务端 600s 上限）。 */
  maxSeconds?: number;
}

const nowIso = (): string => new Date().toISOString();

type DiscardReason = Parameters<VoiceAudioDisposition['onDiscarded']>[1];

function createVoiceHandle(taskId: string, runId: string | undefined, ports: VoiceRoomPorts): VoiceHandle {
  const maxSeconds = ports.maxSeconds ?? VOICE_ROOM_MAX_SECONDS;
  const disposition = ports.audioDisposition;
  let sessionId_: string | undefined;
  let state: VoiceRoomState = { phase: 'idle', taskId, ...(runId === undefined ? {} : { runId }), turns: [] };
  let generation = 0; // 会话代次：leave/resume/scope 撤销推进；旧代次迟到结果整代丢弃
  let currentTurnId: string | undefined;
  let beginning = false;
  let leaving = false;
  const listeners = new Set<(state: VoiceRoomState) => void>();
  const publish = (next: VoiceRoomState): void => {
    state = next;
    for (const listener of [...listeners]) listener(state);
  };
  const amend = (patch: Partial<VoiceRoomState>): void => publish({ ...state, ...patch });
  const live = (attemptGeneration: number): boolean => attemptGeneration === generation && leaseActive(ports.lease());

  const discardAudio = (reason: DiscardReason): void => {
    if (currentTurnId !== undefined) disposition?.onDiscarded(currentTurnId, reason);
  };

  /** scope 在边界处失效：本地收敛为 ended，绝不发起任何服务端调用（fail closed）。 */
  const collapseOnScopeLoss = (attemptGeneration: number): boolean => {
    if (live(attemptGeneration)) return false;
    discardAudio('cancelled');
    currentTurnId = undefined;
    publish({ ...state, phase: 'ended', pendingTurnId: undefined, sessionId: undefined, sessionExpiresAt: undefined, notice: { reason: 'scope-revoked', at: nowIso() } });
    return true;
  };

  async function endSessionQuietly(): Promise<void> {
    const id = sessionId_;
    if (id === undefined) return;
    sessionId_ = undefined;
    amend({ sessionId: undefined, sessionExpiresAt: undefined });
    try {
      const receipt = await ports.session.end(id);
      amend({ lastLeave: { settled: receipt.settled, state: receipt.state, at: nowIso() } });
    } catch {
      // 断线收尾是 best-effort：服务端 stop 幂等、reservation 有自身 deadline 兜底
    }
  }

  async function openSession(attemptGeneration: number): Promise<void> {
    amend({ phase: 'connecting', notice: undefined });
    try {
      const grant = await ports.session.open({ productSessionId: ports.newSessionId(), ...(runId === undefined ? {} : { runId }), maxSeconds });
      if (collapseOnScopeLoss(attemptGeneration)) {
        // 防御性收尾：这个刚打开的会话无人认领，尽力让服务端停掉（scope 已失则请求失败，无副作用）
        void ports.session.end(grant.id).catch(() => undefined);
        return;
      }
      sessionId_ = grant.id;
      amend({ phase: 'idle', sessionId: grant.id, sessionExpiresAt: grant.expiresAt });
    } catch {
      if (!live(attemptGeneration)) return;
      amend({ phase: 'interrupted', notice: { reason: 'session-open-failed', at: nowIso() } });
    }
  }

  const handle: VoiceHandle = {
    state: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    async beginTurn() {
      // 一口一轮：ready（待确认）/connecting/listening/transcribing/interrupted/ended 都不从
      if (beginning || leaving || state.phase !== 'idle') return;
      beginning = true;
      const attemptGeneration = generation;
      try {
        // 先请求权限/开始捕获，成功后才开会话——拒权绝不留下一个已计费的空会话
        let started: 'recording' | 'denied';
        try {
          started = await ports.capture.start();
        } catch {
          if (live(attemptGeneration)) amend({ notice: { reason: 'capture-failed', at: nowIso() } });
          return;
        }
        if (!live(attemptGeneration)) {
          try {
            await ports.capture.cancel();
          } catch {
            /* 收尾失败不影响代次失效 */
          }
          return;
        }
        if (started === 'denied') {
          amend({ notice: { reason: 'permission-denied', at: nowIso() } });
          return;
        }
        if (sessionId_ === undefined) {
          await openSession(attemptGeneration);
          if (state.phase !== 'idle' || sessionId_ === undefined) {
            // 开会话失败：立即收掉已开始的捕获（不给一个没有会话的录音窗）
            try {
              await ports.capture.cancel();
            } catch {
              /* 收尾失败不改变已呈现的失败态 */
            }
            return;
          }
        }
        amend({ phase: 'listening', notice: undefined });
      } finally {
        beginning = false;
      }
    },
    async endTurn() {
      if (state.phase !== 'listening') return;
      const attemptGeneration = generation;
      let audio: DictationAudio | undefined;
      try {
        audio = await ports.capture.stop();
      } catch {
        audio = undefined;
      }
      if (collapseOnScopeLoss(attemptGeneration)) return;
      if (audio === undefined || (audio.bytes === undefined && audio.uri === undefined)) {
        amend({ phase: 'idle', notice: { reason: 'capture-failed', at: nowIso() } });
        return;
      }
      if (audio.bytes !== undefined && audio.bytes.byteLength > VOICE_ROOM_MAX_AUDIO_BYTES) {
        amend({ phase: 'idle', notice: { reason: 'audio-too-large', at: nowIso() } });
        return; // 未进入转写窗口：音频从未被模块收留，无处置回调
      }
      const id = ports.newRequestId();
      currentTurnId = id;
      amend({ phase: 'transcribing', pendingTurnId: id });
      const boundSessionId = sessionId_;
      try {
        const result = await ports.transcribe.transcribe({ requestId: id, audio, ...(boundSessionId === undefined ? {} : { voiceSessionId: boundSessionId }) });
        if (collapseOnScopeLoss(attemptGeneration)) return; // 迟到结果整代丢弃（含音频处置已由收尾方完成）
        const text = typeof result.text === 'string' ? result.text.trim() : '';
        if (text === '') {
          // 空转写无重试价值（同 id 重放仍为空）；按断线语义明确结束
          discardAudio('failed');
          currentTurnId = undefined;
          amend({ phase: 'interrupted', pendingTurnId: undefined, notice: { reason: 'transcription-failed', at: nowIso() } });
          void endSessionQuietly();
          return;
        }
        discardAudio('transcribed');
        currentTurnId = undefined;
        amend({
          phase: 'ready',
          turns: [...state.turns, { turnId: id, transcript: text, state: 'review' as const }],
          pendingTurnId: id,
        });
      } catch {
        if (collapseOnScopeLoss(attemptGeneration)) return;
        // AC1 断线明确结束：转写失败 = 会话上下文不可信 → interrupted + 服务端 stop/settle
        discardAudio('failed');
        currentTurnId = undefined;
        amend({ phase: 'interrupted', pendingTurnId: undefined, notice: { reason: 'transcription-failed', at: nowIso() } });
        void endSessionQuietly();
      }
    },
    editTranscript(editId, text) {
      if (state.pendingTurnId !== editId) return;
      publish({ ...state, turns: state.turns.map((turn) => (turn.turnId === editId && turn.state === 'review' ? { ...turn, transcript: text } : turn)) });
    },
    confirmTranscript(confirmId) {
      if (state.pendingTurnId !== confirmId) return undefined;
      const turn = state.turns.find((entry) => entry.turnId === confirmId);
      if (turn === undefined || turn.state !== 'review') return undefined;
      const text = turn.transcript.trim();
      if (text === '') return undefined; // 空文本不产生指令（宿主提示可编辑或放弃）
      const at = nowIso();
      publish({
        ...state,
        phase: 'idle',
        pendingTurnId: undefined,
        turns: state.turns.map((entry) => (entry.turnId === confirmId ? { ...entry, transcript: text, state: 'confirmed' as const, confirmedAt: at } : entry)),
      });
      return { kind: 'steer', text };
    },
    discardTurn(discardId) {
      if (state.pendingTurnId !== discardId) return;
      publish({
        ...state,
        phase: 'idle',
        pendingTurnId: undefined,
        turns: state.turns.map((entry) => (entry.turnId === discardId ? { ...entry, state: 'discarded' as const } : entry)),
      });
    },
    async resume() {
      if (leaving || state.phase === 'connecting') return;
      generation += 1; // 旧会话的一切迟到结果整代丢弃
      discardAudio(state.phase === 'listening' ? 'cancelled' : 'failed');
      currentTurnId = undefined;
      amend({ pendingTurnId: undefined });
      if (state.phase === 'listening') {
        try {
          await ports.capture.cancel();
        } catch {
          /* 取消失败不阻断恢复 */
        }
      }
      await endSessionQuietly();
      await openSession(generation);
    },
    async leave(reason) {
      if (leaving) return;
      leaving = true;
      generation += 1;
      discardAudio(state.phase === 'listening' ? 'cancelled' : 'session-ended');
      currentTurnId = undefined;
      const at = nowIso();
      const wasListening = state.phase === 'listening';
      publish({
        ...state,
        phase: 'ended',
        pendingTurnId: undefined,
        turns: state.turns.map((entry) => (entry.state === 'review' ? { ...entry, state: 'discarded' as const } : entry)),
        notice: reason === undefined ? state.notice : { reason, at },
      });
      if (wasListening) {
        try {
          await ports.capture.cancel();
        } catch {
          /* 取消失败不阻断收尾 */
        }
      }
      await endSessionQuietly();
    },
    dispose() {
      void handle.leave().catch(() => undefined);
      listeners.clear();
    },
  };
  return handle;
}

export function createVoiceRoom(ports: VoiceRoomPorts): VoiceRoom {
  return {
    join(input) {
      const taskId = input.taskId.trim();
      if (taskId === '') throw new VoiceRoomError('VOICE_ROOM_INVALID_INPUT', 'taskId is required');
      if (!leaseActive(ports.lease())) throw new VoiceRoomError('VOICE_ROOM_SCOPE_CHANGED', 'no active scope lease');
      const runId = input.runId === undefined ? undefined : input.runId.trim();
      return createVoiceHandle(taskId, runId === '' ? undefined : runId, ports);
    },
  };
}
