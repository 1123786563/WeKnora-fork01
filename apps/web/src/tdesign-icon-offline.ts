// apps/web/src/tdesign-icon-offline.ts
// 阻断 tdesign-icons-react 0.6.11 对 tdesign.gtimg.com 的 iconfont/svg-sprite 注入。
// 机制同 frontend/src/utils/tdesign-icon-offline.ts：预插带库内部去重选择器
// 的占位节点（非标准 type/rel，浏览器不发请求），使 checkScriptAndLoad/
// checkLinkAndLoad 命中去重直接返回。CDN URL 为 0.6.11 内部硬编码的 0.4.5。
const SVG_SCRIPT_CLASS = 't-svg-js-stylesheet--unique-class';
const ICONFONT_LINK_CLASS = 't-iconfont-stylesheet--unique-class';
const BLOCKED_SCRIPT_URL = 'https://tdesign.gtimg.com/icon/0.4.5/fonts/index.js';
const BLOCKED_LINK_URL = 'https://tdesign.gtimg.com/icon/0.4.5/fonts/index.css';

let installed = false;

export function installTDesignIconOfflineGuard(): void {
  if (installed || typeof document === 'undefined') return;
  installed = true;

  const body = document.body;
  if (!body) {
    document.addEventListener('DOMContentLoaded', () => installTDesignIconOfflineGuard(), { once: true });
    installed = false;
    return;
  }

  const stubScript = document.querySelector(`script.${SVG_SCRIPT_CLASS}[src="${BLOCKED_SCRIPT_URL}"]`);
  if (!stubScript) {
    const s = document.createElement('script');
    s.setAttribute('class', SVG_SCRIPT_CLASS);
    s.setAttribute('src', BLOCKED_SCRIPT_URL);
    s.setAttribute('type', 'text/no-load'); // 非标准 MIME，跳过 fetch/执行
    s.setAttribute('data-weknora-blocked-cdn', 'tdesign-icons');
    body.appendChild(s);
  }

  const stubLink = document.querySelector(`link.${ICONFONT_LINK_CLASS}[href="${BLOCKED_LINK_URL}"]`);
  if (!stubLink) {
    const l = document.createElement('link');
    l.setAttribute('class', ICONFONT_LINK_CLASS);
    l.setAttribute('href', BLOCKED_LINK_URL);
    l.setAttribute('rel', 'preload-blocked'); // 不声明 stylesheet，不发请求
    l.setAttribute('data-weknora-blocked-cdn', 'tdesign-icons');
    document.head.appendChild(l);
  }
}
