import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ANDROID_ACCEPTANCE_MATRIX, ANDROID_WORKFLOWS } from './android-acceptance.ts';

const here = dirname(fileURLToPath(import.meta.url));
const workspaceRoot = resolve(here, '..', '..', '..');

function walkSources(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...walkSources(full));
    else if (/\.(ts|tsx)$/.test(entry.name) && !/\.test\.(ts|tsx)$/.test(entry.name)) out.push(full);
  }
  return out;
}

test('platform APIs stay inside src/adapters (验收标准 2：平台差异仅在 Adapter)', () => {
  const adaptersDir = join(here, 'adapters');
  const offenders: string[] = [];
  let adapterHits = 0;
  for (const file of walkSources(here)) {
    if (!/require\(['"]react-native['"]\)/.test(readFileSync(file, 'utf8'))) continue;
    if (file.startsWith(adaptersDir)) adapterHits += 1;
    else offenders.push(relative(here, file));
  }
  assert.deepEqual(offenders, [], 'react-native 平台 API 只允许出现在 src/adapters/（module-seams §10）');
  assert.ok(adapterHits >= 1, 'sanity：扫描必须真的看到 adapters/ 中的合法平台接线（防扫描器失效静默通过）');
});

test('screens and routes never import wire clients or shared contracts (module-seams §10)', () => {
  const offenders: string[] = [];
  for (const dir of ['screens', 'app']) {
    for (const file of walkSources(join(here, dir))) {
      if (/@weknora\/(api-client|contracts)/.test(readFileSync(file, 'utf8'))) offenders.push(relative(here, file));
    }
  }
  assert.deepEqual(offenders, [], 'Screen/路由禁止直连 wire 客户端或共享 contracts');
});

test('the Android acceptance matrix covers exactly the seven core workflows with real evidence paths (验收标准 3)', () => {
  assert.deepEqual(
    ANDROID_ACCEPTANCE_MATRIX.map((row) => row.workflow).sort(),
    [...ANDROID_WORKFLOWS].sort(),
    'What to build 的七项核心工作流一项不缺、一项不多',
  );
  for (const row of ANDROID_ACCEPTANCE_MATRIX) {
    assert.ok(
      row.adapterSeam.startsWith('apps/mobile/src/adapters/') || row.adapterSeam.startsWith('apps/mobile/src/app/'),
      `${row.workflow} 的平台 seam 必须落在 Adapter 层`,
    );
    assert.equal(existsSync(join(workspaceRoot, row.adapterSeam)), true, `${row.workflow} 的 seam 文件必须真实存在：${row.adapterSeam}`);
    assert.ok(row.localEvidence.length >= 1, `${row.workflow} 至少引用一条本地证据`);
    for (const evidence of row.localEvidence) {
      assert.equal(
        existsSync(join(workspaceRoot, evidence)),
        true,
        `${row.workflow} 引用的证据必须真实存在（引用计划产物冒充已验证即测试失败）：${evidence}`,
      );
    }
    assert.equal(row.residual.kind, 'blocked-env', `${row.workflow} 的真机残余恒为 blocked-env——绝不记为通过（验收标准 3）`);
    assert.ok(row.residual.reason.trim() !== '' && row.residual.unblock.trim() !== '', `${row.workflow} 必须写明阻塞原因与解锁条件`);
  }
});
