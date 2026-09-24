import type { SecureStorePort } from './secure-store.ts';

const DEVICE_ID_KEY = 'weknora.device-id.v1';

/** 稳定设备身份：SecureStore 持久的随机 id（首次生成后复用）。设备身份按 Deployment 隔离由服务端 (tenant, owner, device) 唯一键承担（ADR-0007）。 */
export interface NativeDeviceIdentity {
  deviceId(): Promise<string | undefined>;
}

export function createSecureDeviceIdentity(store: SecureStorePort): NativeDeviceIdentity {
  let inflight: Promise<string | undefined> | undefined;
  return {
    async deviceId(): Promise<string | undefined> {
      // 并发首次调用共享同一次 read-generate-write（B3-F49）：各自生成 uuid 后
      // 写覆盖先写会产生收不到推送的幽灵设备记录。
      if (inflight !== undefined) return inflight;
      inflight = (async (): Promise<string | undefined> => {
        try {
          const existing = await store.getItemAsync(DEVICE_ID_KEY);
          if (typeof existing === 'string' && existing.trim() !== '') return existing.trim();
          const generated = globalThis.crypto?.randomUUID
            ? globalThis.crypto.randomUUID()
            : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
          await store.setItemAsync(DEVICE_ID_KEY, generated);
          return generated;
        } catch {
          return undefined; // fail closed：无安全存储即无设备身份，不注册
        }
      })();
      try {
        return await inflight;
      } finally {
        inflight = undefined; // 完成后允许后续直读
      }
    },
  };
}

export function createNativeDeviceIdentity(): NativeDeviceIdentity {
  try {
    const secure = require('expo-secure-store') as SecureStorePort;
    return createSecureDeviceIdentity(secure);
  } catch {
    return { deviceId: async () => undefined };
  }
}

/** 原生平台（服务端仅接受 ios/android 枚举，internal/handler/mobile_device.go:195）。
 * 惰性 require：Node 测试 stub 环境解析失败时保守回落 android。不得在模块顶层具名
 * import react-native 的 Platform（app-smoke stub 未导出它）。 */
export function nativeDevicePlatform(): 'ios' | 'android' {
  try {
    const reactNative = require('react-native') as { Platform?: { OS?: string } };
    return reactNative.Platform?.OS === 'ios' ? 'ios' : 'android';
  } catch {
    return 'android';
  }
}
