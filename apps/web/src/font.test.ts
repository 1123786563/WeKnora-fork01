// initFont / applyFontSizeZoom（apps/web/src/font.ts）——Vue useFont.ts applyFont
// 的 boot+change 语义：字号经 <html> CSS zoom 全站应用，reload 后保持。
// px2-settings-general-fontradio parity：React 侧此前只设无消费者的
// --wk-font-scale 变量，视觉不缩放。
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/settings' });
Object.assign(globalThis, { window: dom.window, document: dom.window.document });
const { initFont, applyFontSizeZoom, FONT_SCALES } = await import('./font.ts');
const { writeLocalPreferences } = await import('@weknora/domain/settings/local-preferences');

test('FONT_SCALES matches Vue useFont.FONT_SCALES verbatim', () => {
  assert.deepEqual(FONT_SCALES, { small: 0.875, normal: 1, large: 1.125 });
});

test('applyFontSizeZoom writes the scale as the html zoom (Vue applyFont useFont.ts:238)', () => {
  applyFontSizeZoom('large');
  assert.equal(document.documentElement.style.getPropertyValue('zoom'), String(1.125));
  applyFontSizeZoom('small');
  assert.equal(document.documentElement.style.getPropertyValue('zoom'), String(0.875));
  // normal 也显式写 1 —— Vue applyFont 无条件重写（reload 后 zoom 键恒在）。
  applyFontSizeZoom('normal');
  assert.equal(document.documentElement.style.getPropertyValue('zoom'), String(1));
  // 未知键回退 normal（Vue isFontSizeKey 守卫语义）。
  applyFontSizeZoom('bogus');
  assert.equal(document.documentElement.style.getPropertyValue('zoom'), String(1));
});

test('initFont applies the persisted per-user font size at boot (Vue main.ts:27 initFont)', () => {
  dom.window.localStorage.setItem('weknora_user', JSON.stringify({ id: 'u1' }));
  writeLocalPreferences(dom.window.localStorage, { fontSize: 'large' });
  initFont();
  assert.equal(document.documentElement.style.getPropertyValue('zoom'), String(1.125));
});

test('initFont falls back to zoom 1 when nothing is persisted', () => {
  dom.window.localStorage.clear();
  initFont();
  assert.equal(document.documentElement.style.getPropertyValue('zoom'), String(1));
});
