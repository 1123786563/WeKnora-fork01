import type { ClientRequest } from '../client.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface MobileDeviceRemoteOptions {
  /**
   * 部署 Origin。构造即强校验（共享 requireDeploymentOrigin，B3-F11 收敛）：
   * 绝对 HTTPS、含 host、无 userinfo、无 path/query/fragment。
   */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
  request: Request;
}

/** 与 mobile-core DeviceRegistrationRecord 结构逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface MobileDeviceRegistration {
  deviceId: string;
  platform: string;
  environment: string;
  revision: number;
  scopeGeneration: number;
  revokedAt?: string;
  lastSeenAt?: string;
}

export interface MobileDeviceRemote {
  /** POST /api/v1/mobile/devices/:id/registration-intent（internal/handler/mobile_device.go:129）。 */
  issueIntent(deviceId: string): Promise<{ registrationIntent: string; scopeGeneration: number }>;
  /** PUT /api/v1/mobile/devices/:id（mobile_device.go:178）——两步注册第二步。 */
  register(input: { deviceId: string; token: string; platform: string; registrationIntent: string }): Promise<MobileDeviceRegistration>;
  /** DELETE /api/v1/mobile/devices/:id[?revision=N]（mobile_device.go:243）——204。 */
  revoke(input: { deviceId: string; revision?: number }): Promise<void>;
  /** GET /api/v1/mobile/devices（mobile_device.go:386）——敏感列 json:"-" 不上 wire。 */
  list(): Promise<MobileDeviceRegistration[]>;
}

function requireDeviceId(deviceId: string): string {
  const trimmed = typeof deviceId === 'string' ? deviceId.trim() : '';
  if (trimmed === '') throw new Error('device id is required');
  return trimmed;
}

function unwrap(value: unknown, label: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) throw new Error(`${label} response must be a success envelope`);
  const envelope = value as { success?: unknown; data?: unknown };
  if (envelope.success !== true || !Object.prototype.hasOwnProperty.call(envelope, 'data')) {
    throw new Error(`${label} response.success must be true with data`);
  }
  if (typeof envelope.data !== 'object' || envelope.data === null || Array.isArray(envelope.data)) {
    throw new Error(`${label} data must be an object`);
  }
  return envelope.data as Record<string, unknown>;
}

function optionalTimestamp(data: Record<string, unknown>, key: string): string | undefined {
  const value = data[key];
  return typeof value === 'string' && value.trim() !== '' ? value : undefined;
}

export function createMobileDeviceRemote(options: MobileDeviceRemoteOptions): MobileDeviceRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  const devicePath = (deviceId: string): string => `/api/v1/mobile/devices/${encodeURIComponent(requireDeviceId(deviceId))}`;
  return {
    async issueIntent(deviceId: string): Promise<{ registrationIntent: string; scopeGeneration: number }> {
      const data = unwrap(await request({ method: 'POST', path: `${devicePath(deviceId)}/registration-intent` }), 'device intent');
      if (typeof data.registration_intent !== 'string' || data.registration_intent.trim() === '') {
        throw new Error('device intent registration_intent is required');
      }
      if (typeof data.scope_generation !== 'number' || !Number.isSafeInteger(data.scope_generation) || data.scope_generation < 0) {
        throw new Error('device intent scope_generation must be an integer');
      }
      return { registrationIntent: data.registration_intent, scopeGeneration: data.scope_generation };
    },
    async register(input: { deviceId: string; token: string; platform: string; registrationIntent: string }): Promise<MobileDeviceRegistration> {
      if (typeof input.token !== 'string' || input.token.trim() === '') throw new Error('device token is required');
      if (typeof input.platform !== 'string' || input.platform.trim() === '') throw new Error('device platform is required');
      if (typeof input.registrationIntent !== 'string' || input.registrationIntent.trim() === '') throw new Error('device registration_intent is required');
      const data = unwrap(await request({
        method: 'PUT',
        path: devicePath(input.deviceId),
        body: { token: input.token, platform: input.platform, registration_intent: input.registrationIntent },
      }), 'device register');
      const registration: MobileDeviceRegistration = {
        deviceId: requireDeviceId(String(data.device_id ?? '')),
        platform: String(data.platform ?? ''),
        environment: String(data.environment ?? ''),
        revision: Number(data.revision ?? 0),
        scopeGeneration: Number(data.scope_generation ?? 0),
        ...(optionalTimestamp(data, 'revoked_at') === undefined ? {} : { revokedAt: optionalTimestamp(data, 'revoked_at')! }),
        ...(optionalTimestamp(data, 'last_seen_at') === undefined ? {} : { lastSeenAt: optionalTimestamp(data, 'last_seen_at')! }),
      };
      if (registration.deviceId === '') throw new Error('device register device_id is required');
      return registration;
    },
    async revoke(input: { deviceId: string; revision?: number }): Promise<void> {
      if (input.revision !== undefined && (!Number.isSafeInteger(input.revision) || input.revision <= 0)) {
        throw new Error('device revision must be a positive integer');
      }
      const query = input.revision === undefined ? '' : `?revision=${input.revision}`;
      await request({ method: 'DELETE', path: `${devicePath(input.deviceId)}${query}` }); // 204：无 body 可解
    },
    async list(): Promise<MobileDeviceRegistration[]> {
      const response = await request({ method: 'GET', path: '/api/v1/mobile/devices' });
      if (typeof response !== 'object' || response === null || Array.isArray(response)) {
        throw new Error('device list response must be a success envelope');
      }
      const envelope = response as { success?: unknown; data?: unknown };
      if (envelope.success !== true || !Array.isArray(envelope.data)) {
        throw new Error('device list response.success must be true with a data array');
      }
      const rows = envelope.data as Array<Record<string, unknown>>;
      // wire 行的敏感列（TokenCiphertext/TokenHash/TenantID/SpaceID）在服务端就是 json:"-"（internal/application/repository/mobile_device.go:29-44），
      // 适配器只提取白名单字段，结构性杜绝 token 材料进入语义行（Review Focus #3）。
      return rows.map((row) => ({
        deviceId: typeof row.device_id === 'string' ? row.device_id : '',
        platform: typeof row.platform === 'string' ? row.platform : '',
        environment: typeof row.environment === 'string' ? row.environment : '',
        revision: typeof row.revision === 'number' ? row.revision : 0,
        scopeGeneration: typeof row.scope_generation === 'number' ? row.scope_generation : 0,
        ...(optionalTimestamp(row, 'revoked_at') === undefined ? {} : { revokedAt: optionalTimestamp(row, 'revoked_at')! }),
        ...(optionalTimestamp(row, 'last_seen_at') === undefined ? {} : { lastSeenAt: optionalTimestamp(row, 'last_seen_at')! }),
      }));
    },
  };
}
