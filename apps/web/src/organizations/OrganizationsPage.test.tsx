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
/* tdesign 运行时依赖的 DOM 构造器全局补齐（pilot agents / kb-list 同款）——
 * Popup/Dialog 等组件在 jsdom 下直接读 Element/SVGElement/MutationObserver。 */
Object.defineProperty(dom.window.navigator, 'language', { configurable: true, value: 'zh-CN' });
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  SVGElement: dom.window.SVGElement,
  DocumentFragment: dom.window.DocumentFragment,
  Event: dom.window.Event,
  KeyboardEvent: dom.window.KeyboardEvent,
  MouseEvent: dom.window.MouseEvent,
  MutationObserver: dom.window.MutationObserver,
  IS_REACT_ACT_ENVIRONMENT: true,
  getComputedStyle: dom.window.getComputedStyle?.bind(dom.window),
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? clearTimeout,
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

interface Calls { create: unknown[]; preview: string[]; join: unknown[]; submitJoinRequest: unknown[]; updateRole: unknown[][]; review: unknown[][]; leave: string[]; remove: string[]; update: unknown[][]; membersList: string[]; joinRequestsList: string[]; kbSharesList: string[]; agentSharesList: string[]; inviteCode: string[]; get: string[]; upgrade: unknown[] }

// R017: auth/me drives the page's self-resolved canManageOrg when no explicit
// role prop is passed. The default mirrors an admin home-tenant membership so
// legacy gating-free expectations stay valid; RBAC tests override it.
export interface MeOverride { role: string; canAccessAllTenants?: boolean }

function clientWith(organizations: Organization[], me_?: MeOverride, detailFor?: (id: string) => Organization | undefined): { client: WeKnoraClient; calls: Calls } {
  const calls: Calls = { create: [], preview: [], join: [], submitJoinRequest: [], updateRole: [], review: [], leave: [], remove: [], update: [], membersList: [], joinRequestsList: [], kbSharesList: [], agentSharesList: [], inviteCode: [], get: [], upgrade: [] };
  const organizationsApi = {
    list: async () => ({ items: organizations, total: organizations.length }),
    get: async (id: string) => {
      calls.get.push(id);
      const detail = detailFor?.(id);
      if (detail) return detail;
      return organizations.find((item) => item.id === id) ?? organizations[0];
    },
    create: async (input: unknown) => { calls.create.push(input); return { ...ownerOrg, ...(input as Record<string, unknown>) }; },
    update: async (id: string, input: unknown) => { calls.update.push([id, input]); return organizations[0]; },
    remove: async (id: string) => { calls.remove.push(id); },
    join: async (input: unknown) => { calls.join.push(input); return joinedOrg; },
    submitJoinRequest: async (input: unknown) => { calls.submitJoinRequest.push(input); },
    preview: async (code: string) => { calls.preview.push(code); return { id: 'org-9', name: 'previewed-org', description: '', member_count: 4, share_count: 0, agent_share_count: 0, is_already_member: false, require_approval: true }; },
    search: async () => ({ items: [], total: 0 }),
    joinById: async () => joinedOrg,
    leave: async (id: string) => { calls.leave.push(id); },
    requestRoleUpgrade: async (id: string, input: unknown) => { calls.upgrade.push([id, input]); return {}; },
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
  /* tdesign Popup destroyOnClose 的离场动画定时器在卸载后仍会触发一次
   * body 容器移除——先冲刷这些定时器再清 body，避免 DOMException 逸散到
   * 后续用例的 act 聚合错误里。 */
  await new Promise((resolve) => setTimeout(resolve, 260));
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

/* 迁移后（Task 11b）header 双按钮是 Vue 纯图标 t-button（无 aria/title，
 * OrganizationList.vue:11-23）——create 按钮以 .org-create-icon 图标锚定。 */
function headerActionButton(root: ParentNode, kind: 'join' | 'create'): HTMLElement {
  const buttons = [...root.querySelectorAll('.header-actions .header-action-btn')];
  const button = kind === 'create' ? buttons.find((el) => el.querySelector('.org-create-icon')) : buttons.find((el) => !el.querySelector('.org-create-icon'));
  assert.ok(button, 'expected header ' + kind + ' button (.header-action-btn)');
  return button as HTMLElement;
}

/* 设置弹窗经 createPortal 挂 body（Vue Teleport 同构）。 */
function settingsDialog(): HTMLElement {
  const dialog = document.body.querySelector('.settings-overlay');
  assert.ok(dialog, 'expected the portaled settings modal (.settings-overlay)');
  return dialog as HTMLElement;
}

/* 台账 #7：tdesign-react disabled Button 根标签切到 <div class="t-is-disabled">，
 * disabled 断言走 classList（kb-list 同款）。 */
function isDisabledControl(el: Element): boolean {
  return (el as HTMLButtonElement).disabled === true || el.classList.contains('t-is-disabled');
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

function textButtons(root: ParentNode, text: string): HTMLElement[] {
  /* tdesign Button disabled 时根标签为 div.t-button（台账 #7）——两类都收。 */
  return [...root.querySelectorAll('button, .t-button')].filter((el): el is HTMLElement => el instanceof HTMLElement && el.textContent === text);
}

// Vue-parity selectors (OrganizationList.vue): cards are .org-card tiles
// (skeleton variants excluded); the section headers are .org-section-header.
function orgCards(root: HTMLElement): HTMLElement[] {
  return [...root.querySelectorAll('.org-card')].filter((el) => !el.classList.contains('org-card-skeleton')) as HTMLElement[];
}

test('zh-CN page renders Vue-parity anatomy and drops the English debug page', async () => {
  const { client } = clientWith([ownerOrg, joinedOrg]);
  const root = await mountPage(client);

  const heading = root.querySelector('h2');
  assert.ok(heading, 'expected an h2 page title');
  assert.equal(heading.textContent, '共享空间');
  assert.match(root.textContent ?? '', /创建或加入共享空间，让多个空间互相协作/);

  headerActionButton(root, 'join');
  headerActionButton(root, 'create');

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
  const statBadges = [...root.querySelectorAll('.feature-badge.stat-member, .feature-badge.stat-kb, .feature-badge.stat-agent')];
  assert.ok(statBadges.length >= 6, 'expected member/kb/agent stat badges on every card');
  assert.equal(root.querySelectorAll('.relation-role-tag').length, 1, 'all view hides the owner card tag (section header covers it, card-list-badge.ts)');

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
  await click(headerActionButton(root, 'create'));

  const dialog = settingsDialog();
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
  await click(headerActionButton(root, 'create'));

  const dialog = settingsDialog();
  const form = dialog.querySelector('form');
  assert.ok(form, 'expected create form');
  await submitForm(form as HTMLFormElement);

  assert.equal(calls.create.length, 0, 'blank names must not call the create API');
  const validationToast = root.querySelector('[role="status"]');
  assert.ok(validationToast, 'Vue shows a validation warning through MessagePlugin');
  assert.match(validationToast?.textContent ?? '', /请输入共享空间名称/);
  assert.match(validationToast?.className ?? '', /wk-org-130/, 'validation should use warning styling');
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
  await click(headerActionButton(root, 'create'));

  const dialog = settingsDialog();
  // Vue 几何由 orgs.td.css §6 .settings-modal 承载（90vw/1100px/85vh/750px）。
  const modal = dialog.querySelector('.settings-modal');
  assert.ok(modal, 'Vue .settings-modal shell renders');
  assert.ok(dialog.querySelector('.settings-sidebar .sidebar-title'), 'the 208px sidebar rail renders');

  const createButtons = textButtons(dialog, '创建');
  assert.equal(createButtons.length, 1, 'expected one primary create action');
  assert.ok(createButtons[0]?.closest('.settings-footer'), 'primary action belongs to the .settings-footer');
});

// R484 G4 D6 (R482 report-B3.md D6): the create modal must mirror the Vue
// OrganizationSettingsModal create mode — nav carries the 「基础」 group title
// (organization.navGroups.basic, OrganizationSettingsModal.vue navGroups
// lines 1028-1040), the footer confirm reads common.create (「创建」, line 809)
// and the description textarea shows the TDesign maxlength counter 0/500
// (line 102 :maxlength="500").
test('create modal keeps the Vue nav group title, common.create submit and 0/500 counter', async () => {
  const { client } = clientWith([ownerOrg]);
  const root = await mountPage(client);
  await click(headerActionButton(root, 'create'));

  const dialog = settingsDialog();

  // Nav: the 「基础」 group title precedes the 基本信息 nav item like Vue.
  const nav = dialog.querySelector('.settings-nav');
  assert.ok(nav, 'expected the modal sidebar nav');
  const navTexts = [...nav.querySelectorAll(':scope > *')].map((node) => node.textContent ?? '');
  const groupIndex = navTexts.findIndex((text) => text.trim() === '基础');
  const basicIndex = navTexts.findIndex((text) => text.trim() === '基本信息');
  assert.ok(groupIndex >= 0, 'the Vue nav group title 基础 renders (organization.navGroups.basic)');
  assert.ok(basicIndex > groupIndex, '基本信息 follows the 基础 group title');
  assert.ok(navTexts.some((text) => text.trim() === '权限说明'), 'the permissions nav item renders');

  // Footer confirm reads common.create (创建), not the modal title.
  const footerButtons = [...dialog.querySelectorAll('.settings-footer button')].map((button) => button.textContent ?? '');
  assert.ok(footerButtons.includes('取消'), 'the footer cancel renders');
  assert.ok(footerButtons.includes('创建'), 'the footer confirm reads common.create (创建)');
  assert.equal(footerButtons.filter((label) => label === '创建共享空间').length, 0, 'the footer must not reuse the modal title');

  // Description textarea: Vue t-textarea maxlength counter 0/500.
  const description = dialog.querySelector('textarea[name="organization-description"]') as HTMLTextAreaElement | null;
  assert.ok(description, 'expected the description textarea');
  assert.match(dialog.textContent ?? '', /0\/500/, 'the TDesign t-textarea __limit counter renders (台账 #12)');
});

test('join modal previews an invite code and submits an approval-gated request', async () => {
  const { client, calls } = clientWith([ownerOrg]);
  const root = await mountPage(client);
  await click(headerActionButton(root, 'join'));

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
  const roleSelect = dialogAfterPreview.querySelector('.t-select__wrap');
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

  const dialog = settingsDialog();
  assert.ok(dialog, 'expected settings modal after card click');
  assert.match(dialog.textContent ?? '', /共享空间设置/);
  assert.deepEqual(calls.membersList, ['org-1']);
  assert.deepEqual(calls.joinRequestsList, ['org-1']);
  assert.deepEqual(calls.agentSharesList, ['org-1']);

  await act(async () => {});
  const dialogReady = settingsDialog();
  // R487 K1: Vue labels the members nav entry with t('organization.manageMembers')
  // (成员管理, OrganizationSettingsModal.vue:1002); 共享空间成员 stays the inner
  // list title (Vue :345).
  const membersNav = [...dialogReady.querySelectorAll('.nav-item')].find((item): item is HTMLElement => item instanceof HTMLElement && item.textContent === '成员管理');
  assert.ok(membersNav, 'expected members nav item');
  await click(membersNav);
  await act(async () => {});
  const membersPanel = settingsDialog();
  assert.match(membersPanel.textContent ?? '', /Alice/);

  // R487 K1: the pending join-request count badges the nav entry (Vue L30-33),
  // so the label match tolerates the trailing badge digit.
  const requestsNav = [...membersPanel.querySelectorAll('.nav-item')].find((item): item is HTMLElement => item instanceof HTMLElement && (item.textContent ?? '').startsWith('加入申请'));
  assert.ok(requestsNav, 'expected join-requests nav item labelled 加入申请 like the Vue modal (OrganizationSettingsModal.vue:1008)');
  await click(requestsNav);
  await act(async () => {});
  const requestsPanel = settingsDialog();
  assert.match(requestsPanel.textContent ?? '', /加入申请/, 'section heading uses the Vue 加入申请 wording');
  assert.match(requestsPanel.textContent ?? '', /待审核申请/, 'inner list title keeps the Vue 待审核申请 wording');
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
  const dialog = settingsDialog();
  const membersNav = [...dialog.querySelectorAll('.nav-item')].find((item): item is HTMLElement => item instanceof HTMLElement && item.textContent === '成员管理');
  assert.ok(membersNav);
  await click(membersNav);
  await act(async () => {});
  // R488 D-B5: Vue renders the owner row with a static role tag (管理员) plus
  // the 创建者 badge (OrganizationSettingsModal.vue:466-481) — no role select,
  // no remove affordance on the owner's own row.
  const ownerRow = [...dialog.querySelectorAll('tbody tr')].find((row) => (row.textContent ?? '').includes('Owner')) as HTMLElement | undefined;
  assert.ok(ownerRow, 'owner member row renders');
  assert.match(ownerRow.textContent ?? '', /创建者/, 'owner row carries the 创建者 badge (Vue organization.owner)');
  assert.equal(ownerRow.querySelectorAll('select').length, 0, 'owner role cell is a static tag, not a select (Vue t-tag branch)');
  assert.match(ownerRow.textContent ?? '', /管理员/, 'owner role renders the static 管理员 tag');
  assert.equal(textButtons(dialog, '移除').length, 0);
  // R487 K1: shared-resources nav badges always carry the totals (Vue
  // OrganizationSettingsModal.vue:30-33), so match on the label prefix.
  const agentsNav = [...dialog.querySelectorAll('.nav-item')].find((item): item is HTMLElement => item instanceof HTMLElement && (item.textContent ?? '').startsWith('共享智能体'));
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

  const dialog = settingsDialog();
  const membersNav = [...dialog.querySelectorAll('.nav-item')].find((item): item is HTMLElement => item instanceof HTMLElement && item.textContent === '成员管理');
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

// R488 D-B5 — Vue renders the members list as a table (memberColumns, Vue
// OrganizationSettingsModal.vue:1124-1134): 成员/角色/加入时间/操作 headers, a
// joined-date cell, and badges on the member cell instead of the old
// headerless row cards.
test('members list renders the Vue table anatomy with joined dates and owner badges', async () => {
  const { client } = clientWith([ownerOrg]);
  client.identity.organizations.members.list = async () => ({
    items: [
      { id: 'member-owner', user_id: 'u1', username: 'Alice', email: 'alice@example.dev', role: 'admin', tenant_id: 1, tenant_name: 'Alice Workspace', joined_at: '2030-03-05T00:00:00' },
      { id: 'member-bob', user_id: 'u2', username: 'Bob', email: 'bob@example.dev', role: 'viewer', tenant_id: 2, tenant_name: 'Bob Workspace', joined_at: '2030-03-06T00:00:00' },
    ],
    total: 2,
  });
  const root = await mountPage(client);
  await click(orgCards(root)[0] as HTMLElement);
  await act(async () => {});
  const dialog = settingsDialog();
  await click([...dialog.querySelectorAll('.nav-item')].find((item) => item.textContent === '成员管理') as HTMLElement);
  await act(async () => {});

  const table = dialog.querySelector('table') as HTMLTableElement | null;
  assert.ok(table, 'members list renders as a table (Vue members-table-shell)');
  const headerTexts = [...table.querySelectorAll('thead th')].map((cell) => (cell.textContent ?? '').trim());
  assert.deepEqual(headerTexts, ['成员', '角色', '加入时间', '操作'], 'table headers mirror Vue memberColumns (member/role/joinedAt/operations)');

  const rows = [...table.querySelectorAll('tbody tr')] as HTMLTableRowElement[];
  assert.equal(rows.length, 2);
  // Owner row (tenant 1 = owner_tenant_id): workspace-name primary label +
  // 创建者 + 我 badges + static 管理员 tag + joined date + empty actions cell.
  const ownerRow = rows.find((row) => (row.textContent ?? '').includes('Alice Workspace')) as HTMLTableRowElement;
  assert.ok(ownerRow, 'member primary label is the workspace name (Vue memberPrimaryLabel)');
  assert.match(ownerRow.textContent ?? '', /创建者/, 'owner row shows the 创建者 badge (Vue owner-tag)');
  assert.match(ownerRow.textContent ?? '', /我/, 'own row shows the 我 badge (Vue me-tag, authStore.currentUserId)');
  assert.match(ownerRow.textContent ?? '', /Alice/, 'representative username renders as the secondary label');
  assert.match(ownerRow.textContent ?? '', /2030-03-05/, 'joined date renders in the Vue formatDate YYYY-MM-DD form');
  assert.equal(ownerRow.querySelectorAll('select').length, 0, 'owner role cell is static (Vue t-tag branch)');
  assert.equal((ownerRow.querySelector('td:last-child') as HTMLTableCellElement).textContent, '', 'owner actions cell stays empty (Vue :486 popconfirm skips owner)');
  // Non-owner row: role select enabled + remove affordance in the actions cell.
  const bobRow = rows.find((row) => (row.textContent ?? '').includes('Bob Workspace')) as HTMLTableRowElement;
  assert.ok(bobRow);
  assert.match(bobRow.textContent ?? '', /2030-03-06/, 'non-owner row carries its joined date too');
  // 台账 #8：tdesign Select 根不透传 aria-*；语义类钩子 org-member-role-sel 定位。
  const bobSelect = bobRow.querySelector('.org-member-role-sel') as HTMLElement | null;
  assert.ok(bobSelect, 'non-owner role cell keeps the change-role select (Vue :476-478)');
  assert.equal(bobSelect.classList.contains('t-is-disabled'), false, 'change-role select stays enabled (台账 #7 classList 断言)');
  assert.equal(textButtons(bobRow, '移除').length, 1, 'non-owner actions cell keeps the remove affordance');
});

// R490 C1 — the app router always passes role={scopeRuntime.role()} (never
// null), which used to short-circuit the auth/me fetch that populates
// currentUserId; production then hid the「我」badge while role-less test
// mounts kept seeing it. The badge must render on the role-prop path too.
test('owner row keeps the 我 badge when the router-supplied role prop is set', async () => {
  const { client } = clientWith([ownerOrg]);
  client.identity.organizations.members.list = async () => ({
    items: [
      { id: 'member-owner', user_id: 'u1', username: 'Alice', email: 'alice@example.dev', role: 'admin', tenant_id: 1, tenant_name: 'Alice Workspace', joined_at: '2030-03-05T00:00:00' },
    ],
    total: 1,
  });
  const root = await mountPage(client, undefined, 'owner');
  await click(orgCards(root)[0] as HTMLElement);
  await act(async () => {});
  const dialog = settingsDialog();
  await click([...dialog.querySelectorAll('.nav-item')].find((item) => item.textContent === '成员管理') as HTMLElement);
  await act(async () => {});
  const table = dialog.querySelector('table') as HTMLTableElement | null;
  assert.ok(table);
  const ownerRow = [...table.querySelectorAll('tbody tr')].find((row) => (row.textContent ?? '').includes('Alice Workspace')) as HTMLTableRowElement;
  assert.ok(ownerRow);
  assert.match(ownerRow.textContent ?? '', /创建者/, 'owner row shows the 创建者 badge even with the role prop');
  assert.match(ownerRow.textContent ?? '', /我/, 'own row shows the 我 badge on the role-prop path (production router mount)');
});

// R488 D-B5 — Vue has NO resident add-member form: the invite entry is an
// admin-only icon button that opens a popup (Vue :408-446). The resident
// 「添加成员」 form block must be gone.
test('add-member entry is a popup behind the icon button, not a resident form', async () => {
  const { client, calls } = clientWith([ownerOrg]);
  const inviteCalls: unknown[] = [];
  const searches: string[] = [];
  client.identity.organizations.searchTenantsForInvite = async (_id: string, query: string) => {
    searches.push(query);
    return [{ tenant_id: 42, tenant_name: 'Team Workspace', representative_username: 'rep-user', representative_user_id: 'ru-9', representative_email: 'rep@x.dev' }];
  };
  (client.identity.organizations as unknown as { inviteMember: (id: string, input: unknown) => Promise<void> }).inviteMember = async (id: string, input: unknown) => {
    inviteCalls.push([id, input]);
  };
  const root = await mountPage(client);
  await click(orgCards(root)[0] as HTMLElement);
  await act(async () => {});
  const dialog = settingsDialog();
  await click([...dialog.querySelectorAll('.nav-item')].find((item) => item.textContent === '成员管理') as HTMLElement);
  await act(async () => {});

  // The popup trigger: a small icon button titled 添加成员 (Vue members-list-add-btn).
  const trigger = [...dialog.querySelectorAll('button')].find((button) => button.getAttribute('aria-label') === '添加成员') as HTMLButtonElement | undefined;
  assert.ok(trigger, 'the add-member icon button renders for managing admins (Vue :410-414)');
  // Closed state: no dialog title / tenant picker / role picker in the panel.
  assert.doesNotMatch(dialog.textContent ?? '', /选择空间/, 'no resident add-member form: the tenant picker only exists inside the popup');
  assert.doesNotMatch(dialog.textContent ?? '', /分配角色/, 'the role picker only exists inside the popup');

  await click(trigger);
  await act(async () => {});
  assert.match(dialog.textContent ?? '', /添加成员/, 'popup opens with the Vue dialogTitle');
  assert.match(dialog.textContent ?? '', /共享空间的成员单位是空间/, 'popup carries the Vue tipTenant line');
  assert.match(dialog.textContent ?? '', /输入至少 2 个字符开始搜索/, 'popup carries the Vue searchTenantHint');
  const confirmBtn = [...dialog.querySelectorAll('button')].find((button) => button.textContent === '添加') as HTMLButtonElement | undefined;
  assert.ok(confirmBtn, 'popup footer carries the Vue confirmBtn 添加');
  assert.equal(confirmBtn.disabled, true, 'confirm stays disabled until a workspace is selected (Vue :selectedTenantId == null)');

  // Search → candidate appears → select it → confirm becomes enabled.
  // 台账 #8 衍生：tdesign Input 的 aria-label 落 wrapper，内层 input 才是可输入节点。
  const tenantSearch = dialog.querySelector('.t-input__wrap[aria-label="选择空间"] input') as HTMLInputElement | null;
  assert.ok(tenantSearch, 'popup exposes the tenant search input (Vue searchTenant)');
  await setInputValueAsync(tenantSearch, 'team');
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  assert.deepEqual(searches, ['team'], 'typing routes through searchTenantsForInvite');
  const candidate = [...dialog.querySelectorAll('button')].find((button) => (button.textContent ?? '').includes('Team Workspace')) as HTMLElement | undefined;
  assert.ok(candidate, 'the deduped workspace candidate renders');
  await click(candidate);
  await act(async () => {});
  assert.equal(confirmBtn.disabled, false, 'selecting a candidate enables the confirm button');

  await click(confirmBtn);
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  assert.equal(inviteCalls.length, 1, 'confirm calls inviteMember');
  assert.deepEqual((inviteCalls[0] as unknown[])[1], { tenant_id: 42, representative_user_id: 'ru-9', role: 'viewer' });
  assert.deepEqual(calls.membersList, ['org-1', 'org-1'], 'the member list refetches after adding (Vue fetchMembers)');
  assert.doesNotMatch(dialog.textContent ?? '', /Team Workspace/, 'popup closes and resets on success (Vue resetAddMemberDialog)');
});

test('settings modal exposes an equivalent section selector when the sidebar is hidden on mobile', async () => {
  const { client } = clientWith([ownerOrg]);
  const root = await mountPage(client);
  await click(orgCards(root)[0] as HTMLElement);
  await act(async () => {});

  const dialog = settingsDialog();
  // 台账 #8：tdesign Select 根丢弃 data-*/aria-*，语义类钩子定位；选项列表
  // 只在弹层打开后进入 DOM（closed select 不泄露选项，同 validity 判例）。
  const sectionSelector = dialog.querySelector('.organization-settings-section-selector') as HTMLElement | null;
  assert.ok(sectionSelector, 'expected mobile section selector');
  assert.match(sectionSelector.parentElement?.className ?? '', /wk-org-5/, 'selector should be available at the mobile breakpoint (.wk-org-5 carries the max-width:720px display:block rule)');
  // R487 K1: the standalone invite nav item is gone (Vue embeds the invite
  // affordances in the basic 邀请成员 card) and the members entry reads
  // organization.manageMembers (成员管理).
  const selectorValue = () => (sectionSelector.querySelector('input') as HTMLInputElement | null)?.value ?? '';
  assert.equal(selectorValue(), '基本信息', 'selector starts on the basic section');

  await click(sectionSelector.querySelector('.t-input') as HTMLElement);
  await act(async () => {});
  const optionTexts = [...document.body.querySelectorAll('.t-select-option')].map((node) => (node.textContent ?? '').trim());
  assert.deepEqual(optionTexts, [
    '基本信息',
    '成员管理',
    '加入申请',
    '共享知识库',
    '共享智能体',
  ]);
  const membersOption = [...document.body.querySelectorAll('.t-select-option')].find((node) => (node.textContent ?? '').trim() === '成员管理') as HTMLElement;
  assert.ok(membersOption, 'members option renders in the select popup');
  await click(membersOption);
  await act(async () => {});
  assert.equal(selectorValue(), '成员管理', 'selector reflects the chosen section');
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
  const more = (joinedCard as HTMLElement).querySelector('.more-wrap');
  assert.ok(more);
  await click(more as HTMLElement);
  await act(async () => {});
  const menu = document.body.querySelector('.card-more-popup .popup-menu');
  assert.ok(menu, 't-popup portals the card menu to body (card-more-popup)');
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
  const more = (card as HTMLElement).querySelector('.more-wrap');
  assert.ok(more);
  await click(more as HTMLElement);
  await act(async () => {});
  const menu = [...document.body.querySelectorAll('.card-more-popup .popup-menu')].pop();
  assert.ok(menu, 'expected popup menu for ' + name);
  return menu as HTMLElement;
}

test('viewer/contributor roles disable create and join with the rbac tip and hide delete', async () => {
  for (const role of ['viewer', 'contributor'] as const) {
    const { client } = clientWith([ownerOrg, joinedOrg]);
    const root = await mountPage(client, undefined, role);

    const joinButton = headerActionButton(root, 'join');
    const createButton = headerActionButton(root, 'create');
    assert.equal(isDisabledControl(joinButton), true, role + ' join header button must be disabled');
    assert.equal(isDisabledControl(createButton), true, role + ' create header button must be disabled');

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
    assert.equal(isDisabledControl(headerActionButton(root, 'join')), false, role + ' join must stay enabled');
    assert.equal(isDisabledControl(headerActionButton(root, 'create')), false, role + ' create must stay enabled');
    const menu = await openCardMenu(root, 'parity-org');
    assert.match(menu.textContent ?? '', /删除/);
  }
});

test('empty-state join/create actions are disabled for a viewer', async () => {
  const { client } = clientWith([]);
  const root = await mountPage(client, undefined, 'viewer');
  const actions = [...textButtons(root, '加入共享空间'), ...textButtons(root, '创建共享空间')];
  assert.equal(actions.length, 2);
  assert.equal(isDisabledControl(actions[0]), true);
  assert.equal(isDisabledControl(actions[1]), true);
});

test('viewer cannot edit an owned space when tenant role is below admin', async () => {
  const { client, calls } = clientWith([ownerOrg]);
  const root = await mountPage(client, undefined, 'viewer');

  await click(orgCards(root)[0] as HTMLElement);
  await act(async () => {});

  const dialog = settingsDialog();
  assert.match(dialog.textContent ?? '', /此操作需要当前空间的 admin 或更高角色/);
  assert.equal((dialog.querySelector('input[name="organization-name"]') as HTMLInputElement).disabled, true);
  assert.equal((dialog.querySelector('textarea[name="organization-description"]') as HTMLTextAreaElement).disabled, true);
  assert.equal(textButtons(dialog, '保存').length, 0, 'read-only settings must not expose save');
  assert.equal(calls.update.length, 0);

  const membersNav = [...dialog.querySelectorAll('.nav-item')].find((item): item is HTMLElement => item instanceof HTMLElement && item.textContent === '成员管理');
  assert.ok(membersNav);
  await click(membersNav);
  // R488 D-B5: Vue only offers the role select to admins on non-owner rows
  // (v-if="isAdmin && !isOwnerMember(row)"); anyone else reads a static tag.
  assert.equal(dialog.querySelector('select[aria-label="角色"]'), null, 'non-managing viewers read a static role tag, no select');
  assert.match(dialog.textContent ?? '', /管理员/, 'the role still renders as the static tag text');
  assert.equal(textButtons(dialog, '移除').length, 0);

  const requestsNav = [...dialog.querySelectorAll('.nav-item')].find((item) => item.textContent === '加入申请');
  assert.equal(requestsNav, undefined, 'Vue hides join-request navigation from non-admin organization members');

  // R487 K1: the invite affordances live inside the basic 邀请成员 card behind
  // the admin gate (Vue v-if="isAdmin && orgId") — a non-managing viewer sees
  // neither the nav item nor any invite control.
  const inviteNav = [...dialog.querySelectorAll('.nav-item')].find((item) => (item.textContent ?? '').includes('邀请链接'));
  assert.equal(inviteNav, undefined, 'the standalone 邀请链接 nav item must not render');
  const basicNav = [...dialog.querySelectorAll('.nav-item')].find((item): item is HTMLElement => item instanceof HTMLElement && item.textContent === '基本信息');
  assert.ok(basicNav);
  await click(basicNav);
  assert.equal([...dialog.querySelectorAll('button')].some((button) => button.getAttribute('aria-label') === '刷新邀请码'), false, 'invite-code refresh is admin-only');
  assert.equal(dialog.querySelector('[role="switch"][aria-label="需要审核"]'), null, 'approval switch is admin-only');
  assert.equal(dialog.querySelector('input[aria-label="成员数量上限"]'), null, 'member-limit input is admin-only');
});

test('without a role prop the page resolves canManageOrg from auth/me memberships', async () => {
  // admin membership (default fixture): buttons stay enabled…
  const admin = clientWith([ownerOrg]);
  const adminRoot = await mountPage(admin.client);
  assert.equal(isDisabledControl(headerActionButton(adminRoot, 'create')), false);

  // …viewer membership disables them, mirroring Vue hasRole('admin') === false.
  const viewer = clientWith([ownerOrg], { role: 'viewer' });
  const viewerRoot = await mountPage(viewer.client);
  assert.equal(isDisabledControl(headerActionButton(viewerRoot, 'create')), true);

  // …cross-tenant superuser (can_access_all_tenants) passes like Vue canAccessAllTenants.
  const superuser = clientWith([ownerOrg], { role: 'viewer', canAccessAllTenants: true });
  const superuserRoot = await mountPage(superuser.client);
  assert.equal(isDisabledControl(headerActionButton(superuserRoot, 'create')), false);
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
  const activeRail = root.querySelector('.icon-item-labeled.active');
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
  const activeRail = root.querySelector('.icon-item-labeled.active');
  assert.match(activeRail?.textContent ?? '', /我加入的/);
});

test('rail clicks sync ?scope= (created/joined set it, all removes it)', async () => {
  const { client } = clientWith([ownerOrg, joinedOrg]);
  const root = await mountPage(client);
  const railButtons = [...root.querySelectorAll('.icon-item-labeled')] as HTMLElement[];
  assert.equal(railButtons.length, 3, 'rail mirrors Vue ListSpaceSidebar: all/created/joined');
  const railFor = (label: string) => railButtons.find((item) => item.textContent?.includes(label));
  assert.ok(railFor('全部') && railFor('我创建的') && railFor('我加入的'), 'rail labels match Vue entries');

  await click(railFor('我加入的')!);
  assert.equal(window.location.search, '?scope=joined', 'joined writes ?scope=joined');
  await click(railFor('我创建的')!);
  assert.equal(window.location.search, '?scope=created', 'created writes ?scope=created');
  await click(railFor('全部')!);
  assert.equal(window.location.search, '', 'all removes the ?scope param (KB convention)');

  // Vue ListSpaceSidebar tooltipText: counts ride the t-tooltip content prop
  // (rendered on hover, not as a title attribute — Vue 同构后无 title 属性)。
  assert.equal(railFor('全部')!.querySelector('.icon-label')?.textContent, '全部');
  assert.equal(railFor('我创建的')!.querySelector('.icon-label')?.textContent, '我创建的');
  assert.equal(railFor('我加入的')!.getAttribute('data-space-key'), 'joined');
});

// R435-A3: Vue OrganizationSettingsModal.vue gates the role-upgrade entry with
// canRequestUpgrade (edit mode + space role below admin + tenant admin+) and
// narrows upgradeRoleOptions to roles above the current one.
test('role-upgrade entry follows the Vue canRequestUpgrade/upgradeRoleOptions contract', async () => {
  const { canRequestUpgradeForOrg, upgradeRoleOptionsForRole } = await import('./OrganizationsPage.tsx');

  // Create mode never offers an upgrade request.
  assert.equal(canRequestUpgradeForOrg({ mode: 'create', myRole: 'viewer', tenantAdmin: true }), false);
  // A missing space role cannot request an upgrade.
  assert.equal(canRequestUpgradeForOrg({ mode: 'edit', myRole: '', tenantAdmin: true }), false);
  // Admins (and owners) already hold the top space role.
  assert.equal(canRequestUpgradeForOrg({ mode: 'edit', myRole: 'admin', tenantAdmin: true }), false);
  // Below tenant admin the modal shows the read-only tenant-role hint instead.
  assert.equal(canRequestUpgradeForOrg({ mode: 'edit', myRole: 'editor', tenantAdmin: false }), false);
  assert.equal(canRequestUpgradeForOrg({ mode: 'edit', myRole: 'viewer', tenantAdmin: true }), true);
  assert.equal(canRequestUpgradeForOrg({ mode: 'edit', myRole: 'editor', tenantAdmin: true }), true);

  // Vue upgradeRoleOptions: only roles above the current space role.
  assert.deepEqual(upgradeRoleOptionsForRole('viewer'), ['editor', 'admin']);
  assert.deepEqual(upgradeRoleOptionsForRole('editor'), ['admin']);
  assert.deepEqual(upgradeRoleOptionsForRole('admin'), []);
  assert.deepEqual(upgradeRoleOptionsForRole(''), []);
});

// R436-A4: Vue OrganizationSettingsModal.vue sources hasPendingUpgrade from
// the org detail endpoint (fetchOrgDetail → has_pending_upgrade), disables the
// upgrade entry while a request is pending (title/aria swap to
// organization.upgrade.pending), shows the current-role bar
// (organization.upgrade.currentRole + organization.role.{my_role}), and marks
// the flag locally right after a successful submit.
test('upgrade form reflects has_pending_upgrade from the org detail endpoint', async () => {
  const { client, calls } = clientWith([ownerOrg, joinedOrg], undefined, (id) => (id === joinedOrg.id ? { ...joinedOrg, has_pending_upgrade: true } : undefined));
  const root = await mountPage(client);

  const cards = orgCards(root);
  await click(cards[1]!);
  // Flush the modal's org-detail fetch (Vue fetchOrgDetail parity).
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });

  const dialog = settingsDialog();
  assert.ok(calls.get.includes(joinedOrg.id), 'expected the org detail endpoint to back the upgrade gate');

  // R487 K1: Vue keeps the upgrade entry inside the members section (popup on
  // the members-list header, OrganizationSettingsModal.vue:357), not in basic.
  const membersNav = [...dialog.querySelectorAll('.nav-item')].find((item): item is HTMLElement => item instanceof HTMLElement && item.textContent === '成员管理');
  assert.ok(membersNav, 'expected the members nav to reach the upgrade entry');
  await click(membersNav);
  await act(async () => {});

  // Submit is disabled while a request is pending; the affordance explains why
  // (Vue swaps title/aria to organization.upgrade.pending = 审核中).
  const submit = textButtons(dialog, '提交申请')[0];
  assert.ok(submit, 'expected the upgrade submit button');
  assert.equal(isDisabledControl(submit), true);
  assert.equal(submit.getAttribute('title'), '审核中');
  assert.equal(submit.getAttribute('aria-label'), '审核中');

  // Vue upgrade-current-role-bar: 当前角色 label + role tag of my_role (编辑).
  assert.match(dialog.textContent ?? '', /当前角色/);
  assert.match(dialog.textContent ?? '', /编辑/);
});

test('a successful upgrade request marks the org pending and disables resubmission', async () => {
  const { client, calls } = clientWith([ownerOrg, joinedOrg]);
  const root = await mountPage(client);

  const cards = orgCards(root);
  await click(cards[1]!);
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });

  const dialog = settingsDialog();
  // R487 K1: the upgrade form lives in the members section (Vue position).
  const membersNav = [...dialog.querySelectorAll('.nav-item')].find((item): item is HTMLElement => item instanceof HTMLElement && item.textContent === '成员管理');
  assert.ok(membersNav);
  await click(membersNav);
  await act(async () => {});
  const upgradeForm = [...dialog.querySelectorAll('form')].find((form) => (form.textContent ?? '').includes('提交申请')) as HTMLFormElement;
  assert.ok(upgradeForm, 'expected the upgrade form');
  const submit = textButtons(upgradeForm, '提交申请')[0];
  assert.ok(submit, 'expected the upgrade submit button');
  assert.equal(isDisabledControl(submit), false, 'no pending request yet — submit stays enabled');
  assert.equal(submit.getAttribute('title'), null);

  await submitForm(upgradeForm);
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });

  assert.equal(calls.upgrade.length, 1, 'expected one requestRoleUpgrade call');
  const submitAfter = textButtons(upgradeForm, '提交申请')[0];
  assert.ok(submitAfter, 'expected the upgrade submit button after submit');
  assert.equal(isDisabledControl(submitAfter), true, 'Vue sets hasPendingUpgrade right after success');
  assert.equal(submitAfter.getAttribute('title'), '审核中');
});

// ─── R487 K1: org edit-modal parity with the Vue OrganizationSettingsModal ──
// J3 checklist (report-J3.md part 2): three titled nav groups, invite controls
// embedded in the basic 邀请成员 card, nav badges, emoji avatar picker in edit
// basic, upgrade entry in members, shares description, footer save, 500 counter.

function orgDetailWith(invite: Record<string, unknown>): (id: string) => Organization | undefined {
  return () => ({ ...ownerOrg, invite_code: '', member_count: 2, require_approval: false, searchable: false, invite_code_validity_days: 7, member_limit: 50, ...invite } as unknown as Organization);
}

async function openSettings(client: WeKnoraClient, cardIndex = 0): Promise<HTMLElement> {
  const root = await mountPage(client);
  await click(orgCards(root)[cardIndex] as HTMLElement);
  // Flush the modal's org-detail fetch (Vue fetchOrgDetail parity).
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  return settingsDialog();
}

test('edit nav renders the Vue three titled groups without a standalone invite item', async () => {
  const { client } = clientWith([ownerOrg]);
  const dialog = await openSettings(client);

  const nav = dialog.querySelector('.settings-nav');
  assert.ok(nav, 'expected the modal sidebar nav');
  const texts = [...nav.querySelectorAll(':scope > *')].map((node) => (node.textContent ?? '').trim());
  const titleIndex = (label: string) => texts.findIndex((text) => text === label);
  const basicTitle = titleIndex('基础');
  const managementTitle = titleIndex('成员与协作');
  const resourcesTitle = titleIndex('共享资源');
  assert.ok(basicTitle >= 0, 'the 基础 group title renders');
  assert.ok(managementTitle > basicTitle, 'the 成员与协作 group title follows 基础');
  assert.ok(resourcesTitle > managementTitle, 'the 共享资源 group title comes last');
  assert.ok(texts.some((text) => text.startsWith('基本信息')), '基本信息 renders inside 基础');
  assert.ok(texts.some((text) => text.startsWith('成员管理')), '成员管理 renders inside 成员与协作');
  assert.ok(texts.some((text) => text.startsWith('加入申请')), '加入申请 renders inside 成员与协作');
  assert.ok(texts.some((text) => text.startsWith('共享知识库')), '共享知识库 renders inside 共享资源');
  assert.ok(texts.some((text) => text.startsWith('共享智能体')), '共享智能体 renders inside 共享资源');
  assert.ok(!texts.some((text) => text.includes('邀请链接')), 'the standalone 邀请链接 nav item must be gone (Vue embeds invite in basic)');
});

test('nav badges mirror Vue: pending join requests and shared-resource totals', async () => {
  // clientWith fixtures: 1 pending join request, 0 shared KBs, 1 shared agent.
  const { client } = clientWith([ownerOrg]);
  const dialog = await openSettings(client);
  const nav = dialog.querySelector('.settings-nav') as HTMLElement;

  const badgeOf = (label: string): string | undefined =>
    [...nav.querySelectorAll('.nav-item')].find((item) => (item.textContent ?? '').startsWith(label))?.querySelector('[data-nav-badge]')?.textContent;
  // Vue L30-33: join-requests badge only while pending > 0…
  assert.equal(badgeOf('加入申请'), '1', 'pending join-request count badges the nav entry');
  // …while shared KB/agent totals always badge (nav-badge-count variant).
  assert.equal(badgeOf('共享知识库'), '0', 'shared-KB total badges the nav entry even at 0');
  assert.equal(badgeOf('共享智能体'), '1', 'shared-agent total badges the nav entry');
});

test('basic embeds the Vue invite-member card with all six control groups', async () => {
  const { client } = clientWith([ownerOrg], undefined, orgDetailWith({ invite_code: 'INV-7X', invite_code_expires_at: null }));
  const dialog = await openSettings(client);
  const text = dialog.textContent ?? '';

  assert.match(text, /邀请成员/, 'the invite card title renders (organization.settings.inviteMembers)');
  assert.match(text, /通过邀请码或链接邀请他人加入共享空间/, 'the invite card description renders');
  assert.match(text, /邀请码/, 'control 1: invite code');
  assert.ok(text.includes('INV-7X'), 'the invite code value renders');
  assert.ok([...dialog.querySelectorAll('button')].some((button) => button.getAttribute('aria-label') === '刷新邀请码'), 'control 1: refresh-invite-code affordance');
  assert.match(text, /永不过期/, 'control 1: remaining-validity note (never expires)');
  // R488 D-B4.3: validity is a closed custom select (Vue t-select) — a button
  // carrying the selected label, not a native select leaking every option.
  const validity = dialog.querySelector('button[aria-label="邀请链接有效期"]') as HTMLButtonElement | null;
  assert.ok(validity, 'control 2: validity select renders');
  assert.match(validity.textContent ?? '', /7 天/, 'validity trigger shows the selected label (initialized from the org detail)');
  assert.equal(dialog.querySelector('select[aria-label="邀请链接有效期"]'), null, 'no native select leaks the option list into the DOM');
  assert.match(text, /\/join\?code=INV-7X/, 'control 3: invite link uses the Vue /join?code= formula');
  assert.ok(dialog.querySelector('[role="switch"][aria-label="需要审核"]'), 'control 4: require-approval switch');
  assert.ok(dialog.querySelector('[role="switch"][aria-label="开放可被搜索"]'), 'control 5: searchable switch');
  const limit = dialog.querySelector('.t-input__wrap[aria-label="成员数量上限"] input') as HTMLInputElement | null;
  assert.ok(limit, 'control 6: member-limit input renders');
  assert.equal(limit.value, '50', 'member limit initializes from the org detail');
  assert.match(text, /当前成员数：2/, 'control 6: live member-count hint');
});

test('validity change and the approval toggle save immediately like Vue', async () => {
  const { client, calls } = clientWith([ownerOrg]);
  const dialog = await openSettings(client);

  // R488 D-B4.3: the validity control is a Vue-style closed select — open the
  // dropdown, then click the 30 天 option.
  const validity = dialog.querySelector('button[aria-label="邀请链接有效期"]') as HTMLButtonElement;
  await click(validity);
  const option30 = [...dialog.querySelectorAll('button')].find((button) => (button.textContent ?? '').trim() === '30 天') as HTMLElement | undefined;
  assert.ok(option30, 'validity dropdown lists the Vue options (validity30Days)');
  await click(option30);
  assert.equal(calls.update.length, 1, 'validity change saves immediately (Vue handleValidityChange)');
  assert.deepEqual(calls.update[0], ['org-1', { invite_code_validity_days: 30 }]);

  const approval = dialog.querySelector('[role="switch"][aria-label="需要审核"]') as HTMLElement;
  await click(approval);
  assert.equal(calls.update.length, 2, 'approval toggle saves immediately (Vue handleApprovalToggle)');
  assert.deepEqual(calls.update[1], ['org-1', { require_approval: true }]);
  assert.equal((dialog.querySelector('[role="switch"][aria-label="需要审核"]') as HTMLElement).classList.contains('t-is-checked'), true, 'switch reflects the saved value (tdesign t-is-checked，台账 #7)');
});

// R488 D-B4.1 — Vue keeps the name/description field hints in EDIT mode too
// (setting-info .desc, Vue :59/:97); React only rendered them in create mode.
test('edit basic keeps the Vue name and description field hints', async () => {
  const { client } = clientWith([ownerOrg]);
  const dialog = await openSettings(client);
  const text = dialog.textContent ?? '';

  assert.match(text, /建议使用团队或项目名称，便于成员识别/, 'edit mode renders organization.editor.nameTip (Vue :59)');
  assert.match(text, /描述共享空间的用途和目标，帮助成员了解共享空间/, 'edit mode renders organization.editor.descriptionTip (Vue :97)');
});

// R488 D-B4.2 — Vue invite-card actions are icon-only text buttons with
// tooltips (Vue :122-134/:164-168); React rendered text buttons 复制/刷新邀请码.
test('invite-card copy and refresh actions are icon buttons like Vue', async () => {
  const { client } = clientWith([ownerOrg], undefined, orgDetailWith({ invite_code: 'INV-7X' }));
  const dialog = await openSettings(client);

  const copyButtons = [...dialog.querySelectorAll('button[aria-label="复制"]')];
  assert.equal(copyButtons.length, 2, 'code + link each expose an icon copy button (title tooltip only)');
  assert.ok([...dialog.querySelectorAll('button')].some((button) => button.getAttribute('aria-label') === '刷新邀请码'), 'refresh stays an icon button with the tooltip label');
  for (const label of ['复制', '刷新邀请码']) {
    assert.equal(textButtons(dialog, label).length, 0, 'no text-button variant of ' + label + ' remains (Vue icon-button form)');
  }
});

// R488 D-B4.3 — noise control: the validity options must not leak into the
// closed panel (Vue t-select renders only the selected label) and the member
// limit input must not render ▲▼ stepper glyphs (Vue t-input-number
// theme="normal" has no stepper column).
test('validity options stay inside the dropdown and the member limit drops the stepper glyphs', async () => {
  const { client } = clientWith([ownerOrg], undefined, orgDetailWith({ invite_code: 'INV-7X', invite_code_expires_at: '2099-01-01T00:00:00Z' }));
  const dialog = await openSettings(client);
  const text = dialog.textContent ?? '';

  assert.doesNotMatch(text, /(?<!\d)1 天|30 天/, 'unselected validity options do not leak into the closed panel');
  const limit = dialog.querySelector('.t-input__wrap[aria-label="成员数量上限"] input') as HTMLInputElement | null;
  assert.ok(limit, 'member-limit input renders');
  assert.equal(limit.type, 'number', 'member limit uses a plain number input');
  assert.doesNotMatch(text, /[▲▼]/, 'no stepper glyphs leak (Vue t-input-number theme=normal)');
});

test('refreshing the invite code goes through the invite-code endpoint and updates the code', async () => {
  const { client, calls } = clientWith([ownerOrg], undefined, orgDetailWith({ invite_code: 'OLD-1' }));
  const dialog = await openSettings(client);

  const refresh = [...dialog.querySelectorAll('button')].find((button) => button.getAttribute('aria-label') === '刷新邀请码') as HTMLButtonElement;
  assert.ok(refresh);
  await click(refresh);
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });

  assert.deepEqual(calls.inviteCode, ['org-1'], 'POST /organizations/:id/invite-code regenerates the code');
  assert.match(dialog.textContent ?? '', /GEN-CODE/, 'the generated code replaces the stale one');
});

test('edit basic offers the emoji avatar picker like Vue', async () => {
  const { client } = clientWith([ownerOrg]);
  const dialog = await openSettings(client);

  const picker = dialog.querySelector('button[aria-label="选择 Emoji 作为共享空间头像"]') as HTMLButtonElement | null;
  assert.ok(picker, 'edit-mode basic renders the avatar picker trigger (Vue name-input-wrapper)');
  assert.equal(picker.disabled, false, 'the picker stays enabled for managing admins');
  await click(picker);
  // The emoji grid renders inside the .grid-cols-6 picker popover (the emoji
  // literals differ in Unicode sequence between fixtures and the component, so
  // query structurally instead of by aria-label text).
  const emojiButton = dialog.querySelector('.wk-org-11 button') as HTMLButtonElement | null;
  assert.ok(emojiButton, 'the emoji grid opens on click');
  await click(emojiButton);
  // Reopening the popover shows the clear affordance — proof the emoji landed
  // in form state (Vue avatar-clear-btn renders while formData.avatar is set).
  await click(picker);
  assert.ok(textButtons(dialog, '清除').length > 0, 'choosing an emoji sets the avatar (clear affordance appears)');
});

test('edit footer save mirrors the Vue handleSave payload and basic keeps no inline save', async () => {
  const { client, calls } = clientWith([ownerOrg]);
  const dialog = await openSettings(client);

  const description = dialog.querySelector('textarea[name="organization-description"]') as HTMLTextAreaElement;
  assert.equal(description.maxLength, 500, 'Vue :maxlength=500 applies in edit mode too');
  await setInputValueAsync(description, '更新后的描述');

  const limit = dialog.querySelector('.t-input__wrap[aria-label="成员数量上限"] input') as HTMLInputElement;
  await setInputValueAsync(limit, '100');

  const picker = dialog.querySelector('button[aria-label="选择 Emoji 作为共享空间头像"]') as HTMLButtonElement;
  await click(picker);
  await click(dialog.querySelector('.wk-org-11 button') as HTMLButtonElement);

  const footerButtons = [...dialog.querySelectorAll('.settings-footer button')].map((button) => button.textContent ?? '');
  assert.ok(footerButtons.includes('取消'), 'the Vue settings-footer cancel stays global');
  assert.ok(footerButtons.includes('保存'), 'the edit footer carries the Vue common.save action');
  const basicForm = dialog.querySelector('form') as HTMLFormElement;
  assert.equal(textButtons(basicForm, '保存').length, 0, 'the basic form no longer owns the save button (Vue settings-footer L806-809)');

  await click(textButtons(dialog, '保存')[0]);
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  assert.equal(calls.update.length, 1);
  assert.equal((calls.update[0] as unknown[])[0], 'org-1');
  const payload = (calls.update[0] as unknown[])[1] as Record<string, unknown>;
  // The avatar follows Vue's 'emoji:<glyph>' encoding (selectAvatarEmoji);
  // the glyph itself is matched by prefix to stay byte-sequence agnostic.
  assert.match(String(payload.avatar), /^emoji:/, 'the picked emoji avatar rides along like Vue handleSave');
  assert.deepEqual({ ...payload, avatar: undefined }, {
    name: 'parity-org',
    description: '更新后的描述',
    avatar: undefined,
    require_approval: false,
    searchable: false,
    invite_code_validity_days: 7,
    member_limit: 100,
  });
});

test('members section mirrors the Vue header: 成员管理 title, permission matrix, upgrade entry', async () => {
  const { client } = clientWith([ownerOrg, joinedOrg]);
  const dialog = await openSettings(client, 1);

  const basicPanel = dialog.textContent ?? '';
  assert.doesNotMatch(basicPanel, /申请升级/, 'the upgrade entry no longer lives in basic');

  const membersNav = [...dialog.querySelectorAll('.nav-item')].find((item): item is HTMLElement => item instanceof HTMLElement && item.textContent === '成员管理');
  assert.ok(membersNav);
  await click(membersNav);
  await act(async () => {});

  // The content heading is the first h2 OUTSIDE the sidebar rail (the
  // sidebar-title h2 comes first in DOM order).
  const contentHeadings = [...dialog.querySelectorAll('h2')].filter((node) => node.closest('.settings-sidebar') === null);
  const heading = contentHeadings[0];
  assert.equal(heading?.textContent, '成员管理', 'section h2 reads organization.manageMembers (Vue :302)');
  assert.match(dialog.textContent ?? '', /共享空间成员/, 'inner list title keeps the Vue 共享空间成员 wording');
  assert.match(dialog.textContent ?? '', /提交申请/, 'the upgrade form lives in members (Vue :357)');

  const matrixTrigger = dialog.querySelector('button[aria-label="成员权限"]') as HTMLButtonElement | null;
  assert.ok(matrixTrigger, 'the permission-matrix info trigger renders next to the h2 (Vue :306-310)');
  await click(matrixTrigger);
  await act(async () => {});
  const matrixText = dialog.textContent ?? '';
  assert.match(matrixText, /了解共享空间中不同角色对知识库与智能体的权限范围/, 'matrix popup shows the Vue permissionsDesc');
  for (const role of ['管理员', '编辑', '只读']) assert.match(matrixText, new RegExp(role), 'matrix lists the ' + role + ' role block');
  assert.match(matrixText, /管理共享空间设置、成员及知识库与智能体共享/, 'matrix carries the admin permission row');
});

test('shares section shows the Vue sharedDesc under the 共享知识库 heading', async () => {
  const { client } = clientWith([ownerOrg]);
  const dialog = await openSettings(client);

  const sharesNav = [...dialog.querySelectorAll('.nav-item')].find((item): item is HTMLElement => item instanceof HTMLElement && (item.textContent ?? '').startsWith('共享知识库'));
  assert.ok(sharesNav);
  await click(sharesNav);
  await act(async () => {});

  const contentHeadings = [...dialog.querySelectorAll('h2')].filter((node) => node.closest('.settings-sidebar') === null);
  const heading = contentHeadings[0];
  assert.equal(heading?.textContent, '共享知识库', 'section h2 reads organization.share.sharedKnowledgeBase like Vue');
  assert.match(dialog.textContent ?? '', /查看共享到此共享空间的所有知识库/, 'the sharedDesc line renders (Vue sharedDesc)');
});
