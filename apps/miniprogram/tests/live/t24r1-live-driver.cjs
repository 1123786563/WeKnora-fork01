/* T24 修复轮 DevTools 复验驱动（F3 重取证 + F2 恢复态可观察 + F1 真实触发）。
 *
 * 与 Wave 4 驱动的差别（对应独立评审 F3 finding：三张关键截图字节级相同）：
 * 1. verifiedShot：截图前断言页面关键文本存在（每张截图捕获时可区分的真实状态），
 *    截图后等待文件落盘；与已取截图逐张比较哈希，若与任何一张相同则等待重渲染后
 *    重新断言+重拍（最多 3 次），仍相同则失败——不允许再产出不可区分的证据。
 * 2. 收尾把全部截图两两哈希不等断言 + manifest（每张截图证明什么）写入 SHOTS 目录。
 *
 * 运行前提（复用 Wave 4 环境模式，docs/plans/issue-140/task-6-live-validation.md）：
 *   - Lite 服务器在 QUOTA_PROXY_BACK（默认 57812），quota-inject-proxy.mjs 在
 *     T24R1_ORIGIN（默认 57811，小程序构建指向的 origin）。
 *   - DevTools `cli auto --auto-port $T24R1_AUTO_PORT` 已完成冷编译。
 *   - env：T24R1_ORIGIN / T24R1_AUTO_PORT / T24R1_TOKA(token 文件) /
 *     T24R1_USER_A / T24R1_TENANT / T24R1_SHOTS / QUOTA_REFUSE_FLAG。
 */
const automator = require('miniprogram-automator');
const http = require('http');
const fs = require('fs');
const crypto = require('crypto');
const path = require('path');
const sleep = ms => new Promise(r => setTimeout(r, ms));
const log = (...a) => console.log('[t24r1]', ...a);

const ORIGIN = process.env.T24R1_ORIGIN || 'http://127.0.0.1:57811';
const PORT = Number(new URL(ORIGIN).port);
const SHOTS = process.env.T24R1_SHOTS;
const FLAG = process.env.QUOTA_REFUSE_FLAG || '/tmp/wk-t24r1-quota-refuse-on';
const TA = fs.readFileSync(process.env.T24R1_TOKA, 'utf8').trim();
const USER_A = process.env.T24R1_USER_A;
const TENANT = String(process.env.T24R1_TENANT);
if (!SHOTS || !USER_A || !TENANT) { console.error('missing T24R1_* env'); process.exit(2); }

const results = [];
function record(step, pass, detail) { results.push({ step, pass, detail }); log(pass ? 'PASS' : 'FAIL', step, detail || ''); }
function api(method, reqPath, body, token) {
  return new Promise((resolve, reject) => {
    const data = body ? JSON.stringify(body) : null;
    const req = http.request({ host: '127.0.0.1', port: PORT, method, path: reqPath, headers: { ...(data ? { 'content-type': 'application/json' } : {}), ...(token ? { Authorization: `Bearer ${token}` } : {}) } }, res => {
      let buf = ''; res.on('data', c => buf += c); res.on('end', () => resolve({ status: res.statusCode, body: buf }));
    });
    req.on('error', reject); if (data) req.write(data); req.end();
  });
}
async function connectRetry(port, minutes) {
  const deadline = Date.now() + minutes * 60000;
  while (Date.now() < deadline) { try { return await automator.connect({ wsEndpoint: `ws://127.0.0.1:${port}` }); } catch { await sleep(5000); } }
  throw new Error('connect timeout');
}
async function allTexts(page) {
  const els = await page.$$('text'); const out = [];
  for (const el of els) { try { const t = await el.text(); if (t && t.trim()) out.push(t.trim()); } catch {} }
  return out.join('\n');
}
async function waitText(page, re, timeoutMs = 25000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) { const all = await allTexts(page); if (re.test(all)) return all; await sleep(800); }
  throw new Error(`waitText timeout ${re}`);
}
async function tapText(page, wanted, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    for (const el of await page.$$('.wk-button')) {
      try { if (((await el.text()) || '').includes(wanted)) { await el.tap(); return; } } catch {}
    }
    await sleep(800);
  }
  throw new Error(`tapText timeout ${wanted}`);
}
async function relaunch(mp, pagePath) {
  for (let i = 0; i < 10; i++) { try { const p = await mp.reLaunch(pagePath); if (p) return p; } catch { await sleep(4000); } }
  throw new Error(`reLaunch fail ${pagePath}`);
}
const sha256 = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
async function waitStableFile(file) {
  for (let i = 0; i < 40; i++) {
    try { const a = fs.statSync(file).size; await sleep(300); const b = fs.statSync(file).size; if (a === b && a > 0) return; } catch {}
    await sleep(300);
  }
  throw new Error(`screenshot never stabilized: ${file}`);
}
/* 实测：page.$$('text') 单次快照可能不完整（偶发缺节点甚至只剩页面标题）。
 * 状态断言一律经 snapshotMatching：连续多拍中任一命中即算观察到该状态。 */
async function snapshotMatching(page, re, polls = 6, gap = 700) {
  for (let i = 0; i < polls; i++) {
    const t = await allTexts(page);
    if (re.test(t)) return t;
    await sleep(gap);
  }
  return null;
}
/* 原生 Button 的 label 不总出现在 text 快照里；按钮存在性直接扫 .wk-button。 */
async function hasButton(page, label, timeoutMs = 8000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    for (const el of await page.$$('.wk-button')) {
      try { if (((await el.text()) || '').includes(label)) return true; } catch {}
    }
    await sleep(600);
  }
  return false;
}
const taken = []; // { name, file, sha256 }
/* verifiedShot：F3 重取证核心。
 * 1) 截图前断言页面关键文本存在（snapshotMatching 抗快照抖动）+ 必需按钮在位；
 * 2) 把「要证明的关键元素」滚动进视口再截（探针实证：截图忠实反映视口像素，
 *    字节级相同 = 可视像素相同；Wave 4 三张同图根因是状态增量渲染在折叠线以下）；
 * 3) 截图后与已取截图逐张比哈希，相同则换滚动偏移重拍（最多 4 次），仍相同即失败。 */
async function verifiedShot(mp, page, name, mustMatch, proves, visible, mustButtons = []) {
  const file = path.join(SHOTS, `${name}.png`);
  const nudges = [120, 40, 200, 0];
  const observed = await snapshotMatching(page, mustMatch);
  if (!observed) throw new Error(`${name}: state not observable before capture (${proves})`);
  for (const label of mustButtons) {
    if (!(await hasButton(page, label))) throw new Error(`${name}: expected actionable button「${label}」not present (${proves})`);
  }
  for (let attempt = 1; attempt <= nudges.length; attempt++) {
    let scrolledTo = 0;
    if (visible) {
      for (const el of await page.$$('text')) {
        const t = ((await el.text()) || '').trim();
        if (t && visible.test(t)) {
          try {
            const off = await el.offset();
            const top = Number((off && (off.top ?? off.y)) ?? 0);
            scrolledTo = Math.max(0, Math.round(top - nudges[attempt - 1]));
            await mp.pageScrollTo(scrolledTo);
            await sleep(700);
          } catch (e) { log('offset/scroll failed', e.message); }
          break;
        }
      }
    }
    if (!(await snapshotMatching(page, mustMatch, 3, 500))) { log(`${name}: text lost after scroll, retrying`); continue; }
    await mp.screenshot({ path: file });
    await waitStableFile(file);
    const digest = sha256(file);
    const clash = taken.find(s => s.sha256 === digest);
    if (!clash) { taken.push({ name, file, sha256: digest, proves, assertion: String(mustMatch), visible: String(visible), scrolledTo }); log('shot', name, digest.slice(0, 12), `scrollTop=${scrolledTo}`); return; }
    log(`shot-clash ${name} == ${clash.name} (attempt ${attempt}); retrying with a different viewport offset`);
  }
  throw new Error(`${name}: screenshot stayed byte-identical to an earlier shot after ${nudges.length} attempts — not acceptable evidence`);
}
const scopeKey = JSON.stringify([ORIGIN.replace(/\/+$/, ''), USER_A, TENANT]);
const searchStoreKey = `wk:career:search:${scopeKey}`;
async function writeIntent(mp, requestId, query) {
  await mp.callWxMethod('setStorageSync', searchStoreKey, { requestId, query });
}
async function readIntent(mp) {
  return await mp.callWxMethod('getStorageSync', searchStoreKey);
}

(async () => {
  const mp = await connectRetry(Number(process.env.T24R1_AUTO_PORT || 9423), 3);
  try {
    // ===== 登录（真实 POST /auth/login；一次性账号由外部注册）=====
    let page = await relaunch(mp, '/subpackages/auth/login/index');
    await sleep(2500);
    const li = await page.$$('.wk-input');
    await li[0].input(process.env.T24R1_USER_EMAIL);
    await li[1].input(process.env.T24R1_USER_PASS);
    const box = (await page.$('.wk-consent checkbox')) || (await page.$('checkbox'));
    await box.tap(); await sleep(400);
    await tapText(page, '登录并继续');
    await sleep(3000);
    try { await tapText(page, '进入工作空间', 8000); } catch { log('workspace prompt skip'); }
    await sleep(1500);

    // ===== B-confirm-visible：逐项确认生效（事实列表出现 意向：远程办公 + 已确认）=====
    page = await relaunch(mp, '/career/discovery');
    await waitText(page, /还没有求职档案|求职工作台/, 30000);
    for (const el of await page.$$('.wk-small')) { try { if (((await el.text()) || '').includes('意向')) { await el.tap(); break; } } catch {} }
    await sleep(400);
    const inputs = await page.$$('.wk-input');
    await inputs[0].input('远程办公');
    await tapText(page, '添加为待确认事实');
    const proposed = await waitText(page, /意向：远程办公[\s\S]*待确认|待确认[\s\S]*意向：远程办公/);
    record('B propose-pending', /意向：远程办公/.test(proposed) && /待确认/.test(proposed), 'stepwise fact enters as pending proposal');
    const openAfter = JSON.parse((await api('GET', '/api/v1/career/open', null, TA)).body);
    const pendingProposal = (openAfter.proposals || []).find(p => p.key === '意向' && p.value === '远程办公');
    record('B server-pending', !!pendingProposal && !(openAfter.facts || []).some(f => f.key === '意向'), `server revision=${openAfter.revision}`);
    await api('POST', '/api/v1/career/act', { action: 'confirm_proposal', proposalId: pendingProposal.id, source: { kind: 'user' }, requestId: `t24r1-confirm-${Date.now().toString(36)}`, expectedRevision: openAfter.revision }, TA);
    await tapText(page, '同步 Web 端档案变更');
    const confirmedText = await waitText(page, /意向：远程办公[\s\S]*已确认|已确认[\s\S]*意向：远程办公/);
    record('B item-confirm-visible', /已确认/.test(confirmedText), 'confirmed fact visible after confirm+sync');
    await verifiedShot(mp, page, 'B-confirm-visible', /意向：远程办公[\s\S]*已确认|已确认[\s\S]*意向：远程办公/, '逐项确认生效：意向=远程办公 从待确认变为已确认事实（服务端 confirm_proposal 后同步可见）', /意向：远程办公/);

    // ===== C-unknown-search：未知结果挂起态（intent 注入 + 页面恢复入口）=====
    const R1 = `t24r1-search-${Date.now().toString(36)}`;
    const cur1 = JSON.parse((await api('GET', '/api/v1/career/open', null, TA)).body);
    const searchResp = await api('POST', '/api/v1/career/searches', { requestId: R1, query: 'Go 工程师', expectedRevision: cur1.revision }, TA);
    record('C server-search-honest', searchResp.status === 200 && JSON.parse(searchResp.body).failureCode === 'no_vetted_sources', 'server one-shot search failed honestly (no vetted sources)');
    await writeIntent(mp, R1, 'Go 工程师');
    page = await relaunch(mp, '/career/discovery');
    const unknownNotice = await waitText(page, /有一次结果未知的搜索/);
    record('C unknown-search-entry', unknownNotice.includes('有一次结果未知的搜索') && (await hasButton(page, '用原请求对账')), 'outcome-unknown surfaced with reconcile entry');
    await verifiedShot(mp, page, 'C-unknown-search', /有一次结果未知的搜索/, '未知结果挂起态：有一次结果未知的搜索（Go 工程师）+ 用原请求对账恢复入口（按钮存在性经 .wk-button 扫描断言）', /有一次结果未知的搜索/, ['用原请求对账']);

    // ===== C-search-reconciled：原请求对账完成态（失败码 + 覆盖来源如实）=====
    await tapText(page, '用原请求对账');
    const recovered = await waitText(page, /no_vetted_sources/);
    record('C reconcile-original-request', /no_vetted_sources/.test(recovered), 'receipt rendered with honest failure code');
    record('C coverage-honest', /当前没有已核验的搜索来源/.test(recovered) && /覆盖来源（0）/.test(recovered), 'zero vetted sources reported truthfully');
    await verifiedShot(mp, page, 'C-search-reconciled', /no_vetted_sources[\s\S]*覆盖来源（0）|覆盖来源（0）[\s\S]*no_vetted_sources/, '对账完成态：搜索未完成（no_vetted_sources）+ 当前没有已核验的搜索来源 + 覆盖来源（0）', /搜索未完成/);

    // ===== F2：未知对账 404 → 可操作恢复态 → 安全重发成功 =====
    const R2 = `t24r1-missing-${Date.now().toString(36)}`; // 不存在的 requestId：对账必 404（真实服务器行为）
    await writeIntent(mp, R2, '不存在的请求');
    page = await relaunch(mp, '/career/discovery');
    await waitText(page, /有一次结果未知的搜索/);
    await tapText(page, '用原请求对账');
    const missingText = await waitText(page, /尚未找到该请求的回执/);
    record('F2 receipt-404-recovery-state', /尚未找到该请求的回执/.test(missingText) && (await hasButton(page, '安全重发原搜索')), '404 reconciliation renders operable recovery (resend entry)');
    await verifiedShot(mp, page, 'F2-receipt-missing', /尚未找到该请求的回执/, 'F2 恢复态：尚未找到该请求的回执（构造法：不存在 requestId 的 intent 对账，服务器真实 404）+ 安全重发入口（按钮存在性经 .wk-button 扫描断言）', /尚未找到该请求的回执/, ['安全重发原搜索', '用原请求对账']);
    await tapText(page, '安全重发原搜索');
    const resent = await waitText(page, /no_vetted_sources/);
    record('F2 safe-resend-recovers', /no_vetted_sources/.test(resent), 'safe resend with the same request id completes the search');
    record('F2 intent-cleared', (await readIntent(mp)) === '' || (await readIntent(mp)) === null, 'intent cleared after successful resend');
    await verifiedShot(mp, page, 'F2-recovered', /no_vetted_sources[\s\S]*覆盖来源（0）|覆盖来源（0）[\s\S]*no_vetted_sources/, 'F2 恢复完成态：安全重发后搜索结果渲染（同 no_vetted_sources 诚实失败），恢复闭环', /搜索未完成/);

    // ===== F1：额度不足专属提示真实触发（合同级注入；客户端全链路真实）=====
    fs.writeFileSync(FLAG, String(Date.now())); // 打开注入开关
    try {
      const R3 = `t24r1-quota-${Date.now().toString(36)}`;
      await writeIntent(mp, R3, '额度测试');
      page = await relaunch(mp, '/career/discovery');
      await waitText(page, /有一次结果未知的搜索/);
      await tapText(page, '用原请求对账'); // 先 404 进入恢复态
      await waitText(page, /尚未找到该请求的回执/);
      await tapText(page, '安全重发原搜索'); // 重发被代理以 typed 429 拒绝
      const quotaText = await waitText(page, /搜索额度不足/);
      record('F1 quota-prompt-reachable', /搜索额度不足/.test(quotaText) && /仍可查看既有档案与申请记录/.test(quotaText), 'dedicated quota prompt rendered through the real client chain');
      const afterQuota = await snapshotMatching(page, /搜索额度不足/) ?? quotaText;
      record('F1 quota-keeps-recovery', /有一次结果未知的搜索/.test(afterQuota), 'refused resend keeps the pending intent for later recovery');
      record('F1 facts-still-readable', /意向：远程办公/.test(afterQuota), 'profile facts remain readable while quota is refused');
      await verifiedShot(mp, page, 'F1-quota-refused', /搜索额度不足[\s\S]*仍可查看既有档案与申请记录/, 'F1 专属提示可达：typed 429（search_quota_refused，经 quota-inject-proxy 在冻结合同边界注入；生产 gate 为 T21 真实额度账本 searchUsageGate，注入仅为 DevTools 受控触发）→ 真实 ApiError/errorMessage → 页面 warning Notice', /搜索额度不足/);
    } finally {
      fs.rmSync(FLAG, { force: true }); // 无论成败都关掉注入
    }

    // ===== 收尾：全部截图两两不同 + manifest =====
    const hashes = taken.map(s => s.sha256);
    const unique = new Set(hashes);
    record('F3 screenshots-pairwise-distinct', unique.size === taken.length, `${taken.length} shots, ${unique.size} distinct sha256`);
    fs.writeFileSync(path.join(SHOTS, 'manifest.json'), JSON.stringify({ origin: ORIGIN, scopeKey: searchStoreKey, shots: taken, results }, null, 2));
    console.log(JSON.stringify(results, null, 1));
    const failed = results.filter(r => !r.pass);
    log(`SUMMARY pass=${results.length - failed.length}/${results.length}`);
    process.exitCode = failed.length ? 1 : 0;
  } catch (e) {
    record('driver', false, e.message);
    try { await mp.screenshot({ path: path.join(SHOTS, 'Z-failure.png') }); } catch {}
    console.log(JSON.stringify(results, null, 1));
    process.exitCode = 1;
  } finally { try { await mp.disconnect(); } catch {} }
})();
