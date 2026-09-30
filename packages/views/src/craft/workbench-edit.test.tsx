// T09 (#135) front-end domain rule: requesting a serialized edit is a
// server authority the edit panel only PROJECTS. Only a member whose current
// Task role allows writing (owner or collaborator) sees the edit composer; a
// Viewer sees a read-only notice and the request callback is never invoked.
// The panel projects the RAW PostCraftRun wire envelope (snake_case
// data.run_id / writer_acquisition / initiated_by) — the single
// projection-and-rejection point lives here, the assembly resolves the raw
// payload. The panel surfaces the server's serialization answers verbatim:
// the writer-acquisition conflict (another writing run holds the Workspace),
// an unknown lease outcome (never painted as acquired) and the actual
// initiating member the server recorded — it never derives authority,
// identity or conflict state client-side.
import assert from 'node:assert/strict';
import test from 'node:test';
import React, { act } from 'react';
import { JSDOM } from 'jsdom';
import { CraftEditRequestPanel, projectEditOutcome } from './workbench-edit.tsx';

// The PostCraftRun wire envelope, exactly as the server answers it.
const acquiredWire = {
  success: true,
  data: { run_id: 'run-1', session_id: 's-1', status: 'running' },
  writer_acquisition: { workspace_id: 'ws-1', status: 'acquired' },
  initiated_by: 'u2',
};

async function mount(overrides: {
  canWrite?: boolean;
  runActive?: boolean;
  locale?: 'zh' | 'en';
  onRequestEdit?: (prompt: string) => Promise<unknown>;
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
  const prompts: string[] = [];
  await act(async () => root.render(
    <CraftEditRequestPanel
      locale={overrides.locale ?? 'zh'}
      canWrite={overrides.canWrite ?? true}
      runActive={overrides.runActive ?? false}
      onRequestEdit={overrides.onRequestEdit ?? (async (prompt: string) => {
        prompts.push(prompt);
        return acquiredWire;
      })}
    />,
  ));
  return {
    container,
    prompts,
    async rerender(props: { canWrite?: boolean; runActive?: boolean }) {
      await act(async () => root.render(
        <CraftEditRequestPanel
          locale={overrides.locale ?? 'zh'}
          canWrite={props.canWrite ?? overrides.canWrite ?? true}
          runActive={props.runActive ?? overrides.runActive ?? false}
          onRequestEdit={overrides.onRequestEdit ?? (async (prompt: string) => {
            prompts.push(prompt);
            return acquiredWire;
          })}
        />,
      ));
    },
    async unmount() { await act(async () => root.unmount()); dom.window.close(); },
  };
}

function promptArea(container: HTMLElement): HTMLTextAreaElement | null {
  return container.querySelector('textarea[data-testid="craft-edit-prompt"]');
}

function submitButton(container: HTMLElement): HTMLButtonElement | null {
  return container.querySelector('button[data-testid="craft-edit-submit"]');
}

async function typePrompt(area: HTMLTextAreaElement, text: string) {
  const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(area), 'value')?.set;
  assert.ok(setter);
  await act(async () => {
    setter.call(area, text);
    area.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

async function submitForm(container: HTMLElement) {
  const form = container.querySelector('form');
  assert.ok(form, 'the composer form is present');
  await act(async () => form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })));
}

test('a viewer sees a read-only notice and can never request an edit', async () => {
  const view = await mount({ canWrite: false });
  try {
    assert.match(view.container.textContent ?? '', /只读/);
    assert.equal(promptArea(view.container), null, 'no composer for a read-only member');
    assert.equal(submitButton(view.container), null, 'no submit control for a read-only member');
    assert.deepEqual(view.prompts, [], 'the request callback is never invoked');
  } finally { await view.unmount(); }
});

test('a collaborator requests one serialized edit and the panel projects the recorded initiator', async () => {
  const view = await mount({});
  try {
    const area = promptArea(view.container);
    const button = submitButton(view.container);
    assert.ok(area && button, 'the composer is present for a writing member');
    await typePrompt(area, '把标题改成蓝色');
    await submitForm(view.container);
    assert.deepEqual(view.prompts, ['把标题改成蓝色'], 'exactly one request carries the prompt');
    assert.match(view.container.textContent ?? '', /u2/, 'the actual initiating member the server recorded is shown');
    assert.match(view.container.textContent ?? '', /run-1/, 'the admitted run id is shown');
  } finally { await view.unmount(); }
});

test('a wire without initiated_by shows the run without inventing an initiator', async () => {
  const view = await mount({
    onRequestEdit: async () => ({
      success: true,
      data: { run_id: 'run-noactor', status: 'running' },
      writer_acquisition: { workspace_id: 'ws-1', status: 'acquired' },
    }),
  });
  try {
    const area = promptArea(view.container);
    assert.ok(area);
    await typePrompt(area, '旧版服务端');
    await submitForm(view.container);
    assert.match(view.container.textContent ?? '', /run-noactor/, 'the admitted run id is shown');
    assert.doesNotMatch(view.container.textContent ?? '', /发起成员/, 'no initiator line when the wire carries none');
  } finally { await view.unmount(); }
});

test('a writer-lease conflict is surfaced as an alert and keeps the draft for the guided retry', async () => {
  const view = await mount({
    onRequestEdit: async () => ({
      success: true,
      data: { run_id: 'run-2', status: 'running' },
      writer_acquisition: { workspace_id: 'ws-1', status: 'conflict' },
      initiated_by: 'u2',
    }),
  });
  try {
    const area = promptArea(view.container);
    const button = submitButton(view.container);
    assert.ok(area && button);
    await typePrompt(area, '并发修改');
    await submitForm(view.container);
    const alert = view.container.querySelector('[role="alert"]');
    assert.ok(alert, 'the conflict is announced, never swallowed');
    assert.match(alert.textContent ?? '', /另一个写入/);
    assert.match(view.container.textContent ?? '', /run-2/, 'the admitted run id stays visible (admission is not undone)');
    assert.equal((promptArea(view.container) as HTMLTextAreaElement).value, '并发修改',
      'a conflicted request keeps the draft — the notice guides an explicit retry');
  } finally { await view.unmount(); }
});

test('an unknown writer-lease outcome gets a neutral notice, never painted as acquired', async () => {
  const view = await mount({
    onRequestEdit: async () => ({
      success: true,
      data: { run_id: 'run-3', status: 'running' },
      writer_acquisition: { workspace_id: 'ws-1', status: 'unknown' },
      initiated_by: 'u2',
    }),
  });
  try {
    const area = promptArea(view.container);
    assert.ok(area);
    await typePrompt(area, '租约不明');
    await submitForm(view.container);
    const notice = view.container.querySelector('[data-testid="craft-edit-unknown-lease"]');
    assert.ok(notice, 'the unknown lease outcome is announced');
    assert.equal(notice.getAttribute('role'), 'status', 'unknown is a neutral status, not a conflict alert');
    assert.match(notice.textContent ?? '', /租约结果不明/);
    assert.equal(view.container.querySelectorAll('[role="alert"]').length, 0, 'unknown is never escalated to an alert');
    assert.match(view.container.textContent ?? '', /run-3/, 'the admitted run id is shown');
    assert.equal((promptArea(view.container) as HTMLTextAreaElement).value, '',
      'the request went through — the composer resets like an acquired answer');
  } finally { await view.unmount(); }
});

test('a refused request (revoked collaborator) keeps the draft for an explicit retry', async () => {
  let rejectOnce = true;
  const view = await mount({
    onRequestEdit: async () => {
      if (rejectOnce) {
        rejectOnce = false;
        throw new Error('没有修改权限');
      }
      return acquiredWire;
    },
  });
  try {
    const area = promptArea(view.container);
    const button = submitButton(view.container);
    assert.ok(area && button);
    await typePrompt(area, '撤销后的重试');
    await submitForm(view.container);
    const alert = view.container.querySelector('[role="alert"]');
    assert.ok(alert, 'the refusal is announced');
    assert.match(alert.textContent ?? '', /没有修改权限/);
    assert.equal((promptArea(view.container) as HTMLTextAreaElement).value, '撤销后的重试', 'the refused prompt returns to the composer');
    assert.equal(submitButton(view.container)?.disabled, false, 'retry stays available');
    await submitForm(view.container);
    assert.match(view.container.textContent ?? '', /u2/, 'the retry succeeded and projects the initiator');
  } finally { await view.unmount(); }
});

test('a malformed wire is rejected with the bilingual invalid-response wording', async () => {
  const view = await mount({ onRequestEdit: async () => ({ success: true }) });
  try {
    const area = promptArea(view.container);
    assert.ok(area);
    await typePrompt(area, '畸形响应');
    await submitForm(view.container);
    const alert = view.container.querySelector('[role="alert"]');
    assert.ok(alert, 'the malformed response is announced');
    assert.match(alert.textContent ?? '', /服务器响应无效/, 'the wording comes from the EDIT_STRINGS table');
    assert.equal((promptArea(view.container) as HTMLTextAreaElement).value, '畸形响应', 'a dropped projection keeps the draft');
  } finally { await view.unmount(); }
});

test('while a writing run is active the composer is disabled with the serialization reason', async () => {
  const view = await mount({ runActive: true });
  try {
    const area = promptArea(view.container);
    assert.ok(area, 'the composer stays visible for a writing member');
    await typePrompt(area, '排队中的修改');
    assert.equal(submitButton(view.container)?.disabled, true, 'no second concurrent edit may be submitted while a draft exists');
    assert.match(view.container.textContent ?? '', /正在执行/, 'the serialization reason is visible');
    assert.deepEqual(view.prompts, []);
    // When the run finishes the same member can request the next edit.
    await view.rerender({ runActive: false });
    assert.equal(submitButton(view.container)?.disabled, false);
    await submitForm(view.container);
    assert.deepEqual(view.prompts, ['排队中的修改'], 'the queued edit is submitted once the slot frees');
  } finally { await view.unmount(); }
});

test('the projection drops malformed wires instead of guessing authority', () => {
  assert.equal(projectEditOutcome(null), null);
  assert.equal(projectEditOutcome({}), null);
  assert.equal(projectEditOutcome({ data: {} }), null, 'a run envelope without run_id is malformed');
  assert.equal(projectEditOutcome({ data: { run_id: '' }, initiated_by: 'u2' }), null);
  assert.equal(projectEditOutcome({ data: { run_id: 'r' }, initiated_by: '' }), null, 'an empty initiator is malformed, never defaulted');
  // An unknown writer status is rejected, never defaulted to acquired.
  assert.equal(projectEditOutcome({ data: { run_id: 'r' }, writer_acquisition: { workspace_id: 'w', status: 'locked' } }), null);
  const conflict = projectEditOutcome({ data: { run_id: 'r' }, writer_acquisition: { workspace_id: 'w', status: 'conflict' }, initiated_by: 'u2' });
  assert.ok(conflict);
  assert.equal(conflict.writerAcquisition?.status, 'conflict');
  assert.equal(conflict.actorUserId, 'u2');
  // A missing writer acquisition stays null (the server may not answer one).
  const noLease = projectEditOutcome({ data: { run_id: 'r' } });
  assert.ok(noLease);
  assert.equal(noLease.writerAcquisition, null);
  assert.equal(noLease.actorUserId, null, 'no initiated_by on the wire — no client-side initiator');
});
