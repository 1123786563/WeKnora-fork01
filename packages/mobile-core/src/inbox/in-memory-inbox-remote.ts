import type { InboxBackendPage, InboxRemote } from './notification-inbox.ts';

export interface InboxScriptPage {
  items: Array<{
    notificationId: string;
    kind?: string;
    title?: string;
    body?: string;
    createdAt?: string;
    read?: boolean;
    deepLink?: string;
  }>;
  unreadCount?: number;
  nextCursor?: string;
}

export interface ScenarioInboxRemote {
  remote: InboxRemote;
  /** 观测调用序列（「markRead 不触发 inbox 重取」等行为断言用）。 */
  calls(): string[];
  setPages(pages: InboxScriptPage[]): void;
}

/** in-memory scenario Adapter：每次 inbox() 消费一页，用尽后重复最后一页。 */
export function createScenarioInboxRemote(pages: InboxScriptPage[]): ScenarioInboxRemote {
  let queue = [...pages];
  const log: string[] = [];
  return {
    remote: {
      async inbox(cursor) {
        log.push(`inbox${cursor === undefined ? '' : `:${cursor}`}`);
        if (queue.length === 0) throw new Error('INBOX_SCRIPT_EXHAUSTED');
        const page = queue.length > 1 ? queue.shift()! : queue[0]!;
        const backendPage: InboxBackendPage = {
          items: page.items.map((item) => ({
            notificationId: item.notificationId,
            kind: item.kind ?? 'attention',
            title: item.title ?? '',
            body: item.body ?? '',
            createdAt: item.createdAt ?? '2026-09-24T00:00:00Z',
            read: item.read ?? false,
            ...(item.deepLink === undefined ? {} : { deepLink: item.deepLink }),
          })),
          unreadCount: page.unreadCount ?? page.items.filter((item) => item.read !== true).length,
          ...(page.nextCursor === undefined || page.nextCursor === '' ? {} : { nextCursor: page.nextCursor }),
        };
        return backendPage;
      },
      async markRead(notificationId) {
        log.push(`markRead:${notificationId}`);
      },
    },
    calls() {
      return [...log];
    },
    setPages(next) {
      queue = [...next];
    },
  };
}
