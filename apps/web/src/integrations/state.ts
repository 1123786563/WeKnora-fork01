export type IntegrationLoadState =
  | { status: 'loading' }
  | { status: 'ready' }
  | { status: 'error'; message: string };

export interface IntegrationChannel {
  id: string;
  name: string;
  agentId?: string;
  enabled: boolean;
  kind: 'im' | 'embed';
}

export interface IntegrationSnapshot {
  channels: IntegrationChannel[];
  apiBaseUrl?: string;
  apiKeys?: Array<{ id: string; name: string; masked: string }>;
}

export const drawerTabs = ['configuration', 'playground', 'embed'] as const;
export type DrawerTab = (typeof drawerTabs)[number];

export function nextTab<T extends string>(tabs: readonly T[], current: T, key: string): T | null {
  if (!['ArrowRight', 'ArrowLeft', 'Home', 'End'].includes(key)) return null;
  const index = tabs.indexOf(current);
  if (index < 0) return tabs[0] ?? null;
  if (key === 'Home') return tabs[0] ?? null;
  if (key === 'End') return tabs[tabs.length - 1] ?? null;
  return tabs[(index + (key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length] ?? null;
}

export function visibleChannels(snapshot: IntegrationSnapshot, tab: 'im' | 'embed', agentId: string | null): IntegrationChannel[] {
  return snapshot.channels.filter((channel) => channel.kind === tab && (!agentId || channel.agentId === agentId));
}
