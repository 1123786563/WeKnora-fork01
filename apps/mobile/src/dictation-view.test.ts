import test from 'node:test';
import assert from 'node:assert/strict';
import type { DictationTranscriptionPort } from '@weknora/mobile-core';
import { createMobileVoiceTranscriptionRemote } from '@weknora/api-client/mobile/voice';
import { applyConfirmedDictation, DICTATION_FAILURE_COPY } from './dictation-view.ts';

test('applyConfirmedDictation appends without clobbering and caps at the goal-text limit', () => {
  assert.equal(applyConfirmedDictation('', '整理周报', 500), '整理周报', '空手打：确认文本即全部内容');
  assert.equal(applyConfirmedDictation('手写目标', '整理周报', 500), '手写目标\n整理周报', '非空手打：追加一行，不覆盖既有文字');
  const capped = applyConfirmedDictation('a'.repeat(499), '很长'.repeat(50), 500);
  assert.equal(capped.length, 500, '越界在 500 字上限处截断（保住单条意图记录的 SecureStore 预算）');
  assert.ok(capped.startsWith('a'.repeat(499)), '截断保序：既有文字优先保留');
});

test('every dictation failure surface has honest user copy', () => {
  for (const key of ['denied', 'capture-failed', 'audio-too-large', 'transcription-failed'] as const) {
    assert.equal(typeof DICTATION_FAILURE_COPY[key], 'string');
    assert.notEqual(DICTATION_FAILURE_COPY[key].trim(), '');
  }
  assert.ok(DICTATION_FAILURE_COPY['transcription-failed'].includes('不受影响'), '失败文案必须声明手打文字不受影响');
});

test('the api-client voice remote structurally satisfies the mobile-core transcription port', async () => {
  // 编译期证明（MobileVoiceAudio ≡ DictationAudio、结果结构可赋值）+ 运行期一次真实调用。
  const remote = createMobileVoiceTranscriptionRemote({
    origin: 'https://weknora.example.test',
    request: async () => ({ success: true, data: { text: '结构一致' } }),
  });
  const port: DictationTranscriptionPort = remote;
  const result = await port.transcribe({ requestId: 'req-1', audio: { bytes: new Uint8Array([1]), mimeType: 'audio/wav' } });
  assert.equal(result.text, '结构一致');
});
