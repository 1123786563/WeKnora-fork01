import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileVoiceSessionRemote } from '@weknora/api-client/mobile/voice-sessions';
import { createMobileVoiceTranscriptionRemote } from '@weknora/api-client/mobile/voice';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import {
  createInMemoryCredentialStore, createMobileRuntime, createTaskOffice, createVoiceRoom,
  type TaskOffice, type VoiceAudioDisposition, type VoiceHandle,
} from '@weknora/mobile-core';
import { createNativeRequestId } from './adapters/request-id.ts';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';
import { createTaskDetailController } from './task-detail-view.ts';

export type VoiceRoomIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

/** 与 task-start/task-intervention 冒烟相同的 opt-in 语义 + disallowedDeploymentHost 主机防线（自包含）。 */
export function voiceRoomIntegrationConfig(env: Record<string, string | undefined>): VoiceRoomIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    return { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' };
  }
  let parsed: URL;
  try {
    parsed = new URL(deploymentOrigin);
  } catch {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

export interface VoiceRoomIntegrationEvidence {
  deploymentOrigin: string;
  sessionOpened: boolean;
  /** 真实 Task/Run 绑定（start bound 才 true——「绑定 Task 的 Voice Room」的 live 证据）。 */
  taskBound: boolean;
  /** 轮次转写结果：成功 / 部署未配置语音计价（503，诚实结论）/ 失败。 */
  turn: 'transcribed' | 'charging-unconfigured' | 'failed';
  /** 确认文字经 act(steer) 的真实回执（无活动 Run 或无转写时 skipped，不伪造）。 */
  confirmSteer: 'accepted' | 'conflict' | 'unknown' | 'skipped-no-transcript' | 'failed';
  /** AC1：leave 的服务端结束事实（分支未行使该动作时如实标 not-exercised，不冒充真实回执）。 */
  disconnect: 'ended-settled' | 'ended-unsettled' | 'not-exercised' | 'failed';
  /** AC1：恢复以新会话开启（分支未行使该动作时如实标 not-exercised）。 */
  resume: 'new-session' | 'not-exercised' | 'failed';
  /** AC1：原始音频处置回调真实发生（scripted 捕获 + disposition 观察）。 */
  rawAudioDiscarded: boolean;
  /** AC2：语音通道零决定能力（确认只产 steer + 句柄无决定方法的结构断言）。 */
  voiceNeverDecided: boolean;
  errorReason?: string;
  timestamp: string;
}

/** 1 秒 8kHz 16bit 单声道静音 WAV（确定性合成；真实 multipart 字节上载，与 #56 同源手法）。 */
function silentWavBytes(): Uint8Array {
  const sampleRate = 8000;
  const samples = sampleRate;
  const buffer = new ArrayBuffer(44 + samples * 2);
  const view = new DataView(buffer);
  const ascii = (offset: number, text: string): void => {
    for (let i = 0; i < text.length; i += 1) view.setUint8(offset + i, text.charCodeAt(i));
  };
  ascii(0, 'RIFF'); view.setUint32(4, 36 + samples * 2, true); ascii(8, 'WAVE');
  ascii(12, 'fmt '); view.setUint32(16, 16, true); view.setUint16(20, 1, true); view.setUint16(22, 1, true);
  view.setUint32(24, sampleRate, true); view.setUint32(28, sampleRate * 2, true);
  view.setUint16(32, 2, true); view.setUint16(34, 16, true);
  ascii(36, 'data'); view.setUint32(40, samples * 2, true);
  return new Uint8Array(buffer);
}

const decisionShapedKeys = (handle: VoiceHandle): string[] =>
  Object.keys(handle).filter((key) => /decide|approv|interaction|command/i.test(key));

/**
 * charging-unconfigured（诚实 503）分支的证据落点：此路径从未执行 leave()/resume()——
 * 无会话可结（leave 无从结算）、resume 必再次 503——AC1 两字段以 not-exercised 如实标注，
 * 不冒充真实回执（字段契约必须与真实动作对齐）。
 */
export function markChargingUnconfigured(evidence: VoiceRoomIntegrationEvidence, handle: VoiceHandle): VoiceRoomIntegrationEvidence {
  evidence.turn = 'charging-unconfigured';
  evidence.confirmSteer = 'skipped-no-transcript';
  evidence.disconnect = 'not-exercised';
  evidence.resume = 'not-exercised';
  evidence.voiceNeverDecided = decisionShapedKeys(handle).length === 0;
  evidence.errorReason = 'deployment has no voice pricing configured (honest 503); AC1 leave/resume not exercised';
  return evidence;
}

/**
 * 真实端到端（AC3 live）：生产 JSON transport + Runtime 授权通道 + 具体 Remote Adapter +
 * Voice Room 模块 + 真实 Task Office。真实 start 一个 Task（WEKNORA_MOBILE_TEST_START_TASK=1
 * 门控；未门控时以探针 taskId 验证无 Run 的会话/转写/结束/恢复路径），scripted 捕获 + 真实
 * 转写，确认文字经真实 act(steer)。每一步如实记录，失败落 errorReason，绝不伪造通过。
 * 诚实性要点：(1) confirmTranscript 只能消费一次——先取意图再断言再提交；(2) 未配置语音
 * 计价的部署以一次裸 open 的错误码如实区分（503 → charging-unconfigured），不伪造转写成功；
 * (3) charging-unconfigured 分支的 AC1 字段如实标 not-exercised（见 markChargingUnconfigured）。
 */
export async function runVoiceRoomIntegration(config: Extract<VoiceRoomIntegrationConfig, { enabled: true }>): Promise<VoiceRoomIntegrationEvidence> {
  const evidence: VoiceRoomIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    sessionOpened: false,
    taskBound: false,
    turn: 'failed',
    confirmSteer: 'failed',
    disconnect: 'failed',
    resume: 'failed',
    rawAudioDiscarded: false,
    voiceNeverDecided: false,
    timestamp: new Date().toISOString(),
  };
  const fetcher: FetchLike = (input, init) => fetch(input, init as RequestInit);
  const runtime = createMobileRuntime({
    credentialStore: createInMemoryCredentialStore(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
    },
  });
  try {
    const snapshot = await runtime.signIn({
      deployment: { origin: config.deploymentOrigin, label: 'Voice room integration' },
      email: config.email,
      password: config.password,
    });
    if (snapshot.surface !== 'authorized') {
      evidence.errorReason = `surface ${snapshot.surface}`;
      return evidence;
    }
    // 真实 Task 绑定：start 一个 Task 并反查 taskId（与 task-intervention 冒烟同序）。
    let taskId = 'voice-room-integration-probe';
    let runId: string | undefined;
    let office: TaskOffice | undefined;
    if (process.env.WEKNORA_MOBILE_TEST_START_TASK === '1') {
      const agentsEnvelope = (await runtime.authorizedRequest({ method: 'GET', path: '/api/v1/agents' })) as { success?: boolean; data?: Array<{ id?: unknown }> };
      const agentId = typeof agentsEnvelope?.data?.[0]?.id === 'string' ? agentsEnvelope.data[0].id : undefined;
      if (agentId === undefined) {
        evidence.errorReason = 'no agent available on the deployment';
        return evidence;
      }
      const remote = createTaskOfficeRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input), stream: (input, onChunk) => runtime.authorizedEventStream(input, onChunk) });
      office = createTaskOffice({ backend: remote, detail: remote, commands: remote, lease: () => runtime.scopeLease(), newRequestId: createNativeRequestId() });
      const requestId = createNativeRequestId()();
      const receipt = await office.start({ text: `T27 语音房集成验证：${new Date().toISOString()}`, agentId, budgetUpper: 10 }, { requestId });
      if (receipt.phase === 'bound' && receipt.runId !== undefined) {
        const page = await office.tasks({});
        const card = page.items.find((item) => item.runId === receipt.runId);
        if (card !== undefined) {
          taskId = card.taskId;
          runId = receipt.runId;
          evidence.taskBound = true;
        }
      }
    }

    const sessionRemote = createMobileVoiceSessionRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) });
    const discarded: string[] = [];
    const disposition: VoiceAudioDisposition = { onDiscarded: (turnId) => { discarded.push(turnId); } };
    const room = createVoiceRoom({
      session: {
        open: async (input) => {
          const grant = await sessionRemote.open(input);
          return { id: grant.id, expiresAt: grant.expiresAt, maxSeconds: grant.maxSeconds };
        },
        end: (sessionId) => sessionRemote.end(sessionId),
      },
      transcribe: createMobileVoiceTranscriptionRemote({ origin: config.deploymentOrigin, request: (input) => runtime.authorizedRequest(input) }),
      capture: {
        start: async () => 'recording',
        stop: async () => ({ bytes: silentWavBytes(), mimeType: 'audio/wav', fileName: 'room-turn.wav' }),
        cancel: async () => undefined,
      },
      newRequestId: createNativeRequestId(),
      newSessionId: createNativeRequestId(),
      lease: () => runtime.scopeLease(),
      audioDisposition: disposition,
    });
    const handle: VoiceHandle = room.join({ taskId, ...(runId === undefined ? {} : { runId }) });

    // 一轮：真实授权 + 真实 multipart 转写
    await handle.beginTurn();
    await handle.endTurn();
    if (handle.state().phase === 'ready') {
      evidence.turn = 'transcribed';
      evidence.sessionOpened = true;
      evidence.rawAudioDiscarded = discarded.length === 1;
      // confirmTranscript 只能消费一次：先取意图，再做 AC2 结构断言，最后经真实 act 提交
      const intent = handle.confirmTranscript(handle.state().pendingTurnId!);
      evidence.voiceNeverDecided = decisionShapedKeys(handle).length === 0 && intent?.kind === 'steer';
      if (intent !== undefined && office !== undefined && runId !== undefined) {
        const detailHandle = office.open({ taskId, runId });
        const detailController = createTaskDetailController(detailHandle);
        try {
          await detailController.whenSettled();
          const receipt = await detailController.act(intent);
          evidence.confirmSteer = receipt.outcome === 'accepted' ? 'accepted' : receipt.outcome === 'conflict' ? 'conflict' : receipt.outcome === 'unknown' ? 'unknown' : 'failed';
        } finally {
          detailController.dispose();
        }
      } else {
        evidence.confirmSteer = 'skipped-no-transcript';
      }
    } else if (handle.state().notice?.reason === 'session-open-failed' || handle.state().notice?.reason === 'transcription-failed') {
      // open/转写失败：以一次裸 open 区分「计价未配置」（503 诚实结论）与其它失败
      try {
        await sessionRemote.open({ productSessionId: `probe-${createNativeRequestId()()}` });
        evidence.errorReason = 'room saw a failure but a bare session open succeeded';
        return evidence;
      } catch (error) {
        const code = (error as { code?: string }).code;
        if (code === 'VOICE_CHARGING_UNCONFIGURED') {
          return markChargingUnconfigured(evidence, handle);
        }
        evidence.errorReason = `voice session failed (${code ?? 'unknown'})`;
        return evidence;
      }
    } else {
      evidence.errorReason = `turn phase ${handle.state().phase}, notice ${handle.state().notice?.reason ?? 'none'}`;
      return evidence;
    }

    // AC1：明确结束（真实 stop/settle）
    await handle.leave();
    const leaveState = handle.state();
    evidence.disconnect = leaveState.lastLeave?.settled === true ? 'ended-settled' : leaveState.lastLeave !== undefined ? 'ended-unsettled' : 'failed';
    // AC1：恢复以新会话开启
    await handle.resume();
    evidence.resume = handle.state().sessionId !== undefined && handle.state().phase === 'idle' ? 'new-session' : 'failed';
    await handle.leave();
    return evidence;
  } catch (error) {
    evidence.errorReason = error instanceof Error ? error.message : String(error);
    return evidence;
  } finally {
    runtime.dispose();
  }
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitVoiceRoomIntegrationEvidence(evidence: VoiceRoomIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
