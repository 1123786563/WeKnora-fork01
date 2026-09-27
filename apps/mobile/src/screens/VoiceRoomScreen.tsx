import { Button, ScrollView, Text, TextInput, View } from 'react-native';
import type { VoiceRoomViewState } from '../voice-room-view.ts';
import { VOICE_ROOM_APPROVAL_COPY, VOICE_ROOM_NOTICE_COPY, VOICE_ROOM_PHASE_COPY } from '../voice-room-view.ts';

export interface VoiceRoomScreenProps {
  state?: VoiceRoomViewState;
  onBeginTurn?(): void;
  onEndTurn?(): void;
  onEditTranscript?(text: string): void;
  onConfirmTranscript?(): void;
  onRetrySubmit?(): void;
  onDiscardTurn?(): void;
  onResume?(): void;
  onLeave?(): void;
}

/** 绑定 Task 的 Voice Room 屏（module-seams §10）：只见 VoiceRoomViewState + 回调，
 * 不见 wire、request id、turnId。已确认文字作为语音交互记录常驻可见（CONTEXT.md:339）。 */
export function VoiceRoomScreen({ state, onBeginTurn, onEndTurn, onEditTranscript, onConfirmTranscript, onRetrySubmit, onDiscardTurn, onResume, onLeave }: VoiceRoomScreenProps) {
  if (state === undefined) {
    return (
      <View>
        <Text>请先登录并激活空间，再加入语音房。</Text>
      </View>
    );
  }
  const pending = state.turns.find((turn) => turn.turnId === state.pendingTurnId);
  return (
    <ScrollView>
      <Text>语音房 · 任务 {state.taskId}{state.runId === undefined ? '' : ` · Run ${state.runId}`}</Text>
      <Text>{VOICE_ROOM_PHASE_COPY[state.phase]}</Text>
      {state.notice !== undefined && <Text>{VOICE_ROOM_NOTICE_COPY[state.notice.reason]}</Text>}
      {state.sessionId !== undefined && <Text numberOfLines={1}>会话有效期至 {state.sessionExpiresAt ?? '—'}</Text>}
      <Text>{VOICE_ROOM_APPROVAL_COPY}</Text>

      {state.phase === 'idle' && <Button title="开始说话" onPress={() => { onBeginTurn?.(); }} />}
      {state.phase === 'listening' && <Button title="结束本轮" onPress={() => { onEndTurn?.(); }} />}
      {state.phase === 'transcribing' && <Text>转写中，请稍候…</Text>}
      {state.phase === 'interrupted' && <Button title="恢复语音房" onPress={() => { onResume?.(); }} />}
      {(state.phase === 'idle' || state.phase === 'interrupted' || state.phase === 'ready') && <Button title="离开语音房" onPress={() => { onLeave?.(); }} />}

      {pending !== undefined && (
        <View>
          <Text>本轮转写（可校对）</Text>
          <TextInput value={pending.transcript} onChangeText={(text) => { onEditTranscript?.(text); }} multiline />
          <Button title="确认写入任务" disabled={state.submitting === true || pending.transcript.trim() === ''} onPress={() => { onConfirmTranscript?.(); }} />
          <Button title="放弃本轮" onPress={() => { onDiscardTurn?.(); }} />
        </View>
      )}
      {state.lastSubmitError !== undefined && (
        <View>
          <Text>{state.lastSubmitError}</Text>
          <Button title="重试写入任务" disabled={state.submitting === true} onPress={() => { onRetrySubmit?.(); }} />
        </View>
      )}

      {(state.turns.filter((turn) => turn.state === 'confirmed')).length > 0 && <Text>已确认的文字（写入任务）</Text>}
      {[...state.turns].reverse().map((turn) => (
        <View key={turn.turnId}>
          <Text numberOfLines={2}>{turn.state === 'confirmed' ? `✓ ${turn.transcript}` : turn.state === 'review' ? '待确认…' : '已放弃'}</Text>
        </View>
      ))}
    </ScrollView>
  );
}
