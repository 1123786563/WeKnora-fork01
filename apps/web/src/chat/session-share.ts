// SP13 Task 8 — 会话分享纯函数宿主侧工具：分享链接拼装、剪贴板写入、
// 只读页 token 归一与无效链接 404 判定。client.queryHistory.share 每次调用
// 都 mint/rotate 新 token（Task 5），链接形态固定为
// `${location.origin}/platform/shared/${share_token}`（Ruling P-2：只读页挂在
// platformRoute 下，复用登录守卫与 shell）。
import { ApiError } from '@weknora/api-client';

/** 只读分享页路由前缀（router.tsx platformRoute `shared/$token`）。 */
export const SHARED_SESSION_PATH_PREFIX = '/platform/shared/';

/** origin + share_token → 只读链接；token 为空即后端契约被破坏，直接抛错。 */
export function buildShareLink(origin: string, token: string): string {
  const normalizedOrigin = origin.trim().replace(/\/+$/, '');
  const shareToken = normalizeShareToken(token);
  if (!shareToken) throw new Error('share token must not be empty');
  return `${normalizedOrigin}${SHARED_SESSION_PATH_PREFIX}${encodeURIComponent(shareToken)}`;
}

/** 路由 / URL 读到的 token 归一（剥空白）；非字符串/空值返回空串。 */
export function normalizeShareToken(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

/** GET /api/v1/shared/sessions/:token 404 → 只读页“链接无效或已撤销”占位。 */
export function isShareLinkInvalidError(reason: unknown): boolean {
  return reason instanceof ApiError && reason.status === 404;
}

/**
 * 剪贴板写入（packages/views message-list writeClipboardText 先例的本地
 * 镜像：navigator.clipboard.writeText 优先，降级隐藏 textarea + execCommand）。
 * 不直接复用 views 导出是为了避免把 markdown 渲染链（marked/katex）拖进
 * PlatformShell 的模块图——shell 在每个 platform 页面与全部 node 测试里加载。
 */
export async function writeShareLinkClipboard(text: string, clipboard?: { writeText(t: string): Promise<void> }): Promise<void> {
  const target = clipboard ?? (typeof navigator !== 'undefined' ? navigator.clipboard : undefined);
  if (target?.writeText) {
    await target.writeText(text);
    return;
  }
  if (typeof document === 'undefined') throw new Error('Clipboard is unavailable');
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.setAttribute('readonly', '');
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  try { document.execCommand('copy'); } finally { textarea.remove(); }
}
