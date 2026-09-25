import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

// apps/miniprogram/tests → 上三级 = 仓库根。
const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

test('maintenance-mode gate: the legacy native miniprogram orchestrator tree is gone (replace-dont-layer)', () => {
  assert.equal(existsSync(path.join(repoRoot, 'miniprogram', 'app.json')), false, '旧原生编排树 miniprogram/ 必须整体删除，不得与新编排器长期并存');
  assert.equal(existsSync(path.join(repoRoot, 'miniprogram')), false, '目录本身也应消失（不留空壳）');
});

test('maintenance-mode gate: exactly one miniprogram orchestrator in the pnpm workspace', () => {
  const yaml = readFileSync(path.join(repoRoot, 'pnpm-workspace.yaml'), 'utf8');
  const entries = [...yaml.matchAll(/^\s*-\s+(\S+)\s*$/gm)].map(match => match[1]);
  const miniprogramEntries = entries.filter(entry => entry.includes('miniprogram'));
  assert.deepEqual(miniprogramEntries, ['apps/miniprogram']);
});

test('maintenance-mode gate: the surviving orchestrator consumes the deep modules', () => {
  const pkg = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8'));
  assert.ok(pkg.dependencies['@weknora/mobile-core'], 'apps/miniprogram 必须依赖 @weknora/mobile-core（深模块复用门槛）');
  assert.equal(pkg.dependencies['@weknora/mobile-core'], 'workspace:*');
});
