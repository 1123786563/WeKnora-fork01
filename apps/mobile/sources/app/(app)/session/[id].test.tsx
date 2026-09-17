// Mounted `/session/[id]` route integration tests (W25 I-1 fix round).
//
// The independent review found the upload chain had no production assembly
// point: the route rendered ConversationScreen without `attachments`, so the
// picker/camera entries could never appear in a real build. These cases mount
// the REAL route module with only platform boundaries stubbed (react-native,
// navigation, AsyncStorage, expo pickers, network fetch) and drive the
// production chain end to end: route -> product resource verification (real
// resolveProductSessionResources over the real ProductAuthSession transport)
// -> ConversationScreen with the assembled attachments pipeline -> real
// document picker / camera adapters -> upload manager -> upload port ->
// multipart POST carrying the auth session's bearer -> uploaded chip.

import * as React from 'react';
// @ts-expect-error react-test-renderer has no declarations in this workspace.
import { act, create } from 'react-test-renderer';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => {
  const ORIGIN = 'https://api.example';
  const resourceRows: Record<string, Record<string, unknown>> = {
    space: { id: 'space-1', tenant_id: 'tenant-1', owner_id: 'user-1', members: ['user-1'] },
    agent: { id: 'agent-1', tenant_id: 'tenant-1', owner_id: 'user-1', space_id: 'space-1', name: 'Agent One' },
    target: { id: 'target-1', tenant_id: 'tenant-1', owner_id: 'user-1', space_id: 'space-1', workspace_ref: 'workspace-1', state: 'active' },
    workspace: { id: 'workspace-1', tenant_id: 'tenant-1', owner_id: 'user-1', space_id: 'space-1', target_id: 'target-1' },
  };
  return {
    ORIGIN,
    resourceRows,
    resourceGets: [] as Array<{ url: string; headers: Record<string, string> }>,
    uploadPosts: [] as Array<{ url: string; headers: Record<string, string>; body: unknown }>,
    routeParams: {} as Record<string, string | undefined>,
    fetch: null as null | ((url: string, init?: { method?: string; headers?: Record<string, string>; body?: unknown }) => Promise<unknown>),
    getDocumentAsync: vi.fn(),
    requestCameraPermissionsAsync: vi.fn(),
    launchCameraAsync: vi.fn(),
    storage: new Map<string, string>(),
  };
});

vi.mock('react-native', async () => {
  const ReactModule = await import('react');
  const host = (name: string) => (props: { children?: unknown }) => ReactModule.createElement(name, props, props.children as never);
  return { View: host('View'), Text: host('Text'), Pressable: host('Pressable'), TextInput: host('TextInput'), ScrollView: host('ScrollView') };
});

vi.mock('@/-session/SessionView', async () => {
  const ReactModule = await import('react');
  return { SessionView: (props: Record<string, unknown>) => ReactModule.createElement('SessionView', props) };
});

vi.mock('@react-navigation/native', () => ({
  useRoute: () => ({ params: mocks.routeParams }),
}));

vi.mock('@react-native-async-storage/async-storage', () => ({
  default: {
    getItemAsync: async (key: string) => mocks.storage.get(key) ?? null,
    setItemAsync: async (key: string, value: string) => { mocks.storage.set(key, value); },
    deleteItemAsync: async (key: string) => { mocks.storage.delete(key); },
  },
}));

// Real product modules re-exported under their `@/` specifiers so the mounted
// route executes production code paths, not test doubles.
vi.mock('@/weknora/platform/host', async () => await import('../../../weknora/platform/host'));
vi.mock('@/weknora/conversations/ConversationScreen', async () => await import('../../../weknora/conversations/ConversationScreen'));
vi.mock('@/weknora/conversations/view-model', async () => await import('../../../weknora/conversations/view-model'));
vi.mock('@/weknora/conversations/resources', async () => await import('../../../weknora/conversations/resources'));
vi.mock('@/weknora/conversations/execution-projection', async () => await import('../../../weknora/conversations/execution-projection'));
vi.mock('@/weknora/resources/product-session-attachments', async () => await import('../../../weknora/resources/product-session-attachments'));

vi.mock('@/weknora/platform/execution-storage', async () => {
  const actual = await import('../../../weknora/platform/execution-storage');
  return {
    ...actual,
    // The native encrypted store is injected on device; tests supply an
    // in-memory twin so the route passes its event-storage gate.
    getExecutionStorage: () => ({ read: async () => [], commit: async () => undefined }),
  };
});

vi.mock('expo-document-picker', () => ({ getDocumentAsync: mocks.getDocumentAsync }));
vi.mock('expo-image-picker', () => ({
  requestCameraPermissionsAsync: mocks.requestCameraPermissionsAsync,
  launchCameraAsync: mocks.launchCameraAsync,
}));

vi.mock('@/weknora/auth/session', async () => {
  const { createProductAuthSession, createJsonTransport } = await import('@weknora/api-client');
  const { createProductScope } = await import('../../../weknora/platform/product-session');
  let value: {
    credential: { kind: 'bearer'; accessToken: string; refreshToken?: string };
    authSession: ReturnType<typeof createProductAuthSession>;
    scope: ReturnType<typeof createProductScope>;
  } | null = null;
  return {
    useProductAuth: () => {
      if (value) return value;
      // Real auth session over the test fetch handler: bearer injection and
      // the refresh coordinator run as production code.
      const transport = createJsonTransport(((url: string, init?: unknown) => (mocks.fetch as nonNullable<typeof mocks.fetch>)(url, init as never)) as never);
      const authSession = createProductAuthSession({
        baseURL: mocks.ORIGIN,
        transport,
        credentials: {
          read: async () => ({ kind: 'bearer', accessToken: 'token-1', refreshToken: 'refresh-1' }),
          write: async () => undefined,
          clear: async () => undefined,
        },
      });
      const scope = createProductScope({ origin: mocks.ORIGIN, userId: null, tenantId: null });
      scope.switchTo({ origin: mocks.ORIGIN, userId: 'user-1', tenantId: 'tenant-1' });
      value = {
        credential: { kind: 'bearer', accessToken: 'token-1', refreshToken: 'refresh-1' },
        authSession,
        scope,
      };
      return value;
    },
  };
});

type nonNullable<T> = T & {};

function jsonResponse(body: unknown, status = 200) {
  return {
    status,
    headers: { get: (name: string) => (name.toLowerCase() === 'content-type' ? 'application/json' : null) },
    json: async () => body,
    text: async () => JSON.stringify(body),
  };
}

beforeEach(() => {
  mocks.resourceGets.length = 0;
  mocks.uploadPosts.length = 0;
  mocks.routeParams = {
    id: 'session-1',
    spaceId: 'space-1',
    agentId: 'agent-1',
    targetId: 'target-1',
    workspaceRef: 'workspace-1',
    resourceUserId: 'user-1',
    resourceTenantId: 'tenant-1',
    runId: 'run-1',
  };
  mocks.getDocumentAsync.mockReset().mockResolvedValue({ canceled: true, assets: null });
  mocks.requestCameraPermissionsAsync.mockReset().mockResolvedValue({ granted: true, status: 'granted' });
  mocks.launchCameraAsync.mockReset().mockResolvedValue({ canceled: true, assets: null });
  mocks.fetch = async (url: string, init?: { method?: string; headers?: Record<string, string>; body?: unknown }) => {
    const method = init?.method ?? 'GET';
    if (url.startsWith('content://') || url.startsWith('file://')) {
      // The platform local file port reads the picked device-local URI.
      const type = url.endsWith('.jpg') || url.endsWith('.jpeg') ? 'image/jpeg' : url.endsWith('.png') ? 'image/png' : 'application/pdf';
      const bytes = url.includes('photo') ? 'jpeg-bytes' : 'pdf-bytes';
      return {
        status: 200,
        headers: { get: () => null },
        blob: async () => new Blob([bytes], { type }),
        text: async () => bytes,
      };
    }
    const record = { url, headers: init?.headers ?? {} };
    if (url.includes('/api/v1/organizations/')) { mocks.resourceGets.push(record); return jsonResponse({ success: true, data: mocks.resourceRows.space }); }
    if (url.includes('/api/v1/agents/')) { mocks.resourceGets.push(record); return jsonResponse({ success: true, data: mocks.resourceRows.agent }); }
    if (url.includes('/api/v1/execution-targets/')) { mocks.resourceGets.push(record); return jsonResponse({ success: true, data: mocks.resourceRows.target }); }
    if (url.includes('/api/v1/execution-workspaces/')) { mocks.resourceGets.push(record); return jsonResponse({ success: true, data: mocks.resourceRows.workspace }); }
    if (method === 'POST' && url.endsWith('/attachments')) {
      mocks.uploadPosts.push({ url, headers: init?.headers ?? {}, body: init?.body });
      return jsonResponse({ success: true, data: { id: `attachment-${mocks.uploadPosts.length}` } });
    }
    throw new Error(`unexpected fetch ${method} ${url}`);
  };
  vi.stubGlobal('fetch', mocks.fetch);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

async function mountRoute() {
  const hostModule = await import('../../../weknora/platform/host');
  // Imported after the mocks above are registered so the route module sees them.
  const routeModule = await import('./[id]');
  let renderer: { root: any; unmount: () => unknown };
  await act(async () => {
    renderer = create(React.createElement(
      hostModule.MobileHostProvider,
      { initialHost: { backend: 'weknora', origin: mocks.ORIGIN } },
      React.createElement(routeModule.default),
    )) as never;
  });
  return renderer!;
}

/** Pumps the act queue until the predicate holds (resource effects settle). */
async function waitForRoute(renderer: { root: any }, ready: () => boolean) {
  for (let attempt = 0; attempt < 100 && !ready(); attempt++) {
    await act(async () => { await new Promise<void>((resolve) => { setTimeout(resolve, 0); }); });
  }
  expect(ready()).toBe(true);
}

describe('mounted product session route (W25 attachments assembly)', () => {
  it('assembles the upload chain: entries render and a picked document posts as multipart with the auth bearer', async () => {
    mocks.getDocumentAsync.mockResolvedValue({
      canceled: false,
      assets: [{ uri: 'content://picker/a.pdf', name: 'a.pdf', size: 11, mimeType: 'application/pdf', lastModified: 1 }],
    });
    const renderer = await mountRoute();

    // I-1 closure assertion: the production route passes a non-empty
    // attachments pipeline, so the real ConversationScreen renders the
    // picker entry (it stays hidden when `attachments` is undefined).
    // (The RN mock wraps each host node, so a label matches >= 1 nodes.)
    await waitForRoute(renderer, () => renderer.root.findAllByProps({ accessibilityLabel: '附件' }).length >= 1);
    expect(renderer.root.findAllByProps({ accessibilityLabel: '拍照' }).length).toBeGreaterThan(0);

    // Resource verification ran through the real auth session transport.
    expect(mocks.resourceGets).toHaveLength(4);
    expect(mocks.resourceGets.every((get) => get.headers.authorization === 'Bearer token-1')).toBe(true);

    // Drive the real picker entry end to end.
    await act(async () => { await renderer.root.findByProps({ accessibilityLabel: '附件' }).props.onPress(); });
    await waitForRoute(renderer, () => renderer.root.findAll((node: any) => node.props?.children === '已附加').length >= 1);

    expect(mocks.uploadPosts).toHaveLength(1);
    const post = mocks.uploadPosts[0];
    expect(post.url).toBe(`${mocks.ORIGIN}/api/v1/sessions/session-1/attachments`);
    expect(post.headers.authorization).toBe('Bearer token-1');
    expect(post.body).toBeInstanceOf(FormData);
    const file = (post.body as FormData).get('file') as { name?: string };
    expect(file?.name).toBe('a.pdf');
    // The device-local path never travels to the backend.
    expect(post.url).not.toContain('content://');
    expect([...(post.body as FormData).keys()]).toEqual(['file']);

    await act(async () => { renderer.unmount(); });
  });

  it('drives the camera entry through the native adapter into the same upload chain', async () => {
    mocks.launchCameraAsync.mockResolvedValue({
      canceled: false,
      assets: [{ uri: 'file:///cache/photo-1.jpg', width: 2, height: 2, fileName: 'photo-1.jpg', fileSize: 5, mimeType: 'image/jpeg' }],
    });
    const renderer = await mountRoute();
    await waitForRoute(renderer, () => renderer.root.findAllByProps({ accessibilityLabel: '拍照' }).length >= 1);

    await act(async () => { await renderer.root.findByProps({ accessibilityLabel: '拍照' }).props.onPress(); });
    await waitForRoute(renderer, () => renderer.root.findAll((node: any) => node.props?.children === '已附加').length >= 1);

    expect(mocks.requestCameraPermissionsAsync).toHaveBeenCalledTimes(1);
    expect(mocks.launchCameraAsync).toHaveBeenCalledTimes(1);
    expect(mocks.uploadPosts).toHaveLength(1);
    expect(mocks.uploadPosts[0].url).toBe(`${mocks.ORIGIN}/api/v1/sessions/session-1/attachments`);
    const file = (mocks.uploadPosts[0].body as FormData).get('file') as { name?: string };
    expect(file?.name).toBe('photo-1.jpg');

    await act(async () => { renderer.unmount(); });
  });

  it('keeps the retained Happy route attachment-free when product metadata is absent', async () => {
    mocks.routeParams = { id: 'session-1' };
    const renderer = await mountRoute();
    await act(async () => { await new Promise<void>((resolve) => { setTimeout(resolve, 0); }); });

    expect(renderer.root.findAllByType('SessionView')).toHaveLength(1);
    expect(renderer.root.findAllByProps({ accessibilityLabel: '附件' })).toHaveLength(0);
    expect(mocks.uploadPosts).toHaveLength(0);

    await act(async () => { renderer.unmount(); });
  });
});
