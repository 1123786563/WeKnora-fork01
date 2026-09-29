import assert from 'node:assert/strict'
import test from 'node:test'

import { resolveJoinRedirect } from './joinRedirect.ts'

test('token-carrying join links forward to /register preserving the token', async () => {
  // React joinRoute beforeLoad：window.location.replace(`/register?token=…`)，
  // 无论登录态——兑换/校验交给 register 入口的 loginBeforeLoad（Vue 侧为
  // 守卫 redeemLoginInviteForLoggedInVisitor + Login.vue 邀请注册流）。
  assert.deepEqual(resolveJoinRedirect({ token: 'invite-1' }), {
    path: '/register',
    query: { token: 'invite-1' },
  })
  // 空白 token 视为缺失（trim 后对齐 React 的非空判定）
  assert.deepEqual(resolveJoinRedirect({ token: '   ' }), {
    path: '/platform/organizations',
    query: {},
  })
  // 数组 token 视为缺失（对齐 React validateSearch 仅接受 string）
  assert.deepEqual(resolveJoinRedirect({ token: ['a', 'b'] }), {
    path: '/platform/organizations',
    query: {},
  })
})

test('anonymous join token wins over code — code is dropped like React', async () => {
  // React joinRoute：token 分支先行并 return，code 只在无 token 的
  // authenticated 分支里作为 invite_code 透传。
  assert.deepEqual(resolveJoinRedirect({ token: 'invite-1', code: 'legacy' }), {
    path: '/register',
    query: { token: 'invite-1' },
  })
})

test('no-token join links keep the organizations landing with code→invite_code', async () => {
  assert.deepEqual(resolveJoinRedirect({ code: 'legacy-code' }), {
    path: '/platform/organizations',
    query: { invite_code: 'legacy-code' },
  })
  assert.deepEqual(resolveJoinRedirect({}), {
    path: '/platform/organizations',
    query: {},
  })
  assert.deepEqual(resolveJoinRedirect(null), {
    path: '/platform/organizations',
    query: {},
  })
})
