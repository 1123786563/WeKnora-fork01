import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';

/**
 * Device Registration 深模块（#41，spec §4「注册设备」+ CONTEXT.md「注册设备」）：
 * - 两步注册：先取服务端签名的 registration intent（epoch=current+1，5 分钟过期），
 *   再以 intent + token 完成绑定——客户端不能自造未来 epoch；
 * - 409（intent 过期/并发）：恰好重取一次新 intent 重试，二次冲突显式 DEVICE_CONFLICT；
 * - token 接管：同 deviceId 重复 register 直接绑定新 token（服务端 upsert，revision 递增）；
 * - 单飞：同一 registry 上的注册/撤销串行提交，并发接管不交错 epoch；
 * - scope 围栏：每次提交前与两步之间检查 lease（切租户/换部署/登出即拒，迟到结果不落地）。
 * token 经 authorizedRequest 通道传输，从不进入返回记录、视图或错误文本。
 */

export type DevicePlatform = 'ios' | 'android';

export interface DeviceRegistrationRecord {
  deviceId: string;
  platform: string;
  environment: string;
  revision: number;
  scopeGeneration: number;
  revokedAt?: string;
  lastSeenAt?: string;
}

export interface DeviceRemote {
  issueIntent(deviceId: string): Promise<{ registrationIntent: string; scopeGeneration: number }>;
  register(input: { deviceId: string; token: string; platform: string; registrationIntent: string }): Promise<DeviceRegistrationRecord>;
  revoke(input: { deviceId: string; revision?: number }): Promise<void>;
  list(): Promise<DeviceRegistrationRecord[]>;
}

export interface DevicePorts {
  remote: DeviceRemote;
  lease(): ScopeLease | undefined;
}

export type DeviceErrorCode =
  | 'DEVICE_SCOPE_CHANGED'
  | 'DEVICE_INVALID_INPUT'
  | 'DEVICE_CONFLICT'
  | 'DEVICE_NOT_FOUND'
  | 'DEVICE_BACKEND';

export class DeviceError extends Error {
  constructor(readonly code: DeviceErrorCode, options?: { cause?: unknown }) {
    super(code, options);
    this.name = 'DeviceError';
  }
}

export interface DeviceRegistry {
  register(input: { deviceId: string; token: string; platform: DevicePlatform }): Promise<DeviceRegistrationRecord>;
  revoke(input: { deviceId: string; revision?: number }): Promise<void>;
  list(): Promise<DeviceRegistrationRecord[]>;
}

const deviceIdMaxLength = 128;
/** 对齐服务端 token 上限（internal/handler/mobile_device.go:195）。 */
const tokenMaxLength = 4096;

function wireStatus(error: unknown): number | undefined {
  if (typeof error !== 'object' || error === null) return undefined;
  if ((error as { name?: unknown }).name !== 'ApiError') return undefined;
  const status = (error as { status?: unknown }).status;
  return typeof status === 'number' ? status : undefined;
}

export function createDeviceRegistry(ports: DevicePorts): DeviceRegistry {
  let queue: Promise<unknown> = Promise.resolve();
  const serialized = <T>(action: () => Promise<T>): Promise<T> => {
    const next = queue.then(action, action);
    queue = next.then(() => {}, () => {});
    return next;
  };
  const requireLease = (): ScopeLease => {
    const lease = ports.lease();
    if (lease === undefined || !leaseActive(lease)) throw new DeviceError('DEVICE_SCOPE_CHANGED');
    return lease;
  };
  const attemptRegister = async (
    lease: ScopeLease,
    deviceId: string,
    token: string,
    platform: string,
  ): Promise<DeviceRegistrationRecord> => {
    const intent = await ports.remote.issueIntent(deviceId);
    if (!leaseActive(lease)) throw new DeviceError('DEVICE_SCOPE_CHANGED');
    return ports.remote.register({ deviceId, token, platform, registrationIntent: intent.registrationIntent });
  };
  return {
    register(input) {
      const deviceId = typeof input.deviceId === 'string' ? input.deviceId.trim() : '';
      const token = typeof input.token === 'string' ? input.token.trim() : '';
      const platform = input.platform;
      if (deviceId === '' || deviceId.length > deviceIdMaxLength) return Promise.reject(new DeviceError('DEVICE_INVALID_INPUT'));
      if (token === '' || token.length > tokenMaxLength) return Promise.reject(new DeviceError('DEVICE_INVALID_INPUT'));
      if (platform !== 'ios' && platform !== 'android') return Promise.reject(new DeviceError('DEVICE_INVALID_INPUT'));
      return serialized(async () => {
        const lease = requireLease();
        try {
          return await attemptRegister(lease, deviceId, token, platform);
        } catch (error) {
          if (error instanceof DeviceError) throw error;
          if (wireStatus(error) !== 409) throw new DeviceError('DEVICE_BACKEND', { cause: error });
          if (!leaseActive(lease)) throw new DeviceError('DEVICE_SCOPE_CHANGED');
          try {
            // intent 过期/并发（409）：恰好重取一次新 intent 再试（Review Focus #4：重试有界）
            return await attemptRegister(lease, deviceId, token, platform);
          } catch (retryError) {
            if (retryError instanceof DeviceError) throw retryError;
            if (wireStatus(retryError) === 409) throw new DeviceError('DEVICE_CONFLICT', { cause: retryError });
            throw new DeviceError('DEVICE_BACKEND', { cause: retryError });
          }
        }
      });
    },
    revoke(input) {
      const deviceId = typeof input.deviceId === 'string' ? input.deviceId.trim() : '';
      if (deviceId === '' || deviceId.length > deviceIdMaxLength) return Promise.reject(new DeviceError('DEVICE_INVALID_INPUT'));
      if (input.revision !== undefined && (!Number.isSafeInteger(input.revision) || input.revision <= 0)) {
        return Promise.reject(new DeviceError('DEVICE_INVALID_INPUT'));
      }
      return serialized(async (): Promise<void> => {
        requireLease();
        try {
          await ports.remote.revoke({ deviceId, ...(input.revision === undefined ? {} : { revision: input.revision }) });
        } catch (error) {
          if (error instanceof DeviceError) throw error;
          if (wireStatus(error) === 404) throw new DeviceError('DEVICE_NOT_FOUND', { cause: error });
          throw new DeviceError('DEVICE_BACKEND', { cause: error });
        }
      });
    },
    async list() {
      requireLease();
      try {
        return await ports.remote.list();
      } catch (error) {
        if (error instanceof DeviceError) throw error;
        throw new DeviceError('DEVICE_BACKEND', { cause: error });
      }
    },
  };
}
