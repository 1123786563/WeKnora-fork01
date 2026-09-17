import * as React from 'react';
// @ts-expect-error react-test-renderer has no declarations in this workspace.
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({
  initialURL: null as string | null,
  lastResponse: null as any,
  onURL: null as ((event: { url: string }) => void) | null,
  onNotification: null as ((response: any) => void) | null,
  replace: vi.fn(),
  auth: null as any,
}));

vi.mock('react-native', async () => {
  const ReactModule = await import('react');
  const host = (name: string) => (props: any) => ReactModule.createElement(name, props, props.children);
  return { Linking: {
    getInitialURL: () => Promise.resolve(mocks.initialURL),
    addEventListener: (_event: string, listener: (event: { url: string }) => void) => {
      mocks.onURL = listener;
      return { remove: () => { mocks.onURL = null; } };
    },
  }, Pressable: host('Pressable'), Text: host('Text'), View: host('View') };
});
vi.mock('expo-notifications', () => ({
  addNotificationResponseReceivedListener: (listener: (response: any) => void) => {
    mocks.onNotification = listener;
    return { remove: () => { mocks.onNotification = null; } };
  },
  getLastNotificationResponseAsync: () => Promise.resolve(mocks.lastResponse),
}));
// expo-router's useRouter returns a stable router reference across renders; an
// unstable mock would change openIntent/acceptRaw identities every render and
// re-run the listener effects on each state update.
vi.mock('expo-router', () => {
  const router = { replace: mocks.replace };
  return { useRouter: () => router };
});
vi.mock('@/weknora/auth/session', () => ({ useProductAuth: () => mocks.auth }));

import { NotificationRouter, useNotificationIntent } from './NotificationRouter';

function Harness() {
  const state = useNotificationIntent();
  return React.createElement('Intent', { intent: state.intent, error: state.error, retry: state.retry, dismiss: state.dismiss });
}

function auth(credential: any = { kind: 'bearer', accessToken: 'access' }) {
  return {
    credential,
    authSession: { baseURL: 'https://api.example', transport: { send: vi.fn(async () => ({ status: 200, body: { data: { owner_id: 'u1', tenant_id: 't1' } } })) } },
    scope: { identity: () => ({ userId: 'u1', tenantId: 't1' }) },
  };
}

function response(url: string) {
  return { notification: { request: { content: { data: { url } } } } };
}

let renderer: ReactTestRenderer;
beforeEach(() => {
  mocks.initialURL = null;
  mocks.lastResponse = null;
  mocks.onURL = null;
  mocks.onNotification = null;
  mocks.replace.mockReset();
  mocks.auth = auth();
  (globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
});
afterEach(() => { if (renderer) act(() => renderer.unmount()); });

describe('NotificationRouter', () => {
  it('recovers an intent after login and navigates to the product workbench', async () => {
    mocks.auth = auth(null);
    mocks.initialURL = 'weknora://execution?tenant=t1&run=r1';
    await act(async () => { renderer = create(React.createElement(NotificationRouter, { children: React.createElement(Harness) })); });
    expect(renderer.root.findByType('Intent').props.intent.runID).toBe('r1');

    mocks.auth = auth();
    await act(async () => { renderer.update(React.createElement(NotificationRouter, { children: React.createElement(Harness) })); });
    expect(mocks.replace).toHaveBeenCalledWith('/(app)');
  });

  it('uses the terminated-app notification response and deduplicates it', async () => {
    mocks.lastResponse = response('weknora://execution?tenant=t1&run=r2');
    await act(async () => { renderer = create(React.createElement(NotificationRouter, { children: React.createElement(Harness) })); });
    await act(async () => { mocks.onNotification?.(mocks.lastResponse); });
    expect(mocks.replace).toHaveBeenCalledTimes(1);
    expect(mocks.replace).toHaveBeenCalledWith('/(app)');
    expect(mocks.auth.authSession.transport.send).toHaveBeenCalledTimes(1);
  });

  it('fails closed for an ownership response missing tenant or owner', async () => {
    mocks.auth = auth();
    mocks.auth.authSession.transport.send.mockResolvedValue({ status: 200, body: { data: { run_id: 'r3' } } });
    mocks.initialURL = 'weknora://execution?tenant=t1&run=r3';
    await act(async () => { renderer = create(React.createElement(NotificationRouter, { children: React.createElement(Harness) })); });
    expect(mocks.replace).not.toHaveBeenCalled();
    expect(renderer.root.findByType('Intent').props.error).toBe('NOTIFICATION_UNAVAILABLE');
  });

  it('rejects expired and repeated notification intents without commands', async () => {
    vi.useFakeTimers();
    try {
      mocks.auth = auth(null);
      mocks.initialURL = 'weknora://execution?tenant=t1&run=r4';
      await act(async () => { renderer = create(React.createElement(NotificationRouter, { children: React.createElement(Harness) })); });
      vi.advanceTimersByTime(10 * 60 * 1000 + 1);
      mocks.auth = auth();
      await act(async () => { renderer.update(React.createElement(NotificationRouter, { children: React.createElement(Harness) })); });
      expect(mocks.replace).not.toHaveBeenCalled();
      expect(renderer.root.findByType('Intent').props.error).toBe('NOTIFICATION_EXPIRED');

      await act(async () => { mocks.onURL?.({ url: 'weknora://execution?tenant=t1&run=r5' }); });
      await act(async () => { mocks.onURL?.({ url: 'weknora://execution?tenant=t1&run=r5' }); });
      expect(mocks.auth.authSession.transport.send).toHaveBeenCalledTimes(1);
      expect(mocks.auth.authSession.transport.send.mock.calls.every((call: any[]) => call[0].method === 'GET')).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });
});
