import type { DictationFailure } from '@weknora/mobile-core';

/** 听写失败/拒权文案（对照 ATTENTION_RECEIPT_COPY 先例：值不得出现内部 wire 细节）。 */
export const DICTATION_FAILURE_COPY: Record<'denied' | DictationFailure, string> = {
  denied: '麦克风权限被拒绝；可直接输入文字，或在系统设置授权后重试。',
  'capture-failed': '录音不可用；可直接输入文字后重试。',
  'audio-too-large': '录音过大（超过 16 MiB）；请缩短录音。',
  'transcription-failed': '转写失败；可重试或放弃，已输入的文字不受影响。',
};

/**
 * AC2 落点：确认后的转写**追加**进目标输入（不覆盖手打文字——「拒权、取消和迟到转写不破坏
 * 文字输入」的确认侧对偶），并在 maxLength（GOAL_TEXT_MAX_LENGTH=500）处截断，保住
 * #36 的单条意图记录 SecureStore ~2048B 预算（NewTaskScreen.tsx:7-13 注释同源）。
 */
export function applyConfirmedDictation(existing: string, confirmed: string, maxLength: number): string {
  const joined = existing.trim() === '' ? confirmed : `${existing}\n${confirmed}`;
  return joined.length <= maxLength ? joined : joined.slice(0, maxLength);
}
