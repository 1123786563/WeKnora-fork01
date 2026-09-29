import assert from 'node:assert/strict'
import test from 'node:test'

import { resolveChatIndexRedirect } from './chatIndexRedirect.ts'

test('bare /platform/chat redirects to the global chat entry (AGT-13)', async () => {
  // React /platform/chat 无 id 渲染完整聊天工作区（会话列表+工作区）；
  // Vue 等价物是 creatChat 全局入口，query 原样透传。
  assert.deepEqual(resolveChatIndexRedirect({}), {
    path: '/platform/creatChat',
    query: {},
  })
  assert.deepEqual(resolveChatIndexRedirect(null), {
    path: '/platform/creatChat',
    query: {},
  })
  assert.deepEqual(resolveChatIndexRedirect({ kb: '7' }), {
    path: '/platform/creatChat',
    query: { kb: '7' },
  })
  // 数组取首个、null 忽略（vue-router query 的多值形态）
  assert.deepEqual(resolveChatIndexRedirect({ kb: ['7', '8'], empty: null }), {
    path: '/platform/creatChat',
    query: { kb: '7' },
  })
})
