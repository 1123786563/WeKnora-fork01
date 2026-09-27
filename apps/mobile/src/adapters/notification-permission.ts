/**
 * 通知权限 Adapter（Issue #70：Android 13+ POST_NOTIFICATIONS 运行时权限）。Android 13 起
 * 通知运行时权限弹窗不随 FCM token 获取自动出现：未授权时通知静默不显示，设备注册与 Inbox
 * 权威同步形同虚设。iOS 的授权请求与本 Adapter 走同一 Expo 面（平台差异收敛在
 * expo-notifications 内部，App 层无平台分支——module-seams §10「平台差异仅在 Adapter」）。
 * 惰性 require：Node 测试链/缺原生配置的构建解析失败返回 undefined（fail closed：不注册
 * 设备、不阻塞登录/授权主流程——与 push-token/device-identity 同型先例）。
 */
export type NotificationPermissionOutcome = 'granted' | 'denied' | 'unavailable';

export interface NotificationPermissionPort {
  /** 已授权零弹窗；未授权恰好请求一次；结果如实三值，绝不伪造 granted。 */
  ensure(): Promise<NotificationPermissionOutcome>;
}

/** expo-notifications 权限面的结构切片（只依赖本 Adapter 真正消费的形状）。 */
export interface ExpoNotificationsPermissionsLike {
  getPermissionsAsync(): Promise<{ granted?: boolean }>;
  requestPermissionsAsync(): Promise<{ granted?: boolean }>;
}

/** 注入式构造（测试注入 stub；与 dictation-capture 的 ExpoAudioLike 注入同型）。 */
export function createNotificationPermissionFrom(notifications: ExpoNotificationsPermissionsLike): NotificationPermissionPort {
  return {
    async ensure() {
      try {
        const current = await notifications.getPermissionsAsync();
        if (current?.granted === true) return 'granted'; // 已授权：零弹窗请求
      } catch {
        return 'unavailable'; // 权限面不可用：如实返回 'unavailable'，不伪造结论（与下方 requestPermissionsAsync 失败路径同型）
      }
      try {
        const requested = await notifications.requestPermissionsAsync();
        return requested?.granted === true ? 'granted' : 'denied';
      } catch {
        return 'unavailable';
      }
    },
  };
}

/** 原生组合路径（惰性 require；解析失败/缺导出 fail closed → undefined）。 */
export function createNativeNotificationPermissionIfAvailable(): NotificationPermissionPort | undefined {
  try {
    const notifications = require('expo-notifications') as Partial<ExpoNotificationsPermissionsLike>;
    if (typeof notifications.getPermissionsAsync !== 'function' || typeof notifications.requestPermissionsAsync !== 'function') {
      return undefined;
    }
    return createNotificationPermissionFrom(notifications as ExpoNotificationsPermissionsLike);
  } catch {
    return undefined;
  }
}
