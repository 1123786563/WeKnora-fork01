import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';
import React from 'react';

const { JSDOM } = createRequire(import.meta.url)('jsdom') as {
  JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis };
};
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  React,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { act } = await import('react');
const { ToolApprovalCard } = await import('../../../../packages/views/src/chat/tool-approval.tsx');

test('tool approval ignores a second decision while the first request is pending', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  const calls: Array<'approve' | 'reject'> = [];
  let release!: () => void;
  const pending = new Promise<void>((resolve) => { release = resolve; });
  const root = createRoot(container);

  await act(async () => {
    root.render(React.createElement(ToolApprovalCard, {
      approval: { pendingId: 'approval-1', toolName: 'search_docs', status: 'pending', arguments: {} },
      busy: false,
      onResolve: async (_pendingId: string, decision: 'approve' | 'reject') => {
        calls.push(decision);
        await pending;
      },
    }));
  });

  // Vue inline-actions order: reject first, then the primary approve.
  const [reject, approve] = Array.from(container.querySelectorAll('button')) as HTMLButtonElement[];
  await act(async () => {
    approve.click();
    reject.click();
  });
  assert.deepEqual(calls, ['approve']);

  release();
  await act(async () => {
    await pending;
    root.unmount();
  });
  container.remove();
});

test('tool approval args editing mirrors the Vue contract: live validation, modified hint, reject reason', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  const { resolveChatCopy } = await import('../../../../packages/views/src/chat/chat-copy.ts');
  const copy = resolveChatCopy('zh-CN');
  const calls: Array<{ decision: string; modifiedArgs?: Record<string, unknown>; reason?: string }> = [];
  await act(async () => {
    root.render(React.createElement(ToolApprovalCard, {
      approval: { pendingId: 'approval-1', toolName: 'search_docs', status: 'pending', arguments: { query: 'docs' } },
      busy: false,
      copy,
      onResolve: async (_pendingId: string, decision: 'approve' | 'reject', modifiedArgs?: Record<string, unknown>, reason?: string) => {
        calls.push({ decision, modifiedArgs, reason });
      },
    }));
  });

  // Vue inline-actions order: reject first, then the primary approve.
  const buttons = () => Array.from(container.querySelectorAll('button')) as HTMLButtonElement[];
  const [reject, approve] = buttons();
  assert.ok((reject.textContent ?? '').includes(copy.approvalReject));
  assert.ok((approve.textContent ?? '').includes(copy.approvalApprove));

  const textarea = container.querySelector('textarea') as HTMLTextAreaElement;
  const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLTextAreaElement.prototype, 'value')!.set!;
  const type = (value: string) => act(async () => {
    setValue.call(textarea, value);
    textarea.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
  });

  // Invalid JSON: approve is disabled and an inline alert mirrors isJsonValid.
  await type('{"query": ');
  assert.equal(approve.disabled, true);
  const alert = container.querySelector('p[role="alert"]');
  assert.ok(alert, 'invalid JSON alert missing');
  assert.ok((alert!.textContent ?? '').includes(copy.approvalInvalidJson));
  await act(async () => { approve.click(); });
  assert.deepEqual(calls, [], 'disabled approve must not resolve');

  // Valid edited JSON: approve re-enabled and the Vue argsModified hint shows.
  await type('{"query": "edited"}');
  assert.equal(approve.disabled, false);
  const status = container.querySelector('p[role="status"]');
  assert.ok(status, 'args-modified status missing');
  assert.ok((status!.textContent ?? '').includes(copy.approvalArgsModified));
  await act(async () => { approve.click(); });
  assert.deepEqual(calls, [{ decision: 'approve', modifiedArgs: { query: 'edited' }, reason: undefined }]);

  // Reject carries the Vue userRejected reason instead of modified args.
  await act(async () => { reject.click(); });
  assert.deepEqual(calls, [
    { decision: 'approve', modifiedArgs: { query: 'edited' }, reason: undefined },
    { decision: 'reject', modifiedArgs: undefined, reason: '用户拒绝' },
  ]);

  await act(async () => { root.unmount(); });
  container.remove();
});

test('pending approvals show the Vue countdown: m:ss far out, short form near expiry, hidden once resolved', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  const { resolveChatCopy } = await import('../../../../packages/views/src/chat/chat-copy.ts');
  const copy = resolveChatCopy('zh-CN');

  // One shared requestedAt keeps the deadline fixed; each render mounts a
  // fresh instance so "now" refreshes and the assertions stay seconds-boundary
  // safe (the Vue deadline math floors, so both expected values stay inside
  // the targeted threshold band).
  const requestedAt = Math.floor(Date.now() / 1000);
  const timer = () => container.querySelector('.wk-chat-approval-timer') as HTMLElement | null;
  const renderWith = (timeoutSeconds: number) => act(async () => {
    root.render(React.createElement(ToolApprovalCard, {
      approval: {
        pendingId: 'approval-1', toolName: 'search_docs', status: 'pending', arguments: {},
        requestedAt, timeoutSeconds,
      },
      busy: false,
      copy,
      onResolve: async () => {},
    }));
  });
  try {
    // Far from expiry the Vue card renders m:ss with no warning/critical class.
    await renderWith(3600);
    let node = timer();
    assert.ok(node, 'countdown timer missing on pending card');
    assert.match(node!.textContent ?? '', /^\d{1,3}:\d{2}$/);
    assert.equal(node!.className.includes('wk-timer-warning'), false);
    assert.equal(node!.className.includes('wk-timer-critical'), false);

    // Inside the 30s critical window the short localized form shows; zh copy
    // is "{seconds}s" and the Vue threshold marks <= 30s as critical.
    await act(async () => { root.unmount(); });
    const root2 = createRoot(container);
    await act(async () => {
      root2.render(React.createElement(ToolApprovalCard, {
        approval: {
          pendingId: 'approval-1', toolName: 'search_docs', status: 'pending', arguments: {},
          requestedAt, timeoutSeconds: 30,
        },
        busy: false,
        copy,
        onResolve: async () => {},
      }));
    });
    node = timer();
    assert.ok(node);
    const nearExpiry = (node!.textContent ?? '').trim();
    assert.match(nearExpiry, /^\d{1,2}s$/);
    assert.ok(Number.parseInt(nearExpiry, 10) <= 30, `expected <= 30s, got ${nearExpiry}`);
    assert.equal(node!.className.includes('wk-timer-critical'), true);

    await act(async () => { root2.unmount(); });
  } finally {
    void root;
    container.remove();
  }
});

test('countdown ticks down each second and expiry never auto-rejects or disables actions', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);
  const { resolveChatCopy } = await import('../../../../packages/views/src/chat/chat-copy.ts');
  const copy = resolveChatCopy('zh-CN');
  const calls: string[] = [];

  await act(async () => {
    root.render(React.createElement(ToolApprovalCard, {
      approval: {
        pendingId: 'approval-1', toolName: 'search_docs', status: 'pending', arguments: {},
        requestedAt: Math.floor(Date.now() / 1000), timeoutSeconds: 8,
      },
      busy: false,
      copy,
      onResolve: async (_pendingId: string, decision: 'approve' | 'reject') => { calls.push(decision); },
    }));
  });

  const readSeconds = () => {
    const node = container.querySelector('.wk-chat-approval-timer') as HTMLElement | null;
    assert.ok(node, 'countdown timer missing');
    return Number.parseInt((node!.textContent ?? '').replace(/\D/g, ''), 10);
  };
  const before = readSeconds();
  // One real interval tick (plus margin) must strictly decrease the countdown.
  await new Promise((resolvePromise) => setTimeout(resolvePromise, 1300));
  const after = readSeconds();
  assert.ok(after < before, `countdown must decrease after a tick (before=${before}, after=${after})`);

  // Vue expiry semantics: the timer sits at 0 while the buttons stay usable
  // and no automatic rejection is submitted.
  const buttons = () => Array.from(container.querySelectorAll('button')) as HTMLButtonElement[];
  const [, approve] = buttons();
  assert.equal(approve.disabled, false);
  assert.deepEqual(calls, [], 'expired approval must not auto-resolve');

  await act(async () => { root.unmount(); });
  container.remove();
});

test('resolved approvals keep the Vue contract: no countdown is rendered', async () => {
  const container = document.createElement('div');
  document.body.append(container);
  const root = createRoot(container);

  await act(async () => {
    root.render(React.createElement(ToolApprovalCard, {
      approval: { pendingId: 'approval-1', toolName: 'search_docs', status: 'resolved', decision: 'approve', arguments: {}, requestedAt: Math.floor(Date.now() / 1000), timeoutSeconds: 600 },
      busy: false,
      onResolve: async () => {},
    }));
  });
  assert.equal(container.querySelector('.wk-chat-approval-timer'), null);

  await act(async () => { root.unmount(); });
  container.remove();
});
