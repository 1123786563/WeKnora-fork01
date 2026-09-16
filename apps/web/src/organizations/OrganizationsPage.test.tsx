import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import type { Organization, WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') || specifier.endsWith('.svg') ? { shortCircuit: true, url: 'data:text/javascript,export default "stub"' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/organizations' });
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  MouseEvent: dom.window.MouseEvent,
  IS_REACT_ACT_ENVIRONMENT: true,
});

const { createRoot } = await import('react-dom/client');
const { OrganizationsPage } = await import('./OrganizationsPage.tsx');

const ownerOrg: Organization = {
  id: 'org-1',
  name: 'parity-org',
  description: '',
  owner_id: 'user-1',
  owner_tenant_id: 1,
  is_owner: true,
  my_role: 'admin',
  member_count: 2,
  share_count: 3,
  agent_share_count: 1,
  pending_join_request_count: 0,
} as unknown as Organization;

const joinedOrg: Organization = {
  id: 'org-2',
  name: 'joined-org',
  description: '团队空间',
  owner_id: 'user-2',
  owner_tenant_id: 2,
  is_owner: false,
  my_role: 'editor',
  member_count: 3,
  share_count: 2,
  agent_share_count: 0,
  pending_join_request_count: 0,
} as unknown as Organization;

interface Calls { create: unknown[]; preview: string[]; join: unknown[]; submitJoinRequest: unknown[]; updateRole: unknown[][]; review: unknown[][]; leave: string[]; remove: string[]; update: unknown[][]; membersList: string[]; joinRequestsList: string[]; kbSharesList: string[]; agentSharesList: string[]; inviteCode: string[] }

// R017: auth/me drives the page's self-resolved canManageOrg when no explicit
// role prop is passed. The default mirrors an admin home-tenant membership so
// legacy gating-free expectations stay valid; RBAC tests override it.
export interface MeOverride { role: string; canAccessAllTenants?: boolean }

function clientWith(organizations: Organization[], me_?: MeOverride): { client: WeKnoraClient; calls: Calls } {
  const calls: Calls = { create: [], preview: [], join: [], submitJoinRequest: [], updateRole: [], review: [], leave: [], remove: [], update: [], membersList: [], joinRequestsList: [], kbSharesList: [], agentSharesList: [], inviteCode: [] };
  const organizationsApi = {
    list: async () => ({ items: organizations, total: organizations.length }),
    get: async (id: string) => organizations.find((item) => item.id === id) ?? organizations[0],
    create: async (input: unknown) => { calls.create.push(input); return { ...ownerOrg, ...(input as Record<string, unknown>) }; },
    update: async (id: string, input: unknown) => { calls.update.push([id, input]); return organizations[0]; },
    remove: async (id: string) => { calls.remove.push(id); },
    join: async (input: unknown) => { calls.join.push(input); return joinedOrg; },
    submitJoinRequest: async (input: unknown) => { calls.submitJoinRequest.push(input); },
    preview: async (code: string) => { calls.preview.push(code); return { id: 'org-9', name: 'previewed-org', description: '', member_count: 4, share_count: 0, agent_share_count: 0, is_already_member: false, require_approval: true }; },
    search: async () => ({ items: [], total: 0 }),
    joinById: async () => joinedOrg,
    leave: async (id: string) => { calls.leave.push(id); },
    requestRoleUpgrade: async () => ({}),
    generateInviteCode: async (id: string) => { calls.inviteCode.push(id); return { inviteCode: 'GEN-CODE' }; },
    members: {
      list: async (id: string) => { calls.membersList.push(id); return { items: [{ id: 'm1', user_id: 'u1', username: 'Alice', email: 'a@x.dev', role: 'admin', tenant_id: 1, joined_at: '2030-01-01' }], total: 1 }; },
      updateRole: async (...input: unknown[]) => { calls.updateRole.push(input); },
      remove: async () => {},
    },
    joinRequests: {
      list: async (id: string) => { calls.joinRequestsList.push(id); return { items: [{ id: 'r1', user_id: 'u9', username: 'Bob', email: 'b@x.dev', message: '', request_type: 'join', requested_role: 'viewer', status: 'pending', created_at: '2030-01-02' }], total: 1 }; },
      review: async (...input: unknown[]) => { calls.review.push(input); },
    },
    knowledgeBaseShares: {
      listForOrganization: async (id: string) => { calls.kbSharesList.push(id); return { items: [], total: 0 }; },
      remove: async () => {},
    },
    agentShares: {
      listForOrganization: async (id: string) => { calls.agentSharesList.push(id); return { items: [{ id: 'agent-share-1', agent_id: 'agent-1', agent_name: 'Research agent', permission: 'viewer' }], total: 1 }; },
      remove: async () => {},
    },
  };
  const me = me_ ?? { role: 'admin' };
  const auth = {
    me: async () => ({
      user: { id: 'u1', can_access_all_tenants: me.canAccessAllTenants === true },
      tenant: { id: 1 },
      memberships: [{ tenant_id: 1, role: me.role }],
    }),
  };
  return { client: { auth, identity: { organizations: organizationsApi } } as unknown as WeKnoraClient, calls };
}

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.history.replaceState({}, '', '/platform/organizations');
});

async function mountPage(client: WeKnoraClient, inviteCode?: string, role?: 'owner' | 'admin' | 'contributor' | 'viewer'): Promise<HTMLElement> {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<OrganizationsPage client={client} inviteCode={inviteCode} role={role} />);
  });
  return container;
}

function buttonWithLabel(root: HTMLElement, label: string): HTMLButtonElement {
  const button = root.querySelector('button[aria-label="' + label + '"]');
  assert.ok(button, 'expected button with aria-label ' + label);
  return button as HTMLButtonElement;
}

async function click(element: HTMLElement): Promise<void> {
  await act(async () => element.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true })));
}

function setInputValue(input: HTMLInputElement | HTMLTextAreaElement, value: string): void {
  const proto = input instanceof dom.window.HTMLTextAreaElement ? dom.window.HTMLTextAreaElement.prototype : dom.window.HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, 'value')?.set;
  setter?.call(input, value);
  input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
}

async function setInputValueAsync(input: HTMLInputElement | HTMLTextAreaElement, value: string): Promise<void> {
  await act(async () => setInputValue(input, value));
}

async function selectValue(select: HTMLSelectElement, value: string): Promise<void> {
  await act(async () => {
    select.value = value;
    select.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
  });
}

async function submitForm(form: HTMLFormElement): Promise<void> {
  await act(async () => form.dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true })));
}

function textButtons(root: HTMLElement, text: string): HTMLButtonElement[] {
  return [...root.querySelectorAll('button')].filter((button) => button.textContent === text) as HTMLButtonElement[];
}

// Semantic queries replacing the deleted organizations.css class selectors:
// cards are the role="button" tiles carrying the titled org-name span (section
// headers and the ⋯ affordance are role="button" too but title nothing), and
// the rail lives in the labeled <aside>. "is-active" survives in the TSX as a
// state hook (its styles are utilities), so the rail selector stays class-based.
function orgCards(root: HTMLElement): HTMLElement[] {
  return [...root.querySelectorAll('[role="button"]')].filter((el) => el.querySelector('span[title]') !== null) as HTMLElement[];
}

test('zh-CN page renders Vue-parity anatomy and drops the English debug page', async () => {
  const { client } = clientWith([ownerOrg, joinedOrg]);
  const root = await mountPage(client);

  const heading = root.querySelector('h2');
  assert.ok(heading, 'expected an h2 page title');
  assert.equal(heading.textContent, '共享空间');
  assert.match(root.textContent ?? '', /创建或加入共享空间，让多个空间互相协作/);

  buttonWithLabel(root, '加入共享空间');
  buttonWithLabel(root, '创建共享空间');

  assert.match(root.textContent ?? '', /我创建的/);
  const createdHeader = [...root.querySelectorAll('[role="button"]')].find((el) => (el.textContent ?? '').startsWith('我创建的'));
  assert.ok(createdHeader && /\d/.test(createdHeader.textContent ?? ''), 'expected a live section count chip');

  const cards = orgCards(root);
  assert.equal(cards.length, 2);
  assert.match(root.textContent ?? '', /parity-org/);
  assert.match(root.textContent ?? '', /joined-org/);
  assert.match(root.textContent ?? '', /暂无描述/);
  assert.match(root.textContent ?? '', /团队空间/);
  assert.ok(root.querySelector('[style*="linear-gradient"]'), 'expected gradient avatar');
  assert.ok(cards[0]?.querySelector('svg[width="56"]'), 'expected constellation decoration');
  const statBadges = [...root.querySelectorAll('[title="成员数量"], [title="知识库"], [title="智能体"]')];
  assert.ok(statBadges.length >= 6, 'expected member/kb/agent stat badges on every card');
  for (const badge of statBadges) {
    const label = badge.getAttribute('title') ?? '';
    assert.ok(['成员数量', '知识库', '智能体'].includes(label), 'unexpected stat badge label: ' + label);
  }

  const text = root.textContent ?? '';
  assert.doesNotMatch(text, /WORKSPACE ORGANIZATIONS/);
  assert.doesNotMatch(text, /Manage organization membership/);
  assert.doesNotMatch(text, /Reload/);
  assert.doesNotMatch(text, /members · .* KB shares/);
  const textareas = root.querySelectorAll('form textarea');
  assert.equal(textareas.length, 0, 'create organization must live in a modal, not an inline form');
});

test('empty state mirrors the Vue empty markup with join and create actions', async () => {
  const { client } = clientWith([]);
  const root = await mountPage(client);
  const text = root.textContent ?? '';
  assert.match(text, /您还没有加入任何共享空间/);
  assert.match(text, /创建一个共享空间或通过邀请码加入现有共享空间/);
  const joinActions = textButtons(root, '加入共享空间');
  const createActions = textButtons(root, '创建共享空间');
  assert.equal(joinActions.length + createActions.length, 2, 'expected the two empty-state actions');
  assert.match(joinActions[0]?.textContent ?? '', /加入共享空间/);
  assert.match(createActions[0]?.textContent ?? '', /创建共享空间/);
});

test('create header button opens a modal and the create API is called on submit', async () => {
  const { client, calls } = clientWith([ownerOrg]);
  const root = await mountPage(client);
  await click(buttonWithLabel(root, '创建共享空间'));

  const dialog = root.querySelector('[role="dialog"]');
  assert.ok(dialog, 'expected create modal');
  assert.match(dialog.textContent ?? '', /创建共享空间/);

  const nameInput = dialog.querySelector('input[name="organization-name"]');
  assert.ok(nameInput, 'expected name input inside the modal');
  await setInputValueAsync(nameInput as HTMLInputElement, '新空间');
  const description = dialog.querySelector('textarea[name="organization-description"]');
  assert.ok(description, 'expected description input inside the modal');
  await setInputValueAsync(description as HTMLTextAreaElement, '  描述  ');
  const form = dialog.querySelector('form');
  assert.ok(form, 'expected create form');
  await submitForm(form as HTMLFormElement);
  assert.equal(calls.create.length, 1);
  assert.equal((calls.create[0] as { name: string }).name, '新空间');
  assert.equal((calls.create[0] as { description: string }).description, '描述');
});

test('blank create names show the Vue validation warning instead of failing silently', async () => {
  const { client, calls } = clientWith([ownerOrg]);
  const root = await mountPage(client);
  await click(buttonWithLabel(root, '创建共享空间'));

  const dialog = root.querySelector('[role="dialog"]') as HTMLElement | null;
  assert.ok(dialog, 'expected create modal');
  const form = dialog.querySelector('form');
  assert.ok(form, 'expected create form');
  await submitForm(form as HTMLFormElement);

  assert.equal(calls.create.length, 0, 'blank names must not call the create API');
  const validationToast = root.querySelector('[role="status"]');
  assert.ok(validationToast, 'Vue shows a validation warning through MessagePlugin');
  assert.match(validationToast?.textContent ?? '', /请输入共享空间名称/);
  assert.match(validationToast?.className ?? '', /text-\[#faad14\]/, 'validation should use warning styling');
});

test('list failure renders an error state and retry recovers without the empty state', async () => {
  let attempts = 0;
  const { client } = clientWith([ownerOrg]);
  client.identity.organizations.list = async () => { attempts += 1; if (attempts === 1) throw new Error('organizations unavailable'); return { items: [ownerOrg], total: 1 }; };
  const root = await mountPage(client);
  assert.match(root.textContent ?? '', /organizations unavailable/);
  assert.doesNotMatch(root.textContent ?? '', /您还没有加入任何共享空间/);
  await click(textButtons(root, '重试')[0]!);
  await act(async () => {});
  assert.match(root.textContent ?? '', /parity-org/);
  assert.equal(attempts, 2);
});

test('create modal matches the Vue editor dimensions and keeps the primary action in its footer', async () => {
  const { client } = clientWith([ownerOrg]);
  const root = await mountPage(client);
  await click(buttonWithLabel(root, '创建共享空间'));

  const dialog = root.querySelector('[role="dialog"]') as HTMLElement | null;
  assert.ok(dialog, 'expected create modal');
  assert.match(dialog.className, /h-\[85vh\]/, 'Vue editor uses an 85vh modal');
  assert.match(dialog.className, /max-w-\[1100px\]/, 'Vue editor caps the modal at 1100px');
  assert.match(dialog.className, /max-h-\[750px\]/, 'Vue editor caps the modal height at 750px');

  const createButtons = textButtons(dialog, '创建共享空间');
  assert.equal(createButtons.length, 1, 'expected one primary create action');
  assert.match(createButtons[0]?.parentElement?.className ?? '', /border-t/, 'primary action belongs to the footer');
});

test('join modal previews an invite code and submits an approval-gated request', async () => {
  const { client, calls } = clientWith([ownerOrg]);
  const root = await mountPage(client);
  await click(buttonWithLabel(root, '加入共享空间'));

  const dialog = root.querySelector('[role="dialog"]');
  assert.ok(dialog, 'expected join modal');
  assert.match(dialog.textContent ?? '', /输入邀请码/);

  const codeInput = dialog.querySelector('input[name="join-code"]');
  assert.ok(codeInput, 'expected invite code input');
  await setInputValueAsync(codeInput as HTMLInputElement, 'CODE123');
  const previewButtons = textButtons(dialog as HTMLElement, '查看');
  assert.equal(previewButtons.length, 1, 'expected preview action button');
  await click(previewButtons[0]);
  assert.deepEqual(calls.preview, ['CODE123']);

  const dialogAfterPreview = root.querySelector('[role="dialog"]');
  assert.ok(dialogAfterPreview);
  assert.match(dialogAfterPreview.textContent ?? '', /previewed-org/);
  assert.match(dialogAfterPreview.textContent ?? '', /需要审核/);
  const roleSelect = dialogAfterPreview.querySelector('select');
  assert.ok(roleSelect, 'approval-gated preview must offer a requested role');

  const submitButtons = textButtons(dialogAfterPreview as HTMLElement, '申请加入');
  assert.equal(submitButtons.length, 1, 'expected request-to-join submit button');
  await click(submitButtons[0]);
  assert.equal(calls.submitJoinRequest.length, 1);
  assert.equal((calls.submitJoinRequest[0] as { invite_code: string }).invite_code, 'CODE123');
});

test('card click opens the shared-space settings modal with members and join requests', async () => {
  const { client, calls } = clientWith([ownerOrg]);
  const root = await mountPage(client);

  const card = orgCards(root)[0];
  assert.ok(card);
  await click(card as HTMLElement);
  await act(async () => {});

  const dialog = root.querySelector('[role="dialog"]');
  assert.ok(dialog, 'expected settings modal after card click');
  assert.match(dialog.textContent ?? '', /共享空间设置/);
  assert.deepEqual(calls.membersList, ['org-1']);
  assert.deepEqual(calls.joinRequestsList, ['org-1']);
  assert.deepEqual(calls.agentSharesList, ['org-1']);

  await act(async () => {});
  const dialogReady = root.querySelector('[role="dialog"]') as HTMLElement;
  const membersNav = [...dialogReady.querySelectorAll('button')].find((button) => button.textContent === '共享空间成员');
  assert.ok(membersNav, 'expected members nav item');
  await click(membersNav);
  await act(async () => {});
  const membersPanel = root.querySelector('[role="dialog"]') as HTMLElement;
  assert.match(membersPanel.textContent ?? '', /Alice/);

  const requestsNav = [...membersPanel.querySelectorAll('button')].find((button) => button.textContent === '待审核申请');
  assert.ok(requestsNav, 'expected join-requests nav item');
  await click(requestsNav);
  await act(async () => {});
  const requestsPanel = root.querySelector('[role="dialog"]') as HTMLElement;
  assert.match(requestsPanel.textContent ?? '', /Bob/);

  const approveButtons = textButtons(requestsPanel, '通过');
  assert.equal(approveButtons.length, 1, 'expected approve button for pending join request');
  await click(approveButtons[0]);
  assert.equal(calls.review.length, 1);
  assert.equal((calls.review[0] as unknown[])[0], 'org-1');
  assert.equal((calls.review[0] as unknown[])[1], 'r1');
});

test('settings renders shared agents and protects the organization owner member', async () => {
  const { client } = clientWith([ownerOrg]);
  client.identity.organizations.members.list = async () => ({ items: [{ id: 'owner-member', user_id: 'user-1', username: 'Owner', email: 'owner@x.dev', role: 'admin', tenant_id: 1, joined_at: '2030-01-01' }], total: 1 });
  const root = await mountPage(client);
  await click(orgCards(root)[0] as HTMLElement);
  await act(async () => {});
  const dialog = root.querySelector('[role="dialog"]') as HTMLElement;
  const membersNav = [...dialog.querySelectorAll('button')].find((button) => button.textContent === '共享空间成员');
  assert.ok(membersNav);
  await click(membersNav);
  assert.equal((dialog.querySelector('select[aria-label="角色"]') as HTMLSelectElement).disabled, true);
  assert.equal(textButtons(dialog, '移除').length, 0);
  const agentsNav = [...dialog.querySelectorAll('button')].find((button) => button.textContent === '共享智能体');
  assert.ok(agentsNav);
  await click(agentsNav);
  assert.match(dialog.textContent ?? '', /Research agent/);
});

test('members section filters the Vue-parity member list by name or email', async () => {
  const { client } = clientWith([ownerOrg]);
  client.identity.organizations.members.list = async () => ({
    items: [
      { id: 'member-alice', user_id: 'u1', username: 'Alice', email: 'alice@example.dev', role: 'admin', tenant_id: 1, joined_at: '2030-01-01' },
      { id: 'member-bob', user_id: 'u2', username: 'Bob', email: 'bob@example.dev', role: 'viewer', tenant_id: 2, joined_at: '2030-01-02' },
    ],
    total: 2,
  });
  const root = await mountPage(client);
  await click(orgCards(root)[0] as HTMLElement);
  await act(async () => {});

  const dialog = root.querySelector('[role="dialog"]') as HTMLElement;
  const membersNav = [...dialog.querySelectorAll('button')].find((button) => button.textContent === '共享空间成员');
  assert.ok(membersNav);
  await click(membersNav);
  await act(async () => {});
  assert.match(dialog.textContent ?? '', /Alice/);
  assert.match(dialog.textContent ?? '', /Bob/);

  const search = dialog.querySelector('input[placeholder="搜索成员…"]') as HTMLInputElement | null;
  assert.ok(search, 'Vue members section exposes a member search input');
  await setInputValueAsync(search, 'bob@');

  assert.match(dialog.textContent ?? '', /Bob/);
  assert.doesNotMatch(dialog.textContent ?? '', /Alice/);
  assert.equal(dialog.querySelector('[aria-label="共享空间成员 count"]')?.textContent, '1');
});

test('settings modal exposes an equivalent section selector when the sidebar is hidden on mobile', async () => {
  const { client } = clientWith([ownerOrg]);
  const root = await mountPage(client);
  await click(orgCards(root)[0] as HTMLElement);
  await act(async () => {});

  const dialog = root.querySelector('[role="dialog"]') as HTMLElement | null;
  assert.ok(dialog, 'expected settings modal');
  const sectionSelector = dialog.querySelector('[data-testid="organization-settings-section-selector"]') as HTMLSelectElement | null;
  assert.ok(sectionSelector, 'expected mobile section selector');
  assert.match(sectionSelector.parentElement?.className ?? '', /max-\[720px\]:block/, 'selector should be available at the mobile breakpoint');
  assert.deepEqual([...sectionSelector.options].map((option) => [option.value, option.textContent]), [
    ['basic', '基本信息'],
    ['members', '共享空间成员'],
    ['requests', '待审核申请'],
    ['shares', '共享知识库'],
    ['agents', '共享智能体'],
    ['invite', '邀请链接'],
  ]);
  assert.equal(sectionSelector.value, 'basic');

  await selectValue(sectionSelector, 'members');
  assert.equal(sectionSelector.value, 'members');
  assert.match(dialog.textContent ?? '', /Alice/, 'selector change should render the selected section');
});

test('invite_code prop auto-previews the linked organization', async () => {
  const { client, calls } = clientWith([ownerOrg]);
  await mountPage(client, 'LINK1');
  assert.deepEqual(calls.preview, ['LINK1']);
});

test('more menu offers leave for joined spaces and hides delete for non-owners', async () => {
  const { client } = clientWith([ownerOrg, joinedOrg]);
  const root = await mountPage(client);
  const cards = orgCards(root);
  const joinedCard = cards.find((card) => card.textContent?.includes('joined-org'));
  assert.ok(joinedCard);
  const more = (joinedCard as HTMLElement).querySelector('[role="button"][aria-label="编辑"]');
  assert.ok(more);
  await click(more as HTMLElement);
  const menu = (joinedCard as HTMLElement).querySelector('[role="button"][aria-label="编辑"] > div');
  assert.ok(menu);
  assert.match(menu.textContent ?? '', /退出共享空间/);
  assert.doesNotMatch(menu.textContent ?? '', /删除/);
});

// ─── R017 RBAC: tenant-role gating on write operations ───────────────────
// Vue OrganizationList.vue:499-504 — canManageOrg = hasRole('admin') ||
// canAccessAllTenants; owner/admin keep the write affordances, contributor/
// viewer get them disabled with the rbac tip. Delete is additionally
// restricted to owned spaces (v-if="org.is_owner && canManageOrg").

const NEED_TENANT_ADMIN_TIP = '此操作需要当前空间的 admin 或更高角色，请联系空间 Owner 调整权限。';

async function openCardMenu(root: HTMLElement, name: string): Promise<HTMLElement> {
  const card = orgCards(root).find((entry) => entry.textContent?.includes(name));
  assert.ok(card, 'expected card ' + name);
  const more = (card as HTMLElement).querySelector('[role="button"][aria-label="编辑"]');
  assert.ok(more);
  await click(more as HTMLElement);
  const menu = (card as HTMLElement).querySelector('[role="button"][aria-label="编辑"] > div');
  assert.ok(menu, 'expected popup menu for ' + name);
  return menu as HTMLElement;
}

test('viewer/contributor roles disable create and join with the rbac tip and hide delete', async () => {
  for (const role of ['viewer', 'contributor'] as const) {
    const { client } = clientWith([ownerOrg, joinedOrg]);
    const root = await mountPage(client, undefined, role);

    const joinButton = buttonWithLabel(root, '加入共享空间');
    const createButton = buttonWithLabel(root, '创建共享空间');
    assert.equal(joinButton.disabled, true, role + ' join header button must be disabled');
    assert.equal(createButton.disabled, true, role + ' create header button must be disabled');
    assert.equal(joinButton.getAttribute('title'), NEED_TENANT_ADMIN_TIP, 'join tooltip mirrors Vue noPermissionTip');
    assert.equal(createButton.getAttribute('title'), NEED_TENANT_ADMIN_TIP, 'create tooltip mirrors Vue noPermissionTip');

    // Owned card: delete item is v-if="org.is_owner && canManageOrg" → hidden.
    const ownerMenu = await openCardMenu(root, 'parity-org');
    assert.doesNotMatch(ownerMenu.textContent ?? '', /删除/);
    assert.match(ownerMenu.textContent ?? '', /共享空间设置/);

    // Joined card: leave stays available for any role (v-if="!org.is_owner").
    const joinedMenu = await openCardMenu(root, 'joined-org');
    assert.match(joinedMenu.textContent ?? '', /退出共享空间/);
  }
});

test('admin/owner roles keep create and join enabled and offer delete on owned spaces', async () => {
  for (const role of ['admin', 'owner'] as const) {
    const { client } = clientWith([ownerOrg]);
    const root = await mountPage(client, undefined, role);
    assert.equal(buttonWithLabel(root, '加入共享空间').disabled, false, role + ' join must stay enabled');
    assert.equal(buttonWithLabel(root, '创建共享空间').disabled, false, role + ' create must stay enabled');
    const menu = await openCardMenu(root, 'parity-org');
    assert.match(menu.textContent ?? '', /删除/);
  }
});

test('empty-state join/create actions are disabled for a viewer', async () => {
  const { client } = clientWith([]);
  const root = await mountPage(client, undefined, 'viewer');
  const actions = [...textButtons(root, '加入共享空间'), ...textButtons(root, '创建共享空间')];
  assert.equal(actions.length, 2);
  assert.equal((actions[0] as HTMLButtonElement).disabled, true);
  assert.equal((actions[1] as HTMLButtonElement).disabled, true);
});

test('viewer cannot edit an owned space when tenant role is below admin', async () => {
  const { client, calls } = clientWith([ownerOrg]);
  const root = await mountPage(client, undefined, 'viewer');

  await click(orgCards(root)[0] as HTMLElement);
  await act(async () => {});

  const dialog = root.querySelector('[role="dialog"]') as HTMLElement | null;
  assert.ok(dialog, 'expected settings modal');
  assert.match(dialog.textContent ?? '', /此操作需要当前空间的 admin 或更高角色/);
  assert.equal((dialog.querySelector('input[name="organization-name"]') as HTMLInputElement).disabled, true);
  assert.equal((dialog.querySelector('textarea[name="organization-description"]') as HTMLTextAreaElement).disabled, true);
  assert.equal(textButtons(dialog, '保存').length, 0, 'read-only settings must not expose save');
  assert.equal(calls.update.length, 0);

  const membersNav = [...dialog.querySelectorAll('button')].find((button) => button.textContent === '共享空间成员');
  assert.ok(membersNav);
  await click(membersNav);
  const memberRole = dialog.querySelector('select[aria-label="角色"]') as HTMLSelectElement | null;
  assert.ok(memberRole);
  assert.equal(memberRole.disabled, true);
  assert.equal(textButtons(dialog, '移除').length, 0);

  const requestsNav = [...dialog.querySelectorAll('button')].find((button) => button.textContent === '待审核申请');
  assert.ok(requestsNav);
  await click(requestsNav);
  assert.equal(textButtons(dialog, '通过').length, 0);
  assert.equal(textButtons(dialog, '拒绝').length, 0);

  const inviteNav = [...dialog.querySelectorAll('button')].find((button) => button.textContent === '邀请链接');
  assert.ok(inviteNav);
  await click(inviteNav);
  assert.equal(textButtons(dialog, '邀请成员').length, 0);
});

test('without a role prop the page resolves canManageOrg from auth/me memberships', async () => {
  // admin membership (default fixture): buttons stay enabled…
  const admin = clientWith([ownerOrg]);
  const adminRoot = await mountPage(admin.client);
  assert.equal(buttonWithLabel(adminRoot, '创建共享空间').disabled, false);

  // …viewer membership disables them, mirroring Vue hasRole('admin') === false.
  const viewer = clientWith([ownerOrg], { role: 'viewer' });
  const viewerRoot = await mountPage(viewer.client);
  assert.equal(buttonWithLabel(viewerRoot, '创建共享空间').disabled, true);

  // …cross-tenant superuser (can_access_all_tenants) passes like Vue canAccessAllTenants.
  const superuser = clientWith([ownerOrg], { role: 'viewer', canAccessAllTenants: true });
  const superuserRoot = await mountPage(superuser.client);
  assert.equal(buttonWithLabel(superuserRoot, '创建共享空间').disabled, false);
});

// ─── R017 shell sub-filter: ?scope= deep link + URL sync ──────────────────
// Same ?scope= convention as the KB list (App.tsx read/writeScopeToUrl):
// values all|created|joined, 'all' removes the param.

test('?scope=created deep-link selects the 我创建的 rail and lists only owned spaces', async () => {
  window.history.replaceState({}, '', '/platform/organizations?scope=created');
  const { client } = clientWith([ownerOrg, joinedOrg]);
  const root = await mountPage(client);
  const cards = orgCards(root);
  assert.equal(cards.length, 1, 'only owned spaces render under ?scope=created');
  assert.match(cards[0]?.textContent ?? '', /parity-org/);
  const activeRail = root.querySelector('aside button.is-active');
  assert.ok(activeRail, 'expected an active rail entry');
  assert.match(activeRail?.textContent ?? '', /我创建的/);
});

test('?scope=joined deep-link selects the 我加入的 rail and lists only joined spaces', async () => {
  window.history.replaceState({}, '', '/platform/organizations?scope=joined');
  const { client } = clientWith([ownerOrg, joinedOrg]);
  const root = await mountPage(client);
  const cards = orgCards(root);
  assert.equal(cards.length, 1);
  assert.match(cards[0]?.textContent ?? '', /joined-org/);
  const activeRail = root.querySelector('aside button.is-active');
  assert.match(activeRail?.textContent ?? '', /我加入的/);
});

test('rail clicks sync ?scope= (created/joined set it, all removes it)', async () => {
  const { client } = clientWith([ownerOrg, joinedOrg]);
  const root = await mountPage(client);
  const railButtons = [...root.querySelectorAll('aside button')] as HTMLButtonElement[];
  assert.equal(railButtons.length, 3, 'rail mirrors Vue ListSpaceSidebar: all/created/joined');
  const railFor = (label: string) => railButtons.find((button) => button.textContent?.includes(label));
  assert.ok(railFor('全部') && railFor('我创建的') && railFor('我加入的'), 'rail labels match Vue entries');

  await click(railFor('我加入的')!);
  assert.equal(window.location.search, '?scope=joined', 'joined writes ?scope=joined');
  await click(railFor('我创建的')!);
  assert.equal(window.location.search, '?scope=created', 'created writes ?scope=created');
  await click(railFor('全部')!);
  assert.equal(window.location.search, '', 'all removes the ?scope param (KB convention)');

  // Vue ListSpaceSidebar tooltipText: collapsed-strip tooltips carry counts.
  assert.equal(railFor('全部')!.getAttribute('title'), '全部 (2)');
  assert.equal(railFor('我创建的')!.getAttribute('title'), '我创建的 (1)');
  assert.equal(railFor('我加入的')!.getAttribute('title'), '我加入的 (1)');
});
