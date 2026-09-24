import test from 'node:test';
import assert from 'node:assert/strict';
import { MaterialError } from '@weknora/mobile-core';
import type { MaterialIndex, MaterialRef, MaterialIntent, TaskMaterialHandle } from '@weknora/mobile-core';
import { createMaterialsController, MATERIAL_ERROR_COPY } from './materials-view.ts';

const GRANT_URL = 'https://weknora.example.test/api/v1/workbench/artifacts/download?index=0';

function fakeHandle(script: { grantError?: unknown } = {}): TaskMaterialHandle {
  const index: MaterialIndex = {
    runId: 'run-1',
    materials: [
      { materialId: 'msg-1:0', index: 0, kind: 'artifact', name: 'report.md', mime: 'text/markdown', size: 9, version: 'aaaaaaaaaaaaaaaa', sourceRun: 'run-1' },
      { materialId: 'msg-1:1', index: 1, kind: 'diff', name: 'changes.diff', mime: 'text/x-diff', size: 30, version: 'bbbbbbbbbbbbbbbb', sourceRun: 'run-1' },
    ],
    terminal: { available: true },
  };
  return {
    async index() { return index; },
    async open(ref: MaterialRef) {
      if (ref.kind === 'terminal') {
        return { kind: 'terminal', lines: [{ seq: 1, occurredAt: '2026-09-24T00:00:01Z', stream: 'stdout', text: '$ ls\n' }], nextCursor: 1, readOnly: true };
      }
      if (ref.kind === 'evidence') {
        return { kind: 'evidence', citations: [{ seq: 5, occurredAt: '2026-09-24T00:00:05Z', type: 'tool.started', detail: '{"tool":"shell"}' }] };
      }
      if (ref.materialId === 'msg-1:9') throw new MaterialError('MATERIAL_NOT_FOUND');
      const entry = index.materials[0]!;
      return { kind: 'artifact', entry, preview: { state: 'supported' }, text: '# report\n' };
    },
    async act(intent: MaterialIntent) {
      if (intent.kind === 'terminal-input') throw new MaterialError('MATERIAL_TERMINAL_READ_ONLY');
      // 模块合同（Task 6 mapError）：传输错误在模块边界翻译为 MaterialError 上抛——
      // 测试替身据此只抛 MaterialError，不模拟原始传输错误形态。
      if (script.grantError !== undefined) throw script.grantError;
      return { kind: 'grant', materialId: 'msg-1:0', url: GRANT_URL, expiresAt: '2026-09-24T01:00:00Z' };
    },
    subscribe() { return () => {}; },
    close() {},
  };
}

test('the controller loads the index, opens views and records grants', async () => {
  const controller = createMaterialsController(fakeHandle(), { runId: 'run-1' });
  await controller.load();
  assert.equal(controller.state().index?.materials.length, 2);
  await controller.openMaterial('msg-1:0');
  assert.equal(controller.state().view?.kind, 'artifact');
  await controller.openTerminal();
  assert.equal(controller.state().view?.kind, 'terminal');
  await controller.openEvidence();
  assert.equal(controller.state().view?.kind, 'evidence');
  await controller.download('msg-1:0');
  assert.equal(controller.state().grant?.url, GRANT_URL);
  assert.equal(controller.state().grant?.name, 'report.md');
  await controller.share('msg-1:0');
  assert.equal(controller.state().error, undefined);
});

test('material error codes map to user copy, never raw internals', async () => {
  assert.equal(MATERIAL_ERROR_COPY.MATERIAL_GRANT_EXPIRED, '下载授权已过期，请重新获取。');
  assert.equal(MATERIAL_ERROR_COPY.MATERIAL_TERMINAL_READ_ONLY, '终端为只读，不能输入。');
  // 501 artifact_signing_disabled 由模块 mapError 翻译为 MATERIAL_SIGNING_DISABLED；
  // 控制器只映射 MaterialError（messageOf），故替身按模块合同抛 MaterialError。
  const controller = createMaterialsController(fakeHandle({ grantError: new MaterialError('MATERIAL_SIGNING_DISABLED') }), { runId: 'run-1' });
  await controller.load();
  await controller.download('msg-1:0');
  assert.equal(controller.state().error, '此部署未配置签名密钥，暂无法下载（请联系管理员）。');
  await controller.openMaterial('msg-1:9');
  assert.equal(controller.state().error, '该材料已不存在，请刷新列表。');
});

test('dispose closes the handle exactly once', async () => {
  const handle = fakeHandle();
  let closes = 0;
  const countingHandle: TaskMaterialHandle = { ...handle, close() { closes += 1; } };
  const controller = createMaterialsController(countingHandle, { runId: 'run-1' });
  await controller.load();
  assert.equal(closes, 0, 'close must not fire before dispose');
  controller.dispose();
  controller.dispose(); // 幂等
  assert.equal(closes, 1, 'double dispose must close the handle exactly once');
  assert.equal(controller.state().loading, false);
});
