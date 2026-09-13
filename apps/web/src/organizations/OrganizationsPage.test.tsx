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

interface Calls { create: unknown[]; preview: string[]; join: unknown[]; submitJoinRequest: unknown[]; updateRole: unknown[][]; review: unknown[][]; leave: string[]; remove: string[]; update: unknown[][]; membersList: string[]; joinRequestsList: string[]; kbSharesList: string[]; inviteCode: string[] }

function clientWith(organizations: Organization[]): { client: WeKnoraClient; calls: Calls } {
  const calls: Calls = { create: [], preview: [], join: [], submitJoinRequest: [], updateRole: [], review: [], leave: [], remove: [], update: [], membersList: [], joinRequestsList: [], kbSharesList: [], inviteCode: [] };
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
  };
  return { client: { identity: { organizations: organizationsApi } } as unknown as WeKnoraClient, calls };
}

let mountedRoot: Root | undefined;

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
});

async function mountPage(client: WeKnoraClient, inviteCode?: string): Promise<HTMLElement> {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<OrganizationsPage client={client} inviteCode={inviteCode} />);
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

async function submitForm(form: HTMLFormElement): Promise<void> {
  await act(async () => form.dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true })));
}

function textButtons(root: HTMLElement, text: string): HTMLButtonElement[] {
  return [...root.querySelectorAll('button')].filter((button) => button.textContent === text) as HTMLButtonElement[];
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
  const createdCount = root.querySelector('.org-section-count');
  assert.ok(createdCount, 'expected a live section count chip');

  const cards = root.querySelectorAll('.org-card');
  assert.equal(cards.length, 2);
  assert.match(root.textContent ?? '', /parity-org/);
  assert.match(root.textContent ?? '', /joined-org/);
  assert.match(root.textContent ?? '', /暂无描述/);
  assert.match(root.textContent ?? '', /团队空间/);
  assert.ok(root.querySelector('.org-avatar'), 'expected gradient avatar');
  assert.ok(root.querySelector('.card-decoration svg'), 'expected constellation decoration');
  const statBadges = root.querySelectorAll('.feature-badge');
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
  const actions = root.querySelectorAll('.empty-state-actions button');
  assert.equal(actions.length, 2);
  assert.match(actions[0]?.textContent ?? '', /加入共享空间/);
  assert.match(actions[1]?.textContent ?? '', /创建共享空间/);
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
  const form = dialog.querySelector('form');
  assert.ok(form, 'expected create form');
  await submitForm(form as HTMLFormElement);
  assert.equal(calls.create.length, 1);
  assert.equal((calls.create[0] as { name: string }).name, '新空间');
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

  const card = root.querySelector('.org-card');
  assert.ok(card);
  await click(card as HTMLElement);
  await act(async () => {});

  const dialog = root.querySelector('[role="dialog"]');
  assert.ok(dialog, 'expected settings modal after card click');
  assert.match(dialog.textContent ?? '', /共享空间设置/);
  assert.deepEqual(calls.membersList, ['org-1']);
  assert.deepEqual(calls.joinRequestsList, ['org-1']);

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

test('invite_code prop auto-previews the linked organization', async () => {
  const { client, calls } = clientWith([ownerOrg]);
  await mountPage(client, 'LINK1');
  assert.deepEqual(calls.preview, ['LINK1']);
});

test('more menu offers leave for joined spaces and hides delete for non-owners', async () => {
  const { client } = clientWith([ownerOrg, joinedOrg]);
  const root = await mountPage(client);
  const cards = root.querySelectorAll('.org-card');
  const joinedCard = [...cards].find((card) => card.textContent?.includes('joined-org'));
  assert.ok(joinedCard);
  const more = (joinedCard as HTMLElement).querySelector('.more-wrap');
  assert.ok(more);
  await click(more as HTMLElement);
  const menu = (joinedCard as HTMLElement).querySelector('.popup-menu');
  assert.ok(menu);
  assert.match(menu.textContent ?? '', /退出共享空间/);
  assert.doesNotMatch(menu.textContent ?? '', /删除/);
});
