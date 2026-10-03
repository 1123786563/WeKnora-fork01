/* T24 修复轮 F1 DevTools 复验辅助（自动化脚手架，非业务逻辑，不进构建产物）。
 *
 * 背景（如实声明构造法，CAREER-OCR H3 更正）：生产 Office 的 SearchQuotaGate
 * 是真实额度账本 searchUsageGate（internal/modules/career/office.go:389 以
 * searchUsageGate 覆盖 passThrough 初值；usage.go 自述其为生产 gate）——T21 已
 * 落地，真实服务器在额度耗尽时能原生产出 typed 429。本代理仍按注入法触发：
 * 只在小程序信任的 API origin 的
 * 冻结合同边界（POST /api/v1/career/searches）按 T11 handler.go 的响应形状
 * （429 + {"error":{"code":"search_quota_refused"}}）回放拒绝，其余请求原样
 * 转发上游 Lite 服务器。小程序侧全链路真实：真实构建、真实 transport、
 * 真实 ApiError 解码、真实 errorMessage 映射、真实页面 Notice 渲染。
 *
 * 控制文件存在时才注入（touch/rm 即可运行中切换）；端口经 env 配置。
 */
import http from 'node:http';
import fs from 'node:fs';

const FRONT = Number(process.env.QUOTA_PROXY_FRONT ?? 57811);
const BACK = Number(process.env.QUOTA_PROXY_BACK ?? 57812);
const FLAG = process.env.QUOTA_REFUSE_FLAG ?? '/tmp/wk-t24r1-quota-refuse-on';

const server = http.createServer((req, res) => {
  const refused = fs.existsSync(FLAG) && req.method === 'POST' && req.url.startsWith('/api/v1/career/searches');
  if (refused) {
    req.resume();
    res.writeHead(429, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ error: { code: 'search_quota_refused', message: 'search quota refused' } }));
    console.log(`[quota-proxy] REFUSED ${req.method} ${req.url}`);
    return;
  }
  const upstream = http.request({ host: '127.0.0.1', port: BACK, method: req.method, path: req.url, headers: { ...req.headers, host: `127.0.0.1:${BACK}` } }, up => {
    res.writeHead(up.statusCode ?? 502, up.headers);
    up.pipe(res);
  });
  upstream.on('error', err => { console.error('[quota-proxy] upstream error', err.message); res.writeHead(502); res.end('quota-proxy upstream error'); });
  req.pipe(upstream);
});
server.listen(FRONT, '127.0.0.1', () => console.log(`[quota-proxy] listening 127.0.0.1:${FRONT} -> 127.0.0.1:${BACK}; refuse-flag=${FLAG}`));
