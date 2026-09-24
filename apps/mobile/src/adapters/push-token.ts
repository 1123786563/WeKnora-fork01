/**
 * 原生推送 token 适配器（module-seams §12：APNs/FCM 属 true external——Push Port +
 * scripted Adapter，真机验收单独进行）。Node 测试环境与无推送凭据的设备上
 * require('expo-notifications') 失败或取不到 token：返回 undefined，fail closed
 * （不注册设备，不阻塞登录/授权主流程）。
 */
export interface NativePushTokenSource {
  token(): Promise<string | undefined>;
}

export function createNativePushTokenIfAvailable(): NativePushTokenSource {
  return {
    async token(): Promise<string | undefined> {
      try {
        const notifications = require('expo-notifications') as {
          getDevicePushTokenAsync?: () => Promise<{ data?: unknown }>;
        };
        if (typeof notifications.getDevicePushTokenAsync !== 'function') return undefined;
        const push = await notifications.getDevicePushTokenAsync();
        return typeof push?.data === 'string' && push.data.trim() !== '' ? push.data : undefined;
      } catch {
        return undefined;
      }
    },
  };
}
