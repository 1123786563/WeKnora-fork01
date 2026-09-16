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

  const [approve, reject] = Array.from(container.querySelectorAll('button')) as HTMLButtonElement[];
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
