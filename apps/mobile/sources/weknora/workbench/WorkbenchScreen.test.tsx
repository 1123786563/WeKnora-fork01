import * as React from 'react';
// @ts-expect-error react-test-renderer has no declarations in this workspace.
import { act, create } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';
import type { WorkbenchExecutionItem } from '@weknora/api-client';
import { groupWorkbenchExecutions, resolveWorkbenchNavigation, WORKBENCH_GROUPS } from './WorkbenchScreen';

// The mount case only asserts the initial loading shell; keep the transport
// pending so no real network call ever leaves the test process.
vi.stubGlobal('fetch', vi.fn(() => new Promise(() => undefined)));

vi.mock('react-native', async () => {
  const ReactModule = await import('react');
  const host = (name: string) => (props: any) => ReactModule.createElement(name, props, props.children);
  return { ActivityIndicator: host('ActivityIndicator'), FlatList: host('FlatList'), Pressable: host('Pressable'), Text: host('Text'), View: host('View') };
});
vi.mock('@expo/vector-icons', async () => {
  const ReactModule = await import('react');
  return { Ionicons: (props: any) => ReactModule.createElement('Ionicons', props) };
});
vi.mock('expo-router', () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock('@/weknora/platform/host', () => ({ useMobileHost: () => ({ origin: 'https://api.example' }) }));
vi.mock('@/sync/storage', () => ({ storage: { getState: () => ({ sessions: {} }) } }));
vi.mock('@/weknora/auth/session', () => ({
  useProductAuth: () => ({
    loading: false,
    credential: { kind: 'bearer', accessToken: 'token' },
    scope: { identity: () => ({ origin: 'https://api.example', userId: 'u1', tenantId: 't1' }), subscribe: () => () => undefined },
  }),
}));
// The workbench renders the W16 pending-notification card above its sections.
// This suite asserts grouping/navigation only, so stub the card (which needs
// the full router provider) rather than mounting it.
vi.mock('@/weknora/notifications/NotificationRouter', () => ({
  PendingNotificationCard: () => null,
}));
// The workbench consumes the W10 navigation producer; run the real validator
// (a dependency-free module) so the asserted destinations match production.
vi.mock('@/hooks/useNavigateToSession', async () => {
  const navigation = await import('../../utils/productSessionNavigation');
  return { createProductSessionNavigation: navigation.createProductSessionNavigation };
});

function item(overrides: Partial<WorkbenchExecutionItem> = {}): WorkbenchExecutionItem {
  return {
    run_id: 'run-1', session_id: 's-1', status: 'running',
    agent_id: 'agent-1', target_id: 'platform', workspace_ref: 'ws', space_id: 'space-1',
    created_at: '2026-09-12T10:00:00Z', updated_at: '2026-09-12T10:00:01Z',
    ...overrides,
  };
}

describe('groupWorkbenchExecutions', () => {
  it('bins every known status into exactly one of the four product sections', () => {
    const grouped = groupWorkbenchExecutions([
      item({ run_id: 'a', status: 'running' }),
      item({ run_id: 'b', status: 'reconciling' }),
      item({ run_id: 'c', status: 'queued' }),
      item({ run_id: 'd', status: 'waiting_user' }),
      item({ run_id: 'e', status: 'failed' }),
      item({ run_id: 'f', status: 'succeeded' }),
      item({ run_id: 'g', status: 'canceled' }),
    ]);
    expect(grouped.running.map((row) => row.run_id)).toEqual(['a', 'b']);
    expect(grouped.pending.map((row) => row.run_id)).toEqual(['c', 'd']);
    expect(grouped.failed.map((row) => row.run_id)).toEqual(['e']);
    expect(grouped.completed.map((row) => row.run_id)).toEqual(['f', 'g']);
  });

  it('keeps section order stable and drops unknown statuses', () => {
    expect(WORKBENCH_GROUPS.map((group) => group.key)).toEqual(['running', 'pending', 'failed', 'completed']);
    const grouped = groupWorkbenchExecutions([item({ run_id: 'x', status: 'jogging' })]);
    expect([...grouped.running, ...grouped.pending, ...grouped.failed, ...grouped.completed]).toEqual([]);
  });
});

describe('resolveWorkbenchNavigation', () => {
  const identity = { userId: 'u1', tenantId: 't1' };

  it('builds the full product destination from the run snapshot', () => {
    const destination = resolveWorkbenchNavigation(item(), identity);
    expect(destination).toMatchObject({
      spaceId: 'space-1', agentId: 'agent-1', targetId: 'platform', workspaceRef: 'ws',
      resourceUserId: 'u1', resourceTenantId: 't1', resourceSessionId: 's-1', runId: 'run-1',
    });
  });

  it('prefers the stored server-issued productSession metadata', () => {
    const stored = { metadata: { productSession: {
      spaceId: 'space-9', agentId: 'agent-9', targetId: 'platform', workspaceRef: 'ws-9',
      resourceUserId: 'u1', resourceTenantId: 't1', resourceSessionId: 's-1', runId: 'run-1',
    } } };
    expect(resolveWorkbenchNavigation(item(), identity, stored)).toMatchObject({ spaceId: 'space-9', agentId: 'agent-9' });
    // Metadata belonging to another session/run is ignored; the row falls
    // back to its own snapshot instead of inheriting foreign destinations.
    expect(resolveWorkbenchNavigation(item({ session_id: 's-2', run_id: 'run-2' }), identity, stored)).toMatchObject({ spaceId: 'space-1', resourceSessionId: 's-2', runId: 'run-2' });
  });

  it('fails closed when the snapshot misses a mandatory field', () => {
    expect(resolveWorkbenchNavigation(item({ space_id: undefined }), identity)).toBeUndefined();
    expect(resolveWorkbenchNavigation(item({ agent_id: undefined }), identity)).toBeUndefined();
    expect(resolveWorkbenchNavigation(item(), { userId: null, tenantId: null })).toBeUndefined();
  });
});

describe('WorkbenchScreen', () => {
  it('mounts the four product sections with the space header', async () => {
    const { WorkbenchScreen } = await import('./WorkbenchScreen');
    let renderer: ReturnType<typeof create>;
    await act(async () => { renderer = create(React.createElement(WorkbenchScreen)); });
    expect(renderer!.root.findByProps({ accessibilityLabel: 'workbench-screen' })).toBeDefined();
    expect(renderer!.root.findByProps({ accessibilityLabel: 'workbench-space' })).toBeDefined();
    for (const group of WORKBENCH_GROUPS) {
      expect(renderer!.root.findByProps({ accessibilityLabel: `workbench-group-${group.key}` })).toBeDefined();
    }
    expect(renderer!.root.findAllByProps({ accessibilityLabel: 'workbench-loading' }).length).toBeGreaterThan(0);
    await act(async () => renderer!.unmount());
  });
});
