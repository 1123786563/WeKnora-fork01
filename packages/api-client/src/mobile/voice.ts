import type { ClientRequest } from '../client.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface MobileVoiceRemoteOptions {
  /** 部署 Origin。构造即强校验（共享 requireDeploymentOrigin，B3-F11 收敛）：绝对 HTTPS、含 host、无 userinfo、无 path/query/fragment。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
  request: Request;
}

/** 与 mobile-core DictationAudio 结构逐字一致（结构可赋值由 apps/mobile 的类型证明测试落实）。 */
export interface MobileVoiceAudio {
  uri?: string;
  bytes?: Uint8Array;
  mimeType: string;
  fileName?: string;
}

export interface MobileVoiceTranscriptionInput {
  requestId: string;
  audio: MobileVoiceAudio;
  /** 可选：把本次转写用量记到自己的语音 session（服务端 mobile_voice.go:503-520）；听写不用。 */
  voiceSessionId?: string;
}

/**
 * 结果面与服务端成功信封逐字对齐：{text, audio_seconds, settled}
 * （internal/handler/mobile_voice.go:601）。不留前向兼容占位字段——
 * 未知信封字段一律不透传（最终审查 t56：移除永不为 true 的死字段 replay）。
 */
export interface MobileVoiceTranscriptionResult {
  text: string;
  audioSeconds?: number;
  settled?: boolean;
}

export interface MobileVoiceTranscriptionRemote {
  /** POST /api/v1/mobile/voice/transcriptions（internal/handler/mobile_voice.go:445，multipart：request_id / voice_session_id? / 文件 part `audio`）。 */
  transcribe(input: MobileVoiceTranscriptionInput): Promise<MobileVoiceTranscriptionResult>;
}

const VOICE_TRANSCRIBE_PATH = '/api/v1/mobile/voice/transcriptions';

function requireRequestId(requestId: string): string {
  const trimmed = typeof requestId === 'string' ? requestId.trim() : '';
  if (trimmed === '') throw new Error('voice transcription request id is required');
  return trimmed;
}

export function createMobileVoiceTranscriptionRemote(options: MobileVoiceRemoteOptions): MobileVoiceTranscriptionRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  return {
    async transcribe(input: MobileVoiceTranscriptionInput): Promise<MobileVoiceTranscriptionResult> {
      const requestId = requireRequestId(input.requestId);
      const audio = input.audio;
      if (typeof audio !== 'object' || audio === null) throw new Error('voice transcription audio is required');
      if (audio.uri === undefined && audio.bytes === undefined) {
        throw new Error('voice transcription audio requires bytes or uri');
      }
      if (typeof FormData === 'undefined') throw new Error('voice transcription requires multipart form data support');
      const fileName = audio.fileName ?? (audio.uri !== undefined ? 'dictation.m4a' : 'dictation.audio');
      const form = new FormData();
      form.append('request_id', requestId);
      if (input.voiceSessionId !== undefined && input.voiceSessionId.trim() !== '') {
        form.append('voice_session_id', input.voiceSessionId);
      }
      if (audio.uri !== undefined) {
        // RN FormData 原生识别 {uri,name,type}（Expo 文件上传形态；Node 环境不会进入该分支）。
        form.append('audio', { uri: audio.uri, name: fileName, type: audio.mimeType } as unknown as Blob);
      } else {
        form.append('audio', new Blob([new Uint8Array(audio.bytes!)], { type: audio.mimeType }), fileName);
      }
      let envelope: unknown;
      try {
        envelope = await request({ method: 'POST', path: VOICE_TRANSCRIBE_PATH, body: form });
      } catch (error) {
        // wire→语义翻译：服务端错误体 {"success":false,"error":"..."} 的 error 串只进 ApiError.message
        // （errors.ts:51-60），code 为 HTTP_<status>；503 = 未配置语音计价（voice_charging_unconfigured）。
        const status = error instanceof Error && 'status' in error ? Number((error as { status?: unknown }).status) : NaN;
        if (status === 503) {
          throw Object.assign(new Error('voice charging is not configured on this deployment'), { code: 'VOICE_CHARGING_UNCONFIGURED' as const });
        }
        if (!Number.isNaN(status)) {
          throw Object.assign(new Error(`voice transcription failed (HTTP ${status})`), { code: 'VOICE_TRANSCRIBE_FAILED' as const });
        }
        throw error;
      }
      if (typeof envelope !== 'object' || envelope === null || Array.isArray(envelope)) {
        throw new Error('voice transcription response must be a success envelope');
      }
      const record = envelope as { success?: unknown; data?: unknown };
      if (record.success !== true || typeof record.data !== 'object' || record.data === null) {
        throw new Error('voice transcription response must be a success envelope: success must be true with data');
      }
      const data = record.data as { text?: unknown; audio_seconds?: unknown; settled?: unknown };
      if (typeof data.text !== 'string') throw new Error('voice transcription data.text must be a string');
      return {
        text: data.text,
        ...(typeof data.audio_seconds === 'number' ? { audioSeconds: data.audio_seconds } : {}),
        ...(data.settled === true ? { settled: true } : {}),
      };
    },
  };
}
