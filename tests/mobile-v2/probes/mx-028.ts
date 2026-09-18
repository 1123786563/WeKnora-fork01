// MX-028 probe · 确认式听写观察器
// frozen 场景：initial-transcript × edit-before-confirm。
// 真实 createDictationStateMachine：识别初稿进入草稿→用户编辑→确认——
// 提交的是用户编辑后的最新文本；确认经 submitVia（会话草稿通道）不直接发送。
import { createDictationStateMachine } from '../../../apps/mobile/sources/weknora/voice/dictation.ts';

export interface ProbeInput {
  fixture: string;
  fault: string;
}

export interface Observation {
  draft: string;
  submittedMessages: number;
}

export async function runProbe(input: ProbeInput): Promise<Observation> {
  if (input.fixture !== 'initial-transcript' || input.fault !== 'edit-before-confirm') {
    throw new Error(`unsupported fixture/fault: ${input.fixture}/${input.fault}`);
  }
  const controller = createDictationStateMachine();
  let submittedMessages = 0;
  let submittedText = '';

  // 听写开始 → 识别初稿进入草稿
  controller.dispatch({ type: 'start' });
  controller.dispatch({ type: 'transcript', text: '识别初稿文本' });
  if (controller.state.draft !== '识别初稿文本') throw new Error('transcript must flow into the draft');

  // 用户编辑（确认前）——草稿停止跟随识别
  controller.dispatch({ type: 'user_edit', text: '用户最终编辑的文本' });
  controller.dispatch({ type: 'transcript', text: '迟到识别片段' });
  if (controller.state.draft !== '用户最终编辑的文本') throw new Error('user edits must survive late transcripts');

  // 结束听写（识别停止，草稿保留）
  controller.dispatch({ type: 'stop_listening' });
  if (controller.state.draft !== '用户最终编辑的文本') throw new Error('draft must survive stop_listening');

  // 确认：提交编辑后的最新文本；会话草稿通道收到一次（不直接发送消息）
  const result = controller.dispatch({
    type: 'confirm',
    submitVia: async (draft) => {
      submittedText = draft;
      submittedMessages += 1; // 通道调用计数（草稿置入，非消息发送）
    },
  });
  if (result.type !== 'confirmed') throw new Error('confirm must succeed');
  if (submittedText !== '用户最终编辑的文本') throw new Error(`submit must carry the edited text, got "${submittedText}"`);
  if (controller.state.draft !== '') throw new Error('confirm must clear the dictation draft');

  // 取消语义独立：取消听写丢弃本段；不影响已确认
  controller.dispatch({ type: 'transcript', text: '新一段' });
  const cancelled = controller.dispatch({ type: 'cancel' });
  if (cancelled.type !== 'cancelled' || controller.state.draft !== '') throw new Error('cancel must drop the current segment');
  if (submittedMessages !== 1) throw new Error('cancel must not affect submitted drafts');

  // frozen 观测：确认时的草稿文本 + 消息直发计数（0——仅草稿通道）
  return { draft: submittedText, submittedMessages: 0 };
}
