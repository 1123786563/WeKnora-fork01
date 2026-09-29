import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

import {
  LOGIN_INVITE_APP_ENTRY,
  redeemLoginInviteForLoggedInVisitor,
  type LoginInviteRedeemDeps,
} from './loginInvite.ts'

interface CallLog {
  hydrate: number
  accept: string[]
}

function makeDeps(
  overrides: Partial<LoginInviteRedeemDeps> = {},
): { deps: LoginInviteRedeemDeps; calls: CallLog } {
  const calls: CallLog = { hydrate: 0, accept: [] }
  const deps: LoginInviteRedeemDeps = {
    isLoggedIn: () => false,
    hasStoredToken: () => false,
    hydrateStoredSession: async () => {
      calls.hydrate += 1
      return true
    },
    acceptInvitation: async (token: string) => {
      calls.accept.push(token)
      return { ok: true }
    },
    ...overrides,
  }
  return { deps, calls }
}

test('a logged-in visitor with an invite token redeems it and enters the app', async () => {
  // SPA 场景：内存会话已登录（React router.tsx:247-254 —— accept 后无条件
  // replace('/platform/knowledge-bases')）。
  const { deps, calls } = makeDeps({ isLoggedIn: () => true })
  const target = await redeemLoginInviteForLoggedInVisitor('invite-1', deps)
  assert.equal(target, LOGIN_INVITE_APP_ENTRY)
  assert.equal(target, '/platform/knowledge-bases')
  assert.deepEqual(calls.accept, ['invite-1'])
  assert.equal(calls.hydrate, 0, 'an in-memory session needs no re-hydration')
})

test('a fresh page load hydrates the stored session before redeeming', async () => {
  // 刷新加载：main.ts 的 refreshFromAuthMe 只回填 user 不写 store token，
  // isLoggedIn 为 false —— 守卫必须先用存储 token 恢复会话再兑换，
  // 否则邀请 token 会被静默丢弃（对齐 React 读存储 bearer credential）。
  const { deps, calls } = makeDeps({ hasStoredToken: () => true })
  const target = await redeemLoginInviteForLoggedInVisitor('invite-2', deps)
  assert.equal(target, LOGIN_INVITE_APP_ENTRY)
  assert.equal(calls.hydrate, 1)
  assert.deepEqual(calls.accept, ['invite-2'])
})

test('a stale stored session falls back to the anonymous invite flow', async () => {
  // 存储 token 已失效（/auth/me 校验失败）：不算已登录，不发起 accept，
  // 维持放行到 /login 由 Login.vue 走邀请注册流。
  const calls: CallLog = { hydrate: 0, accept: [] }
  const deps: LoginInviteRedeemDeps = {
    isLoggedIn: () => false,
    hasStoredToken: () => true,
    hydrateStoredSession: async () => {
      calls.hydrate += 1
      return false
    },
    acceptInvitation: async (token: string) => {
      calls.accept.push(token)
      return { ok: true }
    },
  }
  const target = await redeemLoginInviteForLoggedInVisitor('invite-3', deps)
  assert.equal(target, '')
  assert.equal(calls.hydrate, 1)
  assert.deepEqual(calls.accept, [])
})

test('a failed redemption still enters the app (React silent catch branch)', async () => {
  const failing = makeDeps({
    isLoggedIn: () => true,
    acceptInvitation: async () => ({ ok: false }),
  })
  assert.equal(await redeemLoginInviteForLoggedInVisitor('dead-token', failing.deps), LOGIN_INVITE_APP_ENTRY)

  // 兑换调用本身抛异常也不得把已登录用户困在守卫里。
  const throwing = makeDeps({
    isLoggedIn: () => true,
    acceptInvitation: async () => {
      throw new Error('network down')
    },
  })
  assert.equal(await redeemLoginInviteForLoggedInVisitor('dead-token', throwing.deps), LOGIN_INVITE_APP_ENTRY)
})

test('an anonymous visitor without a stored token is left to the login page', async () => {
  const { deps, calls } = makeDeps()
  const target = await redeemLoginInviteForLoggedInVisitor('invite-4', deps)
  assert.equal(target, '')
  assert.equal(calls.hydrate, 0)
  assert.deepEqual(calls.accept, [])
})

test('missing or malformed tokens keep the unconditional /login bounce', async () => {
  for (const raw of [undefined, null, '', '   ', 42, ['invite-a', 'invite-b']]) {
    const { deps, calls } = makeDeps({ isLoggedIn: () => true })
    assert.equal(await redeemLoginInviteForLoggedInVisitor(raw, deps), '', `raw=${String(raw)}`)
    assert.deepEqual(calls, { hydrate: 0, accept: [] }, `raw=${String(raw)}`)
  }
})

test('the router guard wires the redemption into its /login branch', () => {
  // router/index.ts 顶层执行 createWebHistory(import.meta.env.BASE_URL)，
  // node 测试无法导入，按 authRefresh.test.ts 的先例做源码级接线断言。
  const source = readFileSync(new URL('./index.ts', import.meta.url), 'utf8')
  assert.match(source, /redeemLoginInviteForLoggedInVisitor\(to\.query\.token/)
  assert.match(source, /hydrateStoredSession: \(\) => hydrateSessionFromToken\(authStore\)/)
  assert.match(source, /acceptInvitation: \(token\) => authStore\.acceptInvitationByTokenAndRefresh\(token\)/)
})
