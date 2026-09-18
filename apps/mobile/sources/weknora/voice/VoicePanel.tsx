/**
 * W31 — realtime voice panel: three deliberately separated controls.
 *
 * While the voice connection is up the panel offers exactly the three
 * affordances the product separates:
 * - 静音 (mute the microphone; media-surface only),
 * - 停止播放 (barge-in: stop the assistant audio; NO cancel API),
 * - 结束语音连接 (close the provider session; the run keeps executing).
 *
 * 取消任务 is NOT an audio control: it is an explicit product command with
 * its own button, gated by the capability surface (the W37 protocol gate
 * keeps it disabled unless control commands are allowed) and only offered
 * while a run is actually active.
 *
 * The `utterance` prop is the policy seam the realtime provider's user
 * transcripts ride: the composed W31 handler (see ConversationScreen)
 * classifies the text, routes audio-stop/end/cancel to these same separated
 * controls, and surfaces — never approves — the W05 structured approval
 * card. `notice` renders that outcome so a spoken answer tells the user to
 * press the card instead of silently doing nothing.
 */
import * as React from 'react';
import { Pressable, Text, View } from 'react-native';
import type { RealtimeVoiceSession, RealtimeVoiceState, VoiceControls, VoiceUtteranceOutcome } from './realtime';

export interface VoicePanelProps {
  session: RealtimeVoiceSession;
  controls: VoiceControls;
  /** Cancel is a product command: W37-gated capability, run must be active. */
  canCancel: boolean;
  runActive: boolean;
  /**
   * Provider transcript seam (the composed createVoiceUtteranceHandler).
   * The native realtime adapter invokes it with each finalized user
   * utterance; nothing on this panel calls it directly.
   */
  utterance?(text: string): VoiceUtteranceOutcome;
  /** Last policy outcome hint (e.g. approval answers stay on the card). */
  notice?: string | null;
}

const stateText: Record<RealtimeVoiceState, string> = {
  idle: '语音未连接',
  connecting: '正在接通语音…',
  connected: '语音已连接',
  renewing: '正在重新准入…',
  interrupted: '语音已断开（后台或来电）',
  ended: '语音已结束',
  failed: '语音连接失败',
};

const failureText: Record<string, string> = {
  ADMISSION_HTTP: '语音准入被拒绝',
  ADMISSION_RESPONSE_INVALID: '语音准入响应无效',
  ADMISSION_NO_TOKEN: '语音会话无可用令牌',
  RELEASE_HTTP: '语音会话结算失败',
  RENEWAL_EXHAUSTED: '语音续期次数已用尽，请重新接通',
};

export function VoicePanel({ session, controls, canCancel, runActive, notice }: VoicePanelProps) {
  const [, redraw] = React.useReducer((value: number) => value + 1, 0);
  const [muted, setMuted] = React.useState(false);
  React.useEffect(() => session.subscribe(() => redraw()), [redraw, session]);
  const state = session.state();
  const failure = session.failure();
  const connected = state === 'connected';
  const toggleMute = () => {
    const next = !muted;
    setMuted(next);
    session.setMuted(next);
  };
  return (
    <View accessibilityLabel="voice-panel" style={{ flexDirection: 'column', gap: 8, paddingVertical: 6 }}>
      <Text accessibilityLabel="voice-state">{stateText[state]}</Text>
      {failure ? (
        <Text accessibilityRole="alert" accessibilityLabel="voice-failure">
          {failureText[failure.code] ?? failure.message}
        </Text>
      ) : null}
      {notice ? (
        <Text accessibilityRole="alert" accessibilityLabel="voice-utterance-notice">
          {notice}
        </Text>
      ) : null}
      {connected ? (
        <View style={{ flexDirection: 'row', gap: 8 }}>
          <Pressable accessibilityRole="button" accessibilityLabel={muted ? '取消静音' : '静音'} onPress={toggleMute}>
            <Text>{muted ? '取消静音' : '静音'}</Text>
          </Pressable>
          <Pressable accessibilityRole="button" accessibilityLabel="停止播放" onPress={() => controls.interrupt()}>
            <Text>停止播放</Text>
          </Pressable>
          <Pressable accessibilityRole="button" accessibilityLabel="结束语音" onPress={() => void controls.endVoice()}>
            <Text>结束语音连接</Text>
          </Pressable>
        </View>
      ) : (
        <Pressable accessibilityRole="button" accessibilityLabel="接通语音" onPress={() => void session.begin()}>
          <Text>接通语音</Text>
        </Pressable>
      )}
      {canCancel && runActive ? (
        <Pressable accessibilityRole="button" accessibilityLabel="取消任务" onPress={() => void controls.cancelTask()}>
          <Text>取消任务</Text>
        </Pressable>
      ) : null}
    </View>
  );
}
