// 已登录访客带邀请链接打开 /login?token=xxx 时的守卫处置逻辑。
//
// 对齐 React 基准 apps/web/src/router.tsx loginBeforeLoad 的 token 分支：
// 已登录（或存储中持有 bearer —— 先恢复会话）时先用 token 兑换邀请，
// 兑换失败被静默吞掉（React 的 catch 分支无任何提示），随后无论成败都
// 进入应用首页：有效 token 下用户加入对应空间，失效 token 也不把已
// 登录用户困在登录页。
//
// 独立成纯模块（依赖注入）而不是写进 router/index.ts，是因为后者顶层
// 执行 createWebHistory(import.meta.env.BASE_URL) 且用 @/ 别名，node
// 测试（tsx --test）无法导入；同模式的先例见 src/config/settingsRoute.ts。

export const LOGIN_INVITE_APP_ENTRY = '/platform/knowledge-bases'

export interface LoginInviteRedeemDeps {
  /** SPA 内存会话是否已登录（authStore.isLoggedIn）。 */
  isLoggedIn(): boolean
  /** localStorage 是否持有 weknora_token（刷新加载时 store 尚未回填）。 */
  hasStoredToken(): boolean
  /** 从存储 token 恢复并校验会话（router 的 hydrateSessionFromToken）。 */
  hydrateStoredSession(): Promise<boolean>
  /** 兑换邀请 token（authStore.acceptInvitationByTokenAndRefresh；失败返回 ok:false 不抛）。 */
  acceptInvitation(token: string): Promise<{ ok: boolean }>
}

/**
 * 返回已登录访客兑换邀请后应进入的路由；'' 表示不适用（无 token /
 * 未登录），调用方维持既有守卫行为（弹回应用或放行到登录页走匿名
 * 邀请注册流）。
 */
export async function redeemLoginInviteForLoggedInVisitor(
  rawToken: unknown,
  deps: LoginInviteRedeemDeps,
): Promise<string> {
  const token = typeof rawToken === 'string' ? rawToken.trim() : ''
  if (!token) return ''
  let loggedIn = deps.isLoggedIn()
  if (!loggedIn && deps.hasStoredToken()) {
    loggedIn = await deps.hydrateStoredSession()
  }
  if (!loggedIn) return ''
  try {
    await deps.acceptInvitation(token)
  } catch {
    // React router.tsx catch 分支：兑换失败静默，仍进入应用。
  }
  return LOGIN_INVITE_APP_ENTRY
}
