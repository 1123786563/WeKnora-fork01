import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createDictation,
  DICTATION_MAX_AUDIO_BYTES,
  type DictationAudio,
  type DictationPhase,
  type DictationState,
} from './dictation.ts';
import { createScenarioDictationTranscriber, createScriptedDictationCapture } from './in-memory-dictation.ts';

const audio = (): DictationAudio => ({ bytes: new Uint8Array([1, 2, 3]), mimeType: 'audio/wav', fileName: 'd.wav' });
const tick = async (): Promise<void> => { await new Promise<void>((resolve) => setImmediate(resolve)); };

test('AC2 happy path: stop → transcribe → editable review → confirm returns the edited text and never re-dispatches', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const phases: DictationPhase[] = [];
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  dictation.subscribe((next) => phases.push(next.phase));

  await dictation.begin();
  assert.equal(dictation.state().phase, 'recording');

  const finishing = dictation.finish();
  await tick(); await tick();
  assert.equal(dictation.state().phase, 'transcribing');

  transcriber.resolve(0, { text: '整理知识库并发周报', audioSeconds: 4 });
  await finishing;
  assert.equal(dictation.state().phase, 'review');
  assert.equal(dictation.state().transcript, '整理知识库并发周报');

  dictation.editTranscript('整理知识库并发周报（已校对）');
  assert.equal(dictation.state().transcript, '整理知识库并发周报（已校对）');

  assert.equal(dictation.confirmTranscript(), '整理知识库并发周报（已校对）');
  assert.equal(dictation.state().phase, 'idle');
  assert.equal(transcriber.requests.length, 1, 'AC2：确认只消费既有转写，模块绝不再次派发，也绝不提交');
  assert.deepEqual(transcriber.requests[0]!.audio.mimeType, 'audio/wav');
  assert.deepEqual(capture.calls, ['start', 'stop']);
  assert.deepEqual(phases, ['recording', 'transcribing', 'review', 'review', 'idle']);
});

test('AC1: cancelling during recording discards the audio, never dispatches transcription, and leaves the module reusable', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });

  await dictation.begin();
  await dictation.cancel();
  assert.equal(dictation.state().phase, 'idle');
  assert.deepEqual(capture.calls, ['start', 'cancel'], '取消路径不调用 stop——音频被丢弃而不是被消费');
  assert.equal(transcriber.requests.length, 0, '取消录音绝不触发转写派发（也就不可能取消/影响 Task）');

  // 模块可复用：取消后再次听写走完整流
  await dictation.begin();
  const finishing = dictation.finish();
  await tick(); await tick();
  transcriber.resolve(0, { text: '第二次听写' });
  await finishing;
  assert.equal(dictation.state().phase, 'review');
});

test('microphone denial surfaces denied without any stop or transcription work, and begin can re-request', async () => {
  const capture = createScriptedDictationCapture({ start: 'denied' });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });

  await dictation.begin();
  assert.equal(dictation.state().phase, 'denied');
  assert.deepEqual(capture.calls, ['start']);
  assert.equal(transcriber.requests.length, 0);

  await dictation.begin(); // 拒权不是终态：用户授权后可重试
  assert.equal(dictation.state().phase, 'denied');
  assert.deepEqual(capture.calls, ['start', 'start']);
});

test('discarding the review transcript returns to idle without any further dispatch', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  const finishing = dictation.finish();
  await tick(); await tick();
  transcriber.resolve(0, { text: '将被放弃' });
  await finishing;

  dictation.discardTranscript();
  assert.equal(dictation.state().phase, 'idle');
  assert.equal(dictation.state().transcript, undefined);
  assert.equal(transcriber.requests.length, 1);
});

test('confirmTranscript refuses an empty transcript and keeps the review surface', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  const finishing = dictation.finish();
  await tick(); await tick();
  transcriber.resolve(0, { text: '  ' }); // 服务端成功但空文本：没有可确认的内容
  await finishing;
  assert.equal(dictation.state().failure, 'transcription-failed', '空转写按失败处理，不产出空 review');

  await dictation.begin();
  const second = dictation.finish();
  await tick(); await tick();
  transcriber.resolve(1, { text: '有内容' });
  await second;
  dictation.editTranscript('   ');
  assert.equal(dictation.confirmTranscript(), undefined, '空白确认被拒绝');
  assert.equal(dictation.state().phase, 'review');
});

test('a late transcription that resolves after cancel is dropped entirely (it must never reach review)', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  const finishing = dictation.finish();
  await tick(); await tick();
  assert.equal(dictation.state().phase, 'transcribing');

  await dictation.cancel(); // 转写在途时取消
  assert.equal(dictation.state().phase, 'idle');

  transcriber.resolve(0, { text: '迟到的转写' }); // 服务端此刻才回包
  await finishing;
  await tick();
  assert.equal(dictation.state().phase, 'idle', '迟到结果整代丢弃');
  assert.equal(dictation.state().transcript, undefined);
});

test('cancel during a slow stop() drops the capture without dispatching transcription', async () => {
  const capture = createScriptedDictationCapture({ stop: audio(), manualStop: true });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();

  const finishing = dictation.finish(); // stop() 挂起（manualStop）
  await dictation.cancel(); // 用户在 stop 返回前取消
  capture.resolveStop(audio()); // stop 此刻才返回音频
  await finishing;
  await tick();

  assert.equal(transcriber.requests.length, 0, '已取消的停止不得派发转写（一次用户已明确取消的计费转写）');
  assert.equal(dictation.state().phase, 'idle');
});

test('retry after a transcription failure reuses the same request id (idempotent replay, never a second charge)', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  const finishing = dictation.finish();
  await tick(); await tick();
  transcriber.resolve(0, new Error('network failed after dispatch'));
  await finishing;
  assert.equal(dictation.state().phase, 'failed');
  assert.equal(dictation.state().failure, 'transcription-failed');

  const retrying = dictation.retryTranscription();
  assert.equal(dictation.state().phase, 'transcribing');
  transcriber.resolve(1, { text: '重试后的转写' });
  await retrying;
  assert.equal(transcriber.requests[1]!.requestId, 'req-1', '同一意图同一 request_id（W04 重放语义，服务端从结果行幂等应答）');
  assert.equal(dictation.state().phase, 'review');
});

test('the adapter-level failure code surfaces for presentation and evidence', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  const finishing = dictation.finish();
  await tick(); await tick();
  transcriber.resolve(0, Object.assign(new Error('voice charging is not configured'), { code: 'VOICE_CHARGING_UNCONFIGURED' }));
  await finishing;
  assert.equal(dictation.state().failure, 'transcription-failed');
  assert.equal(dictation.state().failureCode, 'VOICE_CHARGING_UNCONFIGURED');
});

test('recording auto-finishes at the configured duration cap (aligned with the 120s server budget window)', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1', maxDurationMs: 25 });
  await dictation.begin();
  assert.equal(dictation.state().phase, 'recording');
  await new Promise((resolve) => setTimeout(resolve, 80));
  assert.equal(dictation.state().phase, 'transcribing', '到点自动 finish，录音不越过服务端预算窗');
  transcriber.resolve(0, { text: '到点转写' });
  await tick();
  assert.equal(dictation.state().phase, 'review');
});

test('an oversized capture (> 16 MiB) fails closed before any upload', async () => {
  const capture = createScriptedDictationCapture({
    stop: { bytes: new Uint8Array(DICTATION_MAX_AUDIO_BYTES + 1), mimeType: 'audio/wav' },
  });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  await dictation.finish();
  assert.equal(dictation.state().failure, 'audio-too-large');
  assert.equal(transcriber.requests.length, 0, '超限在派发前拒绝，不浪费一次注定 400 的上载');
});

test('a failing capture start surfaces capture-failed and never reaches transcription', async () => {
  const capture = createScriptedDictationCapture({ start: new Error('microphone busy') });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  assert.equal(dictation.state().phase, 'failed');
  assert.equal(dictation.state().failure, 'capture-failed');
  assert.equal(transcriber.requests.length, 0);
});

test('a second begin while recording is ignored (no double capture)', async () => {
  const capture = createScriptedDictationCapture({ stop: audio() });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  await dictation.begin();
  assert.deepEqual(capture.calls, ['start'], '录音中的重复 begin 零副作用');
  await dictation.cancel(); // 清理时长定时器（否则 120s 定时器挂起测试进程）
});

test('stop without any available audio is a capture failure, not an empty upload', async () => {
  const capture = createScriptedDictationCapture({ stop: undefined });
  const transcriber = createScenarioDictationTranscriber();
  const dictation = createDictation({ capture, transcribe: transcriber, newRequestId: () => 'req-1' });
  await dictation.begin();
  await dictation.finish();
  assert.equal(dictation.state().failure, 'capture-failed');
  assert.equal(transcriber.requests.length, 0);
});

test('dropping an intent deletes the uri-sourced audio exactly once via audioCleanup (#70)', async () => {
  const deleted: string[] = [];
  const dictation = createDictation({
    capture: {
      start: async () => 'recording' as const,
      stop: async () => ({ uri: 'file:///cache/d1.m4a', mimeType: 'audio/mp4', fileName: 'd.m4a' }),
      cancel: async () => undefined,
    },
    transcribe: { async transcribe() { return { text: '转写成功' }; } },
    newRequestId: () => 'req-cleanup-1',
    audioCleanup: async (audio) => { if (audio.uri !== undefined) deleted.push(audio.uri); },
  });
  await dictation.begin();
  await dictation.finish();
  await dictation.confirmTranscript();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(deleted, ['file:///cache/d1.m4a'], '确认（dropIntent 已发生）后原始音频文件被清理恰好一次');
});

test('bytes-sourced audio never triggers audioCleanup (nothing on disk to delete)', async () => {
  let cleanups = 0;
  const dictation = createDictation({
    capture: {
      start: async () => 'recording' as const,
      stop: async () => ({ bytes: new Uint8Array([1, 2, 3]), mimeType: 'audio/wav' }),
      cancel: async () => undefined,
    },
    transcribe: { async transcribe() { return { text: 'ok' }; } },
    newRequestId: () => 'req-cleanup-2',
    audioCleanup: async () => { cleanups += 1; },
  });
  await dictation.begin();
  await dictation.finish();
  await dictation.discardTranscript();
  assert.equal(cleanups, 0, 'bytes 源音频只在内存，无文件可删');
});

test('a failed transcription retains the audio for retry; the eventual drop deletes it once', async () => {
  const deleted: string[] = [];
  let attempts = 0;
  const dictation = createDictation({
    capture: {
      start: async () => 'recording' as const,
      stop: async () => ({ uri: 'file:///cache/retry.m4a', mimeType: 'audio/mp4' }),
      cancel: async () => undefined,
    },
    transcribe: {
      async transcribe() {
        attempts += 1;
        if (attempts === 1) throw new Error('network down');
        return { text: '重试成功' };
      },
    },
    newRequestId: () => 'req-cleanup-3',
    audioCleanup: async (audio) => { if (audio.uri !== undefined) deleted.push(audio.uri); },
  });
  await dictation.begin();
  await dictation.finish();
  assert.equal(deleted.length, 0, '失败重试窗口内音频必须保留（服务端同 requestId 幂等重放），绝不提前删除');
  await dictation.retryTranscription();
  assert.equal(dictation.state().phase, 'review');
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(deleted, ['file:///cache/retry.m4a'], '重试成功进入 review 后（dropIntent）清理恰好一次');
});

test('a rejecting audioCleanup never fails the flow nor leaks an unhandled rejection (Review Focus 3)', async () => {
  const unhandled: unknown[] = [];
  const onUnhandled = (reason: unknown) => { unhandled.push(reason); };
  process.on('unhandledRejection', onUnhandled);
  const dictation = createDictation({
    capture: {
      start: async () => 'recording' as const,
      stop: async () => ({ uri: 'file:///cache/gone.m4a', mimeType: 'audio/mp4' }),
      cancel: async () => undefined,
    },
    transcribe: { async transcribe() { return { text: 'ok' }; } },
    newRequestId: () => 'req-cleanup-4',
    audioCleanup: async () => { throw new Error('EFILEGONE'); },
  });
  await dictation.begin();
  await dictation.finish();
  assert.equal(dictation.state().phase, 'review', '清理失败不得影响转写主流程');
  await new Promise((resolve) => setTimeout(resolve, 20));
  process.off('unhandledRejection', onUnhandled);
  assert.deepEqual(unhandled, [], '清理 rejection 必须被模块吞掉');
});
