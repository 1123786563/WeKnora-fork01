import type { DeviceRemote, DeviceRegistrationRecord } from './device-registry.ts';

/** ApiError 结构性形态（packages/api-client/src/errors.ts:24-33）：name + status。 */
export function wireError(status: number, message: string): Error {
  return Object.assign(new Error(message), { name: 'ApiError', status });
}

export interface ScenarioDeviceSnapshot {
  active: DeviceRegistrationRecord[];
  intentsIssued: number;
}

export interface ScenarioDeviceRemote {
  remote: DeviceRemote;
  snapshot(): ScenarioDeviceSnapshot;
  /** 模拟过期/并发 intent：接下来 count 次 register 调用一律 409。 */
  conflictNextRegisters(count: number): void;
}

/**
 * in-memory scenario Adapter（module-seams §12：remote-but-owned 依赖用 in-memory
 * scenario Adapter 做 Module 测试）。复刻服务端两步注册的关键语义：intent 携带
 * current+1 epoch、bind 校验 intent、token 接管 bump revision（internal/handler/mobile_device.go:74-101/178-241）。
 */
export function createScenarioDeviceRemote(): ScenarioDeviceRemote {
  const rows = new Map<string, DeviceRegistrationRecord>();
  let epoch = 0;
  let conflicts = 0;
  let intents = 0;
  return {
    remote: {
      async issueIntent(deviceId) {
        intents += 1;
        epoch += 1;
        return { registrationIntent: `intent:${deviceId}:${epoch}`, scopeGeneration: epoch };
      },
      async register(input) {
        if (conflicts > 0) {
          conflicts -= 1;
          throw wireError(409, 'registration intent is stale');
        }
        if (input.registrationIntent !== `intent:${input.deviceId}:${epoch}`) {
          throw wireError(409, 'registration intent is stale');
        }
        const existing = rows.get(input.deviceId);
        const record: DeviceRegistrationRecord = {
          deviceId: input.deviceId,
          platform: input.platform,
          environment: 'development',
          revision: (existing?.revision ?? 0) + 1,
          scopeGeneration: epoch,
        };
        rows.set(input.deviceId, record);
        return { ...record };
      },
      async revoke(input) {
        const row = rows.get(input.deviceId);
        if (row === undefined) throw wireError(404, 'mobile device not found');
        if (input.revision !== undefined && input.revision !== row.revision) {
          throw wireError(409, 'mobile device revision conflict');
        }
        rows.delete(input.deviceId);
      },
      async list() {
        return [...rows.values()].map((row) => ({ ...row }));
      },
    },
    snapshot() {
      return { active: [...rows.values()].map((row) => ({ ...row })), intentsIssued: intents };
    },
    conflictNextRegisters(count) {
      conflicts = count;
    },
  };
}
