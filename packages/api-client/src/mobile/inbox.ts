import type { ClientRequest } from '../client.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';

type Request = (input: ClientRequest) => Promise<unknown>;

export interface MobileInboxRemoteOptions {
  /** 部署 Origin：构造即强校验（共享 requireDeploymentOrigin，B3-F11 收敛）。 */
  origin: string;
  /** 授权通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
  request: Request;
}

/** 与 mobile-core InboxBackendItem 结构逐字一致（结构可赋值由 apps/mobile typecheck 证明）。 */
export interface RemoteInboxItem {
  notificationId: string;
  kind: string;
  title: string;
  body: string;
  createdAt: string;
  read: boolean;
  deepLink?: string;
}

/** 与 mobile-core InboxBackendPage 结构逐字一致。 */
export interface RemoteInboxPage {
  items: RemoteInboxItem[];
  unreadCount: number;
  nextCursor?: string;
}

export interface MobileInboxRemote {
  /** GET /api/v1/workbench/inbox?cursor=（internal/handler/session/workbench_inbox.go:193）。 */
  inbox(cursor?: string): Promise<RemoteInboxPage>;
  /** POST /api/v1/workbench/inbox/read（workbench_inbox.go:214）——只置已读，服务端不执行任何业务操作。 */
  markRead(notificationId: string): Promise<void>;
}

function requireString(value: unknown, label: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${label} is required`);
  return value;
}

export function createMobileInboxRemote(options: MobileInboxRemoteOptions): MobileInboxRemote {
  requireDeploymentOrigin(options.origin);
  const request = options.request;
  const mapItem = (row: Record<string, unknown>): RemoteInboxItem => ({
    notificationId: requireString(row.notification_id, 'inbox notification_id'),
    kind: requireString(row.kind, 'inbox kind'),
    title: typeof row.title === 'string' ? row.title : '',
    body: typeof row.body === 'string' ? row.body : '',
    createdAt: requireString(row.created_at, 'inbox created_at'),
    read: row.read === true,
    ...(typeof row.deep_link === 'string' && row.deep_link.trim() !== '' ? { deepLink: row.deep_link } : {}),
  });
  return {
    async inbox(cursor?: string): Promise<RemoteInboxPage> {
      const path = typeof cursor === 'string' && cursor.trim() !== ''
        ? `/api/v1/workbench/inbox?cursor=${encodeURIComponent(cursor.trim())}`
        : '/api/v1/workbench/inbox';
      const response = await request({ method: 'GET', path });
      if (typeof response !== 'object' || response === null || Array.isArray(response)) {
        throw new Error('inbox response must be a success envelope');
      }
      const envelope = response as { success?: unknown; data?: unknown };
      if (envelope.success !== true || !Object.prototype.hasOwnProperty.call(envelope, 'data')) {
        throw new Error('inbox response.success must be true with data');
      }
      const data = envelope.data as { items?: unknown; unread_count?: unknown; next_cursor?: unknown };
      if (!Array.isArray(data.items)) throw new Error('inbox data.items must be an array');
      if (typeof data.unread_count !== 'number' || !Number.isSafeInteger(data.unread_count) || data.unread_count < 0) {
        throw new Error('inbox data.unread_count must be a non-negative integer');
      }
      return {
        items: (data.items as Array<Record<string, unknown>>).map(mapItem),
        unreadCount: data.unread_count,
        ...(typeof data.next_cursor === 'string' && data.next_cursor !== '' ? { nextCursor: data.next_cursor } : {}),
      };
    },
    async markRead(notificationId: string): Promise<void> {
      const trimmed = typeof notificationId === 'string' ? notificationId.trim() : '';
      requireString(trimmed, 'notification id');
      const response = await request({ method: 'POST', path: '/api/v1/workbench/inbox/read', body: { notification_id: trimmed } });
      if (typeof response !== 'object' || response === null || Array.isArray(response)) {
        throw new Error('inbox markRead response must be a success envelope');
      }
      if ((response as { success?: unknown }).success !== true) {
        throw new Error('inbox markRead response.success must be true');
      }
    },
  };
}
