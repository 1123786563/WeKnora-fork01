import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import { MaterialError } from './material-errors.ts';
import { createRecordingSharePort, createScenarioMaterialRemote, createScriptedBlobFetch } from './in-memory-material-remote.ts';
import { createTaskMaterial } from './task-material.ts';
import type { MaterialBackendArtifact, TaskMaterialPorts } from './ports.ts';

const SCOPE = { deploymentOrigin: 'https://weknora.example.test', userId: 'member-1', tenantId: 'tenant-1' };
const GRANT_URL = 'https://weknora.example.test/api/v1/workbench/artifacts/download?index=0';

function artifactRow(overrides: Partial<MaterialBackendArtifact> = {}): MaterialBackendArtifact {
  return { id: 'msg-1:0', index: 0, name: 'report.md', mime: 'text/markdown', version: 'aaaaaaaaaaaaaaaa', size: 12, sourceRun: 'run-1', ...overrides };
}

function openModule(script: {
  artifacts?: MaterialBackendArtifact[];
  terminalAvailable?: boolean;
  grantUrl?: string;
  grantError?: unknown;
  blobs?: Record<string, { bytes: Uint8Array; mime: string } | { error: unknown }>;
}) {
  const remote = createScenarioMaterialRemote({
    list: (runId) => ({ runId, artifacts: script.artifacts ?? [artifactRow()], terminalAvailable: script.terminalAvailable ?? true }),
    signedUrl: ({ index }) => {
      if (script.grantError !== undefined) throw script.grantError;
      return { url: script.grantUrl ?? GRANT_URL, expiresAt: '2026-09-24T01:00:00Z', artifact: artifactRow() };
    },
    terminalLog: ({ after }) => ({ lines: [{ seq: after + 1, occurredAt: '2026-09-24T00:00:01Z', stream: 'stdout', text: '$ ls\n' }], nextCursor: after + 1 }),
    events: () => [{ seq: 5, type: 'tool.started', occurredAt: '2026-09-24T00:00:05Z', payload: { tool: 'knowledge_search', knowledge_base_id: 'kb-7' } }],
  });
  const blob = createScriptedBlobFetch(script.blobs ?? { [GRANT_URL]: { bytes: new TextEncoder().encode('# report\n'), mime: 'text/markdown' } });
  const share = createRecordingSharePort();
  const ports: TaskMaterialPorts = { remote, blob, share };
  const lease = new RuntimeScopeLease(SCOPE);
  const handle = createTaskMaterial(ports).open({ lease: lease.asScopeLease() });
  return { handle, remote, blob, share, lease };
}

test('index projects kinds, immutable version identities and terminal availability', async () => {
  const { handle } = openModule({
    artifacts: [
      artifactRow(),
      artifactRow({ id: 'msg-1:1', index: 1, name: 'changes.diff', mime: 'text/x-diff', version: 'bbbbbbbbbbbbbbbb' }),
      artifactRow({ id: 'msg-2:0', index: 2, name: 'junit.xml', mime: 'application/xml', version: 'cccccccccccccccc' }),
    ],
  });
  const index = await handle.index({ runId: 'run-1' });
  assert.equal(index.terminal.available, true);
  assert.deepEqual(index.materials.map((entry) => entry.kind), ['artifact', 'diff', 'test-report']);
  assert.ok(index.materials.every((entry) => entry.version !== ''), '每项材料都有非空不可变版本身份（AC1 的 Interface 面）');
});

test('open previewable text fetches a fresh grant, pins origin and caches bytes by version', async () => {
  const { handle, remote, blob } = openModule({});
  const first = await handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(first.kind, 'artifact');
  assert.equal(first.kind === 'artifact' && first.text, '# report\n');
  const second = await handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(second.kind === 'artifact' && second.text, '# report\n');
  assert.equal(remote.grantCalls.length, 1, '字节按 (materialId, version) 缓存——缓存命中不再铸造 grant');
  assert.equal(blob.calls.length, 1);
});

test('cached bytes are keyed by (materialId, version); a version change forces a fresh grant and fetch', async () => {
  // 同一句柄内：服务端对同一 materialId 发布新版本身份（内容变化 → 版本变化，AC1 的 Interface 面）。
  let version = 'aaaaaaaaaaaaaaaa';
  const remote = createScenarioMaterialRemote({
    list: (runId) => ({ runId, artifacts: [artifactRow({ version })], terminalAvailable: true }),
    signedUrl: ({ index }) => ({ url: GRANT_URL, expiresAt: '2026-09-24T01:00:00Z', artifact: artifactRow({ version, index }) }),
    terminalLog: ({ after }) => ({ lines: [], nextCursor: after }),
    events: () => [],
  });
  const blob = createScriptedBlobFetch({ [GRANT_URL]: { bytes: new TextEncoder().encode('# report\n'), mime: 'text/markdown' } });
  const lease = new RuntimeScopeLease(SCOPE);
  const handle = createTaskMaterial({ remote, blob, share: createRecordingSharePort() }).open({ lease: lease.asScopeLease() });

  const first = await handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(first.kind === 'artifact' && first.text, '# report\n');
  assert.equal(remote.grantCalls.length, 1);
  assert.equal(blob.calls.length, 1);

  version = 'dddddddddddddddd';
  await handle.index({ runId: 'run-1' }); // 重新载入索引 → 新版本身份进入条目
  const second = await handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(second.kind === 'artifact' && second.entry.version, 'dddddddddddddddd');
  assert.equal(remote.grantCalls.length, 2, '版本身份变化必须重新铸造 grant 并抓取（不得复用旧字节）');
  assert.equal(blob.calls.length, 2);
});

test('an unsupported mime or oversized material never mints a grant and never fetches', async () => {
  const oversized = openModule({ artifacts: [artifactRow({ name: 'big.csv', mime: 'text/csv', size: 3 * 1024 * 1024 })] });
  const bySize = await oversized.handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-1:0' });
  assert.deepEqual(bySize.kind === 'artifact' && bySize.preview, { state: 'unsupported', reason: 'size' });
  const byMime = openModule({ artifacts: [artifactRow({ name: 'bundle.zip', mime: 'application/zip', size: 5 })] });
  const view = await byMime.handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-1:0' });
  assert.deepEqual(view.kind === 'artifact' && view.preview, { state: 'unsupported', reason: 'mime' });
  assert.equal(oversized.remote.grantCalls.length + byMime.remote.grantCalls.length, 0, 'unsupported 判定先于一切网络调用（Review Focus 5）');
  assert.equal(oversized.blob.calls.length + byMime.blob.calls.length, 0);
});

test('open parses unified diff hunks and falls back to raw on malformed', async () => {
  const diffBytes = new TextEncoder().encode('--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n');
  const good = openModule({
    artifacts: [artifactRow({ name: 'changes.diff', mime: 'text/x-diff' })],
    blobs: { [GRANT_URL]: { bytes: diffBytes, mime: 'text/x-diff' } },
  });
  const view = await good.handle.open({ kind: 'diff', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(view.kind, 'diff');
  assert.equal(view.malformed, false);
  assert.deepEqual(view.hunks[0]!.lines.map((line) => line.origin), ['remove', 'add']);

  const broken = openModule({
    artifacts: [artifactRow({ name: 'weird.diff', mime: 'text/x-diff' })],
    blobs: { [GRANT_URL]: { bytes: new TextEncoder().encode('not a diff\n'), mime: 'text/x-diff' } },
  });
  const raw = await broken.handle.open({ kind: 'diff', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(raw.kind, 'diff');
  assert.equal(raw.kind === 'diff' && raw.malformed, true);
  assert.equal(raw.kind === 'diff' && raw.raw, 'not a diff\n');
});

test('open evidence projects citations and open terminal pages read-only lines', async () => {
  const { handle } = openModule({});
  const evidence = await handle.open({ kind: 'evidence', runId: 'run-1' });
  assert.equal(evidence.kind, 'evidence');
  assert.equal(evidence.citations[0]!.source, 'kb-7');

  const terminal = await handle.open({ kind: 'terminal', runId: 'run-1' });
  assert.equal(terminal.kind === 'terminal' && terminal.readOnly, true, '终端视图恒只读（AC2）');
  assert.equal(terminal.kind === 'terminal' && terminal.lines.length, 1);
  const page2 = await handle.open({ kind: 'terminal', runId: 'run-1', cursor: terminal.kind === 'terminal' ? terminal.nextCursor : 0 });
  assert.equal(page2.kind === 'terminal' && page2.lines.length, 1, '从 nextCursor 续页');
});

test('act download mints a fresh grant every time and act share goes through the share port', async () => {
  const { handle, remote, share } = openModule({});
  const one = await handle.act({ kind: 'download', runId: 'run-1', materialId: 'msg-1:0' });
  const two = await handle.act({ kind: 'download', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(one.kind === 'grant' && one.url, GRANT_URL);
  assert.equal(one.kind === 'grant' && one.expiresAt, '2026-09-24T01:00:00Z');
  assert.equal(remote.grantCalls.length, 2, '每次 act 都新鲜铸造——签名 URL 绝不缓存复用（AC1）');
  const shared = await handle.act({ kind: 'share', runId: 'run-1', materialId: 'msg-1:0' });
  assert.equal(shared.kind, 'shared');
  assert.deepEqual(share.shared, [{ url: GRANT_URL, name: 'report.md' }]);
  assert.equal(remote.grantCalls.length, 3, '分享同样走新鲜 grant');
});

test('act terminal-input is always rejected with MATERIAL_TERMINAL_READ_ONLY (AC2)', async () => {
  const { handle } = openModule({});
  await assert.rejects(
    () => handle.act({ kind: 'terminal-input', text: 'rm -rf /' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_TERMINAL_READ_ONLY',
  );
});

test('a grant that expired during a blob fetch maps to MATERIAL_GRANT_EXPIRED and notifies subscribers (AC2)', async () => {
  const expired = new Error('401') as Error & { status?: number; code?: string };
  expired.status = 401;
  expired.code = 'artifact_grant_expired';
  const { handle } = openModule({ blobs: { [GRANT_URL]: { error: expired } } });
  const events: Array<{ type: string; materialId?: string }> = [];
  handle.subscribe((event) => events.push(event));
  await assert.rejects(
    () => handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-1:0' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_GRANT_EXPIRED',
  );
  assert.deepEqual(events, [{ type: 'grant-expired', materialId: 'msg-1:0' }], '订阅者收到失效事件以驱动重新授权');
});

test('signing disabled and invalid grants map to dedicated codes', async () => {
  const disabled = new Error('501') as Error & { status?: number; code?: string };
  disabled.status = 501;
  disabled.code = 'artifact_signing_disabled';
  const off = openModule({ grantError: disabled });
  await assert.rejects(
    () => off.handle.act({ kind: 'download', runId: 'run-1', materialId: 'msg-1:0' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_SIGNING_DISABLED',
  );

  const invalid = new Error('401') as Error & { status?: number; code?: string };
  invalid.status = 401;
  invalid.code = 'artifact_grant_invalid';
  const tampered = openModule({ grantError: invalid });
  await assert.rejects(
    () => tampered.handle.act({ kind: 'download', runId: 'run-1', materialId: 'msg-1:0' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_GRANT_INVALID',
  );
});

test('a grant url whose origin differs from the active deployment fails closed before any fetch', async () => {
  const foreign = openModule({ grantUrl: 'https://evil.example.test/api/v1/workbench/artifacts/download?index=0' });
  await assert.rejects(
    () => foreign.handle.act({ kind: 'download', runId: 'run-1', materialId: 'msg-1:0' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_GRANT_ORIGIN',
  );
  assert.equal(foreign.blob.calls.length, 0, 'origin 不一致时零字节流量（Review Focus 2）');
});

test('a revoked lease rejects subsequent calls and drops late results', async () => {
  const { handle, lease } = openModule({});
  lease.revoke();
  await assert.rejects(
    () => handle.index({ runId: 'run-1' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_SCOPE_CHANGED',
  );
  await assert.rejects(
    () => handle.act({ kind: 'download', runId: 'run-1', materialId: 'msg-1:0' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_SCOPE_CHANGED',
  );
});

test('unknown material ids answer MATERIAL_NOT_FOUND and close answers MATERIAL_CLOSED', async () => {
  const { handle } = openModule({});
  await assert.rejects(
    () => handle.open({ kind: 'artifact', runId: 'run-1', materialId: 'msg-9:9' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_NOT_FOUND',
  );
  handle.close('test-done');
  await assert.rejects(
    () => handle.index({ runId: 'run-1' }),
    (error: unknown) => error instanceof MaterialError && error.code === 'MATERIAL_CLOSED',
  );
});

test('a failed index load clears the previous projection instead of serving stale entries', async () => {
  const remote = createScenarioMaterialRemote({
    list: (() => {
      let failed = false;
      return (runId: string) => {
        if (failed) throw new Error('backend down');
        failed = true;
        return { runId, artifacts: [artifactRow()], terminalAvailable: true };
      };
    })(),
  });
  const lease = new RuntimeScopeLease(SCOPE);
  const handle = createTaskMaterial({ remote, blob: createScriptedBlobFetch({}), share: createRecordingSharePort() }).open({ lease: lease.asScopeLease() });
  const first = await handle.index({ runId: 'run-1' });
  assert.equal(first.materials.length, 1);
  await assert.rejects(() => handle.index({ runId: 'run-1' }), /MATERIAL_BACKEND/);
});
