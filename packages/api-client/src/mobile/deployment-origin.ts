/**
 * 部署 Origin 的单一构造期防线（B3-F11 收敛，最严格合并版）。
 * 此前在 mobile/ 下有 7 份逐字拷贝且已漂移（inbox 版多 hostname 检查）——
 * 校验规则演进现在只改这一处。八条规则：非空 / 绝对 URL / HTTPS / 无 user
 * info / 含 host / 根路径 / 无 query / 无 fragment；通过返回 parsed.origin。
 */
export function requireDeploymentOrigin(origin: string): string {
  let parsed: URL;
  if (typeof origin !== 'string' || origin.trim() === '') throw new Error('deployment origin is required');
  try { parsed = new URL(origin); } catch { throw new Error(`deployment origin must be an absolute URL: ${origin}`); }
  if (parsed.protocol !== 'https:') throw new Error('deployment origin must use HTTPS');
  if (parsed.username !== '' || parsed.password !== '') throw new Error('deployment origin must not embed user info');
  if (parsed.hostname === '') throw new Error('deployment origin must include a host');
  if (parsed.pathname !== '/') throw new Error('deployment origin must not include a path');
  if (parsed.search !== '' || parsed.hash !== '') throw new Error('deployment origin must not include a query or fragment');
  return parsed.origin;
}
