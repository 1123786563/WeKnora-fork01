import type { ClientRequest } from '../client.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface MobileVoiceSessionRemoteOptions {
  /** 部署 Origin。构造即强校验（共享 requireDeploymentOrigin）：绝对 HTTPS、含 host、无 userinfo、无 path/query/fragment。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
  request: Request;
}

/** 与服务端创建信封逐字对齐（internal/handler/mobile_voice.go:309-315）。
 * `token` 是短期媒体 grant 的明文，只在首次签发响应出现（幂等重放响应没有该字段）；
 * 组合根把它剥离后才交给 mobile-core VoiceSessionPort——media 凭据绝不进入模块状态。 */
export interface MobileVoiceSessionGrant {
  id: string;
  token?: string;
  expiresAt: string;
  maxSeconds: number;
  priceVersion?: string;
}

/** 服务端停止/结算回执（mobile_voice.go:374-419 的三个分支：closed 重放 / unknown 待对账 / 正常结算）。 */
export interface MobileVoiceSessionEnd {
  id: string;
  state: string;
  settled: boolean;
  replay?: boolean;
  reason?: string;
}

export interface MobileVoiceSessionRemote {
  /** POST /api/v1/mobile/voice/sessions。 */
  open(input: { productSessionId: string; runId?: string; maxSeconds?: number }): Promise<MobileVoiceSessionGrant>;
  /** DELETE /api/v1/mobile/voice/sessions/:id（幂等 stop/settle）。 */
  end(sessionId: string): Promise<MobileVoiceSessionEnd>;
}

const SESSIONS_PATH = '/api/v1/mobile/voice/sessions';

type VoiceSessionCode =
  | 'VOICE_SESSION_INVALID' | 'VOICE_BUDGET_DENIED' | 'VOICE_SESSION_CONFLICT' | 'VOICE_PROVIDER_UNAVAILABLE'
  | 'VOICE_OPEN_UNKNOWN' | 'VOICE_CHARGING_UNCONFIGURED' | 'VOICE_SESSION_NOT_FOUND' | 'VOICE_SESSION_END_FAILED';

function coded(code: VoiceSessionCode, message: string): Error {
  return Object.assign(new Error(message), { code });
}

/** ApiError 的 code 是 HTTP_<status>；服务端错误串（{"success":false,"error":"..."}）只进 message
 * （errors.ts:51-62）——语义翻译在此按 status + message 完成。 */
function statusOf(error: unknown): number {
  return error instanceof Error && 'status' in error ? Number((error as { status?: unknown }).status) : NaN;
}

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function translateOpenError(error: unknown): Error {
  const status = statusOf(error);
  const text = messageOf(error);
  switch (status) {
    case 400: return coded('VOICE_SESSION_INVALID', `voice session rejected: ${text}`);
    case 402: return coded('VOICE_BUDGET_DENIED', `voice budget denied: ${text}`);
    case 409: return coded('VOICE_SESSION_CONFLICT', `voice session conflict: ${text}`);
    case 502: return coded('VOICE_PROVIDER_UNAVAILABLE', `voice provider unavailable: ${text}`);
    case 503: return text.includes('voice_open_unknown')
      ? coded('VOICE_OPEN_UNKNOWN', `voice open outcome unknown: ${text}`)
      : coded('VOICE_CHARGING_UNCONFIGURED', `voice charging unconfigured: ${text}`);
    default:
      if (Number.isNaN(status)) return error instanceof Error ? error : new Error(text);
      return coded('VOICE_SESSION_INVALID', `voice session open failed (HTTP ${status}): ${text}`);
  }
}

function translateEndError(error: unknown): Error {
  const status = statusOf(error);
  if (status === 404) return coded('VOICE_SESSION_NOT_FOUND', `voice session not found: ${messageOf(error)}`);
  if (!Number.isNaN(status)) return coded('VOICE_SESSION_END_FAILED', `voice session end failed (HTTP ${status}): ${messageOf(error)}`);
  return error instanceof Error ? error : new Error(messageOf(error));
}

function requireEnvelope(value: unknown): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error('voice session response must be a success envelope');
  }
  const record = value as { success?: unknown; data?: unknown };
  if (record.success !== true || typeof record.data !== 'object' || record.data === null) {
    throw new Error('voice session response must be a success envelope: success must be true with data');
  }
  return record.data as Record<string, unknown>;
}

export function createMobileVoiceSessionRemote(options: MobileVoiceSessionRemoteOptions): MobileVoiceSessionRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  return {
    async open(input) {
      const productSessionId = typeof input.productSessionId === 'string' ? input.productSessionId.trim() : '';
      if (productSessionId === '') throw new Error('voice session product id is required');
      if (input.maxSeconds !== undefined && (!Number.isInteger(input.maxSeconds) || input.maxSeconds <= 0 || input.maxSeconds > 600)) {
        throw new Error('voice session maxSeconds must be an integer in (0, 600]');
      }
      let envelope: unknown;
      try {
        envelope = await request({
          method: 'POST',
          path: SESSIONS_PATH,
          body: {
            session_id: productSessionId,
            ...(input.runId === undefined || input.runId.trim() === '' ? {} : { run_id: input.runId.trim() }),
            ...(input.maxSeconds === undefined ? {} : { max_seconds: input.maxSeconds }),
          },
        });
      } catch (error) {
        throw translateOpenError(error);
      }
      const data = requireEnvelope(envelope);
      if (typeof data.id !== 'string' || data.id === '') throw new Error('voice session data.id must be a non-empty string');
      if (typeof data.expires_at !== 'string' || data.expires_at === '') throw new Error('voice session data.expires_at must be a string');
      if (typeof data.max_seconds !== 'number') throw new Error('voice session data.max_seconds must be a number');
      return {
        id: data.id,
        ...(typeof data.token === 'string' && data.token !== '' ? { token: data.token } : {}),
        expiresAt: data.expires_at,
        maxSeconds: data.max_seconds,
        ...(typeof data.price_version === 'string' && data.price_version !== '' ? { priceVersion: data.price_version } : {}),
      };
    },
    async end(sessionId) {
      const id = typeof sessionId === 'string' ? sessionId.trim() : '';
      if (id === '') throw new Error('voice session id is required');
      let envelope: unknown;
      try {
        envelope = await request({ method: 'DELETE', path: `${SESSIONS_PATH}/${encodeURIComponent(id)}` });
      } catch (error) {
        throw translateEndError(error);
      }
      const data = requireEnvelope(envelope);
      if (typeof data.id !== 'string' || typeof data.state !== 'string') throw new Error('voice session end data must carry id and state');
      return {
        id: data.id,
        state: data.state,
        settled: data.settled === true,
        ...(data.replay === true ? { replay: true } : {}),
        ...(typeof data.reason === 'string' && data.reason !== '' ? { reason: data.reason } : {}),
      };
    },
  };
}
