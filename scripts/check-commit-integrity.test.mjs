/**
 * Tests for scripts/check-commit-integrity.mjs (R460 batch 2).
 * Uses temporary on-disk fixtures so glob expansion and existence probes run
 * against real files. Run: node --test scripts/check-commit-integrity.test.mjs
 */
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {
  collectRelativeImports,
  resolveImportCandidates,
  checkImports,
  extractGlobsFromScript,
  checkTestCoverage,
  parseDirtySharedFiles,
} from './check-commit-integrity.mjs';

function makeFixtureRoot() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'weknora-integrity-'));
}
function writeRel(root, rel, content) {
  fs.mkdirSync(path.dirname(path.join(root, rel)), { recursive: true });
  fs.writeFileSync(path.join(root, rel), content);
}

/* ------------------------------------------------------------------ */
/* import extraction + candidate resolution                            */
/* ------------------------------------------------------------------ */

test('collectRelativeImports picks up all import forms and skips comments/bare specifiers', () => {
  const src = [
    "import { useState } from 'react';", // bare: ignored
    "import { x } from './util';",
    "import './styles.css';",
    "import data from '../shared/data.json';",
    "export { y } from './util';",
    "const dyn = () => import('./lazy');",
    "const r = require('./cjs-thing');",
    "// import './commented-out';",
    "/* import './also-commented'; */",
    "import type { T } from '@weknora/domain';", // bare: ignored
  ].join('\n');
  const found = collectRelativeImports(src);
  assert.deepEqual(
    found.map((f) => f.specifier),
    ['./util', './styles.css', '../shared/data.json', './util', './lazy', './cjs-thing'],
  );
  assert.deepEqual(
    found.map((f) => f.line),
    [2, 3, 4, 5, 6, 7],
  );
});

test('resolveImportCandidates tries extensions and index files', () => {
  const cands = resolveImportCandidates('packages/ui/src/button/index.tsx', './icon');
  assert.ok(cands.includes('packages/ui/src/button/icon'));
  assert.ok(cands.includes('packages/ui/src/button/icon.ts'));
  assert.ok(cands.includes('packages/ui/src/button/icon/index.ts'));
  const asset = resolveImportCandidates('apps/web/src/main.tsx', './logo.svg');
  assert.deepEqual(asset, ['apps/web/src/main.svg'.replace('main', 'logo')]);
});

/* ------------------------------------------------------------------ */
/* check 1: import resolution vs git index                             */
/* ------------------------------------------------------------------ */

test('checkImports: pass case -- every relative import tracked in the index', () => {
  const root = makeFixtureRoot();
  writeRel(root, 'packages/a/src/util.ts', 'export const x = 1;\n');
  writeRel(root, 'packages/a/src/asset.svg', '<svg/>\n');
  writeRel(root, 'packages/a/src/data.json', '{}\n');
  writeRel(root, 'packages/a/src/sub/index.ts', 'export const s = 2;\n');
  writeRel(
    root,
    'packages/a/src/main.ts',
    [
      "import { x } from './util';",
      "import './asset.svg';",
      "import data from './data.json';",
      "import { s } from './sub/index';",
      "import { useState } from 'react';",
      "export { x } from './util';",
    ].join('\n'),
  );
  const trackedSet = new Set([
    'packages/a/src/util.ts',
    'packages/a/src/asset.svg',
    'packages/a/src/data.json',
    'packages/a/src/sub/index.ts',
    'packages/a/src/main.ts',
  ]);
  const result = checkImports({
    sourceFiles: [{ path: 'packages/a/src/main.ts', content: fs.readFileSync(path.join(root, 'packages/a/src/main.ts'), 'utf8') }],
    trackedSet,
    root,
  });
  assert.equal(result.findings.length, 0);
  assert.equal(result.importCount, 5);
});

test('checkImports: reports untracked working-tree targets (missed-commit incident) and fully unresolved imports', () => {
  const root = makeFixtureRoot();
  writeRel(root, 'packages/a/src/util.ts', 'export const x = 1;\n');
  writeRel(root, 'packages/a/src/never-added.ts', 'export const u = 1;\n'); // on disk, NOT tracked
  writeRel(root, 'packages/a/src/generated.ts', 'export const g = 1;\n'); // on disk, gitignored
  writeRel(
    root,
    'packages/a/src/main.ts',
    [
      "import { x } from './util';", // ok
      "import { u } from './never-added';", // P0: working tree only
      "import { m } from './missing';", // P0: nowhere
      "import { g } from './generated';", // WARN: gitignored target
    ].join('\n'),
  );
  const trackedSet = new Set(['packages/a/src/util.ts', 'packages/a/src/main.ts']);
  const result = checkImports({
    sourceFiles: [{ path: 'packages/a/src/main.ts', content: fs.readFileSync(path.join(root, 'packages/a/src/main.ts'), 'utf8') }],
    trackedSet,
    root,
    isIgnored: (p) => p === 'packages/a/src/generated.ts',
  });
  assert.equal(result.findings.length, 3);
  const notInIndex = result.findings.find((f) => f.kind === 'import-target-not-in-index');
  assert.equal(notInIndex.severity, 'P0');
  assert.equal(notInIndex.file, 'packages/a/src/main.ts');
  assert.equal(notInIndex.line, 2);
  assert.ok(notInIndex.detail.includes('never-added.ts'));
  const unresolved = result.findings.find((f) => f.kind === 'import-unresolved');
  assert.equal(unresolved.severity, 'P0');
  assert.equal(unresolved.specifier, './missing');
  const ignored = result.findings.find((f) => f.kind === 'import-target-ignored');
  assert.equal(ignored.severity, 'WARN');
});

test('checkImports: node_modules targets downgrade to WARN (dependency path, not a missed commit)', () => {
  const root = makeFixtureRoot();
  writeRel(root, 'packages/a/node_modules/react-dom/server.js', 'export const r = 1;\n');
  writeRel(root, 'packages/a/src/main.ts', "import { r } from '../node_modules/react-dom/server.js';\n");
  const trackedSet = new Set(['packages/a/src/main.ts']);
  const result = checkImports({
    sourceFiles: [{ path: 'packages/a/src/main.ts', content: fs.readFileSync(path.join(root, 'packages/a/src/main.ts'), 'utf8') }],
    trackedSet,
    root,
  });
  assert.equal(result.findings.length, 1);
  assert.equal(result.findings[0].kind, 'import-target-node-modules');
  assert.equal(result.findings[0].severity, 'WARN');
});

/* ------------------------------------------------------------------ */
/* check 2: test glob coverage                                         */
/* ------------------------------------------------------------------ */

test('checkTestCoverage: flags files outside test:shared and package test globs, passes covered ones', () => {
  const root = makeFixtureRoot();
  writeRel(root, 'packages/contracts/test/contracts.test.ts', 'test("c", () => {});\n');
  writeRel(root, 'packages/ui/src/top.test.ts', 'test("t", () => {});\n');
  writeRel(root, 'packages/ui/src/deep/nested.test.ts', 'test("n", () => {});\n'); // uncovered
  writeRel(root, 'packages/i18n/test/i18n.test.ts', 'test("i", () => {});\n');
  writeRel(root, 'apps/web/src/app.test.tsx', 'test("w", () => {});\n');
  writeRel(root, 'apps/web/src/chat/deep.test.tsx', 'test("w2", () => {});\n');

  const rootScript =
    'tsx --test packages/contracts/test/*.test.ts packages/ui/src/*.test.ts packages/i18n/test/*.test.ts';
  const packageScripts = [
    // quoted globs relative to apps/web; ** must cover nested deep.test.tsx
    { dir: 'apps/web', script: "node --import tsx --test 'src/**/*.test.ts' 'src/**/*.test.tsx'" },
  ];
  const testFiles = [
    'packages/contracts/test/contracts.test.ts',
    'packages/ui/src/top.test.ts',
    'packages/ui/src/deep/nested.test.ts',
    'packages/i18n/test/i18n.test.ts',
    'apps/web/src/app.test.tsx',
    'apps/web/src/chat/deep.test.tsx',
  ];
  const { uncovered, covered } = checkTestCoverage({ testFiles, rootScript, packageScripts, root });
  assert.deepEqual(uncovered, ['packages/ui/src/deep/nested.test.ts']);
  assert.equal(covered.length, 5);
});

test('extractGlobsFromScript: handles bare and quoted glob tokens', () => {
  assert.deepEqual(extractGlobsFromScript('tsx --test packages/a/test/*.test.ts'), [
    'packages/a/test/*.test.ts',
  ]);
  assert.deepEqual(
    extractGlobsFromScript("node --import tsx --test 'src/**/*.test.ts' \"src/**/*.test.tsx\""),
    ['src/**/*.test.ts', 'src/**/*.test.tsx'],
  );
  assert.deepEqual(extractGlobsFromScript('echo none'), []);
  assert.deepEqual(extractGlobsFromScript(undefined), []);
});

/* ------------------------------------------------------------------ */
/* check 3: dirty shared files                                         */
/* ------------------------------------------------------------------ */

test('parseDirtySharedFiles: lists modified packages/** entries and ignores the rest', () => {
  const status = [
    ' M packages/ui/src/button.tsx',
    'M  apps/web/src/x.ts',
    'MM packages/i18n/src/i.ts',
    '?? packages/scratchpad/local.ts',
    'A  packages/core/src/added.ts',
    'R  packages/old/a.ts -> packages/ui/src/renamed.ts',
    ' M frontend/src/y.vue',
    ' M "packages/quoted space/file.ts"',
    '',
  ].join('\n');
  const dirty = parseDirtySharedFiles(status);
  assert.deepEqual(
    dirty.map((d) => d.path),
    ['packages/ui/src/button.tsx', 'packages/i18n/src/i.ts', 'packages/quoted space/file.ts'],
  );
  assert.equal(dirty[0].status, 'M');
  assert.equal(dirty[1].status, 'MM');
});

test('parseDirtySharedFiles: empty status yields empty list', () => {
  assert.deepEqual(parseDirtySharedFiles(''), []);
});
