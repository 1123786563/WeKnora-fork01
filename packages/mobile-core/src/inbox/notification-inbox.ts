import { leaseActive } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { parseNotificationDeepLink } from './deep-link.ts';
import type { DeepLinkTarget } from './deep-link.ts';

/**
 * 行动通知 Inbox 深模块（#41，CONTEXT.md「行动通知」：通知只提示客户端重新同步，
 * 不承载权威任务状态）：
 * - page()/more()：模块拥有 cursor 与查询身份，屏不维护分页状态（module-seams §10）；
 * - 重复通知：跨页重复的 notificationId 不再渲染，经 duplicateNotificationIds 观测
 *   （分页以 created_at 时间戳为 cursor，翻页边界同刻多行可能重复——TaskOffice duplicateRunIds 同构）；
 * - markRead：只置已读（本地投影 + 服务端幂等 Update("read", true)），绝不执行通知
 *   描述的任何业务操作（workbench_inbox.go:123-130 注释为证）；
 * - applyHint：推送 hint 只是触发器——等价一次权威重投影，hint.kind 不进入任何状态；
 * - resolveTarget：通知行 deep_link 一律经安全解析（deep-link.ts 白名单），错误链接返回 undefined；
 * - 迟到拒绝：scope lease 撤销或更新的查询使旧结果按 SUPERSEDED/SCOPE_CHANGED 拒绝。
 */

export interface InboxBackendItem {
  notificationId: string;
  kind: string;
  title: string;
  body: string;
  createdAt: string;
  read: boolean;
  deepLink?: string;
}

export interface InboxBackendPage {
  items: InboxBackendItem[];
  unreadCount: number;
  nextCursor?: string;
}

export interface InboxItem {
  notificationId: string;
  kind: string;
  title: string;
  body: string;
  createdAt: string;
  read: boolean;
  deepLink?: string;
}

export interface InboxView {
  items: InboxItem[];
  unreadCount: number;
  nextCursor?: string;
  duplicateNotificationIds: string[];
}

export interface InboxRemote {
  inbox(cursor?: string): Promise<InboxBackendPage>;
  markRead(notificationId: string): Promise<void>;
}

export interface NotificationInboxPorts {
  remote: InboxRemote;
  lease(): ScopeLease | undefined;
}

export type InboxErrorCode =
  | 'INBOX_SCOPE_CHANGED'
  | 'INBOX_SUPERSEDED'
  | 'INBOX_NO_ACTIVE_QUERY'
  | 'INBOX_INVALID_INPUT'
  | 'INBOX_BACKEND';

export class InboxError extends Error {
  constructor(readonly code: InboxErrorCode, options?: { cause?: unknown }) {
    super(code, options);
    this.name = 'InboxError';
  }
}

export interface NotificationInbox {
  page(): Promise<InboxView>;
  more(): Promise<InboxView>;
  markRead(notificationId: string): Promise<void>;
  /** 推送同步 hint：只触发一次权威重投影，不携带/写入任何业务状态（AC1 客户端半边）。 */
  applyHint(hint: { kind?: string }): Promise<InboxView>;
  resolveTarget(item: InboxItem): DeepLinkTarget | undefined;
  subscribe(listener: (view: InboxView) => void): () => void;
}

export function createNotificationInbox(ports: NotificationInboxPorts): NotificationInbox {
  const listeners = new Set<(view: InboxView) => void>();
  let sequence = 0;
  let view: InboxView | undefined;
  let seen = new Set<string>();
  let duplicates: string[] = [];
  let cursor: string | undefined;
  // 本地已读投影（B3-F1）：markRead 成功即记录；GET 快照只携带服务端视角，
  // merge 时已读行强制 read:true——在途旧快照迟到 settle 不再回退已读状态。
  const markedRead = new Set<string>();
  const requireLease = (): ScopeLease => {
    const lease = ports.lease();
    if (lease === undefined || !leaseActive(lease)) throw new InboxError('INBOX_SCOPE_CHANGED');
    return lease;
  };
  const callRemote = async <T>(action: () => Promise<T>): Promise<T> => {
    try {
      return await action();
    } catch (error) {
      if (error instanceof InboxError) throw error;
      throw new InboxError('INBOX_BACKEND', { cause: error });
    }
  };
  const publish = (next: InboxView): InboxView => {
    view = next;
    for (const listener of listeners) listener(next);
    return next;
  };
  const settle = (ticket: number, lease: ScopeLease): void => {
    if (ticket !== sequence) throw new InboxError('INBOX_SUPERSEDED');
    if (!leaseActive(lease)) throw new InboxError('INBOX_SCOPE_CHANGED');
  };
  const merge = (page: InboxBackendPage, reset: boolean): InboxView => {
    if (reset) {
      seen = new Set();
      duplicates = [];
    }
    const items: InboxItem[] = reset ? [] : [...(view?.items ?? [])];
    for (const item of page.items) {
      if (seen.has(item.notificationId)) {
        duplicates.push(item.notificationId);
        continue;
      }
      seen.add(item.notificationId);
      items.push({ ...item, ...(markedRead.has(item.notificationId) ? { read: true } : {}) }); // B3-F1：已读是本地投影事实
    }
    cursor = page.nextCursor === '' ? undefined : page.nextCursor; // B3-F2：空串游标归一
    return publish({
      items,
      unreadCount: page.unreadCount,
      ...(cursor === undefined ? {} : { nextCursor: cursor }),
      duplicateNotificationIds: [...duplicates],
    });
  };
  const fetchPage = async (reset: boolean): Promise<InboxView> => {
    const lease = requireLease();
    const ticket = ++sequence;
    const page = await callRemote(() => ports.remote.inbox(reset ? undefined : cursor));
    settle(ticket, lease);
    return merge(page, reset);
  };
  return {
    page: () => fetchPage(true),
    more: () => {
      if (view === undefined || cursor === undefined) return Promise.reject(new InboxError('INBOX_NO_ACTIVE_QUERY'));
      return fetchPage(false);
    },
    async markRead(notificationId: string): Promise<void> {
      const trimmed = typeof notificationId === 'string' ? notificationId.trim() : '';
      if (trimmed === '') throw new InboxError('INBOX_INVALID_INPUT');
      const lease = requireLease();
      await callRemote(() => ports.remote.markRead(trimmed));
      if (!leaseActive(lease)) throw new InboxError('INBOX_SCOPE_CHANGED');
      markedRead.add(trimmed); // 服务端已确认的已读事实（B3-F1）
      if (view !== undefined) {
        const wasUnread = view.items.some((item) => item.notificationId === trimmed && !item.read);
        const items = view.items.map((item) => (item.notificationId === trimmed && !item.read ? { ...item, read: true } : item));
        publish({
          items,
          unreadCount: wasUnread ? Math.max(0, view.unreadCount - 1) : view.unreadCount,
          ...(view.nextCursor === undefined ? {} : { nextCursor: view.nextCursor }),
          duplicateNotificationIds: view.duplicateNotificationIds,
        });
      }
    },
    applyHint(hint: { kind?: string }): Promise<InboxView> {
      void hint; // hint 只是触发器：kind 不进入任何状态或视图（AC1）
      return fetchPage(true);
    },
    resolveTarget(item: InboxItem): DeepLinkTarget | undefined {
      return parseNotificationDeepLink(item.deepLink ?? '');
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}
