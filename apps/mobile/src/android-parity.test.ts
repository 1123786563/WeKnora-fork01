import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ANDROID_ACCEPTANCE_MATRIX, ANDROID_WORKFLOWS } from './android-acceptance.ts';

const here = dirname(fileURLToPath(import.meta.url));
const workspaceRoot = resolve(here, '..', '..', '..');

/** react-native 平台行为符号（计划 Global Constraints：「react-native Platform/BackHandler/AppState 等
 *  只允许出现在 apps/mobile/src/adapters/」）——UI 组件（View/Text/Button…）不在此列，screens 合法。 */
const PLATFORM_SYMBOLS = ['Platform', 'BackHandler', 'AppState', 'PermissionsAndroid', 'NativeModules', 'NativeEventEmitter', 'Linking'] as const;

/** CJS 整包 require 形态：react-native 的任何 require 接线（adapters 内合法先例形态）。 */
const REACT_NATIVE_REQUIRE_RE = /require\(['"]react-native['"]\)/;

/** ESM 命名导入中的平台行为符号（修复轮 1：计划原正则只匹配 require 形态，ESM 行为性导入
 *  `import { Platform } from 'react-native'` 完全绕过扫描）。`[^}]*` 含换行覆盖多行 import；
 *  `import type {` 不被 `import\s*\{` 命中——type-only 导入不引入运行时平台行为，保守不抓。 */
const PLATFORM_SYMBOL_IMPORT_RE = new RegExp(
  `import\\s*\\{[^}]*\\b(?:${PLATFORM_SYMBOLS.join('|')})\\b[^}]*\\}\\s*from\\s*['"]react-native['"]`,
);

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
    const source = readFileSync(file, 'utf8');
    const wired = REACT_NATIVE_REQUIRE_RE.test(source) || PLATFORM_SYMBOL_IMPORT_RE.test(source);
    if (!wired) continue;
    if (file.startsWith(adaptersDir)) adapterHits += 1;
    else offenders.push(relative(here, file));
  }
  assert.deepEqual(offenders, [], 'react-native 平台 API 只允许出现在 src/adapters/（module-seams §10）');
  assert.ok(adapterHits >= 1, 'sanity：扫描必须真的看到 adapters/ 中的合法平台接线（防扫描器失效静默通过）');
});

test('the platform-symbol scanner actually fires on ESM behavioral imports (修复轮 1：CJS-only 盲区)', () => {
  const mustFire = [
    "import { Platform } from 'react-native';",
    'import { BackHandler } from "react-native";',
    "import { AppState, View } from 'react-native';",
    "import { View, PermissionsAndroid }\n  from 'react-native';",
    "import { Linking, NativeModules, NativeEventEmitter } from 'react-native';",
  ];
  for (const sample of mustFire) {
    assert.equal(PLATFORM_SYMBOL_IMPORT_RE.test(sample), true, `守卫必须命中 ESM 平台符号导入（否则最惯用写法绕过扫描）：${sample}`);
  }
  const mustNotFire = [
    "import { View, Text } from 'react-native';", // UI 组件——screens 合法形态（实查 src 20 文件全是此类）
    'import { Button, ScrollView, Text, TextInput, View } from "react-native";',
    'import type { Platform as RNPlatform } from "react-native";', // type-only 不引入运行时平台行为
    "import { Platform } from './adapters/app-state.ts';", // 非 react-native 来源（from 子句精确匹配）
  ];
  for (const sample of mustNotFire) {
    assert.equal(PLATFORM_SYMBOL_IMPORT_RE.test(sample), false, `守卫不得误报合法形态：${sample}`);
  }
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
