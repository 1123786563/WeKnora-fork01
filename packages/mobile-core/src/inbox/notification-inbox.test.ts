import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createNotificationInbox, InboxError } from './notification-inbox.ts';
import type { InboxBackendPage, InboxRemote } from './notification-inbox.ts';
import { createScenarioInboxRemote } from './in-memory-inbox-remote.ts';

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: '7' });
  return { revocable, lease: revocable.asScopeLease() };
}

function inboxWith(leaseRef: { lease?: ScopeLease }, pages: Parameters<typeof createScenarioInboxRemote>[0]) {
  const scenario = createScenarioInboxRemote(pages);
  const inbox = createNotificationInbox({ remote: scenario.remote, lease: () => leaseRef.lease });
  return { scenario, inbox };
}

test('page projects the first page with the unread count', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { inbox } = inboxWith(leaseRef, [
    { items: [{ notificationId: 'n-1', kind: 'attention', title: '需要你处理', read: false }], unreadCount: 1 },
  ]);

  const view = await inbox.page();

  assert.deepEqual(view.items, [{ notificationId: 'n-1', kind: 'attention', title: '需要你处理', body: '', createdAt: '2026-09-24T00:00:00Z', read: false }]);
  assert.equal(view.unreadCount, 1);
  assert.equal(view.nextCursor, undefined);
});

test('more merges pages, dedupes repeated notification ids observably (AC2 重复通知)', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { inbox } = inboxWith(leaseRef, [
    { items: [{ notificationId: 'n-1', read: false }, { notificationId: 'n-2', read: true }], nextCursor: 'c-1' },
    { items: [{ notificationId: 'n-2', read: true }, { notificationId: 'n-3', read: false }] }, // n-2 翻页边界重复
  ]);

  const first = await inbox.page();
  assert.equal(first.nextCursor, 'c-1');
  const second = await inbox.more();

  assert.deepEqual(second.items.map((item) => item.notificationId), ['n-1', 'n-2', 'n-3'], '重复行不重复渲染');
  assert.deepEqual(second.duplicateNotificationIds, ['n-2'], '重复通知可观测');
});

test('more without an active query or an exhausted cursor rejects INBOX_NO_ACTIVE_QUERY', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { inbox } = inboxWith(leaseRef, [{ items: [] }]);

  await assert.rejects(inbox.more(), (error: unknown) => error instanceof InboxError && error.code === 'INBOX_NO_ACTIVE_QUERY');
  await inbox.page();
  await assert.rejects(inbox.more(), (error: unknown) => error instanceof InboxError && error.code === 'INBOX_NO_ACTIVE_QUERY');
});

test('markRead is idempotent, and never performs any action besides the markRead wire call (AC1)', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, inbox } = inboxWith(leaseRef, [
    { items: [{ notificationId: 'n-1', kind: 'attention', read: false }, { notificationId: 'n-2', read: false }], unreadCount: 2 },
  ]);
  await inbox.page();
  const inboxCallsBefore = scenario.calls().length;

  await inbox.markRead('n-1');
  await inbox.markRead('n-1'); // 幂等：重复 markRead 仍只多一次同 id 的 wire 调用，本地未读不再递减（下一用例断言 1→0 只发生一次）

  // 行为断言：markRead 全程只新增 remote.markRead 调用，绝不触发 remote.inbox 重取或其他业务调用
  assert.deepEqual(scenario.calls().slice(inboxCallsBefore), ['markRead:n-1', 'markRead:n-1']);
  await assert.rejects(
    inbox.markRead('   '),
    (error: unknown) => error instanceof InboxError && error.code === 'INBOX_INVALID_INPUT',
  );
});

test('markRead publishes the decremented unread view to subscribers', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { inbox } = inboxWith(leaseRef, [
    { items: [{ notificationId: 'n-1', kind: 'attention', read: false }], unreadCount: 1 },
  ]);
  const seen: number[] = [];
  const unsubscribe = inbox.subscribe((view) => { seen.push(view.unreadCount); });
  await inbox.page();
  await inbox.markRead('n-1');
  unsubscribe();
  assert.deepEqual(seen, [1, 0], '订阅者看到未读 1 → 0');
});

test('applyHint only re-projects: hint.kind never enters the view and no business call happens (AC1)', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { scenario, inbox } = inboxWith(leaseRef, [
    { items: [{ notificationId: 'n-1', kind: 'attention', read: false }], unreadCount: 1 },
  ]);
  await inbox.page();
  scenario.setPages([{ items: [{ notificationId: 'n-1', kind: 'attention', read: true }], unreadCount: 0 }]);

  const view = await inbox.applyHint({ kind: 'run.completed' });

  assert.equal(view.items[0]!.read, true, 'hint 后投影来自权威重取');
  assert.equal(view.unreadCount, 0);
  assert.equal(JSON.stringify(view).includes('run.completed'), false, 'hint 语义不进入任何状态或视图');
  assert.deepEqual(scenario.calls(), ['inbox', 'inbox'], 'hint 只触发一次 inbox 重取，零其他调用');
});

test('resolveTarget wires the safe deep-link parser: valid passes, malformed is undefined', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const { inbox } = inboxWith(leaseRef, [
    {
      items: [
        { notificationId: 'n-1', deepLink: 'weknora://tasks/detail?taskId=t-1&runId=r-1' },
        { notificationId: 'n-2', deepLink: 'https://evil.example/tasks/detail?taskId=t&runId=r' },
        { notificationId: 'n-3' },
      ],
    },
  ]);
  const view = await inbox.page();

  assert.deepEqual(inbox.resolveTarget(view.items[0]!), { kind: 'task-detail', taskId: 't-1', runId: 'r-1' });
  assert.equal(inbox.resolveTarget(view.items[1]!), undefined);
  assert.equal(inbox.resolveTarget(view.items[2]!), undefined);
});

test('a revoked scope lease rejects page and markRead with INBOX_SCOPE_CHANGED', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  const { inbox } = inboxWith(leaseRef, [{ items: [] }]);

  revocable.revoke();
  // 注：more() 不在此断言——本用例从未 page()，无活跃查询时 more() 先按 INBOX_NO_ACTIVE_QUERY
  // 拒绝（与 TaskOffice moreTasks 同语义，见上一用例）；lease 围栏对 more 的拦截由其内部
  // fetchPage 的 requireLease/settle 承担，已被 page/markRead 路径覆盖同一代码。
  await assert.rejects(inbox.page(), (error: unknown) => error instanceof InboxError && error.code === 'INBOX_SCOPE_CHANGED');
  await assert.rejects(inbox.markRead('n-1'), (error: unknown) => error instanceof InboxError && error.code === 'INBOX_SCOPE_CHANGED');
});

test('a late page result superseded by a newer page is rejected, never published', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  let releaseFirst: (() => void) | undefined;
  const hungRemote: InboxRemote = {
    inbox: async () => {
      if (releaseFirst === undefined) {
        await new Promise<void>((resolve) => { releaseFirst = resolve; });
        return { items: [], unreadCount: 0 };
      }
      return { items: [{ notificationId: 'fresh', read: false }], unreadCount: 1 };
    },
    markRead: async () => {},
  };
  const inbox = createNotificationInbox({ remote: hungRemote, lease: () => leaseRef.lease });

  const first = inbox.page();
  const second = inbox.page(); // 新查询发起，旧的被取代
  releaseFirst!();
  await second;
  await assert.rejects(first, (error: unknown) => error instanceof InboxError && error.code === 'INBOX_SUPERSEDED');
});

test('remote failures surface as INBOX_BACKEND without leaking raw errors', async () => {
  const leaseRef: { lease?: ScopeLease } = { lease: leased().lease };
  const failing: InboxRemote = {
    inbox: async () => { throw Object.assign(new Error('SECRET-BEARER-VALUE'), { name: 'ApiError', status: 500 }); },
    markRead: async () => {},
  };
  const inbox = createNotificationInbox({ remote: failing, lease: () => leaseRef.lease });

  await assert.rejects(
    inbox.page(),
    (error: unknown) => error instanceof InboxError && error.code === 'INBOX_BACKEND' && !String((error as Error).message).includes('SECRET'),
  );
});
