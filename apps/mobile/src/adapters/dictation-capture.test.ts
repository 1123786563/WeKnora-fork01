import test from 'node:test';
import assert from 'node:assert/strict';
import { createDictationCaptureFrom, createNativeAudioFileCleanupIfAvailable, createNativeDictationCaptureIfAvailable, type ExpoAudioLike } from './dictation-capture.ts';

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

test('cancel deletes the recorder temp file best-effort (Review Focus 2: 文件 URI 不滞留)', async () => {
  const deleted: string[] = [];
  const cleanup = { deleteAsync: async (uri: string) => { deleted.push(uri); } };
  const capture = createDictationCaptureFrom(fakeExpoAudio(), cleanup);
  await capture.start();
  await capture.cancel();
  assert.deepEqual(deleted, ['file:///cache/dictation.m4a'], '取消路径的音频从未进入模块，临时文件由 Adapter 即刻删除');
  assert.equal(await capture.stop(), undefined, '取消后无残留捕获');
});

test('a failing temp-file deletion on cancel is swallowed (cleanup is best-effort, Review Focus 3)', async () => {
  const capture = createDictationCaptureFrom(fakeExpoAudio(), {
    deleteAsync: async () => { throw new Error('EFILEGONE'); },
  });
  await capture.start();
  await capture.cancel(); // 不得抛出
  assert.equal(await capture.stop(), undefined);
});

test('stop-produced audio is the module audioCleanup responsibility; the adapter passes cleanup through the native factory wiring (#70)', async () => {
  // stop 路径不删：返回的 uri 要被模块用于 multipart 上载，删除时点由 Dictation dropIntent 权威决定（Task 3）。
  const deleted: string[] = [];
  const capture = createDictationCaptureFrom(fakeExpoAudio(), { deleteAsync: async (uri) => { deleted.push(uri); } });
  await capture.start();
  const audio = await capture.stop();
  assert.equal(audio?.uri, 'file:///cache/dictation.m4a');
  assert.deepEqual(deleted, [], 'stop 路径的删除时点属于模块（audioCleanup 端口），Adapter 不得抢先删除');

  // 原生文件清理 Adapter 在 Node 链 fail closed（expo-file-system 未安装——这正是惰性边界）
  assert.equal(createNativeAudioFileCleanupIfAvailable(), undefined);
});
