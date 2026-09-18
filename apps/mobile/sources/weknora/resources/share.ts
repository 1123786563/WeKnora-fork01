import { Share } from 'react-native';

/**
 * 系统分享边界（MX-024）：外流范围确认后调用系统分享面板；
 * 分享内容只携带标题与版本说明（不含签名 URL——URL 是单次访问授权，不外流）。
 */
export async function shareArtifactViaSystem(title: string, versionDescription: string): Promise<boolean> {
  try {
    const result = await Share.share({ title, message: `${title} — ${versionDescription}` });
    return result.action === Share.sharedAction;
  } catch {
    return false;
  }
}
