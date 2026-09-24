import test from 'node:test';
import assert from 'node:assert/strict';
import { createMobileVoiceTranscriptionRemote } from './voice.ts';
import type { ClientRequest } from '../client.ts';

const ORIGIN = 'https://weknora.example.test';
const WAV = { bytes: new Uint8Array([1, 2, 3]), mimeType: 'audio/wav', fileName: 'd.wav' } as const;

test('transcribe posts a multipart body with request_id and an audio file part to the voice endpoint', async () => {
  const seen: ClientRequest[] = [];
  const remote = createMobileVoiceTranscriptionRemote({
    origin: ORIGIN,
    request: async (input) => {
      seen.push(input);
      return { success: true, data: { text: '整理周报', audio_seconds: 3, settled: true } };
    },
  });
  const result = await remote.transcribe({ requestId: 'req-1', audio: { ...WAV } });
  assert.deepEqual(result, { text: '整理周报', audioSeconds: 3, settled: true });
  assert.equal(seen[0]!.method, 'POST');
  assert.equal(seen[0]!.path, '/api/v1/mobile/voice/transcriptions');
  const body = seen[0]!.body as FormData;
  assert.ok(body instanceof FormData, '上载体是 FormData（经 authorizedTransport 原样透传）');
  assert.equal(body.get('request_id'), 'req-1');
  const part = body.get('audio');
  assert.ok(part instanceof Blob, 'audio 以文件 part 上载（服务端 c.Request.FormFile("audio")，mobile_voice.go:488）');
  assert.equal((part as Blob).type, 'audio/wav');
});

test('an optional voice_session_id rides the form; a blank one stays absent', async () => {
  const seen: ClientRequest[] = [];
  const remote = createMobileVoiceTranscriptionRemote({
    origin: ORIGIN,
    request: async (input) => { seen.push(input); return { success: true, data: { text: 'x', settled: true } }; },
  });
  await remote.transcribe({ requestId: 'req-1', audio: { ...WAV }, voiceSessionId: 'vs_1' });
  assert.equal((seen[0]!.body as FormData).get('voice_session_id'), 'vs_1');
  await remote.transcribe({ requestId: 'req-2', audio: { ...WAV }, voiceSessionId: '   ' });
  assert.equal((seen[1]!.body as FormData).get('voice_session_id'), null, '空白 voice_session_id 不进表单');
});

test('missing request id or a source-less audio is refused before any wire call', async () => {
  let calls = 0;
  const remote = createMobileVoiceTranscriptionRemote({
    origin: ORIGIN,
    request: async () => { calls += 1; return { success: true, data: { text: 'x' } }; },
  });
  await assert.rejects(() => remote.transcribe({ requestId: '  ', audio: { ...WAV } }), /request id/);
  await assert.rejects(
    () => remote.transcribe({ requestId: 'req-1', audio: { mimeType: 'audio/wav' } }),
    /bytes or uri/,
  );
  assert.equal(calls, 0, '校验发生在任何网络之前');
});

test('a 503 maps to VOICE_CHARGING_UNCONFIGURED; other HTTP failures map to VOICE_TRANSCRIBE_FAILED', async () => {
  const withStatus = (status: number) =>
    createMobileVoiceTranscriptionRemote({
      origin: ORIGIN,
      request: async () => { throw Object.assign(new Error(`Request failed with status ${status}`), { status }); },
    });
  await assert.rejects(
    () => withStatus(503).transcribe({ requestId: 'r', audio: { ...WAV } }),
    (error: unknown) => (error as { code?: string }).code === 'VOICE_CHARGING_UNCONFIGURED',
  );
  await assert.rejects(
    () => withStatus(502).transcribe({ requestId: 'r', audio: { ...WAV } }),
    (error: unknown) => (error as { code?: string }).code === 'VOICE_TRANSCRIBE_FAILED',
  );
});

test('malformed envelopes are refused (success must be true with a string data.text)', async () => {
  const remote = createMobileVoiceTranscriptionRemote({
    origin: ORIGIN,
    request: async () => ({ success: false, error: 'transcribe_failed' }),
  });
  await assert.rejects(() => remote.transcribe({ requestId: 'r', audio: { ...WAV } }), /success envelope/);
  const noText = createMobileVoiceTranscriptionRemote({
    origin: ORIGIN,
    request: async () => ({ success: true, data: { audio_seconds: 1 } }),
  });
  await assert.rejects(() => noText.transcribe({ requestId: 'r', audio: { ...WAV } }), /data\.text/);
});

test('the origin must be a validated credential-free HTTPS origin', async () => {
  assert.throws(
    () => createMobileVoiceTranscriptionRemote({ origin: 'http://insecure.example', request: async () => undefined }),
    /origin/i,
  );
});

test('the result surface is exactly the server envelope fields (unknown fields never pass through)', async () => {
  // 服务端成功信封只有 {text, audio_seconds, settled}（mobile_voice.go:601）；最终审查 t56
  // 移除了永不为 true 的前向兼容死字段 replay——即使信封出现未知/遗留字段，客户端结果面
  // 也不得透传（结构可赋值面与 mobile-core DictationTranscriptionResult 保持最小）。
  const remote = createMobileVoiceTranscriptionRemote({
    origin: ORIGIN,
    request: async () => ({ success: true, data: { text: '信封外字段不透传', replay: true } }),
  });
  const result = await remote.transcribe({ requestId: 'req-1', audio: { ...WAV } });
  assert.deepEqual(result, { text: '信封外字段不透传' });
  assert.equal('replay' in result, false, '遗留占位字段 replay 绝不出现在结果面');
});
