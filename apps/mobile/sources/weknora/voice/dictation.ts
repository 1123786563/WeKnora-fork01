/**
 * 确认式听写（MX-028 / M16）。
 * 冻结规则：
 * - 识别文本进入**未发送草稿**（可编辑）；确认前用户可任意修改——提交的是用户编辑后的
 *   最新文本，不是识别初稿（frozen：edit-before-confirm）；
 * - 确认动作把草稿交给会话发送通道（经确认），不直接发送；
 * - 取消听写=丢弃当前会话识别（不清已确认历史）；与「停止播报」「取消任务」是三个动作。
 */

export interface DictationSession {
  /** 识别出的原始初稿（持续更新） */
  transcript: string;
  /** 用户编辑后的草稿（确认前可继续编辑；默认跟随识别追加，用户改写后停止跟随） */
  draft: string;
  userEdited: boolean;
  listening: boolean;
}

export type DictationEvent =
  | { type: 'start' }
  | { type: 'transcript'; text: string }
  | { type: 'user_edit'; text: string }
  | { type: 'stop_listening' }
  | { type: 'cancel' }
  | { type: 'confirm'; submitVia: (draft: string) => Promise<void> | void };

export type DictationResult =
  | { type: 'draft_updated'; draft: string }
  | { type: 'cancelled' }
  | { type: 'confirmed'; submitted: boolean };

/** 纯状态机：事件驱动（真实语音引擎经事件注入——不模拟识别）。 */
export function createDictationStateMachine() {
  let state: DictationSession = { transcript: '', draft: '', userEdited: false, listening: false };

  return {
    get state(): DictationSession {
      return { ...state };
    },
    dispatch(event: DictationEvent): DictationResult {
      switch (event.type) {
        case 'start': {
          state = { ...state, listening: true };
          return { type: 'draft_updated', draft: state.draft };
        }
        case 'transcript': {
          if (!state.listening) return { type: 'draft_updated', draft: state.draft };
          state = {
            ...state,
            transcript: event.text,
            // 用户未手动编辑时草稿跟随识别；编辑后停止跟随（保护用户修改）
            draft: state.userEdited ? state.draft : event.text,
          };
          return { type: 'draft_updated', draft: state.draft };
        }
        case 'user_edit': {
          state = { ...state, draft: event.text, userEdited: true };
          return { type: 'draft_updated', draft: state.draft };
        }
        case 'stop_listening': {
          state = { ...state, listening: false };
          return { type: 'draft_updated', draft: state.draft };
        }
        case 'cancel': {
          // 取消听写：丢弃本段识别与未确认草稿（已提交内容不受影响）
          state = { transcript: '', draft: '', userEdited: false, listening: false };
          return { type: 'cancelled' };
        }
        case 'confirm': {
          const finalDraft = state.draft;
          state = { transcript: '', draft: '', userEdited: false, listening: false };
          // 提交经显式 submitVia（会话发送通道自行确认）——状态机不直接发
          void Promise.resolve(event.submitVia(finalDraft)).catch(() => undefined);
          return { type: 'confirmed', submitted: true };
        }
      }
    },
  };
}

export type DictationController = ReturnType<typeof createDictationStateMachine>;
