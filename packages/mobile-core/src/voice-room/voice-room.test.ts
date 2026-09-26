import test from 'node:test';
import assert from 'node:assert/strict';
import { createVoiceRoom, VoiceRoomError, VOICE_ROOM_MAX_AUDIO_BYTES, type VoiceHandle, type VoiceRoomPorts } from './voice-room.ts';
import { createScriptedVoiceSession, type ScriptedVoiceSession } from './in-memory-voice-session.ts';
import { createScenarioDictationTranscriber, createScriptedDictationCapture, type ScenarioDictationTranscriber, type ScriptedDictationCapture } from '../voice/in-memory-dictation.ts';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import type { VoiceAudioDisposition } from './voice-room.ts';

const tick = async (): Promise<void> => { await new Promise<void>((resolve) => setImmediate(resolve)); };

function mintLease(): { lease: ScopeLease; revoke: () => void } {
  const internal = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.com', userId: 'u1', tenantId: 't1' });
  return { lease: internal.asScopeLease(), revoke: () => internal.revoke() };
}

interface RoomHarness {
  session: ScriptedVoiceSession;
  capture: ScriptedDictationCapture;
  transcriber: ScenarioDictationTranscriber;
  discarded: Array<{ turnId: string; reason: string }>;
  ports: VoiceRoomPorts;
  revoke: () => void;
}

function harness(options: { start?: 'recording' | 'denied' | Error } = {}): RoomHarness {
  const session = createScriptedVoiceSession();
  const capture = createScriptedDictationCapture({
    start: options.start ?? 'recording',
    stop: { bytes: new Uint8Array([1, 2, 3]), mimeType: 'audio/wav', fileName: 'turn.wav' },
  });
  const transcriber = createScenarioDictationTranscriber();
  const discarded: Array<{ turnId: string; reason: string }> = [];
  const disposition: VoiceAudioDisposition = { onDiscarded: (turnId, reason) => { discarded.push({ turnId, reason }); } };
  const { lease, revoke } = mintLease();
  const ports: VoiceRoomPorts = {
    session,
    capture,
    transcribe: transcriber,
    newRequestId: () => `req-${transcriber.requests.length + 1}`,
    newSessionId: () => `room-${session.opens.length + 1}`,
    lease: () => lease,
    audioDisposition: disposition,
  };
  return { session, capture, transcriber, discarded, ports, revoke };
}

/** 标准一轮：begin → end → 应答转写 → 到 review。返回轮次 id。 */
async function turnToReview(h: RoomHarness, handle: VoiceHandle, text: string, requestIndex = 0): Promise<string> {
  await handle.beginTurn();
  assert.equal(handle.state().phase, 'listening');
  const turning = handle.endTurn();
  await tick(); await tick();
  assert.equal(handle.state().phase, 'transcribing');
  h.transcriber.resolve(requestIndex, { text, audioSeconds: 2 });
  await turning;
  assert.equal(handle.state().phase, 'ready');
  return handle.state().pendingTurnId!;
}

test('AC gates — permission denial: a denied microphone surfaces a notice, dispatches nothing, and opens no session', async () => {
  const h = harness({ start: 'denied' });
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1', runId: 'run-1' });
  await handle.beginTurn();
  assert.equal(handle.state().phase, 'idle');
  assert.equal(handle.state().notice?.reason, 'permission-denied');
  assert.deepEqual(h.capture.calls, ['start'], '拒权后零 stop/零 cancel——没有任何捕获工作');
  assert.equal(h.transcriber.requests.length, 0, '拒权绝不派发转写');
  assert.equal(h.session.opens.length, 0, '拒权轮不开启会话（先权限后会话，拒权不留下已计费空会话）');
  assert.equal(h.discarded.length, 0, '没有收留过音频就没有处置回调');
});

test('AC1 — disconnect has a clear end: a failed turn transcription ends the session, discards audio, and shows interrupted', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1', runId: 'run-1' });
  await handle.beginTurn();
  const turning = handle.endTurn();
  await tick(); await tick();
  h.transcriber.resolve(0, new Error('network dropped'));
  await turning;
  assert.equal(handle.state().phase, 'interrupted', '断线以 interrupted 如实呈现');
  assert.equal(handle.state().notice?.reason, 'transcription-failed');
  assert.equal(h.session.ends.length, 1, '断线明确结束：服务端 stop/settle 恰好一次');
  assert.deepEqual(h.discarded, [{ turnId: 'req-1', reason: 'failed' }], '原始音频恰好处置一次');
  assert.equal(handle.state().turns.length, 0, '失败轮次不进入轮次日志');
});

test('AC1 — resume opens a NEW session with a new product session id and keeps the confirmed-turn log', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1', runId: 'run-1' });
  const first = await turnToReview(h, handle, '第一条指令');
  handle.confirmTranscript(first);
  // 断线：新一轮转写失败（beginTurn 复用仍在窗内的会话）
  await handle.beginTurn();
  const turning = handle.endTurn();
  await tick(); await tick();
  h.transcriber.resolve(1, new Error('drop'));
  await turning;
  assert.equal(handle.state().phase, 'interrupted');
  // 恢复
  await handle.resume();
  assert.equal(handle.state().phase, 'idle');
  assert.equal(h.session.opens.length, 2, '恢复 = 重新授权');
  assert.notEqual(h.session.opens[1]!.productSessionId, h.session.opens[0]!.productSessionId, '恢复绝不复用旧 product session id');
  assert.deepEqual(handle.state().turns.map((turn) => turn.state), ['confirmed'], '已确认文字（语音交互记录）跨断线保留');
  // 恢复后的会话可用（新轮派发到新会话）
  const second = await turnToReview(h, handle, '恢复后的第二条', 2);
  assert.equal(handle.state().pendingTurnId, second);
  assert.equal(h.transcriber.requests.length, 3, '失败轮不重派：恰好 1 成功 + 1 失败 + 1 恢复轮');
});

test('editable transcription: the review transcript can be edited and confirm returns the edited text', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1' });
  const id = await turnToReview(h, handle, '原始转写');
  handle.editTranscript(id, '原始转写（已校对）');
  const intent = handle.confirmTranscript(id);
  assert.deepEqual(intent, { kind: 'steer', text: '原始转写（已校对）' });
  assert.equal(handle.state().phase, 'idle');
  assert.equal(handle.state().turns[0]!.state, 'confirmed');
  assert.ok(handle.state().turns[0]!.confirmedAt !== undefined);
  assert.equal(h.transcriber.requests.length, 1, '确认只消费既有转写，绝不再次派发');
});

test('confirmation into Task: the module itself never submits anything — confirm only returns the intent', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1', runId: 'run-1' });
  const id = await turnToReview(h, handle, '写入任务');
  const before = JSON.stringify({ opens: h.session.opens.length, ends: h.session.ends.length, requests: h.transcriber.requests.length });
  const intent = handle.confirmTranscript(id);
  assert.notEqual(intent, undefined);
  assert.equal(intent!.kind, 'steer', 'AC2：确认只产生 steer 意图');
  const after = JSON.stringify({ opens: h.session.opens.length, ends: h.session.ends.length, requests: h.transcriber.requests.length });
  assert.equal(after, before, '确认动作零网络（提交由宿主经 TaskHandle.act 承担）');
});

test('AC2 — no decision-capable method on the handle, and empty text confirms nothing', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1' });
  const decisionShaped = Object.keys(handle).filter((key) => /decide|approv|interaction|command/i.test(key));
  assert.deepEqual(decisionShaped, [], '语音句柄结构性不携带任何决定/审批方法');
  const id = await turnToReview(h, handle, '有内容的一轮');
  handle.editTranscript(id, '   ');
  assert.equal(handle.confirmTranscript(id), undefined, '空白文本不产生指令');
  assert.equal(handle.state().phase, 'ready', '空白确认不消费轮次（宿主可编辑或放弃）');
});

test('AC1 — raw audio is discarded exactly once per turn on every terminal path and never appears in state', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1' });
  // 成功轮
  const id = await turnToReview(h, handle, '一轮');
  assert.deepEqual(h.discarded, [{ turnId: 'req-1', reason: 'transcribed' }]);
  handle.discardTurn(id);
  assert.equal(JSON.stringify(handle.state()).includes('bytes'), false, '状态序列化不携带音频字节');
  assert.equal(JSON.stringify(handle.state()).includes('audio:'), false, '状态序列化不携带音频字段');
  // 拒权轮（独立构造：拒权不收留音频 → 零处置回调）
  const denied = harness({ start: 'denied' });
  const deniedHandle = createVoiceRoom(denied.ports).join({ taskId: 'task-1' });
  await deniedHandle.beginTurn();
  assert.equal(denied.discarded.length, 0, '拒权轮没有收留过音频就没有处置回调');
});

test('AC1 — oversized audio is refused before the transcription window with zero dispatch', async () => {
  const bigCapture = createScriptedDictationCapture({ stop: { bytes: new Uint8Array(VOICE_ROOM_MAX_AUDIO_BYTES + 1), mimeType: 'audio/wav' } });
  const transcriber = createScenarioDictationTranscriber();
  const discarded: Array<{ turnId: string; reason: string }> = [];
  const { lease } = mintLease();
  const ports: VoiceRoomPorts = { session: createScriptedVoiceSession(), capture: bigCapture, transcribe: transcriber, newRequestId: () => 'req-big', newSessionId: () => 'room-big', lease: () => lease, audioDisposition: { onDiscarded: (turnId, reason) => { discarded.push({ turnId, reason }); } } };
  const handle = createVoiceRoom(ports).join({ taskId: 'task-1' });
  await handle.beginTurn();
  await handle.endTurn();
  assert.equal(handle.state().phase, 'idle');
  assert.equal(handle.state().notice?.reason, 'audio-too-large');
  assert.equal(transcriber.requests.length, 0, '超限音频不派发转写');
  assert.equal(discarded.length, 0, '音频从未被模块收留，无处置回调');
});

test('leave during transcribing ends the room clearly, discards the in-flight turn, and later results change nothing', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1', runId: 'run-1' });
  await handle.beginTurn();
  const turning = handle.endTurn();
  await tick(); await tick();
  assert.equal(handle.state().phase, 'transcribing');
  await handle.leave();
  assert.equal(handle.state().phase, 'ended');
  assert.deepEqual(h.discarded, [{ turnId: 'req-1', reason: 'session-ended' }]);
  h.transcriber.resolve(0, { text: '迟到的转写' });
  await turning;
  assert.equal(handle.state().phase, 'ended', '迟到结果不改变终态');
  assert.equal(handle.state().turns.length, 0, '迟到转写绝不进入轮次日志');
  assert.equal(h.session.ends.length, 1, 'leave 明确结束恰好一次');
  assert.equal(handle.state().lastLeave?.settled, true, '结算事实如实呈现');
});

test('leave is idempotent on the module side', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1' });
  await handle.beginTurn();
  await handle.leave();
  await handle.leave();
  assert.equal(h.session.ends.length, 1, '模块侧 leave 幂等');
  // 服务端幂等由 Task 5 的 Go 测试以真实 handler 证明（replay:true）
});

test('scope revocation collapses the room at the next boundary without further session work and drops late results', async () => {
  const h = harness();
  const handle = createVoiceRoom(h.ports).join({ taskId: 'task-1', runId: 'run-1' });
  await handle.beginTurn();
  const turning = handle.endTurn();
  await tick(); await tick();
  h.revoke();
  h.transcriber.resolve(0, { text: 'scope 已撤销才回来的转写' });
  await turning;
  assert.equal(handle.state().phase, 'ended', 'scope 撤销收敛为 ended');
  assert.equal(handle.state().notice?.reason, 'scope-revoked');
  assert.equal(handle.state().turns.length, 0);
  assert.equal(h.session.ends.length, 0, 'scope 撤销后不再发起任何服务端调用（fail closed）');
  assert.equal(h.session.opens.length, 1, '撤销后零新会话');
});

test('join fails closed: empty taskId is invalid input and a revoked lease is a scope change', () => {
  const h = harness();
  assert.throws(() => createVoiceRoom(h.ports).join({ taskId: '  ' }), (error: unknown) => error instanceof VoiceRoomError && error.code === 'VOICE_ROOM_INVALID_INPUT');
  h.revoke();
  assert.throws(() => createVoiceRoom(h.ports).join({ taskId: 'task-1' }), (error: unknown) => error instanceof VoiceRoomError && error.code === 'VOICE_ROOM_SCOPE_CHANGED');
});
