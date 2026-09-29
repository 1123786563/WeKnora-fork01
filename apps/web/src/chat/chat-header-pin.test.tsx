/**
 * OCR r1 high-14 回归：置顶切换必须传“下一状态”。
 * 宿主 toggleSessionPin(sessionId, pinned) 的 pinned 参数是目标状态
 * （true=pin、false=unpin）；菜单项此前传 Boolean(props.isPinned)（当前
 * 状态），未置顶点置顶发出 unpin（无变化）、已置顶点取消发出 pin（无变化）。
 */
import '../test-tdom-harness.ts';
import assert from 'node:assert/strict';
import test, { afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';

const { createRoot } = await import('react-dom/client');
const { ChatHeader } = await import('./chat-header.tsx');
const { resolveChatCopy } = await import('@weknora/views');

let root: ReturnType<typeof createRoot> | undefined;

afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  /* tdesign Popup destroyOnClose 的离场动画定时器在卸载后仍会触发一次
   * body 容器移除——先冲刷这些定时器再清 body（orgs 用例同款处理）。 */
  await new Promise((resolve) => setTimeout(resolve, 260));
  document.body.replaceChildren();
});

function headerProps(overrides: Partial<Parameters<typeof ChatHeader>[0]>): Parameters<typeof ChatHeader>[0] {
  return {
    copy: resolveChatCopy('zh-CN'),
    title: '会话标题',
    renameTitle: '会话标题',
    renameTitleRequired: '标题不能为空',
    renameTitleFailed: '重命名失败',
    renameCancel: '取消',
    renameConfirm: '确定',
    renameSaving: '保存中…',
    clearConfirmTitle: '清空会话',
    clearConfirmBody: '确认清空？',
    clearConfirmAction: '清空',
    deleteConfirmTitle: '删除会话',
    deleteConfirmBody: '确认删除？',
    deleteConfirmAction: '删除',
    cancelLabel: '取消',
    operationFailed: '操作失败',
    onRenameSession: async () => undefined,
    ...overrides,
  };
}

test('pin menu item asks for the next pin state, not the current one', async () => {
  for (const isPinned of [false, true]) {
    const requested: boolean[] = [];
    const container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
    await act(async () => root?.render(<ChatHeader {...headerProps({ isPinned, onTogglePin: (pinned) => { requested.push(pinned) } })} />));
    await act(async () => { container.querySelector<HTMLButtonElement>('.chat-header__menu-btn')?.click(); });
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
    // TDesign Popup 把内容挂到 body 门户，不在组件容器内查询。
    const pinItem = document.querySelector<HTMLButtonElement>('[data-menu-action="pin"]');
    assert.ok(pinItem, `isPinned=${isPinned}: pin menu item renders when onTogglePin is provided`);
    await act(async () => { pinItem.click(); });
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
    assert.deepEqual(requested, [!isPinned], `isPinned=${isPinned}: must request the next state ${!isPinned}`);
    await act(async () => root?.unmount());
    root = undefined;
    container.remove();
  }
});

test('destructive confirmation invokes the action once and keeps the menu open on rejection', async () => {
  let calls = 0;
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root?.render(<ChatHeader {...headerProps({
    onClearSession: async () => { calls += 1; throw new Error('network failed'); },
  })} />));
  await act(async () => { container.querySelector<HTMLButtonElement>('.chat-header__menu-btn')?.click(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  await act(async () => { document.querySelector<HTMLButtonElement>('[data-menu-action="clear"]')?.click(); });
  await act(async () => { document.querySelector<HTMLButtonElement>('.chat-header-confirm__btn.is-danger')?.click(); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  assert.equal(calls, 1, 'one explicit confirmation should invoke the callback exactly once');
  assert.ok(document.querySelector('.chat-header-confirm'), 'the failed action should leave confirmation available for retry');
  assert.match(document.querySelector('[role="alert"]')?.textContent ?? '', /network failed/);
});
