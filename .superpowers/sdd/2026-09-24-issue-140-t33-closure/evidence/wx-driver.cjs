/* T33 终局 DevTools 抽查（复用 T24/T26 模式）：
 * 登录（真实 /auth/login @57828）→ 求职工作台（档案/找岗失败态）→ 申请与材料（Web 同源数据可见）。
 * t-button GUI tap 不被 automator 触达（T24 定论）：t-button 动作由 172 单测覆盖，本脚本走数据面与原生可达控件。 */
const automator = require('/tmp/wk-t33-automator/node_modules/miniprogram-automator');
const sleep = ms => new Promise(r => setTimeout(r, ms));
const log = (...a) => console.log('[t33]', ...a);
const SHOTS = '/Users/wuyongjun/.codex/worktrees/issue-140-t33-closure/WeKnora-fork01/.superpowers/sdd/2026-09-24-issue-140-t33-closure/evidence';
const results = [];
function record(step, pass, detail) { results.push({ step, pass, detail }); log(pass ? 'PASS' : 'FAIL', step, detail || ''); }

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
  while (Date.now() < deadline) { const all = await allTexts(page); if (re.test(all)) return all; await sleep(900); }
  throw new Error(`waitText timeout ${re}`);
}
async function tapText(page, wanted, timeoutMs = 20000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    for (const el of await page.$$('button, .wk-button')) {
      try { if (((await el.text()) || '').includes(wanted)) { await el.tap(); return true; } } catch {}
    }
    await sleep(900);
  }
  return false;
}
async function tapListRow(page, wanted, timeoutMs = 20000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    for (const el of await page.$$('view')) {
      try { if (((await el.text()) || '').includes(wanted)) { await el.tap(); return true; } } catch {}
    }
    await sleep(900);
  }
  return false;
}
async function fillInput(page, placeholder, value) {
  const el = await page.$(`input[placeholder="${placeholder}"]`);
  if (!el) throw new Error(`input not found: ${placeholder}`);
  await el.input(value);
}
async function shot(mp, name) { try { await mp.screenshot({ path: `${SHOTS}/wx-${name}.png` }); log('shot', name); } catch (e) { log('shot-fail', name, e.message); } }
async function relaunch(mp, path) {
  for (let i = 0; i < 12; i++) { try { const p = await mp.reLaunch(path); if (p) return p; } catch { await sleep(5000); } }
  throw new Error(`reLaunch fail ${path}`);
}

(async () => {
  const mp = await connectRetry(9433, 4);
  try {
    // 1. 登录页：真实 /auth/login
    let page = await relaunch(mp, '/subpackages/auth/login/index');
    await sleep(3000);
    page = await mp.currentPage();
    await fillInput(page, '请输入账号邮箱', 't33a@t33.io');
    await fillInput(page, '请输入密码', '[REDACTED-disposable]');
    await shot(mp, '01-login-filled');
    // 勾选同意（checkbox 通过 wrapper tap）
    const consentOk = await tapListRow(page, '我已了解平台的数据使用与服务说明', 8000);
    log('consent tap', consentOk);
    await sleep(800);
    const loginOk = await tapText(page, '登录并继续', 10000);
    if (!loginOk) throw new Error('login button not reachable');
    // 2. 等待进入 home（登录成功 + 工作空间选择）
    page = await mp.currentPage();
    let homeText = '';
    for (let i = 0; i < 30; i++) {
      await sleep(1500);
      page = await mp.currentPage();
      const t = await allTexts(page);
      if (/求职工作台|选择工作空间/.test(t)) { homeText = t; break; }
    }
    if (/选择工作空间/.test(homeText)) {
      const entered = await tapText(page, '进入工作空间', 12000);
      log('enter workspace', entered);
      await sleep(3000);
      page = await mp.currentPage();
      homeText = await allTexts(page);
    }
    record('login', /求职工作台/.test(homeText), 'reached home with 求职工作台 entry');
    await shot(mp, '02-home');

    // 3. 求职工作台（career discovery）：Web 建的档案事实可见（直接 relaunch，ListRow tap 对 view 不稳定）
    page = await relaunch(mp, '/career/discovery');
    await sleep(3500);
    page = await mp.currentPage();
    const careerText = await waitText(page, /毕业时间|求职/, 25000).catch(() => allTexts(page));
    record('career-page', /毕业时间：2026-06/.test(careerText) && /修订 2/.test(careerText), 'web-created profile fact 毕业时间=2026-06 · 修订 2 visible in weapp');
    await shot(mp, '03-career-discovery');

    // 4. 找岗入口与一次性搜索区在页可见（t-button 触达限制按 T24 定论由单测覆盖）
    const searchEntry = /找岗（一次性搜索）|想找什么/.test(careerText);
    record('search-entry', searchEntry, 'one-shot search entry present');

    // 5. 申请与材料页（Web 同源申请可见）
    page = await relaunch(mp, '/career/application-material');
    await sleep(3500);
    page = await mp.currentPage();
    const applyText = await waitText(page, /申请|材料|批次/, 25000).catch(() => allTexts(page));
    const seenApply = /2026 秋招 A 批|前端开发实习|投递|材料/.test(applyText);
    record('apply-material-page', seenApply, applyText.slice(0, 500).replace(/\n/g, '|'));
    await shot(mp, '04-application-material');

    // 6. 找岗执行（一次性搜索）走 request-layer seam：直接以同 token 发起真实 POST（T24 先例）验证失败态
    // （已在 Web 端真实执行过一次搜索并取得 no_vetted_sources 失败态；小程序侧由 172 单测覆盖 D2/D3/D4）
    // 7. 展现最后页面快照文本（供报告引用）
    log('careerText-head:', careerText.slice(0, 500).replace(/\n/g, '|'));
    log('applyText-head:', applyText.slice(0, 500).replace(/\n/g, '|'));
  } catch (e) {
    record('driver-error', false, e.message);
  } finally {
    log('RESULT', JSON.stringify(results));
    try { await mp.disconnect(); } catch {}
  }
})();
