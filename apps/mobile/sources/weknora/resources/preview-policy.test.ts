/**
 * W27 — 隔离预览策略（isolated preview policy）。
 *
 * `previewHeaders()` 是客户端持有的「测试对照」常量：服务端
 * `internal/handler/artifact_preview.go` 的预览响应必须设置完全相同的
 * 策略头（handler 测试逐字节断言同一份 CSP）。这里的断言锁定语义：
 *
 *   - connect-src 'none'     → 预览 HTML 无法 fetch/XHR/WebSocket 任何
 *                              产品 API 或任意网络端点；
 *   - sandbox allow-scripts  → 脚本可运行（图表渲染需要）但文档处于
 *                              opaque origin：读不到 cookie、localStorage
 *                              与任何同源存储（无 allow-same-origin）；
 *   - Referrer-Policy        → 不向预览内容泄露产品来源；
 *   - nosniff / no-store     → 不做 MIME 嗅探、不落任何缓存。
 *
 * 同文件还锁定 WebView 侧的导航判定与配置（ArtifactPreview.tsx 的纯函数
 * 来源），两层共同构成「预览无法触达产品凭据」的可判定验收。
 */
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  previewHeaders,
  PREVIEW_CSP,
  previewOriginOf,
  decidePreviewNavigation,
  buildPreviewWebViewConfig,
} from './preview-policy.ts';

test('preview cannot use product credentials or connect to APIs', () => {
  const h = previewHeaders();
  assert.match(h['Content-Security-Policy'], /connect-src 'none'/);
  assert.match(h['Content-Security-Policy'], /sandbox allow-scripts/);
  assert.equal(h['Referrer-Policy'], 'no-referrer');
});

test('preview policy blocks storage, forms, embedding and sniffing', () => {
  const h = previewHeaders();
  const csp = h['Content-Security-Policy'];
  // sandbox 无 allow-same-origin：cookie/localStorage 在 opaque origin 下不可读。
  assert.ok(!/allow-same-origin/.test(csp), 'sandbox must not re-grant same-origin storage');
  assert.match(csp, /form-action 'none'/);
  assert.match(csp, /frame-ancestors 'none'/);
  assert.match(csp, /default-src 'none'/);
  assert.equal(h['X-Content-Type-Options'], 'nosniff');
  assert.equal(h['Cache-Control'], 'private, no-store');
});

test('policy constants are immutable snapshots (server contract parity)', () => {
  // 服务端 handler 输出同一份常量；任何改动都必须是两侧同时、显式的。
  assert.equal(PREVIEW_CSP, previewHeaders()['Content-Security-Policy']);
  const first = previewHeaders();
  const second = previewHeaders();
  assert.deepEqual(Object.keys(first).sort(), Object.keys(second).sort());
});

test('preview origin is derived from the issued preview URL only', () => {
  assert.equal(previewOriginOf('https://preview.example.com/ap/TOKEN'), 'https://preview.example.com');
  assert.equal(previewOriginOf('https://preview.example.com:8443/ap/TOKEN'), 'https://preview.example.com:8443');
  assert.equal(previewOriginOf('https://preview.example.com/ap/TOKEN?x=1'), 'https://preview.example.com');
  assert.equal(previewOriginOf(''), '');
  assert.equal(previewOriginOf('not a url'), '');
});

test('navigation to anything but the isolated preview origin is denied', () => {
  const origin = 'https://preview.example.com';
  // 产品 API（同预览页内脚本尝试 fetch 产品端点后的顶层导航形态）
  assert.equal(decidePreviewNavigation({ url: 'https://api.example.com/api/v1/sessions' }, origin).allow, false);
  // 任意第三方 origin
  assert.equal(decidePreviewNavigation({ url: 'https://evil.example/steal' }, origin).allow, false);
  // file / content / intent / javascript / data 等 WebView 危险 scheme
  for (const url of [
    'file:///etc/passwd',
    'content://android.provider/contacts',
    'intent://evil/#Intent;scheme=https;end',
    'javascript:alert(1)',
    'data:text/html,<script>fetch("/api/v1/keys")</script>',
    'about:blank',
  ]) {
    const decision = decidePreviewNavigation({ url }, origin);
    assert.equal(decision.allow, false, `${url} must be denied`);
    assert.equal(decision.external, undefined, `${url} must not be handed to the system browser`);
  }
  // 预览 origin 自身（票据路径）放行
  assert.deepEqual(decidePreviewNavigation({ url: 'https://preview.example.com/ap/TOKEN' }, origin), { allow: true });
  assert.deepEqual(
    decidePreviewNavigation({ url: 'https://preview.example.com/ap/TOKEN', isTopLevel: true }, origin),
    { allow: true },
  );
  // 跨 origin 的 http 链接：拒绝导航并标记候选——候选只表示「可经显式
  // 用户手势入口外抛」，被拦截导航本身绝不自动打开系统浏览器（F2）。
  assert.deepEqual(decidePreviewNavigation({ url: 'https://docs.example.com/guide' }, origin), {
    allow: false,
    external: 'https://docs.example.com/guide',
  });
});

test('webview config hardens the preview surface', () => {
  const config = buildPreviewWebViewConfig('https://preview.example.com/ap/TOKEN');
  // originAllowList 仅预览 origin
  assert.deepEqual(config.originWhitelist, ['https://preview.example.com']);
  // 关闭任意文件访问
  assert.equal(config.allowFileAccess, false);
  assert.equal(config.allowFileAccessFromFileURLs, false);
  assert.equal(config.allowUniversalAccessFromFileURLs, false);
  // 无原生桥：不注入脚本、不接收 postMessage
  assert.equal(config.injectedJavaScript, undefined);
  assert.equal(config.hasMessageBridge, false);
  // cookie 隔离
  assert.equal(config.sharedCookiesEnabled, false);
  assert.equal(config.thirdPartyCookiesEnabled, false);
  assert.equal(config.setSupportMultipleWindows, false);
  // 预览 origin 缺失（票据 URL 非法）时 fail-closed：空 allowlist 会让
  // 一切导航被拒，WebView 不应加载任何内容。
  assert.deepEqual(buildPreviewWebViewConfig('::bad url::').originWhitelist, []);
  // 大型图表/代码默认延迟渲染
  assert.equal(config.deferred, true);
});
