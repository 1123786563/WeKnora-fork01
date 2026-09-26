import type { VoiceHandle, VoiceRoomNoticeReason, VoiceRoomPhase, VoiceRoomState } from '@weknora/mobile-core';
import { TASK_OFFICE_ERROR_COPY } from './task-detail-view.ts';

/** 状态/通知/错误三组文案（对照 DICTATION_FAILURE_COPY 先例：不直出内部码）。 */
export const VOICE_ROOM_PHASE_COPY: Record<VoiceRoomPhase, string> = {
  idle: '语音房已就绪，按下开始说话',
  connecting: '正在加入语音房…',
  listening: '正在聆听…',
  transcribing: '正在转写…',
  ready: '转写完成，可校对后确认',
  interrupted: '语音房已中断：会话已明确结束，可恢复',
  ended: '语音房已结束',
};

export const VOICE_ROOM_NOTICE_COPY: Record<VoiceRoomNoticeReason, string> = {
  'session-open-failed': '语音会话授权失败（可能未配置语音计价或预算不足）；可稍后恢复。',
  'transcription-failed': '本轮转写失败，语音会话已明确结束；恢复后将开启新会话。',
  'permission-denied': '麦克风权限被拒绝；可在系统设置授权后重试。',
  'capture-failed': '录音不可用；可重试本轮。',
  'audio-too-large': '本轮录音过大（超过 16 MiB）；请缩短发言。',
  'scope-revoked': '登录状态或活动空间已变化，语音房已结束；请重新进入。',
};

export const VOICE_ROOM_SUBMIT_ERROR_PREFIX = '指令提交失败：';

/** 审批互斥的常驻呈现（AC2）：语音房内明确告知确认文字的走向与边界。 */
export const VOICE_ROOM_APPROVAL_COPY = '语音确认的文字以「调整指令」进入任务时间线；高风险审批只能在行动收件箱完成，语音无法代替审批。';

export interface VoiceRoomViewState extends VoiceRoomState {
  submitting?: boolean;
  lastSubmitError?: string;
}

export interface VoiceRoomControllerPorts {
  handle: VoiceHandle;
  /** 确认文字的宿主提交通道：TaskHandle.act（spec §8.1）。模块本身零提交。 */
  onConfirmIntent(intent: { kind: 'steer'; text: string }): Promise<unknown>;
}

export interface VoiceRoomController {
  state(): VoiceRoomViewState;
  subscribe(listener: (state: VoiceRoomViewState) => void): () => void;
  beginTurn(): Promise<void>;
  endTurn(): Promise<void>;
  editTranscript(text: string): void;
  confirmTranscript(): Promise<void>;
  discardTurn(): void;
  resume(): Promise<void>;
  leave(): Promise<void>;
  dispose(): void;
}

/** act 失败 → 用户文案。用 .code 形状查表而非 instanceof——tsx 的 CJS/ESM 双实例下
 * instanceof 跨模块不可靠（本计划验证期实锤）；TaskOfficeError 的判别特征就是 .code。 */
const submitMessageOf = (failure: unknown): string => {
  const code = typeof failure === 'object' && failure !== null && 'code' in failure ? (failure as { code?: unknown }).code : undefined;
  if (typeof code === 'string' && TASK_OFFICE_ERROR_COPY[code] !== undefined) {
    return `${VOICE_ROOM_SUBMIT_ERROR_PREFIX}${TASK_OFFICE_ERROR_COPY[code]}`;
  }
  return `${VOICE_ROOM_SUBMIT_ERROR_PREFIX}${failure instanceof Error ? failure.message : String(failure)}`;
};

/** 语音房控制器：把 pending 轮次 id 封装在内部（屏只见文本与状态），确认动作直通宿主 act 通道。 */
export function createVoiceRoomController(ports: VoiceRoomControllerPorts): VoiceRoomController {
  let state: VoiceRoomViewState = { ...ports.handle.state() };
  let disposed = false;
  const listeners = new Set<(state: VoiceRoomViewState) => void>();
  const publish = (next: VoiceRoomViewState): void => {
    state = next;
    for (const listener of [...listeners]) listener(state);
  };
  const unsubscribe = ports.handle.subscribe((next) => {
    if (!disposed) publish({ submitting: state.submitting, lastSubmitError: state.lastSubmitError, ...next });
  });
  const pendingId = (): string | undefined => ports.handle.state().pendingTurnId;
  return {
    state: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    beginTurn() {
      publish({ ...state, lastSubmitError: undefined });
      return ports.handle.beginTurn();
    },
    endTurn() {
      return ports.handle.endTurn();
    },
    editTranscript(text) {
      const id = pendingId();
      if (id !== undefined) ports.handle.editTranscript(id, text);
    },
    async confirmTranscript() {
      const id = pendingId();
      if (id === undefined || state.submitting === true) return;
      const intent = ports.handle.confirmTranscript(id);
      if (intent === undefined) return;
      publish({ ...state, submitting: true, lastSubmitError: undefined });
      try {
        await ports.onConfirmIntent(intent);
        publish({ ...state, submitting: false });
      } catch (failure) {
        publish({ ...state, submitting: false, lastSubmitError: submitMessageOf(failure) });
      }
    },
    discardTurn() {
      const id = pendingId();
      if (id !== undefined) ports.handle.discardTurn(id);
    },
    resume() {
      return ports.handle.resume();
    },
    leave() {
      return ports.handle.leave();
    },
    dispose() {
      if (disposed) return;
      disposed = true;
      unsubscribe();
      listeners.clear();
      ports.handle.dispose();
    },
  };
}
