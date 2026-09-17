/**
 * 推送注册客户端边界（MX-021 / M10）。
 * 冻结规则：
 * - 设备环境隔离：注册绑定 (tenant, user, device)；登出撤销（服务端 RevokeDevicesForOwner）；
 * - 拒绝通知权限不影响任务（注册失败=静默降级为无推送，任务链照常）；
 * - 通知只提示：不携审批正文/凭据；注册请求不含任何敏感 token 之外的字段。
 */

export interface DeviceRegistrationPort {
  register(input: { device_id: string; token: string; platform: string }): Promise<void>;
}

export interface RegistrationOutcome {
  registered: boolean;
  /** 权限拒绝/失败=降级无推送（不阻塞任何任务功能） */
  degraded: boolean;
  reason?: 'permission_denied' | 'network';
}

export async function registerForNotifications(
  port: DeviceRegistrationPort,
  device: { id: string; pushToken: string | null; platform: string },
): Promise<RegistrationOutcome> {
  if (device.pushToken === null) {
    return { registered: false, degraded: true, reason: 'permission_denied' };
  }
  try {
    await port.register({ device_id: device.id, token: device.pushToken, platform: device.platform });
    return { registered: true, degraded: false };
  } catch {
    // 网络失败：降级（下次冷启动重试）；不重试风暴、不阻塞任务
    return { registered: false, degraded: true, reason: 'network' };
  }
}
