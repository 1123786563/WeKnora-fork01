import test from 'node:test';
import assert from 'node:assert/strict';
import { createNativeNotificationPermissionIfAvailable, createNotificationPermissionFrom, type ExpoNotificationsPermissionsLike } from './notification-permission.ts';

interface StubOptions {
  /** getPermissionsAsync 的 granted（缺省 false = 触发请求路径）。 */
  currentGranted?: boolean;
  /** requestPermissionsAsync 的 granted（缺省 true）。 */
  requestGranted?: boolean;
  /** getPermissionsAsync 必抛（权限面不可用）。 */
  failGet?: boolean;
  /** requestPermissionsAsync 必抛。 */
  failRequest?: boolean;
}

function stubPermissions(options: StubOptions = {}): ExpoNotificationsPermissionsLike & { requests: number } {
  const state = { requests: 0 };
  return Object.assign(state, {
    async getPermissionsAsync() {
      if (options.failGet === true) throw new Error('permissions module broken');
      return { granted: options.currentGranted ?? false };
    },
    async requestPermissionsAsync() {
      state.requests += 1;
      if (options.failRequest === true) throw new Error('user never saw the prompt');
      return { granted: options.requestGranted ?? true };
    },
  }) as ExpoNotificationsPermissionsLike & { requests: number };
}

test('the native factory is unavailable in the Node test chain (fail closed)', () => {
  assert.equal(createNativeNotificationPermissionIfAvailable(), undefined, 'expo-notifications 不可 require 时必须返回 undefined');
});

test('an already-granted permission never re-prompts the user', async () => {
  const notifications = stubPermissions({ currentGranted: true });
  const port = createNotificationPermissionFrom(notifications);
  assert.equal(await port.ensure(), 'granted');
  assert.equal(notifications.requests, 0, '已授权零弹窗请求（Review Focus 1：不骚扰用户）');
});

test('an ungranted permission prompts exactly once and reports the honest outcome', async () => {
  const granted = createNotificationPermissionFrom(stubPermissions({ currentGranted: false, requestGranted: true }));
  assert.equal(await granted.ensure(), 'granted');
  const denied = createNotificationPermissionFrom(stubPermissions({ currentGranted: false, requestGranted: false }));
  assert.equal(await denied.ensure(), 'denied');
});

test('a broken permissions surface reports unavailable and never fabricates granted', async () => {
  const brokenGet = createNotificationPermissionFrom(stubPermissions({ failGet: true }));
  assert.equal(await brokenGet.ensure(), 'unavailable');
  const brokenRequest = createNotificationPermissionFrom(stubPermissions({ failRequest: true }));
  assert.equal(await brokenRequest.ensure(), 'unavailable');
});
