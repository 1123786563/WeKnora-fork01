import test from 'node:test';
import assert from 'node:assert/strict';
import { createResourceShelfController } from './resources-view.ts';
import type { ResourcePage, ResourceShelfHandle, ShelfInvalidationEvent } from '@weknora/mobile-core';

const PAGE: ResourcePage = {
  tenantId: '7',
  agents: [{ id: 'agent-1', name: 'Research', summary: '', kind: 'custom', capability: { state: 'supported', reason: '' } }],
  knowledge: [],
  connections: [],
  classVerdicts: { agent: { state: 'supported', reason: '' }, knowledge: { state: 'supported', reason: '' }, connection: { state: 'supported', reason: '' } },
};

function fakeHandle(pages: ResourcePage[]): ResourceShelfHandle & { emit(event: ShelfInvalidationEvent): void } {
  let next = 0;
  const listeners = new Set<(event: ShelfInvalidationEvent) => void>();
  return {
    async browse() {
      const page = pages[Math.min(next, pages.length - 1)]!;
      next += 1;
      return page;
    },
    selection: () => ({ allowed: false, state: 'unavailable', reason: 'not_used' }),
    subscribe(listener) { listeners.add(listener); return () => listeners.delete(listener); },
    close() {},
    emit(event) { for (const listener of [...listeners]) listener(event); },
  };
}

test('the controller loads the page and reloads on authorization revocation', async () => {
  const revoked: ResourcePage = {
    ...PAGE,
    knowledge: [],
    classVerdicts: { ...PAGE.classVerdicts, knowledge: { state: 'forbidden', reason: 'http_403' } },
  };
  const handle = fakeHandle([PAGE, revoked]);
  const controller = createResourceShelfController(handle);

  await controller.whenSettled();
  assert.equal(controller.state().page?.classVerdicts.knowledge.state, 'supported');

  handle.emit({ type: 'authorization-revoked', resourceClass: 'knowledge' });
  await controller.whenSettled();

  assert.equal(controller.state().page?.classVerdicts.knowledge.state, 'forbidden', 'revocation must replace the projection with server facts');
  assert.equal(controller.state().page?.knowledge.length, 0);
});

/** LIFO 解锁：resolveNext 总是解决最新一次在途 browse，便于构造「旧请求后返回」的迟到场景。 */
function deferredHandle(): ResourceShelfHandle & { resolveNext(page: ResourcePage): void } {
  const pending: Array<(page: ResourcePage) => void> = [];
  return {
    browse: () => new Promise<ResourcePage>((resolve) => { pending.push(resolve); }),
    selection: () => ({ allowed: false, state: 'unavailable', reason: 'not_used' }),
    subscribe: () => () => {},
    close() {},
    resolveNext(page) { pending.pop()?.(page); },
  };
}

test('a stale in-flight projection never overwrites a newer one', async () => {
  const handle = deferredHandle();
  const controller = createResourceShelfController(handle);
  const newer: ResourcePage = { ...PAGE, tenantId: '9' };

  const reloaded = controller.refresh();
  handle.resolveNext(newer); // 最新一次 browse 先返回
  await reloaded;
  handle.resolveNext(PAGE); // 迟到的旧结果必须被丢弃
  await new Promise((resolve) => setTimeout(resolve, 0));

  assert.equal(controller.state().page?.tenantId, '9');
});
