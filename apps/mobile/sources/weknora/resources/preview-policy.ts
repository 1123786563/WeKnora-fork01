/**
 * W27 — 隔离预览策略（isolated preview policy）。
 *
 * 生成的产物预览（HTML/图表/代码）在独立 origin 上渲染，该 origin 与产品
 * API 不同源、不携带产品 cookie/token。本模块是客户端侧的策略单一来源：
 *
 *   1. `previewHeaders()` — 预览响应必须携带的安全头常量（测试对照）。
 *      服务端 `internal/handler/artifact_preview.go` 在每个预览响应上设置
 *      完全相同的头；本常量只用于客户端断言与服务端契约锁定，不存在
 *      「客户端自带一份不生效的头」的路径。
 *   2. `decidePreviewNavigation` / `buildPreviewWebViewConfig` — WebView 的
 *      纯函数配置来源（`ArtifactPreview.tsx` 消费）：originAllowList 仅
 *      预览 origin、被拦截外链仅记录候选并经显式用户手势入口外抛、
 *      关闭任意文件访问与原生桥。
 *
 * 本文件不得 import react-native / react-native-webview —— 它同时被
 * `tsx --test`（node:test）与 vitest 复用，必须保持纯模块。
 */

/**
 * 预览响应的 Content-Security-Policy。
 *
 *   - `default-src 'none'`：默认拒绝一切资源加载；
 *   - `script-src 'unsafe-inline'`：图表（mermaid/katex 单文件构建）以内联
 *     脚本渲染，且 `sandbox allow-scripts` 是其唯一能力来源；
 *   - `connect-src 'none'`：fetch/XHR/WebSocket/EventSource 一律拒绝——
 *     预览页脚本无法触达产品 API 或任何网络端点；
 *   - `sandbox allow-scripts`（无 `allow-same-origin`）：文档处于 opaque
 *     origin——cookie、localStorage、sessionStorage、IndexedDB 全部不可读，
 *     也无法 postMessage 到任何同源接收者；
 *   - `form-action 'none'`：表单提交（含跨站 POST）被阻止；
 *   - `frame-ancestors 'none'`：预览页不可被任何页面内嵌；
 *   - `object-src/base-uri/worker-src 'none'`：无插件、无 base 注入、无 worker。
 */
export const PREVIEW_CSP =
  "default-src 'none'; " +
  "script-src 'unsafe-inline'; " +
  "style-src 'unsafe-inline' data:; " +
  "img-src data: blob:; " +
  "font-src data:; " +
  "connect-src 'none'; " +
  "object-src 'none'; " +
  "base-uri 'none'; " +
  "form-action 'none'; " +
  "frame-ancestors 'none'; " +
  "worker-src 'none'; " +
  "sandbox allow-scripts";

/** 预览响应安全头（与服务端 artifact_preview.go 输出逐字节一致）。 */
export function previewHeaders(): Record<string, string> {
  return {
    'Content-Security-Policy': PREVIEW_CSP,
    'Referrer-Policy': 'no-referrer',
    'X-Content-Type-Options': 'nosniff',
    'Cache-Control': 'private, no-store',
    'Cross-Origin-Resource-Policy': 'same-origin',
    'Cross-Origin-Opener-Policy': 'same-origin',
    'Permissions-Policy': 'camera=(), microphone=(), geolocation=(), payment=(), usb=(), display-capture=()',
  };
}

/** 从票据 URL 提取预览 origin（scheme://host[:port]）；解析失败返回 ''。 */
export function previewOriginOf(previewURL: string): string {
  if (!previewURL) return '';
  const match = /^([a-z][a-z0-9+.-]*):\/\//i.exec(previewURL);
  if (!match) return '';
  const scheme = match[1].toLowerCase();
  const rest = previewURL.slice(match[0].length);
  const authority = rest.split(/[/?#]/, 1)[0] ?? '';
  // authority 必须是 host[:port]，且拒绝 userinfo 形态。
  if (!authority || authority.includes('@')) return '';
  return `${scheme}://${authority}`;
}

/** WebView onShouldStartLoadWithRequest 收到的导航请求（结构子集）。 */
export interface PreviewNavigationRequest {
  url: string;
  /** iOS 提供；Android 视为未知（undefined）。 */
  isTopLevel?: boolean;
}

export interface PreviewNavigationDecision {
  allow: boolean;
  /**
   * 非空表示该被拦截导航携带一个外链候选（http(s) 非预览 origin）。
   * 候选不自动外抛：消费方必须经显式用户手势入口（「在浏览器打开」
   * 按钮）才交系统浏览器（F2）。
   */
  external?: string;
}

/**
 * 判定一次 WebView 导航：
 *
 *   - 仅 http(s) 进入后续判定，file:/content:/intent:/javascript:/data:
 *     等危险 scheme 一律拒绝且不产生候选（不能借系统浏览器触发 intent）；
 *   - 与预览 origin 同源 → 放行（票据路径本身）；
 *   - 其他 http(s) origin → 拒绝，并标记为外链候选。
 *
 * `external` 只是「候选标记」：被拦截导航本身绝不触发系统浏览器——系统
 * 浏览器持有产品站点 cookie，预览内容若能自动打开任意 URL（含产品
 * origin）即构成 GET-CSRF/钓鱼面（F2）。消费方必须把候选呈现为显式
 * 用户手势入口（如「在浏览器打开」按钮），点按后才调用 Linking。
 */
export function decidePreviewNavigation(
  request: PreviewNavigationRequest,
  allowedOrigin: string,
): PreviewNavigationDecision {
  const url = request?.url ?? '';
  if (!allowedOrigin) return { allow: false };
  if (!/^https?:\/\//i.test(url)) return { allow: false };
  const origin = previewOriginOf(url);
  if (!origin) return { allow: false };
  if (origin === allowedOrigin) return { allow: true };
  return { allow: false, external: url };
}

/** WebView 的加固配置（纯数据，ArtifactPreview.tsx 展开到 <WebView/>）。 */
export interface PreviewWebViewConfig {
  /** originAllowList：仅预览 origin；票据 URL 非法时为空（fail-closed）。 */
  originWhitelist: string[];
  allowFileAccess: false;
  allowFileAccessFromFileURLs: false;
  allowUniversalAccessFromFileURLs: false;
  /** 无原生桥：不注入任何脚本。 */
  injectedJavaScript: undefined;
  /** 无原生桥：组件不注册 onMessage。 */
  hasMessageBridge: false;
  /** cookie 隔离：不共享 iOS 全局 cookie、不写第三方 cookie。 */
  sharedCookiesEnabled: false;
  thirdPartyCookiesEnabled: false;
  /** 不允许 window.open 新开窗口。 */
  setSupportMultipleWindows: false;
  /** 大型图表/代码默认延迟渲染：先渲染占位卡，用户点按后再挂 WebView。 */
  deferred: true;
}

export function buildPreviewWebViewConfig(previewURL: string): PreviewWebViewConfig {
  const origin = previewOriginOf(previewURL);
  return {
    originWhitelist: origin ? [origin] : [],
    allowFileAccess: false,
    allowFileAccessFromFileURLs: false,
    allowUniversalAccessFromFileURLs: false,
    injectedJavaScript: undefined,
    hasMessageBridge: false,
    sharedCookiesEnabled: false,
    thirdPartyCookiesEnabled: false,
    setSupportMultipleWindows: false,
    deferred: true,
  };
}
