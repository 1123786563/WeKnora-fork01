import { Ionicons } from '@expo/vector-icons';
import { useRouter } from 'expo-router';
import * as React from 'react';
import { ActivityIndicator, FlatList, Pressable, Text, View } from 'react-native';
import {
  createCommercialApi,
  createExecutionsApi,
  createJsonTransport,
  createWeKnoraClient,
  type BearerCredential,
  type WorkbenchExecutionItem,
} from '@weknora/api-client';
import type { MobileHost } from '@/weknora/platform/host';
import type { ProductIdentity } from '@/weknora/platform/product-session';
import { createProductSessionNavigation, type ProductSessionNavigation } from '@/hooks/useNavigateToSession';
import { storage } from '@/sync/storage';
import { useMobileHost } from '@/weknora/platform/host';
import { useProductAuth } from '@/weknora/auth/session';

/** The four product workbench sections, in display order. */
export const WORKBENCH_GROUPS = [
  { key: 'running', label: '运行中', statuses: ['running', 'reconciling'] },
  { key: 'pending', label: '待处理', statuses: ['queued', 'waiting_user'] },
  { key: 'failed', label: '失败', statuses: ['failed'] },
  { key: 'completed', label: '已完成', statuses: ['succeeded', 'canceled'] },
] as const;

export type WorkbenchGroupKey = (typeof WORKBENCH_GROUPS)[number]['key'];

/** Group every execution into exactly one product section. */
export function groupWorkbenchExecutions(items: WorkbenchExecutionItem[]): Record<WorkbenchGroupKey, WorkbenchExecutionItem[]> {
  const grouped: Record<WorkbenchGroupKey, WorkbenchExecutionItem[]> = { running: [], pending: [], failed: [], completed: [] };
  const sectionOf = (status: string): WorkbenchGroupKey | undefined => {
    for (const group of WORKBENCH_GROUPS) {
      if ((group.statuses as readonly string[]).includes(status)) return group.key;
    }
    return undefined;
  };
  for (const item of items) {
    const section = sectionOf(item.status);
    if (section) grouped[section].push(item);
  }
  return grouped;
}

/**
 * Resolve the full product navigation destination for a workbench row.
 * A locally stored session created through the W10 seam wins: its
 * productSession metadata is server-issued and complete. Otherwise the row's
 * immutable run snapshot supplies the destination. When the snapshot is
 * missing a mandatory field (for example the space), the destination is not
 * fabricated: the caller must surface a fail-closed notice instead.
 */
export function resolveWorkbenchNavigation(
  item: Pick<WorkbenchExecutionItem, 'run_id' | 'session_id' | 'agent_id' | 'target_id' | 'workspace_ref' | 'space_id'>,
  identity: Pick<ProductIdentity, 'userId' | 'tenantId'>,
  storedSession?: unknown,
): ProductSessionNavigation | undefined {
  const candidate = (storedSession as { metadata?: { productSession?: Record<string, unknown> } } | null | undefined)?.metadata?.productSession;
  if (candidate && Object.values(candidate).every((value) => typeof value === 'string' && value.trim() !== '')) {
    // Stored metadata already matches this device's product identity because
    // ProductAuth issued it; the identity pair below is still re-checked by
    // the session route before any resource query runs.
    const metadata = candidate as unknown as ProductSessionNavigation;
    if (metadata.resourceSessionId === item.session_id || metadata.runId === item.run_id) return metadata;
  }
  if (!identity.userId || !identity.tenantId) return undefined;
  try {
    return createProductSessionNavigation({
      sessionId: item.session_id,
      spaceId: item.space_id ?? '',
      agentId: item.agent_id ?? '',
      targetId: item.target_id ?? '',
      workspaceRef: item.workspace_ref ?? '',
      userId: identity.userId,
      tenantId: identity.tenantId,
      runId: item.run_id,
    });
  } catch {
    return undefined;
  }
}

function workbenchClient(host: MobileHost, credential: BearerCredential) {
  const transport = createJsonTransport(fetch);
  const client = createWeKnoraClient({
    baseURL: host.origin,
    transport: {
      send: (request) => transport.send({
        ...request,
        headers: { ...request.headers, authorization: `Bearer ${credential.accessToken}` },
      }),
    },
  });
  return { executions: createExecutionsApi((input) => client.request(input)), commercial: createCommercialApi((input) => client.request(input)) };
}

const WORKBENCH_MAX_PAGES = 50;

export function WorkbenchScreen() {
  const host = useMobileHost();
  const auth = useProductAuth();
  const router = useRouter();
  const identity = auth.scope.identity();
  const [items, setItems] = React.useState<WorkbenchExecutionItem[]>([]);
  const [state, setState] = React.useState<'loading' | 'error' | 'ready'>('loading');
  const [message, setMessage] = React.useState<string | null>(null);
  const [notice, setNotice] = React.useState<string | null>(null);
  const [billingAvailable, setBillingAvailable] = React.useState(false);

  const reload = React.useCallback((signal: AbortSignal) => {
    if (!host || !auth.credential || auth.credential.kind !== 'bearer') return;
    const credential = auth.credential;
    const { executions } = workbenchClient(host, credential);
    (async () => {
      const collected: WorkbenchExecutionItem[] = [];
      let cursor: string | undefined;
      for (let page = 0; page < WORKBENCH_MAX_PAGES; page += 1) {
        const result = await executions.list({ ...(cursor === undefined ? {} : { cursor }) }, signal);
        collected.push(...result.items);
        if (!result.next_cursor) break;
        cursor = result.next_cursor;
      }
      setItems(collected);
      setState('ready');
      setMessage(null);
    })().catch((cause) => {
      if (signal.aborted) return;
      setState('error');
      setMessage(cause instanceof Error ? cause.message : 'WORKBENCH_LIST_UNAVAILABLE');
    });
  }, [auth.credential, host]);

  // A workspace switch (scope transition) invalidates every in-flight list.
  React.useEffect(() => {
    const controller = new AbortController();
    setState('loading');
    reload(controller.signal);
    return () => controller.abort();
  }, [reload, identity.tenantId, identity.userId]);

  // The billing entry is gated by its own commercial permission: the probe is
  // the member's own consumption summary, never the tenant order book.
  React.useEffect(() => {
    if (!host || !auth.credential || auth.credential.kind !== 'bearer') return;
    const controller = new AbortController();
    const { commercial } = workbenchClient(host, auth.credential);
    commercial.summary(controller.signal).then(() => {
      if (!controller.signal.aborted) setBillingAvailable(true);
    }).catch(() => {
      if (!controller.signal.aborted) setBillingAvailable(false);
    });
    return () => controller.abort();
  }, [auth.credential, host]);

  const groups = React.useMemo(() => groupWorkbenchExecutions(items), [items]);

  const navigateProduct = (item: WorkbenchExecutionItem) => {
    setNotice(null);
    const destination = resolveWorkbenchNavigation(item, identity, storage.getState().sessions[item.session_id]);
    if (!destination) {
      setNotice('该运行缺少完整产品元数据（如空间），无法进入产品会话。');
      return;
    }
    const query = new URLSearchParams(Object.entries(destination)).toString();
    router.push(`/session/${encodeURIComponent(item.session_id)}?${query}` as never);
  };

  return (
    <View style={styles.container} accessibilityLabel="workbench-screen">
      <View style={styles.header} accessibilityLabel="workbench-space">
        <Text style={styles.title}>工作台</Text>
        <Text style={styles.muted}>空间 {identity.tenantId ?? '未选择'} · 用户 {identity.userId ?? '未登录'}</Text>
        <View style={styles.headerActions}>
          <Pressable accessibilityRole="button" accessibilityLabel="最近会话" style={styles.chip} onPress={() => router.push('/session/recent')}>
            <Text>最近会话</Text>
          </Pressable>
          <Pressable accessibilityRole="button" accessibilityLabel="切换空间" style={styles.chip} onPress={() => router.push('/settings/account')}>
            <Text>切换空间</Text>
          </Pressable>
          {billingAvailable ? (
            <Pressable accessibilityRole="button" accessibilityLabel="任务消耗" style={styles.chip} onPress={() => router.push('/settings/usage')}>
              <Text>任务消耗</Text>
            </Pressable>
          ) : null}
        </View>
      </View>

      {notice ? <Text accessibilityRole="alert" style={styles.error}>{notice}</Text> : null}
      {state === 'loading' ? <ActivityIndicator accessibilityLabel="workbench-loading" /> : null}
      {state === 'error' ? (
        <Pressable accessibilityRole="button" onPress={() => {
          const controller = new AbortController();
          setState('loading');
          reload(controller.signal);
        }}>
          <Text style={styles.error}>{message ?? '工作台列表加载失败'} 点击重试。</Text>
        </Pressable>
      ) : null}
      {state === 'ready' && items.length === 0 ? <Text style={styles.muted}>暂无执行记录。</Text> : null}

      {WORKBENCH_GROUPS.map((group) => (
        <View key={group.key} accessibilityLabel={`workbench-group-${group.key}`}>
          <Text style={styles.groupTitle}>{group.label}（{groups[group.key].length}）</Text>
          {state === 'ready' && groups[group.key].length === 0 ? <Text style={styles.muted}>暂无</Text> : null}
          {state !== 'ready' ? null : (
            <FlatList
              data={groups[group.key]}
              keyExtractor={(row) => row.run_id}
              renderItem={({ item }) => (
                <Pressable
                  accessibilityRole="button"
                  accessibilityLabel={`workbench-execution-${item.run_id}`}
                  style={styles.row}
                  onPress={() => navigateProduct(item)}
                >
                  <Ionicons name="flash-outline" size={20} color="#53708a" />
                  <View style={{ flex: 1 }}>
                    <Text style={styles.title} numberOfLines={1}>{item.agent_id || item.session_id}</Text>
                    <Text style={styles.muted} numberOfLines={1}>{item.status} · {item.created_at}</Text>
                  </View>
                </Pressable>
              )}
            />
          )}
        </View>
      ))}
    </View>
  );
}

export default WorkbenchScreen;

const styles = {
  // Extra bottom padding keeps rows clear of the floating product tab bar.
  container: { flex: 1, padding: 16, gap: 10, paddingBottom: 88 } as const,
  header: { gap: 4 } as const,
  headerActions: { flexDirection: 'row', gap: 8 } as const,
  chip: { borderWidth: 1, borderColor: '#9db2c0', borderRadius: 8, paddingHorizontal: 10, paddingVertical: 4 } as const,
  groupTitle: { fontSize: 15, fontWeight: '600', color: '#14212b', marginTop: 6 } as const,
  row: { flexDirection: 'row', alignItems: 'center', gap: 10, paddingVertical: 10, borderBottomWidth: 1, borderBottomColor: '#d7e0e7' } as const,
  title: { fontSize: 16, color: '#14212b' } as const,
  muted: { color: '#637783', fontSize: 12 } as const,
  error: { color: '#b42318' } as const,
};
