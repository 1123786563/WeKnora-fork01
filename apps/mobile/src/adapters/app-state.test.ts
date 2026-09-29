import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import * as nodeModule from 'node:module';
import { after, test } from 'node:test';

// react-native 在 Node 无真身：resolve hook 注入可控 stub（先例：voice-dictation-integration-smoke.test.ts）。
// AppState 由 getter 提供，行为经 globalThis.__RN_APPSTATE_MODE 在各测试内切换（无需重新 import）。
type ResolveNext = (specifier: string, context: unknown) => unknown;
type ResolveHook = (specifier: string, context: unknown, nextResolve: ResolveNext) => unknown;
const moduleWithHooks = nodeModule as typeof nodeModule & {
  registerHooks?: (hooks: { resolve: ResolveHook }) => void;
};

const stubDir = mkdtempSync(join(tmpdir(), 'weknora-appstate-stub-'));
const stubPath = join(stubDir, 'react-native.cjs');
writeFileSync(
  stubPath,
  `
const mode = () => globalThis.__RN_APPSTATE_MODE ?? 'ok';
module.exports = {
  get AppState() {
    if (mode() === 'throw') throw new Error('AppState unavailable');
    if (mode() === 'absent') return undefined;
    if (mode() === 'missing-listener') return {}; // 监听器方法缺失：走 adapter 的 typeof no-op 降级（RN 契约下 addEventListener 恒返回 subscription，不存在「方法在而返回 undefined」的真实形态）
    return {
      addEventListener(event, handler) {
        globalThis.__RN_APPSTATE_LAST = { event, handler };
        return { remove() { globalThis.__RN_APPSTATE_REMOVED = true; } };
      },
    };
  },
};
`,
);
if (moduleWithHooks.registerHooks) {
  moduleWithHooks.registerHooks({
    resolve: (specifier, context, nextResolve) =>
      specifier === 'react-native'
        ? { shortCircuit: true, url: pathToFileURL(stubPath).href }
        : nextResolve(specifier, context),
  });
}
after(() => {
  rmSync(stubDir, { recursive: true, force: true });
});

const loadMod = () => import('./app-state.ts');

test('foreground/background AppState changes map onto the two-state lifecycle contract (#70 后台限制)', async () => {
  const { createNativeAppStateLifecycle } = await loadMod();
  const seen: Array<'active' | 'background'> = [];
  const stop = createNativeAppStateLifecycle().subscribe((state) => seen.push(state));
  const last = (globalThis as { __RN_APPSTATE_LAST?: { event: string; handler: (state: string) => void } }).__RN_APPSTATE_LAST;
  assert.ok(last, 'subscribe 必须注册 AppState change 监听');
  assert.equal(last!.event, 'change');
  last!.handler('active');
  last!.handler('background');
  last!.handler('unknown'); // 未知状态保守视为 background（fail closed：不误触发前台同步）
  assert.deepEqual(seen, ['active', 'background', 'background']);
  (globalThis as { __RN_APPSTATE_REMOVED?: boolean }).__RN_APPSTATE_REMOVED = false;
  stop();
  assert.equal((globalThis as { __RN_APPSTATE_REMOVED?: boolean }).__RN_APPSTATE_REMOVED, true, '退订必须移除原生监听');
});

test('an unusable AppState degrades to a no-op subscription instead of crashing startup (#70 fail closed)', async () => {
  const { createNativeAppStateLifecycle } = await loadMod();
  for (const mode of ['absent', 'missing-listener', 'throw']) {
    (globalThis as { __RN_APPSTATE_MODE?: string }).__RN_APPSTATE_MODE = mode;
    const seen: string[] = [];
    const stop = createNativeAppStateLifecycle().subscribe((state) => seen.push(state));
    stop(); // no-op 退订必须可安全调用
    assert.deepEqual(seen, [], `${mode} 模式下不得产生事件`);
  }
  delete (globalThis as { __RN_APPSTATE_MODE?: string }).__RN_APPSTATE_MODE;
});
