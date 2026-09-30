/**
 * 行动通知安全深链解析（#41，AC2「错误 deep link 的验证」）。
 *
 * 服务端 workbench_notifications.deep_link 列当前无生产写入方（迁移 000190 默认 ''），
 * 因此该值一律按不可信输入处理：白名单 scheme + host + path，参数逐项校验，
 * 其余全部拒绝（返回 undefined，调用方不得导航）。合法格式唯一：
 *   weknora://tasks/detail?taskId=<非空 ≤128>&runId=<非空 ≤128>
 * （参数名与 app 内文件路由 /tasks/detail 一致；weknora://oidc 属 OIDC 回调，不在通知白名单。）
 */

export interface TaskDetailDeepLinkTarget {
  kind: 'task-detail';
  taskId: string;
  runId: string;
}

export type DeepLinkTarget = TaskDetailDeepLinkTarget;

const DEEP_LINK_SCHEME = 'weknora:';
const DEEP_LINK_HOSTS: ReadonlySet<string> = new Set(['tasks']);
const DEEP_LINK_PATH = '/detail';
const ID_MAX_LENGTH = 128;
/** 对齐 workbench_notifications.deep_link 列宽（VARCHAR(512)）。 */
const DEEP_LINK_MAX_LENGTH = 512;
/** 解码后的 id 不得含控制字符（CRLF/头注入）。 */
const CONTROL_CHARACTERS = /[\u0000-\u001f\u007f]/;

export function parseNotificationDeepLink(value: string): DeepLinkTarget | undefined {
  if (typeof value !== 'string' || value.trim() === '') return undefined;
  if (value.length > DEEP_LINK_MAX_LENGTH) return undefined;
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    return undefined;
  }
  if (parsed.protocol !== DEEP_LINK_SCHEME) return undefined;
  if (!DEEP_LINK_HOSTS.has(parsed.host)) return undefined;
  if (parsed.pathname !== DEEP_LINK_PATH) return undefined;
  if (parsed.username !== '' || parsed.password !== '') return undefined;
  if (parsed.hash !== '') return undefined;
  const taskId = (parsed.searchParams.get('taskId') ?? '').trim();
  const runId = (parsed.searchParams.get('runId') ?? '').trim();
  if (taskId === '' || runId === '') return undefined;
  if (taskId.length > ID_MAX_LENGTH || runId.length > ID_MAX_LENGTH) return undefined;
  if (CONTROL_CHARACTERS.test(taskId) || CONTROL_CHARACTERS.test(runId)) return undefined;
  return { kind: 'task-detail', taskId, runId };
}
