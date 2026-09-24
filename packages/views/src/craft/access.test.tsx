import assert from 'node:assert/strict';
import test from 'node:test';
import React, { act } from 'react';
import { JSDOM } from 'jsdom';
import { CraftAccess, type CraftTaskMember } from './access.tsx';

const ownerMembers: CraftTaskMember[] = [
  { user_id: 'owner', role: 'owner' },
  { user_id: 'viewer-with-a-very-long-identifier-that-must-wrap-on-narrow-screens', role: 'viewer' },
];

async function mount(role: 'owner' | 'collaborator' | 'viewer', members = ownerMembers, callbacks = {
  onGrant: async (_userId: string, _role: 'collaborator' | 'viewer') => {},
  onRevoke: async (_userId: string) => {},
}) {
  const dom = new JSDOM('<!doctype html><html><body></body></html>');
  Object.assign(globalThis, {
    window: dom.window,
    document: dom.window.document,
    HTMLElement: dom.window.HTMLElement,
    Element: dom.window.Element,
    Node: dom.window.Node,
    Event: dom.window.Event,
    IS_REACT_ACT_ENVIRONMENT: true,
  });
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
  const { createRoot } = await import('../../../../apps/web/node_modules/react-dom/client.js');
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => root.render(<CraftAccess role={role} members={members} {...callbacks} />));
  return {
    container,
    async unmount() { await act(async () => root.unmount()); dom.window.close(); },
  };
}

function field(container: HTMLElement, label: string): HTMLInputElement | HTMLSelectElement {
  const labelled = [...container.querySelectorAll('label')].find(item => item.textContent?.startsWith(label));
  assert.ok(labelled, `${label} control is present`);
  const control = labelled.querySelector('input, select');
  assert.ok(control, `${label} control is associated with its label`);
  return control;
}

function setControlValue(control: HTMLInputElement | HTMLSelectElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(control), 'value')?.set;
  assert.ok(setter, 'native value setter exists');
  setter.call(control, value);
  control.dispatchEvent(new Event(control.tagName === 'SELECT' ? 'change' : 'input', { bubbles: true }));
}

test('Craft access projects Owner controls and keeps Viewer and Collaborator read only', async () => {
  for (const role of ['viewer', 'collaborator'] as const) {
    const view = await mount(role);
    try {
      assert.match(view.container.textContent ?? '', /viewer-with-a-very-long-identifier/);
      assert.doesNotMatch(view.container.textContent ?? '', /Add member|Revoke/);
      assert.equal(view.container.querySelector('[aria-live]'), null);
    } finally { await view.unmount(); }
  }
  const owner = await mount('owner');
  try {
    assert.ok(owner.container.querySelector('button[type="submit"]'));
    assert.equal(owner.container.querySelectorAll('button').length, 2);
    const longId = [...owner.container.querySelectorAll('li span')].find(item => item.textContent?.startsWith('viewer-with'));
    assert.ok(longId);
    assert.match(longId.getAttribute('style') ?? '', /overflow-wrap:\s*anywhere/);
  } finally { await owner.unmount(); }
});

test('grant failure keeps the draft and role, announces the error, and allows retry', async () => {
  let rejectGrant!: (error: Error) => void;
  const view = await mount('owner', ownerMembers, {
    onGrant: () => new Promise<void>((_resolve, reject) => { rejectGrant = reject; }),
    onRevoke: async () => {},
  });
  try {
    const input = field(view.container, 'User ID') as HTMLInputElement;
    const select = field(view.container, 'Role') as HTMLSelectElement;
    await act(async () => {
      setControlValue(input, '  new-user  ');
      setControlValue(select, 'collaborator');
      view.container.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    });
    assert.equal((view.container.querySelector('button[type="submit"]') as HTMLButtonElement).disabled, true);
    assert.match(view.container.querySelector('[aria-live]')?.textContent ?? '', /Adding member/);
    await act(async () => rejectGrant(new Error('network down')));
    assert.equal(input.value, '  new-user  ');
    assert.equal(select.value, 'collaborator');
    assert.equal((view.container.querySelector('button[type="submit"]') as HTMLButtonElement).disabled, false);
    assert.match(view.container.querySelector('[aria-live]')?.textContent ?? '', /could not be added/i);
  } finally { await view.unmount(); }
});

test('grant success clears the draft only after acknowledgement and announces success', async () => {
  let resolveGrant!: () => void;
  const onGrant = async () => new Promise<void>(resolve => { resolveGrant = resolve; });
  const view = await mount('owner', ownerMembers, { onGrant, onRevoke: async () => {} });
  try {
    const input = field(view.container, 'User ID') as HTMLInputElement;
    await act(async () => {
      setControlValue(input, 'new-user');
      view.container.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    });
    assert.equal(input.value, 'new-user');
    assert.equal(view.container.querySelectorAll('li').length, ownerMembers.length);
    await act(async () => resolveGrant());
    assert.equal(input.value, '');
    assert.equal(view.container.querySelectorAll('li').length, ownerMembers.length, 'members remain server-projected until props refresh');
    assert.match(view.container.querySelector('[aria-live]')?.textContent ?? '', /added/i);
  } finally { await view.unmount(); }
});

test('revoke failure retains the member and reports error; success waits for refreshed members', async () => {
  let resolveRevoke!: () => void;
  let callCount = 0;
  const view = await mount('owner', ownerMembers, {
    onGrant: async () => {},
    onRevoke: async () => {
      callCount++;
      if (callCount === 1) throw new Error('network down');
      await new Promise<void>(resolve => { resolveRevoke = resolve; });
    },
  });
  try {
    const revoke = [...view.container.querySelectorAll('button')].find(button => button.getAttribute('aria-label')?.startsWith('Revoke')) as HTMLButtonElement | undefined;
    assert.ok(revoke);
    await act(async () => revoke.click());
    assert.equal(view.container.querySelectorAll('li').length, ownerMembers.length);
    assert.match(view.container.querySelector('[aria-live]')?.textContent ?? '', /could not be revoked/i);
    await act(async () => revoke.click());
    assert.equal(revoke.disabled, true);
    assert.equal(view.container.querySelectorAll('li').length, ownerMembers.length);
    await act(async () => resolveRevoke());
    assert.equal(view.container.querySelectorAll('li').length, ownerMembers.length, 'successful callback does not optimistically remove the member');
    assert.match(view.container.querySelector('[aria-live]')?.textContent ?? '', /revoked/i);
  } finally { await view.unmount(); }
});
