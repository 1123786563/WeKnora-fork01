/** 构建期 App 身份（T37 #67）：官方标准客户端恒为 official；企业自构建构建以
 * EXPO_PUBLIC_WEKNORA_APP_ID 声明 enterprise:<slug>。非法值回落 official——
 * fail closed 到官方通道，企业注册只会在服务端策略允许时发生。 */
export const OFFICIAL_WEKNORA_APP_ID = 'official';
const ENTERPRISE_APP_ID_PATTERN = /^enterprise:[a-z0-9][a-z0-9-]{0,31}$/;

export function isValidWeKnoraAppId(appId: string): boolean {
  return appId === OFFICIAL_WEKNORA_APP_ID || ENTERPRISE_APP_ID_PATTERN.test(appId);
}

export function resolveWeKnoraAppId(raw: string | undefined): string {
  const trimmed = typeof raw === 'string' ? raw.trim() : '';
  return isValidWeKnoraAppId(trimmed) ? trimmed : OFFICIAL_WEKNORA_APP_ID;
}
