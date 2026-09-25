import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createMobileVoiceTranscriptionRemote } from '@weknora/api-client/mobile/voice';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createDictation, createInMemoryCredentialStore, createMobileRuntime, type TaskStartReceipt } from '@weknora/mobile-core';
import { createNativeRequestId } from './adapters/request-id.ts';
import { disallowedDeploymentHost } from './runtime-integration-smoke.ts';
import { createNewTaskController } from './new-task-view.ts';
import { applyConfirmedDictation } from './dictation-view.ts';
import { GOAL_TEXT_MAX_LENGTH } from './screens/NewTaskScreen.tsx';

export type VoiceDictationIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

/** 与 task-start-integration-smoke.ts 相同的 opt-in 语义 + disallowedDeploymentHost 主机防线（自包含，不跨计划 import）。 */
export function voiceDictationIntegrationConfig(env: Record<string, string | undefined>): VoiceDictationIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    return { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' };
  }
  let parsed: URL;
  try { parsed = new URL(deploymentOrigin); } catch {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  const hostRejection = disallowedDeploymentHost(parsed.hostname, 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL');
  if (hostRejection) return { enabled: false, disposition: 'invalid', reason: hostRejection };
  return { enabled: true, deploymentOrigin: parsed.origin, email, password };
}

export interface VoiceDictationIntegrationEvidence {
  deploymentOrigin: string;
  /** AC1：取消路径真实执行（scripted 捕获 granted → recording → cancel → idle）。 */
  recordingCancelled: boolean;
  /** AC1：取消后手写目标文本逐字保留。 */
  cancelKeptGoalText: boolean;
  /** AC2：整个听写流（含取消与确认）期间 Task start 通道零调用。 */
  dictationNeverSubmitted: boolean;
  /** 转写结果如实记录：成功 / 部署未配置语音计价（503）/ 失败。 */
  transcription: 'transcribed' | 'charging-unconfigured' | 'failed';
  confirmedTextLength?: number;
  editedBeforeConfirm?: boolean;
  errorReason?: string;
  timestamp: string;
}

/** 1 秒 8kHz 16bit 单声道静音 WAV（确定性合成，无外部 fixture；真实 multipart 字节上载）。 */
function silentWavBytes(): Uint8Array {
  const sampleRate = 8000;
  const samples = sampleRate; // 1 秒
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

/**
 * 真实端到端（AC3）：生产 JSON transport + Runtime 授权通道 + 具体 Remote Adapter +
 * Dictation 模块 + 真实 New 控制器（提交通道以 spy 断言零调用——AC2 的行为证据）。
 * 捕获是 scripted Adapter（spec：system audio 属 true external，真机验收 #69/#70）。
 * 服务器若未配置 voice admission，如实记录 charging-unconfigured（合法结论，不伪造转写成功）。
 */
export async function runVoiceDictationIntegration(config: Extract<VoiceDictationIntegrationConfig, { enabled: true }>): Promise<VoiceDictationIntegrationEvidence> {
  const evidence: VoiceDictationIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    recordingCancelled: false,
    cancelKeptGoalText: false,
    dictationNeverSubmitted: false,
    transcription: 'failed',
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
  let submissions = 0;
  try {
    const snapshot = await runtime.signIn({
      deployment: { origin: config.deploymentOrigin, label: 'Voice dictation integration' },
      email: config.email,
      password: config.password,
    });
    if (snapshot.surface !== 'authorized') {
      evidence.errorReason = `surface ${snapshot.surface}`;
      return evidence;
    }
    const controller = createNewTaskController({
      office: {
        start: async (): Promise<TaskStartReceipt> => {
          submissions += 1;
          throw new Error('dictation must never submit a task');
        },
        reconcilePending: async () => [],
      },
      agents: async () => [],
      newRequestId: createNativeRequestId(),
    });
    await controller.whenInitialized();
    const typedGoal = '手写目标：周报';
    controller.update({ text: typedGoal });

    const dictation = createDictation({
      capture: {
        start: async () => 'recording',
        stop: async () => ({ bytes: silentWavBytes(), mimeType: 'audio/wav', fileName: 'dictation.wav' }),
        cancel: async () => undefined,
      },
      transcribe: createMobileVoiceTranscriptionRemote({
        origin: config.deploymentOrigin,
        request: (input) => runtime.authorizedRequest(input),
      }),
      newRequestId: createNativeRequestId(),
    });

    // AC1 live：取消录音不取消 Task（草稿原样、零转写派发、零提交）
    await dictation.begin();
    if (dictation.state().phase === 'recording') {
      await dictation.cancel();
      evidence.recordingCancelled = dictation.state().phase === 'idle';
      evidence.cancelKeptGoalText = controller.state().draft.text === typedGoal;
    }

    // 转写 live：真实 multipart POST /api/v1/mobile/voice/transcriptions
    await dictation.begin();
    await dictation.finish();
    const settled = dictation.state();
    if (settled.phase === 'review' && typeof settled.transcript === 'string') {
      const edited = `${settled.transcript.trim()}（已校对）`;
      dictation.editTranscript(edited);
      const confirmed = dictation.confirmTranscript();
      if (confirmed !== undefined) {
        evidence.transcription = 'transcribed';
        evidence.editedBeforeConfirm = confirmed === edited;
        controller.update({ text: applyConfirmedDictation(controller.state().draft.text, confirmed, GOAL_TEXT_MAX_LENGTH) });
        evidence.cancelKeptGoalText = evidence.cancelKeptGoalText && controller.state().draft.text.startsWith(typedGoal);
        evidence.confirmedTextLength = controller.state().draft.text.length;
      }
    } else if (settled.failureCode === 'VOICE_CHARGING_UNCONFIGURED') {
      evidence.transcription = 'charging-unconfigured';
    } else {
      evidence.transcription = 'failed';
      evidence.errorReason = `${settled.failure ?? 'unknown'}${settled.failureCode === undefined ? '' : ` (${settled.failureCode})`}`;
    }
    evidence.dictationNeverSubmitted = submissions === 0;
    return evidence;
  } catch (error) {
    evidence.transcription = 'failed';
    evidence.errorReason = error instanceof Error ? error.message : String(error); // 失败仍产出证据（不含凭据）
    // dictationNeverSubmitted 保持初始 false：听写流未走完时不得以「提交计数为 0」伪造 AC2 证据。
    return evidence;
  } finally {
    runtime.dispose(); // 释放 lease/凭据通道（先例：task-start-integration-smoke.ts:125）
  }
}

/** Emits only the redacted evidence contract, including failed live outcomes. */
export function emitVoiceDictationIntegrationEvidence(evidence: VoiceDictationIntegrationEvidence, emit: (record: string) => void): void {
  emit(JSON.stringify(evidence));
}
