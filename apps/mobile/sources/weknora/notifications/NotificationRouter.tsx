import * as React from 'react';
import { Linking, Pressable, Text, View } from 'react-native';
import * as Notifications from 'expo-notifications';
import { useRouter } from 'expo-router';
import { useProductAuth } from '@/weknora/auth/session';
import { notificationLinkKey, parseNotificationLink, type NotificationLink } from './deep-link';

const INTENT_TTL_MS = 10 * 60 * 1000;

export interface NotificationIntent extends NotificationLink {
  receivedAt: number;
  authorizedAt?: number;
}

type ResolveTenant = (tenantID: string, signal: AbortSignal) => Promise<boolean>;
type VerifyExecution = (link: NotificationLink, signal: AbortSignal) => Promise<boolean>;

interface NotificationRouterProps {
  children: React.ReactNode;
  /** Optional member/space resolver supplied by the product tenant picker. */
  resolveTenant?: ResolveTenant;
  /** Injectable ownership check used by unit tests and alternate API hosts. */
  verifyExecution?: VerifyExecution;
}

interface NotificationRouterContextValue {
  intent: NotificationIntent | null;
  error: string | null;
  retry(): void;
  dismiss(): void;
}

const NotificationRouterContext = React.createContext<NotificationRouterContextValue | null>(null);

function notificationURL(response: Notifications.NotificationResponse): string | null {
  const data = response.notification.request.content.data;
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null;
  const record = data as Record<string, unknown>;
  for (const key of ['url', 'deep_link', 'deepLink']) {
    if (typeof record[key] === 'string') return record[key];
  }
  return null;
}

function bodyData(body: unknown): Record<string, unknown> | null {
  if (!body || typeof body !== 'object' || Array.isArray(body)) return null;
  const root = body as Record<string, unknown>;
  const data = root.data;
  return data && typeof data === 'object' && !Array.isArray(data) ? data as Record<string, unknown> : root;
}

function stringField(record: Record<string, unknown>, ...keys: string[]): string | undefined {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === 'string' && value.trim() !== '') return value;
  }
  return undefined;
}

export function NotificationRouter({ children, resolveTenant, verifyExecution }: NotificationRouterProps) {
  const auth = useProductAuth();
  const router = useRouter();
  const [intent, setIntent] = React.useState<NotificationIntent | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const seen = React.useRef(new Map<string, number>());
  const processing = React.useRef(new Set<string>());
  const abortRef = React.useRef<AbortController | null>(null);
  const current = auth.scope.identity;

  const defaultVerify = React.useCallback(async (link: NotificationLink, signal: AbortSignal): Promise<boolean> => {
    const identity = current();
    // A deep link may suggest a tenant, but it can never select one. A
    // caller that owns a member picker supplies resolveTenant explicitly.
    if (!identity.userId || !auth.authSession) return false;
    if (identity.tenantId !== link.tenantID) {
      // A trusted member picker may switch the product scope before the
      // ownership query. Without that explicit seam, a URI cannot switch
      // tenants by itself.
      if (!resolveTenant || !(await resolveTenant(link.tenantID, signal))) return false;
    }
    const result = await auth.authSession.transport.send({
      method: 'GET',
      url: `${auth.authSession.baseURL}/api/v1/workbench/executions/${encodeURIComponent(link.runID)}`,
      headers: { accept: 'application/json' },
      signal,
    });
    if (result.status < 200 || result.status >= 300) return false;
    const data = bodyData(result.body);
    if (!data) return false;
    // The endpoint's HTTP status alone is not a portable scope proof. Require
    // the ownership fields from the W03 response envelope and fail closed if
    // an older/proxy response omits either field.
    const owner = stringField(data, 'owner_id', 'ownerId', 'user_id', 'userId');
    const tenant = stringField(data, 'tenant_id', 'tenantId', 'space_tenant_id');
    if (!owner || !tenant) return false;
    return owner === identity.userId && tenant === link.tenantID;
  }, [auth.authSession, current, resolveTenant]);

  const openIntent = React.useCallback(async (next: NotificationIntent) => {
    const key = notificationLinkKey(next);
    if (Date.now() - next.receivedAt > INTENT_TTL_MS) {
      setIntent(null);
      setError('NOTIFICATION_EXPIRED');
      return;
    }
    if (!auth.credential || processing.current.has(key)) return;
    if (next.authorizedAt) {
      router.replace('/(app)' as never);
      return;
    }
    const controller = new AbortController();
    abortRef.current?.abort();
    abortRef.current = controller;
    processing.current.add(key);
    setError(null);
    try {
      const allowed = await (verifyExecution ?? defaultVerify)(next, controller.signal);
      if (!allowed) {
        setError('NOTIFICATION_UNAVAILABLE');
        return;
      }
      // The API response is the ownership check. The URI itself never carries
      // an action, and opening it does not issue approve/cancel traffic.
      seen.current.set(key, Date.now());
      // The verified run is presented by the product workbench. Keeping the
      // intent in context lets WorkbenchScreen render the pending card and
      // avoids falling through to Happy's legacy SessionView route.
      const authorized = { ...next, authorizedAt: Date.now() };
      setIntent(authorized);
      router.replace('/(app)' as never);
    } catch (cause) {
      if (!(cause instanceof Error && cause.name === 'AbortError')) setError('NOTIFICATION_UNAVAILABLE');
    } finally {
      processing.current.delete(key);
    }
  }, [auth.credential, defaultVerify, router, verifyExecution]);

  const acceptRaw = React.useCallback((raw: string | null) => {
    if (!raw) return;
    let parsed: NotificationLink;
    try {
      parsed = parseNotificationLink(raw);
    } catch {
      return;
    }
    const key = notificationLinkKey(parsed);
    const previous = seen.current.get(key);
    if (previous && Date.now() - previous < INTENT_TTL_MS) return;
    seen.current.set(key, Date.now());
    const next = { ...parsed, receivedAt: Date.now() };
    setError(null);
    setIntent(next);
    void openIntent(next);
  }, [openIntent]);

  React.useEffect(() => {
    let active = true;
    void Linking.getInitialURL().then((raw) => { if (active) acceptRaw(raw); }).catch(() => undefined);
    const urlSubscription = Linking.addEventListener('url', ({ url }) => acceptRaw(url));
    const notificationSubscription = Notifications.addNotificationResponseReceivedListener((response) => acceptRaw(notificationURL(response)));
    // A notification tap can launch a terminated app before the response
    // listener is installed. Expo exposes that response separately; feed it
    // through the same parser/deduplication path as foreground taps.
    void Notifications.getLastNotificationResponseAsync()
      .then((response) => { if (active && response) acceptRaw(notificationURL(response)); })
      .catch(() => undefined);
    return () => {
      active = false;
      abortRef.current?.abort();
      urlSubscription.remove();
      notificationSubscription.remove();
    };
  }, [acceptRaw]);

  React.useEffect(() => {
    if (intent && auth.credential) void openIntent(intent);
  }, [auth.credential, intent, openIntent]);

  const value = React.useMemo<NotificationRouterContextValue>(() => ({
    intent,
    error,
    retry: () => { if (intent) void openIntent(intent); },
    dismiss: () => { abortRef.current?.abort(); setIntent(null); setError(null); },
  }), [error, intent, openIntent]);

  return <NotificationRouterContext.Provider value={value}>{children}</NotificationRouterContext.Provider>;
}

export function useNotificationIntent(): NotificationRouterContextValue {
  const value = React.useContext(NotificationRouterContext);
  if (!value) throw new Error('NOTIFICATION_ROUTER_REQUIRED');
  return value;
}

/** A non-authorizing card. Decisions still belong to the W05 interaction API. */
export function PendingNotificationCard() {
  const { intent, error, retry, dismiss } = useNotificationIntent();
  if (!intent) return null;
  return (
    <View accessibilityLabel="pending-notification-card" style={{ margin: 12, padding: 12, borderRadius: 12, backgroundColor: '#25252b' }}>
      <Text style={{ color: '#fff', fontWeight: '600' }}>有一项任务需要查看</Text>
      <Text style={{ color: '#d5d5dc', marginTop: 4 }}>任务 {intent.runID}</Text>
      {error ? <Text accessibilityRole="alert" style={{ color: '#ff9b9b', marginTop: 4 }}>{error === 'NOTIFICATION_EXPIRED' ? '通知已过期' : '当前空间无法访问此任务'}</Text> : null}
      <View style={{ flexDirection: 'row', gap: 12, marginTop: 8 }}>
        <Pressable accessibilityRole="button" accessibilityLabel="打开待处理任务" onPress={retry}><Text style={{ color: '#9dd7ff' }}>重新打开</Text></Pressable>
        <Pressable accessibilityRole="button" accessibilityLabel="关闭待处理通知" onPress={dismiss}><Text style={{ color: '#d5d5dc' }}>关闭</Text></Pressable>
      </View>
    </View>
  );
}
