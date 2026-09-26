/* T33 终局 DevTools 抽查（复用 T24/T26 模式）：
 * 登录（真实 /auth/login @57828）→ 求职工作台（档案/找岗失败态）→ 申请与材料（Web 同源数据可见）。
 * t-button GUI tap 不被 automator 触达（T24 定论）：t-button 动作由 172 单测覆盖，本脚本走数据面与原生可达控件。
 * 机器强相关路径/端口一律 env 注入（对齐 T24 live-driver 写法），换机重放示例：
 *   T33_AUTOMATOR=<node_modules/miniprogram-automator 绝对路径> T33_WS_PORT=9433 \
 *   T33_SHOTS=<截图输出目录> node wx-driver.cjs
 * 未注入时：automator 依次尝试 NODE_PATH 解析与历史 /tmp 安装位；截图回落本脚本所在目录；端口回落 9433。 */

function loadAutomator() {
  const candidates = [process.env.T33_AUTOMATOR, 'miniprogram-automator', '/tmp/wk-t33-automator/node_modules/miniprogram-automator'].filter(Boolean);
  for (const candidate of candidates) { try { return require(candidate); } catch {} }
  throw new Error('miniprogram-automator not found: set T33_AUTOMATOR to its module path (npm i miniprogram-automator)');
}
const automator = loadAutomator();
const sleep = ms => new Promise(r => setTimeout(r, ms));
const log = (...a) => console.log('[t33]', ...a);
const SHOTS = process.env.T33_SHOTS || __dirname;
const WS_PORT = Number(process.env.T33_WS_PORT || 9433);
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
  const mp = await connectRetry(WS_PORT, 4);
  try {
    // 1. 登录页：真实 /auth/login
    let page = await relaunch(mp, '/subpackages/auth/login/index');
    await sleep(3000);
    page = await mp.currentPage();
    await fillInput(page, '请输入账号邮箱', 't33a@t33.io');
    await fillInput(page, '请输入密码', '[REDACTED-disposable]');
    await shot(mp, '01-login-filled');
    // 勾选同意（checkbox 通过 wrapper tap）：登录前置条件，失败即 record 并快速失败（对齐 T24「tap 超时即 throw」，
    // 不允许勾选未生效仍继续点登录、把根因埋进 login 步骤）
    const consentOk = await tapListRow(page, '我已了解平台的数据使用与服务说明', 8000);
    record('consent-tap', consentOk, consentOk ? 'consent checkbox tapped before login' : 'consent row not reachable');
    if (!consentOk) throw new Error('consent checkbox not reachable (login precondition)');
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

    // 5. 申请与材料页（Web 同源申请可见）。
    // 页面可达（静态文案）与数据可见（数据专属标记）分开断言：静态文案（申请与材料/投递等）页面渲染即恒在，
    // 混入数据断言会在 Web 同源数据未加载时假阳性 PASS（ocr1-030）。
    page = await relaunch(mp, '/career/application-material');
    await sleep(3500);
    page = await mp.currentPage();
    const applyText = await waitText(page, /2026 秋招 A 批|前端开发实习/, 25000).catch(() => allTexts(page));
    record('apply-material-page', /申请|材料/.test(applyText), 'application-material page reached (static copy visible)');
    const seenApply = /2026 秋招 A 批|前端开发实习/.test(applyText);
    record('apply-material-data', seenApply, 'web-created application fact 2026 秋招 A 批 / 前端开发实习 visible in weapp');
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
    // 任一步骤 FAIL（含 driver-error）必须以非零退出码结束（ocr1-031），
    // 供按退出码判定的门禁/复验工具消费
    process.exitCode = results.some(r => !r.pass) ? 1 : 0;
  }
})();
