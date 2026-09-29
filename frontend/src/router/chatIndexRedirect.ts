// 裸 /platform/chat 的纯重定向决策。独立成无别名依赖的模块（先例：
// joinRedirect.ts / loginInvite.ts），router/index.ts 顶层执行
// createWebHistory 且用 @/ 别名，node 测试（tsx --test）无法导入。
//
// AGT-13：Vue 路由此前只注册 chat/:chatid，裸 /platform/chat 落到无组件
// 路由渲染空白；React 同路径渲染完整聊天工作区（会话列表+工作区）。
// 行为语义等价且改动最小的对齐：无 chatid 时重定向全局聊天入口
// /platform/creatChat（同一个 creatChat.vue 组件），query 原样透传。

export interface ChatIndexRedirectTarget {
  path: string
  query: Record<string, string>
}

/**
 * 解析裸 /platform/chat 的目标路由。query 取 RouteLocationNormalized
 * ['query']（string | string[] | null 值字典）；单值字符串原样透传，
 * 数组取首个，null 忽略。
 */
export function resolveChatIndexRedirect(query: unknown): ChatIndexRedirectTarget {
  const q = (query ?? {}) as Record<string, unknown>
  const passthrough: Record<string, string> = {}
  for (const [key, value] of Object.entries(q)) {
    if (typeof value === 'string') passthrough[key] = value
    else if (Array.isArray(value) && typeof value[0] === 'string') passthrough[key] = value[0]
  }
  return { path: '/platform/creatChat', query: passthrough }
}
