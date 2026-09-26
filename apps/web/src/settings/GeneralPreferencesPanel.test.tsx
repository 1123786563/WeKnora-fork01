import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { after, afterEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  HTMLInputElement: dom.window.HTMLInputElement,
  HTMLTextAreaElement: dom.window.HTMLTextAreaElement,
  HTMLSelectElement: dom.window.HTMLSelectElement,
  Element: dom.window.Element,
  Node: dom.window.Node,
  SVGElement: dom.window.SVGElement,
  DocumentFragment: dom.window.DocumentFragment,
  Event: dom.window.Event,
  KeyboardEvent: dom.window.KeyboardEvent,
  MouseEvent: dom.window.MouseEvent,
  MutationObserver: dom.window.MutationObserver,
  getComputedStyle: dom.window.getComputedStyle?.bind(dom.window),
  requestAnimationFrame: dom.window.requestAnimationFrame?.bind(dom.window) ?? ((cb: FrameRequestCallback) => setTimeout(cb, 16)),
  cancelAnimationFrame: dom.window.cancelAnimationFrame?.bind(dom.window) ?? clearTimeout,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
// tdesign 命令式 API（MessagePlugin 等）在 React 19 下需要适配器（main.tsx 引
// es/_util/react-19-adapter；node/tsx 下组件走 lib 入口，直接对 lib 的
// react-render 注入 createRoot——OllamaSettingsPanel.test 先例，da187f072）。
// **必须注入**：本面板的 change handlers 真实调用 MessagePlugin.success，无适配
// 器时 tdesign react-render 落到 React 19 已移除的 ReactDOM.render（undefined），
// 插件 promise 链坏死会阻断 node:test 子进程退出（pair 模式实测 3/3 挂起）。
{
  const { renderAdapter } = await import('tdesign-react/lib/_util/react-render.js');
  renderAdapter(createRoot);
}
// toast 断言不落测试（da187f072 ollama 判例同口径）：插件渲染是异步 promise 链，
// DOM 级断言竞态、调用级 spy 突变共享插件对象亦致子进程不退出；面板只依赖
// 「MessagePlugin.success(文案)」调用形态（与 OllamaSettingsPanel 一致且已被该
// 面板覆盖验证），此处只断言持久化与视觉变量应用。
const { GeneralPreferencesPanel } = await import('./GeneralPreferencesPanel.tsx');

// 本文件是仓库首个在 jsdom 测试里真实执行 MessagePlugin 渲染链的面板测试
// （tdesign message 容器 + 异步 unmount；模块级还引入 loading/plugin.js 副作用
// 链——旧版面板走自研 settings-toast，从不加载这些模块）。已实证两类 child 模式
// （node --test 多文件）不稳定：①测试结束后进程不退出（句柄滞留）——本 after()
// 的 unref 兜底定时器修复（结果经 IPC 流式上报完毕才触发，正常自然退出零影响）；
// ②机器高负载（load 40+，含并行会话套件自旋）时测试中段偶发停摆——负载回落后
// 0 复发（13/13 多轮稳定），与既有 agent-editor.test.tsx 环境性挂起（主 checkout
// 基线同样复现）同性质，留环境治理而非本文件可修。
after(() => {
  const guard = setTimeout(() => process.exit(0), 1000);
  guard.unref?.();
});
const { readLocalPreferences } = await import('@weknora/domain/settings/local-preferences');

let mountedRoot: Root | undefined;
afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  dom.window.localStorage.clear();
});

// T12a：面板平移为 TDesign Select/RadioGroup/Switch（= Vue t-select /
// t-radio-group / t-switch）。Toast 走 tdesign MessagePlugin（Vue
// GeneralSettings.vue:229/243/251/262/274 MessagePlugin.success 同构，
// da187f072 判例）。
async function mountPanel(liteMode = false) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<GeneralPreferencesPanel liteMode={liteMode} />);
  });
  return container;
}

/** tdesign Select 交互：点开 trigger，在 body 弹层里点文本匹配的选项。 */
async function pickOption(container: Element, selectIndex: number, optionText: string) {
  const trigger = container.querySelectorAll('.t-select__wrap')[selectIndex];
  assert.ok(trigger, 'select ' + selectIndex + ' missing');
  const inner = trigger.querySelector('.t-input') as HTMLElement;
  await act(async () => { inner.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true })); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  const options = Array.from(document.body.querySelectorAll<HTMLElement>('.t-select-option'));
  if (options.length === 0) {
    await act(async () => { inner.dispatchEvent(new dom.window.MouseEvent('mousedown', { bubbles: true, cancelable: true })); });
    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  }
  const target = Array.from(document.body.querySelectorAll<HTMLElement>('.t-select-option'))
    .find((el) => (el.textContent ?? '').trim() === optionText || (el.textContent ?? '').trim().startsWith(optionText));
  assert.ok(target, 'option missing: ' + optionText + ' (have ' + Array.from(document.body.querySelectorAll('.t-select-option')).map((el) => el.textContent).join(',') + ')');
  await act(async () => { target.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true })); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
}

/** 读 tdesign Select trigger 当前显示文本（闭合态只显示选中 label）。 */
function selectText(container: Element, selectIndex: number): string {
  const trigger = container.querySelectorAll('.t-select__wrap')[selectIndex];
  return (trigger?.querySelector('input.t-input__inner') as HTMLInputElement | null)?.value ?? '';
}

test('Lite mode exposes Vue auto-update switch and persists it without dropping other settings', async () => {
  dom.window.localStorage.setItem('WeKnora_settings', JSON.stringify({ selectedAgentId: 'agent-1', autoCheckUpdate: false }));
  const container = await mountPanel(true);
  // tdesign Switch（Vue t-switch 同构）：button[role=switch]，状态走
  // t-is-checked 类（台账 #2 根标签差异，无 aria-checked 属性）。
  const toggle = container.querySelector<HTMLButtonElement>('[role="switch"]');

  assert.ok(toggle, 'Lite GeneralSettings must expose the auto-update switch');
  assert.equal(toggle.classList.contains('t-is-checked'), false, 'switch starts unchecked');
  assert.ok(container.textContent?.includes('开启后自动检查并在后台下载最新版本安装包。'));

  await act(async () => toggle.click());

  assert.equal(toggle.classList.contains('t-is-checked'), true, 'switch flips on click');
  assert.deepEqual(JSON.parse(dom.window.localStorage.getItem('WeKnora_settings') ?? '{}'), {
    selectedAgentId: 'agent-1',
    autoCheckUpdate: true,
  });
});

test('non-Lite GeneralSettings does not expose the desktop-only auto-update control', async () => {
  const container = await mountPanel(false);
  assert.equal(container.querySelector('[role="switch"]'), null);
  assert.equal(container.textContent?.includes('自动检查更新'), false);
});

// Vue parity anchor: GeneralSettings.vue 字体大小 t-radio-group + the WeKnora
// global checked override (frontend/src/assets/theme/theme/theme.css:120-141,
// solid brand bg + white text). T12a：直译为 tdesign RadioGroup + Radio.Button
// —— DOM 与 Vue 逐字一致（div.t-radio-group > label.t-radio-button）。
test('font size radio group: selection moves t-is-checked and persists', async () => {
  const container = await mountPanel();
  const frame = container.querySelector('.t-radio-group');
  assert.ok(frame, 'the t-radio-group renders');
  assert.match(frame.className, /t-radio-group__outline/, 'outline variant like the Vue t-radio-group');
  const radios = Array.from(frame.querySelectorAll<HTMLLabelElement>('label.t-radio-button'));
  assert.equal(radios.length, 3);

  // Vue default size is normal (useFont.ts) → middle button checked.
  assert.ok(radios[1]!.classList.contains('t-is-checked'));
  assert.equal(radios[0]!.classList.contains('t-is-checked'), false);
  assert.equal(radios[2]!.classList.contains('t-is-checked'), false);
  assert.deepEqual(radios.map((radio) => radio.querySelector('.t-radio-button__label')?.textContent), ['小', '正常', '大']);

  await act(async () => {
    radios[2]!.click();
  });

  assert.ok(radios[2]!.classList.contains('t-is-checked'));
  assert.equal(radios[1]!.classList.contains('t-is-checked'), false);

  // Persisted like Vue useFont.setFontSize (localStorage-backed) and the font
  // scale CSS variable applied immediately for live preview parity.
  assert.equal(readLocalPreferences(dom.window.localStorage).fontSize, 'large');
  assert.equal(document.documentElement.style.getPropertyValue('--wk-font-scale'), String(1.125));
  // Vue useFont.applyFont（useFont.ts:238）把字号经 <html> CSS zoom 全站应用
  // （px2-settings-general-fontradio parity：单设 --wk-font-scale 无消费者、
  // 视觉不缩放）。
  assert.equal(document.documentElement.style.getPropertyValue('zoom'), String(1.125));
});

test('general preferences keeps four TDesign selects (language/theme/sans/mono)', async () => {
  const container = await mountPanel();
  const selects = Array.from(container.querySelectorAll('.t-select__wrap'));
  assert.equal(selects.length, 4);
  for (const select of selects) {
    assert.ok(select.querySelector('input.t-input__inner'), 'each select keeps its trigger input');
  }
});

test('applies persisted font preferences when the panel mounts', async () => {
  dom.window.localStorage.setItem('font_sans', 'georgia');
  dom.window.localStorage.setItem('font_mono', 'monaco');
  // Pre-namespacing flat key: adopted into the active user's namespace by the
  // startup migration, exactly like Vue preferenceStorage (login required —
  // anon skips migration and keeps its own WeKnora_anon_* namespace).
  dom.window.localStorage.setItem('weknora_user', JSON.stringify({ id: 'u1' }));
  dom.window.localStorage.setItem('weknora-font-size', 'large');
  const { migratePreferencesIntoUser, resetMigrationLatch } = await import('@weknora/domain/settings/local-preferences');
  resetMigrationLatch();
  migratePreferencesIntoUser(dom.window.localStorage);

  await mountPanel();

  assert.match(document.documentElement.style.getPropertyValue('--wk-font-sans'), /Georgia/);
  assert.match(document.documentElement.style.getPropertyValue('--wk-font-mono'), /Monaco/);
  assert.equal(document.documentElement.style.getPropertyValue('--wk-font-scale'), '1.125');
  assert.equal(document.documentElement.style.getPropertyValue('zoom'), '1.125');
});

// Vue GeneralSettings.vue handleLanguageChange 写 locale 并以新 locale 弹
// MessagePlugin.success(t('language.languageSaved'))——toast 呈现不在此断言
// （文件头注），此处断言持久化与 locale 事件广播。
test('language change persists the Vue locale key and broadcasts the change', async () => {
  const container = await mountPanel();
  const events: string[] = [];
  dom.window.addEventListener('weknora:locale-changed', () => events.push('fired'));

  await pickOption(container, 0, 'English');

  assert.equal(dom.window.localStorage.getItem('locale'), 'en-US');
  assert.equal(events.length, 1);
});

// Vue GeneralSettings.vue handleThemeChange 接受值时写 per-user theme 键并广播
// weknora:theme-changed（initTheme 监听重应用），拒绝值时静默回滚。
test('theme change persists under the per-user theme key', async () => {
  const container = await mountPanel();

  await pickOption(container, 1, '深色');

  assert.equal(readLocalPreferences(dom.window.localStorage).theme, 'dark');
});

// The shared i18n tables carry every font option label (font.sans.* /
// font.mono.*) in all 5 locales, mirroring frontend/src/i18n/locales/*.ts.
// jsdom's UA resolves to the linux platform set; pin the localized labels so a
// locale that loses its font keys (falling back to the zh table) fails here.
test('font option labels follow the active locale (en-US Vue table)', async () => {
  dom.window.localStorage.setItem('locale', 'en-US');
  const container = await mountPanel();

  const sansOptions = await openOptionTexts(container, 2);
  assert.equal(sansOptions.length, 4);
  assert.equal(sansOptions[0], 'System Default');
  assert.equal(sansOptions[sansOptions.length - 1], 'Generic Sans-Serif');
  const monoOptions = await openOptionTexts(container, 3);
  assert.equal(monoOptions.length, 4);
  assert.equal(monoOptions[monoOptions.length - 1], 'Generic Monospace');
});

test('font option labels follow the active locale (ja-JP Vue table)', async () => {
  dom.window.localStorage.setItem('locale', 'ja-JP');
  const container = await mountPanel();

  const sansOptions = await openOptionTexts(container, 2);
  assert.equal(sansOptions.length, 4);
  assert.equal(sansOptions[0], 'システムデフォルト');
  assert.equal(sansOptions[sansOptions.length - 1], '汎用サンセリフ');
});

/** 打开某个 select 的弹层并读全部选项文本（闭合态 t-select 只显示选中项）。 */
async function openOptionTexts(container: Element, selectIndex: number): Promise<string[]> {
  const trigger = container.querySelectorAll('.t-select__wrap')[selectIndex];
  assert.ok(trigger, 'select ' + selectIndex + ' missing');
  const inner = trigger.querySelector('.t-input') as HTMLElement;
  // 之前打开过的弹层可能仍挂在 body：按「本次新出现」过滤选项。
  const before = new Set(Array.from(document.body.querySelectorAll('.t-select-option')));
  await act(async () => { inner.dispatchEvent(new dom.window.MouseEvent('click', { bubbles: true, cancelable: true })); });
  await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)); });
  return Array.from(document.body.querySelectorAll<HTMLElement>('.t-select-option'))
    .filter((el) => !before.has(el))
    .map((el) => (el.textContent ?? '').trim());
}

test('font previews keep the Vue .font-preview anatomy', async () => {
  const container = await mountPanel();
  const previews = Array.from(container.querySelectorAll('[data-testid^="font-preview-"]'));

  assert.equal(previews.length, 2);
  for (const preview of previews) {
    assert.ok(preview.classList.contains('font-preview'), 'previews carry the Vue .font-preview class');
  }
  assert.ok(previews[1]!.classList.contains('font-preview--mono'), 'the mono preview keeps its --mono modifier');
});

// --- R441 A4: font selects read/write the per-user namespace (Vue
// preferenceStorage parity), not the pre-namespacing flat keys. ---

test('sans font change persists under WeKnora_{uid}_font_sans, never the flat key', async () => {
  dom.window.localStorage.setItem('weknora_user', JSON.stringify({ id: 'u1' }));
  const container = await mountPanel();

  // jsdom resolves to the linux platform set; pick a visible sans key.
  await pickOption(container, 2, 'Noto Sans CJK');

  assert.equal(dom.window.localStorage.getItem('WeKnora_u1_font_sans'), 'noto-cjk');
  assert.equal(dom.window.localStorage.getItem('font_sans'), null);
});

test('mono font change persists under WeKnora_{uid}_font_mono, never the flat key', async () => {
  dom.window.localStorage.setItem('weknora_user', JSON.stringify({ id: 'u1' }));
  const container = await mountPanel();

  await pickOption(container, 3, 'DejaVu Sans Mono');

  assert.equal(dom.window.localStorage.getItem('WeKnora_u1_font_mono'), 'dejavu-mono');
  assert.equal(dom.window.localStorage.getItem('font_mono'), null);
});

test('panel adopts legacy flat font keys at mount and never reads them back for user u2', async () => {
  // u1 changed fonts while logged in; the flat keys are u1-era leftovers.
  dom.window.localStorage.setItem('weknora_user', JSON.stringify({ id: 'u1' }));
  dom.window.localStorage.setItem('font_sans', 'georgia');
  dom.window.localStorage.setItem('font_mono', 'monaco');
  // Earlier tests in this process already consumed the module-level latch for
  // u1; reset it so this mount performs the adoption like a fresh login would.
  const { resetMigrationLatch } = await import('@weknora/domain/settings/local-preferences');
  resetMigrationLatch();
  await mountPanel();
  assert.equal(dom.window.localStorage.getItem('WeKnora_u1_font_sans'), 'georgia');
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();

  // u2 logs in on the same browser: u1's font choices must not surface, and
  // the flat leftovers must already have been consumed by the u1 migration.
  dom.window.localStorage.clear();
  dom.window.localStorage.setItem('weknora_user', JSON.stringify({ id: 'u2' }));
  dom.window.localStorage.setItem('font_sans', 'georgia');
  dom.window.localStorage.setItem('font_mono', 'monaco');
  const second = await mountPanel();

  // 闭合态 trigger：u2 挂载时对重新种下的 flat 键执行了自己的迁移（u2 命名
  // 空间 = georgia/monaco，与 Vue preferenceStorage 语义一致）；georgia 不在
  // linux 可见字体列表里，t-select 直接显示原始值（tdesign 两端一致——旧原生
  // select 会回退显示首项，是本测试历史上的 display artifact）。
  assert.equal(selectText(second, 2), 'georgia', 'u2 reads its own adopted namespace, not u1');
  assert.equal(dom.window.localStorage.getItem('WeKnora_u2_font_sans'), 'georgia');
});
