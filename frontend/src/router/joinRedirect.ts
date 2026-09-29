// /join 入口的纯重定向决策。独立成无别名依赖的模块（先例：
// settingsRoute.ts / loginInvite.ts），router/index.ts 顶层执行
// createWebHistory 且用 @/ 别名，node 测试（tsx --test）无法导入。
//
// 对齐 React 基准 apps/web/src/router.tsx joinRoute beforeLoad 的分支：
// - 带 token（邀请分享链接）：一律转发 /register?token=…，不在 /join 兑换。
//   React 用 window.location.replace 硬跳 register，后续由 loginBeforeLoad
//   的 token 分支接手——已登录先 accept-by-token 再进应用（Vue 侧对应守卫
//   的 redeemLoginInviteForLoggedInVisitor，见 index.ts 的 /login|/register
//   分支）；匿名则由登录/注册卡 lookup token，失败横幅停在原 URL。
//   这样 /join 不再是 token 的死胡同（AUTH-2：此前 token 被丢弃落裸
//   /login；AUTH-8：已登录静默弹回组织页且从不兑换）。
// - 无 token：维持原有落地页——组织列表，并把旧版 code 参数转换为
//   invite_code（React joinRoute 的 fallbackPath 同为 organizations）。

export interface JoinRedirectTarget {
  path: string
  query: Record<string, string>
}

/**
 * 解析 /join 的目标路由。query 取 RouteLocationNormalized['query']
 * （string | string[] | null 值字典）；token 仅接受单一字符串值，
 * 数组/空串视为缺失（对齐 React validateSearch 的 string 判定）。
 */
export function resolveJoinRedirect(query: unknown): JoinRedirectTarget {
  const q = (query ?? {}) as Record<string, unknown>
  const token = typeof q.token === 'string' ? q.token.trim() : ''
  if (token) {
    return { path: '/register', query: { token } }
  }
  const code = typeof q.code === 'string' ? q.code.trim() : ''
  return {
    path: '/platform/organizations',
    query: code ? { invite_code: code } : {},
  }
}
