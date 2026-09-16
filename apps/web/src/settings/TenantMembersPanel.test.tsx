import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import { renderToStaticMarkup } from 'react-dom/server';
import type { TenantInvitation, TenantMember, TenantRole, WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  Event: dom.window.Event,
  CustomEvent: dom.window.CustomEvent,
  FocusEvent: dom.window.FocusEvent,
  NodeFilter: dom.window.NodeFilter,
  MouseEvent: dom.window.MouseEvent,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle.bind(dom.window),
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { TenantMembersPanel } = await import('./TenantMembersPanel.tsx');

const alice: TenantMember = { user_id: 'u1', username: 'Alice', email: 'alice@example.com', role: 'viewer', status: 'active', joined_at: '2030-01-01' };
const bob: TenantMember = { user_id: 'u2', username: 'Bob', email: 'bob@example.com', role: 'viewer', status: 'active', joined_at: '2030-01-02' };
let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

function clientWithList(list: (tenantId: number, input: { q?: string; page: number; pageSize: number }) => Promise<{ items: TenantMember[]; total: number }>): WeKnoraClient {
  return {
    auth: { me: async () => ({ user: { id: 'current-user' } }) },
    identity: { tenants: { members: { list }, invitations: { listTenant: async () => ({ items: [], total: 0 }) } } },
  } as unknown as WeKnoraClient;
}

async function mount(client: WeKnoraClient, initialMembers?: { items: TenantMember[]; total: number }, role: 'viewer' | 'admin' = 'viewer', locale: 'en-US' | 'zh-CN' = 'en-US') {
  window.localStorage.setItem('locale', locale);
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<TenantMembersPanel client={client} tenantId={1} role={role} initialMembers={initialMembers} />);
  });
  return container;
}

test('tenant members keeps viewer state read-only', () => {
  const html = renderToStaticMarkup(<TenantMembersPanel client={{} as never} tenantId={1} role="viewer" initialMembers={{ items: [alice], total: 1 }} />);
  assert.match(html, /Alice/);
  assert.doesNotMatch(html, /Invite member|Send invitation|Remove/);
});

// Adapted for the Vue-parity invite flow: the always-open invite form is gone
// (frontend/src/views/settings/TenantMembers.vue anchors invite inside a
// popup/dialog opened from the list-header add button). The manager surface is
// asserted through the shared-key strings that replace the old hardcoded copy.
test('tenant members exposes manager search, invite and role controls', () => {
  window.localStorage.setItem('locale', 'en-US');
  const html = renderToStaticMarkup(<TenantMembersPanel client={{} as never} tenantId={1} role="admin" initialMembers={{ items: [alice], total: 1 }} />);
  assert.match(html, /Add Member/);
  assert.match(html, /Role for Alice/);
  assert.match(html, /Remove/);
});

test('member list header and localized search stay mounted during initial loading', async () => {
  const request = deferred<{ items: TenantMember[]; total: number }>();
  const container = await mount(clientWithList(() => request.promise));

  const header = container.querySelector('.members-list-header');
  const search = container.querySelector<HTMLInputElement>('input[type="search"]');
  assert.ok(header);
  assert.equal(header.querySelector('.members-list-title')?.textContent, 'Workspace members');
  assert.equal(header.querySelector('.members-list-count-badge')?.textContent, '0');
  assert.equal(search?.placeholder, 'Search by name or email');
  assert.match(container.textContent ?? '', /Loading members/);

  await act(async () => request.resolve({ items: [alice], total: 1 }));
  assert.strictEqual(container.querySelector('.members-list-header'), header);
  assert.strictEqual(container.querySelector('input[type="search"]'), search);
  assert.equal(header.querySelector('.members-list-count-badge')?.textContent, '1');
  assert.match(container.textContent ?? '', /Alice/);
});

test('search remains mounted and clearing reloads the unfiltered member list', async () => {
  const searchRequest = deferred<{ items: TenantMember[]; total: number }>();
  const clearRequest = deferred<{ items: TenantMember[]; total: number }>();
  const queries: Array<string | undefined> = [];
  const client = clientWithList((_tenantId, input) => {
    queries.push(input.q);
    return input.q ? searchRequest.promise : clearRequest.promise;
  });
  const container = await mount(client, { items: [alice], total: 1 }, 'admin');
  const header = container.querySelector('.members-list-header');
  const search = container.querySelector<HTMLInputElement>('input[type="search"]');
  assert.ok(header && search);
  assert.ok(container.querySelector('button[aria-label="Add Member"]'), 'manager invite entry stays mounted');

  await act(async () => {
    const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
    setValue?.call(search, 'Bob');
    search.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
    search.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
  assert.equal(search.value, 'Bob');
  await act(async () => search.form?.dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true })));
  assert.strictEqual(container.querySelector('.members-list-header'), header);
  assert.strictEqual(container.querySelector('input[type="search"]'), search);
  assert.match(container.textContent ?? '', /Loading members/);

  await act(async () => searchRequest.resolve({ items: [bob], total: 1 }));
  assert.match(container.textContent ?? '', /Bob/);
  const clear = container.querySelector<HTMLButtonElement>('button[aria-label="Clear search"]');
  assert.ok(clear, 'a keyboard-focusable clear control should appear for a non-empty query');

  await act(async () => clear.click());
  assert.equal(search.value, '');
  assert.strictEqual(container.querySelector('.members-list-header'), header);
  assert.match(container.textContent ?? '', /Loading members/);

  await act(async () => clearRequest.resolve({ items: [alice], total: 2 }));
  assert.deepEqual(queries, ['Bob', undefined]);
  assert.match(container.textContent ?? '', /Alice/);
  assert.equal(header.querySelector('.members-list-count-badge')?.textContent, '2');
  assert.equal(container.querySelector('button[aria-label="Clear search"]'), null);
});

// ---------------------------------------------------------------------------
// Vue-parity rebuild (r038). Baseline: frontend/src/views/settings/TenantMembers.vue
// ---------------------------------------------------------------------------

const shareLinkInvitation: TenantInvitation = { id: 9, tenant_id: 1, invitee_user_id: 'link', role: 'viewer', status: 'pending', expires_at: '2030-01-09T12:06:00Z', created_at: '2030-01-02T12:06:00Z', is_share_link: true, accepted_count: 0, invite_url: '/register?token=abc' };

interface ListCall { q?: string; page: number; pageSize: number }

function parityClient(options: {
  members?: { items: TenantMember[]; total: number };
  invitations?: { items: TenantInvitation[]; total: number };
  currentUserId?: string;
} = {}) {
  const memberCalls: ListCall[] = [];
  const invitationCalls: Array<{ page?: number; pageSize?: number }> = [];
  const revoked: number[] = [];
  const created: Array<{ email: string; role: TenantRole }> = [];
  const auditCalls: number[] = [];
  const client = {
    auth: { me: async () => ({ user: { id: options.currentUserId ?? 'current-user' } }) },
    identity: { tenants: {
      members: {
        list: async (_tenantId: number, input: ListCall) => { memberCalls.push(input); return options.members ?? { items: [], total: 0 }; },
      },
      invitations: {
        listTenant: async (_tenantId: number, input: { page?: number; pageSize?: number }) => { invitationCalls.push(input); return options.invitations ?? { items: [], total: 0 }; },
        create: async (_tenantId: number, input: { email: string; role: TenantRole }) => { created.push(input); return { id: 77, tenant_id: 1, invitee_user_id: 'pending-user', role: input.role, status: 'pending', expires_at: '2030-01-09T00:00:00Z', created_at: '2030-01-02T00:00:00Z' } satisfies TenantInvitation; },
        revoke: async (_tenantId: number, invitationId: number) => { revoked.push(invitationId); },
        createInviteLink: async () => { throw new Error('createInviteLink not expected in this test'); },
      },
      auditLog: { list: async () => { auditCalls.push(1); return { items: [], nextCursor: 0 }; } },
    } },
  } as unknown as WeKnoraClient;
  return { client, memberCalls, invitationCalls, revoked, created, auditCalls };
}

test('zh-CN panel anatomy mirrors the Vue baseline: header row, two tables, pagers, no English', async () => {
  const { client } = parityClient({
    members: { items: [
      { user_id: 'u1', username: ' paritytester ', email: 'parity-test@local.dev', role: 'owner', status: 'active', joined_at: '2030-01-01T11:58:00Z' },
      { user_id: 'u2', username: 'parityinvitee', email: 'parity-invitee@local.dev', role: 'contributor', status: 'active', joined_at: '2030-01-02T12:06:00Z' },
    ], total: 2 },
    invitations: { items: [shareLinkInvitation], total: 1 },
    currentUserId: 'u1',
  });
  const container = await mount(client, {
    items: [
      { user_id: 'u1', username: ' paritytester ', email: 'parity-test@local.dev', role: 'owner', status: 'active', joined_at: '2030-01-01T11:58:00Z' },
      { user_id: 'u2', username: 'parityinvitee', email: 'parity-invitee@local.dev', role: 'contributor', status: 'active', joined_at: '2030-01-02T12:06:00Z' },
    ], total: 2,
  }, 'admin', 'zh-CN');
  const text = container.textContent ?? '';

  // Header row: title + info trigger + audit link; subtitle + RBAC doc link.
  assert.match(text, /成员管理/);
  assert.ok(container.querySelector('button[aria-label="角色权限说明"]'), 'info-circle trigger next to the title');
  assert.match(container.querySelector<HTMLButtonElement>('button[aria-label="角色权限说明"]')?.getAttribute('aria-label') ?? '', /角色权限说明/);
  const auditBtn = [...container.querySelectorAll('button')].find((b) => (b.textContent ?? '').includes('审计日志'));
  assert.ok(auditBtn, 'audit log entry sits inline with the title');
  assert.match(auditBtn?.textContent ?? '', /审计日志/);
  assert.match(text, /邀请伙伴加入当前空间并分配角色。只有 Owner 可以新增或移除成员。/);
  const docLink = container.querySelector<HTMLAnchorElement>('a[href*="RBAC"]');
  assert.ok(docLink);
  assert.equal(docLink.target, '_blank');
  assert.match(docLink.href, /RBAC/);
  assert.match(docLink.textContent ?? '', /了解 RBAC/);

  // Section order: 待接受的邀请 renders BEFORE 空间成员.
  const pendingTitle = text.indexOf('待接受的邀请');
  const membersTitle = text.indexOf('空间成员');
  assert.ok(pendingTitle >= 0 && membersTitle > pendingTitle, 'pending invitations section precedes the member list');
  assert.match(text, /发出后等待对方在站内确认。7 天未响应将自动过期。/);

  // Invitations table with Vue columns and share-link row anatomy.
  const pendingShell = container.querySelector('.pending-invitations-table');
  assert.ok(pendingShell);
  assert.deepEqual([...pendingShell.querySelectorAll('thead th')].map((th) => th.textContent), ['被邀请人', '角色', '邀请人', '到期时间', '状态', '操作']);
  const inviteeCell = pendingShell.querySelector('.member-cell');
  assert.match(inviteeCell?.textContent ?? '', /通过链接邀请/);
  assert.match(inviteeCell?.textContent ?? '', /尚无成员加入/);
  assert.match(pendingShell.querySelector('.status-tag')?.textContent ?? '', /生效中/);
  assert.ok(pendingShell.querySelector('button[aria-label="复制邀请链接"]'), 'per-row copy action for an active share link');
  assert.ok(pendingShell.querySelector('button[aria-label="撤销"]'), 'per-row revoke action');

  // Members list header: count badge, search, add-member + share-link icon buttons.
  const header = container.querySelector('.members-list-header');
  assert.equal(header?.querySelector('.members-list-title')?.textContent, '空间成员');
  assert.equal(header?.querySelector('.members-list-count-badge')?.textContent, '2');
  const search = container.querySelector<HTMLInputElement>('input[type="search"]');
  assert.equal(search?.placeholder, '按姓名或邮箱搜索');
  const addBtn = container.querySelector<HTMLButtonElement>('button[aria-label="邀请成员"]');
  assert.match(addBtn?.getAttribute('aria-label') ?? '', /邀请成员/);
  const shareBtn = container.querySelector<HTMLButtonElement>('button[aria-label="生成共享链接"]');
  assert.match(shareBtn?.getAttribute('aria-label') ?? '', /生成共享链接/);

  // Members table with Vue columns, owner badge, role select, remove control.
  const membersShell = container.querySelector('.members-list-wrap .data-table-shell');
  assert.ok(membersShell);
  assert.deepEqual([...membersShell.querySelectorAll('thead th')].map((th) => th.textContent), ['姓名与邮箱', '角色', '加入时间', '操作']);
  const rows = [...membersShell.querySelectorAll('tbody tr')];
  assert.equal(rows.length, 2);
  const ownerCell = rows[0].querySelector('.member-cell');
  assert.match(ownerCell?.textContent ?? '', /paritytester/);
  assert.match(ownerCell?.textContent ?? '', /parity-test@local.dev/);
  assert.match(rows[0].querySelector('.role-cell')?.textContent ?? '', /所有者/);
  assert.equal(rows[0].querySelector('select'), null, 'self/owner row renders a role tag, not a select');
  assert.match(rows[1].querySelector('.role-cell')?.textContent ?? '', /编辑/);
  assert.ok(rows[1].querySelector('select'), 'other rows get the role dropdown');
  assert.ok(rows[1].querySelector('button[aria-label="移除"]'), 'remove icon button on non-self rows');
  assert.equal(rows[0].querySelector('button[aria-label="移除"]'), null);
  assert.match(rows[0].textContent ?? '', /2030\/01\/01/);

  // Pager footers on both tables: 共 N 条数据 + page-size select + jumper.
  const pagers = [...container.querySelectorAll('.data-table-shell__pager')];
  assert.equal(pagers.length, 2);
  for (const pager of pagers) {
    assert.match(pager.textContent ?? '', /共 \d+ 条数据/);
    assert.match(pager.textContent ?? '', /跳至/);
    assert.match(pager.textContent ?? '', /\/1 页/);
    const sizeSelect = pager.querySelector('select');
    assert.ok(sizeSelect, 'page-size select is rendered');
    assert.match(sizeSelect?.selectedOptions[0]?.textContent ?? '', /条\/页/);
  }
  assert.match(pagers[0].textContent ?? '', /共 1 条数据/);
  assert.match(pagers[1].textContent ?? '', /共 2 条数据/);

  // No English leftovers in the visible zh-CN copy.
  for (const leak of ['Pending invitations', 'Refresh invitations', 'Invite member', 'Invite colleagues', 'Send invitation', 'Revoke', 'Search', 'Loading members', 'member(s)', 'Role permissions', 'Audit log', 'Previous', 'Next', 'expires ', 'current user']) {
    assert.doesNotMatch(text, new RegExp(leak.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), 'English leftover: ' + leak);
  }
});

test('permissions popover and audit drawer open from the header row', async () => {
  const { client, auditCalls } = parityClient({ members: { items: [alice], total: 1 }, currentUserId: 'u1' });
  const container = await mount(client, { items: [alice], total: 1 }, 'admin', 'zh-CN');

  await act(async () => container.querySelector<HTMLButtonElement>('button[aria-label="角色权限说明"]')?.click());
  const popover = container.querySelector('[role="dialog"]');
  assert.ok(popover, 'info trigger opens the role-permission matrix');
  assert.match(popover?.textContent ?? '', /角色权限说明/);
  for (const role of ['所有者', '管理员', '编辑', '访客']) assert.match(popover?.textContent ?? '', new RegExp(role));
  for (const perm of ['管理成员', '修改空间配置', '配置模型 \/ 向量库 \/ IM 通道', '创建并编辑自己的知识库和智能体', '查看空间内容']) {
    assert.match(popover?.textContent ?? '', new RegExp(perm));
  }
  assert.match(popover?.textContent ?? '', /我/, 'current role is badged with 我');

  await act(async () => [...container.querySelectorAll('button')].find((b) => (b.textContent ?? '').includes('审计日志'))?.click());
  // The audit surface now mirrors Vue SettingDrawer (TenantMembers.vue:383-386):
  // a right-side drawer teleported to document.body, named via aria-labelledby.
  const audit = [...document.querySelectorAll('aside[role="dialog"]')].find((el) => el.textContent?.includes('审计日志'));
  assert.ok(audit, 'audit link opens the audit log drawer');
  assert.equal(auditCalls.length, 1, 'audit log is lazy-loaded on first open');
  assert.match(audit?.textContent ?? '', /刷新/);
  assert.match(audit?.textContent ?? '', /暂无审计事件。/);
});

test('invite dialog opens from the add-member button and sends the invitation', async () => {
  const { client, created, invitationCalls } = parityClient({ members: { items: [alice], total: 1 } });
  const container = await mount(client, { items: [alice], total: 1 }, 'admin', 'zh-CN');
  assert.equal(container.querySelector('[role="dialog"]'), null, 'dialog stays closed until the add button is clicked');

  const addBtn = container.querySelector<HTMLButtonElement>('button[aria-label="邀请成员"]');
  await act(async () => addBtn?.click());
  // The project Dialog follows Vue Teleport/Radix Portal semantics and mounts
  // the modal under document.body rather than inside the panel root.
  const dialog = document.querySelector('[role="dialog"]');
  assert.ok(dialog);
  assert.match(dialog?.textContent ?? '', /邀请成员/);
  const email = dialog?.querySelector<HTMLInputElement>('input[type="email"]');
  assert.ok(email, 'email field');
  assert.equal(email?.placeholder, 'invitee@example.com');
  const labels = dialog?.querySelector('select');
  assert.ok(labels, 'role select');
  assert.deepEqual([...(labels?.options ?? [])].map((option) => option.textContent), ['所有者', '管理员', '编辑', '访客']);
  assert.match(dialog?.textContent ?? '', /取消/);
  assert.match(dialog?.textContent ?? '', /发送邀请/);

  await act(async () => {
    const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
    setValue?.call(email, 'new@example.com');
    email.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
    email.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
  const form = dialog?.querySelector('form');
  await act(async () => form?.dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true })));
  assert.deepEqual(created, [{ email: 'new@example.com', role: 'contributor' }]);
  assert.ok(invitationCalls.length >= 2, 'invitations reload after the invite is sent');
  assert.equal(document.querySelector('[role="dialog"]'), null, 'dialog closes after a successful send');
  assert.match(container.textContent ?? '', /邀请已发出，等待对方接受。/);
});

test('pending invitation rows expose status badges and an inline revoke confirm', async () => {
  const direct: TenantInvitation = { id: 5, tenant_id: 1, invitee_user_id: 'u9', invitee_email: 'pending@example.com', role: 'viewer', status: 'pending', expires_at: '2030-01-09T12:06:00Z', created_at: '2030-01-02T12:06:00Z', invite_url: '/register?token=xyz' };
  const { client, revoked, invitationCalls } = parityClient({ invitations: { items: [direct, shareLinkInvitation], total: 2 }, members: { items: [alice], total: 1 } });
  const container = await mount(client, { items: [alice], total: 1 }, 'admin', 'zh-CN');
  const shell = container.querySelector('.pending-invitations-table');
  assert.ok(shell);
  const rows = [...shell.querySelectorAll('tbody tr')];
  assert.equal(rows.length, 2);
  const directCells = rows[0].querySelectorAll('td');
  assert.match(rows[0].querySelector('.member-cell')?.textContent ?? '', /pending@example.com/);
  assert.match(rows[0].querySelector('.status-tag')?.textContent ?? '', /待接受/);
  assert.match(directCells[1]?.textContent ?? '', /访客/);

  const revokeTrigger = rows[0].querySelector<HTMLButtonElement>('button[aria-label="撤销"]');
  assert.ok(revokeTrigger);
  await act(async () => revokeTrigger.click());
  const confirm = rows[0].querySelector('[role="alertdialog"]');
  assert.ok(confirm, 'inline popconfirm anchored to the revoke button');
  assert.match(confirm?.textContent ?? '', /撤销后，pending@example.com 将无法再接受此邀请/);
  assert.match(confirm?.textContent ?? '', /取消/);
  const confirmButtons = confirm?.querySelectorAll<HTMLButtonElement>('button');
  const confirmBtn = confirmButtons?.[confirmButtons.length - 1];
  await act(async () => confirmBtn?.click());
  assert.deepEqual(revoked, [5]);
  assert.ok(invitationCalls.length >= 2, 'invitations reload after revoke');

  const shareRow = rows[1];
  assert.match(shareRow.querySelector('.member-cell')?.textContent ?? '', /通过链接邀请/);
  assert.match(shareRow.textContent ?? '', /生效中/);
});

test('members pager drives server-side pagination like the Vue table', async () => {
  const members = Array.from({ length: 25 }, (_unused, index): TenantMember => ({ user_id: 'u' + index, username: 'member' + index, email: 'm' + index + '@example.com', role: 'viewer', status: 'active', joined_at: '2030-01-03' }));
  const { client, memberCalls } = parityClient({ members: { items: members.slice(0, 20), total: 25 } });
  const container = await mount(client, { items: members.slice(0, 20), total: 25 }, 'admin', 'zh-CN');
  const shell = container.querySelector('.members-list-wrap .data-table-shell');
  assert.ok(shell);
  assert.match(shell?.textContent ?? '', /共 25 条数据/);
  const pager = shell?.querySelector('.data-table-shell__pager');
  const next = pager?.querySelector<HTMLButtonElement>('button[aria-label="下一步"]');
  await act(async () => next?.click());
  assert.deepEqual(memberCalls.at(-1), { q: undefined, page: 2, pageSize: 20 });

  // Jumper navigates (跳至 1) while still on pageSize 20.
  const jumper = pager?.querySelector<HTMLInputElement>('input');
  assert.ok(jumper);
  await act(async () => {
    const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
    setValue?.call(jumper, '1');
    jumper.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {
    jumper.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
  });
  assert.deepEqual(memberCalls.at(-1), { q: undefined, page: 1, pageSize: 20 });

  const size = pager?.querySelector<HTMLSelectElement>('select');
  assert.ok(size);
  await act(async () => {
    const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLSelectElement.prototype, 'value')?.set;
    setValue?.call(size, '50');
    size.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
  assert.deepEqual(memberCalls.at(-1), { q: undefined, page: 1, pageSize: 50 });

  // With 25 records on 50 条/页 the jumper clamps out-of-range pages (no request).
  const callsAfterSize = memberCalls.length;
  await act(async () => {
    const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
    setValue?.call(jumper, '2');
    jumper.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });
  await act(async () => {
    jumper.dispatchEvent(new dom.window.KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
  });
  assert.equal(memberCalls.length, callsAfterSize);
  assert.deepEqual(memberCalls.at(-1), { q: undefined, page: 1, pageSize: 50 });
});
