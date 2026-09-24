import test from 'node:test';
import assert from 'node:assert/strict';
import { createDictationCaptureFrom, createNativeDictationCaptureIfAvailable, type ExpoAudioLike } from './dictation-capture.ts';

interface FakeOptions {
  /** 权限结果（缺省 granted）。 */
  permission?: boolean;
  /** prepareToRecordAsync 必失败（录音机占用等）。 */
  failPrepare?: boolean;
}

/** 手写 fake（expo-audio 未安装于 Node 链——这正是 createNativeDictationCaptureIfAvailable 被测的惰性边界）。 */
function fakeExpoAudio(options: FakeOptions = {}): ExpoAudioLike {
  return {
    requestRecordingPermissionsAsync: async () => ({ granted: options.permission ?? true }),
    setAudioModeAsync: async () => undefined,
    RecordingPresets: { HIGH_QUALITY: { extension: '.m4a' } },
    AudioModule: {
      AudioRecorder: class {
        uri: string | null = null;
        async prepareToRecordAsync() {
          if (options.failPrepare === true) throw new Error('microphone busy');
          this.uri = 'file:///cache/dictation.m4a';
        }
        record(): void { /* fake */ }
        async stop(): Promise<void> { /* fake */ }
        release(): void { /* fake */ }
      },
    },
  } as unknown as ExpoAudioLike;
}

test('the native adapter is unavailable in the Node test chain (fail closed, no microphone surface)', () => {
  assert.equal(createNativeDictationCaptureIfAvailable(), undefined, 'expo-audio 不可 require 时必须返回 undefined');
});

test('granted permission records and stop returns the native file source', async () => {
  const capture = createDictationCaptureFrom(fakeExpoAudio());
  assert.equal(await capture.start(), 'recording');
  const audio = await capture.stop();
  assert.deepEqual(audio, { uri: 'file:///cache/dictation.m4a', mimeType: 'audio/mp4', fileName: 'dictation.m4a' });
});

test('denied permission never prepares a recorder', async () => {
  const capture = createDictationCaptureFrom(fakeExpoAudio({ permission: false }));
  assert.equal(await capture.start(), 'denied');
  assert.equal(await capture.stop(), undefined, '拒权路径零捕获，stop 无音频');
});

test('cancel stops and releases the recorder (raw audio is dropped, not transcribed)', async () => {
  const capture = createDictationCaptureFrom(fakeExpoAudio());
  await capture.start();
  await capture.cancel();
  assert.equal(await capture.stop(), undefined, '取消后录音机已释放，无残留捕获');
});

test('stop without a live recorder is undefined, not a throw', async () => {
  const capture = createDictationCaptureFrom(fakeExpoAudio());
  assert.equal(await capture.stop(), undefined);
});

test('a recorder failure in start rejects (the module maps it to capture-failed)', async () => {
  const capture = createDictationCaptureFrom(fakeExpoAudio({ failPrepare: true }));
  await assert.rejects(() => capture.start(), /microphone busy/);
});
