import type { NetworkStatusPort } from '@weknora/mobile-core';

/** 惰性解析 expo-network（Node 测试链无此模块；与 push-token/device-identity 同模式）。
 *  解析失败返回 undefined——组合根不装配离线拦截（gate 透传语义，见 offline-gate.ts）。 */
export function createNativeNetworkStatusIfAvailable(): NetworkStatusPort | undefined {
  try {
    const network = require('expo-network') as { getNetworkStateAsync(): Promise<{ isInternetReachable: boolean | null }> };
    return {
      async online() {
        return (await network.getNetworkStateAsync()).isInternetReachable === true; // null/undefined 一律离线（fail closed）
      },
    };
  } catch {
    return undefined;
  }
}
