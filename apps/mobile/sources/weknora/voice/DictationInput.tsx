/**
 * W29 — hold-to-talk dictation input.
 *
 * Renders the voice affordance for the conversation control panel: a
 * press-and-hold button (press-in begins the capture, press-out finishes
 * into a transcript draft), a cancel control while recording, and the
 * failure notice for every typed error. A denied microphone permission
 * only surfaces a notice — the panel's text input remains the fallback,
 * so the user can always keep typing.
 *
 * Lifecycle: any non-active AppState (backgrounding, incoming call
 * overlays) and unmounting cancel the in-flight dictation, which stops
 * the recorder and deletes the temporary audio file. Space switches are
 * handled inside the controller through the product scope seam.
 */
import * as React from 'react';
import { AppState, Pressable, Text, View } from 'react-native';
import type { DictationController, DictationState } from './dictation';

const failureText: Record<string, string> = {
  MIC_PERMISSION_DENIED: '麦克风权限被拒绝，已切回文本输入',
  RECORD_INTERRUPTED: '录音已中断，草稿已保留',
  TRANSCRIBE_TIMEOUT: '转写超时，草稿已保留',
  TRANSCRIBE_FAILED: '转写失败，草稿已保留',
  EMPTY_AUDIO: '未录到声音，草稿已保留',
  AUDIO_TOO_LONG: '录音时间过长，草稿已保留',
  SCOPE_CHANGED: '已切换空间，本次语音已取消',
};

const holdText: Record<DictationState, string> = {
  idle: '按住说话',
  recording: '松开结束',
  transcribing: '转写中…',
  ready: '按住继续说话',
  error: '按住说话',
};

export function DictationInput({ controller }: { controller: DictationController }) {
  React.useEffect(() => {
    // 来电/后台: leaving the active state cancels the in-flight dictation.
    const subscription = AppState.addEventListener('change', (next) => {
      if (next !== 'active') void controller.cancel();
    });
    return () => {
      subscription.remove();
      // Unmounting (navigation away, space switch remount) releases the
      // recorder and deletes the temporary audio.
      void controller.cancel();
    };
  }, [controller]);
  const state = controller.state();
  const failure = controller.failure();
  return (
    <View accessibilityLabel="dictation-input" style={{ flexDirection: 'row', gap: 8, alignItems: 'center' }}>
      {state === 'error' && failure ? (
        <Text accessibilityRole="alert" accessibilityLabel="dictation-error">
          {failureText[failure.code] ?? '语音输入失败，草稿已保留'}
        </Text>
      ) : null}
      <Pressable
        accessibilityRole="button"
        accessibilityLabel="按住说话"
        disabled={state === 'transcribing'}
        onPressIn={() => void controller.begin()}
        onPressOut={() => void controller.finish()}
      >
        <Text>{holdText[state]}</Text>
      </Pressable>
      {state === 'recording' ? (
        <Pressable accessibilityRole="button" accessibilityLabel="取消录音" onPress={() => void controller.cancel()}>
          <Text>取消</Text>
        </Pressable>
      ) : null}
    </View>
  );
}
